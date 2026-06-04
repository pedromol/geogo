package config

import (
	"flag"
	"fmt"
	"net/netip"
	"os"
	"strings"

	"geogo/internal/allow"
)

type Config struct {
	Addr                       string
	DBPath                     string
	Allowed                    allow.Rules
	HealthCheckIP              string
	HealthCheckExpectedCountry string
}

type stringFlag struct {
	Value  *string
	WasSet *bool
}

func (f *stringFlag) String() string {
	if f == nil || f.Value == nil {
		return ""
	}
	return *f.Value
}

func (f *stringFlag) Set(s string) error {
	if f.WasSet != nil {
		*f.WasSet = true
	}
	if f.Value != nil {
		*f.Value = s
	}
	return nil
}

func Load() (Config, error) {
	addr := flag.String("addr", ":8080", "HTTP listen address")

	dbPathFlagValue := "./GeoLite2-City.mmdb"
	dbPathFlagWasSet := false
	flag.Var(&stringFlag{Value: &dbPathFlagValue, WasSet: &dbPathFlagWasSet}, "db", "Path to MaxMind MMDB file (e.g. GeoLite2-City.mmdb)")

	flag.Parse()

	dbPath := strings.TrimSpace(dbPathFlagValue)
	if !dbPathFlagWasSet {
		if v := strings.TrimSpace(os.Getenv("GEOIP_DB_PATH")); v != "" {
			dbPath = v
		}
	}
	if dbPath == "" {
		return Config{}, fmt.Errorf("missing GeoIP database path: pass -db, set GEOIP_DB_PATH, or ensure ./GeoLite2-City.mmdb exists")
	}

	healthCheckIP := "8.8.8.8"
	if v := strings.TrimSpace(os.Getenv("HEALTHCHECK_IP")); v != "" {
		healthCheckIP = v
	}
	if _, err := netip.ParseAddr(healthCheckIP); err != nil {
		return Config{}, fmt.Errorf("invalid HEALTHCHECK_IP: %w", err)
	}

	healthCheckExpectedCountry := "United States"
	if v := strings.TrimSpace(os.Getenv("HEALTHCHECK_EXPECTED_COUNTRY")); v != "" {
		healthCheckExpectedCountry = v
	}

	return Config{
		Addr:   *addr,
		DBPath: dbPath,
		Allowed: allow.Rules{
			Country:   allow.NewSetFromCSV(os.Getenv("ALLOWED_COUNTRY")),
			City:      allow.NewSetFromCSV(os.Getenv("ALLOWED_CITY")),
			Continent: allow.NewSetFromCSV(os.Getenv("ALLOWED_CONTINENT")),
		},
		HealthCheckIP:              healthCheckIP,
		HealthCheckExpectedCountry: healthCheckExpectedCountry,
	}, nil
}
