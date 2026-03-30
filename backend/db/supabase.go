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
	"net/url"
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

	// Treat any 4xx/5xx as a failure — a 401 means the service key is wrong,
	// which is just as broken as a 500 from the database's perspective.
	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase returned %d", resp.StatusCode)
	}
	return nil
}

// CallRPC invokes a Supabase Postgres function via the RPC endpoint.
// payload is JSON-encoded as the request body; dest receives the decoded response.
func (c *SupabaseClient) CallRPC(ctx context.Context, fn string, payload, dest any) error {
	return c.doPost(ctx, "/rest/v1/rpc/"+fn, payload, dest)
}

// doGet performs an authenticated GET against the Supabase REST API.
// table is the PostgREST table path (e.g. "movies"); params are appended as query string.
func (c *SupabaseClient) doGet(ctx context.Context, table string, params url.Values, dest any) error {
	endpoint := c.baseURL + "/rest/v1/" + table + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("building GET request for %s: %w", table, err)
	}
	c.injectAuthHeaders(req)
	return c.execute(req, dest)
}

// doPost performs an authenticated POST against the Supabase REST API.
// path is relative to baseURL (e.g. "/rest/v1/interactions" or "/rest/v1/rpc/match_movies").
func (c *SupabaseClient) doPost(ctx context.Context, path string, payload, dest any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload for %s: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building POST request for %s: %w", path, err)
	}
	c.injectAuthHeaders(req)

	// Request minimal response body on inserts — we don't need the echoed row.
	if dest == nil {
		req.Header.Set("Prefer", "return=minimal")
	}

	return c.execute(req, dest)
}

// execute dispatches a pre-built request and decodes the response into dest.
func (c *SupabaseClient) execute(req *http.Request, dest any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s %s: %w", req.Method, req.URL.Path, err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase %s %s returned %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(respBytes))
	}

	if dest != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, dest); err != nil {
			return fmt.Errorf("decoding response from %s %s: %w", req.Method, req.URL.Path, err)
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
