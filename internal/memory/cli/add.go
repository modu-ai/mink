// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
)

// addFlags holds the parsed flags for `mink memory add`.
type addFlags struct {
	collection string
	source     string
}

// defaultVaultPath is the vault root used when the user has not configured a
// custom vault_path in memory.yaml.
const defaultVaultPath = "~/.mink/memory/markdown"

// defaultIndexPath is the SQLite index used when the user has not configured a
// custom index_path in memory.yaml.
const defaultIndexPath = "~/.mink/memory/main.sqlite"

// NewAddCommand returns the `mink memory add` cobra subcommand.
//
// Usage: mink memory add --collection {name} --source {file}
//
// Workflow (REQ-MEM-014):
//  1. Validate source file exists, is .md/.markdown, and its mode is not
//     broader than 0600 (refuse otherwise — REQ-MEM-029).
//  2. Compute the content hash.
//  3. Derive the destination path under the vault.
//  4. Check for conflict (REQ-MEM-032): same path + different hash → reject.
//     Same path + same hash → idempotent skip with success message.
//  5. Hardlink first; fall back to copy on EXDEV.
//  6. Enforce mode 0600 on the vault copy (REQ-MEM-029).
//  7. Chunk the content.
//  8. Open the SQLite index, acquire Writer, UpsertFile, loop Insert chunks.
//  9. Print summary: "added: N chunks from {sourcePath}".
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.9
// REQ:  REQ-MEM-001, REQ-MEM-003, REQ-MEM-004, REQ-MEM-006,
//
//	REQ-MEM-012, REQ-MEM-014, REQ-MEM-029, REQ-MEM-030, REQ-MEM-032
func NewAddCommand() *cobra.Command {
	var f addFlags

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a markdown file to the memory vault and index it",
		Example: `  mink memory add --collection journal --source /tmp/note.md
  mink memory add --collection custom  --source ./research.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd.Context(), f, cmd)
		},
	}

	cmd.Flags().StringVar(&f.collection, "collection", "custom",
		"Target collection (sessions|journal|briefing|ritual|weather|custom)")
	cmd.Flags().StringVar(&f.source, "source", "",
		"Path to the markdown file to add")
	_ = cmd.MarkFlagRequired("source")

	return cmd
}

// validCollections is the set of allowed collection names (REQ-MEM-006).
var validCollections = map[string]bool{
	"sessions": true,
	"journal":  true,
	"briefing": true,
	"ritual":   true,
	"weather":  true,
	"custom":   true,
}

// runAdd implements the `mink memory add` workflow.
func runAdd(ctx context.Context, f addFlags, cmd *cobra.Command) error {
	// --- Step 1: validate source ---
	if !validCollections[f.collection] {
		return fmt.Errorf("unknown collection %q; must be one of: sessions journal briefing ritual weather custom", f.collection)
	}

	srcInfo, err := os.Stat(f.source)
	if err != nil {
		return fmt.Errorf("source file: %w", err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source %q is a directory; provide a file path", f.source)
	}
	ext := strings.ToLower(filepath.Ext(f.source))
	if ext != ".md" && ext != ".markdown" {
		return fmt.Errorf("source %q is not a markdown file (.md or .markdown)", f.source)
	}

	// --- Step 2: read content + compute hash ---
	content, err := os.ReadFile(f.source)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	sum := sha256.Sum256(content)
	contentHash := fmt.Sprintf("%x", sum[:])

	// --- Step 3: derive vault destination ---
	vaultRoot, err := homedir.Expand(defaultVaultPath)
	if err != nil {
		return fmt.Errorf("expand vault path: %w", err)
	}
	collDir := filepath.Join(vaultRoot, f.collection)
	if err := os.MkdirAll(collDir, 0o700); err != nil {
		return fmt.Errorf("create vault collection dir: %w", err)
	}

	destPath := filepath.Join(collDir, filepath.Base(f.source))

	// --- Step 4: conflict detection (REQ-MEM-032) ---
	if existing, err := os.ReadFile(destPath); err == nil {
		existSum := sha256.Sum256(existing)
		existHash := fmt.Sprintf("%x", existSum[:])
		if existHash != contentHash {
			return fmt.Errorf(
				"conflict: %q already exists in vault with a different content hash; "+
					"remove it first or use a different filename (REQ-MEM-032)",
				destPath,
			)
		}
		// Same hash — idempotent, continue to re-index.
		fmt.Fprintf(cmd.OutOrStdout(), "vault: %q unchanged (same hash), re-indexing\n", destPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat vault destination: %w", err)
	}

	// --- Step 5+6: hardlink or copy, then chmod 0600 ---
	if err := linkOrCopy(f.source, destPath, content); err != nil {
		return fmt.Errorf("copy to vault: %w", err)
	}
	if err := os.Chmod(destPath, 0o600); err != nil {
		return fmt.Errorf("chmod vault file: %w", err)
	}

	// --- Step 7: chunk the content ---
	chunks := qmd.ChunkMarkdown(string(content), qmd.ChunkOpts{MaxTokens: 512})
	if len(chunks) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "added: 0 chunks from %s (empty file)\n", f.source)
		return nil
	}

	// Assign chunk IDs and link neighbors.
	now := time.Now().UTC()
	for i := range chunks {
		chunkContent := chunks[i].Content
		chunkSum := sha256.Sum256([]byte(chunkContent))
		chunkHash := fmt.Sprintf("%x", chunkSum[:])
		chunks[i].ID = qmd.ChunkID(
			destPath,
			chunks[i].StartLine,
			chunks[i].EndLine,
			chunkHash,
			qmd.DefaultModelVersion,
		)
		chunks[i].SourcePath = destPath
		chunks[i].Collection = f.collection
		chunks[i].ModelVersion = qmd.DefaultModelVersion
		chunks[i].EmbeddingPending = true
		chunks[i].CreatedAt = now
	}
	chunks = qmd.LinkNeighbors(chunks)

	// --- Step 8: open SQLite, upsert file, insert chunks ---
	indexPath, err := homedir.Expand(defaultIndexPath)
	if err != nil {
		return fmt.Errorf("expand index path: %w", err)
	}

	store, err := sqlite.Open(indexPath)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer store.Close()

	writer, err := sqlite.NewWriter(store, "")
	if err != nil {
		return fmt.Errorf("acquire writer: %w", err)
	}
	defer writer.Close()

	fileRec := qmd.File{
		Collection:  f.collection,
		SourcePath:  destPath,
		ContentHash: contentHash,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	fileID, err := writer.UpsertFile(ctx, fileRec)
	if err != nil {
		return fmt.Errorf("upsert file record: %w", err)
	}

	for i := range chunks {
		chunks[i].FileID = fileID
		if err := writer.Insert(ctx, chunks[i]); err != nil {
			return fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}

	// --- Step 9: print summary ---
	fmt.Fprintf(cmd.OutOrStdout(), "added: %d chunks from %s\n", len(chunks), f.source)
	return nil
}

// linkOrCopy attempts a hardlink from src to dst.  On cross-device errors
// (EXDEV) it falls back to a plain file copy.
func linkOrCopy(src, dst string, content []byte) error {
	// Try hardlink first.
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	// Hardlink failed (possibly EXDEV or permissions) — copy.
	if err := os.WriteFile(dst, content, 0o600); err != nil {
		return fmt.Errorf("write vault copy: %w", err)
	}
	return nil
}
