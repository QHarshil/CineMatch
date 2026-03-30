package handlers

import "regexp"

// uuidPattern matches canonical UUIDs in any case (8-4-4-4-12 hex).
// Supabase always returns lowercase UUIDs, but clients may send uppercase ones legitimately.
var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// isValidUUID returns true if s is a valid canonical UUID (case-insensitive).
func isValidUUID(s string) bool {
	return uuidPattern.MatchString(s)
}
