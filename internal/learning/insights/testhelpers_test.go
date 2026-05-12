package insights

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// writeTestJSONLFile creates a .jsonl file in baseDir/bucket/ with n trajectories.
// When baseDir is an engine's MinkHome, callers must use the MinkHome value;
// the engine internally appends "trajectories/" to MinkHome.
// Scanner tests that call NewTrajectoryReader directly pass baseDir = t.TempDir()
// so files are written to dir/bucket/.
func writeTestJSONLFile(t *testing.T, baseDir, bucket, filename string, n int) {
	t.Helper()
	dir := filepath.Join(baseDir, bucket)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	for i := 0; i < n; i++ {
		ts := parseDate(filename[:10]) // Use the date from filename.
		tr := &trajectory.Trajectory{
			SessionID: fmt.Sprintf("%s-%s-%d", bucket, filename, i),
			Timestamp: ts.Add(time.Duration(i) * time.Hour),
			Model:     "anthropic/claude-sonnet-4-6",
			Completed: bucket == "success",
			Conversations: []trajectory.TrajectoryEntry{
				{From: trajectory.RoleHuman, Value: "test prompt"},
				{From: trajectory.RoleGPT, Value: "test response"},
			},
			Metadata: trajectory.TrajectoryMetadata{
				TokensInput:  1000,
				TokensOutput: 500,
				DurationMs:   30000,
				TurnCount:    2,
			},
		}
		if err := writeTrajectoryLine(f, tr); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

// writeTestJSONLFileWithTrajectories writes custom trajectories to baseDir/bucket/filename.
func writeTestJSONLFileWithTrajectories(t *testing.T, baseDir, bucket, filename string, trajectories []*trajectory.Trajectory) {
	t.Helper()
	dir := filepath.Join(baseDir, bucket)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	for _, tr := range trajectories {
		if err := writeTrajectoryLine(f, tr); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

// writeMixedJSONLFile writes a .jsonl file with some valid and some invalid lines.
func writeMixedJSONLFile(t *testing.T, baseDir, bucket, filename string) {
	t.Helper()
	dir := filepath.Join(baseDir, bucket)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	valid := &trajectory.Trajectory{
		SessionID: "valid-001",
		Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		Model:     "anthropic/claude-sonnet-4-6",
		Completed: true,
		Conversations: []trajectory.TrajectoryEntry{
			{From: trajectory.RoleHuman, Value: "hello"},
		},
		Metadata: trajectory.TrajectoryMetadata{TurnCount: 1, DurationMs: 1000},
	}

	data, _ := json.Marshal(valid)
	_, _ = fmt.Fprintf(f, "{\"conversations\":[broken\n") // invalid line 1
	_, _ = fmt.Fprintf(f, "%s\n", data)                   // valid line 2
	_, _ = fmt.Fprintf(f, "{another_invalid}\n")          // invalid line 3
}

// parseDate parses "YYYY-MM-DD" to time.Time (UTC midnight).
func parseDate(s string) time.Time {
	if len(s) != 10 {
		return time.Now().UTC()
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Now().UTC()
	}
	return t.UTC()
}

// makeTrajectory creates a trajectory with given fields.
func makeTrajectory(sessionID, model string, ts time.Time, completed bool, inputTokens, outputTokens int, durationMs int64) *trajectory.Trajectory {
	return &trajectory.Trajectory{
		SessionID: sessionID,
		Timestamp: ts,
		Model:     model,
		Completed: completed,
		Conversations: []trajectory.TrajectoryEntry{
			{From: trajectory.RoleHuman, Value: "test prompt"},
			{From: trajectory.RoleGPT, Value: "test response"},
		},
		Metadata: trajectory.TrajectoryMetadata{
			TokensInput:  inputTokens,
			TokensOutput: outputTokens,
			DurationMs:   durationMs,
			TurnCount:    2,
		},
	}
}
