// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-006, REQ-BR-007, REQ-BR-014, REQ-BR-015
// AC: AC-BR-006, AC-BR-012, AC-BR-013
// M2-T2, M2-T7 — WebSocket upgrade + frame loop + 10MB inbound limit.

package bridge

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// WebSocketHandler is the http.Handler bound to GET /bridge/ws.
//
// On upgrade it:
//  1. Validates the session cookie via AuthRequest (no CSRF — WebSocket
//     same-origin policy already gates the upgrade)
//  2. Accepts the upgrade with coder/websocket
//  3. Registers a SessionCloser so logout (CloseSessionsByCookieHash) can
//     terminate the connection with code 4403
//  4. Enters the frame read loop, dispatching each inbound message through
//     the QueryEngineAdapter
//
// Rejection mapping (REQ-BR-014, REQ-BR-015):
//   - unauthenticated/expired/revoked cookie → close 4401 before upgrade
//   - frame > 10 MB → close 4413
//   - bad_origin → 403 (pre-upgrade)
//
// @MX:WARN goroutine + context cancellation across upgrade boundary.
// @MX:REASON Holding a read loop over the WebSocket while exposing a
// Closer to the registry creates a producer/consumer pair that must
// shut down together; mishandling deadlocks the close path on logout.
type WebSocketHandler struct {
	cfg MuxConfig
}

// NewWebSocketHandler constructs the WebSocket handler. cfg.Adapter is
// substituted with noopAdapter when nil.
func NewWebSocketHandler(cfg MuxConfig) *WebSocketHandler {
	if cfg.Adapter == nil {
		cfg.Adapter = noopAdapter{}
	}
	return &WebSocketHandler{cfg: cfg}
}

func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sid, cookieHash, authErr := AuthRequest(r, h.cfg.Auth, h.cfg.Revocation, false)
	if authErr != nil {
		switch authErr.Reason {
		case "bad_origin":
			http.Error(w, `{"error":"bad_origin"}`, http.StatusForbidden)
		default:
			// Pre-upgrade unauthenticated/expired/revoked surface as 401;
			// the browser cannot observe a WebSocket close code before the
			// handshake completes, so REQ-BR-014's "close 4401" applies to
			// post-upgrade rejection only.
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
		}
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: h.cfg.WSAcceptOrigins,
	})
	if err != nil {
		// websocket.Accept already wrote a 4xx response.
		return
	}
	conn.SetReadLimit(MaxInboundBytes)

	// Allocate a fresh per-connection session ID derived from the cookie's
	// session ID plus a 4-byte salt — multiple tabs reuse the same cookie
	// but each upgrade creates a distinct WebUISession.
	connID := sid + "-" + randSuffix()

	closer := &wsCloser{conn: conn}
	sender := &wsSender{conn: conn, ctx: r.Context()}
	if err := h.cfg.Registry.Add(WebUISession{
		ID:           connID,
		CookieHash:   cookieHash,
		Transport:    TransportWebSocket,
		OpenedAt:     time.Now(),
		LastActivity: time.Now(),
		State:        SessionStateOpen,
	}); err != nil {
		_ = conn.Close(websocket.StatusInternalError, "registry add failed")
		return
	}
	h.cfg.Registry.RegisterCloser(connID, closer)
	h.cfg.Registry.RegisterSender(connID, sender)

	defer func() {
		h.cfg.Registry.UnregisterCloser(connID)
		h.cfg.Registry.UnregisterSender(connID)
		h.cfg.Registry.Remove(connID)
		_ = conn.CloseNow()
	}()

	h.runFrameLoop(r.Context(), conn, connID)
}

// runFrameLoop reads inbound frames until error or context cancellation.
// Returns silently — caller's deferred cleanup handles registry teardown.
func (h *WebSocketHandler) runFrameLoop(parent context.Context, conn *websocket.Conn, sessionID string) {
	for {
		ctx, cancel := context.WithCancel(parent)
		_, data, err := conn.Read(ctx)
		cancel()
		if err != nil {
			h.handleReadError(conn, err)
			return
		}

		msg, decodeErr := DecodeInbound(sessionID, data, time.Now())
		if errors.Is(decodeErr, ErrInboundTooLarge) {
			_ = conn.Close(websocket.StatusCode(ClosePayloadTooLarge), "payload_too_large")
			return
		}
		if decodeErr != nil {
			// Malformed inbound: report error frame and continue (do not
			// kill the session for client-side mistakes).
			_ = conn.Write(parent, websocket.MessageText,
				[]byte(`{"type":"error","payload":{"error":"malformed"}}`))
			continue
		}

		if err := dispatchInbound(h.cfg, msg); err != nil {
			_ = conn.Write(parent, websocket.MessageText,
				[]byte(`{"type":"error","payload":{"error":"adapter_failure"}}`))
			continue
		}
	}
}

// dispatchInbound routes an InboundMessage to either the permission
// requester (when permission_response and a requester is wired) or the
// QueryEngine adapter. Centralizing the branch keeps WS / SSE / POST
// inbound paths consistent.
func dispatchInbound(cfg MuxConfig, msg InboundMessage) error {
	if msg.Type == InboundPermissionResponse && cfg.permRequester != nil {
		return cfg.permRequester.HandleInboundPermissionResponse(msg.Payload)
	}
	return cfg.Adapter.HandleInbound(msg)
}

// handleReadError translates a coder/websocket read error into the most
// specific close code we can deduce. For unknown errors we close
// CloseInternalError (1011) so the client backs off per spec.md §6.2.
func (h *WebSocketHandler) handleReadError(conn *websocket.Conn, err error) {
	// MessageTooBig surfaces as a websocket.CloseError with the
	// MessageTooBig (1009) close code. We map to CloseMessageTooBig and let
	// the conn close hook propagate; if the limit was hit on the read path
	// itself the library has already started shutdown.
	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		// Already closing from the peer side; CloseNow in defer will finish.
		return
	}
	_ = conn.Close(websocket.StatusInternalError, "read_error")
}

// wsCloser implements SessionCloser by writing a close frame with the
// requested code. Safe for concurrent use; idempotent.
type wsCloser struct {
	conn *websocket.Conn
	mu   sync.Mutex
	done bool
}

// wsSender implements SessionSender by writing the JSON envelope as a
// MessageText frame. Safe for concurrent use — coder/websocket's Write
// is internally synchronized.
type wsSender struct {
	conn *websocket.Conn
	ctx  context.Context
}

func (s *wsSender) SendOutbound(msg OutboundMessage) error {
	body, err := encodeOutboundJSON(msg)
	if err != nil {
		return err
	}
	return s.conn.Write(s.ctx, websocket.MessageText, body)
}

func (c *wsCloser) Close(code CloseCode) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return nil
	}
	c.done = true
	return c.conn.Close(websocket.StatusCode(code), closeReason(code))
}

func closeReason(code CloseCode) string {
	switch code {
	case CloseNormal:
		return "normal_closure"
	case CloseGoingAway:
		return "going_away"
	case CloseMessageTooBig:
		return "message_too_big"
	case CloseUnauthenticated:
		return "unauthenticated"
	case CloseSessionRevoked:
		return "session_revoked"
	case CloseSessionTimeout:
		return "session_timeout"
	case ClosePayloadTooLarge:
		return "payload_too_large"
	case CloseRateLimited:
		return "rate_limited"
	case CloseBridgeUnavailable:
		return "bridge_unavailable"
	}
	return ""
}

func randSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
