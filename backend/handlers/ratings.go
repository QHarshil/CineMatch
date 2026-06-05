package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/harshilc/cinematch-backend/omdb"
)

// RatingsFetcher fetches aggregate IMDb/Rotten Tomatoes ratings for a title.
// *omdb.Client satisfies it; tests supply a stub.
type RatingsFetcher interface {
	Fetch(ctx context.Context, title string, year int, mediaType string) (*omdb.Ratings, error)
}

// RatingsCache caches ratings by movie ID. *omdb.Cache satisfies it.
type RatingsCache interface {
	Get(id string) (*omdb.Ratings, bool)
	Set(id string, ratings *omdb.Ratings)
}

// GetMovieRatings handles GET /movies/{id}/ratings.
//
// It returns a title's IMDb and Rotten Tomatoes scores from OMDb, served from
// an in-memory cache after the first lookup. When OMDb is unconfigured or
// unreachable it returns an empty (null) ratings body, so the detail page
// degrades gracefully rather than erroring.
func GetMovieRatings(querier DBQuerier, fetcher RatingsFetcher, cache RatingsCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !isValidUUID(id) {
			writeError(w, http.StatusBadRequest, "id must be a valid UUID")
			return
		}

		if cached, ok := cache.Get(id); ok {
			writeJSON(w, http.StatusOK, ratingsOrEmpty(cached))
			return
		}

		// OMDb key not configured: ratings are simply unavailable.
		if fetcher == nil {
			writeJSON(w, http.StatusOK, omdb.Ratings{})
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

		ratings, err := fetcher.Fetch(r.Context(), movie.Title, movie.ReleaseYear, movie.MediaType)
		if err != nil {
			slog.Warn("omdb fetch failed", "movie_id", id, "error", err)
			writeJSON(w, http.StatusOK, omdb.Ratings{})
			return
		}

		// Cache successes and misses alike so a title is queried at most once per TTL.
		cache.Set(id, ratings)
		writeJSON(w, http.StatusOK, ratingsOrEmpty(ratings))
	}
}

// ratingsOrEmpty unwraps a possibly-nil Ratings into a value, so the response is
// always a JSON object ({"imdb_rating":null,"rt_rating":null}) rather than null.
func ratingsOrEmpty(r *omdb.Ratings) omdb.Ratings {
	if r == nil {
		return omdb.Ratings{}
	}
	return *r
}
