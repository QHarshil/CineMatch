package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
	custommw "github.com/harshilc/cinematch-backend/middleware"
	"github.com/joho/godotenv"
)

func main() {
	// In production (Railway), env vars are injected directly.
	// godotenv.Load is a no-op if .env is absent, which is expected in production.
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, reading environment variables directly")
	}

	supabase := db.NewSupabaseClient(
		os.Getenv("SUPABASE_URL"),
		os.Getenv("SUPABASE_SECRET_KEY"),
	)

	r := chi.NewRouter()

	// Middleware order matters: RequestID and RealIP must come before logging
	// so log lines include the request ID and real client IP.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(custommw.StructuredLogger())
	r.Use(middleware.Recoverer)
	r.Use(custommw.CORSHandler())
	r.Use(custommw.RateLimiter())

	bootTime := time.Now()
	r.Get("/health", handlers.Health(supabase, bootTime))

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("cinematch backend ready", "port", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
