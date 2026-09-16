package main

import (
	"context"
	"net/netip"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	geoippb "github.com/dmitriimoskin/geo-weather-app/proto/geoip"
	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

type cityRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

// geoIPServer implements geoippb.GeoIPServiceServer. The reader is held
// behind an atomic pointer so /internal/update-db can hot-swap it without
// restarting the pod.
type geoIPServer struct {
	geoippb.UnimplementedGeoIPServiceServer
	reader atomic.Pointer[maxminddb.Reader]
}

func (s *geoIPServer) setReader(r *maxminddb.Reader) {
	s.reader.Store(r)
}

func (s *geoIPServer) Lookup(_ context.Context, req *geoippb.LookupRequest) (*geoippb.LookupResponse, error) {
	addr, err := netip.ParseAddr(req.GetIp())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid ip %q: %v", req.GetIp(), err)
	}

	reader := s.reader.Load()
	if reader == nil {
		return nil, status.Error(codes.Unavailable, "geoip database not loaded yet")
	}

	var rec cityRecord
	result := reader.Lookup(addr)
	if err := result.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "lookup: %v", err)
	}
	if !result.Found() {
		return nil, status.Errorf(codes.NotFound, "no location data for %s", req.GetIp())
	}
	if err := result.Decode(&rec); err != nil {
		return nil, status.Errorf(codes.Internal, "decode: %v", err)
	}

	city := rec.City.Names["en"]

	return &geoippb.LookupResponse{
		Country:   rec.Country.ISOCode,
		City:      city,
		Latitude:  rec.Location.Latitude,
		Longitude: rec.Location.Longitude,
	}, nil
}
