// Package tui provides snapshot tests for the /help slash command.
package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/modu-ai/mink/internal/cli/tui/snapshots"
)

// TestSlashHelp_LocalNoNetwork_Snapshot verifies that /help renders locally
// (without any ChatStream call) and that the output contains at least 6
// slash commands. AC-CLITUI-017
func TestSlashHelp_LocalNoNetwork_Snapshot(t *testing.T) {
	snapshots.SetupAsciiTermenv()

	// Arrange: track ChatStream calls — there must be zero.
	callCount := 0
	mock := &mockDaemonClient{
		chatStreamFunc: func(_ context.Context, _ []ChatMessage) (<-chan StreamEvent, error) {
			callCount++
			ch := make(chan StreamEvent, 1)
			close(ch)
			return ch, nil
		},
	}

	model := NewModel(mock, "test-session", true /* noColor */)

	// Simulate window sizing.
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// Act: simulate /help input via Update directly (no tea.Program needed).
	// Set the input value and press Enter.
	model.input.SetValue("/help")

	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updatedModel.(*Model)

	// Assert: no ChatStream calls (local command).
	if callCount > 0 {
		t.Errorf("expected 0 ChatStream calls, got %d", callCount)
	}

	// Assert: /help response is in the messages.
	var helpMsg string
	for _, msg := range m.messages {
		if strings.Contains(msg.Content, "/help") {
			helpMsg = msg.Content
			break
		}
	}
	if helpMsg == "" {
		t.Fatalf("/help response not found in messages: %+v", m.messages)
	}

	// Assert: at least 6 slash commands appear in the /help output.
	expectedCommands := []string{"/help", "/save", "/load", "/clear", "/quit", "/session"}
	foundCount := 0
	for _, cmd := range expectedCommands {
		if strings.Contains(helpMsg, cmd) {
			foundCount++
		}
	}
	if foundCount < 6 {
		t.Errorf("expected at least 6 slash commands in /help, found %d\nOutput:\n%s", foundCount, helpMsg)
	}

	// Capture the rendered view for snapshot (not yet quitting).
	got := []byte(m.View())
	snapshots.RequireSnapshot(t, "slash_help_local", got)
}

// TestSlashHelp_TeatestProgram verifies /help via full teatest program run.
// This is a complementary test that exercises the tea.Program path.
func TestSlashHelp_TeatestProgram(t *testing.T) {
	snapshots.SetupAsciiTermenv()

	mock := &mockDaemonClient{}
	model := NewModel(mock, "session", true)

	tm := teatest.NewTestModel(
		t,
		model,
		teatest.WithInitialTermSize(80, 24),
	)

	// Allow initial render.
	time.Sleep(20 * time.Millisecond)

	// Type /help and Enter.
	tm.Type("/help")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for /help text to appear.
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "/help")
		},
		teatest.WithDuration(3*time.Second),
		teatest.WithCheckInterval(10*time.Millisecond),
	)

	// Success: /help output was rendered.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
