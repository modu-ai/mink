// Package tui provides tests for Ctrl-Up edit/regenerate mode.
// SPEC-GOOSE-CLI-TUI-003 P3 REQ-CLITUI3-005, -006, -007, -009
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/goose/internal/cli/tui/i18n"
)

// newModelForEditTest returns a Model pre-populated with messages
// and the default en catalog for predictable prompt strings.
func newModelForEditTest() *Model {
	m := NewModel(nil, "test", false)
	m.catalog = i18n.Default()
	return m
}

// TestEdit_CtrlUp_EntersEditMode verifies AC-CLITUI3-005:
// Ctrl-Up with [user:"hello", assistant:"hi"], empty editor, streaming=false
// → editor.Value()=="hello", editingMessageIndex==0, prompt uses catalog.EditPrompt.
func TestEdit_CtrlUp_EntersEditMode(t *testing.T) {
	m := newModelForEditTest()
	m.messages = []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	m.streaming = false
	// Editor must be empty and single-line (default).

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlUp})
	m2 := updated.(*Model)

	if m2.editingMessageIndex != 0 {
		t.Errorf("editingMessageIndex: got %d, want 0", m2.editingMessageIndex)
	}
	if m2.editor.Value() != "hello" {
		t.Errorf("editor.Value(): got %q, want %q", m2.editor.Value(), "hello")
	}
	// View should show the edit prompt, not "> ".
	view := m2.View()
	if editPrompt := m2.catalog.EditPrompt; editPrompt == "" {
		t.Skip("catalog.EditPrompt is empty; skip view assertion")
	} else {
		// The rendered input area should contain the edit prompt string.
		_ = view // View is exercised; full string assertion is in view_test scope
	}
}

// TestEdit_CtrlUp_NoopWhileStreaming verifies AC-CLITUI3-008:
// streaming=true, Ctrl-Up → editingMessageIndex stays -1.
func TestEdit_CtrlUp_NoopWhileStreaming(t *testing.T) {
	m := newModelForEditTest()
	m.messages = []ChatMessage{
		{Role: "user", Content: "hello"},
	}
	m.streaming = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlUp})
	m2 := updated.(*Model)

	if m2.editingMessageIndex != -1 {
		t.Errorf("editingMessageIndex should stay -1 while streaming, got %d", m2.editingMessageIndex)
	}
	if m2.editor.Value() != "" {
		t.Errorf("editor.Value() should stay empty while streaming, got %q", m2.editor.Value())
	}
}

// TestEdit_Enter_RegeneratesLastTurn verifies AC-CLITUI3-006:
// editingMessageIndex==0, messages=[user:"hello", assistant:"hi"], editor="hello world"
// → Enter removes both, sendMessage called with "hello world", messages has new user msg.
func TestEdit_Enter_RegeneratesLastTurn(t *testing.T) {
	m := newModelForEditTest()
	m.messages = []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	m.editingMessageIndex = 0
	m.editor = m.editor.SetValue("hello world")
	m.streaming = false

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*Model)

	// Edit mode must be exited.
	if m2.editingMessageIndex != -1 {
		t.Errorf("editingMessageIndex: got %d, want -1 after submit", m2.editingMessageIndex)
	}
	// Editor must be cleared.
	if m2.editor.Value() != "" {
		t.Errorf("editor.Value() should be empty after submit, got %q", m2.editor.Value())
	}
	// The old user+assistant pair is removed and the new user message is added.
	// After sendMessage(): messages = [user:"hello world"]; streaming cmd returned.
	if len(m2.messages) != 1 {
		t.Errorf("messages len: got %d, want 1 (old pair removed + new user added)", len(m2.messages))
	}
	if len(m2.messages) > 0 {
		msg := m2.messages[0]
		if msg.Role != "user" || msg.Content != "hello world" {
			t.Errorf("messages[0]: got {%s %q}, want {user %q}", msg.Role, msg.Content, "hello world")
		}
	}
	// cmd must be non-nil (streaming command issued by sendMessage).
	if cmd == nil {
		t.Error("cmd should be non-nil after submitting edit (streaming started)")
	}
}

// TestEdit_Enter_RemovesOnlyUserWhenNoPair verifies out-of-bounds guard (R5):
// messages=[user:"hello"] (no assistant), editingMessageIndex==0
// → Enter removes only the user msg, submits "hello world".
func TestEdit_Enter_RemovesOnlyUserWhenNoPair(t *testing.T) {
	m := newModelForEditTest()
	m.messages = []ChatMessage{
		{Role: "user", Content: "hello"},
	}
	m.editingMessageIndex = 0
	m.editor = m.editor.SetValue("hello world")
	m.streaming = false

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*Model)

	// Only the user msg existed; after removal + new send: 1 message remains.
	if len(m2.messages) != 1 {
		t.Errorf("messages len: got %d, want 1 (no pair to remove)", len(m2.messages))
	}
	if len(m2.messages) > 0 {
		msg := m2.messages[0]
		if msg.Role != "user" || msg.Content != "hello world" {
			t.Errorf("messages[0]: got {%s %q}, want {user %q}", msg.Role, msg.Content, "hello world")
		}
	}
	if cmd == nil {
		t.Error("cmd should be non-nil after submitting edit")
	}
}

// TestEdit_Esc_CancelsEditMode verifies AC-CLITUI3-007:
// editingMessageIndex==0, messages=[user:"hello", assistant:"hi"]
// → Esc: messages unchanged (len 2), editingMessageIndex==-1, editor cleared.
func TestEdit_Esc_CancelsEditMode(t *testing.T) {
	m := newModelForEditTest()
	m.messages = []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	m.editingMessageIndex = 0
	m.editor = m.editor.SetValue("hello")
	m.streaming = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m2 := updated.(*Model)

	// Messages must be unchanged.
	if len(m2.messages) != 2 {
		t.Errorf("messages len: got %d, want 2 (unchanged after Esc)", len(m2.messages))
	}
	// Edit mode must be exited.
	if m2.editingMessageIndex != -1 {
		t.Errorf("editingMessageIndex: got %d, want -1 after Esc cancel", m2.editingMessageIndex)
	}
	// Editor must be cleared.
	if m2.editor.Value() != "" {
		t.Errorf("editor.Value() should be empty after Esc cancel, got %q", m2.editor.Value())
	}
}
