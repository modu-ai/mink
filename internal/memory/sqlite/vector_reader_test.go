// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import (
	"context"
	"encoding/binary"
	"math"
	"path/filepath"
	"testing"

	"github.com/modu-ai/mink/internal/memory/retrieval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoVec0 skips the test when the sqlite-vec extension is unavailable.
func skipIfNoVec0(t *testing.T, store *Store) {
	t.Helper()
	if !store.HasVec0() {
		t.Skip("sqlite-vec (vec0) extension not available; skipping vector search test")
	}
}

func TestVectorReaderAdapter_unavailable(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.sqlite"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// If vec0 is available, skip this test (it tests the unavailable path).
	if store.HasVec0() {
		t.Skip("vec0 is available; this test is for the unavailable path")
	}

	adapter := NewVectorReaderAdapter(store)
	_, err = adapter.SearchVector(context.Background(), make([]float32, 1024), "", 5)
	require.Error(t, err)
	assert.ErrorIs(t, err, retrieval.ErrVec0Unavailable)
}

func TestVectorReaderAdapter_available(t *testing.T) {
	if !CGOEnabled {
		t.Skip("sqlite package requires cgo")
	}

	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "vec.sqlite"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	skipIfNoVec0(t, store)

	// With an empty embeddings table, vector search should return no results.
	adapter := NewVectorReaderAdapter(store)
	hits, err := adapter.SearchVector(context.Background(), make([]float32, 1024), "", 5)
	require.NoError(t, err)
	assert.Empty(t, hits)
}

func TestPackFloat32LE(t *testing.T) {
	input := []float32{1.0, -0.5, 0.0, 3.14}
	blob := packFloat32LE(input)

	require.Equal(t, len(input)*4, len(blob), "blob length must be 4*len(input)")

	for i, expected := range input {
		bits := binary.LittleEndian.Uint32(blob[i*4:])
		got := math.Float32frombits(bits)
		assert.InDelta(t, float64(expected), float64(got), 1e-7,
			"element %d round-trip failed", i)
	}
}

func TestPackFloat32LE_empty(t *testing.T) {
	blob := packFloat32LE(nil)
	assert.Empty(t, blob)
}
