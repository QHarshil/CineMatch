// Package handlers contains HTTP handler functions for the CineMatch API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/harshilc/cinematch-backend/db"
)

// HealthChecker combines liveness and stats queries needed by the health endpoint.
type HealthChecker interface {
	Ping() error
	GetTableStats(ctx context.Context) (db.TableStats, error)
}

type healthResponse struct {
	Status        string         `json:"status"`
	Version       string         `json:"version"`
	UptimeSeconds float64        `json:"uptime_seconds"`
	Database      string         `json:"database"`
	Stats         *db.TableStats `json:"stats,omitempty"`
}

const buildVersion = "0.1.0"

// Health returns a handler that reports service liveness, Supabase reachability,
// and database row counts for free-tier monitoring.
func Health(supabase HealthChecker, startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := supabase.Ping(); err != nil {
			dbStatus = "unreachable"
		}

		resp := healthResponse{
			Status:        "ok",
			Version:       buildVersion,
			UptimeSeconds: time.Since(startTime).Seconds(),
			Database:      dbStatus,
		}

		// Fetch stats only when DB is reachable.
		if dbStatus == "ok" {
			stats, err := supabase.GetTableStats(r.Context())
			if err == nil {
				resp.Stats = &stats
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode health response", http.StatusInternalServerError)
		}
	}
}
