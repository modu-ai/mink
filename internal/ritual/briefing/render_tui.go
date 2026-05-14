package briefing

import (
	"fmt"
	"strings"
)

// TUIPanel is the structured render output consumed by the TUI sessionmenu
// briefing panel. It is intentionally framework-agnostic (no bubbletea
// dependency) so that the briefing package can stay free of TUI imports.
// The bubbletea integration wraps a TUIPanel inside a panel widget.
//
// REQ-BR-002, REQ-BR-033.
type TUIPanel struct {
	// Title is rendered in the panel header (e.g. "MORNING BRIEFING").
	Title string
	// Lines are the body lines, one per row. Lines may contain embedded
	// newlines if a section needs explicit blank rows.
	Lines []string
	// Footer is the optional status summary rendered after a separator.
	Footer string
}

// RenderTUI builds a TUIPanel from the briefing payload. The output is
// terminal-friendly plain text (no ANSI codes); the bubbletea panel widget
// is responsible for applying its own styling on top of the strings.
//
// The content is semantically identical to the CLI plain rendering and the
// Telegram rendering -- only the rendering surface differs (AC-007).
func RenderTUI(payload *BriefingPayload) *TUIPanel {
	if payload == nil {
		return &TUIPanel{
			Title: "MORNING BRIEFING",
			Lines: []string{"(no briefing payload available)"},
		}
	}

	var lines []string

	// Weather
	lines = append(lines, "Weather")
	switch {
	case payload.Weather.Offline || payload.Status["weather"] == "offline":
		lines = append(lines, "  offline (cached)")
	case payload.Status["weather"] == "error":
		lines = append(lines, "  error")
	case payload.Status["weather"] == "timeout":
		lines = append(lines, "  timeout")
	default:
		if payload.Weather.Current != nil {
			c := payload.Weather.Current
			lines = append(lines, fmt.Sprintf("  Temp: %.1f°C (feels %.1f°C)", c.Temp, c.FeelsLike))
			lines = append(lines, fmt.Sprintf("  Cond: %s", c.Condition))
			if c.Location != "" {
				lines = append(lines, fmt.Sprintf("  Loc:  %s", c.Location))
			}
		}
		if payload.Weather.AirQuality != nil {
			lines = append(lines, fmt.Sprintf("  AQI:  %d (%s)", payload.Weather.AirQuality.AQI, payload.Weather.AirQuality.Level))
		}
	}
	lines = append(lines, "")

	// Journal Recall
	lines = append(lines, "Journal Recall")
	switch {
	case payload.JournalRecall.Offline || payload.Status["journal"] == "offline":
		lines = append(lines, "  journal unavailable")
	case payload.Status["journal"] == "error":
		lines = append(lines, "  error")
	default:
		if len(payload.JournalRecall.Anniversaries) > 0 {
			for _, a := range payload.JournalRecall.Anniversaries {
				lines = append(lines, fmt.Sprintf("  %dY: %s", a.YearsAgo, a.Date))
			}
		} else {
			lines = append(lines, "  no anniversaries today")
		}
		if payload.JournalRecall.MoodTrend != nil {
			lines = append(lines, fmt.Sprintf("  Mood: %s (%s)",
				payload.JournalRecall.MoodTrend.Trend,
				payload.JournalRecall.MoodTrend.Period))
		}
	}
	lines = append(lines, "")

	// Date
	lines = append(lines, "Date")
	lines = append(lines, fmt.Sprintf("  %s (%s)", payload.DateCalendar.Today, payload.DateCalendar.DayOfWeek))
	if payload.DateCalendar.SolarTerm != nil {
		lines = append(lines, fmt.Sprintf("  Solar Term: %s (%s)",
			payload.DateCalendar.SolarTerm.Name, payload.DateCalendar.SolarTerm.NameHanja))
	}
	if payload.DateCalendar.Holiday != nil {
		lines = append(lines, fmt.Sprintf("  Holiday: %s", payload.DateCalendar.Holiday.Name))
	}
	lines = append(lines, "")

	// Mantra
	if payload.Mantra.Text != "" {
		lines = append(lines, "Mantra")
		lines = append(lines, fmt.Sprintf("  \"%s\"", payload.Mantra.Text))
	}

	// Footer: module status summary
	var footerParts []string
	for _, mod := range []string{"weather", "journal", "date", "mantra"} {
		st := payload.Status[mod]
		if st == "" {
			st = "skipped"
		}
		footerParts = append(footerParts, mod+":"+st)
	}
	footer := strings.Join(footerParts, "  ")

	return &TUIPanel{
		Title:  "MORNING BRIEFING",
		Lines:  lines,
		Footer: footer,
	}
}

// String renders the TUIPanel as a single multiline string suitable for
// snapshot testing or for embedding in a plain-text fallback view.
func (p *TUIPanel) String() string {
	if p == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(p.Title)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("-", len(p.Title)))
	sb.WriteString("\n")
	for _, l := range p.Lines {
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	if p.Footer != "" {
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		sb.WriteString(p.Footer)
		sb.WriteString("\n")
	}
	return sb.String()
}
