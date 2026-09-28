package db

import (
	"context"
	"fmt"
)

// SearchFilters narrows a hybrid search. Zero values mean no filter.
type SearchFilters struct {
	MediaType  string   // "movie" or "tv"
	Genres     []string // matches titles with any of these genres
	MinYear    int
	MaxYear    int
	MinRating  float64 // TMDB vote average, 0-10
	MaxRuntime int     // minutes; titles with unknown runtime are excluded
	Language   string  // ISO 639-1 original language
	ExcludeIDs []string
}

// SearchHit is one hybrid search result plus the evidence behind its rank.
// A nil rank means that ranker did not return the title.
type SearchHit struct {
	Movie
	Similarity   *float64 `json:"similarity"`
	SemanticRank *int     `json:"semantic_rank"`
	KeywordRank  *int     `json:"keyword_rank"`
	TitleRank    *int     `json:"title_rank"`
	Score        float64  `json:"score"`
}

type hybridSearchParams struct {
	QueryText      string    `json:"query_text"`
	QueryEmbedding []float32 `json:"query_embedding,omitempty"`
	MatchCount     int       `json:"match_count"`
	MediaType      string    `json:"filter_media_type,omitempty"`
	Genres         []string  `json:"filter_genres,omitempty"`
	MinYear        int       `json:"min_year,omitempty"`
	MaxYear        int       `json:"max_year,omitempty"`
	MinRating      float64   `json:"min_rating,omitempty"`
	MaxRuntime     int       `json:"max_runtime,omitempty"`
	Language       string    `json:"filter_language,omitempty"`
	ExcludeIDs     []string  `json:"exclude_ids,omitempty"`
}

// HybridSearch calls the search_titles_hybrid RPC, which fuses pgvector
// similarity, full-text rank, and trigram title matches with reciprocal rank
// fusion. A nil embedding runs keyword and title matching only.
func (c *SupabaseClient) HybridSearch(ctx context.Context, query string, embedding []float32, limit int, f SearchFilters) ([]SearchHit, error) {
	params := hybridSearchParams{
		QueryText:      query,
		QueryEmbedding: embedding,
		MatchCount:     limit,
		MediaType:      f.MediaType,
		Genres:         f.Genres,
		MinYear:        f.MinYear,
		MaxYear:        f.MaxYear,
		MinRating:      f.MinRating,
		MaxRuntime:     f.MaxRuntime,
		Language:       f.Language,
		ExcludeIDs:     f.ExcludeIDs,
	}
	var hits []SearchHit
	if err := c.CallRPC(ctx, "search_titles_hybrid", params, &hits); err != nil {
		return nil, fmt.Errorf("search_titles_hybrid rpc: %w", err)
	}
	return hits, nil
}
