// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeStats overrides the index path and runs `mink memory stats`.
func executeStats(t *testing.T, indexPath string, args ...string) (string, error) {
	t.Helper()
	prev := statsIndexPathOverride
	statsIndexPathOverride = indexPath
	t.Cleanup(func() { statsIndexPathOverride = prev })

	root := &cobra.Command{Use: "mink", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewMemoryCommand())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(append([]string{"memory"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// insertFileAndChunksForStats inserts a file + N chunks into the store.
func insertFileAndChunksForStats(
	t *testing.T,
	store *sqlite.Store,
	sourcePath, collection string,
	chunkCount int,
) {
	t.Helper()
	ctx := context.Background()
	w, err := sqlite.NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	now := time.Now().UTC()
	f := qmd.File{
		Collection:  collection,
		SourcePath:  sourcePath,
		ContentHash: "hash-" + collection,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	fileID, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)

	for i := range chunkCount {
		chunk := qmd.Chunk{
			ID:               qmd.ChunkID(sourcePath, i+1, i+5, "x", qmd.DefaultModelVersion),
			FileID:           fileID,
			Collection:       collection,
			SourcePath:       sourcePath,
			StartLine:        i + 1,
			EndLine:          i + 5,
			Content:          "chunk content",
			EmbeddingPending: true,
			ModelVersion:     qmd.DefaultModelVersion,
			CreatedAt:        now,
		}
		require.NoError(t, w.Insert(ctx, chunk))
	}
}

func TestStats_tableOutputContainsAllCollections(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	insertFileAndChunksForStats(t, store, "/vault/journal/a.md", "journal", 3)
	insertFileAndChunksForStats(t, store, "/vault/custom/b.md", "custom", 5)

	out, err := executeStats(t, dbPath, "stats")
	require.NoError(t, err)

	assert.Contains(t, out, "journal")
	assert.Contains(t, out, "custom")
	assert.Contains(t, out, "TOTAL")
	assert.Contains(t, out, "COLLECTION")
}

func TestStats_jsonOutputSchemaIsValid(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	insertFileAndChunksForStats(t, store, "/vault/journal/a.md", "journal", 2)

	out, err := executeStats(t, dbPath, "stats", "--json")
	require.NoError(t, err)

	var output statsOutput
	require.NoError(t, json.Unmarshal([]byte(out), &output), "stats --json must emit valid JSON")
	assert.NotNil(t, output.PerCollection)
	assert.NotEmpty(t, output.Total.Collection)
}

func TestStats_emptyVaultShowsZeroTotals(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	_ = store.Close()

	out, err := executeStats(t, dbPath, "stats")
	require.NoError(t, err)

	// Should still print header and TOTAL row with 0 values.
	assert.Contains(t, out, "TOTAL")
}
