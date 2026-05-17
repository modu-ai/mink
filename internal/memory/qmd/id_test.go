// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package qmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkID_deterministic(t *testing.T) {
	id1 := ChunkID("/vault/journal/note.md", 1, 20, "abc123", "v1-no-embed")
	id2 := ChunkID("/vault/journal/note.md", 1, 20, "abc123", "v1-no-embed")
	assert.Equal(t, id1, id2, "identical inputs must produce identical ID")
}

func TestChunkID_length(t *testing.T) {
	id := ChunkID("/vault/test.md", 1, 10, "deadbeef", "v1-no-embed")
	assert.Len(t, id, 16, "chunk ID must be exactly 16 hex characters")
}

func TestChunkID_modelVersionChange(t *testing.T) {
	id1 := ChunkID("/vault/test.md", 1, 10, "abc123", "v1-no-embed")
	id2 := ChunkID("/vault/test.md", 1, 10, "abc123", "v2-nomic-embed-text")
	assert.NotEqual(t, id1, id2, "different modelVersion must produce different ID")
}

func TestChunkID_sourcePathChange(t *testing.T) {
	id1 := ChunkID("/vault/a.md", 1, 10, "abc123", "v1-no-embed")
	id2 := ChunkID("/vault/b.md", 1, 10, "abc123", "v1-no-embed")
	assert.NotEqual(t, id1, id2, "different sourcePath must produce different ID")
}

func TestChunkID_lineRangeChange(t *testing.T) {
	id1 := ChunkID("/vault/test.md", 1, 10, "abc123", "v1-no-embed")
	id2 := ChunkID("/vault/test.md", 2, 10, "abc123", "v1-no-embed")
	assert.NotEqual(t, id1, id2, "different startLine must produce different ID")
}

func TestChunkID_contentHashChange(t *testing.T) {
	id1 := ChunkID("/vault/test.md", 1, 10, "hash-a", "v1-no-embed")
	id2 := ChunkID("/vault/test.md", 1, 10, "hash-b", "v1-no-embed")
	assert.NotEqual(t, id1, id2, "different contentHash must produce different ID")
}
