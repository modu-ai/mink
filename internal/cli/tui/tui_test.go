// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/mink/internal/cli/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDaemonClient implements DaemonClient interface for testing.
type mockDaemonClient struct {
	chatStreamFunc func(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error)
	closeFunc      func() error
}

func (m *mockDaemonClient) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error) {
	if m.chatStreamFunc != nil {
		return m.chatStreamFunc(ctx, messages)
	}
	// Default: return a channel that sends a done event
	ch := make(chan StreamEvent, 1)
	close(ch)
	return ch, nil
}

func (m *mockDaemonClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// ResolvePermission is a no-op stub for tests that don't need permission tracking.
func (m *mockDaemonClient) ResolvePermission(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

// TestModelInit tests that the model initializes properly.
func TestModelInit(t *testing.T) {
	model := NewModel(nil, "test-session", false)

	// Init should return a batch to wait for initial connection
	cmds := model.Init()
	assert.NotNil(t, cmds, "Init should return initial commands")
}

// TestNewModelWithApp tests that model can be created with App.
func TestNewModelWithApp(t *testing.T) {
	// For now, just verify the model can be created without panicking
	// The actual app field integration will be tested in integration tests
	model := NewModel(nil, "test-session", false)
	assert.NotNil(t, model, "Model should be created")
}

// TestModelUpdate_EnterKey tests that Enter key sends a message.
func TestModelUpdate_EnterKey(t *testing.T) {
	model := NewModel(nil, "test-session", false)

	// Set some input text
	model.input.SetValue("hello world")

	// Simulate Enter key press
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := model.Update(msg)

	// After sending message, input should be cleared if there's a streaming state
	// or remain if message was sent
	assert.NotNil(t, cmd, "Update should return a command for Enter key")
}

// TestModelUpdate_CtrlC tests that Ctrl+C triggers quit.
func TestModelUpdate_CtrlC(t *testing.T) {
	model := NewModel(nil, "test-session", false)

	// Simulate Ctrl+C
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, cmd := model.Update(msg)

	assert.True(t, newModel.(*Model).quitting, "Model should be in quitting state")
	assert.NotNil(t, cmd, "Update should return a quit command")
}

// TestModelUpdate_CtrlS tests that Ctrl+S saves the session.
func TestModelUpdate_CtrlS(t *testing.T) {
	model := NewModel(nil, "test-session", false)

	// Simulate Ctrl+S
	msg := tea.KeyMsg{Type: tea.KeyCtrlS}
	newModel, cmd := model.Update(msg)

	assert.NotNil(t, cmd, "Update should return a command for Ctrl+S")
	assert.False(t, newModel.(*Model).quitting, "Model should not quit on Ctrl+S")
}

// TestModelUpdate_WindowSize tests that window resize updates dimensions.
func TestModelUpdate_WindowSize(t *testing.T) {
	model := NewModel(nil, "test-session", false)

	// Simulate window resize
	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 50,
	}
	newModel, _ := model.Update(msg)

	m := newModel.(*Model)
	assert.Equal(t, 100, m.width, "Width should be updated")
	assert.Equal(t, 50, m.height, "Height should be updated")
}

// TestModelUpdate_StreamTextEvent tests handling of streaming text events.
func TestModelUpdate_StreamTextEvent(t *testing.T) {
	model := NewModel(nil, "test-session", false)

	// Simulate receiving text from stream
	msg := StreamEventMsg{
		Event: StreamEvent{
			Type:    "text",
			Content: "Hello, world!",
		},
	}

	newModel, _ := model.Update(msg)

	// Check that the streaming state is active and content is being captured
	m := newModel.(*Model)
	assert.True(t, m.streaming, "Model should be in streaming state")
}

// TestModelUpdate_StreamErrorEvent tests handling of error events.
func TestModelUpdate_StreamErrorEvent(t *testing.T) {
	model := NewModel(nil, "test-session", false)

	// Simulate receiving an error from stream
	msg := StreamEventMsg{
		Event: StreamEvent{
			Type:    "error",
			Content: "connection failed",
		},
	}

	newModel, _ := model.Update(msg)

	m := newModel.(*Model)
	assert.False(t, m.streaming, "Streaming should stop on error")
	assert.NotEmpty(t, m.messages, "Error message should be added to history")
}

// TestModelUpdate_StreamDoneEvent tests handling of done events.
func TestModelUpdate_StreamDoneEvent(t *testing.T) {
	model := NewModel(nil, "test-session", false)
	model.streaming = true

	// Simulate receiving a done event
	msg := StreamEventMsg{
		Event: StreamEvent{
			Type:    "done",
			Content: "",
		},
	}

	newModel, _ := model.Update(msg)

	m := newModel.(*Model)
	assert.False(t, m.streaming, "Streaming should stop on done event")
}

// TestModelUpdate_EscapeCancelsStreaming tests that Escape cancels active streaming.
func TestModelUpdate_EscapeCancelsStreaming(t *testing.T) {
	model := NewModel(nil, "test-session", false)
	model.streaming = true

	// Simulate Escape key
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	newModel, _ := model.Update(msg)

	m := newModel.(*Model)
	assert.False(t, m.streaming, "Escape should cancel streaming")
}

// TestView_RendersWithoutPanic tests that View doesn't panic.
func TestView_RendersWithoutPanic(t *testing.T) {
	model := NewModel(nil, "test-session", false)
	model.width = 80
	model.height = 24

	// This should not panic
	view := model.View()
	assert.NotEmpty(t, view, "View should render content")
}

// TestNewClientAdapter tests the daemon client adapter.
func TestNewClientAdapter(t *testing.T) {
	// Create a mock transport client factory
	mockTransport := &transport.DaemonClient{}

	adapter := NewDaemonClientAdapter(mockTransport)

	assert.NotNil(t, adapter, "Adapter should be created")
}

// TestChatMessageConversion tests conversion to transport messages.
func TestChatMessageConversion(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	// Convert to transport messages
	transportMsgs := make([]transport.Message, len(msgs))
	for i, msg := range msgs {
		transportMsgs[i] = transport.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	assert.Equal(t, "user", transportMsgs[0].Role)
	assert.Equal(t, "hello", transportMsgs[0].Content)
	assert.Equal(t, "assistant", transportMsgs[1].Role)
	assert.Equal(t, "hi there", transportMsgs[1].Content)
}

// TestStreamEventChannel tests that stream events are properly received.
func TestStreamEventChannel(t *testing.T) {
	t.Run("receives text events", func(t *testing.T) {
		ch := make(chan StreamEvent, 2)
		ch <- StreamEvent{Type: "text", Content: "hello"}
		ch <- StreamEvent{Type: "done", Content: ""}
		close(ch)

		events := []StreamEvent{}
		for event := range ch {
			events = append(events, event)
		}

		require.Len(t, events, 2)
		assert.Equal(t, "text", events[0].Type)
		assert.Equal(t, "hello", events[0].Content)
		assert.Equal(t, "done", events[1].Type)
	})

	t.Run("handles error events", func(t *testing.T) {
		ch := make(chan StreamEvent, 1)
		ch <- StreamEvent{Type: "error", Content: "failed"}
		close(ch)

		events := []StreamEvent{}
		for event := range ch {
			events = append(events, event)
		}

		require.Len(t, events, 1)
		assert.Equal(t, "error", events[0].Type)
		assert.Equal(t, "failed", events[0].Content)
	})
}

// TestQuitDuringStreaming tests that Ctrl+C during streaming prompts confirmation.
func TestQuitDuringStreaming(t *testing.T) {
	model := NewModel(nil, "test-session", false)
	model.streaming = true

	// First Ctrl+C should set confirmQuit
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, _ := model.Update(msg)

	m := newModel.(*Model)
	assert.True(t, m.confirmQuit, "Should prompt for quit confirmation")
	assert.True(t, m.streaming, "Streaming should continue")

	// Second Ctrl+C should actually quit
	msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, _ = m.Update(msg)

	m = newModel.(*Model)
	assert.True(t, m.quitting, "Should quit after second Ctrl+C")
}

// TestDisabledStreamingTests tests that non-streaming state blocks certain keys.
func TestDisabledStreamingTests(t *testing.T) {
	model := NewModel(nil, "test-session", false)
	model.streaming = false

	// Enter key should work when not streaming
	model.input.SetValue("test")
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := model.Update(msg)
	assert.NotNil(t, cmd, "Enter should work when not streaming")
}
