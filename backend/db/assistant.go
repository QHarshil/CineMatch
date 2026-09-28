package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AssistantRun is one audited assistant request, written to assistant_runs.
type AssistantRun struct {
	ID                string          `json:"id"`
	UserID            string          `json:"user_id"`
	RequestID         string          `json:"request_id,omitempty"`
	PromptVersion     string          `json:"prompt_version"`
	Model             string          `json:"model"`
	Status            string          `json:"status"`
	InputSHA256       string          `json:"input_sha256"`
	InputChars        int             `json:"input_chars"`
	Steps             json.RawMessage `json:"steps"`
	PickIDs           []string        `json:"pick_ids"`
	UngroundedDropped int             `json:"ungrounded_dropped"`
	InputTokens       int             `json:"input_tokens"`
	OutputTokens      int             `json:"output_tokens"`
	LatencyMS         int             `json:"latency_ms"`
}

// AssistantUsage totals assistant activity since a cutoff.
type AssistantUsage struct {
	UserRuns    int `json:"user_runs"`
	TotalRuns   int `json:"total_runs"`
	TotalTokens int `json:"total_tokens"`
}

// InsertAssistantRun appends an audit row.
func (c *SupabaseClient) InsertAssistantRun(ctx context.Context, run AssistantRun) error {
	if run.PickIDs == nil {
		run.PickIDs = []string{}
	}
	if len(run.Steps) == 0 {
		run.Steps = json.RawMessage("[]")
	}
	if err := c.doPost(ctx, "/rest/v1/assistant_runs", run, nil); err != nil {
		return fmt.Errorf("inserting assistant run: %w", err)
	}
	return nil
}

// AssistantUsageSince returns the user's run count plus global run and token
// totals since the cutoff, via the assistant_usage RPC.
func (c *SupabaseClient) AssistantUsageSince(ctx context.Context, userID string, since time.Time) (AssistantUsage, error) {
	payload := map[string]string{
		"p_user_id": userID,
		"p_since":   since.UTC().Format(time.RFC3339),
	}
	var rows []AssistantUsage
	if err := c.CallRPC(ctx, "assistant_usage", payload, &rows); err != nil {
		return AssistantUsage{}, fmt.Errorf("assistant_usage rpc: %w", err)
	}
	if len(rows) == 0 {
		return AssistantUsage{}, nil
	}
	return rows[0], nil
}

// MoviesByIDs fetches the given titles in the order the IDs were passed.
// Unknown IDs are skipped.
func (c *SupabaseClient) MoviesByIDs(ctx context.Context, ids []string) ([]Movie, error) {
	if len(ids) == 0 {
		return []Movie{}, nil
	}
	params := url.Values{}
	params.Set("select", movieSelectFields)
	params.Set("id", "in.("+strings.Join(ids, ",")+")")

	var rows []Movie
	if err := c.doGet(ctx, "movies", params, &rows); err != nil {
		return nil, fmt.Errorf("fetching movies by id: %w", err)
	}
	byID := make(map[string]Movie, len(rows))
	for _, m := range rows {
		byID[m.ID] = m
	}
	ordered := make([]Movie, 0, len(rows))
	for _, id := range ids {
		if m, ok := byID[id]; ok {
			ordered = append(ordered, m)
		}
	}
	return ordered, nil
}

// SimilarToTitle returns the titles nearest to movieID in embedding space
// that pass the filters, excluding the title itself. It reuses the hybrid
// search RPC with the title's own vector and no query text, so "like
// Parasite, but a series" is one filtered vector search. Returns nil when the
// title has no embedding.
func (c *SupabaseClient) SimilarToTitle(ctx context.Context, movieID string, limit int, f SearchFilters) ([]SearchHit, error) {
	vectors, err := c.movieEmbeddings(ctx, []string{movieID})
	if err != nil {
		return nil, err
	}
	vec, ok := vectors[movieID]
	if !ok {
		return nil, nil
	}
	f.ExcludeIDs = append(f.ExcludeIDs, movieID)
	return c.HybridSearch(ctx, "", vec, limit, f)
}

// RecentPositiveTitles returns the titles a user most recently liked or
// watched, newest first.
func (c *SupabaseClient) RecentPositiveTitles(ctx context.Context, userID string, limit int) ([]Movie, error) {
	params := url.Values{}
	params.Set("select", "movie_id")
	params.Set("user_id", "eq."+userID)
	params.Set("type", "in.(like,watch)")
	params.Set("order", "created_at.desc")
	params.Set("limit", strconv.Itoa(limit))

	var rows []struct {
		MovieID string `json:"movie_id"`
	}
	if err := c.doGet(ctx, "interactions", params, &rows); err != nil {
		return nil, fmt.Errorf("fetching recent positive interactions: %w", err)
	}
	ids := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if !seen[row.MovieID] {
			seen[row.MovieID] = true
			ids = append(ids, row.MovieID)
		}
	}
	return c.MoviesByIDs(ctx, ids)
}

// TitlesNamed returns titles whose name matches exactly, ignoring case. A
// film and a series can share a name, so it returns up to two.
func (c *SupabaseClient) TitlesNamed(ctx context.Context, title string) ([]Movie, error) {
	// ilike without wildcards is a case-insensitive equality; escape the
	// pattern characters so a title cannot act as a wildcard.
	escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(strings.TrimSpace(title))
	params := url.Values{}
	params.Set("select", movieSelectFields)
	params.Set("title", "ilike."+escaped)
	params.Set("order", "popularity.desc")
	params.Set("limit", "2")

	var movies []Movie
	if err := c.doGet(ctx, "movies", params, &movies); err != nil {
		return nil, fmt.Errorf("fetching titles named %q: %w", title, err)
	}
	return movies, nil
}
