package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/modu-ai/mink/internal/ritual/briefing"
	"github.com/spf13/cobra"
)

// BriefingCollectorFactory creates collectors for briefing modules.
// This factory allows mocking in tests and future real wiring.
type BriefingCollectorFactory func() (weather, journal, date, mantra briefing.Collector)

// NewBriefingCommand creates the briefing subcommand.
// @MX:NOTE This command uses mock collectors in M1; real wiring will be added in M2.
func NewBriefingCommand(factory BriefingCollectorFactory) *cobra.Command {
	var plain bool
	var channels []string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "briefing",
		Short: "Generate morning briefing with weather, journal recall, date, and mantra",
		Long: `Generate a morning briefing that combines:
  - Current weather conditions and air quality
  - Journal recall (anniversaries and mood trend)
  - Date and calendar information (solar terms, holidays)
  - Daily mantra

Output is formatted for terminal display with ANSI colors and emoji by default.
Use --plain for plain text output suitable for logging or pipes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			userID := "default-user" // TODO: Get from config or flag in M2
			today := time.Now().Truncate(24 * time.Hour)

			// Create collectors using factory
			weather, journal, date, mantra := factory()

			// Create orchestrator
			orchestrator := briefing.NewOrchestrator(weather, journal, date, mantra)

			// Run collectors
			payload, err := orchestrator.Run(ctx, userID, today)
			if err != nil {
				return fmt.Errorf("orchestration failed: %w", err)
			}

			// Render output
			output := briefing.RenderCLI(payload, plain)
			fmt.Fprint(cmd.OutOrStdout(), output)

			return nil
		},
	}

	// Flags
	cmd.Flags().BoolVar(&plain, "plain", false, "disable ANSI colors and emoji (plain text output)")
	cmd.Flags().StringSliceVar(&channels, "channels", []string{}, "specific channels to collect (default: all)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "skip actual collection and show configuration")

	return cmd
}

// MockBriefingCollectorFactory creates mock collectors for testing.
// This function simulates successful collection for all modules.
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
			DayOfWeek: "목요일",
			SolarTerm: &briefing.SolarTerm{
				Name:      "입하",
				NameHanja: "立夏",
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

// mockCollector is a test double for Collector interface.
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
