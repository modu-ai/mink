// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View renders the TUI.
// Implements tea.Model interface.
// @MX:ANCHOR This generates the entire UI display string.
// @MX:REASON fan_in >= 3: bubbletea runtime, snapshot tests, modal overlay branch added (P3).
func (m *Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	// Permission modal overlay: when active, render modal over base view.
	// SPEC-GOOSE-CLI-TUI-002 P3 AC-CLITUI-003
	if m.permissionState.Active {
		baseView := m.renderBaseView()
		modalView := m.permissionState.View()
		return baseView + "\n" + modalView
	}

	return m.renderBaseView()
}

// renderBaseView renders the standard TUI without modal overlay.
func (m *Model) renderBaseView() string {
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

// spinnerFrames is a simple spinner character cycle used during streaming.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// renderStatusBar generates the status bar showing session and daemon info.
// @MX:WARN: [AUTO] Complexity ≥15 — streaming + permission + cost branches.
// @MX:REASON: fan_in >= 3 and ≥15 if-branches: streaming, cost, color, confirmQuit, session all branch here.
func (m *Model) renderStatusBar(applyStyle func(lipgloss.Style) lipgloss.Style) string {
	if m.streaming {
		return m.renderStreamingStatusBar(applyStyle)
	}

	statusText := ""
	if m.sessionName != "" {
		statusText = "Session: " + m.sessionName
	} else {
		statusText = "Session: (unnamed)"
	}

	daemonStatus := " | Daemon: " + m.daemonAddr

	// Phase C4: surface chat history depth so users can see at a glance how
	// long the current session has grown.
	msgCount := fmt.Sprintf(" | Messages: %d", len(m.messages))

	// Cost estimate (optional, shown after streaming completes).
	costStr := ""
	if m.cumulativeCost > 0 && m.pricing != nil {
		costStr = fmt.Sprintf(" | ~$%.4f", m.cumulativeCost)
	}

	rendered := statusText + daemonStatus + msgCount + costStr

	// Apply faint style if color is enabled.
	if !m.noColor {
		baseStyle := lipgloss.NewStyle().Faint(true)
		return applyStyle(baseStyle).Render(rendered)
	}
	return rendered
}

// renderStreamingStatusBar renders the statusbar during active streaming.
// Shows spinner, token count, throughput, elapsed time, abort hint, and cost.
func (m *Model) renderStreamingStatusBar(applyStyle func(lipgloss.Style) lipgloss.Style) string {
	// Spinner frame based on token count (changes with each update).
	frame := spinnerFrames[m.tokenCount%len(spinnerFrames)]

	// Token count.
	tokStr := fmt.Sprintf("↑ %d tok", m.tokenCount)

	// Throughput (t/s).
	throughput := m.currentThroughput
	if throughput < 0 || math.IsNaN(throughput) {
		throughput = 0
	}
	rateStr := fmt.Sprintf("~%.0f t/s", throughput)

	// Elapsed time (use injectable clock for deterministic tests).
	elapsed := time.Duration(0)
	if !m.streamStartTime.IsZero() {
		now := time.Now()
		if m.clock != nil {
			now = m.clock()
		}
		elapsed = now.Sub(m.streamStartTime)
		elapsed = max(elapsed, 0)
	}
	elapsedStr := fmt.Sprintf("%.1fs elapsed", elapsed.Seconds())

	// Cost estimate (optional).
	costStr := ""
	if m.cumulativeCost > 0 && m.pricing != nil {
		costStr = fmt.Sprintf(" | ~$%.4f", m.cumulativeCost)
	}

	// Abort hint.
	abortHint := "Ctrl-C: abort"

	// "Streaming" label preserved for backward compat with existing tests.
	rendered := fmt.Sprintf("%s Streaming | %s | %s | %s%s | %s", frame, tokStr, rateStr, elapsedStr, costStr, abortHint)

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
