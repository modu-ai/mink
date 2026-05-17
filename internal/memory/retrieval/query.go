// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package retrieval — M4 hybrid query runner.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T4.2
// REQ:  REQ-MEM-008 (AC-MEM-008 default mode), REQ-MEM-009 (AC-MEM-009 additive score)
package retrieval

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modu-ai/mink/internal/memory/ollama"
	"github.com/modu-ai/mink/internal/memory/qmd"
)

// ErrFellBackToBM25 is returned wrapped alongside BM25-only results when the
// hybrid runner could not obtain a query embedding (e.g. ollama unavailable).
// Callers check errors.Is(err, ErrFellBackToBM25) to emit a warning to the user
// but still treat the call as successful (exit code 0).
var ErrFellBackToBM25 = errors.New("retrieval: ollama unavailable; fell back to BM25-only")

// EmbeddingLookup fetches stored embeddings for a set of chunk IDs.
// Returns a map keyed by chunk_id; missing chunks are silently omitted.
// Implemented by sqlite.EmbeddingLookupStore.
type EmbeddingLookup interface {
	LookupEmbeddings(ctx context.Context, chunkIDs []string) (map[string][]float32, error)
}

// HybridConfig tunes the additive hybrid score:
//
//	score = α·cosine + β·bm25_norm + γ·decay
type HybridConfig struct {
	// Alpha is the weight of the cosine similarity component. Default: 0.7.
	Alpha float64

	// Beta is the weight of the normalised BM25 score. Default: 0.3.
	Beta float64

	// Gamma is the weight of the temporal decay factor. Default: 0.0 (disabled).
	Gamma float64

	// DecayHalfLife is the half-life used by DecayFactor when Gamma > 0.
	// Default: DefaultDecayHalfLife (30 days).
	DecayHalfLife time.Duration

	// CandidatePoolMultiplier determines the over-fetch factor.
	// K' = max(Limit * CandidatePoolMultiplier, 20). Default: 2.
	CandidatePoolMultiplier int
}

// DefaultHybridConfig returns the default HybridConfig (α=0.7, β=0.3, γ=0.0).
func DefaultHybridConfig() HybridConfig {
	return HybridConfig{
		Alpha:                   0.7,
		Beta:                    0.3,
		Gamma:                   0.0,
		DecayHalfLife:           DefaultDecayHalfLife,
		CandidatePoolMultiplier: 2,
	}
}

// HybridRunner combines BM25 + vector + decay into a single additive score.
//
// @MX:ANCHOR: [AUTO] Hybrid retrieval entry point — fan_in >= 3
// (cli/search query mode, future gRPC, integration tests).
// @MX:REASON: Default mode for AC-MEM-008; must remain a stable contract.
// Changing the scoring formula or concurrency model requires a SPEC amendment.
type HybridRunner struct {
	bm25Reader  BM25Reader
	vecReader   VectorReader
	lookup      ChunkLookup
	embedLookup EmbeddingLookup
	embed       EmbedFunc
	model       string
	cfg         HybridConfig
}

// NewHybridRunner constructs a HybridRunner with the given dependencies.
func NewHybridRunner(
	bm25 BM25Reader,
	vec VectorReader,
	lookup ChunkLookup,
	embedLookup EmbeddingLookup,
	embed EmbedFunc,
	model string,
	cfg HybridConfig,
) *HybridRunner {
	return &HybridRunner{
		bm25Reader:  bm25,
		vecReader:   vec,
		lookup:      lookup,
		embedLookup: embedLookup,
		embed:       embed,
		model:       model,
		cfg:         cfg,
	}
}

// candidateScore accumulates per-candidate raw scores before normalisation.
type candidateScore struct {
	chunkID   string
	bm25Raw   float64
	cosine    float64
	hasCosine bool
}

// RunHybrid executes the hybrid query.
//
//  1. Embed q via embed function (5s timeout).  On ShouldFallbackToBM25 error
//     → return BM25-only path with ErrFellBackToBM25 wrapped.
//  2. Concurrently call bm25Reader.SearchBM25 and vectorReader.SearchVector,
//     each with K' = max(Limit*CandidatePoolMultiplier, 20).
//  3. Union candidate chunk_ids; dedup.
//  4. Hydrate chunks via lookup.LookupChunks.
//  5. Hydrate embeddings via embedLookup.LookupEmbeddings.
//  6. For each candidate compute additive hybrid score.
//  7. Rank by score desc, take top Limit.
//  8. Build []qmd.Result with Snippet via MakeSnippet.
//
// @MX:WARN: [AUTO] Concurrent BM25 + vector search goroutines with shared result
// collection. Both goroutines write to dedicated channels; the main goroutine
// reads from both sequentially — no shared mutable state.
// @MX:REASON: Parallel fan-out to two independent readers is required for
// latency (AC-MEM latency target).  Any change to the concurrency pattern must
// preserve the anti-data-race invariant (go test -race must pass).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T4.2
// REQ:  REQ-MEM-008, REQ-MEM-009
func (r *HybridRunner) RunHybrid(ctx context.Context, q string, opts qmd.SearchOpts) ([]qmd.Result, error) {
	// Validate query.
	if strings.TrimSpace(q) == "" {
		return nil, ErrEmptyQuery
	}

	// Apply default limit.
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	multiplier := r.cfg.CandidatePoolMultiplier
	if multiplier <= 0 {
		multiplier = 2
	}
	kPrime := max(limit*multiplier, 20)

	// --- Step 1: Embed query (5-second timeout). ---
	embedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	queryEmbed, embedErr := r.embed(embedCtx, r.model, q)
	if embedErr != nil {
		if ollama.ShouldFallbackToBM25(embedErr) || errors.Is(embedErr, ErrVec0Unavailable) {
			return r.runBM25FallbackWithSentinel(ctx, q, opts, limit)
		}
		return nil, embedErr
	}

	// --- Step 2: Concurrent BM25 + vector search. ---
	type bm25Res struct {
		hits []Hit
		err  error
	}
	type vecRes struct {
		hits []Hit
		err  error
	}

	bm25Ch := make(chan bm25Res, 1)
	vecCh := make(chan vecRes, 1)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		hits, err := r.bm25Reader.SearchBM25(ctx, q, opts.Collection, kPrime)
		bm25Ch <- bm25Res{hits: hits, err: err}
	}()

	go func() {
		defer wg.Done()
		hits, err := r.vecReader.SearchVector(ctx, queryEmbed, opts.Collection, kPrime)
		vecCh <- vecRes{hits: hits, err: err}
	}()

	wg.Wait()

	bm25Result := <-bm25Ch
	vecResult := <-vecCh

	// Handle vector reader unavailability — fall back to BM25 only.
	if vecResult.err != nil {
		if errors.Is(vecResult.err, ErrVec0Unavailable) || ollama.ShouldFallbackToBM25(vecResult.err) {
			return r.runBM25FallbackWithSentinel(ctx, q, opts, limit)
		}
		return nil, vecResult.err
	}
	if bm25Result.err != nil {
		return nil, bm25Result.err
	}

	// --- Step 3: Union + dedup. ---
	scoreMap := make(map[string]*candidateScore)

	for _, h := range bm25Result.hits {
		cs := &candidateScore{chunkID: h.ChunkID, bm25Raw: h.Score}
		scoreMap[h.ChunkID] = cs
	}

	for _, h := range vecResult.hits {
		// vec0 returns cosine distance (lower = closer); convert to similarity.
		cosine := 1.0 - h.Score
		if cosine < 0 {
			cosine = 0
		}
		if cs, ok := scoreMap[h.ChunkID]; ok {
			cs.cosine = cosine
			cs.hasCosine = true
		} else {
			scoreMap[h.ChunkID] = &candidateScore{
				chunkID:   h.ChunkID,
				cosine:    cosine,
				hasCosine: true,
			}
		}
	}

	if len(scoreMap) == 0 {
		return []qmd.Result{}, nil
	}

	// --- Step 4: Hydrate chunks. ---
	chunkIDs := make([]string, 0, len(scoreMap))
	for id := range scoreMap {
		chunkIDs = append(chunkIDs, id)
	}

	chunks, err := r.lookup.LookupChunks(ctx, chunkIDs)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return []qmd.Result{}, nil
	}

	chunkByID := make(map[string]qmd.Chunk, len(chunks))
	for _, ch := range chunks {
		chunkByID[ch.ID] = ch
	}

	// --- Step 5: Hydrate embeddings. ---
	embeddings, err := r.embedLookup.LookupEmbeddings(ctx, chunkIDs)
	if err != nil {
		return nil, err
	}

	// --- Step 6: Compute additive scores. ---
	// Normalise BM25: bm25_norm = raw / max(all bm25_raw).
	var maxBM25 float64
	for _, cs := range scoreMap {
		if cs.bm25Raw > maxBM25 {
			maxBM25 = cs.bm25Raw
		}
	}

	now := time.Now()
	alpha := r.cfg.Alpha
	beta := r.cfg.Beta
	gamma := r.cfg.Gamma
	halfLife := r.cfg.DecayHalfLife
	if halfLife <= 0 {
		halfLife = DefaultDecayHalfLife
	}

	type scoredCandidate struct {
		chunkID string
		score   float64
	}
	var candidates []scoredCandidate

	for _, cs := range scoreMap {
		ch, ok := chunkByID[cs.chunkID]
		if !ok {
			continue // chunk deleted between search and hydration
		}

		bm25Norm := 0.0
		if maxBM25 > 0 {
			bm25Norm = cs.bm25Raw / maxBM25
		}

		cosine := 0.0
		if cs.hasCosine {
			cosine = cs.cosine
		} else if emb, hasEmb := embeddings[cs.chunkID]; hasEmb && len(emb) > 0 {
			// Chunk was in BM25 results only; compute cosine from stored embedding.
			cosine = cosineSimilarityF32(queryEmbed, emb)
		}

		decay := 0.0
		if gamma > 0 {
			decay = DecayFactor(ch.CreatedAt, now, halfLife)
		}

		score := alpha*cosine + beta*bm25Norm + gamma*decay
		candidates = append(candidates, scoredCandidate{chunkID: cs.chunkID, score: score})
	}

	// --- Step 7: Rank desc, take top Limit. ---
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	// --- Step 8: Build results with snippets. ---
	results := make([]qmd.Result, 0, len(candidates))
	for _, c := range candidates {
		ch, ok := chunkByID[c.chunkID]
		if !ok {
			continue
		}
		results = append(results, qmd.Result{
			Chunk:   ch,
			Score:   c.score,
			Snippet: MakeSnippet(ch.Content, q),
		})
	}

	return results, nil
}

// runBM25FallbackWithSentinel executes a BM25-only search and returns the
// results wrapped with ErrFellBackToBM25.  Used when ollama is unreachable or
// the vector extension is unavailable.
func (r *HybridRunner) runBM25FallbackWithSentinel(ctx context.Context, q string, opts qmd.SearchOpts, limit int) ([]qmd.Result, error) {
	hits, err := r.bm25Reader.SearchBM25(ctx, q, opts.Collection, limit)
	if err != nil {
		return nil, err
	}

	if len(hits) == 0 {
		return []qmd.Result{}, ErrFellBackToBM25
	}

	chunkIDs := make([]string, len(hits))
	for i, h := range hits {
		chunkIDs[i] = h.ChunkID
	}

	chunks, err := r.lookup.LookupChunks(ctx, chunkIDs)
	if err != nil {
		return nil, err
	}

	chunkByID := make(map[string]qmd.Chunk, len(chunks))
	for _, ch := range chunks {
		chunkByID[ch.ID] = ch
	}

	// Normalise BM25 scores for consistent output format.
	var maxScore float64
	for _, h := range hits {
		if h.Score > maxScore {
			maxScore = h.Score
		}
	}

	results := make([]qmd.Result, 0, len(hits))
	for _, h := range hits {
		ch, ok := chunkByID[h.ChunkID]
		if !ok {
			continue
		}
		norm := 0.0
		if maxScore > 0 {
			norm = h.Score / maxScore
		}
		results = append(results, qmd.Result{
			Chunk:   ch,
			Score:   norm,
			Snippet: MakeSnippet(ch.Content, q),
		})
	}

	return results, ErrFellBackToBM25
}

// cosineSimilarityF32 computes cosine similarity between two float32 vectors.
// Returns 0 when either vector is zero-length, lengths differ, or both norms are 0.
func cosineSimilarityF32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
