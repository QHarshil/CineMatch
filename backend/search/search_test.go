package search

import (
	"context"
	"errors"
	"testing"

	"github.com/harshilc/cinematch-backend/db"
)

type stubStore struct {
	gotEmbedding []float32
	hits         []db.SearchHit
	err          error
}

func (s *stubStore) HybridSearch(_ context.Context, _ string, embedding []float32, _ int, _ db.SearchFilters) ([]db.SearchHit, error) {
	s.gotEmbedding = embedding
	return s.hits, s.err
}

type stubEmbedder struct {
	vec []float32
	err error
}

func (e stubEmbedder) Embed(context.Context, string) ([]float32, error) { return e.vec, e.err }

func TestServiceSearch(t *testing.T) {
	vec := []float32{0.1, 0.2}
	tests := []struct {
		name          string
		embedder      *stubEmbedder
		storeErr      error
		wantRetrieval string
		wantVector    bool
		wantErr       bool
	}{
		{name: "embeds the query for hybrid retrieval", embedder: &stubEmbedder{vec: vec}, wantRetrieval: RetrievalHybrid, wantVector: true},
		{name: "no embedder runs keyword-only", wantRetrieval: RetrievalKeyword},
		{name: "embedding failure degrades to keyword", embedder: &stubEmbedder{err: errors.New("budget spent")}, wantRetrieval: RetrievalKeyword},
		{name: "canceled request stops", embedder: &stubEmbedder{err: context.Canceled}, wantErr: true},
		{name: "database failure is returned", embedder: &stubEmbedder{vec: vec}, storeErr: errors.New("db down"), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &stubStore{err: tc.storeErr}
			svc := NewService(store, nil)
			if tc.embedder != nil {
				svc = NewService(store, *tc.embedder)
			}

			result, err := svc.Search(context.Background(), "slow-burn sci-fi", 10, db.SearchFilters{})
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Retrieval != tc.wantRetrieval {
				t.Errorf("retrieval = %q, want %q", result.Retrieval, tc.wantRetrieval)
			}
			if (store.gotEmbedding != nil) != tc.wantVector {
				t.Errorf("embedding passed = %v, want %v", store.gotEmbedding != nil, tc.wantVector)
			}
			if result.Hits == nil {
				t.Error("hits should be an empty slice, not nil")
			}
		})
	}
}
