// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package retrieval implements the search dispatcher layer for MINK's QMD
// memory subsystem.
//
// The package deliberately does NOT import internal/memory/sqlite to avoid
// the import cycle:
//
//	sqlite → qmd (ok)
//	retrieval → qmd (ok)
//	retrieval → sqlite (forbidden: sqlite already pulls qmd; cycle through types)
//
// Instead, retrieval defines minimal interfaces (BM25Reader, ChunkLookup) that
// the sqlite package satisfies externally through adapter types.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T2.2
// REQ:  REQ-MEM-015, REQ-MEM-016, REQ-MEM-017, REQ-MEM-018
package retrieval

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/modu-ai/mink/internal/memory/qmd"
)

// ErrEmptyQuery is returned when the query string is empty or whitespace-only.
var ErrEmptyQuery = errors.New("retrieval: query must not be empty")

// ErrModeNotImplementedM2 is returned when opts.Mode is "vsearch" or "query"
// which are not yet wired in M2. These modes will be implemented in M3/M4.
var ErrModeNotImplementedM2 = errors.New("retrieval: vsearch and query modes are not implemented in M2 (coming M3/M4)")

// defaultLimit is used when SearchOpts.Limit is 0.
const defaultLimit = 10

// snippetMaxRunes is the maximum rune length of a generated snippet.
const snippetMaxRunes = 256

// Hit mirrors the sqlite.Hit structure without importing the sqlite package,
// maintaining the anti-cycle constraint.
type Hit struct {
	ChunkID string
	Score   float64
}

// BM25Reader is the interface that sqlite.Reader satisfies.
// Defined here so retrieval does not import sqlite directly.
type BM25Reader interface {
	SearchBM25(ctx context.Context, q, collection string, k int) ([]Hit, error)
}

// ChunkLookup is the interface that sqlite.ChunkLookupStore satisfies.
// Defined here so retrieval does not import sqlite directly.
type ChunkLookup interface {
	LookupChunks(ctx context.Context, chunkIDs []string) ([]qmd.Chunk, error)
}

// BM25Runner dispatches BM25 full-text search queries and hydrates results
// with full Chunk metadata.
//
// @MX:ANCHOR: [AUTO] Central retrieval dispatcher; invoked by CLI search and
// the QMDSearcher interface implementation.
// @MX:REASON: fan_in >= 3 (cli/search.go, integration tests, future gRPC handler).
// Contract: RunBM25 must not modify the database.
type BM25Runner struct {
	reader BM25Reader
	lookup ChunkLookup
}

// NewBM25Runner constructs a BM25Runner with the given reader and lookup
// adapters.
func NewBM25Runner(r BM25Reader, lookup ChunkLookup) *BM25Runner {
	return &BM25Runner{reader: r, lookup: lookup}
}

// RunBM25 executes a BM25 search and returns hydrated, ranked results.
//
// Validation:
//   - opts.Limit defaults to 10 when zero.
//   - Empty or whitespace-only q returns ErrEmptyQuery.
//   - opts.Mode "vsearch" or "query" returns ErrModeNotImplementedM2.
//
// Result ordering matches the BM25 rank order returned by the reader (highest
// score first after normalisation).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T2.2
// REQ:  REQ-MEM-015, REQ-MEM-016
func (r *BM25Runner) RunBM25(ctx context.Context, q string, opts qmd.SearchOpts) ([]qmd.Result, error) {
	// Validate mode.
	if opts.Mode == "vsearch" || opts.Mode == "query" {
		return nil, ErrModeNotImplementedM2
	}

	// Validate query.
	if strings.TrimSpace(q) == "" {
		return nil, ErrEmptyQuery
	}

	// Apply default limit.
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	// Execute BM25 search.
	hits, err := r.reader.SearchBM25(ctx, q, opts.Collection, limit)
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

	// Build results in hit order (already BM25-ranked).
	results := make([]qmd.Result, 0, len(hits))
	for _, h := range hits {
		ch, ok := chunkByID[h.ChunkID]
		if !ok {
			// Chunk was deleted between search and hydration — skip it.
			continue
		}
		results = append(results, qmd.Result{
			Chunk:   ch,
			Score:   h.Score,
			Snippet: MakeSnippet(ch.Content, q),
		})
	}

	return results, nil
}

// MakeSnippet produces a ≤256-rune excerpt from content centred on the first
// case-insensitive match of any whitespace-separated token in query.
//
// Rules:
//   - If no token matches, returns the leading 256 runes of content (or all of
//     content when shorter than 256 runes).
//   - If the content fits entirely within 256 runes, it is returned as-is.
//   - The window is centred on the match start; "…" is prepended/appended when
//     the window is truncated.
//   - Rune counting is used throughout so Korean and other multi-byte text is
//     handled safely.
//
// @MX:NOTE: [AUTO] Snippet generation algorithm: centre-window on first query
// token match, rune-safe, 256-rune hard cap with ellipsis markers.
func MakeSnippet(content, query string) string {
	runes := []rune(content)
	total := len(runes)

	if total <= snippetMaxRunes {
		return content
	}

	// Find the first match position of any token in the query.
	matchStart := -1
	lower := strings.ToLower(content)
	for token := range strings.FieldsSeq(query) {
		tok := strings.ToLower(token)
		if idx := strings.Index(lower, tok); idx >= 0 {
			// Convert byte offset to rune offset.
			matchStart = utf8.RuneCountInString(content[:idx])
			break
		}
	}

	if matchStart < 0 {
		// No match — return leading 256 runes.
		return string(runes[:snippetMaxRunes]) + "…"
	}

	// Centre the window on matchStart.
	half := snippetMaxRunes / 2
	start := matchStart - half
	end := start + snippetMaxRunes

	prefix := "…"
	suffix := "…"

	if start <= 0 {
		start = 0
		prefix = ""
	}
	if end >= total {
		end = total
		suffix = ""
	}
	// Recalculate start if end was clamped.
	if end-start < snippetMaxRunes && start > 0 {
		start = end - snippetMaxRunes
		if start < 0 {
			start = 0
			prefix = ""
		}
	}

	return prefix + string(runes[start:end]) + suffix
}
