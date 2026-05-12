// Package tui provides snapshot tests for the multi-line editor view.
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/mink/internal/cli/tui/snapshots"
)

// TestSnapshot_EditorMultiline verifies the multi-line editor view snapshot.
// AC-CLITUI-009
func TestSnapshot_EditorMultiline(t *testing.T) {
	snapshots.SetupAsciiTermenv()

	model := NewModel(nil, "test-session", true)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// Simulate typing "hello" in single mode, then toggle to multi.
	model.input.SetValue("hello")

	// Toggle to multi-line via Ctrl-N.
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m := updatedModel.(*Model)

	got := []byte(m.View())
	snapshots.RequireSnapshot(t, "editor_multiline", got)
}
