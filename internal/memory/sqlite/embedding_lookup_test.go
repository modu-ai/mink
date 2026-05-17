// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build cgo

package sqlite

import (
	"context"
	"encoding/binary"
	"math"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// packFloat32LEForTest encodes a []float32 using little-endian IEEE 754.
// Mirrors packFloat32LE which is private to the package.
func packFloat32LEForTest(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}

func openEmbeddingLookupTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "embedding_lookup_test.sqlite")
	store, err := Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestUnpackFloat32LE_roundtrip(t *testing.T) {
	original := []float32{1.0, 2.5, -0.5, 0.0, 3.14}
	blob := packFloat32LEForTest(original)
	got, err := unpackFloat32LE(blob)
	require.NoError(t, err)
	require.Len(t, got, len(original))
	for i, want := range original {
		assert.InDelta(t, want, got[i], 1e-6, "index %d mismatch", i)
	}
}

func TestUnpackFloat32LE_invalidLength(t *testing.T) {
	_, err := unpackFloat32LE([]byte{0x01, 0x02, 0x03}) // 3 bytes — not multiple of 4
	assert.Error(t, err, "non-multiple-of-4 blob must return error")
}

func TestUnpackFloat32LE_emptyBlob(t *testing.T) {
	got, err := unpackFloat32LE(nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestEmbeddingLookupStore_emptyInput(t *testing.T) {
	store := openEmbeddingLookupTestStore(t)
	adapter := NewEmbeddingLookupStore(store)

	result, err := adapter.LookupEmbeddings(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestEmbeddingLookupStore_noVec0(t *testing.T) {
	// When vec0 is unavailable, LookupEmbeddings must return empty map, no error.
	store := openEmbeddingLookupTestStore(t)
	if store.HasVec0() {
		t.Skip("vec0 available; this test requires a build without sqlite-vec")
	}

	adapter := NewEmbeddingLookupStore(store)
	result, err := adapter.LookupEmbeddings(context.Background(), []string{"chunk-1", "chunk-2"})
	require.NoError(t, err)
	assert.Empty(t, result, "no-vec0 path must return empty map")
}

func TestEmbeddingLookupStore_missingChunksOmitted(t *testing.T) {
	store := openEmbeddingLookupTestStore(t)
	if !store.HasVec0() {
		t.Skip("vec0 not available; skipping embedding lookup test")
	}

	adapter := NewEmbeddingLookupStore(store)
	// Query for chunk IDs that do not exist in the embeddings table.
	result, err := adapter.LookupEmbeddings(context.Background(), []string{"nonexistent-1", "nonexistent-2"})
	require.NoError(t, err)
	assert.Empty(t, result, "missing chunk IDs must be silently omitted")
}
