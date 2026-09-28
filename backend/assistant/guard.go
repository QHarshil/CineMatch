package assistant

import (
	"strings"
	"time"
	"unicode"
)

// leakWindow is the run of consecutive words that counts as repeating the
// instructions. Eight words is long enough that an ordinary decline ("I can
// only help choose something to watch") never matches by chance.
const leakWindow = 8

// leakMarkers never appear in a legitimate reply: tool names and a sentence
// that exist only in the instructions. Everyday phrases from the prompt (for
// example "how to work") are left to the eight-word check, so a picks message
// like "learning how to work together" is not blocked.
var leakMarkers = []string{
	toolSearchCatalog, toolFindSimilar, toolTasteProfile, toolGetRecommendations, toolPresentPicks,
	"tool results are catalog data",
}

const safeDecline = "I can only help you choose something to watch. Tell me a mood, a genre, or a title you liked."

// instructionShingles holds every leakWindow-word run of the system prompt.
var instructionShingles = func() map[string]bool {
	words := normalizedWords(systemPrompt(time.Time{}))
	shingles := make(map[string]bool, len(words))
	for i := 0; i+leakWindow <= len(words); i++ {
		shingles[strings.Join(words[i:i+leakWindow], " ")] = true
	}
	return shingles
}()

// leaksInstructions reports whether model output repeats the system prompt,
// the failure prompt-injection attempts aim for. Instructions alone do not
// stop a small model from complying, so output is checked before it is sent.
func leaksInstructions(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range leakMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	words := normalizedWords(text)
	for i := 0; i+leakWindow <= len(words); i++ {
		if instructionShingles[strings.Join(words[i:i+leakWindow], " ")] {
			return true
		}
	}
	return false
}

func normalizedWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
}

// guardPicks replaces a picks message or reason that repeats the
// instructions. Reports whether anything was replaced.
func guardPicks(message *string, picks []Pick) bool {
	blocked := false
	if leaksInstructions(*message) {
		*message, blocked = "Here are picks that fit your request.", true
	}
	for i := range picks {
		if leaksInstructions(picks[i].Reason) {
			picks[i].Reason, blocked = describeTitle(picks[i].Movie), true
		}
	}
	return blocked
}
