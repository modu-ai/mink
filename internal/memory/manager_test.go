package memory

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

// managerMockProvider is a test implementation of MemoryProvider for manager tests.
type managerMockProvider struct {
	name            string
	tools           []ToolSchema
	avail           bool
	initFn          func(sessionID string, ctx SessionContext) error
	onTurnStartFn   func(sessionID string, turn int, msg Message)
	onSessionEndFn  func(sessionID string, messages []Message)
	onPreCompressFn func(sessionID string, messages []Message) string
}

func (m *managerMockProvider) Name() string      { return m.name }
func (m *managerMockProvider) IsAvailable() bool { return m.avail }
func (m *managerMockProvider) Initialize(sessionID string, ctx SessionContext) error {
	if m.initFn != nil {
		return m.initFn(sessionID, ctx)
	}
	return nil
}
func (m *managerMockProvider) GetToolSchemas() []ToolSchema { return m.tools }
func (m *managerMockProvider) HandleToolCall(toolName string, args json.RawMessage, tctx ToolContext) (string, error) {
	return "{}", nil
}
func (m *managerMockProvider) OnTurnStart(sessionID string, turn int, msg Message) {
	if m.onTurnStartFn != nil {
		m.onTurnStartFn(sessionID, turn, msg)
	}
}
func (m *managerMockProvider) OnSessionEnd(sessionID string, messages []Message) {
	if m.onSessionEndFn != nil {
		m.onSessionEndFn(sessionID, messages)
	}
}
func (m *managerMockProvider) OnPreCompress(sessionID string, messages []Message) string {
	if m.onPreCompressFn != nil {
		return m.onPreCompressFn(sessionID, messages)
	}
	return ""
}
func (m *managerMockProvider) OnDelegation(sessionID, task, result string) {
	// No-op for tests
}
func (m *managerMockProvider) SystemPromptBlock() string {
	return ""
}
func (m *managerMockProvider) Prefetch(query, sessionID string) (RecallResult, error) {
	return RecallResult{}, nil
}
func (m *managerMockProvider) QueuePrefetch(query, sessionID string) {
	// No-op
}
func (m *managerMockProvider) SyncTurn(sessionID, userContent, assistantContent string) error {
	return nil
}

// TestManager_InitializeWithoutBuiltin_ReturnsErrBuiltinRequired tests AC-001
func TestManager_InitializeWithoutBuiltin_ReturnsErrBuiltinRequired(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Try to initialize without registering Builtin
	ctx := context.Background()
	err = manager.Initialize(ctx, "s1", SessionContext{})

	if err == nil {
		t.Fatal("Expected error when initializing without Builtin")
	}

	if !Is(err, ErrBuiltinRequired) {
		t.Errorf("Expected ErrBuiltinRequired, got: %v", err)
	}
}

// TestManager_SecondPlugin_ReturnsErrOnlyOnePluginAllowed tests AC-002
func TestManager_SecondPlugin_ReturnsErrOnlyOnePluginAllowed(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register Builtin
	builtin := &managerMockProvider{name: "builtin", tools: []ToolSchema{{Name: "builtin_tool"}}}
	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Register first plugin
	pluginA := &managerMockProvider{name: "plugin-a", tools: []ToolSchema{{Name: "plugin_a_tool"}}}
	if err := manager.RegisterPlugin(pluginA); err != nil {
		t.Fatalf("RegisterPlugin (first) failed: %v", err)
	}

	// Try to register second plugin
	pluginB := &managerMockProvider{name: "plugin-b", tools: []ToolSchema{{Name: "plugin_b_tool"}}}
	err = manager.RegisterPlugin(pluginB)

	if err == nil {
		t.Fatal("Expected error when registering second plugin")
	}

	if !Is(err, ErrOnlyOnePluginAllowed) {
		t.Errorf("Expected ErrOnlyOnePluginAllowed, got: %v", err)
	}

	// Verify only Builtin and first plugin are registered
	schemas := manager.GetAllToolSchemas()
	if len(schemas) != 2 {
		t.Errorf("Expected 2 tool schemas, got %d", len(schemas))
	}
}

// TestManager_NameCollision_CaseInsensitive tests AC-003
func TestManager_NameCollision_CaseInsensitive(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register Builtin
	builtin := &managerMockProvider{name: "builtin", tools: []ToolSchema{{Name: "builtin_tool"}}}
	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Try to register plugin with colliding name (case-insensitive)
	plugin := &managerMockProvider{name: "Builtin", tools: []ToolSchema{{Name: "plugin_tool"}}}
	err = manager.RegisterPlugin(plugin)

	if err == nil {
		t.Fatal("Expected error for name collision (case-insensitive)")
	}

	if !Is(err, ErrNameCollision) {
		t.Errorf("Expected ErrNameCollision, got: %v", err)
	}
}

// TestManager_ToolNameCollision_AtRegistration tests AC-004
func TestManager_ToolNameCollision_AtRegistration(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register Builtin with memory_recall tool
	builtin := &managerMockProvider{
		name: "builtin",
		tools: []ToolSchema{
			{Name: "memory_recall", Description: "Recall facts"},
		},
	}
	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Try to register plugin with conflicting tool name
	plugin := &managerMockProvider{
		name: "plugin-a",
		tools: []ToolSchema{
			{Name: "memory_recall", Description: "Different recall"}, // Collision!
		},
	}
	err = manager.RegisterPlugin(plugin)

	if err == nil {
		t.Fatal("Expected error for tool name collision")
	}

	if !Is(err, ErrToolNameCollision) {
		t.Errorf("Expected ErrToolNameCollision, got: %v", err)
	}

	// Verify Builtin state unchanged
	schemas := manager.GetAllToolSchemas()
	if len(schemas) != 1 {
		t.Errorf("Expected 1 tool schema, got %d", len(schemas))
	}
	if schemas[0].Name != "memory_recall" {
		t.Errorf("Expected memory_recall, got %s", schemas[0].Name)
	}
}

// TestManager_BuiltinOnlyFlow_NoError tests AC-016
func TestManager_BuiltinOnlyFlow_NoError(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()
	cfg.Plugin.Name = "" // No plugin configured

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register only Builtin
	builtin := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{
			{Name: "memory_recall", Description: "Recall facts"},
			{Name: "memory_save", Description: "Save fact"},
		},
	}
	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Initialize should work
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Errorf("Initialize failed: %v", err)
	}

	// Should have 2 tool schemas
	schemas := manager.GetAllToolSchemas()
	if len(schemas) != 2 {
		t.Errorf("Expected 2 tool schemas, got %d", len(schemas))
	}

	// HandleToolCall should route correctly
	result, err := manager.HandleToolCall(ctx, "memory_recall", nil, ToolContext{})
	if err != nil {
		t.Errorf("HandleToolCall failed: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

// TestManager_RegisterBuiltin_InvalidName_ReturnsErr
func TestManager_RegisterBuiltin_InvalidName_ReturnsErr(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Try to register provider with invalid name (contains uppercase)
	provider := &managerMockProvider{name: "InvalidName", tools: []ToolSchema{}}
	err = manager.RegisterBuiltin(provider)

	if err == nil {
		t.Fatal("Expected error for invalid provider name")
	}

	if !Is(err, ErrInvalidProviderName) {
		t.Errorf("Expected ErrInvalidProviderName, got: %v", err)
	}
}

// TestManager_RegisterPlugin_BeforeBuiltin_ReturnsErrBuiltinRequired
func TestManager_RegisterPlugin_BeforeBuiltin_ReturnsErrBuiltinRequired(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Try to register plugin before Builtin
	plugin := &managerMockProvider{name: "plugin-a", tools: []ToolSchema{{Name: "plugin_tool"}}}
	err = manager.RegisterPlugin(plugin)

	if err == nil {
		t.Fatal("Expected error when registering plugin before builtin")
	}

	if !Is(err, ErrBuiltinRequired) {
		t.Errorf("Expected ErrBuiltinRequired, got: %v", err)
	}
}
