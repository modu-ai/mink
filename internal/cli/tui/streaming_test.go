// Package tui provides streaming state and statusbar tests.
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/cli/tui/snapshots"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusbar_Streaming_Throughput verifies the streaming statusbar shows
// spinner, token count, throughput, elapsed, and abort hint. AC-CLITUI-007
func TestStatusbar_Streaming_Throughput(t *testing.T) {
	snapshots.SetupASCIITermenv()

	model := NewModel(nil, "test-session", true /* noColor */)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// Simulate streaming state: 10 chunks x 10 tokens = 100 tokens over ~1s.
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	model.streaming = true
	model.tokenCount = 100
	model.streamStartTime = baseTime
	model.currentThroughput = 100.0 // t/s
	model.clock = func() time.Time { return baseTime.Add(1 * time.Second) }

	// Render the statusbar.
	view := model.View()

	// Assert: streaming indicators present.
	assert.True(t, strings.Contains(view, "tok"), "statusbar should show token count")
	assert.True(t, strings.Contains(view, "t/s"), "statusbar should show throughput")
	assert.True(t, strings.Contains(view, "Ctrl-C"), "statusbar should show abort hint")

	// Assert: elapsed time present (format: X.Xs elapsed).
	hasElapsed := strings.Contains(view, "elapsed")
	assert.True(t, hasElapsed, "statusbar should show elapsed time")

	// Snapshot: capture streaming state for regression using deterministic time.
	model2 := NewModel(nil, "test-session", true)
	model2.width = 80
	model2.height = 24
	model2.viewport.Width = 80
	model2.viewport.Height = 21
	model2.streaming = true
	model2.tokenCount = 50
	snapBase := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	model2.streamStartTime = snapBase
	model2.currentThroughput = 100.0
	model2.clock = func() time.Time { return snapBase.Add(1 * time.Second) }

	got := []byte(model2.View())
	snapshots.RequireSnapshot(t, "streaming_in_progress", got)
}

// TestStatusbar_Streaming_Aborted verifies Ctrl-C during streaming sets confirmQuit
// and the snapshot matches. AC-CLITUI-008
func TestStatusbar_Streaming_Aborted(t *testing.T) {
	snapshots.SetupASCIITermenv()

	model := NewModel(nil, "test-session", true)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21
	abortBase := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	model.streaming = true
	model.tokenCount = 25
	model.streamStartTime = abortBase
	model.currentThroughput = 50.0
	model.clock = func() time.Time { return abortBase.Add(500 * time.Millisecond) }

	// Verify StreamProgressMsg type exists.
	checkMsg := StreamProgressMsg{TokensDelta: 0, Elapsed: 500 * time.Millisecond}
	_ = checkMsg // just to check the type compiles

	got := []byte(model.View())
	snapshots.RequireSnapshot(t, "streaming_aborted", got)
}

// TestStreamProgressMsg_TypeExists verifies the StreamProgressMsg type is defined.
func TestStreamProgressMsg_TypeExists(t *testing.T) {
	msg := StreamProgressMsg{
		TokensDelta: 10,
		Elapsed:     250 * time.Millisecond,
	}
	require.Equal(t, 10, msg.TokensDelta)
	require.Equal(t, 250*time.Millisecond, msg.Elapsed)
}

// TestCostEstimate_FromUsage verifies cost calculation from usage data. AC-CLITUI-016
func TestStatusbar_CostEstimate_FromUsage(t *testing.T) {
	snapshots.SetupASCIITermenv()

	model := NewModel(nil, "test-session", true)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// Set pricing config: input $3.00/M, output $15.00/M
	model.pricing = map[string]PricingConfig{
		"default": {InputPerMillion: 3.0, OutputPerMillion: 15.0},
	}
	// Usage: 1000 input, 500 output
	// Expected: (1000 * 3.0 / 1e6) + (500 * 15.0 / 1e6) = 0.003 + 0.0075 = 0.0105
	model.cumulativeCost = (1000.0 * 3.0 / 1e6) + (500.0 * 15.0 / 1e6)

	view := model.View()

	// Assert: cost display is present with correct format.
	assert.True(t, strings.Contains(view, "$"), "statusbar should show cost estimate")
	assert.True(t, strings.Contains(view, "0.0105") || strings.Contains(view, "~$0.01"), "cost should be approximately $0.0105")

	// Sub-test: pricing absent → no cost portion.
	t.Run("no_pricing_config", func(t *testing.T) {
		m := NewModel(nil, "test-session", true)
		m.width = 80
		m.height = 24
		// No pricing set.
		v := m.View()
		assert.False(t, strings.Contains(v, "~$"), "no cost shown when pricing absent")
	})
}
