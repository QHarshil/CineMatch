package db

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

// maxEmbeddingHistory caps how many recent positive interactions feed a user's
// taste vector. Recent likes dominate the result, and the cap bounds the work
// done per interaction.
const maxEmbeddingHistory = 100

// recencyDecay weights each older positive interaction relative to the next more
// recent one when averaging movie embeddings into a taste vector.
const recencyDecay = 0.92

// PositiveInteractionMovieIDs returns the movie IDs a user liked or watched,
// most recent first, capped at maxEmbeddingHistory.
func (c *SupabaseClient) PositiveInteractionMovieIDs(ctx context.Context, userID string) ([]string, error) {
	params := url.Values{}
	params.Set("select", "movie_id")
	params.Set("user_id", "eq."+userID)
	params.Set("type", "in.(like,watch)")
	params.Set("order", "created_at.desc")
	params.Set("limit", strconv.Itoa(maxEmbeddingHistory))

	var rows []struct {
		MovieID string `json:"movie_id"`
	}
	if err := c.doGet(ctx, "interactions", params, &rows); err != nil {
		return nil, fmt.Errorf("fetching positive interactions for user %s: %w", userID, err)
	}

	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.MovieID
	}
	return ids, nil
}

// movieEmbeddings fetches the stored embedding for each given movie ID.
func (c *SupabaseClient) movieEmbeddings(ctx context.Context, movieIDs []string) (map[string][]float32, error) {
	if len(movieIDs) == 0 {
		return map[string][]float32{}, nil
	}

	params := url.Values{}
	params.Set("select", "id,embedding")
	params.Set("id", "in.("+strings.Join(movieIDs, ",")+")")

	var rows []struct {
		ID        string `json:"id"`
		Embedding string `json:"embedding"`
	}
	if err := c.doGet(ctx, "movies", params, &rows); err != nil {
		return nil, fmt.Errorf("fetching movie embeddings: %w", err)
	}

	out := make(map[string][]float32, len(rows))
	for _, row := range rows {
		if row.Embedding == "" {
			continue
		}
		vec, err := parseVectorString(row.Embedding)
		if err != nil {
			return nil, fmt.Errorf("parsing embedding for movie %s: %w", row.ID, err)
		}
		out[row.ID] = vec
	}
	return out, nil
}

// UpsertUserEmbedding writes or replaces a user's taste vector.
func (c *SupabaseClient) UpsertUserEmbedding(ctx context.Context, userID string, embedding []float32) error {
	payload := map[string]string{
		"user_id":   userID,
		"embedding": vectorToString(embedding),
	}
	if err := c.doUpsert(ctx, "/rest/v1/user_embeddings", payload); err != nil {
		return fmt.Errorf("upserting user embedding: %w", err)
	}
	return nil
}

// DeleteUserEmbedding removes a user's taste vector, returning them to the
// cold-start (popularity) path until they interact again.
func (c *SupabaseClient) DeleteUserEmbedding(ctx context.Context, userID string) error {
	params := url.Values{}
	params.Set("user_id", "eq."+userID)
	if err := c.doDelete(ctx, "/rest/v1/user_embeddings", params); err != nil {
		return fmt.Errorf("deleting user embedding: %w", err)
	}
	return nil
}

// RefreshUserEmbedding recomputes a user's taste vector from the movies they have
// liked or watched and stores it. Recent interactions are weighted more heavily
// so recommendations track evolving taste. With no positive interactions the
// stored vector is removed. This is what makes /recommend actually personalize:
// match_movies has nothing to query against until this vector exists.
func (c *SupabaseClient) RefreshUserEmbedding(ctx context.Context, userID string) error {
	movieIDs, err := c.PositiveInteractionMovieIDs(ctx, userID)
	if err != nil {
		return err
	}
	if len(movieIDs) == 0 {
		return c.DeleteUserEmbedding(ctx, userID)
	}

	embeddings, err := c.movieEmbeddings(ctx, movieIDs)
	if err != nil {
		return err
	}

	taste := recencyWeightedMean(movieIDs, embeddings)
	if taste == nil {
		return c.DeleteUserEmbedding(ctx, userID)
	}
	return c.UpsertUserEmbedding(ctx, userID, taste)
}

// recencyWeightedMean averages movie embeddings with exponentially decaying
// weights (movieIDs are ordered newest-first) and L2-normalizes the result.
// Returns nil when none of the movie IDs had a usable embedding.
func recencyWeightedMean(movieIDsByRecency []string, embeddings map[string][]float32) []float32 {
	var sum []float32
	var weightSum float64
	weight := 1.0

	for _, id := range movieIDsByRecency {
		vec, ok := embeddings[id]
		if !ok {
			continue
		}
		if sum == nil {
			sum = make([]float32, len(vec))
		} else if len(vec) != len(sum) {
			continue // defend against dimension drift in the catalog
		}
		for i, v := range vec {
			sum[i] += float32(weight) * v
		}
		weightSum += weight
		weight *= recencyDecay
	}

	if sum == nil || weightSum == 0 {
		return nil
	}

	var norm float64
	for i := range sum {
		sum[i] /= float32(weightSum)
		norm += float64(sum[i]) * float64(sum[i])
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range sum {
			sum[i] /= float32(norm)
		}
	}
	return sum
}

// vectorToString serializes a float32 slice to the pgvector text format
// "[v1,v2,...]" used by PostgREST for inserts into vector columns.
func vectorToString(vec []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
