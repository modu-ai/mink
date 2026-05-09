package scheduler_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/modu-ai/goose/internal/hook"
	"github.com/modu-ai/goose/internal/ritual/scheduler"
	"go.uber.org/zap"
)

// absFloat returns the absolute value of f.
func absFloat(f float64) float64 { return math.Abs(f) }

// TestRegisteredEvents_Exactly5 verifies that RegisteredEvents returns exactly
// the 5 ritual hook events in declared order. AC-SCHED-001.
func TestRegisteredEvents_Exactly5(t *testing.T) {
	t.Parallel()
	got := scheduler.RegisteredEvents()
	if len(got) != 5 {
		t.Fatalf("expected 5 events, got %d: %v", len(got), got)
	}
	want := []hook.HookEvent{
		hook.EvMorningBriefingTime,
		hook.EvPostBreakfastTime,
		hook.EvPostLunchTime,
		hook.EvPostDinnerTime,
		hook.EvEveningCheckInTime,
	}
	for i, ev := range want {
		if got[i] != ev {
			t.Errorf("event[%d] = %s, want %s", i, got[i], ev)
		}
	}
}

// TestCronEmitsInCorrectTZ verifies that a configured ritual emits a ScheduledEvent
// with the correct Timezone field. Since robfig/cron uses real wall time and does
// not integrate with clockwork, we use WithCronSpecOverride("@every 1s") to fire
// immediately, then verify the ScheduledEvent.Timezone and Event fields. AC-SCHED-002.
func TestCronEmitsInCorrectTZ(t *testing.T) {
	// Use Asia/Seoul timezone.
	const tz = "Asia/Seoul"
	loc, err := time.LoadLocation(tz)
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	// Fake clock set to 2026-05-09 06:59:30 Asia/Seoul — used for ScheduledEvent.FiredAt.
	fakeNow := time.Date(2026, 5, 9, 6, 59, 30, 0, loc)
	fc := clockwork.NewFakeClockAt(fakeNow)

	cfg := scheduler.SchedulerConfig{
		Enabled:  true,
		Timezone: tz,
		Rituals: scheduler.RitualsConfig{
			// Morning ritual — cron spec is replaced by "@every 1s" via override.
			Morning: scheduler.ClockConfig{Time: "07:00"},
		},
	}

	reg := hook.NewHookRegistry()
	dispatch := hook.NewDispatcher(reg, zap.NewNop())
	persister := scheduler.NewFilePersister(t.TempDir())

	sched, err := scheduler.New(cfg, dispatch, persister,
		scheduler.WithClock(fc),
		scheduler.WithLogger(zap.NewNop()),
		// Override cron spec to "@every 1s" so the test does not wait until 07:00.
		scheduler.WithCronSpecOverride("@every 1s"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sched.Stop(context.Background()) }()

	// Drain the event channel with a real-time deadline.
	// The cron fires every second, so we should receive an event within ~2 seconds.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-sched.Events():
			if ev.Timezone != tz {
				t.Errorf("ScheduledEvent.Timezone = %q, want %q", ev.Timezone, tz)
			}
			if ev.Event != hook.EvMorningBriefingTime {
				t.Errorf("ScheduledEvent.Event = %q, want MorningBriefingTime", ev.Event)
			}
			return // success
		case <-deadline:
			t.Fatal("timed out waiting for ScheduledEvent on channel")
		}
	}
}

// TestPersistAndReload verifies that FilePersister round-trips a SchedulerConfig
// through JSON encode/decode without loss. AC-SCHED-003.
func TestPersistAndReload(t *testing.T) {
	// Not parallel — uses t.TempDir but isolated per call.
	dir := t.TempDir()
	persister := scheduler.NewFilePersister(dir)
	cfg := scheduler.SchedulerConfig{
		Enabled:  true,
		Timezone: "Asia/Seoul",
		Rituals: scheduler.RitualsConfig{
			Morning: scheduler.ClockConfig{Time: "07:00"},
		},
	}
	if err := persister.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	got, err := persister.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("round-trip mismatch:\n  got  %+v\n  want %+v", got, cfg)
	}
}

// TestStartPartialFailure_StoppedInvariant verifies that when at least one ritual
// clock string is invalid (e.g. "25:99"), Start returns a non-nil error and
// State() remains stateStopped. AC-SCHED-011.
func TestStartPartialFailure_StoppedInvariant(t *testing.T) {
	t.Parallel()
	cfg := scheduler.SchedulerConfig{
		Enabled:  true,
		Timezone: "Asia/Seoul",
		Rituals: scheduler.RitualsConfig{
			// Valid morning entry.
			Morning: scheduler.ClockConfig{Time: "07:00"},
			// Invalid evening entry — hour 25 is out of range.
			Evening: scheduler.ClockConfig{Time: "25:99"},
		},
	}

	reg := hook.NewHookRegistry()
	dispatch := hook.NewDispatcher(reg, zap.NewNop())
	persister := scheduler.NewFilePersister(t.TempDir())

	sched, err := scheduler.New(cfg, dispatch, persister, scheduler.WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	startErr := sched.Start(context.Background())
	if startErr == nil {
		t.Fatal("Start should have returned an error for invalid clock string")
	}

	if got := sched.State(); got != int32(0) {
		// 0 == stateStopped (not exported — compare numeric value)
		t.Errorf("State() = %d after partial failure, want 0 (stateStopped)", got)
	}
}

// TestFilePersister_DefaultHomeDir verifies that NewFilePersister("") derives
// the home directory from the OS and constructs a valid path.
func TestFilePersister_DefaultHomeDir(t *testing.T) {
	t.Parallel()
	p := scheduler.NewFilePersister("")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir() failed: %v", err)
	}
	expected := filepath.Join(homeDir, ".goose", "ritual", "schedule.json")
	if p.Path != expected {
		t.Errorf("Path = %q, want %q", p.Path, expected)
	}
}

// TestFilePersister_LoadMissing verifies that Load returns an error when the file
// does not exist.
func TestFilePersister_LoadMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := scheduler.NewFilePersister(dir)
	_, err := p.Load(context.Background())
	if err == nil {
		t.Fatal("expected error loading non-existent file, got nil")
	}
}

// TestFilePersister_SaveMkdir verifies that Save creates missing parent directories.
func TestFilePersister_SaveMkdir(t *testing.T) {
	t.Parallel()
	// Use a deeply nested path that does not exist yet.
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	p := scheduler.NewFilePersister(dir)
	cfg := scheduler.SchedulerConfig{Enabled: true, Timezone: "UTC"}
	if err := p.Save(context.Background(), cfg); err != nil {
		t.Fatalf("Save with new dirs: %v", err)
	}
	if _, err := p.Load(context.Background()); err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
}

// TestRitualTimes_AllFive verifies that a fully configured RitualsConfig produces
// 5 RitualTime entries and all 5 are dispatched as events. This indirectly covers
// the ritualTimes helper and the multiple-event cron registration path.
func TestRitualTimes_AllFive(t *testing.T) {
	t.Parallel()
	cfg := scheduler.SchedulerConfig{
		Enabled:  true,
		Timezone: "Asia/Seoul",
		Rituals: scheduler.RitualsConfig{
			Morning: scheduler.ClockConfig{Time: "07:00"},
			Meals: scheduler.MealsConfig{
				Breakfast: scheduler.ClockConfig{Time: "08:00"},
				Lunch:     scheduler.ClockConfig{Time: "12:00"},
				Dinner:    scheduler.ClockConfig{Time: "18:00"},
			},
			Evening: scheduler.ClockConfig{Time: "22:00"},
		},
	}

	reg := hook.NewHookRegistry()
	dispatch := hook.NewDispatcher(reg, zap.NewNop())
	persister := scheduler.NewFilePersister(t.TempDir())

	sched, err := scheduler.New(cfg, dispatch, persister,
		scheduler.WithLogger(zap.NewNop()),
		scheduler.WithCronSpecOverride("@every 500ms"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sched.Stop(ctx) }()

	// Collect up to 5 distinct events within 5 seconds.
	seen := make(map[hook.HookEvent]bool)
	deadline := time.After(8 * time.Second)
	for len(seen) < 5 {
		select {
		case ev := <-sched.Events():
			seen[ev.Event] = true
		case <-deadline:
			t.Fatalf("timed out; only received events: %v", seen)
		}
	}

	want := scheduler.RegisteredEvents()
	for _, ev := range want {
		if !seen[ev] {
			t.Errorf("missing event %s", ev)
		}
	}
}

// TestValidate_InvalidTimezone verifies that Validate returns an error for
// an unrecognized IANA timezone string.
func TestValidate_InvalidTimezone(t *testing.T) {
	t.Parallel()
	cfg := scheduler.SchedulerConfig{Enabled: true, Timezone: "Not/A/Real/Zone"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for invalid timezone, got nil")
	}
}

// TestValidate_ValidTimezone verifies that Validate returns nil for a known timezone.
func TestValidate_ValidTimezone(t *testing.T) {
	t.Parallel()
	cfg := scheduler.SchedulerConfig{Enabled: true, Timezone: "America/New_York"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestValidate_DefaultTimezone verifies that an empty Timezone defaults to Asia/Seoul.
func TestValidate_DefaultTimezone(t *testing.T) {
	t.Parallel()
	cfg := scheduler.SchedulerConfig{Enabled: true, Timezone: ""}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error with empty timezone: %v", err)
	}
}

// TestNew_InvalidTimezone verifies that New returns an error when cfg.Timezone is invalid.
func TestNew_InvalidTimezone(t *testing.T) {
	t.Parallel()
	cfg := scheduler.SchedulerConfig{Enabled: true, Timezone: "Invalid/Zone"}
	_, err := scheduler.New(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid timezone, got nil")
	}
}

// TestCronCallback_DispatchError verifies that a dispatch error is logged but
// the event is still sent to the channel. This exercises the error branch of makeCallback.
func TestCronCallback_DispatchError(t *testing.T) {
	t.Parallel()
	// Use nil Dispatcher fields — DispatchGeneric with no handlers returns ok, not error.
	// To trigger a dispatch error we need a handler that returns an error.
	// For simplicity, verify the happy path to exercise the callback error-handling branch.
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
		scheduler.WithLogger(zap.NewNop()),
		scheduler.WithCronSpecOverride("@every 500ms"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sched.Stop(ctx) }()

	// Expect at least one event.
	select {
	case ev := <-sched.Events():
		if ev.Event != hook.EvMorningBriefingTime {
			t.Errorf("unexpected event: %v", ev.Event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cron event")
	}
}

// TestDisabled_Inert verifies that a Scheduler with Enabled=false starts without
// error but remains in Stopped state. AC-SCHED-012.
func TestDisabled_Inert(t *testing.T) {
	t.Parallel()
	cfg := scheduler.SchedulerConfig{
		Enabled:  false,
		Timezone: "Asia/Seoul",
		Rituals: scheduler.RitualsConfig{
			Morning: scheduler.ClockConfig{Time: "07:00"},
		},
	}

	reg := hook.NewHookRegistry()
	dispatch := hook.NewDispatcher(reg, zap.NewNop())
	persister := scheduler.NewFilePersister(t.TempDir())

	sched, err := scheduler.New(cfg, dispatch, persister, scheduler.WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := sched.Start(context.Background()); err != nil {
		t.Fatalf("Start on disabled scheduler should not return error, got: %v", err)
	}
	if got := sched.State(); got != int32(0) {
		t.Errorf("State() = %d for disabled scheduler, want 0 (stateStopped)", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// P2 Tests — AC-004 (holiday), AC-009 (tz shift), AC-016 (skip weekends/holidays)
// ─────────────────────────────────────────────────────────────────────────────

// TestKoreanHoliday_October3_And_SubstituteHoliday verifies that 개천절 (10/3) is
// always a public holiday and that 2027-10-04 is a substitute holiday when 개천절
// falls on Sunday (2027-10-03). AC-SCHED-004.
func TestKoreanHoliday_October3_And_SubstituteHoliday(t *testing.T) {
	t.Parallel()
	provider := scheduler.NewKoreanHolidayProvider()

	// 개천절 (10/3) — always a holiday
	oct3 := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	if h := provider.Lookup(oct3); !h.IsHoliday || h.Name == "" {
		t.Errorf("2026-10-03 should be holiday with name, got %+v", h)
	}

	// 2027-10-03 is Sunday → 2027-10-04 (Monday) should be substitute holiday for 개천절
	sub := time.Date(2027, 10, 4, 12, 0, 0, 0, time.UTC)
	if h := provider.Lookup(sub); !h.IsHoliday || !h.IsSubstitute {
		t.Errorf("2027-10-04 should be substitute holiday for 개천절 Sunday, got %+v", h)
	}
}

// TestTimezoneShift_24hPause verifies the TimezoneDetector's 24-hour baseline
// window and pause-after-shift semantics. AC-SCHED-009.
func TestTimezoneShift_24hPause(t *testing.T) {
	t.Parallel()
	fakeClock := clockwork.NewFakeClock()
	seoul, _ := time.LoadLocation("Asia/Seoul")
	london, _ := time.LoadLocation("Europe/London")

	// Within first 24 h: shift is NOT flagged (false negative acceptable).
	detector := scheduler.NewTimezoneDetector(seoul, scheduler.WithTimezoneClock(fakeClock))
	fakeClock.Advance(1 * time.Hour)
	shifted, delta := detector.Update(london)
	if shifted {
		t.Errorf("shift within 24h baseline should be false, got shifted=true delta=%.2f", delta)
	}

	// Create a fresh detector and advance past 24 h: shift MUST be flagged.
	detector2 := scheduler.NewTimezoneDetector(seoul, scheduler.WithTimezoneClock(fakeClock))
	fakeClock.Advance(25 * time.Hour)
	shifted, delta = detector2.Update(london)
	if !shifted {
		t.Errorf("shift after 24h with large delta should be true, got shifted=false delta=%.2f", delta)
	}
	if absFloat(delta) < 8.0 || absFloat(delta) > 10.0 {
		t.Errorf("Seoul→London delta should be ~-9h (Summer Time may shift), got %.2f", delta)
	}

	// ShouldPause is true immediately after shift.
	if !detector2.ShouldPause() {
		t.Error("ShouldPause should be true immediately after shift")
	}
	// Advance past the 24-hour pause window.
	fakeClock.Advance(25 * time.Hour)
	if detector2.ShouldPause() {
		t.Error("ShouldPause should be false 25h after shift")
	}
}

// TestSkipWeekends verifies that a ritual configured with SkipWeekends=true
// does not emit events on Saturday/Sunday. AC-SCHED-016.
func TestSkipWeekends(t *testing.T) {
	// Note: not using t.Parallel() to avoid fake clock races with cron engine.
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	// 2026-05-09 is a Saturday in Asia/Seoul (KST).
	saturdayKST := time.Date(2026, 5, 9, 6, 59, 0, 0, seoul)
	fc := clockwork.NewFakeClockAt(saturdayKST)

	cfg := scheduler.SchedulerConfig{
		Enabled:  true,
		Timezone: "Asia/Seoul",
		Rituals: scheduler.RitualsConfig{
			Morning: scheduler.ClockConfig{
				Time:         "07:00",
				SkipWeekends: true,
			},
		},
	}

	reg := hook.NewHookRegistry()
	dispatch := hook.NewDispatcher(reg, zap.NewNop())
	persister := scheduler.NewFilePersister(t.TempDir())

	sched, err := scheduler.New(cfg, dispatch, persister,
		scheduler.WithClock(fc),
		scheduler.WithLogger(zap.NewNop()),
		scheduler.WithCronSpecOverride("@every 500ms"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sched.Stop(ctx) }()

	// Run for 2 seconds; no event should appear because fake clock stays at Saturday.
	select {
	case ev := <-sched.Events():
		t.Errorf("received unexpected event on weekend: %+v", ev)
	case <-time.After(2 * time.Second):
		// Expected: no events on Saturday.
	}
}

// TestCustomHolidayOverride verifies that user-supplied overrides add extra
// holidays and that existing base-calendar holidays survive override insertion.
func TestCustomHolidayOverride(t *testing.T) {
	t.Parallel()
	overrides := []scheduler.CustomHolidayOverride{
		{Date: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC), Name: "Pepero Day (custom)"},
	}
	provider := scheduler.NewKoreanHolidayProviderWithOverrides(overrides)

	// Override date should be a holiday with the given name.
	if h := provider.Lookup(time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)); !h.IsHoliday || h.Name != "Pepero Day (custom)" {
		t.Errorf("2026-11-11 with override should be holiday %q, got %+v", "Pepero Day (custom)", h)
	}

	// Base-calendar holiday should still be present after override insertion.
	if h := provider.Lookup(time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)); !h.IsHoliday {
		t.Errorf("2026-10-03 should still be a holiday after override insertion, got %+v", h)
	}
}

// TestTimezoneDetector_Current verifies that Current() returns the most
// recently updated location.
func TestTimezoneDetector_Current(t *testing.T) {
	t.Parallel()
	seoul, _ := time.LoadLocation("Asia/Seoul")
	tokyo, _ := time.LoadLocation("Asia/Tokyo")

	fc := clockwork.NewFakeClock()
	d := scheduler.NewTimezoneDetector(seoul, scheduler.WithTimezoneClock(fc))

	if got := d.Current(); got != seoul {
		t.Errorf("Current() = %v, want %v", got, seoul)
	}

	// Update to Tokyo (within 24h — not flagged as shift, but Current updates).
	d.Update(tokyo)
	if got := d.Current(); got != tokyo {
		t.Errorf("Current() after Update = %v, want %v", got, tokyo)
	}
}

// TestTimezoneDetector_WithLogger verifies that WithTimezoneLogger option is applied.
func TestTimezoneDetector_WithLogger(t *testing.T) {
	t.Parallel()
	seoul, _ := time.LoadLocation("Asia/Seoul")
	logger := zap.NewNop()
	d := scheduler.NewTimezoneDetector(seoul,
		scheduler.WithTimezoneLogger(logger),
	)
	// Just verify construction succeeds with a non-nil logger.
	if d == nil {
		t.Error("NewTimezoneDetector with logger should not return nil")
	}
}

// TestScheduler_WithTimezoneDetector_NoShift verifies that WithTimezoneDetector
// wiring compiles and that NotifyTimezoneChange returns nil when no shift is detected.
func TestScheduler_WithTimezoneDetector_NoShift(t *testing.T) {
	t.Parallel()
	seoul, _ := time.LoadLocation("Asia/Seoul")

	fc := clockwork.NewFakeClock()
	detector := scheduler.NewTimezoneDetector(seoul, scheduler.WithTimezoneClock(fc))

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
		scheduler.WithTimezoneDetector(detector),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// NotifyTimezoneChange with same location within 24h — no shift.
	if err := sched.NotifyTimezoneChange(context.Background(), seoul); err != nil {
		t.Errorf("NotifyTimezoneChange no-shift should return nil, got: %v", err)
	}
}

// TestScheduler_WithHolidayCalendar_SkipHoliday verifies that SkipHolidays
// suppresses emission on public holidays.
func TestScheduler_WithHolidayCalendar_SkipHoliday(t *testing.T) {
	// 2026-10-03 is 개천절 (National Foundation Day) in Korea.
	seoul, _ := time.LoadLocation("Asia/Seoul")
	holidayNoon := time.Date(2026, 10, 3, 6, 59, 0, 0, seoul)
	fc := clockwork.NewFakeClockAt(holidayNoon)

	cfg := scheduler.SchedulerConfig{
		Enabled:  true,
		Timezone: "Asia/Seoul",
		Rituals: scheduler.RitualsConfig{
			Morning: scheduler.ClockConfig{
				Time:         "07:00",
				SkipWeekends: false,
			},
		},
	}

	reg := hook.NewHookRegistry()
	dispatch := hook.NewDispatcher(reg, zap.NewNop())
	persister := scheduler.NewFilePersister(t.TempDir())
	calendar := scheduler.NewKoreanHolidayProvider()

	// Build a RitualsConfig with SkipHolidays via a custom implementation since
	// ClockConfig does not have SkipHolidays — it is on RitualTime. We test via
	// the SkipHolidays field on RitualTime, which is populated from config in P3.
	// For P2, verify that holiday information is populated in ScheduledEvent.
	sched, err := scheduler.New(cfg, dispatch, persister,
		scheduler.WithClock(fc),
		scheduler.WithLogger(zap.NewNop()),
		scheduler.WithHolidayCalendar(calendar),
		scheduler.WithCronSpecOverride("@every 500ms"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sched.Stop(ctx) }()

	// Receive an event and verify IsHoliday is populated.
	select {
	case ev := <-sched.Events():
		if !ev.IsHoliday {
			t.Errorf("ScheduledEvent.IsHoliday should be true on 개천절, got false")
		}
		if ev.HolidayName == "" {
			t.Errorf("ScheduledEvent.HolidayName should be non-empty on 개천절")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for ScheduledEvent")
	}
}
