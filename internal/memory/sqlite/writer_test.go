// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/clawmem"
	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestWriter_idempotentInsert(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	s := openTestStore(t)
	ctx := context.Background()

	w, err := NewWriter(s, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	now := time.Now().UTC()
	f := qmd.File{
		Collection:  "journal",
		SourcePath:  "/vault/journal/note.md",
		ContentHash: "deadbeef",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	fileID, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)
	assert.Greater(t, fileID, int64(0))

	chunk := qmd.Chunk{
		ID:               "testchunk01234567",
		FileID:           fileID,
		Collection:       "journal",
		SourcePath:       f.SourcePath,
		StartLine:        1,
		EndLine:          10,
		Content:          "Hello, world.",
		EmbeddingPending: true,
		ModelVersion:     qmd.DefaultModelVersion,
		CreatedAt:        now,
	}

	// First insert.
	require.NoError(t, w.Insert(ctx, chunk))

	// Second insert (same chunk_id) must not error — idempotent upsert.
	require.NoError(t, w.Insert(ctx, chunk))

	// Verify exactly one row in chunks.
	var count int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chunks WHERE chunk_id = ?", chunk.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestWriter_concurrentWritersSerialize(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	// Two goroutines race to acquire the writer lock.  The first succeeds; the
	// second must wait (or time out with ErrWriterBusy).
	s := openTestStore(t)

	var wg sync.WaitGroup
	results := make([]error, 2)

	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w, err := NewWriter(s, "")
			if err != nil {
				results[idx] = err
				return
			}
			// Hold the lock briefly.
			time.Sleep(20 * time.Millisecond)
			_ = w.Close()
		}(i)
	}

	wg.Wait()

	// At most one goroutine should have received ErrWriterBusy; both
	// succeeding is also valid if they acquired the lock sequentially.
	busyCount := 0
	for _, e := range results {
		if e == ErrWriterBusy {
			busyCount++
		}
	}
	assert.LessOrEqual(t, busyCount, 1, "at most one goroutine may be denied the lock")
}

func TestWriter_UpsertFile(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	s := openTestStore(t)
	ctx := context.Background()

	w, err := NewWriter(s, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	now := time.Now().UTC()
	f := qmd.File{
		Collection:  "briefing",
		SourcePath:  "/vault/briefing/2026-01-01.md",
		ContentHash: "cafebabe",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id1, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)
	assert.Greater(t, id1, int64(0))

	// Upsert with updated hash — must return the same file_id.
	f.ContentHash = "newHash"
	f.UpdatedAt = now.Add(time.Minute)
	id2, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "UpsertFile on same source_path must return same file_id")
}

// TestWriter_UpsertFile_mirrorEnabled verifies that when a ClawMem mirror is
// attached via WithMirror, a successful UpsertFile call copies the markdown
// source file to the configured vault path.
//
// AC-MEM-024: mirror write fires after primary SQLite commit.
func TestWriter_UpsertFile_mirrorEnabled(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	// Create a real source markdown file in a temp dir.
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	content := []byte("# Mirror integration test\n")
	srcPath := filepath.Join(srcDir, "note.md")
	require.NoError(t, os.WriteFile(srcPath, content, 0o600))

	// Build the mirror.
	logger := zaptest.NewLogger(t)
	m, err := clawmem.NewMirror(clawmem.Config{
		Enabled:   true,
		VaultPath: vaultDir,
	}, logger)
	require.NoError(t, err)

	// Open store and attach mirror.
	s := openTestStore(t)
	ctx := context.Background()

	w, err := NewWriter(s, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()
	w.WithMirror(m, logger)

	now := time.Now().UTC()
	f := qmd.File{
		Collection:  "journal",
		SourcePath:  srcPath,
		ContentHash: "abc123",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err = w.UpsertFile(ctx, f)
	require.NoError(t, err)

	// Verify the mirror file was written.
	mirrorPath := filepath.Join(vaultDir, "journal", "note.md")
	got, err := os.ReadFile(mirrorPath)
	require.NoError(t, err)
	assert.Equal(t, content, got, "mirror file content must match source")
}

// TestWriter_UpsertFile_mirrorDisabled verifies that when no mirror is
// attached, UpsertFile succeeds without attempting any filesystem writes
// outside the SQLite store.
func TestWriter_UpsertFile_mirrorDisabled(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	s := openTestStore(t)
	ctx := context.Background()

	w, err := NewWriter(s, "")
	require.NoError(t, err)
	defer func() { _ = w.Close() }()
	// No WithMirror call — mirror is nil.

	now := time.Now().UTC()
	f := qmd.File{
		Collection:  "journal",
		SourcePath:  "/nonexistent/path/note.md",
		ContentHash: "deadbeef",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	// Must succeed even though SourcePath does not exist on disk.
	id, err := w.UpsertFile(ctx, f)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))
}
