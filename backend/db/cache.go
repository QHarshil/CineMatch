package db

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// PopularLister loads movies in popularity order. Implemented by SupabaseClient.
type PopularLister interface {
	ListMovies(ctx context.Context, limit, offset int) ([]Movie, error)
}

// PopularMoviesCache holds an in-memory copy of the top popular movies.
// If Supabase becomes unreachable, the Go backend serves this cached snapshot
// so users still see content instead of an error page.
type PopularMoviesCache struct {
	mu     sync.RWMutex
	movies []Movie
	source PopularLister
}

// NewPopularMoviesCache loads the cache once so it is warm on startup, then
// refreshes it every refreshInterval until ctx is done.
func NewPopularMoviesCache(ctx context.Context, source PopularLister, refreshInterval time.Duration) *PopularMoviesCache {
	c := &PopularMoviesCache{source: source}
	c.refresh(ctx)
	go c.backgroundRefresh(ctx, refreshInterval)
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

func (c *PopularMoviesCache) refresh(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	movies, err := c.source.ListMovies(ctx, 50, 0)
	if err != nil {
		slog.Warn("failed to refresh popular movies cache", "error", err)
		return
	}
	c.mu.Lock()
	c.movies = movies
	c.mu.Unlock()
	slog.Info("popular movies cache refreshed", "count", len(movies))
}

func (c *PopularMoviesCache) backgroundRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}
