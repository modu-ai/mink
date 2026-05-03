// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// View renders the TUI.
// Implements tea.Model interface.
// @MX:ANCHOR This generates the entire UI display string.
func (m *Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	// Style helpers
	var applyStyle func(lipgloss.Style) lipgloss.Style
	if m.noColor {
		applyStyle = func(s lipgloss.Style) lipgloss.Style {
			return lipgloss.NewStyle()
		}
	} else {
		applyStyle = func(s lipgloss.Style) lipgloss.Style {
			return s
		}
	}

	// Build status bar
	statusBar := m.renderStatusBar(applyStyle)

	// Build input area
	inputArea := m.renderInputArea(applyStyle)

	// Combine all sections
	return lipgloss.JoinVertical(lipgloss.Left,
		statusBar,
		m.viewport.View(),
		inputArea,
	)
}

// renderStatusBar generates the status bar showing session and daemon info.
// @MX:NOTE This displays session name, daemon address, and streaming state.
func (m *Model) renderStatusBar(applyStyle func(lipgloss.Style) lipgloss.Style) string {
	statusText := ""
	if m.sessionName != "" {
		statusText = "Session: " + m.sessionName
	} else {
		statusText = "Session: (unnamed)"
	}

	daemonStatus := " | Daemon: " + m.daemonAddr
	if m.streaming {
		daemonStatus += " [Streaming...]"
	}

	// Phase C4: surface chat history depth so users can see at a glance how
	// long the current session has grown.
	msgCount := fmt.Sprintf(" | Messages: %d", len(m.messages))

	rendered := statusText + daemonStatus + msgCount

	// Apply faint style if color is enabled
	if !m.noColor {
		baseStyle := lipgloss.NewStyle().Faint(true)
		return applyStyle(baseStyle).Render(rendered)
	}
	return rendered
}

// renderInputArea generates the input field with prompt.
// @MX:NOTE This renders the text input field with the ">" prompt.
func (m *Model) renderInputArea(applyStyle func(lipgloss.Style) lipgloss.Style) string {
	prompt := "> "
	if m.noColor {
		return prompt + m.input.View()
	}

	// Apply green color to prompt if enabled
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")) // Green
	return applyStyle(promptStyle).Render(prompt) + m.input.View()
}
