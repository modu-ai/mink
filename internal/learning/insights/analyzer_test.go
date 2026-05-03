package insights

import (
	"fmt"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/learning/trajectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RED #13: TestAnalyzer_Pattern_Detected — AC-INSIGHTS-013
func TestAnalyzer_Pattern_Detected(t *testing.T) {
	t.Parallel()

	// 4 sessions with the exact same prompt (identical prefix within 50 runes).
	// Using the same prompt text ensures snippetOf returns the same key.
	a := NewAnalyzer()
	var trajectories []*trajectory.Trajectory
	base := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		trajectories = append(trajectories, &trajectory.Trajectory{
			SessionID: fmt.Sprintf("cr-session-%d", i),
			Timestamp: base.Add(time.Duration(i*7) * 24 * time.Hour),
			Model:     "model-a",
			Completed: true,
			Conversations: []trajectory.TrajectoryEntry{
				// Same prompt text across all 4 sessions — same key after snippetOf.
				{From: trajectory.RoleHuman, Value: "오후 2시경에 코드 리뷰 해줘"},
				{From: trajectory.RoleGPT, Value: "OK"},
			},
			Metadata: trajectory.TrajectoryMetadata{TurnCount: 2},
		})
	}

	insights := a.AnalyzePatterns(trajectories)

	// AC-INSIGHTS-013: at least 1 pattern insight, evidence >= 4 sessions, confidence >= 0.7.
	require.NotEmpty(t, insights, "must detect at least one pattern")
	found := false
	for _, ins := range insights {
		if ins.Observations >= 4 {
			found = true
			assert.GreaterOrEqual(t, ins.Confidence, 0.7, "confidence must be >= 0.7 for strong pattern")
			assert.NotEmpty(t, ins.Evidence, "evidence must not be empty")
			break
		}
	}
	assert.True(t, found, "must have an insight with >= 4 observations")
}

// RED #14: TestAnalyzer_Preference_ShorterResponses — AC-INSIGHTS-014
func TestAnalyzer_Preference_ShorterResponses(t *testing.T) {
	t.Parallel()

	// 5 sessions, user requests shorter responses in 4 of them.
	a := NewAnalyzer()
	var trajectories []*trajectory.Trajectory
	base := time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)

	shorterPhrases := []string{
		"더 짧게 답해줘",
		"brief please",
		"keep it concise",
		"short answer only",
	}
	for i, phrase := range shorterPhrases {
		trajectories = append(trajectories, &trajectory.Trajectory{
			SessionID: fmt.Sprintf("pref-session-%d", i),
			Timestamp: base.Add(time.Duration(i) * 24 * time.Hour),
			Model:     "model-a",
			Completed: true,
			Conversations: []trajectory.TrajectoryEntry{
				{From: trajectory.RoleHuman, Value: phrase},
				{From: trajectory.RoleGPT, Value: "OK"},
			},
			Metadata: trajectory.TrajectoryMetadata{TurnCount: 2},
		})
	}
	// Session without the preference.
	trajectories = append(trajectories, &trajectory.Trajectory{
		SessionID: "neutral-session",
		Timestamp: base.Add(5 * 24 * time.Hour),
		Model:     "model-a",
		Completed: true,
		Conversations: []trajectory.TrajectoryEntry{
			{From: trajectory.RoleHuman, Value: "just a normal question"},
			{From: trajectory.RoleGPT, Value: "OK"},
		},
		Metadata: trajectory.TrajectoryMetadata{TurnCount: 2},
	})

	insights := a.AnalyzePreferences(trajectories)

	// AC-INSIGHTS-014: "prefers_shorter_responses" insight with confidence >= 0.7.
	require.NotEmpty(t, insights, "must detect preference insight")
	found := false
	for _, ins := range insights {
		if ins.Title == "prefers_shorter_responses" {
			found = true
			assert.GreaterOrEqual(t, ins.Confidence, 0.7)
			break
		}
	}
	assert.True(t, found, "must have prefers_shorter_responses insight")
}

// RED #15: TestAnalyzer_Error_FailoverReasonFreq — AC-INSIGHTS-015
func TestAnalyzer_Error_FailoverReasonFreq(t *testing.T) {
	t.Parallel()

	// 10 "rate_limit", 3 "context_overflow".
	a := NewAnalyzer()
	var trajectories []*trajectory.Trajectory
	base := time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		trajectories = append(trajectories, &trajectory.Trajectory{
			SessionID: fmt.Sprintf("rl-%d", i),
			Timestamp: base.Add(time.Duration(i) * time.Hour),
			Model:     "model-a",
			Completed: false,
			Metadata:  trajectory.TrajectoryMetadata{FailureReason: "rate_limit"},
		})
	}
	for i := 0; i < 3; i++ {
		trajectories = append(trajectories, &trajectory.Trajectory{
			SessionID: fmt.Sprintf("co-%d", i),
			Timestamp: base.Add(time.Duration(i+10) * time.Hour),
			Model:     "model-a",
			Completed: false,
			Metadata:  trajectory.TrajectoryMetadata{FailureReason: "context_overflow"},
		})
	}

	insights := a.AnalyzeErrors(trajectories)

	// AC-INSIGHTS-015: top 2 are rate_limit and context_overflow.
	require.GreaterOrEqual(t, len(insights), 2, "must have at least 2 error insights")
	assert.Equal(t, "rate_limit", insights[0].Title, "rate_limit must be first (10 occurrences)")
	assert.Equal(t, "context_overflow", insights[1].Title, "context_overflow must be second (3 occurrences)")

	// Both must have observations and confidence.
	assert.Equal(t, 10, insights[0].Observations)
	assert.Equal(t, 3, insights[1].Observations)
	assert.Greater(t, insights[0].Confidence, 0.0)
	assert.Greater(t, insights[1].Confidence, 0.0)
}

// RED #16: TestAnalyzer_Opportunity_UnusedTool — AC-INSIGHTS-016
func TestAnalyzer_Opportunity_UnusedTool(t *testing.T) {
	t.Parallel()

	// Available: memory_recall. Never used in 30-day trajectories.
	a := NewAnalyzer()
	var trajectories []*trajectory.Trajectory
	base := time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		trajectories = append(trajectories, &trajectory.Trajectory{
			SessionID: fmt.Sprintf("s-%d", i),
			Timestamp: base.Add(time.Duration(i) * 24 * time.Hour),
			Model:     "model-a",
			Completed: true,
			Conversations: []trajectory.TrajectoryEntry{
				{From: trajectory.RoleHuman, Value: "do something"},
				{From: trajectory.RoleTool, Value: "Read"}, // different tool used
			},
			Metadata: trajectory.TrajectoryMetadata{TurnCount: 2},
		})
	}

	availableTools := []string{"memory_recall"}
	insights := a.AnalyzeOpportunities(trajectories, availableTools)

	// AC-INSIGHTS-016: "memory_recall" never used → opportunity insight.
	require.NotEmpty(t, insights)
	found := false
	for _, ins := range insights {
		if ins.Title == "memory_recall_never_used" {
			found = true
			assert.Equal(t, 1.0, ins.Confidence, "unused tool has definitive confidence 1.0")
			assert.NotEmpty(t, ins.Evidence, "evidence must not be empty")
			break
		}
	}
	assert.True(t, found, "must detect memory_recall as unused opportunity")
}

// TestAnalyzer_NoEvidenceInsightsSuppressed verifies REQ-INSIGHTS-015.
// Insights with empty Evidence must be suppressed.
func TestAnalyzer_NoEvidenceInsightsSuppressed(t *testing.T) {
	t.Parallel()

	a := NewAnalyzer()
	// Empty trajectory set → no patterns possible.
	insights := a.AnalyzePatterns([]*trajectory.Trajectory{})
	assert.Empty(t, insights, "no evidence → no patterns")

	// Error insights require non-empty FailureReason.
	insights = a.AnalyzeErrors([]*trajectory.Trajectory{
		{
			SessionID: "s1",
			Completed: false,
			Metadata:  trajectory.TrajectoryMetadata{FailureReason: ""}, // empty reason
		},
	})
	assert.Empty(t, insights, "empty FailureReason → no error insight")
}

// TestAnalyzer_PatternMinThreshold checks that < 3 occurrences don't create patterns.
func TestAnalyzer_PatternMinThreshold(t *testing.T) {
	t.Parallel()

	a := NewAnalyzer()
	// Only 2 sessions with the same prefix — below threshold of 3.
	var trajectories []*trajectory.Trajectory
	for i := 0; i < 2; i++ {
		trajectories = append(trajectories, &trajectory.Trajectory{
			SessionID: fmt.Sprintf("s-%d", i),
			Timestamp: time.Now(),
			Conversations: []trajectory.TrajectoryEntry{
				{From: trajectory.RoleHuman, Value: "repeated prompt"},
			},
		})
	}

	insights := a.AnalyzePatterns(trajectories)
	assert.Empty(t, insights, "2 occurrences is below the 3-occurrence threshold")
}
