package handlers

import (
	"net/http"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/middleware"
)

const (
	retrievalCandidateCount = 50 // Stage-1: number of candidates fetched from pgvector
	recommendedMovieCount   = 20 // final count returned after ranking
)

// RecommendForUser handles GET /recommend
//
// Two-stage pipeline:
//
//	Stage 1 (here): fetch user embedding -> match_movies RPC -> top-50 candidates
//	Stage 2 (Task 7): POST candidates to Python ranker -> re-scored top-20
//
// Cold-start fallback: users without an embedding receive popular movies.
// The ranker stage is stubbed as a passthrough until Task 7 wires it up.
// The authenticated user ID is taken from the JWT via RequireAuth middleware —
// users cannot request recommendations on behalf of other users.
func RecommendForUser(querier DBQuerier) http.HandlerFunc {
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

		// Stage-2 ranker stub: return the top candidates ordered by cosine similarity.
		// Task 7 replaces this with a POST to the Python ranker service.
		ranked := stubRank(candidates, recommendedMovieCount)
		writeJSON(w, http.StatusOK, ranked)
	}
}

type recommendResponse struct {
	Movies []db.Movie `json:"movies"`
	Source string     `json:"source"` // "personalized" | "popular"
}

func popularMoviesResponse(movies []db.Movie) recommendResponse {
	return recommendResponse{Movies: movies, Source: "popular"}
}

// stubRank converts MovieCandidates to the recommend response format, capped at n results.
// Replaced in Task 7 by a real call to the Python ranker microservice.
func stubRank(candidates []db.MovieCandidate, n int) recommendResponse {
	if len(candidates) > n {
		candidates = candidates[:n]
	}
	movies := make([]db.Movie, len(candidates))
	for i, c := range candidates {
		movies[i] = c.Movie
	}
	return recommendResponse{Movies: movies, Source: "personalized"}
}
