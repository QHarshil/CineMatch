// Package omdb fetches aggregate IMDb and Rotten Tomatoes ratings from the OMDb
// API. TMDB does not carry these scores, so the movie detail page enriches a
// title via OMDb, looked up by title + release year + type. An in-memory TTL
// cache keeps OMDb calls well under the free tier's daily limit.
package omdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "http://www.omdbapi.com/"

// Client queries the OMDb API with a configured key.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient returns an OMDb client. A 5-second timeout keeps a slow OMDb from
// blocking the ratings endpoint.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Ratings holds the aggregate scores OMDb returns, and doubles as the JSON
// shape served by the ratings endpoint. Either field may be nil when OMDb has
// no value for that source (common for TV and obscure titles).
type Ratings struct {
	IMDb *float64 `json:"imdb_rating"` // 0-10
	RT   *int     `json:"rt_rating"`   // 0-100
}

// omdbResponse mirrors the subset of OMDb's JSON used here.
type omdbResponse struct {
	Response   string `json:"Response"`
	ImdbRating string `json:"imdbRating"`
	Ratings    []struct {
		Source string `json:"Source"`
		Value  string `json:"Value"`
	} `json:"Ratings"`
}

// Fetch looks up a title's ratings by name, year, and media type ("movie" or
// "tv", mapped to OMDb's "movie"/"series"). It returns (nil, nil) when OMDb has
// no match, so callers can cache the miss without treating it as an error.
func (c *Client) Fetch(ctx context.Context, title string, year int, mediaType string) (*Ratings, error) {
	if c.apiKey == "" {
		return nil, errors.New("omdb: api key not configured")
	}

	q := url.Values{}
	q.Set("apikey", c.apiKey)
	q.Set("t", title)
	if year > 0 {
		q.Set("y", strconv.Itoa(year))
	}
	if mediaType == "tv" {
		q.Set("type", "series")
	} else {
		q.Set("type", "movie")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("omdb: building request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("omdb: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("omdb: HTTP %d", resp.StatusCode)
	}

	var body omdbResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("omdb: decoding response: %w", err)
	}
	// OMDb signals "no match" with Response:"False" rather than an HTTP error.
	if !strings.EqualFold(body.Response, "True") {
		return nil, nil
	}

	out := &Ratings{}
	if v, err := strconv.ParseFloat(body.ImdbRating, 64); err == nil {
		out.IMDb = &v
	}
	for _, r := range body.Ratings {
		if r.Source == "Rotten Tomatoes" {
			if n, err := strconv.Atoi(strings.TrimSuffix(r.Value, "%")); err == nil {
				out.RT = &n
			}
		}
	}
	return out, nil
}

// Cache is a concurrency-safe TTL cache of ratings keyed by movie ID. It lets
// the ratings endpoint serve repeat views without re-querying OMDb.
type Cache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

type cacheEntry struct {
	ratings  *Ratings
	cachedAt time.Time
}

// NewCache returns a ratings cache with the given freshness window.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, entries: make(map[string]cacheEntry)}
}

// Get returns the cached ratings for id and whether a fresh entry exists.
func (c *Cache) Get(id string) (*Ratings, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[id]
	if !ok || time.Since(entry.cachedAt) > c.ttl {
		return nil, false
	}
	return entry.ratings, true
}

// Set stores ratings for id, stamped with the current time. A nil ratings value
// is cached too, so a known OMDb miss is not re-fetched until the TTL elapses.
func (c *Cache) Set(id string, ratings *Ratings) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id] = cacheEntry{ratings: ratings, cachedAt: time.Now()}
}
