// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/modu-ai/mink/internal/memory/ollama"
	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
)

// reindexFlags holds the parsed flags for `mink memory reindex`.
type reindexFlags struct {
	collection string
	model      string
	all        bool
	reembed    bool
}

// reindexIndexPathOverride, when non-empty, replaces defaultIndexPath in
// runReindex.  Used only in tests to inject a per-test SQLite path.
var reindexIndexPathOverride string

// NewReindexCommand returns the `mink memory reindex` cobra subcommand.
//
// Usage: mink memory reindex [--collection NAME] [--model NAME] [--all] [--reembed]
//
// Workflow (AC-MEM-016):
//  1. Open Store, query chunks with stale model_version.
//  2. For each affected file_id: read markdown, re-chunk, delete old chunks,
//     insert new chunks.  Writer is acquired + released per file so that
//     concurrent read calls (AC-MEM-022) are not blocked for the whole run.
//  3. Optionally trigger Backfiller.RunOnce when --reembed is set.
//  4. Print summary line.
//
// @MX:ANCHOR: [AUTO] Entry point for the reindex subcommand.
// @MX:REASON: fan_in >= 3 (cobra RunE, integration tests, future scheduled job).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T5.1
// REQ:  REQ-MEM-016, REQ-MEM-022
func NewReindexCommand() *cobra.Command {
	var f reindexFlags

	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild stale index entries from vault markdown files",
		Long: `reindex scans for chunks whose model_version does not match the current
default (or --model) and re-chunks the parent markdown file, replacing the
stale rows with fresh chunks marked embedding_pending=1.

Concurrent read (search) operations are not blocked because the Writer lock
is held only for the per-file DELETE+INSERT transaction.`,
		Example: `  mink memory reindex
  mink memory reindex --collection journal
  mink memory reindex --model nomic-embed-text --reembed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReindex(cmd, f)
		},
	}

	cmd.Flags().StringVar(&f.collection, "collection", "",
		"Restrict reindex to this collection (default: all collections)")
	cmd.Flags().StringVar(&f.model, "model", qmd.DefaultModelVersion,
		"Target model version; chunks with a different version are re-indexed")
	cmd.Flags().BoolVar(&f.all, "all", false,
		"Force re-index of all chunks regardless of model_version")
	cmd.Flags().BoolVar(&f.reembed, "reembed", false,
		"After re-indexing, run a backfill pass to produce fresh embeddings (requires Ollama)")

	return cmd
}

// reindexFileRow is one row from the stale-chunks query.
type reindexFileRow struct {
	fileID     int64
	sourcePath string
	collection string
}

// runReindex implements the `mink memory reindex` workflow.
func runReindex(cmd *cobra.Command, f reindexFlags) error {
	ctx := cmd.Context()

	rawIndex := defaultIndexPath
	if reindexIndexPathOverride != "" {
		rawIndex = reindexIndexPathOverride
	}
	indexPath, err := homedir.Expand(rawIndex)
	if err != nil {
		return fmt.Errorf("reindex: expand index path: %w", err)
	}

	store, err := sqlite.Open(indexPath)
	if err != nil {
		return fmt.Errorf("reindex: open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Build the query for stale file_ids.
	staleFiles, err := queryStaleFiles(ctx, store, f)
	if err != nil {
		return fmt.Errorf("reindex: query stale files: %w", err)
	}

	var (
		reindexedFiles  int
		reindexedChunks int
		orphanCount     int
		skippedCount    int
	)

	for _, row := range staleFiles {
		content, err := os.ReadFile(row.sourcePath)
		if err != nil {
			// Markdown file missing from disk — warn and count as orphan.
			if os.IsNotExist(err) {
				log.Printf("reindex: warn: source file missing for file_id=%d path=%q (orphan)", row.fileID, row.sourcePath)
				orphanCount++
				continue
			}
			// Other read error — skip.
			log.Printf("reindex: warn: read source file %q: %v (skipping)", row.sourcePath, err)
			skippedCount++
			continue
		}

		// Re-chunk the markdown content.
		chunks := qmd.ChunkMarkdown(string(content), qmd.ChunkOpts{MaxTokens: 512})
		now := time.Now().UTC()
		for i := range chunks {
			chunkContent := chunks[i].Content
			chunkSum := sha256.Sum256([]byte(chunkContent))
			chunkHash := fmt.Sprintf("%x", chunkSum[:])
			chunks[i].ID = qmd.ChunkID(
				row.sourcePath,
				chunks[i].StartLine,
				chunks[i].EndLine,
				chunkHash,
				f.model,
			)
			chunks[i].FileID = row.fileID
			chunks[i].SourcePath = row.sourcePath
			chunks[i].Collection = row.collection
			chunks[i].ModelVersion = f.model
			chunks[i].EmbeddingPending = true
			chunks[i].CreatedAt = now
		}
		chunks = qmd.LinkNeighbors(chunks)

		// Hold the Writer only for this file's DELETE+INSERT so readers
		// are not blocked across the entire reindex run (AC-MEM-022).
		if err := reindexOneFile(ctx, store, row.fileID, chunks); err != nil {
			log.Printf("reindex: warn: reindex file_id=%d: %v (skipping)", row.fileID, err)
			skippedCount++
			continue
		}

		reindexedFiles++
		reindexedChunks += len(chunks)
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"reindexed: %d files, %d chunks (orphan: %d, skipped: %d)\n",
		reindexedFiles, reindexedChunks, orphanCount, skippedCount)

	// Optional re-embed pass.
	if f.reembed {
		ollamaClient := newOllamaClientForCLI()
		bf := sqlite.NewBackfiller(store, ollamaClient, f.model)
		_, _, _ = bf.RunOnce(ctx)
	}

	return nil
}

// queryStaleFiles returns all distinct (file_id, source_path, collection)
// rows whose chunks have a model_version that differs from f.model (or all
// rows when f.all is set).
func queryStaleFiles(ctx context.Context, store *sqlite.Store, f reindexFlags) ([]reindexFileRow, error) {
	db := store.DB()

	var (
		rows interface {
			Next() bool
			Scan(dest ...any) error
			Close() error
			Err() error
		}
		err error
	)

	// Build query based on flags.
	if f.all {
		if f.collection == "" {
			const q = `
SELECT DISTINCT c.file_id, fi.source_path, fi.collection
FROM chunks c
JOIN files fi ON fi.file_id = c.file_id`
			rows, err = db.QueryContext(ctx, q)
		} else {
			const q = `
SELECT DISTINCT c.file_id, fi.source_path, fi.collection
FROM chunks c
JOIN files fi ON fi.file_id = c.file_id
WHERE fi.collection = ?`
			rows, err = db.QueryContext(ctx, q, f.collection)
		}
	} else {
		// Default: only chunks with a different model_version.
		if f.collection == "" {
			const q = `
SELECT DISTINCT c.file_id, fi.source_path, fi.collection
FROM chunks c
JOIN files fi ON fi.file_id = c.file_id
WHERE c.model_version != ?`
			rows, err = db.QueryContext(ctx, q, f.model)
		} else {
			const q = `
SELECT DISTINCT c.file_id, fi.source_path, fi.collection
FROM chunks c
JOIN files fi ON fi.file_id = c.file_id
WHERE c.model_version != ?
  AND fi.collection = ?`
			rows, err = db.QueryContext(ctx, q, f.model, f.collection)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("queryStaleFiles: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []reindexFileRow
	for rows.Next() {
		var r reindexFileRow
		if scanErr := rows.Scan(&r.fileID, &r.sourcePath, &r.collection); scanErr != nil {
			return nil, fmt.Errorf("queryStaleFiles: scan: %w", scanErr)
		}
		result = append(result, r)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("queryStaleFiles: close: %w", err)
	}
	return result, nil
}

// reindexOneFile deletes all existing chunks for fileID and inserts the new
// ones inside a single Writer lock (per-file granularity).
func reindexOneFile(ctx context.Context, store *sqlite.Store, fileID int64, chunks []qmd.Chunk) error {
	// Acquire the Writer for this file only — released at function end.
	w, err := sqlite.NewWriter(store, "")
	if err != nil {
		return fmt.Errorf("reindexOneFile: acquire writer: %w", err)
	}
	defer func() { _ = w.Close() }()

	// Delete all existing chunks for this file.  The ON DELETE CASCADE on the
	// chunks table propagates through chunk_id to chunks_fts and embeddings.
	if err := w.DeleteChunksByFileID(ctx, fileID); err != nil {
		return fmt.Errorf("reindexOneFile: delete old chunks: %w", err)
	}

	// Insert new chunks.
	for i := range chunks {
		if err := w.Insert(ctx, chunks[i]); err != nil {
			return fmt.Errorf("reindexOneFile: insert chunk %d: %w", i, err)
		}
	}

	return nil
}

// newOllamaClientForCLI builds an ollama client using the default empty host
// (resolves to http://localhost:11434 by the ollama package).
func newOllamaClientForCLI() sqlite.EmbedClient {
	return ollama.NewClient("")
}
