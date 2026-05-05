// Package permission provides persistent storage for tool permission decisions.
package permission

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissionStore_AtomicWrite_FlockSafe verifies atomic writes and concurrent safety.
// RED: tests that Store correctly persists and reads permission decisions.
func TestPermissionStore_AtomicWrite_FlockSafe(t *testing.T) {
	t.Run("save and load single decision", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "permissions.json")

		store := NewStore(path)

		err := store.Save("Bash", DecisionAllowAlways)
		require.NoError(t, err)

		decisions, err := store.Load()
		require.NoError(t, err)

		assert.Equal(t, DecisionAllowAlways, decisions["Bash"])
	})

	t.Run("has returns correct decision", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "permissions.json")

		store := NewStore(path)
		err := store.Save("Bash", DecisionAllowAlways)
		require.NoError(t, err)

		decision, ok := store.Has("Bash")
		assert.True(t, ok)
		assert.Equal(t, DecisionAllowAlways, decision)

		_, ok = store.Has("FileWrite")
		assert.False(t, ok)
	})

	t.Run("schema version is 1", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "permissions.json")

		store := NewStore(path)
		err := store.Save("Bash", DecisionAllowAlways)
		require.NoError(t, err)

		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &raw))

		version, ok := raw["version"].(float64)
		require.True(t, ok, "version field must be present")
		assert.Equal(t, float64(1), version)

		tools, ok := raw["tools"].(map[string]interface{})
		require.True(t, ok, "tools field must be present")
		assert.Equal(t, "allow", tools["Bash"])
	})

	t.Run("atomic write uses tmp then rename", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "permissions.json")

		store := NewStore(path)

		// Write multiple times; each write should be atomic
		for i := 0; i < 5; i++ {
			err := store.Save("Bash", DecisionAllowAlways)
			require.NoError(t, err)
		}

		// Tmp file should not remain after save
		tmpPath := path + ".tmp"
		_, err := os.Stat(tmpPath)
		assert.True(t, os.IsNotExist(err), "tmp file should not remain after save")
	})

	t.Run("concurrent saves are race-safe", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "permissions.json")
		store := NewStore(path)

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = store.Save("Bash", DecisionAllowAlways)
			}()
		}
		wg.Wait()

		// Final state should be valid
		decisions, err := store.Load()
		require.NoError(t, err)
		assert.Equal(t, DecisionAllowAlways, decisions["Bash"])
	})

	t.Run("load from nonexistent file returns empty map", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent.json")

		store := NewStore(path)
		decisions, err := store.Load()
		require.NoError(t, err)
		assert.Empty(t, decisions)
	})

	t.Run("deny always is persisted correctly", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "permissions.json")

		store := NewStore(path)
		err := store.Save("FileWrite", DecisionDenyAlways)
		require.NoError(t, err)

		decision, ok := store.Has("FileWrite")
		assert.True(t, ok)
		assert.Equal(t, DecisionDenyAlways, decision)
	})

	t.Run("parent directories created automatically", func(t *testing.T) {
		dir := t.TempDir()
		nestedPath := filepath.Join(dir, "a", "b", "c", "permissions.json")

		store := NewStore(nestedPath)
		err := store.Save("Bash", DecisionAllowAlways)
		require.NoError(t, err)

		_, err = os.Stat(nestedPath)
		assert.NoError(t, err, "file should exist after save with nested path")
	})
}
