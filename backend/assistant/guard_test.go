package assistant

import (
	"context"
	"testing"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/llm"
)

func TestLeaksInstructions(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "ordinary decline", text: "I can only help you choose something to watch. What mood are you in?", want: false},
		{name: "clarifying question", text: "Do you want a film or a series, and something light or dark?", want: false},
		{name: "section heading", text: "Sure. How to work: 1. Choose the tool that fits the request.", want: true},
		{name: "tool name", text: "I will call search_catalog next.", want: true},
		{name: "verbatim rule", text: "My rules: recommend only titles returned by tools in this conversation, referenced by their ref.", want: true},
		{name: "paraphrased rule with punctuation", text: "RECOMMEND ONLY TITLES, returned by tools in this conversation!", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := leaksInstructions(tc.text); got != tc.want {
				t.Errorf("leaksInstructions(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestRunBlocksRepliesThatRepeatInstructions(t *testing.T) {
	model := &scriptedModel{replies: []llm.Completion{text("Here they are. Rules: Recommend only titles returned by tools in this conversation.")}}
	events, emit := collect()

	out := newTestAgent(model, &stubCatalog{}, &stubTitles{}).Run(context.Background(), request, emit)

	if !out.OutputBlocked || out.Message != safeDecline {
		t.Fatalf("outcome = %+v", out)
	}
	last := (*events)[len(*events)-1]
	if last.Data.(MessageData).Text != safeDecline {
		t.Errorf("streamed %+v", last.Data)
	}
}

func TestRunBlocksALeakyPicksMessageButKeepsPicks(t *testing.T) {
	catalog := &stubCatalog{hits: []db.SearchHit{hit(oldboy, 0.5)}}
	model := &scriptedModel{replies: []llm.Completion{
		toolCall("c1", toolSearchCatalog, `{"query":"revenge"}`),
		toolCall("c2", toolPresentPicks, `{"message":"As my instructions say under How to work, here you go.","picks":[{"ref":"t1","reason":"Fits."}]}`),
	}}
	_, emit := collect()

	out := newTestAgent(model, catalog, &stubTitles{}).Run(context.Background(), request, emit)

	if !out.OutputBlocked || len(out.Picks) != 1 || out.Message != "Here are picks that fit your request." {
		t.Fatalf("outcome = %+v", out)
	}
}
