package briefing

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestOrchestrator_Integration_HappyPath tests the complete flow with all collectors succeeding.
// This is an end-to-end integration test verifying AC-001 happy path.
func TestOrchestrator_Integration_HappyPath(t *testing.T) {
	ctx := context.Background()
	userID := "test-user-123"
	today := time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC)

	// Create mock collectors with realistic data
	weather := &mockCollector{
		module: &WeatherModule{
			Current: &WeatherCurrent{
				Temp:      22.0,
				FeelsLike: 21.0,
				Humidity:  55.0,
				Condition: "Sunny",
				Location:  "Seoul",
			},
			AirQuality: &AirQuality{
				PM25:  10.0,
				PM10:  20.0,
				AQI:   30,
				Level: "Good",
			},
			Offline: false,
		},
		status: "ok",
	}

	journal := &mockCollector{
		module: &RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{
					YearsAgo:  1,
					Date:      "2025-05-14",
					Text:      "First anniversary",
					EmojiMood: "😊",
					Anniversary: &Anniversary{
						Type: "1Y",
						Name: "1 Year Ago",
					},
				},
			},
			MoodTrend: &MoodTrend{
				Period:     "7 days",
				AvgValence: 0.7,
				AvgArousal: 0.5,
				Trend:      "improving",
			},
			Offline: false,
		},
		status: "ok",
	}

	date := &mockCollector{
		module: &DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
			SolarTerm: &SolarTerm{
				Name:      "입하",
				NameHanja: "立夏",
				Date:      "2026-05-05",
			},
			Holiday: nil,
		},
		status: "ok",
	}

	mantra := &mockCollector{
		module: &MantraModule{
			Text:   "Every day is a new beginning",
			Source: "Daily Wisdom",
			Index:  134,
			Total:  365,
		},
		status: "ok",
	}

	orchestrator := NewOrchestrator(weather, journal, date, mantra)

	// Run orchestration
	payload, err := orchestrator.Run(ctx, userID, today)

	// Verify no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify payload is fully populated
	verifyWeatherModule(t, &payload.Weather, false)
	verifyJournalModule(t, &payload.JournalRecall, false)
	verifyDateModule(t, &payload.DateCalendar)
	verifyMantraModule(t, &payload.Mantra)

	// Verify all statuses are "ok"
	for module, status := range payload.Status {
		if status != "ok" {
			t.Errorf("expected module '%s' status='ok', got '%s'", module, status)
		}
	}

	// Verify timestamp
	if payload.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should be set")
	}
}

// TestOrchestrator_Integration_OfflinePath tests graceful degradation when collectors are offline.
// This verifies AC-002 offline path.
func TestOrchestrator_Integration_OfflinePath(t *testing.T) {
	ctx := context.Background()
	userID := "test-user-456"
	today := time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC)

	// Weather collector offline
	weather := &mockCollector{
		module: &WeatherModule{
			Offline: true,
		},
		status: "offline",
	}

	// Journal collector offline
	journal := &mockCollector{
		module: &RecallModule{
			Offline: true,
		},
		status: "offline",
	}

	// Date and Mantra still work
	date := &mockCollector{
		module: &DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
			SolarTerm: &SolarTerm{
				Name:      "입하",
				NameHanja: "立夏",
				Date:      "2026-05-05",
			},
		},
		status: "ok",
	}

	mantra := &mockCollector{
		module: &MantraModule{
			Text:  "Test mantra",
			Index: 0,
			Total: 1,
		},
		status: "ok",
	}

	orchestrator := NewOrchestrator(weather, journal, date, mantra)

	// Run orchestration
	payload, err := orchestrator.Run(ctx, userID, today)

	// Verify no error (graceful degradation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify offline modules
	if !payload.Weather.Offline {
		t.Error("Weather should be marked offline")
	}

	if payload.Status["weather"] != "offline" {
		t.Errorf("expected weather status='offline', got '%s'", payload.Status["weather"])
	}

	if !payload.JournalRecall.Offline {
		t.Error("Journal should be marked offline")
	}

	if payload.Status["journal"] != "offline" {
		t.Errorf("expected journal status='offline', got '%s'", payload.Status["journal"])
	}

	// Verify working modules still populated
	if payload.DateCalendar.Today != "2026-05-14" {
		t.Errorf("Date should still be populated, got: %s", payload.DateCalendar.Today)
	}

	if payload.Status["date"] != "ok" {
		t.Errorf("expected date status='ok', got '%s'", payload.Status["date"])
	}

	if payload.Mantra.Text != "Test mantra" {
		t.Errorf("Mantra should still be populated, got: %s", payload.Mantra.Text)
	}

	if payload.Status["mantra"] != "ok" {
		t.Errorf("expected mantra status='ok', got '%s'", payload.Status["mantra"])
	}
}

// TestOrchestrator_Integration_ErrorPath tests handling of collector errors.
func TestOrchestrator_Integration_ErrorPath(t *testing.T) {
	ctx := context.Background()
	userID := "test-user-789"
	today := time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC)

	// Create collectors that fail
	weather := &mockCollector{
		err: errors.New("weather service unavailable"),
	}

	journal := &mockCollector{
		err: errors.New("journal database error"),
	}

	date := &mockCollector{
		module: &DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
		status: "ok",
	}

	mantra := &mockCollector{
		err: errors.New("mantra service error"),
	}

	orchestrator := NewOrchestrator(weather, journal, date, mantra)

	// Run orchestration
	payload, err := orchestrator.Run(ctx, userID, today)

	// Should not error, but return partial payload
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify error statuses
	if payload.Status["weather"] != "error" {
		t.Errorf("expected weather status='error', got '%s'", payload.Status["weather"])
	}

	if payload.Status["journal"] != "error" {
		t.Errorf("expected journal status='error', got '%s'", payload.Status["journal"])
	}

	if payload.Status["mantra"] != "error" {
		t.Errorf("expected mantra status='error', got '%s'", payload.Status["mantra"])
	}

	// Date should still work
	if payload.Status["date"] != "ok" {
		t.Errorf("expected date status='ok', got '%s'", payload.Status["date"])
	}
}

// TestOrchestrator_Integration_TimeoutPath tests collector timeout handling.
func TestOrchestrator_Integration_TimeoutPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	userID := "test-user-timeout"
	today := time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC)

	// Weather collector will timeout
	weather := &mockCollector{
		delay:  200 * time.Millisecond,
		module: &WeatherModule{},
		status: "ok",
	}

	// Other collectors complete quickly
	journal := &mockCollector{
		module: &RecallModule{},
		status: "ok",
	}

	date := &mockCollector{
		module: &DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
		status: "ok",
	}

	mantra := &mockCollector{
		module: &MantraModule{
			Text: "Test",
		},
		status: "ok",
	}

	orchestrator := NewOrchestrator(weather, journal, date, mantra)

	// Run orchestration
	payload, err := orchestrator.Run(ctx, userID, today)

	// Should not error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Weather should timeout
	if payload.Status["weather"] != "timeout" {
		t.Errorf("expected weather status='timeout', got '%s'", payload.Status["weather"])
	}

	// Other modules should complete
	for _, module := range []string{"journal", "date", "mantra"} {
		if payload.Status[module] != "ok" {
			t.Errorf("expected %s status='ok', got '%s'", module, payload.Status[module])
		}
	}
}

// Helper functions for verification

func verifyWeatherModule(t *testing.T, w *WeatherModule, expectOffline bool) {
	t.Helper()

	if w.Offline != expectOffline {
		t.Errorf("expected Offline=%v, got %v", expectOffline, w.Offline)
	}

	if !expectOffline {
		if w.Current == nil {
			t.Error("Current should not be nil when online")
		} else {
			if w.Current.Temp == 0 {
				t.Error("Current.Temp should be set")
			}
		}

		if w.AirQuality == nil {
			t.Error("AirQuality should not be nil when online")
		}
	}
}

func verifyJournalModule(t *testing.T, r *RecallModule, expectOffline bool) {
	t.Helper()

	if r.Offline != expectOffline {
		t.Errorf("expected Offline=%v, got %v", expectOffline, r.Offline)
	}

	if !expectOffline {
		if r.Anniversaries == nil {
			t.Error("Anniversaries should not be nil when online")
		}

		if r.MoodTrend == nil {
			t.Error("MoodTrend should not be nil when online")
		} else {
			if r.MoodTrend.Period == "" {
				t.Error("MoodTrend.Period should be set")
			}
		}
	}
}

func verifyDateModule(t *testing.T, d *DateModule) {
	t.Helper()

	if d.Today == "" {
		t.Error("Today should be set")
	}

	if d.DayOfWeek == "" {
		t.Error("DayOfWeek should be set")
	}
}

func verifyMantraModule(t *testing.T, m *MantraModule) {
	t.Helper()

	if m.Text == "" {
		t.Error("Text should be set")
	}

	if m.Total == 0 {
		t.Error("Total should be set")
	}
}
