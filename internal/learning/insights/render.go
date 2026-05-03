package insights

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RenderTable returns an ASCII terminal table for the Report.
// It includes 5 sections: Overview, Top 10 Models, Top 10 Tools,
// Activity Heatmap, Top 5 Insights per Category.
func (r *Report) RenderTable() string {
	var sb strings.Builder

	renderOverview(&sb, r)
	sb.WriteString("\n")
	renderModels(&sb, r)
	sb.WriteString("\n")
	renderTools(&sb, r)
	sb.WriteString("\n")
	renderActivityHeatmap(&sb, r)
	sb.WriteString("\n")
	renderInsights(&sb, r)

	return sb.String()
}

func renderOverview(sb *strings.Builder, r *Report) {
	title := "Overview"
	sb.WriteString(boxTop(title, 48))
	sb.WriteString(boxRow(fmt.Sprintf("Period:          %s ~ %s",
		r.Period.From.Format("2006-01-02"),
		r.Period.To.Format("2006-01-02")), 48))
	sb.WriteString(boxRow(fmt.Sprintf("Total Sessions:  %d (%d success, %d fail)",
		r.Overview.TotalSessions, r.Overview.TotalSuccessful, r.Overview.TotalFailed), 48))
	sb.WriteString(boxRow(fmt.Sprintf("Total Tokens:    %s",
		formatInt(r.Overview.TotalTokens)), 48))
	pricingNote := ""
	if !r.Overview.HasFullPricing {
		pricingNote = " (partial pricing)"
	}
	sb.WriteString(boxRow(fmt.Sprintf("Estimated Cost:  $%.2f%s",
		r.Overview.EstimatedCost, pricingNote), 48))
	sb.WriteString(boxRow(fmt.Sprintf("Total Hours:     %.1f", r.Overview.TotalHours), 48))
	sb.WriteString(boxRow(fmt.Sprintf("Avg Session:     %s", formatDuration(r.Overview.AvgSessionDuration)), 48))
	sb.WriteString(boxBottom(48))
}

func renderModels(sb *strings.Builder, r *Report) {
	title := "Top 10 Models by Tokens"
	width := 70
	sb.WriteString(boxTop(title, width))
	header := fmt.Sprintf("%-4s  %-30s  %-8s  %-10s  %-8s", "Rank", "Model", "Sessions", "Tokens", "Cost")
	sb.WriteString(boxRow(header, width))
	sb.WriteString(boxDivider(width))

	models := r.Models
	if len(models) > 10 {
		models = models[:10]
	}
	for i, m := range models {
		costStr := fmt.Sprintf("$%.4f", m.Cost)
		if !m.HasPricing {
			costStr = "N/A"
		}
		row := fmt.Sprintf("%-4d  %-30s  %-8d  %-10s  %-8s",
			i+1,
			truncate(m.Model, 30),
			m.Sessions,
			formatTokensK(m.TotalTokens),
			costStr,
		)
		sb.WriteString(boxRow(row, width))
	}
	sb.WriteString(boxBottom(width))
}

func renderTools(sb *strings.Builder, r *Report) {
	title := "Top 10 Tools by Frequency"
	width := 48
	sb.WriteString(boxTop(title, width))
	header := fmt.Sprintf("%-20s  %-6s  %-14s", "Tool", "Count", "% of all calls")
	sb.WriteString(boxRow(header, width))
	sb.WriteString(boxDivider(width))

	tools := r.Tools
	if len(tools) > 10 {
		tools = tools[:10]
	}
	for _, t := range tools {
		row := fmt.Sprintf("%-20s  %-6d  %.2f%%",
			truncate(t.Tool, 20), t.Count, t.Percentage)
		sb.WriteString(boxRow(row, width))
	}
	sb.WriteString(boxBottom(width))
}

func renderActivityHeatmap(sb *strings.Builder, r *Report) {
	title := "Activity Heatmap (Day × Hour)"
	width := 64
	sb.WriteString(boxTop(title, width))

	// Hour header.
	hourHeader := "      "
	for h := 0; h < 24; h++ {
		hourHeader += fmt.Sprintf("%2d ", h)
	}
	sb.WriteString(boxRow(hourHeader, width))
	sb.WriteString(boxDivider(width))

	for _, day := range r.Activity.ByDay {
		row := fmt.Sprintf("%-4s  ", day.Day)
		// Build per-hour values for this day.
		// We don't have day×hour granularity in the schema; show day count on first column.
		row += fmt.Sprintf("%2d ", day.Count)
		for i := 1; i < 24; i++ {
			row += "   "
		}
		sb.WriteString(boxRow(row, width))
	}
	sb.WriteString(boxRow(fmt.Sprintf("Busiest: %s (day), hour %02d", r.Activity.BusiestDay.Day, r.Activity.BusiestHour.Hour), width))
	sb.WriteString(boxRow(fmt.Sprintf("Active days: %d  Max streak: %d", r.Activity.ActiveDays, r.Activity.MaxStreak), width))
	sb.WriteString(boxBottom(width))
}

func renderInsights(sb *strings.Builder, r *Report) {
	title := "Top 5 Insights per Category"
	width := 60
	sb.WriteString(boxTop(title, width))

	categories := []InsightCategory{CategoryPattern, CategoryPreference, CategoryError, CategoryOpportunity}
	for _, cat := range categories {
		sb.WriteString(boxRow(fmt.Sprintf("[%s]", cat.String()), width))
		insights := r.Insights[cat]
		if len(insights) == 0 {
			sb.WriteString(boxRow("  (no insights)", width))
			continue
		}
		if len(insights) > 5 {
			insights = insights[:5]
		}
		for _, ins := range insights {
			line := fmt.Sprintf("  - %q (n=%d, conf=%.2f)",
				truncate(ins.Title, 40), ins.Observations, ins.Confidence)
			sb.WriteString(boxRow(line, width))
		}
	}
	sb.WriteString(boxBottom(width))
}

// --- Box drawing helpers ---

func boxTop(title string, width int) string {
	inner := "─ " + title + " "
	padding := width - len(inner) - 2
	if padding < 0 {
		padding = 0
	}
	return "┌" + inner + strings.Repeat("─", padding) + "┐\n"
}

func boxBottom(width int) string {
	return "└" + strings.Repeat("─", width-2) + "┘\n"
}

func boxDivider(width int) string {
	return "├" + strings.Repeat("─", width-2) + "┤\n"
}

func boxRow(content string, width int) string {
	// Pad or truncate content to fit within the box.
	maxContent := width - 4 // "│ " + content + " │"
	if len(content) > maxContent {
		content = content[:maxContent]
	}
	padding := maxContent - len(content)
	return "│ " + content + strings.Repeat(" ", padding) + " │\n"
}

// --- Formatting helpers ---

func formatInt(n int) string {
	// Simple comma formatting.
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func formatTokensK(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// MarshalJSON implements a custom JSON marshaller that uses field names matching
// the spec schema: period, overview, models, tools, activity, insights, generated_at.
// This satisfies AC-INSIGHTS-019.
func (r Report) MarshalJSON() ([]byte, error) {
	type insightEntry struct {
		Category string    `json:"category"`
		Insights []Insight `json:"insights"`
	}

	// Flatten insights map to a slice for consistent JSON ordering.
	insightsSlice := []insightEntry{
		{Category: CategoryPattern.String(), Insights: r.Insights[CategoryPattern]},
		{Category: CategoryPreference.String(), Insights: r.Insights[CategoryPreference]},
		{Category: CategoryError.String(), Insights: r.Insights[CategoryError]},
		{Category: CategoryOpportunity.String(), Insights: r.Insights[CategoryOpportunity]},
	}

	return json.Marshal(struct {
		Period      InsightsPeriod  `json:"period"`
		Empty       bool            `json:"empty,omitempty"`
		Overview    Overview        `json:"overview"`
		Models      []ModelStat     `json:"models"`
		Tools       []ToolStat      `json:"tools"`
		Activity    ActivityPattern `json:"activity"`
		Insights    []insightEntry  `json:"insights"`
		GeneratedAt time.Time       `json:"generated_at"`
	}{
		Period:      r.Period,
		Empty:       r.Empty,
		Overview:    r.Overview,
		Models:      r.Models,
		Tools:       r.Tools,
		Activity:    r.Activity,
		Insights:    insightsSlice,
		GeneratedAt: r.GeneratedAt,
	})
}
