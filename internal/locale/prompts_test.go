package locale

import (
	"strings"
	"testing"

	"github.com/pkoukk/tiktoken-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeKRLocale() LocaleContext {
	return LocaleContext{
		Country:           "KR",
		PrimaryLanguage:   "ko-KR",
		Timezone:          "Asia/Seoul",
		Currency:          "KRW",
		MeasurementSystem: "metric",
		CalendarSystem:    "gregorian",
		DetectedMethod:    SourceOS,
	}
}

func makeKRCultural() CulturalContext {
	return ResolveCulturalContext("KR")
}

// TestBuildSystemPromptAddendum_Contains verifies the output includes expected fields.
func TestBuildSystemPromptAddendum_Contains(t *testing.T) {
	loc := makeKRLocale()
	cul := makeKRCultural()

	out := BuildSystemPromptAddendum(loc, cul)

	assert.Contains(t, out, "KR", "country must be present")
	assert.Contains(t, out, "ko-KR", "primary language must be present")
	assert.Contains(t, out, "Asia/Seoul", "timezone must be present")
	assert.Contains(t, out, "KRW", "currency must be present")
	assert.Contains(t, out, "metric", "measurement system must be present")
	assert.Contains(t, out, "gregorian", "calendar system must be present")
	assert.Contains(t, out, "formal", "formality must be present")
	assert.Contains(t, out, "korean_jondaetmal", "honorific system must be present")
	assert.Contains(t, out, "family_first", "name order must be present")
	assert.Contains(t, out, "pipa", "legal flags must be present")
}

// TestBuildSystemPromptAddendum_Bilingual tests AC-LC-005: bilingual prompt includes
// both languages and code-switching directive.
func TestBuildSystemPromptAddendum_Bilingual(t *testing.T) {
	loc := makeKRLocale()
	loc.SecondaryLanguage = "en-US"
	cul := makeKRCultural()

	out := BuildSystemPromptAddendum(loc, cul)

	assert.Contains(t, out, "primary=ko-KR", "primary language must be labeled")
	assert.Contains(t, out, "secondary=en-US", "secondary language must be labeled")
	assert.Contains(t, out, "code-switching is natural", "code-switch directive must be present")
}

// TestBuildSystemPromptAddendum_TokenLimit tests AC-LC-011: output ≤ 400 tokens.
func TestBuildSystemPromptAddendum_TokenLimit(t *testing.T) {
	// Worst case: KR + bilingual primary/secondary.
	loc := makeKRLocale()
	loc.SecondaryLanguage = "en-US"
	cul := makeKRCultural()

	out := BuildSystemPromptAddendum(loc, cul)

	// Use tiktoken-go with cl100k_base encoding (same as OpenAI/Claude tokenizers).
	enc, err := tiktoken.GetEncoding("cl100k_base")
	require.NoError(t, err, "tiktoken cl100k_base must be available")

	tokens := enc.Encode(out, nil, nil)
	assert.LessOrEqual(t, len(tokens), 400,
		"prompt addendum must be ≤ 400 tokens, got %d", len(tokens))
}

// TestBuildSystemPromptAddendum_NoSecondaryLanguage verifies single-language output.
func TestBuildSystemPromptAddendum_NoSecondaryLanguage(t *testing.T) {
	loc := makeKRLocale()
	cul := makeKRCultural()

	out := BuildSystemPromptAddendum(loc, cul)

	// Should not contain bilingual phrasing.
	assert.False(t, strings.Contains(out, "secondary="), "no secondary language label expected")
	assert.False(t, strings.Contains(out, "code-switching"), "no code-switching directive expected")
}

// TestBuildSystemPromptAddendum_EmptyWeekend verifies graceful omission when no weekend days.
func TestBuildSystemPromptAddendum_EmptyWeekend(t *testing.T) {
	loc := LocaleContext{
		Country:           "XX",
		PrimaryLanguage:   "en-XX",
		Timezone:          "UTC",
		Currency:          "USD",
		MeasurementSystem: "metric",
		CalendarSystem:    "gregorian",
		DetectedMethod:    SourceDefault,
	}
	cul := CulturalContext{
		FormalityDefault: FormalityCasual,
		HonorificSystem:  "none",
		NameOrder:        "given_first",
		AddressFormat:    "western",
		WeekendDays:      nil,
		FirstDayOfWeek:   "Monday",
		LegalFlags:       nil,
	}

	out := BuildSystemPromptAddendum(loc, cul)
	assert.NotEmpty(t, out)
	// Should not panic and should produce a valid string.
	assert.Contains(t, out, "XX")
}

// TestBuildSystemPromptAddendum_Deterministic verifies same input → same output.
func TestBuildSystemPromptAddendum_Deterministic(t *testing.T) {
	loc := makeKRLocale()
	cul := makeKRCultural()

	out1 := BuildSystemPromptAddendum(loc, cul)
	out2 := BuildSystemPromptAddendum(loc, cul)

	// The current-time rendering can differ; compare without the timestamp line.
	normalize := func(s string) string {
		lines := strings.Split(s, "\n")
		var filtered []string
		for _, l := range lines {
			if !strings.HasPrefix(l, "- Timezone:") {
				filtered = append(filtered, l)
			}
		}
		return strings.Join(filtered, "\n")
	}
	assert.Equal(t, normalize(out1), normalize(out2), "output must be deterministic (excluding timestamp)")
}

// TestCurrentLocalTime_InvalidTZ verifies graceful handling of invalid TZ.
func TestCurrentLocalTime_InvalidTZ(t *testing.T) {
	result := currentLocalTime("Invalid/Zone")
	assert.Empty(t, result)
}

// TestCurrentLocalTime_ValidTZ verifies a valid TZ returns a non-empty string.
func TestCurrentLocalTime_ValidTZ(t *testing.T) {
	result := currentLocalTime("Asia/Seoul")
	assert.NotEmpty(t, result)
}
