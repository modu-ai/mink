package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestManager_DispatchOrder_InitializeForward_SessionEndReverse tests AC-006
func TestManager_DispatchOrder_InitializeForward_SessionEndReverse(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Track call order
	var mu sync.Mutex
	var initOrder []string
	var sessionEndOrder []string

	builtin := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "builtin_tool"}},
		initFn: func(sessionID string, ctx SessionContext) error {
			mu.Lock()
			initOrder = append(initOrder, "builtin")
			mu.Unlock()
			return nil
		},
	}
	builtin.onSessionEndFn = func(sessionID string, messages []Message) {
		mu.Lock()
		sessionEndOrder = append(sessionEndOrder, "builtin")
		mu.Unlock()
	}

	plugin := &managerMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{{Name: "plugin_tool"}},
		initFn: func(sessionID string, ctx SessionContext) error {
			mu.Lock()
			initOrder = append(initOrder, "plugin-a")
			mu.Unlock()
			return nil
		},
	}
	plugin.onSessionEndFn = func(sessionID string, messages []Message) {
		mu.Lock()
		sessionEndOrder = append(sessionEndOrder, "plugin-a")
		mu.Unlock()
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}
	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Errorf("Initialize failed: %v", err)
	}

	// OnSessionEnd
	manager.OnSessionEnd(ctx, "s1", []Message{})

	// Verify forward order for Initialize
	if len(initOrder) != 2 {
		t.Fatalf("Expected 2 init calls, got %d", len(initOrder))
	}
	if initOrder[0] != "builtin" || initOrder[1] != "plugin-a" {
		t.Errorf("Initialize order wrong: %v", initOrder)
	}

	// Verify reverse order for OnSessionEnd
	if len(sessionEndOrder) != 2 {
		t.Fatalf("Expected 2 OnSessionEnd calls, got %d", len(sessionEndOrder))
	}
	if sessionEndOrder[0] != "plugin-a" || sessionEndOrder[1] != "builtin" {
		t.Errorf("OnSessionEnd order wrong: %v", sessionEndOrder)
	}
}

// TestManager_ProviderPanicIsolated tests AC-007
func TestManager_ProviderPanicIsolated(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	builtinCalled := false
	builtin := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "builtin_tool"}},
		onTurnStartFn: func(sessionID string, turn int, msg Message) {
			builtinCalled = true
		},
	}

	// Plugin that panics
	plugin := &managerMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{{Name: "plugin_tool"}},
		onTurnStartFn: func(sessionID string, turn int, msg Message) {
			panic("intentional panic")
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}
	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// OnTurnStart should not panic, plugin should be skipped
	manager.OnTurnStart(ctx, "s1", 1, Message{})

	if !builtinCalled {
		t.Error("Builtin was not called after plugin panic")
	}
}

// TestManager_IsAvailableFalse_SkipsProvider tests AC-008
func TestManager_IsAvailableFalse_SkipsProvider(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	builtinCalled := false
	builtin := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "builtin_tool"}},
		onTurnStartFn: func(sessionID string, turn int, msg Message) {
			builtinCalled = true
		},
	}

	// Plugin with IsAvailable=false
	pluginCalled := false
	plugin := &managerMockProvider{
		name:  "plugin-a",
		avail: false, // Not available
		tools: []ToolSchema{{Name: "plugin_tool"}},
		onTurnStartFn: func(sessionID string, turn int, msg Message) {
			pluginCalled = true
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}
	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	manager.OnTurnStart(ctx, "s1", 1, Message{})

	if !builtinCalled {
		t.Error("Builtin was not called")
	}
	if pluginCalled {
		t.Error("Plugin was called despite IsAvailable=false")
	}
}

// TestManager_OnTurnStart_DispatchBudget50ms tests AC-011
func TestManager_OnTurnStart_DispatchBudget50ms(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	builtin := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "builtin_tool"}},
	}

	// Plugin that takes too long (>40ms timeout)
	plugin := &managerMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{{Name: "plugin_tool"}},
		onTurnStartFn: func(sessionID string, turn int, msg Message) {
			time.Sleep(100 * time.Millisecond) // Exceeds 40ms timeout
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}
	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	start := time.Now()
	manager.OnTurnStart(ctx, "s1", 1, Message{})
	elapsed := time.Since(start)

	// Should complete within 50ms total budget (with some margin)
	if elapsed > 70*time.Millisecond {
		t.Errorf("OnTurnStart took too long: %v, expected <= 70ms", elapsed)
	}
}

// TestManager_OnPreCompress_Aggregation tests AC-018
func TestManager_OnPreCompress_Aggregation(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	builtin := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "builtin_tool"}},
		onPreCompressFn: func(sessionID string, messages []Message) string {
			return "fact-1"
		},
	}

	plugin := &managerMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{{Name: "plugin_tool"}},
		onPreCompressFn: func(sessionID string, messages []Message) string {
			return "fact-2"
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}
	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	result := manager.OnPreCompress(ctx, "s1", []Message{})

	expected := "fact-1\n\nfact-2"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestManager_InitErrorSuppressesUntilNextSession tests AC-019
func TestManager_InitErrorSuppressesUntilNextSession(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	builtinCalled := false
	builtin := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "builtin_tool"}},
		onTurnStartFn: func(sessionID string, turn int, msg Message) {
			builtinCalled = true
		},
	}

	// Plugin that fails init in session s1, succeeds in s2
	pluginInitCount := 0
	pluginCalled := false
	plugin := &managerMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{{Name: "plugin_tool"}},
		initFn: func(sessionID string, ctx SessionContext) error {
			pluginInitCount++
			if sessionID == "s1" {
				return fmt.Errorf("backend down")
			}
			return nil
		},
	}
	plugin.onTurnStartFn = func(sessionID string, turn int, msg Message) {
		pluginCalled = true
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}
	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	ctx := context.Background()

	// Session s1: plugin init fails
	err = manager.Initialize(ctx, "s1", SessionContext{})
	if err == nil {
		t.Error("Expected error when plugin init fails")
	}

	// Plugin should not be called in s1
	manager.OnTurnStart(ctx, "s1", 1, Message{})
	if pluginCalled {
		t.Error("Plugin was called in s1 despite init failure")
	}
	if !builtinCalled {
		t.Error("Builtin was not called in s1")
	}

	// Reset for s2
	builtinCalled = false

	// Session s2: plugin init succeeds
	if err := manager.Initialize(ctx, "s2", SessionContext{}); err != nil {
		t.Errorf("Initialize s2 failed: %v", err)
	}

	if pluginInitCount != 2 {
		t.Errorf("Expected 2 plugin init calls, got %d", pluginInitCount)
	}

	// Plugin should be called in s2
	manager.OnTurnStart(ctx, "s2", 1, Message{})
	if !pluginCalled {
		t.Error("Plugin was not called in s2 after successful init")
	}
	if !builtinCalled {
		t.Error("Builtin was not called in s2")
	}
}

// TestManager_OnPreCompress_EmptyStringNoWrap tests AC-022
func TestManager_OnPreCompress_EmptyStringNoWrap(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultMemoryConfig()

	manager, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Case A: Both return non-empty
	builtin := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "builtin_tool"}},
		onPreCompressFn: func(sessionID string, messages []Message) string {
			return "fact-1"
		},
	}

	plugin := &managerMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{{Name: "plugin_tool"}},
		onPreCompressFn: func(sessionID string, messages []Message) string {
			return "fact-2"
		},
	}

	if err := manager.RegisterBuiltin(builtin); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}
	if err := manager.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	ctx := context.Background()
	if err := manager.Initialize(ctx, "s1", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	result := manager.OnPreCompress(ctx, "s1", []Message{})
	if result != "fact-1\n\nfact-2" {
		t.Errorf("Case A: Expected 'fact-1\\n\\nfact-2', got %q", result)
	}

	// Case B: Both return empty
	builtin2 := &managerMockProvider{
		name:  "builtin",
		avail: true,
		tools: []ToolSchema{{Name: "builtin_tool"}},
		onPreCompressFn: func(sessionID string, messages []Message) string {
			return ""
		},
	}

	plugin2 := &managerMockProvider{
		name:  "plugin-a",
		avail: true,
		tools: []ToolSchema{{Name: "plugin_tool"}},
		onPreCompressFn: func(sessionID string, messages []Message) string {
			return ""
		},
	}

	manager2, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if err := manager2.RegisterBuiltin(builtin2); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}
	if err := manager2.RegisterPlugin(plugin2); err != nil {
		t.Fatalf("RegisterPlugin failed: %v", err)
	}

	if err := manager2.Initialize(ctx, "s2", SessionContext{}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	result2 := manager2.OnPreCompress(ctx, "s2", []Message{})
	if result2 != "" {
		t.Errorf("Case B: Expected empty string, got %q", result2)
	}
}
