package middleware

import (
	"net/http"
)

// MaxBodySize rejects any request with a body exceeding maxBytes.
// Applied globally to prevent oversized payloads from consuming memory.
// GET/HEAD/OPTIONS requests without bodies are unaffected.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				w.Write([]byte(`{"error":"request body too large"}`)) //nolint:errcheck
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
