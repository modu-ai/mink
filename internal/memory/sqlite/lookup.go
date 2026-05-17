// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
)

// ChunkLookupStore wraps a Store and implements retrieval.ChunkLookup.
//
// ChunkLookupStore is the bridge between the sqlite package (which owns the
// raw database) and the retrieval package (which must not import sqlite
// directly to avoid import cycles).
//
// @MX:NOTE: [AUTO] Adapter that satisfies retrieval.ChunkLookup without creating
// an import cycle. retrieval/ defines the interface; sqlite/ provides the impl.
type ChunkLookupStore struct {
	store *Store
}

// NewChunkLookupStore wraps store in a ChunkLookupStore adapter.
func NewChunkLookupStore(store *Store) *ChunkLookupStore {
	return &ChunkLookupStore{store: store}
}

// LookupChunks fetches full Chunk records for the given chunk IDs in a single
// SELECT … IN (…) query.
//
// Chunks that are not found are silently omitted from the result slice.
// The returned slice preserves the database row order (by chunk_id), which
// may differ from the input order.
func (s *ChunkLookupStore) LookupChunks(ctx context.Context, chunkIDs []string) ([]qmd.Chunk, error) {
	if len(chunkIDs) == 0 {
		return []qmd.Chunk{}, nil
	}

	// Build the parameterised IN clause.
	placeholders := make([]string, len(chunkIDs))
	args := make([]any, 0, len(chunkIDs))
	for i, id := range chunkIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	sql := fmt.Sprintf(`
SELECT
    c.chunk_id,
    c.file_id,
    f.collection,
    f.source_path,
    c.start_line,
    c.end_line,
    c.content,
    c.prev_chunk_id,
    c.next_chunk_id,
    c.embedding_pending,
    c.model_version,
    c.created_at
FROM chunks c
JOIN files f ON f.file_id = c.file_id
WHERE c.chunk_id IN (%s)`, strings.Join(placeholders, ","))

	rows, err := s.store.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite.ChunkLookupStore.LookupChunks: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var chunks []qmd.Chunk
	for rows.Next() {
		var ch qmd.Chunk
		var embPending int
		var createdAtUnix int64

		if scanErr := rows.Scan(
			&ch.ID,
			&ch.FileID,
			&ch.Collection,
			&ch.SourcePath,
			&ch.StartLine,
			&ch.EndLine,
			&ch.Content,
			&ch.PrevChunkID,
			&ch.NextChunkID,
			&embPending,
			&ch.ModelVersion,
			&createdAtUnix,
		); scanErr != nil {
			return nil, fmt.Errorf("sqlite.ChunkLookupStore.LookupChunks: scan: %w", scanErr)
		}

		ch.EmbeddingPending = embPending != 0
		ch.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
		chunks = append(chunks, ch)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("sqlite.ChunkLookupStore.LookupChunks: close rows: %w", err)
	}

	if chunks == nil {
		chunks = []qmd.Chunk{}
	}
	return chunks, nil
}
