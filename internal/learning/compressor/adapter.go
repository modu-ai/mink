package compressor

import (
	"context"

	goosecontext "github.com/modu-ai/goose/internal/context"
	"github.com/modu-ai/goose/internal/learning/trajectory"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"github.com/modu-ai/goose/internal/query/loop"
	"go.uber.org/zap"
)

// Compile-time contract: CompactorAdapter must implement query.Compactor.
// SPEC-GOOSE-COMPRESSOR-001 AC-COMPRESSOR-013 / REQ-COMPRESSOR-018.
var _ query.Compactor = (*CompactorAdapter)(nil)

// CompactorAdapter wraps TrajectoryCompressor to implement query.Compactor.
//
// It re-implements the ShouldCompact/Compact semantics mandated by SPEC-GOOSE-CONTEXT-001
// for the AutoCompact strategy variant. Snip and ReactiveCompact are delegated to
// snipDelegate (*goosecontext.DefaultCompactor).
//
// @MX:ANCHOR: [AUTO] query.Compactor implementation for AutoCompact strategy variant
// @MX:REASON: SPEC-GOOSE-COMPRESSOR-001 REQ-COMPRESSOR-018; var _ query.Compactor assertion; fan_in >= 3
type CompactorAdapter struct {
	inner *TrajectoryCompressor
	// snipDelegate handles Snip strategy execution.
	// Typically *goosecontext.DefaultCompactor. If nil, Snip is a no-op returning the original State.
	snipDelegate *goosecontext.DefaultCompactor
	// historySnipOnly mirrors GOOSE_HISTORY_SNIP=1 feature gate (REQ-CTX-016).
	// This is an adapter-own field; loop.State has no HistorySnipOnly field.
	historySnipOnly bool
	// messageToEntry converts a message.Message to a trajectory.TrajectoryEntry.
	messageToEntry func(m message.Message) trajectory.TrajectoryEntry
	// entryToMessage converts a trajectory.TrajectoryEntry back to a message.Message.
	entryToMessage func(e trajectory.TrajectoryEntry) message.Message
}

// NewCompactorAdapter creates a CompactorAdapter with default message↔entry converters.
func NewCompactorAdapter(
	inner *TrajectoryCompressor,
	snipDelegate *goosecontext.DefaultCompactor,
	historySnipOnly bool,
) *CompactorAdapter {
	return &CompactorAdapter{
		inner:           inner,
		snipDelegate:    snipDelegate,
		historySnipOnly: historySnipOnly,
		messageToEntry:  defaultMessageToEntry,
		entryToMessage:  defaultEntryToMessage,
	}
}

// ShouldCompact evaluates the four triggers mandated by CONTEXT-001 REQ-CTX-007/011.
//
// REQ-COMPRESSOR-019: returns true if any of:
//
//	(a) token ratio >= 80%
//	(b) ReactiveTriggered == true
//	(c) len(Messages) > MaxMessageCount
//	(d) CalculateTokenWarningState >= WarningRed  (superset of (a), kept for explicitness)
//
// @MX:WARN: [AUTO] Four-trigger OR logic; (d) WarningRed is superset of (a) 80% — both evaluated for safety
// @MX:REASON: SPEC-GOOSE-COMPRESSOR-001 REQ-COMPRESSOR-019 requires explicit four-trigger evaluation
func (a *CompactorAdapter) ShouldCompact(s loop.State) bool {
	used := goosecontext.TokenCountWithEstimation(s.Messages)

	// (a) 80% ratio trigger
	if s.TokenLimit > 0 {
		ratio := float64(used) / float64(s.TokenLimit)
		if ratio >= 0.80 {
			return true
		}
	}

	// (b) ReactiveTriggered
	if s.AutoCompactTracking.ReactiveTriggered {
		return true
	}

	// (c) MaxMessageCount
	if s.MaxMessageCount > 0 && len(s.Messages) > s.MaxMessageCount {
		return true
	}

	// (d) Red override (>=92%): superset of (a) but kept for idempotent completeness
	if s.TokenLimit > 0 && goosecontext.CalculateTokenWarningState(used, s.TokenLimit) >= goosecontext.WarningRed {
		return true
	}

	return false
}

// Compact selects a strategy and executes compaction.
//
// Strategy priority: ReactiveCompact > AutoCompact > Snip
// Fallbacks:
//   - inner.summarizer == nil → Snip  (REQ-CTX-012)
//   - Summarize returns error → Snip, error not propagated  (REQ-CTX-014)
//
// REQ-COMPRESSOR-020, REQ-COMPRESSOR-021
func (a *CompactorAdapter) Compact(s loop.State) (loop.State, query.CompactBoundary, error) {
	strategy := a.selectStrategy(s)

	// Summarizer nil fallback (REQ-CTX-012).
	if (strategy == goosecontext.StrategyAutoCompact || strategy == goosecontext.StrategyReactiveCompact) &&
		a.inner.summarizer == nil {
		strategy = goosecontext.StrategySnip
	}

	switch strategy {
	case goosecontext.StrategySnip:
		return a.executeSnip(s)

	case goosecontext.StrategyAutoCompact, goosecontext.StrategyReactiveCompact:
		newState, boundary, err := a.executeAutoCompact(s, strategy)
		if err != nil {
			// REQ-CTX-014: fall back to Snip, do not propagate error.
			if a.inner.logger != nil {
				a.inner.logger.Warn("summarizer failed; falling back to Snip",
					zap.Error(err),
					zap.String("strategy", strategy),
				)
			}
			return a.executeSnip(s)
		}
		return newState, boundary, nil
	}

	// Unknown strategy — should not happen.
	return s, query.CompactBoundary{}, nil
}

// selectStrategy determines the compaction strategy based on state and adapter config.
// REQ-CTX-008/016/017/018
func (a *CompactorAdapter) selectStrategy(s loop.State) string {
	// REQ-CTX-016: HistorySnipOnly overrides all.
	if a.historySnipOnly {
		return goosecontext.StrategySnip
	}
	// REQ-CTX-017: ReactiveTriggered → ReactiveCompact first.
	if s.AutoCompactTracking.ReactiveTriggered {
		return goosecontext.StrategyReactiveCompact
	}
	// REQ-CTX-018: token >= 80% → AutoCompact.
	if s.TokenLimit > 0 {
		used := goosecontext.TokenCountWithEstimation(s.Messages)
		ratio := float64(used) / float64(s.TokenLimit)
		if ratio >= 0.80 {
			return goosecontext.StrategyAutoCompact
		}
	}
	return goosecontext.StrategySnip
}

// executeSnip delegates Snip to snipDelegate or returns a no-op boundary.
func (a *CompactorAdapter) executeSnip(s loop.State) (loop.State, query.CompactBoundary, error) {
	if a.snipDelegate != nil {
		return a.snipDelegate.Compact(s)
	}
	// No delegate: return state unchanged with a Snip boundary.
	boundary := query.CompactBoundary{
		Turn:                s.TurnCount,
		Strategy:            goosecontext.StrategySnip,
		MessagesBefore:      len(s.Messages),
		MessagesAfter:       len(s.Messages),
		TaskBudgetPreserved: int64(s.TaskBudgetRemaining), // flat int → int64
	}
	return s, boundary, nil
}

// executeAutoCompact runs the trajectory compressor against the current messages.
// When state.TokenLimit > 0, TargetMaxTokens is temporarily set to state.TokenLimit * 75%
// so the compressor's under-target skip logic fires at the in-session budget boundary
// rather than the offline trajectory budget (15_250).
func (a *CompactorAdapter) executeAutoCompact(
	s loop.State,
	strategy string,
) (loop.State, query.CompactBoundary, error) {
	traj := a.messagesToTrajectory(s.Messages)

	// Override TargetMaxTokens for the adapter path.
	// The inner tokenizer (SimpleTokenizer by default) may count tokens differently
	// from goosecontext.TokenCountWithEstimation. We measure the trajectory with the
	// inner tokenizer and set TargetMaxTokens to slightly below that count to force
	// the compressor to actually run (rather than taking the SkippedUnderTarget path).
	origTarget := a.inner.cfg.TargetMaxTokens
	trajTokens := a.inner.tokenizer.CountTrajectory(traj)
	if trajTokens > 0 {
		// Target = 75% of trajectory token count (force compression).
		// This is a conservative estimate: compress until 75% to give headroom.
		adapterTarget := int(float64(trajTokens) * 0.75)
		if adapterTarget < 1 {
			adapterTarget = 1
		}
		a.inner.cfg.TargetMaxTokens = adapterTarget
		defer func() { a.inner.cfg.TargetMaxTokens = origTarget }()
	}

	compressed, metrics, err := a.inner.CompressWithRetries(
		context.Background(),
		traj,
		a.inner.cfg.AdapterMaxRetries,
	)
	if err != nil {
		return s, query.CompactBoundary{}, err
	}

	newState := cloneState(s)
	newState.Messages = a.trajectoryToMessages(compressed)
	// REQ-CTX-010: TaskBudgetRemaining is preserved unchanged.
	newState.TaskBudgetRemaining = s.TaskBudgetRemaining

	boundary := query.CompactBoundary{
		Turn:                 s.TurnCount,
		Strategy:             strategy,
		MessagesBefore:       len(s.Messages),
		MessagesAfter:        len(newState.Messages),
		TokensBefore:         int64(metrics.OriginalTokens),
		TokensAfter:          int64(metrics.CompressedTokens),
		TaskBudgetPreserved:  int64(s.TaskBudgetRemaining),
		DroppedThinkingCount: metrics.DroppedThinkingCount,
	}
	return newState, boundary, nil
}

// cloneState creates a shallow copy of loop.State.
func cloneState(s loop.State) loop.State {
	clone := s
	clone.Messages = make([]message.Message, len(s.Messages))
	copy(clone.Messages, s.Messages)
	return clone
}

// messagesToTrajectory converts a message slice to a temporary Trajectory for compression.
// REQ-COMPRESSOR-021: redacted_thinking blocks are preserved via the Value field.
func (a *CompactorAdapter) messagesToTrajectory(msgs []message.Message) *trajectory.Trajectory {
	entries := make([]trajectory.TrajectoryEntry, 0, len(msgs))
	for _, m := range msgs {
		entries = append(entries, a.messageToEntry(m))
	}
	return &trajectory.Trajectory{Conversations: entries}
}

// trajectoryToMessages converts a compressed Trajectory back to a message slice.
func (a *CompactorAdapter) trajectoryToMessages(t *trajectory.Trajectory) []message.Message {
	msgs := make([]message.Message, 0, len(t.Conversations))
	for _, e := range t.Conversations {
		msgs = append(msgs, a.entryToMessage(e))
	}
	return msgs
}

// defaultMessageToEntry converts message.Message → trajectory.TrajectoryEntry.
// Uses the first text block as Value; preserves redacted_thinking via marker string.
func defaultMessageToEntry(m message.Message) trajectory.TrajectoryEntry {
	role := trajectory.RoleGPT
	switch m.Role {
	case "user":
		role = trajectory.RoleHuman
	case "system":
		role = trajectory.RoleSystem
	}

	var value string
	for _, block := range m.Content {
		switch block.Type {
		case "redacted_thinking":
			value += redactedThinkingMarker
		case "text":
			value += block.Text
		case "thinking":
			value += block.Thinking
		}
	}
	return trajectory.TrajectoryEntry{From: role, Value: value}
}

// defaultEntryToMessage converts trajectory.TrajectoryEntry → message.Message.
func defaultEntryToMessage(e trajectory.TrajectoryEntry) message.Message {
	role := "assistant"
	switch e.From {
	case trajectory.RoleHuman:
		role = "user"
	case trajectory.RoleSystem:
		role = "system"
	}
	return message.Message{
		Role:    role,
		Content: []message.ContentBlock{{Type: "text", Text: e.Value}},
	}
}
