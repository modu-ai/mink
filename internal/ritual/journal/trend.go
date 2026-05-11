package journal

import (
	"context"
	"fmt"
	"math"
	"time"
)

// TrendAggregator computes mood trend statistics over a calendar window.
// It queries the journal storage directly, performing all aggregation in Go
// to remain compatible with the simplistic SQLite deployment (no complex window functions).
//
// @MX:ANCHOR: [AUTO] Trend aggregation entry point for weekly/monthly summaries
// @MX:REASON: Called by summary job, chart renderer, and tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-022, AC-025
type TrendAggregator struct {
	storage *Storage
}

// NewTrendAggregator constructs a TrendAggregator backed by the given storage.
func NewTrendAggregator(storage *Storage) *TrendAggregator {
	return &TrendAggregator{storage: storage}
}

// WeeklyTrend aggregates journal entries for userID over the 7-day window
// ending on today (inclusive). The From date is today minus 6 days.
//
// Missing days in the window are represented as math.NaN() in SparklinePoints.
// When EntryCount == 0, AvgValence is math.NaN().
func (a *TrendAggregator) WeeklyTrend(ctx context.Context, userID string, today time.Time) (*Trend, error) {
	from := today.AddDate(0, 0, -6)
	return a.aggregateWindow(ctx, userID, from, today, "week", 7)
}

// MonthlyTrend aggregates journal entries for userID over the 30-day window
// ending on today (inclusive). The From date is today minus 29 days.
func (a *TrendAggregator) MonthlyTrend(ctx context.Context, userID string, today time.Time) (*Trend, error) {
	from := today.AddDate(0, 0, -29)
	return a.aggregateWindow(ctx, userID, from, today, "month", 30)
}

// aggregateWindow is the shared implementation for weekly and monthly trends.
func (a *TrendAggregator) aggregateWindow(
	ctx context.Context,
	userID string,
	from, to time.Time,
	period string,
	days int,
) (*Trend, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	entries, err := a.storage.ListByDateRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("trend query: %w", err)
	}

	// Build a map from "YYYY-MM-DD" to the entries on that day.
	dayMap := make(map[string][]*StoredEntry, days)
	for _, e := range entries {
		key := e.Date.Format("2006-01-02")
		dayMap[key] = append(dayMap[key], e)
	}

	// Populate sparkline: one float per day in [from, from+1, ..., to].
	sparkline := make([]float64, days)
	for i := range days {
		day := from.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		dayEntries := dayMap[key]
		if len(dayEntries) == 0 {
			sparkline[i] = math.NaN()
			continue
		}
		// Use the average valence for entries on this day.
		sum := 0.0
		for _, e := range dayEntries {
			sum += e.Vad.Valence
		}
		sparkline[i] = sum / float64(len(dayEntries))
	}

	// Compute overall averages across all entries in the window.
	trend := &Trend{
		Period:           period,
		From:             from.Truncate(24 * time.Hour),
		To:               to.Truncate(24 * time.Hour),
		EntryCount:       len(entries),
		SparklinePoints:  sparkline,
		MoodDistribution: make(map[string]int),
	}

	if len(entries) == 0 {
		trend.AvgValence = math.NaN()
		trend.AvgArousal = math.NaN()
		trend.AvgDominance = math.NaN()
		return trend, nil
	}

	sumV, sumA, sumD := 0.0, 0.0, 0.0
	for _, e := range entries {
		sumV += e.Vad.Valence
		sumA += e.Vad.Arousal
		sumD += e.Vad.Dominance
		for _, tag := range e.EmotionTags {
			trend.MoodDistribution[tag]++
		}
	}
	n := float64(len(entries))
	trend.AvgValence = sumV / n
	trend.AvgArousal = sumA / n
	trend.AvgDominance = sumD / n

	return trend, nil
}
