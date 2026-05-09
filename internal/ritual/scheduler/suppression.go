// Package scheduler — fired-key suppression store. SPEC-GOOSE-SCHEDULER-001 P4b T-025.
package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FiredKeyStore tracks the (event, userLocalDate, TZ) 3-tuples that have
// already been dispatched. Implementations must be safe for concurrent use.
//
// @MX:NOTE: [AUTO] Suppression seam — JSON impl ships with the package; future
//
//	implementations may persist via MEMORY-001 dispatcher.
//
// @MX:SPEC: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-013, REQ-SCHED-022
type FiredKeyStore interface {
	// Has reports whether key has been recorded.
	Has(key string) bool
	// LastFiredAt returns the recorded timestamp for key. The zero time means
	// the key is unknown.
	LastFiredAt(key string) time.Time
	// Mark records key with firedAt as the most recent dispatch time.
	// Implementations persist atomically.
	Mark(key string, firedAt time.Time) error
}

// BuildFiredKey returns the canonical 3-tuple suppression key:
// "{event}:{userLocalDate}:{TZ}". TZ-aware semantics — a TZ change produces
// a fresh key on the same calendar date (REQ-SCHED-013, research.md §6.1).
func BuildFiredKey(event, userLocalDate, tz string) string {
	return fmt.Sprintf("%s:%s:%s", event, userLocalDate, tz)
}

// JSONFiredKeyStore is a file-backed FiredKeyStore. It loads on construction
// and persists every Mark call atomically (write to .tmp, rename).
type JSONFiredKeyStore struct {
	path string

	mu      sync.RWMutex
	entries map[string]time.Time
}

// NewJSONFiredKeyStore constructs a JSONFiredKeyStore rooted at path. If the
// file is missing, an empty store is returned. Read errors yield an empty
// store (the file is treated as a soft cache).
func NewJSONFiredKeyStore(path string) *JSONFiredKeyStore {
	s := &JSONFiredKeyStore{
		path:    path,
		entries: make(map[string]time.Time),
	}
	_ = s.load()
	return s
}

func (s *JSONFiredKeyStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var raw map[string]time.Time
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = raw
	if s.entries == nil {
		s.entries = make(map[string]time.Time)
	}
	return nil
}

// Has implements FiredKeyStore.
func (s *JSONFiredKeyStore) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.entries[key]
	return ok
}

// LastFiredAt implements FiredKeyStore.
func (s *JSONFiredKeyStore) LastFiredAt(key string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entries[key]
}

// Mark implements FiredKeyStore. Writes the entire map atomically.
func (s *JSONFiredKeyStore) Mark(key string, firedAt time.Time) error {
	s.mu.Lock()
	s.entries[key] = firedAt
	snapshot := make(map[string]time.Time, len(s.entries))
	for k, v := range s.entries {
		snapshot[k] = v
	}
	s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("scheduler suppression: mkdir %q: %w", dir, err)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("scheduler suppression: marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("scheduler suppression: write %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("scheduler suppression: rename: %w", err)
	}
	return nil
}

// noopFiredKeyStore is the default zero-value store used when no
// FiredKeyStore is wired. It records nothing, so suppression and missed-event
// replay are effectively disabled.
type noopFiredKeyStore struct{}

func (noopFiredKeyStore) Has(string) bool              { return false }
func (noopFiredKeyStore) LastFiredAt(string) time.Time { return time.Time{} }
func (noopFiredKeyStore) Mark(string, time.Time) error { return nil }
