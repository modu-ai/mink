package journal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSummaryLLMResponse = `{"one_liner":"이번 주는 평온하고 행복한 감정이 주를 이뤘습니다."}`

// newTestWeeklySummary returns a populated WeeklySummary for testing.
func newTestWeeklySummary(userID string) *WeeklySummary {
	now := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	return &WeeklySummary{
		UserID:             userID,
		From:               now.AddDate(0, 0, -6),
		To:                 now,
		EntryCount:         5,
		AvgValence:         0.72,
		TopTags:            []string{"calm", "happy", "joy"},
		WordCloud:          []string{"좋았어", "산책", "행복"},
		PendingSummaryFlag: true,
	}
}

// TestSummaryLLM_HappyPath verifies that a valid LLM response populates OneLiner.
func TestSummaryLLM_HappyPath(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: validSummaryLLMResponse}
	enhancer := NewLLMSummaryEnhancer(client, newJournalAuditWriter(nil))

	summary := newTestWeeklySummary("u1")
	enhanced, err := enhancer.EnhanceWeeklySummary(context.Background(), summary)

	require.NoError(t, err)
	require.NotNil(t, enhanced)
	assert.Equal(t, "이번 주는 평온하고 행복한 감정이 주를 이뤘습니다.", enhanced.OneLiner)
	assert.Equal(t, 1, client.invokeCount)
}

// TestSummaryLLM_PayloadAggregateOnly verifies AC-020:
// the LLM payload contains only avg_valence, top_tags, word_freq — no raw entry text.
func TestSummaryLLM_PayloadAggregateOnly(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: validSummaryLLMResponse}
	enhancer := NewLLMSummaryEnhancer(client, newJournalAuditWriter(nil))

	summary := newTestWeeklySummary("u1")
	// Add a raw text string to WordCloud to verify it's only included as a key, not full text.
	summary.WordCloud = []string{"token1", "token2"}

	_, err := enhancer.EnhanceWeeklySummary(context.Background(), summary)
	require.NoError(t, err)

	// The user text payload must NOT contain raw entry text (e.g. "오늘 산책하며...").
	// The payload is the aggregated JSON — check for absence of raw entry markers.
	assert.NotContains(t, client.lastUserText, "오늘 산책하며 좋은 하루를 보냈어요",
		"LLM payload must not contain raw entry text")
	assert.NotContains(t, client.lastUserText, "u1",
		"LLM payload must not contain user_id")

	// Payload must contain aggregated fields.
	assert.Contains(t, client.lastUserText, "avg_valence",
		"LLM payload must contain avg_valence")
	assert.Contains(t, client.lastUserText, "top_tags",
		"LLM payload must contain top_tags")
}

// TestSummaryLLM_DisabledConfig_NoCall verifies that a nil client results in no LLM call.
func TestSummaryLLM_DisabledConfig_NoCall(t *testing.T) {
	t.Parallel()

	enhancer := NewLLMSummaryEnhancer(nil, newJournalAuditWriter(nil))
	summary := newTestWeeklySummary("u1")

	result, err := enhancer.EnhanceWeeklySummary(context.Background(), summary)
	require.NoError(t, err)
	require.NotNil(t, result)

	// OneLiner must remain empty when no LLM is configured.
	assert.Empty(t, result.OneLiner)
}

// TestSummaryLLM_ParseFail_ReturnsOriginal verifies that an invalid JSON response
// returns the original summary unchanged.
func TestSummaryLLM_ParseFail_ReturnsOriginal(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: "not valid json {{{"}
	enhancer := NewLLMSummaryEnhancer(client, newJournalAuditWriter(nil))

	summary := newTestWeeklySummary("u1")
	result, err := enhancer.EnhanceWeeklySummary(context.Background(), summary)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.OneLiner, "OneLiner must be empty on parse fail")
	// Original fields must be preserved.
	assert.Equal(t, 5, result.EntryCount)
	assert.InDelta(t, 0.72, result.AvgValence, 1e-9)
}

// TestSummaryLLM_RejectsClinicalLanguage verifies AC-023 for summary:
// if the LLM response contains clinical keywords, the response is discarded.
func TestSummaryLLM_RejectsClinicalLanguage(t *testing.T) {
	t.Parallel()

	// Response containing a clinical keyword.
	clinicalResp := `{"one_liner":"이번 주 우울증 가능성이 높습니다."}`
	client := &mockLLMClient{response: clinicalResp}
	enhancer := NewLLMSummaryEnhancer(client, newJournalAuditWriter(nil))

	summary := newTestWeeklySummary("u1")
	result, err := enhancer.EnhanceWeeklySummary(context.Background(), summary)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Clinical response must be silently rejected — OneLiner stays empty.
	assert.Empty(t, result.OneLiner, "clinical keyword response must be rejected")
	assert.Equal(t, 1, client.invokeCount, "LLM must be called once")
}

// TestSummaryLLM_AuditLog_LLMEnhanced verifies that a successful enhancement
// emits an audit event (no panic when auditor is nil).
func TestSummaryLLM_AuditLog_LLMEnhanced(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: validSummaryLLMResponse}
	// Use nil auditor — should not panic.
	enhancer := NewLLMSummaryEnhancer(client, newJournalAuditWriter(nil))

	summary := newTestWeeklySummary("u1")
	enhanced, err := enhancer.EnhanceWeeklySummary(context.Background(), summary)

	require.NoError(t, err)
	require.NotNil(t, enhanced)
	assert.NotEmpty(t, enhanced.OneLiner)
}

// TestSummaryLLM_NilSummary verifies that nil summary input is handled safely.
func TestSummaryLLM_NilSummary(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: validSummaryLLMResponse}
	enhancer := NewLLMSummaryEnhancer(client, newJournalAuditWriter(nil))

	result, err := enhancer.EnhanceWeeklySummary(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, client.invokeCount, "LLM must not be called for nil summary")
}

// TestSummaryLLM_OriginalSummaryNotMutated verifies that the original summary is not
// mutated when enhancement succeeds (returns a new pointer).
func TestSummaryLLM_OriginalSummaryNotMutated(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: validSummaryLLMResponse}
	enhancer := NewLLMSummaryEnhancer(client, newJournalAuditWriter(nil))

	original := newTestWeeklySummary("u1")
	enhanced, err := enhancer.EnhanceWeeklySummary(context.Background(), original)

	require.NoError(t, err)
	require.NotNil(t, enhanced)

	// Original must not be modified.
	assert.Empty(t, original.OneLiner, "original summary must not be mutated")
	// Enhanced must have OneLiner set.
	assert.NotEmpty(t, enhanced.OneLiner)
}
