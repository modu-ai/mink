package telegram_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryKeyring_StoreAndRetrieve verifies basic store/retrieve round-trip.
func TestMemoryKeyring_StoreAndRetrieve(t *testing.T) {
	kr := telegram.NewMemoryKeyring()
	err := kr.Store("svc", "key1", []byte("secret"))
	require.NoError(t, err)

	got, err := kr.Retrieve("svc", "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("secret"), got)
}

// TestMemoryKeyring_NotFound verifies that retrieving a missing key returns an error.
func TestMemoryKeyring_NotFound(t *testing.T) {
	kr := telegram.NewMemoryKeyring()
	_, err := kr.Retrieve("svc", "missing")
	require.Error(t, err)
}

// TestMemoryKeyring_Overwrite verifies that storing the same key twice overwrites.
func TestMemoryKeyring_Overwrite(t *testing.T) {
	kr := telegram.NewMemoryKeyring()
	require.NoError(t, kr.Store("svc", "key", []byte("v1")))
	require.NoError(t, kr.Store("svc", "key", []byte("v2")))

	got, err := kr.Retrieve("svc", "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), got)
}

// TestMemoryKeyring_Isolation verifies that different service+key pairs are isolated.
func TestMemoryKeyring_Isolation(t *testing.T) {
	kr := telegram.NewMemoryKeyring()
	require.NoError(t, kr.Store("svc-a", "key", []byte("alpha")))
	require.NoError(t, kr.Store("svc-b", "key", []byte("beta")))

	a, err := kr.Retrieve("svc-a", "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("alpha"), a)

	b, err := kr.Retrieve("svc-b", "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("beta"), b)
}
