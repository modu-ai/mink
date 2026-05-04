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
	sid, cookieHash, authErr := AuthRequest(r, h.cfg.Auth, h.cfg.Revocation, false)
	if authErr != nil {
		switch authErr.Reason {
		case "bad_origin":
			writeJSONError(w, http.StatusForbidden, "bad_origin")
		default:
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
		}
		return
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

	defer func() {
		h.cfg.Registry.UnregisterCloser(connID)
		h.cfg.Registry.Remove(connID)
	}()

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
	if err := h.cfg.Adapter.HandleInbound(msg); err != nil {
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
