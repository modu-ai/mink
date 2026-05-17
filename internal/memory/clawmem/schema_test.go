// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package clawmem_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/modu-ai/mink/internal/memory/clawmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectSchemaVersion_v1_0 verifies that a vault containing a ".schema-version"
// file with content "v1.0" is detected as SchemaV1_0.
func TestDetectSchemaVersion_v1_0(t *testing.T) {
	vaultDir := t.TempDir()
	writeSchemaFile(t, vaultDir, "v1.0")

	ver, err := clawmem.DetectSchemaVersion(vaultDir)
	require.NoError(t, err)
	assert.Equal(t, clawmem.SchemaV1_0, ver)
}

// TestDetectSchemaVersion_v1_1 verifies that "v1.1" is detected as SchemaV1_1.
func TestDetectSchemaVersion_v1_1(t *testing.T) {
	vaultDir := t.TempDir()
	writeSchemaFile(t, vaultDir, "v1.1")

	ver, err := clawmem.DetectSchemaVersion(vaultDir)
	require.NoError(t, err)
	assert.Equal(t, clawmem.SchemaV1_1, ver)
}

// TestDetectSchemaVersion_unknown verifies that an unrecognised version string
// ("v9.9") is returned as SchemaUnknown.
func TestDetectSchemaVersion_unknown(t *testing.T) {
	vaultDir := t.TempDir()
	writeSchemaFile(t, vaultDir, "v9.9")

	ver, err := clawmem.DetectSchemaVersion(vaultDir)
	require.NoError(t, err)
	assert.Equal(t, clawmem.SchemaUnknown, ver)
}

// TestDetectSchemaVersion_missing verifies that the absence of a ".schema-version"
// file causes the function to return SchemaV1_0 (treat as fresh vault).
func TestDetectSchemaVersion_missing(t *testing.T) {
	vaultDir := t.TempDir()
	// No .schema-version file written.

	ver, err := clawmem.DetectSchemaVersion(vaultDir)
	require.NoError(t, err)
	assert.Equal(t, clawmem.SchemaV1_0, ver, "missing .schema-version should default to v1.0")
}

// TestIsSupportedVersion verifies that supported and unsupported versions are
// classified correctly.
func TestIsSupportedVersion(t *testing.T) {
	tests := []struct {
		ver  clawmem.SchemaVersion
		want bool
	}{
		{clawmem.SchemaV1_0, true},
		{clawmem.SchemaV1_1, true},
		{clawmem.SchemaUnknown, false},
	}
	for _, tc := range tests {
		got := clawmem.IsSupportedVersion(tc.ver)
		assert.Equal(t, tc.want, got, "IsSupportedVersion(%v)", tc.ver)
	}
}

// writeSchemaFile is a test helper that writes content to
// {vaultDir}/.schema-version with mode 0600.
func writeSchemaFile(t *testing.T, vaultDir, content string) {
	t.Helper()
	path := filepath.Join(vaultDir, ".schema-version")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
