package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, status int, body string, inspect func(*http.Request, chatRequest)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if inspect != nil {
			inspect(r, req)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewClient(Config{BaseURL: srv.URL + "/", APIKey: "test-key", Model: "test-model", ReasoningEffort: "none"})
}

func TestComplete(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		wantContent  string
		wantTool     string
		wantArgs     string
		wantUsage    Usage
		wantErr      error
		wantErrMatch string
	}{
		{
			name:        "tool call with string arguments",
			status:      http.StatusOK,
			body:        `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"search_catalog","arguments":"{\"query\":\"heist\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":120,"completion_tokens":18}}`,
			wantTool:    "search_catalog",
			wantArgs:    `{"query":"heist"}`,
			wantUsage:   Usage{InputTokens: 120, OutputTokens: 18},
			wantContent: "",
		},
		{
			name:     "tool call with object arguments",
			status:   http.StatusOK,
			body:     `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call_2","type":"function","function":{"name":"find_similar","arguments":{"ref":"t3"}}}]},"finish_reason":"tool_calls"}]}`,
			wantTool: "find_similar",
			wantArgs: `{"ref":"t3"}`,
		},
		{
			name:        "strips inline reasoning",
			status:      http.StatusOK,
			body:        `{"choices":[{"message":{"content":"<think>the user wants\nsomething</think>\nWhich decade?"},"finish_reason":"stop"}]}`,
			wantContent: "Which decade?",
		},
		{name: "rate limit maps to ErrRateLimited", status: http.StatusTooManyRequests, body: `{"error":{"message":"quota"}}`, wantErr: ErrRateLimited},
		{name: "surfaces provider errors", status: http.StatusBadRequest, body: `{"error":{"message":"unknown model"}}`, wantErrMatch: "unknown model"},
		{name: "rejects empty choices", status: http.StatusOK, body: `{"choices":[]}`, wantErrMatch: "no choices"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.status, tc.body, nil)
			got, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)

			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			case tc.wantErrMatch != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantErrMatch) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErrMatch)
				}
				return
			case err != nil:
				t.Fatal(err)
			}

			if got.Message.Content != tc.wantContent {
				t.Errorf("content = %q, want %q", got.Message.Content, tc.wantContent)
			}
			if tc.wantTool != "" {
				if len(got.Message.ToolCalls) != 1 {
					t.Fatalf("tool calls = %d, want 1", len(got.Message.ToolCalls))
				}
				fn := got.Message.ToolCalls[0].Function
				if fn.Name != tc.wantTool || fn.Arguments != tc.wantArgs {
					t.Errorf("call = %s(%s), want %s(%s)", fn.Name, fn.Arguments, tc.wantTool, tc.wantArgs)
				}
			}
			if got.Usage != tc.wantUsage {
				t.Errorf("usage = %+v, want %+v", got.Usage, tc.wantUsage)
			}
		})
	}
}

func TestCompleteSendsConfiguredRequest(t *testing.T) {
	tools := []Tool{{Type: "function", Function: FunctionDef{Name: "search_catalog", Parameters: json.RawMessage(`{"type":"object"}`)}}}
	c := newTestClient(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`, func(r *http.Request, req chatRequest) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing bearer token")
		}
		if req.Model != "test-model" || req.ReasoningEffort != "none" || len(req.Tools) != 1 || req.MaxTokens != 800 {
			t.Errorf("request = %+v", req)
		}
	})
	if _, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, tools); err != nil {
		t.Fatal(err)
	}
}

func TestReasoningEffortOmittedWhenUnset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		if _, ok := raw["reasoning_effort"]; ok {
			t.Error("reasoning_effort should be omitted")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("no Authorization header without a key")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Model: "qwen3:8b"})
	if _, err := c.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil); err != nil {
		t.Fatal(err)
	}
}
