package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
	"github.com/harshilc/cinematch-backend/search"
)

type stubTitleSearch struct {
	gotQuery   string
	gotLimit   int
	gotFilters db.SearchFilters
	result     search.Result
	err        error
}

func (s *stubTitleSearch) Search(_ context.Context, q string, limit int, f db.SearchFilters) (search.Result, error) {
	s.gotQuery, s.gotLimit, s.gotFilters = q, limit, f
	return s.result, s.err
}

func TestDiscoverTitles(t *testing.T) {
	sim := 0.61
	hits := []db.SearchHit{{Movie: sampleMovies[0], Similarity: &sim, Score: 0.03}}

	tests := []struct {
		name          string
		query         string
		searchErr     error
		wantStatus    int
		wantCount     int
		wantRetrieval string
		wantFilters   db.SearchFilters
		wantLimit     int
	}{
		{
			name:          "returns hybrid results",
			query:         "?q=dream+heist",
			wantStatus:    http.StatusOK,
			wantCount:     1,
			wantRetrieval: search.RetrievalHybrid,
			wantLimit:     20,
		},
		{
			name:          "parses every filter",
			query:         "?q=space&type=tv&genre=Drama,Sci-Fi+%26+Fantasy&year_min=2010&year_max=2020&rating_min=7.5&runtime_max=60&lang=ko&limit=5",
			wantStatus:    http.StatusOK,
			wantCount:     1,
			wantRetrieval: search.RetrievalHybrid,
			wantLimit:     5,
			wantFilters: db.SearchFilters{
				MediaType: "tv", Genres: []string{"Drama", "Sci-Fi & Fantasy"},
				MinYear: 2010, MaxYear: 2020, MinRating: 7.5, MaxRuntime: 60, Language: "ko",
			},
		},
		{name: "requires q", query: "", wantStatus: http.StatusBadRequest},
		{name: "strips tags before validating q", query: "?q=%3Cb%3E%3C%2Fb%3E", wantStatus: http.StatusBadRequest},
		{name: "rejects long q", query: "?q=" + strings.Repeat("a", 201), wantStatus: http.StatusBadRequest},
		{name: "rejects unknown type", query: "?q=x&type=anime", wantStatus: http.StatusBadRequest},
		{name: "rejects inverted year range", query: "?q=x&year_min=2020&year_max=2010", wantStatus: http.StatusBadRequest},
		{name: "rejects rating above 10", query: "?q=x&rating_min=11", wantStatus: http.StatusBadRequest},
		{name: "rejects bad language code", query: "?q=x&lang=korean", wantStatus: http.StatusBadRequest},
		{name: "rejects too many genres", query: "?q=x&genre=a,b,c,d,e,f", wantStatus: http.StatusBadRequest},
		{name: "rejects limit above 50", query: "?q=x&limit=51", wantStatus: http.StatusBadRequest},
		{
			name:          "falls back to cached title matches",
			query:         "?q=inception",
			searchErr:     errors.New("db unreachable"),
			wantStatus:    http.StatusOK,
			wantCount:     1,
			wantRetrieval: "cached",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			titles := &stubTitleSearch{
				result: search.Result{Hits: hits, Retrieval: search.RetrievalHybrid},
				err:    tc.searchErr,
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/discover"+tc.query, nil)
			handlers.DiscoverTitles(titles, &stubCache{movies: sampleMovies})(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			var body struct {
				Results   []db.SearchHit `json:"results"`
				Retrieval string         `json:"retrieval"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body.Results) != tc.wantCount || body.Retrieval != tc.wantRetrieval {
				t.Errorf("got %d results via %q, want %d via %q", len(body.Results), body.Retrieval, tc.wantCount, tc.wantRetrieval)
			}
			if tc.wantLimit != 0 && titles.gotLimit != tc.wantLimit {
				t.Errorf("limit = %d, want %d", titles.gotLimit, tc.wantLimit)
			}
			if tc.wantFilters.MediaType != "" {
				got, _ := json.Marshal(titles.gotFilters)
				want, _ := json.Marshal(tc.wantFilters)
				if string(got) != string(want) {
					t.Errorf("filters = %s, want %s", got, want)
				}
			}
		})
	}
}
