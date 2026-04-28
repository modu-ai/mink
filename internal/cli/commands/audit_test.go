package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/spf13/cobra"
)

// TestNewAuditCommand_ValidatesFlags tests that the audit command has correct flags
func TestNewAuditCommand_ValidatesFlags(t *testing.T) {
	// Arrange: Create audit command
	cmd := NewAuditCommand()

	// Assert: Command should have correct use and short description
	if cmd.Use != "audit" {
		t.Errorf("Expected use 'audit', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Expected short description")
	}

	// Assert: Command should have query subcommand
	queryCmd := getSubCommand(cmd, "query")
	if queryCmd == nil {
		t.Fatal("Query subcommand not found")
	}

	// Assert: Query command should have required flags
	flags := []string{"since", "until", "type", "log-dir"}
	for _, flag := range flags {
		if f := queryCmd.Flags().Lookup(flag); f == nil {
			t.Errorf("Missing flag: %s", flag)
		}
	}
}

// TestNewAuditCommand_QueryNoFilter tests query with no filters
func TestNewAuditCommand_QueryNoFilter(t *testing.T) {
	// Arrange: Create temporary directory with audit log
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "audit.log")

	// Write test events
	now := time.Now().UTC().Truncate(time.Second)
	event := audit.NewAuditEvent(now, audit.EventTypeFSWrite, audit.SeverityInfo, "Test event", nil)
	data, _ := json.Marshal(event)
	os.WriteFile(logFile, append(data, '\n'), 0644)

	// Arrange: Create command and capture output
	cmd := NewAuditCommand()
	queryCmd := getSubCommand(cmd, "query")
	queryCmd.Flags().Set("log-dir", logDir)

	var output strings.Builder
	queryCmd.SetOut(&output)

	// Act: Execute command
	err := queryCmd.RunE(queryCmd, []string{})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Assert: Should output JSON array
	result := output.String()
	if !strings.HasPrefix(result, "[") {
		t.Errorf("Expected JSON array output, got: %s", result[:min(50, len(result))])
	}

	// Parse and validate JSON
	var events []audit.AuditEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("Failed to parse output JSON: %v\nOutput: %s", err, result)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
}

// TestNewAuditCommand_QueryWithSince tests query with --since flag
func TestNewAuditCommand_QueryWithSince(t *testing.T) {
	// Arrange: Create temporary directory with audit log
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "audit.log")

	// Write test events at different times
	baseTime := time.Now().UTC().Truncate(time.Second)
	events := []audit.AuditEvent{
		audit.NewAuditEvent(baseTime.Add(-24*time.Hour), audit.EventTypeFSWrite, audit.SeverityInfo, "Old event", nil),
		audit.NewAuditEvent(baseTime, audit.EventTypeFSWrite, audit.SeverityInfo, "Recent event", nil),
	}

	file, _ := os.Create(logFile)
	for _, event := range events {
		data, _ := json.Marshal(event)
		file.Write(append(data, '\n'))
	}
	file.Close()

	// Arrange: Create command with --since flag
	sinceTime := baseTime.Add(-12 * time.Hour)
	sinceStr := sinceTime.Format(time.RFC3339)

	cmd := NewAuditCommand()
	queryCmd := getSubCommand(cmd, "query")
	queryCmd.Flags().Set("log-dir", logDir)
	queryCmd.Flags().Set("since", sinceStr)

	var output strings.Builder
	queryCmd.SetOut(&output)

	// Act: Execute command
	err := queryCmd.RunE(queryCmd, []string{})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Assert: Should only return events after --since time
	var resultEvents []audit.AuditEvent
	json.Unmarshal([]byte(output.String()), &resultEvents)

	if len(resultEvents) != 1 {
		t.Errorf("Expected 1 event with --since filter, got %d", len(resultEvents))
	}
	if len(resultEvents) > 0 && resultEvents[0].Message != "Recent event" {
		t.Errorf("Expected 'Recent event', got '%s'", resultEvents[0].Message)
	}
}

// TestNewAuditCommand_QueryWithType tests query with --type flag
func TestNewAuditCommand_QueryWithType(t *testing.T) {
	// Arrange: Create temporary directory with audit log
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "audit.log")

	// Write test events with different types
	now := time.Now().UTC().Truncate(time.Second)
	events := []audit.AuditEvent{
		audit.NewAuditEvent(now, audit.EventTypeFSWrite, audit.SeverityInfo, "File write", nil),
		audit.NewAuditEvent(now, audit.EventTypePermissionGrant, audit.SeverityInfo, "Permission", nil),
	}

	file, _ := os.Create(logFile)
	for _, event := range events {
		data, _ := json.Marshal(event)
		file.Write(append(data, '\n'))
	}
	file.Close()

	// Arrange: Create command with --type flag
	cmd := NewAuditCommand()
	queryCmd := getSubCommand(cmd, "query")
	queryCmd.Flags().Set("log-dir", logDir)
	queryCmd.Flags().Set("type", "fs.write")

	var output strings.Builder
	queryCmd.SetOut(&output)

	// Act: Execute command
	err := queryCmd.RunE(queryCmd, []string{})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Assert: Should only return events of specified type
	var resultEvents []audit.AuditEvent
	json.Unmarshal([]byte(output.String()), &resultEvents)

	if len(resultEvents) != 1 {
		t.Errorf("Expected 1 event with --type filter, got %d", len(resultEvents))
	}
	if len(resultEvents) > 0 && resultEvents[0].Type != audit.EventTypeFSWrite {
		t.Errorf("Expected type fs.write, got %s", resultEvents[0].Type)
	}
}

// TestNewAuditCommand_QueryWithMultipleTypes tests query with comma-separated --type flag
func TestNewAuditCommand_QueryWithMultipleTypes(t *testing.T) {
	// Arrange: Create temporary directory with audit log
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "audit.log")

	// Write test events with different types
	now := time.Now().UTC().Truncate(time.Second)
	events := []audit.AuditEvent{
		audit.NewAuditEvent(now, audit.EventTypeFSWrite, audit.SeverityInfo, "File write", nil),
		audit.NewAuditEvent(now, audit.EventTypePermissionGrant, audit.SeverityInfo, "Permission", nil),
		audit.NewAuditEvent(now, audit.EventTypeCredentialAccessed, audit.SeverityWarning, "Credential", nil),
	}

	file, _ := os.Create(logFile)
	for _, event := range events {
		data, _ := json.Marshal(event)
		file.Write(append(data, '\n'))
	}
	file.Close()

	// Arrange: Create command with multiple --type values
	cmd := NewAuditCommand()
	queryCmd := getSubCommand(cmd, "query")
	queryCmd.Flags().Set("log-dir", logDir)
	queryCmd.Flags().Set("type", "fs.write,permission.grant")

	var output strings.Builder
	queryCmd.SetOut(&output)

	// Act: Execute command
	err := queryCmd.RunE(queryCmd, []string{})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Assert: Should only return events of specified types
	var resultEvents []audit.AuditEvent
	json.Unmarshal([]byte(output.String()), &resultEvents)

	if len(resultEvents) != 2 {
		t.Errorf("Expected 2 events with multiple --type filter, got %d", len(resultEvents))
	}
}

// TestNewAuditCommand_QueryInvalidSince tests query with invalid --since timestamp
func TestNewAuditCommand_QueryInvalidSince(t *testing.T) {
	// Arrange: Create command with invalid --since timestamp
	tmpDir := t.TempDir()
	cmd := NewAuditCommand()
	queryCmd := getSubCommand(cmd, "query")
	queryCmd.Flags().Set("log-dir", tmpDir)
	queryCmd.Flags().Set("since", "invalid-timestamp")

	// Act: Execute command
	err := queryCmd.RunE(queryCmd, []string{})

	// Assert: Should return error
	if err == nil {
		t.Error("Expected error for invalid --since timestamp")
	}
}

// TestNewAuditCommand_QueryEmptyLogDir tests query with empty log directory
func TestNewAuditCommand_QueryEmptyLogDir(t *testing.T) {
	// Arrange: Create command with empty log directory
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logDir, 0755)

	cmd := NewAuditCommand()
	queryCmd := getSubCommand(cmd, "query")
	queryCmd.Flags().Set("log-dir", logDir)

	var output strings.Builder
	queryCmd.SetOut(&output)

	// Act: Execute command
	err := queryCmd.RunE(queryCmd, []string{})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Assert: Should output empty JSON array
	result := strings.TrimSpace(output.String())
	if result != "[]" {
		t.Errorf("Expected empty JSON array '[]', got '%s'", result)
	}
}

// getSubCommand is a helper to get subcommand by name
func getSubCommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
