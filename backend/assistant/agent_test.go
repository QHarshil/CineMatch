package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/llm"
	"github.com/harshilc/cinematch-backend/search"
)

// scriptedModel replays completions in order and records what it was sent.
type scriptedModel struct {
	replies  []llm.Completion
	errs     []error
	received [][]llm.Message
}

func (m *scriptedModel) Model() string { return "scripted" }

func (m *scriptedModel) Complete(_ context.Context, messages []llm.Message, _ []llm.Tool) (llm.Completion, error) {
	i := len(m.received)
	m.received = append(m.received, append([]llm.Message(nil), messages...))
	if i < len(m.errs) && m.errs[i] != nil {
		return llm.Completion{}, m.errs[i]
	}
	if i >= len(m.replies) {
		return llm.Completion{}, errors.New("script exhausted")
	}
	return m.replies[i], nil
}

func toolCall(id, name, args string) llm.Completion {
	return llm.Completion{
		Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID: id, Type: "function", Function: llm.FunctionCall{Name: name, Arguments: args},
		}}},
		Usage: llm.Usage{InputTokens: 100, OutputTokens: 20},
	}
}

func text(s string) llm.Completion {
	return llm.Completion{Message: llm.Message{Role: llm.RoleAssistant, Content: s}, Usage: llm.Usage{InputTokens: 80, OutputTokens: 10}}
}

type stubCatalog struct {
	hits    []db.SearchHit
	err     error
	queries []string
	filters []db.SearchFilters
}

func (c *stubCatalog) Search(_ context.Context, q string, _ int, f db.SearchFilters) (search.Result, error) {
	c.queries = append(c.queries, q)
	c.filters = append(c.filters, f)
	return search.Result{Hits: c.hits, Retrieval: search.RetrievalHybrid}, c.err
}

type stubTitles struct {
	similar        []db.SearchHit
	liked          []db.Movie
	named          []db.Movie
	similarFilters db.SearchFilters
}

func (s *stubTitles) TitlesNamed(context.Context, string) ([]db.Movie, error) {
	return s.named, nil
}

func (s *stubTitles) SimilarToTitle(_ context.Context, _ string, _ int, f db.SearchFilters) ([]db.SearchHit, error) {
	s.similarFilters = f
	return s.similar, nil
}

func (s *stubTitles) RecentPositiveTitles(context.Context, string, int) ([]db.Movie, error) {
	return s.liked, nil
}

var (
	oldboy       = db.Movie{ID: "11111111-0000-4000-8000-000000000001", Title: "Oldboy", MediaType: "movie", Genres: []string{"Drama", "Thriller"}, ReleaseYear: 2003, VoteAverage: 8.3}
	iSawTheDevil = db.Movie{ID: "11111111-0000-4000-8000-000000000002", Title: "I Saw the Devil", MediaType: "movie", Genres: []string{"Thriller", "Horror"}, ReleaseYear: 2010, VoteAverage: 8.0}
	mother       = db.Movie{ID: "11111111-0000-4000-8000-000000000003", Title: "Mother", MediaType: "movie", Genres: []string{"Crime", "Drama"}, ReleaseYear: 2009, VoteAverage: 7.8}
)

func hit(m db.Movie, sim float64) db.SearchHit { return db.SearchHit{Movie: m, Similarity: &sim} }

func newTestAgent(model ChatModel, catalog *stubCatalog, titles *stubTitles) *Agent {
	recs := RecommenderFunc(func(context.Context, string) ([]db.Movie, string, error) {
		return []db.Movie{mother}, "personalized", nil
	})
	a := New(model, catalog, titles, recs)
	a.now = func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) }
	return a
}

func eventTypes(events []Event) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}

func collect() (*[]Event, Emitter) {
	var events []Event
	return &events, func(e Event) { events = append(events, e) }
}

var request = Request{UserID: "user-1", Turns: []Turn{{Role: "user", Content: "a korean revenge thriller"}}}

func TestRunPresentsGroundedPicks(t *testing.T) {
	catalog := &stubCatalog{hits: []db.SearchHit{hit(oldboy, 0.56), hit(iSawTheDevil, 0.53)}}
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolSearchCatalog, `{"query":"revenge thriller","language":"korean","genres":"thriller","min_rating":"7"}`),
		toolCall("c2", toolPresentPicks, `{"message":"Two Korean revenge classics.","picks":[{"ref":"t1","reason":"A man hunts the captor who jailed him."},{"ref":"t2","reason":"An agent hunts a serial killer."}]}`),
	}}
	events, emit := collect()

	out := newTestAgent(model, catalog, &stubTitles{}).Run(context.Background(), request, emit)

	if out.Status != StatusPicks || len(out.Picks) != 2 {
		t.Fatalf("status %q with %d picks", out.Status, len(out.Picks))
	}
	if got := strings.Join(eventTypes(*events), ","); got != "start,tool_call,tool_result,picks" {
		t.Errorf("events = %s", got)
	}
	if out.Picks[0].Movie.Title != "Oldboy" || *out.Picks[0].Similarity != 0.56 || out.Picks[0].Source != toolSearchCatalog {
		t.Errorf("first pick = %+v", out.Picks[0])
	}
	f := catalog.filters[0]
	if f.Language != "ko" || len(f.Genres) != 1 || f.Genres[0] != "Thriller" || f.MinRating != 7 {
		t.Errorf("filters = %+v", f)
	}
	if out.Usage.InputTokens != 200 || out.Usage.OutputTokens != 40 {
		t.Errorf("usage = %+v", out.Usage)
	}
	if len(out.Steps) != 1 || out.Steps[0].ResultCount != 2 {
		t.Errorf("steps = %+v", out.Steps)
	}
	// The tool result sent back to the model carries refs, not UUIDs.
	toolMsg := model.received[1][len(model.received[1])-1]
	if toolMsg.Role != llm.RoleTool || !strings.Contains(toolMsg.Content, `"ref":"t1"`) || strings.Contains(toolMsg.Content, oldboy.ID) {
		t.Errorf("tool message = %s", toolMsg.Content)
	}
}

func TestRunDropsUngroundedPicks(t *testing.T) {
	catalog := &stubCatalog{hits: []db.SearchHit{hit(oldboy, 0.56)}}
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolSearchCatalog, `{"query":"revenge"}`),
		toolCall("c2", toolPresentPicks, `{"message":"Picks.","picks":[{"ref":"t1","reason":"Fits."},{"ref":"t9","reason":"Invented."}]}`),
	}}
	_, emit := collect()

	out := newTestAgent(model, catalog, &stubTitles{}).Run(context.Background(), request, emit)

	if len(out.Picks) != 1 || out.UngroundedDropped != 1 {
		t.Fatalf("picks %d, dropped %d", len(out.Picks), out.UngroundedDropped)
	}
}

func TestRunLetsTheModelRetryAfterAllPicksAreUngrounded(t *testing.T) {
	catalog := &stubCatalog{hits: []db.SearchHit{hit(oldboy, 0.56)}}
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolSearchCatalog, `{"query":"revenge"}`),
		toolCall("c2", toolPresentPicks, `{"message":"Picks.","picks":[{"ref":"t7","reason":"Invented."}]}`),
		toolCall("c3", toolPresentPicks, `{"message":"Picks.","picks":[{"ref":"t1","reason":"Grounded."}]}`),
	}}
	_, emit := collect()

	out := newTestAgent(model, catalog, &stubTitles{}).Run(context.Background(), request, emit)

	if out.Status != StatusPicks || len(out.Picks) != 1 || out.UngroundedDropped != 1 {
		t.Fatalf("status %q, picks %d, dropped %d", out.Status, len(out.Picks), out.UngroundedDropped)
	}
	retryInput := model.received[2]
	if last := retryInput[len(retryInput)-1]; !strings.Contains(last.Content, "none of those refs") {
		t.Errorf("model was not told why: %s", last.Content)
	}
}

func TestRunNeverRecommendsLikedTitles(t *testing.T) {
	titles := &stubTitles{
		liked:   []db.Movie{oldboy},
		similar: []db.SearchHit{hit(iSawTheDevil, 0.61)},
	}
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolTasteProfile, `{}`),
		toolCall("c2", toolFindSimilar, `{"ref":"t1","media_type":"movie"}`),
		toolCall("c3", toolPresentPicks, `{"message":"Based on Oldboy.","picks":[{"ref":"t1","reason":"Liked."},{"ref":"t2","reason":"Similar."}]}`),
	}}
	_, emit := collect()

	out := newTestAgent(model, &stubCatalog{}, titles).Run(context.Background(), request, emit)

	if len(out.Picks) != 1 || out.Picks[0].Movie.Title != "I Saw the Devil" || out.UngroundedDropped != 0 {
		t.Fatalf("picks = %+v, dropped %d", out.Picks, out.UngroundedDropped)
	}
	if out.Picks[0].Source != toolFindSimilar {
		t.Errorf("source = %s", out.Picks[0].Source)
	}
	if titles.similarFilters.MediaType != "movie" {
		t.Errorf("find_similar filters = %+v", titles.similarFilters)
	}
}

func TestRunReturnsTextReplies(t *testing.T) {
	model := &scriptedModel{replies: []llm.Completion{text("Do you want a film or a series?")}}
	events, emit := collect()

	out := newTestAgent(model, &stubCatalog{}, &stubTitles{}).Run(context.Background(), request, emit)

	if out.Status != StatusAnswered || out.Message != "Do you want a film or a series?" {
		t.Fatalf("outcome = %+v", out)
	}
	if got := strings.Join(eventTypes(*events), ","); got != "start,message" {
		t.Errorf("events = %s", got)
	}
}

func TestRunFallsBackToSearch(t *testing.T) {
	tests := []struct {
		name       string
		model      ChatModel
		disabled   bool
		wantNotice string
		wantStart  bool
	}{
		{name: "no model configured", model: nil, wantNotice: noticeOffline},
		{name: "budget spent", model: &scriptedModel{}, disabled: true, wantNotice: noticeBudget},
		{name: "provider rate limit", model: &scriptedModel{errs: []error{llm.ErrRateLimited}}, wantNotice: noticeBusy, wantStart: true},
		{name: "provider error", model: &scriptedModel{errs: []error{errors.New("connection refused")}}, wantNotice: noticeOffline, wantStart: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			catalog := &stubCatalog{hits: []db.SearchHit{hit(oldboy, 0.5), hit(mother, 0.4)}}
			events, emit := collect()
			req := request
			req.ModelDisabled = tc.disabled

			out := newTestAgent(tc.model, catalog, &stubTitles{}).Run(context.Background(), req, emit)

			if out.Status != StatusFallback || out.Message != tc.wantNotice || len(out.Picks) != 2 {
				t.Fatalf("outcome = %+v", out)
			}
			if catalog.queries[0] != "a korean revenge thriller" {
				t.Errorf("fallback query = %q", catalog.queries[0])
			}
			if out.Picks[0].Reason != "Drama and Thriller film from 2003, rated 8.3." {
				t.Errorf("reason = %q", out.Picks[0].Reason)
			}
			hasStart := len(*events) > 0 && (*events)[0].Type == EventStart
			if hasStart != tc.wantStart {
				t.Errorf("start event = %v, want %v", hasStart, tc.wantStart)
			}
		})
	}
}

func TestRunPresentsWhatToolsFoundWhenCallsRunOut(t *testing.T) {
	catalog := &stubCatalog{hits: []db.SearchHit{hit(oldboy, 0.5)}}
	replies := make([]llm.Completion, maxModelCalls)
	for i := range replies {
		replies[i] = toolCall("c", toolSearchCatalog, `{"query":"revenge"}`)
	}
	_, emit := collect()

	out := newTestAgent(&scriptedModel{replies: replies}, catalog, &stubTitles{}).Run(context.Background(), request, emit)

	if out.Status != StatusFallback || len(out.Picks) != 1 || len(out.Steps) != maxModelCalls {
		t.Fatalf("status %q, picks %d, steps %d", out.Status, len(out.Picks), len(out.Steps))
	}
}

func TestRunReportsToolErrorsToTheModel(t *testing.T) {
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolFindSimilar, `{"ref":"t5"}`),
		text("Which title did you have in mind?"),
	}}
	events, emit := collect()

	out := newTestAgent(model, &stubCatalog{}, &stubTitles{}).Run(context.Background(), request, emit)

	if out.Steps[0].Error == "" {
		t.Fatal("expected the step to record an error")
	}
	var result ToolResultData
	for _, e := range *events {
		if e.Type == EventToolResult {
			result = e.Data.(ToolResultData)
		}
	}
	if !strings.Contains(result.Error, "unknown ref") {
		t.Errorf("tool_result error = %q", result.Error)
	}
}

func TestRunReportsEmptyCatalog(t *testing.T) {
	events, emit := collect()
	out := newTestAgent(nil, &stubCatalog{}, &stubTitles{}).Run(context.Background(), request, emit)

	if out.Status != StatusError || (*events)[0].Type != EventError {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestConversationKeepsRecentTurnsAfterInstructions(t *testing.T) {
	turns := make([]Turn, 12)
	for i := range turns {
		turns[i] = Turn{Role: "user", Content: string(rune('a' + i))}
	}
	turns[10].Role = "assistant"

	msgs := newTestAgent(nil, &stubCatalog{}, &stubTitles{}).conversation(turns)

	if len(msgs) != historyTurns+1 || msgs[0].Role != llm.RoleSystem {
		t.Fatalf("got %d messages", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "September 28, 2026") {
		t.Error("system prompt should carry today's date")
	}
	if msgs[1].Content != "e" || msgs[len(msgs)-2].Role != llm.RoleAssistant {
		t.Errorf("unexpected window: first %q", msgs[1].Content)
	}
}

func TestEventsEncodeAsJSON(t *testing.T) {
	sim := 0.5
	data := PicksData{Message: "m", Picks: []Pick{{Movie: oldboy, Reason: "r", Similarity: &sim, Source: toolSearchCatalog}}}
	b, err := json.Marshal(data)
	if err != nil || !strings.Contains(string(b), `"similarity":0.5`) {
		t.Fatalf("encoded %s, %v", b, err)
	}
}

func TestRunSeedsFindSimilarWithANamedTitleOutsideTheFilters(t *testing.T) {
	parasite := db.Movie{ID: "11111111-0000-4000-8000-000000000009", Title: "Parasite", MediaType: "movie", Genres: []string{"Drama"}}
	series := db.Movie{ID: "11111111-0000-4000-8000-000000000010", Title: "Squid Game", MediaType: "tv"}
	catalog := &stubCatalog{hits: []db.SearchHit{hit(iSawTheDevil, 0.3)}}
	titles := &stubTitles{named: []db.Movie{parasite}, similar: []db.SearchHit{hit(series, 0.52)}}
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolSearchCatalog, `{"query":"Parasite","media_type":"tv"}`),
		toolCall("c2", toolFindSimilar, `{"ref":"t2","media_type":"tv"}`),
		toolCall("c3", toolPresentPicks, `{"message":"Series like Parasite.","picks":[{"ref":"t2","reason":"Seed."},{"ref":"t3","reason":"Class satire."}]}`),
	}}
	_, emit := collect()

	out := newTestAgent(model, catalog, titles).Run(context.Background(), request, emit)

	searchResult := model.received[1][len(model.received[1])-1].Content
	if !strings.Contains(searchResult, `"named_titles":[{"ref":"t2","title":"Parasite"`) {
		t.Fatalf("search result = %s", searchResult)
	}
	if len(out.Picks) != 1 || out.Picks[0].Movie.Title != "Squid Game" {
		t.Errorf("picks = %+v", out.Picks)
	}
}

func TestRunNudgesTextAnswersAfterToolsIntoPresentPicks(t *testing.T) {
	catalog := &stubCatalog{hits: []db.SearchHit{hit(oldboy, 0.5), hit(mother, 0.4)}}
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolSearchCatalog, `{"query":"revenge"}`),
		text("Try **Mother** and Oldboy."),
		toolCall("c2", toolPresentPicks, `{"message":"Two picks.","picks":[{"ref":"t2","reason":"A mother hunts the truth."}]}`),
	}}
	_, emit := collect()

	out := newTestAgent(model, catalog, &stubTitles{}).Run(context.Background(), request, emit)

	if out.Status != StatusPicks || out.Picks[0].Movie.Title != "Mother" {
		t.Fatalf("outcome = %+v", out)
	}
	nudge := model.received[2][len(model.received[2])-1]
	if nudge.Role != llm.RoleUser || nudge.Content != finishNudge {
		t.Errorf("nudge = %+v", nudge)
	}
}

func TestRunPresentsMentionedTitlesWhenTheModelKeepsAnsweringInText(t *testing.T) {
	catalog := &stubCatalog{hits: []db.SearchHit{hit(oldboy, 0.5), hit(mother, 0.4)}}
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolSearchCatalog, `{"query":"revenge"}`),
		text("Watch Mother."),
		text("Really, watch Mother."),
	}}
	_, emit := collect()

	out := newTestAgent(model, catalog, &stubTitles{}).Run(context.Background(), request, emit)

	if out.Status != StatusFallback || out.Picks[0].Movie.Title != "Mother" || len(out.Picks) != 2 {
		t.Fatalf("outcome = %+v", out)
	}
}
