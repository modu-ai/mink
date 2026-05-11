package journal

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertEntryOnDate inserts a StoredEntry with the given date and valence.
func insertEntryOnDate(t *testing.T, s *Storage, userID string, date time.Time, valence float64, tags []string) {
	t.Helper()
	e := &StoredEntry{
		UserID:      userID,
		Date:        date,
		Text:        "entry on " + date.Format("2006-01-02"),
		EmotionTags: tags,
		Vad:         Vad{Valence: valence, Arousal: 0.5, Dominance: 0.5},
		WordCount:   3,
		CreatedAt:   date,
	}
	require.NoError(t, s.Insert(context.Background(), e))
}

// TestWeeklyTrend_AggregationWithGaps verifies AC-025 core scenario:
// 6 entries over 7 days (one gap), correct averages and NaN sparkline.
func TestWeeklyTrend_AggregationWithGaps(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	agg := NewTrendAggregator(s)
	ctx := context.Background()

	// today = day 7 (index 6); window = day1..day7.
	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	// day1..day3 = April 16..18; day4 missing; day5..day7 = April 20..22
	valences := []float64{0.2, 0.4, 0.6, 0.5, 0.7, 0.8}
	offsets := []int{0, 1, 2, 4, 5, 6} // day4 (offset 3) is missing

	for i, v := range valences {
		d := today.AddDate(0, 0, -(6 - offsets[i]))
		insertEntryOnDate(t, s, "u1", d, v, []string{"happy"})
	}

	trend, err := agg.WeeklyTrend(ctx, "u1", today)
	require.NoError(t, err)

	assert.Equal(t, "week", trend.Period)
	assert.Equal(t, 6, trend.EntryCount)

	expectedAvg := (0.2 + 0.4 + 0.6 + 0.5 + 0.7 + 0.8) / 6.0
	assert.InDelta(t, expectedAvg, trend.AvgValence, 1e-9)

	require.Len(t, trend.SparklinePoints, 7)

	// day4 (index 3) must be NaN.
	assert.True(t, math.IsNaN(trend.SparklinePoints[3]),
		"missing day must have NaN sparkline; got %v", trend.SparklinePoints[3])

	// All other days must have valid floats.
	validIndices := []int{0, 1, 2, 4, 5, 6}
	for _, idx := range validIndices {
		assert.False(t, math.IsNaN(trend.SparklinePoints[idx]),
			"day at index %d should not be NaN", idx)
	}

	// MoodDistribution sum must equal EntryCount.
	total := 0
	for _, count := range trend.MoodDistribution {
		total += count
	}
	assert.Equal(t, trend.EntryCount, total, "MoodDistribution sum must equal EntryCount")
}

// TestWeeklyTrend_EmptyEntries_AllNaN verifies the edge case where no entries
// exist in the 7-day window.
func TestWeeklyTrend_EmptyEntries_AllNaN(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	agg := NewTrendAggregator(s)
	ctx := context.Background()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	trend, err := agg.WeeklyTrend(ctx, "u1", today)
	require.NoError(t, err)

	assert.Equal(t, 0, trend.EntryCount)
	assert.True(t, math.IsNaN(trend.AvgValence), "AvgValence must be NaN for zero entries")

	require.Len(t, trend.SparklinePoints, 7)
	for i, p := range trend.SparklinePoints {
		assert.True(t, math.IsNaN(p), "sparkline[%d] must be NaN for empty window", i)
	}
}

// TestMonthlyTrend_30Days verifies that MonthlyTrend uses a 30-day window.
func TestMonthlyTrend_30Days(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	agg := NewTrendAggregator(s)
	ctx := context.Background()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)

	// Insert entries at day 1 and day 30 of the window.
	firstDay := today.AddDate(0, 0, -29) // earliest day in 30-day window
	insertEntryOnDate(t, s, "u1", firstDay, 0.6, []string{"calm"})
	insertEntryOnDate(t, s, "u1", today, 0.8, []string{"happy"})

	trend, err := agg.MonthlyTrend(ctx, "u1", today)
	require.NoError(t, err)

	assert.Equal(t, "month", trend.Period)
	assert.Equal(t, 2, trend.EntryCount)
	require.Len(t, trend.SparklinePoints, 30)

	// First sparkline index (day 0 = firstDay) must be valid.
	assert.False(t, math.IsNaN(trend.SparklinePoints[0]))
	// Last sparkline index (day 29 = today) must be valid.
	assert.False(t, math.IsNaN(trend.SparklinePoints[29]))

	assert.InDelta(t, (0.6+0.8)/2.0, trend.AvgValence, 1e-9)
}

// TestWeeklyTrend_InvalidUserID verifies ErrInvalidUserID is returned.
func TestWeeklyTrend_InvalidUserID(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	agg := NewTrendAggregator(s)

	_, err := agg.WeeklyTrend(context.Background(), "", time.Now())
	assert.ErrorIs(t, err, ErrInvalidUserID)
}

// TestWeeklyTrend_MultipleEntriesOnSameDay verifies that sparkline uses the
// per-day average valence when multiple entries exist on the same day.
func TestWeeklyTrend_MultipleEntriesOnSameDay(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	agg := NewTrendAggregator(s)
	ctx := context.Background()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)

	// Two entries on the last day.
	insertEntryOnDate(t, s, "u1", today, 0.4, nil)
	insertEntryOnDate(t, s, "u1", today, 0.8, nil)

	trend, err := agg.WeeklyTrend(ctx, "u1", today)
	require.NoError(t, err)

	assert.Equal(t, 2, trend.EntryCount)
	// Sparkline for last day (index 6) must be avg of 0.4 and 0.8 = 0.6.
	assert.InDelta(t, 0.6, trend.SparklinePoints[6], 1e-9)
}
