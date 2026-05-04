// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-018
// AC: AC-BR-016
// M5-T1 — rate-limit behavior.

package bridge

import (
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func TestRateLimit_AllowsFreshCookie(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	if got := r.Check([]byte("ck")); got != RateAllow {
		t.Fatalf("fresh cookie must allow, got %v", got)
	}
}

func TestRateLimit_EmptyHashIsAlwaysAllowed(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	if got := r.Check(nil); got != RateAllow {
		t.Fatalf("nil hash must allow, got %v", got)
	}
}

func TestRateLimit_TenConsecutiveFailuresRequireFresh(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	ck := []byte("ck")
	for range MaxConsecutiveFailures {
		r.RecordFailure(ck)
	}
	if got := r.Check(ck); got != RateRequireFreshCookie {
		t.Fatalf("expected RateRequireFreshCookie after %d failures, got %v",
			MaxConsecutiveFailures, got)
	}
}

func TestRateLimit_NinthFailureStillAllows(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	ck := []byte("ck")
	for range MaxConsecutiveFailures - 1 {
		r.RecordFailure(ck)
	}
	if got := r.Check(ck); got != RateAllow {
		t.Fatalf("ninth failure must still allow, got %v", got)
	}
}

func TestRateLimit_SuccessResetsStreak(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	ck := []byte("ck")
	for range MaxConsecutiveFailures - 1 {
		r.RecordFailure(ck)
	}
	r.RecordSuccess(ck)
	for range MaxConsecutiveFailures - 1 {
		r.RecordFailure(ck)
	}
	if got := r.Check(ck); got != RateAllow {
		t.Fatalf("after success+9 fails, must allow, got %v", got)
	}
}

// Per spec.md §6.1, the per-minute quota of 60 means up to and including
// 60 attempts pass per window; only the 61st is rejected. CodeRabbit
// Finding #3 flipped the implementation from `>= 60` to `> 60`; these
// tests pin the new boundary so a future refactor cannot silently regress.
func TestRateLimit_SixtyAttemptsInWindowStillAllow(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	r := newRateLimiter(clock)
	ck := []byte("ck")
	for range MaxAttemptsPerWindow {
		r.RecordAttempt(ck)
	}
	if got := r.Check(ck); got != RateAllow {
		t.Fatalf("expected allow after exactly %d attempts (spec.md §6.1 'up to'), got %v",
			MaxAttemptsPerWindow, got)
	}
}

func TestRateLimit_SixtyOneAttemptsInWindowTrips4429(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	r := newRateLimiter(clock)
	ck := []byte("ck")
	for range MaxAttemptsPerWindow + 1 {
		r.RecordAttempt(ck)
	}
	if got := r.Check(ck); got != RateLimited {
		t.Fatalf("expected RateLimited after %d attempts, got %v",
			MaxAttemptsPerWindow+1, got)
	}
}

func TestRateLimit_FiftyNineAttemptsStillAllowed(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	ck := []byte("ck")
	for range MaxAttemptsPerWindow - 1 {
		r.RecordAttempt(ck)
	}
	if got := r.Check(ck); got != RateAllow {
		t.Fatalf("expected allow at 59 attempts, got %v", got)
	}
}

func TestRateLimit_WindowSlidesAfterMinute(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	r := newRateLimiter(clock)
	ck := []byte("ck")

	for range MaxAttemptsPerWindow + 1 {
		r.RecordAttempt(ck)
	}
	if r.Check(ck) != RateLimited {
		t.Fatal("setup expected RateLimited")
	}

	// Advance just past the window.
	clock.Advance(RateLimitWindow + time.Second)
	if got := r.Check(ck); got != RateAllow {
		t.Fatalf("after window slide, must allow, got %v", got)
	}
}

func TestRateLimit_TenFailuresPrecedesSlidingWindow(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	ck := []byte("ck")
	// 10 failures → consecutive failure mode wins even if attempts under 60.
	for range MaxConsecutiveFailures {
		r.RecordAttempt(ck)
		r.RecordFailure(ck)
	}
	if got := r.Check(ck); got != RateRequireFreshCookie {
		t.Fatalf("RequireFreshCookie must take priority, got %v", got)
	}
}

func TestRateLimit_PerCookieIsolation(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	a := []byte("a")
	b := []byte("b")
	for range MaxConsecutiveFailures {
		r.RecordFailure(a)
	}
	if r.Check(a) != RateRequireFreshCookie {
		t.Fatal("a must be locked")
	}
	if r.Check(b) != RateAllow {
		t.Fatal("b must remain free")
	}
}

func TestRateLimit_DropClearsState(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	ck := []byte("ck")
	for range MaxConsecutiveFailures {
		r.RecordFailure(ck)
	}
	r.Drop(ck)
	if got := r.Check(ck); got != RateAllow {
		t.Fatalf("after Drop must allow, got %v", got)
	}
}

func TestRateLimit_ConcurrentRecordRaceFree(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(clockwork.NewFakeClock())
	ck := []byte("ck")
	var wg sync.WaitGroup
	// Four parallel goroutines exercise every mutex-acquiring path so
	// the race detector can prove RecordFailure / RecordSuccess / Drop
	// are also race-clean (CodeRabbit Nitpick #2).
	wg.Add(4)
	go func() {
		defer wg.Done()
		for range 200 {
			r.RecordAttempt(ck)
			r.RecordSuccess(ck)
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			r.RecordFailure(ck)
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			_ = r.Check(ck)
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			r.Drop(ck)
		}
	}()
	wg.Wait()
}

// TestRateLimit_StaleBucketEvicted verifies that a bucket with no live
// state (zero failures, no recent attempts) is removed once its lastSeen
// ages past rateStateTTL and another stateLocked allocation triggers the
// opportunistic prune. CodeRabbit Finding #2.
func TestRateLimit_StaleBucketEvicted(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	r := newRateLimiter(clock)

	ck := []byte("idle-cookie")
	r.RecordAttempt(ck)
	if got := r.bucketCount(); got != 1 {
		t.Fatalf("setup expected 1 bucket, got %d", got)
	}

	// Age past both the sliding window and the bucket TTL so prune
	// finds zero timestamps + zero streak + lastSeen older than TTL.
	clock.Advance(rateStateTTL + RateLimitWindow + time.Second)

	// Touch a fresh cookie so stateLocked allocates a new bucket and
	// runs pruneStaleLocked over the surviving entries.
	r.RecordAttempt([]byte("fresh-cookie"))

	// The idle bucket should be gone, leaving only the fresh cookie.
	if got := r.bucketCount(); got != 1 {
		t.Fatalf("after prune expected 1 bucket (fresh only), got %d", got)
	}
}
