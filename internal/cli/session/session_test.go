// Package session provides session file I/O for the goose CLI.
// @MX:TODO Implement all session file operations (RED phase)
package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSaveLoadRoundtrip verifies that saving and loading preserves messages.
// AC-CLI-010: save/load roundtrip.
func TestSaveLoadRoundtrip(t *testing.T) {
	// Create temporary session directory
	tmpDir := t.TempDir()
	testDir = tmpDir // Override global Dir() for testing

	messages := []Message{
		{Role: "user", Content: "Hello", Timestamp: time.Now().UnixMilli()},
		{Role: "assistant", Content: "Hi there!", Timestamp: time.Now().UnixMilli()},
	}

	// Save
	err := Save("test-session", messages)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := Load("test-session")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify
	if len(loaded) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(loaded))
	}

	for i := range messages {
		if loaded[i].Role != messages[i].Role {
			t.Errorf("Message %d role: expected %s, got %s", i, messages[i].Role, loaded[i].Role)
		}
		if loaded[i].Content != messages[i].Content {
			t.Errorf("Message %d content: expected %s, got %s", i, messages[i].Content, loaded[i].Content)
		}
	}
}

// TestListAfterSaving verifies that List returns saved session names.
func TestListAfterSaving(t *testing.T) {
	tmpDir := t.TempDir()
	testDir = tmpDir

	messages := []Message{
		{Role: "user", Content: "Test", Timestamp: time.Now().UnixMilli()},
	}

	// Save multiple sessions
	names := []string{"session1", "session2", "session3"}
	for _, name := range names {
		if err := Save(name, messages); err != nil {
			t.Fatalf("Save %s failed: %v", name, err)
		}
	}

	// List
	listed, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Verify
	if len(listed) != len(names) {
		t.Errorf("Expected %d sessions, got %d", len(names), len(listed))
	}

	for _, name := range names {
		found := false
		for _, listedName := range listed {
			if listedName == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Session %s not found in list", name)
		}
	}
}

// TestDelete verifies that Delete removes session files.
func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	testDir = tmpDir

	messages := []Message{
		{Role: "user", Content: "Test", Timestamp: time.Now().UnixMilli()},
	}

	// Save session
	if err := Save("to-delete", messages); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify it exists
	if _, err := Load("to-delete"); err != nil {
		t.Fatalf("Load before delete failed: %v", err)
	}

	// Delete
	if err := Delete("to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	if _, err := Load("to-delete"); err == nil {
		t.Error("Expected error loading deleted session, got nil")
	}
}

// TestValidateNameRejectsPathTraversal verifies that ValidateName rejects dangerous names.
// REQ-CLI-020: reject ../foo attempts.
func TestValidateNameRejectsPathTraversal(t *testing.T) {
	dangerousNames := []string{
		"../foo",
		"../../etc/passwd",
		"foo/../../bar",
		"foo/bar",
		"foo\\bar",
		"./hidden",
		"",
		".",
	}

	for _, name := range dangerousNames {
		t.Run(name, func(t *testing.T) {
			if err := ValidateName(name); err == nil {
				t.Errorf("ValidateName(%q): expected error, got nil", name)
			}
		})
	}
}

// TestValidateNameAcceptsValidNames verifies that ValidateName accepts safe names.
func TestValidateNameAcceptsValidNames(t *testing.T) {
	validNames := []string{
		"session1",
		"my-session",
		"test_session",
		"123",
		"a-b-c",
		"Session with spaces",
		"한글세션",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			if err := ValidateName(name); err != nil {
				t.Errorf("ValidateName(%q): expected nil, got %v", name, err)
			}
		})
	}
}

// TestSaveCreatesDirectory verifies that Save creates session directory if not exists.
func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "sessions", "nested")
	testDir = nonExistentDir

	messages := []Message{
		{Role: "user", Content: "Test", Timestamp: time.Now().UnixMilli()},
	}

	// Save to non-existent directory
	if err := Save("test", messages); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nonExistentDir); os.IsNotExist(err) {
		t.Error("Session directory was not created")
	}
}

// TestAtomicWrite verifies that Save uses atomic tmp+rename pattern.
// @MX:NOTE Atomic writes prevent data corruption during concurrent access.
func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	testDir = tmpDir

	messages := []Message{
		{Role: "user", Content: "Test", Timestamp: time.Now().UnixMilli()},
	}

	// Save
	if err := Save("atomic-test", messages); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify no .tmp files remain
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".tmp" {
			t.Errorf("Found leftover .tmp file: %s", file.Name())
		}
	}
}

// TestLoadNonExistentReturnsError verifies that Load returns error for missing sessions.
func TestLoadNonExistentReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	testDir = tmpDir

	// Try to load non-existent session
	_, err := Load("does-not-exist")
	if err == nil {
		t.Error("Expected error loading non-existent session, got nil")
	}
}

// ── T-010: session 패키지 userpath 마이그레이션 ────────────────────────────

// TestDir_DefaultUsesMinkPath는 Dir() 기본값이 .mink/sessions 경로임을 검증한다.
// REQ-MINK-UDM-002. AC-005.
func TestDir_DefaultUsesMinkPath(t *testing.T) {
	// testDir 을 비워서 기본 Dir() 동작 테스트
	oldTestDir := testDir
	testDir = ""
	defer func() { testDir = oldTestDir }()

	// HOME 을 임시 디렉토리로 설정 (os.UserHomeDir 와 $HOME 모두 격리)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })

	dir := Dir()
	if contains := filepath.Base(filepath.Dir(dir)); contains != ".mink" {
		// .mink 경로 검증: dir == fakeHome/.mink/sessions
		expected := filepath.Join(fakeHome, ".mink", "sessions")
		if dir != expected {
			t.Errorf("Dir() = %q, want %q (must use .mink, REQ-MINK-UDM-002)", dir, expected)
		}
	}
}

// TestSave_TmpPrefix_UsesMinkPrefix는 Save 가 .mink-session-* tmp 파일 prefix 를 사용함을 검증한다.
// REQ-MINK-UDM-004. AC-006.
func TestSave_TmpPrefix_UsesMinkPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	testDir = tmpDir
	defer func() { testDir = "" }()

	err := Save("test-prefix", []Message{{Role: "user", Content: "hi", Timestamp: 1}})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 완료 후 tmp 파일은 삭제됨 — 세션 파일 존재 확인
	files, _ := os.ReadDir(tmpDir)
	for _, f := range files {
		name := f.Name()
		if len(name) > 4 && name[:4] == ".goo" {
			t.Errorf("tmp file must not use .goose prefix, got: %s", name)
		}
	}
}
