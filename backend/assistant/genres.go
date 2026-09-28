package assistant

import (
	"fmt"
	"strings"
)

// TMDB uses different genre vocabularies for films and series (for example
// "Science Fiction" vs "Sci-Fi & Fantasy", and series have no Thriller or
// Horror). The model asks in plain words; genreVocab maps each word to the
// stored names per media type. An empty slice means that media type has no
// equivalent genre.
var genreVocab = map[string]struct{ movie, tv []string }{
	"action":             {[]string{"Action"}, []string{"Action & Adventure"}},
	"adventure":          {[]string{"Adventure"}, []string{"Action & Adventure"}},
	"animation":          {[]string{"Animation"}, []string{"Animation"}},
	"animated":           {[]string{"Animation"}, []string{"Animation"}},
	"anime":              {[]string{"Animation"}, []string{"Animation"}},
	"comedy":             {[]string{"Comedy"}, []string{"Comedy"}},
	"crime":              {[]string{"Crime"}, []string{"Crime"}},
	"documentary":        {[]string{"Documentary"}, []string{"Documentary"}},
	"drama":              {[]string{"Drama"}, []string{"Drama"}},
	"family":             {[]string{"Family"}, []string{"Family", "Kids"}},
	"kids":               {[]string{"Family"}, []string{"Kids"}},
	"fantasy":            {[]string{"Fantasy"}, []string{"Sci-Fi & Fantasy"}},
	"science fiction":    {[]string{"Science Fiction"}, []string{"Sci-Fi & Fantasy"}},
	"sci-fi":             {[]string{"Science Fiction"}, []string{"Sci-Fi & Fantasy"}},
	"scifi":              {[]string{"Science Fiction"}, []string{"Sci-Fi & Fantasy"}},
	"sci-fi & fantasy":   {[]string{"Science Fiction", "Fantasy"}, []string{"Sci-Fi & Fantasy"}},
	"history":            {[]string{"History"}, []string{"War & Politics"}},
	"historical":         {[]string{"History"}, []string{"War & Politics"}},
	"horror":             {[]string{"Horror"}, nil},
	"music":              {[]string{"Music"}, nil},
	"musical":            {[]string{"Music"}, nil},
	"mystery":            {[]string{"Mystery"}, []string{"Mystery"}},
	"romance":            {[]string{"Romance"}, nil},
	"romantic":           {[]string{"Romance"}, nil},
	"thriller":           {[]string{"Thriller"}, nil},
	"war":                {[]string{"War"}, []string{"War & Politics"}},
	"war & politics":     {[]string{"War"}, []string{"War & Politics"}},
	"politics":           {nil, []string{"War & Politics"}},
	"western":            {[]string{"Western"}, []string{"Western"}},
	"reality":            {nil, []string{"Reality"}},
	"talk":               {nil, []string{"Talk"}},
	"news":               {nil, []string{"News"}},
	"soap":               {nil, []string{"Soap"}},
	"action & adventure": {[]string{"Action", "Adventure"}, []string{"Action & Adventure"}},
}

// resolveGenres maps requested genre words to stored genre names for the
// media type ("" means both). Words with no equivalent are dropped and
// explained in notes, so the model can rely on the query text instead.
func resolveGenres(requested []string, mediaType string) (genres []string, notes []string) {
	seen := map[string]bool{}
	add := func(names []string) {
		for _, n := range names {
			if !seen[n] {
				seen[n] = true
				genres = append(genres, n)
			}
		}
	}
	for _, word := range requested {
		key := strings.ToLower(strings.TrimSpace(word))
		if key == "" {
			continue
		}
		entry, ok := genreVocab[key]
		if !ok {
			notes = append(notes, fmt.Sprintf("%q is not a catalog genre and was ignored; describe it in the query instead", word))
			continue
		}
		switch mediaType {
		case "movie":
			if len(entry.movie) == 0 {
				notes = append(notes, fmt.Sprintf("films have no %q genre; it was left to the query text", word))
			}
			add(entry.movie)
		case "tv":
			if len(entry.tv) == 0 {
				notes = append(notes, fmt.Sprintf("series have no %q genre; it was left to the query text", word))
			}
			add(entry.tv)
		default:
			add(entry.movie)
			add(entry.tv)
		}
	}
	return genres, notes
}
