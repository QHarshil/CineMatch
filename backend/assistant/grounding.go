package assistant

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/harshilc/cinematch-backend/db"
)

const (
	maxPicks        = 6
	maxReasonLength = 240
)

// groundedTitle is a catalog title a tool returned during this run.
type groundedTitle struct {
	ref        string
	movie      db.Movie
	similarity *float64
	source     string // tool that first surfaced the title
	seedOnly   bool   // a reference title (liked, or named by the person); never recommended back
	order      int
}

// groundingSet tracks every title tools returned in a run. The model refers
// to titles by short refs ("t3"): small models copy those reliably where they
// garble 36-character UUIDs, and a ref can only exist if a tool produced it.
type groundingSet struct {
	byRef map[string]*groundedTitle
	refOf map[string]string // movie ID -> ref
}

func newGroundingSet() *groundingSet {
	return &groundingSet{byRef: map[string]*groundedTitle{}, refOf: map[string]string{}}
}

// add registers a title and returns its ref. A title seen again keeps its
// ref, and keeps the higher similarity when both are known.
func (g *groundingSet) add(m db.Movie, similarity *float64, source string) string {
	if ref, ok := g.refOf[m.ID]; ok {
		t := g.byRef[ref]
		if similarity != nil && (t.similarity == nil || *similarity > *t.similarity) {
			t.similarity = similarity
		}
		return ref
	}
	ref := fmt.Sprintf("t%d", len(g.byRef)+1)
	g.byRef[ref] = &groundedTitle{ref: ref, movie: m, similarity: similarity, source: source, order: len(g.byRef)}
	g.refOf[m.ID] = ref
	return ref
}

func (g *groundingSet) markSeedOnly(ref string) {
	if t, ok := g.byRef[ref]; ok {
		t.seedOnly = true
	}
}

func (g *groundingSet) lookup(ref string) (*groundedTitle, bool) {
	t, ok := g.byRef[strings.TrimSpace(strings.ToLower(ref))]
	return t, ok
}

func (g *groundingSet) size() int { return len(g.byRef) }

// candidates returns recommendable titles in the order tools surfaced them.
func (g *groundingSet) candidates() []*groundedTitle {
	out := make([]*groundedTitle, 0, len(g.byRef))
	for i := 0; i < len(g.byRef); i++ {
		t := g.byRef[fmt.Sprintf("t%d", i+1)]
		if t != nil && !t.seedOnly {
			out = append(out, t)
		}
	}
	return out
}

type presentPicksArgs struct {
	Message string `json:"message"`
	Picks   []struct {
		Ref    string `json:"ref"`
		Reason string `json:"reason"`
	} `json:"picks"`
}

// resolvePicks validates a present_picks call. Every pick must reference a
// title a tool returned; anything else is dropped and counted. Titles the
// person already liked and duplicates are skipped without counting as
// ungrounded.
func (g *groundingSet) resolvePicks(arguments string) (message string, picks []Pick, ungrounded int, problem string) {
	var args presentPicksArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", nil, 0, "present_picks arguments were not valid JSON"
	}
	used := map[string]bool{}
	for _, p := range args.Picks {
		t, ok := g.lookup(p.Ref)
		if !ok {
			ungrounded++
			continue
		}
		if t.seedOnly || used[t.ref] || len(picks) == maxPicks {
			continue
		}
		used[t.ref] = true
		reason := cleanText(p.Reason, maxReasonLength)
		if reason == "" {
			reason = describeTitle(t.movie)
		}
		picks = append(picks, Pick{Movie: t.movie, Reason: reason, Similarity: t.similarity, Source: t.source})
	}
	if len(picks) == 0 {
		return "", nil, ungrounded, "none of those refs came from a tool result; call a tool, then present refs it returned"
	}
	return cleanText(args.Message, 400), picks, ungrounded, ""
}

// describeTitle writes a factual one-line reason from catalog metadata, used
// when the model gave none and for fallback results.
func describeTitle(m db.Movie) string {
	kind := "film"
	if m.MediaType == "tv" {
		kind = "series"
	}
	var b strings.Builder
	if len(m.Genres) > 0 {
		genres := m.Genres
		if len(genres) > 2 {
			genres = genres[:2]
		}
		b.WriteString(strings.Join(genres, " and "))
		b.WriteString(" " + kind)
	} else {
		b.WriteString(strings.ToUpper(kind[:1]) + kind[1:])
	}
	if m.ReleaseYear > 0 {
		fmt.Fprintf(&b, " from %d", m.ReleaseYear)
	}
	if m.VoteAverage > 0 {
		fmt.Fprintf(&b, ", rated %.1f", m.VoteAverage)
	}
	b.WriteString(".")
	return b.String()
}

// Models add bold and italic markers even when asked for plain text.
var (
	markdownBold   = strings.NewReplacer("**", "", "__", "")
	markdownItalic = regexp.MustCompile(`\*(\S(?:[^*]*\S)?)\*`)
)

// stripItalics removes *word* markers that sit on word boundaries, so a
// title like M*A*S*H survives.
func stripItalics(s string) string {
	var b strings.Builder
	last := 0
	for _, m := range markdownItalic.FindAllStringSubmatchIndex(s, -1) {
		start, end := m[0], m[1]
		if (start > 0 && isWordByte(s[start-1])) || (end < len(s) && isWordByte(s[end])) {
			continue
		}
		b.WriteString(s[last:start])
		b.WriteString(s[m[2]:m[3]])
		last = end
	}
	b.WriteString(s[last:])
	return b.String()
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// cleanText trims model-written text, drops control characters and emphasis
// markers, and caps its length. The frontend renders it as plain text, never
// as HTML.
func cleanText(s string, limit int) string {
	s = stripItalics(markdownBold.Replace(s))
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > limit {
		s = strings.TrimSpace(string(runes[:limit-1])) + "…"
	}
	return s
}
