package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// StructuredLogger returns a middleware that emits one JSON log line per request.
// It captures status code via chi's WrapResponseWriter so logging happens after
// the handler writes, giving us the real response code.
func StructuredLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				slog.Info("request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"latency_ms", time.Since(start).Milliseconds(),
					"bytes", ww.BytesWritten(),
					"request_id", middleware.GetReqID(r.Context()),
					"remote_addr", r.RemoteAddr,
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
