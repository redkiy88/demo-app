package main

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// loggingInterceptor logs one line per RPC. Chained alongside the otelgrpc
// stats handler (which only handles tracing/metrics, not logging).
func loggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)

	span := trace.SpanContextFromContext(ctx)
	slog.Info("grpc request",
		"method", info.FullMethod,
		"code", status.Code(err).String(),
		"duration_ms", time.Since(start).Milliseconds(),
		"trace_id", span.TraceID().String(),
	)

	return resp, err
}
