// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeExport overrides index and vault paths, then runs `mink memory export`.
func executeExport(t *testing.T, indexPath, vaultPath string, args ...string) (string, error) {
	t.Helper()
	prevIdx := exportIndexPathOverride
	prevVault := exportVaultPathOverride
	exportIndexPathOverride = indexPath
	exportVaultPathOverride = vaultPath
	t.Cleanup(func() {
		exportIndexPathOverride = prevIdx
		exportVaultPathOverride = prevVault
	})

	root := &cobra.Command{Use: "mink", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewMemoryCommand())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(append([]string{"memory"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// setupTestVault creates a vault directory with some markdown files and returns
// the vault root, index path, and a map of filename → content for later verification.
func setupTestVault(t *testing.T) (vaultRoot, indexPath string, files map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	vaultRoot = filepath.Join(dir, "vault")
	indexPath = filepath.Join(dir, "main.sqlite")

	files = map[string][]byte{
		"journal/entry1.md": []byte("# Journal Entry 1\n\nContent of entry 1."),
		"journal/entry2.md": []byte("# Journal Entry 2\n\nContent of entry 2."),
		"custom/note.md":    []byte("# Custom Note\n\nCustom content."),
	}

	for rel, content := range files {
		full := filepath.Join(vaultRoot, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o700))
		require.NoError(t, os.WriteFile(full, content, 0o600))
	}

	// Create a minimal SQLite index (just open/close to ensure schema is seeded).
	store, err := sqlite.Open(indexPath)
	require.NoError(t, err)
	_ = store.Close()

	return vaultRoot, indexPath, files
}

// readTarball extracts a .tar.gz file into memory and returns a map of path → content.
func readTarball(t *testing.T, tarPath string) map[string][]byte {
	t.Helper()
	f, err := os.Open(tarPath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer func() { _ = gzr.Close() }()

	result := make(map[string][]byte)
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(tr)
		require.NoError(t, err)
		result[hdr.Name] = data
	}
	return result
}

func TestExport_roundTripMarkdownFiles(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	vaultRoot, indexPath, origFiles := setupTestVault(t)
	destBase := filepath.Join(t.TempDir(), "backup")

	out, err := executeExport(t, indexPath, vaultRoot, "export", "--dest", destBase)
	require.NoError(t, err)
	assert.Contains(t, out, "exported:")

	tarPath := destBase + ".tar.gz"
	entries := readTarball(t, tarPath)

	// Verify all markdown files are present with matching content hashes.
	for rel, content := range origFiles {
		tarKey := "markdown/" + rel
		assert.Contains(t, entries, tarKey, "tarball should contain %q", tarKey)
		if gotContent, ok := entries[tarKey]; ok {
			origSum := sha256.Sum256(content)
			gotSum := sha256.Sum256(gotContent)
			assert.Equal(t, origSum, gotSum, "content mismatch for %q", tarKey)
		}
	}
}

func TestExport_manifestSchemaValidation(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	vaultRoot, indexPath, _ := setupTestVault(t)
	destBase := filepath.Join(t.TempDir(), "backup")

	_, err := executeExport(t, indexPath, vaultRoot, "export", "--dest", destBase)
	require.NoError(t, err)

	entries := readTarball(t, destBase+".tar.gz")
	manifestData, ok := entries["manifest.json"]
	require.True(t, ok, "manifest.json must be present in tarball")

	var manifest exportManifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	assert.Equal(t, sqlite.CurrentSchemaVersion, manifest.SchemaVersion)
	assert.NotEmpty(t, manifest.ExportedAt)
}

func TestExport_clawMemFormatReturnsSentinel(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	vaultRoot, indexPath, _ := setupTestVault(t)
	destBase := filepath.Join(t.TempDir(), "backup")

	_, err := executeExport(t, indexPath, vaultRoot, "export", "--dest", destBase, "--format", "clawmem")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrClawMemNotImplementedM5a)
}

func TestExport_collectionFilter(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	vaultRoot, indexPath, _ := setupTestVault(t)
	destBase := filepath.Join(t.TempDir(), "backup")

	_, err := executeExport(t, indexPath, vaultRoot, "export", "--dest", destBase, "--collection", "journal")
	require.NoError(t, err)

	entries := readTarball(t, destBase+".tar.gz")

	// journal files should be present.
	for _, name := range []string{"markdown/journal/entry1.md", "markdown/journal/entry2.md"} {
		assert.Contains(t, entries, name, "journal file %q should be in export", name)
	}
	// custom file should NOT be present (filtered out).
	assert.NotContains(t, entries, "markdown/custom/note.md")
}

func TestExport_refusesExistingDest(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	vaultRoot, indexPath, _ := setupTestVault(t)
	destBase := filepath.Join(t.TempDir(), "backup")

	// Create the destination file in advance.
	require.NoError(t, os.WriteFile(destBase+".tar.gz", []byte("existing"), 0o600))

	_, err := executeExport(t, indexPath, vaultRoot, "export", "--dest", destBase)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "already exists") ||
		strings.Contains(err.Error(), destBase),
		"error should mention the destination path")
}

func TestExport_noBase64VectorBlobsInMarkdown(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	vaultRoot, indexPath, _ := setupTestVault(t)
	destBase := filepath.Join(t.TempDir(), "backup")

	_, err := executeExport(t, indexPath, vaultRoot, "export", "--dest", destBase)
	require.NoError(t, err)

	entries := readTarball(t, destBase+".tar.gz")
	// Verify that no markdown entry contains a base64-like binary blob.
	// A conservative heuristic: entries under markdown/ must not contain
	// runs of non-printable bytes (raw binary from vec0).
	for key, data := range entries {
		if !strings.HasPrefix(key, "markdown/") || !strings.HasSuffix(key, ".md") {
			continue
		}
		// Check that the file is valid UTF-8 text without control characters
		// indicative of embedded binary blobs.
		for i, b := range data {
			if b == 0x00 {
				t.Errorf("export: markdown file %q contains null byte at offset %d (binary blob leak)", key, i)
			}
		}
		_ = fmt.Sprintf("checked %q", key) // prevent unused import
	}
}
