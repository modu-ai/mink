// Package sessionmenu provides tests for the session file loader.
// AC-CLITUI3-001, AC-CLITUI3-002, AC-CLITUI3-003
package sessionmenu

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupHome overrides HOME so sessionsDir() resolves under a temp dir.
func setupHome(t *testing.T) (sessDir string, cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	sessDir = filepath.Join(tmpDir, ".goose", "sessions")
	cleanup = func() { os.Setenv("HOME", oldHome) }
	return sessDir, cleanup
}

// createDummySession writes a minimal .jsonl session file with the given mtime.
func createDummySession(t *testing.T, dir, name string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, name+".jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"hi","ts":0}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
}

// TestSessionMenu_Loader_SortsByMtimeDesc verifies entries come back newest first.
// REQ-CLITUI3-002
func TestSessionMenu_Loader_SortsByMtimeDesc(t *testing.T) {
	sessDir, cleanup := setupHome(t)
	defer cleanup()

	base := time.Now()
	createDummySession(t, sessDir, "old", base.Add(-2*time.Hour))
	createDummySession(t, sessDir, "mid", base.Add(-1*time.Hour))
	createDummySession(t, sessDir, "new", base)

	entries := Load()

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Name != "new" {
		t.Errorf("expected entries[0].Name == 'new', got %q", entries[0].Name)
	}
	if entries[1].Name != "mid" {
		t.Errorf("expected entries[1].Name == 'mid', got %q", entries[1].Name)
	}
	if entries[2].Name != "old" {
		t.Errorf("expected entries[2].Name == 'old', got %q", entries[2].Name)
	}
}

// TestSessionMenu_Loader_CapsTen verifies that at most 10 entries are returned.
// REQ-CLITUI3-002
func TestSessionMenu_Loader_CapsTen(t *testing.T) {
	sessDir, cleanup := setupHome(t)
	defer cleanup()

	base := time.Now()
	for i := 0; i < 15; i++ {
		name := filepath.Join("session", string(rune('a'+i)))
		_ = name
		createDummySession(t, sessDir, string(rune('a'+i)), base.Add(time.Duration(-i)*time.Minute))
	}

	entries := Load()

	if len(entries) != maxEntries {
		t.Errorf("expected %d entries (cap), got %d", maxEntries, len(entries))
	}
}

// TestSessionMenu_Loader_EmptyDir verifies that an empty sessions dir returns nil.
// REQ-CLITUI3-002
func TestSessionMenu_Loader_EmptyDir(t *testing.T) {
	sessDir, cleanup := setupHome(t)
	defer cleanup()

	// Create the sessions directory but leave it empty.
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	entries := Load()

	if len(entries) != 0 {
		t.Errorf("expected 0 entries from empty dir, got %d", len(entries))
	}
}

// TestSessionMenu_Loader_AbsentDir verifies that a missing sessions dir returns nil without error.
// REQ-CLITUI3-002
func TestSessionMenu_Loader_AbsentDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	// Point HOME to a dir that has no .goose/ subtree at all.
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	entries := Load()

	if entries != nil {
		t.Errorf("expected nil from absent dir, got %v", entries)
	}
}

// TestSessionMenu_Loader_IgnoresNonJsonl verifies that non-.jsonl files are skipped.
func TestSessionMenu_Loader_IgnoresNonJsonl(t *testing.T) {
	sessDir, cleanup := setupHome(t)
	defer cleanup()

	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write a non-.jsonl file.
	_ = os.WriteFile(filepath.Join(sessDir, "readme.txt"), []byte("ignore me"), 0o644)
	// Write one valid session.
	createDummySession(t, sessDir, "valid", time.Now())

	entries := Load()

	if len(entries) != 1 {
		t.Errorf("expected 1 entry (only .jsonl), got %d", len(entries))
	}
	if entries[0].Name != "valid" {
		t.Errorf("expected entries[0].Name == 'valid', got %q", entries[0].Name)
	}
}
