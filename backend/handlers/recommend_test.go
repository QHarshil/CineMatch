package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
	"github.com/harshilc/cinematch-backend/middleware"
	"github.com/harshilc/cinematch-backend/ranker"
)

// stubRanker implements handlers.MovieRanker for tests.
type stubRanker struct {
	rankFunc func(ctx context.Context, candidates []db.MovieCandidate, topN int, genres []string, minVote float64) (*ranker.RankResponse, error)
}

func (s *stubRanker) Rank(ctx context.Context, candidates []db.MovieCandidate, topN int, genres []string, minVote float64) (*ranker.RankResponse, error) {
	return s.rankFunc(ctx, candidates, topN, genres, minVote)
}

// successRanker returns the first topN candidates in order with dummy scores.
func successRanker() *stubRanker {
	return &stubRanker{
		rankFunc: func(_ context.Context, candidates []db.MovieCandidate, topN int, _ []string, _ float64) (*ranker.RankResponse, error) {
			n := topN
			if len(candidates) < n {
				n = len(candidates)
			}
			ranked := make([]ranker.RankedMovie, n)
			for i, c := range candidates[:n] {
				ranked[i] = ranker.RankedMovie{MovieID: c.ID, Score: 0.9 - float64(i)*0.01, Rank: i + 1}
			}
			return &ranker.RankResponse{Ranked: ranked, ModelVersion: "test-v1"}, nil
		},
	}
}

func failingRanker() *stubRanker {
	return &stubRanker{
		rankFunc: func(_ context.Context, _ []db.MovieCandidate, _ int, _ []string, _ float64) (*ranker.RankResponse, error) {
			return nil, errors.New("ranker connection refused")
		},
	}
}

func TestRecommendForUser(t *testing.T) {
	validUserID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	sampleEmbedding := make([]float32, 1536)

	tests := []struct {
		name          string
		authenticated bool
		embedding     []float32
		embeddingErr  error
		candidates    []db.MovieCandidate
		candidatesErr error
		popularMovies []db.Movie
		popularErr    error
		ranker        *stubRanker
		wantStatus    int
		wantSource    string
		wantVersion   string
	}{
		{
			name:          "returns personalized results via ranker",
			authenticated: true,
			embedding:     sampleEmbedding,
			candidates:    []db.MovieCandidate{{Movie: sampleMovies[0], Similarity: 0.95}},
			ranker:        successRanker(),
			wantStatus:    http.StatusOK,
			wantSource:    "personalized",
			wantVersion:   "test-v1",
		},
		{
			name:          "falls back to similarity order when ranker fails",
			authenticated: true,
			embedding:     sampleEmbedding,
			candidates:    []db.MovieCandidate{{Movie: sampleMovies[0], Similarity: 0.95}},
			ranker:        failingRanker(),
			wantStatus:    http.StatusOK,
			wantSource:    "similarity_fallback",
		},
		{
			name:          "returns popular fallback for cold-start user",
			authenticated: true,
			embedding:     nil,
			popularMovies: sampleMovies,
			ranker:        successRanker(),
			wantStatus:    http.StatusOK,
			wantSource:    "popular",
		},
		{
			name:          "returns 401 when not authenticated",
			authenticated: false,
			ranker:        successRanker(),
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "falls back to cache when embedding fetch fails",
			authenticated: true,
			embeddingErr:  errors.New("db error"),
			ranker:        successRanker(),
			wantStatus:    http.StatusOK,
			wantSource:    "popular",
		},
		{
			name:          "returns 500 when match_movies fails",
			authenticated: true,
			embedding:     sampleEmbedding,
			candidatesErr: errors.New("rpc error"),
			ranker:        successRanker(),
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

			req := httptest.NewRequest(http.MethodGet, "/recommend", nil)
			if tc.authenticated {
				req = req.WithContext(middleware.WithUserID(req.Context(), validUserID))
			}
			rec := httptest.NewRecorder()

			cache := &stubCache{movies: sampleMovies}
			handlers.RecommendForUser(q, tc.ranker, cache).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.wantSource != "" {
				var body struct {
					Source       string `json:"source"`
					ModelVersion string `json:"model_version"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("decoding response: %v", err)
				}
				if body.Source != tc.wantSource {
					t.Errorf("source = %q, want %q", body.Source, tc.wantSource)
				}
				if tc.wantVersion != "" && body.ModelVersion != tc.wantVersion {
					t.Errorf("model_version = %q, want %q", body.ModelVersion, tc.wantVersion)
				}
			}
		})
	}
}
