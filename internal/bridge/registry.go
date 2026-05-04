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

// Add inserts a new session. Returns ErrSessionIDEmpty if the ID is blank,
// ErrSessionExists if the ID is already present.
func (r *Registry) Add(s WebUISession) error {
	if s.ID == "" {
		return ErrSessionIDEmpty
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sessions[s.ID]; exists {
		return fmt.Errorf("%w: id=%s", ErrSessionExists, s.ID)
	}
	r.sessions[s.ID] = s
	return nil
}

// Get returns the session with the given ID. The boolean is false when no
// such session is registered.
func (r *Registry) Get(id string) (WebUISession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

// Remove deletes the session with the given ID. Removing a missing ID is a
// no-op (idempotent).
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
}

// Snapshot returns a copy of all currently registered sessions. Callers are
// free to mutate the returned slice and its elements without affecting the
// registry.
func (r *Registry) Snapshot() []WebUISession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]WebUISession, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	return out
}

// Len returns the current number of registered sessions.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}
