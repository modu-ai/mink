// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"text/tabwriter"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
)

// statsFlags holds the parsed flags for `mink memory stats`.
type statsFlags struct {
	jsonOut bool
}

// statsIndexPathOverride, when non-empty, replaces defaultIndexPath in runStats.
var statsIndexPathOverride string

// collectionStat holds per-collection aggregated statistics.
type collectionStat struct {
	Collection     string `json:"collection"`
	Files          int    `json:"files"`
	Chunks         int    `json:"chunks"`
	EmbeddingBytes int64  `json:"embedding_bytes"`
	Oldest         string `json:"oldest"`
	Newest         string `json:"newest"`
}

// statsOutput is the JSON schema for `mink memory stats --json`.
type statsOutput struct {
	PerCollection []collectionStat `json:"per_collection"`
	Total         collectionStat   `json:"total"`
}

// NewStatsCommand returns the `mink memory stats` cobra subcommand.
//
// Usage: mink memory stats [--json]
//
// Outputs a 6-column table (COLLECTION, FILES, CHUNKS, EMB_SIZE, OLDEST, NEWEST)
// or a JSON document when --json is set (AC-MEM-037).
//
// @MX:ANCHOR: [AUTO] Entry point for the stats subcommand.
// @MX:REASON: fan_in >= 3 (cobra RunE, integration tests, future dashboard).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T5.4
// REQ:  REQ-MEM-037
func NewStatsCommand() *cobra.Command {
	var f statsFlags

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show per-collection memory vault statistics",
		Example: `  mink memory stats
  mink memory stats --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStats(cmd, f)
		},
	}

	cmd.Flags().BoolVar(&f.jsonOut, "json", false,
		"Emit statistics as JSON")

	return cmd
}

// runStats implements the `mink memory stats` workflow.
func runStats(cmd *cobra.Command, f statsFlags) error {
	ctx := cmd.Context()

	rawIndex := defaultIndexPath
	if statsIndexPathOverride != "" {
		rawIndex = statsIndexPathOverride
	}
	indexPath, err := homedir.Expand(rawIndex)
	if err != nil {
		return fmt.Errorf("stats: expand index path: %w", err)
	}

	store, err := sqlite.Open(indexPath)
	if err != nil {
		return fmt.Errorf("stats: open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	rows, err := queryCollectionStats(ctx, store)
	if err != nil {
		return fmt.Errorf("stats: query: %w", err)
	}

	// Compute per-collection embedding sizes (approximate: rows × 4096 bytes).
	embCounts, err := queryEmbeddingCounts(ctx, store)
	if err != nil {
		return fmt.Errorf("stats: embedding counts: %w", err)
	}
	for i := range rows {
		rows[i].EmbeddingBytes = int64(embCounts[rows[i].Collection]) * 1024 * 4
	}

	total := computeTotal(rows)

	if f.jsonOut {
		return emitStatsJSON(cmd, rows, total)
	}
	return emitStatsTable(cmd, rows, total)
}

// queryCollectionStats returns per-collection aggregated stats from the DB.
func queryCollectionStats(ctx context.Context, store *sqlite.Store) ([]collectionStat, error) {
	const q = `
SELECT
    files.collection,
    count(DISTINCT files.file_id) AS file_count,
    count(chunks.chunk_id)        AS chunk_count,
    min(chunks.created_at)        AS oldest,
    max(chunks.created_at)        AS newest
FROM files
LEFT JOIN chunks ON chunks.file_id = files.file_id
GROUP BY files.collection
ORDER BY files.collection`

	dbRows, err := store.DB().QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("queryCollectionStats: query: %w", err)
	}
	defer func() { _ = dbRows.Close() }()

	var stats []collectionStat
	for dbRows.Next() {
		var s collectionStat
		var oldestUnix, newestUnix *int64
		if scanErr := dbRows.Scan(
			&s.Collection,
			&s.Files,
			&s.Chunks,
			&oldestUnix,
			&newestUnix,
		); scanErr != nil {
			return nil, fmt.Errorf("queryCollectionStats: scan: %w", scanErr)
		}
		if oldestUnix != nil {
			s.Oldest = time.Unix(*oldestUnix, 0).UTC().Format(time.DateOnly)
		}
		if newestUnix != nil {
			s.Newest = time.Unix(*newestUnix, 0).UTC().Format(time.DateOnly)
		}
		stats = append(stats, s)
	}
	if err := dbRows.Close(); err != nil {
		return nil, fmt.Errorf("queryCollectionStats: close: %w", err)
	}
	return stats, nil
}

// queryEmbeddingCounts returns a map of collection → embedding row count.
// The embedding size per row is 1024 float32 = 4096 bytes.
func queryEmbeddingCounts(ctx context.Context, store *sqlite.Store) (map[string]int, error) {
	const q = `
SELECT f.collection, count(e.chunk_id)
FROM embeddings e
JOIN chunks c ON c.chunk_id = e.chunk_id
JOIN files  f ON f.file_id  = c.file_id
GROUP BY f.collection`

	counts := make(map[string]int)
	rows, err := store.DB().QueryContext(ctx, q)
	if err != nil {
		// vec0 may not be present; treat as empty.
		return counts, nil
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var coll string
		var n int
		if scanErr := rows.Scan(&coll, &n); scanErr != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("queryEmbeddingCounts: scan: %w", scanErr)
		}
		counts[coll] = n
	}
	_ = rows.Close()
	return counts, nil
}

// computeTotal aggregates all collection stats into a single TOTAL row.
func computeTotal(rows []collectionStat) collectionStat {
	total := collectionStat{Collection: "TOTAL"}
	for _, r := range rows {
		total.Files += r.Files
		total.Chunks += r.Chunks
		total.EmbeddingBytes += r.EmbeddingBytes
		if total.Oldest == "" || (r.Oldest != "" && r.Oldest < total.Oldest) {
			total.Oldest = r.Oldest
		}
		if total.Newest == "" || (r.Newest != "" && r.Newest > total.Newest) {
			total.Newest = r.Newest
		}
	}
	return total
}

// formatBytes converts bytes to a human-readable string (e.g. "1.20 MiB").
func formatBytes(b int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case b >= gib:
		return fmt.Sprintf("%.2f GiB", float64(b)/float64(gib))
	case b >= mib:
		return fmt.Sprintf("%.2f MiB", float64(b)/float64(mib))
	case b >= kib:
		return fmt.Sprintf("%.0f KiB", math.Round(float64(b)/float64(kib)))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// emitStatsTable writes the human-readable 6-column table (AC-MEM-037).
func emitStatsTable(cmd *cobra.Command, rows []collectionStat, total collectionStat) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "COLLECTION\tFILES\tCHUNKS\tEMB_SIZE\tOLDEST\tNEWEST")
	for _, r := range rows {
		oldest := r.Oldest
		if oldest == "" {
			oldest = "-"
		}
		newest := r.Newest
		if newest == "" {
			newest = "-"
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\t%s\n",
			r.Collection, r.Files, r.Chunks, formatBytes(r.EmbeddingBytes), oldest, newest)
	}
	// Print TOTAL row.
	oldest := total.Oldest
	if oldest == "" {
		oldest = "-"
	}
	newest := total.Newest
	if newest == "" {
		newest = "-"
	}
	fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\t%s\n",
		total.Collection, total.Files, total.Chunks, formatBytes(total.EmbeddingBytes), oldest, newest)
	return w.Flush()
}

// emitStatsJSON emits the statistics as a JSON document.
func emitStatsJSON(cmd *cobra.Command, rows []collectionStat, total collectionStat) error {
	out := statsOutput{
		PerCollection: rows,
		Total:         total,
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
