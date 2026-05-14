package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/ritual/briefing"
	"github.com/modu-ai/mink/internal/ritual/journal"
)

// TestBriefingPanel_CrisisPrepend verifies AC-015 (TUI portion) — REQ-BR-055 / REQ-BR-061.
func TestBriefingPanel_CrisisPrepend(t *testing.T) {
	t.Run("WithCrisisKeyword", func(t *testing.T) {
		// Payload with crisis keyword in mantra.
		// Use hex-escaped "죽고 싶다" to match the journal crisis keyword "죽고 싶".
		crisisMantra := "\xec\xa3\xbd\xea\xb3\xa0 \xec\x8b\xb6\xeb\x8b\xa4" // 죽고 싶다
		payload := &briefing.BriefingPayload{
			GeneratedAt:   time.Date(2026, 5, 15, 7, 0, 0, 0, time.UTC),
			Mantra:        briefing.MantraModule{Text: crisisMantra},
			DateCalendar:  briefing.DateModule{Today: "2026-05-15", DayOfWeek: "\xeb\xaa\xa9\xec\x9a\x94\xec\x9d\xbc"},
			JournalRecall: briefing.RecallModule{},
			Status: map[string]string{
				"weather": "ok",
				"journal": "ok",
				"date":    "ok",
				"mantra":  "ok",
			},
		}
		panel := NewBriefingPanel(payload)
		rendered := panel.Render()

		if !strings.HasPrefix(rendered, journal.CrisisResponse) {
			t.Errorf("BriefingPanel.Render() with crisis keyword: output must start with CrisisResponse.\nGot prefix: %q", briefingSafePrefix(rendered, 100))
		}
	})

	t.Run("WithoutCrisisKeyword", func(t *testing.T) {
		// Use hex-escaped "오늘도 한 걸음" — no crisis keyword present.
		cleanMantra := "\xec\x98\xa4\xeb\x8a\x98\xeb\x8f\x84 \xed\x95\x9c \xea\xb1\xb8\xec\x9d\x8c" // 오늘도 한 걸음
		payload := &briefing.BriefingPayload{
			GeneratedAt:   time.Date(2026, 5, 15, 7, 0, 0, 0, time.UTC),
			Mantra:        briefing.MantraModule{Text: cleanMantra},
			DateCalendar:  briefing.DateModule{Today: "2026-05-15", DayOfWeek: "\xeb\xaa\xa9\xec\x9a\x94\xec\x9d\xbc"},
			JournalRecall: briefing.RecallModule{},
			Status: map[string]string{
				"weather": "ok",
				"journal": "ok",
				"date":    "ok",
				"mantra":  "ok",
			},
		}
		panel := NewBriefingPanel(payload)
		rendered := panel.Render()

		if strings.HasPrefix(rendered, journal.CrisisResponse) {
			t.Error("BriefingPanel.Render() without crisis keyword: output must NOT start with CrisisResponse")
		}
	})
}

// briefingSafePrefix returns the first n bytes of s for error messages.
func briefingSafePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
