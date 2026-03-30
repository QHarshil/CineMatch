package handlers

import (
	"net/http"
	"strings"
)

const (
	searchQueryMinLen  = 1
	searchQueryMaxLen  = 200
	searchDefaultLimit = 20
	searchMaxLimit     = 50
)

// SearchMovies handles GET /search?q=...&limit=20
// Searches movie titles using Postgres ILIKE backed by a trigram GIN index.
func SearchMovies(querier DBQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(q) < searchQueryMinLen {
			writeError(w, http.StatusBadRequest, "q is required")
			return
		}
		if len(q) > searchQueryMaxLen {
			writeError(w, http.StatusBadRequest, "q must be 200 characters or fewer")
			return
		}

		limit, err := boundedIntParam(r, "limit", searchDefaultLimit, 1, searchMaxLimit)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 50")
			return
		}

		movies, err := querier.SearchMoviesByTitle(r.Context(), q, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "search failed")
			return
		}
		writeJSON(w, http.StatusOK, movies)
	}
}
