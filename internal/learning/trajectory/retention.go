package trajectory

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Retention manages automatic deletion of old trajectory files.
type Retention struct {
	baseDir       string // trajectories/ root
	retentionDays int
	logger        *zap.Logger
}

// NewRetention creates a Retention sweeper.
func NewRetention(baseDir string, retentionDays int, logger *zap.Logger) *Retention {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	return &Retention{
		baseDir:       filepath.Join(baseDir, "trajectories"),
		retentionDays: retentionDays,
		logger:        logger,
	}
}

// Sweep deletes trajectory files older than retentionDays UTC calendar days.
// Files that cannot be parsed as a date are skipped silently.
// openFilePaths lists file paths currently held open by a Writer and must
// not be deleted (REQ-TRAJECTORY-009 open-file safety).
func (r *Retention) Sweep(openFilePaths ...string) error {
	openSet := make(map[string]bool, len(openFilePaths))
	for _, p := range openFilePaths {
		openSet[p] = true
	}

	cutoff := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -r.retentionDays)

	buckets := []string{dirSuccess, dirFailed}
	for _, bucket := range buckets {
		dir := filepath.Join(r.baseDir, bucket)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			r.logger.Warn("retention sweep readdir failed",
				zap.String("dir", dir), zap.Error(err))
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
				continue
			}

			fullPath := filepath.Join(dir, entry.Name())

			if openSet[fullPath] {
				continue
			}

			// Parse date from filename: YYYY-MM-DD[-N].jsonl
			fileDate := parseDateFromFilename(entry.Name())
			if fileDate.IsZero() {
				continue
			}

			// File is older than cutoff (strictly before cutoff day).
			if fileDate.Before(cutoff) {
				if err := os.Remove(fullPath); err != nil {
					r.logger.Warn("retention sweep remove failed",
						zap.String("path", fullPath), zap.Error(err))
				}
			}
		}
	}
	return nil
}

// parseDateFromFilename extracts the date from a filename like:
// "2026-04-21.jsonl" or "2026-04-21-1.jsonl".
// Returns zero time on parse failure.
func parseDateFromFilename(name string) time.Time {
	// Strip .jsonl suffix.
	base := strings.TrimSuffix(name, ".jsonl")
	// Take only the first 10 characters (YYYY-MM-DD).
	if len(base) < 10 {
		return time.Time{}
	}
	dateStr := base[:10]
	t, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
	if err != nil {
		return time.Time{}
	}
	return t
}
