package commands

import (
	"testing"

	"github.com/modu-ai/mink/internal/ritual/briefing"
)

// TestBriefingCmd_RealFactory verifies AC-013 — REQ-BR-001 / REQ-BR-002 / REQ-BR-064.
// It confirms that RealBriefingCollectorFactory exists and produces 4 non-nil collectors.
func TestBriefingCmd_RealFactory(t *testing.T) {
	deps := RealCollectorDeps{
		Weather:  briefing.NewWeatherCollector(&briefing.WeatherFetcherImpl{}),
		Journal:  briefing.NewJournalCollector(nil), // nil recaller is acceptable for struct creation
		Date:     briefing.NewDateCollector(),
		Mantra:   briefing.NewMantraCollector(briefing.DefaultConfig()),
		Location: "Seoul",
	}

	factory := RealBriefingCollectorFactory(deps)
	if factory == nil {
		t.Fatal("RealBriefingCollectorFactory must not return nil factory")
	}

	w, j, d, m := factory()
	if w == nil {
		t.Error("weather collector must not be nil")
	}
	if j == nil {
		t.Error("journal collector must not be nil")
	}
	if d == nil {
		t.Error("date collector must not be nil")
	}
	if m == nil {
		t.Error("mantra collector must not be nil")
	}
}

// TestRealCollectorDeps_NilDeps verifies that absent deps produce descriptive error behavior.
func TestRealCollectorDeps_NilDeps(t *testing.T) {
	// This test documents the expected behavior when deps are nil.
	// Currently we just verify RealBriefingCollectorFactory handles it gracefully.
	deps := RealCollectorDeps{
		// All nil — production binary should reject this at startup (REQ-BR-064)
		Weather:  nil,
		Journal:  nil,
		Date:     nil,
		Mantra:   nil,
		Location: "",
	}
	// Factory function itself must not panic even with nil deps.
	// The command RunE should validate and return error early.
	factory := RealBriefingCollectorFactory(deps)
	if factory == nil {
		t.Error("RealBriefingCollectorFactory must return a factory function (validation is deferred to RunE)")
	}
}
