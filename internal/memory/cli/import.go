// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
)

// importFlags holds the parsed flags for `mink memory import`.
type importFlags struct {
	source   string
	strategy string
}

// importIndexPathOverride, when non-empty, replaces defaultIndexPath in runImport.
var importIndexPathOverride string

// importVaultPathOverride, when non-empty, replaces defaultVaultPath in runImport.
var importVaultPathOverride string

// NewImportCommand returns the `mink memory import` cobra subcommand.
//
// Usage: mink memory import --source PATH [--strategy source-wins|skip-conflicts]
//
// Validates schema_version compatibility (AC-MEM-038), merges markdown files,
// and rebuilds chunks for merged files.
//
// @MX:ANCHOR: [AUTO] Entry point for the import subcommand.
// @MX:REASON: fan_in >= 3 (cobra RunE, integration tests, export round-trip tests).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T5.3
// REQ:  REQ-MEM-021, REQ-MEM-038
func NewImportCommand() *cobra.Command {
	var f importFlags

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Merge an exported vault tarball into the local vault",
		Example: `  mink memory import --source /tmp/backup.tar.gz
  mink memory import --source /tmp/backup.tar.gz --strategy skip-conflicts`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImport(cmd, f)
		},
	}

	cmd.Flags().StringVar(&f.source, "source", "",
		"Path to a .tar.gz file produced by `mink memory export`")
	cmd.Flags().StringVar(&f.strategy, "strategy", "source-wins",
		"Conflict resolution strategy: source-wins (default) or skip-conflicts")
	_ = cmd.MarkFlagRequired("source")

	return cmd
}

// runImport implements the `mink memory import` workflow.
//
// @MX:WARN: [AUTO] Multi-step workflow: untar → schema check → file merge → re-chunk.
// @MX:REASON: Partial failure between steps can leave the vault in a mixed state;
// schema check (AC-MEM-038) must gate all subsequent mutations.
func runImport(cmd *cobra.Command, f importFlags) error {
	ctx := cmd.Context()

	if f.strategy != "source-wins" && f.strategy != "skip-conflicts" {
		return fmt.Errorf("import: unknown strategy %q; use source-wins or skip-conflicts", f.strategy)
	}

	rawIndex := defaultIndexPath
	if importIndexPathOverride != "" {
		rawIndex = importIndexPathOverride
	}
	indexPath, err := homedir.Expand(rawIndex)
	if err != nil {
		return fmt.Errorf("import: expand index path: %w", err)
	}

	rawVault := defaultVaultPath
	if importVaultPathOverride != "" {
		rawVault = importVaultPathOverride
	}
	vaultRoot, err := homedir.Expand(rawVault)
	if err != nil {
		return fmt.Errorf("import: expand vault path: %w", err)
	}

	// Step 1: untar source into a temporary directory.
	tmpDir, err := os.MkdirTemp("", "mink-import-*")
	if err != nil {
		return fmt.Errorf("import: create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := untarGz(f.source, tmpDir); err != nil {
		return fmt.Errorf("import: untar source: %w", err)
	}

	// Step 2: parse manifest and validate schema_version.
	manifest, err := readManifest(tmpDir)
	if err != nil {
		return fmt.Errorf("import: read manifest: %w", err)
	}
	if manifest.SchemaVersion != sqlite.CurrentSchemaVersion {
		return fmt.Errorf(
			"import: schema_version mismatch: source=%s, current=%s; cannot import incompatible vault",
			manifest.SchemaVersion, sqlite.CurrentSchemaVersion,
		)
	}

	// Step 3: open the local store.
	store, err := sqlite.Open(indexPath)
	if err != nil {
		return fmt.Errorf("import: open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Step 4: walk the extracted markdown files and merge them.
	mdRoot := filepath.Join(tmpDir, "markdown")
	if _, statErr := os.Stat(mdRoot); os.IsNotExist(statErr) {
		// Nothing to import.
		fmt.Fprintf(cmd.OutOrStdout(), "imported: 0, conflict-resolved: 0, skipped: 0, total: 0\n")
		return nil
	}

	var (
		importedCount int
		conflictCount int
		skippedCount  int
	)

	err = filepath.WalkDir(mdRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		// Compute the relative path from the markdown root.
		rel, err := filepath.Rel(mdRoot, path)
		if err != nil {
			return fmt.Errorf("import: rel path for %q: %w", path, err)
		}

		// Parse collection and filename from the relative path.
		// Expected structure: <collection>/<filename.md>
		parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
		if len(parts) != 2 {
			log.Printf("import: skip unexpected path structure %q", rel)
			return nil
		}
		collection := parts[0]
		filename := parts[1]

		destDir := filepath.Join(vaultRoot, collection)
		if mkErr := os.MkdirAll(destDir, 0o700); mkErr != nil {
			return fmt.Errorf("import: create collection dir %q: %w", destDir, mkErr)
		}
		destFile := filepath.Join(destDir, filename)

		srcContent, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("import: read source file %q: %w", path, err)
		}
		srcSum := sha256.Sum256(srcContent)
		srcHash := fmt.Sprintf("%x", srcSum[:])

		action, err := mergeMarkdownFile(ctx, store, destFile, srcContent, srcHash, collection, f.strategy)
		if err != nil {
			return fmt.Errorf("import: merge %q: %w", destFile, err)
		}
		switch action {
		case "imported":
			importedCount++
		case "conflict":
			conflictCount++
		case "skipped":
			skippedCount++
		}
		return nil
	})
	if err != nil {
		return err
	}

	total := importedCount + conflictCount + skippedCount
	fmt.Fprintf(cmd.OutOrStdout(),
		"imported: %d, conflict-resolved: %d, skipped: %d, total: %d\n",
		importedCount, conflictCount, skippedCount, total)
	return nil
}

// mergeMarkdownFile copies or overwrites a markdown file based on the strategy
// and re-indexes the content.  Returns the action taken: "imported", "conflict",
// or "skipped".
func mergeMarkdownFile(
	ctx context.Context,
	store *sqlite.Store,
	destFile string,
	srcContent []byte,
	srcHash string,
	collection string,
	strategy string,
) (string, error) {
	existing, statErr := os.ReadFile(destFile)
	if statErr == nil {
		// File exists; check hash.
		existSum := sha256.Sum256(existing)
		existHash := fmt.Sprintf("%x", existSum[:])
		if existHash == srcHash {
			// Identical content — idempotent skip.
			return "skipped", nil
		}
		// Different content — conflict.
		switch strategy {
		case "skip-conflicts":
			log.Printf("import: conflict at %q; skipping (--strategy skip-conflicts)", destFile)
			return "skipped", nil
		default: // "source-wins"
			// Backup the existing file.
			bakPath := fmt.Sprintf("%s.bak.%d", destFile, time.Now().UnixNano())
			if err := os.Rename(destFile, bakPath); err != nil {
				return "", fmt.Errorf("mergeMarkdownFile: backup %q: %w", destFile, err)
			}
			log.Printf("import: conflict at %q; backed up to %q (--strategy source-wins)", destFile, bakPath)
		}
	}

	// Write the source file to the vault.
	if err := os.WriteFile(destFile, srcContent, 0o600); err != nil {
		return "", fmt.Errorf("mergeMarkdownFile: write %q: %w", destFile, err)
	}

	// Re-chunk and re-index.
	if err := reindexFile(ctx, store, destFile, srcContent, srcHash, collection); err != nil {
		// Log but do not abort the whole import for a single indexing failure.
		log.Printf("import: warn: reindex %q: %v", destFile, err)
	}

	if statErr == nil {
		// A backup was made — this is a conflict-resolved import.
		return "conflict", nil
	}
	return "imported", nil
}

// reindexFile chunks the given content and upserts the file + chunks into the
// store.  Used by import to index newly merged markdown files.
func reindexFile(
	ctx context.Context,
	store *sqlite.Store,
	sourcePath string,
	content []byte,
	contentHash string,
	collection string,
) error {
	chunks := qmd.ChunkMarkdown(string(content), qmd.ChunkOpts{MaxTokens: 512})
	now := time.Now().UTC()
	for i := range chunks {
		chunkSum := sha256.Sum256([]byte(chunks[i].Content))
		chunkHash := fmt.Sprintf("%x", chunkSum[:])
		chunks[i].ID = qmd.ChunkID(
			sourcePath,
			chunks[i].StartLine,
			chunks[i].EndLine,
			chunkHash,
			qmd.DefaultModelVersion,
		)
		chunks[i].SourcePath = sourcePath
		chunks[i].Collection = collection
		chunks[i].ModelVersion = qmd.DefaultModelVersion
		chunks[i].EmbeddingPending = true
		chunks[i].CreatedAt = now
	}
	chunks = qmd.LinkNeighbors(chunks)

	w, err := sqlite.NewWriter(store, "")
	if err != nil {
		return fmt.Errorf("reindexFile: acquire writer: %w", err)
	}
	defer func() { _ = w.Close() }()

	fileRec := qmd.File{
		Collection:  collection,
		SourcePath:  sourcePath,
		ContentHash: contentHash,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	fileID, err := w.UpsertFile(ctx, fileRec)
	if err != nil {
		return fmt.Errorf("reindexFile: upsert file: %w", err)
	}

	// Delete stale chunks before inserting fresh ones.
	if err := w.DeleteChunksByFileID(ctx, fileID); err != nil {
		return fmt.Errorf("reindexFile: delete old chunks: %w", err)
	}

	for i := range chunks {
		chunks[i].FileID = fileID
		if err := w.Insert(ctx, chunks[i]); err != nil {
			return fmt.Errorf("reindexFile: insert chunk %d: %w", i, err)
		}
	}
	return nil
}

// untarGz extracts a .tar.gz file into destDir.
func untarGz(srcPath, destDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("untarGz: open %q: %w", srcPath, err)
	}
	defer func() { _ = f.Close() }()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("untarGz: gzip reader for %q: %w", srcPath, err)
	}
	defer func() { _ = gzReader.Close() }()

	tr := tar.NewReader(gzReader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("untarGz: tar next: %w", err)
		}

		// Sanitize the path to prevent path traversal attacks.
		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("untarGz: suspicious path %q", hdr.Name)
		}

		destPath := filepath.Join(destDir, cleanName)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o700); err != nil {
				return fmt.Errorf("untarGz: mkdir %q: %w", destPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
				return fmt.Errorf("untarGz: mkdir parent %q: %w", destPath, err)
			}
			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("untarGz: create %q: %w", destPath, err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				_ = outFile.Close()
				return fmt.Errorf("untarGz: write %q: %w", destPath, err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("untarGz: close %q: %w", destPath, err)
			}
		}
	}
	return nil
}

// readManifest parses manifest.json from the extracted archive directory.
func readManifest(extractDir string) (exportManifest, error) {
	manifestPath := filepath.Join(extractDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return exportManifest{}, fmt.Errorf("readManifest: manifest.json not found in archive; is this a valid mink export?")
		}
		return exportManifest{}, fmt.Errorf("readManifest: read manifest: %w", err)
	}
	var m exportManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return exportManifest{}, fmt.Errorf("readManifest: parse manifest: %w", err)
	}
	if m.SchemaVersion == "" {
		return exportManifest{}, fmt.Errorf("readManifest: manifest missing schema_version field")
	}
	return m, nil
}
