package handlers

import (
	"context"
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
	Rank(ctx context.Context, candidates []db.MovieCandidate, topN int, preferredGenres []string, minVotePref float64) (*ranker.RankResponse, error)
}

// RecommendForUser handles GET /recommend
//
// Two-stage pipeline:
//
//	Stage 1: fetch user embedding -> match_movies RPC -> top-50 candidates
//	Stage 2: POST candidates to Python ranker -> re-scored top-20
//
// Cold-start fallback: users without an embedding receive popular movies.
// Ranker fallback: if the ranker is unreachable, candidates are returned
// in their original cosine-similarity order so the endpoint stays available.
func RecommendForUser(querier DBQuerier, movieRanker MovieRanker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		embedding, err := querier.GetUserEmbedding(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load user profile")
			return
		}

		// Cold-start: no embedding means no interaction history yet.
		// Return popular movies so new users see a useful default feed.
		if embedding == nil {
			movies, err := querier.ListMovies(r.Context(), recommendedMovieCount, 0)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load recommendations")
				return
			}
			writeJSON(w, http.StatusOK, popularMoviesResponse(movies))
			return
		}

		candidates, err := querier.MatchMovies(r.Context(), embedding, retrievalCandidateCount)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to retrieve candidates")
			return
		}

		// Stage-2: call the Python ranker to re-score candidates.
		// On failure, degrade gracefully to cosine-similarity order rather than
		// returning an error — partial recommendations are better than none.
		ranked, err := movieRanker.Rank(r.Context(), candidates, recommendedMovieCount, nil, 0)
		if err != nil {
			slog.Warn("ranker unavailable, falling back to similarity order", "error", err)
			writeJSON(w, http.StatusOK, similarityFallback(candidates, recommendedMovieCount))
			return
		}

		writeJSON(w, http.StatusOK, rankedResponse(candidates, ranked))
	}
}

type recommendResponse struct {
	Movies       []db.Movie `json:"movies"`
	Source       string     `json:"source"` // "personalized" | "popular" | "similarity_fallback"
	ModelVersion string     `json:"model_version,omitempty"`
}

func popularMoviesResponse(movies []db.Movie) recommendResponse {
	return recommendResponse{Movies: movies, Source: "popular"}
}

// similarityFallback returns candidates in their original pgvector cosine-similarity
// order when the ranker service is unreachable.
func similarityFallback(candidates []db.MovieCandidate, n int) recommendResponse {
	if len(candidates) > n {
		candidates = candidates[:n]
	}
	movies := make([]db.Movie, len(candidates))
	for i, c := range candidates {
		movies[i] = c.Movie
	}
	return recommendResponse{Movies: movies, Source: "similarity_fallback"}
}

// rankedResponse maps the ranker's scored results back to full Movie objects
// preserving the ranker's sort order.
func rankedResponse(candidates []db.MovieCandidate, ranked *ranker.RankResponse) recommendResponse {
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
	return recommendResponse{
		Movies:       movies,
		Source:       "personalized",
		ModelVersion: ranked.ModelVersion,
	}
}
