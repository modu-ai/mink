// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-017, REQ-BR-011
// AC: AC-BR-015, AC-BR-011
// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-004
// AC: AC-BR-AMEND-004, AC-BR-AMEND-005
// M4-T6 — session resume: parse X-Last-Sequence (WebSocket) and
// Last-Event-ID (SSE), then replay any buffered outbound messages from
// outboundBuffer.
// M4-T3 (AMEND-001) — resumer looks up LogicalID via registry so that a
// new connID sharing the same (CookieHash, Transport) can replay messages
// buffered under the prior connID's LogicalID (cross-connection replay).
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
//
// @MX:NOTE: [AUTO] registry field added in SPEC-GOOSE-BRIDGE-001-AMEND-001
// v0.1.1 (M4-T3). When registry is non-nil, Resume performs a connID →
// LogicalID lookup so that a new connection inherits the buffer of any
// prior connection with the same (CookieHash, Transport).
// @MX:SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001 REQ-BR-AMEND-004
type resumer struct {
	buffer   *outboundBuffer
	registry *Registry // nil disables LogicalID lookup (fallback to connID-keyed)
}

// newResumer constructs a resumer. reg may be nil; when nil the resumer
// falls back to connID-keyed buffer lookups (v0.2.1 compatible behaviour,
// also used by test fixtures that construct the resumer in isolation).
//
// The D1 audit carve-out in SPEC-GOOSE-BRIDGE-001-AMEND-001 §7 documents
// this package-private additive signature change.
func newResumer(buf *outboundBuffer, reg *Registry) *resumer {
	return &resumer{buffer: buf, registry: reg}
}

// Resume returns the buffered outbound messages whose sequence is greater
// than the client's last-known sequence. The session must already exist
// in the registry; this helper does NOT validate session ownership —
// callers must perform auth before invoking Resume.
//
// When a registry is present, Resume resolves the buffer key by looking up
// the LogicalID for sessionID (REQ-BR-AMEND-004). If the lookup succeeds,
// Replay is called with the LogicalID, enabling cross-connection replay for
// sessions that share the same (CookieHash, Transport). If the lookup fails
// (session not found or no LogicalID assigned), the original sessionID is
// used as the buffer key — preserving v0.2.1 behaviour.
//
// @MX:NOTE: [AUTO] connID → LogicalID lookup with fallback is the core of
// cross-connection replay. The registry is the single source of truth for
// this mapping (REQ-BR-AMEND-002).
// @MX:SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001 REQ-BR-AMEND-004 AC-BR-AMEND-004
func (r *resumer) Resume(sessionID string, h http.Header) []OutboundMessage {
	lastSeq := parseLastSequence(h)
	key := sessionID
	if r.registry != nil {
		if lid, ok := r.registry.LogicalID(sessionID); ok && lid != "" {
			key = lid
		}
	}
	return r.buffer.Replay(key, lastSeq)
}
