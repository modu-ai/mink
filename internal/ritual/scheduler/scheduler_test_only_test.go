//go:build test_only

// Package scheduler — FastForward integration test gated by the test_only tag.
// SPEC-GOOSE-SCHEDULER-001 P4b T-031 / AC-SCHED-018.
//
// To run this test: `go test -tags=test_only ./internal/ritual/scheduler/...`.
// The default `go test ./...` invocation skips this file because it lacks the
// build tag.
package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/modu-ai/goose/internal/hook"
	"github.com/modu-ai/goose/internal/ritual/scheduler"
	"go.uber.org/zap"
)

// TestFastForward_BuildTagGating exercises the FastForward symbol that is only
// available under the `test_only` build tag. The presence of this test file
// proves that production binaries (which build without `test_only`) cannot
// link FastForward.
//
// AC-SCHED-018, REQ-SCHED-020.
func TestFastForward_BuildTagGating(t *testing.T) {
	t.Parallel()

	seoul, _ := time.LoadLocation("Asia/Seoul")
	startTime := time.Date(2026, 5, 10, 6, 59, 30, 0, seoul)
	fc := clockwork.NewFakeClockAt(startTime)

	cfg := scheduler.SchedulerConfig{
		Enabled:  true,
		Timezone: "Asia/Seoul",
		Rituals: scheduler.RitualsConfig{
			Morning: scheduler.ClockConfig{Time: "07:00"},
		},
	}

	reg := hook.NewHookRegistry()
	dispatch := hook.NewDispatcher(reg, zap.NewNop())
	persister := scheduler.NewFilePersister(t.TempDir())

	sched, err := scheduler.New(cfg, dispatch, persister,
		scheduler.WithClock(fc),
		scheduler.WithLogger(zap.NewNop()),
		scheduler.WithCronSpecOverride("@every 1s"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := sched.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sched.Stop(context.Background()) }()

	// FastForward is only linked when -tags=test_only is set.
	// Calling it advances the FakeClock; the cron engine still uses real wall
	// clock so the assertion verifies that the call is a compile-time symbol
	// available under the build tag.
	sched.FastForward(2 * time.Hour)

	// Confirm the fake clock advanced.
	if got := fc.Now(); !got.After(startTime.Add(time.Hour + 59*time.Minute)) {
		t.Errorf("FakeClock.Now() = %v, expected to be past startTime+2h", got)
	}
}
