// Package cmdctrl provides the LoopController implementation that wires the
// command adapter to the query loop while preserving the loop's single-owner
// invariant (REQ-QUERY-015).
//
// SPEC: SPEC-GOOSE-CMDLOOP-WIRE-001
package cmdctrl

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/command/adapter"
	"github.com/modu-ai/goose/internal/query/loop"
)

// Sentinel errors
var (
	// ErrEngineUnavailable is returned when the query engine reference is nil.
	// AC-CMDLOOP-013: Graceful degradation when engine is unavailable.
	ErrEngineUnavailable = errors.New("query engine is unavailable")

	// ErrInvalidModelInfo is returned when RequestModelChange receives a
	// zero-value ModelInfo with an empty ID field.
	// AC-CMDLOOP-011: Reject invalid model info.
	ErrInvalidModelInfo = errors.New("invalid model info: ID must be non-empty")
)

// LoopControllerImpl implements adapter.LoopController using lock-free atomic
// operations to enqueue requests from the command dispatcher without violating
// the query loop's single-owner invariant (REQ-QUERY-015).
//
// @MX:ANCHOR: [AUTO] Command adapter to query loop bridge - single-owner invariant enforcement.
// @MX:REASON: All slash command side effects route through this controller; fan_in >= 4 (adapter methods + Snapshot).
// @MX:SPEC: SPEC-GOOSE-CMDLOOP-WIRE-001 REQ-CMDLOOP-001~020
type LoopControllerImpl struct {
	// pendingClear signals RequestClear was called and should be applied
	// on the next loop iteration. Uses atomic.Bool for lock-free coordination.
	pendingClear atomic.Bool

	// pendingCompact signals RequestReactiveCompact was called and should be
	// applied on the next loop iteration. Uses atomic.Bool for lock-free coordination.
	pendingCompact atomic.Bool

	// activeModel holds the current model identifier. Uses atomic.Pointer for
	// lock-free reads and writes. Swapped immediately on RequestModelChange.
	// AC-CMDLOOP-003/004/017: Atomic swap with last-write-wins semantics.
	activeModel atomic.Pointer[command.ModelInfo]

	// engine holds a reference to the query engine for Snapshot support.
	// May be nil - graceful degradation via ErrEngineUnavailable.
	engine interface{} // TODO: Will be *query.QueryEngine once we define the interface

	// logger is an optional structured logger for debugging enqueue/apply events.
	// nil means silent operation (AC-CMDLOOP-012).
	logger interface{} // TODO: Will be *slog.Logger or similar
}

// New creates a new LoopControllerImpl instance.
//
// AC-CMDLOOP-013: Nil engine is allowed - Snapshot returns zero-value.
// AC-CMDLOOP-012: Nil logger is allowed - silent operation.
//
// @MX:ANCHOR: [AUTO] Constructor for LoopController implementation.
// @MX:REASON: Public factory function; will be called by CLI/DAEMON wiring specs.
func New(engine, logger interface{}) *LoopControllerImpl {
	return &LoopControllerImpl{
		engine: engine,
		logger: logger,
	}
}

// RequestClear signals the loop to reset Messages and TurnCount on the next
// iteration. Fire-and-forget; returns error only if ctx is already cancelled.
//
// AC-CMDLOOP-001: Enqueues request for next iteration.
// AC-CMDLOOP-007: Returns ctx.Err() if context is cancelled.
// AC-CMDLOOP-008: Holds request across idle-to-active transitions.
// AC-CMDLOOP-010: Multiple calls coalesce into single application.
//
// @MX:NOTE: [AUTO] Enqueues clear request via atomic.Bool.Store - O(1) operation.
// @MX:SPEC: SPEC-GOOSE-CMDLOOP-WIRE-001 REQ-CMDLOOP-008
func (c *LoopControllerImpl) RequestClear(ctx context.Context) error {
	// Handle nil receiver gracefully (AC-CMDLOOP-018)
	if c == nil {
		return nil
	}

	// Treat nil ctx as Background (AC-CMDLOOP-018)
	if ctx == nil {
		ctx = context.Background()
	}

	// Check if context is already cancelled (AC-CMDLOOP-007)
	if err := ctx.Err(); err != nil {
		return err
	}

	// Set the pending flag - lock-free O(1) operation (AC-CMDLOOP-004)
	c.pendingClear.Store(true)

	// TODO: Add logging when logger is implemented (AC-CMDLOOP-012)

	return nil
}

// RequestReactiveCompact signals the loop to set AutoCompactTracking.ReactiveTriggered
// on the next iteration. The target parameter is ignored in this SPEC (Exclusions §10 #2).
//
// AC-CMDLOOP-002: Enqueues request for next iteration.
// AC-CMDLOOP-007: Returns ctx.Err() if context is cancelled.
// AC-CMDLOOP-009: Holds request across idle-to-active transitions.
// AC-CMDLOOP-010: Multiple calls coalesce into single application.
//
// @MX:NOTE: [AUTO] Enqueues compact request via atomic.Bool.Store - O(1) operation.
// @MX:SPEC: SPEC-GOOSE-CMDLOOP-WIRE-001 REQ-CMDLOOP-009
func (c *LoopControllerImpl) RequestReactiveCompact(ctx context.Context, target int) error {
	// Handle nil receiver gracefully (AC-CMDLOOP-018)
	if c == nil {
		return nil
	}

	// Treat nil ctx as Background (AC-CMDLOOP-018)
	if ctx == nil {
		ctx = context.Background()
	}

	// Check if context is already cancelled (AC-CMDLOOP-007)
	if err := ctx.Err(); err != nil {
		return err
	}

	// Set the pending flag - lock-free O(1) operation (AC-CMDLOOP-004)
	// Note: target parameter is ignored per SPEC Exclusions §10 #2
	c.pendingCompact.Store(true)

	// TODO: Add logging when logger is implemented (AC-CMDLOOP-012)

	return nil
}

// RequestModelChange atomically swaps the active model identifier.
// Returns immediately with the new model stored; next SubmitMessage will use it.
//
// AC-CMDLOOP-003: Atomic swap with immediate visibility.
// AC-CMDLOOP-004: Last-write-wins semantics.
// AC-CMDLOOP-011: Rejects zero-value ModelInfo with empty ID.
// AC-CMDLOOP-017: Changes visible immediately after return.
//
// @MX:NOTE: [AUTO] Swaps active model via atomic.Pointer.Store - O(1) operation.
// @MX:SPEC: SPEC-GOOSE-CMDLOOP-WIRE-001 REQ-CMDLOOP-010
func (c *LoopControllerImpl) RequestModelChange(ctx context.Context, info command.ModelInfo) error {
	// Handle nil receiver gracefully (AC-CMDLOOP-018)
	if c == nil {
		return nil
	}

	// Treat nil ctx as Background (AC-CMDLOOP-018)
	if ctx == nil {
		ctx = context.Background()
	}

	// Check if context is already cancelled (AC-CMDLOOP-007)
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate ModelInfo - reject zero-value (AC-CMDLOOP-011)
	if info.ID == "" {
		return ErrInvalidModelInfo
	}

	// Atomic swap - immediate visibility (AC-CMDLOOP-003/017)
	c.activeModel.Store(&info)

	// TODO: Add logging when logger is implemented (AC-CMDLOOP-012)

	return nil
}

// Snapshot returns a read-only copy of the loop state for /status command.
// Returns synchronously without blocking on in-flight SubmitMessage operations.
//
// AC-CMDLOOP-005: Synchronous return within 100ms.
// AC-CMDLOOP-013: Returns zero-value when engine is nil.
// AC-CMDLOOP-019: TokenCount is always 0 (delegated to future SPEC).
//
// @MX:NOTE: [AUTO] Reads engine state under RLock - lock-free read path.
// @MX:SPEC: SPEC-GOOSE-CMDLOOP-WIRE-001 REQ-CMDLOOP-011
func (c *LoopControllerImpl) Snapshot() adapter.LoopSnapshot {
	// Handle nil receiver gracefully (AC-CMDLOOP-018)
	if c == nil {
		return adapter.LoopSnapshot{}
	}

	// Graceful degradation for nil engine (AC-CMDLOOP-013)
	if c.engine == nil {
		return adapter.LoopSnapshot{}
	}

	// TODO: Implement actual snapshot reading from engine
	// For now, return zero-value as we need engine.SnapshotState() method
	//
	// Per SPEC §6.3 algorithm:
	// 1. state := c.engine.SnapshotState() // New method needed
	// 2. modelID from activeModel.Load() or default
	// 3. Return LoopSnapshot with TokenCount=0

	return adapter.LoopSnapshot{
		TurnCount:  0,
		Model:      "",
		TokenCount: 0, // Always 0 per SPEC Exclusions §10 #4
		TokenLimit: 0,
	}
}

// applyPendingRequests drains the pending request flags and applies them to
// the loop state. Called by the PreIteration hook on each loop iteration.
//
// This is called from within the loop goroutine, so it's the only place where
// state mutation happens (preserving REQ-QUERY-015 single-owner invariant).
//
// This method is exported for use by LoopConfig.PreIteration, but is not
// part of the adapter.LoopController interface.
//
// AC-CMDLOOP-001: Applies Messages=nil, TurnCount=0 when pendingClear is set.
// AC-CMDLOOP-002: Sets ReactiveTriggered=true when pendingCompact is set.
// AC-CMDLOOP-016: Explicit whitelist for state mutation (PreIteration callback).
//
// @MX:NOTE: [AUTO] Drains pending flags via atomic.Bool.Swap - resets to false.
// @MX:SPEC: SPEC-GOOSE-CMDLOOP-WIRE-001 REQ-CMDLOOP-008/009
// @MX:WHITELIST: AC-CMDLOOP-016 - state mutation allowed only in PreIteration callback
func (c *LoopControllerImpl) ApplyPendingRequests(state *loop.State) {
	if c == nil {
		return
	}

	// Check and clear pendingClear flag
	// Swap(false) returns the previous value and sets it to false atomically
	if c.pendingClear.Swap(false) {
		// Apply clear operation: reset Messages and TurnCount
		// AC-CMDLOOP-001
		// AC-CMDLOOP-016-WHITELIST: This is the only place where we mutate state.Messages
		// AC-CMDLOOP-016-WHITELIST: This is the only place where we mutate state.TurnCount
		state.Messages = nil
		state.TurnCount = 0

		// TODO: Add logging when logger is implemented (AC-CMDLOOP-012)
	}

	// Check and clear pendingCompact flag
	if c.pendingCompact.Swap(false) {
		// Apply reactive compact trigger
		// AC-CMDLOOP-002
		// AC-CMDLOOP-016-WHITELIST: This is the only place where we mutate AutoCompactTracking
		state.AutoCompactTracking.ReactiveTriggered = true

		// TODO: Add logging when logger is implemented (AC-CMDLOOP-012)
	}
}
