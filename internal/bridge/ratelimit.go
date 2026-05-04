// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-018
// AC: AC-BR-016
// M5-T1 — reconnect rate-limit per cookie hash.
//
// Two independent guards (spec.md §5.7 REQ-BR-018, §6.2):
//   1. Consecutive failure streak: 10 consecutive failed reconnects
//      invalidate the cookie. Subsequent attempts must close 4401
//      (`unauthenticated`); the client requires a fresh login.
//   2. Sliding window throughput: more than 60 attempts within any
//      rolling 60-second window for the same cookie hash close 4429
//      (`rate_limited`). The client must back off per §6.2 schedule.
//
// Successful reconnects reset the streak counter but do not erase the
// sliding window — a flapping connection that succeeds occasionally
// still triggers 4429 until traffic subsides.
//
// All state is keyed by string(cookieHash) — a stable HMAC-derived
// fingerprint of the local session cookie. Plain cookies are never
// persisted (spec.md §6.4 item 4).

package bridge

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// Rate-limit thresholds per spec.md §5.7 REQ-BR-018.
const (
	MaxConsecutiveFailures = 10
	RateLimitWindow        = time.Minute
	MaxAttemptsPerWindow   = 60
)

// RateDecision is the outcome of a Check call.
type RateDecision int

const (
	// RateAllow means the attempt may proceed.
	RateAllow RateDecision = iota
	// RateRequireFreshCookie means the cookie has burned through its
	// consecutive failure budget; close with CloseUnauthenticated (4401)
	// and require a new login.
	RateRequireFreshCookie
	// RateLimited means the per-minute attempt cap is exhausted; close
	// with CloseRateLimited (4429).
	RateLimited
)

// rateState tracks one cookie-hash bucket.
type rateState struct {
	consecutiveFailures int
	timestamps          []time.Time // sliding window of recent attempts
}

// rateLimiter enforces per-cookie reconnect throttling.
//
// @MX:ANCHOR
// @MX:REASON Authentication path security boundary; bypass would let a
// flapping or hostile cookie loop indefinitely against the auth pipeline.
type rateLimiter struct {
	clock clockwork.Clock
	mu    sync.Mutex
	state map[string]*rateState
}

func newRateLimiter(clock clockwork.Clock) *rateLimiter {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &rateLimiter{
		clock: clock,
		state: make(map[string]*rateState),
	}
}

// Check inspects the bucket for cookieHash and returns the decision the
// caller MUST honour. Check itself does not modify counters; it merely
// inspects state. Callers update counters via RecordAttempt / RecordSuccess
// after the inspection.
func (r *rateLimiter) Check(cookieHash []byte) RateDecision {
	if len(cookieHash) == 0 {
		return RateAllow
	}
	now := r.clock.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	st := r.stateLocked(cookieHash)
	if st.consecutiveFailures >= MaxConsecutiveFailures {
		return RateRequireFreshCookie
	}
	st.timestamps = pruneWindow(st.timestamps, now)
	if len(st.timestamps) >= MaxAttemptsPerWindow {
		return RateLimited
	}
	return RateAllow
}

// RecordAttempt logs an attempt timestamp into the sliding window. Call
// this on every reconnect attempt — successful or not — so the window
// reflects total inbound pressure on the cookie.
func (r *rateLimiter) RecordAttempt(cookieHash []byte) {
	if len(cookieHash) == 0 {
		return
	}
	now := r.clock.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.stateLocked(cookieHash)
	st.timestamps = append(pruneWindow(st.timestamps, now), now)
}

// RecordFailure increments the consecutive-failure streak. Call this AFTER
// auth has rejected the attempt.
func (r *rateLimiter) RecordFailure(cookieHash []byte) {
	if len(cookieHash) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.stateLocked(cookieHash)
	st.consecutiveFailures++
}

// RecordSuccess resets the consecutive-failure streak. The sliding window
// is unaffected — a successful reconnect does not free the per-minute
// budget.
func (r *rateLimiter) RecordSuccess(cookieHash []byte) {
	if len(cookieHash) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.stateLocked(cookieHash)
	st.consecutiveFailures = 0
}

// Drop removes all state for cookieHash. Called when the cookie is
// invalidated (logout, session_revoked) so a freshly issued cookie
// starts with an empty bucket.
func (r *rateLimiter) Drop(cookieHash []byte) {
	if len(cookieHash) == 0 {
		return
	}
	r.mu.Lock()
	delete(r.state, string(cookieHash))
	r.mu.Unlock()
}

// stateLocked returns or lazily allocates the bucket for cookieHash.
// Caller must hold r.mu.
func (r *rateLimiter) stateLocked(cookieHash []byte) *rateState {
	key := string(cookieHash)
	st, ok := r.state[key]
	if !ok {
		st = &rateState{}
		r.state[key] = st
	}
	return st
}

// pruneWindow returns a new slice containing only the entries within the
// active rate-limit window. The returned slice may share backing storage
// with the input.
func pruneWindow(ts []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-RateLimitWindow)
	dropped := 0
	for dropped < len(ts) && !ts[dropped].After(cutoff) {
		dropped++
	}
	if dropped == 0 {
		return ts
	}
	return ts[dropped:]
}
