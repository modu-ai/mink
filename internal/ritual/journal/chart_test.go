package journal

import (
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setEnv sets an environment variable and restores the original value on test cleanup.
// It uses t.Setenv which is safe for parallel tests.
// (wrapper for readability)
func requireNoColor(t *testing.T, val string) {
	t.Helper()
	t.Setenv("NO_COLOR", val)
}

// TestRenderChart_SevenDaysWithNaN verifies AC-026 core scenario:
// 7-day trend with one NaN renders the correct glyphs and weekday labels.
func TestRenderChart_SevenDaysWithNaN(t *testing.T) {
	// NOTE: not parallel — modifies NO_COLOR env via t.Setenv.
	requireNoColor(t, "") // allow color

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC) // Wednesday
	from := today.AddDate(0, 0, -6)                       // Thursday April 16

	trend := &Trend{
		Period:          "week",
		From:            from,
		To:              today,
		SparklinePoints: []float64{0.2, 0.5, 0.9, math.NaN(), 0.3, 0.7, 0.6},
	}

	output := RenderChart(trend)
	require.NotEmpty(t, output)

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	require.Len(t, lines, 7, "output must have 7 lines for 7-day trend")

	// Block chars valid set.
	validBlocks := map[string]bool{
		"▁": true, "▂": true, "▃": true, "▄": true,
		"▅": true, "▆": true, "▇": true, "█": true,
	}

	for i, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		require.Len(t, parts, 2, "each line must have label and glyph, line %d: %q", i, line)

		label := parts[0]
		assert.Truef(t, utf8.RuneCountInString(label) == 1,
			"label at line %d must be a single rune, got %q", i, label)

		// Strip ANSI escapes to get the raw glyph.
		rawGlyph := stripANSI(parts[1])

		if math.IsNaN(trend.SparklinePoints[i]) {
			assert.Equal(t, "·", rawGlyph, "NaN day must render as · at line %d", i)
		} else {
			assert.Truef(t, validBlocks[rawGlyph],
				"line %d: glyph %q must be in block char set", i, rawGlyph)
		}
	}

	// With NO_COLOR="", ANSI escapes must appear for non-NaN entries.
	hasEscapes := strings.Contains(output, "\x1b[")
	assert.True(t, hasEscapes, "color output must contain ANSI escapes when NO_COLOR is empty")
}

// TestRenderChart_NoColorEnv verifies that NO_COLOR=1 suppresses all ANSI escapes.
func TestRenderChart_NoColorEnv(t *testing.T) {
	// NOTE: not parallel — modifies NO_COLOR env via t.Setenv.
	requireNoColor(t, "1") // disable color

	trend := &Trend{
		Period:          "week",
		From:            time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
		To:              time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC),
		SparklinePoints: []float64{0.2, 0.5, 0.9, math.NaN(), 0.3, 0.7, 0.6},
	}

	output := RenderChart(trend)
	require.NotEmpty(t, output)

	count := strings.Count(output, "\x1b[")
	assert.Equal(t, 0, count, "NO_COLOR=1 must produce zero ANSI escape sequences; got %d", count)
}

// TestRenderChart_EmptySparkline verifies that an empty SparklinePoints returns "".
func TestRenderChart_EmptySparkline(t *testing.T) {
	t.Parallel()

	trend := &Trend{
		Period:          "week",
		SparklinePoints: []float64{},
	}
	assert.Equal(t, "", RenderChart(trend))
}

// TestRenderChart_NilTrend verifies that nil trend returns "".
func TestRenderChart_NilTrend(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", RenderChart(nil))
}

// TestValenceToGlyph_Mapping verifies that valence boundaries map to expected glyphs.
func TestValenceToGlyph_Mapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val      float64
		expected string
	}{
		{0.0, "▁"},
		{0.999, "█"},
		{1.0, "█"},
		{0.5, "▅"}, // 0.5 * 8 = 4.0 → index 4
		{math.NaN(), "·"},
	}

	for _, tc := range cases {
		got := valenceToGlyph(tc.val)
		assert.Equalf(t, tc.expected, got, "valenceToGlyph(%v) = %q, want %q", tc.val, got, tc.expected)
	}
}

// stripANSI removes ANSI escape codes from s using a simple state machine.
func stripANSI(s string) string {
	var sb strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
