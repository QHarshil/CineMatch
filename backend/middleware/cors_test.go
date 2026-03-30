package middleware

import (
	"testing"
)

func TestParseOrigins_TrimsTrailingSlash(t *testing.T) {
	origins := parseOrigins("https://cinematch.harshilc.com/ , http://localhost:3000/")
	want := []string{"https://cinematch.harshilc.com", "http://localhost:3000"}

	if len(origins) != len(want) {
		t.Fatalf("got %d origins, want %d: %v", len(origins), len(want), origins)
	}
	for i, got := range origins {
		if got != want[i] {
			t.Errorf("origins[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestParseOrigins_DefaultsToLocalhost(t *testing.T) {
	origins := parseOrigins("")
	if len(origins) != 1 || origins[0] != "http://localhost:3000" {
		t.Errorf("got %v, want [http://localhost:3000]", origins)
	}
}

func TestParseOrigins_HandlesWhitespace(t *testing.T) {
	origins := parseOrigins("  https://example.com  ,  https://other.com  ")
	want := []string{"https://example.com", "https://other.com"}

	if len(origins) != len(want) {
		t.Fatalf("got %d origins, want %d: %v", len(origins), len(want), origins)
	}
	for i, got := range origins {
		if got != want[i] {
			t.Errorf("origins[%d] = %q, want %q", i, got, want[i])
		}
	}
}
