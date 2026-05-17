// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build cgo

package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaVersionSeededOnOpen(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.sqlite"))
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	var version string
	err = s.db.QueryRowContext(context.Background(),
		`SELECT value FROM metadata WHERE key = 'schema_version'`).Scan(&version)
	require.NoError(t, err, "schema_version must be seeded by Open → MigrateSchema")
	assert.Equal(t, CurrentSchemaVersion, version)
}

func TestSchemaVersionIdempotentOnDoubleOpen(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	s1, err := Open(dbPath)
	require.NoError(t, err)
	_ = s1.Close()

	// Second open must not error even with an existing schema_version row.
	s2, err := Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = s2.Close() }()

	var version string
	require.NoError(t, s2.db.QueryRowContext(context.Background(),
		`SELECT value FROM metadata WHERE key = 'schema_version'`).Scan(&version))
	assert.Equal(t, CurrentSchemaVersion, version)
}
