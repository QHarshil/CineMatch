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

// AssistantUsage totals assistant activity since a cutoff.
type AssistantUsage struct {
	UserRuns    int `json:"user_runs"`
	IPRuns      int `json:"ip_runs"`
	TotalRuns   int `json:"total_runs"`
	TotalTokens int `json:"total_tokens"`
}

// RunReservation starts an audited run if the caller is under its limits.
type RunReservation struct {
	RunID         string
	UserID        string
	IPHash        string
	Since         time.Time
	UserLimit     int
	IPLimit       int
	RequestID     string
	PromptVersion string
	Model         string
	InputSHA256   string
	InputChars    int
	// TokenHold is counted against the global token cap until the run
	// finishes, so runs in flight and runs whose final write failed count.
	TokenHold int
}

// ReservationResult says whether the run may start and the usage it saw.
// Reason is "user" or "network" when a limit was reached.
type ReservationResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
	AssistantUsage
}

// ReserveAssistantRun counts the run and inserts its "running" audit row in
// one locked transaction (reserve_assistant_run), so parallel requests cannot
// all pass the limit check.
func (c *SupabaseClient) ReserveAssistantRun(ctx context.Context, r RunReservation) (ReservationResult, error) {
	payload := map[string]any{
		"p_run_id":         r.RunID,
		"p_user_id":        r.UserID,
		"p_ip_hash":        r.IPHash,
		"p_since":          r.Since.UTC().Format(time.RFC3339),
		"p_user_limit":     r.UserLimit,
		"p_ip_limit":       r.IPLimit,
		"p_request_id":     r.RequestID,
		"p_prompt_version": r.PromptVersion,
		"p_model":          r.Model,
		"p_input_sha256":   r.InputSHA256,
		"p_input_chars":    r.InputChars,
		"p_token_hold":     r.TokenHold,
	}
	var rows []ReservationResult
	if err := c.CallRPC(ctx, "reserve_assistant_run", payload, &rows); err != nil {
		return ReservationResult{}, fmt.Errorf("reserve_assistant_run rpc: %w", err)
	}
	if len(rows) == 0 {
		return ReservationResult{}, fmt.Errorf("reserve_assistant_run returned no row")
	}
	return rows[0], nil
}

// AssistantRunResult is the outcome written to a reserved run.
type AssistantRunResult struct {
	Status            string          `json:"status"`
	Model             string          `json:"model"`
	Steps             json.RawMessage `json:"steps"`
	PickIDs           []string        `json:"pick_ids"`
	UngroundedDropped int             `json:"ungrounded_dropped"`
	OutputBlocked     bool            `json:"output_blocked"`
	InputTokens       int             `json:"input_tokens"`
	OutputTokens      int             `json:"output_tokens"`
	LatencyMS         int             `json:"latency_ms"`
}

// FinishAssistantRun records the outcome on a reserved run.
func (c *SupabaseClient) FinishAssistantRun(ctx context.Context, runID string, result AssistantRunResult) error {
	if result.PickIDs == nil {
		result.PickIDs = []string{}
	}
	if len(result.Steps) == 0 {
		result.Steps = json.RawMessage("[]")
	}
	params := url.Values{}
	params.Set("id", "eq."+runID)
	if err := c.doPatch(ctx, "/rest/v1/assistant_runs", params, result); err != nil {
		return fmt.Errorf("finishing assistant run %s: %w", runID, err)
	}
	return nil
}

// AssistantUsageSince returns run counts for the user and for the network
// (by IP hash), plus global run and token totals since the cutoff, via the
// assistant_usage RPC.
func (c *SupabaseClient) AssistantUsageSince(ctx context.Context, userID, ipHash string, since time.Time) (AssistantUsage, error) {
	payload := map[string]string{
		"p_user_id": userID,
		"p_ip_hash": ipHash,
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
