package briefing

import (
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/ritual/journal"
)

// crisisPayload builds a BriefingPayload with a crisis keyword in the mantra.
// Uses the phrase matching journal crisis keyword "죽고 싶".
func crisisPayload() *BriefingPayload {
	mantraText := "\xec\xa3\xbd\xea\xb3\xa0 \xec\x8b\xb6\xeb\x8b\xa4" // 죽고 싶다
	dayOfWeek := "\xeb\xaa\xa9\xec\x9a\x94\xec\x9d\xbc"               // 목요일
	return &BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 15, 7, 0, 0, 0, time.UTC),
		Weather:     WeatherModule{Current: &WeatherCurrent{Temp: 20, Condition: "Clear"}},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{},
		},
		DateCalendar: DateModule{Today: "2026-05-15", DayOfWeek: dayOfWeek},
		Mantra:       MantraModule{Text: mantraText},
		Status: map[string]string{
			"weather": "ok",
			"journal": "ok",
			"date":    "ok",
			"mantra":  "ok",
		},
	}
}

// cleanPayload builds a BriefingPayload with no crisis keywords.
func cleanPayload() *BriefingPayload {
	cleanMantra := "\xec\x98\xa4\xeb\x8a\x98\xeb\x8f\x84 \xed\x95\x9c \xea\xb1\xb8\xec\x9d\x8c" // 오늘도 한 걸음
	dayOfWeek := "\xeb\xaa\xa9\xec\x9a\x94\xec\x9d\xbc"
	return &BriefingPayload{
		GeneratedAt:   time.Date(2026, 5, 15, 7, 0, 0, 0, time.UTC),
		Weather:       WeatherModule{Current: &WeatherCurrent{Temp: 20, Condition: "Clear"}},
		JournalRecall: RecallModule{},
		DateCalendar:  DateModule{Today: "2026-05-15", DayOfWeek: dayOfWeek},
		Mantra:        MantraModule{Text: cleanMantra},
		Status: map[string]string{
			"weather": "ok",
			"journal": "ok",
			"date":    "ok",
			"mantra":  "ok",
		},
	}
}

// TestRenderers_CrisisPrepend verifies AC-015 for CLI + Telegram renderers.
// REQ-BR-055 / REQ-BR-061.
func TestRenderers_CrisisPrepend(t *testing.T) {
	t.Run("TestRenderCLI_CrisisPrepend", func(t *testing.T) {
		// With crisis keyword: output must start with CrisisResponse
		output := RenderCLI(crisisPayload(), true)
		if !strings.HasPrefix(output, journal.CrisisResponse) {
			t.Errorf("RenderCLI with crisis: output must start with CrisisResponse.\nGot prefix: %q", safePrefix(output, 100))
		}
		// Without crisis keyword: output must NOT start with CrisisResponse
		clean := RenderCLI(cleanPayload(), true)
		if strings.HasPrefix(clean, journal.CrisisResponse) {
			t.Error("RenderCLI without crisis: output must NOT start with CrisisResponse")
		}
	})

	t.Run("TestRenderTelegram_CrisisPrepend", func(t *testing.T) {
		// With crisis keyword
		text := RenderTelegramText(crisisPayload())
		if !strings.HasPrefix(text, journal.CrisisResponse) {
			t.Errorf("RenderTelegramText with crisis: output must start with CrisisResponse.\nGot prefix: %q", safePrefix(text, 100))
		}
		// Without crisis keyword
		clean := RenderTelegramText(cleanPayload())
		if strings.HasPrefix(clean, journal.CrisisResponse) {
			t.Error("RenderTelegramText without crisis: output must NOT start with CrisisResponse")
		}
	})
}

// safePrefix returns the first n bytes of s for error messages.
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
