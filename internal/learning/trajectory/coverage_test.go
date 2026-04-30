// coverage_test.go covers remaining uncovered paths to meet the 85% target.
package trajectory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/modu-ai/goose/internal/learning/trajectory"
	"github.com/modu-ai/goose/internal/learning/trajectory/redact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewChain_UserRulesThenBuiltins verifies NewChain applies user rules before built-ins.
func TestNewChain_UserRulesThenBuiltins(t *testing.T) {
	// User rule replaces "CUSTOM" with "<CUSTOM_REDACTED>".
	userRule := redact.Rule{
		Name:        "custom",
		Pattern:     mustCompile(`CUSTOM_SECRET`),
		Replacement: "<CUSTOM_REDACTED>",
	}
	chain := redact.NewChain([]redact.Rule{userRule}, zap.NewNop())
	entry := &redact.Entry{From: "human", Value: "token=CUSTOM_SECRET and email=user@example.com"}
	chain.Apply(entry)
	assert.Contains(t, entry.Value, "<CUSTOM_REDACTED>")
	assert.Contains(t, entry.Value, "<REDACTED:email>")
	assert.NotContains(t, entry.Value, "CUSTOM_SECRET")
}

// TestWriter_MarshalError — exercise the marshal error path (unreachable in normal use,
// but we cover it by calling WriteTrajectory with a trajectory containing an un-marshalable
// function value injected via helper). Since Go's json.Marshal cannot fail on Trajectory,
// we test the no-op behavior of WriteTrajectory when called on a nil trajectory.
func TestWriter_NilTrajectoryNoPanic(t *testing.T) {
	home := newTestHome(t)
	w := trajectory.NewWriter(home, 0, clockwork.NewFakeClock(), zap.NewNop())
	defer w.Close()

	// WriteTrajectory(nil) must not panic.
	assert.NotPanics(t, func() {
		_ = w.WriteTrajectory(nil)
	})
}

// TestWriter_Close_Idempotent verifies double Close does not panic.
func TestWriter_Close_Idempotent(t *testing.T) {
	home := newTestHome(t)
	w := trajectory.NewWriter(home, 0, clockwork.NewFakeClock(), zap.NewNop())
	require.NoError(t, w.Close())
	require.NoError(t, w.Close()) // second close must be safe
}

// TestCollector_MultipleSessionsFlush verifies multiple sessions are independent.
func TestCollector_MultipleSessionsFlush(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))
	w := newTestWriter(t, home, 0, clk)
	cfg := defaultCfg(home)
	c := newTestCollector(t, cfg, w)

	for i := range 3 {
		sid := fmt.Sprintf("multi-sess-%d", i)
		c.OnTurn(sid, []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "hello"}})
		c.OnTerminal(sid, true, trajectory.TrajectoryMetadata{})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Close(ctx)

	dateStr := clk.Now().UTC().Format("2006-01-02")
	successPath := filepath.Join(home, "trajectories", "success", dateStr+".jsonl")
	waitForFile(t, successPath, time.Second)

	data, err := os.ReadFile(successPath)
	require.NoError(t, err)
	lines := splitLines(data)
	count := 0
	for _, l := range lines {
		if len(l) > 0 {
			var m map[string]any
			require.NoError(t, json.Unmarshal(l, &m))
			count++
		}
	}
	assert.Equal(t, 3, count)
}

// TestWriter_CurrentFilePathForBucket covers the exported helper.
func TestWriter_CurrentFilePathForBucket(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))
	w := trajectory.NewWriter(home, 0, clk, zap.NewNop())
	defer w.Close()

	// Before any write, path is empty.
	assert.Empty(t, w.CurrentFilePathForBucket("success"))

	// After write, path is non-empty.
	traj := &trajectory.Trajectory{
		SessionID: "path-test", Completed: true,
		Conversations: []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "hi"}},
	}
	require.NoError(t, w.WriteTrajectory(traj))
	assert.NotEmpty(t, w.CurrentFilePathForBucket("success"))
	assert.Empty(t, w.CurrentFilePathForBucket("failed")) // not written to failed
}

// TestWriter_LogWarn — exercises the logWarn path via the export helper.
func TestWriter_LogWarn(t *testing.T) {
	home := newTestHome(t)
	w := trajectory.NewWriter(home, 0, clockwork.NewFakeClock(), zap.NewNop())
	defer w.Close()

	// Should not panic.
	assert.NotPanics(t, func() {
		w.LogWarn("test warn", "sess-1", "/some/path", os.ErrPermission)
	})

	// nil logger path — should also not panic.
	w2 := trajectory.NewWriter(home, 0, clockwork.NewFakeClock(), nil)
	defer w2.Close()
	assert.NotPanics(t, func() {
		w2.LogWarn("test warn", "sess-2", "", os.ErrPermission)
	})
}

// TestRetention_NoDir — sweep on non-existent dir is a no-op.
func TestRetention_NoDir(t *testing.T) {
	home := newTestHome(t)
	r := trajectory.NewRetention(home, 30, zap.NewNop())
	assert.NoError(t, r.Sweep())
}

// TestRetention_DefaultDays — zero retentionDays defaults to 90.
func TestRetention_DefaultDays(t *testing.T) {
	home := newTestHome(t)
	r := trajectory.NewRetention(home, 0, zap.NewNop())
	// Just verify it doesn't panic.
	assert.NoError(t, r.Sweep())
}

// TestCollector_OnTurn_ChannelFull — when channel is full, drop gracefully.
func TestCollector_OnTurn_ChannelFull(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	w := newTestWriter(t, home, 0, clk)
	cfg := defaultCfg(home)
	chain := redact.NewBuiltinChain(zap.NewNop())
	c := trajectory.NewCollector(cfg, w, chain, zap.NewNop())

	// Flood the channel — must not block or panic.
	assert.NotPanics(t, func() {
		for range 10000 {
			c.OnTurn("flood", []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "x"}})
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Close(ctx)
}

// TestWriter_RotationMultipleRounds verifies N>1 rotations produce -1, -2, ... suffixes.
func TestWriter_RotationMultipleRounds(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))
	// Very small cap to trigger multiple rotations.
	w := trajectory.NewWriter(home, 100, clk, zap.NewNop())
	defer w.Close()

	for i := range 10 {
		traj := &trajectory.Trajectory{
			SessionID: fmt.Sprintf("rot-%d", i),
			Completed: true,
			Conversations: []trajectory.TrajectoryEntry{
				{From: trajectory.RoleHuman, Value: "padding to exceed 100 bytes per trajectory line"},
			},
		}
		_ = w.WriteTrajectory(traj)
	}

	dir := filepath.Join(home, "trajectories", "success")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), 3, "multiple rotations expected")
}

// TestCollector_SystemRoleNotRedacted — end-to-end: system entries survive redaction.
func TestCollector_SystemRoleNotRedacted(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))
	w := newTestWriter(t, home, 0, clk)
	cfg := defaultCfg(home)
	c := newTestCollector(t, cfg, w)

	sid := "system-role-test"
	c.OnTurn(sid, []trajectory.TrajectoryEntry{
		{From: trajectory.RoleSystem, Value: "You are Goose. Email support@goose.ai for help."},
		{From: trajectory.RoleHuman, Value: "My email is user@example.com"},
	})
	c.OnTerminal(sid, true, trajectory.TrajectoryMetadata{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Close(ctx)

	dateStr := clk.Now().UTC().Format("2006-01-02")
	path := filepath.Join(home, "trajectories", "success", dateStr+".jsonl")
	waitForFile(t, path, time.Second)
	lines := readJSONLLines(t, path)
	require.Len(t, lines, 1)

	convs := lines[0]["conversations"].([]any)
	require.Len(t, convs, 2)

	sys := convs[0].(map[string]any)
	assert.Equal(t, "system", sys["from"])
	assert.Contains(t, sys["value"], "support@goose.ai", "system email must not be redacted")

	human := convs[1].(map[string]any)
	assert.Equal(t, "human", human["from"])
	assert.Contains(t, human["value"], "<REDACTED:email>", "human email must be redacted")
	assert.NotContains(t, human["value"], "user@example.com")
}

func mustCompile(pattern string) *regexp.Regexp {
	r, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	return r
}
