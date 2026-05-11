package journal

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAnalyzer() *LocalDictAnalyzer { return NewLocalDictAnalyzer() }

// TestVAD_LocalAnalysis_Happy verifies that a clearly positive sentence returns
// high valence. AC-003
func TestVAD_LocalAnalysis_Happy(t *testing.T) {
	t.Parallel()
	a := newAnalyzer()

	vad, tags, err := a.Analyze(context.Background(), "오늘 정말 행복했어, 오랜만에 웃었어", "")
	require.NoError(t, err)
	require.NotNil(t, vad)

	assert.GreaterOrEqual(t, vad.Valence, 0.7, "happy sentence must have high valence")

	hasPositiveTag := slices.ContainsFunc(tags, func(t string) bool {
		return t == "happy" || t == "grateful" || t == "excited" || t == "proud"
	})
	assert.True(t, hasPositiveTag, "tags should include a positive emotion, got %v", tags)
}

// TestEmoji_SadDetection_Tired verifies that sad/tired emoji + text pulls valence down.
// AC-004
func TestEmoji_SadDetection_Tired(t *testing.T) {
	t.Parallel()
	a := newAnalyzer()

	vad, tags, err := a.Analyze(context.Background(), "😔 힘들다", "")
	require.NoError(t, err)
	require.NotNil(t, vad)

	assert.Less(t, vad.Valence, 0.4, "sad/tired emoji+text must produce low valence")

	hasNegativeTag := slices.ContainsFunc(tags, func(t string) bool {
		return t == "sad" || t == "tired" || t == "lonely" || t == "anxious"
	})
	assert.True(t, hasNegativeTag, "tags should include a negative emotion, got %v", tags)
}

// TestVAD_NegationFlip verifies that negation tokens invert the valence of the keyword.
// research.md §2.3
func TestVAD_NegationFlip(t *testing.T) {
	t.Parallel()
	a := newAnalyzer()

	// Without negation: high valence.
	posVad, _, err := a.Analyze(context.Background(), "오늘 행복했어", "")
	require.NoError(t, err)

	// With negation: valence should be lower.
	negVad, _, err := a.Analyze(context.Background(), "행복하지 않아", "")
	require.NoError(t, err)

	assert.Less(t, negVad.Valence, posVad.Valence,
		"negation must reduce valence (negated=%.2f, positive=%.2f)", negVad.Valence, posVad.Valence)
}

// TestVAD_IntensityModifier verifies that intensity modifiers amplify arousal.
// research.md §2.4
func TestVAD_IntensityModifier(t *testing.T) {
	t.Parallel()
	a := newAnalyzer()

	baseVad, _, err := a.Analyze(context.Background(), "행복했어", "")
	require.NoError(t, err)

	intensVad, _, err := a.Analyze(context.Background(), "너무 행복했어", "")
	require.NoError(t, err)

	assert.GreaterOrEqual(t, intensVad.Arousal, baseVad.Arousal,
		"intensity modifier must maintain or increase arousal")
}

// TestVAD_NoMatch_NeutralFallback verifies that unrecognised text falls back to neutral.
func TestVAD_NoMatch_NeutralFallback(t *testing.T) {
	t.Parallel()
	a := newAnalyzer()

	vad, tags, err := a.Analyze(context.Background(), "xyz unknown words blah", "")
	require.NoError(t, err)
	require.NotNil(t, vad)

	assert.InDelta(t, 0.5, vad.Valence, 0.1, "unmatched text should have near-neutral valence")
	assert.Empty(t, tags, "unmatched text should produce no emotion tags")
}

// TestVAD_TopThreeTags_OrderedByCount verifies that Analyze returns at most 3 tags,
// ordered by hit count descending.
func TestVAD_TopThreeTags_OrderedByCount(t *testing.T) {
	t.Parallel()
	a := newAnalyzer()

	// Repeat happy keywords many times, then a single sad keyword.
	text := "행복 행복 행복 행복 기쁘 기쁘 웃 감사 슬프"
	_, tags, err := a.Analyze(context.Background(), text, "")
	require.NoError(t, err)

	assert.LessOrEqual(t, len(tags), 3, "should return at most 3 tags, got %v", tags)
	// The most frequent category should come first.
	if len(tags) > 0 {
		assert.Equal(t, "happy", tags[0], "most frequent category must be first, got %v", tags)
	}
}
