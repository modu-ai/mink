// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrFTS5Unavailable is returned by Reader.SearchBM25 when the chunks_fts
// virtual table was not created (FTS5 module not available in this SQLite
// build).
var ErrFTS5Unavailable = errors.New("sqlite.Reader: FTS5 not enabled in this build")

// Hit is a single BM25 search result returned by Reader.SearchBM25.
// Score is normalised to a higher-is-better scale (raw FTS5 bm25() scores are
// negative; Reader negates them so callers can compare directly).
type Hit struct {
	ChunkID string
	Score   float64 // normalised: higher value = better match
}

// Reader executes read-only queries against a Store.
//
// @MX:ANCHOR: [AUTO] Primary BM25 search entry-point; called by retrieval.BM25Runner.
// @MX:REASON: fan_in >= 3 (BM25Runner, integration tests, CLI search). Invariant:
// SearchBM25 must never mutate the database.
type Reader struct {
	store *Store
}

// NewReader constructs a Reader backed by the given Store.
func NewReader(store *Store) *Reader {
	return &Reader{store: store}
}

// probeChunksFTS checks whether the chunks_fts virtual table exists and is
// functional. Returns ErrFTS5Unavailable if the table is missing or broken.
func (r *Reader) probeChunksFTS(ctx context.Context) error {
	const probe = `SELECT count(*) FROM chunks_fts WHERE chunks_fts MATCH 'a'`
	var n int
	if err := r.store.db.QueryRowContext(ctx, probe).Scan(&n); err != nil {
		return ErrFTS5Unavailable
	}
	return nil
}

// quoteFTS5Query wraps q in double-quotes and escapes any embedded double-quote
// characters to prevent FTS5 operator injection.
//
// FTS5 treats '"' as a phrase-start delimiter.  By wrapping the entire query
// in double-quotes we force phrase matching, which silences operator parsing
// for characters like *, -, OR, etc.  Embedded " must be doubled per FTS5
// grammar.
func quoteFTS5Query(q string) string {
	escaped := strings.ReplaceAll(q, `"`, `""`)
	return `"` + escaped + `"`
}

// SearchBM25 returns up to k chunk hits matching q against the chunks_fts
// full-text index.
//
// When collection is non-empty, results are restricted to chunks whose parent
// file belongs to that collection.
//
// FTS5 bm25() returns negative values (lower = better rank by default).
// SearchBM25 negates the raw score so that callers receive a higher-is-better
// value.
//
// If the chunks_fts virtual table is absent (FTS5 not compiled in), the call
// returns ErrFTS5Unavailable immediately.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T2.1
// REQ:  REQ-MEM-015, REQ-MEM-016, REQ-MEM-017
func (r *Reader) SearchBM25(ctx context.Context, q, collection string, k int) ([]Hit, error) {
	// Probe FTS5 availability before executing the real query.
	if err := r.probeChunksFTS(ctx); err != nil {
		return nil, err
	}

	quotedQ := quoteFTS5Query(q)

	type rowScanner interface {
		Next() bool
		Scan(dest ...any) error
		Close() error
	}

	var (
		rows rowScanner
		err  error
	)

	if collection == "" {
		const sql = `
SELECT cf.chunk_id, bm25(chunks_fts)
FROM chunks_fts cf
WHERE cf MATCH ?
ORDER BY bm25(chunks_fts)
LIMIT ?`
		rows, err = r.store.db.QueryContext(ctx, sql, quotedQ, k)
	} else {
		// Join with chunks and files to filter by collection.
		const sql = `
SELECT cf.chunk_id, bm25(chunks_fts)
FROM chunks_fts cf
JOIN chunks c  ON c.chunk_id = cf.chunk_id
JOIN files  f  ON f.file_id  = c.file_id
WHERE cf MATCH ?
  AND f.collection = ?
ORDER BY bm25(chunks_fts)
LIMIT ?`
		rows, err = r.store.db.QueryContext(ctx, sql, quotedQ, collection, k)
	}

	if err != nil {
		return nil, fmt.Errorf("sqlite.Reader.SearchBM25: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var hits []Hit
	for rows.Next() {
		var h Hit
		var rawScore float64
		if scanErr := rows.Scan(&h.ChunkID, &rawScore); scanErr != nil {
			return nil, fmt.Errorf("sqlite.Reader.SearchBM25: scan: %w", scanErr)
		}
		// Negate: FTS5 bm25() is negative-lower-is-better; we want positive-higher-is-better.
		h.Score = -rawScore
		hits = append(hits, h)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("sqlite.Reader.SearchBM25: close rows: %w", err)
	}

	if hits == nil {
		hits = []Hit{}
	}
	return hits, nil
}
