package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// allowedExtensions is the whitelist of file extensions that may be downloaded
// from Telegram. Enforces path traversal / arbitrary file write protection
// (strategy-p3.md §C.2).
var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".pdf":  true,
	".txt":  true,
	".md":   true,
	".docx": true,
	".zip":  true,
}

// isAllowedExt reports whether ext (lowercased, dot-prefixed) is in the whitelist.
func isAllowedExt(ext string) bool {
	return allowedExtensions[strings.ToLower(ext)]
}

// inboxFilePath returns the canonical inbox path for a downloaded attachment.
// The filename is always "<message_id><ext>" to prevent path traversal.
func inboxFilePath(inboxDir string, messageID int64, ext string) string {
	return filepath.Join(inboxDir, fmt.Sprintf("%d%s", messageID, ext))
}

// Janitor periodically removes inbox files whose mtime exceeds ttl.
//
// A single goroutine runs Run(ctx) so resource usage is O(1) regardless of
// how many files are downloaded (strategy-p3.md §C.3 option b).
//
// @MX:WARN: [AUTO] Janitor.Run is a long-lived goroutine driven by a ticker.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 REQ-MTGM-E06; per strategy-p3.md §C.3,
// mtime-based sweep requires a persistent goroutine to survive daemon restarts.
type Janitor struct {
	inboxDir  string
	ttl       time.Duration // files older than ttl are removed; default 30 minutes
	tickEvery time.Duration // sweep interval; default 1 minute
	logger    *zap.Logger
}

// NewJanitor creates a Janitor for the given inbox directory.
// ttl and tickEvery default to 30 minutes and 1 minute respectively.
func NewJanitor(inboxDir string, logger *zap.Logger) *Janitor {
	return &Janitor{
		inboxDir:  inboxDir,
		ttl:       30 * time.Minute,
		tickEvery: 1 * time.Minute,
		logger:    logger,
	}
}

// Run starts the sweep loop and blocks until ctx is cancelled.
func (j *Janitor) Run(ctx context.Context) error {
	ticker := time.NewTicker(j.tickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			j.sweepOnce(t)
		}
	}
}

// sweepOnce removes all files in inboxDir whose mtime is older than ttl
// relative to now.
func (j *Janitor) sweepOnce(now time.Time) {
	entries, err := os.ReadDir(j.inboxDir)
	if err != nil {
		if j.logger != nil {
			j.logger.Warn("janitor: failed to read inbox dir", zap.String("dir", j.inboxDir), zap.Error(err))
		}
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > j.ttl {
			path := filepath.Join(j.inboxDir, e.Name())
			if err := os.Remove(path); err == nil {
				if j.logger != nil {
					j.logger.Info("janitor: removed expired inbox file", zap.String("path", path))
				}
			}
		}
	}
}

// downloadAttachment downloads the file at downloadURL to dst using an O_CREATE|O_EXCL
// open to be idempotent — if the file already exists the download is skipped.
//
// Returns the local path and whether the download was performed (false = idempotent skip).
func downloadAttachment(ctx context.Context, downloadURL, dst string) (bool, error) {
	// O_CREATE|O_EXCL fails if file already exists → idempotent skip.
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return false, nil // idempotent skip
		}
		return false, fmt.Errorf("inbox: open %q: %w", dst, err)
	}
	defer func() { _ = f.Close() }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		_ = os.Remove(dst)
		return false, fmt.Errorf("inbox: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_ = os.Remove(dst)
		return false, fmt.Errorf("inbox: download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(dst)
		return false, fmt.Errorf("inbox: download status %d for %s", resp.StatusCode, downloadURL)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = os.Remove(dst)
		return false, fmt.Errorf("inbox: write: %w", err)
	}
	return true, nil
}
