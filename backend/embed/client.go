// Package embed turns search text into vectors in the catalog's embedding
// space. Query vectors must come from the model that embedded the catalog for
// cosine similarity to mean anything, so the backend and the seeder read the
// same EMBED_MODEL setting.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultModel is the model the catalog was embedded with.
	DefaultModel = "text-embedding-3-small"
	// Dimensions is the vector size stored in movies.embedding.
	Dimensions = 1536

	defaultBaseURL = "https://api.openai.com/v1"
	maxInputRunes  = 1000
)

// Embedder returns the embedding for a piece of text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Client calls the OpenAI embeddings endpoint.
type Client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a client for model with a 5-second timeout, so a slow
// upstream degrades search to keyword matching instead of stalling the request.
func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey:     apiKey,
		model:      model,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed returns the query vector for text. Input is truncated to keep a single
// request small; search queries are far shorter than the limit.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	if c.apiKey == "" {
		return nil, errors.New("embed: api key not configured")
	}
	if runes := []rune(text); len(runes) > maxInputRunes {
		text = string(runes[:maxInputRunes])
	}

	body, err := json.Marshal(embeddingRequest{Model: c.model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("embed: encoding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed: building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("embed: reading response: %w", err)
	}
	var parsed embeddingResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("embed: decoding response (HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := http.StatusText(resp.StatusCode)
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return nil, fmt.Errorf("embed: HTTP %d: %s", resp.StatusCode, msg)
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) != Dimensions {
		return nil, fmt.Errorf("embed: expected one %d-dim vector", Dimensions)
	}
	return parsed.Data[0].Embedding, nil
}
