package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const minUpdateInterval = 24 * time.Hour

// lastUpdateMarker returns the path of the small file that tracks when the
// mmdb was last downloaded, so we never re-download more than once per
// minUpdateInterval (MaxMind enforces an undisclosed daily download limit).
func lastUpdateMarker(dbPath string) string {
	return dbPath + ".last-update"
}

func dbExists(dbPath string) bool {
	_, err := os.Stat(dbPath)
	return err == nil
}

// canUpdateNow reports whether enough time has passed since the last
// successful download to attempt another one.
func canUpdateNow(dbPath string) (bool, time.Duration) {
	data, err := os.ReadFile(lastUpdateMarker(dbPath))
	if err != nil {
		return true, 0
	}
	last, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return true, 0
	}
	elapsed := time.Since(last)
	if elapsed >= minUpdateInterval {
		return true, 0
	}
	return false, minUpdateInterval - elapsed
}

// downloadDB fetches the latest GeoLite2-City.mmdb from MaxMind and atomically
// replaces dbPath. accountID/licenseKey are the MaxMind download credentials.
func downloadDB(dbPath, accountID, licenseKey string) error {
	const url = "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(accountID, licenseKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download: unexpected status %s: %s", resp.Status, body)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gunzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	tmpPath := dbPath + ".tmp"

	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if !strings.HasSuffix(hdr.Name, "GeoLite2-City.mmdb") {
			continue
		}
		found = true

		if err := os.MkdirAll(filepath.Dir(tmpPath), 0o755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
		out, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("create tmp file: %w", err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("write tmp file: %w", err)
		}
		out.Close()
		break
	}

	if !found {
		return fmt.Errorf("GeoLite2-City.mmdb not found in downloaded archive")
	}

	if err := os.Rename(tmpPath, dbPath); err != nil {
		return fmt.Errorf("rename into place: %w", err)
	}

	marker := []byte(time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(lastUpdateMarker(dbPath), marker, 0o644); err != nil {
		slog.Warn("failed to write last-update marker", "error", err)
	}

	return nil
}

// ensureDBOnStartup downloads the database only if the file is missing.
// It does not consult canUpdateNow: a missing file always needs downloading
// regardless of the last-update marker (e.g. fresh PVC).
func ensureDBOnStartup(dbPath, accountID, licenseKey string) error {
	if dbExists(dbPath) {
		slog.Info("GeoLite2 database already present", "path", dbPath)
		return nil
	}
	slog.Info("GeoLite2 database missing, downloading", "path", dbPath)
	return downloadDB(dbPath, accountID, licenseKey)
}
