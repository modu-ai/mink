// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test doubles for hybrid runner ---

type mockEmbeddingLookup struct {
	embeddings map[string][]float32
	err        error
}

func (m *mockEmbeddingLookup) LookupEmbeddings(_ context.Context, ids []string) (map[string][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make(map[string][]float32, len(ids))
	for _, id := range ids {
		if emb, ok := m.embeddings[id]; ok {
			result[id] = emb
		}
	}
	return result, nil
}

type mockHybridBM25Reader struct {
	hits     []Hit
	err      error
	captureK *int
}

func (m *mockHybridBM25Reader) SearchBM25(_ context.Context, _, _ string, k int) ([]Hit, error) {
	if m.captureK != nil {
		*m.captureK = k
	}
	return m.hits, m.err
}

type mockHybridVectorReader struct {
	hits []Hit
	err  error
}

func (m *mockHybridVectorReader) SearchVector(_ context.Context, _ []float32, _ string, _ int) ([]Hit, error) {
	return m.hits, m.err
}

type mockHybridChunkLookup struct {
	chunks []qmd.Chunk
}

func (m *mockHybridChunkLookup) LookupChunks(_ context.Context, ids []string) ([]qmd.Chunk, error) {
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

// buildHybridRunner creates a HybridRunner with test doubles and the given config.
func buildHybridRunner(
	bm25Hits []Hit,
	vecHits []Hit,
	chunks []qmd.Chunk,
	embeddings map[string][]float32,
	cfg HybridConfig,
) *HybridRunner {
	return NewHybridRunner(
		&mockHybridBM25Reader{hits: bm25Hits},
		&mockHybridVectorReader{hits: vecHits},
		&mockHybridChunkLookup{chunks: chunks},
		&mockEmbeddingLookup{embeddings: embeddings},
		fakeEmbedFunc,
		"mxbai-embed-large",
		cfg,
	)
}

// --- tests ---

func TestRunHybrid_emptyQueryReturnsErrEmptyQuery(t *testing.T) {
	runner := buildHybridRunner(nil, nil, nil, nil, DefaultHybridConfig())
	_, err := runner.RunHybrid(context.Background(), "   ", qmd.SearchOpts{Limit: 5})
	require.ErrorIs(t, err, ErrEmptyQuery)
}

func TestRunHybrid_additiveScoreFormula(t *testing.T) {
	// Known BM25 raw scores and cosine distances → verify additive formula exactly.
	// bm25_raw: c1=10, c2=5, c3=8 → max=10 → norms: 1.0, 0.5, 0.8
	// vec distances: c1=0.2 (cosine=0.8), c2=0.3 (cosine=0.7), c3=0.1 (cosine=0.9)
	// α=0.7, β=0.3, γ=0:
	//   c1: 0.7*0.8 + 0.3*1.0 = 0.56 + 0.30 = 0.86
	//   c2: 0.7*0.7 + 0.3*0.5 = 0.49 + 0.15 = 0.64
	//   c3: 0.7*0.9 + 0.3*0.8 = 0.63 + 0.24 = 0.87
	// Expected order: c3 > c1 > c2.

	chunks := []qmd.Chunk{
		{ID: "c1", Content: "golang concurrency", CreatedAt: time.Now()},
		{ID: "c2", Content: "python ml", CreatedAt: time.Now()},
		{ID: "c3", Content: "rust safety", CreatedAt: time.Now()},
	}
	bm25Hits := []Hit{
		{ChunkID: "c1", Score: 10},
		{ChunkID: "c2", Score: 5},
		{ChunkID: "c3", Score: 8},
	}
	vecHits := []Hit{
		{ChunkID: "c1", Score: 0.2}, // distance → cosine = 0.8
		{ChunkID: "c2", Score: 0.3}, // distance → cosine = 0.7
		{ChunkID: "c3", Score: 0.1}, // distance → cosine = 0.9
	}

	cfg := DefaultHybridConfig()
	runner := buildHybridRunner(bm25Hits, vecHits, chunks, nil, cfg)

	results, err := runner.RunHybrid(context.Background(), "query", qmd.SearchOpts{Limit: 3})
	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify scores within float epsilon.
	scoreByID := make(map[string]float64)
	for _, r := range results {
		scoreByID[r.Chunk.ID] = r.Score
	}

	assert.InDelta(t, 0.86, scoreByID["c1"], 1e-9, "c1 score")
	assert.InDelta(t, 0.64, scoreByID["c2"], 1e-9, "c2 score")
	assert.InDelta(t, 0.87, scoreByID["c3"], 1e-9, "c3 score")

	// Order: c3 > c1 > c2.
	assert.Equal(t, "c3", results[0].Chunk.ID)
	assert.Equal(t, "c1", results[1].Chunk.ID)
	assert.Equal(t, "c2", results[2].Chunk.ID)
}

func TestRunHybrid_alphaOnlyEquivalentToVectorRanking(t *testing.T) {
	// α=1, β=0, γ=0 → pure cosine ranking.
	chunks := []qmd.Chunk{
		{ID: "c1", Content: "alpha test", CreatedAt: time.Now()},
		{ID: "c2", Content: "alpha test", CreatedAt: time.Now()},
	}
	bm25Hits := []Hit{
		{ChunkID: "c1", Score: 100}, // high BM25 but should not influence result
		{ChunkID: "c2", Score: 1},
	}
	vecHits := []Hit{
		{ChunkID: "c1", Score: 0.4}, // cosine = 0.6
		{ChunkID: "c2", Score: 0.1}, // cosine = 0.9 → should rank first
	}

	cfg := DefaultHybridConfig()
	cfg.Alpha = 1.0
	cfg.Beta = 0.0
	cfg.Gamma = 0.0

	runner := buildHybridRunner(bm25Hits, vecHits, chunks, nil, cfg)
	results, err := runner.RunHybrid(context.Background(), "query", qmd.SearchOpts{Limit: 2})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// c2 has higher cosine, so it should rank first with α=1.
	assert.Equal(t, "c2", results[0].Chunk.ID, "pure cosine: c2 (0.9) > c1 (0.6)")
	assert.InDelta(t, 0.9, results[0].Score, 1e-9)
	assert.InDelta(t, 0.6, results[1].Score, 1e-9)
}

func TestRunHybrid_betaOnlyEquivalentToBM25Ranking(t *testing.T) {
	// α=0, β=1, γ=0 → pure BM25-normalised ranking.
	chunks := []qmd.Chunk{
		{ID: "c1", Content: "bm25 test", CreatedAt: time.Now()},
		{ID: "c2", Content: "bm25 test", CreatedAt: time.Now()},
	}
	bm25Hits := []Hit{
		{ChunkID: "c1", Score: 5},  // norm = 0.5
		{ChunkID: "c2", Score: 10}, // norm = 1.0 → should rank first
	}
	vecHits := []Hit{
		{ChunkID: "c1", Score: 0.1}, // cosine = 0.9 but should not matter
		{ChunkID: "c2", Score: 0.9}, // cosine = 0.1 but should not matter
	}

	cfg := DefaultHybridConfig()
	cfg.Alpha = 0.0
	cfg.Beta = 1.0
	cfg.Gamma = 0.0

	runner := buildHybridRunner(bm25Hits, vecHits, chunks, nil, cfg)
	results, err := runner.RunHybrid(context.Background(), "bm25", qmd.SearchOpts{Limit: 2})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// c2 has higher BM25, so it should rank first with β=1.
	assert.Equal(t, "c2", results[0].Chunk.ID, "pure BM25: c2 (norm=1.0) > c1 (norm=0.5)")
	assert.InDelta(t, 1.0, results[0].Score, 1e-9)
	assert.InDelta(t, 0.5, results[1].Score, 1e-9)
}

func TestRunHybrid_gammaPositive_olderChunksRankLower(t *testing.T) {
	// γ > 0: a 7-day-old chunk should rank higher than a 90-day-old chunk.
	now := time.Now()
	chunks := []qmd.Chunk{
		{ID: "c1", Content: "content same", CreatedAt: now.Add(-7 * 24 * time.Hour)},
		{ID: "c2", Content: "content same", CreatedAt: now.Add(-90 * 24 * time.Hour)},
	}
	// Equal BM25 and vector scores so decay is the deciding factor.
	bm25Hits := []Hit{
		{ChunkID: "c1", Score: 5},
		{ChunkID: "c2", Score: 5},
	}
	vecHits := []Hit{
		{ChunkID: "c1", Score: 0.2},
		{ChunkID: "c2", Score: 0.2},
	}

	cfg := DefaultHybridConfig()
	cfg.Alpha = 0.0
	cfg.Beta = 0.0
	cfg.Gamma = 1.0
	cfg.DecayHalfLife = DefaultDecayHalfLife

	runner := buildHybridRunner(bm25Hits, vecHits, chunks, nil, cfg)
	results, err := runner.RunHybrid(context.Background(), "decay", qmd.SearchOpts{Limit: 2})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// c1 (7 days) should rank above c2 (90 days) with γ > 0.
	assert.Equal(t, "c1", results[0].Chunk.ID, "newer chunk must rank higher when gamma>0")
	assert.Greater(t, results[0].Score, results[1].Score)
}

func TestRunHybrid_embedFailureFallbackToBM25(t *testing.T) {
	// When embed returns a ShouldFallbackToBM25-eligible error, results must be
	// BM25-only and err must wrap ErrFellBackToBM25.
	chunks := []qmd.Chunk{
		{ID: "c1", Content: "fallback content", CreatedAt: time.Now()},
	}
	bm25Hits := []Hit{{ChunkID: "c1", Score: 10}}

	failEmbed := func(_ context.Context, _, _ string) ([]float32, error) {
		return nil, ErrVec0Unavailable
	}

	runner := NewHybridRunner(
		&mockHybridBM25Reader{hits: bm25Hits},
		&mockHybridVectorReader{},
		&mockHybridChunkLookup{chunks: chunks},
		&mockEmbeddingLookup{},
		failEmbed,
		"model",
		DefaultHybridConfig(),
	)

	results, err := runner.RunHybrid(context.Background(), "query", qmd.SearchOpts{Limit: 5})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFellBackToBM25), "error must wrap ErrFellBackToBM25")
	assert.NotEmpty(t, results, "BM25 fallback must return results")
}

func TestRunHybrid_vecReaderUnavailableFallback(t *testing.T) {
	// When VectorReader returns ErrVec0Unavailable, must fall back to BM25.
	chunks := []qmd.Chunk{
		{ID: "c1", Content: "vec unavailable test", CreatedAt: time.Now()},
	}
	bm25Hits := []Hit{{ChunkID: "c1", Score: 5}}

	runner := NewHybridRunner(
		&mockHybridBM25Reader{hits: bm25Hits},
		&mockHybridVectorReader{err: ErrVec0Unavailable},
		&mockHybridChunkLookup{chunks: chunks},
		&mockEmbeddingLookup{},
		fakeEmbedFunc,
		"model",
		DefaultHybridConfig(),
	)

	results, err := runner.RunHybrid(context.Background(), "query", qmd.SearchOpts{Limit: 5})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFellBackToBM25))
	assert.NotEmpty(t, results)
}

func TestRunHybrid_candidatePoolMultiplier(t *testing.T) {
	// CandidatePoolMultiplier = 2 → BM25 reader called with k = 2 * limit.
	var capturedK int
	chunks := []qmd.Chunk{{ID: "c1", Content: "pool test", CreatedAt: time.Now()}}

	bm25Reader := &mockHybridBM25Reader{
		hits:     []Hit{{ChunkID: "c1", Score: 1}},
		captureK: &capturedK,
	}

	cfg := DefaultHybridConfig()
	cfg.CandidatePoolMultiplier = 2

	runner := NewHybridRunner(
		bm25Reader,
		&mockHybridVectorReader{hits: []Hit{{ChunkID: "c1", Score: 0.1}}},
		&mockHybridChunkLookup{chunks: chunks},
		&mockEmbeddingLookup{},
		fakeEmbedFunc,
		"model",
		cfg,
	)

	limit := 5
	_, _ = runner.RunHybrid(context.Background(), "query", qmd.SearchOpts{Limit: limit})
	assert.Equal(t, 20, capturedK, "k' must be max(limit*multiplier, 20) = max(10, 20) = 20")
}

func TestRunHybrid_candidatePoolMultiplierLarge(t *testing.T) {
	// With limit=20 and multiplier=2, k'=40 (no floor needed).
	var capturedK int
	chunks := []qmd.Chunk{{ID: "c1", Content: "large pool test", CreatedAt: time.Now()}}

	bm25Reader := &mockHybridBM25Reader{
		hits:     []Hit{{ChunkID: "c1", Score: 1}},
		captureK: &capturedK,
	}

	cfg := DefaultHybridConfig()
	cfg.CandidatePoolMultiplier = 2

	runner := NewHybridRunner(
		bm25Reader,
		&mockHybridVectorReader{hits: []Hit{{ChunkID: "c1", Score: 0.1}}},
		&mockHybridChunkLookup{chunks: chunks},
		&mockEmbeddingLookup{},
		fakeEmbedFunc,
		"model",
		cfg,
	)

	limit := 20
	_, _ = runner.RunHybrid(context.Background(), "query", qmd.SearchOpts{Limit: limit})
	assert.Equal(t, 40, capturedK, "k' must be 20*2=40 when above floor")
}

func TestRunHybrid_noResults(t *testing.T) {
	runner := buildHybridRunner(nil, nil, nil, nil, DefaultHybridConfig())
	results, err := runner.RunHybrid(context.Background(), "query", qmd.SearchOpts{Limit: 5})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestRunHybrid_defaultLimit(t *testing.T) {
	// Limit=0 must use defaultLimit (10).
	var capturedK int
	chunks := make([]qmd.Chunk, 5)
	hits := make([]Hit, 5)
	for i := range 5 {
		id := "c" + string(rune('0'+i))
		chunks[i] = qmd.Chunk{ID: id, Content: "test", CreatedAt: time.Now()}
		hits[i] = Hit{ChunkID: id, Score: float64(i)}
	}

	bm25Reader := &mockHybridBM25Reader{hits: hits, captureK: &capturedK}
	runner := NewHybridRunner(
		bm25Reader,
		&mockHybridVectorReader{hits: hits},
		&mockHybridChunkLookup{chunks: chunks},
		&mockEmbeddingLookup{},
		fakeEmbedFunc,
		"model",
		DefaultHybridConfig(),
	)

	_, err := runner.RunHybrid(context.Background(), "query", qmd.SearchOpts{Limit: 0})
	require.NoError(t, err)
	// kPrime = max(10*2, 20) = 20
	assert.Equal(t, 20, capturedK)
}

func TestRunHybrid_snippetsPopulated(t *testing.T) {
	chunks := []qmd.Chunk{
		{ID: "c1", Content: "The quick brown fox jumps over the lazy dog", CreatedAt: time.Now()},
	}
	bm25Hits := []Hit{{ChunkID: "c1", Score: 5}}
	vecHits := []Hit{{ChunkID: "c1", Score: 0.2}}

	runner := buildHybridRunner(bm25Hits, vecHits, chunks, nil, DefaultHybridConfig())
	results, err := runner.RunHybrid(context.Background(), "fox", qmd.SearchOpts{Limit: 5})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.NotEmpty(t, results[0].Snippet)
}

func TestRunHybrid_decayDisabledWhenGammaZero(t *testing.T) {
	// γ=0 (default) → very old chunks must not be penalised.
	now := time.Now()
	chunks := []qmd.Chunk{
		// 10-year-old chunk should NOT be penalised when γ=0.
		{ID: "c1", Content: "ancient content", CreatedAt: now.Add(-10 * 365 * 24 * time.Hour)},
		{ID: "c2", Content: "new content", CreatedAt: now},
	}
	bm25Hits := []Hit{
		{ChunkID: "c1", Score: 10}, // higher BM25 but older
		{ChunkID: "c2", Score: 1},
	}
	vecHits := []Hit{
		{ChunkID: "c1", Score: 0.1}, // cosine = 0.9
		{ChunkID: "c2", Score: 0.5}, // cosine = 0.5
	}

	cfg := DefaultHybridConfig()
	// α=0, β=1, γ=0: purely BM25-based.
	cfg.Alpha = 0.0
	cfg.Beta = 1.0
	cfg.Gamma = 0.0

	runner := buildHybridRunner(bm25Hits, vecHits, chunks, nil, cfg)
	results, err := runner.RunHybrid(context.Background(), "content", qmd.SearchOpts{Limit: 2})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// c1 has highest BM25 and γ=0 means no decay penalty.
	assert.Equal(t, "c1", results[0].Chunk.ID, "no decay penalty with γ=0")
}

// TestRunHybrid_cosineSimilarityF32 verifies the helper directly.
func TestCosineSimilarityF32(t *testing.T) {
	t.Run("identical vectors", func(t *testing.T) {
		v := []float32{1, 0, 0}
		got := cosineSimilarityF32(v, v)
		assert.InDelta(t, 1.0, got, 1e-9)
	})

	t.Run("orthogonal vectors", func(t *testing.T) {
		a := []float32{1, 0, 0}
		b := []float32{0, 1, 0}
		got := cosineSimilarityF32(a, b)
		assert.InDelta(t, 0.0, got, 1e-9)
	})

	t.Run("opposite vectors", func(t *testing.T) {
		a := []float32{1, 0, 0}
		b := []float32{-1, 0, 0}
		got := cosineSimilarityF32(a, b)
		assert.InDelta(t, -1.0, got, 1e-9)
	})

	t.Run("length mismatch returns zero", func(t *testing.T) {
		a := []float32{1, 2, 3}
		b := []float32{1, 2}
		got := cosineSimilarityF32(a, b)
		assert.InDelta(t, 0.0, got, 1e-9)
	})

	t.Run("zero vector returns zero", func(t *testing.T) {
		a := []float32{0, 0, 0}
		b := []float32{1, 2, 3}
		got := cosineSimilarityF32(a, b)
		assert.InDelta(t, 0.0, got, 1e-9)
	})

	t.Run("known angle 45 degrees", func(t *testing.T) {
		a := []float32{1, 0}
		b := []float32{1, 1}
		got := cosineSimilarityF32(a, b)
		expected := 1.0 / math.Sqrt(2)
		assert.InDelta(t, expected, got, 1e-6)
	})
}
