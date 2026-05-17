// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package qmd provides QMD (Quantized Markdown + Database) chunking and
// indexing primitives for MINK's lifelong memory subsystem.
//
// The public types defined here are shared by the sqlite and cli sub-packages.
package qmd

import "time"

// DefaultModelVersion is the model version tag used for M1 chunks where no
// embedding model is active.  M3 will replace this with the actual ollama
// model name once embedding is wired in.
const DefaultModelVersion = "v1-no-embed"

// Chunk is a single addressable unit of a markdown source file.
// It maps 1:1 to a row in the SQLite chunks table.
type Chunk struct {
	// ID is the stable chunk identifier derived from source coordinates and
	// content hash.  See ChunkID for the derivation algorithm.
	ID string

	// FileID is the FK reference to the files table row for the source file.
	// Populated by the caller after UpsertFile succeeds.
	FileID int64

	// Collection is the logical collection (e.g. "journal", "sessions").
	Collection string

	// SourcePath is the vault-relative or absolute path of the source markdown
	// file.
	SourcePath string

	// StartLine is the 1-based line number of the first line of the chunk.
	StartLine int

	// EndLine is the 1-based line number of the last line of the chunk.
	EndLine int

	// Content is the raw text of the chunk.
	Content string

	// PrevChunkID is the ID of the preceding chunk in the same source file, or
	// empty for the first chunk.
	PrevChunkID string

	// NextChunkID is the ID of the following chunk in the same source file, or
	// empty for the last chunk.
	NextChunkID string

	// EmbeddingPending is true when the chunk has not yet been embedded by an
	// ollama model.  Set to false once the embedding is stored in the vec0
	// virtual table.
	EmbeddingPending bool

	// ModelVersion identifies the embedding model that was used to produce the
	// chunk_id hash.  Changing the model version marks all previously derived
	// chunk IDs as stale.
	ModelVersion string

	// CreatedAt is the Unix timestamp (seconds) when the chunk was inserted.
	CreatedAt time.Time
}

// File represents a source markdown file tracked in the files table.
type File struct {
	// FileID is set by the store after AUTOINCREMENT insert.
	FileID int64

	// Collection is the vault collection name (e.g. "journal").
	Collection string

	// SourcePath is the absolute path of the markdown file inside the vault.
	SourcePath string

	// ContentHash is the hex-encoded SHA-256 of the file content at ingest time.
	ContentHash string

	// CreatedAt is the Unix timestamp of first insertion.
	CreatedAt time.Time

	// UpdatedAt is the Unix timestamp of the last update.
	UpdatedAt time.Time
}

// SearchOpts configures a Searcher.Search call.  Most fields are reserved for
// M2+ and are included here so the interface is stable.
type SearchOpts struct {
	// Collection, when non-empty, restricts search to a single collection.
	Collection string

	// Limit caps the number of results returned.  Default: 10.
	Limit int

	// Mode selects the retrieval path: "search" (BM25), "vsearch" (vector),
	// or "query" (hybrid).  Defaults to "search" for M1.
	Mode string
}

// Result is a single search match returned by Searcher.Search.
type Result struct {
	// Chunk is the matched chunk with all metadata filled in.
	Chunk Chunk

	// Score is the retrieval score (BM25, cosine, or hybrid).
	Score float64

	// Snippet is a ≤256-character excerpt of the chunk content centered on
	// the query match.
	Snippet string
}
