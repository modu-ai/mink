// Package permission provides the permission modal sub-model.
package permission

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update processes keyboard input when the modal is active.
// Returns the updated PermissionModel and an optional tea.Cmd.
func (m PermissionModel) Update(msg tea.Msg) (PermissionModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyDown:
		m.cursor = (m.cursor + 1) % optCount
		return m, nil

	case tea.KeyUp:
		m.cursor = (m.cursor - 1 + optCount) % optCount
		return m, nil

	case tea.KeyEnter:
		decision := m.SelectedDecision()
		toolName := m.ToolName
		toolUseID := m.ToolUseID
		m.Active = false
		return m, func() tea.Msg {
			return ResolveMsg{
				ToolName:      toolName,
				ToolUseID:     toolUseID,
				ModalDecision: decision,
			}
		}

	case tea.KeyEscape:
		toolName := m.ToolName
		toolUseID := m.ToolUseID
		m.Active = false
		return m, func() tea.Msg {
			return ResolveMsg{
				ToolName:      toolName,
				ToolUseID:     toolUseID,
				ModalDecision: "deny_once",
			}
		}
	}

	return m, nil
}
