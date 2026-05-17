// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build cgo

package sqlite

import (
	"context"
	"fmt"
)

// CurrentSchemaVersion is the canonical schema version for the QMD SQLite index.
// Stored in the metadata table under the key "schema_version" at first Open.
// Used by export/import to validate compatibility (AC-MEM-038).
//
// Bump this constant when the schema changes in a backward-incompatible way.
const CurrentSchemaVersion = "1"

// seedSchemaVersion inserts the schema_version row into the metadata table
// if it does not already exist.  This is called from MigrateSchema so that
// every Open is guaranteed to have the row present.
//
// The INSERT OR IGNORE pattern makes the call idempotent: repeated opens do
// not overwrite an existing version row.
func (s *Store) seedSchemaVersion(ctx context.Context) error {
	const q = `INSERT OR IGNORE INTO metadata(key, value) VALUES ('schema_version', ?)`
	if _, err := s.db.ExecContext(ctx, q, CurrentSchemaVersion); err != nil {
		return fmt.Errorf("sqlite.seedSchemaVersion: %w", err)
	}
	return nil
}
