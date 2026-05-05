// Package tui provides integration tests for the Ctrl-R sessionmenu overlay.
// AC-CLITUI3-001, AC-CLITUI3-002, AC-CLITUI3-003
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/goose/internal/cli/tui/sessionmenu"
)

// setupSessionMenuHome creates a temp HOME with .goose/sessions/ and returns cleanup.
func setupSessionMenuHome(t *testing.T) (sessDir string, cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	sessDir = filepath.Join(tmpDir, ".goose", "sessions")
	cleanup = func() { os.Setenv("HOME", oldHome) }
	return sessDir, cleanup
}

// writeSession writes a minimal valid .jsonl session file with the given mtime.
func writeSession(t *testing.T, dir, name string, mtime time.Time, msgs ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, name+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create %s: %v", path, err)
	}
	enc := json.NewEncoder(f)
	if len(msgs) == 0 {
		msgs = []string{"hello"}
	}
	for i, content := range msgs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		_ = enc.Encode(map[string]interface{}{"role": role, "content": content, "ts": 0})
	}
	f.Close()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
}

// sendKey is a helper to dispatch a single KeyMsg through model.Update.
func sendKey(t *testing.T, m *Model, kt tea.KeyType) *Model {
	t.Helper()
	updated, cmd := m.Update(tea.KeyMsg{Type: kt})
	m2 := updated.(*Model)
	// Execute any emitted command once to process sessionmenu messages.
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			updated2, _ := m2.Update(msg)
			m2 = updated2.(*Model)
		}
	}
	return m2
}

// TestSessionMenu_CtrlR_OpensList verifies AC-CLITUI3-001:
// Ctrl-R with 3 sessions opens overlay, entries sorted mtime desc, cursor=0.
// Esc closes without side effect.
func TestSessionMenu_CtrlR_OpensList(t *testing.T) {
	sessDir, cleanup := setupSessionMenuHome(t)
	defer cleanup()

	base := time.Now()
	writeSession(t, sessDir, "old", base.Add(-2*time.Hour))
	writeSession(t, sessDir, "mid", base.Add(-1*time.Hour))
	writeSession(t, sessDir, "new", base)

	m := NewModel(nil, "", false)

	// Press Ctrl-R.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(*Model)

	if !m.sessionMenuState.IsOpen() {
		t.Fatal("expected sessionmenu to be open after Ctrl-R")
	}

	entries := m.sessionMenuState.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Name != "new" {
		t.Errorf("entries[0] should be 'new' (newest), got %q", entries[0].Name)
	}
	if entries[1].Name != "mid" {
		t.Errorf("entries[1] should be 'mid', got %q", entries[1].Name)
	}
	if entries[2].Name != "old" {
		t.Errorf("entries[2] should be 'old' (oldest), got %q", entries[2].Name)
	}
	if m.sessionMenuState.Cursor() != 0 {
		t.Errorf("initial cursor should be 0, got %d", m.sessionMenuState.Cursor())
	}

	// Press Esc — overlay should close.
	m = sendKey(t, m, tea.KeyEscape)

	if m.sessionMenuState.IsOpen() {
		t.Error("expected sessionmenu to be closed after Esc")
	}
	// No messages should have been loaded (no side effect).
	if len(m.messages) != 0 {
		t.Errorf("expected no messages loaded after Esc, got %d", len(m.messages))
	}
}

// TestSessionMenu_Navigation_LoadsOnEnter verifies AC-CLITUI3-002:
// Arrow Down × 2 → cursor=2 → Enter → session loaded, status message shown.
func TestSessionMenu_Navigation_LoadsOnEnter(t *testing.T) {
	sessDir, cleanup := setupSessionMenuHome(t)
	defer cleanup()

	base := time.Now()
	// Create 3 sessions; "old" is at index 2 (oldest).
	writeSession(t, sessDir, "old", base.Add(-2*time.Hour), "msg1", "reply1")
	writeSession(t, sessDir, "mid", base.Add(-1*time.Hour))
	writeSession(t, sessDir, "new", base)

	m := NewModel(nil, "", false)

	// Open overlay.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(*Model)

	if !m.sessionMenuState.IsOpen() {
		t.Fatal("overlay should be open")
	}

	// Move down twice (cursor 0 → 1 → 2).
	m = sendKey(t, m, tea.KeyDown)
	m = sendKey(t, m, tea.KeyDown)

	if m.sessionMenuState.Cursor() != 2 {
		t.Fatalf("expected cursor=2 after 2× Down, got %d", m.sessionMenuState.Cursor())
	}

	// Press Enter — selects "old" session.
	m = sendKey(t, m, tea.KeyEnter)

	if m.sessionMenuState.IsOpen() {
		t.Error("overlay should be closed after Enter")
	}

	// After loading 'old' (2 content messages) handleSessionMenuLoad appends a
	// system status message, giving 3 total.
	if len(m.messages) != 3 {
		t.Errorf("expected 3 messages (2 content + 1 status) after loading 'old', got %d", len(m.messages))
	}

	// Verify the first two messages are the session content.
	if m.messages[0].Role != "user" || m.messages[0].Content != "msg1" {
		t.Errorf("messages[0] should be user/msg1, got %q/%q", m.messages[0].Role, m.messages[0].Content)
	}
	if m.messages[1].Role != "assistant" || m.messages[1].Content != "reply1" {
		t.Errorf("messages[1] should be assistant/reply1, got %q/%q", m.messages[1].Role, m.messages[1].Content)
	}

	// Verify the last message is the system status showing session was loaded.
	statusMsg := m.messages[2]
	if statusMsg.Role != "system" || !strings.Contains(statusMsg.Content, "old") {
		t.Errorf("messages[2] should be system status containing 'old', got role=%q content=%q",
			statusMsg.Role, statusMsg.Content)
	}
}

// TestSessionMenu_EmptyState_AutoDismiss verifies AC-CLITUI3-003:
// Ctrl-R with empty sessions dir auto-dismisses in same Update cycle.
func TestSessionMenu_EmptyState_AutoDismiss(t *testing.T) {
	sessDir, cleanup := setupSessionMenuHome(t)
	defer cleanup()

	// Create the sessions dir but leave it empty.
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	m := NewModel(nil, "", false)

	// Press Ctrl-R.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(*Model)

	// The overlay opens with 0 entries but a CloseMsg is immediately queued.
	// Execute the command to process the auto-dismiss.
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			updated2, _ := m.Update(msg)
			m = updated2.(*Model)
		}
	}

	// After processing CloseMsg the overlay should be closed.
	if m.sessionMenuState.IsOpen() {
		t.Error("expected sessionmenu to be auto-dismissed when empty")
	}
}

// TestSessionMenu_KeysRoutedToOverlay verifies REQ-CLITUI3-008:
// While overlay is open, keys are consumed by sessionmenu, not input field.
func TestSessionMenu_KeysRoutedToOverlay(t *testing.T) {
	sessDir, cleanup := setupSessionMenuHome(t)
	defer cleanup()

	base := time.Now()
	writeSession(t, sessDir, "s1", base)
	writeSession(t, sessDir, "s2", base.Add(-time.Minute))

	m := NewModel(nil, "", false)

	// Open overlay.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(*Model)

	// Arrow Down should move cursor, not type into input.
	inputBefore := m.input.Value()
	m = sendKey(t, m, tea.KeyDown)

	if !m.sessionMenuState.IsOpen() {
		t.Fatal("overlay should still be open")
	}
	if m.sessionMenuState.Cursor() != 1 {
		t.Errorf("expected cursor=1, got %d", m.sessionMenuState.Cursor())
	}
	if m.input.Value() != inputBefore {
		t.Errorf("input field should be unchanged while overlay is open; was %q, now %q",
			inputBefore, m.input.Value())
	}
}

// TestSessionMenu_View_ContainsOverlay verifies that View() shows overlay when open.
func TestSessionMenu_View_ContainsOverlay(t *testing.T) {
	sessDir, cleanup := setupSessionMenuHome(t)
	defer cleanup()

	base := time.Now()
	writeSession(t, sessDir, "myses", base)

	m := NewModel(nil, "", false)
	// Manually inject a known overlay state for deterministic rendering.
	m.sessionMenuState = sessionmenu.Open([]sessionmenu.Entry{
		{Name: "myses", Path: filepath.Join(sessDir, "myses.jsonl"), ModTime: base},
	})

	view := m.View()
	if !strings.Contains(view, "myses") {
		t.Errorf("View() should contain session name 'myses' when overlay open; got:\n%s", view)
	}
	// Should contain overlay border.
	if !strings.Contains(view, "+--") {
		t.Errorf("View() should contain overlay border when overlay open; got:\n%s", view)
	}
}

// TestSessionMenu_CtrlR_IgnoredWhenPermModalActive verifies that Ctrl-R
// does not open the session menu when the permission modal is active.
func TestSessionMenu_CtrlR_IgnoredWhenPermModalActive(t *testing.T) {
	_, cleanup := setupSessionMenuHome(t)
	defer cleanup()

	m := NewModel(nil, "", false)
	// Simulate active permission modal.
	m.permissionState.Active = true

	// Press Ctrl-R.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(*Model)

	if m.sessionMenuState.IsOpen() {
		t.Error("Ctrl-R should be ignored when permission modal is active")
	}
}

// fmtLoaded is shared by TestSessionMenu_Navigation_LoadsOnEnter and related tests.
// Matches the format string from i18n catalog.
func fmtLoaded(name string, n int) string {
	return fmt.Sprintf("[loaded: %s, %d messages]", name, n)
}
