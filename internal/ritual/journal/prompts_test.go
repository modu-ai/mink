package journal

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forbiddenPromptPhrases are the phrases that must never appear in any prompt template.
// AC-019
var forbiddenPromptPhrases = []string{
	"가장 큰 비밀",
	"서운한 점",
	"숨기고 싶은",
	"부끄러운",
	"가장 후회",
}

// TestPrompts_AllNeutral_NoForbiddenPhrase verifies that no template in the vault
// contains coercive or privacy-invasive phrases. AC-019
func TestPrompts_AllNeutral_NoForbiddenPhrase(t *testing.T) {
	t.Parallel()

	templates := All()
	require.NotEmpty(t, templates, "prompt vault must not be empty")

	for _, tpl := range templates {
		for _, phrase := range forbiddenPromptPhrases {
			assert.False(t, strings.Contains(tpl, phrase),
				"template %q contains forbidden phrase %q", tpl, phrase)
		}
	}
}

// TestPrompts_AllOpenQuestion verifies that every template ends with a question mark.
// AC-019 (open question principle)
func TestPrompts_AllOpenQuestion(t *testing.T) {
	t.Parallel()

	templates := All()
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			t.Parallel()
			lastRune, _ := lastRuneOf(tpl)
			assert.True(t, lastRune == '?' || lastRune == '？',
				"template %q must end with ? or ？ (got %c)", tpl, lastRune)
		})
	}
}

// TestPrompts_PickAnniversary_IncludesDateName verifies that the anniversary prompt
// contains the supplied date name.
func TestPrompts_PickAnniversary_IncludesDateName(t *testing.T) {
	t.Parallel()

	dateName := "결혼기념일"
	prompt := PickAnniversary(dateName)
	assert.Contains(t, prompt, dateName, "anniversary prompt must include the date name")
}

// TestPrompts_LengthBound verifies that every template is at most 100 characters.
// Keeps prompts concise for the evening ritual UX.
func TestPrompts_LengthBound(t *testing.T) {
	t.Parallel()

	templates := All()
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			t.Parallel()
			runeLen := utf8.RuneCountInString(tpl)
			assert.LessOrEqual(t, runeLen, 100,
				"template %q exceeds 100 runes (%d)", tpl, runeLen)
		})
	}
}

// lastRuneOf returns the last rune of s and its byte size.
func lastRuneOf(s string) (rune, int) {
	if s == "" {
		return 0, 0
	}
	r, size := utf8.DecodeLastRuneInString(s)
	return r, size
}
