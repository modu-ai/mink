// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package clawmem_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/modu-ai/mink/internal/memory/clawmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// newObservedMirror builds a Mirror backed by an in-memory zap observer so
// that tests can assert on log entries without writing to stderr.
func newObservedMirror(t *testing.T, cfg clawmem.Config) (*clawmem.Mirror, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	m, err := clawmem.NewMirror(cfg, logger)
	require.NoError(t, err)
	return m, logs
}

// TestMirror_happyPath verifies that when clawmem_compat is enabled a markdown
// file is written to {vault_path}/{collection}/{filename}.md after Write.
// AC-MEM-024.
func TestMirror_happyPath(t *testing.T) {
	vaultDir := t.TempDir()
	logger := zaptest.NewLogger(t)

	m, err := clawmem.NewMirror(clawmem.Config{
		Enabled:   true,
		VaultPath: vaultDir,
	}, logger)
	require.NoError(t, err)

	content := []byte("# Hello\nThis is a test note.\n")
	req := clawmem.WriteRequest{
		Collection: "journal",
		Filename:   "note.md",
		Content:    content,
	}
	require.NoError(t, m.Write(req))

	// Verify the file exists at the expected path.
	dst := filepath.Join(vaultDir, "journal", "note.md")
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got, "mirror file content must match the original")
}

// TestMirror_disabled verifies that no file is written when Enabled=false.
// AC-MEM-024 (negative path).
func TestMirror_disabled(t *testing.T) {
	vaultDir := t.TempDir()
	logger := zaptest.NewLogger(t)

	m, err := clawmem.NewMirror(clawmem.Config{
		Enabled:   false,
		VaultPath: vaultDir,
	}, logger)
	require.NoError(t, err)

	req := clawmem.WriteRequest{
		Collection: "journal",
		Filename:   "note.md",
		Content:    []byte("# Test\n"),
	}
	require.NoError(t, m.Write(req))

	// The collection directory should not have been created.
	dst := filepath.Join(vaultDir, "journal", "note.md")
	_, err = os.Stat(dst)
	assert.True(t, os.IsNotExist(err), "mirror file must not exist when Enabled=false")
}

// TestMirror_writeFailureNonBlocking verifies that a write error (vault path
// is a file, not a dir) does not propagate to the caller — it logs a warn and
// returns nil.
func TestMirror_writeFailureNonBlocking(t *testing.T) {
	// Create a file where the vault directory should be so MkdirAll fails.
	notADir := t.TempDir()
	blocker := filepath.Join(notADir, "journal")
	require.NoError(t, os.WriteFile(blocker, []byte("I am a file"), 0o600))

	m, logs := newObservedMirror(t, clawmem.Config{
		Enabled:   true,
		VaultPath: notADir,
	})

	req := clawmem.WriteRequest{
		Collection: "journal",
		Filename:   "note.md",
		Content:    []byte("# content\n"),
	}
	// The primary insert must not fail — best-effort only.
	err := m.Write(req)
	assert.NoError(t, err, "mirror write failure must not block the primary insert")

	// Exactly one warn-level log entry for op=clawmem_mirror.
	warnEntries := logs.FilterField(zap.String("op", "clawmem_mirror")).All()
	assert.NotEmpty(t, warnEntries, "expected at least one warn log for clawmem_mirror op")
}

// TestMirror_schemaV1_0 verifies that mirror.Write proceeds normally when the
// vault contains a .schema-version file with "v1.0".
func TestMirror_schemaV1_0(t *testing.T) {
	vaultDir := t.TempDir()
	writeSchemaFile(t, vaultDir, "v1.0")

	logger := zaptest.NewLogger(t)
	m, err := clawmem.NewMirror(clawmem.Config{
		Enabled:   true,
		VaultPath: vaultDir,
	}, logger)
	require.NoError(t, err)

	content := []byte("# Schema v1.0 test\n")
	req := clawmem.WriteRequest{
		Collection: "sessions",
		Filename:   "session.md",
		Content:    content,
	}
	require.NoError(t, m.Write(req))

	dst := filepath.Join(vaultDir, "sessions", "session.md")
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

// TestMirror_schemaUnknownReadOnly verifies that when the detected schema
// version is unknown (e.g. "v9.9"), the mirror skips writes and logs an info
// event exactly once across multiple Write calls.  AC-MEM-026.
func TestMirror_schemaUnknownReadOnly(t *testing.T) {
	vaultDir := t.TempDir()
	writeSchemaFile(t, vaultDir, "v9.9")

	m, logs := newObservedMirror(t, clawmem.Config{
		Enabled:   true,
		VaultPath: vaultDir,
	})

	req := clawmem.WriteRequest{
		Collection: "journal",
		Filename:   "note.md",
		Content:    []byte("# Ignored\n"),
	}

	// Call Write multiple times — the info log must fire exactly once.
	for range 3 {
		require.NoError(t, m.Write(req))
	}

	// No file should have been written.
	dst := filepath.Join(vaultDir, "journal", "note.md")
	_, err := os.Stat(dst)
	assert.True(t, os.IsNotExist(err), "no file written in read-only mode")

	// Info log must fire exactly once (not three times).
	infoEntries := logs.FilterField(zap.String("op", "clawmem_schema")).
		FilterField(zap.String("mode", "read_only")).All()
	assert.Equal(t, 1, len(infoEntries),
		"info log for clawmem_schema read_only must fire exactly once")
}

// TestMirror_schemaMissingAssumeV1_0 verifies that when no .schema-version
// file exists, the mirror treats the vault as v1.0 and writes normally.
func TestMirror_schemaMissingAssumeV1_0(t *testing.T) {
	vaultDir := t.TempDir()
	// No .schema-version file.

	logger := zaptest.NewLogger(t)
	m, err := clawmem.NewMirror(clawmem.Config{
		Enabled:   true,
		VaultPath: vaultDir,
	}, logger)
	require.NoError(t, err)

	content := []byte("# Fresh vault test\n")
	req := clawmem.WriteRequest{
		Collection: "briefing",
		Filename:   "brief.md",
		Content:    content,
	}
	require.NoError(t, m.Write(req))

	dst := filepath.Join(vaultDir, "briefing", "brief.md")
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

// TestMirror_homeDirExpansion verifies that a VaultPath starting with "~/" is
// expanded to the real home directory rather than written literally.
// Uses a temp dir sub-path to avoid touching the real ~/.clawmem during tests.
func TestMirror_homeDirExpansion(t *testing.T) {
	// Set HOME to a temp dir so os.UserHomeDir() returns something safe.
	// t.Setenv automatically restores the previous value on test cleanup.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	logger := zaptest.NewLogger(t)
	m, err := clawmem.NewMirror(clawmem.Config{
		Enabled:   true,
		VaultPath: "~/.clawmem/vault",
	}, logger)
	require.NoError(t, err)

	content := []byte("# Expand test\n")
	req := clawmem.WriteRequest{
		Collection: "journal",
		Filename:   "expand.md",
		Content:    content,
	}
	require.NoError(t, m.Write(req))

	// Verify the file landed under tmpHome, not under "~".
	dst := filepath.Join(tmpHome, ".clawmem", "vault", "journal", "expand.md")
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got, "VaultPath '~/...' must expand via os.UserHomeDir()")
}

// TestMirror_concurrentWrites verifies that 10 goroutines writing different
// files all succeed and the race detector reports no issues.
func TestMirror_concurrentWrites(t *testing.T) {
	vaultDir := t.TempDir()
	logger := zaptest.NewLogger(t)

	m, err := clawmem.NewMirror(clawmem.Config{
		Enabled:   true,
		VaultPath: vaultDir,
	}, logger)
	require.NoError(t, err)

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := clawmem.WriteRequest{
				Collection: "sessions",
				Filename: filepath.Join("session", filepath.FromSlash(
					// Unique filename per goroutine.
					"session_"+string(rune('a'+idx))+".md",
				)),
				Content: []byte("# concurrent note\n"),
			}
			errs[idx] = m.Write(req)
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "goroutine %d should not fail", i)
	}
}
