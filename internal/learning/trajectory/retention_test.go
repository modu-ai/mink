package trajectory_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/learning/trajectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AC-TRAJECTORY-008 — Retention sweep deletes old files, keeps recent
func TestRetention_SweepOldFiles(t *testing.T) {
	home := newTestHome(t)

	// Create the trajectory directories.
	successDir := filepath.Join(home, "trajectories", "success")
	require.NoError(t, os.MkdirAll(successDir, 0o700))

	now := time.Now().UTC().Truncate(24 * time.Hour)

	// Create test files.
	oldFile := filepath.Join(successDir, now.AddDate(0, 0, -31).Format("2006-01-02")+".jsonl")
	recentFile := filepath.Join(successDir, now.AddDate(0, 0, -29).Format("2006-01-02")+".jsonl")
	todayFile := filepath.Join(successDir, now.Format("2006-01-02")+".jsonl")

	for _, f := range []string{oldFile, recentFile, todayFile} {
		require.NoError(t, os.WriteFile(f, []byte(`{"test":true}`+"\n"), 0o600))
	}

	r := trajectory.NewRetention(home, 30, nil)
	require.NoError(t, r.Sweep())

	// Old file (31 days ago) must be deleted.
	_, err := os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err), "31-day-old file must be deleted")

	// Recent file (29 days ago) must be kept.
	_, err = os.Stat(recentFile)
	assert.NoError(t, err, "29-day-old file must be kept")

	// Today's file must be kept.
	_, err = os.Stat(todayFile)
	assert.NoError(t, err, "today's file must be kept")
}

// TestRetention_SweepKeepsOpenFile — open file must not be deleted
func TestRetention_SweepKeepsOpenFile(t *testing.T) {
	home := newTestHome(t)
	successDir := filepath.Join(home, "trajectories", "success")
	require.NoError(t, os.MkdirAll(successDir, 0o700))

	now := time.Now().UTC().Truncate(24 * time.Hour)

	// An old file that would normally be deleted.
	oldFile := filepath.Join(successDir, now.AddDate(0, 0, -91).Format("2006-01-02")+".jsonl")
	require.NoError(t, os.WriteFile(oldFile, []byte(`{}`+"\n"), 0o600))

	r := trajectory.NewRetention(home, 30, nil)

	// Pass oldFile as "open" — must be skipped.
	require.NoError(t, r.Sweep(oldFile))

	_, err := os.Stat(oldFile)
	assert.NoError(t, err, "open file must not be deleted even if beyond retention")
}
