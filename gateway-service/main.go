package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	geoippb "github.com/dmitriimoskin/geo-weather-app/proto/geoip"
	weatherpb "github.com/dmitriimoskin/geo-weather-app/proto/weather"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "gateway-service")
	slog.SetDefault(logger)

	httpAddr := getenv("HTTP_ADDR", ":8080")
	geoipAddr := getenv("GEOIP_ADDR", "geoip-service:9090")
	weatherAddr := getenv("WEATHER_ADDR", "weather-service:9090")
	otlpGRPCEndpoint := getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "tempo.monitoring.svc.cluster.local:4317")
	tempoOTLPHTTPURL := getenv("TEMPO_OTLP_HTTP_URL", "http://tempo.monitoring.svc.cluster.local:4318/v1/traces")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := setupTracing(ctx, "gateway-service", otlpGRPCEndpoint)
	if err != nil {
		slog.Error("failed to set up tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutdownCtx)
	}()

	geoipConn, err := grpc.NewClient(geoipAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("failed to dial geoip-service", "error", err)
		os.Exit(1)
	}
	defer geoipConn.Close()

	weatherConn, err := grpc.NewClient(weatherAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("failed to dial weather-service", "error", err)
		os.Exit(1)
	}
	defer weatherConn.Close()

	whereami := &whereamiHandler{
		geoip:   geoippb.NewGeoIPServiceClient(geoipConn),
		weather: weatherpb.NewWeatherServiceClient(weatherConn),
	}
	traces := newTracesProxy(tempoOTLPHTTPURL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("GET /api/whereami", whereami)
	mux.Handle("POST /api/traces", traces)

	handler := otelhttp.NewHandler(requestLogger(mux), "gateway-service")

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("HTTP server starting", "addr", httpAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server stopped", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
