package telegram

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestJanitor_SweepOldFiles verifies that the janitor removes files whose
// modification time is older than the TTL.
func TestJanitor_SweepOldFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a file and back-date its mtime so it appears old.
	oldFile := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(oldFile, []byte("old"), 0o600); err != nil {
		t.Fatalf("create old file: %v", err)
	}
	// Back-date by 2 minutes to exceed the 1-minute TTL used in the test.
	old := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(oldFile, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// Create a file that should be kept (modified just now).
	newFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(newFile, []byte("new"), 0o600); err != nil {
		t.Fatalf("create new file: %v", err)
	}

	j := &Janitor{
		inboxDir:  dir,
		ttl:       1 * time.Minute, // 1-minute TTL
		tickEvery: 100 * time.Millisecond,
	}
	j.sweepOnce(time.Now())

	// Old file should be gone.
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("expected old file to be removed by janitor")
	}
	// New file should remain.
	if _, err := os.Stat(newFile); err != nil {
		t.Errorf("expected new file to remain: %v", err)
	}
}

// TestJanitor_RunCancels verifies that Janitor.Run returns when ctx is cancelled.
func TestJanitor_RunCancels(t *testing.T) {
	dir := t.TempDir()
	j := &Janitor{
		inboxDir:  dir,
		ttl:       30 * time.Minute,
		tickEvery: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := j.Run(ctx)
	if err == nil {
		t.Error("expected non-nil error from Run when ctx is cancelled")
	}
}

// TestJanitor_EmptyDir verifies that sweepOnce does not error on an empty dir.
func TestJanitor_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	j := &Janitor{
		inboxDir:  dir,
		ttl:       1 * time.Minute,
		tickEvery: 1 * time.Second,
	}
	// Must not panic.
	j.sweepOnce(time.Now())
}

// TestDownloadInboundAttachment_ExtWhitelist verifies that files with
// disallowed extensions are rejected.
func TestDownloadInboundAttachment_ExtWhitelist(t *testing.T) {
	allowed := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".pdf", ".txt", ".md", ".docx", ".zip"}
	blocked := []string{".exe", ".sh", ".bat", ".js", ".py"}

	for _, ext := range allowed {
		if !isAllowedExt(ext) {
			t.Errorf("extension %q should be allowed", ext)
		}
	}
	for _, ext := range blocked {
		if isAllowedExt(ext) {
			t.Errorf("extension %q should be blocked", ext)
		}
	}
}

// TestInboxPath verifies that inboxFilePath returns a safe path using only
// the message_id and a whitelisted extension, regardless of attacker-controlled
// file names.
func TestInboxPath(t *testing.T) {
	dir := "/tmp/inbox"
	path := inboxFilePath(dir, 42, ".jpg")

	expected := filepath.Join(dir, "42.jpg")
	if path != expected {
		t.Errorf("inboxFilePath = %q, want %q", path, expected)
	}
}
