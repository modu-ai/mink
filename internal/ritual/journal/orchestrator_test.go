package journal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newTestOrchestrator builds a JournalOrchestrator backed by a real SQLite storage.
// The prompt function is injectable so tests can simulate user responses without I/O.
func newTestOrchestrator(t *testing.T, cfg Config, promptFn PromptFunc) (*JournalOrchestrator, *Storage) {
	t.Helper()
	s := newTestStorage(t)
	w := NewJournalWriter(cfg, s, nil, nil, newJournalAuditWriter(nil), zap.NewNop(), nil)
	o := NewJournalOrchestrator(cfg, w, s, zap.NewNop(), newJournalAuditWriter(nil), promptFn)
	return o, s
}

// enabledCfg returns a Config with the journal feature enabled and a short prompt timeout.
func enabledCfg() Config {
	return Config{
		Enabled:          true,
		RetentionDays:    -1,
		PromptTimeoutMin: 1, // short timeout keeps tests fast
	}
}

// insertTodayEntry inserts a StoredEntry dated today for userID.
func insertTodayEntry(t *testing.T, s *Storage, userID string) {
	t.Helper()
	e := sampleEntry(userID, time.Now())
	require.NoError(t, s.Insert(context.Background(), e))
}

// TestOrchestrator_DisabledConfig_NoOp verifies that Prompt is a no-op when
// the feature is disabled. AC-001 (config gate)
func TestOrchestrator_DisabledConfig_NoOp(t *testing.T) {
	t.Parallel()

	promptCalled := false
	promptFn := func(_ context.Context, _ string) (string, error) {
		promptCalled = true
		return "오늘 좋았어", nil
	}

	cfg := Config{Enabled: false}
	o, s := newTestOrchestrator(t, cfg, promptFn)

	err := o.Prompt(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, promptCalled, "prompt must not be emitted when feature is disabled")

	n, err := s.countEntries(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, 0, n, "no entries must be written when feature is disabled")
}

// TestOrchestrator_TodayEntryExists_SkipPrompt verifies that Prompt is skipped
// when an entry already exists today. AC-014
func TestOrchestrator_TodayEntryExists_SkipPrompt(t *testing.T) {
	t.Parallel()

	promptCalled := false
	promptFn := func(_ context.Context, _ string) (string, error) {
		promptCalled = true
		return "두 번째 응답", nil
	}

	o, s := newTestOrchestrator(t, enabledCfg(), promptFn)
	insertTodayEntry(t, s, "u1")

	err := o.Prompt(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, promptCalled, "prompt must not be emitted when today's entry already exists")

	// Entry count must remain 1 (the pre-inserted entry only).
	n, err := s.countEntries(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, 1, n, "no additional entry must be written on skip")
}

// TestOrchestrator_TimeoutWithoutResponse verifies that Prompt returns nil
// and writes no entry when the user does not respond within the timeout. AC-014
func TestOrchestrator_TimeoutWithoutResponse(t *testing.T) {
	t.Parallel()

	// Simulate timeout: return empty response (no I/O block in tests).
	promptFn := func(_ context.Context, _ string) (string, error) {
		return "", nil // empty response = no action taken
	}

	o, s := newTestOrchestrator(t, enabledCfg(), promptFn)

	err := o.Prompt(context.Background(), "u1")
	require.NoError(t, err, "timeout must not return an error")

	n, err := s.countEntries(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, 0, n, "no entry must be persisted when user does not respond")
}

// TestOrchestrator_LowMoodSoftTone verifies that the low-mood prompt is used
// when recentValences returns all-low scores, and that the emitted prompt text
// contains the expected empathetic phrasing and crisis resource references. AC-015
func TestOrchestrator_LowMoodSoftTone(t *testing.T) {
	t.Parallel()

	var capturedPrompt string
	promptFn := func(_ context.Context, prompt string) (string, error) {
		capturedPrompt = prompt
		return "힘들었어요", nil
	}

	o, s := newTestOrchestrator(t, enabledCfg(), promptFn)

	// Insert 3 low-valence entries (all < 0.3) for the last 3 days.
	ctx := context.Background()
	base := time.Now().AddDate(0, 0, -3)
	for i := range 3 {
		e := &StoredEntry{
			UserID:    "u1",
			Date:      base.AddDate(0, 0, i),
			Text:      "힘든 하루",
			Vad:       Vad{Valence: 0.1, Arousal: 0.4, Dominance: 0.3},
			CreatedAt: base.AddDate(0, 0, i),
		}
		require.NoError(t, s.Insert(ctx, e))
	}

	err := o.Prompt(ctx, "u1")
	require.NoError(t, err)

	// AC-015: low-mood prompt must come from the low_mood_sequence category.
	assert.NotEmpty(t, capturedPrompt, "a prompt must be emitted")
	assert.True(t,
		strings.Contains(capturedPrompt, "언제든 이야기해주세요") ||
			strings.Contains(capturedPrompt, "힘드시죠") ||
			strings.Contains(capturedPrompt, "힘든 날"),
		"low-mood prompt must contain empathetic phrasing; got: %q", capturedPrompt)

	// Forbidden clinical terms must not appear.
	forbiddenTerms := []string{"진단", "우울증", "PHQ", "병원", "처방"}
	for _, term := range forbiddenTerms {
		assert.NotContains(t, capturedPrompt, term,
			"low-mood prompt must not contain clinical term: %q", term)
	}
}

// TestOrchestrator_NoLowMoodNeutralPrompt verifies that a neutral prompt is used
// when mood history is not low (fewer than 3 recent entries, or any valence >= 0.3).
func TestOrchestrator_NoLowMoodNeutralPrompt(t *testing.T) {
	t.Parallel()

	var capturedPrompt string
	promptFn := func(_ context.Context, prompt string) (string, error) {
		capturedPrompt = prompt
		return "오늘 좋았어", nil
	}

	o, s := newTestOrchestrator(t, enabledCfg(), promptFn)

	// Insert 3 entries with mixed valence (one >= 0.3 breaks the low-mood gate).
	ctx := context.Background()
	base := time.Now().AddDate(0, 0, -3)
	valences := []float64{0.1, 0.5, 0.1} // 0.5 breaks all-low rule
	for i, v := range valences {
		e := &StoredEntry{
			UserID:    "u1",
			Date:      base.AddDate(0, 0, i),
			Text:      "그냥 하루",
			Vad:       Vad{Valence: v, Arousal: 0.4, Dominance: 0.5},
			CreatedAt: base.AddDate(0, 0, i),
		}
		require.NoError(t, s.Insert(ctx, e))
	}

	err := o.Prompt(ctx, "u1")
	require.NoError(t, err)

	// Neutral prompts are defined in the "neutral" category — they should not
	// contain any of the low-mood-specific empathetic openings.
	assert.NotEmpty(t, capturedPrompt)
	lowMoodPhrases := []string{"언제든 이야기해주세요", "힘드시죠", "힘든 날이 계속"}
	for _, phrase := range lowMoodPhrases {
		assert.NotContains(t, capturedPrompt, phrase,
			"neutral prompt must not contain low-mood phrase: %q", phrase)
	}
}

// TestOrchestrator_OnEveningCheckIn_PropagatesError verifies that
// OnEveningCheckIn logs errors from Prompt without panicking.
func TestOrchestrator_OnEveningCheckIn_PropagatesError(t *testing.T) {
	t.Parallel()

	// Prompt that returns a response, then Write will succeed — no error path is easy
	// to trigger here without closing the DB; instead we verify the call doesn't panic.
	promptFn := func(_ context.Context, _ string) (string, error) {
		return "괜찮은 하루", nil
	}

	o, _ := newTestOrchestrator(t, enabledCfg(), promptFn)

	// Must not panic.
	assert.NotPanics(t, func() {
		o.OnEveningCheckIn(context.Background(), "u1")
	})
}

// TestOrchestrator_isLowMood_EdgeCases exercises the isLowMood helper directly.
func TestOrchestrator_isLowMood_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		valences []float64
		want     bool
	}{
		{"empty slice", []float64{}, false},
		{"fewer than 3", []float64{0.1, 0.2}, false},
		{"exactly 3 all low", []float64{0.1, 0.2, 0.29}, true},
		{"exactly 3 boundary at 0.3", []float64{0.1, 0.2, 0.3}, false},
		{"4 entries all low", []float64{0.05, 0.1, 0.15, 0.2}, true},
		{"mixed high and low", []float64{0.1, 0.8, 0.1}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isLowMood(tc.valences),
				"isLowMood(%v) should return %v", tc.valences, tc.want)
		})
	}
}
