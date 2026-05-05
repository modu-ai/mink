// Package tui provides tests for /save and /load slash command handlers.
// AC-CLITUI-012, AC-CLITUI-013
package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/cli/session"
)

// setupSessionTestHome overrides HOME to redirect session.Dir() to a tmpdir.
// session.Dir() calls os.UserHomeDir() which reads the HOME env variable on Unix.
func setupSessionTestHome(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	return filepath.Join(tmpDir, ".goose", "sessions"), func() {
		os.Setenv("HOME", oldHome)
	}
}

// TestSession_Save_WritesJsonl verifies AC-CLITUI-012:
// /save <name> writes a .jsonl file with 2 lines and appends a system message
// "[saved: <name>]" to the model viewport.
func TestSession_Save_WritesJsonl(t *testing.T) {
	tmpDir, cleanup := setupSessionTestHome(t)
	defer cleanup()

	// Arrange: model with 1 user + 1 assistant message
	m := NewModel(nil, "", false)
	m.messages = []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}

	cmd := SlashCmd{Name: "save", Args: []string{"test01"}}

	// Act
	response, _ := HandleSlashCmd(cmd, m)

	// Assert: system message contains saved notification
	if !strings.Contains(response, "[saved: test01]") {
		t.Errorf("expected response to contain '[saved: test01]', got: %q", response)
	}

	// Assert: .jsonl file exists in sessions dir
	expectedPath := filepath.Join(tmpDir, "test01.jsonl")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("expected jsonl file at %q, got error: %v", expectedPath, err)
	}

	// Assert: exactly 2 lines
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 jsonl lines, got %d: %q", len(lines), string(data))
	}

	// Assert: first line is user message
	var msg0 struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &msg0); err != nil {
		t.Fatalf("failed to parse line 0: %v", err)
	}
	if msg0.Role != "user" || msg0.Content != "hello" {
		t.Errorf("line 0: expected user/hello, got %q/%q", msg0.Role, msg0.Content)
	}
}

// TestSession_Load_RestoresWithInitialMessages verifies AC-CLITUI-013:
// /load <name> restores messages and adds a "[loaded: <name>, N messages]" system msg.
func TestSession_Load_RestoresWithInitialMessages(t *testing.T) {
	_, cleanup := setupSessionTestHome(t)
	defer cleanup()

	// Arrange: save a session file with 2 messages
	msgs := []session.Message{
		{Role: "user", Content: "hello", Timestamp: 1000},
		{Role: "assistant", Content: "hi", Timestamp: 1001},
	}
	if err := session.Save("test01", msgs); err != nil {
		t.Fatalf("pre-condition Save failed: %v", err)
	}

	// Arrange: model with 0 current messages
	m := NewModel(nil, "", false)

	cmd := SlashCmd{Name: "load", Args: []string{"test01"}}

	// Act
	response, _ := HandleSlashCmd(cmd, m)

	// Assert: response contains loaded notification with count
	if !strings.Contains(response, "[loaded: test01, 2 messages]") {
		t.Errorf("expected response to contain '[loaded: test01, 2 messages]', got: %q", response)
	}

	// Assert: model messages are restored (2 messages)
	if len(m.messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(m.messages))
	}
	if m.messages[0].Role != "user" || m.messages[0].Content != "hello" {
		t.Errorf("message[0]: expected user/hello, got %q/%q", m.messages[0].Role, m.messages[0].Content)
	}
	if m.messages[1].Role != "assistant" || m.messages[1].Content != "hi" {
		t.Errorf("message[1]: expected assistant/hi, got %q/%q", m.messages[1].Role, m.messages[1].Content)
	}

	// Assert: initialMsgs field is set for next ChatStream call
	if len(m.initialMsgs) != 2 {
		t.Errorf("expected 2 initialMsgs, got %d", len(m.initialMsgs))
	}
}

// TestSession_Save_NoArgs verifies error message when no name provided.
func TestSession_Save_NoArgs(t *testing.T) {
	m := NewModel(nil, "", false)
	cmd := SlashCmd{Name: "save", Args: []string{}}
	response, _ := HandleSlashCmd(cmd, m)
	if response != "Usage: /save <name>" {
		t.Errorf("expected usage message, got: %q", response)
	}
}

// TestSession_Load_NoArgs verifies error message when no name provided.
func TestSession_Load_NoArgs(t *testing.T) {
	m := NewModel(nil, "", false)
	cmd := SlashCmd{Name: "load", Args: []string{}}
	response, _ := HandleSlashCmd(cmd, m)
	if response != "Usage: /load <name>" {
		t.Errorf("expected usage message, got: %q", response)
	}
}

// TestSession_Load_NotFound verifies error when session does not exist.
func TestSession_Load_NotFound(t *testing.T) {
	_, cleanup := setupSessionTestHome(t)
	defer cleanup()

	m := NewModel(nil, "", false)
	cmd := SlashCmd{Name: "load", Args: []string{"nonexistent"}}
	response, _ := HandleSlashCmd(cmd, m)
	if !strings.Contains(response, "not found") && !strings.Contains(response, "error") && !strings.Contains(response, "Error") {
		t.Errorf("expected error message for missing session, got: %q", response)
	}
}
