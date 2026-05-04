// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-007
// AC: AC-BR-007
// M3-T1, M3-T2 — outbound chunk dispatcher + per-session monotonic sequence.
// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-003, REQ-BR-AMEND-006, REQ-BR-AMEND-007
// AC: AC-BR-AMEND-003, AC-BR-AMEND-007, AC-BR-AMEND-008
// M3-T2, M3-T3, M3-T5 — buffer + sequence keying by LogicalID, logout hook.

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

// bufferKey returns the key used for buffer Append and sequence tracking.
// When the registry is present and has a non-empty LogicalID for sessionID,
// returns that LogicalID (primary path — REQ-BR-AMEND-003). Falls back to
// sessionID for registry-less dispatcher instances (unit test fixtures) and
// sessions that have not been assigned a LogicalID yet (pre-amendment
// compatibility path).
//
// @MX:NOTE: [AUTO] bufferKey is the single lookup point for the connID →
// LogicalID mapping inside the dispatcher. Both SendOutbound and
// nextSequenceByKey call this helper so the registry lookup happens exactly
// once per emit.
func (d *outboundDispatcher) bufferKey(sessionID string) string {
	if d.registry == nil {
		return sessionID
	}
	if lid, ok := d.registry.LogicalID(sessionID); ok {
		return lid
	}
	return sessionID
}

// nextSequenceByKey returns the next monotonic sequence number for the
// given key (LogicalID or fallback connID), allocating a counter on first
// use. Sequences start at 1 and are shared by all connIDs that map to the
// same key, ensuring (LogicalID, Sequence) uniqueness per REQ-BR-AMEND-006.
func (d *outboundDispatcher) nextSequenceByKey(key string) uint64 {
	d.mu.Lock()
	c, ok := d.sequences[key]
	if !ok {
		c = &atomic.Uint64{}
		d.sequences[key] = c
	}
	d.mu.Unlock()
	return c.Add(1)
}

// dropSequence releases the sequence counter, buffer, and gate state for a
// session that has been removed from the registry. The buffer and sequence
// key is resolved via bufferKey (LogicalID when available, fallback connID).
//
// Transient disconnect policy (spec §10 item 5, research §6.1): for sessions
// with a non-empty LogicalID, dropping the sequence counter means the next
// connID mapping to the same LogicalID will restart from 1. This is
// acceptable because the buffer entries remain (they are keyed by LogicalID
// and have a 24h TTL), and a reconnecting tab will replay them regardless of
// the new counter value. The sequence counter is a monotonic generator, not a
// persistent state — gaps in sequence numbers between reconnect cycles are
// tolerated by the replay protocol.
//
// For intentional logout (eager drop), use dropLogicalBuffer instead.
func (d *outboundDispatcher) dropSequence(sessionID string) {
	key := d.bufferKey(sessionID)
	d.mu.Lock()
	delete(d.sequences, key)
	d.mu.Unlock()
	if d.buffer != nil {
		d.buffer.Drop(key)
	}
	if d.gate != nil {
		d.gate.Drop(sessionID) // gate is connID-keyed (transport unit)
	}
}

// dropLogicalBuffer eagerly removes all buffer entries and sequence counter
// for logicalID. This is the logout hook invoked by
// Registry.CloseSessionsByCookieHash BEFORE closing individual transports,
// ensuring no buffered messages remain replayable after an intentional
// session invalidation (REQ-BR-AMEND-007, AC-BR-AMEND-008).
//
// @MX:ANCHOR: [AUTO] Logout eager-drop: buffer + sequence cleared atomically
// before transport closers run.
// @MX:REASON: Security invariant from REQ-BR-AMEND-007 — buffered messages
// from a deliberately invalidated session MUST NOT be replayable. The hook
// MUST fire before Registry.CloseSessionsByCookieHash invokes closers;
// ordering is enforced by the caller (registry.go CloseSessionsByCookieHash).
// @MX:SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001 REQ-BR-AMEND-007 AC-BR-AMEND-008
func (d *outboundDispatcher) dropLogicalBuffer(logicalID string) {
	if d.buffer != nil {
		d.buffer.Drop(logicalID)
	}
	d.mu.Lock()
	delete(d.sequences, logicalID)
	d.mu.Unlock()
	// Note: gate is connID-keyed, not LogicalID-keyed. Individual transport
	// teardown (via closers) will handle gate cleanup on the connID side.
}

// SendOutbound assigns the next sequence number and dispatches the message
// through the session's registered SessionSender.
//
// Behavior matrix (M4 + M5 follow-up + AMEND-001 M3):
//   - No session AND no sender → ErrSessionUnknown (M3 contract).
//   - Session present, sender absent (reconnecting tab) → buffer for
//     replay, return seq, no error.
//   - Sender present, gate stalled → buffer for replay, skip emit
//     (backpressure per REQ-BR-010), return seq, no error.
//   - Sender present, gate idle → buffer for replay, emit through
//     sender (transport owns ObserveWrite/ObserveDrain bracket so the
//     gate measures actual wire-write boundaries, not dispatch-call
//     boundaries — M5 follow-up transport wire-in).
//
// AMEND-001 buffer keying (REQ-BR-AMEND-003):
//
//	bufferKey(sessionID) resolves the LogicalID for sessionID via the
//	registry. The sequence counter and buffer entry are both keyed by the
//	LogicalID (or fallback to sessionID when no mapping exists). The
//	wire sender still receives the original connID-keyed msg, which is
//	safe because outboundEnvelope does NOT serialise SessionID — only
//	Type, Sequence, and Payload reach the wire (spec §7.1 invariant).
//
// @MX:WARN: [AUTO] Wire envelope invariant dependency (spec §7.1).
// @MX:REASON: The buffer-keying swap (bufKeyMsg.SessionID = logicalID)
// is safe only because outboundEnvelope (outbound.go:147-150) does NOT
// include a SessionID field. If a future wire schema change adds
// "session_id" to the JSON envelope, replayed messages would leak the
// LogicalID to the client instead of the connID. Any envelope schema
// change MUST revisit this swap pattern and spec §7.1.
// @MX:SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001 §7.1
func (d *outboundDispatcher) SendOutbound(sessionID string, t OutboundType, payload []byte) (uint64, error) {
	sender := d.registry.Sender(sessionID)
	_, sessionExists := d.registry.Get(sessionID)
	if sender == nil && !sessionExists {
		return 0, fmt.Errorf("%w: id=%s", ErrSessionUnknown, sessionID)
	}

	// Resolve LogicalID once; used for both sequence counter and buffer key.
	// Falls back to sessionID when no registry mapping exists (unit-test
	// fixtures that construct the dispatcher without sessions registered).
	key := d.bufferKey(sessionID)
	seq := d.nextSequenceByKey(key)
	msg := OutboundMessage{
		SessionID: sessionID, // wire sender receives the connID-keyed msg
		Type:      t,
		Payload:   payload,
		Sequence:  seq,
	}

	// Always buffer when buffering is enabled; resume correctness depends
	// on every sequenced message being recoverable until TTL or eviction.
	// The buffer entry uses LogicalID as the key (AMEND-001 REQ-BR-AMEND-003).
	// Safety: swapping SessionID to the logical key is invisible on the wire
	// because outboundEnvelope does not include a SessionID field (spec §7.1).
	if d.buffer != nil {
		bufKeyMsg := msg
		bufKeyMsg.SessionID = key // key = LogicalID (or fallback connID)
		d.buffer.Append(bufKeyMsg)
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
