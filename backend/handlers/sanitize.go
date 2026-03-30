package handlers

import (
	"regexp"
	"strings"
)

// stripHTMLTags removes HTML/script tags from user-supplied strings.
// This is a defense-in-depth measure — the API returns JSON, not HTML,
// but stripping tags prevents stored XSS if data is ever rendered in a browser.
var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

func sanitizeString(s string) string {
	s = htmlTagPattern.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return s
}
