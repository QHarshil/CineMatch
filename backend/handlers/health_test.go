package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/harshilc/cinematch-backend/handlers"
)

// stubDB satisfies DBPinger so we can control Ping behaviour in tests.
type stubDB struct{ err error }

func (s stubDB) Ping() error { return s.err }

func TestHealth(t *testing.T) {
	bootTime := time.Now().Add(-30 * time.Second)

	tests := []struct {
		name        string
		db          stubDB
		wantStatus  int
		wantDBField string
	}{
		{
			name:        "supabase reachable",
			db:          stubDB{err: nil},
			wantStatus:  http.StatusOK,
			wantDBField: "ok",
		},
		{
			name:        "supabase unreachable",
			db:          stubDB{err: errors.New("connection refused")},
			wantStatus:  http.StatusOK, // service is up, database field signals the problem
			wantDBField: "unreachable",
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
				Status        string  `json:"status"`
				Version       string  `json:"version"`
				UptimeSeconds float64 `json:"uptime_seconds"`
				Database      string  `json:"database"`
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
		})
	}
}
