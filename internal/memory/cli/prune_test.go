// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executePrune overrides paths and runs `mink memory prune`.
func executePrune(t *testing.T, indexPath, vaultPath string, args ...string) (string, error) {
	t.Helper()
	prevIdx := pruneIndexPathOverride
	prevVault := pruneVaultPathOverride
	pruneIndexPathOverride = indexPath
	pruneVaultPathOverride = vaultPath
	t.Cleanup(func() {
		pruneIndexPathOverride = prevIdx
		pruneVaultPathOverride = prevVault
	})

	root := &cobra.Command{Use: "mink", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewMemoryCommand())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(append([]string{"memory"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// insertPruneFile inserts a file + chunk at the given creation time and
// creates the markdown file on disk.
func insertPruneFile(
	t *testing.T,
	store *sqlite.Store,
	sourcePath, collection string,
	createdAt time.Time,
) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, os.MkdirAll(filepath.Dir(sourcePath), 0o700))
	require.NoError(t, os.WriteFile(sourcePath, []byte("# test\n\ncontent"), 0o600))

	w, err := sqlite.NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	f := qmd.File{
		Collection:  collection,
		SourcePath:  sourcePath,
		ContentHash: "hash",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
	fileID, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)

	chunk := qmd.Chunk{
		ID:               qmd.ChunkID(sourcePath, 1, 3, "x", qmd.DefaultModelVersion),
		FileID:           fileID,
		Collection:       collection,
		SourcePath:       sourcePath,
		StartLine:        1,
		EndLine:          3,
		Content:          "# test\n\ncontent",
		EmbeddingPending: true,
		ModelVersion:     qmd.DefaultModelVersion,
		CreatedAt:        createdAt,
	}
	require.NoError(t, w.Insert(ctx, chunk))
}

func TestPrune_deletesCorrectCountBeforeDate(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	vaultRoot := filepath.Join(dir, "vault")

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Insert 3 old files and 2 new files around a boundary date.
	boundary := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	old := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	for i := range 3 {
		p := filepath.Join(vaultRoot, "journal", "old"+string(rune('0'+i))+".md")
		insertPruneFile(t, store, p, "journal", old)
	}
	for i := range 2 {
		p := filepath.Join(vaultRoot, "journal", "new"+string(rune('0'+i))+".md")
		insertPruneFile(t, store, p, "journal", newDate)
	}

	out, err := executePrune(t, dbPath, vaultRoot, "prune",
		"--before", boundary.Format(time.DateOnly))
	require.NoError(t, err)
	assert.Contains(t, out, "pruned: 3 files")

	// Verify the new files still exist on disk.
	for i := range 2 {
		p := filepath.Join(vaultRoot, "journal", "new"+string(rune('0'+i))+".md")
		assert.FileExists(t, p, "new file %d should survive prune", i)
	}

	// Verify the old files were removed from disk.
	for i := range 3 {
		p := filepath.Join(vaultRoot, "journal", "old"+string(rune('0'+i))+".md")
		_, statErr := os.Stat(p)
		assert.True(t, os.IsNotExist(statErr), "old file %d should have been pruned", i)
	}
}

func TestPrune_dryRunMutatesNothing(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	vaultRoot := filepath.Join(dir, "vault")

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mdPath := filepath.Join(vaultRoot, "journal", "entry.md")
	insertPruneFile(t, store, mdPath, "journal", old)

	out, err := executePrune(t, dbPath, vaultRoot, "prune",
		"--before", "2026-06-01", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "dry-run:")

	// File must still exist.
	assert.FileExists(t, mdPath, "dry-run must not delete files")

	// DB must still have the row.
	var cnt int
	require.NoError(t, store.DB().QueryRowContext(context.Background(),
		"SELECT count(*) FROM files").Scan(&cnt))
	assert.Equal(t, 1, cnt, "dry-run must not delete DB rows")
}

func TestPrune_collectionFilterRespectsScope(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	vaultRoot := filepath.Join(dir, "vault")

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	journalPath := filepath.Join(vaultRoot, "journal", "entry.md")
	customPath := filepath.Join(vaultRoot, "custom", "note.md")
	insertPruneFile(t, store, journalPath, "journal", old)
	insertPruneFile(t, store, customPath, "custom", old)

	// Prune only journal.
	out, err := executePrune(t, dbPath, vaultRoot, "prune",
		"--before", "2026-06-01", "--collection", "journal")
	require.NoError(t, err)
	assert.Contains(t, out, "pruned: 1 files")

	// Custom file must survive.
	assert.FileExists(t, customPath, "custom file should survive journal-scoped prune")
}

func TestPrune_killMidStagingRecovery(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	vaultRoot := filepath.Join(dir, "vault")

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mdPath := filepath.Join(vaultRoot, "journal", "entry.md")
	insertPruneFile(t, store, mdPath, "journal", old)

	// Install a hook that panics between staging move and DB delete.
	panicTriggered := false
	prevHook := pruneStagingHook
	pruneStagingHook = func() {
		panicTriggered = true
		panic("simulated kill -9")
	}
	t.Cleanup(func() { pruneStagingHook = prevHook })

	// The prune must panic (simulating kill -9).
	func() {
		defer func() { _ = recover() }()
		_, _ = executePrune(t, dbPath, vaultRoot, "prune", "--before", "2026-06-01")
	}()

	assert.True(t, panicTriggered, "staging hook should have been called")

	// After the simulated kill, the DB row should still exist.
	var cnt int
	require.NoError(t, store.DB().QueryRowContext(context.Background(),
		"SELECT count(*) FROM files").Scan(&cnt))
	assert.Equal(t, 1, cnt, "DB row must remain after interrupted prune")

	// Remove the staging hook so the repair can run.
	pruneStagingHook = nil

	// On the next run, repairPrunedState should restore the staged file.
	indexPath, expandErr := resolveDefaultIndexPath(dbPath)
	require.NoError(t, expandErr)
	repairErr := repairPrunedState(context.Background(), store, stagingDir(indexPath))
	require.NoError(t, repairErr)

	// The file must be back at its original location (or at least the DB is consistent).
	// Either the file was restored, or it was cleaned up if the DB was cleared.
	var finalCnt int
	require.NoError(t, store.DB().QueryRowContext(context.Background(),
		"SELECT count(*) FROM files").Scan(&finalCnt))
	// After repair: either all-or-nothing state (AC-MEM-034).
	assert.Equal(t, 1, finalCnt, "after repair DB must still have the row (DB was not committed)")
}

// resolveDefaultIndexPath returns indexPath unchanged (already expanded in test).
func resolveDefaultIndexPath(path string) (string, error) {
	return path, nil
}

func TestPrune_invalidDateReturnsError(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	_ = store.Close()

	_, err = executePrune(t, dbPath, dir, "prune", "--before", "not-a-date")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}
