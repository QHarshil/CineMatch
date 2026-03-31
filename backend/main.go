package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
	custommw "github.com/harshilc/cinematch-backend/middleware"
	"github.com/harshilc/cinematch-backend/ranker"
	"github.com/joho/godotenv"
)

func main() {
	// In production (Railway), env vars are injected directly.
	// godotenv.Load is a no-op if .env is absent, which is expected in production.
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, reading environment variables directly")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET is required but not set")
		os.Exit(1)
	}

	rankerURL := os.Getenv("RANKER_URL")
	if rankerURL == "" {
		rankerURL = "http://localhost:8000"
	}
	movieRanker := ranker.NewClient(rankerURL)

	supabase := db.NewSupabaseClient(
		os.Getenv("SUPABASE_URL"),
		os.Getenv("SUPABASE_SECRET_KEY"),
	)

	// Cache top 50 popular movies in memory, refreshed hourly.
	// Serves as fallback when Supabase is temporarily unreachable.
	popularCache := db.NewPopularMoviesCache(supabase, 1*time.Hour)

	r := chi.NewRouter()

	// Middleware order matters: RequestID and RealIP must come before logging
	// so log lines include the request ID and real client IP.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(custommw.StructuredLogger())
	r.Use(middleware.Recoverer)
	r.Use(custommw.CORSHandler())
	r.Use(custommw.RateLimiter())
	r.Use(custommw.SecurityHeaders())
	r.Use(custommw.RequireJSONContentType())
	r.Use(custommw.MaxBodySize(10 * 1024)) // 10KB global body limit

	bootTime := time.Now()

	r.Get("/health", handlers.Health(supabase, bootTime))

	// Public endpoints — no auth required.
	r.Get("/movies", handlers.ListMovies(supabase, popularCache))
	r.Get("/movies/{id}", handlers.GetMovieByID(supabase))
	r.With(custommw.SearchRateLimiter()).Get("/search", handlers.SearchMovies(supabase, popularCache))

	// Authenticated endpoints — require a valid Supabase JWT.
	// jwtSecret is captured once at startup so every request avoids an os.Getenv call.
	r.Group(func(r chi.Router) {
		r.Use(custommw.RequireAuth(jwtSecret))
		r.With(custommw.RecommendRateLimiter()).Get("/recommend", handlers.RecommendForUser(supabase, movieRanker, popularCache))
		r.With(custommw.WriteRateLimiter()).Post("/interactions", handlers.ToggleInteraction(supabase))
		r.Get("/interactions", handlers.GetMovieInteractionState(supabase))
		r.With(custommw.WriteRateLimiter()).Put("/ratings", handlers.RecordRating(supabase))
	})

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Capture SIGINT/SIGTERM so Railway can shut down the container cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("cinematch backend ready", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown signal received, draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped cleanly")
}
