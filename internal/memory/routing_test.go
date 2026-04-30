package memory

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

// routingMockProvider is a specialized mock for routing tests with configurable functions.
type routingMockProvider struct {
	name                string
	avail               bool
	tools               []ToolSchema
	systemPromptBlockFn func() string
	handleToolCallFn    func(toolName string, args json.RawMessage, tctx ToolContext) (string, error)
	queuePrefetchFn     func(query, sessionID string)
	onSessionEndFn      func(sessionID string, messages []Message)
	onPreCompressFn     func(sessionID string, messages []Message) string
}

func (m *routingMockProvider) Name() string      { return m.name }
func (m *routingMockProvider) IsAvailable() bool { return m.avail }
func (m *routingMockProvider) Initialize(sessionID string, ctx SessionContext) error {
	return nil
}
func (m *routingMockProvider) GetToolSchemas() []ToolSchema { return m.tools }
func (m *routingMockProvider) SystemPromptBlock() string {
	if m.systemPromptBlockFn != nil {
		return m.systemPromptBlockFn()
	}
	return ""
}
func (m *routingMockProvider) HandleToolCall(toolName string, args json.RawMessage, tctx ToolContext) (string, error) {
	if m.handleToolCallFn != nil {
		return m.handleToolCallFn(toolName, args, tctx)
	}
	return "{}", nil
}
func (m *routingMockProvider) QueuePrefetch(query, sessionID string) {
	if m.queuePrefetchFn != nil {
		m.queuePrefetchFn(query, sessionID)
	}
}
func (m *routingMockProvider) OnSessionEnd(sessionID string, messages []Message) {
	if m.onSessionEndFn != nil {
		m.onSessionEndFn(sessionID, messages)
	}
}
func (m *routingMockProvider) OnPreCompress(sessionID string, messages []Message) string {
	if m.onPreCompressFn != nil {
		return m.onPreCompressFn(sessionID, messages)
	}
	return ""
}
func (m *routingMockProvider) OnTurnStart(sessionID string, turn int, msg Message) {}
func (m *routingMockProvider) OnDelegation(sessionID, task, result string)         {}
func (m *routingMockProvider) Prefetch(query, sessionID string) (RecallResult, error) {
	return RecallResult{}, nil
}
func (m *routingMockProvider) SyncTurn(sessionID, userContent, assistantContent string) error {
	return nil
}

// TestMemoryManager_SystemPromptBlock_Aggregation tests AC-009.
func TestMemoryManager_SystemPromptBlock_Aggregation(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register Builtin with SystemPromptBlock
	builtin := &routingMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{},
		systemPromptBlockFn: func() string {
			return "Builtin context"
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Register plugin with SystemPromptBlock
	plugin := &routingMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{},
		systemPromptBlockFn: func() string {
			return "Plugin context"
		},
	}

	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Get aggregated SystemPromptBlock
	block := manager.SystemPromptBlock("s1")

	expected := "Builtin context\n\nPlugin context"
	if block != expected {
		t.Errorf("Expected %q, got %q", expected, block)
	}
}

// TestMemoryManager_SystemPromptBlock_EmptyWhenUnavailable tests that unavailable providers are skipped.
func TestMemoryManager_SystemPromptBlock_EmptyWhenUnavailable(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register Builtin (available)
	builtin := &routingMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{},
		systemPromptBlockFn: func() string {
			return "Builtin context"
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Register plugin (unavailable)
	plugin := &routingMockProvider{
		name:  "plugin-a",
		avail: false, // Unavailable
		tools: []ToolSchema{},
		systemPromptBlockFn: func() string {
			return "Plugin context (should be skipped)"
		},
	}

	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Get SystemPromptBlock (should only include builtin)
	block := manager.SystemPromptBlock("s1")

	expected := "Builtin context"
	if block != expected {
		t.Errorf("Expected %q, got %q", expected, block)
	}
}

// TestMemoryManager_HandleToolCall_Routing tests AC-010.
func TestMemoryManager_HandleToolCall_Routing(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Track which provider handled the call
	builtinCalled := false
	pluginCalled := false

	// Register Builtin with memory_recall tool
	builtin := &routingMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "memory_recall", Description: "Recall facts"}},
		handleToolCallFn: func(toolName string, args json.RawMessage, tctx ToolContext) (string, error) {
			builtinCalled = true
			return `{"result": "builtin data"}`, nil
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Register plugin with plugin_search tool
	plugin := &routingMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{{Name: "plugin_search", Description: "Plugin search"}},
		handleToolCallFn: func(toolName string, args json.RawMessage, tctx ToolContext) (string, error) {
			pluginCalled = true
			return `{"result": "plugin data"}`, nil
		},
	}

	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test routing to builtin
	result, err := manager.HandleToolCall(ctx, "memory_recall", nil, ToolContext{})
	if err != nil {
		t.Fatalf("HandleToolCall failed for memory_recall: %v", err)
	}
	if !builtinCalled {
		t.Error("Expected builtin to be called for memory_recall")
	}
	if result != `{"result": "builtin data"}` {
		t.Errorf("Expected builtin result, got %s", result)
	}

	// Reset flags
	builtinCalled = false
	pluginCalled = false

	// Test routing to plugin
	result, err = manager.HandleToolCall(ctx, "plugin_search", nil, ToolContext{})
	if err != nil {
		t.Fatalf("HandleToolCall failed for plugin_search: %v", err)
	}
	if !pluginCalled {
		t.Error("Expected plugin to be called for plugin_search")
	}
	if result != `{"result": "plugin data"}` {
		t.Errorf("Expected plugin result, got %s", result)
	}

	// Test unknown tool
	_, err = manager.HandleToolCall(ctx, "unknown_tool", nil, ToolContext{})
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
}

// TestMemoryManager_QueuePrefetch_Async tests AC-012.
func TestMemoryManager_QueuePrefetch_Async(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Track calls
	builtinCalled := make(chan bool, 1)
	pluginCalled := make(chan bool, 1)

	// Register Builtin
	builtin := &routingMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{},
		queuePrefetchFn: func(query, sessionID string) {
			builtinCalled <- true
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Register plugin
	plugin := &routingMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{},
		queuePrefetchFn: func(query, sessionID string) {
			pluginCalled <- true
		},
	}

	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// QueuePrefetch should be async and non-blocking
	manager.QueuePrefetch(ctx, "test query", "s1")

	// Wait for both providers to be called
	<-builtinCalled
	<-pluginCalled
}

// TestMemoryManager_QueuePrefetch_PanicRecovery tests that panics are recovered.
func TestMemoryManager_QueuePrefetch_PanicRecovery(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register Builtin that panics
	builtin := &routingMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{},
		queuePrefetchFn: func(query, sessionID string) {
			panic("intentional panic for testing")
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// QueuePrefetch should not panic (should recover)
	manager.QueuePrefetch(ctx, "test query", "s1")
}

// TestMemoryManager_MessagesNotRetained tests AC-020.
func TestMemoryManager_MessagesNotRetained(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Track received messages
	var receivedMessages [][]Message

	// Register two providers
	builtin := &routingMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{},
		onSessionEndFn: func(sessionID string, messages []Message) {
			// Store reference to messages
			receivedMessages = append(receivedMessages, messages)
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	plugin := &routingMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{},
		onSessionEndFn: func(sessionID string, messages []Message) {
			// Store reference to messages
			receivedMessages = append(receivedMessages, messages)
		},
	}

	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Call OnSessionEnd with original messages
	originalMessages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	manager.OnSessionEnd(ctx, "s1", originalMessages)

	// Verify both providers received messages
	if len(receivedMessages) != 2 {
		t.Fatalf("Expected 2 providers to receive messages, got %d", len(receivedMessages))
	}

	// Modify messages from first provider
	receivedMessages[0][0].Content = "Modified"

	// Verify second provider's messages are not affected (deep copy)
	if receivedMessages[1][0].Content == "Modified" {
		t.Error("Messages were not deep copied - modification affected second provider")
	}

	// Verify original messages are not affected
	if originalMessages[0].Content == "Modified" {
		t.Error("Messages were not deep copied - modification affected original")
	}
}

// TestMemoryManager_OnPreCompress_MessagesNotRetained tests AC-020 for OnPreCompress.
func TestMemoryManager_OnPreCompress_MessagesNotRetained(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Track received messages
	var receivedMessages [][]Message

	// Register two providers
	builtin := &routingMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{},
		onPreCompressFn: func(sessionID string, messages []Message) string {
			// Store reference to messages
			receivedMessages = append(receivedMessages, messages)
			return "hint"
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	plugin := &routingMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{},
		onPreCompressFn: func(sessionID string, messages []Message) string {
			// Store reference to messages
			receivedMessages = append(receivedMessages, messages)
			return "hint"
		},
	}

	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Call OnPreCompress with original messages
	originalMessages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	manager.OnPreCompress(ctx, "s1", originalMessages)

	// Verify both providers received messages
	if len(receivedMessages) != 2 {
		t.Fatalf("Expected 2 providers to receive messages, got %d", len(receivedMessages))
	}

	// Modify messages from first provider
	receivedMessages[0][0].Content = "Modified"

	// Verify second provider's messages are not affected (deep copy)
	if receivedMessages[1][0].Content == "Modified" {
		t.Error("Messages were not deep copied - modification affected second provider")
	}

	// Verify original messages are not affected
	if originalMessages[0].Content == "Modified" {
		t.Error("Messages were not deep copied - modification affected original")
	}
}
