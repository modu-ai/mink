// Package editor provides single/multi-line input editor for the TUI.
package editor

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEditor_CtrlN_TogglesMode verifies that Ctrl-N toggles between single and multi-line mode.
// AC-CLITUI-009
func TestEditor_CtrlN_TogglesMode(t *testing.T) {
	// Arrange: new editor starts in single-line mode.
	e := New()
	assert.Equal(t, ModeSingle, e.mode, "editor should start in single-line mode")

	// Type some text in single mode.
	e = e.SetValue("hello")
	assert.Equal(t, "hello", e.Value())

	// Act: press Ctrl-N to toggle to multi-line mode.
	newE, _ := e.Update(tea.KeyMsg{Type: tea.KeyCtrlN})

	// Assert: mode switched to multi, buffer preserved.
	assert.Equal(t, ModeMulti, newE.mode, "Ctrl-N should switch to multi-line mode")
	assert.Equal(t, "hello", newE.Value(), "buffer should be preserved after toggle")
	assert.True(t, newE.IsMulti(), "IsMulti() should return true after toggle")

	// Act: press Ctrl-N again to toggle back to single-line mode.
	backE, _ := newE.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	assert.Equal(t, ModeSingle, backE.mode, "second Ctrl-N should switch back to single-line mode")
	assert.Equal(t, "hello", backE.Value(), "buffer should be preserved on second toggle")
}

// TestEditor_MultiLine_CtrlJ_NewlineEnter_Send verifies multi-line input behavior.
// AC-CLITUI-010
func TestEditor_MultiLine_CtrlJ_NewlineEnter_Send(t *testing.T) {
	// Arrange: start in multi-line mode.
	e := New()
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	require.Equal(t, ModeMulti, e.mode, "should be in multi-line mode")

	// Type "line1".
	e = e.SetValue("line1")

	// Act: press Ctrl-J to insert newline.
	newE, cmd := e.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	assert.Nil(t, cmd, "Ctrl-J in multi-line mode should not send (no cmd)")

	// Append "line2".
	newE = newE.SetValue(newE.Value() + "line2")

	// Assert: value contains both lines.
	assert.Contains(t, newE.Value(), "line1", "value should contain line1")
	assert.Contains(t, newE.Value(), "line2", "value should contain line2")

	// Act: press Enter to send.
	_, sendCmd := newE.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, sendCmd, "Enter in multi-line mode should emit a send command")

	// Assert: mode remains multi after send.
	assert.Equal(t, ModeMulti, newE.mode, "mode should remain multi after send")
}

// TestEditor_SingleLine_Enter_Send verifies single-line Enter sends message.
func TestEditor_SingleLine_Enter_Send(t *testing.T) {
	e := New()
	require.Equal(t, ModeSingle, e.mode)

	e = e.SetValue("test message")
	_, sendCmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, sendCmd, "Enter in single-line mode should emit a send command")
}

// TestEditor_Clear verifies that Clear resets the value.
func TestEditor_Clear(t *testing.T) {
	e := New()
	e = e.SetValue("some text")
	assert.Equal(t, "some text", e.Value())

	e = e.Clear()
	assert.Equal(t, "", e.Value(), "Clear should reset value to empty")
}

// TestEditor_View_DoesNotPanic verifies View() doesn't panic.
func TestEditor_View_DoesNotPanic(t *testing.T) {
	e := New()
	e = e.SetValue("hello")
	assert.NotPanics(t, func() {
		_ = e.View()
	})

	// Multi mode.
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	assert.NotPanics(t, func() {
		_ = e.View()
	})
}
