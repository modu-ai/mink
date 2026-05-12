// Package tui provides i18n snapshot tests for 4 TUI surfaces in en and ko.
// AC-CLITUI3-009 (ko), AC-CLITUI3-010 (en)
package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/mink/internal/cli/tui/i18n"
	"github.com/modu-ai/mink/internal/cli/tui/permission"
	"github.com/modu-ai/mink/internal/cli/tui/sessionmenu"
	"github.com/modu-ai/mink/internal/cli/tui/snapshots"
)

// newI18NModel creates a model pre-configured for i18n snapshot tests.
// noColor=true ensures ASCII-only output for deterministic snapshots.
func newI18NModel(lang string) *Model {
	m := NewModel(nil, "test", true /* noColor */)
	m.width = 80
	m.height = 24
	m.viewport.Width = 80
	m.viewport.Height = 21
	m.clock = snapshots.FixedClock(time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))
	m.catalog = i18n.Catalogs[lang]
	return m
}

// --- Surface 1: Statusbar idle ---

// TestSnapshot_I18N_StatusbarIdle_Ko verifies the statusbar idle string in Korean.
// AC-CLITUI3-009
func TestSnapshot_I18N_StatusbarIdle_Ko(t *testing.T) {
	snapshots.SetupAsciiTermenv()
	m := newI18NModel("ko")
	snapshots.RequireSnapshot(t, "statusbar_idle_ko", []byte(m.View()))
}

// TestSnapshot_I18N_StatusbarIdle_En verifies the statusbar idle string in English.
// AC-CLITUI3-010
func TestSnapshot_I18N_StatusbarIdle_En(t *testing.T) {
	snapshots.SetupAsciiTermenv()
	m := newI18NModel("en")
	snapshots.RequireSnapshot(t, "statusbar_idle_en", []byte(m.View()))
}

// --- Surface 2: /help response ---

// TestSnapshot_I18N_SlashHelp_Ko verifies the /help output header in Korean.
// AC-CLITUI3-009
func TestSnapshot_I18N_SlashHelp_Ko(t *testing.T) {
	snapshots.SetupAsciiTermenv()
	m := newI18NModel("ko")

	// Execute /help via Update so slash handling uses the Korean catalog.
	m.input.SetValue("/help")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	snapshots.RequireSnapshot(t, "slash_help_ko", []byte(m.View()))
}

// TestSnapshot_I18N_SlashHelp_En verifies the /help output header in English.
// AC-CLITUI3-010
func TestSnapshot_I18N_SlashHelp_En(t *testing.T) {
	snapshots.SetupAsciiTermenv()
	m := newI18NModel("en")

	// Execute /help via Update so slash handling uses the English catalog.
	m.input.SetValue("/help")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	snapshots.RequireSnapshot(t, "slash_help_en", []byte(m.View()))
}

// --- Surface 3: Permission modal ---

// TestSnapshot_I18N_PermissionModal_Ko verifies the permission modal in Korean.
// AC-CLITUI3-009
func TestSnapshot_I18N_PermissionModal_Ko(t *testing.T) {
	snapshots.SetupAsciiTermenv()
	m := newI18NModel("ko")
	m.permissionState = permission.PermissionModel{
		Active:    true,
		ToolName:  "Bash",
		ToolUseID: "t1",
	}
	snapshots.RequireSnapshot(t, "permission_modal_ko", []byte(m.View()))
}

// TestSnapshot_I18N_PermissionModal_En verifies the permission modal in English.
// AC-CLITUI3-010
func TestSnapshot_I18N_PermissionModal_En(t *testing.T) {
	snapshots.SetupAsciiTermenv()
	m := newI18NModel("en")
	m.permissionState = permission.PermissionModel{
		Active:    true,
		ToolName:  "Bash",
		ToolUseID: "t1",
	}
	snapshots.RequireSnapshot(t, "permission_modal_en", []byte(m.View()))
}

// --- Surface 4: Session menu open ---

// TestSnapshot_I18N_SessionMenuOpen_Ko verifies the sessionmenu overlay header in Korean.
// AC-CLITUI3-009
func TestSnapshot_I18N_SessionMenuOpen_Ko(t *testing.T) {
	snapshots.SetupAsciiTermenv()
	m := newI18NModel("ko")
	m.sessionMenuState = sessionmenu.Open([]sessionmenu.Entry{
		{Name: "session-c"},
		{Name: "session-b"},
		{Name: "session-a"},
	})
	snapshots.RequireSnapshot(t, "session_menu_open_ko", []byte(m.View()))
}

// TestSnapshot_I18N_SessionMenuOpen_En verifies the sessionmenu overlay header in English.
// AC-CLITUI3-010
func TestSnapshot_I18N_SessionMenuOpen_En(t *testing.T) {
	snapshots.SetupAsciiTermenv()
	m := newI18NModel("en")
	m.sessionMenuState = sessionmenu.Open([]sessionmenu.Entry{
		{Name: "session-c"},
		{Name: "session-b"},
		{Name: "session-a"},
	})
	snapshots.RequireSnapshot(t, "session_menu_open_en", []byte(m.View()))
}
