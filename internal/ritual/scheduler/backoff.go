// Package scheduler — BackoffManager for SPEC-GOOSE-SCHEDULER-001 P3 T-014.
package scheduler

import (
	"fmt"
	"sync"
	"time"
)

// BackoffManager decides whether a ritual trigger should be deferred based on
// recent user activity. It also tracks per-event defer counts to enforce the
// max_defer_count cap (REQ-SCHED-021).
//
// @MX:ANCHOR: [AUTO] Central defer decision point for all ritual triggers
// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-011, REQ-SCHED-021 — fan_in >= 3 (Scheduler, tests, future callers)
type BackoffManager struct {
	clock        interface{ Now() time.Time }
	activity     ActivityClock
	activeWindow time.Duration
	maxDefer     int

	// mu guards deferCounts — map key is "{event}:{userLocalDate}".
	mu          sync.Mutex
	deferCounts map[string]int
}

// newBackoffManager constructs a BackoffManager with the given parameters.
func newBackoffManager(clock interface{ Now() time.Time }, activity ActivityClock, activeWindow time.Duration, maxDefer int) *BackoffManager {
	return &BackoffManager{
		clock:        clock,
		activity:     activity,
		activeWindow: activeWindow,
		maxDefer:     maxDefer,
		deferCounts:  make(map[string]int),
	}
}

// ShouldDefer returns (defer=true, force=false) when the most recent activity is
// within the active window AND the defer count is below maxDefer.
// It returns (defer=false, force=true) when max defers are exhausted — caller
// must force-emit the event.
// It returns (defer=false, force=false) when the window has passed — normal emit.
func (b *BackoffManager) ShouldDefer(eventKey string) (defer_ bool, force bool) {
	now := b.clock.Now()
	last := b.activity.LastActivityAt()

	// Zero time means no activity recorded — never defer.
	if last.IsZero() {
		return false, false
	}

	elapsed := now.Sub(last)
	if elapsed >= b.activeWindow {
		// Activity window has passed — normal emit, reset counter.
		b.Reset(eventKey)
		return false, false
	}

	// Within active window: check defer count.
	b.mu.Lock()
	count := b.deferCounts[eventKey]
	b.mu.Unlock()

	if count >= b.maxDefer {
		// Max defers exhausted — force emit.
		return false, true
	}

	return true, false
}

// RecordDefer increments the defer counter for eventKey.
func (b *BackoffManager) RecordDefer(eventKey string) {
	b.mu.Lock()
	b.deferCounts[eventKey]++
	b.mu.Unlock()
}

// Reset clears the defer counter for eventKey.
func (b *BackoffManager) Reset(eventKey string) {
	b.mu.Lock()
	delete(b.deferCounts, eventKey)
	b.mu.Unlock()
}

// DeferCount returns the current defer count for eventKey (used in tests).
func (b *BackoffManager) DeferCount(eventKey string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.deferCounts[eventKey]
}

// eventDeferKey builds the backoff map key for a given event and local date.
func eventDeferKey(event string, localDate string) string {
	return fmt.Sprintf("%s:%s", event, localDate)
}
