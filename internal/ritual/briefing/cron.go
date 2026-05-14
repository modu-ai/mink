package briefing

import (
	"context"
	"fmt"
	"time"

	"github.com/modu-ai/mink/internal/hook"
	"go.uber.org/zap"
)

// BriefingHookHandler is a hook.HookHandler that triggers the briefing
// orchestrator when the EvMorningBriefingTime event fires from the
// SCHEDULER-001 cron engine.
//
// The handler is best-effort: pipeline failures are reported through the
// onDone callback (when set) and logged at WARN level, but never propagate
// to the dispatcher (the scheduler must not retry briefing on transient
// failures of an individual collector).
//
// REQ-BR-010 / AC-011.
type BriefingHookHandler struct {
	orch   *Orchestrator
	userID string
	onDone func(payload *BriefingPayload, err error)
	logger *zap.Logger
}

// NewBriefingHookHandler creates a handler wired to the supplied orchestrator.
// onDone is an optional callback invoked with the run result; useful for
// rendering fan-out (CLI / Telegram / TUI) at the SCHEDULER fire site.
func NewBriefingHookHandler(orch *Orchestrator, userID string, onDone func(*BriefingPayload, error), logger *zap.Logger) *BriefingHookHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BriefingHookHandler{
		orch:   orch,
		userID: userID,
		onDone: onDone,
		logger: logger,
	}
}

// Handle implements hook.HookHandler. It runs the orchestrator pipeline and
// forwards the result to onDone (if set). The returned HookJSONOutput is
// always benign (Continue defaulted) so that other registered handlers for
// EvMorningBriefingTime still execute (REQ-HK-002 FIFO).
func (h *BriefingHookHandler) Handle(ctx context.Context, _ hook.HookInput) (hook.HookJSONOutput, error) {
	if h.orch == nil {
		return hook.HookJSONOutput{}, fmt.Errorf("briefing cron: nil orchestrator")
	}
	payload, err := h.orch.Run(ctx, h.userID, time.Now())
	if h.onDone != nil {
		h.onDone(payload, err)
	}
	if err != nil {
		h.logger.Warn("briefing pipeline error",
			zap.String("event", string(hook.EvMorningBriefingTime)),
			zap.String("error_type", "pipeline"),
		)
	}
	return hook.HookJSONOutput{}, nil
}

// Matches implements hook.HookHandler. The briefing handler always matches
// for its registered event (matcher "*" semantics).
func (h *BriefingHookHandler) Matches(_ hook.HookInput) bool {
	return true
}

// RegisterMorningBriefing registers the briefing handler with the supplied
// hook registry under EvMorningBriefingTime + matcher "*".
//
// AC-011: SCHEDULER-001 cron registration test asserts the registry returns
// a *BriefingHookHandler for EvMorningBriefingTime after this call.
//
// @MX:ANCHOR: RegisterMorningBriefing is the single wiring path between
// SCHEDULER-001 morning cron and the briefing orchestrator.
// @MX:REASON: SPEC-MINK-BRIEFING-001 REQ-BR-010; consolidates registration
// so that bootstrap callers do not duplicate the event-name string.
func RegisterMorningBriefing(reg *hook.HookRegistry, h *BriefingHookHandler) error {
	if reg == nil {
		return fmt.Errorf("briefing cron: nil registry")
	}
	if h == nil {
		return fmt.Errorf("briefing cron: nil handler")
	}
	return reg.Register(hook.EvMorningBriefingTime, "*", h)
}
