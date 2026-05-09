// Package scheduler — persistence for ritual schedule config. SPEC-GOOSE-SCHEDULER-001 P1.
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SchedulePersister is the persistence interface for SchedulerConfig.
// Implementations must be safe for concurrent use by a single goroutine at a time
// (the Scheduler itself serialises Save/Load calls via its own lifecycle lock).
type SchedulePersister interface {
	// Save persists the given SchedulerConfig.
	Save(ctx context.Context, cfg SchedulerConfig) error
	// Load retrieves the previously persisted SchedulerConfig.
	Load(ctx context.Context) (SchedulerConfig, error)
}

// FilePersister stores SchedulerConfig as JSON on the local filesystem.
// Writes are atomic: data is written to a .tmp file then renamed to Path.
type FilePersister struct {
	// Path is the absolute path to the schedule JSON file.
	Path string
}

// NewFilePersister constructs a FilePersister rooted at homeDir.
// When homeDir is empty the user's home directory is derived from os.UserHomeDir().
// The default file path is <homeDir>/.goose/ritual/schedule.json.
func NewFilePersister(homeDir string) *FilePersister {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
	}
	return &FilePersister{
		Path: filepath.Join(homeDir, ".goose", "ritual", "schedule.json"),
	}
}

// Save atomically persists cfg to disk.
// It creates the parent directory if absent.
func (p *FilePersister) Save(_ context.Context, cfg SchedulerConfig) error {
	dir := filepath.Dir(p.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("scheduler persist: mkdir %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("scheduler persist: marshal: %w", err)
	}

	tmp := p.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("scheduler persist: write tmp %q: %w", tmp, err)
	}

	if err := os.Rename(tmp, p.Path); err != nil {
		return fmt.Errorf("scheduler persist: rename %q -> %q: %w", tmp, p.Path, err)
	}
	return nil
}

// Load reads and unmarshals the persisted SchedulerConfig.
// Returns an error if the file does not exist or cannot be decoded.
func (p *FilePersister) Load(_ context.Context) (SchedulerConfig, error) {
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return SchedulerConfig{}, fmt.Errorf("scheduler persist: read %q: %w", p.Path, err)
	}

	var cfg SchedulerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return SchedulerConfig{}, fmt.Errorf("scheduler persist: unmarshal: %w", err)
	}
	return cfg, nil
}
