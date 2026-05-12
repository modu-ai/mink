package insights

import (
	"context"
	"time"

	"github.com/modu-ai/mink/internal/learning/compressor"
	"github.com/modu-ai/mink/internal/learning/trajectory"
	"go.uber.org/zap"
)

// InsightsConfig configures the InsightsEngine.
type InsightsConfig struct {
	// MinkHome is the base directory for trajectory files (e.g. $HOME/.goose).
	MinkHome string

	// TelemetryEnabled mirrors config.telemetry.trajectory.enabled.
	// When false, Extract returns an empty Report without error.
	TelemetryEnabled bool

	// Pricing overrides the default pricing table.
	// When nil, DefaultPricingTable() is used.
	Pricing PricingTable

	// AvailableTools is the list of tool names exposed by memory providers.
	// Used by the Opportunity analyzer.
	AvailableTools []string
}

// ExtractOptions controls per-call behavior of Extract.
type ExtractOptions struct {
	// UseLLMSummary enables LLM-generated narratives via Summarizer.
	// Default false: all narratives are heuristic-generated.
	UseLLMSummary bool

	// Summarizer is used when UseLLMSummary == true.
	// If nil and UseLLMSummary is true, narratives fall back to heuristics.
	Summarizer compressor.Summarizer

	// ConfidenceMin filters insights below this threshold (default 0).
	ConfidenceMin float64
}

// ExtractOption is a functional option for Extract.
type ExtractOption func(*ExtractOptions)

// WithLLMSummary enables LLM-based narrative generation.
func WithLLMSummary(s compressor.Summarizer) ExtractOption {
	return func(o *ExtractOptions) {
		o.UseLLMSummary = true
		o.Summarizer = s
	}
}

// WithConfidenceMin sets a minimum confidence threshold for returned insights.
func WithConfidenceMin(min float64) ExtractOption {
	return func(o *ExtractOptions) {
		o.ConfidenceMin = min
	}
}

// InsightsEngine extracts quantitative and qualitative insights from trajectories.
//
// @MX:ANCHOR: [AUTO] Primary public API for the insights subsystem.
// @MX:REASON: Extract is called by CLI-001 and will be consumed by REFLECT-001.
// Any signature change breaks both downstream consumers.
// @MX:SPEC: SPEC-GOOSE-INSIGHTS-001
type InsightsEngine struct {
	cfg      InsightsConfig
	reader   *TrajectoryReader
	pricing  PricingTable
	analyzer *Analyzer
	logger   *zap.Logger
}

// New creates a new InsightsEngine.
func New(cfg InsightsConfig, logger *zap.Logger) *InsightsEngine {
	pricing := cfg.Pricing
	if pricing == nil {
		pricing = DefaultPricingTable()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// MinkHome is used directly as the trajectory base directory.
	// The caller is responsible for providing the correct path
	// (e.g. $HOME/.goose/trajectories or a test temp dir).
	trajDir := cfg.MinkHome
	if trajDir == "" {
		trajDir = "trajectories"
	}

	return &InsightsEngine{
		cfg:      cfg,
		reader:   NewTrajectoryReader(trajDir, logger),
		pricing:  pricing,
		analyzer: NewAnalyzer(),
		logger:   logger,
	}
}

// Extract scans trajectories for the given period and returns a consolidated Report.
// Returns ErrInvalidPeriod if period.From > period.To.
// Returns an empty Report (Empty=true) when telemetry is disabled or no data found.
// Does NOT invoke LLM unless opts include WithLLMSummary (REQ-INSIGHTS-013).
//
// @MX:ANCHOR: [AUTO] Core extraction method; entry point for all insight computation.
// @MX:REASON: Called by CLI-001 (/goose insights) and REFLECT-001. Must remain stable.
// @MX:SPEC: SPEC-GOOSE-INSIGHTS-001
func (e *InsightsEngine) Extract(
	ctx context.Context,
	period InsightsPeriod,
	optFns ...ExtractOption,
) (*Report, error) {
	// Apply options.
	opts := &ExtractOptions{}
	for _, fn := range optFns {
		fn(opts)
	}

	// Validate period.
	if err := period.Validate(); err != nil {
		return nil, err
	}

	// Short-circuit when telemetry is disabled.
	if !e.cfg.TelemetryEnabled {
		e.logger.Info("trajectory telemetry disabled, returning empty report",
			zap.Time("period_from", period.From),
			zap.Time("period_to", period.To))
		return emptyReport(period), nil
	}

	// Collect all trajectories in the period.
	trajectories := e.collectTrajectories(ctx, period)

	if len(trajectories) == 0 {
		return emptyReport(period), nil
	}

	e.logger.Info("insights extraction started",
		zap.Time("period_from", period.From),
		zap.Time("period_to", period.To),
		zap.Int("sessions", len(trajectories)))

	report := &Report{
		Period:      period,
		GeneratedAt: time.Now().UTC(),
	}

	// Quantitative aggregations (deterministic, no LLM).
	report.Overview = aggregateOverview(trajectories, e.pricing)
	report.Models = aggregateModels(trajectories, e.pricing)
	report.Tools = aggregateTools(trajectories)
	report.Activity = aggregateActivity(trajectories)

	// Qualitative analysis (heuristic by default).
	report.Insights = e.analyzer.Analyze(trajectories, e.cfg.AvailableTools)

	// Filter insights below confidence minimum.
	if opts.ConfidenceMin > 0 {
		for cat, insights := range report.Insights {
			filtered := insights[:0]
			for _, ins := range insights {
				if ins.Confidence >= opts.ConfidenceMin {
					filtered = append(filtered, ins)
				}
			}
			report.Insights[cat] = filtered
		}
	}

	// LLM narrative enrichment is opt-in (REQ-INSIGHTS-013).
	if opts.UseLLMSummary && opts.Summarizer != nil {
		e.enrichNarratives(ctx, report, opts.Summarizer)
	}

	return report, nil
}

// collectTrajectories scans both success and failed buckets for the period.
func (e *InsightsEngine) collectTrajectories(ctx context.Context, period InsightsPeriod) []*trajectory.Trajectory {
	ch := e.reader.ScanPeriod(period, "")
	var result []*trajectory.Trajectory
	for {
		select {
		case <-ctx.Done():
			return result
		case t, ok := <-ch:
			if !ok {
				return result
			}
			result = append(result, t)
		}
	}
}

// enrichNarratives calls Summarizer to replace heuristic narratives (opt-in).
// This is a best-effort operation; errors are logged but do not fail Extract.
func (e *InsightsEngine) enrichNarratives(ctx context.Context, report *Report, summarizer compressor.Summarizer) {
	for cat, insights := range report.Insights {
		for i := range insights {
			if len(insights[i].Evidence) == 0 {
				continue
			}
			// Build a minimal turn list for summarization.
			var turns []trajectory.TrajectoryEntry
			for _, ev := range insights[i].Evidence {
				turns = append(turns, trajectory.TrajectoryEntry{
					From:  trajectory.RoleHuman,
					Value: ev.Snippet,
				})
			}
			narrative, err := summarizer.Summarize(ctx, turns, 100)
			if err != nil {
				e.logger.Warn("LLM narrative enrichment failed",
					zap.String("category", cat.String()),
					zap.String("title", insights[i].Title),
					zap.Error(err))
				continue
			}
			insights[i].Narrative = narrative
		}
		report.Insights[cat] = insights
	}
}

// emptyReport returns a Report with Empty=true and the given period.
func emptyReport(period InsightsPeriod) *Report {
	return &Report{
		Period:      period,
		Empty:       true,
		Overview:    Overview{TotalSessions: 0},
		Models:      []ModelStat{},
		Tools:       []ToolStat{},
		Activity:    newEmptyActivity(),
		Insights:    map[InsightCategory][]Insight{},
		GeneratedAt: time.Now().UTC(),
	}
}

// newEmptyActivity returns an ActivityPattern with correctly initialized slices.
func newEmptyActivity() ActivityPattern {
	byDay := make([]DayBucket, 7)
	byHour := make([]HourBucket, 24)
	for i, label := range dayLabels {
		byDay[i].Day = label
	}
	for i := range byHour {
		byHour[i].Hour = i
	}
	return ActivityPattern{ByDay: byDay, ByHour: byHour}
}
