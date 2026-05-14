package briefing

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestArchive_FilePerms validates AC-012: archive file mode 0600 + dir 0700.
func TestArchive_FilePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix permission semantics not enforced on windows")
	}

	dir := filepath.Join(t.TempDir(), "archive")

	payload := &BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC),
		Status:      map[string]string{"weather": "ok", "mantra": "ok"},
		Mantra:      MantraModule{Text: "stay grounded"},
	}

	path, err := WriteArchiveToDir(dir, payload)
	if err != nil {
		t.Fatalf("WriteArchiveToDir: %v", err)
	}

	if filepath.Base(path) != "2026-05-14.md" {
		t.Errorf("basename = %s, want 2026-05-14.md", filepath.Base(path))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != archiveFilePerm {
		t.Errorf("file mode = %o, want %o", mode, archiveFilePerm)
	}

	dinfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := dinfo.Mode().Perm(); mode != archiveDirPerm {
		t.Errorf("dir mode = %o, want %o", mode, archiveDirPerm)
	}
}

// TestArchive_ExistingDirModeReassert validates that WriteArchiveToDir
// re-asserts the dir mode when the directory pre-exists with a wider mode.
func TestArchive_ExistingDirModeReassert(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix permission semantics not enforced on windows")
	}

	dir := filepath.Join(t.TempDir(), "archive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}

	payload := &BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC),
		Status:      map[string]string{"weather": "ok"},
		Mantra:      MantraModule{Text: "ok"},
	}
	if _, err := WriteArchiveToDir(dir, payload); err != nil {
		t.Fatalf("WriteArchiveToDir: %v", err)
	}

	dinfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := dinfo.Mode().Perm(); mode != archiveDirPerm {
		t.Errorf("dir mode after reassert = %o, want %o", mode, archiveDirPerm)
	}
}

// TestArchive_NilPayload ensures defensive validation.
func TestArchive_NilPayload(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteArchiveToDir(dir, nil); err == nil {
		t.Error("expected error for nil payload")
	}
}

// TestArchive_EmptyDir ensures defensive validation against empty dir input.
func TestArchive_EmptyDir(t *testing.T) {
	if _, err := WriteArchiveToDir("", &BriefingPayload{}); err == nil {
		t.Error("expected error for empty dir")
	}
}

// TestArchive_GeneratedAtFallback validates that an unset GeneratedAt falls
// back to time.Now() formatted as today's date.
func TestArchive_GeneratedAtFallback(t *testing.T) {
	dir := t.TempDir()
	payload := &BriefingPayload{
		Status: map[string]string{"weather": "ok"},
		// GeneratedAt left zero.
	}
	path, err := WriteArchiveToDir(dir, payload)
	if err != nil {
		t.Fatalf("WriteArchiveToDir: %v", err)
	}
	expected := time.Now().Format("2006-01-02") + ".md"
	if filepath.Base(path) != expected {
		t.Errorf("basename = %s, want %s", filepath.Base(path), expected)
	}
}

// TestArchive_ContentIncludesMantra validates that the archive content
// reflects the rendered briefing.
func TestArchive_ContentIncludesMantra(t *testing.T) {
	dir := t.TempDir()
	payload := &BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC),
		Status:      map[string]string{"mantra": "ok"},
		Mantra:      MantraModule{Text: "오늘도 한 걸음"},
	}
	path, err := WriteArchiveToDir(dir, payload)
	if err != nil {
		t.Fatalf("WriteArchiveToDir: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(b), "오늘도 한 걸음") {
		t.Errorf("archive content missing mantra: %s", b)
	}
	if !strings.Contains(string(b), "MORNING BRIEFING") {
		t.Errorf("archive content missing header: %s", b)
	}
}

// TestArchive_IdempotentSameDay validates that repeated writes on the same
// day overwrite the previous file rather than failing.
func TestArchive_IdempotentSameDay(t *testing.T) {
	dir := t.TempDir()
	payload := &BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC),
		Status:      map[string]string{"mantra": "ok"},
		Mantra:      MantraModule{Text: "first"},
	}
	path1, err := WriteArchiveToDir(dir, payload)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}

	payload.Mantra.Text = "second"
	path2, err := WriteArchiveToDir(dir, payload)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if path1 != path2 {
		t.Errorf("path1 = %s, path2 = %s, expected equal", path1, path2)
	}
	b, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(b), "second") {
		t.Errorf("expected overwritten content, got: %s", b)
	}
	if strings.Contains(string(b), "first") {
		t.Errorf("old content not truncated: %s", b)
	}
}

// TestArchiveDir validates the canonical home-relative path computation.
func TestArchiveDir(t *testing.T) {
	dir, err := ArchiveDir()
	if err != nil {
		t.Fatalf("ArchiveDir: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join(".mink", "briefing")) {
		t.Errorf("ArchiveDir = %s, want ...%s", dir, filepath.Join(".mink", "briefing"))
	}
}
