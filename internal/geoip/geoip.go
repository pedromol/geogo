package geoip

import (
	"net/netip"

	"github.com/oschwald/maxminddb-golang/v2"
)

type Data struct {
	Continent string
	Country   string
	City      string

	Latitude  float64
	Longitude float64
	TimeZone  string
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

func getSafeName(names map[string]string) string {
	if names == nil {
		return ""
	}
	return names["en"]
}

func Lookup(db *maxminddb.Reader, ip netip.Addr) (Data, error) {
	var record cityRecord
	if err := db.Lookup(ip).Decode(&record); err != nil {
		return Data{}, err
	}

	return Data{
		Continent: getSafeName(record.Continent.Names),
		Country:   getSafeName(record.Country.Names),
		City:      getSafeName(record.City.Names),
		Latitude:  record.Location.Latitude,
		Longitude: record.Location.Longitude,
		TimeZone:  record.Location.TimeZone,
	}, nil
}
