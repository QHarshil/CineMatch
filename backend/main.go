package main

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/harshilc/cinematch-backend/assistant"
	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/embed"
	"github.com/harshilc/cinematch-backend/handlers"
	"github.com/harshilc/cinematch-backend/llm"
	custommw "github.com/harshilc/cinematch-backend/middleware"
	"github.com/harshilc/cinematch-backend/omdb"
	"github.com/harshilc/cinematch-backend/ranker"
	"github.com/harshilc/cinematch-backend/search"
	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: cloudLoggingAttr})))

	// The repo-root .env is shared with scripts/. Cloud Run injects env vars
	// directly, so a missing file is expected there.
	if err := godotenv.Load("../.env"); err != nil {
		slog.Info("no ../.env found, reading environment variables directly")
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

	// OMDb supplies IMDb / Rotten Tomatoes scores that TMDB lacks. The key is
	// optional: without it the ratings endpoint returns empty bodies.
	var ratingsFetcher handlers.RatingsFetcher
	if omdbKey := os.Getenv("OMDB_API_KEY"); omdbKey != "" {
		ratingsFetcher = omdb.NewClient(omdbKey)
	} else {
		slog.Info("OMDB_API_KEY not set, movie ratings disabled")
	}
	ratingsCache := omdb.NewCache(24 * time.Hour)

	// Query vectors must come from the model that embedded the catalog, so they
	// use OpenAI. Without a key, search runs keyword and title matching only.
	var queryEmbedder embed.Embedder
	if openAIKey := os.Getenv("OPENAI_API_KEY"); openAIKey != "" {
		queryEmbedder = embed.NewBudgeted(embed.NewClient(openAIKey), envInt("EMBED_DAILY_LIMIT", 5000), 1000)
	} else {
		slog.Info("OPENAI_API_KEY not set, search runs keyword-only")
	}
	titleSearch := search.NewService(supabase, queryEmbedder)
	recommendations := handlers.NewRecommendationPipeline(supabase, movieRanker, popularCache)

	// The assistant speaks the OpenAI chat API, so LLM_BASE_URL can point at
	// Ollama locally or a hosted provider in production. Without it the
	// assistant answers from search alone.
	var chatModel assistant.ChatModel
	if baseURL, model := os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_MODEL"); baseURL != "" && model != "" {
		chatModel = llm.NewClient(llm.Config{
			BaseURL:         baseURL,
			APIKey:          os.Getenv("LLM_API_KEY"),
			Model:           model,
			ReasoningEffort: os.Getenv("LLM_REASONING_EFFORT"),
			Temperature:     0.3,
		})
	} else {
		slog.Info("LLM_BASE_URL or LLM_MODEL not set, assistant serves search results only")
	}
	agent := assistant.New(chatModel, titleSearch, supabase, assistant.RecommenderFunc(
		func(ctx context.Context, userID string) ([]db.Movie, string, error) {
			feed, err := recommendations.Recommend(ctx, userID)
			return feed.Movies, feed.Source, err
		}))
	assistantLimits := handlers.AssistantLimits{
		UserDailyRuns:     envInt("ASSISTANT_USER_DAILY_RUNS", 25),
		GuestDailyRuns:    envInt("ASSISTANT_GUEST_DAILY_RUNS", 8),
		IPDailyRuns:       envInt("ASSISTANT_IP_DAILY_RUNS", 20),
		GlobalDailyRuns:   envInt("ASSISTANT_DAILY_RUNS", 1000),
		GlobalDailyTokens: envInt("ASSISTANT_DAILY_TOKENS", 2_000_000),
		IPHashKey:         ipHashKey(jwtSecret),
	}

	r := chi.NewRouter()

	// Middleware order matters: RequestID and ClientIP must come before logging
	// and rate limiting so both see the request ID and the real client IP.
	// Cloud Run sets TRUSTED_PROXY_HOPS=1; locally the header is ignored.
	r.Use(middleware.RequestID)
	r.Use(custommw.ClientIP(envInt("TRUSTED_PROXY_HOPS", 0)))
	r.Use(custommw.StructuredLogger())
	r.Use(middleware.Recoverer)
	r.Use(custommw.CORSHandler(custommw.ParseOrigins(os.Getenv("ALLOWED_ORIGINS"))))
	r.Use(custommw.RateLimiter(envInt("RATE_LIMIT_RPM", 60)))
	r.Use(custommw.SecurityHeaders())
	r.Use(custommw.RequireJSONContentType())
	r.Use(custommw.MaxBodySize(10 * 1024)) // 10KB global body limit

	bootTime := time.Now()

	r.Get("/health", handlers.Health(supabase, bootTime))

	// Public endpoints: no auth required.
	r.Get("/movies", handlers.ListMovies(supabase, popularCache))
	r.Get("/movies/{id}", handlers.GetMovieByID(supabase))
	r.Get("/movies/{id}/ratings", handlers.GetMovieRatings(supabase, ratingsFetcher, ratingsCache))
	r.With(custommw.SearchRateLimiter()).Get("/search", handlers.SearchMovies(supabase, popularCache))
	r.With(custommw.SearchRateLimiter()).Get("/discover", handlers.DiscoverTitles(titleSearch, popularCache))

	// Authenticated endpoints: require a valid Supabase JWT.
	// jwtSecret is captured once at startup so every request avoids an os.Getenv call.
	r.Group(func(r chi.Router) {
		r.Use(custommw.RequireAuth(jwtSecret, custommw.SupabaseJWKSURL(os.Getenv("SUPABASE_URL"))))
		r.With(custommw.RecommendRateLimiter()).Get("/recommend", handlers.RecommendForUser(supabase, movieRanker, popularCache))
		r.With(custommw.WriteRateLimiter()).Post("/interactions", handlers.ToggleInteraction(supabase))
		r.Get("/interactions", handlers.GetMovieInteractionState(supabase))
		r.With(custommw.WriteRateLimiter()).Put("/ratings", handlers.RecordRating(supabase))
		r.With(custommw.AssistantRateLimiter()).Post("/assistant", handlers.RunAssistant(agent, supabase, assistantLimits))
		r.Get("/assistant/usage", handlers.GetAssistantUsage(agent, supabase, assistantLimits))
	})

	// Cloud Run injects PORT; locally APP_PORT or 8080.
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("APP_PORT")
	}
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

	// Cloud Run sends SIGTERM before stopping an instance; drain in-flight requests.
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

// envInt reads a positive integer setting, falling back to def when unset or
// invalid.
func envInt(name string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n > 0 {
		return n
	}
	return def
}

// ipHashKey derives a key for hashing client IPs from a server secret, so the
// audit log never holds raw addresses and the hashes cannot be reversed by
// hashing every IPv4 address.
func ipHashKey(secret string) []byte {
	sum := sha256.Sum256([]byte("assistant-ip-quota:" + secret))
	return sum[:]
}

// cloudLoggingAttr renames slog's level and msg keys to the severity and
// message fields Cloud Logging reads, so each line is indexed as a
// structured entry with the right severity.
func cloudLoggingAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.LevelKey:
		a.Key = "severity"
	case slog.MessageKey:
		a.Key = "message"
	}
	return a
}
