package embed

import (
	"container/list"
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// ErrBudgetExhausted is returned once the daily call cap is reached. Callers
// fall back to keyword search.
var ErrBudgetExhausted = errors.New("embed: daily budget exhausted")

// Budgeted wraps an Embedder with an LRU cache and a per-UTC-day cap on
// upstream calls, so repeated queries are free and a traffic spike cannot run
// up the bill. Cache hits do not count against the cap. The cap is per
// instance; total spend is bounded by cap times the Cloud Run max instances.
type Budgeted struct {
	next       Embedder
	dailyLimit int
	now        func() time.Time

	mu      sync.Mutex
	day     string
	used    int
	entries map[string]*list.Element
	order   *list.List // front = most recently used
	maxSize int
}

type cacheEntry struct {
	key    string
	vector []float32
}

// NewBudgeted caps upstream calls at dailyLimit per day and caches up to
// cacheSize distinct queries (about 6 KB each).
func NewBudgeted(next Embedder, dailyLimit, cacheSize int) *Budgeted {
	return &Budgeted{
		next:       next,
		dailyLimit: dailyLimit,
		now:        time.Now,
		entries:    make(map[string]*list.Element, cacheSize),
		order:      list.New(),
		maxSize:    cacheSize,
	}
}

// Embed returns a cached vector when available, otherwise spends one unit of
// the daily budget on an upstream call.
func (b *Budgeted) Embed(ctx context.Context, text string) ([]float32, error) {
	key := normalizeQuery(text)

	b.mu.Lock()
	if el, ok := b.entries[key]; ok {
		b.order.MoveToFront(el)
		vec := el.Value.(*cacheEntry).vector
		b.mu.Unlock()
		return vec, nil
	}
	b.rollDay()
	if b.used >= b.dailyLimit {
		b.mu.Unlock()
		return nil, ErrBudgetExhausted
	}
	// Reserve before calling so concurrent misses cannot overshoot the cap.
	b.used++
	b.mu.Unlock()

	vec, err := b.next.Embed(ctx, key)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	b.store(key, vec)
	b.mu.Unlock()
	return vec, nil
}

// Usage reports upstream calls spent today and the daily cap.
func (b *Budgeted) Usage() (used, limit int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rollDay()
	return b.used, b.dailyLimit
}

// rollDay resets the counter at UTC midnight. Callers hold b.mu.
func (b *Budgeted) rollDay() {
	today := b.now().UTC().Format(time.DateOnly)
	if today != b.day {
		b.day = today
		b.used = 0
	}
}

// store inserts a vector and evicts the least recently used entry when full.
// Callers hold b.mu.
func (b *Budgeted) store(key string, vec []float32) {
	if el, ok := b.entries[key]; ok {
		b.order.MoveToFront(el)
		return
	}
	b.entries[key] = b.order.PushFront(&cacheEntry{key: key, vector: vec})
	if b.order.Len() > b.maxSize {
		oldest := b.order.Back()
		b.order.Remove(oldest)
		delete(b.entries, oldest.Value.(*cacheEntry).key)
	}
}

// normalizeQuery folds case and whitespace so trivially different spellings
// of the same query share a cache entry.
func normalizeQuery(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}
