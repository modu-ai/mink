// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
)

// executeReindex builds a cobra tree, overrides the index path, and runs
// `mink memory reindex` with the given args.
func executeReindex(t *testing.T, indexPath string, args ...string) (string, error) {
	t.Helper()
	prev := reindexIndexPathOverride
	reindexIndexPathOverride = indexPath
	t.Cleanup(func() { reindexIndexPathOverride = prev })

	root := &cobra.Command{Use: "mink", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewMemoryCommand())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(append([]string{"memory"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// insertFileAndChunk is a test helper that inserts a file + chunk with the given
// model_version into the store.
func insertFileAndChunk(
	t *testing.T,
	store *sqlite.Store,
	sourcePath, collection, modelVersion string,
) (int64, string) {
	t.Helper()
	ctx := context.Background()
	w, err := sqlite.NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	now := time.Now().UTC()
	f := qmd.File{
		Collection:  collection,
		SourcePath:  sourcePath,
		ContentHash: "testhash",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	fileID, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)

	chunkID := fmt.Sprintf("chunk-%d", fileID)
	chunk := qmd.Chunk{
		ID:               chunkID,
		FileID:           fileID,
		Collection:       collection,
		SourcePath:       sourcePath,
		StartLine:        1,
		EndLine:          5,
		Content:          "# Hello\n\nsome content",
		EmbeddingPending: true,
		ModelVersion:     modelVersion,
		CreatedAt:        now,
	}
	require.NoError(t, w.Insert(ctx, chunk))
	return fileID, chunkID
}

// modelVersionForChunk returns the model_version for any chunk belonging to fileID.
func modelVersionForChunk(t *testing.T, store *sqlite.Store, fileID int64) string {
	t.Helper()
	var v string
	err := store.DB().QueryRowContext(context.Background(),
		"SELECT model_version FROM chunks WHERE file_id = ? LIMIT 1", fileID).Scan(&v)
	require.NoError(t, err)
	return v
}

func TestReindex_staleChunkIsReplacedWithFreshVersion(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Create a real markdown file in the temp dir.
	mdPath := filepath.Join(dir, "note.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("# Title\n\nContent paragraph."), 0o600))

	// Insert a chunk with a stale model_version.
	fileID, _ := insertFileAndChunk(t, store, mdPath, "custom", "stale-v0")
	assert.Equal(t, "stale-v0", modelVersionForChunk(t, store, fileID))

	// Run reindex targeting the stale version.
	out, err := executeReindex(t, dbPath, "reindex")
	require.NoError(t, err)
	assert.Contains(t, out, "reindexed:")

	// After reindex the model_version must match DefaultModelVersion.
	assert.Equal(t, qmd.DefaultModelVersion, modelVersionForChunk(t, store, fileID))
}

func TestReindex_orphanFileLogsWarning(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Insert a chunk whose source file does NOT exist on disk.
	missingPath := filepath.Join(dir, "missing.md")
	insertFileAndChunk(t, store, missingPath, "custom", "stale-v0")

	// Reindex should exit 0 even when the file is missing (orphan count > 0).
	out, err := executeReindex(t, dbPath, "reindex")
	require.NoError(t, err, "orphan file should not cause exit != 0")
	assert.Contains(t, out, "orphan:")
}

func TestReindex_collectionFilter(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Insert one stale chunk in "journal" and one in "custom".
	mdJournal := filepath.Join(dir, "journal.md")
	mdCustom := filepath.Join(dir, "custom.md")
	require.NoError(t, os.WriteFile(mdJournal, []byte("# J\n\nJ content"), 0o600))
	require.NoError(t, os.WriteFile(mdCustom, []byte("# C\n\nC content"), 0o600))

	journalID, _ := insertFileAndChunk(t, store, mdJournal, "journal", "stale-v0")
	customID, _ := insertFileAndChunk(t, store, mdCustom, "custom", "stale-v0")

	// Reindex only the "journal" collection.
	_, err = executeReindex(t, dbPath, "reindex", "--collection", "journal")
	require.NoError(t, err)

	// Only the journal chunk should have an updated model_version.
	assert.Equal(t, qmd.DefaultModelVersion, modelVersionForChunk(t, store, journalID))
	assert.Equal(t, "stale-v0", modelVersionForChunk(t, store, customID))
}

func TestReindex_concurrentReadDoesNotBlock(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	mdPath := filepath.Join(dir, "note.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("# Concurrent\n\ndata"), 0o600))
	insertFileAndChunk(t, store, mdPath, "custom", "stale-v0")

	// Concurrently run BM25 searches while reindex proceeds.
	var wg sync.WaitGroup
	errs := make(chan error, 5)

	for range 5 {
		wg.Go(func() {
			r := sqlite.NewReader(store)
			_, searchErr := r.SearchBM25(context.Background(), "concurrent", "", 5)
			if searchErr != nil && searchErr != sqlite.ErrFTS5Unavailable {
				errs <- searchErr
			}
		})
	}

	// Run reindex.
	_, reindexErr := executeReindex(t, dbPath, "reindex")

	wg.Wait()
	close(errs)

	require.NoError(t, reindexErr)
	for e := range errs {
		t.Errorf("concurrent search error: %v", e)
	}
}
