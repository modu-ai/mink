// Package permission provides the permission modal sub-model.
package permission

import (
	"fmt"
	"strings"
)

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
