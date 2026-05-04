// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-007
// AC: AC-BR-007
// M3-T1, M3-T2 — outbound chunk dispatcher + per-session monotonic sequence.

package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// ErrSessionUnknown is returned when SendOutbound or RequestPermission
// targets a session ID that has no registered SessionSender.
var ErrSessionUnknown = errors.New("bridge: session unknown or sender not registered")

// SessionSender is the transport-side hook invoked once per outbound message.
// Implementations (wsSender, sseSender) own the wire serialization for their
// transport. Sender is responsible for any internal buffering / framing.
//
// @MX:ANCHOR
// @MX:REASON Outbound emission contract; every transport (WS, SSE, future
// gRPC) implements this so the dispatcher stays transport-agnostic.
type SessionSender interface {
	SendOutbound(msg OutboundMessage) error
}

// outboundDispatcher assigns monotonic per-session sequences and delegates
// to the registered SessionSender for actual wire emission. M4 attached
// the per-session ring buffer (replay safety) and the flush-gate
// (backpressure); M5 added the OTel metrics surface — all nil-safe so
// earlier tests that construct the dispatcher without buffering or
// metrics still pass.
type outboundDispatcher struct {
	registry  *Registry
	buffer    *outboundBuffer // M4-T1: replay buffer; nil disables buffering
	gate      *flushGate      // M4-T4: backpressure gate; nil disables
	metrics   *bridgeMetrics  // M5-T2: OTel counters; nil disables metrics
	mu        sync.Mutex
	sequences map[string]*atomic.Uint64
}

func newOutboundDispatcher(reg *Registry, buf *outboundBuffer, gate *flushGate, metrics *bridgeMetrics) *outboundDispatcher {
	return &outboundDispatcher{
		registry:  reg,
		buffer:    buf,
		gate:      gate,
		metrics:   metrics,
		sequences: make(map[string]*atomic.Uint64),
	}
}

// nextSequence returns the next sequence number for the given session,
// allocating a counter on first use. Sequences start at 1 and increase
// monotonically; replay correctness depends on this invariant.
func (d *outboundDispatcher) nextSequence(sessionID string) uint64 {
	d.mu.Lock()
	c, ok := d.sequences[sessionID]
	if !ok {
		c = &atomic.Uint64{}
		d.sequences[sessionID] = c
	}
	d.mu.Unlock()
	return c.Add(1)
}

// dropSequence releases the sequence counter for a session that has been
// removed from the registry. Failing to call this on session shutdown leaks
// counter memory across many connection cycles. Also clears any per-session
// buffer + flush-gate state so memory does not accumulate.
func (d *outboundDispatcher) dropSequence(sessionID string) {
	d.mu.Lock()
	delete(d.sequences, sessionID)
	d.mu.Unlock()
	if d.buffer != nil {
		d.buffer.Drop(sessionID)
	}
	if d.gate != nil {
		d.gate.Drop(sessionID)
	}
}

// SendOutbound assigns the next sequence number and dispatches the message
// through the session's registered SessionSender.
//
// Behavior matrix (M4 + M5 follow-up):
//   - No session AND no sender → ErrSessionUnknown (M3 contract).
//   - Session present, sender absent (reconnecting tab) → buffer for
//     replay, return seq, no error.
//   - Sender present, gate stalled → buffer for replay, skip emit
//     (backpressure per REQ-BR-010), return seq, no error.
//   - Sender present, gate idle → buffer for replay, emit through
//     sender (transport owns ObserveWrite/ObserveDrain bracket so the
//     gate measures actual wire-write boundaries, not dispatch-call
//     boundaries — M5 follow-up transport wire-in).
func (d *outboundDispatcher) SendOutbound(sessionID string, t OutboundType, payload []byte) (uint64, error) {
	sender := d.registry.Sender(sessionID)
	_, sessionExists := d.registry.Get(sessionID)
	if sender == nil && !sessionExists {
		return 0, fmt.Errorf("%w: id=%s", ErrSessionUnknown, sessionID)
	}

	seq := d.nextSequence(sessionID)
	msg := OutboundMessage{
		SessionID: sessionID,
		Type:      t,
		Payload:   payload,
		Sequence:  seq,
	}

	// Always buffer when buffering is enabled; resume correctness depends
	// on every sequenced message being recoverable until TTL or eviction.
	if d.buffer != nil {
		d.buffer.Append(msg)
	}

	// No live transport → message is buffered for resume; not an error.
	if sender == nil {
		return seq, nil
	}

	// Backpressure: while stalled, buffer-only path; producer keeps
	// generating sequences but the wire is paused (REQ-BR-010). The
	// stall is detected here at the dispatcher level so the producer
	// path short-circuits before paying the sender call cost; the
	// gate's *transitions* (ObserveWrite/ObserveDrain) live inside
	// the transport sender now (M5 follow-up).
	if d.gate != nil && d.gate.Stalled(sessionID) {
		return seq, nil
	}

	if err := sender.SendOutbound(msg); err != nil {
		return seq, fmt.Errorf("bridge: outbound emit failed (seq=%d): %w", seq, err)
	}
	if d.metrics != nil {
		d.metrics.RecordOutbound(context.Background(), 1)
	}
	return seq, nil
}

// outboundEnvelope is the on-the-wire shape produced by every transport.
// Sequence is included so the client can detect gaps and resume.
type outboundEnvelope struct {
	Type     string          `json:"type"`
	Sequence uint64          `json:"sequence"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// encodeOutboundJSON renders an OutboundMessage to the canonical JSON
// envelope used by both the WebSocket text frame and the SSE event data
// field.
func encodeOutboundJSON(msg OutboundMessage) ([]byte, error) {
	env := outboundEnvelope{
		Type:     string(msg.Type),
		Sequence: msg.Sequence,
		Payload:  json.RawMessage(msg.Payload),
	}
	if len(msg.Payload) == 0 {
		env.Payload = nil
	}
	return json.Marshal(env)
}
