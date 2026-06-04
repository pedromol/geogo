package main

import (
	"log"
	"net/http"
	"net/netip"
	"time"

	"geogo/internal/api"
	"geogo/internal/config"

	"github.com/oschwald/maxminddb-golang/v2"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := maxminddb.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open GeoIP database: %v", err)
	}
	defer db.Close()

	healthCheckAddr, err := netip.ParseAddr(cfg.HealthCheckIP)
	if err != nil {
		log.Fatalf("invalid healthcheck ip: %v", err)
	}

	a := api.New(db, cfg.Allowed, cfg.HealthCheckIP, healthCheckAddr, cfg.HealthCheckExpectedCountry)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}
