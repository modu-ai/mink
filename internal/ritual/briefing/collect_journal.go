package briefing

import (
	"context"
	"time"
)

// JournalRecaller defines the interface for recalling journal entries.
// This abstraction allows tests to mock the journal recall service.
type JournalRecaller interface {
	FindAnniversaryEvents(ctx context.Context, userID string, today time.Time) ([]*MockAnniversaryEntry, error)
	GetWeeklyTrend(ctx context.Context, userID string, today time.Time) (*MockMoodTrend, error)
}

// JournalCollector collects journal recall information for the briefing.
type JournalCollector struct {
	recaller JournalRecaller
}

// NewJournalCollector creates a new JournalCollector.
func NewJournalCollector(recaller JournalRecaller) *JournalCollector {
	return &JournalCollector{recaller: recaller}
}

// Collect fetches journal recall data and returns a RecallModule.
// If fetch fails, it sets Offline=true.
// Status is "ok" if fetch succeeds, "offline" on error, "timeout" on context cancel.
func (c *JournalCollector) Collect(ctx context.Context, userID string, today time.Time) (*RecallModule, string) {
	module := &RecallModule{
		Anniversaries: nil,
		MoodTrend:     nil,
		Offline:       false,
	}

	// Check if context is already cancelled
	if ctx.Err() != nil {
		module.Offline = true
		return module, "timeout"
	}

	// Fetch anniversary events
	anniversaries, err := c.recaller.FindAnniversaryEvents(ctx, userID, today)
	if err != nil {
		module.Offline = true
		return module, "offline"
	}

	// Fetch weekly trend
	trend, err := c.recaller.GetWeeklyTrend(ctx, userID, today)
	if err != nil {
		module.Offline = true
		return module, "offline"
	}

	// Map mock types to briefing types
	if len(anniversaries) > 0 {
		module.Anniversaries = make([]*AnniversaryEntry, len(anniversaries))
		for i, a := range anniversaries {
			module.Anniversaries[i] = &AnniversaryEntry{
				YearsAgo:  a.YearsAgo,
				Date:      a.Date,
				Text:      a.Text,
				EmojiMood: a.EmojiMood,
				Anniversary: &Anniversary{
					Type: a.Anniversary.Type,
					Name: a.Anniversary.Name,
				},
			}
		}
	}

	if trend != nil {
		module.MoodTrend = &MoodTrend{
			Period:     trend.Period,
			AvgValence: trend.AvgValence,
			AvgArousal: trend.AvgArousal,
			Trend:      trend.Trend,
		}
	}

	return module, "ok"
}

// Mock journal data types for testing.

// MockAnniversaryEntry represents an anniversary entry (test/mock structure).
type MockAnniversaryEntry struct {
	YearsAgo    int
	Date        string
	Text        string
	EmojiMood   string
	Anniversary *MockAnniversary
}

// MockAnniversary represents an anniversary (test/mock structure).
type MockAnniversary struct {
	Type string
	Name string
}

// MockMoodTrend represents mood trend (test/mock structure).
type MockMoodTrend struct {
	Period     string
	AvgValence float64
	AvgArousal float64
	Trend      string
}
