package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/harshilc/cinematch-backend/db"
)

const (
	searchQueryMinLen  = 1
	searchQueryMaxLen  = 200
	searchDefaultLimit = 20
	searchMaxLimit     = 50
)

// SearchMovies handles GET /search?q=...&limit=20
// Searches movie titles using Postgres ILIKE backed by a trigram GIN index.
// If Supabase is unreachable, falls back to a basic title match against cached movies.
func SearchMovies(querier DBQuerier, cache PopularCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))

		// Sanitize the search query.
		q = sanitizeString(q)

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
			slog.Warn("supabase unreachable for search, falling back to cache filter", "error", err)
			if cached := cache.Get(); cached != nil {
				writeJSON(w, http.StatusOK, filterCachedMovies(cached, q, limit))
				return
			}
			writeError(w, http.StatusInternalServerError, "search failed")
			return
		}
		writeJSON(w, http.StatusOK, movies)
	}
}

// filterCachedMovies does a basic case-insensitive title match against cached movies
// when the database is unreachable.
func filterCachedMovies(movies []db.Movie, query string, limit int) []db.Movie {
	lower := strings.ToLower(query)
	var matched []db.Movie
	for _, m := range movies {
		if strings.Contains(strings.ToLower(m.Title), lower) {
			matched = append(matched, m)
			if len(matched) >= limit {
				break
			}
		}
	}
	if matched == nil {
		return []db.Movie{}
	}
	return matched
}
