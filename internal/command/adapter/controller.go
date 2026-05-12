package adapter

import (
	"context"

	"github.com/modu-ai/mink/internal/command"
)

// LoopController is the abstraction the adapter uses to communicate with the
// query loop without violating the loop's single-owner invariant
// (SPEC-GOOSE-QUERY-001 REQ-QUERY-015).
//
// All side-effecting slash commands route through this interface rather than
// mutating loop.State directly, preserving the loop's single-goroutine ownership.
//
// @MX:ANCHOR: [AUTO] Boundary between command adapter and query loop.
// @MX:REASON: All side-effecting slash commands route through this interface; fan_in >= 7 (6 adapter methods + Snapshot).
// @MX:SPEC: SPEC-GOOSE-CMDCTX-001 REQ-CMDCTX-006/007/008/016
type LoopController interface {
	// RequestClear signals the loop to reset Messages and TurnCount on the
	// next iteration. Fire-and-forget; returns error only if the request
	// could not be enqueued.
	RequestClear(ctx context.Context) error

	// RequestReactiveCompact signals the loop to set
	// AutoCompactTracking.ReactiveTriggered = true on the next iteration.
	// target == 0 means "use compactor default".
	RequestReactiveCompact(ctx context.Context, target int) error

	// RequestModelChange signals the loop to swap the active model on the
	// next submitMessage. The adapter does not validate info; ResolveModelAlias
	// does that upstream.
	RequestModelChange(ctx context.Context, info command.ModelInfo) error

	// Snapshot returns a deep-enough copy of the loop state for read-only
	// inspection by /status. Called from any goroutine.
	Snapshot() LoopSnapshot
}

// LoopSnapshot is the read-only view of loop state surfaced to the adapter.
// The adapter never mutates loop.State directly (REQ-CMDCTX-016).
type LoopSnapshot struct {
	// TurnCount is the number of completed turns.
	TurnCount int
	// Model is the active model identifier.
	Model string
	// TokenCount is the current accumulated token count.
	TokenCount int64
	// TokenLimit is the maximum token count for the context window.
	TokenLimit int64
}
