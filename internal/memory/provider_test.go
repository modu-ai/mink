package memory

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a minimal MemoryProvider implementation for interface tests.
// Uses a distinct name from managerMockProvider to avoid redeclaration.
type interfaceMockProvider struct {
	BaseProvider
	name string
}

func (m *interfaceMockProvider) Name() string {
	return m.name
}

func (m *interfaceMockProvider) IsAvailable() bool {
	return true
}

func (m *interfaceMockProvider) Initialize(sessionID string, ctx SessionContext) error {
	return nil
}

func (m *interfaceMockProvider) GetToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			Name:        "test_tool",
			Description: "A test tool",
			Parameters:  json.RawMessage(`{"type":"object"}`),
			Owner:       m.name,
		},
	}
}

// TestMemoryProvider_Interface_SatisfiedByMock verifies that mockProvider satisfies the interface.
func TestMemoryProvider_Interface_SatisfiedByMock(t *testing.T) {
	// This test compiles if interfaceMockProvider implements MemoryProvider.
	var _ MemoryProvider = (*interfaceMockProvider)(nil)

	p := &interfaceMockProvider{name: "test"}
	assert.Equal(t, "test", p.Name())
	assert.True(t, p.IsAvailable())
}

// TestBaseProvider_SystemPromptBlock_ReturnsEmpty verifies no-op default returns empty string.
func TestBaseProvider_SystemPromptBlock_ReturnsEmpty(t *testing.T) {
	var bp BaseProvider
	assert.Empty(t, bp.SystemPromptBlock())
}

// TestBaseProvider_Prefetch_ReturnsEmptyResult verifies no-op default returns empty result.
func TestBaseProvider_Prefetch_ReturnsEmptyResult(t *testing.T) {
	var bp BaseProvider
	result, err := bp.Prefetch("test query", "session123")
	require.NoError(t, err)
	assert.Empty(t, result.Items)
	assert.Equal(t, 0, result.TotalTokens)
}

// TestBaseProvider_AllOptionalMethods_NoPanic verifies all optional methods are callable without panic.
func TestBaseProvider_AllOptionalMethods_NoPanic(t *testing.T) {
	var bp BaseProvider
	msg := Message{Role: "user", Content: "test"}
	messages := []Message{msg}

	// These should not panic
	assert.NotPanics(t, func() {
		bp.SystemPromptBlock()
	})

	assert.NotPanics(t, func() {
		bp.Prefetch("query", "session")
	})

	assert.NotPanics(t, func() {
		bp.QueuePrefetch("query", "session")
	})

	assert.NotPanics(t, func() {
		bp.SyncTurn("session", "user msg", "assistant msg")
	})

	assert.NotPanics(t, func() {
		bp.HandleToolCall("tool", json.RawMessage(`{}`), ToolContext{})
	})

	assert.NotPanics(t, func() {
		bp.OnTurnStart("session", 1, msg)
	})

	assert.NotPanics(t, func() {
		bp.OnSessionEnd("session", messages)
	})

	assert.NotPanics(t, func() {
		bp.OnPreCompress("session", messages)
	})

	assert.NotPanics(t, func() {
		bp.OnDelegation("session", "task", "result")
	})
}

// TestBaseProvider_QueuePrefetch_NoBlock verifies QueuePrefetch returns immediately.
func TestBaseProvider_QueuePrefetch_NoBlock(t *testing.T) {
	var bp BaseProvider

	// QueuePrefetch should return immediately (no blocking)
	done := make(chan bool)
	go func() {
		bp.QueuePrefetch("query", "session")
		close(done)
	}()

	select {
	case <-done:
		// OK - returned immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("QueuePrefetch blocked - should return immediately")
	}
}
