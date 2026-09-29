package db

import "testing"

func TestSearchFiltersActive(t *testing.T) {
	tests := []struct {
		name string
		f    SearchFilters
		want bool
	}{
		{name: "none", f: SearchFilters{}, want: false},
		{name: "excluded IDs only", f: SearchFilters{ExcludeIDs: []string{"m1"}}, want: false},
		{name: "media type", f: SearchFilters{MediaType: "tv"}, want: true},
		{name: "genre", f: SearchFilters{Genres: []string{"Crime"}}, want: true},
		{name: "years", f: SearchFilters{MaxYear: 2000}, want: true},
		{name: "rating", f: SearchFilters{MinRating: 7}, want: true},
		{name: "runtime", f: SearchFilters{MaxRuntime: 120}, want: true},
		{name: "language", f: SearchFilters{Language: "ko"}, want: true},
	}
	for _, tc := range tests {
		if got := tc.f.Active(); got != tc.want {
			t.Errorf("%s: Active() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsLanguageCode(t *testing.T) {
	for code, want := range map[string]bool{"en": true, "ko": true, "EN": false, "eng": false, "": false} {
		if got := IsLanguageCode(code); got != want {
			t.Errorf("IsLanguageCode(%q) = %v", code, got)
		}
	}
}
