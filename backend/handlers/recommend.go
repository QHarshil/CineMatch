package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/middleware"
	"github.com/harshilc/cinematch-backend/ranker"
)

const (
	retrievalCandidateCount = 50 // Stage-1: number of candidates fetched from pgvector
	recommendedMovieCount   = 20 // final count returned after ranking
)

// MovieRanker re-scores Stage-1 candidates via the Python ranker service.
// Implemented by ranker.Client; stubbed in tests.
type MovieRanker interface {
	Rank(ctx context.Context, candidates []db.MovieCandidate, topN int, user ranker.UserContext) (*ranker.RankResponse, error)
}

// Recommendation is a ranked feed plus how it was produced.
type Recommendation struct {
	Movies       []db.Movie `json:"movies"`
	Source       string     `json:"source"` // "personalized" | "popular" | "similarity_fallback"
	ModelVersion string     `json:"model_version,omitempty"`
}

// pipelineError carries a message that is safe to return to clients.
type pipelineError string

func (e pipelineError) Error() string { return string(e) }

// RecommendationPipeline runs the two-stage pipeline:
//
//	Stage 1: fetch user embedding -> match_movies RPC -> top-50 candidates
//	Stage 2: POST candidates to Python ranker -> re-scored top-20
//
// Cold-start fallback: users without an embedding receive popular movies.
// Ranker fallback: if the ranker is unreachable, candidates are returned
// in their original cosine-similarity order so recommendations stay available.
// Shared by GET /recommend and the assistant's recommendations tool.
type RecommendationPipeline struct {
	querier DBQuerier
	ranker  MovieRanker
	cache   PopularCache
}

// NewRecommendationPipeline wires the pipeline's dependencies.
func NewRecommendationPipeline(querier DBQuerier, movieRanker MovieRanker, cache PopularCache) *RecommendationPipeline {
	return &RecommendationPipeline{querier: querier, ranker: movieRanker, cache: cache}
}

// Recommend returns the user's ranked feed.
func (p *RecommendationPipeline) Recommend(ctx context.Context, userID string) (Recommendation, error) {
	embedding, err := p.querier.GetUserEmbedding(ctx, userID)
	if err != nil {
		// Supabase unreachable: serve cached popular movies instead of failing.
		slog.Warn("supabase unreachable for user embedding, serving cached popular", "error", err)
		if cached := p.cache.Get(); cached != nil {
			return popularMoviesResponse(cached[:min(recommendedMovieCount, len(cached))]), nil
		}
		return Recommendation{}, pipelineError("failed to load user profile")
	}

	// Cold-start: no embedding means no interaction history yet.
	// Return popular movies so new users see a useful default feed.
	if embedding == nil {
		movies, err := p.querier.ListMovies(ctx, recommendedMovieCount, 0)
		if err != nil {
			slog.Warn("supabase unreachable for cold-start popular, serving cache", "error", err)
			if cached := p.cache.Get(); cached != nil {
				return popularMoviesResponse(cached[:min(recommendedMovieCount, len(cached))]), nil
			}
			return Recommendation{}, pipelineError("failed to load recommendations")
		}
		return popularMoviesResponse(movies), nil
	}

	candidates, err := p.querier.MatchMovies(ctx, embedding, retrievalCandidateCount)
	if err != nil {
		return Recommendation{}, pipelineError("failed to retrieve candidates")
	}

	// Stage-2: call the Python ranker to re-score candidates.
	// On failure, degrade gracefully to cosine-similarity order rather than
	// returning an error; partial recommendations are better than none.
	stats, statsErr := p.querier.UserInteractionStats(ctx, userID)
	if statsErr != nil {
		slog.Warn("failed to load user stats for ranker, using defaults", "error", statsErr)
		stats = db.UserStats{LikeRatio: 0.5}
	}

	ranked, err := p.ranker.Rank(ctx, candidates, recommendedMovieCount, ranker.UserContext{
		LikeRatio:        stats.LikeRatio,
		InteractionCount: stats.Total,
	})
	if err != nil {
		slog.Warn("ranker unavailable, falling back to similarity order", "error", err)
		return similarityFallback(candidates, recommendedMovieCount), nil
	}
	return rankedResponse(candidates, ranked), nil
}

// RecommendForUser handles GET /recommend with the two-stage pipeline.
func RecommendForUser(querier DBQuerier, movieRanker MovieRanker, cache PopularCache) http.HandlerFunc {
	pipeline := NewRecommendationPipeline(querier, movieRanker, cache)
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		feed, err := pipeline.Recommend(r.Context(), userID)
		if err != nil {
			msg := "failed to load recommendations"
			var safe pipelineError
			if errors.As(err, &safe) {
				msg = safe.Error()
			}
			writeError(w, http.StatusInternalServerError, msg)
			return
		}
		writeJSON(w, http.StatusOK, feed)
	}
}

func popularMoviesResponse(movies []db.Movie) Recommendation {
	return Recommendation{Movies: movies, Source: "popular"}
}

// similarityFallback returns candidates in their original pgvector cosine-similarity
// order when the ranker service is unreachable.
func similarityFallback(candidates []db.MovieCandidate, n int) Recommendation {
	if len(candidates) > n {
		candidates = candidates[:n]
	}
	movies := make([]db.Movie, len(candidates))
	for i, c := range candidates {
		movies[i] = c.Movie
	}
	return Recommendation{Movies: movies, Source: "similarity_fallback"}
}

// rankedResponse maps the ranker's scored results back to full Movie objects
// preserving the ranker's sort order.
func rankedResponse(candidates []db.MovieCandidate, ranked *ranker.RankResponse) Recommendation {
	movieByID := make(map[string]db.Movie, len(candidates))
	for _, c := range candidates {
		movieByID[c.ID] = c.Movie
	}

	movies := make([]db.Movie, 0, len(ranked.Ranked))
	for _, r := range ranked.Ranked {
		if m, ok := movieByID[r.MovieID]; ok {
			movies = append(movies, m)
		}
	}
	return Recommendation{
		Movies:       movies,
		Source:       "personalized",
		ModelVersion: ranked.ModelVersion,
	}
}
