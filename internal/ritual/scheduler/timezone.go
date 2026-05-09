// Package scheduler — timezone shift detector for SPEC-GOOSE-SCHEDULER-001 P2.
package scheduler

import (
	"math"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"
)

// TimezoneDetector tracks the user's timezone and detects significant UTC-offset
// shifts that warrant a 24-hour scheduling pause.
//
// Shift detection semantics (REQ-SCHED-008):
//   - A shift is flagged only after the baseline has been set for at least 24 hours
//     (i.e., after the process has been running for >= 24 h). Within the first 24 h
//     of process start the detector returns shifted=false (acceptable false negative,
//     see research.md §6.2).
//   - A shift requires |deltaHours| >= 2.0 UTC offset change.
//   - After a flagged shift, ShouldPause() returns true for 24 hours.
//
// @MX:ANCHOR: [AUTO] TimezoneDetector — primary timezone shift detection entry point
// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-008 — fan_in >= 3 (New, Update, ShouldPause, Scheduler.NotifyTimezoneChange)
type TimezoneDetector struct {
	mu            sync.Mutex
	current       *time.Location
	baseline      *time.Location
	baselineSetAt time.Time
	pauseUntil    time.Time
	logger        *zap.Logger
	clock         clockwork.Clock
}

// TimezoneOption is a functional option for TimezoneDetector construction.
type TimezoneOption func(*TimezoneDetector)

// WithTimezoneClock overrides the default real clock with the given clock.
// Primarily used in tests to inject a clockwork.FakeClock.
func WithTimezoneClock(c clockwork.Clock) TimezoneOption {
	return func(d *TimezoneDetector) { d.clock = c }
}

// WithTimezoneLogger overrides the default nop logger.
func WithTimezoneLogger(l *zap.Logger) TimezoneOption {
	return func(d *TimezoneDetector) {
		if l != nil {
			d.logger = l
		}
	}
}

// NewTimezoneDetector constructs a TimezoneDetector with the given initial timezone.
// The baseline is set to initial at construction time.
func NewTimezoneDetector(initial *time.Location, opts ...TimezoneOption) *TimezoneDetector {
	d := &TimezoneDetector{
		current:  initial,
		baseline: initial,
		logger:   zap.NewNop(),
		clock:    clockwork.NewRealClock(),
	}
	// Record the baseline timestamp at construction.
	// Do NOT call d.clock.Now() before applying opts — clock may be overridden.
	for _, opt := range opts {
		opt(d)
	}
	d.baselineSetAt = d.clock.Now()
	return d
}

// Current returns the most recently set timezone location.
func (d *TimezoneDetector) Current() *time.Location {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.current
}

// Update compares loc against the recorded baseline and returns:
//   - shifted=true, deltaHours≠0 if |deltaHours| >= 2.0 AND 24h have elapsed
//     since the baseline was set.
//   - shifted=false, deltaHours=0 otherwise.
//
// When shifted=true, a 24-hour pause window is started and the baseline is
// updated to loc.
func (d *TimezoneDetector) Update(loc *time.Location) (shifted bool, deltaHours float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := d.clock.Now()

	// Compute UTC offset delta in hours between baseline and new timezone.
	_, baselineOffset := now.In(d.baseline).Zone()
	_, newOffset := now.In(loc).Zone()
	deltaSeconds := float64(newOffset - baselineOffset)
	delta := deltaSeconds / 3600.0

	d.current = loc

	// Only flag a shift when:
	//   (a) the baseline has been set for at least 24 h, AND
	//   (b) |delta| >= 2.0 h
	elapsed := now.Sub(d.baselineSetAt)
	if elapsed < 24*time.Hour || math.Abs(delta) < 2.0 {
		return false, 0
	}

	// Record the shift.
	d.logger.Info("timezone_shift_detected",
		zap.String("baseline", d.baseline.String()),
		zap.String("new", loc.String()),
		zap.Float64("delta_hours", delta),
	)

	d.pauseUntil = now.Add(24 * time.Hour)
	d.baseline = loc
	d.baselineSetAt = now
	return true, delta
}

// ShouldPause reports whether the scheduler should suppress ritual event
// emission because a timezone shift was recently detected.
func (d *TimezoneDetector) ShouldPause() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pauseUntil.IsZero() {
		return false
	}
	return d.clock.Now().Before(d.pauseUntil)
}
