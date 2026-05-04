// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-006, REQ-BR-015
// AC: AC-BR-006, AC-BR-013
// M2-T3, M2-T4, M2-T7 — inbound message validation pipeline + dispatch + 10MB limit.

package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// MaxInboundBytes is the configured per-message inbound size ceiling
// (REQ-BR-015). Crossing it forces close 4413 / HTTP 413.
const MaxInboundBytes = 10 << 20 // 10 MB

// ErrInboundTooLarge signals an inbound payload that exceeded MaxInboundBytes
// before validation. Carriage out as close 4413 (WebSocket) or HTTP 413.
var ErrInboundTooLarge = errors.New("bridge: inbound message exceeds size limit")

// ErrInboundMalformed signals a JSON parse failure or missing/unknown type
// field on an inbound message envelope.
var ErrInboundMalformed = errors.New("bridge: inbound message malformed")

// inboundEnvelope is the on-the-wire shape produced by the browser. Type
// dispatches into one of the InboundType constants from types.go; Payload
// carries the type-specific body verbatim for the QueryEngine adapter.
type inboundEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// QueryEngineAdapter is the seam between Bridge and the QueryEngine session
// runner (spec.md §3 IN scope item 3 / REQ-BR-006). M2 ships a stub-friendly
// interface; real wiring is owned by the QueryEngine SPEC.
//
// @MX:ANCHOR
// @MX:REASON Cross-package boundary; future change ripples through every
// inbound message type. Keep narrow.
type QueryEngineAdapter interface {
	// HandleInbound is invoked once per validated InboundMessage. The
	// adapter MUST return promptly; long-running work belongs in the
	// QueryEngine, not in the Bridge frame loop.
	HandleInbound(msg InboundMessage) error
}

// noopAdapter is the default QueryEngineAdapter used when none is configured.
// Useful for tests and for early bring-up where the QueryEngine is not yet
// wired in.
type noopAdapter struct{}

func (noopAdapter) HandleInbound(_ InboundMessage) error { return nil }

// DecodeInbound parses an envelope and returns a typed InboundMessage bound
// to the supplied session ID. Returns ErrInboundTooLarge when payload exceeds
// MaxInboundBytes; ErrInboundMalformed for any other parse failure.
func DecodeInbound(sessionID string, raw []byte, now time.Time) (InboundMessage, error) {
	if len(raw) > MaxInboundBytes {
		return InboundMessage{}, fmt.Errorf("%w: %d bytes (limit %d)",
			ErrInboundTooLarge, len(raw), MaxInboundBytes)
	}
	var env inboundEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return InboundMessage{}, fmt.Errorf("%w: %v", ErrInboundMalformed, err)
	}
	t, ok := classifyInbound(env.Type)
	if !ok {
		return InboundMessage{}, fmt.Errorf("%w: unknown type %q", ErrInboundMalformed, env.Type)
	}
	return InboundMessage{
		SessionID:  sessionID,
		Type:       t,
		Payload:    []byte(env.Payload),
		ReceivedAt: now,
	}, nil
}

// classifyInbound maps the wire string to a known InboundType. Unknown
// strings return false so the caller can reject the message.
func classifyInbound(s string) (InboundType, bool) {
	switch s {
	case string(InboundChat):
		return InboundChat, true
	case string(InboundAttachment):
		return InboundAttachment, true
	case string(InboundPermissionResponse):
		return InboundPermissionResponse, true
	case string(InboundControl):
		return InboundControl, true
	}
	return "", false
}
