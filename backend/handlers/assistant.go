package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/harshilc/cinematch-backend/assistant"
	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/llm"
	"github.com/harshilc/cinematch-backend/middleware"
)

const (
	assistantMaxTurns      = 12
	assistantMaxTurnRunes  = 800
	assistantMaxTotalRunes = 3000
	assistantRunTimeout    = 75 * time.Second
	// The server's WriteTimeout (60s) would cut long streams; the handler
	// extends its own deadline past the run timeout.
	assistantWriteDeadline = 90 * time.Second
	auditWriteTimeout      = 5 * time.Second
)

// AssistantRunner runs the agent. Implemented by assistant.Agent.
type AssistantRunner interface {
	Run(ctx context.Context, req assistant.Request, emit assistant.Emitter) assistant.Outcome
	ModelName() string
}

// AssistantStore reserves, finishes, and counts audited runs. Implemented by
// db.SupabaseClient.
type AssistantStore interface {
	AssistantUsageSince(ctx context.Context, userID, ipHash string, since time.Time) (db.AssistantUsage, error)
	ReserveAssistantRun(ctx context.Context, r db.RunReservation) (db.ReservationResult, error)
	FinishAssistantRun(ctx context.Context, runID string, result db.AssistantRunResult) error
}

// AssistantLimits caps use per UTC day. Per-user runs keep one account from
// using the whole budget; the global run and token caps keep total spend
// under the model provider's free tier. Past a global cap the assistant still
// answers, from search alone.
type AssistantLimits struct {
	UserDailyRuns  int
	GuestDailyRuns int // anonymous sessions are free to create, so they get fewer runs
	// IPDailyRuns caps one network across all its accounts, so creating guest
	// after guest cannot spend the shared budget.
	IPDailyRuns       int
	GlobalDailyRuns   int
	GlobalDailyTokens int
	// IPHashKey keys the HMAC of client IPs; the audit log stores only the hash.
	IPHashKey []byte
}

// ipHash returns the keyed hash that identifies a network in the audit log.
// IPv6 is grouped by /64, the block a single subscriber usually holds, so
// rotating addresses inside it does not reset the network limit.
func (l AssistantLimits) ipHash(r *http.Request) string {
	network := r.RemoteAddr
	if ip := net.ParseIP(r.RemoteAddr); ip != nil && ip.To4() == nil {
		network = ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	mac := hmac.New(sha256.New, l.IPHashKey)
	mac.Write([]byte(network))
	return hex.EncodeToString(mac.Sum(nil))
}

// runsFor returns the caller's daily run limit.
func (l AssistantLimits) runsFor(ctx context.Context) int {
	if middleware.IsGuestFromContext(ctx) {
		return l.GuestDailyRuns
	}
	return l.UserDailyRuns
}

func (l AssistantLimits) modelBudgetSpent(u db.AssistantUsage) bool {
	return u.TotalRuns >= l.GlobalDailyRuns || u.TotalTokens >= l.GlobalDailyTokens
}

type assistantRequestBody struct {
	Messages []assistant.Turn `json:"messages"`
}

type quotaError struct {
	Error    string `json:"error"`
	ResetsAt string `json:"resets_at"`
}

// DoneData closes the stream with the audit ID, cost, and remaining quota.
type DoneData struct {
	RunID          string    `json:"run_id"`
	Status         string    `json:"status"`
	Model          string    `json:"model"`
	Usage          llm.Usage `json:"usage"`
	LatencyMS      int       `json:"latency_ms"`
	RemainingToday int       `json:"remaining_today"`
}

// RunAssistant handles POST /assistant and streams the run as server-sent
// events: start, tool_call, tool_result, then picks or message, then done.
// The run is reserved (counted) before it starts; if that fails, no model
// call is made.
func RunAssistant(runner AssistantRunner, store AssistantStore, limits AssistantLimits) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var body assistantRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "body must be JSON with a messages array")
			return
		}
		turns, err := validateTurns(body.Messages)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		dayStart, resetsAt := utcDay(time.Now())
		dailyRuns := limits.runsFor(r.Context())
		prompt := turns[len(turns)-1].Content
		sum := sha256.Sum256([]byte(prompt))
		runID := newRunID()
		reservation, err := store.ReserveAssistantRun(r.Context(), db.RunReservation{
			RunID:         runID,
			UserID:        userID,
			IPHash:        limits.ipHash(r),
			Since:         dayStart,
			UserLimit:     dailyRuns,
			IPLimit:       limits.IPDailyRuns,
			RequestID:     chimw.GetReqID(r.Context()),
			PromptVersion: assistant.PromptVersion,
			Model:         runner.ModelName(),
			InputSHA256:   hex.EncodeToString(sum[:]),
			InputChars:    len([]rune(prompt)),
		})
		if err != nil {
			slog.Error("assistant run reservation failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, "assistant is temporarily unavailable")
			return
		}
		if !reservation.Allowed {
			msg := fmt.Sprintf("daily limit of %d assistant requests reached", dailyRuns)
			if reservation.Reason == "network" {
				msg = "daily assistant limit reached for this network"
			}
			writeJSON(w, http.StatusTooManyRequests, quotaError{Error: msg, ResetsAt: resetsAt.Format(time.RFC3339)})
			return
		}
		usage := reservation.AssistantUsage

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming unsupported")
			return
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(assistantWriteDeadline))
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		emit := func(e assistant.Event) {
			if err := writeSSE(w, e.Type, e.Data); err == nil {
				flusher.Flush()
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), assistantRunTimeout)
		defer cancel()
		started := time.Now()
		outcome := runner.Run(ctx, assistant.Request{
			UserID:        userID,
			Turns:         turns,
			ModelDisabled: limits.modelBudgetSpent(usage),
		}, emit)
		latency := int(time.Since(started).Milliseconds())

		// The reservation already counts this run; a failed update only loses
		// the outcome. Write it even if the client disconnected mid-stream.
		auditCtx, cancelAudit := context.WithTimeout(context.WithoutCancel(r.Context()), auditWriteTimeout)
		if err := store.FinishAssistantRun(auditCtx, runID, runResult(outcome, latency)); err != nil {
			slog.Error("assistant audit write failed", "run_id", runID, "error", err)
		}
		cancelAudit()

		emit(assistant.Event{Type: assistant.EventDone, Data: DoneData{
			RunID:          runID,
			Status:         outcome.Status,
			Model:          outcome.Model,
			Usage:          outcome.Usage,
			LatencyMS:      latency,
			RemainingToday: remainingRuns(dailyRuns, usage, limits) - 1,
		}})
		slog.Info("assistant run",
			"run_id", runID,
			"status", outcome.Status,
			"model", outcome.Model,
			"prompt_version", assistant.PromptVersion,
			"tool_calls", len(outcome.Steps),
			"picks", len(outcome.Picks),
			"ungrounded_dropped", outcome.UngroundedDropped,
			"output_blocked", outcome.OutputBlocked,
			"input_tokens", outcome.Usage.InputTokens,
			"output_tokens", outcome.Usage.OutputTokens,
			"latency_ms", latency,
		)
	}
}

type assistantUsageResponse struct {
	Used           int    `json:"used"`
	Limit          int    `json:"limit"`
	Remaining      int    `json:"remaining"`
	ResetsAt       string `json:"resets_at"`
	ModelAvailable bool   `json:"model_available"`
}

// GetAssistantUsage handles GET /assistant/usage: today's quota for the
// signed-in user and whether answers will come from the model or from search.
func GetAssistantUsage(runner AssistantRunner, store AssistantStore, limits AssistantLimits) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		dayStart, resetsAt := utcDay(time.Now())
		usage, err := store.AssistantUsageSince(r.Context(), userID, limits.ipHash(r), dayStart)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "assistant is temporarily unavailable")
			return
		}
		dailyRuns := limits.runsFor(r.Context())
		writeJSON(w, http.StatusOK, assistantUsageResponse{
			Used:           usage.UserRuns,
			Limit:          dailyRuns,
			Remaining:      remainingRuns(dailyRuns, usage, limits),
			ResetsAt:       resetsAt.Format(time.RFC3339),
			ModelAvailable: runner.ModelName() != "none" && !limits.modelBudgetSpent(usage),
		})
	}
}

// remainingRuns is what the caller can still run today: the tighter of the
// account limit and the network limit.
func remainingRuns(dailyRuns int, usage db.AssistantUsage, limits AssistantLimits) int {
	return max(0, min(dailyRuns-usage.UserRuns, limits.IPDailyRuns-usage.IPRuns))
}

// validateTurns checks roles and lengths, cleans the text, and keeps the most
// recent turns that fit the total length budget.
func validateTurns(turns []assistant.Turn) ([]assistant.Turn, error) {
	if len(turns) == 0 {
		return nil, errors.New("messages must not be empty")
	}
	if len(turns) > assistantMaxTurns {
		turns = turns[len(turns)-assistantMaxTurns:]
	}
	cleaned := make([]assistant.Turn, 0, len(turns))
	for _, t := range turns {
		if t.Role != "user" && t.Role != "assistant" {
			return nil, errors.New("message role must be user or assistant")
		}
		content := cleanTurn(t.Content)
		runes := []rune(content)
		if t.Role == "user" && len(runes) > assistantMaxTurnRunes {
			return nil, fmt.Errorf("messages must be %d characters or fewer", assistantMaxTurnRunes)
		}
		if len(runes) > assistantMaxTurnRunes {
			content = string(runes[:assistantMaxTurnRunes])
		}
		cleaned = append(cleaned, assistant.Turn{Role: t.Role, Content: content})
	}
	last := cleaned[len(cleaned)-1]
	if last.Role != "user" || last.Content == "" {
		return nil, errors.New("the last message must be a non-empty user message")
	}

	total := 0
	start := len(cleaned)
	for start > 0 {
		n := len([]rune(cleaned[start-1].Content))
		if total+n > assistantMaxTotalRunes {
			break
		}
		total += n
		start--
	}
	return cleaned[start:], nil
}

// cleanTurn drops control characters, keeping line breaks. Chat text is only
// rendered as plain text, so angle brackets ("under <2h") are left alone.
func cleanTurn(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// runResult is the outcome written to the reserved audit row.
func runResult(out assistant.Outcome, latencyMS int) db.AssistantRunResult {
	steps, err := json.Marshal(out.Steps)
	if err != nil || out.Steps == nil {
		steps = []byte("[]")
	}
	pickIDs := make([]string, len(out.Picks))
	for i, p := range out.Picks {
		pickIDs[i] = p.Movie.ID
	}
	return db.AssistantRunResult{
		Status:            out.Status,
		Model:             out.Model,
		Steps:             steps,
		PickIDs:           pickIDs,
		UngroundedDropped: out.UngroundedDropped,
		OutputBlocked:     out.OutputBlocked,
		InputTokens:       out.Usage.InputTokens,
		OutputTokens:      out.Usage.OutputTokens,
		LatencyMS:         latencyMS,
	}
}

// writeSSE writes one server-sent event. JSON has no raw newlines, so the
// payload always fits on a single data line.
func writeSSE(w io.Writer, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	return err
}

// utcDay returns the start of the current UTC day and the next reset.
func utcDay(now time.Time) (start, next time.Time) {
	start = now.UTC().Truncate(24 * time.Hour)
	return start, start.Add(24 * time.Hour)
}

// newRunID returns a random version 4 UUID.
func newRunID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
