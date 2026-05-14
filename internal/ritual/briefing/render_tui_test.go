package briefing

import (
	"strings"
	"testing"
	"time"
)

func sampleBriefingForTUI() *BriefingPayload {
	return &BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC),
		Status: map[string]string{
			"weather": "ok",
			"journal": "ok",
			"date":    "ok",
			"mantra":  "ok",
		},
		Weather: WeatherModule{
			Current: &WeatherCurrent{
				Temp:      18.5,
				FeelsLike: 17.0,
				Condition: "Clear",
				Location:  "Seoul",
			},
			AirQuality: &AirQuality{AQI: 45, Level: "good"},
		},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{YearsAgo: 1, Date: "2025-05-14"},
			},
		},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
		Mantra: MantraModule{Text: "오늘도 한 걸음"},
	}
}

// TestRenderTUI_HappyPath validates the TUI panel surfaces semantic content
// for all 4 modules.
func TestRenderTUI_HappyPath(t *testing.T) {
	panel := RenderTUI(sampleBriefingForTUI())
	if panel == nil {
		t.Fatal("panel nil")
	}
	if panel.Title != "MORNING BRIEFING" {
		t.Errorf("Title = %s, want MORNING BRIEFING", panel.Title)
	}

	all := strings.Join(panel.Lines, "\n")
	want := []string{
		"Weather", "18.5°C", "Seoul", "AQI:  45",
		"Journal Recall", "2025-05-14",
		"Date", "2026-05-14", "목요일",
		"Mantra", "오늘도 한 걸음",
	}
	for _, w := range want {
		if !strings.Contains(all, w) {
			t.Errorf("panel lines missing %q\n%s", w, all)
		}
	}

	if !strings.Contains(panel.Footer, "weather:ok") {
		t.Errorf("Footer missing weather:ok: %s", panel.Footer)
	}
}

// TestRenderTUI_OfflineWeather verifies graceful degradation.
func TestRenderTUI_OfflineWeather(t *testing.T) {
	p := sampleBriefingForTUI()
	p.Weather.Offline = true
	p.Status["weather"] = "offline"

	panel := RenderTUI(p)
	all := strings.Join(panel.Lines, "\n")
	if !strings.Contains(all, "offline") {
		t.Errorf("expected offline marker, got: %s", all)
	}
}

// TestRenderTUI_NoMantra omits the mantra section.
func TestRenderTUI_NoMantra(t *testing.T) {
	p := sampleBriefingForTUI()
	p.Mantra.Text = ""
	panel := RenderTUI(p)
	all := strings.Join(panel.Lines, "\n")
	if strings.Contains(all, "Mantra") {
		t.Errorf("expected no Mantra header, got: %s", all)
	}
}

// TestRenderTUI_NilPayload returns a safe placeholder panel.
func TestRenderTUI_NilPayload(t *testing.T) {
	panel := RenderTUI(nil)
	if panel == nil {
		t.Fatal("panel nil")
	}
	if len(panel.Lines) == 0 {
		t.Error("expected at least one placeholder line")
	}
}

// TestTUIPanel_String renders the panel as a snapshot-friendly string.
func TestTUIPanel_String(t *testing.T) {
	panel := RenderTUI(sampleBriefingForTUI())
	s := panel.String()
	if !strings.Contains(s, "MORNING BRIEFING") {
		t.Errorf("String() missing title: %s", s)
	}
	if !strings.Contains(s, "오늘도 한 걸음") {
		t.Errorf("String() missing mantra: %s", s)
	}
	if !strings.HasSuffix(strings.TrimSpace(s), "weather:ok  journal:ok  date:ok  mantra:ok") {
		t.Errorf("String() footer wrong: ...%s", s[len(s)-100:])
	}
}

// TestTUIPanel_NilString safely handles a nil pointer receiver.
func TestTUIPanel_NilString(t *testing.T) {
	var p *TUIPanel
	if got := p.String(); got != "" {
		t.Errorf("nil panel String = %q, want empty", got)
	}
}

// TestRenderTUI_DegradedModuleStatuses ensures status footer reflects
// non-ok statuses for all 4 modules.
func TestRenderTUI_DegradedModuleStatuses(t *testing.T) {
	p := &BriefingPayload{
		Status: map[string]string{
			"weather": "error",
			"journal": "timeout",
			"date":    "skipped",
		},
		DateCalendar: DateModule{Today: "2026-05-14", DayOfWeek: "목요일"},
	}
	panel := RenderTUI(p)
	if !strings.Contains(panel.Footer, "weather:error") {
		t.Errorf("Footer missing weather:error: %s", panel.Footer)
	}
	if !strings.Contains(panel.Footer, "journal:timeout") {
		t.Errorf("Footer missing journal:timeout: %s", panel.Footer)
	}
	if !strings.Contains(panel.Footer, "date:skipped") {
		t.Errorf("Footer missing date:skipped: %s", panel.Footer)
	}
	if !strings.Contains(panel.Footer, "mantra:skipped") {
		t.Errorf("Footer missing mantra:skipped (auto-fill): %s", panel.Footer)
	}
}
