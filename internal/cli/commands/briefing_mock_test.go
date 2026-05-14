package commands

import (
	"context"
	"time"

	"github.com/modu-ai/mink/internal/ritual/briefing"
)

// MockBriefingCollectorFactory creates mock collectors for testing.
// Moved from briefing.go to this test file as part of T-301 (REQ-BR-064 / AC-013).
// All existing TestBriefingCommand_* tests continue to compile because this file
// is in the same package (commands).
func MockBriefingCollectorFactory() (weather, journal, date, mantra briefing.Collector) {
	weather = &mockCollector{
		module: &briefing.WeatherModule{
			Current: &briefing.WeatherCurrent{
				Temp:      18.5,
				FeelsLike: 17.0,
				Humidity:  65.0,
				Condition: "Cloudy",
				Location:  "Seoul, South Korea",
			},
			AirQuality: &briefing.AirQuality{
				PM25:  15.0,
				PM10:  25.0,
				AQI:   45,
				Level: "Good",
			},
			Offline: false,
		},
		status: "ok",
	}

	journal = &mockCollector{
		module: &briefing.RecallModule{
			Anniversaries: []*briefing.AnniversaryEntry{
				{
					YearsAgo:  1,
					Date:      "2025-05-14",
					Text:      "Project milestone",
					EmojiMood: "🎉",
					Anniversary: &briefing.Anniversary{
						Type: "1Y",
						Name: "1 Year Ago",
					},
				},
			},
			MoodTrend: &briefing.MoodTrend{
				Period:     "7 days",
				AvgValence: 0.6,
				AvgArousal: 0.4,
				Trend:      "improving",
			},
			Offline: false,
		},
		status: "ok",
	}

	date = &mockCollector{
		module: &briefing.DateModule{
			Today:     time.Now().Format("2006-01-02"),
			DayOfWeek: "\xeb\xaa\xa9\xec\x9a\x94\xec\x9d\xbc", // 목요일
			SolarTerm: &briefing.SolarTerm{
				Name:      "\xec\x9e\x85\xed\x95\x98", // 입하
				NameHanja: "\xe7\xab\x8b\xe5\xa4\x8f", // 立夏
				Date:      "2026-05-05",
			},
			Holiday: nil,
		},
		status: "ok",
	}

	mantra = &mockCollector{
		module: &briefing.MantraModule{
			Text:   "Every day is a new beginning",
			Source: "Daily Wisdom",
			Index:  0,
			Total:  365,
		},
		status: "ok",
	}

	return weather, journal, date, mantra
}

// mockCollector is a test double for the briefing.Collector interface.
// Moved from briefing.go to this test file as part of T-301 (REQ-BR-064 / AC-013).
type mockCollector struct {
	module any
	status string
	delay  time.Duration
	err    error
}

func (m *mockCollector) Collect(ctx context.Context, userID string, today time.Time) (any, string) {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, "timeout"
		case <-time.After(m.delay):
		}
	}

	if m.err != nil {
		return nil, "error"
	}

	return m.module, m.status
}
