package briefing

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockCollector is a test double for Collector interface.
type mockCollector struct {
	module any
	status string
	delay  time.Duration
	err    error
}

func (m *mockCollector) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	if m.delay > 0 {
		// Simulate slow collection
		select {
		case <-ctx.Done():
			return nil, "timeout"
		case <-time.After(m.delay):
			// Continue
		}
	}

	if m.err != nil {
		return nil, "error"
	}

	return m.module, m.status
}

func TestOrchestrator_Run(t *testing.T) {
	t.Run("happy path - all collectors succeed", func(t *testing.T) {
		ctx := context.Background()
		userID := "user123"
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		orchestrator := &Orchestrator{
			weather: &mockCollector{
				module: &WeatherModule{
					Current: &WeatherCurrent{Temp: 20.0},
					Offline: false,
				},
				status: "ok",
			},
			journal: &mockCollector{
				module: &RecallModule{
					Anniversaries: []*AnniversaryEntry{},
					Offline:       false,
				},
				status: "ok",
			},
			date: &mockCollector{
				module: &DateModule{
					Today:     "2026-05-14",
					DayOfWeek: "목요일",
				},
				status: "ok",
			},
			mantra: &mockCollector{
				module: &MantraModule{
					Text:  "오늘도 좋은 하루!",
					Index: 0,
					Total: 1,
				},
				status: "ok",
			},
		}

		payload, err := orchestrator.Run(ctx, userID, today)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if payload.Weather.Current.Temp != 20.0 {
			t.Errorf("expected Weather.Current.Temp=20.0, got %f", payload.Weather.Current.Temp)
		}

		if payload.DateCalendar.DayOfWeek != "목요일" {
			t.Errorf("expected DayOfWeek='목요일', got '%s'", payload.DateCalendar.DayOfWeek)
		}

		if payload.Mantra.Text != "오늘도 좋은 하루!" {
			t.Errorf("expected Mantra.Text='오늘도 좋은 하루!', got '%s'", payload.Mantra.Text)
		}

		// Check all statuses are "ok"
		for module, status := range payload.Status {
			if status != "ok" {
				t.Errorf("expected module '%s' status='ok', got '%s'", module, status)
			}
		}
	})

	t.Run("partial failure - one collector offline", func(t *testing.T) {
		ctx := context.Background()
		userID := "user123"
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		orchestrator := &Orchestrator{
			weather: &mockCollector{
				module: &WeatherModule{Offline: true},
				status: "offline",
			},
			journal: &mockCollector{
				module: &RecallModule{Offline: false},
				status: "ok",
			},
			date: &mockCollector{
				module: &DateModule{Today: "2026-05-14"},
				status: "ok",
			},
			mantra: &mockCollector{
				module: &MantraModule{Text: "test"},
				status: "ok",
			},
		}

		payload, err := orchestrator.Run(ctx, userID, today)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Weather should be offline
		if payload.Status["weather"] != "offline" {
			t.Errorf("expected weather status='offline', got '%s'", payload.Status["weather"])
		}

		// Other modules should be ok
		if payload.Status["journal"] != "ok" {
			t.Errorf("expected journal status='ok', got '%s'", payload.Status["journal"])
		}
	})

	t.Run("all collectors fail - minimal payload", func(t *testing.T) {
		ctx := context.Background()
		userID := "user123"
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		orchestrator := &Orchestrator{
			weather: &mockCollector{
				err: errors.New("weather failed"),
			},
			journal: &mockCollector{
				err: errors.New("journal failed"),
			},
			date: &mockCollector{
				err: errors.New("date failed"),
			},
			mantra: &mockCollector{
				err: errors.New("mantra failed"),
			},
		}

		payload, err := orchestrator.Run(ctx, userID, today)

		// Should not error, but return minimal payload
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// All statuses should be "error"
		for module, status := range payload.Status {
			if status != "error" {
				t.Errorf("expected module '%s' status='error', got '%s'", module, status)
			}
		}
	})

	t.Run("timeout - collector exceeds deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		userID := "user123"
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		orchestrator := &Orchestrator{
			weather: &mockCollector{
				delay:  200 * time.Millisecond, // Will timeout
				module: &WeatherModule{},
				status: "ok",
			},
			journal: &mockCollector{
				module: &RecallModule{},
				status: "ok",
			},
			date: &mockCollector{
				module: &DateModule{},
				status: "ok",
			},
			mantra: &mockCollector{
				module: &MantraModule{},
				status: "ok",
			},
		}

		payload, err := orchestrator.Run(ctx, userID, today)

		// Should not error, but weather should be timeout
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if payload.Status["weather"] != "timeout" {
			t.Errorf("expected weather status='timeout', got '%s'", payload.Status["weather"])
		}

		// Other modules should complete
		if payload.Status["journal"] != "ok" {
			t.Errorf("expected journal status='ok', got '%s'", payload.Status["journal"])
		}
	})
}
