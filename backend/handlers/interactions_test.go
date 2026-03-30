package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
	"github.com/harshilc/cinematch-backend/middleware"
)

func TestRecordInteraction(t *testing.T) {
	validMovieID := "11111111-1111-1111-1111-111111111111"
	validUserID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	tests := []struct {
		name            string
		body            string
		injectUser      string // user ID injected into context; empty = unauthenticated
		dbErr           error
		totalCount      int
		perMovieCount   int
		wantStatus      int
		wantCapture     *db.InteractionInsert // non-nil: verify what was written to DB
	}{
		{
			name:       "records like interaction",
			body:       `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser: validUserID,
			wantStatus: http.StatusCreated,
			wantCapture: &db.InteractionInsert{
				UserID:  validUserID,
				MovieID: validMovieID,
				Type:    "like",
			},
		},
		{
			name:       "returns 401 when not authenticated",
			body:       `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "returns 400 for invalid movie UUID",
			body:       `{"movie_id":"not-a-uuid","type":"watch"}`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 400 for unknown interaction type",
			body:       `{"movie_id":"` + validMovieID + `","type":"love"}`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 400 for malformed JSON",
			body:       `not json`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 400 for unknown JSON fields",
			body:       `{"movie_id":"` + validMovieID + `","type":"like","extra":"field"}`,
			injectUser: validUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on db error",
			body:       `{"movie_id":"` + validMovieID + `","type":"skip"}`,
			injectUser: validUserID,
			dbErr:      errors.New("insert failed"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:        "returns 429 when user total cap reached",
			body:        `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser:  validUserID,
			totalCount:  500,
			wantStatus:  http.StatusTooManyRequests,
		},
		{
			name:          "returns 429 when per-movie cap reached",
			body:          `{"movie_id":"` + validMovieID + `","type":"like"}`,
			injectUser:    validUserID,
			totalCount:    10,
			perMovieCount: 5,
			wantStatus:    http.StatusTooManyRequests,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var captured db.InteractionInsert
			q := &stubQuerier{
				insertInteraction: func(_ context.Context, i db.InteractionInsert) error {
					captured = i
					return tc.dbErr
				},
				countUserInteractions: func(_ context.Context, _ string) (int, error) {
					return tc.totalCount, nil
				},
				countUserMovieInteractions: func(_ context.Context, _, _ string) (int, error) {
					return tc.perMovieCount, nil
				},
			}
			handler := handlers.RecordInteraction(q)

			req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			// Inject authenticated user ID into context the same way RequireAuth does.
			if tc.injectUser != "" {
				req = req.WithContext(middleware.WithUserID(req.Context(), tc.injectUser))
			}

			rec := httptest.NewRecorder()
			handler(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.wantCapture != nil {
				if captured != *tc.wantCapture {
					t.Errorf("captured interaction = %+v, want %+v", captured, *tc.wantCapture)
				}
			}
		})
	}
}
