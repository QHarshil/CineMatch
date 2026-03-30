package main

import (
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
