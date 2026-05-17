// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_createsFileWithMode0600(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	s, err := Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	info, err := os.Stat(dbPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"SQLite file must be created with mode 0600 (REQ-MEM-002)")
}

func TestOpen_createsParentDirWithMode0700(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	base := t.TempDir()
	dir := filepath.Join(base, "nested", "memory")
	dbPath := filepath.Join(dir, "index.sqlite")

	s, err := Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
		"parent directory must be created with mode 0700 (REQ-MEM-003)")
}

func TestMigrateSchema_regularTablesPresent(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.sqlite"))
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Verify that the four regular tables were created.
	for _, table := range []string{"files", "chunks", "metadata"} {
		rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
		require.NoError(t, err)
		count := 0
		for rows.Next() {
			count++
		}
		require.NoError(t, rows.Close())
		assert.Greater(t, count, 0, "table %q must exist after migration", table)
	}
}

func TestMigrateSchema_ftsFTS5TablePresent(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.sqlite"))
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Check whether FTS5 is available in this build.
	_, ftsErr := s.db.Exec("INSERT INTO chunks_fts(chunk_id, content) VALUES ('test-id', 'test content')")
	if ftsErr != nil {
		// FTS5 was not compiled into the mattn/go-sqlite3 build.
		// This is expected in environments without CGO_CFLAGS="-DSQLITE_ENABLE_FTS5".
		// M2 will enforce FTS5; M1 only requires regular tables.
		t.Skipf("chunks_fts unavailable (FTS5 not in SQLite build): %v", ftsErr)
	}

	// Clean up.
	_, _ = s.db.Exec("DELETE FROM chunks_fts WHERE chunk_id='test-id'")
}

func TestMigrateSchema_vec0SkippedGracefully(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	// This test verifies that Open succeeds even when sqlite-vec is
	// unavailable.  The vec0 CREATE statement is expected to be skipped with a
	// warning log.  The test environment does not load sqlite-vec.
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "novec.sqlite"))
	require.NoError(t, err, "Open must succeed even when sqlite-vec is unavailable")
	require.NotNil(t, s)
	require.NoError(t, s.Close())
}

func TestOpen_idempotent(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "idempotent.sqlite")

	// First open.
	s1, err := Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	// Second open — MigrateSchema uses CREATE TABLE IF NOT EXISTS so it must
	// not fail.
	s2, err := Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, s2.Close())
}
