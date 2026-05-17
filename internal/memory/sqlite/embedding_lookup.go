// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build cgo

package sqlite

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// EmbeddingLookupStore wraps a Store and implements retrieval.EmbeddingLookup.
//
// It fetches stored 32-bit little-endian IEEE 754 float blobs from the
// embeddings virtual table and decodes them into []float32 slices.
//
// @MX:NOTE: [AUTO] Adapter satisfying retrieval.EmbeddingLookup without
// creating an import cycle.  Import direction: sqlite → retrieval (OK).
type EmbeddingLookupStore struct {
	store *Store
}

// NewEmbeddingLookupStore wraps store in an EmbeddingLookupStore adapter.
func NewEmbeddingLookupStore(store *Store) *EmbeddingLookupStore {
	return &EmbeddingLookupStore{store: store}
}

// LookupEmbeddings fetches the stored embedding for each chunk_id in a single
// query.  Returns a map keyed by chunk_id; missing chunks are silently omitted.
//
// When vec0 is unavailable (HasVec0 == false), returns an empty map and no
// error so the caller can treat missing embeddings as cosine score = 0.
//
// SQL: SELECT chunk_id, embedding FROM embeddings WHERE chunk_id IN (?,?,...)
func (e *EmbeddingLookupStore) LookupEmbeddings(ctx context.Context, chunkIDs []string) (map[string][]float32, error) {
	if len(chunkIDs) == 0 {
		return map[string][]float32{}, nil
	}

	// When vec0 is unavailable, return empty map (caller treats cosine as 0).
	if !e.store.HasVec0() {
		return map[string][]float32{}, nil
	}

	// Build parameterised IN clause.
	placeholders := make([]string, len(chunkIDs))
	args := make([]any, len(chunkIDs))
	for i, id := range chunkIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	sql := fmt.Sprintf(
		`SELECT chunk_id, embedding FROM embeddings WHERE chunk_id IN (%s)`,
		strings.Join(placeholders, ","),
	)

	rows, err := e.store.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite.EmbeddingLookupStore.LookupEmbeddings: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]float32, len(chunkIDs))
	for rows.Next() {
		var chunkID string
		var blob []byte
		if scanErr := rows.Scan(&chunkID, &blob); scanErr != nil {
			return nil, fmt.Errorf("sqlite.EmbeddingLookupStore.LookupEmbeddings: scan: %w", scanErr)
		}
		vec, decErr := unpackFloat32LE(blob)
		if decErr != nil {
			// Skip malformed blobs silently; caller treats as cosine = 0.
			continue
		}
		result[chunkID] = vec
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("sqlite.EmbeddingLookupStore.LookupEmbeddings: close rows: %w", err)
	}

	return result, nil
}

// unpackFloat32LE decodes a little-endian IEEE 754 byte slice into []float32.
// Returns an error when the blob length is not a multiple of 4.
//
// @MX:NOTE: [AUTO] Inverse of packFloat32LE / packEmbedding.  Used by
// EmbeddingLookupStore to decode stored blobs for hybrid scoring.
func unpackFloat32LE(blob []byte) ([]float32, error) {
	if len(blob)%4 != 0 {
		return nil, fmt.Errorf("sqlite.unpackFloat32LE: blob length %d is not a multiple of 4", len(blob))
	}
	n := len(blob) / 4
	out := make([]float32, n)
	for i := range n {
		bits := binary.LittleEndian.Uint32(blob[i*4:])
		out[i] = math.Float32frombits(bits)
	}
	return out, nil
}
