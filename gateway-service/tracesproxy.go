package main

import (
	"io"
	"log/slog"
	"net/http"
	"time"
)

// tracesProxy forwards browser-exported OTLP/HTTP spans to Tempo's OTLP HTTP
// receiver. This is the only way spans reach Tempo from the browser: the
// browser cannot reach cluster-internal services directly, only this
// gateway (which is the one thing exposed through the Gateway API route).
type tracesProxy struct {
	tempoOTLPHTTPURL string // e.g. http://tempo.monitoring.svc.cluster.local:4318/v1/traces
	client           *http.Client
}

func newTracesProxy(tempoOTLPHTTPURL string) *tracesProxy {
	return &tracesProxy{
		tempoOTLPHTTPURL: tempoOTLPHTTPURL,
		client:           &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *tracesProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, p.tempoOTLPHTTPURL, r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		slog.Error("failed to forward traces to tempo", "error", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
