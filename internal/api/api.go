package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"geogo/internal/allow"
	"geogo/internal/geoip"

	"github.com/oschwald/maxminddb-golang/v2"
)

type GeoResponse struct {
	IP        string    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`

	Continent string `json:"continent,omitempty"`
	Country   string `json:"country,omitempty"`
	City      string `json:"city,omitempty"`

	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	TimeZone  string  `json:"time_zone,omitempty"`
}

type IsAllowedResponse struct {
	IP      string `json:"ip"`
	Allowed bool   `json:"allowed"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type App struct {
	db                         *maxminddb.Reader
	allowed                    allow.Rules
	healthCheckIP              netip.Addr
	healthCheckIPRaw           string
	healthCheckExpectedCountry string
}

func New(db *maxminddb.Reader, allowed allow.Rules, healthCheckIPRaw string, healthCheckIP netip.Addr, healthCheckExpectedCountry string) *App {
	return &App{
		db:                         db,
		allowed:                    allowed,
		healthCheckIP:              healthCheckIP,
		healthCheckIPRaw:           healthCheckIPRaw,
		healthCheckExpectedCountry: healthCheckExpectedCountry,
	}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.handleHealthcheck)
	mux.HandleFunc("/access/", a.handleAccess)
	mux.HandleFunc("/forward-auth", a.handleForwardAuth)
	mux.HandleFunc("/forward-auth/", a.handleForwardAuth)
	mux.HandleFunc("/info/", a.handleInfo)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func requireMethodGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
	return false
}

func parseIPFromPath(path, prefix, expected string) (string, netip.Addr, int, string) {
	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if raw == "" {
		return "", netip.Addr{}, http.StatusBadRequest, fmt.Sprintf("missing ip in path (expected %s)", expected)
	}
	if strings.Contains(raw, "/") {
		return "", netip.Addr{}, http.StatusNotFound, "not found"
	}
	ip, err := netip.ParseAddr(raw)
	if err != nil {
		return "", netip.Addr{}, http.StatusBadRequest, "invalid ip address"
	}
	return raw, ip, 0, ""
}

func toGeoResponse(ip string, data geoip.Data) GeoResponse {
	return GeoResponse{
		IP:        ip,
		Timestamp: time.Now().UTC(),
		Continent: data.Continent,
		Country:   data.Country,
		City:      data.City,
		Latitude:  data.Latitude,
		Longitude: data.Longitude,
		TimeZone:  data.TimeZone,
	}
}

func (a *App) lookup(ip netip.Addr) (geoip.Data, error) {
	return geoip.Lookup(a.db, ip)
}

func parseForwardedForIP(r *http.Request) (string, netip.Addr, bool) {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if ip, err := netip.ParseAddr(first); err == nil {
			return first, ip, true
		}
	}

	if xri := strings.TrimSpace(r.Header.Get("X-Real-Ip")); xri != "" {
		if ip, err := netip.ParseAddr(xri); err == nil {
			return xri, ip, true
		}
	}

	if cfi := strings.TrimSpace(r.Header.Get("Cf-Connecting-Ip")); cfi != "" {
		if ip, err := netip.ParseAddr(cfi); err == nil {
			return cfi, ip, true
		}
	}

	if fwd := strings.TrimSpace(r.Header.Get("Forwarded")); fwd != "" {
		for _, part := range strings.Split(fwd, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
			if !ok || strings.ToLower(strings.TrimSpace(k)) != "for" {
				continue
			}
			v = strings.TrimSpace(v)
			v = strings.Trim(v, "\"")
			v = strings.TrimPrefix(v, "[")
			v = strings.TrimSuffix(v, "]")
			if host, _, err := net.SplitHostPort(v); err == nil {
				v = host
			}
			if ip, err := netip.ParseAddr(v); err == nil {
				return v, ip, true
			}
		}
	}

	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return "", netip.Addr{}, false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return host, ip, true
	}
	return "", netip.Addr{}, false
}

func (a *App) handleAccess(w http.ResponseWriter, r *http.Request) {
	if !requireMethodGet(w, r) {
		return
	}

	raw, ip, status, msg := parseIPFromPath(r.URL.Path, "/access/", "/access/{IP-ADDRESS}")
	if status != 0 {
		writeJSON(w, status, ErrorResponse{Error: msg})
		return
	}

	data, err := a.lookup(ip)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("mmdb lookup failed: %v", err)})
		return
	}

	if !allow.Check(data.Continent, data.Country, data.City, a.allowed) {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "ip not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, IsAllowedResponse{IP: raw, Allowed: true})
}

func (a *App) handleForwardAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	if r.URL.Path != "/forward-auth" && r.URL.Path != "/forward-auth/" {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
		return
	}

	raw, ip, ok := parseForwardedForIP(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "missing or invalid client ip"})
		return
	}

	data, err := a.lookup(ip)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("mmdb lookup failed: %v", err)})
		return
	}

	if !allow.Check(data.Continent, data.Country, data.City, a.allowed) {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "ip not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, IsAllowedResponse{IP: raw, Allowed: true})
}

func (a *App) handleInfo(w http.ResponseWriter, r *http.Request) {
	if !requireMethodGet(w, r) {
		return
	}

	raw, ip, status, msg := parseIPFromPath(r.URL.Path, "/info/", "/info/{IP-ADDRESS}")
	if status != 0 {
		writeJSON(w, status, ErrorResponse{Error: msg})
		return
	}

	data, err := a.lookup(ip)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("mmdb lookup failed: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, toGeoResponse(raw, data))
}

func (a *App) handleHealthcheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethodGet(w, r) {
		return
	}

	data, err := a.lookup(a.healthCheckIP)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if data.Country != a.healthCheckExpectedCountry {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toGeoResponse(a.healthCheckIPRaw, data))
}
