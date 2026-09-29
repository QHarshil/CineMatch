package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/search"
)

const (
	discoverDefaultLimit = 20
	discoverMaxLimit     = 50
	discoverMaxGenres    = 5
	discoverMaxGenreLen  = 40
)

// TitleSearch runs natural-language retrieval. Implemented by search.Service.
type TitleSearch interface {
	Search(ctx context.Context, q string, limit int, f db.SearchFilters) (search.Result, error)
}

// retrievalCached marks results served from the popular cache while the
// database is unreachable.
const retrievalCached = "cached"

type discoverResponse struct {
	Results   []db.SearchHit `json:"results"`
	Retrieval string         `json:"retrieval"`
}

// DiscoverTitles handles GET /discover?q=...
//
// Natural-language search: "slow-burn sci-fi with a twist" or an exact title
// both work. When Supabase is unreachable it serves title matches from the
// popular cache.
func DiscoverTitles(titles TitleSearch, cache PopularCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := sanitizeString(r.URL.Query().Get("q"))
		if q == "" {
			writeError(w, http.StatusBadRequest, "q is required")
			return
		}
		if len([]rune(q)) > searchQueryMaxLen {
			writeError(w, http.StatusBadRequest, "q must be 200 characters or fewer")
			return
		}
		limit, err := boundedIntParam(r, "limit", discoverDefaultLimit, 1, discoverMaxLimit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		filters, err := parseSearchFilters(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := titles.Search(r.Context(), q, limit, filters)
		if err != nil {
			slog.Warn("hybrid search failed, serving cached title matches", "error", err)
			if cached := cache.Get(); cached != nil {
				writeJSON(w, http.StatusOK, discoverResponse{
					Results:   cachedHits(filterCachedMovies(cached, q, limit)),
					Retrieval: retrievalCached,
				})
				return
			}
			writeError(w, http.StatusInternalServerError, "search failed")
			return
		}
		writeJSON(w, http.StatusOK, discoverResponse{Results: result.Hits, Retrieval: result.Retrieval})
	}
}

// parseSearchFilters reads the optional filter parameters shared by the
// discover endpoint.
func parseSearchFilters(r *http.Request) (db.SearchFilters, error) {
	query := r.URL.Query()
	var f db.SearchFilters

	switch t := query.Get("type"); t {
	case "", "movie", "tv":
		f.MediaType = t
	default:
		return f, errors.New("type must be movie or tv")
	}

	if raw := query.Get("genre"); raw != "" {
		for _, g := range strings.Split(raw, ",") {
			g = sanitizeString(g)
			if g == "" {
				continue
			}
			if len(g) > discoverMaxGenreLen {
				return f, errors.New("genre names must be 40 characters or fewer")
			}
			f.Genres = append(f.Genres, g)
		}
		if len(f.Genres) > discoverMaxGenres {
			return f, fmt.Errorf("at most %d genres", discoverMaxGenres)
		}
	}

	var err error
	if f.MinYear, err = boundedIntParam(r, "year_min", 0, db.MinFilterYear, db.MaxFilterYear); err != nil {
		return f, err
	}
	if f.MaxYear, err = boundedIntParam(r, "year_max", 0, db.MinFilterYear, db.MaxFilterYear); err != nil {
		return f, err
	}
	if f.MinYear > 0 && f.MaxYear > 0 && f.MinYear > f.MaxYear {
		return f, errors.New("year_min must not exceed year_max")
	}
	if f.MaxRuntime, err = boundedIntParam(r, "runtime_max", 0, 1, db.MaxFilterRuntime); err != nil {
		return f, err
	}
	if raw := query.Get("rating_min"); raw != "" {
		rating, err := strconv.ParseFloat(raw, 64)
		if err != nil || rating < 0 || rating > db.MaxFilterRating {
			return f, fmt.Errorf("rating_min must be a number between 0 and %g", db.MaxFilterRating)
		}
		f.MinRating = rating
	}
	if lang := query.Get("lang"); lang != "" {
		if !db.IsLanguageCode(lang) {
			return f, errors.New("lang must be a two-letter ISO 639-1 code")
		}
		f.Language = lang
	}
	return f, nil
}

func cachedHits(movies []db.Movie) []db.SearchHit {
	hits := make([]db.SearchHit, len(movies))
	for i, m := range movies {
		hits[i] = db.SearchHit{Movie: m}
	}
	return hits
}
