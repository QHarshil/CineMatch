package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/httprate"
)

// TestRateLimiterEnforces429 verifies that a per-IP limiter returns 429
// once the request budget is exhausted. We use a custom limit of 2 req/min
// rather than the production 60 so the test doesn't require 61 round-trips.
func TestRateLimiterEnforces429(t *testing.T) {
	limit := 2
	limiter := httprate.LimitByIP(limit, time.Minute)

	noop := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := limiter(noop)

	// First `limit` requests should pass.
	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}

	// The next request from the same IP must be rate-limited.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request after limit: got %d, want 429", rec.Code)
	}
}

// TestRateLimiterIsolatesIPs confirms that two different IPs have independent budgets.
func TestRateLimiterIsolatesIPs(t *testing.T) {
	limit := 1
	limiter := httprate.LimitByIP(limit, time.Minute)

	noop := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := limiter(noop)

	exhaustIP := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":9999"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	// Exhaust IP A.
	if code := exhaustIP("10.0.0.1"); code != http.StatusOK {
		t.Fatalf("IP A first request: got %d, want 200", code)
	}
	if code := exhaustIP("10.0.0.1"); code != http.StatusTooManyRequests {
		t.Fatalf("IP A second request: got %d, want 429", code)
	}

	// IP B should be unaffected.
	if code := exhaustIP("10.0.0.2"); code != http.StatusOK {
		t.Fatalf("IP B first request: got %d, want 200", code)
	}
}
