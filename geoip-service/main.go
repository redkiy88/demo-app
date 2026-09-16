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
	"google.golang.org/grpc"

	geoippb "github.com/dmitriimoskin/geo-weather-app/proto/geoip"
	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "geoip-service")
	slog.SetDefault(logger)

	dbPath := getenv("DB_PATH", "/data/GeoLite2-City.mmdb")
	accountID := os.Getenv("MAXMIND_ACCOUNT_ID")
	licenseKey := os.Getenv("MAXMIND_LICENSE_KEY")
	grpcAddr := getenv("GRPC_ADDR", ":9090")
	httpAddr := getenv("INTERNAL_HTTP_ADDR", ":8081")
	otlpEndpoint := getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "tempo.monitoring.svc.cluster.local:4317")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := setupTracing(ctx, "geoip-service", otlpEndpoint)
	if err != nil {
		slog.Error("failed to set up tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutdownCtx)
	}()

	if err := ensureDBOnStartup(dbPath, accountID, licenseKey); err != nil {
		slog.Error("failed to prepare geoip database on startup", "error", err)
		os.Exit(1)
	}

	reader, err := maxminddb.Open(dbPath)
	if err != nil {
		slog.Error("failed to open geoip database", "error", err)
		os.Exit(1)
	}

	srv := &geoIPServer{}
	srv.setReader(reader)

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(loggingInterceptor),
	)
	geoippb.RegisterGeoIPServiceServer(grpcServer, srv)

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

	internalServer := &http.Server{
		Addr:              httpAddr,
		Handler:           newInternalMux(srv, dbPath, accountID, licenseKey),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("internal HTTP server starting", "addr", httpAddr)
		if err := internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("internal HTTP server stopped", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = internalServer.Shutdown(shutdownCtx)
}
