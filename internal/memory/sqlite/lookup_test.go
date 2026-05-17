// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLookupStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "lookup_test.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func insertLookupChunk(t *testing.T, w *Writer, collection, sourcePath, content string, start, end int) string {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	f := qmd.File{
		Collection:  collection,
		SourcePath:  sourcePath,
		ContentHash: "lkhash-" + sourcePath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	fileID, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)

	chunkID := qmd.ChunkID(sourcePath, start, end, "testhash", qmd.DefaultModelVersion)
	chunk := qmd.Chunk{
		ID:               chunkID,
		FileID:           fileID,
		Collection:       collection,
		SourcePath:       sourcePath,
		StartLine:        start,
		EndLine:          end,
		Content:          content,
		EmbeddingPending: true,
		ModelVersion:     qmd.DefaultModelVersion,
		CreatedAt:        now,
	}
	require.NoError(t, w.Insert(ctx, chunk))
	return chunkID
}

func TestChunkLookupStore_LookupChunks_singleChunk(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := setupLookupStore(t)
	w, err := NewWriter(store, "")
	require.NoError(t, err)

	id := insertLookupChunk(t, w, "journal", "/vault/j/a.md", "hello world", 1, 5)
	require.NoError(t, w.Close())

	lookup := NewChunkLookupStore(store)
	chunks, err := lookup.LookupChunks(context.Background(), []string{id})
	require.NoError(t, err)
	require.Len(t, chunks, 1)

	assert.Equal(t, id, chunks[0].ID)
	assert.Equal(t, "journal", chunks[0].Collection)
	assert.Equal(t, "/vault/j/a.md", chunks[0].SourcePath)
	assert.Equal(t, "hello world", chunks[0].Content)
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Equal(t, 5, chunks[0].EndLine)
	assert.True(t, chunks[0].EmbeddingPending)
}

func TestChunkLookupStore_LookupChunks_multipleChunks(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := setupLookupStore(t)
	w, err := NewWriter(store, "")
	require.NoError(t, err)

	id1 := insertLookupChunk(t, w, "custom", "/vault/c/1.md", "content one", 1, 3)
	id2 := insertLookupChunk(t, w, "custom", "/vault/c/2.md", "content two", 4, 6)
	require.NoError(t, w.Close())

	lookup := NewChunkLookupStore(store)
	chunks, err := lookup.LookupChunks(context.Background(), []string{id1, id2})
	require.NoError(t, err)
	assert.Len(t, chunks, 2)
}

func TestChunkLookupStore_LookupChunks_emptyInput(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := setupLookupStore(t)

	lookup := NewChunkLookupStore(store)
	chunks, err := lookup.LookupChunks(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, chunks)
}

func TestChunkLookupStore_LookupChunks_nonexistentID(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := setupLookupStore(t)

	lookup := NewChunkLookupStore(store)
	chunks, err := lookup.LookupChunks(context.Background(), []string{"nonexistent-id"})
	require.NoError(t, err)
	assert.Empty(t, chunks, "missing chunk must be silently omitted")
}

func TestBM25ReaderAdapter_delegatesToReader(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	store := setupLookupStore(t)
	w, err := NewWriter(store, "")
	require.NoError(t, err)

	// Insert a chunk so the FTS5 probe can succeed if FTS5 is available.
	insertLookupChunk(t, w, "custom", "/vault/adapter/test.md", "adapter test document", 1, 5)
	require.NoError(t, w.Close())

	reader := NewReader(store)
	adapter := NewBM25ReaderAdapter(reader)

	// Probe FTS5 availability.
	var n int
	ftsAvail := store.db.QueryRow("SELECT count(*) FROM chunks_fts WHERE chunks_fts MATCH 'a'").Scan(&n) == nil

	ctx := context.Background()
	hits, err := adapter.SearchBM25(ctx, "adapter", "", 10)
	if !ftsAvail {
		// Without FTS5 the adapter must propagate ErrFTS5Unavailable.
		assert.Error(t, err)
		return
	}

	require.NoError(t, err)
	// Hits are empty or contain adapter chunk; we just verify the call succeeds.
	assert.NotNil(t, hits)
}
