package briefing

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRenderCLI_PlainText(t *testing.T) {
	now := time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC)

	payload := &BriefingPayload{
		Weather: WeatherModule{
			Current: &WeatherCurrent{
				Temp:      18.5,
				FeelsLike: 17.0,
				Humidity:  65.0,
				Condition: "Cloudy",
				Location:  "Seoul",
			},
			AirQuality: &AirQuality{
				PM25:  15.0,
				PM10:  25.0,
				AQI:   45,
				Level: "Good",
			},
			Offline: false,
		},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{
					YearsAgo:  1,
					Date:      "2025-05-14",
					Text:      "First journal entry",
					EmojiMood: "😊",
					Anniversary: &Anniversary{
						Type: "1Y",
						Name: "1 Year Ago",
					},
				},
			},
			MoodTrend: &MoodTrend{
				Period:     "7 days",
				AvgValence: 0.6,
				AvgArousal: 0.4,
				Trend:      "improving",
			},
			Offline: false,
		},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
			SolarTerm: &SolarTerm{
				Name:      "입하",
				NameHanja: "立夏",
				Date:      "2026-05-05",
			},
			Holiday: nil,
		},
		Mantra: MantraModule{
			Text:   "오늘도 좋은 하루가 되길",
			Source: "Daily Wisdom",
			Index:  0,
			Total:  365,
		},
		Status: map[string]string{
			"weather": "ok",
			"journal": "ok",
			"date":    "ok",
			"mantra":  "ok",
		},
		GeneratedAt: now,
	}

	output := RenderCLI(payload, true)

	// Verify no ANSI codes
	if strings.Contains(output, "\033[") {
		t.Errorf("plain output should not contain ANSI codes, got: %s", output)
	}

	// Verify key content
	checks := []struct {
		mustContain bool
		substring   string
	}{
		{true, "MORNING BRIEFING"},
		{true, "Generated:"},
		{true, "Weather"},
		{true, "Temperature:"},
		{true, "18.5°C"},
		{true, "Condition:"},
		{true, "Cloudy"},
		{true, "AQI:"},
		{true, "Journal Recall"},
		{true, "Anniversaries:"},
		{true, "1 found"},
		{true, "Mood Trend:"},
		{true, "improving"},
		{true, "Date & Calendar"},
		{true, "Today:"},
		{true, "2026-05-14"},
		{true, "Day of Week:"},
		{true, "목요일"},
		{true, "Solar Term:"},
		{true, "입하"},
		{true, "立夏"},
		{true, "Daily Mantra"},
		{true, "오늘도 좋은 하루가 되길"},
		{true, "Module Status:"},
		{false, "\033["}, // No ANSI
	}

	for _, check := range checks {
		contains := strings.Contains(output, check.substring)
		if contains != check.mustContain {
			t.Errorf("output mustContain=%v for '%s', got %v", check.mustContain, check.substring, contains)
		}
	}
}

func TestRenderCLI_OfflineStatus(t *testing.T) {
	now := time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC)

	payload := &BriefingPayload{
		Weather: WeatherModule{
			Offline: true,
		},
		JournalRecall: RecallModule{
			Offline: true,
		},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
		Mantra: MantraModule{
			Text: "Offline mantra",
		},
		Status: map[string]string{
			"weather": "offline",
			"journal": "offline",
			"date":    "ok",
			"mantra":  "ok",
		},
		GeneratedAt: now,
	}

	output := RenderCLI(payload, true)

	if !strings.Contains(output, "offline") {
		t.Errorf("output should indicate offline status")
	}

	if !strings.Contains(output, "2026-05-14") {
		t.Errorf("date module should still render")
	}

	if !strings.Contains(output, "Offline mantra") {
		t.Errorf("mantra should still render")
	}
}

func TestRenderCLI_ErrorStatus(t *testing.T) {
	now := time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC)

	payload := &BriefingPayload{
		Weather:       WeatherModule{},
		JournalRecall: RecallModule{},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
		Mantra: MantraModule{},
		Status: map[string]string{
			"weather": "error",
			"journal": "error",
			"date":    "ok",
			"mantra":  "error",
		},
		GeneratedAt: now,
	}

	output := RenderCLI(payload, true)

	errorCount := strings.Count(output, "error")
	if errorCount < 3 {
		t.Errorf("output should show error status for 3 modules, got %d", errorCount)
	}
}

func TestRenderCLI_Golden(t *testing.T) {
	now := time.Date(2026, 5, 14, 7, 30, 0, 0, time.UTC)

	payload := &BriefingPayload{
		Weather: WeatherModule{
			Current: &WeatherCurrent{
				Temp:      22.0,
				FeelsLike: 21.0,
				Humidity:  55.0,
				Condition: "Partly Cloudy",
				Location:  "Seoul, South Korea",
			},
			AirQuality: &AirQuality{
				PM25:  12.0,
				PM10:  20.0,
				AQI:   35,
				Level: "Good",
			},
			Offline: false,
		},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{
					YearsAgo:  3,
					Date:      "2023-05-14",
					Text:      "Project kickoff day",
					EmojiMood: "🚀",
					Anniversary: &Anniversary{
						Type: "3Y",
						Name: "3 Years Ago",
					},
				},
			},
			MoodTrend: &MoodTrend{
				Period:     "7 days",
				AvgValence: 0.7,
				AvgArousal: 0.5,
				Trend:      "stable",
			},
			Offline: false,
		},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
			SolarTerm: &SolarTerm{
				Name:      "입하",
				NameHanja: "立夏",
				Date:      "2026-05-05",
			},
			Holiday: nil,
		},
		Mantra: MantraModule{
			Text:   "Every day is a new beginning",
			Source: "Anonymous",
			Index:  134,
			Total:  365,
		},
		Status: map[string]string{
			"weather": "ok",
			"journal": "ok",
			"date":    "ok",
			"mantra":  "ok",
		},
		GeneratedAt: now,
	}

	output := RenderCLI(payload, true)

	goldenPath := "testdata/golden_cli_render.txt"
	t.Logf("Writing golden file to %s", goldenPath)
	if err := os.MkdirAll("testdata", 0755); err != nil {
		t.Fatalf("failed to create testdata dir: %v", err)
	}
	if err := os.WriteFile(goldenPath, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write golden file: %v", err)
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}

	if string(golden) != output {
		t.Errorf("golden file mismatch")
	}
}

func TestIsTerminal(t *testing.T) {
	if isTerminal(os.Stdout) {
		t.Log("stdout is a TTY")
	} else {
		t.Log("stdout is not a TTY")
	}

	sb := &strings.Builder{}
	if isTerminal(sb) {
		t.Error("strings.Builder should not be detected as terminal")
	}
}
