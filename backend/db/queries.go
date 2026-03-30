package db

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// movieSelectFields excludes the embedding column on list/search endpoints.
// Returning 1536 floats per movie (~24KB each) on paginated responses is wasteful;
// the embedding is only needed server-side for similarity search.
const movieSelectFields = "id,tmdb_id,title,overview,genres,release_year,poster_path,backdrop_path,vote_average,popularity,runtime"

// Movie is the public representation of a movie row, safe to return to API clients.
type Movie struct {
	ID           string   `json:"id"`
	TmdbID       int      `json:"tmdb_id"`
	Title        string   `json:"title"`
	Overview     string   `json:"overview"`
	Genres       []string `json:"genres"`
	ReleaseYear  int      `json:"release_year"`
	PosterPath   string   `json:"poster_path"`
	BackdropPath string   `json:"backdrop_path"`
	VoteAverage  float64  `json:"vote_average"`
	Popularity   float64  `json:"popularity"`
	Runtime      int      `json:"runtime"`
}

// MovieCandidate extends Movie with a cosine similarity score from the match_movies RPC.
// Similarity is in [0, 1] where 1.0 is identical to the query embedding.
type MovieCandidate struct {
	Movie
	Similarity float64 `json:"similarity"`
}

// InteractionInsert carries the fields written when a user signals preference.
type InteractionInsert struct {
	UserID  string `json:"user_id"`
	MovieID string `json:"movie_id"`
	Type    string `json:"type"`
}

// ListMovies returns movies ordered by popularity descending.
func (c *SupabaseClient) ListMovies(ctx context.Context, limit, offset int) ([]Movie, error) {
	params := url.Values{}
	params.Set("select", movieSelectFields)
	params.Set("order", "popularity.desc")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(offset))

	var movies []Movie
	if err := c.doGet(ctx, "movies", params, &movies); err != nil {
		return nil, fmt.Errorf("listing movies: %w", err)
	}
	return movies, nil
}

// GetMovieByID fetches a single movie by its UUID. Returns nil if the movie does not exist.
func (c *SupabaseClient) GetMovieByID(ctx context.Context, id string) (*Movie, error) {
	params := url.Values{}
	params.Set("select", movieSelectFields)
	params.Set("id", "eq."+id)

	var movies []Movie
	if err := c.doGet(ctx, "movies", params, &movies); err != nil {
		return nil, fmt.Errorf("fetching movie %s: %w", id, err)
	}
	if len(movies) == 0 {
		return nil, nil
	}
	return &movies[0], nil
}

// SearchMoviesByTitle returns movies whose title matches query via Postgres ILIKE.
// The trigram GIN index on movies.title makes partial matches fast without full-text overhead.
func (c *SupabaseClient) SearchMoviesByTitle(ctx context.Context, query string, limit int) ([]Movie, error) {
	params := url.Values{}
	params.Set("select", movieSelectFields)
	params.Set("title", "ilike.*"+query+"*")
	params.Set("order", "popularity.desc")
	params.Set("limit", strconv.Itoa(limit))

	var movies []Movie
	if err := c.doGet(ctx, "movies", params, &movies); err != nil {
		return nil, fmt.Errorf("searching movies for %q: %w", query, err)
	}
	return movies, nil
}

// InsertInteraction records one user signal (like/dislike/watch/skip).
func (c *SupabaseClient) InsertInteraction(ctx context.Context, interaction InteractionInsert) error {
	if err := c.doPost(ctx, "/rest/v1/interactions", interaction, nil); err != nil {
		return fmt.Errorf("inserting interaction: %w", err)
	}
	return nil
}

// GetUserEmbedding returns the stored preference vector for a user.
// Returns nil (no error) if the user has no embedding yet — callers should fall back
// to popularity-based recommendations in that case.
func (c *SupabaseClient) GetUserEmbedding(ctx context.Context, userID string) ([]float32, error) {
	params := url.Values{}
	params.Set("select", "embedding")
	params.Set("user_id", "eq."+userID)

	// pgvector returns vectors as the string "[0.1,-0.2,...]" over the REST API.
	var rows []struct {
		Embedding string `json:"embedding"`
	}
	if err := c.doGet(ctx, "user_embeddings", params, &rows); err != nil {
		return nil, fmt.Errorf("fetching embedding for user %s: %w", userID, err)
	}
	if len(rows) == 0 || rows[0].Embedding == "" {
		return nil, nil
	}
	return parseVectorString(rows[0].Embedding)
}

// MatchMovies calls the match_movies Postgres RPC for Stage-1 candidate retrieval.
// It returns the top-N movies by cosine similarity to queryEmbedding.
func (c *SupabaseClient) MatchMovies(ctx context.Context, queryEmbedding []float32, limit int) ([]MovieCandidate, error) {
	payload := struct {
		QueryEmbedding []float32 `json:"query_embedding"`
		MatchCount     int       `json:"match_count"`
	}{
		QueryEmbedding: queryEmbedding,
		MatchCount:     limit,
	}

	var candidates []MovieCandidate
	if err := c.CallRPC(ctx, "match_movies", payload, &candidates); err != nil {
		return nil, fmt.Errorf("match_movies rpc: %w", err)
	}
	return candidates, nil
}

// CountUserInteractions returns the total number of interactions for a user.
func (c *SupabaseClient) CountUserInteractions(ctx context.Context, userID string) (int, error) {
	params := url.Values{}
	params.Set("select", "id")
	params.Set("user_id", "eq."+userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.baseURL+"/rest/v1/interactions?"+params.Encode(), nil)
	if err != nil {
		return 0, fmt.Errorf("building count request: %w", err)
	}
	c.injectAuthHeaders(req)
	req.Header.Set("Prefer", "count=exact")
	req.Header.Set("Range-Unit", "items")
	req.Header.Set("Range", "0-0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("counting user interactions: %w", err)
	}
	defer resp.Body.Close()

	return parseContentRangeCount(resp.Header.Get("Content-Range")), nil
}

// CountUserMovieInteractions returns the number of interactions a user has for a specific movie.
func (c *SupabaseClient) CountUserMovieInteractions(ctx context.Context, userID, movieID string) (int, error) {
	params := url.Values{}
	params.Set("select", "id")
	params.Set("user_id", "eq."+userID)
	params.Set("movie_id", "eq."+movieID)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.baseURL+"/rest/v1/interactions?"+params.Encode(), nil)
	if err != nil {
		return 0, fmt.Errorf("building count request: %w", err)
	}
	c.injectAuthHeaders(req)
	req.Header.Set("Prefer", "count=exact")
	req.Header.Set("Range-Unit", "items")
	req.Header.Set("Range", "0-0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("counting user-movie interactions: %w", err)
	}
	defer resp.Body.Close()

	return parseContentRangeCount(resp.Header.Get("Content-Range")), nil
}

// TableStats holds row counts for monitoring the database size on the free tier.
type TableStats struct {
	MovieCount      int `json:"movie_count"`
	UserCount       int `json:"user_count"`
	InteractionCount int `json:"interaction_count"`
}

// GetTableStats returns row counts for core tables.
// Used by the /health endpoint for database size monitoring.
func (c *SupabaseClient) GetTableStats(ctx context.Context) (TableStats, error) {
	var stats TableStats

	tables := []struct {
		name  string
		dest  *int
	}{
		{"movies", &stats.MovieCount},
		{"users", &stats.UserCount},
		{"interactions", &stats.InteractionCount},
	}

	for _, t := range tables {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.baseURL+"/rest/v1/"+t.name+"?select=id", nil)
		if err != nil {
			return stats, fmt.Errorf("building count request for %s: %w", t.name, err)
		}
		c.injectAuthHeaders(req)
		req.Header.Set("Prefer", "count=exact")
		req.Header.Set("Range-Unit", "items")
		req.Header.Set("Range", "0-0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return stats, fmt.Errorf("counting %s: %w", t.name, err)
		}
		resp.Body.Close()
		*t.dest = parseContentRangeCount(resp.Header.Get("Content-Range"))
	}

	return stats, nil
}

// parseContentRangeCount extracts the total from a Supabase Content-Range header.
// Format: "0-0/42" where 42 is the total count. Returns 0 if unparseable.
func parseContentRangeCount(header string) int {
	// Content-Range: 0-0/42 or */42 for empty results
	parts := strings.Split(header, "/")
	if len(parts) != 2 {
		return 0
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return n
}

// parseVectorString converts the PostgreSQL vector string "[0.1,-0.2,0.3,...]" to []float32.
// pgvector uses this text format in all PostgREST responses.
func parseVectorString(s string) ([]float32, error) {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	vec := make([]float32, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("parsing vector element %d %q: %w", i, p, err)
		}
		vec[i] = float32(f)
	}
	return vec, nil
}
