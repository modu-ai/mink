// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test doubles ---

// mockVectorReader implements VectorReader for tests.
type mockVectorReader struct {
	hits []Hit
	err  error
}

func (m *mockVectorReader) SearchVector(_ context.Context, _ []float32, _ string, _ int) ([]Hit, error) {
	return m.hits, m.err
}

// mockVChunkLookup implements ChunkLookup for vsearch tests.
// Named distinctly to avoid collision with search_test.go's mockChunkLookup.
type mockVChunkLookup struct {
	chunks []qmd.Chunk
	err    error
}

func (m *mockVChunkLookup) LookupChunks(_ context.Context, ids []string) ([]qmd.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return only chunks whose ID appears in ids.
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	var out []qmd.Chunk
	for _, ch := range m.chunks {
		if idSet[ch.ID] {
			out = append(out, ch)
		}
	}
	return out, nil
}

// fakeChunk builds a minimal qmd.Chunk for test assertions.
func fakeChunk(id, content string) qmd.Chunk {
	return qmd.Chunk{
		ID:        id,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

// fakeEmbedFunc returns a fixed 1024-d vector.
func fakeEmbedFunc(_ context.Context, _, _ string) ([]float32, error) {
	v := make([]float32, 1024)
	for i := range 1024 {
		v[i] = float32(i) * 0.001
	}
	return v, nil
}

// --- tests ---

func TestRunVector_happyPath(t *testing.T) {
	chunks := []qmd.Chunk{
		fakeChunk("c1", "golang concurrency patterns"),
		fakeChunk("c2", "goroutine leak detection"),
	}

	reader := &mockVectorReader{
		hits: []Hit{
			{ChunkID: "c1", Score: 0.1},  // distance 0.1 → similarity 0.9
			{ChunkID: "c2", Score: 0.25}, // distance 0.25 → similarity 0.75
		},
	}
	lookup := &mockVChunkLookup{chunks: chunks}

	runner := NewVectorRunner(reader, lookup, fakeEmbedFunc, "mxbai-embed-large")
	results, err := runner.RunVector(context.Background(), "golang concurrency", qmd.SearchOpts{Limit: 10})

	require.NoError(t, err)
	assert.Len(t, results, 2)
	// First result has lower distance (higher similarity).
	assert.Equal(t, "c1", results[0].Chunk.ID)
	assert.InDelta(t, 0.9, results[0].Score, 1e-6)
	assert.Equal(t, "c2", results[1].Chunk.ID)
	assert.InDelta(t, 0.75, results[1].Score, 1e-6)
}

func TestRunVector_emptyQueryReturnsErrEmptyQuery(t *testing.T) {
	runner := NewVectorRunner(&mockVectorReader{}, &mockVChunkLookup{}, fakeEmbedFunc, "model")
	_, err := runner.RunVector(context.Background(), "   ", qmd.SearchOpts{})
	require.ErrorIs(t, err, ErrEmptyQuery)
}

func TestRunVector_embedErrorPropagated(t *testing.T) {
	embedErr := errors.New("ollama: server unreachable")
	failEmbed := func(_ context.Context, _, _ string) ([]float32, error) {
		return nil, embedErr
	}
	runner := NewVectorRunner(&mockVectorReader{}, &mockVChunkLookup{}, failEmbed, "model")
	_, err := runner.RunVector(context.Background(), "query", qmd.SearchOpts{})
	require.Error(t, err)
	assert.ErrorIs(t, err, embedErr)
}

func TestRunVector_readerErrVec0UnavailablePropagated(t *testing.T) {
	reader := &mockVectorReader{err: ErrVec0Unavailable}
	runner := NewVectorRunner(reader, &mockVChunkLookup{}, fakeEmbedFunc, "model")
	_, err := runner.RunVector(context.Background(), "query", qmd.SearchOpts{})
	require.ErrorIs(t, err, ErrVec0Unavailable)
}

func TestRunVector_deletedChunkSilentlySkipped(t *testing.T) {
	// Reader returns c1 and c2, but lookup only knows c1.
	reader := &mockVectorReader{
		hits: []Hit{
			{ChunkID: "c1", Score: 0.1},
			{ChunkID: "c2", Score: 0.2}, // deleted between search and hydration
		},
	}
	lookup := &mockVChunkLookup{
		chunks: []qmd.Chunk{fakeChunk("c1", "content one")},
	}

	runner := NewVectorRunner(reader, lookup, fakeEmbedFunc, "model")
	results, err := runner.RunVector(context.Background(), "query", qmd.SearchOpts{Limit: 10})

	require.NoError(t, err)
	assert.Len(t, results, 1, "deleted chunk must be silently skipped")
	assert.Equal(t, "c1", results[0].Chunk.ID)
}

func TestRunVector_defaultLimitApplied(t *testing.T) {
	// Verify that limit 0 is treated as defaultLimit (10).
	hits := make([]Hit, 5)
	chunks := make([]qmd.Chunk, 5)
	for i := range 5 {
		id := "c" + string(rune('0'+i))
		hits[i] = Hit{ChunkID: id, Score: float64(i) * 0.1}
		chunks[i] = fakeChunk(id, "content")
	}

	var capturedK int
	limitCapture := &limitCaptureReader{hits: hits, captureK: &capturedK}
	runner := NewVectorRunner(limitCapture, &mockVChunkLookup{chunks: chunks}, fakeEmbedFunc, "model")

	_, err := runner.RunVector(context.Background(), "query", qmd.SearchOpts{Limit: 0})
	require.NoError(t, err)
	assert.Equal(t, defaultLimit, capturedK)
}

func TestRunVector_emptyResults(t *testing.T) {
	reader := &mockVectorReader{hits: nil}
	runner := NewVectorRunner(reader, &mockVChunkLookup{}, fakeEmbedFunc, "model")
	results, err := runner.RunVector(context.Background(), "query", qmd.SearchOpts{Limit: 5})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// limitCaptureReader captures the k value passed to SearchVector.
type limitCaptureReader struct {
	hits     []Hit
	captureK *int
}

func (r *limitCaptureReader) SearchVector(_ context.Context, _ []float32, _ string, k int) ([]Hit, error) {
	*r.captureK = k
	return r.hits, nil
}
