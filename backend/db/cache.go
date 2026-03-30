package db

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// PopularMoviesCache holds an in-memory copy of the top popular movies.
// If Supabase becomes unreachable, the Go backend serves this cached snapshot
// so users still see content instead of an error page.
type PopularMoviesCache struct {
	mu     sync.RWMutex
	movies []Movie
	client *SupabaseClient
}

// NewPopularMoviesCache creates a cache that refreshes every refreshInterval.
// It performs an initial synchronous load so the cache is warm on startup.
func NewPopularMoviesCache(client *SupabaseClient, refreshInterval time.Duration) *PopularMoviesCache {
	c := &PopularMoviesCache{client: client}
	c.refresh()
	go c.backgroundRefresh(refreshInterval)
	return c
}

// Get returns the cached popular movies. Returns nil if the cache is empty
// (only happens if the initial load also failed).
func (c *PopularMoviesCache) Get() []Movie {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return a copy so callers can't mutate the cache.
	if len(c.movies) == 0 {
		return nil
	}
	out := make([]Movie, len(c.movies))
	copy(out, c.movies)
	return out
}

func (c *PopularMoviesCache) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	movies, err := c.client.ListMovies(ctx, 50, 0)
	if err != nil {
		slog.Warn("failed to refresh popular movies cache", "error", err)
		return
	}
	c.mu.Lock()
	c.movies = movies
	c.mu.Unlock()
	slog.Info("popular movies cache refreshed", "count", len(movies))
}

func (c *PopularMoviesCache) backgroundRefresh(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		c.refresh()
	}
}
