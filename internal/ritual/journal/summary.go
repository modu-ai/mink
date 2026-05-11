package journal

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

// WeeklySummary holds the aggregated statistics for a 7-day journal period.
// LLM-assisted narrative summary is added in M3; M2 provides local aggregation only.
type WeeklySummary struct {
	// UserID is the user this summary was generated for.
	UserID string
	// From is the inclusive start of the 7-day window.
	From time.Time
	// To is the inclusive end of the 7-day window.
	To time.Time
	// EntryCount is the number of journal entries in the window.
	EntryCount int
	// AvgValence is the mean valence across all entries; math.NaN() when EntryCount == 0.
	AvgValence float64
	// TopTags holds the top-3 emotion tags by frequency.
	TopTags []string
	// WordCloud contains the top-10+ tokens by frequency across all entry texts.
	WordCloud []string
	// PendingSummaryFlag indicates that this summary should be shown in the next evening prompt.
	PendingSummaryFlag bool
}

// SummaryJob generates weekly summaries for a user.
// Summaries are local aggregation only (no LLM calls in M2).
//
// @MX:ANCHOR: [AUTO] Weekly summary generation entry point
// @MX:REASON: Called by RunWeeklyJob, orchestrator, and tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-018, AC-021
type SummaryJob struct {
	storage *Storage
	cfg     Config
	auditor *journalAuditWriter
	// clock provides the current time; use realClock in production, mock in tests.
	clock func() time.Time
}

// NewSummaryJob constructs a SummaryJob.
// Pass nil for clock to use time.Now.
func NewSummaryJob(storage *Storage, cfg Config, auditor *journalAuditWriter, clock func() time.Time) *SummaryJob {
	if clock == nil {
		clock = time.Now
	}
	return &SummaryJob{
		storage: storage,
		cfg:     cfg,
		auditor: auditor,
		clock:   clock,
	}
}

// RunWeekly generates a weekly summary for userID based on the 7-day window ending
// on the day before now (so "this week" means the completed 7 days).
//
// When config.WeeklySummary is false the function is a no-op and returns (nil, nil).
// When EntryCount == 0 the summary is not considered renderable (PendingSummaryFlag=false).
func (j *SummaryJob) RunWeekly(ctx context.Context, userID string) (*WeeklySummary, error) {
	if !j.cfg.WeeklySummary {
		return nil, nil
	}
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	now := j.clock()
	to := now.AddDate(0, 0, -1).Truncate(24 * time.Hour) // yesterday
	from := to.AddDate(0, 0, -6)                         // 7 days inclusive

	entries, err := j.storage.ListByDateRange(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}

	summary := &WeeklySummary{
		UserID:     userID,
		From:       from,
		To:         to,
		EntryCount: len(entries),
		AvgValence: math.NaN(),
	}

	if len(entries) == 0 {
		// Zero entries — emit no summary and do not set PendingSummaryFlag.
		j.auditor.emitOperation("weekly_summary_generated", userID, "ok_empty")
		return summary, nil
	}

	// Compute average valence.
	sumV := 0.0
	for _, e := range entries {
		sumV += e.Vad.Valence
	}
	summary.AvgValence = sumV / float64(len(entries))

	// Compute top-3 emotion tags by frequency.
	summary.TopTags = topNTags(entries, 3)

	// Build word cloud from all entry texts.
	summary.WordCloud = wordCloudTokens(entries, 10)

	// Mark as pending so the next evening prompt presents the summary.
	summary.PendingSummaryFlag = true

	j.auditor.emitOperation("weekly_summary_generated", userID, "ok")
	return summary, nil
}

// RunWeeklyCron is the cron-style entry point for the weekly summary job.
// It should be scheduled to run on Sunday at 22:00.
// In tests, supply a mock clock to set.clock.
func (j *SummaryJob) RunWeeklyCron(ctx context.Context, userID string) {
	now := j.clock()
	// Only execute on Sundays.
	if now.Weekday() != time.Sunday {
		return
	}
	_, _ = j.RunWeekly(ctx, userID)
}

// topNTags returns the top-n emotion tags by frequency across all entries.
// Tags are sorted by frequency descending then alphabetically for stability.
func topNTags(entries []*StoredEntry, n int) []string {
	freq := make(map[string]int)
	for _, e := range entries {
		for _, tag := range e.EmotionTags {
			freq[tag]++
		}
	}
	type kv struct {
		k string
		v int
	}
	sorted := make([]kv, 0, len(freq))
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].v != sorted[j].v {
			return sorted[i].v > sorted[j].v
		}
		return sorted[i].k < sorted[j].k
	})
	out := make([]string, 0, min(n, len(sorted)))
	for i := range min(n, len(sorted)) {
		out = append(out, sorted[i].k)
	}
	return out
}

// wordCloudTokens returns the top-n tokens by frequency across all entry texts.
// Tokens are whitespace-separated; stopwords are not filtered in M2.
func wordCloudTokens(entries []*StoredEntry, n int) []string {
	freq := make(map[string]int)
	for _, e := range entries {
		for tok := range strings.FieldsSeq(e.Text) {
			// Normalize to lowercase for token frequency.
			tok = strings.ToLower(tok)
			if tok != "" {
				freq[tok]++
			}
		}
	}
	type kv struct {
		k string
		v int
	}
	sorted := make([]kv, 0, len(freq))
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].v != sorted[j].v {
			return sorted[i].v > sorted[j].v
		}
		return sorted[i].k < sorted[j].k
	})
	out := make([]string, 0, min(n, len(sorted)))
	for i := range min(n, len(sorted)) {
		out = append(out, sorted[i].k)
	}
	return out
}
