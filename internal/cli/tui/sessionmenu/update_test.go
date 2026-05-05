// Package sessionmenu provides tests for the sessionmenu update handler.
// AC-CLITUI3-001, AC-CLITUI3-002, AC-CLITUI3-003
package sessionmenu

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// testEntries returns a fixed slice of 3 entries for use in update tests.
func testEntries() []Entry {
	base := time.Now()
	return []Entry{
		{Name: "alpha", Path: "/tmp/alpha.jsonl", ModTime: base},
		{Name: "beta", Path: "/tmp/beta.jsonl", ModTime: base.Add(-time.Minute)},
		{Name: "gamma", Path: "/tmp/gamma.jsonl", ModTime: base.Add(-2 * time.Minute)},
	}
}

// runCmd executes a tea.Cmd and returns the resulting tea.Msg (nil if cmd is nil).
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// TestSessionMenu_Update_CursorDown verifies arrow-down increments cursor.
// REQ-CLITUI3-008
func TestSessionMenu_Update_CursorDown(t *testing.T) {
	m := Open(testEntries())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 1 {
		t.Errorf("expected cursor=1 after Down, got %d", m.Cursor())
	}
}

// TestSessionMenu_Update_CursorClamps verifies no wrap at bottom or top.
// REQ-CLITUI3-008, AC-CLITUI3-001
func TestSessionMenu_Update_CursorClamps(t *testing.T) {
	entries := testEntries() // 3 entries, idx 0..2
	m := Open(entries)

	// Move past the bottom.
	for range 5 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.Cursor() != 2 {
		t.Errorf("expected cursor clamped at 2 (bottom), got %d", m.Cursor())
	}

	// Move past the top.
	for range 5 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	if m.Cursor() != 0 {
		t.Errorf("expected cursor clamped at 0 (top), got %d", m.Cursor())
	}
}

// TestSessionMenu_Update_EnterEmitsLoadMsg verifies Enter produces LoadMsg with correct Name.
// REQ-CLITUI3-008, AC-CLITUI3-002
func TestSessionMenu_Update_EnterEmitsLoadMsg(t *testing.T) {
	m := Open(testEntries())
	// Move cursor to entry[1] (beta).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(cmd)

	loadMsg, ok := msg.(LoadMsg)
	if !ok {
		t.Fatalf("expected LoadMsg, got %T (%v)", msg, msg)
	}
	if loadMsg.Name != "beta" {
		t.Errorf("expected LoadMsg.Name == 'beta', got %q", loadMsg.Name)
	}
	if m.IsOpen() {
		t.Error("expected overlay to be closed after Enter")
	}
}

// TestSessionMenu_Update_EscEmitsCloseMsg verifies Esc dismisses and emits CloseMsg.
// REQ-CLITUI3-008, AC-CLITUI3-001
func TestSessionMenu_Update_EscEmitsCloseMsg(t *testing.T) {
	m := Open(testEntries())
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	msg := runCmd(cmd)

	if _, ok := msg.(CloseMsg); !ok {
		t.Fatalf("expected CloseMsg, got %T (%v)", msg, msg)
	}
	if m.IsOpen() {
		t.Error("expected overlay to be closed after Esc")
	}
}

// TestSessionMenu_Update_EmptyEnterEmitsCloseMsg verifies Enter on empty list = CloseMsg.
// REQ-CLITUI3-004, AC-CLITUI3-003
func TestSessionMenu_Update_EmptyEnterEmitsCloseMsg(t *testing.T) {
	m := Open([]Entry{})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(cmd)

	if _, ok := msg.(CloseMsg); !ok {
		t.Fatalf("expected CloseMsg on empty Enter, got %T (%v)", msg, msg)
	}
	if m.IsOpen() {
		t.Error("expected overlay to be closed after Enter on empty list")
	}
}

// TestSessionMenu_Update_ClosedModelIsNoOp verifies that a closed model ignores all keys.
func TestSessionMenu_Update_ClosedModelIsNoOp(t *testing.T) {
	m := New() // closed
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd from closed model")
	}
	if m2.IsOpen() {
		t.Error("expected model to remain closed")
	}
}
