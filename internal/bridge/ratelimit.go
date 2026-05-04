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

	// rateStateTTL bounds how long an idle bucket survives in r.state.
	// Buckets with no live streak (consecutiveFailures==0) and no live
	// timestamps (window already pruned) are eligible for opportunistic
	// eviction once their lastSeen ages past this TTL. The value is
	// intentionally generous: cookieLifetime is 24h (auth.go) so a
	// per-cookie bucket shorter than that would risk losing the failure
	// streak across the cookie's natural backoff schedule.
	//
	// SPEC: SPEC-GOOSE-BRIDGE-001 M5 follow-up — CodeRabbit Finding #2
	// (rate-limit map unbounded growth across long-lived processes).
	rateStateTTL = 30 * time.Minute
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

// rateState tracks one cookie-hash bucket. lastSeen is refreshed on every
// access (Check / RecordAttempt / RecordFailure / RecordSuccess) and is
// consulted by pruneStaleLocked to evict idle buckets.
type rateState struct {
	consecutiveFailures int
	timestamps          []time.Time // sliding window of recent attempts
	lastSeen            time.Time
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
//
// Threshold semantics (spec.md §6.1, REQ-BR-018):
//   - consecutiveFailures >= 10 ⇒ RateRequireFreshCookie. Spec wording
//     "accept up to 10 consecutive failed reconnection attempts before
//     invalidating the cookie" — counters are bumped after each attempt
//     fails, so the 11th attempt observes streak == 10 and is blocked.
//   - len(timestamps) > 60 ⇒ RateLimited. Per spec.md §6.1, the
//     per-minute quota of 60 means up to and including 60 attempts pass
//     per window; only the 61st is rejected. (CodeRabbit Finding #3.)
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
	if len(st.timestamps) > MaxAttemptsPerWindow {
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

// stateLocked returns or lazily allocates the bucket for cookieHash and
// refreshes its lastSeen timestamp. On allocation, performs an
// opportunistic prune of stale neighbours so r.state cannot grow
// unboundedly across long-lived processes (CodeRabbit Finding #2).
// Caller must hold r.mu.
func (r *rateLimiter) stateLocked(cookieHash []byte) *rateState {
	key := string(cookieHash)
	now := r.clock.Now()
	st, ok := r.state[key]
	if !ok {
		// New-bucket allocation is rare under steady-state traffic, so
		// piggyback the prune sweep here. Cost: O(N) per allocation.
		r.pruneStaleLocked(now)
		st = &rateState{lastSeen: now}
		r.state[key] = st
		return st
	}
	st.lastSeen = now
	return st
}

// pruneStaleLocked drops buckets whose lastSeen is older than rateStateTTL
// AND have no live state (no failure streak, no attempts within the
// sliding window after pruneWindow). Caller must hold r.mu.
func (r *rateLimiter) pruneStaleLocked(now time.Time) {
	cutoff := now.Add(-rateStateTTL)
	for k, st := range r.state {
		if st.lastSeen.After(cutoff) {
			continue
		}
		if st.consecutiveFailures != 0 {
			continue
		}
		// Pruning the window here avoids a stale read keeping the bucket
		// alive past TTL when every timestamp has already aged out.
		st.timestamps = pruneWindow(st.timestamps, now)
		if len(st.timestamps) != 0 {
			continue
		}
		delete(r.state, k)
	}
}

// bucketCount returns the number of live cookie buckets. Test-only
// helper consulted by the eviction test.
func (r *rateLimiter) bucketCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.state)
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
