package handlers

import "regexp"

// uuidPattern matches the canonical UUID format (8-4-4-4-12 hex).
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// isValidUUID returns true if s is a lowercase canonical UUID.
func isValidUUID(s string) bool {
	return uuidPattern.MatchString(s)
}
