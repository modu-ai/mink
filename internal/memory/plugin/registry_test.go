// Package plugin provides the plugin adapter and factory registry for external memory providers.
package plugin

import (
	"encoding/json"
	"testing"

	"github.com/modu-ai/goose/internal/memory"
)

// mockPlugin is a mock implementation of MemoryProvider for testing.
type mockPlugin struct {
	memory.BaseProvider
	name string
}

func (m *mockPlugin) Name() string {
	return m.name
}

func (m *mockPlugin) IsAvailable() bool {
	return true
}

func (m *mockPlugin) Initialize(sessionID string, ctx memory.SessionContext) error {
	return nil
}

func (m *mockPlugin) GetToolSchemas() []memory.ToolSchema {
	return nil
}

func (m *mockPlugin) HandleToolCall(toolName string, args json.RawMessage, ctx memory.ToolContext) (string, error) {
	return "", nil
}

// mockPluginFactory creates a mock plugin instance.
func mockPluginFactory(config any) (memory.MemoryProvider, error) {
	return &mockPlugin{name: "mock-plugin"}, nil
}

// TestPlugin_FactoryRegistry_KnownPlugin_Succeeds verifies that Lookup
// successfully creates a plugin instance for a registered name (AC-021 case A).
func TestPlugin_FactoryRegistry_KnownPlugin_Succeeds(t *testing.T) {
	t.Parallel()

	// Register a mock plugin factory
	RegisterFactory("mock-plugin", mockPluginFactory)

	// Lookup should succeed
	provider, err := Lookup("mock-plugin", nil)
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	if provider.Name() != "mock-plugin" {
		t.Errorf("Expected provider name 'mock-plugin', got: %s", provider.Name())
	}
}

// TestPlugin_FactoryRegistry_UnknownPlugin_ReturnsErrUnknownPlugin verifies that
// Lookup returns memory.ErrUnknownPlugin for unregistered names (AC-021 case B).
func TestPlugin_FactoryRegistry_UnknownPlugin_ReturnsErrUnknownPlugin(t *testing.T) {
	t.Parallel()

	// Try to lookup an unregistered plugin
	_, err := Lookup("non-existent-plugin", nil)

	if err == nil {
		t.Fatal("Lookup should return error for unknown plugin")
	}

	// Verify it's the correct error type
	if !memory.IsErrUnknownPlugin(err) {
		t.Errorf("Expected ErrUnknownPlugin, got: %v", err)
	}
}

// TestPlugin_RegisterFactory_DuplicateName_Panics verifies that registering
// a duplicate factory name causes a panic (safety check).
func TestPlugin_RegisterFactory_DuplicateName_Panics(t *testing.T) {
	t.Parallel()

	// Register a factory
	RegisterFactory("duplicate-test", mockPluginFactory)

	// Attempting to register again should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when registering duplicate factory name")
		}
	}()

	RegisterFactory("duplicate-test", mockPluginFactory)
}

// TestPlugin_Adapter_SatisfiesMemoryProvider verifies that PluginProvider
// adapter satisfies the MemoryProvider interface (compile-time check).
func TestPlugin_Adapter_SatisfiesMemoryProvider(t *testing.T) {
	t.Parallel()

	// This is a compile-time interface check
	// If PluginProvider doesn't satisfy MemoryProvider, this won't compile
	var _ memory.MemoryProvider = &PluginProvider{}
}

// fullMockProvider implements every MemoryProvider method to exercise the
// adapter delegation paths (when wrapped provider implements optional methods).
type fullMockProvider struct {
	memory.BaseProvider
	name              string
	prefetchCalled    bool
	syncTurnCalled    bool
	systemPromptCalls int
	turnStartCalls    int
	sessionEndCalls   int
	preCompressCalls  int
	delegationCalls   int
	queuePrefetchHits int
}

func (m *fullMockProvider) Name() string      { return m.name }
func (m *fullMockProvider) IsAvailable() bool { return true }
func (m *fullMockProvider) Initialize(string, memory.SessionContext) error {
	return nil
}
func (m *fullMockProvider) GetToolSchemas() []memory.ToolSchema { return nil }
func (m *fullMockProvider) HandleToolCall(string, json.RawMessage, memory.ToolContext) (string, error) {
	return "ok", nil
}
func (m *fullMockProvider) Prefetch(string, string) (memory.RecallResult, error) {
	m.prefetchCalled = true
	return memory.RecallResult{}, nil
}
func (m *fullMockProvider) SyncTurn(string, string, string) error {
	m.syncTurnCalled = true
	return nil
}
func (m *fullMockProvider) SystemPromptBlock() string {
	m.systemPromptCalls++
	return "block"
}
func (m *fullMockProvider) OnTurnStart(string, int, memory.Message) { m.turnStartCalls++ }
func (m *fullMockProvider) OnSessionEnd(string, []memory.Message)   { m.sessionEndCalls++ }
func (m *fullMockProvider) OnPreCompress(string, []memory.Message) string {
	m.preCompressCalls++
	return "hint"
}
func (m *fullMockProvider) OnDelegation(string, string, string) { m.delegationCalls++ }
func (m *fullMockProvider) QueuePrefetch(string, string)        { m.queuePrefetchHits++ }

// TestPlugin_Adapter_DelegatesAllMethods exercises every adapter method to
// verify delegation to the wrapped provider when present.
func TestPlugin_Adapter_DelegatesAllMethods(t *testing.T) {
	t.Parallel()

	mock := &fullMockProvider{name: "wrapped"}
	adapter := NewPluginProvider(mock)

	if adapter.Name() != "wrapped" {
		t.Errorf("Name: expected wrapped, got %s", adapter.Name())
	}
	if !adapter.IsAvailable() {
		t.Error("IsAvailable: expected true")
	}
	if err := adapter.Initialize("s1", memory.SessionContext{}); err != nil {
		t.Errorf("Initialize: %v", err)
	}
	if got := adapter.GetToolSchemas(); got != nil {
		t.Errorf("GetToolSchemas: expected nil")
	}
	if out, err := adapter.HandleToolCall("t", nil, memory.ToolContext{}); err != nil || out != "ok" {
		t.Errorf("HandleToolCall: got %q err=%v", out, err)
	}
	if _, err := adapter.Prefetch("q", "s1"); err != nil || !mock.prefetchCalled {
		t.Errorf("Prefetch not delegated: err=%v called=%v", err, mock.prefetchCalled)
	}
	if err := adapter.SyncTurn("s1", "u", "a"); err != nil || !mock.syncTurnCalled {
		t.Errorf("SyncTurn not delegated: err=%v called=%v", err, mock.syncTurnCalled)
	}
	if got := adapter.SystemPromptBlock(); got != "block" || mock.systemPromptCalls != 1 {
		t.Errorf("SystemPromptBlock: got %q calls=%d", got, mock.systemPromptCalls)
	}
	adapter.OnTurnStart("s1", 1, memory.Message{})
	if mock.turnStartCalls != 1 {
		t.Errorf("OnTurnStart not delegated")
	}
	adapter.OnSessionEnd("s1", nil)
	if mock.sessionEndCalls != 1 {
		t.Errorf("OnSessionEnd not delegated")
	}
	if got := adapter.OnPreCompress("s1", nil); got != "hint" || mock.preCompressCalls != 1 {
		t.Errorf("OnPreCompress: got %q calls=%d", got, mock.preCompressCalls)
	}
	adapter.OnDelegation("s1", "task", "result")
	if mock.delegationCalls != 1 {
		t.Errorf("OnDelegation not delegated")
	}
	adapter.QueuePrefetch("q", "s1")
	if mock.queuePrefetchHits != 1 {
		t.Errorf("QueuePrefetch not delegated")
	}
}

// minimalMockProvider implements only the required interface methods (via
// BaseProvider no-ops). This exercises the adapter's default-fallback paths.
type minimalMockProvider struct {
	memory.BaseProvider
}

func (m *minimalMockProvider) Name() string                                   { return "minimal" }
func (m *minimalMockProvider) IsAvailable() bool                              { return true }
func (m *minimalMockProvider) Initialize(string, memory.SessionContext) error { return nil }
func (m *minimalMockProvider) GetToolSchemas() []memory.ToolSchema            { return nil }

// TestPlugin_Adapter_FallbackDefaults verifies adapter returns empty defaults
// when the wrapped provider only implements required methods (BaseProvider).
func TestPlugin_Adapter_FallbackDefaults(t *testing.T) {
	t.Parallel()

	adapter := NewPluginProvider(&minimalMockProvider{})

	// BaseProvider's Prefetch returns empty result and nil error
	if res, err := adapter.Prefetch("q", "s"); err != nil || len(res.Items) != 0 {
		t.Errorf("Prefetch fallback: items=%d err=%v", len(res.Items), err)
	}
	if err := adapter.SyncTurn("s", "u", "a"); err != nil {
		t.Errorf("SyncTurn fallback: %v", err)
	}
	if got := adapter.SystemPromptBlock(); got != "" {
		t.Errorf("SystemPromptBlock fallback: %q", got)
	}
	if got := adapter.OnPreCompress("s", nil); got != "" {
		t.Errorf("OnPreCompress fallback: %q", got)
	}
	// These should not panic
	adapter.OnTurnStart("s", 0, memory.Message{})
	adapter.OnSessionEnd("s", nil)
	adapter.OnDelegation("s", "t", "r")
	adapter.QueuePrefetch("q", "s")
}
