// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
)

// pruneFlags holds the parsed flags for `mink memory prune`.
type pruneFlags struct {
	before     string
	collection string
	dryRun     bool
}

// pruneIndexPathOverride, when non-empty, replaces defaultIndexPath in runPrune.
var pruneIndexPathOverride string

// pruneVaultPathOverride, when non-empty, replaces defaultVaultPath in runPrune.
var pruneVaultPathOverride string

// pruneStagingHook is an optional test hook called between the staging move and
// the SQLite DELETE.  If it panics the two-phase commit recovery path is exercised.
// Production code leaves this nil.
var pruneStagingHook func()

// pruneFileRow represents one file eligible for deletion.
type pruneFileRow struct {
	fileID     int64
	sourcePath string
}

// stagingDir returns the staging directory path used by the prune two-phase commit.
func stagingDir(indexPath string) string {
	// Place staging under the same directory as the SQLite index so it survives
	// a kill -9 and is discoverable on the next run.
	return filepath.Join(filepath.Dir(indexPath), ".staged-deletion")
}

// NewPruneCommand returns the `mink memory prune` cobra subcommand.
//
// Usage: mink memory prune --before YYYY-MM-DD [--collection NAME] [--dry-run]
//
// Uses a two-phase commit pattern to maximise atomicity between disk and SQLite
// (AC-MEM-034):
//  1. Move markdown files to a staging directory.
//  2. DELETE chunks + files rows in a single transaction.
//  3. Commit; then hard-delete staged files.
//
// On interrupted runs (kill -9 between steps 1 and 2), repairPrunedState
// restores or cleans staged files on the next invocation.
//
// @MX:ANCHOR: [AUTO] Entry point for the prune subcommand.
// @MX:REASON: fan_in >= 3 (cobra RunE, integration tests, scheduled cleanup job).
//
// @MX:WARN: [AUTO] Two-phase commit between disk and SQLite.
// @MX:REASON: Strict atomicity is impossible across a filesystem and a database.
// The staging pattern minimises the window for partial state (AC-MEM-034).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T5.5
// REQ:  REQ-MEM-021, REQ-MEM-034
func NewPruneCommand() *cobra.Command {
	var f pruneFlags

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove old markdown files and their index entries",
		Long: `prune deletes markdown files and their SQLite index entries for files
older than --before.

A two-phase commit pattern is used to maximise atomicity:
  1. Markdown files are moved to a staging directory.
  2. The SQLite DELETE is committed.
  3. Staged files are hard-deleted.

On interrupted runs a repair pass restores or discards staged files on the
next invocation of any mink memory subcommand.`,
		Example: `  mink memory prune --before 2026-01-01
  mink memory prune --before 2026-01-01 --collection journal
  mink memory prune --before 2026-01-01 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrune(cmd, f)
		},
	}

	cmd.Flags().StringVar(&f.before, "before", "",
		"Delete files created before this date (YYYY-MM-DD, exclusive)")
	cmd.Flags().StringVar(&f.collection, "collection", "",
		"Restrict pruning to this collection")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false,
		"Print what would be deleted without making changes")
	_ = cmd.MarkFlagRequired("before")

	return cmd
}

// runPrune implements the `mink memory prune` workflow.
func runPrune(cmd *cobra.Command, f pruneFlags) error {
	ctx := cmd.Context()

	beforeDate, err := time.Parse(time.DateOnly, f.before)
	if err != nil {
		return fmt.Errorf("prune: invalid --before %q; expected YYYY-MM-DD: %w", f.before, err)
	}
	beforeUnix := beforeDate.Unix()

	rawIndex := defaultIndexPath
	if pruneIndexPathOverride != "" {
		rawIndex = pruneIndexPathOverride
	}
	indexPath, err := homedir.Expand(rawIndex)
	if err != nil {
		return fmt.Errorf("prune: expand index path: %w", err)
	}

	store, err := sqlite.Open(indexPath)
	if err != nil {
		return fmt.Errorf("prune: open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Repair any interrupted prune staging from a previous run.
	if repairErr := repairPrunedState(ctx, store, stagingDir(indexPath)); repairErr != nil {
		log.Printf("prune: repair staged state: %v (continuing)", repairErr)
	}

	// Query files eligible for deletion.
	candidates, err := queryPruneCandidates(ctx, store, beforeUnix, f.collection)
	if err != nil {
		return fmt.Errorf("prune: query candidates: %w", err)
	}

	if len(candidates) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "pruned: 0 files, 0 chunks\n")
		return nil
	}

	if f.dryRun {
		return emitDryRunPlan(cmd, candidates)
	}

	pruned, chunks, err := doPrune(ctx, store, candidates, indexPath)
	if err != nil {
		return fmt.Errorf("prune: execute: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "pruned: %d files, %d chunks\n", pruned, chunks)
	return nil
}

// queryPruneCandidates returns files created before beforeUnix (optionally filtered
// by collection).
func queryPruneCandidates(
	ctx context.Context,
	store *sqlite.Store,
	beforeUnix int64,
	collection string,
) ([]pruneFileRow, error) {
	db := store.DB()
	var (
		rows interface {
			Next() bool
			Scan(dest ...any) error
			Close() error
		}
		err error
	)

	if collection == "" {
		const q = `SELECT file_id, source_path FROM files WHERE created_at < ?`
		rows, err = db.QueryContext(ctx, q, beforeUnix)
	} else {
		const q = `SELECT file_id, source_path FROM files WHERE created_at < ? AND collection = ?`
		rows, err = db.QueryContext(ctx, q, beforeUnix, collection)
	}
	if err != nil {
		return nil, fmt.Errorf("queryPruneCandidates: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []pruneFileRow
	for rows.Next() {
		var r pruneFileRow
		if scanErr := rows.Scan(&r.fileID, &r.sourcePath); scanErr != nil {
			return nil, fmt.Errorf("queryPruneCandidates: scan: %w", scanErr)
		}
		result = append(result, r)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("queryPruneCandidates: close: %w", err)
	}
	return result, nil
}

// stagedFile records a file that was moved to the staging directory.
type stagedFile struct {
	original string // original vault path
	staged   string // staging directory path (empty if file was already missing)
}

// doPrune performs the two-phase commit delete.
// Returns (prunedFiles, prunedChunks, error).
func doPrune(
	ctx context.Context,
	store *sqlite.Store,
	candidates []pruneFileRow,
	indexPath string,
) (int, int, error) {
	staging := stagingDir(indexPath)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return 0, 0, fmt.Errorf("executePrune: create staging dir: %w", err)
	}

	// Phase 1: move markdown files to staging directory.
	// Track which files were staged so they can be restored on failure.
	var staged []stagedFile

	for _, c := range candidates {
		if _, statErr := os.Stat(c.sourcePath); os.IsNotExist(statErr) {
			// File already missing from disk — skip the move but still delete from DB.
			staged = append(staged, stagedFile{original: c.sourcePath, staged: ""})
			continue
		}
		// Use a unique name inside staging to avoid collisions.
		stagedName := filepath.Join(staging,
			fmt.Sprintf("%d_%s", c.fileID, filepath.Base(c.sourcePath)))
		if err := os.Rename(c.sourcePath, stagedName); err != nil {
			// Move failed — restore all previously staged files and abort.
			restoreStaged(staged)
			return 0, 0, fmt.Errorf("executePrune: stage file %q: %w", c.sourcePath, err)
		}
		staged = append(staged, stagedFile{original: c.sourcePath, staged: stagedName})
	}

	// Optional test hook: allows kill-9 simulation in tests.
	if pruneStagingHook != nil {
		pruneStagingHook()
	}

	// Phase 2: DELETE chunks + files rows in a single transaction.
	fileIDs := make([]int64, len(candidates))
	for i, c := range candidates {
		fileIDs[i] = c.fileID
	}

	chunkCount, err := deleteFromDB(ctx, store, fileIDs)
	if err != nil {
		// DB delete failed — restore staged files.
		restoreStaged(staged)
		return 0, 0, fmt.Errorf("executePrune: db delete: %w", err)
	}

	// Phase 3: hard-delete staged files (DB commit already succeeded).
	for _, sf := range staged {
		if sf.staged == "" {
			continue // file was already missing from disk
		}
		if err := os.Remove(sf.staged); err != nil {
			log.Printf("executePrune: warn: hard-delete staged file %q: %v", sf.staged, err)
		}
	}

	// Remove staging dir if empty.
	_ = os.Remove(staging)

	return len(candidates), chunkCount, nil
}

// deleteFromDB removes all chunks and file rows for the given fileIDs in a
// single SQLite transaction.
//
// Returns the total number of chunk rows deleted.
func deleteFromDB(ctx context.Context, store *sqlite.Store, fileIDs []int64) (int, error) {
	db := store.DB()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("deleteFromDB: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Count chunks before deletion (for reporting).
	totalChunks := 0
	for _, fid := range fileIDs {
		var cnt int
		if scanErr := tx.QueryRowContext(ctx, `SELECT count(*) FROM chunks WHERE file_id = ?`, fid).Scan(&cnt); scanErr == nil {
			totalChunks += cnt
		}
		// Delete FTS entries.
		rows, qErr := tx.QueryContext(ctx, `SELECT chunk_id FROM chunks WHERE file_id = ?`, fid)
		if qErr == nil {
			for rows.Next() {
				var cid string
				if sErr := rows.Scan(&cid); sErr == nil {
					_, _ = tx.ExecContext(ctx, "DELETE FROM chunks_fts WHERE chunk_id = ?", cid)
					_, _ = tx.ExecContext(ctx, "DELETE FROM embeddings WHERE chunk_id = ?", cid)
				}
			}
			_ = rows.Close()
		}
		if _, dErr := tx.ExecContext(ctx, `DELETE FROM chunks WHERE file_id = ?`, fid); dErr != nil {
			err = dErr
			return 0, fmt.Errorf("deleteFromDB: delete chunks for file_id=%d: %w", fid, dErr)
		}
		if _, dErr := tx.ExecContext(ctx, `DELETE FROM files WHERE file_id = ?`, fid); dErr != nil {
			err = dErr
			return 0, fmt.Errorf("deleteFromDB: delete file file_id=%d: %w", fid, dErr)
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("deleteFromDB: commit: %w", err)
	}
	return totalChunks, nil
}

// restoreStaged moves staged files back to their original locations.
// Used on failure to roll back Phase 1.
func restoreStaged(staged []stagedFile) {
	for _, sf := range staged {
		if sf.staged == "" {
			continue
		}
		if err := os.Rename(sf.staged, sf.original); err != nil {
			log.Printf("restoreStaged: warn: restore %q → %q: %v", sf.staged, sf.original, err)
		}
	}
}

// repairPrunedState is called at the start of every mink memory subcommand.
// It checks for leftover staging files from an interrupted prune and either:
//   - Restores them to their original locations (if the DB still has the rows), or
//   - Hard-deletes them (if the DB rows were already removed).
//
// The staging directory uses the convention:
//
//	{indexDir}/.staged-deletion/{fileID}_{filename}
//
// The fileID prefix is used to query the DB and determine recovery action.
func repairPrunedState(ctx context.Context, store *sqlite.Store, stagingPath string) error {
	entries, err := os.ReadDir(stagingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to repair
		}
		return fmt.Errorf("repairPrunedState: read staging dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		stagedPath := filepath.Join(stagingPath, name)

		// Parse the file ID from the name prefix "{fileID}_{rest}".
		var fileID int64
		restIdx := strings.Index(name, "_")
		if restIdx <= 0 {
			log.Printf("repairPrunedState: unparseable staged file %q; skipping", stagedPath)
			continue
		}
		if _, scanErr := fmt.Sscanf(name[:restIdx], "%d", &fileID); scanErr != nil {
			log.Printf("repairPrunedState: parse file_id from %q: %v (skipping)", name, scanErr)
			continue
		}

		// Check if the DB still has a row for this file.
		var sourcePath string
		err := store.DB().QueryRowContext(ctx,
			`SELECT source_path FROM files WHERE file_id = ?`, fileID,
		).Scan(&sourcePath)

		if err != nil {
			// DB row is gone — the commit completed; hard-delete the staged file.
			if removeErr := os.Remove(stagedPath); removeErr != nil {
				log.Printf("repairPrunedState: hard-delete %q: %v", stagedPath, removeErr)
			}
			continue
		}

		// DB row exists — the commit did not happen; restore the file.
		if _, statErr := os.Stat(sourcePath); os.IsNotExist(statErr) {
			if renameErr := os.Rename(stagedPath, sourcePath); renameErr != nil {
				log.Printf("repairPrunedState: restore %q → %q: %v", stagedPath, sourcePath, renameErr)
			} else {
				log.Printf("repairPrunedState: restored %q → %q", stagedPath, sourcePath)
			}
		}
	}

	// Attempt to remove the staging directory if it is now empty.
	_ = os.Remove(stagingPath)
	return nil
}

// emitDryRunPlan prints the files that would be deleted without making changes.
func emitDryRunPlan(cmd *cobra.Command, candidates []pruneFileRow) error {
	fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would prune %d files:\n", len(candidates))
	for _, c := range candidates {
		fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %s\n", c.fileID, c.sourcePath)
	}
	return nil
}
