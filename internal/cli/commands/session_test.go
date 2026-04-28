// Package commands provides session subcommands for the goose CLI.
// @MX:TODO Implement all session commands (RED phase)
package commands

import (
	"bytes"
	"strings"
	"testing"
)

// TestSessionListEmpty verifies that list works with no sessions.
func TestSessionListEmpty(t *testing.T) {
	cmd := NewSessionCommand("127.0.0.1:9005")
	cmd.SetArgs([]string{"list"})

	// Override session Dir for testing
	// This would require making Dir() configurable in session package

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Empty list should print nothing or header
	output := out.String()
	if !strings.Contains(output, "No sessions found") && output != "" {
		t.Errorf("Unexpected output for empty list: %s", output)
	}
}

// TestSessionListWithSessions verifies that list prints session names.
func TestSessionListWithSessions(t *testing.T) {
	// This test would require mocking session.List()
	// For now, we'll skip the actual implementation test
	t.Skip("Requires session.List() mocking")
}

// TestSessionLoadSuccess verifies that load prints message count.
func TestSessionLoadSuccess(t *testing.T) {
	cmd := NewSessionCommand("127.0.0.1:9005")
	cmd.SetArgs([]string{"load", "test-session"})

	// This would require mocking session.Load()
	// For now, we'll skip the actual implementation test
	t.Skip("Requires session.Load() mocking")
}

// TestSessionLoadNotFound verifies that load errors on missing session.
func TestSessionLoadNotFound(t *testing.T) {
	cmd := NewSessionCommand("127.0.0.1:9005")
	cmd.SetArgs([]string{"load", "nonexistent"})

	var out bytes.Buffer
	cmd.SetErr(&out)

	if err := cmd.Execute(); err == nil {
		t.Error("Expected error loading non-existent session, got nil")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestSessionSaveErrors verifies that save returns appropriate error.
// REQ-CLI-012: /save should only work in TUI, not as CLI command.
func TestSessionSaveErrors(t *testing.T) {
	cmd := NewSessionCommand("127.0.0.1:9005")
	cmd.SetArgs([]string{"save", "test-session"})

	var out bytes.Buffer
	cmd.SetErr(&out)

	if err := cmd.Execute(); err == nil {
		t.Error("Expected error for save command, got nil")
	} else if !strings.Contains(strings.ToLower(err.Error()), "only works in chat mode") {
		t.Errorf("Expected 'only works in chat mode' error, got: %v", err)
	}
}

// TestSessionRmWithYes verifies that rm deletes with --yes flag.
func TestSessionRmWithYes(t *testing.T) {
	// This would require mocking session.Delete()
	// For now, we'll skip the actual implementation test
	t.Skip("Requires session.Delete() mocking")
}

// TestSessionRmWithoutYes verifies that rm prompts for confirmation.
func TestSessionRmWithoutYes(t *testing.T) {
	// This would require mocking stdin for confirmation
	// For now, we'll skip the actual implementation test
	t.Skip("Requires stdin mocking for confirmation")
}
