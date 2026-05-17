// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/modu-ai/mink/internal/memory/ollama"
	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/retrieval"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
)

// defaultOllamaModel is the embedding model used for vsearch/query when --model is
// not specified.
const defaultOllamaModel = "mxbai-embed-large"

// searchFlags holds the parsed flags for `mink memory search`.
type searchFlags struct {
	collection    string
	limit         int
	mode          string
	model         string
	jsonOut       bool
	alpha         float64
	beta          float64
	gamma         float64
	decayHalfLife time.Duration
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
// Mode "query" (hybrid, M4) is the default.
// Mode "search" (BM25) and "vsearch" (vector, M3) are also available.
//
// Exit codes:
//
//	0 — success (including zero matches, and BM25 fallback from vsearch/query)
//	1 — user error (bad flag, bad mode)
//	2 — infrastructure error (SQLite unavailable, FTS5 missing)
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T2.3, T3.6, T4.4
// REQ:  REQ-MEM-008, REQ-MEM-015, REQ-MEM-016, REQ-MEM-017, REQ-MEM-018, REQ-MEM-019, REQ-MEM-030
func NewSearchCommand() *cobra.Command {
	var f searchFlags

	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search the memory vault using hybrid, BM25, or vector search",
		Args:  cobra.ExactArgs(1),
		Example: `  mink memory search "golang concurrency"
  mink memory search "오늘 날씨" --collection journal
  mink memory search "machine learning" --limit 5 --json
  mink memory search "embeddings" --mode vsearch
  mink memory search "rust safety" --mode search
  mink memory search "deep learning" --alpha 0.5 --beta 0.5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, args[0], f)
		},
	}

	cmd.Flags().StringVar(&f.collection, "collection", "",
		"Restrict search to this collection (sessions|journal|briefing|ritual|weather|custom)")
	cmd.Flags().IntVar(&f.limit, "limit", 10,
		"Maximum number of results (1–100)")
	cmd.Flags().StringVar(&f.mode, "mode", "query",
		"Retrieval mode: query (hybrid, default), search (BM25), vsearch (vector)")
	cmd.Flags().StringVar(&f.model, "model", defaultOllamaModel,
		"Ollama embedding model for --mode vsearch or --mode query")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false,
		"Emit results as a JSON array")
	cmd.Flags().Float64Var(&f.alpha, "alpha", 0,
		"Cosine weight for hybrid mode (overrides default 0.7; 0 = use default)")
	cmd.Flags().Float64Var(&f.beta, "beta", 0,
		"BM25 weight for hybrid mode (overrides default 0.3; 0 = use default)")
	cmd.Flags().Float64Var(&f.gamma, "gamma", 0,
		"Decay weight for hybrid mode (overrides default 0.0; 0 = use default)")
	cmd.Flags().DurationVar(&f.decayHalfLife, "decay-half-life", 0,
		"Temporal decay half-life for hybrid mode (0 = use default 30d)")

	return cmd
}

// runSearch implements the `mink memory search` workflow.
//
// @MX:ANCHOR: [AUTO] CLI entry point wiring BM25Runner, VectorRunner, and HybridRunner.
// @MX:REASON: fan_in >= 3 (cobra RunE, integration tests, future gRPC bridge).
// Invariant: must propagate exit codes 0/1/2 correctly.
func runSearch(cmd *cobra.Command, query string, f searchFlags) error {
	// --- Validate flags ---
	if f.limit < 1 {
		cmd.SilenceUsage = false
		return fmt.Errorf("--limit must be at least 1")
	}
	if f.limit > hardCapLimit {
		f.limit = hardCapLimit
	}

	// --- Validate mode ---
	switch f.mode {
	case "query", "search", "vsearch":
		// All three modes are wired in M4.
	default:
		return fmt.Errorf("unknown --mode %q; must be query, search, or vsearch", f.mode)
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

	// --- Dispatch based on mode ---
	opts := qmd.SearchOpts{
		Collection: f.collection,
		Limit:      f.limit,
		Mode:       f.mode,
	}

	switch f.mode {
	case "vsearch":
		return runVsearch(cmd, store, query, opts, f.model)
	case "search":
		return runBM25Search(cmd, store, query, opts)
	default: // "query"
		return runHybridQuery(cmd, store, query, opts, f)
	}
}

// runBM25Search executes a BM25 search and writes results to cmd's output.
func runBM25Search(cmd *cobra.Command, store *sqlite.Store, query string, opts qmd.SearchOpts) error {
	ctx := cmd.Context()
	sqliteReader := sqlite.NewReader(store)
	readerAdapter := sqlite.NewBM25ReaderAdapter(sqliteReader)
	lookupAdapter := sqlite.NewChunkLookupStore(store)
	runner := retrieval.NewBM25Runner(readerAdapter, lookupAdapter)

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

	return emitResults(cmd, results, false)
}

// runVsearch executes vector similarity search, falling back to BM25 on
// recoverable Ollama errors or when vec0 is unavailable (AC-MEM-019).
func runVsearch(cmd *cobra.Command, store *sqlite.Store, query string, opts qmd.SearchOpts, model string) error {
	ctx := cmd.Context()
	ollamaClient := ollama.NewClient("")
	embedFunc := func(embedCtx context.Context, m, text string) ([]float32, error) {
		return ollamaClient.Embed(embedCtx, m, text)
	}

	vecReader := sqlite.NewVectorReaderAdapter(store)
	lookupAdapter := sqlite.NewChunkLookupStore(store)
	vRunner := retrieval.NewVectorRunner(vecReader, lookupAdapter, embedFunc, model)

	results, err := vRunner.RunVector(ctx, query, opts)
	if err != nil {
		if ollama.ShouldFallbackToBM25(err) || errors.Is(err, retrieval.ErrVec0Unavailable) {
			// Transparent fall-back to BM25 with exit code 0 (AC-MEM-019).
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
				"warning: ollama unreachable; falling back to BM25\n")
			bm25Opts := opts
			bm25Opts.Mode = "search"
			return runBM25Search(cmd, store, query, bm25Opts)
		}
		return fmt.Errorf("vsearch error: %w", err)
	}

	return emitResults(cmd, results, false)
}

// runHybridQuery executes hybrid BM25 + vector + decay search (mode "query").
// On ErrFellBackToBM25 it emits a warning to stderr and returns results with exit 0.
func runHybridQuery(cmd *cobra.Command, store *sqlite.Store, query string, opts qmd.SearchOpts, f searchFlags) error {
	ctx := cmd.Context()

	ollamaClient := ollama.NewClient("")
	embedFunc := func(embedCtx context.Context, m, text string) ([]float32, error) {
		return ollamaClient.Embed(embedCtx, m, text)
	}

	bm25Reader := sqlite.NewBM25ReaderAdapter(sqlite.NewReader(store))
	vecReader := sqlite.NewVectorReaderAdapter(store)
	lookupAdapter := sqlite.NewChunkLookupStore(store)
	embedLookup := sqlite.NewEmbeddingLookupStore(store)

	cfg := retrieval.DefaultHybridConfig()
	// Apply user overrides when non-zero flags are provided.
	if f.alpha != 0 {
		cfg.Alpha = f.alpha
	}
	if f.beta != 0 {
		cfg.Beta = f.beta
	}
	if f.gamma != 0 {
		cfg.Gamma = f.gamma
	}
	if f.decayHalfLife > 0 {
		cfg.DecayHalfLife = f.decayHalfLife
	}

	runner := retrieval.NewHybridRunner(bm25Reader, vecReader, lookupAdapter, embedLookup, embedFunc, f.model, cfg)
	results, err := runner.RunHybrid(ctx, query, opts)

	if errors.Is(err, retrieval.ErrFellBackToBM25) {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"warning: ollama unreachable; hybrid degraded to BM25-only\n")
		// Results are still valid — treat as success.
		return emitResults(cmd, results, false)
	}
	if err != nil {
		if errors.Is(err, retrieval.ErrEmptyQuery) {
			return err
		}
		return fmt.Errorf("query error: %w", err)
	}

	// Apply MMR re-ranking (λ=0.7, k=opts.Limit).
	if len(results) > 1 {
		results = retrieval.MMRRerank(results, nil, retrieval.MMRConfig{Lambda: 0.7}, opts.Limit)
	}

	return emitResults(cmd, results, false)
}

// isJSONMode reports whether the --json flag is set.
func isJSONMode(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("json")
	return f != nil && f.Value.String() == "true"
}

// emitResults dispatches to JSON or table format based on the --json flag.
func emitResults(cmd *cobra.Command, results []qmd.Result, _ bool) error {
	if isJSONMode(cmd) {
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
