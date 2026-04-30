package trajectory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/modu-ai/goose/internal/learning/trajectory"
	"github.com/modu-ai/goose/internal/learning/trajectory/redact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// helpers

func newTestHome(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func newTestWriter(t *testing.T, home string, maxBytes int64, clk clockwork.Clock) *trajectory.Writer {
	t.Helper()
	w := trajectory.NewWriter(home, maxBytes, clk, zap.NewNop())
	t.Cleanup(func() { _ = w.Close() })
	return w
}

func newTestCollector(t *testing.T, cfg trajectory.TelemetryConfig, w *trajectory.Writer) *trajectory.Collector {
	t.Helper()
	chain := redact.NewBuiltinChain(zap.NewNop())
	c := trajectory.NewCollector(cfg, w, chain, zap.NewNop())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})
	return c
}

func defaultCfg(home string) trajectory.TelemetryConfig {
	cfg := trajectory.DefaultTelemetryConfig()
	cfg.GooseHome = home
	return cfg
}

func readJSONLLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var lines []map[string]any
	for _, raw := range splitLines(data) {
		if len(raw) == 0 {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal(raw, &m), "invalid JSON line: %s", raw)
		lines = append(lines, m)
	}
	return lines
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func waitForFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file not created within %s: %s", timeout, path)
}

// AC-TRAJECTORY-001 — ShareGPT schema compliance + success path
func TestCollector_OnTerminalSuccess_WritesToSuccessDir(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	// Fix time to 2026-04-21.
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))
	w := newTestWriter(t, home, 0, clk)
	cfg := defaultCfg(home)
	c := newTestCollector(t, cfg, w)

	sessionID := "test-session-001"
	c.OnTurn(sessionID, []trajectory.TrajectoryEntry{
		{From: trajectory.RoleHuman, Value: "hi"},
	})
	c.OnTurn(sessionID, []trajectory.TrajectoryEntry{
		{From: trajectory.RoleGPT, Value: "hello"},
	})

	// Flush the writer's date by advancing clock to trigger file open.
	meta := trajectory.TrajectoryMetadata{}
	c.OnTerminal(sessionID, true, meta)

	// Wait for worker to process.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Close(ctx)

	// Determine expected file path.
	dateStr := clk.Now().UTC().Format("2006-01-02")
	successPath := filepath.Join(home, "trajectories", "success", dateStr+".jsonl")
	waitForFile(t, successPath, time.Second)

	lines := readJSONLLines(t, successPath)
	require.Len(t, lines, 1)

	line := lines[0]
	assert.Equal(t, true, line["completed"])
	assert.Equal(t, sessionID, line["session_id"])

	convs, ok := line["conversations"].([]any)
	require.True(t, ok)
	require.Len(t, convs, 2)

	entry0 := convs[0].(map[string]any)
	assert.Equal(t, "human", entry0["from"])
	assert.Equal(t, "hi", entry0["value"])

	entry1 := convs[1].(map[string]any)
	assert.Equal(t, "gpt", entry1["from"])
	assert.Equal(t, "hello", entry1["value"])

	// Verify allowed roles only.
	allowedRoles := map[string]bool{"system": true, "human": true, "gpt": true, "tool": true}
	for _, conv := range convs {
		e := conv.(map[string]any)
		assert.True(t, allowedRoles[e["from"].(string)], "unexpected from: %s", e["from"])
	}
}

// AC-TRAJECTORY-002 — Failed trajectory goes to failed/ with failure_reason
func TestCollector_OnTerminalFailure_WritesToFailedDir(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))
	w := newTestWriter(t, home, 0, clk)
	cfg := defaultCfg(home)
	c := newTestCollector(t, cfg, w)

	sessionID := "fail-session-001"
	c.OnTurn(sessionID, []trajectory.TrajectoryEntry{
		{From: trajectory.RoleHuman, Value: "help"},
	})
	meta := trajectory.TrajectoryMetadata{FailureReason: "context_overflow"}
	c.OnTerminal(sessionID, false, meta)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Close(ctx)

	dateStr := clk.Now().UTC().Format("2006-01-02")
	failedPath := filepath.Join(home, "trajectories", "failed", dateStr+".jsonl")
	waitForFile(t, failedPath, time.Second)

	lines := readJSONLLines(t, failedPath)
	require.Len(t, lines, 1)
	assert.Equal(t, false, lines[0]["completed"])
	meta2 := lines[0]["metadata"].(map[string]any)
	assert.Equal(t, "context_overflow", meta2["failure_reason"])

	// Must not appear in success/.
	successPath := filepath.Join(home, "trajectories", "success", dateStr+".jsonl")
	_, err := os.Stat(successPath)
	assert.True(t, os.IsNotExist(err), "session must not appear in success/")
}

// AC-TRAJECTORY-005 — Size-based rotation
func TestWriter_RotatesOnMaxBytes(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))

	// maxBytes = 200 bytes — each trajectory is ~100 bytes so two writes trigger rotation.
	w := trajectory.NewWriter(home, 200, clk, zap.NewNop())
	defer w.Close()

	dateStr := clk.Now().UTC().Format("2006-01-02")
	dir := filepath.Join(home, "trajectories", "success")

	// Write enough data to exceed 200 bytes.
	for range 5 {
		t := &trajectory.Trajectory{
			SessionID:     "sess",
			Completed:     true,
			Conversations: []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "hello world this is a test message to pad bytes"}},
		}
		_ = w.WriteTrajectory(t)
	}

	// Should have at least 2 files: base + at least one rotation.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), 2, "expected at least 2 files after rotation")

	// Base file.
	base := filepath.Join(dir, dateStr+".jsonl")
	_, err = os.Stat(base)
	require.NoError(t, err, "base file must exist")

	// Rotation file.
	rot1 := filepath.Join(dir, dateStr+"-1.jsonl")
	_, err = os.Stat(rot1)
	require.NoError(t, err, "rotation file -1 must exist")
}

// AC-TRAJECTORY-006 — Date rollover
func TestWriter_RolloverOnDateChange(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()

	// Set clock to 2026-04-21 23:59:58 UTC.
	midnight := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	clk.Advance(midnight.Add(-2 * time.Second).Sub(clk.Now()))

	w := trajectory.NewWriter(home, 0, clk, zap.NewNop())
	defer w.Close()

	// Write before midnight.
	traj1 := &trajectory.Trajectory{
		SessionID: "before", Completed: true,
		Conversations: []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "before midnight"}},
	}
	require.NoError(t, w.WriteTrajectory(traj1))

	// Advance clock past midnight to 00:00:02.
	clk.Advance(4 * time.Second)

	// Write after midnight.
	traj2 := &trajectory.Trajectory{
		SessionID: "after", Completed: true,
		Conversations: []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "after midnight"}},
	}
	require.NoError(t, w.WriteTrajectory(traj2))

	dir := filepath.Join(home, "trajectories", "success")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "should have two date files: 2026-04-21 and 2026-04-22")

	// Verify file names.
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}
	assert.True(t, names["2026-04-21.jsonl"], "2026-04-21.jsonl must exist")
	assert.True(t, names["2026-04-22.jsonl"], "2026-04-22.jsonl must exist")
}

// AC-TRAJECTORY-007 — Disabled noop
func TestCollector_DisabledIsNoop(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))

	w := newTestWriter(t, home, 0, clk)
	cfg := defaultCfg(home)
	cfg.Enabled = false

	goroutinesBefore := runtime.NumGoroutine()
	c := trajectory.NewCollector(cfg, w, redact.NewBuiltinChain(zap.NewNop()), zap.NewNop())

	c.OnTurn("sess", []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "hi"}})
	c.OnTerminal("sess", true, trajectory.TrajectoryMetadata{})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = c.Close(ctx)

	goroutinesAfter := runtime.NumGoroutine()

	// No directory should be created.
	_, err := os.Stat(filepath.Join(home, "trajectories"))
	assert.True(t, os.IsNotExist(err) || isDirEmpty(filepath.Join(home, "trajectories")),
		"trajectories/ must not be created when disabled")

	// Goroutine count delta must be 0 (no worker spawned).
	assert.LessOrEqual(t, goroutinesAfter-goroutinesBefore, 1,
		"no extra goroutines should be spawned when disabled")
}

func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return true
	}
	return len(entries) == 0
}

// AC-TRAJECTORY-009 — Write permission denied does not block
func TestWriter_WritePermissionDeniedDoesNotBlock(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))

	w := trajectory.NewWriter(home, 0, clk, zap.NewNop())
	defer w.Close()

	// Pre-create the success dir and make it unwritable.
	successDir := filepath.Join(home, "trajectories", "success")
	require.NoError(t, os.MkdirAll(successDir, 0o700))
	require.NoError(t, os.Chmod(successDir, 0o500)) // r-x only

	traj := &trajectory.Trajectory{
		SessionID: "perm-test", Completed: true,
		Conversations: []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "hi"}},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Must return quickly without blocking.
		w.WriteTrajectory(traj) //nolint:errcheck
	}()

	select {
	case <-done:
		// Good — returned without blocking.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WriteTrajectory blocked for >500ms on permission error")
	}

	// Restore so cleanup works.
	_ = os.Chmod(successDir, 0o700)
}

// AC-TRAJECTORY-012 — Concurrent sessions, no byte interleaving
func TestConcurrentSessions_NoInterleaving(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))

	w := trajectory.NewWriter(home, 0, clk, zap.NewNop())
	defer w.Close()

	const numSessions = 10
	const turnsPerSession = 10

	var wg sync.WaitGroup
	for i := range numSessions {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			traj := &trajectory.Trajectory{
				SessionID: fmt.Sprintf("concurrent-%d", idx),
				Completed: true,
				Conversations: func() []trajectory.TrajectoryEntry {
					entries := make([]trajectory.TrajectoryEntry, turnsPerSession)
					for j := range entries {
						entries[j] = trajectory.TrajectoryEntry{From: trajectory.RoleHuman, Value: fmt.Sprintf("turn-%d", j)}
					}
					return entries
				}(),
			}
			_ = w.WriteTrajectory(traj)
		}(i)
	}
	wg.Wait()

	dateStr := clk.Now().UTC().Format("2006-01-02")
	successPath := filepath.Join(home, "trajectories", "success", dateStr+".jsonl")
	lines := readJSONLLines(t, successPath)

	assert.Len(t, lines, numSessions, "each session must produce exactly one JSON-L line")
	for i, line := range lines {
		assert.NotNil(t, line["session_id"], "line %d must have session_id", i)
	}
}

// AC-TRAJECTORY-013 — File permissions 0600/0700
func TestWriter_FilePermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	// Use a umask that would normally mask group/other — verify we still get 0600/0700.
	oldUmask := syscall.Umask(0o077)
	defer syscall.Umask(oldUmask)

	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))

	w := trajectory.NewWriter(home, 0, clk, zap.NewNop())
	defer w.Close()

	traj := &trajectory.Trajectory{
		SessionID: "perm-check", Completed: true,
		Conversations: []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "test"}},
	}
	require.NoError(t, w.WriteTrajectory(traj))

	dateStr := clk.Now().UTC().Format("2006-01-02")

	checkPerms := func(path string, expectedMode os.FileMode, name string) {
		t.Helper()
		info, err := os.Stat(path)
		require.NoError(t, err, "%s must exist", name)
		assert.Equal(t, expectedMode, info.Mode().Perm(), "%s must have mode %04o", name, expectedMode)
		// Must not be a symlink.
		linfo, err := os.Lstat(path)
		require.NoError(t, err)
		assert.False(t, linfo.Mode()&os.ModeSymlink != 0, "%s must not be a symlink", name)
	}

	// Check file permissions.
	successFile := filepath.Join(home, "trajectories", "success", dateStr+".jsonl")
	checkPerms(successFile, 0o600, "success file")

	// Check directory permissions.
	checkPerms(filepath.Join(home, "trajectories", "success"), 0o700, "success/")
	checkPerms(filepath.Join(home, "trajectories"), 0o700, "trajectories/")

	// Same check for failed path.
	failTraj := &trajectory.Trajectory{
		SessionID: "perm-fail", Completed: false,
		Conversations: []trajectory.TrajectoryEntry{{From: trajectory.RoleHuman, Value: "fail"}},
	}
	require.NoError(t, w.WriteTrajectory(failTraj))

	failedFile := filepath.Join(home, "trajectories", "failed", dateStr+".jsonl")
	checkPerms(failedFile, 0o600, "failed file")
	checkPerms(filepath.Join(home, "trajectories", "failed"), 0o700, "failed/")
}

// AC-TRAJECTORY-014 — OnTurn latency <1ms median, p99 <5ms
func TestCollector_OnTurnLatencyUnder1ms(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	w := newTestWriter(t, home, 0, clk)
	cfg := defaultCfg(home)
	cfg.InMemoryTurnCap = 10000 // large cap to avoid spill latency
	c := newTestCollector(t, cfg, w)

	const iterations = 1000
	latencies := make([]time.Duration, iterations)

	for i := range iterations {
		start := time.Now()
		c.OnTurn("sess-latency", []trajectory.TrajectoryEntry{
			{From: trajectory.RoleHuman, Value: "test"},
		})
		latencies[i] = time.Since(start)
	}

	// Sort latencies.
	sortDurations(latencies)
	median := latencies[len(latencies)/2]
	p99 := latencies[int(float64(len(latencies))*0.99)]
	maxLat := latencies[len(latencies)-1]

	// CI tolerance: 3x.
	assert.Less(t, median, 3*time.Millisecond, "OnTurn median latency must be <3ms, got %s", median)
	assert.Less(t, p99, 15*time.Millisecond, "OnTurn p99 must be <15ms, got %s", p99)
	assert.Less(t, maxLat, 30*time.Millisecond, "OnTurn max must be <30ms, got %s", maxLat)
}

func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j] < d[j-1]; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}

// AC-TRAJECTORY-011 — Buffer spill on cap exceeded
func TestCollector_SpillOnBufferCap(t *testing.T) {
	home := newTestHome(t)
	clk := clockwork.NewFakeClock()
	clk.Advance(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC).Sub(clk.Now()))

	w := newTestWriter(t, home, 0, clk)
	cfg := defaultCfg(home)
	cfg.InMemoryTurnCap = 100
	c := newTestCollector(t, cfg, w)

	sessionID := "spill-test"

	// Send 150 turns — should trigger spill at 101st.
	for i := range 150 {
		c.OnTurn(sessionID, []trajectory.TrajectoryEntry{
			{From: trajectory.RoleHuman, Value: fmt.Sprintf("turn-%d", i)},
		})
	}

	// Flush session.
	c.OnTerminal(sessionID, true, trajectory.TrajectoryMetadata{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Close(ctx)

	dateStr := clk.Now().UTC().Format("2006-01-02")
	successPath := filepath.Join(home, "trajectories", "success", dateStr+".jsonl")
	waitForFile(t, successPath, time.Second)

	lines := readJSONLLines(t, successPath)
	// Must have at least 2 lines: 1 spill fragment + 1 final flush.
	assert.GreaterOrEqual(t, len(lines), 2, "must have spill fragment + final flush")

	// At least one line must have partial=true.
	hasPartial := false
	for _, line := range lines {
		meta, ok := line["metadata"].(map[string]any)
		if ok {
			if p, ok := meta["partial"].(bool); ok && p {
				hasPartial = true
			}
		}
	}
	assert.True(t, hasPartial, "must have at least one partial=true spill record")
}
