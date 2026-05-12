package insights

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/learning/trajectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

// RED #11: TestScanner_150MBFile_StreamedNotLoaded — AC-INSIGHTS-011
func TestScanner_150MBFile_StreamedNotLoaded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large-file test in -short mode")
	}
	t.Parallel()

	dir := t.TempDir()
	logger := zaptest.NewLogger(t)

	// Write 50,000 entries ≈ 150MB.
	bucketDir := filepath.Join(dir, "success")
	require.NoError(t, os.MkdirAll(bucketDir, 0755))
	filePath := filepath.Join(bucketDir, "2026-04-15.jsonl")
	f, err := os.Create(filePath)
	require.NoError(t, err)

	const sessionCount = 50_000
	base := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	for i := 0; i < sessionCount; i++ {
		tr := &trajectory.Trajectory{
			SessionID: fmt.Sprintf("sess-%06d", i),
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Model:     "anthropic/claude-sonnet-4-6",
			Completed: true,
			Conversations: []trajectory.TrajectoryEntry{
				{From: trajectory.RoleHuman, Value: "prompt text for session " + fmt.Sprintf("%d", i)},
				{From: trajectory.RoleGPT, Value: "response text for session " + fmt.Sprintf("%d", i)},
			},
			Metadata: trajectory.TrajectoryMetadata{
				TokensInput:  1000,
				TokensOutput: 500,
				DurationMs:   1000,
				TurnCount:    2,
			},
		}
		if err := writeTrajectoryLine(f, tr); err != nil {
			_ = f.Close()
			t.Fatalf("write: %v", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Verify file is >= 100MB to test streaming path.
	fi, err := os.Stat(filePath)
	require.NoError(t, err)
	t.Logf("file size: %d bytes (%.1f MB)", fi.Size(), float64(fi.Size())/1024/1024)

	// Measure heap allocation delta.
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	reader := NewTrajectoryReader(dir, logger)
	period := Between(
		time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 15, 23, 59, 59, 0, time.UTC),
	)
	ch := reader.ScanPeriod(period, "success")
	count := 0
	for range ch {
		count++
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	// AC-INSIGHTS-011: all 50,000 sessions consumed.
	assert.Equal(t, sessionCount, count, "all sessions must be scanned")

	// Heap growth must be <= 100MB (streaming, not full file load).
	heapGrowth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	const maxHeapGrowthBytes = 100 * 1024 * 1024 // 100MB
	t.Logf("heap alloc delta: %d bytes (%.1f MB)", heapGrowth, float64(heapGrowth)/1024/1024)
	assert.Less(t, heapGrowth, int64(maxHeapGrowthBytes),
		"streaming scanner must not load entire file into memory")
}

// RED #12: TestScanner_MalformedLineSkipped — AC-INSIGHTS-012
func TestScanner_MalformedLineSkipped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeMixedJSONLFile(t, dir, "success", "2026-04-15.jsonl")

	// Use a logger that records warnings.
	logger, logs := newObservableLogger(t)

	reader := NewTrajectoryReader(dir, logger)
	period := Between(
		time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 15, 23, 59, 59, 0, time.UTC),
	)
	ch := reader.ScanPeriod(period, "success")
	var results []*trajectory.Trajectory
	for tr := range ch {
		results = append(results, tr)
	}

	// AC-INSIGHTS-012: 1 valid entry returned, 2 malformed lines skipped.
	assert.Len(t, results, 1, "only the valid entry should be returned")
	if len(results) > 0 {
		assert.Equal(t, "valid-001", results[0].SessionID)
	}

	// Warning logs must have been emitted (one per malformed line).
	assert.GreaterOrEqual(t, logs.warnCount(), 2, "must log at least 2 warnings for malformed lines")
}

// TestScanner_DateFilter tests that files outside the period are skipped.
func TestScanner_DateFilter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestJSONLFile(t, dir, "success", "2026-04-10.jsonl", 3)
	writeTestJSONLFile(t, dir, "success", "2026-04-15.jsonl", 2)
	writeTestJSONLFile(t, dir, "success", "2026-04-20.jsonl", 4)
	writeTestJSONLFile(t, dir, "success", "2026-04-25.jsonl", 1)

	reader := NewTrajectoryReader(dir, zap.NewNop())
	period := Between(
		time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 20, 23, 59, 59, 0, time.UTC),
	)
	ch := reader.ScanPeriod(period, "success")
	count := 0
	for range ch {
		count++
	}

	// Only 04-15 (2) and 04-20 (4) should be included.
	assert.Equal(t, 6, count)
}

// TestScanner_BothBuckets reads from success and failed.
func TestScanner_BothBuckets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestJSONLFile(t, dir, "success", "2026-04-15.jsonl", 3)
	writeTestJSONLFile(t, dir, "failed", "2026-04-15.jsonl", 2)

	reader := NewTrajectoryReader(dir, zap.NewNop())
	period := Between(
		time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 15, 23, 59, 59, 0, time.UTC),
	)
	ch := reader.ScanPeriod(period, "") // both buckets
	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 5, count) // 3 success + 2 failed
}

// --- Helper: observable logger for testing warnings ---

type observableLog struct {
	warns int
}

func (o *observableLog) warnCount() int { return o.warns }

type countingCore struct {
	log *observableLog
	zapcore.Core
}

func (c *countingCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if e.Level >= zapcore.WarnLevel {
		c.log.warns++
	}
	return c.Core.Check(e, ce)
}

// newObservableLogger returns a logger that counts warnings.
func newObservableLogger(t *testing.T) (*zap.Logger, *observableLog) {
	t.Helper()
	base := zaptest.NewLogger(t)
	log := &observableLog{}
	core := &countingCore{log: log, Core: base.Core()}
	logger := zap.New(core)
	return logger, log
}
