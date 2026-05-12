// Package builtin implements the BuiltinProvider with SQLite FTS5 backend.
package builtin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/memory"
)

// TestBuiltin_UserMdReadOnly_WritingReturnsError verifies that any attempt
// to write to USER.md returns memory.ErrUserMdReadOnly (AC-014).
func TestBuiltin_UserMdReadOnly_WritingReturnsError(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for memory files
	tempDir := t.TempDir()
	userMdPath := filepath.Join(tempDir, "USER.md")
	dbPath := filepath.Join(tempDir, "test.db")

	// Create provider with test paths
	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	// Initialize the provider
	if err := provider.Initialize("test-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Set USER.md path
	provider.SetUserMdPath(userMdPath)

	// Try to write to USER.md - should fail
	err = provider.WriteUserMd("test content")
	if err == nil {
		t.Fatal("WriteUserMd should return error, got nil")
	}

	if !errors.Is(err, memory.ErrUserMdReadOnly) {
		t.Fatalf("Expected ErrUserMdReadOnly, got: %v", err)
	}
}

// TestBuiltin_UserMdReadOnly_ContentIncludedInPrompt verifies that USER.md
// content is included in SystemPromptBlock even though it's read-only (AC-014).
func TestBuiltin_UserMdReadOnly_ContentIncludedInPrompt(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	userMdPath := filepath.Join(tempDir, "USER.md")
	memoryMdPath := filepath.Join(tempDir, "MEMORY.md")
	dbPath := filepath.Join(tempDir, "test.db")

	// Create USER.md with test content
	userMdContent := "# User Instructions\nThis is important context."
	if err := os.WriteFile(userMdPath, []byte(userMdContent), 0600); err != nil {
		t.Fatalf("Failed to create USER.md: %v", err)
	}

	// Create provider with test paths
	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	// Initialize and set paths
	if err := provider.Initialize("test-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	provider.SetUserMdPath(userMdPath)
	provider.SetMemoryMdPath(memoryMdPath)

	// Get system prompt block
	block := provider.SystemPromptBlock()

	// Should contain USER.md content
	if !strings.Contains(block, userMdContent) {
		t.Errorf("SystemPromptBlock should contain USER.md content.\nGot: %s\nExpected to contain: %s", block, userMdContent)
	}
}

// TestBuiltin_MemoryMd_AppendOnSyncTurn verifies that SyncTurn appends
// entries to MEMORY.md in the correct format (AC-015).
func TestBuiltin_MemoryMd_AppendOnSyncTurn(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	memoryMdPath := filepath.Join(tempDir, "MEMORY.md")
	dbPath := filepath.Join(tempDir, "test.db")

	// Create provider with test paths
	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	// Initialize and set paths
	if err := provider.Initialize("test-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	provider.SetMemoryMdPath(memoryMdPath)

	// Call SyncTurn
	sessionID := "session-123"
	userContent := "What is the capital of France?"
	assistantContent := "The capital of France is Paris."

	if err := provider.SyncTurn(sessionID, userContent, assistantContent); err != nil {
		t.Fatalf("SyncTurn failed: %v", err)
	}

	// Verify MEMORY.md was created with correct content
	content, err := os.ReadFile(memoryMdPath)
	if err != nil {
		t.Fatalf("Failed to read MEMORY.md: %v", err)
	}

	expectedLine := "- [" + sessionID + "] " + userContent + "\n"
	if !strings.Contains(string(content), expectedLine) {
		t.Errorf("MEMORY.md should contain formatted line.\nGot: %s\nExpected: %s", string(content), expectedLine)
	}

	// Verify file permissions
	info, err := os.Stat(memoryMdPath)
	if err != nil {
		t.Fatalf("Failed to stat MEMORY.md: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("MEMORY.md should have 0600 permissions, got: %v", info.Mode().Perm())
	}
}

// TestBuiltin_SystemPromptBlock_Truncation verifies that SystemPromptBlock
// truncates MEMORY.md to fit within 8KB limit.
func TestBuiltin_SystemPromptBlock_Truncation(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	userMdPath := filepath.Join(tempDir, "USER.md")
	memoryMdPath := filepath.Join(tempDir, "MEMORY.md")
	dbPath := filepath.Join(tempDir, "test.db")

	// Create USER.md with 1KB content
	userMdContent := strings.Repeat("USER ", 256) // ~1KB
	if err := os.WriteFile(userMdPath, []byte(userMdContent), 0600); err != nil {
		t.Fatalf("Failed to create USER.md: %v", err)
	}

	// Create MEMORY.md with 10KB content (larger than 8KB limit)
	largeMemoryContent := strings.Repeat("MEMORY ", 2560) // ~10KB
	if err := os.WriteFile(memoryMdPath, []byte(largeMemoryContent), 0600); err != nil {
		t.Fatalf("Failed to create MEMORY.md: %v", err)
	}

	// Create provider with test paths
	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	// Initialize and set paths
	if err := provider.Initialize("test-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	provider.SetUserMdPath(userMdPath)
	provider.SetMemoryMdPath(memoryMdPath)

	// Get system prompt block
	block := provider.SystemPromptBlock()

	// Total size should be <= 8KB (8192 bytes)
	if len(block) > 8192 {
		t.Errorf("SystemPromptBlock should be <= 8KB, got: %d bytes", len(block))
	}

	// Should contain USER.md content
	if !strings.Contains(block, userMdContent) {
		t.Error("SystemPromptBlock should contain USER.md content")
	}

	// Should contain portion of MEMORY.md (last part due to truncation)
	if !strings.Contains(block, "MEMORY") {
		t.Error("SystemPromptBlock should contain part of MEMORY.md")
	}
}

// TestBuiltin_MemoryMd_CreatesDirectoryIfMissing verifies that SyncTurn
// auto-creates ~/.goose/memory/ directory with permission 0700 if missing.
func TestBuiltin_MemoryMd_CreatesDirectoryIfMissing(t *testing.T) {
	t.Parallel()

	// Create a nested path that doesn't exist
	tempDir := t.TempDir()
	memoryDir := filepath.Join(tempDir, ".goose", "memory")
	memoryMdPath := filepath.Join(memoryDir, "MEMORY.md")
	dbPath := filepath.Join(tempDir, "test.db")

	// Verify directory doesn't exist
	if _, err := os.Stat(memoryDir); !os.IsNotExist(err) {
		t.Fatal("Test setup error: directory should not exist")
	}

	// Create provider with test paths
	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	// Initialize and set paths
	if err := provider.Initialize("test-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	provider.SetMemoryMdPath(memoryMdPath)

	// Call SyncTurn - should auto-create directory
	if err := provider.SyncTurn("session-123", "test content", ""); err != nil {
		t.Fatalf("SyncTurn failed: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(memoryDir)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("Path should be a directory")
	}

	// Verify directory permissions
	if info.Mode().Perm() != 0700 {
		t.Errorf("Directory should have 0700 permissions, got: %v", info.Mode().Perm())
	}

	// Verify MEMORY.md was created
	if _, err := os.Stat(memoryMdPath); err != nil {
		t.Fatalf("MEMORY.md was not created: %v", err)
	}
}
