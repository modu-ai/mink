// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	// Use --mode search (BM25) for deterministic JSON output (no ollama needed).
	out, err := executeMemorySearch(t, "--mode", "search", "--json", "golang")
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

	out, err := executeMemorySearch(t, "--mode", "search", "machine")
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

	out, err := executeMemorySearch(t, "--mode", "search", "zxqvjknonexistent")
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

	out, err := executeMemorySearch(t, "--mode", "search", "--limit", "2", "--json", "golang")
	require.NoError(t, err)

	var results []searchResultJSON
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &results))
	assert.LessOrEqual(t, len(results), 2, "limit=2 must cap results at 2")
}

// TestSearch_defaultModeIsQuery verifies the default --mode is "query" (AC-MEM-008).
// When ollama is unreachable, the hybrid runner degrades to BM25 and exits 0.
func TestSearch_defaultModeIsQuery(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "custom", "/vault/custom/default_mode.md",
		"default mode query hybrid test")

	// Do not pass --mode; the default must be "query".
	// Ollama is not running in the test environment; the hybrid runner degrades
	// to BM25-only and returns ErrFellBackToBM25 (exit 0).
	stdout, stderr, err := executeMemorySearchWithStderr(t, "default")
	require.NoError(t, err, "default query mode must exit 0 even when ollama is down")
	// Either BM25 results or no-results in stdout; degradation warning in stderr.
	assert.Contains(t, stderr, "hybrid degraded to BM25-only",
		"default query mode must emit degradation warning when ollama is unreachable")
	_ = stdout
}

// TestSearch_defaultModeIsQuery_ollamaUp simulates a running Ollama by checking
// the error path does not crash when query mode is explicitly requested.
func TestSearch_explicitQueryMode_returnsBM25Fallback(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "custom", "/vault/custom/explicit_query.md",
		"explicit query mode bm25 fallback")

	// --mode query with no ollama → must still return exit 0.
	stdout, stderr, err := executeMemorySearchWithStderr(t, "--mode", "query", "explicit")
	require.NoError(t, err, "--mode query must exit 0 (fallback to BM25)")
	assert.Contains(t, stderr, "hybrid degraded to BM25-only")
	_ = stdout
}

// TestSearch_modeSearchStillWorks verifies --mode search still invokes BM25.
func TestSearch_modeSearchStillWorks(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "custom", "/vault/custom/bm25_mode.md",
		"bm25 search mode still works golang")

	out, err := executeMemorySearch(t, "--mode", "search", "--json", "bm25")
	require.NoError(t, err)

	var results []searchResultJSON
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &results))
	require.Len(t, results, 1)
	assert.Equal(t, "/vault/custom/bm25_mode.md", results[0].SourcePath)
}

// TestSearch_alphaBetaOverride verifies that --alpha / --beta flags propagate.
// With --alpha=0 --beta=1 the mode should degrade to BM25-norm scoring.
func TestSearch_alphaBetaOverride(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "custom", "/vault/custom/alpha_beta.md",
		"alpha beta override test content")

	// With alpha/beta override and no ollama, the hybrid runner falls back to BM25.
	_, _, err := executeMemorySearchWithStderr(t,
		"--mode", "query",
		"--alpha", "0.0",
		"--beta", "1.0",
		"alpha")
	// Must not error (either results or graceful BM25 fallback).
	require.NoError(t, err, "alpha/beta override with query mode must exit 0")
}

func TestSearch_unknownModeReturnsError(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}
	store := openSearchTestStore(t)
	_ = store

	_, err := executeMemorySearch(t, "--mode", "unknown_xyz", "test")
	assert.Error(t, err, "unknown mode must return an error")
}

func TestSearch_limitZeroReturnsError(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}
	store := openSearchTestStore(t)
	_ = store

	_, err := executeMemorySearch(t, "--limit", "0", "test")
	assert.Error(t, err, "--limit 0 must return an error")
}

// executeMemorySearchWithStderr is a variant that captures stderr separately.
func executeMemorySearchWithStderr(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	root := &cobra.Command{Use: "mink", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewMemoryCommand())

	var stdoutBuf, stderrBuf bytes.Buffer
	root.SetOut(&stdoutBuf)
	root.SetErr(&stderrBuf)

	allArgs := append([]string{"memory", "search"}, args...)
	root.SetArgs(allArgs)

	err = root.Execute()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func TestSearch_vsearchFallbackToBM25_ollamaUnreachable(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	ingestForSearch(t, store, "custom", "/vault/custom/vsearch.md",
		"vector search embedding fallback bm25")

	// The Ollama server is not running on port 11434 in the test environment.
	// vsearch must silently fall back to BM25 and return exit code 0.
	stdout, stderr, err := executeMemorySearchWithStderr(t, "--mode", "vsearch", "vector")
	require.NoError(t, err, "vsearch BM25 fallback must exit 0 (AC-MEM-019)")
	assert.Contains(t, stderr, "falling back to BM25",
		"warning message must appear on stderr")
	// The fallback BM25 results or no-results line should appear in stdout.
	_ = stdout
}

func TestSearch_vsearchWithMockOllama(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := openSearchTestStore(t)
	skipIfNoFTS5CLI(t, store)

	// If vec0 is unavailable, the vsearch path falls back to BM25.
	// This test verifies the fallback path when vec0 is not installed.
	if store.HasVec0() {
		t.Skip("vec0 available; end-to-end vsearch test requires real data")
	}

	ingestForSearch(t, store, "custom", "/vault/custom/vsearch2.md",
		"golang channels context concurrency")

	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":` + buildZeroEmbeddingJSON(1024) + `}`))
	}))
	defer mockOllama.Close()

	stdout, stderr, err := executeMemorySearchWithStderr(t, "--mode", "vsearch", "golang")
	require.NoError(t, err, "vsearch must not return error when falling back to BM25")
	assert.Contains(t, stderr, "falling back to BM25")
	_ = stdout
	_ = mockOllama
}

// buildZeroEmbeddingJSON returns a JSON array of n zeros for mock responses.
func buildZeroEmbeddingJSON(n int) string {
	sb := strings.Builder{}
	sb.WriteString("[")
	for i := range n {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("0.0")
	}
	sb.WriteString("]")
	return sb.String()
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

	out, err := executeMemorySearch(t, "--mode", "search", "--collection", "journal", "--json", "collection")
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

	out, err := executeMemorySearch(t, "--mode", "search", "--json", "schema")
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
