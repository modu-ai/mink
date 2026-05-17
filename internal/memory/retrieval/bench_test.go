// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// corpus10K holds the pre-built synthetic corpus for 10K-chunk benchmarks.
// Populated once by init10KCorpus.
var corpus10K struct {
	bm25Hits   []Hit
	vecHits    []Hit
	chunks     []qmd.Chunk
	embeddings map[string][]float32
}

// init10KCorpus builds a synthetic 10,000-chunk corpus in memory.
// Called from TestMain equivalent (sync.Once pattern inside the bench).
func init10KCorpus(n int) {
	const dim = 1024
	corpus10K.bm25Hits = make([]Hit, n)
	corpus10K.vecHits = make([]Hit, n)
	corpus10K.chunks = make([]qmd.Chunk, n)
	corpus10K.embeddings = make(map[string][]float32, n)

	// Shared embedding vector (same for all chunks to minimise alloc overhead).
	sharedEmb := make([]float32, dim)
	for i := range dim {
		sharedEmb[i] = float32(i) * 0.001
	}

	now := time.Now()
	for i := range n {
		id := fmt.Sprintf("bench-chunk-%d", i)

		corpus10K.bm25Hits[i] = Hit{
			ChunkID: id,
			Score:   float64(n-i) * 0.1, // descending BM25 scores
		}
		corpus10K.vecHits[i] = Hit{
			ChunkID: id,
			Score:   float64(i) * 0.0001, // ascending cosine distance (lower = closer)
		}
		corpus10K.chunks[i] = qmd.Chunk{
			ID:        id,
			Content:   fmt.Sprintf("synthetic content for chunk %d", i),
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
		corpus10K.embeddings[id] = sharedEmb
	}
}

// staticBM25Reader returns pre-computed hits without any computation.
type staticBM25Reader struct{ hits []Hit }

func (s *staticBM25Reader) SearchBM25(_ context.Context, _, _ string, k int) ([]Hit, error) {
	if k > len(s.hits) {
		return s.hits, nil
	}
	return s.hits[:k], nil
}

// staticVectorReader returns pre-computed hits without any computation.
type staticVectorReader struct{ hits []Hit }

func (s *staticVectorReader) SearchVector(_ context.Context, _ []float32, _ string, k int) ([]Hit, error) {
	if k > len(s.hits) {
		return s.hits, nil
	}
	return s.hits[:k], nil
}

// staticChunkLookup returns a pre-indexed map of chunks.
type staticChunkLookup struct{ byID map[string]qmd.Chunk }

func (s *staticChunkLookup) LookupChunks(_ context.Context, ids []string) ([]qmd.Chunk, error) {
	out := make([]qmd.Chunk, 0, len(ids))
	for _, id := range ids {
		if ch, ok := s.byID[id]; ok {
			out = append(out, ch)
		}
	}
	return out, nil
}

// staticEmbeddingLookup returns a pre-built embeddings map.
type staticEmbeddingLookup struct{ embeddings map[string][]float32 }

func (s *staticEmbeddingLookup) LookupEmbeddings(_ context.Context, ids []string) (map[string][]float32, error) {
	result := make(map[string][]float32, len(ids))
	for _, id := range ids {
		if emb, ok := s.embeddings[id]; ok {
			result[id] = emb
		}
	}
	return result, nil
}

// buildBenchRunner constructs a HybridRunner backed by the static corpus.
func buildBenchRunner(n int) *HybridRunner {
	byID := make(map[string]qmd.Chunk, n)
	for _, ch := range corpus10K.chunks[:n] {
		byID[ch.ID] = ch
	}

	return NewHybridRunner(
		&staticBM25Reader{hits: corpus10K.bm25Hits[:n]},
		&staticVectorReader{hits: corpus10K.vecHits[:n]},
		&staticChunkLookup{byID: byID},
		&staticEmbeddingLookup{embeddings: corpus10K.embeddings},
		fakeEmbedFunc,
		"mxbai-embed-large",
		DefaultHybridConfig(),
	)
}

// TestRunHybrid_10K_latency is a CI-friendly regression test that verifies
// RunHybrid on a 10K synthetic corpus completes in under 1 second.
//
// The ≤200ms target from plan.md §1 is validated separately by
// BenchmarkRunHybrid_10K under non-CI conditions.
func TestRunHybrid_10K_latency(t *testing.T) {
	const n = 10_000
	init10KCorpus(n)

	runner := buildBenchRunner(n)

	start := time.Now()
	results, err := runner.RunHybrid(context.Background(), "synthetic content", qmd.SearchOpts{Limit: 10})
	elapsed := time.Since(start)

	// ErrFellBackToBM25 is acceptable (fakeEmbedFunc does not fail, but
	// if it did fall back we still want to measure timing).
	if err != nil && !errors.Is(err, ErrFellBackToBM25) {
		require.NoError(t, err)
	}

	assert.NotEmpty(t, results, "10K corpus must return non-empty results")
	assert.Less(t, elapsed, time.Second,
		"RunHybrid on 10K corpus must complete in < 1s (CI-friendly), got %v", elapsed)
}

// BenchmarkRunHybrid_10K measures the latency of RunHybrid on a 10K-chunk
// synthetic corpus.  The stretch goal is ≤200ms median latency (plan.md §1).
//
// Run with: go test -bench=BenchmarkRunHybrid_10K -benchtime=10x ./internal/memory/retrieval/
func BenchmarkRunHybrid_10K(b *testing.B) {
	const n = 10_000
	init10KCorpus(n)

	runner := buildBenchRunner(n)

	ctx := context.Background()
	opts := qmd.SearchOpts{Limit: 10}

	b.ReportAllocs()

	for b.Loop() {
		results, err := runner.RunHybrid(ctx, "synthetic benchmark query", opts)
		if err != nil && !errors.Is(err, ErrFellBackToBM25) {
			b.Fatal("unexpected error:", err)
		}
		if len(results) == 0 {
			b.Fatal("expected non-empty results")
		}
	}
}
