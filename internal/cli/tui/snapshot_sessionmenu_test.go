// Package tui provides snapshot tests for the sessionmenu overlay.
// AC-CLITUI3-004
package tui

import (
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/cli/tui/sessionmenu"
	"github.com/modu-ai/goose/internal/cli/tui/snapshots"
)

// TestSnapshot_SessionMenuOpen verifies that the session menu overlay renders
// correctly with 3 mock entries in mtime desc order.
// AC-CLITUI3-004
func TestSnapshot_SessionMenuOpen(t *testing.T) {
	// Force ASCII profile for deterministic snapshot output.
	snapshots.SetupAsciiTermenv()

	m := NewModel(nil, "test-session", true /* noColor */)
	m.clock = snapshots.FixedClock(time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))
	m.width = 80
	m.height = 24
	m.viewport.Width = 80
	m.viewport.Height = 21

	// Open sessionmenu with 3 mock entries already in mtime desc order.
	entries := []sessionmenu.Entry{
		{Name: "session-c"},
		{Name: "session-b"},
		{Name: "session-a"},
	}
	m.sessionMenuState = sessionmenu.Open(entries)

	output := []byte(m.View())
	snapshots.RequireSnapshot(t, "session_menu_open", output)
}
