// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package memory

// This file defines the QMD (Quantized Markdown + Database) public API
// interfaces for MINK's lifelong memory subsystem.
//
// These interfaces are intentionally minimal for M1.  M3 extends Searcher
// with vector and hybrid modes; M5 adds reindex, export, and import.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 (M1)
// REQ:  REQ-MEM-001..006, REQ-MEM-012, REQ-MEM-014, REQ-MEM-025

import (
	"context"

	"github.com/modu-ai/mink/internal/memory/qmd"
)

// QMDIndexer accepts markdown content and persists chunks into the SQLite
// index.  Implementations must serialize writes via a single-writer mutex
// (REQ-MEM-025).
//
// @MX:ANCHOR: [AUTO] Public API boundary — multiple callers expected (cli/add, hook/publish, session export).
// @MX:REASON: fan_in >= 3 once M3+M5 hooks are wired; invariant must not change.
type QMDIndexer interface {
	// Ingest reads a markdown source file, chunks it, and inserts all chunks
	// into the SQLite index.  The file is also hardlinked (or copied) into the
	// vault under collection.
	Ingest(ctx context.Context, collection, sourcePath string) error

	// Insert persists a single pre-built chunk.  Used by tests and the reindex
	// worker (M5) which operates at the chunk level.
	Insert(ctx context.Context, chunk qmd.Chunk) error
}

// QMDSearcher returns ranked chunks matching a query string.
// The Searcher interface is a placeholder for M1; the full retrieval pipeline
// (BM25, vsearch, hybrid) is added in M2–M4.
type QMDSearcher interface {
	// Search returns up to opts.Limit results matching q.  The retrieval mode
	// is selected by opts.Mode.
	Search(ctx context.Context, q string, opts qmd.SearchOpts) ([]qmd.Result, error)
}
