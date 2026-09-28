// Package search runs natural-language retrieval over the catalog. It embeds
// the query when an embedder is available and degrades to keyword and title
// matching when it is not, so search keeps working without an OpenAI key or
// after the daily embedding budget is spent.
package search

import (
	"context"
	"errors"
	"log/slog"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/embed"
)

// Retrieval modes reported to clients so the UI can say how results matched.
const (
	RetrievalHybrid  = "hybrid"  // semantic + keyword + title
	RetrievalKeyword = "keyword" // no query vector available
)

// Store runs the hybrid retrieval query. Implemented by db.SupabaseClient.
type Store interface {
	HybridSearch(ctx context.Context, query string, embedding []float32, limit int, f db.SearchFilters) ([]db.SearchHit, error)
}

// Result holds ranked hits and the retrieval mode that produced them.
type Result struct {
	Hits      []db.SearchHit
	Retrieval string
}

// Service combines query embedding with hybrid retrieval.
type Service struct {
	store    Store
	embedder embed.Embedder
}

// NewService returns a search service. A nil embedder runs keyword-only.
func NewService(store Store, embedder embed.Embedder) *Service {
	return &Service{store: store, embedder: embedder}
}

// Search embeds q and runs hybrid retrieval. Embedding failures are logged
// and the search continues keyword-only; database failures are returned.
func (s *Service) Search(ctx context.Context, q string, limit int, f db.SearchFilters) (Result, error) {
	var vec []float32
	retrieval := RetrievalKeyword
	if s.embedder != nil {
		v, err := s.embedder.Embed(ctx, q)
		switch {
		case err == nil:
			vec, retrieval = v, RetrievalHybrid
		case errors.Is(err, context.Canceled):
			return Result{}, err
		default:
			slog.Warn("query embedding unavailable, using keyword search", "error", err)
		}
	}
	hits, err := s.store.HybridSearch(ctx, q, vec, limit, f)
	if err != nil {
		return Result{}, err
	}
	if hits == nil {
		hits = []db.SearchHit{}
	}
	return Result{Hits: hits, Retrieval: retrieval}, nil
}
