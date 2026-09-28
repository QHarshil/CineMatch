// Package assistant runs CineMatch's grounded recommendation agent. An LLM
// plans with read-only catalog tools (hybrid search, similar titles, taste
// profile, the two-stage recommender) and must finish by presenting titles
// those tools returned. Picks are checked against tool results before they
// reach the person, so the agent cannot recommend a title outside the catalog.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/llm"
	"github.com/harshilc/cinematch-backend/search"
)

const (
	maxModelCalls  = 5
	fallbackPicks  = 6
	historyTurns   = 8
	assistantTurns = "assistant"
)

// ChatModel completes a conversation with tool calling. Implemented by
// llm.Client.
type ChatModel interface {
	Complete(ctx context.Context, messages []llm.Message, tools []llm.Tool) (llm.Completion, error)
	Model() string
}

// Catalog runs hybrid search. Implemented by search.Service.
type Catalog interface {
	Search(ctx context.Context, q string, limit int, f db.SearchFilters) (search.Result, error)
}

// TitleStore looks up titles and a person's likes. Implemented by
// db.SupabaseClient.
type TitleStore interface {
	SimilarToTitle(ctx context.Context, movieID string, limit int, f db.SearchFilters) ([]db.SearchHit, error)
	RecentPositiveTitles(ctx context.Context, userID string, limit int) ([]db.Movie, error)
	TitlesNamed(ctx context.Context, title string) ([]db.Movie, error)
}

// Recommender returns the person's ranked feed and how it was produced.
type Recommender interface {
	Recommend(ctx context.Context, userID string) ([]db.Movie, string, error)
}

// RecommenderFunc adapts a function to Recommender.
type RecommenderFunc func(ctx context.Context, userID string) ([]db.Movie, string, error)

// Recommend calls f.
func (f RecommenderFunc) Recommend(ctx context.Context, userID string) ([]db.Movie, string, error) {
	return f(ctx, userID)
}

// Turn is one message of the conversation as the client sent it.
type Turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is one assistant run.
type Request struct {
	UserID string
	Turns  []Turn
	// ModelDisabled skips the LLM, for example once the daily token budget is
	// spent, and serves search results directly.
	ModelDisabled bool
}

// Agent runs the tool-calling loop.
type Agent struct {
	model   ChatModel // nil when no LLM is configured
	catalog Catalog
	titles  TitleStore
	recs    Recommender
	now     func() time.Time
}

// New returns an agent. A nil model serves search results without the LLM.
func New(model ChatModel, catalog Catalog, titles TitleStore, recs Recommender) *Agent {
	return &Agent{model: model, catalog: catalog, titles: titles, recs: recs, now: time.Now}
}

// ModelName is recorded in the audit log.
func (a *Agent) ModelName() string {
	if a.model == nil {
		return "none"
	}
	return a.model.Model()
}

// Fallback notices shown when results come from search instead of the model.
const (
	noticeOffline = "The assistant model is offline right now, so these are direct search matches for your request."
	noticeBudget  = "The assistant has used its model budget for today, so these are direct search matches for your request."
	noticeBusy    = "The model provider is at capacity right now, so these are direct search matches for your request."
	noticeSteps   = "Top search matches for your request."
)

// finishNudge asks the model to finish through present_picks after it
// answered in plain text.
const finishNudge = "Call present_picks now with the refs of the titles you chose. Do not answer in plain text."

// Run executes one request, streaming progress through emit.
func (a *Agent) Run(ctx context.Context, req Request, emit Emitter) Outcome {
	out := Outcome{Model: a.ModelName()}
	switch {
	case a.model == nil:
		return a.fallback(ctx, req, emit, out, noticeOffline)
	case req.ModelDisabled:
		return a.fallback(ctx, req, emit, out, noticeBudget)
	}

	emit(Event{EventStart, StartData{Model: out.Model, PromptVersion: PromptVersion}})
	messages := a.conversation(req.Turns)
	grounded := newGroundingSet()
	nudged := false

	for call := 0; call < maxModelCalls; call++ {
		completion, err := a.model.Complete(ctx, messages, toolDefinitions)
		if err != nil {
			if ctx.Err() != nil {
				out.Status = StatusError
				return out
			}
			slog.Warn("assistant model call failed", "error", err, "model", out.Model)
			notice := noticeOffline
			if errors.Is(err, llm.ErrRateLimited) {
				notice = noticeBusy
			}
			if grounded.size() > 0 {
				return a.presentGrounded(grounded, emit, out, notice, "")
			}
			return a.fallback(ctx, req, emit, out, notice)
		}
		out.Usage.Add(completion.Usage)

		calls := completion.Message.ToolCalls
		if len(calls) == 0 {
			text := cleanText(completion.Message.Content, 600)
			// Text after tool calls usually lists titles, which would skip the
			// grounding check. Ask once for present_picks; if the model still
			// answers in text, present the grounded titles it mentioned.
			if grounded.size() > 0 {
				if !nudged {
					nudged = true
					messages = append(messages, completion.Message, llm.Message{Role: llm.RoleUser, Content: finishNudge})
					continue
				}
				return a.presentGrounded(grounded, emit, out, noticeSteps, text)
			}
			if text == "" {
				return a.fallback(ctx, req, emit, out, noticeOffline)
			}
			if leaksInstructions(text) {
				slog.Warn("assistant reply blocked: repeated its instructions", "model", out.Model)
				text, out.OutputBlocked = safeDecline, true
			}
			emit(Event{EventMessage, MessageData{Text: text}})
			out.Status, out.Message = StatusAnswered, text
			return out
		}

		messages = append(messages, completion.Message)
		for _, tc := range calls {
			if tc.Function.Name == toolPresentPicks {
				message, picks, ungrounded, problem := grounded.resolvePicks(tc.Function.Arguments)
				out.UngroundedDropped += ungrounded
				if len(picks) > 0 {
					if leaksInstructions(message) {
						slog.Warn("assistant message blocked: repeated its instructions", "model", out.Model)
						message, out.OutputBlocked = "Here are picks that fit your request.", true
					}
					emit(Event{EventPicks, PicksData{Message: message, Picks: picks, Dropped: out.UngroundedDropped}})
					out.Status, out.Message, out.Picks = StatusPicks, message, picks
					return out
				}
				// Nothing grounded: tell the model why and let it try again.
				messages = append(messages, toolMessage(tc.ID, encodeForModel(map[string]string{"error": problem})))
				continue
			}
			result, step := a.execute(ctx, req.UserID, tc, grounded, emit)
			out.Steps = append(out.Steps, step)
			messages = append(messages, toolMessage(tc.ID, result.content))
		}
	}

	// The call budget ran out before present_picks.
	if grounded.size() > 0 {
		return a.presentGrounded(grounded, emit, out, noticeSteps, "")
	}
	return a.fallback(ctx, req, emit, out, noticeOffline)
}

// conversation builds the model input: instructions, then recent turns.
func (a *Agent) conversation(turns []Turn) []llm.Message {
	if len(turns) > historyTurns {
		turns = turns[len(turns)-historyTurns:]
	}
	messages := make([]llm.Message, 0, len(turns)+1)
	messages = append(messages, llm.Message{Role: llm.RoleSystem, Content: systemPrompt(a.now())})
	for _, t := range turns {
		role := llm.RoleUser
		if t.Role == assistantTurns {
			role = llm.RoleAssistant
		}
		messages = append(messages, llm.Message{Role: role, Content: t.Content})
	}
	return messages
}

func toolMessage(callID, content string) llm.Message {
	return llm.Message{Role: llm.RoleTool, ToolCallID: callID, Content: content}
}

// execute runs one read-only tool call and records it.
func (a *Agent) execute(ctx context.Context, userID string, tc llm.ToolCall, g *groundingSet, emit Emitter) (toolResult, StepRecord) {
	name, arguments := tc.Function.Name, tc.Function.Arguments
	if strings.TrimSpace(arguments) == "" {
		arguments = "{}"
	}
	args := json.RawMessage(arguments)
	if !json.Valid(args) {
		args = json.RawMessage("{}")
	}
	emit(Event{EventToolCall, ToolCallData{ID: tc.ID, Tool: name, Label: labelFor(name, arguments), Args: args}})

	toolCtx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	started := time.Now()

	var result toolResult
	switch name {
	case toolSearchCatalog:
		result = a.searchCatalog(toolCtx, arguments, g)
	case toolFindSimilar:
		result = a.findSimilar(toolCtx, arguments, g)
	case toolTasteProfile:
		result = a.tasteProfile(toolCtx, userID, g)
	case toolGetRecommendations:
		result = a.recommendations(toolCtx, userID, g)
	default:
		result = errorResult("unknown tool " + name)
	}
	latency := int(time.Since(started).Milliseconds())

	emit(Event{EventToolResult, ToolResultData{
		ID: tc.ID, Tool: name, Count: result.count, LatencyMS: latency,
		Retrieval: result.retrieval, Titles: result.titles, Error: result.err,
	}})
	return result, StepRecord{
		Tool: name, Args: redactForAudit(string(args)), ResultCount: result.count,
		LatencyMS: latency, Error: result.err,
	}
}

// presentGrounded shows titles tools already returned when the model could
// not finish. Titles named in the model's text, if any, come first. Reasons
// come from catalog metadata, not the model.
func (a *Agent) presentGrounded(g *groundingSet, emit Emitter, out Outcome, notice, modelText string) Outcome {
	candidates := preferMentioned(g.candidates(), modelText)
	if len(candidates) == 0 {
		emit(Event{EventMessage, MessageData{Text: notice}})
		out.Status, out.Message = StatusFallback, notice
		return out
	}
	picks := make([]Pick, 0, fallbackPicks)
	for _, t := range candidates[:min(len(candidates), fallbackPicks)] {
		picks = append(picks, Pick{Movie: t.movie, Reason: describeTitle(t.movie), Similarity: t.similarity, Source: t.source})
	}
	emit(Event{EventPicks, PicksData{Message: notice, Picks: picks, Dropped: out.UngroundedDropped}})
	out.Status, out.Message, out.Picks = StatusFallback, notice, picks
	return out
}

// fallback answers with hybrid search on the latest message, so the feature
// still returns catalog titles when the model is off, over budget, or down.
func (a *Agent) fallback(ctx context.Context, req Request, emit Emitter, out Outcome, notice string) Outcome {
	query := ""
	for i := len(req.Turns) - 1; i >= 0; i-- {
		if req.Turns[i].Role != assistantTurns {
			query = cleanText(req.Turns[i].Content, 200)
			break
		}
	}
	result, err := a.catalog.Search(ctx, query, fallbackPicks, db.SearchFilters{})
	if err != nil || len(result.Hits) == 0 {
		msg := "Search is unavailable right now. Please try again in a moment."
		if err == nil {
			msg = "Nothing in the catalog matched that. Try describing a mood, a plot, or a title."
		}
		emit(Event{EventError, ErrorData{Code: "no_results", Message: msg}})
		out.Status, out.Message = StatusError, msg
		return out
	}
	g := newGroundingSet()
	for _, hit := range result.Hits {
		g.add(hit.Movie, hit.Similarity, toolSearchCatalog)
	}
	return a.presentGrounded(g, emit, out, notice, "")
}

// preferMentioned moves candidates whose titles appear in text to the front,
// keeping tool order within each group.
func preferMentioned(candidates []*groundedTitle, text string) []*groundedTitle {
	if text == "" {
		return candidates
	}
	lower := strings.ToLower(text)
	mentioned := make([]*groundedTitle, 0, len(candidates))
	rest := make([]*groundedTitle, 0, len(candidates))
	for _, t := range candidates {
		if strings.Contains(lower, strings.ToLower(t.movie.Title)) {
			mentioned = append(mentioned, t)
		} else {
			rest = append(rest, t)
		}
	}
	return append(mentioned, rest...)
}
