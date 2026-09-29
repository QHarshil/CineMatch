package handlers

import (
	"context"

	"github.com/harshilc/cinematch-backend/db"
)

// DBQuerier is satisfied by db.SupabaseClient. Handlers take the interface so
// tests can pass stubs.
type DBQuerier interface {
	ListMovies(ctx context.Context, limit, offset int) ([]db.Movie, error)
	GetMovieByID(ctx context.Context, id string) (*db.Movie, error)
	SearchMoviesByTitle(ctx context.Context, query string, limit int) ([]db.Movie, error)
	UpsertInteraction(ctx context.Context, interaction db.InteractionInsert) error
	DeleteInteraction(ctx context.Context, userID, movieID, interactionType string) error
	GetUserMovieInteractions(ctx context.Context, userID, movieID string) ([]db.InteractionRow, error)
	UpsertRating(ctx context.Context, rating db.RatingUpsert) error
	DeleteRating(ctx context.Context, userID, movieID string) error
	GetUserMovieRating(ctx context.Context, userID, movieID string) (*db.RatingRow, error)
	CountUserInteractions(ctx context.Context, userID string) (int, error)
	RefreshUserEmbedding(ctx context.Context, userID string) error
}
