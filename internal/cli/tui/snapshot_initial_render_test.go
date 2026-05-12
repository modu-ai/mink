// Package tui provides snapshot tests for the initial TUI render.
package tui

import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/modu-ai/mink/internal/cli/tui/snapshots"
)

// TestSnapshot_ChatREPL_InitialRender verifies that the TUI initial render
// matches the golden snapshot exactly in a fixed 80×24 ASCII-only environment.
// AC-CLITUI-002
func TestSnapshot_ChatREPL_InitialRender(t *testing.T) {
	// Must be called first to strip all ANSI color codes.
	snapshots.SetupAsciiTermenv()

	// Arrange: model with mock client and noColor=true for pure ASCII output.
	mock := &mockDaemonClient{}
	model := NewModel(mock, "test-session", true /* noColor */)

	// Act: create a test program at 80×24.
	tm := teatest.NewTestModel(
		t,
		model,
		teatest.WithInitialTermSize(80, 24),
	)

	// Wait a moment for the initial render to complete, then quit.
	time.Sleep(20 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	finalOut := tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second))
	got, err := io.ReadAll(finalOut)
	if err != nil {
		t.Fatalf("failed to read final output: %v", err)
	}

	// Assert: compare against golden file.
	snapshots.RequireSnapshot(t, "chat_repl_initial_render", got)
}
