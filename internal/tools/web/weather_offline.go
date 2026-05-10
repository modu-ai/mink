package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// diskOfflineStore implements OfflineStore using atomic file writes.
// Files are stored as JSON under baseDir with names derived from provider,
// latitude, and longitude (rounded to 2 decimal places).
//
// @MX:ANCHOR: [AUTO] diskOfflineStore — disk-based weather fallback store
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-005(d/e) — fan_in >= 3 (weather_current, tests, offline_test)
type diskOfflineStore struct {
	baseDir string
}

// NewDiskOfflineStore returns an OfflineStore that persists files under baseDir.
// The directory is created with 0700 permissions on the first write.
func NewDiskOfflineStore(baseDir string) OfflineStore {
	return &diskOfflineStore{baseDir: baseDir}
}

// offlinePath derives the canonical file path for a given provider + coordinates.
// Latitude and longitude are rounded to 2 decimal places (~1.1 km grid) for
// cache-key normalization that is consistent with the bbolt cache key policy.
func (s *diskOfflineStore) offlinePath(provider string, lat, lon float64) string {
	// Round to 2 decimal places: %.2f gives "37.57", "-122.42", etc.
	name := fmt.Sprintf("latest-%s-%.2f-%.2f.json", provider, lat, lon)
	return filepath.Join(s.baseDir, name)
}

// SaveLatest persists report to the provider+coordinates slot using an atomic
// write (temp file in same directory + rename) with 0600 permissions.
// The base directory is created if absent.
func (s *diskOfflineStore) SaveLatest(provider string, lat, lon float64, report *WeatherReport) error {
	if err := os.MkdirAll(s.baseDir, 0700); err != nil {
		return fmt.Errorf("offline store: create base dir: %w", err)
	}

	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("offline store: marshal report: %w", err)
	}

	target := s.offlinePath(provider, lat, lon)

	// Write to a temp file in the same directory to guarantee atomic rename.
	tmp, err := os.CreateTemp(s.baseDir, ".weather-tmp-*")
	if err != nil {
		return fmt.Errorf("offline store: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Ensure cleanup on any error path.
	if _, writeErr := tmp.Write(data); writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("offline store: write temp file: %w", writeErr)
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("offline store: chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("offline store: close temp file: %w", err)
	}

	// Atomic rename: on POSIX systems this is guaranteed to be atomic.
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("offline store: rename temp to target: %w", err)
	}
	return nil
}

// LoadLatest retrieves the last saved WeatherReport for provider+coordinates.
// Returns ErrNoFallbackAvailable when:
//   - the file does not exist
//   - the file contains corrupt/invalid JSON (evicts the file in this case)
func (s *diskOfflineStore) LoadLatest(provider string, lat, lon float64) (*WeatherReport, error) {
	path := s.offlinePath(provider, lat, lon)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoFallbackAvailable
		}
		return nil, fmt.Errorf("offline store: read file: %w", err)
	}

	var report WeatherReport
	if err := json.Unmarshal(data, &report); err != nil {
		// Corrupt JSON: evict the file and signal no fallback.
		_ = os.Remove(path)
		return nil, ErrNoFallbackAvailable
	}

	return &report, nil
}
