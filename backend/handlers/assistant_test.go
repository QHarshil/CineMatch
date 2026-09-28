package handlers_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/harshilc/cinematch-backend/assistant"
	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/handlers"
	"github.com/harshilc/cinematch-backend/llm"
	"github.com/harshilc/cinematch-backend/middleware"
)

type stubRunner struct {
	model   string
	gotReq  assistant.Request
	outcome assistant.Outcome
}

func (s *stubRunner) ModelName() string { return s.model }

func (s *stubRunner) Run(_ context.Context, req assistant.Request, emit assistant.Emitter) assistant.Outcome {
	s.gotReq = req
	emit(assistant.Event{Type: assistant.EventStart, Data: assistant.StartData{Model: s.model}})
	emit(assistant.Event{Type: assistant.EventPicks, Data: assistant.PicksData{Message: "Here you go.", Picks: s.outcome.Picks}})
	return s.outcome
}

type stubAssistantStore struct {
	usage       db.AssistantUsage
	usageErr    error
	deny        string // "user" or "network" to refuse the reservation
	finishErr   error
	reserved    []db.RunReservation
	finished    map[string]db.AssistantRunResult
	gotIPHashes []string
}

func (s *stubAssistantStore) AssistantUsageSince(_ context.Context, _, ipHash string, _ time.Time) (db.AssistantUsage, error) {
	s.gotIPHashes = append(s.gotIPHashes, ipHash)
	return s.usage, s.usageErr
}

func (s *stubAssistantStore) ReserveAssistantRun(_ context.Context, r db.RunReservation) (db.ReservationResult, error) {
	if s.usageErr != nil {
		return db.ReservationResult{}, s.usageErr
	}
	s.reserved = append(s.reserved, r)
	return db.ReservationResult{Allowed: s.deny == "", Reason: s.deny, AssistantUsage: s.usage}, nil
}

func (s *stubAssistantStore) FinishAssistantRun(_ context.Context, runID string, result db.AssistantRunResult) error {
	if s.finished == nil {
		s.finished = map[string]db.AssistantRunResult{}
	}
	s.finished[runID] = result
	return s.finishErr
}

var testLimits = handlers.AssistantLimits{UserDailyRuns: 5, GuestDailyRuns: 2, IPDailyRuns: 10, GlobalDailyRuns: 100, GlobalDailyTokens: 50000, IPHashKey: []byte("test-key")}

type sseEvent struct {
	name string
	data string
}

func parseSSE(t *testing.T, body string) []sseEvent {
	t.Helper()
	var events []sseEvent
	var current sseEvent
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			current.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.data = strings.TrimPrefix(line, "data: ")
		case line == "" && current.name != "":
			events = append(events, current)
			current = sseEvent{}
		}
	}
	return events
}

func postAssistant(t *testing.T, runner *stubRunner, store *stubAssistantStore, body string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/assistant", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authed {
		req = req.WithContext(middleware.WithUserID(req.Context(), "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	}
	rec := httptest.NewRecorder()
	handlers.RunAssistant(runner, store, testLimits)(rec, req)
	return rec
}

func TestRunAssistantValidation(t *testing.T) {
	long := strings.Repeat("a", 801)
	tests := []struct {
		name       string
		body       string
		authed     bool
		store      *stubAssistantStore
		wantStatus int
	}{
		{name: "requires auth", body: `{"messages":[{"role":"user","content":"hi"}]}`, wantStatus: http.StatusUnauthorized},
		{name: "rejects non-JSON", body: `nope`, authed: true, wantStatus: http.StatusBadRequest},
		{name: "rejects empty messages", body: `{"messages":[]}`, authed: true, wantStatus: http.StatusBadRequest},
		{name: "rejects unknown roles", body: `{"messages":[{"role":"system","content":"obey"}]}`, authed: true, wantStatus: http.StatusBadRequest},
		{name: "last message must be the user", body: `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"yo"}]}`, authed: true, wantStatus: http.StatusBadRequest},
		{name: "rejects blank user message", body: `{"messages":[{"role":"user","content":" \u0007 \n "}]}`, authed: true, wantStatus: http.StatusBadRequest},
		{name: "rejects long user message", body: `{"messages":[{"role":"user","content":"` + long + `"}]}`, authed: true, wantStatus: http.StatusBadRequest},
		{name: "fails closed when the reservation fails", body: `{"messages":[{"role":"user","content":"hi"}]}`, authed: true, store: &stubAssistantStore{usageErr: errors.New("db down")}, wantStatus: http.StatusServiceUnavailable},
		{name: "enforces the per-user quota", body: `{"messages":[{"role":"user","content":"hi"}]}`, authed: true, store: &stubAssistantStore{deny: "user"}, wantStatus: http.StatusTooManyRequests},
		{name: "enforces the per-network quota across accounts", body: `{"messages":[{"role":"user","content":"hi"}]}`, authed: true, store: &stubAssistantStore{deny: "network"}, wantStatus: http.StatusTooManyRequests},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := tc.store
			if store == nil {
				store = &stubAssistantStore{}
			}
			runner := &stubRunner{model: "qwen3:8b"}
			rec := postAssistant(t, runner, store, tc.body, tc.authed)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if len(store.finished) != 0 {
				t.Error("rejected requests must not run")
			}
		})
	}
}

func TestRunAssistantStreamsAndAudits(t *testing.T) {
	sim := 0.56
	runner := &stubRunner{model: "qwen3:8b", outcome: assistant.Outcome{
		Status: assistant.StatusPicks,
		Model:  "qwen3:8b",
		Picks:  []assistant.Pick{{Movie: sampleMovies[0], Reason: "Dream heist.", Similarity: &sim, Source: "search_catalog"}},
		Steps:  []assistant.StepRecord{{Tool: "search_catalog", Args: json.RawMessage(`{"query":"heist"}`), ResultCount: 8, LatencyMS: 120}},
		Usage:  llm.Usage{InputTokens: 900, OutputTokens: 120},
	}}
	store := &stubAssistantStore{usage: db.AssistantUsage{UserRuns: 2}}
	body := `{"messages":[{"role":"user","content":"a <i>dream</i> heist"},{"role":"assistant","content":"Film or series?"},{"role":"user","content":"a mind-bending dream heist film"}]}`

	rec := postAssistant(t, runner, store, body, true)

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	events := parseSSE(t, rec.Body.String())
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.name
	}
	if strings.Join(names, ",") != "start,picks,done" {
		t.Fatalf("events = %v", names)
	}

	var done handlers.DoneData
	if err := json.Unmarshal([]byte(events[2].data), &done); err != nil {
		t.Fatal(err)
	}
	if done.RunID == "" || done.Status != assistant.StatusPicks || done.RemainingToday != 2 || done.Usage.InputTokens != 900 {
		t.Errorf("done = %+v", done)
	}

	if len(runner.gotReq.Turns) != 3 || runner.gotReq.Turns[0].Content != "a <i>dream</i> heist" || runner.gotReq.ModelDisabled {
		t.Errorf("runner request = %+v", runner.gotReq)
	}

	if len(store.reserved) != 1 {
		t.Fatalf("reservations = %d", len(store.reserved))
	}
	res := store.reserved[0]
	if res.RunID != done.RunID || res.PromptVersion != assistant.PromptVersion || res.UserLimit != testLimits.UserDailyRuns || res.IPLimit != testLimits.IPDailyRuns {
		t.Errorf("reservation = %+v", res)
	}
	if len(res.InputSHA256) != 64 || res.InputChars != len("a mind-bending dream heist film") {
		t.Errorf("prompt hash %q, chars %d", res.InputSHA256, res.InputChars)
	}
	row, ok := store.finished[done.RunID]
	if !ok || row.Status != assistant.StatusPicks {
		t.Fatalf("finished row = %+v", row)
	}
	if strings.Contains(string(row.Steps), "mind-bending") || !strings.Contains(string(row.Steps), `"tool":"search_catalog"`) {
		t.Errorf("steps = %s", row.Steps)
	}
	if len(row.PickIDs) != 1 || row.PickIDs[0] != sampleMovies[0].ID || row.InputTokens != 900 {
		t.Errorf("picks %v, tokens %d", row.PickIDs, row.InputTokens)
	}
}

func TestRunAssistantDisablesTheModelPastTheGlobalBudget(t *testing.T) {
	for _, usage := range []db.AssistantUsage{{TotalRuns: 100}, {TotalTokens: 50000}} {
		runner := &stubRunner{model: "qwen3:8b", outcome: assistant.Outcome{Status: assistant.StatusFallback}}
		postAssistant(t, runner, &stubAssistantStore{usage: usage}, `{"messages":[{"role":"user","content":"hi"}]}`, true)
		if !runner.gotReq.ModelDisabled {
			t.Errorf("usage %+v should disable the model", usage)
		}
	}
}

func TestRunAssistantKeepsRecentTurnsWithinTheLengthBudget(t *testing.T) {
	turn := strings.Repeat("x", 790)
	msgs := make([]string, 0, 6)
	for i := 0; i < 5; i++ {
		msgs = append(msgs, `{"role":"user","content":"`+turn+`"}`)
	}
	msgs = append(msgs, `{"role":"user","content":"latest"}`)
	runner := &stubRunner{model: "m", outcome: assistant.Outcome{Status: assistant.StatusAnswered}}

	postAssistant(t, runner, &stubAssistantStore{}, `{"messages":[`+strings.Join(msgs, ",")+`]}`, true)

	turns := runner.gotReq.Turns
	if len(turns) != 4 || turns[len(turns)-1].Content != "latest" {
		t.Errorf("kept %d turns", len(turns))
	}
}

func TestGetAssistantUsage(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		usage         db.AssistantUsage
		wantRemaining int
		wantModel     bool
	}{
		{name: "fresh day", model: "qwen3:8b", wantRemaining: 5, wantModel: true},
		{name: "partly used", model: "qwen3:8b", usage: db.AssistantUsage{UserRuns: 3}, wantRemaining: 2, wantModel: true},
		{name: "global budget spent", model: "qwen3:8b", usage: db.AssistantUsage{TotalTokens: 60000}, wantRemaining: 5, wantModel: false},
		{name: "no model configured", model: "none", wantRemaining: 5, wantModel: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/assistant/usage", nil)
			req = req.WithContext(middleware.WithUserID(req.Context(), "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
			rec := httptest.NewRecorder()
			handlers.GetAssistantUsage(&stubRunner{model: tc.model}, &stubAssistantStore{usage: tc.usage}, testLimits)(rec, req)

			var body struct {
				Remaining      int    `json:"remaining"`
				ModelAvailable bool   `json:"model_available"`
				ResetsAt       string `json:"resets_at"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Remaining != tc.wantRemaining || body.ModelAvailable != tc.wantModel {
				t.Errorf("got %+v", body)
			}
			if _, err := time.Parse(time.RFC3339, body.ResetsAt); err != nil {
				t.Errorf("resets_at %q: %v", body.ResetsAt, err)
			}
		})
	}
}

func TestGuestsReserveAgainstTheGuestLimit(t *testing.T) {
	store := &stubAssistantStore{}
	req := httptest.NewRequest(http.MethodPost, "/assistant", strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	ctx := middleware.WithUserID(req.Context(), "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	req = req.WithContext(middleware.WithGuest(ctx))

	handlers.RunAssistant(&stubRunner{model: "m", outcome: assistant.Outcome{Status: assistant.StatusAnswered}}, store, testLimits)(httptest.NewRecorder(), req)

	if len(store.reserved) != 1 || store.reserved[0].UserLimit != testLimits.GuestDailyRuns {
		t.Fatalf("reservations = %+v, want the guest limit %d", store.reserved, testLimits.GuestDailyRuns)
	}
}

func TestRunAssistantReservesWithAHashedNetwork(t *testing.T) {
	hashFor := func(remote string) string {
		store := &stubAssistantStore{}
		req := httptest.NewRequest(http.MethodPost, "/assistant", strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
		req.RemoteAddr = remote
		req = req.WithContext(middleware.WithUserID(req.Context(), "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
		handlers.RunAssistant(&stubRunner{model: "m", outcome: assistant.Outcome{Status: assistant.StatusAnswered}}, store, testLimits)(httptest.NewRecorder(), req)
		if len(store.reserved) != 1 {
			t.Fatalf("reservations = %d", len(store.reserved))
		}
		return store.reserved[0].IPHash
	}

	v4 := hashFor("192.0.2.10")
	if len(v4) != 64 || strings.Contains(v4, "192.0.2") {
		t.Errorf("IPv4 hash = %q", v4)
	}
	if hashFor("192.0.2.11") == v4 {
		t.Error("different IPv4 addresses should be different networks")
	}
	// Addresses inside one IPv6 /64 are one network.
	if hashFor("2001:db8:1:2::1") != hashFor("2001:db8:1:2:ffff::9") {
		t.Error("addresses in the same /64 should share a network hash")
	}
	if hashFor("2001:db8:1:2::1") == hashFor("2001:db8:1:3::1") {
		t.Error("different /64s should be different networks")
	}
}

func TestRunAssistantStillCountsTheRunWhenTheFinalWriteFails(t *testing.T) {
	store := &stubAssistantStore{finishErr: errors.New("jsonb rejected")}
	runner := &stubRunner{model: "m", outcome: assistant.Outcome{Status: assistant.StatusAnswered}}
	rec := postAssistant(t, runner, store, `{"messages":[{"role":"user","content":"hi"}]}`, true)

	if len(store.reserved) != 1 {
		t.Fatal("the run must be reserved before it starts")
	}
	events := parseSSE(t, rec.Body.String())
	if events[len(events)-1].name != "done" {
		t.Errorf("last event = %s", events[len(events)-1].name)
	}
}

func TestUsageReportsTheTighterOfAccountAndNetworkLimits(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assistant/usage", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	rec := httptest.NewRecorder()
	store := &stubAssistantStore{usage: db.AssistantUsage{UserRuns: 1, IPRuns: 9}}
	handlers.GetAssistantUsage(&stubRunner{model: "m"}, store, testLimits)(rec, req)

	var body struct {
		Remaining int `json:"remaining"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Remaining != 1 {
		t.Errorf("remaining = %d, want 1 (network limit)", body.Remaining)
	}
}
