package main

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	weatherpb "github.com/dmitriimoskin/geo-weather-app/proto/weather"
)

type weatherServer struct {
	weatherpb.UnimplementedWeatherServiceServer
	client *weatherClient
}

func (s *weatherServer) GetCurrent(ctx context.Context, req *weatherpb.GetCurrentRequest) (*weatherpb.GetCurrentResponse, error) {
	weather, err := s.client.GetCurrent(ctx, req.GetLatitude(), req.GetLongitude())
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "fetch weather: %v", err)
	}

	return &weatherpb.GetCurrentResponse{
		TemperatureC: weather.TemperatureC,
		WindspeedKmh: weather.WindspeedKmh,
		WeatherCode:  weather.WeatherCode,
		IsDay:        weather.IsDay,
	}, nil
}
