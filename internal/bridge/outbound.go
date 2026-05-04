// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-007
// AC: AC-BR-007
// M3-T1, M3-T2 — outbound chunk dispatcher + per-session monotonic sequence.

package bridge

import (
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
// to the registered SessionSender for actual wire emission.
type outboundDispatcher struct {
	registry  *Registry
	mu        sync.Mutex
	sequences map[string]*atomic.Uint64
}

func newOutboundDispatcher(reg *Registry) *outboundDispatcher {
	return &outboundDispatcher{
		registry:  reg,
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
// counter memory across many connection cycles.
func (d *outboundDispatcher) dropSequence(sessionID string) {
	d.mu.Lock()
	delete(d.sequences, sessionID)
	d.mu.Unlock()
}

// SendOutbound assigns the next sequence number and dispatches the message
// through the session's registered SessionSender. Returns ErrSessionUnknown
// when no sender is registered for sessionID.
func (d *outboundDispatcher) SendOutbound(sessionID string, t OutboundType, payload []byte) (uint64, error) {
	sender := d.registry.Sender(sessionID)
	if sender == nil {
		return 0, fmt.Errorf("%w: id=%s", ErrSessionUnknown, sessionID)
	}
	seq := d.nextSequence(sessionID)
	msg := OutboundMessage{
		SessionID: sessionID,
		Type:      t,
		Payload:   payload,
		Sequence:  seq,
	}
	if err := sender.SendOutbound(msg); err != nil {
		return seq, fmt.Errorf("bridge: outbound emit failed (seq=%d): %w", seq, err)
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
