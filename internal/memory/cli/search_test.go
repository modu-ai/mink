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
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- helper: ingest chunks directly into a test store ----

func ingestForSearch(t *testing.T, store *sqlite.Store, collection, sourcePath, content string) string {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	w, err := sqlite.NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	f := qmd.File{
		Collection:  collection,
		SourcePath:  sourcePath,
		ContentHash: "testhash-" + sourcePath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	fileID, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)

	chunkID := qmd.ChunkID(sourcePath, 1, 10, "content-hash", qmd.DefaultModelVersion)
	chunk := qmd.Chunk{
		ID:               chunkID,
		FileID:           fileID,
		Collection:       collection,
		SourcePath:       sourcePath,
		StartLine:        1,
		EndLine:          10,
		Content:          content,
		EmbeddingPending: false,
		ModelVersion:     qmd.DefaultModelVersion,
		CreatedAt:        now,
	}
	require.NoError(t, w.Insert(ctx, chunk))
	return chunkID
}

// openSearchTestStore opens a fresh SQLite store in a tempdir and registers
// cleanup. Also sets the package-level searchIndexPathOverride for the
// duration of the test.
func openSearchTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "search_test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)

	// Inject the path so runSearch (called via executeMemorySearch) uses it.
	origOverride := searchIndexPathOverride
	searchIndexPathOverride = dbPath
	t.Cleanup(func() {
		_ = store.Close()
		searchIndexPathOverride = origOverride
	})
	return store
}

// skipIfNoFTS5CLI skips the test if chunks_fts is unavailable in this build.
func skipIfNoFTS5CLI(t *testing.T, store *sqlite.Store) {
	t.Helper()
	var n int
	if err := store.DB().QueryRow(
		"SELECT count(*) FROM chunks_fts WHERE chunks_fts MATCH 'a'").Scan(&n); err != nil {
		t.Skipf("FTS5 not available in this SQLite build: %v", err)
	}
}

// executeMemorySearch builds the full "mink memory search …" command tree and
// executes it, returning captured stdout.
func executeMemorySearch(t *testing.T, args ...string) (string, error) {
	t.Helper()

	root := &cobra.Command{Use: "mink", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewMemoryCommand())

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	allArgs := append([]string{"memory", "search"}, args...)
	root.SetArgs(allArgs)

	err := root.Execute()
	return buf.String(), err
}

// ---- tests ----

func TestSearch_jsonOutput(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "custom", "/vault/custom/doc1.md",
		"golang concurrency goroutines channels select")
	ingestForSearch(t, store, "custom", "/vault/custom/doc2.md",
		"python machine learning tensorflow neural network")

	out, err := executeMemorySearch(t, "--json", "golang")
	require.NoError(t, err)

	assert.True(t, json.Valid([]byte(out)), "output must be valid JSON: %s", out)

	var results []searchResultJSON
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &results))

	require.Len(t, results, 1, "only the golang chunk should match")
	assert.Equal(t, "/vault/custom/doc1.md", results[0].SourcePath)
	assert.Greater(t, results[0].Score, 0.0)
	assert.NotEmpty(t, results[0].Snippet)
	assert.NotEmpty(t, results[0].ChunkID)
	assert.Greater(t, results[0].EndLine, 0)
}

func TestSearch_tableOutput(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "journal", "/vault/journal/entry.md",
		"machine learning artificial intelligence deep learning")

	out, err := executeMemorySearch(t, "machine")
	require.NoError(t, err)

	// Table output should contain the source path.
	assert.Contains(t, out, "/vault/journal/entry.md")
}

func TestSearch_noResults(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "custom", "/vault/custom/nomatch.md",
		"hello world this is a test document")

	out, err := executeMemorySearch(t, "zxqvjknonexistent")
	require.NoError(t, err)
	assert.Contains(t, out, "no results",
		"zero-match query must print '(no results)'")
}

func TestSearch_limitFlag(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	for i := range 5 {
		ingestForSearch(t, store,
			"custom",
			filepath.Join("/vault/custom", strings.Repeat("x", i+1)+".md"),
			strings.Repeat("golang concurrency ", 5))
	}

	out, err := executeMemorySearch(t, "--limit", "2", "--json", "golang")
	require.NoError(t, err)

	var results []searchResultJSON
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &results))
	assert.LessOrEqual(t, len(results), 2, "limit=2 must cap results at 2")
}

func TestSearch_modeVsearchReturnsError(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}
	// This test does not need a real store — mode is rejected before SQLite is opened.
	store := openSearchTestStore(t)
	_ = store // path override set; just need the cleanup

	_, err := executeMemorySearch(t, "--mode", "vsearch", "test")
	assert.Error(t, err, "vsearch mode must return an error in M2")
}

func TestSearch_collectionFilter(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "journal", "/vault/journal/coll.md",
		"collection filter journal document")
	ingestForSearch(t, store, "custom", "/vault/custom/coll.md",
		"collection filter custom document")

	out, err := executeMemorySearch(t, "--collection", "journal", "--json", "collection")
	require.NoError(t, err)

	var results []searchResultJSON
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &results))
	for _, r := range results {
		assert.Contains(t, r.SourcePath, "journal",
			"collection filter must restrict to journal; got %s", r.SourcePath)
	}
}

func TestSearch_jsonSchemaFields(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "custom", "/vault/custom/schema.md",
		"schema validation test document content")

	out, err := executeMemorySearch(t, "--json", "schema")
	require.NoError(t, err)

	var results []searchResultJSON
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &results))
	require.NotEmpty(t, results)

	r := results[0]
	assert.NotEmpty(t, r.ChunkID, "chunk_id must be present")
	assert.NotEmpty(t, r.SourcePath, "source_path must be present")
	assert.Greater(t, r.StartLine, 0, "start_line must be > 0")
	assert.Greater(t, r.EndLine, 0, "end_line must be > 0")
	assert.Greater(t, r.Score, 0.0, "score must be > 0")
	assert.NotEmpty(t, r.Snippet, "snippet must be present")
}
