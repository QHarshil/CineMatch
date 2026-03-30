package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders adds standard security headers to every API response.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
		})
	}
}

// RequireJSONContentType rejects POST/PUT/PATCH requests that don't send
// application/json. This prevents content-type confusion attacks.
func RequireJSONContentType() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				ct := r.Header.Get("Content-Type")
				if !strings.HasPrefix(ct, "application/json") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnsupportedMediaType)
					w.Write([]byte(`{"error":"Content-Type must be application/json"}`)) //nolint:errcheck
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
