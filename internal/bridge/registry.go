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

// Registry holds the active WebUISession set keyed by ID. All methods are
// safe for concurrent use.
//
// @MX:ANCHOR
// @MX:REASON Single source of truth for session lifecycle; consumed by
// server, mux, auth handlers, replay, and OTel exporter.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]WebUISession
}

// NewRegistry returns an empty Registry ready for use.
func NewRegistry() *Registry {
	return &Registry{
		sessions: make(map[string]WebUISession),
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

// Remove deletes the session with the given ID. Removing a missing ID is a
// no-op (idempotent).
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
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
