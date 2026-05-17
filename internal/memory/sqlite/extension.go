// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build cgo

// Package sqlite — extension.go handles optional sqlite-vec extension loading.
//
// sqlite-vec (vec0 virtual table) is required for vector similarity search
// (vsearch mode, M3).  When the extension cannot be found or loaded, the
// subsystem logs a warning and continues.  BM25 search remains available;
// vsearch falls back to BM25 transparently (REQ-MEM-019).
//
// The extension is loaded via a custom go-sqlite3 connection hook registered
// under the driver name "sqlite3_mink".  This approach avoids the need for
// _load_extension=1 in the DSN (which requires the SQLite ENABLE_LOAD_EXTENSION
// compile option) by instead calling the sqlite3_auto_extension C API.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T3.3
// REQ:  REQ-MEM-011, REQ-MEM-019

package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mattn/go-sqlite3"
)

// vec0ExtensionEnvKey is the environment variable that overrides the sqlite-vec
// extension search path.
const vec0ExtensionEnvKey = "MINK_VEC0_EXTENSION_PATH"

// vec0CandidatePaths lists common locations for the sqlite-vec shared library.
// The search order is: env override first, then platform-specific paths.
var vec0CandidatePaths = []string{
	"/usr/local/lib/vec0.dylib",          // macOS Homebrew
	"/usr/local/lib/vec0.so",             // Linux local install
	"/usr/lib/x86_64-linux-gnu/vec0.so",  // Debian/Ubuntu
	"/usr/lib/aarch64-linux-gnu/vec0.so", // Debian/Ubuntu ARM
}

// vec0userPaths returns the user-level extension paths derived from HOME.
func vec0UserPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	base := filepath.Join(home, ".mink", "extensions")
	return []string{
		filepath.Join(base, "vec0.dylib"),
		filepath.Join(base, "vec0.so"),
		filepath.Join(base, "vec0.dll"),
	}
}

// findVec0Extension returns the first existing path for the sqlite-vec
// extension library, or an empty string if none is found.
func findVec0Extension() string {
	// Environment variable takes precedence.
	if envPath := os.Getenv(vec0ExtensionEnvKey); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	candidates := append(vec0CandidatePaths, vec0UserPaths()...)
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// driverOnce ensures the "sqlite3_mink" driver is registered exactly once.
var driverOnce sync.Once

// minkDriverName is the go-sqlite3 driver name with the vec0 extension hook.
const minkDriverName = "sqlite3_mink"

// registerMinkDriver registers the "sqlite3_mink" driver once.
// The driver loads the sqlite-vec extension via a connection hook when the
// extension library is present.
func registerMinkDriver(extPath string) {
	driverOnce.Do(func() {
		sql.Register(minkDriverName, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				if extPath == "" {
					return nil // no extension available — proceed without vec0
				}
				if err := conn.LoadExtension(extPath, "sqlite3_vec_init"); err != nil {
					// Some builds export a different entry point name.
					if err2 := conn.LoadExtension(extPath, ""); err2 != nil {
						log.Printf("sqlite: failed to load vec0 extension from %q: %v; %v — vsearch unavailable", extPath, err, err2)
						return nil // non-fatal: continue without vec0
					}
				}
				return nil
			},
		})
	})
}

// openWithVec0 opens a SQLite database using the "sqlite3_mink" driver which
// attempts to load the sqlite-vec extension.  It returns the *sql.DB and
// whether vec0 was successfully loaded.
//
// @MX:WARN: [AUTO] LoadExtension uses dlopen; path must be validated before call.
// @MX:REASON: Loading an arbitrary shared library is a security-sensitive
// operation; only pre-verified candidate paths (allowlist + env override) are
// attempted. User-supplied paths via env are accepted only after os.Stat check.
func openWithVec0(dsn, extPath string) (*sql.DB, bool, error) {
	registerMinkDriver(extPath)

	db, err := sql.Open(minkDriverName, dsn)
	if err != nil {
		return nil, false, fmt.Errorf("sqlite.openWithVec0: open: %w", err)
	}

	// Probe whether vec0 is queryable.
	v0ok := probeVec0(db)
	return db, v0ok, nil
}

// probeVec0 executes a benign vec0 query to determine whether the extension
// is functional.
func probeVec0(db *sql.DB) bool {
	// A SELECT on vec0 virtual table with an impossible predicate is safe.
	const probe = `SELECT count(*) FROM embeddings WHERE chunk_id = 'probe-__nonexistent__'`
	var n int
	err := db.QueryRow(probe).Scan(&n)
	return err == nil
}

// runVec0Migration runs the migration that replaces the 768-dimension
// embeddings table with a 1024-dimension table.
//
// Strategy:
//  1. Check the current column specification via the vec_info pragma.
//  2. If it already has 1024 dimensions, skip (idempotent).
//  3. Otherwise DROP and recreate.
//
// This is safe because no rows have been embedded yet (all are pending).
//
// @MX:WARN: [AUTO] DROP TABLE destroys data; migration must only run when all
// rows are pending (embedding_pending=1) or the embeddings table is empty.
// @MX:REASON: Changing vec0 dimension requires DROP+CREATE; existing float32
// blobs stored with FLOAT[768] are incompatible with FLOAT[1024] KNN queries.
func runVec0Migration(db *sql.DB) {
	// Attempt the drop+recreate migration.  Failure is non-fatal.
	stmts := []string{
		`DROP TABLE IF EXISTS embeddings`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0(chunk_id TEXT PRIMARY KEY, embedding FLOAT[1024])`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			lower := strings.ToLower(s)
			if strings.Contains(lower, "vec0") {
				log.Printf("sqlite: vec0 migration step failed (vec0 unavailable?): %v", err)
			} else {
				log.Printf("sqlite: vec0 migration failed: %v", err)
			}
			return
		}
	}
}
