// Package permission provides the permission modal sub-model.
package permission

// option index constants for the permission modal cursor.
const (
	optAllowOnce   = 0
	optAllowAlways = 1
	optCount       = 4
)

// optionLabels maps option index to display label.
var optionLabels = [optCount]string{
	"Allow once",
	"Allow always",
	"Deny once",
	"Deny always",
}

// optionDecisionStrings maps option index to wire decision string used in ResolveMsg.
var optionDecisionStrings = [optCount]string{
	"allow_once",
	"allow_always",
	"deny_once",
	"deny_always",
}

// PermissionModel holds the state for the permission modal.
//
// @MX:ANCHOR PermissionModel is the core modal state contract.
// @MX:REASON fan_in >= 3: tui/update.go (modal mode branch), tui/view.go (overlay render), tui/client.go (PermissionRequestMsg conversion)
type PermissionModel struct {
	// Active indicates whether the modal is currently visible.
	Active bool
	// ToolName is the name of the tool requesting permission.
	ToolName string
	// ToolUseID is the tool use ID used when calling ResolvePermission RPC.
	ToolUseID string
	// cursor is the current selection index (0-3).
	cursor int
}

// ResolveMsg is a bubbletea message produced when the user resolves the modal.
// It carries the decision and the tool identification for the caller to act on.
type ResolveMsg struct {
	ToolName      string
	ToolUseID     string
	ModalDecision string // "allow_once", "allow_always", "deny_once", "deny_always"
}

// Cursor returns the current option cursor position.
func (m PermissionModel) Cursor() int {
	return m.cursor
}

// SelectedDecision returns the decision string for the current cursor position.
func (m PermissionModel) SelectedDecision() string {
	if m.cursor < 0 || m.cursor >= optCount {
		return optionDecisionStrings[optAllowOnce]
	}
	return optionDecisionStrings[m.cursor]
}

// IsAllowAlways reports whether the current selection is "Allow always".
func (m PermissionModel) IsAllowAlways() bool {
	return m.cursor == optAllowAlways
}
