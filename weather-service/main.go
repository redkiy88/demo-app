package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"

	weatherpb "github.com/dmitriimoskin/geo-weather-app/proto/weather"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "weather-service")
	slog.SetDefault(logger)

	grpcAddr := getenv("GRPC_ADDR", ":9090")
	otlpEndpoint := getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "tempo.monitoring.svc.cluster.local:4317")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := setupTracing(ctx, "weather-service", otlpEndpoint)
	if err != nil {
		slog.Error("failed to set up tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutdownCtx)
	}()

	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   5 * time.Second,
	}

	srv := &weatherServer{client: newWeatherClient(httpClient)}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(loggingInterceptor),
	)
	weatherpb.RegisterWeatherServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("failed to listen", "addr", grpcAddr, "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("gRPC server starting", "addr", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server stopped", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	grpcServer.GracefulStop()
}
