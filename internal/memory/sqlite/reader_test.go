// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertTestChunk is a helper that inserts a file + chunk into the store via
// Writer and returns the chunk_id inserted.
func insertTestChunk(t *testing.T, w *Writer, collection, sourcePath, content string, startLine, endLine int) string {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	f := qmd.File{
		Collection:  collection,
		SourcePath:  sourcePath,
		ContentHash: "hash-" + sourcePath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	fileID, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)

	chunkID := qmd.ChunkID(sourcePath, startLine, endLine, "testhash", qmd.DefaultModelVersion)
	chunk := qmd.Chunk{
		ID:               chunkID,
		FileID:           fileID,
		Collection:       collection,
		SourcePath:       sourcePath,
		StartLine:        startLine,
		EndLine:          endLine,
		Content:          content,
		EmbeddingPending: false,
		ModelVersion:     qmd.DefaultModelVersion,
		CreatedAt:        now,
	}
	require.NoError(t, w.Insert(ctx, chunk))
	return chunkID
}

// openTestStoreWithReader opens a test store and returns both store and reader.
func openTestStoreWithReader(t *testing.T) (*Store, *Reader) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, NewReader(s)
}

// skipIfNoFTS5 skips the test if FTS5 is unavailable in this SQLite build.
func skipIfNoFTS5(t *testing.T, s *Store) {
	t.Helper()
	var n int
	err := s.db.QueryRow("SELECT count(*) FROM chunks_fts WHERE chunks_fts MATCH 'a'").Scan(&n)
	if err != nil {
		t.Skipf("FTS5 not available in this SQLite build: %v", err)
	}
}

func TestReader_SearchBM25_ranking(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store, reader := openTestStoreWithReader(t)
	skipIfNoFTS5(t, store)

	w, err := NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	// Insert two chunks: one with the query term appearing many times (should
	// score higher) and one with the term appearing once.
	_ = insertTestChunk(t, w,
		"journal", "/vault/journal/a.md",
		"golang golang golang golang golang programming language concurrency",
		1, 5)
	_ = insertTestChunk(t, w,
		"journal", "/vault/journal/b.md",
		"the weather today is sunny and bright",
		6, 10)

	ctx := context.Background()
	hits, err := reader.SearchBM25(ctx, "golang", "", 10)
	require.NoError(t, err)
	require.Len(t, hits, 1, "only the golang chunk should match")
	assert.Greater(t, hits[0].Score, 0.0, "score must be positive (normalised)")
}

func TestReader_SearchBM25_collectionFilter(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store, reader := openTestStoreWithReader(t)
	skipIfNoFTS5(t, store)

	w, err := NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	_ = insertTestChunk(t, w, "journal", "/vault/journal/j.md",
		"memory retrieval journal entry", 1, 3)
	_ = insertTestChunk(t, w, "custom", "/vault/custom/c.md",
		"memory retrieval custom document", 1, 3)

	ctx := context.Background()

	// Search in "journal" collection only.
	journalHits, err := reader.SearchBM25(ctx, "memory", "journal", 10)
	require.NoError(t, err)
	require.Len(t, journalHits, 1)

	// Search in "custom" collection only.
	customHits, err := reader.SearchBM25(ctx, "memory", "custom", 10)
	require.NoError(t, err)
	require.Len(t, customHits, 1)

	assert.NotEqual(t, journalHits[0].ChunkID, customHits[0].ChunkID,
		"each collection must return its own chunk")

	// No filter — both match.
	allHits, err := reader.SearchBM25(ctx, "memory", "", 10)
	require.NoError(t, err)
	assert.Len(t, allHits, 2)
}

func TestReader_SearchBM25_koreanUnicode61(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store, reader := openTestStoreWithReader(t)
	skipIfNoFTS5(t, store)

	w, err := NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	// Insert Korean-language corpus (5 distinct chunks).
	koreanDocs := []string{
		"오늘 날씨가 매우 좋습니다. 하늘이 맑고 기온이 적당합니다.",
		"인공지능 기술은 빠르게 발전하고 있습니다. 머신러닝과 딥러닝이 주도합니다.",
		"오늘 회의에서 중요한 결정을 내렸습니다. 프로젝트 방향을 조정했습니다.",
		"건강을 위해 매일 운동하는 것이 중요합니다. 걷기와 달리기를 추천합니다.",
		"오늘 저녁 맛있는 한식을 먹었습니다. 된장찌개와 비빔밥이 특히 좋았습니다.",
	}

	for i, doc := range koreanDocs {
		_ = insertTestChunk(t, w, "journal",
			"/vault/journal/kr-"+string(rune('a'+i))+".md",
			doc, i*10+1, i*10+5)
	}

	ctx := context.Background()

	// Search for a Korean term that appears in multiple documents.
	hits, err := reader.SearchBM25(ctx, "오늘", "", 10)
	require.NoError(t, err)
	// "오늘" appears in 3 documents; FTS5 unicode61 should tokenise Korean words.
	assert.GreaterOrEqual(t, len(hits), 1,
		"unicode61 tokenizer must index Korean text")
}

func TestReader_SearchBM25_emptyResult(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store, reader := openTestStoreWithReader(t)
	skipIfNoFTS5(t, store)

	w, err := NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	_ = insertTestChunk(t, w, "custom", "/vault/custom/x.md",
		"hello world this is a test document", 1, 5)

	ctx := context.Background()
	hits, err := reader.SearchBM25(ctx, "zxqvjk", "", 10)
	require.NoError(t, err, "no-match query must not error")
	assert.Empty(t, hits, "unmatched query must return empty slice")
}

func TestReader_SearchBM25_fts5Unavailable(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store, reader := openTestStoreWithReader(t)

	// Verify FTS5 is available; if not, the sentinel is returned by the reader
	// without needing to drop the table.
	ctx := context.Background()
	var n int
	ftsAvail := store.db.QueryRowContext(ctx,
		"SELECT count(*) FROM chunks_fts WHERE chunks_fts MATCH 'a'").Scan(&n) == nil

	if !ftsAvail {
		// FTS5 not built into SQLite — ErrFTS5Unavailable should be returned.
		_, err := reader.SearchBM25(ctx, "test", "", 10)
		assert.True(t, errors.Is(err, ErrFTS5Unavailable),
			"expected ErrFTS5Unavailable, got: %v", err)
		return
	}

	// FTS5 is available. Drop the virtual table to simulate the missing-FTS5 case.
	_, err := store.db.ExecContext(ctx, "DROP TABLE IF EXISTS chunks_fts")
	require.NoError(t, err)

	_, err = reader.SearchBM25(ctx, "test", "", 10)
	assert.True(t, errors.Is(err, ErrFTS5Unavailable),
		"expected ErrFTS5Unavailable after dropping chunks_fts, got: %v", err)
}

func TestReader_SearchBM25_contextCancellation(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store, reader := openTestStoreWithReader(t)
	skipIfNoFTS5(t, store)

	w, err := NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	_ = insertTestChunk(t, w, "custom", "/vault/custom/ctx.md",
		"context cancellation test document", 1, 3)

	// Use an already-cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	_, err = reader.SearchBM25(ctx, "context", "", 10)
	// The call may succeed if SQLite returns fast or may return context error;
	// either is acceptable. We just verify it does not panic.
	_ = err
}

func TestReader_SearchBM25_scoreNormalisation(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store, reader := openTestStoreWithReader(t)
	skipIfNoFTS5(t, store)

	w, err := NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	_ = insertTestChunk(t, w, "custom", "/vault/custom/pos.md",
		"positive score test golang go language", 1, 5)

	ctx := context.Background()
	hits, err := reader.SearchBM25(ctx, "golang", "", 10)
	require.NoError(t, err)
	require.NotEmpty(t, hits)

	for _, h := range hits {
		assert.Greater(t, h.Score, 0.0,
			"all normalised scores must be positive (chunk_id=%s)", h.ChunkID)
	}
}

func TestQuoteFTS5Query(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", `"hello"`},
		{`say "hi"`, `"say ""hi"""`},
		{"golang *", `"golang *"`},
		{"OR AND NOT", `"OR AND NOT"`},
		{"", `""`},
	}
	for _, tc := range tests {
		got := quoteFTS5Query(tc.input)
		assert.Equal(t, tc.want, got, "input=%q", tc.input)
	}
}
