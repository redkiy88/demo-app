package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

// newInternalMux serves /internal/update-db and /healthz. It is not exposed
// through the Gateway — only reachable from inside the cluster (ClusterIP).
func newInternalMux(srv *geoIPServer, dbPath, accountID, licenseKey string) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /internal/update-db", func(w http.ResponseWriter, r *http.Request) {
		if ok, retryAfter := canUpdateNow(dbPath); !ok {
			w.Header().Set("Retry-After", retryAfter.Round(time.Second).String())
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":       "too soon since last download",
				"retry_after": retryAfter.Round(time.Second).String(),
			})
			return
		}

		if err := downloadDB(dbPath, accountID, licenseKey); err != nil {
			slog.Error("manual db update failed", "error", err)
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		newReader, err := maxminddb.Open(dbPath)
		if err != nil {
			slog.Error("failed to reopen db after update", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		old := srv.reader.Swap(newReader)
		if old != nil {
			// Give in-flight lookups a moment to finish before closing.
			go func() {
				time.Sleep(5 * time.Second)
				_ = old.Close()
			}()
		}

		slog.Info("geoip database updated and reloaded")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	return mux
}
