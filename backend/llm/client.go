// Package llm is a small client for OpenAI-compatible chat completions with
// tool calling. The same code runs against Ollama locally and hosted providers
// (Groq, Gemini, OpenRouter, OpenAI); only the base URL, key, and model change.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Message roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// ErrRateLimited reports that the provider rejected the call with HTTP 429,
// typically a free-tier quota. Callers fall back instead of retrying.
var ErrRateLimited = errors.New("llm: provider rate limit reached")

// Message is one chat turn.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall is a function call requested by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
	// ExtraContent carries provider data that must be echoed back unchanged.
	// Gemini puts a thought_signature here and rejects the next request
	// with HTTP 400 if it is missing.
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

// FunctionCall names the tool and carries its arguments as a JSON string.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// UnmarshalJSON accepts arguments as a JSON string (the OpenAI format) or as
// a raw object, which some OpenAI-compatible servers return.
func (f *FunctionCall) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Name = raw.Name
	var asString string
	if err := json.Unmarshal(raw.Arguments, &asString); err == nil {
		f.Arguments = asString
		return nil
	}
	f.Arguments = string(raw.Arguments)
	return nil
}

// Tool declares a function the model may call. Parameters is a JSON Schema.
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef describes a callable tool.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Usage is the token cost of one completion.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Add accumulates another completion's usage.
func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
}

// Completion is the model's reply to one request.
type Completion struct {
	Message      Message
	FinishReason string
	Usage        Usage
}

// Config selects the provider and model.
type Config struct {
	BaseURL string // e.g. http://localhost:11434/v1 or https://api.groq.com/openai/v1
	APIKey  string // empty for local Ollama
	Model   string
	// ReasoningEffort is sent only when set. "none" turns off thinking on
	// reasoning models (Ollama, Gemini); some providers reject the field.
	ReasoningEffort string
	Temperature     float64
	MaxTokens       int
	Timeout         time.Duration
}

// Client calls a chat completions endpoint.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// NewClient returns a client for the configured provider.
func NewClient(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 800
	}
	return &Client{cfg: cfg, httpClient: &http.Client{Timeout: cfg.Timeout}}
}

// Model returns the configured model name, recorded in the audit log.
func (c *Client) Model() string { return c.cfg.Model }

type chatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Tools           []Tool    `json:"tools,omitempty"`
	Temperature     float64   `json:"temperature"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
}

// errorMessage extracts the provider's error text. OpenAI and Ollama return
// {"error": {...}}; Gemini's compatibility layer wraps it in an array.
func errorMessage(raw []byte, status int) string {
	type body struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	var single body
	if json.Unmarshal(raw, &single) == nil && single.Error != nil && single.Error.Message != "" {
		return single.Error.Message
	}
	var list []body
	if json.Unmarshal(raw, &list) == nil && len(list) > 0 && list[0].Error != nil && list[0].Error.Message != "" {
		return list[0].Error.Message
	}
	return http.StatusText(status)
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   *string    `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// thinkBlock matches reasoning some local models inline in their content.
var thinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)

// Complete sends the conversation and tool definitions and returns the reply.
func (c *Client) Complete(ctx context.Context, messages []Message, tools []Tool) (Completion, error) {
	body, err := json.Marshal(chatRequest{
		Model:           c.cfg.Model,
		Messages:        messages,
		Tools:           tools,
		Temperature:     c.cfg.Temperature,
		MaxTokens:       c.cfg.MaxTokens,
		ReasoningEffort: c.cfg.ReasoningEffort,
	})
	if err != nil {
		return Completion{}, fmt.Errorf("llm: encoding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Completion{}, fmt.Errorf("llm: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Completion{}, fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Completion{}, fmt.Errorf("llm: reading response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return Completion{}, ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		return Completion{}, fmt.Errorf("llm: HTTP %d: %s", resp.StatusCode, errorMessage(raw, resp.StatusCode))
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Completion{}, fmt.Errorf("llm: decoding response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Completion{}, errors.New("llm: response had no choices")
	}

	choice := parsed.Choices[0]
	content := ""
	if choice.Message.Content != nil {
		content = strings.TrimSpace(thinkBlock.ReplaceAllString(*choice.Message.Content, ""))
	}
	return Completion{
		Message: Message{
			Role:      RoleAssistant,
			Content:   content,
			ToolCalls: choice.Message.ToolCalls,
		},
		FinishReason: choice.FinishReason,
		Usage: Usage{
			InputTokens:  parsed.Usage.PromptTokens,
			OutputTokens: parsed.Usage.CompletionTokens,
		},
	}, nil
}
