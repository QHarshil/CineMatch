package omdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient points an OMDb client at a stub server returning canned JSON.
func newTestClient(t *testing.T, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := NewClient("testkey")
	c.baseURL = srv.URL + "/"
	return c
}

func TestFetch(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantIMDb *float64
		wantRT   *int
	}{
		{
			name:     "imdb and rotten tomatoes present",
			body:     `{"Response":"True","imdbRating":"8.8","Ratings":[{"Source":"Internet Movie Database","Value":"8.8/10"},{"Source":"Rotten Tomatoes","Value":"87%"}]}`,
			wantIMDb: ptrFloat(8.8),
			wantRT:   ptrInt(87),
		},
		{
			name:     "imdb present, no rotten tomatoes",
			body:     `{"Response":"True","imdbRating":"7.0","Ratings":[{"Source":"Internet Movie Database","Value":"7.0/10"}]}`,
			wantIMDb: ptrFloat(7.0),
			wantRT:   nil,
		},
		{
			name:     "imdb N/A is omitted",
			body:     `{"Response":"True","imdbRating":"N/A","Ratings":[]}`,
			wantIMDb: nil,
			wantRT:   nil,
		},
		{
			name:     "no match returns nil ratings",
			body:     `{"Response":"False","Error":"Movie not found!"}`,
			wantIMDb: nil,
			wantRT:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.body)
			got, err := c.Fetch(context.Background(), "Some Title", 2010, "movie")
			if err != nil {
				t.Fatalf("Fetch returned error: %v", err)
			}

			if tc.wantIMDb == nil && tc.wantRT == nil {
				// A "no match" body yields a nil Ratings; a matched body with no
				// usable scores yields a non-nil Ratings with nil fields. Either
				// way both scores must be absent.
				if got != nil && (got.IMDb != nil || got.RT != nil) {
					t.Fatalf("expected no scores, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected ratings, got nil")
			}
			assertFloatPtr(t, "IMDb", got.IMDb, tc.wantIMDb)
			assertIntPtr(t, "RT", got.RT, tc.wantRT)
		})
	}
}

func TestFetchRequiresAPIKey(t *testing.T) {
	c := NewClient("")
	if _, err := c.Fetch(context.Background(), "Title", 2000, "movie"); err == nil {
		t.Fatal("expected error when api key is empty")
	}
}

func TestCache(t *testing.T) {
	c := NewCache(20 * time.Millisecond)
	ratings := &Ratings{IMDb: ptrFloat(9.0)}

	if _, ok := c.Get("missing"); ok {
		t.Fatal("unknown id should be a miss")
	}

	c.Set("movie-1", ratings)
	got, ok := c.Get("movie-1")
	if !ok || got != ratings {
		t.Fatalf("expected cache hit, got ok=%v value=%+v", ok, got)
	}

	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get("movie-1"); ok {
		t.Fatal("entry should expire after the TTL")
	}
}

func ptrFloat(f float64) *float64 { return &f }
func ptrInt(n int) *int           { return &n }

func assertFloatPtr(t *testing.T, label string, got, want *float64) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("%s presence mismatch: got %v want %v", label, got, want)
	}
	if got != nil && *got != *want {
		t.Errorf("%s = %v, want %v", label, *got, *want)
	}
}

func assertIntPtr(t *testing.T, label string, got, want *int) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("%s presence mismatch: got %v want %v", label, got, want)
	}
	if got != nil && *got != *want {
		t.Errorf("%s = %v, want %v", label, *got, *want)
	}
}
