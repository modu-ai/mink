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

// RealCollectorDeps holds the real collector instances needed by
// RealBriefingCollectorFactory. Callers populate each field from their
// dependency injection site (e.g., the cobra root command setup).
//
// T-302 / REQ-BR-064 / AC-013.
type RealCollectorDeps struct {
	// Weather is the real weather data collector.
	Weather *briefing.WeatherCollector
	// Journal is the real journal recall collector.
	Journal *briefing.JournalCollector
	// Date is the real date/calendar collector.
	Date *briefing.DateCollector
	// Mantra is the real daily mantra collector.
	Mantra *briefing.MantraCollector
	// Location is the user location string forwarded to the weather adapter.
	Location string
}

// RealBriefingCollectorFactory returns a BriefingCollectorFactory that wraps
// the concrete collector implementations supplied via deps. The returned
// factory is wired with the adapter layer (collect_adapters.go) so the
// Orchestrator receives properly typed Collector values.
//
// REQ-BR-001 (weather), REQ-BR-004 (journal recall), REQ-BR-007 (date),
// REQ-BR-010 (mantra). REQ-BR-064 / AC-013.
//
// @MX:ANCHOR: RealBriefingCollectorFactory is the production wiring entry point.
// @MX:REASON: [AUTO] Binds all 4 collector adapters; called by CLI root setup. REQ-BR-064.
func RealBriefingCollectorFactory(deps RealCollectorDeps) BriefingCollectorFactory {
	return func() (weather, journal, date, mantra briefing.Collector) {
		weather = briefing.NewWeatherCollectorAdapter(deps.Weather, deps.Location)
		journal = briefing.NewJournalCollectorAdapter(deps.Journal)
		date = briefing.NewDateCollectorAdapter(deps.Date)
		mantra = briefing.NewMantraCollectorAdapter(deps.Mantra)
		return weather, journal, date, mantra
	}
}
