package briefing

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// File system permissions enforced by the archive writer.
//
// REQ-BR-051: archive directory MUST be 0700, files MUST be 0600.
const (
	archiveDirPerm  os.FileMode = 0o700
	archiveFilePerm os.FileMode = 0o600
)

// ArchiveDir returns the canonical archive directory under the user's home
// directory: ~/.mink/briefing.
//
// The directory itself is not created by this function. Use WriteArchive
// or WriteArchiveToDir to write a payload (those create the directory with
// the required mode).
func ArchiveDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("briefing archive: resolve home: %w", err)
	}
	return filepath.Join(home, ".mink", "briefing"), nil
}

// WriteArchive renders the briefing payload to plain text and persists it to
// ~/.mink/briefing/YYYY-MM-DD.md. The parent directory is created with
// mode 0700 and the file is written with mode 0600.
//
// The filename uses payload.GeneratedAt formatted as YYYY-MM-DD when set,
// otherwise time.Now() in the local timezone.
//
// @MX:ANCHOR: WriteArchive is the single archive write path for the briefing
// pipeline (REQ-BR-012, REQ-BR-030).
// @MX:REASON: SPEC-MINK-BRIEFING-001 REQ-BR-051; the 0600/0700 mode
// invariant must hold across all archive writes (Privacy invariant 2).
func WriteArchive(payload *BriefingPayload) (string, error) {
	dir, err := ArchiveDir()
	if err != nil {
		return "", err
	}
	return WriteArchiveToDir(dir, payload)
}

// WriteArchiveToDir is the test-friendly variant of WriteArchive. It writes
// the rendered payload to the supplied directory rather than the user home.
// Behavior is otherwise identical to WriteArchive: mkdir 0700, write 0600.
//
// Returns the absolute path of the written file on success.
func WriteArchiveToDir(dir string, payload *BriefingPayload) (string, error) {
	if payload == nil {
		return "", fmt.Errorf("briefing archive: nil payload")
	}
	if dir == "" {
		return "", fmt.Errorf("briefing archive: empty dir")
	}

	if err := os.MkdirAll(dir, archiveDirPerm); err != nil {
		return "", fmt.Errorf("briefing archive: mkdir: %w", err)
	}
	// Re-assert directory mode in case the directory already existed with a
	// different mode (MkdirAll is a no-op on existing directories).
	if err := os.Chmod(dir, archiveDirPerm); err != nil {
		return "", fmt.Errorf("briefing archive: chmod dir: %w", err)
	}

	date := time.Now().Format("2006-01-02")
	if !payload.GeneratedAt.IsZero() {
		date = payload.GeneratedAt.Format("2006-01-02")
	}
	path := filepath.Join(dir, date+".md")

	content := RenderCLI(payload, true)

	// O_TRUNC ensures idempotent same-day overwrites.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, archiveFilePerm)
	if err != nil {
		return "", fmt.Errorf("briefing archive: open: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(content); err != nil {
		return "", fmt.Errorf("briefing archive: write: %w", err)
	}

	// Re-assert file mode (umask may have masked bits).
	if err := os.Chmod(path, archiveFilePerm); err != nil {
		return "", fmt.Errorf("briefing archive: chmod file: %w", err)
	}

	return path, nil
}
