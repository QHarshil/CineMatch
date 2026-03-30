// Package handlers contains HTTP handler functions for the CineMatch API.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// DBPinger is satisfied by db.SupabaseClient and can be replaced with a mock in tests.
type DBPinger interface {
	Ping() error
}

type healthResponse struct {
	Status        string  `json:"status"`
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Database      string  `json:"database"`
}

const buildVersion = "0.1.0"

// Health returns a handler that reports service liveness and Supabase reachability.
// startTime is captured at server boot so uptime reflects actual process age.
func Health(supabase DBPinger, startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := supabase.Ping(); err != nil {
			dbStatus = "unreachable"
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(healthResponse{
			Status:        "ok",
			Version:       buildVersion,
			UptimeSeconds: time.Since(startTime).Seconds(),
			Database:      dbStatus,
		}); err != nil {
			http.Error(w, "failed to encode health response", http.StatusInternalServerError)
		}
	}
}
