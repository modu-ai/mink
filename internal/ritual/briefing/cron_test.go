package briefing

import (
	"context"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/hook"
	"go.uber.org/zap"
)

// stubCollector is a deterministic Collector for cron tests. Each test
// constructs its own to avoid shared state between subtests.
type stubCollector struct {
	mod    any
	status string
}

func (s *stubCollector) Collect(_ context.Context, _ string, _ time.Time) (any, string) {
	return s.mod, s.status
}

func newOkOrchestrator() *Orchestrator {
	return NewOrchestrator(
		&stubCollector{mod: &WeatherModule{Current: &WeatherCurrent{Temp: 18.0}}, status: "ok"},
		&stubCollector{mod: &RecallModule{}, status: "ok"},
		&stubCollector{mod: &DateModule{Today: "2026-05-14", DayOfWeek: "목요일"}, status: "ok"},
		&stubCollector{mod: &MantraModule{Text: "test"}, status: "ok"},
	)
}

func TestBriefingHookHandler_Handle_Runs(t *testing.T) {
	orch := newOkOrchestrator()
	var seen *BriefingPayload
	h := NewBriefingHookHandler(orch, "user-1", func(p *BriefingPayload, err error) {
		if err != nil {
			t.Errorf("onDone err: %v", err)
		}
		seen = p
	}, zap.NewNop())

	out, err := h.Handle(context.Background(), hook.HookInput{})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// out is intentionally benign.
	_ = out
	if seen == nil {
		t.Fatal("onDone payload nil")
	}
	if seen.Status["weather"] != "ok" {
		t.Errorf("Status[weather] = %s, want ok", seen.Status["weather"])
	}
}

func TestBriefingHookHandler_Handle_NilOrchestrator(t *testing.T) {
	h := NewBriefingHookHandler(nil, "u", nil, zap.NewNop())
	if _, err := h.Handle(context.Background(), hook.HookInput{}); err == nil {
		t.Error("expected error for nil orchestrator")
	}
}

func TestBriefingHookHandler_Handle_NilLogger(t *testing.T) {
	// Constructor should swap nil with Nop().
	h := NewBriefingHookHandler(newOkOrchestrator(), "u", nil, nil)
	if h.logger == nil {
		t.Error("logger should default to zap.NewNop")
	}
}

func TestBriefingHookHandler_Matches(t *testing.T) {
	h := NewBriefingHookHandler(newOkOrchestrator(), "u", nil, zap.NewNop())
	if !h.Matches(hook.HookInput{}) {
		t.Error("Matches should always be true")
	}
}

// TestCronWiring validates AC-011: registry returns the briefing handler
// for EvMorningBriefingTime after RegisterMorningBriefing.
func TestCronWiring(t *testing.T) {
	reg := hook.NewHookRegistry()
	orch := newOkOrchestrator()
	h := NewBriefingHookHandler(orch, "user-1", nil, zap.NewNop())

	if err := RegisterMorningBriefing(reg, h); err != nil {
		t.Fatalf("RegisterMorningBriefing: %v", err)
	}

	handlers := reg.Handlers(hook.EvMorningBriefingTime, hook.HookInput{})
	if len(handlers) == 0 {
		t.Fatal("expected at least 1 handler registered for EvMorningBriefingTime")
	}
	var found bool
	for _, hh := range handlers {
		if _, ok := hh.(*BriefingHookHandler); ok {
			found = true
			break
		}
	}
	if !found {
		t.Error("BriefingHookHandler not found in registry handlers")
	}
}

func TestRegisterMorningBriefing_NilRegistry(t *testing.T) {
	h := NewBriefingHookHandler(newOkOrchestrator(), "u", nil, zap.NewNop())
	if err := RegisterMorningBriefing(nil, h); err == nil {
		t.Error("expected error for nil registry")
	}
}

func TestRegisterMorningBriefing_NilHandler(t *testing.T) {
	reg := hook.NewHookRegistry()
	if err := RegisterMorningBriefing(reg, nil); err == nil {
		t.Error("expected error for nil handler")
	}
}

// TestBriefingHookHandler_OnDoneCalledOnError validates that onDone is still
// invoked when the orchestrator pipeline returns errors for all modules.
// This ensures fan-out renderers can produce a degraded "briefing unavailable"
// view (REQ-BR-041).
func TestBriefingHookHandler_OnDoneCalledOnError(t *testing.T) {
	badOrch := NewOrchestrator(
		&stubCollector{mod: &WeatherModule{}, status: "error"},
		&stubCollector{mod: &RecallModule{}, status: "error"},
		&stubCollector{mod: &DateModule{}, status: "error"},
		&stubCollector{mod: &MantraModule{}, status: "error"},
	)
	var got *BriefingPayload
	h := NewBriefingHookHandler(badOrch, "u", func(p *BriefingPayload, _ error) {
		got = p
	}, zap.NewNop())

	if _, err := h.Handle(context.Background(), hook.HookInput{}); err != nil {
		t.Fatalf("Handle (degraded): %v", err)
	}
	if got == nil {
		t.Fatal("expected onDone to be invoked with a payload even on degraded run")
	}
}
