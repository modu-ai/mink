// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
)

// ErrClawMemNotImplementedM5a is returned when --format clawmem is requested.
// The ClawMem export path is planned for M5b.
var ErrClawMemNotImplementedM5a = errors.New("export: clawmem format is not implemented in M5a; use --format tar")

// exportFlags holds the parsed flags for `mink memory export`.
type exportFlags struct {
	dest       string
	format     string
	collection string
}

// exportIndexPathOverride, when non-empty, replaces defaultIndexPath in runExport.
var exportIndexPathOverride string

// exportVaultPathOverride, when non-empty, replaces defaultVaultPath in runExport.
var exportVaultPathOverride string

// exportManifest is the JSON structure written as manifest.json inside the tarball.
type exportManifest struct {
	SchemaVersion    string `json:"schema_version"`
	ExportedAt       string `json:"exported_at"`
	CollectionFilter string `json:"collection_filter"`
	FileCount        int    `json:"file_count"`
	ChunkCount       int    `json:"chunk_count"`
}

// NewExportCommand returns the `mink memory export` cobra subcommand.
//
// Usage: mink memory export --dest PATH [--format tar|clawmem] [--collection NAME]
//
// For M5a only "tar" format is implemented. "clawmem" returns
// ErrClawMemNotImplementedM5a.
//
// @MX:ANCHOR: [AUTO] Entry point for the export subcommand.
// @MX:REASON: fan_in >= 3 (cobra RunE, integration tests, import command).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T5.2
// REQ:  REQ-MEM-020
func NewExportCommand() *cobra.Command {
	var f exportFlags

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the memory vault as a tarball",
		Example: `  mink memory export --dest /tmp/backup
  mink memory export --dest /tmp/journal-only --collection journal
  mink memory export --dest /tmp/backup --format tar`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(cmd, f)
		},
	}

	cmd.Flags().StringVar(&f.dest, "dest", "",
		"Destination path (without extension; .tar.gz will be appended)")
	cmd.Flags().StringVar(&f.format, "format", "tar",
		"Export format: tar (default) or clawmem (M5b stub)")
	cmd.Flags().StringVar(&f.collection, "collection", "",
		"Export only this collection (default: all)")
	_ = cmd.MarkFlagRequired("dest")

	return cmd
}

// runExport implements the `mink memory export` workflow.
func runExport(cmd *cobra.Command, f exportFlags) error {
	if f.format == "clawmem" {
		return ErrClawMemNotImplementedM5a
	}
	if f.format != "tar" {
		return fmt.Errorf("export: unknown format %q; use tar or clawmem", f.format)
	}

	ctx := cmd.Context()

	// Resolve paths.
	rawIndex := defaultIndexPath
	if exportIndexPathOverride != "" {
		rawIndex = exportIndexPathOverride
	}
	indexPath, err := homedir.Expand(rawIndex)
	if err != nil {
		return fmt.Errorf("export: expand index path: %w", err)
	}

	rawVault := defaultVaultPath
	if exportVaultPathOverride != "" {
		rawVault = exportVaultPathOverride
	}
	vaultRoot, err := homedir.Expand(rawVault)
	if err != nil {
		return fmt.Errorf("export: expand vault path: %w", err)
	}

	destPath := f.dest + ".tar.gz"

	// Refuse to overwrite an existing destination.
	if _, statErr := os.Stat(destPath); statErr == nil {
		return fmt.Errorf("export: destination %q already exists; remove it first", destPath)
	}

	store, err := sqlite.Open(indexPath)
	if err != nil {
		return fmt.Errorf("export: open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Count files and chunks for the manifest.
	fileCount, chunkCount, err := countFilesAndChunks(ctx, store, f.collection)
	if err != nil {
		return fmt.Errorf("export: count stats: %w", err)
	}

	// Flush the WAL so the SQLite snapshot is consistent.
	if _, execErr := store.DB().ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); execErr != nil {
		return fmt.Errorf("export: wal checkpoint: %w", execErr)
	}

	// Create the output file.
	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("export: create output file: %w", err)
	}
	// On error, remove the partial file.
	var writeErr error
	defer func() {
		_ = outFile.Close()
		if writeErr != nil {
			_ = os.Remove(destPath)
		}
	}()

	gzWriter := gzip.NewWriter(outFile)
	tarWriter := tar.NewWriter(gzWriter)

	// Walk the vault directory and add markdown files to the tarball.
	walkRoot := vaultRoot
	if f.collection != "" {
		walkRoot = filepath.Join(vaultRoot, f.collection)
	}

	if err := addMarkdownFilesToTar(tarWriter, walkRoot, vaultRoot); err != nil {
		writeErr = err
		return fmt.Errorf("export: add markdown files: %w", err)
	}

	// Add the SQLite index file.
	if err := addFileToTar(tarWriter, indexPath, "main.sqlite"); err != nil {
		writeErr = err
		return fmt.Errorf("export: add sqlite file: %w", err)
	}

	// Add manifest.json.
	manifest := exportManifest{
		SchemaVersion:    sqlite.CurrentSchemaVersion,
		ExportedAt:       time.Now().UTC().Format(time.RFC3339),
		CollectionFilter: f.collection,
		FileCount:        fileCount,
		ChunkCount:       chunkCount,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		writeErr = err
		return fmt.Errorf("export: marshal manifest: %w", err)
	}
	if err := addBytesToTar(tarWriter, "manifest.json", manifestBytes); err != nil {
		writeErr = err
		return fmt.Errorf("export: add manifest: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		writeErr = err
		return fmt.Errorf("export: close tar: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		writeErr = err
		return fmt.Errorf("export: close gzip: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"exported: %s (%d files, %d chunks)\n", destPath, fileCount, chunkCount)
	return nil
}

// addMarkdownFilesToTar walks walkRoot and adds each .md file to the tarball
// with a relative path of markdown/{collection}/{filename}.
func addMarkdownFilesToTar(tw *tar.Writer, walkRoot, vaultRoot string) error {
	if _, err := os.Stat(walkRoot); os.IsNotExist(err) {
		// Vault root does not exist yet — nothing to export.
		return nil
	}
	return filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		// Compute the relative path inside the tarball.
		rel, err := filepath.Rel(vaultRoot, path)
		if err != nil {
			return fmt.Errorf("addMarkdownFilesToTar: rel path for %q: %w", path, err)
		}
		tarPath := "markdown/" + filepath.ToSlash(rel)

		return addFileToTar(tw, path, tarPath)
	})
}

// addFileToTar adds a regular file to the tarball under tarPath.
func addFileToTar(tw *tar.Writer, filePath, tarPath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("addFileToTar: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("addFileToTar: stat %q: %w", filePath, err)
	}

	hdr := &tar.Header{
		Name:     tarPath,
		Mode:     0o600,
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("addFileToTar: write header %q: %w", tarPath, err)
	}
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("addFileToTar: copy %q: %w", tarPath, err)
	}
	return nil
}

// addBytesToTar adds an in-memory byte slice as tarPath inside the tarball.
func addBytesToTar(tw *tar.Writer, tarPath string, data []byte) error {
	hdr := &tar.Header{
		Name:     tarPath,
		Mode:     0o600,
		Size:     int64(len(data)),
		ModTime:  time.Now().UTC(),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("addBytesToTar: write header %q: %w", tarPath, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("addBytesToTar: write data %q: %w", tarPath, err)
	}
	return nil
}

// countFilesAndChunks returns the total file and chunk counts for the export
// scope (optionally filtered by collection).
func countFilesAndChunks(ctx context.Context, store *sqlite.Store, collection string) (fileCount, chunkCount int, err error) {
	db := store.DB()
	if collection == "" {
		err = db.QueryRowContext(ctx, `SELECT count(*) FROM files`).Scan(&fileCount)
		if err != nil {
			return 0, 0, fmt.Errorf("countFilesAndChunks: count files: %w", err)
		}
		err = db.QueryRowContext(ctx, `SELECT count(*) FROM chunks`).Scan(&chunkCount)
		if err != nil {
			return 0, 0, fmt.Errorf("countFilesAndChunks: count chunks: %w", err)
		}
	} else {
		err = db.QueryRowContext(ctx,
			`SELECT count(*) FROM files WHERE collection = ?`, collection).Scan(&fileCount)
		if err != nil {
			return 0, 0, fmt.Errorf("countFilesAndChunks: count files by collection: %w", err)
		}
		err = db.QueryRowContext(ctx,
			`SELECT count(*) FROM chunks c JOIN files f ON f.file_id = c.file_id WHERE f.collection = ?`,
			collection).Scan(&chunkCount)
		if err != nil {
			return 0, 0, fmt.Errorf("countFilesAndChunks: count chunks by collection: %w", err)
		}
	}
	return fileCount, chunkCount, nil
}
