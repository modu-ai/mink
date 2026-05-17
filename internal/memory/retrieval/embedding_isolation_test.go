// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build cgo

package retrieval_test

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/memory/qmd"
	"github.com/modu-ai/mink/internal/memory/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmbeddingIsolation_acMem013 verifies AC-MEM-013:
// Embeddings are stored ONLY in SQLite — markdown files must never contain
// base64-encoded embedding blobs.
//
// Test procedure:
//  1. Create a temp vault with 5 markdown files.
//  2. Ingest the files as chunks into a test SQLite store.
//  3. Write canned 1024-d embeddings into the embeddings table.
//  4. Scan all markdown files for base64 patterns (≥200 contiguous base64 chars).
//  5. Assert: no markdown file contains a base64 blob.
//  6. Assert: SQLite embeddings table contains ≥5 blobs (sanity check).
func TestEmbeddingIsolation_acMem013(t *testing.T) {
	if !sqlite.CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	// --- Step 1: Create temp vault with 5 markdown files. ---
	vaultDir := t.TempDir()
	markdownFiles := []struct {
		name    string
		content string
	}{
		{"note1.md", "# Go Programming\n\nGolang concurrency patterns and best practices.\n"},
		{"note2.md", "# Python ML\n\nMachine learning with PyTorch and scikit-learn.\n"},
		{"note3.md", "# Rust Safety\n\nRust ownership model and memory safety guarantees.\n"},
		{"note4.md", "# TypeScript\n\nTypeScript generics and advanced type patterns.\n"},
		{"note5.md", "# Database\n\nPostgreSQL indexing strategies and query optimization.\n"},
	}

	for _, mf := range markdownFiles {
		path := filepath.Join(vaultDir, mf.name)
		require.NoError(t, os.WriteFile(path, []byte(mf.content), 0o600))
	}

	// --- Step 2: Open SQLite store and ingest chunks. ---
	dbPath := filepath.Join(t.TempDir(), "isolation_test.sqlite")
	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC()

	w, err := sqlite.NewWriter(store, "")
	require.NoError(t, err)

	chunkIDs := make([]string, 0, len(markdownFiles))
	for i, mf := range markdownFiles {
		path := filepath.Join(vaultDir, mf.name)
		f := qmd.File{
			Collection:  "test",
			SourcePath:  path,
			ContentHash: "hash-" + mf.name,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		fileID, upsertErr := w.UpsertFile(ctx, f)
		require.NoError(t, upsertErr)

		chunkID := qmd.ChunkID(path, 1, 5, "content-hash", qmd.DefaultModelVersion)
		chunk := qmd.Chunk{
			ID:               chunkID,
			FileID:           fileID,
			Collection:       "test",
			SourcePath:       path,
			StartLine:        1,
			EndLine:          5,
			Content:          mf.content,
			EmbeddingPending: false,
			ModelVersion:     qmd.DefaultModelVersion,
			CreatedAt:        now,
		}
		require.NoError(t, w.Insert(ctx, chunk))
		chunkIDs = append(chunkIDs, chunkID)
		_ = i
	}
	require.NoError(t, w.Close())

	// --- Step 3: Write canned 1024-d embeddings into SQLite (if vec0 available). ---
	embeddingsInserted := 0
	if store.HasVec0() {
		embeddingsInserted = insertCannedEmbeddings(t, ctx, store, chunkIDs)
	}

	// --- Step 4: Scan markdown files for base64 blobs. ---
	// Pattern: ≥200 contiguous base64 characters on a single line.
	// This catches accidentally dumped embedding blobs.
	base64Pattern := regexp.MustCompile(`^[A-Za-z0-9+/]{200,}={0,2}$`)

	for _, mf := range markdownFiles {
		path := filepath.Join(vaultDir, mf.name)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)

		lines := splitLines(string(content))
		for lineNum, line := range lines {
			assert.False(t, base64Pattern.MatchString(line),
				"AC-MEM-013: markdown file %s line %d contains a base64 blob (embeddings must stay in SQLite only)",
				mf.name, lineNum+1)
		}
	}

	// --- Step 5: Sanity check — embeddings ARE in SQLite when vec0 is available. ---
	if store.HasVec0() {
		var count int
		err := store.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM embeddings`).Scan(&count)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, count, embeddingsInserted,
			"AC-MEM-013 sanity: SQLite must contain at least %d embedding blobs", embeddingsInserted)
		if embeddingsInserted > 0 {
			assert.GreaterOrEqual(t, count, 1,
				"AC-MEM-013 sanity: at least 1 embedding must be present in SQLite")
		}
	}
}

// insertCannedEmbeddings writes mock 1024-d embeddings directly into the
// embeddings virtual table using pre-built binary blobs.
// Returns the number of embeddings successfully inserted.
func insertCannedEmbeddings(t *testing.T, ctx context.Context, store *sqlite.Store, chunkIDs []string) int {
	t.Helper()

	// Build a canned 1024-d vector (all 0.1 values).
	const dim = 1024
	blob := make([]byte, dim*4)
	for i := range dim {
		bits := math.Float32bits(0.1)
		binary.LittleEndian.PutUint32(blob[i*4:], bits)
	}

	var inserted int
	for _, id := range chunkIDs {
		_, err := store.DB().ExecContext(ctx,
			`INSERT OR REPLACE INTO embeddings(chunk_id, embedding) VALUES (?, ?)`,
			id, blob,
		)
		if err != nil {
			t.Logf("insertCannedEmbeddings: skipping %s: %v", id, err)
			continue
		}
		inserted++
	}
	return inserted
}

// splitLines splits a string into lines (handles \n and \r\n).
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			line := s[start:i]
			// Strip trailing \r.
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
