// seed_movies populates the Supabase movies table with TMDB data and OpenAI embeddings.
//
// Usage:
//
//	go run seed_movies.go [--dry-run]
//
// Required env vars: TMDB_READ_ACCESS_TOKEN, OPENAI_API_KEY, SUPABASE_URL, SUPABASE_SECRET_KEY
// Reads ../.env relative to the scripts/ directory, then falls back to process environment.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

const (
	tmdbBaseURL    = "https://api.themoviedb.org/3"
	openAIBaseURL  = "https://api.openai.com/v1"
	embeddingModel = "text-embedding-3-small"

	targetMovieCount = 500
	tmdbPageSize     = 20
	tmdbPages        = targetMovieCount / tmdbPageSize // 25

	// 5 concurrent workers; actual request rate is throttled to openAIRPM below.
	embedWorkers = 5
	// Stay at 80 RPM — safely under Tier-1's 100 RPM hard limit, with headroom
	// for occasional retries and other API activity on the same key.
	openAIRPM = 80
	upsertBatchSize = 50

	// 260ms between TMDB page requests keeps us under the 40 req/10s limit
	// while accounting for small network latency variance.
	tmdbRequestDelay = 260 * time.Millisecond
)

// — TMDB types ——————————————————————————————————————————————————————————————

type tmdbGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type tmdbMovie struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Overview     string  `json:"overview"`
	GenreIDs     []int   `json:"genre_ids"`
	ReleaseDate  string  `json:"release_date"` // "YYYY-MM-DD" or ""
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	Popularity   float64 `json:"popularity"`
}

// — Supabase row type ————————————————————————————————————————————————————————

// movieRow matches the Supabase movies table schema.
// Embedding is []float64 because json.Unmarshal decodes JSON numbers as float64;
// pgvector accepts a JSON array for vector columns over the PostgREST API.
type movieRow struct {
	TmdbID       int       `json:"tmdb_id"`
	Title        string    `json:"title"`
	Overview     string    `json:"overview"`
	Genres       []string  `json:"genres"`
	ReleaseYear  int       `json:"release_year"`
	PosterPath   string    `json:"poster_path"`
	BackdropPath string    `json:"backdrop_path"`
	VoteAverage  float64   `json:"vote_average"`
	Popularity   float64   `json:"popularity"`
	Embedding    []float64 `json:"embedding"`
}

// — Worker pipeline ——————————————————————————————————————————————————————————

type embedResult struct {
	row movieRow
	err error
}

// — main —————————————————————————————————————————————————————————————————————

func main() {
	dryRun := flag.Bool("dry-run", false, "fetch and embed without writing to the database")
	flag.Parse()

	// The seeder is run from scripts/ so secrets are one level up.
	if err := godotenv.Load("../.env"); err != nil {
		slog.Info("no ../.env found, reading environment variables directly")
	}

	cfg := struct {
		tmdbToken   string
		openAIKey   string
		supabaseURL string
		supabaseKey string
	}{
		tmdbToken:   os.Getenv("TMDB_READ_ACCESS_TOKEN"),
		openAIKey:   os.Getenv("OPENAI_API_KEY"),
		supabaseURL: os.Getenv("SUPABASE_URL"),
		supabaseKey: os.Getenv("SUPABASE_SECRET_KEY"),
	}

	if cfg.tmdbToken == "" || cfg.openAIKey == "" || cfg.supabaseURL == "" || cfg.supabaseKey == "" {
		slog.Error("missing required env vars",
			"required", "TMDB_READ_ACCESS_TOKEN, OPENAI_API_KEY, SUPABASE_URL, SUPABASE_SECRET_KEY")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	slog.Info("fetching TMDB genre list")
	genreMap, err := fetchGenreMap(client, cfg.tmdbToken)
	if err != nil {
		slog.Error("genre fetch failed", "error", err)
		os.Exit(1)
	}
	slog.Info("genre map loaded", "genres", len(genreMap))

	slog.Info("fetching movies from TMDB discover", "pages", tmdbPages, "target", targetMovieCount)
	tmdbMovies, err := fetchAllMovies(client, cfg.tmdbToken)
	if err != nil {
		slog.Error("movie fetch failed", "error", err)
		os.Exit(1)
	}
	tmdbMovies = deduplicateByTmdbID(tmdbMovies)
	slog.Info("movies after deduplication", "count", len(tmdbMovies))

	// Rate-limit embedding calls to openAIRPM to avoid Tier-1 429s.
	embeddingLimiter := rate.NewLimiter(rate.Limit(openAIRPM)/60, 1)

	slog.Info("generating embeddings", "workers", embedWorkers, "rpm_limit", openAIRPM, "movies", len(tmdbMovies))
	rows, embedErrors := generateEmbeddings(client, cfg.openAIKey, tmdbMovies, genreMap, embeddingLimiter)
	if embedErrors > 0 {
		slog.Warn("some embeddings failed", "failed", embedErrors, "succeeded", len(rows))
	}
	slog.Info("embeddings complete", "count", len(rows))

	if *dryRun {
		slog.Info("dry-run mode: skipping database writes", "would_upsert", len(rows))
		return
	}

	slog.Info("upserting to Supabase", "total", len(rows), "batch_size", upsertBatchSize)
	upserted, err := upsertMovies(client, cfg.supabaseURL, cfg.supabaseKey, rows)
	if err != nil {
		slog.Error("upsert failed", "error", err, "upserted_before_failure", upserted)
		os.Exit(1)
	}
	slog.Info("seed complete", "upserted", upserted)
}

// — TMDB helpers —————————————————————————————————————————————————————————————

// fetchGenreMap returns a map of TMDB genre ID -> genre name.
func fetchGenreMap(client *http.Client, tmdbToken string) (map[int]string, error) {
	resp, err := tmdbGET(client, tmdbToken, "/genre/movie/list", map[string]string{"language": "en"})
	if err != nil {
		return nil, fmt.Errorf("fetching genre list: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Genres []tmdbGenre `json:"genres"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding genre list: %w", err)
	}

	genreMap := make(map[int]string, len(body.Genres))
	for _, g := range body.Genres {
		genreMap[g.ID] = g.Name
	}
	return genreMap, nil
}

// fetchAllMovies pages through TMDB discover sorted by popularity, returning up to targetMovieCount movies.
// A 260ms delay between pages keeps request rate under the 40/10s TMDB limit.
func fetchAllMovies(client *http.Client, tmdbToken string) ([]tmdbMovie, error) {
	movies := make([]tmdbMovie, 0, targetMovieCount)

	for page := 1; page <= tmdbPages; page++ {
		if page > 1 {
			time.Sleep(tmdbRequestDelay)
		}

		resp, err := tmdbGET(client, tmdbToken, "/discover/movie", map[string]string{
			"sort_by":        "popularity.desc",
			"include_adult":  "false",
			"include_video":  "false",
			"language":       "en-US",
			"page":           strconv.Itoa(page),
			"vote_count.gte": "50", // exclude obscure titles with too few votes to trust ratings
		})
		if err != nil {
			return nil, fmt.Errorf("discover page %d: %w", page, err)
		}

		var body struct {
			Results []tmdbMovie `json:"results"`
		}
		if jsonErr := json.NewDecoder(resp.Body).Decode(&body); jsonErr != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding discover page %d: %w", page, jsonErr)
		}
		resp.Body.Close()

		movies = append(movies, body.Results...)
		slog.Info("tmdb page fetched", "page", page, "running_total", len(movies))
	}

	return movies, nil
}

// tmdbGET performs an authenticated GET request to the TMDB API.
func tmdbGET(client *http.Client, token, path string, params map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, tmdbBaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("building TMDB request for %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TMDB GET %s: %w", path, err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("TMDB %s returned %d: %s", path, resp.StatusCode, string(body))
	}
	return resp, nil
}

// — OpenAI helpers ———————————————————————————————————————————————————————————

// generateEmbeddings fans out embedding generation across embedWorkers goroutines.
// The shared limiter enforces openAIRPM so we never exceed Tier-1 rate limits.
// Returns completed rows and the count of movies that failed embedding (logged as warnings).
func generateEmbeddings(client *http.Client, apiKey string, movies []tmdbMovie, genreMap map[int]string, limiter *rate.Limiter) ([]movieRow, int) {
	results := make(chan embedResult, len(movies))
	sem := make(chan struct{}, embedWorkers)
	var wg sync.WaitGroup

	for _, m := range movies {
		wg.Add(1)
		go func(movie tmdbMovie) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Wait for a rate-limit token before calling OpenAI.
			// This serialises bursts across all workers to stay under 80 RPM.
			if err := limiter.Wait(context.Background()); err != nil {
				results <- embedResult{err: fmt.Errorf("rate limiter cancelled for movie %d: %w", movie.ID, err)}
				return
			}

			text := buildEmbeddingText(movie.Title, movie.Overview)
			embedding, err := callOpenAIEmbedding(client, apiKey, text)
			if err != nil {
				results <- embedResult{err: fmt.Errorf("movie %d %q: %w", movie.ID, movie.Title, err)}
				return
			}

			results <- embedResult{row: movieRow{
				TmdbID:      movie.ID,
				Title:       movie.Title,
				Overview:    movie.Overview,
				Genres:      genreNamesFromIDs(movie.GenreIDs, genreMap),
				ReleaseYear: extractReleaseYear(movie.ReleaseDate),
				PosterPath:   movie.PosterPath,
				BackdropPath: movie.BackdropPath,
				VoteAverage:  movie.VoteAverage,
				Popularity:  movie.Popularity,
				Embedding:   embedding,
			}}
		}(m)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var rows []movieRow
	var errCount int
	for r := range results {
		if r.err != nil {
			slog.Warn("embedding failed", "error", r.err)
			errCount++
			continue
		}
		rows = append(rows, r.row)
	}
	return rows, errCount
}

// callOpenAIEmbedding sends one embedding request to the OpenAI API.
func callOpenAIEmbedding(client *http.Client, apiKey, text string) ([]float64, error) {
	body, err := json.Marshal(map[string]string{
		"model": embeddingModel,
		"input": text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling embedding request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, openAIBaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding embedding response: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding in response")
	}
	return result.Data[0].Embedding, nil
}

// — Supabase upsert ——————————————————————————————————————————————————————————

// upsertMovies sends rows to Supabase in batches, using tmdb_id as the conflict target
// so re-running the seeder updates existing rows rather than duplicating them.
func upsertMovies(client *http.Client, supabaseURL, serviceKey string, rows []movieRow) (int, error) {
	total := 0
	for i := 0; i < len(rows); i += upsertBatchSize {
		end := i + upsertBatchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[i:end]

		body, err := json.Marshal(batch)
		if err != nil {
			return total, fmt.Errorf("marshalling batch starting at index %d: %w", i, err)
		}

		// on_conflict=tmdb_id + resolution=merge-duplicates performs an upsert.
		req, err := http.NewRequest(
			http.MethodPost,
			supabaseURL+"/rest/v1/movies?on_conflict=tmdb_id",
			bytes.NewReader(body),
		)
		if err != nil {
			return total, fmt.Errorf("building upsert request: %w", err)
		}
		req.Header.Set("apikey", serviceKey)
		req.Header.Set("Authorization", "Bearer "+serviceKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Prefer", "resolution=merge-duplicates,return=minimal")

		resp, err := client.Do(req)
		if err != nil {
			return total, fmt.Errorf("upsert request failed: %w", err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			return total, fmt.Errorf("supabase upsert returned %d: %s", resp.StatusCode, string(respBody))
		}

		total += len(batch)
		slog.Info("batch upserted", "batch_size", len(batch), "running_total", total)
	}
	return total, nil
}

// — Pure helpers (also tested) ———————————————————————————————————————————————

// deduplicateByTmdbID removes movies with duplicate tmdb_id values, keeping the first
// occurrence. TMDB's discover pagination occasionally returns the same movie on multiple
// pages (e.g. when a movie's popularity score changes between requests).
func deduplicateByTmdbID(movies []tmdbMovie) []tmdbMovie {
	seen := make(map[int]bool, len(movies))
	unique := make([]tmdbMovie, 0, len(movies))
	for _, m := range movies {
		if !seen[m.ID] {
			seen[m.ID] = true
			unique = append(unique, m)
		}
	}
	return unique
}

// buildEmbeddingText constructs the string that gets embedded.
// Combining title and overview gives the model enough semantic signal to distinguish
// similarly-named movies and capture genre/tone from the description.
func buildEmbeddingText(title, overview string) string {
	if overview == "" {
		return title
	}
	return title + ". " + overview
}

// genreNamesFromIDs maps TMDB genre IDs to human-readable names using the pre-fetched genre map.
// IDs not present in the map are silently skipped (can happen if TMDB adds genres without notice).
func genreNamesFromIDs(ids []int, genreMap map[int]string) []string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := genreMap[id]; ok {
			names = append(names, name)
		}
	}
	return names
}

// extractReleaseYear parses the 4-digit year from a TMDB release_date string ("YYYY-MM-DD").
// Returns 0 if the string is absent or malformed.
func extractReleaseYear(releaseDate string) int {
	if len(releaseDate) < 4 {
		return 0
	}
	year, err := strconv.Atoi(releaseDate[:4])
	if err != nil {
		return 0
	}
	return year
}
