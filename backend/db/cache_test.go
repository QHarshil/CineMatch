package db

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// scriptedLister returns one scripted response per call, repeating the last.
type scriptedLister struct {
	mu        sync.Mutex
	calls     int
	responses []struct {
		movies []Movie
		err    error
	}
}

func (l *scriptedLister) ListMovies(context.Context, int, int) ([]Movie, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := l.responses[min(l.calls, len(l.responses)-1)]
	l.calls++
	return r.movies, r.err
}

func (l *scriptedLister) callCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

func TestPopularMoviesCache(t *testing.T) {
	down := errors.New("database unreachable")
	lister := &scriptedLister{responses: []struct {
		movies []Movie
		err    error
	}{
		{err: down},
		{movies: []Movie{{ID: "m1", Title: "Arrival"}}},
		{err: down},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := NewPopularMoviesCache(ctx, lister, 10*time.Millisecond)
	if cache.Get() != nil {
		t.Fatal("a failed first load should leave the cache empty")
	}

	deadline := time.Now().Add(time.Second)
	for cache.Get() == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	got := cache.Get()
	if len(got) != 1 || got[0].Title != "Arrival" {
		t.Fatalf("after a refresh the cache holds %+v", got)
	}

	// A failed refresh keeps the last good snapshot, and Get returns a copy.
	got[0].Title = "changed"
	time.Sleep(30 * time.Millisecond)
	if again := cache.Get(); len(again) != 1 || again[0].Title != "Arrival" {
		t.Errorf("snapshot changed to %+v", again)
	}

	cancel()
	time.Sleep(20 * time.Millisecond)
	stopped := lister.callCount()
	time.Sleep(40 * time.Millisecond)
	if n := lister.callCount(); n != stopped {
		t.Errorf("refreshed %d more times after the context ended", n-stopped)
	}
}
