// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/memory/retrieval"
)

// BM25ReaderAdapter wraps a Reader and satisfies the retrieval.BM25Reader
// interface by translating sqlite.Hit values to retrieval.Hit values.
//
// This adapter lives in the sqlite package (not retrieval) so the import
// direction is sqlite → retrieval, avoiding any import cycle.
type BM25ReaderAdapter struct {
	r *Reader
}

// NewBM25ReaderAdapter constructs a BM25ReaderAdapter around the given Reader.
func NewBM25ReaderAdapter(r *Reader) *BM25ReaderAdapter {
	return &BM25ReaderAdapter{r: r}
}

// SearchBM25 delegates to the underlying Reader and converts sqlite.Hit
// slices into retrieval.Hit slices.
func (a *BM25ReaderAdapter) SearchBM25(ctx context.Context, q, collection string, k int) ([]retrieval.Hit, error) {
	sqliteHits, err := a.r.SearchBM25(ctx, q, collection, k)
	if err != nil {
		return nil, fmt.Errorf("BM25ReaderAdapter: %w", err)
	}

	out := make([]retrieval.Hit, len(sqliteHits))
	for i, h := range sqliteHits {
		out[i] = retrieval.Hit{
			ChunkID: h.ChunkID,
			Score:   h.Score,
		}
	}
	return out, nil
}
