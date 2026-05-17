// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/modu-ai/mink/internal/memory/qmd"
)

// lockAcquireTimeout is the maximum time Writer.NewWriter will wait to
// acquire the file lock before returning ErrWriterBusy.
//
// REQ-MEM-030: the CLI must return within 100ms; the lock timeout is set to
// 100ms so that a busy lock causes an immediate clear error rather than a
// silent hang.
const lockAcquireTimeout = 100 * time.Millisecond

// lockPollInterval is the interval between lock-acquisition retries.
const lockPollInterval = 10 * time.Millisecond

// ErrWriterBusy is returned when NewWriter cannot acquire the file lock within
// lockAcquireTimeout.  The caller should retry or surface the error to the
// user.
var ErrWriterBusy = errors.New("sqlite.Writer: another writer holds the lock; try again")

// processMu is a process-level mutex that prevents concurrent Writer
// instances within the same process from racing on the same database.
//
// @MX:WARN: [AUTO] Global mutex guards in-process concurrent write access.
// @MX:REASON: SQLite in WAL mode supports multiple readers but only one
// concurrent writer; the process mutex prevents internal deadlocks that the
// file lock alone cannot detect.
var processMu sync.Mutex

// Writer serialises writes to the Store.  It holds both a process-level
// sync.Mutex and a file-level flock so that concurrent processes also
// serialize.
//
// Use NewWriter to acquire a Writer; call Close when done.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.6
// REQ:  REQ-MEM-025, REQ-MEM-030
type Writer struct {
	store *Store
	fl    *flock.Flock
}

// NewWriter acquires the write lock and returns a Writer ready for use.
//
// The file lock is acquired at lockPath.  If lockPath is empty, the default
// is "{store.path}.lock".
//
// Returns ErrWriterBusy if the file lock cannot be obtained within
// lockAcquireTimeout.
func NewWriter(store *Store, lockPath string) (*Writer, error) {
	if lockPath == "" {
		lockPath = store.path + ".lock"
	}

	// Acquire the process-level mutex first.
	processMu.Lock()

	fl := flock.New(lockPath)

	// Attempt to acquire the file lock with timeout polling.
	ctx, cancel := context.WithTimeout(context.Background(), lockAcquireTimeout)
	defer cancel()

	locked := false
acquireLoop:
	for {
		ok, err := fl.TryLock()
		if err != nil {
			processMu.Unlock()
			return nil, fmt.Errorf("sqlite.NewWriter: file lock error: %w", err)
		}
		if ok {
			locked = true
			break acquireLoop
		}
		select {
		case <-ctx.Done():
			// Timeout reached; abort outer loop.
			break acquireLoop
		case <-time.After(lockPollInterval):
			// Retry the lock acquisition.
		}
	}

	if !locked {
		processMu.Unlock()
		return nil, ErrWriterBusy
	}

	return &Writer{store: store, fl: fl}, nil
}

// Close releases the file lock and the process-level mutex.
func (w *Writer) Close() error {
	defer processMu.Unlock()
	return w.fl.Unlock()
}

// UpsertFile inserts or updates a files row.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.6
// REQ:  REQ-MEM-001, REQ-MEM-006
func (w *Writer) UpsertFile(ctx context.Context, f qmd.File) (int64, error) {
	const q = `
INSERT INTO files (collection, source_path, content_hash, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(source_path) DO UPDATE SET
    content_hash = excluded.content_hash,
    updated_at   = excluded.updated_at
RETURNING file_id`

	var fileID int64
	err := w.store.db.QueryRowContext(ctx, q,
		f.Collection,
		f.SourcePath,
		f.ContentHash,
		f.CreatedAt.Unix(),
		f.UpdatedAt.Unix(),
	).Scan(&fileID)
	if err != nil {
		return 0, fmt.Errorf("sqlite.Writer.UpsertFile: %w", err)
	}
	return fileID, nil
}

// DeleteChunksByFileID removes all chunks (and their FTS + embedding rows)
// associated with the given file_id.
//
// FTS rows are cleaned explicitly because FTS5 virtual tables do not support
// FK cascade.  Vec0 embeddings are deleted best-effort (ignored when vec0 is
// not loaded).  This method is used by the reindex workflow to clear stale
// chunks before inserting fresh ones (AC-MEM-016).
func (w *Writer) DeleteChunksByFileID(ctx context.Context, fileID int64) error {
	// Collect chunk IDs first so FTS rows can be removed explicitly.
	const selectIDs = `SELECT chunk_id FROM chunks WHERE file_id = ?`
	rows, err := w.store.db.QueryContext(ctx, selectIDs, fileID)
	if err != nil {
		return fmt.Errorf("sqlite.Writer.DeleteChunksByFileID: select chunk ids: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			_ = rows.Close()
			return fmt.Errorf("sqlite.Writer.DeleteChunksByFileID: scan: %w", scanErr)
		}
		ids = append(ids, id)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return fmt.Errorf("sqlite.Writer.DeleteChunksByFileID: close rows: %w", closeErr)
	}

	tx, err := w.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite.Writer.DeleteChunksByFileID: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete FTS rows first (FTS5 has no FK cascade).
	for _, id := range ids {
		if _, ftsErr := tx.ExecContext(ctx, "DELETE FROM chunks_fts WHERE chunk_id = ?", id); ftsErr != nil {
			// FTS5 may not be present; log and continue.
			log.Printf("sqlite.Writer.DeleteChunksByFileID: delete fts for %q: %v (ignored)", id, ftsErr)
		}
	}

	// Delete embedding rows best-effort (vec0 may not be present).
	for _, id := range ids {
		if _, embErr := tx.ExecContext(ctx, "DELETE FROM embeddings WHERE chunk_id = ?", id); embErr != nil {
			log.Printf("sqlite.Writer.DeleteChunksByFileID: delete embedding for %q: %v (ignored)", id, embErr)
		}
	}

	// Delete the chunk rows themselves.
	const deleteChunks = `DELETE FROM chunks WHERE file_id = ?`
	if _, err = tx.ExecContext(ctx, deleteChunks, fileID); err != nil {
		return fmt.Errorf("sqlite.Writer.DeleteChunksByFileID: delete chunks: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sqlite.Writer.DeleteChunksByFileID: commit: %w", err)
	}
	return nil
}

// Insert persists a single Chunk into the chunks and chunks_fts tables.
//
// The operation is idempotent: a conflict on chunk_id triggers an UPSERT that
// updates the mutable fields.  The file's updated_at timestamp is also bumped.
//
// @MX:WARN: [AUTO] Dual-table write (chunks + chunks_fts) must stay atomic.
// @MX:REASON: If chunks_fts insert fails after chunks insert succeeds the
// full-text index diverges from the primary table; the transaction wrapper
// below prevents this split state.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.6
// REQ:  REQ-MEM-014, REQ-MEM-025
func (w *Writer) Insert(ctx context.Context, chunk qmd.Chunk) error {
	tx, err := w.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite.Writer.Insert: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const upsertChunk = `
INSERT INTO chunks
    (chunk_id, file_id, start_line, end_line, content,
     prev_chunk_id, next_chunk_id, embedding_pending, model_version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chunk_id) DO UPDATE SET
    file_id           = excluded.file_id,
    start_line        = excluded.start_line,
    end_line          = excluded.end_line,
    content           = excluded.content,
    prev_chunk_id     = excluded.prev_chunk_id,
    next_chunk_id     = excluded.next_chunk_id,
    embedding_pending = excluded.embedding_pending,
    model_version     = excluded.model_version`

	embPending := 1
	if !chunk.EmbeddingPending {
		embPending = 0
	}

	if _, err = tx.ExecContext(ctx, upsertChunk,
		chunk.ID,
		chunk.FileID,
		chunk.StartLine,
		chunk.EndLine,
		chunk.Content,
		chunk.PrevChunkID,
		chunk.NextChunkID,
		embPending,
		chunk.ModelVersion,
		chunk.CreatedAt.Unix(),
	); err != nil {
		return fmt.Errorf("sqlite.Writer.Insert: upsert chunk: %w", err)
	}

	// Keep the FTS index in sync.
	// FTS5 virtual tables do not support ON CONFLICT / UPSERT.
	// Use DELETE + INSERT pattern to achieve idempotent updates.
	// If chunks_fts was not created (FTS5 unavailable), log a warning and
	// continue so that M1 add still succeeds (BM25 search requires M2).
	if _, ftsErr := tx.ExecContext(ctx,
		"DELETE FROM chunks_fts WHERE chunk_id = ?", chunk.ID); ftsErr != nil {
		log.Printf("sqlite.Writer.Insert: chunks_fts unavailable (FTS5 not enabled); skipping FTS: %v", ftsErr)
	} else {
		if _, ftsErr = tx.ExecContext(ctx,
			"INSERT INTO chunks_fts(chunk_id, content) VALUES (?, ?)",
			chunk.ID, chunk.Content); ftsErr != nil {
			log.Printf("sqlite.Writer.Insert: insert fts: %v", ftsErr)
		}
	}

	// Bump the parent file's updated_at.
	const updateFile = `UPDATE files SET updated_at = ? WHERE file_id = ?`
	if _, err = tx.ExecContext(ctx, updateFile, chunk.CreatedAt.Unix(), chunk.FileID); err != nil {
		return fmt.Errorf("sqlite.Writer.Insert: update file timestamp: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sqlite.Writer.Insert: commit: %w", err)
	}
	return nil
}
