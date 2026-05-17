// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/modu-ai/mink/internal/memory/ollama"
)

// defaultBackfillBatchSize is the number of pending chunks processed per
// RunOnce call.
const defaultBackfillBatchSize = 16

// defaultBackfillInterval is the tick period for the background Run loop.
const defaultBackfillInterval = 30 * time.Second

// EmbedClient is the subset of ollama.Client used by the backfill worker.
// Abstracted as an interface for testability.
type EmbedClient interface {
	Embed(ctx context.Context, model, text string) ([]float32, error)
}

// Backfiller processes chunks with embedding_pending = 1 and stores their
// embeddings in the vec0 virtual table.
//
// @MX:WARN: [AUTO] Run starts a long-lived goroutine; context cancellation is
// the only stop mechanism.
// @MX:REASON: The background loop must not leak when the caller context is
// cancelled (e.g., CLI process exit). Any early return from Run without
// honouring ctx.Done() would prevent clean shutdown.
type Backfiller struct {
	store     *Store
	client    EmbedClient
	model     string
	batchSize int
	interval  time.Duration
	logger    *log.Logger
}

// NewBackfiller constructs a Backfiller with default batch size and interval.
func NewBackfiller(store *Store, client EmbedClient, model string) *Backfiller {
	return &Backfiller{
		store:     store,
		client:    client,
		model:     model,
		batchSize: defaultBackfillBatchSize,
		interval:  defaultBackfillInterval,
		logger:    log.Default(),
	}
}

// RunOnce processes one batch of pending chunks.
//
// SQL pattern:
//
//	SELECT chunk_id, content FROM chunks WHERE embedding_pending = 1 LIMIT ?
//
// For each row, Embed is called and the result is stored atomically:
//   - INSERT OR REPLACE INTO embeddings(chunk_id, embedding)
//   - UPDATE chunks SET embedding_pending = 0, model_version = ?
//
// Recoverable embed errors (ollama.ShouldFallbackToBM25 == true) leave the
// chunk pending and continue to the next.  Unrecoverable errors abort RunOnce.
//
// Returns (succeeded, failed, err) where err is only non-nil for
// unrecoverable failures.
//
// @MX:ANCHOR: [AUTO] Central embedding backfill entry point for pending chunks.
// @MX:REASON: fan_in >= 3 (EnqueueAsync, Run loop, direct test calls).
// Contract: RunOnce must leave the database in a consistent state even on
// partial failure.
func (b *Backfiller) RunOnce(ctx context.Context) (succeeded, failed int, err error) {
	// Fetch a batch of pending chunks.
	const fetchSQL = `SELECT chunk_id, content FROM chunks WHERE embedding_pending = 1 LIMIT ?`
	rows, err := b.store.db.QueryContext(ctx, fetchSQL, b.batchSize)
	if err != nil {
		return 0, 0, fmt.Errorf("sqlite.Backfiller.RunOnce: fetch pending: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pendingChunk struct {
		id      string
		content string
	}
	var pending []pendingChunk
	for rows.Next() {
		var pc pendingChunk
		if scanErr := rows.Scan(&pc.id, &pc.content); scanErr != nil {
			return succeeded, failed, fmt.Errorf("sqlite.Backfiller.RunOnce: scan: %w", scanErr)
		}
		pending = append(pending, pc)
	}
	if err := rows.Close(); err != nil {
		return succeeded, failed, fmt.Errorf("sqlite.Backfiller.RunOnce: close rows: %w", err)
	}

	for _, pc := range pending {
		vec, embedErr := b.client.Embed(ctx, b.model, pc.content)
		if embedErr != nil {
			if ollama.ShouldFallbackToBM25(embedErr) {
				b.logger.Printf("sqlite.Backfiller.RunOnce: recoverable embed error for %q (leaving pending): %v", pc.id, embedErr)
				failed++
				continue
			}
			// Unrecoverable error — stop processing.
			return succeeded, failed, fmt.Errorf("sqlite.Backfiller.RunOnce: embed %q: %w", pc.id, embedErr)
		}

		if storeErr := b.storeEmbedding(ctx, pc.id, vec); storeErr != nil {
			b.logger.Printf("sqlite.Backfiller.RunOnce: store embedding for %q: %v", pc.id, storeErr)
			failed++
			continue
		}
		succeeded++
	}

	return succeeded, failed, nil
}

// storeEmbedding writes the embedding and clears the pending flag in a single
// transaction.
func (b *Backfiller) storeEmbedding(ctx context.Context, chunkID string, embedding []float32) error {
	tx, err := b.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	blob := packEmbedding(embedding)

	const upsertEmbed = `INSERT OR REPLACE INTO embeddings(chunk_id, embedding) VALUES (?, ?)`
	if _, err = tx.ExecContext(ctx, upsertEmbed, chunkID, blob); err != nil {
		return fmt.Errorf("upsert embedding: %w", err)
	}

	const updateChunk = `UPDATE chunks SET embedding_pending = 0, model_version = ? WHERE chunk_id = ?`
	if _, err = tx.ExecContext(ctx, updateChunk, b.model, chunkID); err != nil {
		return fmt.Errorf("update chunk pending flag: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Run starts a loop that calls RunOnce every interval.
//
// Errors from RunOnce are logged but do not stop the loop.  The loop exits
// when ctx is cancelled.
func (b *Backfiller) Run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, fail, err := b.RunOnce(ctx)
			if err != nil {
				b.logger.Printf("sqlite.Backfiller.Run: RunOnce error: %v", err)
			} else if ok+fail > 0 {
				b.logger.Printf("sqlite.Backfiller.Run: embedded %d chunks (%d failed)", ok, fail)
			}
		}
	}
}

// EnqueueAsync triggers a one-shot RunOnce call in a goroutine and returns
// immediately.  Used by `mink memory add` to backfill in the background after
// inserting new chunks (REQ-MEM-030: CLI must return within 100ms).
//
// The goroutine inherits ctx so it stops if the parent context is cancelled.
func (b *Backfiller) EnqueueAsync(ctx context.Context) {
	go func() {
		ok, fail, err := b.RunOnce(ctx)
		if err != nil {
			b.logger.Printf("sqlite.Backfiller.EnqueueAsync: RunOnce error: %v", err)
		} else if ok+fail > 0 {
			b.logger.Printf("sqlite.Backfiller.EnqueueAsync: embedded %d chunks (%d failed)", ok, fail)
		}
	}()
}

// packEmbedding encodes a []float32 as a little-endian IEEE 754 byte slice.
// This matches packFloat32LE in vector_reader.go but is duplicated here to
// keep the backfill logic self-contained (no circular import risk).
//
// @MX:WARN: [AUTO] Little-endian IEEE 754 packing invariant for sqlite-vec blob.
// @MX:REASON: sqlite-vec INSERT requires the same binary format as SearchVector
// MATCH queries.  Using any other encoding silently corrupts stored vectors.
func packEmbedding(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}
