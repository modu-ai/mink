// Package builtin implements the BuiltinProvider with SQLite FTS5 backend.
package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/memory"
	"go.uber.org/zap"
)

// TestIntegration_BuiltinOnly_FullLifecycle verifies the complete lifecycle
// with MemoryManager using only BuiltinProvider (AC-016).
func TestIntegration_BuiltinOnly_FullLifecycle(t *testing.T) {
	t.Parallel()

	// Create temporary directory for test
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	// Create MemoryManager
	logger := zap.NewNop()
	cfg := memory.MemoryConfig{
		Builtin: memory.BuiltinConfig{
			DBPath:  dbPath,
			MaxRows: 1000,
		},
		Plugin: memory.PluginConfig{
			Name: "", // No plugin
		},
	}

	manager, err := memory.New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Create and register BuiltinProvider
	builtinProvider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	if err := manager.RegisterBuiltin(builtinProvider); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Step 1: Initialize
	sessionID := "test-session-123"
	ctx := context.Background()
	sctx := memory.SessionContext{
		Platform:     "test",
		AgentContext: map[string]string{"user_id": "user-456"},
	}

	if err := manager.Initialize(ctx, sessionID, sctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Step 2: OnTurnStart
	turnNumber := 1
	message := memory.Message{
		Role:    "user",
		Content: "What is the capital of France?",
	}

	manager.OnTurnStart(ctx, sessionID, turnNumber, message)

	// Step 3: SyncTurn (save some data)
	userContent := "The capital of France is Paris"
	assistantContent := "Correct, Paris is the capital of France."

	if err := builtinProvider.SyncTurn(sessionID, userContent, assistantContent); err != nil {
		t.Fatalf("SyncTurn failed: %v", err)
	}

	// Step 4: Prefetch (should now return data)
	recallResult, err := builtinProvider.Prefetch("Paris", sessionID)
	if err != nil {
		t.Fatalf("Prefetch failed: %v", err)
	}

	if len(recallResult.Items) == 0 {
		t.Error("Expected non-empty recall result after SyncTurn")
	}

	// Verify content
	found := false
	for _, item := range recallResult.Items {
		if strings.Contains(item.Content, "Paris") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find 'Paris' in recall results")
	}

	// Step 5: Get tool schemas
	schemas := manager.GetAllToolSchemas()
	if len(schemas) == 0 {
		t.Error("Expected tool schemas from builtin provider")
	}

	// Verify builtin tools are present
	hasRecall := false
	hasSave := false
	for _, schema := range schemas {
		if schema.Name == "memory_recall" {
			hasRecall = true
		}
		if schema.Name == "memory_save" {
			hasSave = true
		}
	}

	if !hasRecall {
		t.Error("Expected memory_recall tool schema")
	}

	if !hasSave {
		t.Error("Expected memory_save tool schema")
	}

	// Step 6: HandleToolCall
	toolCtx := memory.ToolContext{
		SessionID: sessionID,
	}

	// Test memory_recall
	recallArgs := []byte(`{"query": "Paris", "limit": 5}`)
	recallJSON, err := manager.HandleToolCall(ctx, "memory_recall", recallArgs, toolCtx)
	if err != nil {
		t.Fatalf("HandleToolCall (recall) failed: %v", err)
	}

	if !strings.Contains(recallJSON, "Paris") {
		t.Errorf("Expected recall result to contain 'Paris', got: %s", recallJSON)
	}

	// Step 7: OnSessionEnd
	messages := []memory.Message{
		{Role: "user", Content: "What is the capital?"},
		{Role: "assistant", Content: "Paris"},
	}

	manager.OnSessionEnd(ctx, sessionID, messages)

	// All steps completed successfully
}

// TestIntegration_BuiltinOnly_SystemPromptBlock verifies that SystemPromptBlock
// works correctly through the manager.
func TestIntegration_BuiltinOnly_SystemPromptBlock(t *testing.T) {
	t.Parallel()

	// Create temporary directory for test
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	// Create MemoryManager
	logger := zap.NewNop()
	cfg := memory.MemoryConfig{
		Builtin: memory.BuiltinConfig{
			DBPath:  dbPath,
			MaxRows: 1000,
		},
		Plugin: memory.PluginConfig{
			Name: "", // No plugin
		},
	}

	manager, err := memory.New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Create and register BuiltinProvider
	builtinProvider, err := NewBuiltin(dbPath, logger)
	if err != nil {
		t.Fatalf("NewBuiltin failed: %v", err)
	}

	if err := manager.RegisterBuiltin(builtinProvider); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Get system prompt block
	sessionID := "test-session-456"
	block := manager.SystemPromptBlock(sessionID)

	// Should return empty string (no USER.md or MEMORY.md files)
	if block != "" {
		t.Errorf("Expected empty system prompt block, got: %s", block)
	}
}
