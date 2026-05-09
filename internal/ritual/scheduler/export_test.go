// Package scheduler — test-only exports for white-box testing.
// This file is compiled only during `go test`.
package scheduler

import "context"

// WithCronSpecOverride is the test-exported version of withCronSpecOverride.
// It replaces all ritual cron specs with spec, enabling fast-firing tests
// without waiting for wall-clock HH:MM triggers.
var WithCronSpecOverride = withCronSpecOverride

// RunDailyLearnerForTest invokes the private runDailyLearner method on a
// Scheduler. It exists solely so the AC-SCHED-017 test can drive the learner
// callback synchronously without waiting for the 03:00 cron tick — robfig/cron
// uses real wall time and does not integrate with clockwork.
func RunDailyLearnerForTest(s *Scheduler, ctx context.Context) {
	s.runDailyLearner(ctx)
}
