// Package sessionmenu provides the View renderer for the Ctrl-R overlay.
package sessionmenu

import (
	"fmt"
	"strings"
)

// View renders the sessionmenu overlay as a string.
// header is the catalog.SessionMenuHeader. empty is catalog.SessionMenuEmpty.
// Returns empty string when the overlay is not open.
// REQ-CLITUI3-003, REQ-CLITUI3-004
func (m Model) View(header, empty string) string {
	if !m.open {
		return ""
	}

	var lines []string
	lines = append(lines, "+----------------------------------+")
	lines = append(lines, fmt.Sprintf("| %-32s |", header))
	lines = append(lines, "+----------------------------------+")

	if len(m.entries) == 0 {
		lines = append(lines, fmt.Sprintf("| %-32s |", empty))
	} else {
		for i, e := range m.entries {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			name := e.Name
			if len(name) > 30 {
				name = name[:27] + "..."
			}
			lines = append(lines, fmt.Sprintf("| %s%-30s |", cursor, name))
		}
	}

	lines = append(lines, "+----------------------------------+")

	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l + "\n")
	}
	return sb.String()
}
