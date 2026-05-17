// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package clawmem provides optional mirror-write compatibility with ClawMem
// vaults.  When clawmem_compat.enabled is true in memory.yaml, every markdown
// file written to the MINK vault is also copied to the ClawMem vault path.
// The SQLite index is never mirrored — only the source markdown files.
//
// Mirror writes are best-effort: a failure logs a structured zap warning and
// does not block the primary write path.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T5.6, T5.7
// REQ:  AC-MEM-024, AC-MEM-026, AC-MEM-036
package clawmem

import (
	"os"
	"path/filepath"
	"strings"
)

// SchemaVersion represents a ClawMem vault schema version.
type SchemaVersion string

const (
	// SchemaV1_0 is the baseline ClawMem vault schema.
	SchemaV1_0 SchemaVersion = "v1.0"

	// SchemaV1_1 is a minor revision of the ClawMem vault schema that remains
	// compatible with the mirror writer.
	SchemaV1_1 SchemaVersion = "v1.1"

	// SchemaUnknown is returned when the schema version file contains a value
	// that this package does not recognise.  Mirror writes are disabled in
	// read-only mode when this is detected.
	SchemaUnknown SchemaVersion = "unknown"
)

// schemaVersionFile is the sentinel file path within a ClawMem vault that
// stores the schema version string.
const schemaVersionFile = ".schema-version"

// DetectSchemaVersion reads the .schema-version file inside vaultDir and
// returns the corresponding SchemaVersion constant.
//
// If the file does not exist, the vault is treated as a fresh ClawMem install
// and SchemaV1_0 is returned (no error).
//
// If the file exists but contains an unrecognised version, SchemaUnknown is
// returned (no error — the caller decides how to handle it).
func DetectSchemaVersion(vaultDir string) (SchemaVersion, error) {
	path := filepath.Join(vaultDir, schemaVersionFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fresh vault — assume baseline schema.
			return SchemaV1_0, nil
		}
		return SchemaUnknown, err
	}

	ver := SchemaVersion(strings.TrimSpace(string(raw)))
	switch ver {
	case SchemaV1_0, SchemaV1_1:
		return ver, nil
	default:
		return SchemaUnknown, nil
	}
}

// IsSupportedVersion reports whether the mirror writer supports writing to a
// vault with the given schema version.
//
// SchemaV1_0 and SchemaV1_1 are writable; SchemaUnknown triggers read-only
// mode.
func IsSupportedVersion(v SchemaVersion) bool {
	return v == SchemaV1_0 || v == SchemaV1_1
}
