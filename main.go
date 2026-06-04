package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

type geoResponse struct {
	IP        string    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`

	Continent string `json:"continent,omitempty"`
	Country   string `json:"country,omitempty"`
	City      string `json:"city,omitempty"`

	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	TimeZone  string  `json:"time_zone,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type cityRecord struct {
	Continent struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"continent"`

	Country struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`

	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`

	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPathFlag := flag.String("db", "", "Path to MaxMind MMDB file (e.g. GeoLite2-City.mmdb)")
	flag.Parse()

	dbPath := strings.TrimSpace(*dbPathFlag)
	if dbPath == "" {
		dbPath = strings.TrimSpace(os.Getenv("GEOIP_DB_PATH"))
	}
	if dbPath == "" {
		log.Fatal("missing GeoIP database path: pass -db or set GEOIP_DB_PATH")
	}

	db, err := maxminddb.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open GeoIP database: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		raw := strings.Trim(r.URL.Path, "/")
		if raw == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing ip in path (expected /{IP-ADDRESS})"})
			return
		}
		if strings.Contains(raw, "/") {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
			return
		}

		ip, err := netip.ParseAddr(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid ip address"})
			return
		}

		var record cityRecord
		if err := db.Lookup(ip).Decode(&record); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("mmdb lookup failed: %v", err)})
			return
		}

		res := geoResponse{
			IP:        raw,
			Timestamp: time.Now().UTC(),
			Continent: record.Continent.Names["en"],
			Country:   record.Country.Names["en"],
			City:      record.City.Names["en"],
			Latitude:  record.Location.Latitude,
			Longitude: record.Location.Longitude,
			TimeZone:  record.Location.TimeZone,
		}
		writeJSON(w, http.StatusOK, res)
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}
