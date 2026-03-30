package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
)

// stubQuerier provides controlled responses for handler tests.
// Fields left nil will panic if called — intentional, to catch unexpected calls.
type stubQuerier struct {
	listMoviesFunc    func(ctx context.Context, limit, offset int) ([]db.Movie, error)
	getMovieByIDFunc  func(ctx context.Context, id string) (*db.Movie, error)
	searchFunc        func(ctx context.Context, query string, limit int) ([]db.Movie, error)
	insertInteraction func(ctx context.Context, i db.InteractionInsert) error
	getUserEmbedding  func(ctx context.Context, userID string) ([]float32, error)
	matchMovies       func(ctx context.Context, emb []float32, limit int) ([]db.MovieCandidate, error)
}

func (s *stubQuerier) ListMovies(ctx context.Context, limit, offset int) ([]db.Movie, error) {
	return s.listMoviesFunc(ctx, limit, offset)
}
func (s *stubQuerier) GetMovieByID(ctx context.Context, id string) (*db.Movie, error) {
	return s.getMovieByIDFunc(ctx, id)
}
func (s *stubQuerier) SearchMoviesByTitle(ctx context.Context, q string, limit int) ([]db.Movie, error) {
	return s.searchFunc(ctx, q, limit)
}
func (s *stubQuerier) InsertInteraction(ctx context.Context, i db.InteractionInsert) error {
	return s.insertInteraction(ctx, i)
}
func (s *stubQuerier) GetUserEmbedding(ctx context.Context, userID string) ([]float32, error) {
	return s.getUserEmbedding(ctx, userID)
}
func (s *stubQuerier) MatchMovies(ctx context.Context, emb []float32, limit int) ([]db.MovieCandidate, error) {
	return s.matchMovies(ctx, emb, limit)
}

var sampleMovies = []db.Movie{
	{ID: "11111111-1111-1111-1111-111111111111", Title: "Inception", TmdbID: 27205, Popularity: 99.9},
	{ID: "22222222-2222-2222-2222-222222222222", Title: "The Dark Knight", TmdbID: 155, Popularity: 98.5},
}

func TestListMovies(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		dbMovies   []db.Movie
		dbErr      error
		wantStatus int
		wantCount  int
	}{
		{
			name:       "returns movies with default pagination",
			query:      "",
			dbMovies:   sampleMovies,
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "returns empty slice when no movies exist",
			query:      "?limit=5&offset=0",
			dbMovies:   []db.Movie{},
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name:       "rejects limit above 100",
			query:      "?limit=200",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects negative offset",
			query:      "?offset=-1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on db error",
			query:      "",
			dbErr:      errors.New("connection reset"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &stubQuerier{
				listMoviesFunc: func(_ context.Context, _, _ int) ([]db.Movie, error) {
					return tc.dbMovies, tc.dbErr
				},
			}
			handler := handlers.ListMovies(q)
			req := httptest.NewRequest(http.MethodGet, "/movies"+tc.query, nil)
			rec := httptest.NewRecorder()
			handler(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK {
				var movies []db.Movie
				if err := json.NewDecoder(rec.Body).Decode(&movies); err != nil {
					t.Fatalf("decoding response: %v", err)
				}
				if len(movies) != tc.wantCount {
					t.Errorf("movie count = %d, want %d", len(movies), tc.wantCount)
				}
			}
		})
	}
}

func TestGetMovieByID(t *testing.T) {
	validID := "11111111-1111-1111-1111-111111111111"

	tests := []struct {
		name       string
		id         string
		dbMovie    *db.Movie
		dbErr      error
		wantStatus int
	}{
		{
			name:       "returns movie for valid UUID",
			id:         validID,
			dbMovie:    &sampleMovies[0],
			wantStatus: http.StatusOK,
		},
		{
			name:       "returns 404 when movie not found",
			id:         validID,
			dbMovie:    nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "returns 400 for invalid UUID",
			id:         "not-a-uuid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on db error",
			id:         validID,
			dbErr:      errors.New("timeout"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &stubQuerier{
				getMovieByIDFunc: func(_ context.Context, _ string) (*db.Movie, error) {
					return tc.dbMovie, tc.dbErr
				},
			}

			r := chi.NewRouter()
			r.Get("/movies/{id}", handlers.GetMovieByID(q))

			req := httptest.NewRequest(http.MethodGet, "/movies/"+tc.id, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}
