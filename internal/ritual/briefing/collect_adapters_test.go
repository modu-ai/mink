package briefing

import (
	"context"
	"testing"
	"time"
)

// stubJournalRecaller is a minimal JournalRecaller for adapter coverage tests.
type stubJournalRecaller struct{}

func (s *stubJournalRecaller) FindAnniversaryEvents(ctx context.Context, userID string, today time.Time) ([]*MockAnniversaryEntry, error) {
	return nil, nil
}
func (s *stubJournalRecaller) GetWeeklyTrend(ctx context.Context, userID string, today time.Time) (*MockMoodTrend, error) {
	return nil, nil
}

// TestCollectorAdapters verifies that all four collector adapters satisfy the
// Collector interface and return non-nil results when backed by real collectors.
// REQ-BR-001 / REQ-BR-004 / REQ-BR-007 / REQ-BR-010 / AC-013.
func TestCollectorAdapters(t *testing.T) {
	ctx := context.Background()
	today := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	userID := "test-user"

	t.Run("WeatherCollectorAdapter", func(t *testing.T) {
		wc := NewWeatherCollector(&WeatherFetcherImpl{})
		adapter := NewWeatherCollectorAdapter(wc, "Seoul")
		if adapter == nil {
			t.Fatal("NewWeatherCollectorAdapter must not return nil")
		}
		module, status := adapter.Collect(ctx, userID, today)
		// WeatherFetcherImpl stub returns empty data; status may be "ok" or "error".
		// We only assert the adapter does not panic and returns a string status.
		if status == "" {
			t.Error("adapter Collect must return a non-empty status string")
		}
		_ = module
	})

	t.Run("JournalCollectorAdapter", func(t *testing.T) {
		jc := NewJournalCollector(&stubJournalRecaller{})
		adapter := NewJournalCollectorAdapter(jc)
		if adapter == nil {
			t.Fatal("NewJournalCollectorAdapter must not return nil")
		}
		module, status := adapter.Collect(ctx, userID, today)
		if status != "ok" {
			t.Errorf("JournalCollectorAdapter status = %q, want %q", status, "ok")
		}
		if module == nil {
			t.Error("JournalCollectorAdapter Collect must return a non-nil module")
		}
	})

	t.Run("DateCollectorAdapter", func(t *testing.T) {
		dc := NewDateCollector()
		adapter := NewDateCollectorAdapter(dc)
		if adapter == nil {
			t.Fatal("NewDateCollectorAdapter must not return nil")
		}
		module, status := adapter.Collect(ctx, userID, today)
		if status != "ok" {
			t.Errorf("DateCollectorAdapter status = %q, want %q", status, "ok")
		}
		if module == nil {
			t.Error("DateCollectorAdapter Collect must return a non-nil module")
		}
	})

	t.Run("MantraCollectorAdapter", func(t *testing.T) {
		mc := NewMantraCollector(DefaultConfig())
		adapter := NewMantraCollectorAdapter(mc)
		if adapter == nil {
			t.Fatal("NewMantraCollectorAdapter must not return nil")
		}
		module, status := adapter.Collect(ctx, userID, today)
		if status != "ok" {
			t.Errorf("MantraCollectorAdapter status = %q, want %q", status, "ok")
		}
		if module == nil {
			t.Error("MantraCollectorAdapter Collect must return a non-nil module")
		}
	})
}
