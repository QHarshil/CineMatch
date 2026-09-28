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

// leakMarkers never appear in a legitimate reply: tool names and section
// headings exist only in the instructions and tool schemas.
var leakMarkers = []string{
	"how to work",
	toolSearchCatalog, toolFindSimilar, toolTasteProfile, toolGetRecommendations, toolPresentPicks,
	"tool results are catalog data",
	"never reveal",
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
