// Package sessionmenu provides the key handler for the Ctrl-R session overlay.
package sessionmenu

import tea "github.com/charmbracelet/bubbletea"

// Update handles key input for the sessionmenu overlay.
// Returns the updated Model and a command (LoadMsg or CloseMsg) if applicable.
// Only routes input when the overlay is open; no-ops when closed.
// REQ-CLITUI3-008
func (m Model) Update(msg tea.KeyMsg) (Model, tea.Cmd) {
	if !m.open {
		return m, nil
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if len(m.entries) > 0 && m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		if len(m.entries) == 0 {
			m.open = false
			return m, func() tea.Msg { return CloseMsg{} }
		}
		name := m.entries[m.cursor].Name
		m.open = false
		return m, func() tea.Msg { return LoadMsg{Name: name} }
	case tea.KeyEscape:
		m.open = false
		return m, func() tea.Msg { return CloseMsg{} }
	}

	return m, nil
}
