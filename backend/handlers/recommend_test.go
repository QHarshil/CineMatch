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
	rankFunc func(ctx context.Context, candidates []db.MovieCandidate, topN int, user ranker.UserContext) (*ranker.RankResponse, error)
}

func (s *stubRanker) Rank(ctx context.Context, candidates []db.MovieCandidate, topN int, user ranker.UserContext) (*ranker.RankResponse, error) {
	return s.rankFunc(ctx, candidates, topN, user)
}

// successRanker returns the first topN candidates in order with dummy scores.
func successRanker() *stubRanker {
	return &stubRanker{
		rankFunc: func(_ context.Context, candidates []db.MovieCandidate, topN int, _ ranker.UserContext) (*ranker.RankResponse, error) {
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
		rankFunc: func(_ context.Context, _ []db.MovieCandidate, _ int, _ ranker.UserContext) (*ranker.RankResponse, error) {
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

func TestRecommendExcludesSeenTitlesAndExplainsPicks(t *testing.T) {
	seenID := "aaaaaaaa-0000-0000-0000-000000000001"
	freshID := "aaaaaaaa-0000-0000-0000-000000000002"
	var requestedCount int
	q := &stubQuerier{
		getUserEmbedding: func(context.Context, string) ([]float32, error) { return make([]float32, 1536), nil },
		interactedMovieIDs: func(context.Context, string) (map[string]bool, error) {
			return map[string]bool{seenID: true}, nil
		},
		matchMovies: func(_ context.Context, _ []float32, limit int) ([]db.MovieCandidate, error) {
			requestedCount = limit
			return []db.MovieCandidate{
				{Movie: db.Movie{ID: seenID, Title: "Already Liked"}, Similarity: 0.95},
				{Movie: db.Movie{ID: freshID, Title: "New Pick"}, Similarity: 0.71},
			}, nil
		},
		nearestLikedTitles: func(_ context.Context, _ string, ids []string) ([]db.LikedMatch, error) {
			return []db.LikedMatch{{MovieID: ids[0], LikedID: seenID, LikedTitle: "Already Liked", Similarity: 0.83}}, nil
		},
	}
	factorRanker := &stubRanker{rankFunc: func(_ context.Context, candidates []db.MovieCandidate, _ int, _ ranker.UserContext) (*ranker.RankResponse, error) {
		ranked := make([]ranker.RankedMovie, len(candidates))
		for i, c := range candidates {
			ranked[i] = ranker.RankedMovie{MovieID: c.ID, Rank: i + 1, Factors: []ranker.Factor{{Feature: "similarity", Contribution: 0.21}}}
		}
		return &ranker.RankResponse{Ranked: ranked, ModelVersion: "lambdamart-v1"}, nil
	}}

	feed, err := handlers.NewRecommendationPipeline(q, factorRanker, &stubCache{}).Recommend(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if requestedCount != 51 {
		t.Errorf("fetched %d candidates, want 51 to cover the seen title", requestedCount)
	}
	if len(feed.Movies) != 1 || feed.Movies[0].ID != freshID {
		t.Fatalf("movies = %+v", feed.Movies)
	}
	exp, ok := feed.Explanations[freshID]
	if !ok || exp.Similarity != 0.71 || exp.BecauseYouLiked == nil || exp.BecauseYouLiked.Title != "Already Liked" {
		t.Fatalf("explanation = %+v", exp)
	}
	if len(exp.Factors) != 1 || exp.Factors[0].Feature != "similarity" {
		t.Errorf("factors = %+v", exp.Factors)
	}
}

func TestRecommendExplanationsSurviveALikedTitleLookupFailure(t *testing.T) {
	q := &stubQuerier{
		getUserEmbedding: func(context.Context, string) ([]float32, error) { return make([]float32, 1536), nil },
		matchMovies: func(context.Context, []float32, int) ([]db.MovieCandidate, error) {
			return []db.MovieCandidate{{Movie: db.Movie{ID: "m1"}, Similarity: 0.6}}, nil
		},
		nearestLikedTitles: func(context.Context, string, []string) ([]db.LikedMatch, error) {
			return nil, errors.New("rpc failed")
		},
	}
	feed, err := handlers.NewRecommendationPipeline(q, failingRanker(), &stubCache{}).Recommend(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if feed.Source != "similarity_fallback" || feed.Explanations["m1"].Similarity != 0.6 || feed.Explanations["m1"].BecauseYouLiked != nil {
		t.Errorf("feed = %+v", feed)
	}
}
