package briefing

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockJournalRecaller is a test double for JournalRecaller.
type mockJournalRecaller struct {
	anniversaries []*MockAnniversaryEntry
	trend         *MockMoodTrend
	err           error
}

func (m *mockJournalRecaller) FindAnniversaryEvents(ctx context.Context, userID string, today time.Time) ([]*MockAnniversaryEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.anniversaries, nil
}

func (m *mockJournalRecaller) GetWeeklyTrend(ctx context.Context, userID string, today time.Time) (*MockMoodTrend, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.trend, nil
}

func TestJournalCollector_Collect(t *testing.T) {
	t.Run("happy path - anniversary and trend", func(t *testing.T) {
		ctx := context.Background()
		userID := "user123"
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		recaller := &mockJournalRecaller{
			anniversaries: []*MockAnniversaryEntry{
				{
					YearsAgo:    1,
					Date:        "2025-05-14",
					Text:        "First journal entry",
					EmojiMood:   "😊",
					Anniversary: &MockAnniversary{Type: "1Y", Name: "1 Year Ago"},
				},
			},
			trend: &MockMoodTrend{
				Period:     "7 days",
				AvgValence: 0.7,
				AvgArousal: 0.6,
				Trend:      "improving",
			},
		}

		collector := NewJournalCollector(recaller)
		module, status := collector.Collect(ctx, userID, today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		if module.Offline {
			t.Error("expected Offline=false, got true")
		}

		if len(module.Anniversaries) != 1 {
			t.Errorf("expected 1 anniversary, got %d", len(module.Anniversaries))
		}

		if module.MoodTrend == nil {
			t.Fatal("expected MoodTrend to be populated")
		}
		if module.MoodTrend.AvgValence != 0.7 {
			t.Errorf("expected AvgValence=0.7, got %f", module.MoodTrend.AvgValence)
		}
	})

	t.Run("no entries - empty result", func(t *testing.T) {
		ctx := context.Background()
		userID := "user123"
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		recaller := &mockJournalRecaller{
			anniversaries: []*MockAnniversaryEntry{},
			trend:         nil,
		}

		collector := NewJournalCollector(recaller)
		module, status := collector.Collect(ctx, userID, today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		if module.Anniversaries != nil {
			t.Errorf("expected Anniversaries=nil when empty, got %v", module.Anniversaries)
		}

		if module.MoodTrend != nil {
			t.Error("expected MoodTrend=nil when no entries")
		}
	})

	t.Run("offline - error fetching", func(t *testing.T) {
		ctx := context.Background()
		userID := "user123"
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		recaller := &mockJournalRecaller{
			err: errors.New("journal storage unavailable"),
		}

		collector := NewJournalCollector(recaller)
		module, status := collector.Collect(ctx, userID, today)

		if status != "offline" {
			t.Errorf("expected status 'offline', got '%s'", status)
		}

		if !module.Offline {
			t.Error("expected Offline=true on error")
		}
	})

	t.Run("anniversary only - trend nil", func(t *testing.T) {
		ctx := context.Background()
		userID := "user123"
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		recaller := &mockJournalRecaller{
			anniversaries: []*MockAnniversaryEntry{
				{
					YearsAgo:    3,
					Date:        "2023-05-14",
					Text:        "Three years ago today",
					EmojiMood:   "🎉",
					Anniversary: &MockAnniversary{Type: "3Y", Name: "3 Years Ago"},
				},
			},
			trend: nil,
		}

		collector := NewJournalCollector(recaller)
		module, status := collector.Collect(ctx, userID, today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		if len(module.Anniversaries) != 1 {
			t.Errorf("expected 1 anniversary, got %d", len(module.Anniversaries))
		}

		if module.MoodTrend != nil {
			t.Error("expected MoodTrend=nil when trend is nil")
		}
	})
}
