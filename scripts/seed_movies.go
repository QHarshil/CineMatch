// seed_movies populates the Supabase movies catalog with TMDB data and OpenAI
// embeddings. It seeds movies and/or TV shows, restricted to a set of original
// languages, in popularity order (initial seed) or by most recent release (the
// freshness cron).
//
// Usage:
//
//	go run seed_movies.go [--dry-run] [--media movie|tv|both] [--mode popular|recent] [--count N] [--languages en,ko]
//
// Examples:
//
//	go run seed_movies.go --media both --count 700                       # initial catalog (en, ko)
//	go run seed_movies.go --media both --mode recent --count 100         # monthly refresh
//	go run seed_movies.go --media both --languages en                    # English only
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
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

const (
	tmdbBaseURL    = "https://api.themoviedb.org/3"
	openAIBaseURL  = "https://api.openai.com/v1"
	embeddingModel = "text-embedding-3-small"

	tmdbPageSize = 20

	// 5 concurrent workers; actual request rate is throttled to openAIRPM below.
	embedWorkers = 5
	// Stay at 80 RPM — safely under Tier-1's 100 RPM hard limit, with headroom
	// for occasional retries and other API activity on the same key.
	openAIRPM       = 80
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

// tmdbItem holds the fields shared by TMDB movie and TV results. Movies use
// title/release_date; TV shows use name/first_air_date. mediaType is set by the
// seeder, not decoded from the API.
type tmdbItem struct {
	mediaType        string
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	Name             string  `json:"name"`
	Overview         string  `json:"overview"`
	GenreIDs         []int   `json:"genre_ids"`
	ReleaseDate      string  `json:"release_date"`
	FirstAirDate     string  `json:"first_air_date"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	Popularity       float64 `json:"popularity"`
	OriginalLanguage string  `json:"original_language"`
}

// displayTitle returns the movie title or TV show name, whichever is present.
func (t tmdbItem) displayTitle() string {
	if t.Title != "" {
		return t.Title
	}
	return t.Name
}

// dateString returns the release date (movie) or first air date (TV).
func (t tmdbItem) dateString() string {
	if t.ReleaseDate != "" {
		return t.ReleaseDate
	}
	return t.FirstAirDate
}

// — Supabase row type ————————————————————————————————————————————————————————

// movieRow matches the Supabase movies table schema.
// Embedding is []float64 because json.Unmarshal decodes JSON numbers as float64;
// pgvector accepts a JSON array for vector columns over the PostgREST API.
type movieRow struct {
	TmdbID           int       `json:"tmdb_id"`
	MediaType        string    `json:"media_type"`
	Title            string    `json:"title"`
	Overview         string    `json:"overview"`
	Genres           []string  `json:"genres"`
	ReleaseYear      int       `json:"release_year"`
	PosterPath       string    `json:"poster_path"`
	BackdropPath     string    `json:"backdrop_path"`
	VoteAverage      float64   `json:"vote_average"`
	Popularity       float64   `json:"popularity"`
	OriginalLanguage string    `json:"original_language"`
	Embedding        []float64 `json:"embedding"`
}

type embedResult struct {
	row movieRow
	err error
}

// — main —————————————————————————————————————————————————————————————————————

func main() {
	dryRun := flag.Bool("dry-run", false, "fetch and embed without writing to the database")
	media := flag.String("media", "movie", "which catalog to seed: movie | tv | both")
	mode := flag.String("mode", "popular", "ordering: popular (most popular) | recent (newest releases)")
	count := flag.Int("count", 500, "number of titles to fetch per media type (split across languages)")
	languages := flag.String("languages", "en,ko", "comma-separated TMDB original-language codes to include")
	flag.Parse()

	mediaTypes, err := parseMediaTypes(*media)
	if err != nil {
		slog.Error("invalid --media", "error", err)
		os.Exit(1)
	}
	if *mode != "popular" && *mode != "recent" {
		slog.Error("invalid --mode, want popular or recent", "got", *mode)
		os.Exit(1)
	}
	langs := parseLanguages(*languages)

	// The seeder is run from scripts/ so secrets are one level up.
	if err := godotenv.Load("../.env"); err != nil {
		slog.Info("no ../.env found, reading environment variables directly")
	}

	cfg := struct {
		tmdbToken, openAIKey, supabaseURL, supabaseKey string
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

	slog.Info("fetching TMDB genre lists")
	genreMap, err := fetchGenreMap(client, cfg.tmdbToken, mediaTypes)
	if err != nil {
		slog.Error("genre fetch failed", "error", err)
		os.Exit(1)
	}
	slog.Info("genre map loaded", "genres", len(genreMap))

	var items []tmdbItem
	for _, mt := range mediaTypes {
		slog.Info("fetching from TMDB", "media", mt, "mode", *mode, "languages", langs, "target", *count)
		fetched, err := fetchItems(client, cfg.tmdbToken, mt, *mode, *count, langs)
		if err != nil {
			slog.Error("fetch failed", "media", mt, "error", err)
			os.Exit(1)
		}
		items = append(items, fetched...)
	}
	items = deduplicate(items)
	slog.Info("items after deduplication", "count", len(items))

	embeddingLimiter := rate.NewLimiter(rate.Limit(openAIRPM)/60, 1)
	slog.Info("generating embeddings", "workers", embedWorkers, "rpm_limit", openAIRPM, "items", len(items))
	rows, embedErrors := generateEmbeddings(client, cfg.openAIKey, items, genreMap, embeddingLimiter)
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

// fetchGenreMap returns a merged map of TMDB genre ID -> name across the
// requested media types. Movie and TV genre lists overlap but each has a few
// unique entries; merging keeps a single lookup table.
func fetchGenreMap(client *http.Client, token string, mediaTypes []string) (map[int]string, error) {
	genreMap := make(map[int]string)
	for _, mt := range mediaTypes {
		resp, err := tmdbGET(client, token, "/genre/"+mt+"/list", map[string]string{"language": "en"})
		if err != nil {
			return nil, fmt.Errorf("fetching %s genre list: %w", mt, err)
		}
		var body struct {
			Genres []tmdbGenre `json:"genres"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decoding %s genre list: %w", mt, err)
		}
		for _, g := range body.Genres {
			genreMap[g.ID] = g.Name
		}
	}
	return genreMap, nil
}

// fetchItems pages through TMDB discover for one media type across the given
// original languages, returning up to count items (split evenly across
// languages) with mediaType stamped on each.
func fetchItems(client *http.Client, token, mediaType, mode string, count int, languages []string) ([]tmdbItem, error) {
	perLang := count / len(languages)
	if perLang < 1 {
		perLang = count
	}
	items := make([]tmdbItem, 0, count)
	firstRequest := true

	for _, lang := range languages {
		pages := (perLang + tmdbPageSize - 1) / tmdbPageSize
		collected := 0
		for page := 1; page <= pages && collected < perLang; page++ {
			if !firstRequest {
				time.Sleep(tmdbRequestDelay)
			}
			firstRequest = false

			params := discoverParams(mediaType, mode, page)
			params["with_original_language"] = lang

			resp, err := tmdbGET(client, token, "/discover/"+mediaType, params)
			if err != nil {
				return nil, fmt.Errorf("discover %s [%s] page %d: %w", mediaType, lang, page, err)
			}
			var body struct {
				Results []tmdbItem `json:"results"`
			}
			jsonErr := json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			if jsonErr != nil {
				return nil, fmt.Errorf("decoding %s [%s] page %d: %w", mediaType, lang, page, jsonErr)
			}
			if len(body.Results) == 0 {
				break // no more results for this language
			}

			for i := range body.Results {
				if collected >= perLang {
					break
				}
				body.Results[i].mediaType = mediaType
				items = append(items, body.Results[i])
				collected++
			}
			slog.Info("tmdb page fetched", "media", mediaType, "lang", lang, "page", page, "running_total", len(items))
		}
	}
	return items, nil
}

// discoverParams builds the /discover query for a media type and ordering mode.
// "recent" sorts by release date and loosens the vote-count floor so brand-new
// titles (which have few votes) still appear; "popular" sorts by popularity.
func discoverParams(mediaType, mode string, page int) map[string]string {
	params := map[string]string{
		"include_adult": "false",
		"language":      "en-US",
		"page":          strconv.Itoa(page),
	}
	today := time.Now().UTC().Format("2006-01-02")

	dateField := "primary_release_date"
	if mediaType == "tv" {
		dateField = "first_air_date"
	} else {
		params["include_video"] = "false"
	}
	params[dateField+".lte"] = today // never seed unreleased titles

	if mode == "recent" {
		params["sort_by"] = dateField + ".desc"
		params["vote_count.gte"] = "10"
	} else {
		params["sort_by"] = "popularity.desc"
		params["vote_count.gte"] = "50" // exclude obscure titles with too few votes to trust ratings
	}
	return params
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
// Returns completed rows and the count of items that failed embedding.
func generateEmbeddings(client *http.Client, apiKey string, items []tmdbItem, genreMap map[int]string, limiter *rate.Limiter) ([]movieRow, int) {
	results := make(chan embedResult, len(items))
	sem := make(chan struct{}, embedWorkers)
	var wg sync.WaitGroup

	for _, it := range items {
		wg.Add(1)
		go func(item tmdbItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Wait for a rate-limit token before calling OpenAI to stay under 80 RPM.
			if err := limiter.Wait(context.Background()); err != nil {
				results <- embedResult{err: fmt.Errorf("rate limiter cancelled for %s %d: %w", item.mediaType, item.ID, err)}
				return
			}

			embedding, err := callOpenAIEmbedding(client, apiKey, buildEmbeddingText(item.displayTitle(), item.Overview))
			if err != nil {
				results <- embedResult{err: fmt.Errorf("%s %d %q: %w", item.mediaType, item.ID, item.displayTitle(), err)}
				return
			}

			results <- embedResult{row: movieRow{
				TmdbID:           item.ID,
				MediaType:        item.mediaType,
				Title:            item.displayTitle(),
				Overview:         item.Overview,
				Genres:           genreNamesFromIDs(item.GenreIDs, genreMap),
				ReleaseYear:      extractReleaseYear(item.dateString()),
				PosterPath:       item.PosterPath,
				BackdropPath:     item.BackdropPath,
				VoteAverage:      item.VoteAverage,
				Popularity:       item.Popularity,
				OriginalLanguage: item.OriginalLanguage,
				Embedding:        embedding,
			}}
		}(it)
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
	body, err := json.Marshal(map[string]string{"model": embeddingModel, "input": text})
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

// upsertMovies sends rows to Supabase in batches, using (tmdb_id, media_type) as
// the conflict target so re-running the seeder updates existing rows rather than
// duplicating them. Requires the composite unique index from the media_type migration.
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

		req, err := http.NewRequest(
			http.MethodPost,
			supabaseURL+"/rest/v1/movies?on_conflict=tmdb_id,media_type",
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

// parseMediaTypes expands the --media flag into the list of TMDB media types to seed.
func parseMediaTypes(media string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(media)) {
	case "movie":
		return []string{"movie"}, nil
	case "tv":
		return []string{"tv"}, nil
	case "both", "all":
		return []string{"movie", "tv"}, nil
	default:
		return nil, fmt.Errorf("want movie, tv, or both; got %q", media)
	}
}

// parseLanguages splits the --languages flag into TMDB original-language codes,
// defaulting to English if the flag is empty.
func parseLanguages(languages string) []string {
	var out []string
	for _, part := range strings.Split(languages, ",") {
		if code := strings.ToLower(strings.TrimSpace(part)); code != "" {
			out = append(out, code)
		}
	}
	if len(out) == 0 {
		return []string{"en"}
	}
	return out
}

// deduplicate removes items sharing the same (media_type, tmdb_id). TMDB's
// pagination occasionally repeats a title across pages when its popularity
// shifts between requests, and movie/TV IDs are separate namespaces.
func deduplicate(items []tmdbItem) []tmdbItem {
	seen := make(map[string]bool, len(items))
	unique := make([]tmdbItem, 0, len(items))
	for _, it := range items {
		key := it.mediaType + ":" + strconv.Itoa(it.ID)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, it)
		}
	}
	return unique
}

// buildEmbeddingText constructs the string that gets embedded.
// Combining title and overview gives the model enough semantic signal to distinguish
// similarly-named titles and capture genre/tone from the description.
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

// extractReleaseYear parses the 4-digit year from a TMDB date string ("YYYY-MM-DD").
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
