// Package scheduler — Scheduler struct implementing SPEC-GOOSE-SCHEDULER-001 P1/P2/P3.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonboulle/clockwork"
	cron "github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/modu-ai/mink/internal/hook"
)

// dispatcherI is the minimal interface used by Scheduler to dispatch hook events.
// Accepting an interface instead of *hook.Dispatcher allows test doubles (e.g. slowDispatcher).
//
// @MX:NOTE: [AUTO] Narrow interface for dispatcher — enables slow-consumer test double injection
type dispatcherI interface {
	DispatchGeneric(ctx context.Context, ev hook.HookEvent, input hook.HookInput) (hook.DispatchResult, error)
}

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
	dispatch  dispatcherI
	persister SchedulePersister
	// eventCh is a buffered channel (cap 32) for fired ScheduledEvents.
	// P3: a dedicated worker goroutine drains workerCh; cron callbacks only enqueue.
	eventCh  chan ScheduledEvent
	state    atomic.Int32
	clock    clockwork.Clock
	logger   *zap.Logger
	location *time.Location
	// cronSpecOverride replaces all ritual cron specs when non-empty.
	// Used only by tests via withCronSpecOverride option.
	cronSpecOverride string

	// mu serialises lifecycle transitions (Start and Stop).
	// It protects the cron pointer and the workerDone channel close operation,
	// preventing concurrent Stop() calls from double-closing workerDone and
	// racing on the cron pointer.
	//
	// @MX:WARN: [AUTO] lifecycle mutex — must not be held across blocking I/O or cron.Stop() timeout
	// @MX:REASON: W1 fix (REVIEW-SCHEDULER-001-2026-05-10): concurrent Stop() caused panic: close of closed channel at scheduler.go:349; cron pointer race between Start and Stop
	mu sync.Mutex

	// P2 fields: timezone detector and holiday calendar (both optional, nil = disabled).
	tzDetector *TimezoneDetector
	holidays   HolidayCalendar
	// tzPauseUntil is an atomic snapshot of TimezoneDetector.pauseUntil for lock-free read.
	// Written only from NotifyTimezoneChange (serialised by caller); read from cron callbacks.
	tzPauseUntil atomic.Value // stores time.Time

	// P3 fields: backoff manager and dispatcher worker lifecycle.
	backoff *BackoffManager
	// workerCh carries ScheduledEvents from cron callbacks to the dispatcher worker.
	// Cap 32 matches eventCh to ensure the cron goroutine never blocks.
	//
	// @MX:WARN: [AUTO] goroutine spawned in Start — lifecycle managed by workerDone/Stop
	// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-015 — dispatcher worker decoupled from cron goroutine
	workerCh   chan ScheduledEvent
	workerDone chan struct{}
	workerWG   sync.WaitGroup

	// nighttimeWarnOnce ensures the nighttime_override WARN log fires at most once.
	nighttimeWarnOnce sync.Once

	// P4a fields: PatternLearner and the source of daily ActivityPatterns.
	// Both are nil when PatternLearner.Enabled == false; in that case the
	// 03:00 daily learner cron entry is not registered.
	patternReader PatternReader
	learner       *PatternLearner

	// P4b field: 3-tuple suppression and missed-event replay store.
	// Defaults to noopFiredKeyStore (no persistence) when WithFiredKeyStore is
	// not used.
	firedKeys FiredKeyStore
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
// When set, the callback populates ScheduledEvent.IsHoliday and
// HolidayName (canonical English key, see KoreanHolidayName helper) and
// honours RitualTime.SkipHolidays (REQ-SCHED-018).
func WithHolidayCalendar(c HolidayCalendar) Option {
	return func(s *Scheduler) { s.holidays = c }
}

// WithActivityClock injects an ActivityClock into the Scheduler for backoff decisions.
// If not provided, backoff is effectively disabled (noActivityClock returns zero time).
// (REQ-SCHED-011, REQ-SCHED-021)
func WithActivityClock(a ActivityClock) Option {
	return func(s *Scheduler) {
		if s.backoff != nil {
			s.backoff.activity = a
		}
	}
}

// WithPatternReader wires a PatternReader into the Scheduler. When set together
// with cfg.PatternLearner.Enabled == true, Start registers a 03:00-local daily
// cron entry that runs PatternLearner against the latest ActivityPattern and
// dispatches a RitualTimeProposal Notification when drift exceeds the
// threshold. (REQ-SCHED-006, REQ-SCHED-019)
func WithPatternReader(p PatternReader) Option {
	return func(s *Scheduler) { s.patternReader = p }
}

// WithFiredKeyStore wires a FiredKeyStore into the Scheduler. When set, every
// dispatched ritual is recorded with its 3-tuple key (REQ-SCHED-013), and
// Start replays missed events whose schedule fell within the configured
// MissedEventReplayMaxDelay (REQ-SCHED-022).
func WithFiredKeyStore(store FiredKeyStore) Option {
	return func(s *Scheduler) { s.firedKeys = store }
}

// withCronSpecOverride replaces all ritual cron specs with the given spec.
// This is intentionally unexported and used only in tests via the export_test.go bridge.
//
// @MX:WARN: [AUTO] test-only hook — must never be called in production paths
// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 P1 — cronSpecOverride bypasses HH:MM validation; callers are test files only
func withCronSpecOverride(spec string) Option {
	return func(s *Scheduler) { s.cronSpecOverride = spec }
}

// New constructs a Scheduler from cfg using the concrete *hook.Dispatcher.
// Returns an error if cfg contains an invalid timezone.
func New(cfg SchedulerConfig, dispatch *hook.Dispatcher, persister SchedulePersister, opts ...Option) (*Scheduler, error) {
	return NewWithDispatcher(cfg, dispatch, persister, opts...)
}

// NewWithDispatcher constructs a Scheduler using the dispatcherI interface.
// This allows test doubles (e.g. slow-consumer wrappers) to be injected.
func NewWithDispatcher(cfg SchedulerConfig, dispatch dispatcherI, persister SchedulePersister, opts ...Option) (*Scheduler, error) {
	loc, err := cfg.Location()
	if err != nil {
		return nil, err
	}

	bCfg := cfg.Backoff.effectiveBackoff()
	realClock := clockwork.NewRealClock()
	bm := newBackoffManager(realClock, noActivityClock{}, bCfg.ActiveWindow, bCfg.MaxDeferCount)

	s := &Scheduler{
		cfg:        cfg,
		dispatch:   dispatch,
		persister:  persister,
		eventCh:    make(chan ScheduledEvent, 32),
		workerCh:   make(chan ScheduledEvent, 32),
		workerDone: make(chan struct{}),
		clock:      realClock,
		logger:     zap.NewNop(),
		location:   loc,
		backoff:    bm,
	}
	// Initialise the PatternLearner whenever a learner config is supplied; the
	// 03:00 cron registration in Start gates on Enabled+PatternReader.
	s.learner = NewPatternLearner(cfg.PatternLearner)
	// Default fired-key store is a no-op; WithFiredKeyStore overrides.
	s.firedKeys = noopFiredKeyStore{}

	for _, opt := range opts {
		opt(s)
	}
	// Sync backoff clock to the potentially overridden s.clock.
	s.backoff.clock = s.clock
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
		if _, err := engine.AddFunc(spec, s.makeCallback(ritualCopy)); err != nil {
			errs = append(errs, fmt.Errorf("ritual %s: cron.AddFunc: %w", rt.Event, err))
		}
	}

	if len(errs) > 0 {
		// Do not start the cron engine — keep state=Stopped.
		return errors.Join(errs...)
	}

	// P4a: register the 03:00-local daily learner cron when wired and enabled.
	if s.cfg.PatternLearner.Enabled && s.patternReader != nil {
		if _, err := engine.AddFunc("0 3 * * *", func() {
			s.runDailyLearner(ctx)
		}); err != nil {
			return fmt.Errorf("daily learner cron: %w", err)
		}
	}

	// P3: start dispatcher worker before the cron engine so it is ready to drain.
	s.workerWG.Add(1)
	go s.runWorker(ctx)

	engine.Start()
	s.cron = engine
	s.state.Store(int32(stateRunning))

	// P4b: replay missed events whose scheduled time falls within
	// MissedEventReplayMaxDelay of now. Runs synchronously on the caller's
	// goroutine; events are pushed onto workerCh just like normal cron fires.
	s.replayMissedEvents(rituals)

	// Best-effort persist: log on error but do not fail Start.
	if err := s.persister.Save(ctx, s.cfg); err != nil {
		s.logger.Warn("scheduler persist.Save failed", zap.Error(err))
	}
	return nil
}

// runWorker is the P3 dispatcher worker goroutine.
// It drains workerCh and calls the dispatcher, then forwards the event to
// the consumer-facing eventCh. Runs until workerDone is closed.
//
// @MX:WARN: [AUTO] long-running goroutine — stopped by close(workerDone) in Stop
// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-015 — must not block cron goroutine
func (s *Scheduler) runWorker(ctx context.Context) {
	defer s.workerWG.Done()
	for {
		select {
		case ev, ok := <-s.workerCh:
			if !ok {
				return
			}
			s.workerDispatch(ctx, ev)
		case <-s.workerDone:
			// Drain remaining items before exiting (graceful 3s handled by Stop).
			for {
				select {
				case ev := <-s.workerCh:
					s.workerDispatch(ctx, ev)
				default:
					return
				}
			}
		}
	}
}

// workerDispatch calls the dispatcher and forwards the event to eventCh.
func (s *Scheduler) workerDispatch(ctx context.Context, ev ScheduledEvent) {
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

// Stop gracefully halts the cron engine and transitions the Scheduler to Stopped.
// Idempotent: calling Stop on an already-stopped Scheduler is a no-op.
// Concurrent calls are serialised by mu; only the first caller performs the
// actual shutdown; subsequent callers return immediately once state is Stopped.
func (s *Scheduler) Stop(_ context.Context) error {
	s.mu.Lock()
	// Idempotent: if already stopped, return immediately.
	if s.state.Load() != int32(stateRunning) {
		s.mu.Unlock()
		return nil
	}
	// Transition to Stopped under the lock so concurrent callers see the new
	// state before mu is released and short-circuit above.
	s.state.Store(int32(stateStopped))
	// Capture cron under the lock, then release before blocking operations.
	cronEngine := s.cron
	s.cron = nil
	s.mu.Unlock()

	// Stop the cron engine outside the lock — cron.Stop() may block up to 3s
	// waiting for in-progress jobs to finish.
	if cronEngine != nil {
		stopCtx := cronEngine.Stop()
		// Wait for any in-progress job to finish, with a brief real-time deadline.
		select {
		case <-stopCtx.Done():
		case <-time.After(3 * time.Second):
			s.logger.Warn("scheduler stop: timed out waiting for running jobs")
		}
	}

	// Signal the worker goroutine to finish and wait up to 3s.
	// close(workerDone) is safe here: mu guarantees only one goroutine reaches
	// this point (state guard above prevents re-entry).
	if s.workerDone != nil {
		close(s.workerDone)
		done := make(chan struct{})
		go func() {
			s.workerWG.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			s.logger.Warn("scheduler stop: timed out waiting for worker goroutine")
		}
	}

	return nil
}

// evaluateSkipConditions performs the read-only skip checks for a single
// cron tick: timezone pause window, weekend skip, and holiday skip.
// Returns skip=true when the tick should be discarded silently (with the
// appropriate Info log already emitted), and the holiday info for downstream
// event construction otherwise.
//
// Stages 1-3 of makeCallback per REVIEW-SCHEDULER-001-2026-05-10 W3.
func (s *Scheduler) evaluateSkipConditions(rt RitualTime, now, localTime time.Time) (skip bool, isHoliday bool, holidayName string) {
	// P2: TZ pause window.
	if pauseVal := s.tzPauseUntil.Load(); pauseVal != nil {
		if until, ok := pauseVal.(time.Time); ok && now.Before(until) {
			s.logger.Info("ritual_paused_tz_shift",
				zap.String("event", string(rt.Event)),
				zap.Time("pause_until", until),
			)
			return true, false, ""
		}
	}

	// P2: Weekend skip.
	if rt.SkipWeekends {
		wd := localTime.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			s.logger.Info("ritual_skipped_weekend",
				zap.String("event", string(rt.Event)),
				zap.String("weekday", wd.String()),
			)
			return true, false, ""
		}
	}

	// P2: Holiday skip + info passthrough.
	if s.holidays != nil {
		info := s.holidays.Lookup(localTime)
		isHoliday = info.IsHoliday
		holidayName = info.Name
		if rt.SkipHolidays && isHoliday {
			s.logger.Info("ritual_skipped_holiday",
				zap.String("event", string(rt.Event)),
				zap.String("holiday", holidayName),
			)
			return true, isHoliday, holidayName
		}
	}
	return false, isHoliday, holidayName
}

// dispatchScheduledEvent constructs the ScheduledEvent, performs duplicate
// suppression check (3-tuple, TZ-aware), records the fired key, and enqueues
// onto the worker channel. When the firedKey is already present, emits a
// suppressed fire-log entry and returns without enqueueing.
//
// Stages 6-8 of makeCallback per REVIEW-SCHEDULER-001-2026-05-10 W3.
func (s *Scheduler) dispatchScheduledEvent(
	rt RitualTime,
	now time.Time,
	localDate, tz string,
	isHoliday bool,
	holidayName string,
	backoffApplied bool,
	delayHint time.Duration,
) {
	firedKey := BuildFiredKey(string(rt.Event), localDate, tz)
	if s.firedKeys != nil && s.firedKeys.Has(firedKey) {
		ev := ScheduledEvent{
			Event:         rt.Event,
			FiredAt:       now,
			ScheduledAt:   now,
			Timezone:      tz,
			UserLocalDate: localDate,
			IsHoliday:     isHoliday,
			HolidayName:   holidayName,
		}
		EmitFireLog(s.logger, ev, true, "duplicate_suppressed")
		return
	}

	ev := ScheduledEvent{
		Event:          rt.Event,
		FiredAt:        now,
		ScheduledAt:    now,
		Timezone:       tz,
		UserLocalDate:  localDate,
		IsHoliday:      isHoliday,
		HolidayName:    holidayName,
		BackoffApplied: backoffApplied,
		DelayHint:      delayHint,
	}

	// P4b: emit canonical fire-log entry (REQ-SCHED-004 schema).
	EmitFireLog(s.logger, ev, false, "")

	// P4b: record the fired key before enqueueing so a duplicate cron tick
	// during the same minute is suppressed.
	if s.firedKeys != nil {
		if err := s.firedKeys.Mark(firedKey, now); err != nil {
			s.logger.Warn("firedkeys_mark_error", zap.Error(err))
		}
	}

	// P3: enqueue onto workerCh (non-blocking). Worker dispatches asynchronously.
	select {
	case s.workerCh <- ev:
	default:
		s.logger.Warn("ritual_workerch_full", zap.String("event", string(ev.Event)))
	}
}

// makeCallback builds the cron callback closure for a given RitualTime.
// P3: cron callbacks only enqueue events onto workerCh; dispatch runs in the worker.
//
// @MX:WARN: [AUTO] Cron callback closure orchestrates 8 lifecycle stages
//
//	(TZ pause / weekend / holiday / nighttime warn / backoff / suppression /
//	event build / mark+enqueue). Skip checks (1-3) and dispatch (6-8) are
//	extracted to helpers; nighttime warn (4) and backoff (5) remain inline.
//
// @MX:REASON: Centralized lifecycle preserves atomic per-tick semantics —
//
//	further extraction would scatter early-return guards and break the
//	single-shot defer/force-emit invariant.
//
// @MX:SPEC: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-013, REQ-SCHED-021
func (s *Scheduler) makeCallback(rt RitualTime) func() {
	return func() {
		now := s.clock.Now()
		localTime := now.In(s.location)

		skip, isHoliday, holidayName := s.evaluateSkipConditions(rt, now, localTime)
		if skip {
			return
		}

		// P3: nighttime override WARN log (fires at most once per Scheduler instance).
		if s.cfg.AllowNighttime {
			s.nighttimeWarnOnce.Do(func() {
				s.logger.Warn("ritual_nighttime_override",
					zap.String("event", string(rt.Event)),
					zap.Bool("nighttime_override", true),
				)
			})
		}

		// P3: backoff check.
		localDate := localTime.Format("2006-01-02")
		deferKey := eventDeferKey(string(rt.Event), localDate)
		var backoffApplied bool
		var delayHint time.Duration

		if s.backoff != nil {
			shouldDefer, forceEmit := s.backoff.ShouldDefer(deferKey)

			if shouldDefer {
				// Defer: record and return without emitting.
				s.backoff.RecordDefer(deferKey)
				s.logger.Info("ritual_backoff_deferred",
					zap.String("event", string(rt.Event)),
					zap.Bool("backoff_applied", true),
				)
				return
			}

			if forceEmit {
				// Force emit after max defers.
				count := s.backoff.DeferCount(deferKey)
				delayHint = time.Duration(count) * s.backoff.activeWindow
				backoffApplied = true
				s.logger.Warn("ritual_force_emit",
					zap.String("event", string(rt.Event)),
					zap.Bool("force_emit", true),
					zap.Int("defer_count", count),
				)
				s.backoff.Reset(deferKey)
			}
		}

		tz := s.cfg.effectiveTimezone()
		s.dispatchScheduledEvent(rt, now, localDate, tz, isHoliday, holidayName, backoffApplied, delayHint)
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

// replayMissedEvents inspects each configured ritual and emits a one-shot
// replay event for any whose scheduled local time today already passed and
// whose 3-tuple suppression key is unset. The replay window is capped at
// cfg.MissedEventReplayMaxDelay (default 1 hour, REQ-SCHED-022).
//
// Replays carry IsReplay=true and DelayMinutes=<actual>, so downstream
// consumers can adapt tone (e.g. BRIEFING-001 may say "you missed your
// morning briefing 30 minutes ago" instead of "good morning").
func (s *Scheduler) replayMissedEvents(rituals []RitualTime) {
	if s.firedKeys == nil {
		return
	}
	now := s.clock.Now()
	maxDelay := s.cfg.effectiveMissedReplayDelay()
	tz := s.cfg.effectiveTimezone()

	for _, rt := range rituals {
		if rt.Clock == "" {
			continue
		}
		h, m, err := parseClock(rt.Clock)
		if err != nil {
			continue
		}
		// Today's scheduled local time.
		localNow := now.In(s.location)
		scheduledLocal := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), h, m, 0, 0, s.location)
		if !localNow.After(scheduledLocal) {
			// Schedule has not happened yet today.
			continue
		}
		key := BuildFiredKey(string(rt.Event), scheduledLocal.Format("2006-01-02"), tz)
		if s.firedKeys.Has(key) {
			// Already fired today.
			continue
		}
		delta := localNow.Sub(scheduledLocal)
		ev := ScheduledEvent{
			Event:         rt.Event,
			FiredAt:       now,
			ScheduledAt:   scheduledLocal,
			Timezone:      tz,
			UserLocalDate: scheduledLocal.Format("2006-01-02"),
			IsReplay:      true,
			DelayMinutes:  int(delta.Minutes()),
		}
		if delta > maxDelay {
			// Too stale — log skip and move on.
			EmitFireLog(s.logger, ev, true, "missed_event_too_stale")
			continue
		}
		// Within the replay window: enqueue exactly once.
		select {
		case s.workerCh <- ev:
		default:
			s.logger.Warn("ritual_replay_workerch_full", zap.String("event", string(ev.Event)))
		}
		// Record the replay so later callbacks do not re-fire it today.
		if err := s.firedKeys.Mark(key, now); err != nil {
			s.logger.Warn("firedkeys_mark_error", zap.Error(err))
		}
	}
}

// runDailyLearner is the 03:00-local cron callback. It reads the current
// ActivityPattern, runs the learner against each configured ritual clock, and
// dispatches a RitualTimeProposal Notification for every kind whose drift
// exceeds the threshold. The persisted config is never mutated; user
// confirmation is required before commit (REQ-SCHED-019).
//
// @MX:NOTE: [AUTO] 03:00-local daily learner — runs only when PatternReader and PatternLearner.Enabled are set
// @MX:SPEC: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-006, REQ-SCHED-019
func (s *Scheduler) runDailyLearner(ctx context.Context) {
	if s.patternReader == nil || s.learner == nil {
		return
	}
	pat, err := s.patternReader.ReadActivityPattern(ctx)
	if err != nil {
		s.logger.Warn("daily_learner_read_error", zap.Error(err))
		return
	}

	type kindClock struct {
		kind  RitualKind
		clock string
	}
	pairs := []kindClock{
		{KindMorning, s.cfg.Rituals.Morning.Time},
		{KindBreakfast, s.cfg.Rituals.Meals.Breakfast.Time},
		{KindLunch, s.cfg.Rituals.Meals.Lunch.Time},
		{KindDinner, s.cfg.Rituals.Meals.Dinner.Time},
		{KindEvening, s.cfg.Rituals.Evening.Time},
	}

	for _, p := range pairs {
		if p.clock == "" {
			continue
		}
		proposal, err := s.learner.Observe(p.kind, p.clock, pat)
		if err != nil {
			s.logger.Warn("daily_learner_observe_error",
				zap.String("kind", p.kind.String()),
				zap.Error(err),
			)
			continue
		}
		if proposal == nil {
			continue
		}

		payload := map[string]any{
			"kind":             "RitualTimeProposal",
			"confirm_required": true,
			"proposal":         proposal,
		}
		if _, err := s.dispatch.DispatchGeneric(ctx, hook.EvNotification, hook.HookInput{
			HookEvent:  hook.EvNotification,
			CustomData: payload,
		}); err != nil {
			s.logger.Warn("daily_learner_dispatch_error",
				zap.String("kind", p.kind.String()),
				zap.Error(err),
			)
		}
	}
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
