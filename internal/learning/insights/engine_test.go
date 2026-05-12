package insights

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/learning/trajectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RED #1: TestOverview_DeterministicAggregate — AC-INSIGHTS-001
func TestOverview_DeterministicAggregate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Write 5 trajectories: total input=10,000 output=3,000 duration=300s.
	trajectories := make([]*trajectory.Trajectory, 5)
	ts := time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)
	for i := range trajectories {
		trajectories[i] = makeTrajectory(
			"session-"+string(rune('A'+i)),
			"anthropic/claude-sonnet-4-6",
			ts.Add(time.Duration(i)*time.Hour),
			true,
			2000,  // 5×2000 = 10,000 input tokens
			600,   // 5×600  = 3,000 output tokens
			60000, // 5×60,000ms = 300s total
		)
	}
	writeTestJSONLFileWithTrajectories(t, dir, "success", "2026-04-15.jsonl", trajectories)

	cfg := InsightsConfig{
		TelemetryEnabled: true,
		MinkHome:         dir,
	}
	engine := New(cfg, nil)
	period := Last(30)

	// Call twice for determinism check.
	r1, err1 := engine.Extract(t.Context(), period)
	r2, err2 := engine.Extract(t.Context(), period)

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NotNil(t, r1)
	require.NotNil(t, r2)

	// AC-INSIGHTS-001: both calls produce identical Overview.
	assert.Equal(t, r1.Overview, r2.Overview, "Overview must be deterministic")
	assert.Equal(t, 5, r1.Overview.TotalSessions)
	assert.Equal(t, 13_000, r1.Overview.TotalTokens, "10,000 input + 3,000 output")
	assert.InDelta(t, 300.0/3600.0, r1.Overview.TotalHours, 0.001, "300 seconds = 1/12 hour")
}

// RED #2: TestModels_TokenDescSort — AC-INSIGHTS-002
func TestModels_TokenDescSort(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create 12 sessions across 15 models.
	ts := time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)
	var trajectories []*trajectory.Trajectory

	// 15 different model names, varying token counts.
	models := []struct {
		name   string
		input  int
		output int
	}{
		{"model-01", 5000, 2000},
		{"model-02", 4000, 1800},
		{"model-03", 3000, 1500},
		{"model-04", 2500, 1200},
		{"model-05", 2000, 1000},
		{"model-06", 1800, 900},
		{"model-07", 1500, 750},
		{"model-08", 1200, 600},
		{"model-09", 1000, 500},
		{"model-10", 800, 400},
		{"model-11", 600, 300},
		{"model-12", 500, 250},
		{"model-13", 400, 200},
		{"model-14", 300, 150},
		{"model-15", 200, 100},
	}
	for i, m := range models {
		tr := makeTrajectory(
			"s-"+m.name,
			m.name,
			ts.Add(time.Duration(i)*time.Minute),
			true,
			m.input,
			m.output,
			10000,
		)
		trajectories = append(trajectories, tr)
	}
	writeTestJSONLFileWithTrajectories(t, dir, "success", "2026-04-15.jsonl", trajectories)

	cfg := InsightsConfig{TelemetryEnabled: true, MinkHome: dir}
	engine := New(cfg, nil)

	report, err := engine.Extract(t.Context(), Last(30))
	require.NoError(t, err)

	// AC-INSIGHTS-002: 15 models total, sorted by token count descending.
	assert.Len(t, report.Models, 15)
	for i := 0; i < len(report.Models)-1; i++ {
		assert.GreaterOrEqual(t, report.Models[i].TotalTokens, report.Models[i+1].TotalTokens,
			"models must be sorted by TotalTokens descending at index %d vs %d", i, i+1)
	}
}

// RED #3: TestTools_CountPercentageRounding — AC-INSIGHTS-003
func TestTools_CountPercentageRounding(t *testing.T) {
	t.Parallel()

	// tool A: 40, tool B: 30, tool C: 30 → total 100.
	trajectories := []*trajectory.Trajectory{
		{
			SessionID: "s1",
			Timestamp: time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC),
			Model:     "model-a",
			Completed: true,
			Conversations: buildToolConversations(
				map[string]int{"A": 40, "B": 30, "C": 30},
			),
			Metadata: trajectory.TrajectoryMetadata{TurnCount: 100, DurationMs: 1000},
		},
	}

	tools := aggregateTools(trajectories)

	require.NotEmpty(t, tools)
	// Top tool is A (40 calls).
	assert.Equal(t, "A", tools[0].Tool)
	assert.Equal(t, 40, tools[0].Count)
	assert.InDelta(t, 40.00, tools[0].Percentage, 0.01)

	// B and C have 30 each → 30.00%.
	found := make(map[string]ToolStat)
	for _, ts := range tools {
		found[ts.Tool] = ts
	}
	assert.InDelta(t, 30.00, found["B"].Percentage, 0.01)
	assert.InDelta(t, 30.00, found["C"].Percentage, 0.01)
}

// RED #4: TestActivity_ByDay_MonFirst — AC-INSIGHTS-004
func TestActivity_ByDay_MonFirst(t *testing.T) {
	t.Parallel()

	// Build trajectories: Mon=10, Tue=5, Wed=0, Thu=0, Fri=0, Sat=0, Sun=8.
	// 2026-04-13 is Monday, 2026-04-14 is Tuesday, 2026-04-19 is Sunday.
	var trajectories []*trajectory.Trajectory
	monDate := time.Date(2026, 4, 13, 9, 0, 0, 0, time.UTC) // Monday
	tueDate := time.Date(2026, 4, 14, 9, 0, 0, 0, time.UTC) // Tuesday
	sunDate := time.Date(2026, 4, 19, 9, 0, 0, 0, time.UTC) // Sunday

	for i := 0; i < 10; i++ {
		trajectories = append(trajectories, makeTrajectory(
			"mon-"+string(rune('a'+i)), "model", monDate.Add(time.Duration(i)*time.Minute), true, 100, 50, 1000,
		))
	}
	for i := 0; i < 5; i++ {
		trajectories = append(trajectories, makeTrajectory(
			"tue-"+string(rune('a'+i)), "model", tueDate.Add(time.Duration(i)*time.Minute), true, 100, 50, 1000,
		))
	}
	for i := 0; i < 8; i++ {
		trajectories = append(trajectories, makeTrajectory(
			"sun-"+string(rune('a'+i)), "model", sunDate.Add(time.Duration(i)*time.Minute), true, 100, 50, 1000,
		))
	}

	activity := aggregateActivity(trajectories)

	// AC-INSIGHTS-004: ByDay length == 7, Mon-first.
	assert.Len(t, activity.ByDay, 7)
	assert.Equal(t, "Mon", activity.ByDay[0].Day)
	assert.Equal(t, "Tue", activity.ByDay[1].Day)
	assert.Equal(t, "Sun", activity.ByDay[6].Day)

	assert.Equal(t, 10, activity.ByDay[0].Count, "Mon should have 10")
	assert.Equal(t, 5, activity.ByDay[1].Count, "Tue should have 5")
	assert.Equal(t, 0, activity.ByDay[2].Count, "Wed should be 0")
	assert.Equal(t, 8, activity.ByDay[6].Count, "Sun should have 8")

	assert.Equal(t, "Mon", activity.BusiestDay.Day)
}

// RED #5: TestActivity_ByHour_0to23 — AC-INSIGHTS-005
func TestActivity_ByHour_0to23(t *testing.T) {
	t.Parallel()

	// Sessions at various hours.
	var trajectories []*trajectory.Trajectory
	base := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	for h := 0; h < 24; h++ {
		trajectories = append(trajectories, makeTrajectory(
			"h-"+string(rune('A'+h%26)),
			"model",
			base.Add(time.Duration(h)*time.Hour),
			true, 100, 50, 1000,
		))
	}

	activity := aggregateActivity(trajectories)

	// AC-INSIGHTS-005: ByHour length == 24, indices 0-23.
	assert.Len(t, activity.ByHour, 24)
	for i, b := range activity.ByHour {
		assert.Equal(t, i, b.Hour)
	}
	assert.GreaterOrEqual(t, activity.BusiestHour.Hour, 0)
	assert.LessOrEqual(t, activity.BusiestHour.Hour, 23)
}

// RED #6: TestActivity_MaxStreak_ConsecutiveDays — AC-INSIGHTS-006
func TestActivity_MaxStreak_ConsecutiveDays(t *testing.T) {
	t.Parallel()

	// Active dates: 2026-04-15, 16, 17 (3 consecutive), then 20, 21, 22, 23, 24 (5 consecutive).
	activeDates := []time.Time{
		time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC),
	}
	var trajectories []*trajectory.Trajectory
	for i, d := range activeDates {
		trajectories = append(trajectories, makeTrajectory(
			"s-"+string(rune('A'+i)), "model", d, true, 100, 50, 1000,
		))
	}

	activity := aggregateActivity(trajectories)

	// AC-INSIGHTS-006: MaxStreak == 5 (Apr 20-24), ActiveDays == 8.
	assert.Equal(t, 5, activity.MaxStreak)
	assert.Equal(t, 8, activity.ActiveDays)
}

// RED #7: TestModels_PricingMissing_NA — AC-INSIGHTS-007
func TestModels_PricingMissing_NA(t *testing.T) {
	t.Parallel()

	pricing := PricingTable{
		"anthropic/claude-opus-4-7": {InputPer1K: 0.015, OutputPer1K: 0.075},
	}

	trajectories := []*trajectory.Trajectory{
		makeTrajectory("s1", "anthropic/claude-opus-4-7", time.Now(), true, 1000, 500, 1000),
		makeTrajectory("s2", "unknown-model", time.Now(), true, 1000, 500, 1000),
	}

	models := aggregateModels(trajectories, pricing)

	found := make(map[string]ModelStat)
	for _, m := range models {
		found[m.Model] = m
	}

	// AC-INSIGHTS-007: known model has pricing, unknown does not.
	opus, ok := found["anthropic/claude-opus-4-7"]
	require.True(t, ok)
	assert.True(t, opus.HasPricing)
	assert.Greater(t, opus.Cost, 0.0)

	unknown, ok := found["unknown-model"]
	require.True(t, ok)
	assert.False(t, unknown.HasPricing)
	assert.Equal(t, 0.0, unknown.Cost)
}

// RED #10: TestExtract_EmptyTrajectories_NoError — AC-INSIGHTS-010
func TestExtract_EmptyTrajectories_NoError(t *testing.T) {
	t.Parallel()

	// Test with telemetry disabled.
	cfg := InsightsConfig{TelemetryEnabled: false}
	engine := New(cfg, nil)

	report, err := engine.Extract(t.Context(), Last(7))

	require.NoError(t, err, "disabled telemetry must not return error")
	require.NotNil(t, report)
	assert.True(t, report.Empty)
	assert.Equal(t, 0, report.Overview.TotalSessions)
}

// TestExtract_EmptyDir_NoError tests with empty trajectory directory.
func TestExtract_EmptyDir_NoError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := InsightsConfig{TelemetryEnabled: true, MinkHome: dir}
	engine := New(cfg, nil)

	report, err := engine.Extract(t.Context(), Last(7))
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Empty)
	assert.Equal(t, 0, report.Overview.TotalSessions)
}

// RED #17: TestExtract_LLMSummaryOffByDefault — AC-INSIGHTS-017
func TestExtract_LLMSummaryOffByDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestJSONLFile(t, dir, "success", "2026-04-15.jsonl", 2)

	cfg := InsightsConfig{TelemetryEnabled: true, MinkHome: dir}
	engine := New(cfg, nil)

	mock := &mockSummarizer{}

	// Extract WITHOUT WithLLMSummary option — mock must never be called.
	report, err := engine.Extract(t.Context(), Last(30))
	require.NoError(t, err)
	require.NotNil(t, report)

	// AC-INSIGHTS-017: Summarizer mock called 0 times by default.
	assert.Equal(t, 0, mock.callCount,
		"Summarizer must not be called when UseLLMSummary is false (default)")
}

// RED #18: TestRenderTable_FiveSections — AC-INSIGHTS-018
func TestRenderTable_FiveSections(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestJSONLFile(t, dir, "success", "2026-04-15.jsonl", 3)

	cfg := InsightsConfig{TelemetryEnabled: true, MinkHome: dir}
	engine := New(cfg, nil)

	report, err := engine.Extract(t.Context(), Last(30))
	require.NoError(t, err)

	rendered := report.RenderTable()

	// AC-INSIGHTS-018: 5 section headers must be present.
	assert.Contains(t, rendered, "Overview", "must have Overview section")
	assert.Contains(t, rendered, "Top 10 Models", "must have Top 10 Models section")
	assert.Contains(t, rendered, "Top 10 Tools", "must have Top 10 Tools section")
	assert.Contains(t, rendered, "Activity Heatmap", "must have Activity Heatmap section")
	assert.Contains(t, rendered, "Top 5 Insights", "must have Top 5 Insights section")

	// ASCII box-drawing characters must be present.
	assert.Contains(t, rendered, "┌", "must use Unicode box-drawing characters")
	assert.Contains(t, rendered, "└", "must use Unicode box-drawing characters")
	assert.Contains(t, rendered, "│", "must use Unicode box-drawing characters")
}

// RED #19: TestJSONExport_TopLevelFields — AC-INSIGHTS-019
func TestJSONExport_TopLevelFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestJSONLFile(t, dir, "success", "2026-04-15.jsonl", 2)

	cfg := InsightsConfig{TelemetryEnabled: true, MinkHome: dir}
	engine := New(cfg, nil)

	report, err := engine.Extract(t.Context(), Last(30))
	require.NoError(t, err)

	data, err := json.Marshal(report)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	// AC-INSIGHTS-019: required top-level fields.
	requiredFields := []string{"period", "overview", "models", "tools", "activity", "insights"}
	for _, field := range requiredFields {
		assert.Contains(t, raw, field, "JSON must contain top-level field %q", field)
	}
}

// --- Helper types ---

// mockSummarizer tracks call count for testing UseLLMSummary=false default.
type mockSummarizer struct {
	callCount int
}

func (m *mockSummarizer) Summarize(_ context.Context, _ []trajectory.TrajectoryEntry, _ int) (string, error) {
	m.callCount++
	return "mock summary", nil
}

// buildToolConversations creates conversation turns representing tool invocations.
func buildToolConversations(toolCounts map[string]int) []trajectory.TrajectoryEntry {
	var entries []trajectory.TrajectoryEntry
	for tool, count := range toolCounts {
		for i := 0; i < count; i++ {
			entries = append(entries, trajectory.TrajectoryEntry{
				From:  trajectory.RoleTool,
				Value: tool,
			})
		}
	}
	return entries
}
