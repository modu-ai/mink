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
	"os"
	"path/filepath"
	"testing"

	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeImport overrides paths and runs `mink memory import`.
func executeImport(t *testing.T, indexPath, vaultPath string, args ...string) (string, error) {
	t.Helper()
	prevIdx := importIndexPathOverride
	prevVault := importVaultPathOverride
	importIndexPathOverride = indexPath
	importVaultPathOverride = vaultPath
	t.Cleanup(func() {
		importIndexPathOverride = prevIdx
		importVaultPathOverride = prevVault
	})

	root := &cobra.Command{Use: "mink", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewMemoryCommand())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(append([]string{"memory"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// buildTestTarball creates a .tar.gz at destPath with given manifest and
// markdown file entries.
func buildTestTarball(t *testing.T, destPath string, manifest exportManifest, files map[string][]byte) {
	t.Helper()
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	// Add manifest.json.
	manifestData, err := json.Marshal(manifest)
	require.NoError(t, err)
	hdr := &tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(manifestData)), Typeflag: tar.TypeReg}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err = tw.Write(manifestData)
	require.NoError(t, err)

	// Add markdown files.
	for path, content := range files {
		hdr := &tar.Header{
			Name:     path,
			Mode:     0o600,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err = tw.Write(content)
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())
}

func TestImport_roundTrip(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	vaultRoot := filepath.Join(dir, "vault")
	indexPath := filepath.Join(dir, "main.sqlite")

	// Open the store to seed schema.
	store, err := sqlite.Open(indexPath)
	require.NoError(t, err)
	_ = store.Close()

	// Build a tarball with two markdown files.
	tarPath := filepath.Join(dir, "export.tar.gz")
	buildTestTarball(t, tarPath, exportManifest{
		SchemaVersion: sqlite.CurrentSchemaVersion,
		ExportedAt:    "2026-05-17T00:00:00Z",
		FileCount:     2,
		ChunkCount:    4,
	}, map[string][]byte{
		"markdown/journal/note1.md": []byte("# Note 1\n\nContent 1."),
		"markdown/custom/note2.md":  []byte("# Note 2\n\nContent 2."),
	})

	out, err := executeImport(t, indexPath, vaultRoot, "import", "--source", tarPath)
	require.NoError(t, err)
	assert.Contains(t, out, "imported:")

	// Verify files landed in the vault.
	assert.FileExists(t, filepath.Join(vaultRoot, "journal", "note1.md"))
	assert.FileExists(t, filepath.Join(vaultRoot, "custom", "note2.md"))
}

func TestImport_schemaMismatchRejectsWithNonZeroExit(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	vaultRoot := filepath.Join(dir, "vault")
	indexPath := filepath.Join(dir, "main.sqlite")

	store, err := sqlite.Open(indexPath)
	require.NoError(t, err)
	_ = store.Close()

	tarPath := filepath.Join(dir, "incompatible.tar.gz")
	buildTestTarball(t, tarPath, exportManifest{
		SchemaVersion: "999", // incompatible version
		ExportedAt:    "2026-05-17T00:00:00Z",
	}, nil)

	_, err = executeImport(t, indexPath, vaultRoot, "import", "--source", tarPath)
	require.Error(t, err, "schema mismatch should return an error (AC-MEM-038)")
	assert.Contains(t, err.Error(), "schema_version mismatch")
}

func TestImport_conflictSourceWinsBacksUpExisting(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	vaultRoot := filepath.Join(dir, "vault")
	indexPath := filepath.Join(dir, "main.sqlite")

	store, err := sqlite.Open(indexPath)
	require.NoError(t, err)
	_ = store.Close()

	// Pre-create a conflicting file in the vault.
	journalDir := filepath.Join(vaultRoot, "journal")
	require.NoError(t, os.MkdirAll(journalDir, 0o700))
	existingPath := filepath.Join(journalDir, "note.md")
	require.NoError(t, os.WriteFile(existingPath, []byte("# Old Content"), 0o600))

	// Import a tarball with different content for the same file.
	tarPath := filepath.Join(dir, "export.tar.gz")
	newContent := []byte("# New Content\n\nNew data.")
	buildTestTarball(t, tarPath, exportManifest{
		SchemaVersion: sqlite.CurrentSchemaVersion,
		ExportedAt:    "2026-05-17T00:00:00Z",
	}, map[string][]byte{
		"markdown/journal/note.md": newContent,
	})

	out, err := executeImport(t, indexPath, vaultRoot, "import", "--source", tarPath, "--strategy", "source-wins")
	require.NoError(t, err)
	assert.Contains(t, out, "conflict-resolved:")

	// The vault file should now contain new content.
	got, err := os.ReadFile(existingPath)
	require.NoError(t, err)
	newSum := sha256.Sum256(newContent)
	gotSum := sha256.Sum256(got)
	assert.Equal(t, newSum, gotSum, "vault file must contain source content after source-wins")

	// A backup file should exist.
	entries, err := os.ReadDir(journalDir)
	require.NoError(t, err)
	hasBak := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == "" && e.Name() != "note.md" {
			hasBak = true
		}
		// Backup name contains ".bak."
		if len(e.Name()) > 4 && e.Name() != "note.md" {
			hasBak = true
		}
	}
	assert.True(t, hasBak, "a backup file must exist after source-wins conflict resolution")
}

func TestImport_conflictSkipConflictsPreservesOriginal(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	vaultRoot := filepath.Join(dir, "vault")
	indexPath := filepath.Join(dir, "main.sqlite")

	store, err := sqlite.Open(indexPath)
	require.NoError(t, err)
	_ = store.Close()

	journalDir := filepath.Join(vaultRoot, "journal")
	require.NoError(t, os.MkdirAll(journalDir, 0o700))
	existingPath := filepath.Join(journalDir, "note.md")
	origContent := []byte("# Original")
	require.NoError(t, os.WriteFile(existingPath, origContent, 0o600))

	tarPath := filepath.Join(dir, "export.tar.gz")
	buildTestTarball(t, tarPath, exportManifest{
		SchemaVersion: sqlite.CurrentSchemaVersion,
		ExportedAt:    "2026-05-17T00:00:00Z",
	}, map[string][]byte{
		"markdown/journal/note.md": []byte("# Different Content"),
	})

	out, err := executeImport(t, indexPath, vaultRoot, "import", "--source", tarPath, "--strategy", "skip-conflicts")
	require.NoError(t, err)
	assert.Contains(t, out, "skipped:")

	// Original file must be preserved.
	got, err := os.ReadFile(existingPath)
	require.NoError(t, err)
	origSum := sha256.Sum256(origContent)
	gotSum := sha256.Sum256(got)
	assert.Equal(t, origSum, gotSum, "original file must not be overwritten with skip-conflicts strategy")
}

func TestImport_missingManifestRejectsWithError(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	vaultRoot := filepath.Join(dir, "vault")
	indexPath := filepath.Join(dir, "main.sqlite")

	store, err := sqlite.Open(indexPath)
	require.NoError(t, err)
	_ = store.Close()

	// Build a tarball WITHOUT a manifest.json.
	tarPath := filepath.Join(dir, "no-manifest.tar.gz")
	f, err := os.OpenFile(tarPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	require.NoError(t, err)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	// Add only a dummy file.
	hdr := &tar.Header{Name: "markdown/custom/note.md", Mode: 0o600, Size: 5, Typeflag: tar.TypeReg}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err = tw.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())
	_ = f.Close()

	_, err = executeImport(t, indexPath, vaultRoot, "import", "--source", tarPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest")
}
