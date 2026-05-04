// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-017, REQ-BR-011
// AC: AC-BR-015, AC-BR-011
// M4-T6 — session resume: parse X-Last-Sequence (WebSocket) and
// Last-Event-ID (SSE), then replay any buffered outbound messages from
// outboundBuffer.
//
// The two header names are normative:
//   - WebSocket: clients send X-Last-Sequence on the upgrade request.
//     This is a custom header chosen because WebSocket has no equivalent
//     of Last-Event-ID. Spec.md §7.15 uses this name.
//   - SSE: clients send Last-Event-ID per the EventSource specification.
//     The bridge encodes the per-message sequence into the SSE id: field
//     so the browser hands it back transparently on reconnect.
//
// Both header values are decimal uint64. Empty / missing / malformed
// values resolve to lastSeq=0 which causes Replay to return every queued
// entry — the safe fallback when the client has lost track of progress.

package bridge

import (
	"net/http"
	"strconv"
)

// HeaderLastSequence is the WebSocket-upgrade resume header.
const HeaderLastSequence = "X-Last-Sequence"

// HeaderLastEventID is the SSE resume header (defined by the EventSource
// HTML living standard).
const HeaderLastEventID = "Last-Event-ID"

// parseLastSequence returns the uint64 value of the X-Last-Sequence header
// (preferred) or Last-Event-ID (fallback). Returns 0 when both are absent
// or unparseable; callers treat 0 as "replay everything still buffered".
func parseLastSequence(h http.Header) uint64 {
	if v := h.Get(HeaderLastSequence); v != "" {
		if seq, err := strconv.ParseUint(v, 10, 64); err == nil {
			return seq
		}
	}
	if v := h.Get(HeaderLastEventID); v != "" {
		if seq, err := strconv.ParseUint(v, 10, 64); err == nil {
			return seq
		}
	}
	return 0
}

// resumer wires a request's resume headers to the per-session outbound
// buffer. Returns the messages to be replayed to the client in their
// original sequence order.
type resumer struct {
	buffer *outboundBuffer
}

func newResumer(buf *outboundBuffer) *resumer {
	return &resumer{buffer: buf}
}

// Resume returns the buffered outbound messages whose sequence is greater
// than the client's last-known sequence. The session must already exist
// in the registry; this helper does NOT validate session ownership —
// callers must perform auth before invoking Resume.
func (r *resumer) Resume(sessionID string, h http.Header) []OutboundMessage {
	lastSeq := parseLastSequence(h)
	return r.buffer.Replay(sessionID, lastSeq)
}
