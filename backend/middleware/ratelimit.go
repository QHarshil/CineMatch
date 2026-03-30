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
