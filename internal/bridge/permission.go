// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-008
// AC: AC-BR-008
// M3-T4 — outbound permission_request + 60s default-deny timeout.

package bridge

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// PermissionTimeout is the wall-clock deadline before a pending permission
// request is treated as denied (REQ-BR-008).
const PermissionTimeout = 60 * time.Second

// ErrPermissionDenied is returned by RequestPermission when the browser
// explicitly denied the request. Distinguishable from a timeout (the
// caller observes granted=false + ErrPermissionTimeout).
var ErrPermissionDenied = errors.New("bridge: permission denied")

// ErrPermissionTimeout is returned by RequestPermission when no response
// arrived inside PermissionTimeout. The browser may have disconnected,
// backgrounded the tab, or simply ignored the prompt.
var ErrPermissionTimeout = errors.New("bridge: permission request timed out")

// permissionStore tracks in-flight requests keyed by their generated request
// ID. Each entry carries a single-buffered channel the inbound dispatcher
// uses to deliver the response.
type permissionStore struct {
	mu      sync.Mutex
	pending map[string]chan permissionVerdict
	clock   clockwork.Clock
}

type permissionVerdict struct {
	granted bool
}

func newPermissionStore(clk clockwork.Clock) *permissionStore {
	if clk == nil {
		clk = clockwork.NewRealClock()
	}
	return &permissionStore{
		pending: make(map[string]chan permissionVerdict),
		clock:   clk,
	}
}

// register allocates a request ID + response channel pair. The channel is
// buffered (size 1) so an early Resolve() never blocks even if the caller
// has not yet entered its select.
func (s *permissionStore) register() (requestID string, ch chan permissionVerdict) {
	id := newRequestID()
	ch = make(chan permissionVerdict, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()
	return id, ch
}

// resolve delivers a verdict for the given request ID. Returns false if
// the ID is unknown (already resolved or never registered).
func (s *permissionStore) resolve(id string, granted bool) bool {
	s.mu.Lock()
	ch, ok := s.pending[id]
	if ok {
		delete(s.pending, id)
	}
	s.mu.Unlock()
	if !ok {
		return false
	}
	ch <- permissionVerdict{granted: granted}
	return true
}

// drop removes a pending request without delivering a verdict. Used on
// timeout to free the map entry.
func (s *permissionStore) drop(id string) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}

// pendingCount is exposed for test introspection.
func (s *permissionStore) pendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// permissionRequester binds a permissionStore to an outboundDispatcher
// so the public RequestPermission API can issue outbound traffic + wait
// for the inbound reply.
type permissionRequester struct {
	store      *permissionStore
	dispatcher *outboundDispatcher
	timeout    time.Duration
}

func newPermissionRequester(store *permissionStore, disp *outboundDispatcher, timeout time.Duration) *permissionRequester {
	if timeout <= 0 {
		timeout = PermissionTimeout
	}
	return &permissionRequester{store: store, dispatcher: disp, timeout: timeout}
}

// permissionRequestEnvelope is the wire body wrapped inside the
// OutboundPermissionRequest payload. The browser echoes request_id back
// inside the InboundPermissionResponse for correlation.
type permissionRequestEnvelope struct {
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// permissionResponseEnvelope is the inbound reply shape.
type permissionResponseEnvelope struct {
	RequestID string `json:"request_id"`
	Granted   bool   `json:"granted"`
}

// permissionTimeoutPayload is sent as OutboundStatus on timeout so the
// browser observes that its prompt was abandoned (REQ-BR-008).
type permissionTimeoutPayload struct {
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

// Request emits an OutboundPermissionRequest and blocks until either:
//   - inbound dispatcher delivers a verdict (granted true|false)
//   - ctx is cancelled (returns ctx.Err)
//   - PermissionTimeout elapses (granted=false, ErrPermissionTimeout, plus
//     OutboundStatus{permission_timeout} emitted to the session)
func (r *permissionRequester) Request(ctx context.Context, sessionID string, payload []byte) (bool, error) {
	id, ch := r.store.register()

	envelope, err := json.Marshal(permissionRequestEnvelope{
		RequestID: id,
		Payload:   json.RawMessage(payload),
	})
	if err != nil {
		r.store.drop(id)
		return false, fmt.Errorf("bridge: permission envelope marshal: %w", err)
	}
	if _, err := r.dispatcher.SendOutbound(sessionID, OutboundPermissionRequest, envelope); err != nil {
		r.store.drop(id)
		return false, err
	}

	timer := r.store.clock.NewTimer(r.timeout)
	defer timer.Stop()

	select {
	case v := <-ch:
		if !v.granted {
			return false, ErrPermissionDenied
		}
		return true, nil
	case <-timer.Chan():
		r.store.drop(id)
		// Best-effort: tell the browser its prompt expired. Failure to
		// emit the status (e.g., session already closed) does not change
		// the outcome — default-deny still applies.
		statusPayload, _ := json.Marshal(permissionTimeoutPayload{
			Code: "permission_timeout", RequestID: id,
		})
		_, _ = r.dispatcher.SendOutbound(sessionID, OutboundStatus, statusPayload)
		return false, ErrPermissionTimeout
	case <-ctx.Done():
		r.store.drop(id)
		return false, ctx.Err()
	}
}

// HandleInboundPermissionResponse parses an InboundPermissionResponse
// payload and routes the verdict to the matching pending request.
// Unknown request IDs are silently dropped (idempotent / late delivery).
func (r *permissionRequester) HandleInboundPermissionResponse(payload []byte) error {
	var env permissionResponseEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return fmt.Errorf("%w: permission response decode: %v", ErrInboundMalformed, err)
	}
	if env.RequestID == "" {
		return fmt.Errorf("%w: permission response missing request_id", ErrInboundMalformed)
	}
	r.store.resolve(env.RequestID, env.Granted)
	return nil
}

func newRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
