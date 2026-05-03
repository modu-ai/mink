package insights

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderTable_FiveSections is also covered in engine_test.go RED #18.
// This test provides additional render-specific assertions.
func TestRenderTable_OverviewValues(t *testing.T) {
	t.Parallel()

	report := &Report{
		Period: Between(
			time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		),
		Overview: Overview{
			TotalSessions:      128,
			TotalSuccessful:    124,
			TotalFailed:        4,
			TotalTokens:        2_341_589,
			EstimatedCost:      12.47,
			HasFullPricing:     false,
			TotalHours:         18.3,
			AvgSessionDuration: 514, // ~8m34s
		},
		Models:   []ModelStat{},
		Tools:    []ToolStat{},
		Activity: newEmptyActivity(),
		Insights: map[InsightCategory][]Insight{},
	}

	rendered := report.RenderTable()

	assert.Contains(t, rendered, "128")
	assert.Contains(t, rendered, "124")
	assert.Contains(t, rendered, "$12.47")
}

// TestRenderTable_PricingNA verifies "N/A" for models without pricing.
func TestRenderTable_PricingNA(t *testing.T) {
	t.Parallel()

	report := &Report{
		Period: Last(7),
		Overview: Overview{
			TotalSessions: 1,
		},
		Models: []ModelStat{
			{Model: "unknown-model", Sessions: 1, TotalTokens: 1000, HasPricing: false, Cost: 0},
		},
		Tools:    []ToolStat{},
		Activity: newEmptyActivity(),
		Insights: map[InsightCategory][]Insight{},
	}

	rendered := report.RenderTable()
	assert.Contains(t, rendered, "N/A", "unknown-model cost must render as N/A")
}

// TestJSONExport_Schema verifies all required JSON fields are present.
// This is the primary test for AC-INSIGHTS-019.
func TestJSONExport_Schema(t *testing.T) {
	t.Parallel()

	report := &Report{
		Period: Between(
			time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		),
		Overview: Overview{TotalSessions: 5},
		Models: []ModelStat{
			{Model: "model-a", Sessions: 5, TotalTokens: 5000, HasPricing: true, Cost: 0.01},
		},
		Tools: []ToolStat{
			{Tool: "Read", Count: 10, Percentage: 100.0},
		},
		Activity: newEmptyActivity(),
		Insights: map[InsightCategory][]Insight{
			CategoryPattern: {
				{
					Category:     CategoryPattern,
					Title:        "test_pattern",
					Observations: 3,
					Confidence:   0.8,
					Evidence: []EvidenceRef{
						{SessionID: "s1", Snippet: "test"},
					},
				},
			},
		},
		GeneratedAt: time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(report)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	// AC-INSIGHTS-019: top-level fields.
	requiredFields := []string{"period", "overview", "models", "tools", "activity", "insights"}
	for _, field := range requiredFields {
		assert.Contains(t, raw, field, "must have field: %s", field)
	}

	// Verify period has from/to.
	var period map[string]interface{}
	require.NoError(t, json.Unmarshal(raw["period"], &period))
	assert.Contains(t, period, "from")
	assert.Contains(t, period, "to")
}

// TestJSONExport_InsightsStructure verifies the insights field contains 4 categories.
func TestJSONExport_InsightsStructure(t *testing.T) {
	t.Parallel()

	report := &Report{
		Period:   Last(7),
		Overview: Overview{},
		Models:   []ModelStat{},
		Tools:    []ToolStat{},
		Activity: newEmptyActivity(),
		Insights: map[InsightCategory][]Insight{
			CategoryPattern:     {},
			CategoryPreference:  {},
			CategoryError:       {},
			CategoryOpportunity: {},
		},
		GeneratedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(report)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	// Insights should be a JSON array with 4 entries.
	var insightsArr []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["insights"], &insightsArr))
	assert.Len(t, insightsArr, 4, "should have 4 category entries in insights array")

	// Each entry has a "category" field.
	for _, entry := range insightsArr {
		assert.Contains(t, entry, "category")
		assert.Contains(t, entry, "insights")
	}
}

// TestFormatInt tests comma formatting for large numbers.
func TestFormatInt(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1,000", formatInt(1000))
	assert.Equal(t, "1,234,567", formatInt(1234567))
	assert.Equal(t, "999", formatInt(999))
}

// TestFormatTokensK tests token count abbreviation.
func TestFormatTokensK(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1.0K", formatTokensK(1000))
	assert.Equal(t, "1.2M", formatTokensK(1234567))
	assert.Equal(t, "500", formatTokensK(500))
}
