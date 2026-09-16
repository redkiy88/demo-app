package main

import (
	"net"
	"net/http"
	"strings"
)

// clientIP extracts the originating client IP. Envoy Gateway appends the
// real client address as the first entry in X-Forwarded-For; fall back to
// RemoteAddr for direct/local testing.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
