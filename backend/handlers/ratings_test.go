package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
	"github.com/harshilc/cinematch-backend/omdb"
)

type stubFetcher struct {
	ratings *omdb.Ratings
	err     error
	called  bool
}

func (s *stubFetcher) Fetch(_ context.Context, _ string, _ int, _ string) (*omdb.Ratings, error) {
	s.called = true
	return s.ratings, s.err
}

func TestGetMovieRatings(t *testing.T) {
	validID := "11111111-1111-1111-1111-111111111111"
	imdb := 8.5
	rt := 91

	tests := []struct {
		name       string
		id         string
		movie      *db.Movie
		movieErr   error
		ratings    *omdb.Ratings
		fetchErr   error
		nilFetcher bool
		wantStatus int
		wantIMDb   *float64
		wantRT     *int
	}{
		{
			name:       "returns omdb ratings",
			id:         validID,
			movie:      &db.Movie{ID: validID, Title: "Inception", ReleaseYear: 2010},
			ratings:    &omdb.Ratings{IMDb: &imdb, RT: &rt},
			wantStatus: http.StatusOK,
			wantIMDb:   &imdb,
			wantRT:     &rt,
		},
		{
			name:       "invalid uuid is rejected",
			id:         "not-a-uuid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing movie is 404",
			id:         validID,
			movie:      nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "omdb error degrades to empty body",
			id:         validID,
			movie:      &db.Movie{ID: validID, Title: "Obscure", ReleaseYear: 1999},
			fetchErr:   errors.New("omdb down"),
			wantStatus: http.StatusOK,
		},
		{
			name:       "no key configured returns empty body",
			id:         validID,
			nilFetcher: true,
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &stubQuerier{
				getMovieByIDFunc: func(_ context.Context, _ string) (*db.Movie, error) {
					return tc.movie, tc.movieErr
				},
			}
			var fetcher handlers.RatingsFetcher
			if !tc.nilFetcher {
				fetcher = &stubFetcher{ratings: tc.ratings, err: tc.fetchErr}
			}

			r := chi.NewRouter()
			r.Get("/movies/{id}/ratings", handlers.GetMovieRatings(q, fetcher, omdb.NewCache(time.Minute)))

			req := httptest.NewRequest(http.MethodGet, "/movies/"+tc.id+"/ratings", nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				return
			}

			var got omdb.Ratings
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decoding response: %v", err)
			}
			if (got.IMDb == nil) != (tc.wantIMDb == nil) {
				t.Fatalf("imdb_rating presence mismatch: got %v want %v", got.IMDb, tc.wantIMDb)
			}
			if got.IMDb != nil && *got.IMDb != *tc.wantIMDb {
				t.Errorf("imdb_rating = %v, want %v", *got.IMDb, *tc.wantIMDb)
			}
			if (got.RT == nil) != (tc.wantRT == nil) {
				t.Fatalf("rt_rating presence mismatch: got %v want %v", got.RT, tc.wantRT)
			}
			if got.RT != nil && *got.RT != *tc.wantRT {
				t.Errorf("rt_rating = %v, want %v", *got.RT, *tc.wantRT)
			}
		})
	}
}

func TestGetMovieRatingsServesCache(t *testing.T) {
	validID := "11111111-1111-1111-1111-111111111111"
	imdb := 7.7
	cache := omdb.NewCache(time.Minute)
	cache.Set(validID, &omdb.Ratings{IMDb: &imdb})

	// A fetcher that would error, plus a querier that fails the test if called,
	// proves the cache short-circuits before any OMDb lookup.
	fetcher := &stubFetcher{err: errors.New("should not be called")}
	q := &stubQuerier{
		getMovieByIDFunc: func(_ context.Context, _ string) (*db.Movie, error) {
			t.Fatal("GetMovieByID should not run on a cache hit")
			return nil, nil
		},
	}

	r := chi.NewRouter()
	r.Get("/movies/{id}/ratings", handlers.GetMovieRatings(q, fetcher, cache))
	req := httptest.NewRequest(http.MethodGet, "/movies/"+validID+"/ratings", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if fetcher.called {
		t.Error("fetcher was called despite a cache hit")
	}
}
