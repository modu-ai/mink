package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/ritual/briefing"
)

// updateGolden, when set, rewrites the snapshot file from the current output.
//
// Usage: go test ./internal/cli/tui -run TestBriefingPanel_Snapshot -update-golden
var updateGolden = flag.Bool("update-golden", false, "rewrite TUI briefing panel snapshot from current output")

func sampleBriefingForPanel() *briefing.BriefingPayload {
	return &briefing.BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC),
		Status: map[string]string{
			"weather": "ok",
			"journal": "ok",
			"date":    "ok",
			"mantra":  "ok",
		},
		Weather: briefing.WeatherModule{
			Current: &briefing.WeatherCurrent{
				Temp:      18.5,
				FeelsLike: 17.0,
				Condition: "Clear",
				Location:  "Seoul",
			},
			AirQuality: &briefing.AirQuality{AQI: 45, Level: "good"},
		},
		JournalRecall: briefing.RecallModule{
			Anniversaries: []*briefing.AnniversaryEntry{
				{YearsAgo: 1, Date: "2025-05-14"},
			},
		},
		DateCalendar: briefing.DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
		Mantra: briefing.MantraModule{Text: "오늘도 한 걸음"},
	}
}

// TestBriefingPanel_Snapshot validates AC-008: the TUI panel renders the
// briefing payload to a deterministic, snapshot-stable representation.
//
// Snapshot file: internal/cli/tui/snapshots/briefing_panel.txt
// To regenerate after intentional changes:
//
//	go test ./internal/cli/tui -run TestBriefingPanel_Snapshot -update-golden
//
// SPEC-MINK-BRIEFING-001 AC-008.
func TestBriefingPanel_Snapshot(t *testing.T) {
	panel := NewBriefingPanel(sampleBriefingForPanel())
	got := panel.Render()

	if got == "" {
		t.Fatal("panel rendered empty string")
	}

	goldenPath := filepath.Join("snapshots", "briefing_panel.txt")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir snapshots: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden snapshot updated: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden snapshot %s: %v\nRun with -update-golden to bootstrap.", goldenPath, err)
	}

	if got != string(want) {
		t.Errorf("snapshot mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestBriefingPanel_NilPayload validates safe handling of nil inputs.
func TestBriefingPanel_NilPayload(t *testing.T) {
	if got := (&BriefingPanel{}).Render(); got != "" {
		t.Errorf("empty panel = %q, want empty", got)
	}
	if got := NewBriefingPanel(nil).Render(); got != "" {
		t.Errorf("nil payload panel = %q, want empty", got)
	}
	var p *BriefingPanel
	if got := p.Render(); got != "" {
		t.Errorf("nil receiver panel = %q, want empty", got)
	}
}

// TestBriefingPanel_TitleOverride verifies the Title field overrides the
// underlying TUIPanel title.
func TestBriefingPanel_TitleOverride(t *testing.T) {
	panel := &BriefingPanel{
		Title:   "CUSTOM TITLE",
		Payload: sampleBriefingForPanel(),
	}
	out := panel.Render()
	if !strings.Contains(out, "CUSTOM TITLE") {
		t.Errorf("Render() does not contain custom title: %s", out)
	}
	if strings.Contains(out, "  MORNING BRIEFING\n") {
		t.Errorf("Render() leaked default title with custom override: %s", out)
	}
}

// TestBriefingPanel_DegradedStatus ensures the footer reflects non-ok statuses.
func TestBriefingPanel_DegradedStatus(t *testing.T) {
	p := sampleBriefingForPanel()
	p.Status["weather"] = "offline"
	p.Weather.Offline = true

	out := NewBriefingPanel(p).Render()
	if !strings.Contains(out, "weather:offline") {
		t.Errorf("footer missing weather:offline: %s", out)
	}
	if !strings.Contains(out, "offline (cached)") {
		t.Errorf("body missing offline marker: %s", out)
	}
}
