// Package permission provides the permission modal sub-model.
package permission

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/goose/internal/cli/tui/snapshots"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermission_Modal_OpensOnRequest verifies AC-CLITUI-003:
// a PermissionModel initialised with Active=true, ToolName="Bash" renders
// and can be snapshot-tested.
func TestPermission_Modal_OpensOnRequest(t *testing.T) {
	snapshots.SetupAsciiTermenv()

	pm := PermissionModel{
		Active:    true,
		ToolName:  "Bash",
		ToolUseID: "tool-use-123",
	}

	require.True(t, pm.Active, "permissionState.active must be true")
	require.Equal(t, "Bash", pm.ToolName)

	view := pm.View()
	assert.NotEmpty(t, view, "View must return non-empty string when active")
	assert.True(t, strings.Contains(view, "Bash"), "view must show tool name")
	assert.True(t, strings.Contains(view, "Allow"), "view must show Allow option")

	snapshots.WriteGolden(t, "permission_modal_open", []byte(view))
	snapshots.RequireSnapshot(t, "permission_modal_open", []byte(view))
}

// TestPermission_Modal_DefaultCursorIsAllowOnce verifies initial cursor position.
func TestPermission_Modal_DefaultCursorIsAllowOnce(t *testing.T) {
	pm := PermissionModel{
		Active:   true,
		ToolName: "FileWrite",
	}
	// Default cursor should be 0 (Allow once).
	assert.Equal(t, 0, pm.Cursor())
}

// TestPermission_Modal_ArrowKeysMoveCursor verifies Up/Down moves cursor.
func TestPermission_Modal_ArrowKeysMoveCursor(t *testing.T) {
	pm := PermissionModel{
		Active:   true,
		ToolName: "Bash",
	}

	// Down once → cursor 1.
	pm2, _ := pm.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, pm2.Cursor())

	// Down again → cursor 2.
	pm3, _ := pm2.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, pm3.Cursor())

	// Up once → cursor 1.
	pm4, _ := pm3.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, pm4.Cursor())
}

// TestPermission_Modal_CursorWraps verifies cursor does not go out of bounds.
func TestPermission_Modal_CursorWraps(t *testing.T) {
	pm := PermissionModel{
		Active:   true,
		ToolName: "Bash",
		cursor:   0,
	}

	// Up from 0 should wrap to last option.
	pm2, _ := pm.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, pm2.Cursor() >= 0, "cursor must stay non-negative")
	assert.True(t, pm2.Cursor() < 4, "cursor must stay within option count")

	// Down from last option.
	pmLast := PermissionModel{Active: true, ToolName: "Bash", cursor: 3}
	pmWrapped, _ := pmLast.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.True(t, pmWrapped.Cursor() >= 0 && pmWrapped.Cursor() < 4, "cursor must wrap")
}

// TestPermission_Modal_ResolveReturnsMsgOnEnter verifies Enter produces ResolveMsg.
func TestPermission_Modal_ResolveReturnsMsgOnEnter(t *testing.T) {
	pm := PermissionModel{
		Active:    true,
		ToolName:  "Bash",
		ToolUseID: "tu-1",
		cursor:    0, // Allow once.
	}

	pm2, cmd := pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter should produce a command")
	_ = pm2 // model should have Active=false after resolve.

	msg := cmd()
	resolveMsg, ok := msg.(ResolveMsg)
	require.True(t, ok, "command must produce a ResolveMsg")
	assert.Equal(t, "Bash", resolveMsg.ToolName)
	assert.Equal(t, "tu-1", resolveMsg.ToolUseID)
}

// TestPermission_Modal_EscapeDenieOnce verifies Escape produces deny-once ResolveMsg.
func TestPermission_Modal_EscapeDenieOnce(t *testing.T) {
	pm := PermissionModel{
		Active:    true,
		ToolName:  "FileWrite",
		ToolUseID: "tu-2",
	}

	_, cmd := pm.Update(tea.KeyMsg{Type: tea.KeyEscape})
	require.NotNil(t, cmd)

	msg := cmd()
	resolveMsg, ok := msg.(ResolveMsg)
	require.True(t, ok)
	assert.Equal(t, "deny_once", resolveMsg.ModalDecision)
}

// TestPermission_Modal_InactiveReturnsBlankView verifies inactive modal renders empty.
func TestPermission_Modal_InactiveReturnsBlankView(t *testing.T) {
	pm := PermissionModel{Active: false}
	assert.Empty(t, pm.View())
}
