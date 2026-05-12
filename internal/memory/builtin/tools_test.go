// Package builtin implements the BuiltinProvider with SQLite FTS5 backend.
package builtin

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/memory"
)

// TestBuiltin_GetToolSchemas_ReturnsExpectedTools verifies that GetToolSchemas
// returns memory_recall and memory_save tool definitions with proper JSON Schema.
func TestBuiltin_GetToolSchemas_ReturnsExpectedTools(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	schemas := provider.GetToolSchemas()

	if len(schemas) != 2 {
		t.Fatalf("Expected 2 tool schemas, got: %d", len(schemas))
	}

	// Verify memory_recall schema
	recallSchema := findSchema(schemas, "memory_recall")
	if recallSchema == nil {
		t.Fatal("memory_recall schema not found")
	}

	if recallSchema.Owner != "builtin" {
		t.Errorf("Expected owner 'builtin', got: %s", recallSchema.Owner)
	}

	if recallSchema.Parameters == nil {
		t.Fatal("memory_recall parameters should not be nil")
	}

	// Verify parameters contain expected fields
	var params map[string]interface{}
	if err := json.Unmarshal(recallSchema.Parameters, &params); err != nil {
		t.Fatalf("Failed to unmarshal parameters: %v", err)
	}

	if props, ok := params["properties"].(map[string]interface{}); !ok {
		t.Error("Parameters should have 'properties' field")
	} else {
		// Check for 'query' property
		if _, hasQuery := props["query"]; !hasQuery {
			t.Error("memory_recall parameters should have 'query' property")
		}

		// Check for 'limit' property
		if _, hasLimit := props["limit"]; !hasLimit {
			t.Error("memory_recall parameters should have 'limit' property")
		}
	}

	// Verify memory_save schema
	saveSchema := findSchema(schemas, "memory_save")
	if saveSchema == nil {
		t.Fatal("memory_save schema not found")
	}

	if saveSchema.Owner != "builtin" {
		t.Errorf("Expected owner 'builtin', got: %s", saveSchema.Owner)
	}

	if saveSchema.Parameters == nil {
		t.Fatal("memory_save parameters should not be nil")
	}

	// Verify parameters contain expected fields
	var saveParams map[string]interface{}
	if err := json.Unmarshal(saveSchema.Parameters, &saveParams); err != nil {
		t.Fatalf("Failed to unmarshal parameters: %v", err)
	}

	if props, ok := saveParams["properties"].(map[string]interface{}); !ok {
		t.Error("Parameters should have 'properties' field")
	} else {
		// Check for 'key' property
		if _, hasKey := props["key"]; !hasKey {
			t.Error("memory_save parameters should have 'key' property")
		}

		// Check for 'content' property
		if _, hasContent := props["content"]; !hasContent {
			t.Error("memory_save parameters should have 'content' property")
		}

		// Check for 'source' property
		if _, hasSource := props["source"]; !hasSource {
			t.Error("memory_save parameters should have 'source' property")
		}
	}
}

// TestBuiltin_IsAvailable_NoIO verifies that IsAvailable performs no I/O
// and only checks if db != nil (AC-017).
func TestBuiltin_IsAvailable_NoIO(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	// Before initialization, should return false (no I/O performed)
	if provider.IsAvailable() {
		t.Error("IsAvailable should return false before initialization")
	}

	// Initialize
	tempDir2 := t.TempDir()
	dbPath2 := tempDir2 + "/test.db"
	provider2, err2 := NewBuiltin(dbPath2, nil)
	if err2 != nil {
		t.Fatalf("NewBuiltin failed: %v", err2)
	}

	if err := provider2.Initialize("test-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider2.Close()

	// After initialization, should return true (no I/O performed)
	if !provider2.IsAvailable() {
		t.Error("IsAvailable should return true after initialization")
	}
}

// TestBuiltin_HandleToolCall_Recall verifies that HandleToolCall correctly
// dispatches memory_recall tool calls.
func TestBuiltin_HandleToolCall_Recall(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	if err := provider.Initialize("test-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Add some test data
	if err := provider.SyncTurn("test-session", "Paris is the capital of France", "Correct"); err != nil {
		t.Fatalf("SyncTurn failed: %v", err)
	}

	// Call memory_recall tool
	args := json.RawMessage(`{"query": "Paris", "limit": 5}`)
	ctx := memory.ToolContext{
		SessionID: "test-session",
	}

	result, err := provider.HandleToolCall("memory_recall", args, ctx)
	if err != nil {
		t.Fatalf("HandleToolCall failed: %v", err)
	}

	// Verify result contains expected data
	if !strings.Contains(result, "Paris") {
		t.Errorf("Expected result to contain 'Paris', got: %s", result)
	}

	// Verify result is valid JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(result), &jsonData); err != nil {
		t.Errorf("Result should be valid JSON, got: %s", result)
	}
}

// TestBuiltin_HandleToolCall_Save verifies that HandleToolCall correctly
// dispatches memory_save tool calls.
func TestBuiltin_HandleToolCall_Save(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	if err := provider.Initialize("test-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer provider.Close()

	// Call memory_save tool
	args := json.RawMessage(`{"key": "test-fact", "content": "Berlin is the capital of Germany", "source": "user"}`)
	ctx := memory.ToolContext{
		SessionID: "test-session",
	}

	result, err := provider.HandleToolCall("memory_save", args, ctx)
	if err != nil {
		t.Fatalf("HandleToolCall failed: %v", err)
	}

	// Verify result indicates success
	if !strings.Contains(result, "success") {
		t.Errorf("Expected result to indicate success, got: %s", result)
	}

	// Verify the fact was saved by recalling it
	recallArgs := json.RawMessage(`{"query": "Berlin", "limit": 5}`)
	recallResult, err := provider.HandleToolCall("memory_recall", recallArgs, ctx)
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	if !strings.Contains(recallResult, "Berlin") {
		t.Errorf("Expected recall result to contain 'Berlin', got: %s", recallResult)
	}
}

// TestBuiltin_HandleToolCall_UnknownTool verifies that HandleToolCall
// returns error for unknown tools.
func TestBuiltin_HandleToolCall_UnknownTool(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	provider, err := NewBuiltin(dbPath, nil)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	args := json.RawMessage(`{}`)
	ctx := memory.ToolContext{
		SessionID: "test-session",
	}

	_, err = provider.HandleToolCall("unknown_tool", args, ctx)
	if err == nil {
		t.Fatal("HandleToolCall should return error for unknown tool")
	}

	var toolErr *memory.ErrToolNotHandled
	if !errors.As(err, &toolErr) {
		t.Errorf("Expected ErrToolNotHandled, got: %v", err)
	}
}

// findSchema is a helper to find a schema by name.
func findSchema(schemas []memory.ToolSchema, name string) *memory.ToolSchema {
	for i := range schemas {
		if schemas[i].Name == name {
			return &schemas[i]
		}
	}
	return nil
}
