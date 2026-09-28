package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP sets r.RemoteAddr to the caller's IP for rate limiting and quotas.
//
// Proxies append the address they saw to X-Forwarded-For, so only the last
// trustedHops entries were written by infrastructure; anything before them
// came from the client and can be forged. chi's RealIP trusts the first
// entry, which would let a caller choose their own rate-limit key. Behind
// Cloud Run set trustedHops to 1 (Google's front end appends the client IP);
// with 0 the header is ignored and the socket address is used.
func ClientIP(trustedHops int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = clientIP(r, trustedHops)
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request, trustedHops int) string {
	if trustedHops > 0 {
		var hops []string
		for _, header := range r.Header.Values("X-Forwarded-For") {
			for _, part := range strings.Split(header, ",") {
				if part = strings.TrimSpace(part); part != "" {
					hops = append(hops, part)
				}
			}
		}
		if len(hops) >= trustedHops {
			if ip := net.ParseIP(hops[len(hops)-trustedHops]); ip != nil {
				return ip.String()
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
