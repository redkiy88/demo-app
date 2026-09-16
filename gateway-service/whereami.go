package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	geoippb "github.com/dmitriimoskin/geo-weather-app/proto/geoip"
	weatherpb "github.com/dmitriimoskin/geo-weather-app/proto/weather"
)

type whereamiResponse struct {
	IP           string  `json:"ip"`
	Country      string  `json:"country"`
	City         string  `json:"city"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	TemperatureC float64 `json:"temperature_c"`
	WindspeedKmh float64 `json:"windspeed_kmh"`
	WeatherCode  int32   `json:"weather_code"`
	IsDay        bool    `json:"is_day"`
}

type whereamiHandler struct {
	geoip   geoippb.GeoIPServiceClient
	weather weatherpb.WeatherServiceClient
}

func (h *whereamiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ip := clientIP(r)

	loc, err := h.geoip.Lookup(ctx, &geoippb.LookupRequest{Ip: ip})
	if err != nil {
		slog.Error("geoip lookup failed", "ip", ip, "error", err)
		http.Error(w, "could not determine location", http.StatusBadGateway)
		return
	}

	weather, err := h.weather.GetCurrent(ctx, &weatherpb.GetCurrentRequest{
		Latitude:  loc.GetLatitude(),
		Longitude: loc.GetLongitude(),
	})
	if err != nil {
		slog.Error("weather lookup failed", "lat", loc.GetLatitude(), "lon", loc.GetLongitude(), "error", err)
		http.Error(w, "could not determine weather", http.StatusBadGateway)
		return
	}

	resp := whereamiResponse{
		IP:           ip,
		Country:      loc.GetCountry(),
		City:         loc.GetCity(),
		Latitude:     loc.GetLatitude(),
		Longitude:    loc.GetLongitude(),
		TemperatureC: weather.GetTemperatureC(),
		WindspeedKmh: weather.GetWindspeedKmh(),
		WeatherCode:  weather.GetWeatherCode(),
		IsDay:        weather.GetIsDay(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
