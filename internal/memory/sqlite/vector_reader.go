// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/modu-ai/mink/internal/memory/retrieval"
)

// VectorReaderAdapter wraps a Store and implements retrieval.VectorReader by
// issuing sqlite-vec kNN queries against the embeddings virtual table.
//
// The adapter is created in the cli/search.go vsearch path and passed to
// retrieval.VectorRunner.
//
// @MX:NOTE: [AUTO] Adapter satisfying retrieval.VectorReader without creating
// an import cycle.  The import direction is sqlite → retrieval (OK).
type VectorReaderAdapter struct {
	store *Store
}

// NewVectorReaderAdapter constructs a VectorReaderAdapter around the given Store.
func NewVectorReaderAdapter(store *Store) *VectorReaderAdapter {
	return &VectorReaderAdapter{store: store}
}

// SearchVector queries the embeddings virtual table for the top-k chunks
// closest to queryEmbed using cosine distance.
//
// When the Store does not have vec0 available (Store.HasVec0() == false),
// SearchVector returns retrieval.ErrVec0Unavailable so the caller can fall
// back to BM25.
//
// Binary format: float32 slice packed as little-endian IEEE 754.  This is the
// format expected by the sqlite-vec MATCH operator.
//
// @MX:WARN: [AUTO] Binary float packing — endianness and length invariants.
// @MX:REASON: sqlite-vec requires a specific wire format (32-bit little-endian
// IEEE 754 floats, no padding).  Incorrect packing causes silent wrong results
// rather than errors from the virtual table engine.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T3.4
// REQ:  REQ-MEM-019, REQ-MEM-020
func (a *VectorReaderAdapter) SearchVector(
	ctx context.Context,
	queryEmbed []float32,
	collection string,
	k int,
) ([]retrieval.Hit, error) {
	if !a.store.HasVec0() {
		return nil, retrieval.ErrVec0Unavailable
	}

	blob := packFloat32LE(queryEmbed)

	var (
		rows interface {
			Next() bool
			Scan(dest ...any) error
			Close() error
		}
		err error
	)

	if collection == "" {
		const sql = `
SELECT e.chunk_id, e.distance
FROM embeddings e
WHERE e.embedding MATCH ? AND k = ?
ORDER BY e.distance`
		rows, err = a.store.db.QueryContext(ctx, sql, blob, k)
	} else {
		const sql = `
SELECT e.chunk_id, e.distance
FROM embeddings e
JOIN chunks c  ON c.chunk_id = e.chunk_id
JOIN files  f  ON f.file_id  = c.file_id
WHERE e.embedding MATCH ? AND k = ?
  AND f.collection = ?
ORDER BY e.distance`
		rows, err = a.store.db.QueryContext(ctx, sql, blob, k, collection)
	}

	if err != nil {
		return nil, fmt.Errorf("sqlite.VectorReaderAdapter.SearchVector: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var hits []retrieval.Hit
	for rows.Next() {
		var h retrieval.Hit
		if scanErr := rows.Scan(&h.ChunkID, &h.Score); scanErr != nil {
			return nil, fmt.Errorf("sqlite.VectorReaderAdapter.SearchVector: scan: %w", scanErr)
		}
		hits = append(hits, h)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("sqlite.VectorReaderAdapter.SearchVector: close rows: %w", err)
	}

	if hits == nil {
		hits = []retrieval.Hit{}
	}
	return hits, nil
}

// packFloat32LE encodes a []float32 as a byte slice using little-endian
// IEEE 754 encoding (4 bytes per element).  This is the binary format
// required by the sqlite-vec MATCH operator.
//
// @MX:WARN: [AUTO] Little-endian IEEE 754 packing for sqlite-vec binary protocol.
// @MX:REASON: sqlite-vec MATCH requires exactly 4*len(v) bytes with each float32
// in little-endian order.  Using big-endian or native order corrupts the vector.
func packFloat32LE(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}
