// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-001, REQ-BR-002, REQ-BR-003, REQ-BR-007, REQ-BR-009, REQ-BR-010
// AC: AC-BR-001, AC-BR-002, AC-BR-003
// M0-T1, M0-T2, M0-T3 — core type signatures per spec.md §6.3.

// Package bridge implements the goosed daemon ↔ localhost Web UI bridge.
// Scope: SPEC-GOOSE-BRIDGE-001 v0.2.0 — single HTTP listener exposing
// WebSocket / SSE / POST inbound on a loopback bind, gated by HMAC session
// cookies and CSRF double-submit tokens.
package bridge

import (
	"context"
	"time"
)

// Bridge is the public surface of the daemon-side Web UI bridge.
//
// @MX:ANCHOR
// @MX:REASON Public API; fan_in ≥ 3 (server, mux, integration tests).
type Bridge interface {
	// Start binds the HTTP listener and begins accepting WebSocket / SSE / POST
	// connections. Returns ErrNonLoopbackBind if the configured bind host is
	// not a loopback alias.
	Start(ctx context.Context) error

	// Stop performs a graceful shutdown of the listener and all active sessions.
	// Sessions are notified via CloseGoingAway (1001).
	Stop(ctx context.Context) error

	// Sessions returns a snapshot of active Web UI sessions.
	Sessions() []WebUISession

	// Metrics returns the current OTel metrics snapshot.
	Metrics() Metrics
}

// WebUISession represents a single browser tab's connection to the bridge.
// Cookie and CSRF values are stored as HMAC hashes only; raw secrets are
// never persisted or logged (spec.md §6.4 item 4).
type WebUISession struct {
	ID           string    // session UUID
	CookieHash   []byte    // HMAC(local session cookie)
	CSRFHash     []byte    // HMAC(CSRF token)
	Transport    Transport // websocket | sse
	OpenedAt     time.Time
	LastActivity time.Time
	State        SessionState
}

// Transport identifies the wire protocol used by a session.
type Transport string

const (
	TransportWebSocket Transport = "websocket"
	TransportSSE       Transport = "sse"
)

// SessionState tracks the lifecycle stage of a WebUISession.
// open → active → idle → reconnecting → closed (terminal).
type SessionState string

const (
	SessionStateOpen         SessionState = "open"
	SessionStateActive       SessionState = "active"
	SessionStateIdle         SessionState = "idle"
	SessionStateReconnecting SessionState = "reconnecting"
	SessionStateClosed       SessionState = "closed"
)

// BindAddress is a loopback-validated bind target. Construction does not
// guarantee validity; callers must invoke verifyLoopbackBind on the
// "host:port" form before binding.
type BindAddress struct {
	Host string // "127.0.0.1" | "::1" | "localhost"
	Port int    // 1..65535
}

// InboundMessage represents a payload arriving from the browser.
type InboundMessage struct {
	SessionID  string
	Type       InboundType
	Payload    []byte
	ReceivedAt time.Time
}

// InboundType enumerates the inbound message dispatch keys.
type InboundType string

const (
	InboundChat               InboundType = "chat"
	InboundAttachment         InboundType = "attachment"
	InboundPermissionResponse InboundType = "permission_response"
	InboundControl            InboundType = "control" // ping, abort
)

// OutboundMessage represents a payload sent from the daemon to the browser.
// Sequence is monotonically increasing per session for replay ordering.
type OutboundMessage struct {
	SessionID string
	Type      OutboundType
	Payload   []byte
	Sequence  uint64
}

// OutboundType enumerates the outbound message dispatch keys.
type OutboundType string

const (
	OutboundChunk             OutboundType = "chunk"
	OutboundNotification      OutboundType = "notification"
	OutboundPermissionRequest OutboundType = "permission_request"
	OutboundStatus            OutboundType = "status"
	OutboundError             OutboundType = "error"
)

// FlushGate signals browser-side write-queue backpressure. Callers MUST
// check Stalled before issuing high-volume writes; if true, Wait blocks
// until the queue drains below the low-watermark.
//
// @MX:ANCHOR
// @MX:REASON Backpressure contract — implementations populated in M4.
type FlushGate interface {
	Stalled(sessionID string) bool
	Wait(ctx context.Context, sessionID string) error
	ObserveWrite(sessionID string, bytes int)
	ObserveDrain(sessionID string, bytes int)
}

// CloseCode represents the WebSocket close code matrix from spec.md §6.1.
type CloseCode uint16

const (
	CloseNormal            CloseCode = 1000
	CloseGoingAway         CloseCode = 1001
	CloseMessageTooBig     CloseCode = 1009
	CloseInternalError     CloseCode = 1011
	CloseUnauthenticated   CloseCode = 4401
	CloseSessionRevoked    CloseCode = 4403
	CloseSessionTimeout    CloseCode = 4408
	ClosePayloadTooLarge   CloseCode = 4413
	CloseRateLimited       CloseCode = 4429
	CloseBridgeUnavailable CloseCode = 4500
)

// Metrics is a placeholder for OTel measurements wired in M5
// (REQ-BR-004). M0 returns a zero-valued struct.
type Metrics struct {
	ActiveSessions        int64
	InboundMessagesTotal  uint64
	OutboundMessagesTotal uint64
	FlushGateStalls       uint64
	ReconnectAttempts     uint64
}
