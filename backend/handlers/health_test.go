package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
)

// stubHealthDB satisfies handlers.HealthChecker so we can control behavior in tests.
type stubHealthDB struct {
	pingErr  error
	stats    db.TableStats
	statsErr error
}

func (s stubHealthDB) Ping() error { return s.pingErr }
func (s stubHealthDB) GetTableStats(_ context.Context) (db.TableStats, error) {
	return s.stats, s.statsErr
}

func TestHealth(t *testing.T) {
	bootTime := time.Now().Add(-30 * time.Second)

	tests := []struct {
		name        string
		db          stubHealthDB
		wantStatus  int
		wantDBField string
		wantStats   bool
	}{
		{
			name: "supabase reachable with stats",
			db: stubHealthDB{
				stats: db.TableStats{MovieCount: 494, UserCount: 10, InteractionCount: 150},
			},
			wantStatus:  http.StatusOK,
			wantDBField: "ok",
			wantStats:   true,
		},
		{
			name:        "supabase unreachable",
			db:          stubHealthDB{pingErr: errors.New("connection refused")},
			wantStatus:  http.StatusOK,
			wantDBField: "unreachable",
			wantStats:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := handlers.Health(tc.db, bootTime)
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			var body struct {
				Status        string         `json:"status"`
				Version       string         `json:"version"`
				UptimeSeconds float64        `json:"uptime_seconds"`
				Database      string         `json:"database"`
				Stats         *db.TableStats `json:"stats"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response body: %v", err)
			}

			if body.Status != "ok" {
				t.Errorf("status field = %q, want \"ok\"", body.Status)
			}
			if body.Database != tc.wantDBField {
				t.Errorf("database field = %q, want %q", body.Database, tc.wantDBField)
			}
			if body.UptimeSeconds < 25 {
				t.Errorf("uptime_seconds = %.2f, expected at least 25s (boot was 30s ago)", body.UptimeSeconds)
			}
			if body.Version == "" {
				t.Error("version field must not be empty")
			}
			if tc.wantStats && body.Stats == nil {
				t.Error("expected stats in response, got nil")
			}
			if tc.wantStats && body.Stats != nil && body.Stats.MovieCount != 494 {
				t.Errorf("movie_count = %d, want 494", body.Stats.MovieCount)
			}
			if !tc.wantStats && body.Stats != nil {
				t.Error("expected no stats when db unreachable, got stats")
			}
		})
	}
}
