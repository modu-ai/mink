// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build cgo

// Package sqlite provides the SQLite-backed storage layer for MINK's QMD
// memory subsystem.  The package requires cgo; the build tag ensures a clear
// compile-time error on unsupported configurations.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	// mattn/go-sqlite3 registers the "sqlite3" driver via its init function.
	// This is the sole permitted cgo dependency for the memory subsystem.
	// REQ-MEM-011: cgo usage limited to sqlite-vec + mattn/go-sqlite3 stack.
	_ "github.com/mattn/go-sqlite3"
)

// ErrNoCGO is returned by Open when the package was built without cgo support.
// Users must set CGO_ENABLED=1 and recompile.
var ErrNoCGO = errors.New("internal/memory/sqlite requires cgo (set CGO_ENABLED=1 and rebuild)")

// Store wraps a *sql.DB opened against a SQLite file.
// All write operations go through Writer to ensure single-writer semantics.
type Store struct {
	db      *sql.DB
	path    string
	hasVec0 bool // cached result from HasVec0 probe; set during Open
}

// Open opens (or creates) the SQLite index file at path and runs schema
// migrations.
//
// Security guarantees (REQ-MEM-002, REQ-MEM-003, REQ-MEM-029):
//   - The parent directory is created with mode 0700 if it does not exist.
//   - The database file is created with mode 0600.
//   - After opening, the file mode is enforced to 0600.
//
// M3: Open now attempts to load the sqlite-vec extension.  If the extension
// is unavailable, Open still succeeds; HasVec0 will return false and vsearch
// falls back to BM25 (REQ-MEM-019).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.5, T3.3
// REQ:  REQ-MEM-002, REQ-MEM-003, REQ-MEM-011, REQ-MEM-019, REQ-MEM-029
func Open(path string) (*Store, error) {
	// Ensure parent directory exists with mode 0700.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("sqlite.Open: create parent dir %q: %w", dir, err)
	}

	// Open the SQLite file via the mink driver that attempts to load sqlite-vec.
	// The _foreign_keys pragma enables cascade deletes.
	dsn := fmt.Sprintf("%s?_foreign_keys=on", path)

	extPath := findVec0Extension()
	db, v0ok, err := openWithVec0(dsn, extPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite.Open: open %q: %w", path, err)
	}

	if extPath == "" {
		log.Printf("sqlite.Open: sqlite-vec extension not found; vsearch will fall back to BM25 (REQ-MEM-019)")
	} else if !v0ok {
		log.Printf("sqlite.Open: sqlite-vec extension found at %q but vec0 table is not queryable; vsearch will fall back to BM25", extPath)
	}

	// Enforce single-writer mode at the SQLite level (WAL is fine for readers).
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite.Open: set WAL mode: %w", err)
	}

	// Enforce mode 0600 on the file (create + existing).
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite.Open: chmod 0600 %q: %w", path, err)
	}

	s := &Store{db: db, path: path, hasVec0: v0ok}

	ctx := context.Background()
	if err := s.MigrateSchema(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite.Open: migrate schema: %w", err)
	}

	// Run the vec0 1024d migration when vec0 is available.
	if v0ok {
		runVec0Migration(db)
		// Re-probe after migration since the table was recreated.
		s.hasVec0 = probeVec0(db)
	}

	return s, nil
}

// HasVec0 reports whether the sqlite-vec extension is loaded and the embeddings
// virtual table is queryable.  The result is cached at Open time.
//
// When HasVec0 returns false, vector search (vsearch) is unavailable and the
// caller must fall back to BM25 (REQ-MEM-019).
func (s *Store) HasVec0() bool {
	return s.hasVec0
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for read queries.  Callers must not hold
// the returned handle across a Store.Close call.
func (s *Store) DB() *sql.DB {
	return s.db
}

// MigrateSchema executes the embedded Schema DDL against the open database.
//
// Execution strategy:
//   - Split Schema on ";" to obtain individual statements.
//   - Execute each statement independently.
//   - If a statement references "vec0" (the sqlite-vec virtual table) and
//     fails because the extension is not loaded, log a warning and continue.
//     All regular tables and the chunks_fts FTS5 table must succeed.
//
// @MX:WARN: [AUTO] Schema migration executes DDL statements in a loop; vec0
// statement failure is silently skipped.
// @MX:REASON: sqlite-vec extension is optional; graceful degrade is
// required by SPEC §8 Risk R7 and REQ-MEM-011.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.5
// REQ:  REQ-MEM-011
func (s *Store) MigrateSchema(ctx context.Context) error {
	statements := splitSQL(Schema)
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, trimmed); err != nil {
			lower := strings.ToLower(trimmed)
			// Gracefully skip vec0 failures when sqlite-vec is unavailable.
			if strings.Contains(lower, "vec0") {
				log.Printf("sqlite.MigrateSchema: vec0 unavailable (sqlite-vec not loaded); skipping: %v", err)
				continue
			}
			// Gracefully skip FTS5 failures when the SQLite build does not
			// include the fts5 module.  FTS5 is required for M2 BM25 search;
			// M1 only needs the regular tables.
			if strings.Contains(lower, "fts5") {
				log.Printf("sqlite.MigrateSchema: fts5 unavailable (SQLite built without SQLITE_ENABLE_FTS5); skipping: %v", err)
				continue
			}
			return fmt.Errorf("sqlite.MigrateSchema: execute DDL: %w", err)
		}
	}

	// Seed schema_version into the metadata table (idempotent INSERT OR IGNORE).
	// Called here so every Open is guaranteed to have the row present.
	if err := s.seedSchemaVersion(ctx); err != nil {
		return fmt.Errorf("sqlite.MigrateSchema: seed schema version: %w", err)
	}

	return nil
}

// splitSQL splits a SQL script into individual statements on ";".
// It trims whitespace and skips comment-only lines.
func splitSQL(script string) []string {
	parts := strings.Split(script, ";")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		// Skip pure-comment blocks.
		lines := strings.Split(trimmed, "\n")
		allComment := true
		for _, line := range lines {
			l := strings.TrimSpace(line)
			if l != "" && !strings.HasPrefix(l, "--") {
				allComment = false
				break
			}
		}
		if !allComment {
			result = append(result, trimmed)
		}
	}
	return result
}
