package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
)

func TestSearchMovies(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		dbMovies   []db.Movie
		dbErr      error
		wantStatus int
		wantCount  int
	}{
		{
			name:       "returns matches for valid query",
			query:      "?q=inception",
			dbMovies:   sampleMovies[:1],
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "returns 400 when q is missing",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 400 when q exceeds 200 chars",
			query:      "?q=" + strings.Repeat("a", 201),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 400 when limit exceeds 50",
			query:      "?q=batman&limit=100",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on db error",
			query:      "?q=nolan",
			dbErr:      errors.New("db unreachable"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "returns empty slice when no matches",
			query:      "?q=xyzzy",
			dbMovies:   []db.Movie{},
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &stubQuerier{
				searchFunc: func(_ context.Context, _ string, _ int) ([]db.Movie, error) {
					return tc.dbMovies, tc.dbErr
				},
			}
			handler := handlers.SearchMovies(q)
			req := httptest.NewRequest(http.MethodGet, "/search"+tc.query, nil)
			rec := httptest.NewRecorder()
			handler(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK {
				var movies []db.Movie
				if err := json.NewDecoder(rec.Body).Decode(&movies); err != nil {
					t.Fatalf("decoding response: %v", err)
				}
				if len(movies) != tc.wantCount {
					t.Errorf("result count = %d, want %d", len(movies), tc.wantCount)
				}
			}
		})
	}
}
