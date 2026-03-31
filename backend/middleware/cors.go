// Package middleware contains HTTP middleware for the CineMatch API server.
package middleware

import (
	"net/http"
	"os"
	"strings"

	chiCors "github.com/go-chi/cors"
)

// CORSHandler returns a CORS middleware driven by the ALLOWED_ORIGINS env var.
// Origins are comma-separated (e.g. "http://localhost:3000,https://cinematch.harshilc.com").
// Wildcards are intentionally excluded to prevent credential leakage from authenticated routes.
func CORSHandler() func(http.Handler) http.Handler {
	raw := os.Getenv("ALLOWED_ORIGINS")
	origins := parseOrigins(raw)

	return chiCors.Handler(chiCors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "X-Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

func parseOrigins(raw string) []string {
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(o)
		trimmed = strings.TrimRight(trimmed, "/")
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	// Fallback to localhost only so the server remains usable locally without a misconfigured env.
	if len(origins) == 0 {
		return []string{"http://localhost:3000"}
	}
	return origins
}
