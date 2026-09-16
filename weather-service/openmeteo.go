package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("weather-service")

const cacheTTL = 5 * time.Minute

type currentWeather struct {
	TemperatureC float64
	WindspeedKmh float64
	WeatherCode  int32
	IsDay        bool
}

type openMeteoResponse struct {
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
		Windspeed   float64 `json:"windspeed"`
		WeatherCode int32   `json:"weathercode"`
		IsDay       int32   `json:"is_day"`
	} `json:"current_weather"`
}

type weatherClient struct {
	httpClient *http.Client

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	value     currentWeather
	expiresAt time.Time
}

func newWeatherClient(httpClient *http.Client) *weatherClient {
	return &weatherClient{
		httpClient: httpClient,
		cache:      make(map[string]cacheEntry),
	}
}

// cacheKey rounds coordinates to ~1km precision so nearby lookups share a
// cache entry instead of hammering Open-Meteo for every slightly different IP.
func cacheKey(lat, lon float64) string {
	return fmt.Sprintf("%.2f,%.2f", lat, lon)
}

func (c *weatherClient) GetCurrent(ctx context.Context, lat, lon float64) (currentWeather, error) {
	key := cacheKey(lat, lon)

	c.mu.Lock()
	if entry, ok := c.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.Unlock()
		return entry.value, nil
	}
	c.mu.Unlock()

	weather, err := c.fetch(ctx, lat, lon)
	if err != nil {
		return currentWeather{}, err
	}

	c.mu.Lock()
	c.cache[key] = cacheEntry{value: weather, expiresAt: time.Now().Add(cacheTTL)}
	c.mu.Unlock()

	return weather, nil
}

func (c *weatherClient) fetch(ctx context.Context, lat, lon float64) (currentWeather, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("current_weather", "true")

	reqURL := "https://api.open-meteo.com/v1/forecast?" + q.Encode()

	// Explicit span (rather than relying solely on otelhttp's transport-level
	// auto-instrumentation) so the span_id we log matches the span a human
	// would actually click on in Tempo for this specific outbound call.
	ctx, opSpan := tracer.Start(ctx, "fetch-open-meteo")
	defer opSpan.End()
	span := opSpan.SpanContext()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return currentWeather{}, fmt.Errorf("build request: %w", err)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		slog.Error("outbound http request", "target", "open-meteo", "duration_ms", duration, "trace_id", span.TraceID().String(), "span_id", span.SpanID().String(), "error", err)
		return currentWeather{}, fmt.Errorf("call open-meteo: %w", err)
	}
	defer resp.Body.Close()

	slog.Info("outbound http request", "target", "open-meteo", "status", resp.StatusCode, "duration_ms", duration, "trace_id", span.TraceID().String(), "span_id", span.SpanID().String())

	if resp.StatusCode != http.StatusOK {
		return currentWeather{}, fmt.Errorf("open-meteo returned status %s", resp.Status)
	}

	var parsed openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return currentWeather{}, fmt.Errorf("decode response: %w", err)
	}

	return currentWeather{
		TemperatureC: parsed.CurrentWeather.Temperature,
		WindspeedKmh: parsed.CurrentWeather.Windspeed,
		WeatherCode:  parsed.CurrentWeather.WeatherCode,
		IsDay:        parsed.CurrentWeather.IsDay == 1,
	}, nil
}
