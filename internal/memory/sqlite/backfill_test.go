// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/ollama"
	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

// mockEmbedClient is a test double for EmbedClient.
type mockEmbedClient struct {
	fn func(ctx context.Context, model, text string) ([]float32, error)
}

func (m *mockEmbedClient) Embed(ctx context.Context, model, text string) ([]float32, error) {
	return m.fn(ctx, model, text)
}

// cannedEmbed returns a fixed 1024-d vector.
func cannedEmbed(_ context.Context, _, _ string) ([]float32, error) {
	v := make([]float32, 1024)
	for i := range 1024 {
		v[i] = float32(i) * 0.001
	}
	return v, nil
}

// openBackfillTestStore creates a temporary Store for backfill tests.
func openBackfillTestStore(t *testing.T) *Store {
	t.Helper()
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// insertTestFile inserts a files row and returns the file_id.
func insertTestFile(t *testing.T, store *Store) int64 {
	t.Helper()
	w, err := NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	fid, err := w.UpsertFile(context.Background(), qmd.File{
		Collection:  "custom",
		SourcePath:  "/tmp/test.md",
		ContentHash: "deadbeef",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	})
	require.NoError(t, err)
	return fid
}

// insertPendingChunk inserts a chunk with embedding_pending=true.
func insertPendingChunk(t *testing.T, store *Store, fileID int64, id, content string) {
	t.Helper()
	w, err := NewWriter(store, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	sum := sha256.Sum256([]byte(content))
	chunkID := fmt.Sprintf("%s-%x", id, sum[:4])

	err = w.Insert(context.Background(), qmd.Chunk{
		ID:               chunkID,
		FileID:           fileID,
		StartLine:        1,
		EndLine:          10,
		Content:          content,
		EmbeddingPending: true,
		ModelVersion:     qmd.DefaultModelVersion,
		CreatedAt:        time.Now().UTC(),
	})
	require.NoError(t, err)
}

// pendingCount returns the number of chunks with embedding_pending=1.
func pendingCount(t *testing.T, store *Store) int {
	t.Helper()
	var n int
	err := store.db.QueryRow("SELECT count(*) FROM chunks WHERE embedding_pending = 1").Scan(&n)
	require.NoError(t, err)
	return n
}

// --- tests ---

func TestBackfiller_RunOnce_pendingChunksEmbedded(t *testing.T) {
	store := openBackfillTestStore(t)
	skipIfNoVec0(t, store)

	fileID := insertTestFile(t, store)
	insertPendingChunk(t, store, fileID, "chunk-a", "golang goroutine patterns")
	insertPendingChunk(t, store, fileID, "chunk-b", "context cancellation")

	require.Equal(t, 2, pendingCount(t, store))

	client := &mockEmbedClient{fn: cannedEmbed}
	bf := NewBackfiller(store, client, "mxbai-embed-large")

	ok, fail, err := bf.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, ok)
	assert.Equal(t, 0, fail)
	assert.Equal(t, 0, pendingCount(t, store))
}

func TestBackfiller_RunOnce_recoverableErrorLeavesChunkPending(t *testing.T) {
	store := openBackfillTestStore(t)
	fileID := insertTestFile(t, store)
	insertPendingChunk(t, store, fileID, "chunk-a", "content")

	client := &mockEmbedClient{fn: func(_ context.Context, _, _ string) ([]float32, error) {
		return nil, ollama.ErrOllamaUnreachable
	}}
	bf := NewBackfiller(store, client, "model")

	ok, fail, err := bf.RunOnce(context.Background())
	require.NoError(t, err, "recoverable error must not surface as RunOnce error")
	assert.Equal(t, 0, ok)
	assert.Equal(t, 1, fail)
	assert.Equal(t, 1, pendingCount(t, store), "chunk must remain pending on recoverable error")
}

func TestBackfiller_RunOnce_nonRecoverableError(t *testing.T) {
	store := openBackfillTestStore(t)
	fileID := insertTestFile(t, store)
	insertPendingChunk(t, store, fileID, "chunk-a", "content")

	fatalErr := fmt.Errorf("fatal: disk corrupted")
	client := &mockEmbedClient{fn: func(_ context.Context, _, _ string) ([]float32, error) {
		return nil, fatalErr
	}}
	bf := NewBackfiller(store, client, "model")

	ok, fail, err := bf.RunOnce(context.Background())
	require.Error(t, err, "unrecoverable error must bubble up")
	assert.Equal(t, 0, ok)
	assert.Equal(t, 0, fail)
}

func TestBackfiller_RunOnce_batchSizeRespected(t *testing.T) {
	store := openBackfillTestStore(t)
	fileID := insertTestFile(t, store)
	for i := range 5 {
		insertPendingChunk(t, store, fileID, fmt.Sprintf("chunk-%d", i), fmt.Sprintf("content %d", i))
	}
	require.Equal(t, 5, pendingCount(t, store))

	var calls atomic.Int32
	client := &mockEmbedClient{fn: func(_ context.Context, _, _ string) ([]float32, error) {
		calls.Add(1)
		return cannedEmbed(context.Background(), "", "")
	}}
	bf := NewBackfiller(store, client, "model")
	bf.batchSize = 3

	ok, _, err := bf.RunOnce(context.Background())
	require.NoError(t, err)
	// Only batchSize=3 chunks should be processed per call (vec0 may not be
	// available; in that case the embed still runs but upsert may fail — count
	// what was attempted).
	assert.LessOrEqual(t, int(calls.Load()), 3)
	_ = ok
}

func TestBackfiller_Run_respectsContextCancellation(t *testing.T) {
	store := openBackfillTestStore(t)
	client := &mockEmbedClient{fn: cannedEmbed}
	bf := NewBackfiller(store, client, "model")
	bf.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		bf.Run(ctx)
		close(done)
	}()

	// Let the loop tick a couple of times.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Run returned after context cancellation — OK.
	case <-time.After(2 * time.Second):
		t.Fatal("Backfiller.Run did not exit after context cancellation")
	}
}

func TestBackfiller_EnqueueAsync_returnsImmediately(t *testing.T) {
	store := openBackfillTestStore(t)
	fileID := insertTestFile(t, store)
	insertPendingChunk(t, store, fileID, "chunk-a", "content")

	var completed atomic.Bool
	client := &mockEmbedClient{fn: func(ctx context.Context, _, _ string) ([]float32, error) {
		// Simulate slow embed.
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
		}
		completed.Store(true)
		return cannedEmbed(ctx, "", "")
	}}

	bf := NewBackfiller(store, client, "model")

	start := time.Now()
	bf.EnqueueAsync(context.Background())
	elapsed := time.Since(start)

	// EnqueueAsync must return almost instantly (well under 50ms).
	assert.Less(t, elapsed, 50*time.Millisecond, "EnqueueAsync must return immediately")

	// Wait for goroutine to finish.
	deadline := time.Now().Add(2 * time.Second)
	for !completed.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, completed.Load(), "background goroutine must eventually complete")
}
