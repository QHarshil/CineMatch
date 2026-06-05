package main

import (
	"strings"
	"testing"
)

func TestBuildEmbeddingText(t *testing.T) {
	tests := []struct {
		title    string
		overview string
		want     string
	}{
		{
			title:    "Inception",
			overview: "A thief who steals corporate secrets through dream-sharing technology.",
			want:     "Inception. A thief who steals corporate secrets through dream-sharing technology.",
		},
		{
			title:    "Untitled Documentary",
			overview: "",
			want:     "Untitled Documentary",
		},
		{
			title:    "Movie",
			overview: "Some overview.",
			want:     "Movie. Some overview.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			got := buildEmbeddingText(tc.title, tc.overview)
			if got != tc.want {
				t.Errorf("buildEmbeddingText(%q, %q) = %q, want %q", tc.title, tc.overview, got, tc.want)
			}
		})
	}
}

func TestExtractReleaseYear(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"2010-07-16", 2010},
		{"1999-03-31", 1999},
		{"2024-01-01", 2024},
		{"", 0},
		{"abc", 0},
		{"20", 0},         // too short
		{"abcd-01-01", 0}, // non-numeric year
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := extractReleaseYear(tc.input)
			if got != tc.want {
				t.Errorf("extractReleaseYear(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestGenreNamesFromIDs(t *testing.T) {
	genreMap := map[int]string{
		28:  "Action",
		18:  "Drama",
		878: "Science Fiction",
	}

	tests := []struct {
		name string
		ids  []int
		want []string
	}{
		{
			name: "maps known IDs",
			ids:  []int{28, 18},
			want: []string{"Action", "Drama"},
		},
		{
			name: "skips unknown IDs silently",
			ids:  []int{28, 9999},
			want: []string{"Action"},
		},
		{
			name: "returns empty slice for no IDs",
			ids:  []int{},
			want: []string{},
		},
		{
			name: "returns empty slice when all IDs unknown",
			ids:  []int{1111, 2222},
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := genreNamesFromIDs(tc.ids, genreMap)
			if len(got) != len(tc.want) {
				t.Fatalf("genreNamesFromIDs len = %d, want %d (got %v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("genre[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseMediaTypes(t *testing.T) {
	tests := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"movie", []string{"movie"}, false},
		{"tv", []string{"tv"}, false},
		{"both", []string{"movie", "tv"}, false},
		{"TV", []string{"tv"}, false},
		{"music", nil, true},
	}
	for _, tc := range tests {
		got, err := parseMediaTypes(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseMediaTypes(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseMediaTypes(%q) unexpected error: %v", tc.in, err)
		}
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("parseMediaTypes(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestDeduplicate(t *testing.T) {
	// Same tmdb_id under different media types is two distinct titles.
	items := []tmdbItem{
		{ID: 1, mediaType: "movie"},
		{ID: 1, mediaType: "movie"}, // duplicate, dropped
		{ID: 1, mediaType: "tv"},    // same id, different namespace, kept
		{ID: 2, mediaType: "movie"},
	}
	if got := deduplicate(items); len(got) != 3 {
		t.Fatalf("deduplicate kept %d items, want 3 (%v)", len(got), got)
	}
}

func TestTmdbItemAccessors(t *testing.T) {
	movie := tmdbItem{Title: "Inception", ReleaseDate: "2010-07-16"}
	if movie.displayTitle() != "Inception" || movie.dateString() != "2010-07-16" {
		t.Errorf("movie accessors: %q %q", movie.displayTitle(), movie.dateString())
	}
	show := tmdbItem{Name: "Breaking Bad", FirstAirDate: "2008-01-20"}
	if show.displayTitle() != "Breaking Bad" || show.dateString() != "2008-01-20" {
		t.Errorf("tv accessors: %q %q", show.displayTitle(), show.dateString())
	}
}

func TestDiscoverParams(t *testing.T) {
	movieRecent := discoverParams("movie", "recent", 1)
	if movieRecent["sort_by"] != "primary_release_date.desc" {
		t.Errorf("movie recent sort = %q", movieRecent["sort_by"])
	}
	if _, ok := movieRecent["primary_release_date.lte"]; !ok {
		t.Error("movie params missing primary_release_date.lte upper bound")
	}

	tvPopular := discoverParams("tv", "popular", 2)
	if tvPopular["sort_by"] != "popularity.desc" {
		t.Errorf("tv popular sort = %q", tvPopular["sort_by"])
	}
	if _, ok := tvPopular["first_air_date.lte"]; !ok {
		t.Error("tv params missing first_air_date.lte upper bound")
	}
	if tvPopular["page"] != "2" {
		t.Errorf("page = %q, want 2", tvPopular["page"])
	}
}
