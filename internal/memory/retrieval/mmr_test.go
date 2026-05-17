// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"fmt"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orthogonalEmbedding returns a unit vector with a 1 in position i.
func orthogonalEmbedding(size, pos int) []float32 {
	v := make([]float32, size)
	v[pos%size] = 1.0
	return v
}

// nearIdenticalEmbedding returns a vector almost identical to a given one
// (only the first component slightly different).
func nearIdenticalEmbedding(size int, val float32) []float32 {
	v := make([]float32, size)
	for i := range v {
		v[i] = val + float32(i)*0.001
	}
	return v
}

// TestMMRRerank_diverseResults verifies MMR picks diverse items, not near-duplicates.
func TestMMRRerank_diverseResults(t *testing.T) {
	// 4 candidates: c1/c2 are near-identical (high cosine), c3/c4 are orthogonal.
	// Without MMR the top-2 by score would be c1 and c2 (both score=1.0).
	// With MMR (λ=0.7), c3 should be selected over c2 because selecting c1
	// first penalises near-duplicates.

	const dim = 4
	emb1 := []float32{1, 0, 0, 0}
	emb2 := []float32{0.99, 0.01, 0.01, 0.01} // near-duplicate of c1
	emb3 := []float32{0, 1, 0, 0}             // orthogonal to c1
	emb4 := []float32{0, 0, 1, 0}

	_ = dim

	candidates := []qmd.Result{
		{Chunk: qmd.Chunk{ID: "c1", Content: "a b c d", SourcePath: "/p1"}, Score: 1.0},
		{Chunk: qmd.Chunk{ID: "c2", Content: "a b c e", SourcePath: "/p2"}, Score: 0.9},
		{Chunk: qmd.Chunk{ID: "c3", Content: "x y z w", SourcePath: "/p3"}, Score: 0.8},
		{Chunk: qmd.Chunk{ID: "c4", Content: "m n o p", SourcePath: "/p4"}, Score: 0.7},
	}

	embeddings := map[string][]float32{
		"c1": emb1,
		"c2": emb2,
		"c3": emb3,
		"c4": emb4,
	}

	cfg := MMRConfig{Lambda: 0.7}
	result := MMRRerank(candidates, embeddings, cfg, 2)
	require.Len(t, result, 2)

	// c1 is always selected first (highest score).
	assert.Equal(t, "c1", result[0].Chunk.ID, "first selection must be highest-scoring candidate")

	// c3 (orthogonal to c1) should be selected over c2 (near-duplicate of c1).
	ids := []string{result[0].Chunk.ID, result[1].Chunk.ID}
	assert.Contains(t, ids, "c3", "MMR must prefer diverse candidate c3 over near-duplicate c2")
}

// TestMMRRerank_lambdaOne_pureRelevance verifies λ=1 returns top-k by score.
func TestMMRRerank_lambdaOne_pureRelevance(t *testing.T) {
	candidates := []qmd.Result{
		{Chunk: qmd.Chunk{ID: "c1"}, Score: 1.0},
		{Chunk: qmd.Chunk{ID: "c2"}, Score: 0.9},
		{Chunk: qmd.Chunk{ID: "c3"}, Score: 0.8},
		{Chunk: qmd.Chunk{ID: "c4"}, Score: 0.7},
	}

	cfg := MMRConfig{Lambda: 1.0}
	result := MMRRerank(candidates, nil, cfg, 3)
	require.Len(t, result, 3)

	// Pure relevance: order preserved.
	assert.Equal(t, "c1", result[0].Chunk.ID)
	assert.Equal(t, "c2", result[1].Chunk.ID)
	assert.Equal(t, "c3", result[2].Chunk.ID)
}

// TestMMRRerank_lambdaZero_pureDiv verifies λ=0 returns the most diverse subset.
func TestMMRRerank_lambdaZero_pureDiv(t *testing.T) {
	// All embeddings are orthogonal → maximum diversity = any k items.
	// With λ=0, MMR picks based solely on similarity to already-selected items.
	// First pick is always index 0 (argmax of -max_sim when S is empty → ties at 0).
	candidates := []qmd.Result{
		{Chunk: qmd.Chunk{ID: "c1", Content: "alpha beta"}, Score: 0.5},
		{Chunk: qmd.Chunk{ID: "c2", Content: "gamma delta"}, Score: 0.9},
		{Chunk: qmd.Chunk{ID: "c3", Content: "epsilon zeta"}, Score: 0.1},
	}
	embeddings := map[string][]float32{
		"c1": {1, 0, 0},
		"c2": {0, 1, 0},
		"c3": {0, 0, 1},
	}

	cfg := MMRConfig{Lambda: 0.0}
	result := MMRRerank(candidates, embeddings, cfg, 2)
	require.Len(t, result, 2, "λ=0 must still return k results")

	// With λ=0, diversity dominates.  The two items selected should be the
	// most orthogonal pair.  Since all are orthogonal, any 2 are maximally diverse.
	// The key check is that we get 2 distinct results.
	assert.NotEqual(t, result[0].Chunk.ID, result[1].Chunk.ID)
}

// TestMMRRerank_kEqualsLen_noop verifies k >= len(candidates) returns input as-is.
func TestMMRRerank_kEqualsLen_noop(t *testing.T) {
	candidates := []qmd.Result{
		{Chunk: qmd.Chunk{ID: "c1"}, Score: 1.0},
		{Chunk: qmd.Chunk{ID: "c2"}, Score: 0.5},
	}

	cfg := MMRConfig{Lambda: 0.7}
	result := MMRRerank(candidates, nil, cfg, 10)
	require.Len(t, result, 2, "k >= len must return all candidates")
	assert.Equal(t, "c1", result[0].Chunk.ID)
	assert.Equal(t, "c2", result[1].Chunk.ID)
}

// TestMMRRerank_acMem010_sourceDiversityRatio is the AC-MEM-010 acceptance test.
// Build 30+ candidates skewed toward 3 source paths; verify top-10 ratio ≤ 30%.
func TestMMRRerank_acMem010_sourceDiversityRatio(t *testing.T) {
	// 3 "dominant" paths × 10 candidates each = 30; 4 "other" paths × 1 = 4.
	dominantPaths := []string{"/hot/path/a.md", "/hot/path/b.md", "/hot/path/c.md"}
	var candidates []qmd.Result
	embeddings := make(map[string][]float32)

	// 30 candidates from 3 dominant paths — all with high relevance scores.
	for pi, path := range dominantPaths {
		for j := range 10 {
			id := fmt.Sprintf("dom-%d-%d", pi, j)
			// Near-identical embeddings within a path group (high intra-group cosine).
			emb := nearIdenticalEmbedding(32, float32(pi+1)*0.1)
			// Slight per-item variation so they're not perfectly identical.
			emb[j%32] += float32(j) * 0.001
			embeddings[id] = emb
			candidates = append(candidates, qmd.Result{
				Chunk: qmd.Chunk{
					ID:         id,
					SourcePath: path,
					Content:    fmt.Sprintf("dominant content %d %d", pi, j),
					CreatedAt:  time.Now(),
				},
				Score: 0.8 + float64(j)*0.001, // high but similar scores
			})
		}
	}

	// 4 diverse candidates from unique paths with orthogonal embeddings.
	diversePaths := []string{"/other/x.md", "/other/y.md", "/other/z.md", "/other/w.md"}
	for di, path := range diversePaths {
		id := fmt.Sprintf("div-%d", di)
		emb := orthogonalEmbedding(32, di+3) // orthogonal to dominant groups
		embeddings[id] = emb
		candidates = append(candidates, qmd.Result{
			Chunk: qmd.Chunk{
				ID:         id,
				SourcePath: path,
				Content:    fmt.Sprintf("diverse content %d", di),
				CreatedAt:  time.Now(),
			},
			Score: 0.75, // slightly lower than dominant
		})
	}

	require.Len(t, candidates, 34, "expected 34 candidates")

	cfg := MMRConfig{Lambda: 0.7}
	k := 10
	result := MMRRerank(candidates, embeddings, cfg, k)
	require.Len(t, result, k)

	// Count max occurrences of any single source path.
	pathCount := make(map[string]int)
	for _, r := range result {
		pathCount[r.Chunk.SourcePath]++
	}

	for path, count := range pathCount {
		ratio := float64(count) / float64(k)
		assert.LessOrEqual(t, ratio, 0.30+1e-9,
			"AC-MEM-010: path %q has ratio %.2f > 0.30 in top-%d", path, ratio, k)
	}
}

// TestMMRRerank_noEmbeddingFallback verifies Jaccard fallback when embeddings absent.
func TestMMRRerank_noEmbeddingFallback(t *testing.T) {
	// Candidates without embeddings; MMR uses Jaccard.
	candidates := []qmd.Result{
		{Chunk: qmd.Chunk{ID: "c1", Content: "golang concurrency goroutines"}, Score: 1.0},
		{Chunk: qmd.Chunk{ID: "c2", Content: "golang concurrency channels"}, Score: 0.9},
		{Chunk: qmd.Chunk{ID: "c3", Content: "python machine learning"}, Score: 0.8},
	}

	// No embeddings provided.
	cfg := MMRConfig{Lambda: 0.7}
	result := MMRRerank(candidates, nil, cfg, 2)
	require.Len(t, result, 2)

	// c1 is selected first.  c3 has low Jaccard similarity to c1 (different tokens),
	// so it should be preferred over c2 (shares "golang concurrency" tokens with c1).
	assert.Equal(t, "c1", result[0].Chunk.ID)
	assert.Equal(t, "c3", result[1].Chunk.ID,
		"Jaccard fallback: c3 (python ml) more diverse than c2 (golang concurrency channels)")
}

// TestMMRRerank_empty verifies empty input returns empty output.
func TestMMRRerank_empty(t *testing.T) {
	result := MMRRerank(nil, nil, MMRConfig{Lambda: 0.7}, 5)
	assert.Empty(t, result)
}

// TestMMRRerank_kZero returns empty slice.
func TestMMRRerank_kZero(t *testing.T) {
	candidates := []qmd.Result{
		{Chunk: qmd.Chunk{ID: "c1"}, Score: 1.0},
	}
	result := MMRRerank(candidates, nil, MMRConfig{Lambda: 0.7}, 0)
	assert.Empty(t, result)
}

// TestJaccardSimilarity verifies the Jaccard helper.
func TestJaccardSimilarity(t *testing.T) {
	t.Run("identical sets", func(t *testing.T) {
		a := tokenSet("hello world")
		b := tokenSet("hello world")
		assert.InDelta(t, 1.0, jaccardSimilarity(a, b), 1e-9)
	})

	t.Run("disjoint sets", func(t *testing.T) {
		a := tokenSet("alpha beta")
		b := tokenSet("gamma delta")
		assert.InDelta(t, 0.0, jaccardSimilarity(a, b), 1e-9)
	})

	t.Run("50% overlap", func(t *testing.T) {
		a := tokenSet("a b")
		b := tokenSet("a c")
		// intersection={a}=1, union={a,b,c}=3 → 1/3 ≈ 0.333
		got := jaccardSimilarity(a, b)
		assert.InDelta(t, 1.0/3.0, got, 1e-9)
	})

	t.Run("both empty returns zero", func(t *testing.T) {
		assert.InDelta(t, 0.0, jaccardSimilarity(nil, nil), 1e-9)
	})
}
