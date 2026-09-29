package embed

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func vectorJSON(dims int) string {
	vals := make([]string, dims)
	for i := range vals {
		vals[i] = "0.01"
	}
	return `{"data":[{"embedding":[` + strings.Join(vals, ",") + `]}]}`
}

func TestClientEmbed(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{name: "returns the vector", status: http.StatusOK, body: vectorJSON(Dimensions)},
		{name: "surfaces the API error message", status: http.StatusTooManyRequests, body: `{"error":{"message":"rate limit reached"}}`, wantErr: "rate limit reached"},
		{name: "rejects a wrong-sized vector", status: http.StatusOK, body: vectorJSON(8), wantErr: "1536-dim"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotReq embeddingRequest
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/embeddings" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("unexpected request %s %q", r.URL.Path, r.Header.Get("Authorization"))
				}
				_ = json.NewDecoder(r.Body).Decode(&gotReq)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := NewClient("test-key", DefaultModel)
			c.baseURL = srv.URL
			vec, err := c.Embed(context.Background(), "slow-burn sci-fi")

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(vec) != Dimensions {
				t.Errorf("len = %d, want %d", len(vec), Dimensions)
			}
			if gotReq.Model != DefaultModel || gotReq.Input != "slow-burn sci-fi" {
				t.Errorf("request = %+v", gotReq)
			}
		})
	}
}

func TestClientEmbedRequiresKey(t *testing.T) {
	if _, err := NewClient("", DefaultModel).Embed(context.Background(), "x"); err == nil {
		t.Fatal("expected an error without an API key")
	}
}

type countingEmbedder struct {
	calls int
	err   error
}

func (c *countingEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return []float32{float32(len(text))}, nil
}

func TestBudgetedCachesNormalizedQueries(t *testing.T) {
	upstream := &countingEmbedder{}
	b := NewBudgeted(upstream, 10, 10)

	for _, q := range []string{"Slow Burn  Sci-Fi", "slow burn sci-fi", "  SLOW burn sci-fi "} {
		if _, err := b.Embed(context.Background(), q); err != nil {
			t.Fatal(err)
		}
	}
	if upstream.calls != 1 {
		t.Errorf("upstream calls = %d, want 1", upstream.calls)
	}
	if used, _ := b.Usage(); used != 1 {
		t.Errorf("used = %d, want 1", used)
	}
}

func TestBudgetedStopsAtDailyLimitAndResets(t *testing.T) {
	upstream := &countingEmbedder{}
	b := NewBudgeted(upstream, 2, 10)
	day := time.Date(2026, 9, 28, 23, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return day }

	for _, q := range []string{"a", "b"} {
		if _, err := b.Embed(context.Background(), q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Embed(context.Background(), "c"); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("err = %v, want ErrBudgetExhausted", err)
	}
	if _, err := b.Embed(context.Background(), "a"); err != nil {
		t.Errorf("cached query should still work after the cap: %v", err)
	}

	day = day.Add(2 * time.Hour)
	if _, err := b.Embed(context.Background(), "c"); err != nil {
		t.Errorf("budget should reset at UTC midnight: %v", err)
	}
}

func TestBudgetedEvictsLeastRecentlyUsed(t *testing.T) {
	upstream := &countingEmbedder{}
	b := NewBudgeted(upstream, 100, 2)
	ctx := context.Background()

	for _, q := range []string{"a", "b", "a", "c"} { // "b" is least recent when "c" arrives
		if _, err := b.Embed(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	calls := upstream.calls
	if _, err := b.Embed(ctx, "a"); err != nil || upstream.calls != calls {
		t.Errorf("a should still be cached")
	}
	if _, err := b.Embed(ctx, "b"); err != nil || upstream.calls != calls+1 {
		t.Errorf("b should have been evicted")
	}
}

func TestBudgetedDoesNotCacheFailures(t *testing.T) {
	upstream := &countingEmbedder{err: errors.New("upstream down")}
	b := NewBudgeted(upstream, 10, 10)
	if _, err := b.Embed(context.Background(), "a"); err == nil {
		t.Fatal("expected upstream error")
	}
	upstream.err = nil
	if _, err := b.Embed(context.Background(), "a"); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
	if upstream.calls != 2 {
		t.Errorf("calls = %d, want 2", upstream.calls)
	}
}
