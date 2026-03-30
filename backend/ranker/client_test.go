package ranker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/ranker"
)

var testCandidates = []db.MovieCandidate{
	{
		Movie: db.Movie{
			ID:          "aaaaaaaa-0000-0000-0000-000000000001",
			TmdbID:      27205,
			Title:       "Inception",
			Genres:      []string{"Action", "Science Fiction"},
			ReleaseYear: 2010,
			VoteAverage: 8.8,
			Popularity:  850.0,
			Runtime:     148,
		},
		Similarity: 0.92,
	},
	{
		Movie: db.Movie{
			ID:          "aaaaaaaa-0000-0000-0000-000000000002",
			TmdbID:      155,
			Title:       "The Dark Knight",
			Genres:      []string{"Action", "Drama"},
			ReleaseYear: 2008,
			VoteAverage: 9.0,
			Popularity:  700.0,
			Runtime:     152,
		},
		Similarity: 0.88,
	},
}

func TestClientRank(t *testing.T) {
	t.Run("successful rank request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/rank" {
				t.Fatalf("expected /rank, got %s", r.URL.Path)
			}
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("expected application/json, got %s", ct)
			}

			// Verify the request body shape.
			var body struct {
				Candidates   []json.RawMessage `json:"candidates"`
				UserFeatures json.RawMessage   `json:"user_features"`
				TopN         int               `json:"top_n"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding request body: %v", err)
			}
			if len(body.Candidates) != 2 {
				t.Fatalf("expected 2 candidates, got %d", len(body.Candidates))
			}
			if body.TopN != 5 {
				t.Fatalf("expected top_n=5, got %d", body.TopN)
			}

			resp := map[string]any{
				"ranked": []map[string]any{
					{"movie_id": "aaaaaaaa-0000-0000-0000-000000000001", "score": 0.91, "rank": 1},
					{"movie_id": "aaaaaaaa-0000-0000-0000-000000000002", "score": 0.87, "rank": 2},
				},
				"model_version": "feature-linear-v1",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := ranker.NewClient(srv.URL)
		result, err := client.Rank(context.Background(), testCandidates, 5, []string{"Action"}, 7.0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Ranked) != 2 {
			t.Fatalf("expected 2 ranked, got %d", len(result.Ranked))
		}
		if result.ModelVersion != "feature-linear-v1" {
			t.Errorf("model_version = %q, want %q", result.ModelVersion, "feature-linear-v1")
		}
		if result.Ranked[0].MovieID != "aaaaaaaa-0000-0000-0000-000000000001" {
			t.Errorf("first ranked = %q, want aaaaaaaa-...-1", result.Ranked[0].MovieID)
		}
	})

	t.Run("ranker returns non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := ranker.NewClient(srv.URL)
		_, err := client.Rank(context.Background(), testCandidates, 5, nil, 0)
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})

	t.Run("ranker unreachable", func(t *testing.T) {
		client := ranker.NewClient("http://127.0.0.1:1") // nothing listening
		_, err := client.Rank(context.Background(), testCandidates, 5, nil, 0)
		if err == nil {
			t.Fatal("expected error for unreachable ranker")
		}
	})

	t.Run("passes empty genres as empty array", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				UserFeatures struct {
					PreferredGenres []string `json:"preferred_genres"`
				} `json:"user_features"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			// nil genres should be serialized as empty array, not null
			if body.UserFeatures.PreferredGenres == nil {
				t.Error("preferred_genres should not be null")
			}
			resp := map[string]any{
				"ranked":        []any{},
				"model_version": "test",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := ranker.NewClient(srv.URL)
		_, err := client.Rank(context.Background(), testCandidates, 5, []string{}, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
