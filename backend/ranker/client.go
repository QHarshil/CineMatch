// Package ranker provides an HTTP client for the Python ranking microservice.
//
// The Go backend calls Rank() after Stage-1 retrieval (match_movies RPC)
// to re-score candidates using the feature-weighted ranker. If the ranker
// is unreachable or errors, the caller should fall back to similarity order.
package ranker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/harshilc/cinematch-backend/db"
)

// Client talks to the Python FastAPI ranker over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a ranker client pointing at the given base URL (e.g. "http://localhost:8000").
// A 5-second timeout prevents a slow ranker from blocking the entire recommend request.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ModelVersion is the ranker model the backend requests in production.
const ModelVersion = "lambdamart-v1"

// rankRequest mirrors the Python ranker's POST /rank schema.
type rankRequest struct {
	Candidates   []candidateMovie `json:"candidates"`
	UserFeatures userFeatures     `json:"user_features"`
	TopN         int              `json:"top_n"`
	Model        string           `json:"model"`
}

type candidateMovie struct {
	MovieID     string   `json:"movie_id"`
	Title       string   `json:"title"`
	Genres      []string `json:"genres"`
	ReleaseYear int      `json:"release_year"`
	VoteAverage float64  `json:"vote_average"`
	Popularity  float64  `json:"popularity"`
	Runtime     int      `json:"runtime"`
	Similarity  float64  `json:"similarity"`
}

type userFeatures struct {
	PreferredGenres      []string `json:"preferred_genres"`
	MinVotePref          float64  `json:"min_vote_preference"`
	UserLikeRatio        float64  `json:"user_like_ratio"`
	UserInteractionCount int      `json:"user_interaction_count"`
}

// UserContext carries the per-user signals the ranker uses to personalize scoring.
type UserContext struct {
	PreferredGenres  []string
	MinVotePref      float64
	LikeRatio        float64
	InteractionCount int
}

// RankResponse is the parsed response from the Python ranker.
type RankResponse struct {
	Ranked       []RankedMovie `json:"ranked"`
	ModelVersion string        `json:"model_version"`
}

// RankedMovie is a single entry in the ranker's response.
type RankedMovie struct {
	MovieID string  `json:"movie_id"`
	Score   float64 `json:"score"`
	Rank    int     `json:"rank"`
}

// Rank sends Stage-1 candidates to the Python ranker and returns re-scored
// results, requesting the production model. user carries the per-user signals
// derived from interaction history; pass a zero UserContext for new users.
func (c *Client) Rank(
	ctx context.Context,
	candidates []db.MovieCandidate,
	topN int,
	user UserContext,
) (*RankResponse, error) {
	genres := user.PreferredGenres
	if genres == nil {
		genres = []string{} // serialize as [] not null so Pydantic accepts it
	}
	body := rankRequest{
		Candidates: mapCandidates(candidates),
		UserFeatures: userFeatures{
			PreferredGenres:      genres,
			MinVotePref:          user.MinVotePref,
			UserLikeRatio:        user.LikeRatio,
			UserInteractionCount: user.InteractionCount,
		},
		TopN:  topN,
		Model: ModelVersion,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling rank request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rank", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building rank request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ranker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ranker returned HTTP %d", resp.StatusCode)
	}

	var result RankResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding ranker response: %w", err)
	}
	return &result, nil
}

func mapCandidates(candidates []db.MovieCandidate) []candidateMovie {
	out := make([]candidateMovie, len(candidates))
	for i, c := range candidates {
		out[i] = candidateMovie{
			MovieID:     c.ID,
			Title:       c.Title,
			Genres:      c.Genres,
			ReleaseYear: c.ReleaseYear,
			VoteAverage: c.VoteAverage,
			Popularity:  c.Popularity,
			Runtime:     c.Runtime,
			Similarity:  c.Similarity,
		}
	}
	return out
}
