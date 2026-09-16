package main

import (
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// requestLogger logs one line per HTTP request. Must run inside otelhttp
// (i.e. otelhttp.NewHandler(requestLogger(mux), ...)) so the span/trace_id
// already exist in the request context by the time we log.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		span := trace.SpanContextFromContext(r.Context())
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"trace_id", span.TraceID().String(),
			"span_id", span.SpanID().String(),
		)
	})
}
