// Package db provides the Supabase REST and RPC client used by the CineMatch backend.
// This client uses the service key, which bypasses Row Level Security.
// It must never be instantiated from frontend code or exposed via API responses.
package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SupabaseClient wraps an http.Client configured for Supabase REST and RPC calls.
// The service key grants full database access, so this struct must stay server-side only.
type SupabaseClient struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
}

// NewSupabaseClient constructs a SupabaseClient. baseURL is the Supabase project URL
// (e.g. https://xyz.supabase.co) and serviceKey is the secret service-role key.
func NewSupabaseClient(baseURL, serviceKey string) *SupabaseClient {
	return &SupabaseClient{
		baseURL:    baseURL,
		serviceKey: serviceKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Ping verifies connectivity to the Supabase REST API.
// Used by the /health endpoint to surface database reachability.
func (c *SupabaseClient) Ping() error {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		c.baseURL+"/rest/v1/",
		nil,
	)
	if err != nil {
		return fmt.Errorf("building supabase ping request: %w", err)
	}
	c.injectAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("supabase ping failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("supabase returned %d", resp.StatusCode)
	}
	return nil
}

// CallRPC invokes a Supabase Postgres function via the REST API RPC endpoint.
// payload is JSON-marshalled and sent as the request body.
// dest is populated with the decoded JSON response.
func (c *SupabaseClient) CallRPC(ctx context.Context, fn string, payload, dest any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling rpc payload for %s: %w", fn, err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/rest/v1/rpc/"+fn,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("building rpc request for %s: %w", fn, err)
	}
	c.injectAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling supabase rpc %s: %w", fn, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading rpc response from %s: %w", fn, err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase rpc %s returned %d: %s", fn, resp.StatusCode, string(respBytes))
	}

	if dest != nil {
		if err := json.Unmarshal(respBytes, dest); err != nil {
			return fmt.Errorf("decoding rpc response from %s: %w", fn, err)
		}
	}
	return nil
}

// injectAuthHeaders adds Supabase service-key auth to every outgoing request.
// The service key bypasses RLS — callers are responsible for enforcing authorization.
func (c *SupabaseClient) injectAuthHeaders(req *http.Request) {
	req.Header.Set("apikey", c.serviceKey)
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("Content-Type", "application/json")
}
