// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mitchellh/go-homedir"
	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/retrieval"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
)

// searchFlags holds the parsed flags for `mink memory search`.
type searchFlags struct {
	collection string
	limit      int
	mode       string
	jsonOut    bool
}

// searchIndexPathOverride, when non-empty, replaces defaultIndexPath in runSearch.
// Used only in tests to inject a per-test SQLite path without touching the real vault.
var searchIndexPathOverride string

// searchResultJSON is the JSON schema emitted by `mink memory search --json`.
// Field names match the AC-MEM-015 verification schema.
type searchResultJSON struct {
	ChunkID    string  `json:"chunk_id"`
	SourcePath string  `json:"source_path"`
	StartLine  int     `json:"start_line"`
	EndLine    int     `json:"end_line"`
	Score      float64 `json:"score"`
	Snippet    string  `json:"snippet"`
}

// hardCapLimit is the maximum allowed value for the --limit flag.
const hardCapLimit = 100

// NewSearchCommand returns the `mink memory search` cobra subcommand.
//
// Usage: mink memory search QUERY [--collection NAME] [--limit N] [--mode MODE] [--json]
//
// Mode "search" (BM25) is the only mode implemented in M2.
// Modes "vsearch" and "query" return ErrModeNotImplementedM2 (M3/M4).
//
// Exit codes:
//
//	0 — success (including zero matches)
//	1 — user error (bad flag, bad mode)
//	2 — infrastructure error (SQLite unavailable, FTS5 missing)
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T2.3
// REQ:  REQ-MEM-015, REQ-MEM-016, REQ-MEM-017, REQ-MEM-018, REQ-MEM-030
func NewSearchCommand() *cobra.Command {
	var f searchFlags

	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search the memory vault using BM25 full-text search",
		Args:  cobra.ExactArgs(1),
		Example: `  mink memory search "golang concurrency"
  mink memory search "오늘 날씨" --collection journal
  mink memory search "machine learning" --limit 5 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, args[0], f)
		},
	}

	cmd.Flags().StringVar(&f.collection, "collection", "",
		"Restrict search to this collection (sessions|journal|briefing|ritual|weather|custom)")
	cmd.Flags().IntVar(&f.limit, "limit", 10,
		"Maximum number of results (1–100)")
	cmd.Flags().StringVar(&f.mode, "mode", "search",
		"Retrieval mode: search (BM25, default), vsearch (M3), query (M4)")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false,
		"Emit results as a JSON array")

	return cmd
}

// runSearch implements the `mink memory search` workflow.
//
// @MX:ANCHOR: [AUTO] CLI entry point wiring BM25Runner to the search subcommand.
// @MX:REASON: fan_in >= 3 (cobra RunE, integration tests, future gRPC bridge). Invariant:
// must propagate exit codes 0/1/2 correctly.
func runSearch(cmd *cobra.Command, query string, f searchFlags) error {
	// --- Validate flags ---
	if f.limit < 1 {
		cmd.SilenceUsage = false
		return fmt.Errorf("--limit must be at least 1")
	}
	if f.limit > hardCapLimit {
		f.limit = hardCapLimit
	}

	// --- Validate mode (early-out before touching SQLite) ---
	switch f.mode {
	case "search":
		// OK — BM25 wired in M2.
	case "vsearch", "query":
		return retrieval.ErrModeNotImplementedM2
	default:
		return fmt.Errorf("unknown --mode %q; must be search, vsearch, or query", f.mode)
	}

	// --- Open SQLite index ---
	rawPath := defaultIndexPath
	if searchIndexPathOverride != "" {
		rawPath = searchIndexPathOverride
	}
	indexPath, err := homedir.Expand(rawPath)
	if err != nil {
		return fmt.Errorf("expand index path: %w", err)
	}

	store, err := sqlite.Open(indexPath)
	if err != nil {
		// Infrastructure error → exit 2.
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: open index: %v\n", err)
		os.Exit(2)
		return nil // unreachable; satisfies compiler
	}
	defer func() { _ = store.Close() }()

	// --- Build retrieval pipeline ---
	sqliteReader := sqlite.NewReader(store)
	readerAdapter := sqlite.NewBM25ReaderAdapter(sqliteReader)
	lookupAdapter := sqlite.NewChunkLookupStore(store)
	runner := retrieval.NewBM25Runner(readerAdapter, lookupAdapter)

	ctx := cmd.Context()
	opts := qmd.SearchOpts{
		Collection: f.collection,
		Limit:      f.limit,
		Mode:       f.mode,
	}

	results, err := runner.RunBM25(ctx, query, opts)
	if err != nil {
		if errors.Is(err, retrieval.ErrEmptyQuery) {
			return err // user error → exit 1
		}
		if errors.Is(err, sqlite.ErrFTS5Unavailable) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: FTS5 not available in this SQLite build\n")
			os.Exit(2)
			return nil
		}
		return fmt.Errorf("search error: %w", err)
	}

	// --- Emit output ---
	if f.jsonOut {
		return emitJSON(cmd, results)
	}
	return emitTable(cmd, results)
}

// emitJSON writes the results as a JSON array to cmd's stdout.
func emitJSON(cmd *cobra.Command, results []qmd.Result) error {
	out := make([]searchResultJSON, len(results))
	for i, r := range results {
		out[i] = searchResultJSON{
			ChunkID:    r.Chunk.ID,
			SourcePath: r.Chunk.SourcePath,
			StartLine:  r.Chunk.StartLine,
			EndLine:    r.Chunk.EndLine,
			Score:      r.Score,
			Snippet:    r.Snippet,
		}
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// emitTable writes human-readable tabular output to cmd's stdout.
// Format: chunk_id  score  source:start-end  snippet…
func emitTable(cmd *cobra.Command, results []qmd.Result) error {
	if len(results) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no results)")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "CHUNK_ID\tSCORE\tSOURCE\tSNIPPET")
	for _, r := range results {
		loc := fmt.Sprintf("%s:%d-%d", r.Chunk.SourcePath, r.Chunk.StartLine, r.Chunk.EndLine)
		snippet := r.Snippet
		const maxSnippetDisplay = 60
		if len([]rune(snippet)) > maxSnippetDisplay {
			snippet = string([]rune(snippet)[:maxSnippetDisplay]) + "…"
		}
		_, _ = fmt.Fprintf(w, "%s\t%.4f\t%s\t%s\n",
			r.Chunk.ID, r.Score, loc, snippet)
	}
	return w.Flush()
}
