// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-001
// AC: AC-BR-001
// M0-T4 — in-memory session registry with concurrent-safe CRUD.

package bridge

import (
	"errors"
	"fmt"
	"sync"
)

// ErrSessionExists is returned by Registry.Add when a session with the same
// ID is already registered.
var ErrSessionExists = errors.New("bridge: session already registered")

// ErrSessionIDEmpty is returned by Registry.Add when the WebUISession.ID is
// the empty string.
var ErrSessionIDEmpty = errors.New("bridge: session ID is empty")

// SessionCloser is the connection-side hook the registry can invoke to
// terminate an active transport (WebSocket frame loop or SSE response).
// M1 defines the contract; M2 wires real WebSocket / SSE implementations.
type SessionCloser interface {
	// Close terminates the transport with the given close code. Returning
	// an error is informational; callers MUST NOT depend on synchronous
	// cleanup — actual frame transmission is asynchronous.
	Close(code CloseCode) error
}

// Registry holds the active WebUISession set keyed by ID. All methods are
// safe for concurrent use.
//
// @MX:ANCHOR
// @MX:REASON Single source of truth for session lifecycle; consumed by
// server, mux, auth handlers, replay, and OTel exporter.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]WebUISession
	closers  map[string]SessionCloser
}

// NewRegistry returns an empty Registry ready for use.
func NewRegistry() *Registry {
	return &Registry{
		sessions: make(map[string]WebUISession),
		closers:  make(map[string]SessionCloser),
	}
}

// cloneSession deep-copies the byte-slice fields of a WebUISession so the
// caller cannot mutate registry-internal state through a shared backing array.
// The session's CookieHash and CSRFHash are sensitive (HMAC of cookie / CSRF
// secrets, spec.md §6.4 item 4) and the registry's invariants depend on those
// values being stable for the session's lifetime.
func cloneSession(s WebUISession) WebUISession {
	if s.CookieHash != nil {
		s.CookieHash = append([]byte(nil), s.CookieHash...)
	}
	if s.CSRFHash != nil {
		s.CSRFHash = append([]byte(nil), s.CSRFHash...)
	}
	return s
}

// Add inserts a new session. Returns ErrSessionIDEmpty if the ID is blank,
// ErrSessionExists if the ID is already present. The CookieHash / CSRFHash
// byte slices are deep-copied to isolate the caller's backing arrays from
// registry-internal storage.
func (r *Registry) Add(s WebUISession) error {
	if s.ID == "" {
		return ErrSessionIDEmpty
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sessions[s.ID]; exists {
		return fmt.Errorf("%w: id=%s", ErrSessionExists, s.ID)
	}
	r.sessions[s.ID] = cloneSession(s)
	return nil
}

// Get returns the session with the given ID. The boolean is false when no
// such session is registered. Returned byte-slice fields are deep copies;
// the caller may mutate them freely without affecting the registry.
func (r *Registry) Get(id string) (WebUISession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok {
		return WebUISession{}, false
	}
	return cloneSession(s), true
}

// Remove deletes the session with the given ID and any registered closer.
// Removing a missing ID is a no-op (idempotent).
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
	delete(r.closers, id)
}

// RegisterCloser associates a transport-level closer with the session ID.
// The transport (WebSocket / SSE) calls this on connect; logoutHandler and
// graceful shutdown invoke the closer to terminate the transport.
// Replacing an existing closer for the same ID is allowed (silent overwrite).
func (r *Registry) RegisterCloser(id string, c SessionCloser) {
	if id == "" || c == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closers[id] = c
}

// UnregisterCloser drops the closer associated with the session ID.
// Used when the transport disconnects gracefully without invoking close.
func (r *Registry) UnregisterCloser(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.closers, id)
}

// CloseSessionsByCookieHash invokes the registered closer for every session
// whose CookieHash equals the given hash, sending the supplied close code.
// Returns the number of closers invoked. Sessions without a registered
// closer are skipped (logout completes when transports register on next
// connect attempt and observe the revocation).
func (r *Registry) CloseSessionsByCookieHash(cookieHash []byte, code CloseCode) int {
	if len(cookieHash) == 0 {
		return 0
	}
	r.mu.RLock()
	matches := make([]SessionCloser, 0)
	for id, s := range r.sessions {
		if hashesEqual(s.CookieHash, cookieHash) {
			if c, ok := r.closers[id]; ok {
				matches = append(matches, c)
			}
		}
	}
	r.mu.RUnlock()

	for _, c := range matches {
		_ = c.Close(code)
	}
	return len(matches)
}

func hashesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Snapshot returns a copy of all currently registered sessions. Callers are
// free to mutate the returned slice and its elements (including the
// CookieHash / CSRFHash byte slices) without affecting the registry.
func (r *Registry) Snapshot() []WebUISession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]WebUISession, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, cloneSession(s))
	}
	return out
}

// Len returns the current number of registered sessions.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}
