package insights

import (
	"math"
	"sort"
	"strings"

	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// aggregateTools computes per-tool usage statistics from a set of trajectories.
// Tool calls are inferred from RoleTool entries in conversation turns.
// Returns results sorted by Count descending (ties broken by tool name).
func aggregateTools(trajectories []*trajectory.Trajectory) []ToolStat {
	countMap := make(map[string]int)
	sessionMap := make(map[string]map[string]struct{})

	for _, t := range trajectories {
		for _, entry := range t.Conversations {
			if entry.From != trajectory.RoleTool {
				continue
			}
			toolName := extractToolName(entry.Value)
			if toolName == "" {
				continue
			}
			countMap[toolName]++
			if sessionMap[toolName] == nil {
				sessionMap[toolName] = make(map[string]struct{})
			}
			sessionMap[toolName][t.SessionID] = struct{}{}
		}
	}

	// Compute total calls for percentage.
	total := 0
	for _, c := range countMap {
		total += c
	}

	result := make([]ToolStat, 0, len(countMap))
	for tool, count := range countMap {
		pct := 0.0
		if total > 0 {
			pct = roundTo2(float64(count) / float64(total) * 100.0)
		}
		result = append(result, ToolStat{
			Tool:       tool,
			Count:      count,
			Percentage: pct,
			UsedIn:     len(sessionMap[tool]),
		})
	}

	// Sort by Count descending, then by tool name ascending for determinism.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Tool < result[j].Tool
	})

	return result
}

// extractToolName extracts a tool name from a RoleTool entry value.
// Tool entries typically start with "tool_name:" or are just the tool name.
func extractToolName(value string) string {
	// Try "tool_name: ..." format.
	if idx := strings.Index(value, ":"); idx > 0 {
		candidate := strings.TrimSpace(value[:idx])
		if isValidToolName(candidate) {
			return candidate
		}
	}
	// Fall back to the entire value if it looks like a bare tool name.
	trimmed := strings.TrimSpace(value)
	if isValidToolName(trimmed) {
		return trimmed
	}
	return ""
}

// isValidToolName returns true for simple identifier-like strings.
func isValidToolName(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !isAlphanumOrUnderscore(r) {
			return false
		}
	}
	return true
}

// isAlphanumOrUnderscore reports whether r is a letter, digit, or underscore.
func isAlphanumOrUnderscore(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

// roundTo2 rounds a float64 to 2 decimal places.
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}
