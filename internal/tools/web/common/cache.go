package common

import (
	"encoding/binary"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var cacheBucket = []byte("web_cache")

// Cache is a bbolt-backed TTL key-value store for web tool responses.
// Each entry stores the value bytes and an expiry timestamp.
// The clock function is injected for testability.
//
// @MX:ANCHOR: [AUTO] TTL cache for all web tool HTTP responses
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-007 — fan_in >= 2 (http_fetch + web_search)
type Cache struct {
	db    *bolt.DB
	clock func() time.Time
}

// cacheEntry is the binary layout stored in bbolt.
// Format: [8 bytes unix-nano expires_at][remaining bytes: value]
type cacheEntry struct {
	expiresAt int64 // Unix nanoseconds
	value     []byte
}

// marshal encodes entry to bytes.
func (e cacheEntry) marshal() []byte {
	b := make([]byte, 8+len(e.value))
	binary.BigEndian.PutUint64(b[:8], uint64(e.expiresAt))
	copy(b[8:], e.value)
	return b
}

// unmarshalEntry decodes raw bytes into a cacheEntry.
func unmarshalEntry(b []byte) (cacheEntry, error) {
	if len(b) < 8 {
		return cacheEntry{}, fmt.Errorf("cache: entry too short (%d bytes)", len(b))
	}
	ns := int64(binary.BigEndian.Uint64(b[:8]))
	return cacheEntry{expiresAt: ns, value: b[8:]}, nil
}

// OpenCache opens (or creates) a bbolt database at path and returns a Cache.
// clock is used to determine TTL expiry; use time.Now in production.
// The caller must call Close when done.
func OpenCache(path string, clock func() time.Time) (*Cache, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("cache: open bolt db at %s: %w", path, err)
	}
	// Ensure the bucket exists.
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(cacheBucket)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cache: create bucket: %w", err)
	}
	return &Cache{db: db, clock: clock}, nil
}

// Get retrieves the value for key.
// Returns (value, true, nil) on cache hit, (nil, false, nil) on miss or expiry.
// Expired entries are lazily deleted on read.
func (c *Cache) Get(key string) ([]byte, bool, error) {
	var result []byte
	var hit bool

	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(cacheBucket)
		if b == nil {
			return nil
		}
		raw := b.Get([]byte(key))
		if raw == nil {
			return nil
		}
		entry, err := unmarshalEntry(raw)
		if err != nil {
			// Corrupted entry — treat as miss.
			return b.Delete([]byte(key))
		}

		now := c.clock().UnixNano()
		// expires_at >= now means still valid: entry is NOT expired when
		// now == expires_at (contract.md: "expires_at > now check, not >=").
		// Expiry condition: now > expires_at (strictly after).
		if entry.expiresAt >= now {
			result = make([]byte, len(entry.value))
			copy(result, entry.value)
			hit = true
		} else {
			// Lazy eviction of expired entry.
			return b.Delete([]byte(key))
		}
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("cache: get %q: %w", key, err)
	}
	return result, hit, nil
}

// Set stores value under key with the given TTL.
// An existing entry for key is overwritten.
func (c *Cache) Set(key string, value []byte, ttl time.Duration) error {
	expiresAt := c.clock().Add(ttl).UnixNano()
	entry := cacheEntry{expiresAt: expiresAt, value: value}

	return c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(cacheBucket)
		if b == nil {
			return fmt.Errorf("cache: bucket missing")
		}
		return b.Put([]byte(key), entry.marshal())
	})
}

// SetRaw stores raw bytes directly under key, bypassing the TTL encoding.
// Intended only for testing corrupted-entry scenarios.
func (c *Cache) SetRaw(key string, raw []byte) error {
	return c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(cacheBucket)
		if b == nil {
			return fmt.Errorf("cache: bucket missing")
		}
		return b.Put([]byte(key), raw)
	})
}

// Close releases the underlying bbolt database file lock.
func (c *Cache) Close() error {
	return c.db.Close()
}
