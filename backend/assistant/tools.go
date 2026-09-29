package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/harshilc/cinematch-backend/db"
	"github.com/harshilc/cinematch-backend/llm"
)

// Tool names.
const (
	toolSearchCatalog      = "search_catalog"
	toolFindSimilar        = "find_similar"
	toolTasteProfile       = "get_taste_profile"
	toolGetRecommendations = "get_recommendations"
	toolPresentPicks       = "present_picks"
	// toolUnknown replaces a name the model made up, in events and the audit log.
	toolUnknown = "unknown"
)

var knownTools = map[string]bool{
	toolSearchCatalog: true, toolFindSimilar: true, toolTasteProfile: true, toolGetRecommendations: true,
}

const (
	defaultSearchLimit = 8
	maxSearchLimit     = 10
	toolTimeout        = 8 * time.Second
	overviewPreview    = 220
)

func function(name, description, schema string) llm.Tool {
	return llm.Tool{Type: "function", Function: llm.FunctionDef{
		Name:        name,
		Description: description,
		Parameters:  json.RawMessage(schema),
	}}
}

// filterSchema lists the optional filters shared by search_catalog and
// find_similar. The descriptions repeat "only if asked" because small models
// otherwise add constraints the person never stated.
const filterSchema = `"media_type":{"type":"string","enum":["movie","tv"],"description":"Only when the person wants films (movie) or series (tv)"},
			"genres":{"type":"array","items":{"type":"string"},"description":"Only genres the person asked for, such as Thriller, Comedy, Drama, Science Fiction, Animation, Horror, Romance, Crime"},
			"min_year":{"type":"integer","description":"Earliest release year, only if asked"},
			"max_year":{"type":"integer","description":"Latest release year, only if asked"},
			"min_rating":{"type":"number","description":"Minimum TMDB rating 0 to 10, only if they ask for highly rated titles"},
			"max_runtime":{"type":"integer","description":"Maximum minutes (per episode for series), only if asked"},
			"language":{"type":"string","description":"Original language code such as en or ko, only if asked"}`

// toolDefinitions are read-only: the agent can look things up but cannot
// change a profile. Likes and dislikes happen only when the person clicks.
var toolDefinitions = []llm.Tool{
	function(toolSearchCatalog,
		"Search the catalog by mood, plot, theme, or title. Returns up to 10 titles with a ref for each.",
		`{"type":"object","properties":{
			"query":{"type":"string","description":"The mood, plot, theme, or title in plain words"},
			`+filterSchema+`,
			"limit":{"type":"integer","description":"How many results, 1 to 10"}
		},"required":["query"]}`),
	function(toolFindSimilar,
		"Find titles closest in plot and tone to one title, by its ref from an earlier result. Filters narrow the results, for example media_type tv for series like a film.",
		`{"type":"object","properties":{
			"ref":{"type":"string","description":"A ref such as t2"},
			`+filterSchema+`
		},"required":["ref"]}`),
	function(toolTasteProfile,
		"Get the titles this person recently liked or watched, and their most common genres.",
		`{"type":"object","properties":{}}`),
	function(toolGetRecommendations,
		"Get this person's personalized ranking from the CineMatch recommender.",
		`{"type":"object","properties":{}}`),
	function(toolPresentPicks,
		"Present the final picks. Only refs returned by tools are accepted.",
		`{"type":"object","properties":{
			"message":{"type":"string","description":"One or two sentences introducing the picks"},
			"picks":{"type":"array","minItems":1,"maxItems":6,"items":{"type":"object","properties":{
				"ref":{"type":"string"},
				"reason":{"type":"string","description":"One sentence on why it fits, from the tool results"}
			},"required":["ref","reason"]}}
		},"required":["message","picks"]}`),
}

// parseFlexNumber reads a JSON number or a numeric string; small models
// often quote numbers. An empty string or null reads as zero.
func parseFlexNumber(b []byte) (float64, error) {
	var x float64
	if err := json.Unmarshal(b, &x); err == nil {
		return x, nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return 0, err
	}
	if s = strings.TrimSpace(s); s == "" {
		return 0, nil
	}
	x, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("expected a number, got %q", s)
	}
	return x, nil
}

// flexInt accepts 2015 or "2015".
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	x, err := parseFlexNumber(b)
	*f = flexInt(x)
	return err
}

// flexFloat accepts 7.5 or "7.5".
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	x, err := parseFlexNumber(b)
	*f = flexFloat(x)
	return err
}

// flexStrings accepts ["Crime","Drama"] or "Crime, Drama".
type flexStrings []string

func (f *flexStrings) UnmarshalJSON(b []byte) error {
	var list []string
	if err := json.Unmarshal(b, &list); err == nil {
		*f = list
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*f = append(*f, part)
		}
	}
	return nil
}

type searchArgs struct {
	Query      string      `json:"query"`
	MediaType  string      `json:"media_type"`
	Genres     flexStrings `json:"genres"`
	MinYear    flexInt     `json:"min_year"`
	MaxYear    flexInt     `json:"max_year"`
	MinRating  flexFloat   `json:"min_rating"`
	MaxRuntime flexInt     `json:"max_runtime"`
	Language   string      `json:"language"`
	Limit      flexInt     `json:"limit"`
}

var languageCode = regexp.MustCompile(`^[a-z]{2}$`)

// languageNames maps names the model may use instead of ISO codes.
var languageNames = map[string]string{
	"english": "en", "korean": "ko", "japanese": "ja", "spanish": "es",
	"french": "fr", "hindi": "hi", "chinese": "zh", "german": "de",
}

// filters validates and clamps search arguments. Out-of-range values are
// dropped, not rejected, so one bad field does not waste a tool call.
func (a searchArgs) filters() (db.SearchFilters, []string) {
	var f db.SearchFilters
	switch strings.ToLower(a.MediaType) {
	case "movie", "film", "movies", "films":
		f.MediaType = "movie"
	case "tv", "series", "show", "shows":
		f.MediaType = "tv"
	}
	genres, notes := resolveGenres(a.Genres, f.MediaType)
	f.Genres = genres
	if y := int(a.MinYear); y >= 1900 && y <= 2100 {
		f.MinYear = y
	}
	if y := int(a.MaxYear); y >= 1900 && y <= 2100 {
		f.MaxYear = y
	}
	if f.MinYear > 0 && f.MaxYear > 0 && f.MinYear > f.MaxYear {
		f.MinYear, f.MaxYear = f.MaxYear, f.MinYear
	}
	if r := float64(a.MinRating); r > 0 && r <= 10 {
		f.MinRating = r
	}
	if m := int(a.MaxRuntime); m > 0 && m <= 600 {
		f.MaxRuntime = m
	}
	lang := strings.ToLower(strings.TrimSpace(a.Language))
	if code, ok := languageNames[lang]; ok {
		lang = code
	}
	if languageCode.MatchString(lang) {
		f.Language = lang
	}
	return f, notes
}

func (a searchArgs) limit() int {
	n := int(a.Limit)
	if n < 1 {
		return defaultSearchLimit
	}
	return min(n, maxSearchLimit)
}

// titleView is the compact form of a title shown to the model.
type titleView struct {
	Ref      string   `json:"ref"`
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Year     int      `json:"year,omitempty"`
	Genres   []string `json:"genres,omitempty"`
	Rating   float64  `json:"rating,omitempty"`
	Runtime  int      `json:"runtime_min,omitempty"`
	Language string   `json:"language,omitempty"`
	Match    *float64 `json:"match,omitempty"`
	Overview string   `json:"overview,omitempty"`
}

func viewOf(ref string, m db.Movie, similarity *float64) titleView {
	v := titleView{
		Ref:      ref,
		Title:    m.Title,
		Type:     m.MediaType,
		Year:     m.ReleaseYear,
		Genres:   m.Genres,
		Rating:   math.Round(m.VoteAverage*10) / 10,
		Runtime:  m.Runtime,
		Language: m.OriginalLanguage,
		Overview: truncateWords(m.Overview, overviewPreview),
	}
	if similarity != nil {
		rounded := math.Round(*similarity*100) / 100
		v.Match = &rounded
	}
	return v
}

func truncateWords(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	cut := string(runes[:limit])
	if i := strings.LastIndex(cut, " "); i > limit/2 {
		cut = cut[:i]
	}
	return cut + "…"
}

// toolResult is what one tool call produced.
type toolResult struct {
	content   string // JSON sent back to the model
	count     int
	retrieval string
	titles    []string
	err       string
}

func encodeForModel(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"could not encode result"}`
	}
	return string(b)
}

func errorResult(msg string) toolResult {
	return toolResult{content: encodeForModel(map[string]string{"error": msg}), err: msg}
}

func titlesOf(views []titleView) []string {
	out := make([]string, 0, min(len(views), 4))
	for _, v := range views[:min(len(views), 4)] {
		out = append(out, v.Title)
	}
	return out
}

func (a *Agent) searchCatalog(ctx context.Context, arguments string, g *groundingSet) toolResult {
	var args searchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return errorResult("arguments were not valid JSON: " + err.Error())
	}
	query := cleanText(args.Query, 200)
	if query == "" {
		return errorResult("query is required")
	}
	filters, notes := args.filters()
	result, err := a.catalog.Search(ctx, query, args.limit(), filters)
	if err != nil {
		return errorResult("catalog search is unavailable")
	}
	views := make([]titleView, 0, len(result.Hits))
	for _, hit := range result.Hits {
		ref := g.add(hit.Movie, hit.Similarity, toolSearchCatalog)
		views = append(views, viewOf(ref, hit.Movie, hit.Similarity))
	}
	payload := map[string]any{"results": views}
	if named := a.namedTitlesOutsideFilters(ctx, query, filters, g); len(named) > 0 {
		payload["named_titles"] = named
		notes = append(notes, "named_titles match the query by name but fall outside the filters; use their refs with find_similar, never as picks")
	}
	if len(notes) > 0 {
		payload["notes"] = notes
	}
	if len(views) == 0 {
		payload["hint"] = "No matches. Loosen a filter or rephrase the query."
	}
	return toolResult{content: encodeForModel(payload), count: len(views), retrieval: result.Retrieval, titles: titlesOf(views)}
}

// namedTitlesOutsideFilters handles "like Parasite, but a series": models
// often search the named title with the series filter, which hides the film.
// When a short filtered query is exactly a catalog title, that title comes
// back separately as a seed for find_similar.
func (a *Agent) namedTitlesOutsideFilters(ctx context.Context, query string, f db.SearchFilters, g *groundingSet) []titleView {
	filtered := f.MediaType != "" || len(f.Genres) > 0 || f.MinYear > 0 || f.MaxYear > 0 ||
		f.MinRating > 0 || f.MaxRuntime > 0 || f.Language != ""
	if !filtered || len(strings.Fields(query)) > 6 {
		return nil
	}
	named, err := a.titles.TitlesNamed(ctx, query)
	if err != nil {
		return nil
	}
	var views []titleView
	for _, m := range named {
		if _, seen := g.refOf[m.ID]; seen {
			continue
		}
		ref := g.add(m, nil, toolSearchCatalog)
		g.markSeedOnly(ref)
		views = append(views, viewOf(ref, m, nil))
	}
	return views
}

func (a *Agent) findSimilar(ctx context.Context, arguments string, g *groundingSet) toolResult {
	var args struct {
		Ref string `json:"ref"`
		searchArgs
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return errorResult("arguments were not valid JSON: " + err.Error())
	}
	seed, ok := g.lookup(args.Ref)
	if !ok {
		return errorResult("unknown ref; use a ref from an earlier result, or search for the title first")
	}
	filters, notes := args.filters()
	similar, err := a.titles.SimilarToTitle(ctx, seed.movie.ID, defaultSearchLimit, filters)
	if err != nil {
		return errorResult("similarity lookup is unavailable")
	}
	views := make([]titleView, 0, len(similar))
	for _, hit := range similar {
		ref := g.add(hit.Movie, hit.Similarity, toolFindSimilar)
		views = append(views, viewOf(ref, hit.Movie, hit.Similarity))
	}
	payload := map[string]any{"similar_to": seed.movie.Title, "results": views}
	if len(notes) > 0 {
		payload["notes"] = notes
	}
	return toolResult{content: encodeForModel(payload), count: len(views), titles: titlesOf(views)}
}

func (a *Agent) tasteProfile(ctx context.Context, userID string, g *groundingSet) toolResult {
	liked, err := a.titles.RecentPositiveTitles(ctx, userID, 10)
	if err != nil {
		return errorResult("taste profile is unavailable")
	}
	if len(liked) == 0 {
		return toolResult{content: encodeForModel(map[string]any{
			"recent_likes": []titleView{},
			"note":         "No likes yet. Ask what they enjoy, or use search_catalog.",
		})}
	}
	views := make([]titleView, 0, len(liked))
	genreCounts := map[string]int{}
	for _, m := range liked {
		ref := g.add(m, nil, toolTasteProfile)
		g.markSeedOnly(ref)
		views = append(views, viewOf(ref, m, nil))
		for _, genre := range m.Genres {
			genreCounts[genre]++
		}
	}
	payload := map[string]any{
		"recent_likes": views,
		"top_genres":   topKeys(genreCounts, 3),
		"note":         "These are already liked, so do not present them. Use find_similar on them.",
	}
	return toolResult{content: encodeForModel(payload), count: len(views), titles: titlesOf(views)}
}

func (a *Agent) recommendations(ctx context.Context, userID string, g *groundingSet) toolResult {
	movies, source, err := a.recs.Recommend(ctx, userID)
	if err != nil {
		return errorResult("recommendations are unavailable")
	}
	movies = movies[:min(len(movies), maxSearchLimit)]
	views := make([]titleView, 0, len(movies))
	for _, m := range movies {
		ref := g.add(m, nil, toolGetRecommendations)
		views = append(views, viewOf(ref, m, nil))
	}
	payload := map[string]any{"source": source, "results": views}
	if source == "popular" {
		payload["note"] = "This person has no taste profile yet, so these are popular titles."
	}
	return toolResult{content: encodeForModel(payload), count: len(views), titles: titlesOf(views)}
}

// topKeys returns the n most frequent keys, alphabetical on ties.
func topKeys(counts map[string]int, n int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys[:min(n, len(keys))]
}

// labelFor describes a tool call in plain words for the activity timeline.
func labelFor(name, arguments string) string {
	switch name {
	case toolSearchCatalog:
		var args searchArgs
		_ = json.Unmarshal([]byte(arguments), &args)
		f, _ := args.filters()
		label := "Searching the catalog"
		if q := cleanText(args.Query, 80); q != "" {
			label = fmt.Sprintf("Searching for %q", q)
		}
		var parts []string
		switch f.MediaType {
		case "movie":
			parts = append(parts, "films")
		case "tv":
			parts = append(parts, "series")
		}
		if len(f.Genres) > 0 {
			parts = append(parts, strings.Join(f.Genres, ", "))
		}
		switch {
		case f.MinYear > 0 && f.MaxYear > 0:
			parts = append(parts, fmt.Sprintf("%d to %d", f.MinYear, f.MaxYear))
		case f.MinYear > 0:
			parts = append(parts, fmt.Sprintf("%d or later", f.MinYear))
		case f.MaxYear > 0:
			parts = append(parts, fmt.Sprintf("%d or earlier", f.MaxYear))
		}
		if f.MinRating > 0 {
			parts = append(parts, fmt.Sprintf("rated %.1f+", f.MinRating))
		}
		if f.MaxRuntime > 0 {
			parts = append(parts, fmt.Sprintf("up to %d min", f.MaxRuntime))
		}
		if f.Language != "" {
			parts = append(parts, "language "+f.Language)
		}
		if len(parts) > 0 {
			label += " · " + strings.Join(parts, " · ")
		}
		return label
	case toolFindSimilar:
		var args searchArgs
		_ = json.Unmarshal([]byte(arguments), &args)
		switch f, _ := args.filters(); f.MediaType {
		case "movie":
			return "Finding films with a similar plot and tone"
		case "tv":
			return "Finding series with a similar plot and tone"
		}
		return "Finding titles with a similar plot and tone"
	case toolTasteProfile:
		return "Reading your recent likes"
	case toolGetRecommendations:
		return "Checking your personalized ranking"
	default:
		return "Calling an unknown tool"
	}
}

// argumentText joins the keys and string values of tool arguments, decoded,
// so the output guard sees the text a person would read.
func argumentText(arguments string) string {
	var parsed any
	if err := json.Unmarshal([]byte(arguments), &parsed); err != nil {
		return arguments
	}
	var parts []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			parts = append(parts, x)
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for k, e := range x {
				parts = append(parts, k)
				walk(e)
			}
		}
	}
	walk(parsed)
	return strings.Join(parts, " ")
}

var (
	emailPattern = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)
	phonePattern = regexp.MustCompile(`\+?\d[\d\s().-]{7,}\d`)
)

// redactForAudit masks emails and phone numbers in tool arguments before they
// are stored, since search queries are built from what the person typed. It
// also drops NUL characters, which Postgres jsonb rejects; a rejected write
// would lose the run's audit record.
func redactForAudit(arguments string) json.RawMessage {
	var parsed any
	if err := json.Unmarshal([]byte(arguments), &parsed); err != nil {
		return json.RawMessage(`{}`)
	}
	clean, err := json.Marshal(scrubStrings(parsed))
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return clean
}

// scrubStrings redacts and strips NULs from every string in a decoded JSON
// value. Decoding already turned lone UTF-16 surrogates into U+FFFD.
func scrubStrings(v any) any {
	switch x := v.(type) {
	case string:
		x = strings.ReplaceAll(x, "\x00", "")
		x = emailPattern.ReplaceAllString(x, "[email]")
		return phonePattern.ReplaceAllString(x, "[phone]")
	case []any:
		for i := range x {
			x[i] = scrubStrings(x[i])
		}
		return x
	case map[string]any:
		clean := make(map[string]any, len(x))
		for k, val := range x {
			clean[scrubStrings(k).(string)] = scrubStrings(val)
		}
		return clean
	default:
		return v
	}
}
