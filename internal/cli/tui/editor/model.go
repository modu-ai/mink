// Package editor provides single/multi-line input editor for the TUI.
package editor

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Mode represents the editor input mode.
type Mode int

const (
	// ModeSingle is single-line input mode (default).
	ModeSingle Mode = iota
	// ModeMulti is multi-line textarea input mode.
	ModeMulti
)

// SendMsg is emitted when the user presses Enter to submit content.
// The TUI update loop should handle this to trigger ChatStream.
type SendMsg struct {
	// Content is the full text submitted by the user.
	Content string
}

// EditorModel wraps either textinput or textarea for single/multi-line input.
// @MX:ANCHOR: [AUTO] EditorModel is the core editor state contract.
// @MX:REASON: fan_in >= 3 — consumed by tui/update.go (key dispatch), tui/view.go (render), tui/model.go (field).
type EditorModel struct {
	mode   Mode // Single or Multi
	single textinput.Model
	multi  textarea.Model
}

// New creates a new EditorModel in single-line mode with default settings.
func New() EditorModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()

	ta := textarea.New()
	ta.Placeholder = "Type a message (Enter to send, Ctrl-J for newline)..."
	ta.ShowLineNumbers = false

	return EditorModel{
		mode:   ModeSingle,
		single: ti,
		multi:  ta,
	}
}

// Value returns the current input text regardless of mode.
func (e EditorModel) Value() string {
	if e.mode == ModeMulti {
		return e.multi.Value()
	}
	return e.single.Value()
}

// IsMulti returns true when in multi-line mode.
func (e EditorModel) IsMulti() bool {
	return e.mode == ModeMulti
}

// SetValue sets the editor value in the active mode component.
func (e EditorModel) SetValue(s string) EditorModel {
	if e.mode == ModeMulti {
		e.multi.SetValue(s)
	} else {
		e.single.SetValue(s)
	}
	return e
}

// Clear resets the editor value to empty string.
func (e EditorModel) Clear() EditorModel {
	if e.mode == ModeMulti {
		e.multi.SetValue("")
	} else {
		e.single.SetValue("")
	}
	return e
}

// Toggle switches between single and multi-line mode, preserving the current buffer.
// @MX:NOTE: [AUTO] Toggle preserves buffer across single/multi mode switch. REQ-CLITUI-008
func (e EditorModel) Toggle() EditorModel {
	current := e.Value()
	if e.mode == ModeSingle {
		e.mode = ModeMulti
		e.multi.SetValue(current)
		e.multi.Focus()
		e.single.Blur()
	} else {
		e.mode = ModeSingle
		e.single.SetValue(current)
		e.single.Focus()
		e.multi.Blur()
	}
	return e
}

// Update handles bubbletea messages, routing key presses for mode toggle and send.
func (e EditorModel) Update(msg tea.Msg) (EditorModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Pass through to active component.
		return e.updateComponents(msg)
	}

	switch keyMsg.Type {
	case tea.KeyCtrlN:
		// Toggle single/multi mode.
		return e.Toggle(), nil

	case tea.KeyCtrlJ:
		// In multi-line mode: insert newline (handled by textarea natively).
		// In single-line mode: no-op.
		if e.mode == ModeMulti {
			var cmd tea.Cmd
			e.multi, cmd = e.multi.Update(msg)
			return e, cmd
		}
		return e, nil

	case tea.KeyEnter:
		// Enter submits the content regardless of mode.
		content := e.Value()
		if content == "" {
			return e, nil
		}
		return e, func() tea.Msg {
			return SendMsg{Content: content}
		}
	}

	// Pass remaining keys to active component.
	return e.updateComponents(msg)
}

// updateComponents forwards the message to the active inner component.
func (e EditorModel) updateComponents(msg tea.Msg) (EditorModel, tea.Cmd) {
	var cmd tea.Cmd
	if e.mode == ModeMulti {
		e.multi, cmd = e.multi.Update(msg)
	} else {
		e.single, cmd = e.single.Update(msg)
	}
	return e, cmd
}

// View renders the active input component.
func (e EditorModel) View() string {
	if e.mode == ModeMulti {
		return e.multi.View()
	}
	return e.single.View()
}
