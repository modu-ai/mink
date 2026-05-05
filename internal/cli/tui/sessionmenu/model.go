// Package sessionmenu implements the Ctrl-R session picker overlay.
// SPEC-GOOSE-CLI-TUI-003 P2 REQ-CLITUI3-003, -004, -008
package sessionmenu

import "time"

// Entry represents a single session entry in the menu.
type Entry struct {
	Name    string
	Path    string
	ModTime time.Time
}

// Model holds the state of the sessionmenu overlay.
type Model struct {
	entries []Entry
	cursor  int
	open    bool
}

// LoadMsg is emitted when the user selects a session to load.
// @MX:ANCHOR LoadMsg bridges sessionmenu.Update and tui.Update message dispatch.
// @MX:REASON fan_in >= 3: update.go handler, sessionmenu_tui_test.go, update_test.go all consume it.
type LoadMsg struct{ Name string }

// CloseMsg is emitted when the user dismisses the menu without loading.
type CloseMsg struct{}

// New creates a new closed Model.
func New() Model { return Model{} }

// Open creates a Model loaded with entries in the open state.
// If entries is empty, the overlay still opens (shows empty state).
func Open(entries []Entry) Model { return Model{entries: entries, open: true} }

// IsOpen reports whether the overlay is visible.
func (m Model) IsOpen() bool { return m.open }

// Entries returns the list of session entries (may be empty).
func (m Model) Entries() []Entry { return m.entries }

// Cursor returns the current cursor index.
func (m Model) Cursor() int { return m.cursor }
