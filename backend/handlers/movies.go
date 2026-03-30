package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ListMovies handles GET /movies?limit=20&offset=0
// Returns movies ordered by popularity descending. limit is capped at 100.
func ListMovies(querier DBQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := boundedIntParam(r, "limit", 20, 1, 100)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 100")
			return
		}
		offset, err := boundedIntParam(r, "offset", 0, 0, 1_000_000)
		if err != nil {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}

		movies, err := querier.ListMovies(r.Context(), limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to fetch movies")
			return
		}
		writeJSON(w, http.StatusOK, movies)
	}
}

// GetMovieByID handles GET /movies/{id}
// Returns 404 if the UUID is not found in the database.
func GetMovieByID(querier DBQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !isValidUUID(id) {
			writeError(w, http.StatusBadRequest, "id must be a valid UUID")
			return
		}

		movie, err := querier.GetMovieByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to fetch movie")
			return
		}
		if movie == nil {
			writeError(w, http.StatusNotFound, "movie not found")
			return
		}
		writeJSON(w, http.StatusOK, movie)
	}
}

// boundedIntParam reads a query param, applies a default when absent, and enforces min/max.
func boundedIntParam(r *http.Request, name string, defaultVal, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return n, nil
}
