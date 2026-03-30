package middleware

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/httprate"
)

// RateLimiter enforces a per-IP request limit read from RATE_LIMIT_RPM (default 60).
// Excess requests receive HTTP 429 with a Retry-After header.
// The limit is per-IP so one client cannot starve others.
func RateLimiter() func(http.Handler) http.Handler {
	rpm := 60
	if raw := os.Getenv("RATE_LIMIT_RPM"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			rpm = n
		}
	}
	return httprate.LimitByIP(rpm, time.Minute)
}

// SearchRateLimiter enforces 30 requests/min per IP on the search endpoint.
func SearchRateLimiter() func(http.Handler) http.Handler {
	return httprate.LimitByIP(30, time.Minute)
}

// WriteRateLimiter enforces 20 requests/min per authenticated user on write endpoints.
// Falls back to per-IP limiting for unauthenticated requests (which will be rejected
// by RequireAuth anyway).
func WriteRateLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(20, time.Minute, httprate.WithKeyFuncs(userOrIPKey))
}

// RecommendRateLimiter enforces 10 requests/min per authenticated user.
func RecommendRateLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(10, time.Minute, httprate.WithKeyFuncs(userOrIPKey))
}

// userOrIPKey extracts the authenticated user ID from request context for rate
// limiting keyed by user. Falls back to remote IP if no user is in context.
func userOrIPKey(r *http.Request) (string, error) {
	if uid, ok := UserIDFromContext(r.Context()); ok {
		return "user:" + uid, nil
	}
	return "ip:" + r.RemoteAddr, nil
}
