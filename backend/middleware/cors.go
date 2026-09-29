// Package middleware contains HTTP middleware for the CineMatch API server.
package middleware

import (
	"net/http"
	"strings"

	chiCors "github.com/go-chi/cors"
)

// CORSHandler returns a CORS middleware that allows the given origins.
// Wildcards are intentionally excluded to prevent credential leakage from authenticated routes.
func CORSHandler(origins []string) func(http.Handler) http.Handler {
	return chiCors.Handler(chiCors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "X-Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

// ParseOrigins splits a comma-separated origin list such as
// "http://localhost:3000,https://cinematch.harshilc.com", dropping trailing
// slashes browsers never send.
func ParseOrigins(raw string) []string {
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(o)
		trimmed = strings.TrimRight(trimmed, "/")
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	// Default to localhost when ALLOWED_ORIGINS is unset.
	if len(origins) == 0 {
		return []string{"http://localhost:3000"}
	}
	return origins
}
