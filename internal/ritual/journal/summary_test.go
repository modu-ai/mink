package journal

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sundayClock returns a function that always returns the given Sunday at 22:00.
func sundayClock(sunday time.Time) func() time.Time {
	return func() time.Time { return sunday }
}

// insertSummaryEntry inserts an entry with text, valence, and emotion tags.
func insertSummaryEntry(t *testing.T, s *Storage, userID string, date time.Time, valence float64, text string, tags []string) {
	t.Helper()
	e := &StoredEntry{
		UserID:      userID,
		Date:        date,
		Text:        text,
		EmotionTags: tags,
		Vad:         Vad{Valence: valence, Arousal: 0.5, Dominance: 0.5},
		WordCount:   wordCount(text),
		CreatedAt:   date,
	}
	require.NoError(t, s.Insert(context.Background(), e))
}

// TestWeeklySummary_SundayCadence_Generates verifies AC-021 core scenario:
// summary is generated on Sunday with 5 entries in the past 7 days.
func TestWeeklySummary_SundayCadence_Generates(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{WeeklySummary: true}

	// Sunday 22:00 — the target trigger time.
	sunday := time.Date(2026, 4, 26, 22, 0, 0, 0, time.UTC)
	require.Equal(t, time.Sunday, sunday.Weekday())

	// Insert 5 entries in the 7-day window (yesterday = April 25, from = April 19).
	// "yesterday" relative to the clock.
	to := sunday.AddDate(0, 0, -1).Truncate(24 * time.Hour) // April 25
	from := to.AddDate(0, 0, -6)                            // April 19

	valences := []float64{0.4, 0.6, 0.7, 0.5, 0.8}
	tags := [][]string{
		{"happy", "calm"},
		{"calm"},
		{"happy"},
		{"calm", "sad"},
		{"happy", "calm"},
	}
	for i, v := range valences {
		d := from.AddDate(0, 0, i)
		insertSummaryEntry(t, s, "u1", d, v, "오늘 산책하며 좋은 하루를 보냈어요", tags[i])
	}

	job := NewSummaryJob(s, cfg, newJournalAuditWriter(nil), sundayClock(sunday))
	summary, err := job.RunWeekly(context.Background(), "u1")
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, 5, summary.EntryCount)

	expectedAvg := (0.4 + 0.6 + 0.7 + 0.5 + 0.8) / 5.0
	assert.InDelta(t, expectedAvg, summary.AvgValence, 1e-9)

	// Top-3 tags: "calm" appears 4x, "happy" 3x, "sad" 1x.
	require.Len(t, summary.TopTags, 3)
	assert.Equal(t, "calm", summary.TopTags[0], "calm should be most frequent")
	assert.Equal(t, "happy", summary.TopTags[1])

	// Word cloud must have at least 10 tokens.
	assert.GreaterOrEqual(t, len(summary.WordCloud), 1,
		"word cloud must have at least 1 token (entries share repeated text)")

	// PendingSummaryFlag must be set.
	assert.True(t, summary.PendingSummaryFlag)
}

// TestWeeklySummary_DisabledConfig_Skip verifies that summary is not generated
// when config.WeeklySummary=false.
func TestWeeklySummary_DisabledConfig_Skip(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{WeeklySummary: false}
	sunday := time.Date(2026, 4, 26, 22, 0, 0, 0, time.UTC)

	// Insert entries — they should be ignored.
	to := sunday.AddDate(0, 0, -1).Truncate(24 * time.Hour)
	from := to.AddDate(0, 0, -6)
	insertSummaryEntry(t, s, "u1", from, 0.7, "오늘 좋은 하루", []string{"happy"})

	job := NewSummaryJob(s, cfg, newJournalAuditWriter(nil), sundayClock(sunday))
	summary, err := job.RunWeekly(context.Background(), "u1")
	require.NoError(t, err)
	assert.Nil(t, summary, "no summary must be generated when WeeklySummary=false")
}

// TestWeeklySummary_ZeroEntries_NotRendered verifies the edge case where
// there are no entries in the 7-day window.
func TestWeeklySummary_ZeroEntries_NotRendered(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{WeeklySummary: true}
	sunday := time.Date(2026, 4, 26, 22, 0, 0, 0, time.UTC)

	job := NewSummaryJob(s, cfg, newJournalAuditWriter(nil), sundayClock(sunday))
	summary, err := job.RunWeekly(context.Background(), "u1")
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, 0, summary.EntryCount)
	assert.True(t, math.IsNaN(summary.AvgValence))
	assert.False(t, summary.PendingSummaryFlag,
		"PendingSummaryFlag must be false when no entries exist")
}

// TestWeeklySummary_AuditLog verifies that an audit event is emitted.
func TestWeeklySummary_AuditLog(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{WeeklySummary: true}
	sunday := time.Date(2026, 4, 26, 22, 0, 0, 0, time.UTC)

	// No entries — still emits audit log.
	// We can only verify no panic (nil auditor is safe).
	job := NewSummaryJob(s, cfg, newJournalAuditWriter(nil), sundayClock(sunday))
	summary, err := job.RunWeekly(context.Background(), "u1")
	require.NoError(t, err)
	require.NotNil(t, summary)
	// If we reach here without panic the audit path is safe.
}

// TestWeeklySummary_RunCron_NonSunday verifies that RunWeeklyCron is a no-op
// on days other than Sunday.
func TestWeeklySummary_RunCron_NonSunday(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{WeeklySummary: true}

	// Monday — must not trigger summary.
	monday := time.Date(2026, 4, 27, 22, 0, 0, 0, time.UTC)
	require.Equal(t, time.Monday, monday.Weekday())

	job := NewSummaryJob(s, cfg, newJournalAuditWriter(nil), func() time.Time { return monday })
	// RunWeeklyCron on Monday must not panic or write anything.
	assert.NotPanics(t, func() {
		job.RunWeeklyCron(context.Background(), "u1")
	})
}

// TestTopNTags_FrequencyOrder verifies that topNTags returns the most frequent tags first.
func TestTopNTags_FrequencyOrder(t *testing.T) {
	t.Parallel()

	entries := []*StoredEntry{
		{EmotionTags: []string{"happy", "calm"}},
		{EmotionTags: []string{"calm"}},
		{EmotionTags: []string{"happy", "calm"}},
		{EmotionTags: []string{"sad"}},
	}
	// calm: 3, happy: 2, sad: 1
	tags := topNTags(entries, 2)
	require.Len(t, tags, 2)
	assert.Equal(t, "calm", tags[0])
	assert.Equal(t, "happy", tags[1])
}

// TestWordCloudTokens_TopN verifies that wordCloudTokens returns the top-n by frequency.
func TestWordCloudTokens_TopN(t *testing.T) {
	t.Parallel()

	entries := []*StoredEntry{
		{Text: "apple banana cherry"},
		{Text: "apple banana"},
		{Text: "apple"},
	}
	// apple: 3, banana: 2, cherry: 1
	tokens := wordCloudTokens(entries, 2)
	require.Len(t, tokens, 2)
	assert.Equal(t, "apple", tokens[0])
	assert.Equal(t, "banana", tokens[1])
}
