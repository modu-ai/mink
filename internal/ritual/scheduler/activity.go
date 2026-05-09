// Package scheduler — ActivityClock DI seam for BackoffManager.
// SPEC-GOOSE-SCHEDULER-001 P3 T-013.
package scheduler

import "time"

// ActivityClock returns the timestamp of the most recent user activity.
// Concrete implementations are wired in P4 or later (e.g. a QueryEngine adapter).
//
// @MX:NOTE: [AUTO] DI seam — production adapter wired in P4 (QUERY-001 integration)
// @MX:SPEC: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-011
type ActivityClock interface {
	// LastActivityAt returns the wall-clock time of the most recent user activity.
	// Returns zero time when no activity has been recorded.
	LastActivityAt() time.Time
}

// noActivityClock is the zero-value ActivityClock used when no real implementation
// is wired. It returns zero time, so BackoffManager.ShouldDefer always returns false.
type noActivityClock struct{}

func (noActivityClock) LastActivityAt() time.Time { return time.Time{} }
