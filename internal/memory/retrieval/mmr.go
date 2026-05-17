// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"math"
	"strings"

	"github.com/modu-ai/mink/internal/memory/qmd"
)

// MMRConfig tunes the MMR diversity re-ranker.
type MMRConfig struct {
	// Lambda is the relevance weight.  Default 0.7 (1.0 = pure relevance).
	// Bounded to [0, 1].
	Lambda float64
}

// MMRRerank returns the top-k diversified candidates using Maximal Marginal
// Relevance (Carbonell & Goldstein 1998).
//
// Algorithm:
//
//	Start S = {}; for k iterations:
//	  pick c* = argmax_c [ λ·rel(c) - (1-λ)·max_{s∈S} sim(c, s) ]
//
// where rel(c) is the hybrid score from RunHybrid and sim(c, s) uses stored
// chunk embeddings.  For chunks without embeddings, Jaccard similarity over
// whitespace-tokenised, lowercased content is used as a coarse proxy.
//
// @MX:ANCHOR: [AUTO] MMR diversity re-ranker — fan_in >= 3
// (cli/search query mode, integration tests, future gRPC).
// @MX:REASON: AC-MEM-010 top-10 same-source-path ratio ≤ 30% invariant.
// Changing the selection criterion (argmax formula) requires a SPEC amendment.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T4.3
// REQ:  REQ-MEM-010 (AC-MEM-010 MMR diversity)
func MMRRerank(candidates []qmd.Result, embeddings map[string][]float32, cfg MMRConfig, k int) []qmd.Result {
	if len(candidates) == 0 {
		return candidates
	}
	if k >= len(candidates) {
		return candidates
	}
	if k <= 0 {
		return []qmd.Result{}
	}

	// Clamp lambda to [0, 1].
	lambda := cfg.Lambda
	if lambda < 0 {
		lambda = 0
	}
	if lambda > 1 {
		lambda = 1
	}

	// Build Jaccard token sets for fallback similarity.
	tokenSets := make([]map[string]bool, len(candidates))
	for i, c := range candidates {
		tokenSets[i] = tokenSet(c.Chunk.Content)
	}

	// selected tracks which candidate indices have been chosen.
	selected := make([]int, 0, k)
	// remaining holds the indices not yet selected.
	remaining := make([]int, len(candidates))
	for i := range candidates {
		remaining[i] = i
	}

	for len(selected) < k && len(remaining) > 0 {
		bestIdx := -1
		bestMMR := math.Inf(-1)

		for _, ri := range remaining {
			rel := candidates[ri].Score

			// Compute the maximum similarity to any already-selected candidate.
			maxSim := 0.0
			for _, si := range selected {
				sim := candidateSimilarity(ri, si, candidates, embeddings, tokenSets)
				if sim > maxSim {
					maxSim = sim
				}
			}

			mmr := lambda*rel - (1-lambda)*maxSim
			if mmr > bestMMR {
				bestMMR = mmr
				bestIdx = ri
			}
		}

		if bestIdx < 0 {
			break
		}

		selected = append(selected, bestIdx)

		// Remove bestIdx from remaining.
		newRemaining := remaining[:0]
		for _, ri := range remaining {
			if ri != bestIdx {
				newRemaining = append(newRemaining, ri)
			}
		}
		remaining = newRemaining
	}

	// Build output in selection order.
	out := make([]qmd.Result, len(selected))
	for i, idx := range selected {
		out[i] = candidates[idx]
	}
	return out
}

// candidateSimilarity computes the similarity between candidates[i] and
// candidates[j].  Uses cosine similarity when both have stored embeddings;
// falls back to Jaccard similarity over token sets otherwise.
func candidateSimilarity(
	i, j int,
	candidates []qmd.Result,
	embeddings map[string][]float32,
	tokenSets []map[string]bool,
) float64 {
	idI := candidates[i].Chunk.ID
	idJ := candidates[j].Chunk.ID

	embI, hasI := embeddings[idI]
	embJ, hasJ := embeddings[idJ]

	if hasI && hasJ && len(embI) > 0 && len(embJ) > 0 {
		sim := cosineSimilarityF32(embI, embJ)
		if sim < 0 {
			sim = 0
		}
		return sim
	}

	// Jaccard fallback.
	return jaccardSimilarity(tokenSets[i], tokenSets[j])
}

// tokenSet tokenises content into a set of lowercased whitespace-split tokens.
func tokenSet(content string) map[string]bool {
	set := make(map[string]bool)
	for token := range strings.FieldsSeq(content) {
		set[strings.ToLower(token)] = true
	}
	return set
}

// jaccardSimilarity computes |A ∩ B| / |A ∪ B|.
// Returns 0 when both sets are empty.
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	var intersection int
	for token := range a {
		if b[token] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
