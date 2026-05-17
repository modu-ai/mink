// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package retrieval

import (
	"context"
	"errors"
	"strings"

	"github.com/modu-ai/mink/internal/memory/qmd"
)

// ErrVec0Unavailable is returned by VectorReader.SearchVector when the
// sqlite-vec extension is not loaded or the embeddings table is not queryable.
var ErrVec0Unavailable = errors.New("retrieval: sqlite-vec (vec0) is not available; vsearch requires the extension")

// VectorReader abstracts the sqlite-vec kNN query from the retrieval layer.
// Implemented by sqlite.VectorReaderAdapter.
type VectorReader interface {
	// SearchVector queries the embeddings virtual table for the top-k chunks
	// whose embedding is closest to queryEmbed (cosine distance, lower = closer).
	// When vec0 is unavailable the implementation returns ErrVec0Unavailable.
	SearchVector(ctx context.Context, queryEmbed []float32, collection string, k int) ([]Hit, error)
}

// EmbedFunc abstracts the ollama client's Embed method for testability.
type EmbedFunc func(ctx context.Context, model, text string) ([]float32, error)

// VectorRunner dispatches vector kNN search queries and hydrates results with
// full Chunk metadata.
//
// @MX:ANCHOR: [AUTO] Central vsearch dispatcher; invoked by CLI vsearch path.
// @MX:REASON: fan_in >= 3 (cli/search.go, integration tests, future gRPC handler).
// Contract: RunVector must not modify the database.
type VectorRunner struct {
	reader VectorReader
	lookup ChunkLookup
	embed  EmbedFunc
	model  string // ollama embedding model, e.g. "mxbai-embed-large"
}

// NewVectorRunner constructs a VectorRunner with the given dependencies.
func NewVectorRunner(r VectorReader, lookup ChunkLookup, embed EmbedFunc, model string) *VectorRunner {
	return &VectorRunner{
		reader: r,
		lookup: lookup,
		embed:  embed,
		model:  model,
	}
}

// RunVector embeds q via the embed function, then performs vector kNN search.
//
// On embed failure the error is returned unchanged so the caller can inspect
// it with ollama.ShouldFallbackToBM25 to decide whether to fall back.
//
// Score convention: the underlying vec0 engine returns cosine distance (lower =
// closer).  RunVector converts to similarity via 1.0 - distance so that a
// higher score always means a better match (consistent with BM25 convention).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T3.4
// REQ:  REQ-MEM-019, REQ-MEM-020
func (r *VectorRunner) RunVector(ctx context.Context, q string, opts qmd.SearchOpts) ([]qmd.Result, error) {
	// Validate query.
	if strings.TrimSpace(q) == "" {
		return nil, ErrEmptyQuery
	}

	// Apply default limit.
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	// Embed the query.
	queryEmbed, err := r.embed(ctx, r.model, q)
	if err != nil {
		return nil, err // caller decides whether to fall back
	}

	// Execute vector kNN search.
	hits, err := r.reader.SearchVector(ctx, queryEmbed, opts.Collection, limit)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return []qmd.Result{}, nil
	}

	// Collect chunk IDs for bulk hydration.
	chunkIDs := make([]string, len(hits))
	for i, h := range hits {
		chunkIDs[i] = h.ChunkID
	}

	// Hydrate chunks.
	chunks, err := r.lookup.LookupChunks(ctx, chunkIDs)
	if err != nil {
		return nil, err
	}

	// Build a chunk index by ID for O(1) lookup.
	chunkByID := make(map[string]qmd.Chunk, len(chunks))
	for _, ch := range chunks {
		chunkByID[ch.ID] = ch
	}

	// Build a score index by chunk ID.
	scoreByID := make(map[string]float64, len(hits))
	for _, h := range hits {
		// Convert cosine distance to similarity score: higher = better match.
		scoreByID[h.ChunkID] = 1.0 - h.Score
	}

	// Build results in hit order (already distance-ranked).
	results := make([]qmd.Result, 0, len(hits))
	for _, h := range hits {
		ch, ok := chunkByID[h.ChunkID]
		if !ok {
			// Chunk was deleted between search and hydration — skip it.
			continue
		}
		results = append(results, qmd.Result{
			Chunk:   ch,
			Score:   scoreByID[h.ChunkID],
			Snippet: MakeSnippet(ch.Content, q),
		})
	}

	return results, nil
}
