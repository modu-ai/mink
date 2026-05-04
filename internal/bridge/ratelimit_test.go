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

func TestRateLimit_SixtyAttemptsInWindowTrips4429(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	r := newRateLimiter(clock)
	ck := []byte("ck")
	for range MaxAttemptsPerWindow {
		r.RecordAttempt(ck)
	}
	if got := r.Check(ck); got != RateLimited {
		t.Fatalf("expected RateLimited after %d attempts, got %v",
			MaxAttemptsPerWindow, got)
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

	for range MaxAttemptsPerWindow {
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
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 200 {
			r.RecordAttempt(ck)
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			_ = r.Check(ck)
		}
	}()
	wg.Wait()
}
