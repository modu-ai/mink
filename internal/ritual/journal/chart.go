package journal

import (
	"math"
	"os"
	"strings"
	"time"
)

// blockChars holds the 8-step Unicode block elements used in sparkline rendering.
// Index 0 = lowest valence bucket, index 7 = highest.
var blockChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// dayLabels maps time.Weekday (0=Sunday … 6=Saturday) to a short Korean label.
var dayLabels = [7]string{"일", "월", "화", "수", "목", "금", "토"}

// RenderChart renders a Trend's SparklinePoints as a compact terminal chart.
//
// Each sparkline point maps to a Unicode block glyph (▁▂▃▄▅▆▇█) based on its
// valence bucket. NaN values (missing days) are rendered as "·".
//
// ANSI color escapes are included when the NO_COLOR environment variable is
// empty or unset (https://no-color.org). Setting NO_COLOR to any non-empty
// value disables all escape sequences.
//
// The output contains one line per sparkline point.
// Returns an empty string when SparklinePoints is nil or empty.
func RenderChart(trend *Trend) string {
	if trend == nil || len(trend.SparklinePoints) == 0 {
		return ""
	}

	useColor := os.Getenv("NO_COLOR") == ""
	var sb strings.Builder

	for i, val := range trend.SparklinePoints {
		// Compute the weekday label for this index.
		day := trend.From.AddDate(0, 0, i)
		label := dayLabels[day.Weekday()]

		glyph := valenceToGlyph(val)
		if useColor && !math.IsNaN(val) {
			glyph = colorize(val, glyph)
		}

		sb.WriteString(label)
		sb.WriteString(" ")
		sb.WriteString(glyph)
		sb.WriteString("\n")
	}

	return sb.String()
}

// valenceToGlyph converts a valence value in [0, 1] to a Unicode block element.
// NaN returns "·" (middle dot) indicating a missing day.
func valenceToGlyph(val float64) string {
	if math.IsNaN(val) {
		return "·"
	}
	clamped := math.Max(0, math.Min(1, val))
	idx := int(clamped * float64(len(blockChars)))
	if idx >= len(blockChars) {
		idx = len(blockChars) - 1
	}
	return blockChars[idx]
}

// colorize wraps a glyph in ANSI color codes based on valence level.
// low < 0.35 → red; 0.35 ≤ val < 0.65 → yellow; val ≥ 0.65 → green.
func colorize(val float64, glyph string) string {
	var code string
	switch {
	case val < 0.35:
		code = "\x1b[31m" // red
	case val < 0.65:
		code = "\x1b[33m" // yellow
	default:
		code = "\x1b[32m" // green
	}
	return code + glyph + "\x1b[0m"
}

// ensure time package is used (required for dayLabel computation in RenderChart).
var _ = time.Sunday
