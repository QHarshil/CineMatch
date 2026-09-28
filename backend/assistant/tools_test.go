package assistant

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestResolveGenres(t *testing.T) {
	tests := []struct {
		name      string
		requested []string
		mediaType string
		want      []string
		wantNotes int
	}{
		{name: "film vocabulary", requested: []string{"sci-fi", "Thriller"}, mediaType: "movie", want: []string{"Science Fiction", "Thriller"}},
		{name: "series vocabulary", requested: []string{"sci-fi", "action"}, mediaType: "tv", want: []string{"Sci-Fi & Fantasy", "Action & Adventure"}},
		{name: "both vocabularies", requested: []string{"fantasy"}, want: []string{"Fantasy", "Sci-Fi & Fantasy"}},
		{name: "series have no thriller genre", requested: []string{"thriller", "crime"}, mediaType: "tv", want: []string{"Crime"}, wantNotes: 1},
		{name: "unknown words are noted", requested: []string{"slow burn"}, wantNotes: 1},
		{name: "duplicates collapse", requested: []string{"sci-fi", "science fiction"}, mediaType: "movie", want: []string{"Science Fiction"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, notes := resolveGenres(tc.requested, tc.mediaType)
			if !reflect.DeepEqual(got, tc.want) || len(notes) != tc.wantNotes {
				t.Errorf("got %v with %d notes, want %v with %d", got, len(notes), tc.want, tc.wantNotes)
			}
		})
	}
}

func TestSearchArgsFilters(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{name: "quoted numbers and names", args: `{"query":"x","media_type":"series","min_year":"2015","min_rating":"7.5","language":"Korean"}`, want: `{"MediaType":"tv","Genres":null,"MinYear":2015,"MaxYear":0,"MinRating":7.5,"MaxRuntime":0,"Language":"ko","ExcludeIDs":null}`},
		{name: "out of range values dropped", args: `{"query":"x","min_year":1200,"min_rating":42,"max_runtime":-5,"language":"klingon"}`, want: `{"MediaType":"","Genres":null,"MinYear":0,"MaxYear":0,"MinRating":0,"MaxRuntime":0,"Language":"","ExcludeIDs":null}`},
		{name: "inverted years swap", args: `{"query":"x","min_year":2020,"max_year":2010,"max_runtime":120}`, want: `{"MediaType":"","Genres":null,"MinYear":2010,"MaxYear":2020,"MinRating":0,"MaxRuntime":120,"Language":"","ExcludeIDs":null}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args searchArgs
			if err := json.Unmarshal([]byte(tc.args), &args); err != nil {
				t.Fatal(err)
			}
			f, _ := args.filters()
			got, _ := json.Marshal(f)
			if string(got) != tc.want {
				t.Errorf("filters = %s\nwant      %s", got, tc.want)
			}
		})
	}
}

func TestSearchArgsLimit(t *testing.T) {
	for raw, want := range map[string]int{`{}`: defaultSearchLimit, `{"limit":3}`: 3, `{"limit":"50"}`: maxSearchLimit} {
		var args searchArgs
		_ = json.Unmarshal([]byte(raw), &args)
		if got := args.limit(); got != want {
			t.Errorf("%s: limit = %d, want %d", raw, got, want)
		}
	}
}

func TestLabelFor(t *testing.T) {
	got := labelFor(toolSearchCatalog, `{"query":"space survival","media_type":"movie","genres":["sci-fi"],"min_year":2010,"max_runtime":130}`)
	want := `Searching for "space survival" · films · Science Fiction · 2010 or later · up to 130 min`
	if got != want {
		t.Errorf("label = %q\nwant    %q", got, want)
	}
	if labelFor(toolTasteProfile, `{}`) != "Reading your recent likes" {
		t.Error("taste profile label")
	}
}

func TestRedactForAudit(t *testing.T) {
	got := string(redactForAudit(`{"query":"films for jane.doe@example.com, call +1 (416) 555-0199"}`))
	if strings.Contains(got, "jane") || strings.Contains(got, "555") || !strings.Contains(got, "[email]") || !strings.Contains(got, "[phone]") {
		t.Errorf("redacted = %s", got)
	}
	if string(redactForAudit(`not json`)) != `{}` {
		t.Error("invalid JSON should become {}")
	}
	if got := string(redactForAudit(`{"query":"heist\u0000film","genres":["Crime\u0000"]}`)); got != `{"genres":["Crime"],"query":"heistfilm"}` {
		t.Errorf("NUL not stripped: %s", got)
	}
}

func TestCleanText(t *testing.T) {
	if got := cleanText("like **Parasite** and *Mother*, or M*A*S*H", 100); got != "like Parasite and Mother, or M*A*S*H" {
		t.Errorf("emphasis: got %q", got)
	}
	if got := cleanText("  two\nlines\x00 here  ", 100); got != "two lines here" {
		t.Errorf("got %q", got)
	}
	if got := cleanText(strings.Repeat("a", 50), 10); len([]rune(got)) != 10 || !strings.HasSuffix(got, "…") {
		t.Errorf("truncated to %q", got)
	}
}

func TestTopKeys(t *testing.T) {
	got := topKeys(map[string]int{"Drama": 3, "Crime": 3, "Comedy": 1, "Thriller": 2}, 3)
	if !reflect.DeepEqual(got, []string{"Crime", "Drama", "Thriller"}) {
		t.Errorf("got %v", got)
	}
}
