package assistant

import (
	"encoding/json"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/llm"
)

// Event types streamed to the client as server-sent events.
const (
	EventStart      = "start"
	EventToolCall   = "tool_call"
	EventToolResult = "tool_result"
	EventPicks      = "picks"
	EventMessage    = "message"
	EventDone       = "done"
	EventError      = "error"
)

// Run statuses, stored in assistant_runs.status.
const (
	StatusPicks    = "picks"    // the model presented grounded titles
	StatusAnswered = "answered" // the model replied in text (a question or a decline)
	StatusFallback = "fallback" // search results served without the model's final answer
	StatusError    = "error"    // nothing could be returned
)

// Event is one streamed update. Data is JSON-encoded as the SSE payload.
type Event struct {
	Type string
	Data any
}

// Emitter receives events as the run progresses.
type Emitter func(Event)

// StartData opens the stream.
type StartData struct {
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
}

// ToolCallData announces a tool call before it runs, with a readable label,
// so the person sees what the agent is doing as it does it.
type ToolCallData struct {
	ID    string          `json:"id"`
	Tool  string          `json:"tool"`
	Label string          `json:"label"`
	Args  json.RawMessage `json:"args"`
}

// ToolResultData reports what a tool returned.
type ToolResultData struct {
	ID        string   `json:"id"`
	Tool      string   `json:"tool"`
	Count     int      `json:"count"`
	LatencyMS int      `json:"latency_ms"`
	Retrieval string   `json:"retrieval,omitempty"`
	Titles    []string `json:"titles,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// Pick is one recommended title. Similarity is the retrieval score behind it
// when one exists; the UI shows it as a confidence signal.
type Pick struct {
	Movie      db.Movie `json:"movie"`
	Reason     string   `json:"reason"`
	Similarity *float64 `json:"similarity,omitempty"`
	Source     string   `json:"source"`
}

// PicksData carries the final recommendations.
type PicksData struct {
	Message string `json:"message"`
	Picks   []Pick `json:"picks"`
	// Dropped counts picks rejected because no tool returned them.
	Dropped int `json:"dropped"`
}

// MessageData carries a text reply: a clarifying question, a decline, or a
// notice that results came from the fallback path.
type MessageData struct {
	Text string `json:"text"`
}

// ErrorData reports a failure the client should display.
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StepRecord is the audit entry for one tool call.
type StepRecord struct {
	Tool        string          `json:"tool"`
	Args        json.RawMessage `json:"args"`
	ResultCount int             `json:"result_count"`
	LatencyMS   int             `json:"latency_ms"`
	Error       string          `json:"error,omitempty"`
}

// Outcome summarizes a run for the audit log and the done event.
type Outcome struct {
	Status            string
	Message           string
	Picks             []Pick
	Steps             []StepRecord
	UngroundedDropped int
	Usage             llm.Usage
	Model             string
}
