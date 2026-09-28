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
	retrievalCandidateCount = 50  // Stage-1: candidates passed to the ranker
	maxRetrievalCount       = 150 // upper bound when over-fetching to replace seen titles
	recommendedMovieCount   = 20  // final count returned after ranking
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
	// Explanations are keyed by movie ID; absent for popular feeds.
	Explanations map[string]Explanation `json:"explanations,omitempty"`
}

// Explanation says why a title was recommended.
type Explanation struct {
	// Similarity is the cosine similarity between the title and the taste vector.
	Similarity float64 `json:"similarity"`
	// BecauseYouLiked is the liked title closest to this one.
	BecauseYouLiked *LikedTitle `json:"because_you_liked,omitempty"`
	// Factors are the ranker features that raised the score, largest first.
	Factors []ranker.Factor `json:"factors,omitempty"`
}

// LikedTitle is a title the user liked, with its similarity to the pick.
type LikedTitle struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Similarity float64 `json:"similarity"`
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

	// Titles the user already rated sit close to their taste vector and would
	// crowd the top of the feed, so fetch extra and drop them.
	seen, err := p.querier.InteractedMovieIDs(ctx, userID)
	if err != nil {
		slog.Warn("failed to load interacted titles, not excluding them", "error", err)
	}
	fetch := min(retrievalCandidateCount+len(seen), maxRetrievalCount)
	candidates, err := p.querier.MatchMovies(ctx, embedding, fetch)
	if err != nil {
		return Recommendation{}, pipelineError("failed to retrieve candidates")
	}
	candidates = excludeSeen(candidates, seen, retrievalCandidateCount)

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
		feed := similarityFallback(candidates, recommendedMovieCount)
		feed.Explanations = p.explain(ctx, userID, candidates, feed.Movies, nil)
		return feed, nil
	}
	feed := rankedResponse(candidates, ranked)
	feed.Explanations = p.explain(ctx, userID, candidates, feed.Movies, ranked)
	return feed, nil
}

// excludeSeen drops titles the user has interacted with and caps the list.
func excludeSeen(candidates []db.MovieCandidate, seen map[string]bool, limit int) []db.MovieCandidate {
	kept := make([]db.MovieCandidate, 0, min(len(candidates), limit))
	for _, c := range candidates {
		if !seen[c.ID] && len(kept) < limit {
			kept = append(kept, c)
		}
	}
	return kept
}

// explain builds per-title explanations from retrieval similarity, ranker
// factors, and the closest liked title. The liked-title lookup is best effort:
// on failure the explanations still carry similarity and factors.
func (p *RecommendationPipeline) explain(ctx context.Context, userID string, candidates []db.MovieCandidate, movies []db.Movie, ranked *ranker.RankResponse) map[string]Explanation {
	similarity := make(map[string]float64, len(candidates))
	for _, c := range candidates {
		similarity[c.ID] = c.Similarity
	}
	factors := map[string][]ranker.Factor{}
	if ranked != nil {
		for _, r := range ranked.Ranked {
			factors[r.MovieID] = r.Factors
		}
	}

	ids := make([]string, len(movies))
	for i, m := range movies {
		ids[i] = m.ID
	}
	liked := map[string]*LikedTitle{}
	if matches, err := p.querier.NearestLikedTitles(ctx, userID, ids); err != nil {
		slog.Warn("failed to find nearest liked titles", "error", err)
	} else {
		for _, m := range matches {
			liked[m.MovieID] = &LikedTitle{ID: m.LikedID, Title: m.LikedTitle, Similarity: m.Similarity}
		}
	}

	out := make(map[string]Explanation, len(movies))
	for _, id := range ids {
		out[id] = Explanation{Similarity: similarity[id], BecauseYouLiked: liked[id], Factors: factors[id]}
	}
	return out
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
