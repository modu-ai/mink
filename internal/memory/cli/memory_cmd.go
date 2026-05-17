// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package cli provides the `mink memory` subcommand family for MINK's QMD
// memory subsystem.
//
// M1 implements: memory add
// M2 implements: memory search (BM25 full-text search)
// M5 implements: memory reindex, export, import, stats, prune
//
// SPEC: SPEC-MINK-MEMORY-QMD-001
package cli

import "github.com/spf13/cobra"

// NewMemoryCommand returns the parent `mink memory` cobra command.
// Subcommands are registered on it by each milestone (add for M1, etc.).
//
// @MX:ANCHOR: [AUTO] Parent command for memory subcommand family.
// @MX:REASON: All memory subcommands (add/search/reindex/export/import/stats/prune)
// are added here; fan_in >= 7 at SPEC completion.
func NewMemoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage MINK's lifelong memory vault",
		Long: `The memory subcommand family manages MINK's QMD-based lifelong memory.

Markdown files are the single source of truth (REQ-MEM-001).  The SQLite
index is a derived, rebuildable artefact.

Available subcommands (M1):
  add       Add a markdown file to the memory vault and index it

Available subcommands (M2):
  search    Full-text (BM25) search across the memory vault

Available subcommands (M5):
  reindex   Rebuild stale index entries from vault markdown files
  export    Export the vault as a tarball
  import    Merge an exported vault tarball into the local vault
  stats     Show per-collection memory vault statistics
  prune     Remove old markdown files and their index entries`,
	}

	// M1 subcommands.
	cmd.AddCommand(NewAddCommand())

	// M2 subcommands.
	cmd.AddCommand(NewSearchCommand())

	// M5 subcommands.
	cmd.AddCommand(
		NewReindexCommand(),
		NewExportCommand(),
		NewImportCommand(),
		NewStatsCommand(),
		NewPruneCommand(),
	)

	return cmd
}
