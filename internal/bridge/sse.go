// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-011, REQ-BR-014, REQ-BR-015
// AC: AC-BR-011, AC-BR-012, AC-BR-013
// M2-T6 — SSE handler + Last-Event-ID + POST /bridge/inbound (SSE fallback inbound).

package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// SSEHandler is the http.Handler bound to GET /bridge/stream.
//
// Behaviour:
//   - Validates the session cookie via AuthRequest (no CSRF on GET stream)
//   - Sets Content-Type: text/event-stream + cache headers
//   - Reads Last-Event-ID for resume hint (M4 will use it; M2 stores in
//     SessionState only)
//   - Registers an sseCloser so logout can terminate the stream
//   - Holds the response open until the client disconnects or the closer fires
//
// On unauthenticated/expired/revoked: HTTP 401 with body
// {"error":"unauthenticated"} (REQ-BR-014: SSE uses HTTP status, not close code).
type SSEHandler struct {
	cfg MuxConfig
}

// NewSSEHandler constructs the SSE stream handler.
func NewSSEHandler(cfg MuxConfig) *SSEHandler {
	if cfg.Adapter == nil {
		cfg.Adapter = noopAdapter{}
	}
	return &SSEHandler{cfg: cfg}
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Metrics != nil {
		h.cfg.Metrics.RecordReconnect(r.Context(), 1)
	}

	sid, cookieHash, authErr := AuthRequest(r, h.cfg.Auth, h.cfg.Revocation, false)
	if authErr != nil {
		// Same RecordFailure semantics as ws.go (CodeRabbit Finding #4).
		if h.cfg.RateLimiter != nil && len(authErr.CookieHash) > 0 {
			h.cfg.RateLimiter.RecordAttempt(authErr.CookieHash)
			h.cfg.RateLimiter.RecordFailure(authErr.CookieHash)
		}
		switch authErr.Reason {
		case "bad_origin":
			writeJSONError(w, http.StatusForbidden, "bad_origin")
		default:
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
		}
		return
	}

	if h.cfg.RateLimiter != nil && len(cookieHash) > 0 {
		h.cfg.RateLimiter.RecordAttempt(cookieHash)
		switch h.cfg.RateLimiter.Check(cookieHash) {
		case RateRequireFreshCookie:
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
			return
		case RateLimited:
			writeJSONError(w, http.StatusTooManyRequests, "rate_limited")
			return
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "stream_not_supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx-style proxy buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	connID := sid + "-" + randSuffix()
	closer := newSSECloser(w, flusher)
	sender := newSSESender(w, flusher, h.cfg.gate, connID)

	if err := h.cfg.Registry.Add(WebUISession{
		ID:           connID,
		CookieHash:   cookieHash,
		Transport:    TransportSSE,
		OpenedAt:     time.Now(),
		LastActivity: time.Now(),
		State:        SessionStateOpen,
	}); err != nil {
		return
	}
	h.cfg.Registry.RegisterCloser(connID, closer)
	h.cfg.Registry.RegisterSender(connID, sender)

	if h.cfg.RateLimiter != nil && len(cookieHash) > 0 {
		h.cfg.RateLimiter.RecordSuccess(cookieHash)
	}

	defer func() {
		h.cfg.Registry.UnregisterCloser(connID)
		h.cfg.Registry.UnregisterSender(connID)
		h.cfg.Registry.Remove(connID)
	}()

	// Replay buffered outbound messages whose sequence is greater than
	// Last-Event-ID. Same caveat as ws.go: connID-keyed buffer means
	// this only fires when the same connID re-stream-opens; cookie-hash
	// logical mapping is a separate amendment.
	if h.cfg.resumer != nil {
		for _, msg := range h.cfg.resumer.Resume(connID, r.Header) {
			_ = sender.SendOutbound(msg)
		}
	}

	// Hold the connection open until the client disconnects or closer fires.
	select {
	case <-r.Context().Done():
	case <-closer.done:
	}
}

// sseCloser writes an SSE error event with the given close code, then
// signals the handler goroutine to return so the HTTP response can complete.
type sseCloser struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
	done    chan struct{}
	closed  bool
}

func newSSECloser(w http.ResponseWriter, f http.Flusher) *sseCloser {
	return &sseCloser{w: w, flusher: f, done: make(chan struct{})}
}

// sseSender implements SessionSender by writing the JSON envelope as an
// SSE event. The "id:" line carries the sequence so the browser's
// EventSource exposes it to the next reconnect via Last-Event-ID
// (M4 resume scaffold).
//
// SendOutbound brackets the multi-line write with gate.ObserveWrite +
// defer ObserveDrain so backpressure measures actual transport pressure
// (M5 follow-up — symmetric with wsSender).
type sseSender struct {
	w         http.ResponseWriter
	flusher   http.Flusher
	mu        sync.Mutex
	gate      *flushGate
	sessionID string
}

func newSSESender(w http.ResponseWriter, f http.Flusher, gate *flushGate, sessionID string) *sseSender {
	return &sseSender{w: w, flusher: f, gate: gate, sessionID: sessionID}
}

func (s *sseSender) SendOutbound(msg OutboundMessage) error {
	body, err := encodeOutboundJSON(msg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gate != nil {
		s.gate.ObserveWrite(s.sessionID, len(msg.Payload))
		defer s.gate.ObserveDrain(s.sessionID, len(msg.Payload))
	}
	if _, err := fmt.Fprintf(s.w, "id: %d\nevent: %s\ndata: %s\n\n",
		msg.Sequence, msg.Type, body); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (c *sseCloser) Close(code CloseCode) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	payload, _ := json.Marshal(map[string]any{
		"code":   uint16(code),
		"reason": closeReason(code),
	})
	fmt.Fprintf(c.w, "event: error\ndata: %s\n\n", payload)
	c.flusher.Flush()
	close(c.done)
	return nil
}

// InboundPostHandler is the http.Handler bound to POST /bridge/inbound.
// SSE fallback's inbound channel: same validation pipeline as WebSocket
// frames, but each request carries exactly one inbound message.
type InboundPostHandler struct {
	cfg MuxConfig
}

// NewInboundPostHandler constructs the SSE-fallback inbound POST handler.
func NewInboundPostHandler(cfg MuxConfig) *InboundPostHandler {
	if cfg.Adapter == nil {
		cfg.Adapter = noopAdapter{}
	}
	return &InboundPostHandler{cfg: cfg}
}

func (h *InboundPostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sid, _, authErr := AuthRequest(r, h.cfg.Auth, h.cfg.Revocation, true)
	if authErr != nil {
		switch authErr.Reason {
		case "bad_origin":
			writeJSONError(w, http.StatusForbidden, "bad_origin")
		case "csrf_mismatch":
			writeJSONError(w, http.StatusForbidden, "csrf_mismatch")
		default:
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
		}
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, MaxInboundBytes+1))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "read_failed")
		return
	}
	if len(body) > MaxInboundBytes {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":       "size_exceeded",
			"limit_bytes": MaxInboundBytes,
		})
		return
	}

	msg, decodeErr := DecodeInbound(sid, body, time.Now())
	if decodeErr != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed")
		return
	}
	if err := dispatchInbound(r.Context(), h.cfg, msg); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "adapter_failure")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// parseLastEventID returns the integer sequence carried by the SSE
// Last-Event-ID header, or 0 if absent / malformed.
// Exposed as a helper for M4 (resume).
func parseLastEventID(r *http.Request) uint64 {
	v := r.Header.Get("Last-Event-ID")
	if v == "" {
		return 0
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
