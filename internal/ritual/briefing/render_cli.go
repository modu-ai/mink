package briefing

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ModuleStatusOrder defines the canonical output order for the Module Status section.
// Using a fixed slice instead of map iteration prevents Go map non-determinism.
// T-309 / REQ-BR-063 / AC-017.
var ModuleStatusOrder = []string{"weather", "journal", "date", "mantra"}

// RenderCLI renders the briefing payload to a formatted string.
// If plain is false and stdout is a TTY, uses ANSI colors and emoji.
// Otherwise, renders plain text without formatting.
func RenderCLI(payload *BriefingPayload, plain bool) string {
	var sb strings.Builder

	// Detect TTY if not explicitly forced to plain
	isTTY := !plain && isTerminal(os.Stdout)

	// Header
	if isTTY {
		sb.WriteString("\n" + color("┌────────────────────────────────────────┐", colorBold) + "\n")
		sb.WriteString(color("│         🌅 MORNING BRIEFING 🌅         │", colorBold) + "\n")
		sb.WriteString(color("└────────────────────────────────────────┘", colorBold) + "\n\n")
	} else {
		sb.WriteString("MORNING BRIEFING\n")
		sb.WriteString(strings.Repeat("=", 40) + "\n\n")
	}

	// Timestamp
	sb.WriteString(formatLine("Generated", payload.GeneratedAt.Format(time.RFC1123), "", isTTY))
	sb.WriteString("\n")

	// Weather Section
	renderWeather(&sb, &payload.Weather, payload.Status["weather"], isTTY)

	// Journal Section
	renderJournal(&sb, &payload.JournalRecall, payload.Status["journal"], isTTY)

	// Date Section
	renderDate(&sb, &payload.DateCalendar, payload.Status["date"], isTTY)

	// Mantra Section
	renderMantra(&sb, &payload.Mantra, payload.Status["mantra"], isTTY)

	// Status Summary
	if isTTY {
		sb.WriteString("\n" + color("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━", colorBold) + "\n")
		sb.WriteString(color("Module Status:", colorBold) + "\n")
	} else {
		sb.WriteString("\n" + strings.Repeat("-", 40) + "\n")
		sb.WriteString("Module Status:\n")
	}

	// Use ModuleStatusOrder to guarantee deterministic output regardless of
	// Go map iteration randomization (T-309 / REQ-BR-063 / AC-017).
	for _, module := range ModuleStatusOrder {
		status, ok := payload.Status[module]
		if !ok {
			status = "skipped"
		}
		sb.WriteString(formatStatus(module, status, isTTY))
	}

	sb.WriteString("\n")

	// T-305: Prepend crisis hotline response when payload or rendered text
	// contains a crisis keyword. REQ-BR-055 / REQ-BR-061 / AC-015.
	return PrependCrisisResponseIfDetected(sb.String())
}

// renderWeather renders the weather module section.
func renderWeather(sb *strings.Builder, weather *WeatherModule, status string, isTTY bool) {
	header := "Weather"
	if isTTY {
		header = "🌤️  " + header
	}
	renderSectionHeader(sb, header, isTTY)

	if status == "offline" || weather.Offline {
		sb.WriteString(formatLine("Status", "offline (using cached data)", "", isTTY))
	} else if status == "error" {
		sb.WriteString(formatLine("Status", "error", "red", isTTY))
	} else {
		if weather.Current != nil {
			sb.WriteString(formatLine("Temperature", formatTemp(weather.Current.Temp), "", isTTY))
			sb.WriteString(formatLine("Feels Like", formatTemp(weather.Current.FeelsLike), "", isTTY))
			sb.WriteString(formatLine("Condition", weather.Current.Condition, "", isTTY))
			sb.WriteString(formatLine("Humidity", fmt.Sprintf("%.0f%%", weather.Current.Humidity), "", isTTY))
			if weather.Current.Location != "" {
				sb.WriteString(formatLine("Location", weather.Current.Location, "", isTTY))
			}
		}

		if weather.AirQuality != nil {
			aqiColor := aqiColor(weather.AirQuality.AQI)
			sb.WriteString(formatLine("AQI", fmt.Sprintf("%d (%s)", weather.AirQuality.AQI, weather.AirQuality.Level), aqiColor, isTTY))
		}
	}
	sb.WriteString("\n")
}

// renderJournal renders the journal recall module section.
func renderJournal(sb *strings.Builder, recall *RecallModule, status string, isTTY bool) {
	header := "Journal Recall"
	if isTTY {
		header = "📔 " + header
	}
	renderSectionHeader(sb, header, isTTY)

	if status == "offline" || recall.Offline {
		sb.WriteString(formatLine("Status", "offline (journal access unavailable)", "", isTTY))
	} else if status == "error" {
		sb.WriteString(formatLine("Status", "error", "red", isTTY))
	} else {
		if len(recall.Anniversaries) > 0 {
			sb.WriteString(formatLine("Anniversaries", fmt.Sprintf("%d found", len(recall.Anniversaries)), "", isTTY))
			for _, ann := range recall.Anniversaries {
				emoji := ann.EmojiMood
				if emoji == "" {
					emoji = "📝"
				}
				fmt.Fprintf(sb, "  %s %dY: %s\n", emoji, ann.YearsAgo, ann.Date)
			}
		} else {
			sb.WriteString(formatLine("Anniversaries", "none", "", isTTY))
		}

		if recall.MoodTrend != nil {
			trendIcon := moodTrendIcon(recall.MoodTrend.Trend)
			sb.WriteString(formatLine("Mood Trend", fmt.Sprintf("%s %s", recall.MoodTrend.Trend, trendIcon), "", isTTY))
			sb.WriteString(formatLine("  Period", recall.MoodTrend.Period, "", isTTY))
		}
	}
	sb.WriteString("\n")
}

// renderDate renders the date calendar module section.
func renderDate(sb *strings.Builder, date *DateModule, status string, isTTY bool) {
	header := "Date & Calendar"
	if isTTY {
		header = "📅 " + header
	}
	renderSectionHeader(sb, header, isTTY)

	if status == "error" {
		sb.WriteString(formatLine("Status", "error", "red", isTTY))
	} else {
		sb.WriteString(formatLine("Today", date.Today, "", isTTY))
		sb.WriteString(formatLine("Day of Week", date.DayOfWeek, "", isTTY))

		if date.SolarTerm != nil {
			sb.WriteString(formatLine("Solar Term", fmt.Sprintf("%s (%s)", date.SolarTerm.Name, date.SolarTerm.NameHanja), "", isTTY))
		}

		if date.Holiday != nil {
			holidayStatus := ""
			if date.Holiday.IsHoliday {
				holidayStatus = " (holiday)"
			}
			sb.WriteString(formatLine("Holiday", fmt.Sprintf("%s (%s)%s", date.Holiday.Name, date.Holiday.NameHanja, holidayStatus), "", isTTY))
		}
	}
	sb.WriteString("\n")
}

// renderMantra renders the mantra module section.
func renderMantra(sb *strings.Builder, mantra *MantraModule, status string, isTTY bool) {
	header := "Daily Mantra"
	if isTTY {
		header = "✨ " + header
	}
	renderSectionHeader(sb, header, isTTY)

	if status == "error" {
		sb.WriteString(formatLine("Status", "error", "red", isTTY))
	} else if mantra.Text != "" {
		if isTTY {
			sb.WriteString(color("  \""+mantra.Text+"\"", colorCyan) + "\n")
		} else {
			fmt.Fprintf(sb, "  \"%s\"\n", mantra.Text)
		}
		if mantra.Source != "" {
			sb.WriteString(formatLine("Source", mantra.Source, "", isTTY))
		}
		sb.WriteString(formatLine("Index", fmt.Sprintf("%d/%d", mantra.Index+1, mantra.Total), "", isTTY))
	}
	sb.WriteString("\n")
}

// renderSectionHeader renders a section header with consistent formatting.
func renderSectionHeader(sb *strings.Builder, title string, isTTY bool) {
	if isTTY {
		sb.WriteString(color(title, colorBold+colorYellow) + "\n")
	} else {
		sb.WriteString(title + "\n")
	}
}

// formatLine formats a key-value pair with consistent spacing.
func formatLine(key, value, valueColor string, isTTY bool) string {
	const keyWidth = 14
	paddedKey := key + ":"
	if len(paddedKey) < keyWidth {
		paddedKey += strings.Repeat(" ", keyWidth-len(paddedKey))
	}

	if valueColor != "" && isTTY {
		return fmt.Sprintf("  %s%s\n", paddedKey, color(value, valueColor))
	}
	return fmt.Sprintf("  %s%s\n", paddedKey, value)
}

// formatStatus formats a module status line.
func formatStatus(module, status string, isTTY bool) string {
	var statusIcon string
	var statusColor string

	switch status {
	case "ok":
		statusIcon = "✓"
		statusColor = colorGreen
	case "offline":
		statusIcon = "○"
		statusColor = colorYellow
	case "timeout":
		statusIcon = "⏱"
		statusColor = colorYellow
	case "error":
		statusIcon = "✗"
		statusColor = colorRed
	case "skipped":
		statusIcon = "⊘"
		statusColor = colorGray
	default:
		statusIcon = "?"
		statusColor = colorGray
	}

	moduleName := cases.Title(language.Und).String(module)
	if isTTY {
		return fmt.Sprintf("  %s: %s%s\n", moduleName, color(status, statusColor), color(statusIcon, statusColor))
	}
	return fmt.Sprintf("  %s: %s %s\n", moduleName, status, statusIcon)
}

// formatTemp formats temperature with degree symbol.
func formatTemp(temp float64) string {
	return fmt.Sprintf("%.1f°C", temp)
}

// aqiColor returns the appropriate color for AQI level.
func aqiColor(aqi int) string {
	switch {
	case aqi <= 50:
		return "green"
	case aqi <= 100:
		return "yellow"
	case aqi <= 150:
		return "orange"
	default:
		return "red"
	}
}

// moodTrendIcon returns an emoji representing mood trend.
func moodTrendIcon(trend string) string {
	switch trend {
	case "improving":
		return "📈"
	case "stable":
		return "➡️"
	case "declining":
		return "📉"
	default:
		return "➡️"
	}
}

// isTerminal checks if the writer is a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// ANSI color codes
const (
	colorBold   = "1"
	colorRed    = "31"
	colorGreen  = "32"
	colorYellow = "33"
	colorCyan   = "36"
	colorGray   = "90"
)

// color wraps text with ANSI color codes.
func color(text, colorCode string) string {
	return fmt.Sprintf("\033[%sm%s\033[0m", colorCode, text)
}
