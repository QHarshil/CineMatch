package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
)

func TestRecommendForUser(t *testing.T) {
	validUserID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	sampleEmbedding := make([]float32, 1536)

	tests := []struct {
		name          string
		userID        string
		embedding     []float32
		embeddingErr  error
		candidates    []db.MovieCandidate
		candidatesErr error
		popularMovies []db.Movie
		popularErr    error
		wantStatus    int
		wantSource    string
	}{
		{
			name:       "returns personalized results when embedding exists",
			userID:     validUserID,
			embedding:  sampleEmbedding,
			candidates: []db.MovieCandidate{{Movie: sampleMovies[0], Similarity: 0.95}},
			wantStatus: http.StatusOK,
			wantSource: "personalized",
		},
		{
			name:          "returns popular fallback for cold-start user",
			userID:        validUserID,
			embedding:     nil,
			popularMovies: sampleMovies,
			wantStatus:    http.StatusOK,
			wantSource:    "popular",
		},
		{
			name:       "returns 400 for invalid userId",
			userID:     "not-a-uuid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:         "returns 500 when embedding fetch fails",
			userID:       validUserID,
			embeddingErr: errors.New("db error"),
			wantStatus:   http.StatusInternalServerError,
		},
		{
			name:          "returns 500 when match_movies fails",
			userID:        validUserID,
			embedding:     sampleEmbedding,
			candidatesErr: errors.New("rpc error"),
			wantStatus:    http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &stubQuerier{
				getUserEmbedding: func(_ context.Context, _ string) ([]float32, error) {
					return tc.embedding, tc.embeddingErr
				},
				matchMovies: func(_ context.Context, _ []float32, _ int) ([]db.MovieCandidate, error) {
					return tc.candidates, tc.candidatesErr
				},
				listMoviesFunc: func(_ context.Context, _, _ int) ([]db.Movie, error) {
					return tc.popularMovies, tc.popularErr
				},
			}

			r := chi.NewRouter()
			r.Get("/recommend/{userId}", handlers.RecommendForUser(q))

			req := httptest.NewRequest(http.MethodGet, "/recommend/"+tc.userID, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.wantSource != "" {
				var body struct {
					Source string `json:"source"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("decoding response: %v", err)
				}
				if body.Source != tc.wantSource {
					t.Errorf("source = %q, want %q", body.Source, tc.wantSource)
				}
			}
		})
	}
}
