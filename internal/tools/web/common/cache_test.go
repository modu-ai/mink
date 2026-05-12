package common_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCacheTTL verifies DC-07 / AC-WEB-007: cache hit returns cached data
// without an external fetch, and after clock-advance past TTL the same key
// produces a cache miss.
func TestCacheTTL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cache.db")

	now := time.Now()
	clock := func() time.Time { return now }

	c, err := common.OpenCache(dbPath, clock)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	key := "web_search:sha256:abc123"
	value := []byte(`{"result":"hello"}`)

	t.Run("Miss on empty cache", func(t *testing.T) {
		got, hit, err := c.Get(key)
		require.NoError(t, err)
		assert.False(t, hit)
		assert.Nil(t, got)
	})

	t.Run("Hit after Set", func(t *testing.T) {
		require.NoError(t, c.Set(key, value, 24*time.Hour))
		got, hit, err := c.Get(key)
		require.NoError(t, err)
		assert.True(t, hit)
		assert.Equal(t, value, got)
	})

	t.Run("BoundaryExact: expires_at == now is still a hit", func(t *testing.T) {
		// The entry expires_at is set to now + 24h when stored.
		// Advance clock to exactly expires_at — must still be a hit (> check).
		now = now.Add(24 * time.Hour)
		got, hit, err := c.Get(key)
		require.NoError(t, err)
		assert.True(t, hit, "entry at exact expires_at boundary should still be a hit")
		assert.Equal(t, value, got)
	})

	t.Run("TTL expired after 25h returns miss", func(t *testing.T) {
		// Advance 1 more second past expires_at to trigger expiry.
		now = now.Add(time.Second)
		got, hit, err := c.Get(key)
		require.NoError(t, err)
		assert.False(t, hit, "entry past TTL must be a miss")
		assert.Nil(t, got)
	})
}

// TestCacheIsolation verifies that two Cache instances with different directories
// do not share data (each test uses t.TempDir()).
func TestCacheIsolation(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	clock := func() time.Time { return time.Now() }

	c1, err := common.OpenCache(filepath.Join(dir1, "cache.db"), clock)
	require.NoError(t, err)
	defer func() { _ = c1.Close() }()

	c2, err := common.OpenCache(filepath.Join(dir2, "cache.db"), clock)
	require.NoError(t, err)
	defer func() { _ = c2.Close() }()

	key := "test:key"
	require.NoError(t, c1.Set(key, []byte("v1"), time.Hour))

	_, hit, err := c2.Get(key)
	require.NoError(t, err)
	assert.False(t, hit, "separate cache instances must not share data")
}

// TestCacheOpenInvalidPath verifies that OpenCache returns an error for an
// invalid path (e.g. a directory that does not exist under a read-only root).
func TestCacheOpenInvalidPath(t *testing.T) {
	_, err := common.OpenCache("/nonexistent/dir/cache.db", func() time.Time { return time.Now() })
	assert.Error(t, err)
}

// TestCacheCorruptedEntry verifies that a corrupted/short entry is treated as
// a miss rather than crashing.
func TestCacheCorruptedEntry(t *testing.T) {
	dir := t.TempDir()
	clock := func() time.Time { return time.Now() }
	c, err := common.OpenCache(filepath.Join(dir, "cache.db"), clock)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	// Insert raw bytes that are too short to be a valid entry (less than 8 bytes).
	require.NoError(t, c.SetRaw("corrupt_key", []byte{0x01, 0x02}))

	// Get should handle corruption gracefully and return a miss.
	got, hit, err := c.Get("corrupt_key")
	require.NoError(t, err)
	assert.False(t, hit)
	assert.Nil(t, got)
}

// TestCacheDeleteExpired verifies that Set overwrites an existing entry.
func TestCacheOverwrite(t *testing.T) {
	dir := t.TempDir()
	clock := func() time.Time { return time.Now() }

	c, err := common.OpenCache(filepath.Join(dir, "cache.db"), clock)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	key := "key"
	require.NoError(t, c.Set(key, []byte("original"), time.Hour))
	require.NoError(t, c.Set(key, []byte("updated"), time.Hour))

	got, hit, err := c.Get(key)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, []byte("updated"), got)
}
