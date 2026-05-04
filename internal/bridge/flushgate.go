// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-010
// AC: AC-BR-010
// M4-T4, M4-T5 — flush-gate watermark backpressure.
//
// Semantics (spec.md §5.3 REQ-BR-010):
//   - Per-session counters track outstanding outbound bytes and frames in
//     the transport's write queue.
//   - When bytes >= HighWatermarkBytes OR frames >= HighWatermarkFrames,
//     the gate transitions to stalled. Stalled() returns true and Wait()
//     blocks subsequent emit attempts.
//   - The gate transitions back to draining once both bytes < LowWatermarkBytes
//     AND frames < LowWatermarkFrames. Wait() callers unblock at that point.
//   - The drain transition is observed by the Go write loop via
//     ObserveDrain; clients do not send acks (REQ-BR-010 normative wording).
//
// Concurrency: gate transitions are gated by a closeable channel per
// session. Closing on drain wakes every parked Wait() caller atomically;
// re-stalling allocates a fresh channel so future waits suspend.

package bridge

import (
	"context"
	"sync"
	"sync/atomic"
)

// Watermark defaults from spec.md §5.3 REQ-BR-010.
const (
	HighWatermarkBytes  = 256 * 1024
	LowWatermarkBytes   = 64 * 1024
	HighWatermarkFrames = 64
	LowWatermarkFrames  = 16
)

// flushGateState holds per-session backpressure counters and the channel
// that signals "drained below low watermark".
type flushGateState struct {
	bytes   int
	frames  int
	stalled bool
	// drained is closed when the gate is NOT stalled and replaced with a
	// fresh open channel each time a stall transition occurs. Wait()
	// blocks on this channel; ObserveDrain closes it when transitioning
	// out of stalled.
	drained chan struct{}
}

func newFlushGateState() *flushGateState {
	ch := make(chan struct{})
	close(ch) // not stalled at construction
	return &flushGateState{drained: ch}
}

// flushGate is the FlushGate implementation used by bridgeServer. Exposes
// the stall counter via Stalls() for the M5 OTel metrics wire-up.
//
// @MX:ANCHOR
// @MX:REASON Backpressure invariant — every transport write must traverse
// Stalled()/Wait() before queueing. Skipping the gate breaks REQ-BR-010.
type flushGate struct {
	mu     sync.Mutex
	states map[string]*flushGateState
	stalls atomic.Uint64 // total stall transitions for metrics
}

func newFlushGate() *flushGate {
	return &flushGate{
		states: make(map[string]*flushGateState),
	}
}

func (g *flushGate) stateLocked(sessionID string) *flushGateState {
	st, ok := g.states[sessionID]
	if !ok {
		st = newFlushGateState()
		g.states[sessionID] = st
	}
	return st
}

// Stalled reports whether the named session has crossed the high watermark
// and not yet drained back below the low watermark.
func (g *flushGate) Stalled(sessionID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if st, ok := g.states[sessionID]; ok {
		return st.stalled
	}
	return false
}

// Wait blocks until the session is no longer stalled, ctx is cancelled, or
// the deadline fires. Returns ctx.Err() on cancellation; nil otherwise. A
// session that is already draining returns immediately.
func (g *flushGate) Wait(ctx context.Context, sessionID string) error {
	for {
		g.mu.Lock()
		st := g.stateLocked(sessionID)
		if !st.stalled {
			g.mu.Unlock()
			return nil
		}
		drained := st.drained
		g.mu.Unlock()

		select {
		case <-drained:
			// Re-check under lock; spurious wakeups are harmless because
			// the loop re-evaluates st.stalled.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ObserveWrite records that delta bytes (and one frame) entered the
// transport write queue. May trigger a stall transition.
func (g *flushGate) ObserveWrite(sessionID string, bytes int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	st := g.stateLocked(sessionID)
	st.bytes += bytes
	st.frames++

	if !st.stalled && (st.bytes >= HighWatermarkBytes || st.frames >= HighWatermarkFrames) {
		st.stalled = true
		st.drained = make(chan struct{})
		g.stalls.Add(1)
	}
}

// ObserveDrain records that delta bytes (and one frame) departed the
// transport write queue. May trigger a drain transition that releases all
// parked Wait() callers.
func (g *flushGate) ObserveDrain(sessionID string, bytes int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	st := g.stateLocked(sessionID)
	st.bytes -= bytes
	if st.bytes < 0 {
		st.bytes = 0
	}
	st.frames--
	if st.frames < 0 {
		st.frames = 0
	}

	if st.stalled && st.bytes < LowWatermarkBytes && st.frames < LowWatermarkFrames {
		st.stalled = false
		close(st.drained)
	}
}

// Stalls returns the number of high-watermark transitions observed since
// process start. Used by M5 OTel exporter (bridge.flush_gate.stalls).
func (g *flushGate) Stalls() uint64 {
	return g.stalls.Load()
}

// Drop releases per-session state. Called on session close.
func (g *flushGate) Drop(sessionID string) {
	g.mu.Lock()
	if st, ok := g.states[sessionID]; ok {
		// Wake any parked waiters; their next iteration sees no state and returns.
		if st.stalled {
			close(st.drained)
		}
		delete(g.states, sessionID)
	}
	g.mu.Unlock()
}

// Compile-time interface compliance.
var _ FlushGate = (*flushGate)(nil)
