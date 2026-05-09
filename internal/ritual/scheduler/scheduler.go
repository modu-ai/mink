// Package scheduler — Scheduler struct implementing SPEC-GOOSE-SCHEDULER-001 P1/P2.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jonboulle/clockwork"
	cron "github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/modu-ai/goose/internal/hook"
)

// schedulerState encodes the running/stopped state of the Scheduler.
type schedulerState int32

const (
	// stateStopped indicates the Scheduler is not running.
	stateStopped schedulerState = 0
	// stateRunning indicates the Scheduler is active and dispatching events.
	stateRunning schedulerState = 1
)

// Scheduler drives proactive ritual event emission using robfig/cron.
// All exported methods are safe to call from any goroutine.
//
// @MX:ANCHOR: [AUTO] Central Scheduler struct — Start/Stop/State are entry points for all callers
// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-001 — fan_in >= 3 (New, Start, Stop, State, NotifyTimezoneChange callers)
type Scheduler struct {
	cfg       SchedulerConfig
	cron      *cron.Cron
	dispatch  *hook.Dispatcher
	persister SchedulePersister
	// eventCh is a buffered channel for fired ScheduledEvents.
	// P1: capacity 32. P3 will introduce a dedicated worker that drains it.
	eventCh  chan ScheduledEvent
	state    atomic.Int32
	clock    clockwork.Clock
	logger   *zap.Logger
	location *time.Location
	// cronSpecOverride replaces all ritual cron specs when non-empty.
	// Used only by tests via withCronSpecOverride option.
	cronSpecOverride string

	// P2 fields: timezone detector and holiday calendar (both optional, nil = disabled).
	tzDetector *TimezoneDetector
	holidays   HolidayCalendar
	// tzPauseUntil is an atomic snapshot of TimezoneDetector.pauseUntil for lock-free read.
	// Written only from NotifyTimezoneChange (serialised by caller); read from cron callbacks.
	tzPauseUntil atomic.Value // stores time.Time
}

// Option is a functional option for Scheduler construction.
type Option func(*Scheduler)

// WithClock overrides the default real clock with the given clock.
// Primarily used in tests to inject a clockwork.FakeClock.
func WithClock(c clockwork.Clock) Option {
	return func(s *Scheduler) { s.clock = c }
}

// WithLogger overrides the default nop logger.
func WithLogger(l *zap.Logger) Option {
	return func(s *Scheduler) {
		if l != nil {
			s.logger = l
		}
	}
}

// WithTimezoneDetector wires a TimezoneDetector into the Scheduler.
// When set, NotifyTimezoneChange will consult the detector and potentially
// start a 24-hour suppression window on detected shifts (REQ-SCHED-008).
func WithTimezoneDetector(d *TimezoneDetector) Option {
	return func(s *Scheduler) { s.tzDetector = d }
}

// WithHolidayCalendar wires a HolidayCalendar into the Scheduler.
// When set, the callback populates ScheduledEvent.IsHoliday/HolidayName and
// honours RitualTime.SkipHolidays (REQ-SCHED-018).
func WithHolidayCalendar(c HolidayCalendar) Option {
	return func(s *Scheduler) { s.holidays = c }
}

// withCronSpecOverride replaces all ritual cron specs with the given spec.
// This is intentionally unexported and used only in tests via the export_test.go bridge.
//
// @MX:WARN: [AUTO] test-only hook — must never be called in production paths
// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 P1 — cronSpecOverride bypasses HH:MM validation; callers are test files only
func withCronSpecOverride(spec string) Option {
	return func(s *Scheduler) { s.cronSpecOverride = spec }
}

// New constructs a Scheduler from cfg.
// Returns an error if cfg contains an invalid timezone.
func New(cfg SchedulerConfig, dispatch *hook.Dispatcher, persister SchedulePersister, opts ...Option) (*Scheduler, error) {
	loc, err := cfg.Location()
	if err != nil {
		return nil, err
	}

	s := &Scheduler{
		cfg:       cfg,
		dispatch:  dispatch,
		persister: persister,
		eventCh:   make(chan ScheduledEvent, 32),
		clock:     clockwork.NewRealClock(),
		logger:    zap.NewNop(),
		location:  loc,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Start registers all configured rituals with the underlying cron engine and begins
// dispatching events.
//
// Semantics:
//   - If cfg.Enabled == false: logs "scheduler disabled", state stays Stopped, returns nil.
//   - If cfg has an invalid timezone: state stays Stopped, returns error.
//   - If any ritual clock string fails to parse: stops and returns aggregated error.
//   - On full success: starts cron, sets state=Running, persists cfg (best-effort).
//
// AC-SCHED-011, AC-SCHED-012.
func (s *Scheduler) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		s.logger.Info("scheduler disabled, not starting")
		return nil
	}

	if err := s.cfg.Validate(); err != nil {
		return err
	}

	engine := newCronEngine(s.location, s.logger)
	rituals := ritualTimes(s.cfg.Rituals)

	var errs []error
	for _, rt := range rituals {
		var spec string
		if s.cronSpecOverride != "" {
			// Test-only path: bypass HH:MM parsing and use the override spec directly.
			spec = s.cronSpecOverride
		} else {
			h, m, err := parseClock(rt.Clock)
			if err != nil {
				errs = append(errs, fmt.Errorf("ritual %s: %w", rt.Event, err))
				continue
			}
			spec = clockToCronExpr(h, m)
		}
		// Capture loop variables for the closure.
		ritualCopy := rt
		if _, err := engine.AddFunc(spec, s.makeCallback(ctx, ritualCopy)); err != nil {
			errs = append(errs, fmt.Errorf("ritual %s: cron.AddFunc: %w", rt.Event, err))
		}
	}

	if len(errs) > 0 {
		// Do not start the cron engine — keep state=Stopped.
		return errors.Join(errs...)
	}

	engine.Start()
	s.cron = engine
	s.state.Store(int32(stateRunning))

	// Best-effort persist: log on error but do not fail Start.
	if err := s.persister.Save(ctx, s.cfg); err != nil {
		s.logger.Warn("scheduler persist.Save failed", zap.Error(err))
	}
	return nil
}

// Stop gracefully halts the cron engine and transitions the Scheduler to Stopped.
// Idempotent: calling Stop on an already-stopped Scheduler is a no-op.
func (s *Scheduler) Stop(_ context.Context) error {
	if s.cron != nil {
		stopCtx := s.cron.Stop()
		// Wait for any in-progress job to finish, with a brief real-time deadline.
		select {
		case <-stopCtx.Done():
		case <-time.After(3 * time.Second):
			s.logger.Warn("scheduler stop: timed out waiting for running jobs")
		}
	}
	s.state.Store(int32(stateStopped))
	return nil
}

// makeCallback builds the cron callback closure for a given RitualTime.
func (s *Scheduler) makeCallback(ctx context.Context, rt RitualTime) func() {
	return func() {
		now := s.clock.Now()
		localTime := now.In(s.location)

		// P2: check timezone shift pause window.
		if pauseVal := s.tzPauseUntil.Load(); pauseVal != nil {
			if until, ok := pauseVal.(time.Time); ok && now.Before(until) {
				s.logger.Info("ritual_paused_tz_shift",
					zap.String("event", string(rt.Event)),
					zap.Time("pause_until", until),
				)
				return
			}
		}

		// P2: check weekend skip.
		if rt.SkipWeekends {
			wd := localTime.Weekday()
			if wd == time.Saturday || wd == time.Sunday {
				s.logger.Info("ritual_skipped_weekend",
					zap.String("event", string(rt.Event)),
					zap.String("weekday", wd.String()),
				)
				return
			}
		}

		// P2: check holiday skip.
		var isHoliday bool
		var holidayName string
		if s.holidays != nil {
			info := s.holidays.Lookup(localTime)
			isHoliday = info.IsHoliday
			holidayName = info.Name
			if rt.SkipHolidays && isHoliday {
				s.logger.Info("ritual_skipped_holiday",
					zap.String("event", string(rt.Event)),
					zap.String("holiday", holidayName),
				)
				return
			}
		}

		ev := ScheduledEvent{
			Event:         rt.Event,
			FiredAt:       now,
			ScheduledAt:   now, // P1: no look-ahead scheduling yet
			Timezone:      s.cfg.Timezone,
			UserLocalDate: localTime.Format("2006-01-02"),
			IsHoliday:     isHoliday,
			HolidayName:   holidayName,
		}

		// P1: synchronous DispatchGeneric. P3 will introduce eventCh worker.
		if _, err := s.dispatch.DispatchGeneric(ctx, ev.Event, hook.HookInput{
			HookEvent:  ev.Event,
			CustomData: map[string]any{"scheduled_event": ev},
		}); err != nil {
			s.logger.Error("ritual_dispatch_error",
				zap.String("event", string(ev.Event)),
				zap.Error(err),
			)
		}

		select {
		case s.eventCh <- ev:
		default:
			s.logger.Warn("ritual_eventch_full", zap.String("event", string(ev.Event)))
		}
	}
}

// NotifyTimezoneChange updates the timezone detector with the new location.
// If a significant shift (>= 2h, after 24h baseline) is detected, it:
//   - Sets a 24-hour emit suppression window.
//   - Dispatches hook.EvNotification with shift details (REQ-SCHED-008).
//
// Returns nil in all normal cases (shift detected or not).
func (s *Scheduler) NotifyTimezoneChange(ctx context.Context, newLoc *time.Location) error {
	if s.tzDetector == nil {
		return nil
	}

	oldLoc := s.tzDetector.Current()
	shifted, delta := s.tzDetector.Update(newLoc)
	if !shifted {
		return nil
	}

	now := s.clock.Now()
	pauseUntil := now.Add(24 * time.Hour)
	s.tzPauseUntil.Store(pauseUntil)

	payload := map[string]any{
		"type":        "tz_shift",
		"from":        oldLoc.String(),
		"to":          newLoc.String(),
		"delta_hours": delta,
		"pause_until": pauseUntil.Format(time.RFC3339),
	}

	if _, err := s.dispatch.DispatchGeneric(ctx, hook.EvNotification, hook.HookInput{
		HookEvent:  hook.EvNotification,
		CustomData: payload,
	}); err != nil {
		s.logger.Warn("tz_shift_notification_dispatch_error", zap.Error(err))
	}
	return nil
}

// State returns the current schedulerState as int32.
// 0 == stateStopped, 1 == stateRunning.
func (s *Scheduler) State() int32 {
	return s.state.Load()
}

// Events returns the read-only channel of fired ScheduledEvents.
// The channel is buffered (capacity 32). P3 will introduce a dedicated worker.
// Callers must not close this channel.
func (s *Scheduler) Events() <-chan ScheduledEvent {
	return s.eventCh
}

// ritualTimes converts the configured RitualsConfig into a slice of RitualTime entries
// for all non-empty clock strings.
func ritualTimes(cfg RitualsConfig) []RitualTime {
	var out []RitualTime
	if cfg.Morning.Time != "" {
		out = append(out, RitualTime{
			Event:        hook.EvMorningBriefingTime,
			Clock:        cfg.Morning.Time,
			SkipWeekends: cfg.Morning.SkipWeekends,
		})
	}
	if cfg.Meals.Breakfast.Time != "" {
		out = append(out, RitualTime{
			Event:        hook.EvPostBreakfastTime,
			Clock:        cfg.Meals.Breakfast.Time,
			SkipWeekends: cfg.Meals.Breakfast.SkipWeekends,
		})
	}
	if cfg.Meals.Lunch.Time != "" {
		out = append(out, RitualTime{
			Event:        hook.EvPostLunchTime,
			Clock:        cfg.Meals.Lunch.Time,
			SkipWeekends: cfg.Meals.Lunch.SkipWeekends,
		})
	}
	if cfg.Meals.Dinner.Time != "" {
		out = append(out, RitualTime{
			Event:        hook.EvPostDinnerTime,
			Clock:        cfg.Meals.Dinner.Time,
			SkipWeekends: cfg.Meals.Dinner.SkipWeekends,
		})
	}
	if cfg.Evening.Time != "" {
		out = append(out, RitualTime{
			Event:        hook.EvEveningCheckInTime,
			Clock:        cfg.Evening.Time,
			SkipWeekends: cfg.Evening.SkipWeekends,
		})
	}
	return out
}
