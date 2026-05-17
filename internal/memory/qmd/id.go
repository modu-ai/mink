// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package qmd

import (
	"crypto/sha256"
	"fmt"
)

// ChunkID returns a stable 16-hex-character (8-byte) identifier for a chunk.
//
// The identifier is derived as:
//
//	hex(sha256("{sourcePath}:{startLine}:{endLine}:{contentHash}:{modelVersion}"))[:16]
//
// Including modelVersion in the derivation means that changing the embedding
// model automatically marks all previously derived chunk IDs as stale
// (REQ-MEM-005).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.4
// REQ:  REQ-MEM-005
func ChunkID(sourcePath string, startLine, endLine int, contentHash, modelVersion string) string {
	raw := fmt.Sprintf("%s:%d:%d:%s:%s", sourcePath, startLine, endLine, contentHash, modelVersion)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:8]) // 8 bytes → 16 hex chars
}
