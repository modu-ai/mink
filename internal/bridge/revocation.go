// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-016 (v0.2.0)
// AC: AC-BR-014
// M1-T5 — RevocationStore: cookie-hash → revoked timestamp.
// Companion to logoutHandler. Future verifyCookie callers should consult
// IsRevoked before accepting a session cookie.

package bridge

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// RevocationStore tracks cookie hashes that have been explicitly logged out.
// Entries auto-expire after cookieLifetime so the store cannot grow without
// bound; once the cookie itself would have expired anyway, retaining its
// revocation record adds no security value.
type RevocationStore struct {
	mu       sync.RWMutex
	revoked  map[string]time.Time // key = base64(cookieHash), value = expiry instant
	lifetime time.Duration
	clock    clockwork.Clock
}

// NewRevocationStore returns a store with the standard 24h retention window.
func NewRevocationStore(clock clockwork.Clock) *RevocationStore {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &RevocationStore{
		revoked:  make(map[string]time.Time),
		lifetime: cookieLifetime,
		clock:    clock,
	}
}

// Revoke marks the given cookie hash as revoked. Subsequent IsRevoked calls
// return true until the retention window elapses.
func (r *RevocationStore) Revoke(cookieHash []byte) {
	if len(cookieHash) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gcLocked()
	r.revoked[string(cookieHash)] = r.clock.Now().Add(r.lifetime)
}

// IsRevoked returns true if the cookie hash has been revoked and its
// retention window has not yet elapsed.
func (r *RevocationStore) IsRevoked(cookieHash []byte) bool {
	if len(cookieHash) == 0 {
		return false
	}
	r.mu.RLock()
	exp, ok := r.revoked[string(cookieHash)]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	return r.clock.Now().Before(exp)
}

// Len returns the number of currently retained revocation entries (after GC).
func (r *RevocationStore) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gcLocked()
	return len(r.revoked)
}

// gcLocked drops entries past their expiry. Caller must hold r.mu.
func (r *RevocationStore) gcLocked() {
	now := r.clock.Now()
	for k, exp := range r.revoked {
		if !now.Before(exp) {
			delete(r.revoked, k)
		}
	}
}
