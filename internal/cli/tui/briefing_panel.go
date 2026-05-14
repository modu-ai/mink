package tui

import (
	"strings"

	"github.com/modu-ai/mink/internal/ritual/briefing"
)

// BriefingPanel renders a briefing payload for embedding in the TUI session.
// The panel wraps internal/ritual/briefing.TUIPanel (the framework-agnostic
// structured output) with terminal-friendly framing characters.
//
// This is a snapshot-friendly helper -- the bubbletea tea.Model integration
// for live `/briefing` slash dispatch is delivered in a follow-up PR.
//
// SPEC-MINK-BRIEFING-001 REQ-BR-002 / REQ-BR-033, AC-008.
type BriefingPanel struct {
	// Title override. Empty string falls back to TUIPanel.Title.
	Title string
	// Payload is the briefing result to render.
	Payload *briefing.BriefingPayload
}

// NewBriefingPanel constructs a panel with default Title.
func NewBriefingPanel(payload *briefing.BriefingPayload) *BriefingPanel {
	return &BriefingPanel{Payload: payload}
}

// Render returns the panel as a deterministic, snapshot-friendly multiline
// string. The output is plain ASCII frame + Unicode body lines (the body
// preserves Korean glyphs from the underlying TUIPanel).
//
// Returns empty string when the panel or payload is nil.
func (p *BriefingPanel) Render() string {
	if p == nil || p.Payload == nil {
		return ""
	}
	panel := briefing.RenderTUI(p.Payload)
	if panel == nil {
		return ""
	}

	title := p.Title
	if title == "" {
		title = panel.Title
	}

	const sep = "========================================"
	const sepThin = "----------------------------------------"

	var sb strings.Builder
	sb.WriteString(sep)
	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(sep)
	sb.WriteString("\n")
	for _, line := range panel.Lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if panel.Footer != "" {
		sb.WriteString(sepThin)
		sb.WriteString("\n")
		sb.WriteString(panel.Footer)
		sb.WriteString("\n")
	}
	sb.WriteString(sep)
	sb.WriteString("\n")
	// T-305: Prepend crisis hotline response when the rendered text contains a
	// crisis keyword. REQ-BR-055 / REQ-BR-061 / AC-015.
	return briefing.PrependCrisisResponseIfDetected(sb.String())
}
