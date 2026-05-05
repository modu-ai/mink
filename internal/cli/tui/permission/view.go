// Package permission provides the permission modal sub-model.
package permission

import (
	"fmt"
	"strings"
)

// ViewWithLabels renders the modal using provided labels instead of the default optionLabels.
// labels[0..3] correspond to AllowOnce, AllowAlways, DenyOnce, DenyAlways respectively.
// prompt is shown as a context line above the options.
// Returns empty string when the modal is not Active.
//
// @MX:ANCHOR ViewWithLabels is the locale-aware rendering path used by tui/view.go.
// @MX:REASON fan_in >= 3: tui/view.go (main render), permission/modal_test.go (unit), future snapshot tests
func (m PermissionModel) ViewWithLabels(labels [4]string, prompt string) string {
	if !m.Active {
		return ""
	}

	var lines []string
	lines = append(lines, "+----------------------------------+")
	lines = append(lines, fmt.Sprintf("| Permission Request: %-11s |", m.ToolName))
	lines = append(lines, "+----------------------------------+")
	lines = append(lines, "|                                  |")

	if prompt != "" {
		// Truncate prompt to fit the box width (30 chars usable inside padding).
		displayPrompt := prompt
		if len(displayPrompt) > 30 {
			displayPrompt = displayPrompt[:27] + "..."
		}
		lines = append(lines, fmt.Sprintf("| %-32s |", displayPrompt))
	}

	for i, label := range labels {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		lines = append(lines, fmt.Sprintf("| %s%-30s |", cursor, label))
	}

	lines = append(lines, "|                                  |")
	lines = append(lines, "| [Up/Down] navigate [Enter] select|")
	lines = append(lines, "| [Esc] deny once                  |")
	lines = append(lines, "+----------------------------------+")

	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l + "\n")
	}
	return sb.String()
}

// View renders the permission modal as a string.
// Returns empty string when not Active.
func (m PermissionModel) View() string {
	if !m.Active {
		return ""
	}

	// Simple ASCII border for deterministic snapshot testing.
	var lines []string
	lines = append(lines, "+----------------------------------+")
	lines = append(lines, fmt.Sprintf("| Permission Request: %-11s |", m.ToolName))
	lines = append(lines, "+----------------------------------+")
	lines = append(lines, "|                                  |")

	for i, label := range optionLabels {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		lines = append(lines, fmt.Sprintf("| %s%-30s |", cursor, label))
	}

	lines = append(lines, "|                                  |")
	lines = append(lines, "| [Up/Down] navigate [Enter] select|")
	lines = append(lines, "| [Esc] deny once                  |")
	lines = append(lines, "+----------------------------------+")

	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l + "\n")
	}
	return sb.String()
}
