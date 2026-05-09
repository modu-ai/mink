// Package scheduler — config types and validation for SPEC-GOOSE-SCHEDULER-001.
package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrQuietHoursViolation is returned by Start when a ritual is configured in the
// [23:00, 06:00) quiet-hours window and AllowNighttime is false. (REQ-SCHED-014)
var ErrQuietHoursViolation = errors.New("scheduler: ritual time falls within quiet hours [23:00, 06:00)")

// BackoffConfig holds parameters for the BackoffManager. (REQ-SCHED-011, REQ-SCHED-021)
type BackoffConfig struct {
	// ActiveWindow is the duration after the last user activity during which ritual
	// triggers are deferred. Default: 10 minutes.
	ActiveWindow time.Duration
	// MaxDeferCount is the maximum number of consecutive defers before force-emit.
	// Default: 3.
	MaxDeferCount int
}

// effectiveBackoff returns a BackoffConfig with defaults applied.
func (b BackoffConfig) effectiveBackoff() BackoffConfig {
	if b.ActiveWindow <= 0 {
		b.ActiveWindow = 10 * time.Minute
	}
	if b.MaxDeferCount <= 0 {
		b.MaxDeferCount = 3
	}
	return b
}

// SchedulerConfig holds the top-level configuration for the ritual scheduler.
// Keys map to the YAML path scheduler.* in the application config.
type SchedulerConfig struct {
	// Enabled controls whether the scheduler starts on application launch.
	// Defaults to true if not set.
	Enabled bool
	// Timezone is an IANA time zone name (e.g. "Asia/Seoul"). Defaults to "Asia/Seoul"
	// when empty. An explicitly set value must be a valid IANA name.
	Timezone string
	// Rituals holds per-meal and per-context scheduling configuration.
	Rituals RitualsConfig
	// AllowNighttime disables the [23:00, 06:00) quiet-hours floor when true.
	// Default: false. (REQ-SCHED-014)
	AllowNighttime bool
	// Backoff holds parameters for deferring ritual triggers during active sessions.
	// (REQ-SCHED-011, REQ-SCHED-021)
	Backoff BackoffConfig
	// PatternLearner holds parameters for the daily PatternLearner cron job.
	// (REQ-SCHED-006, REQ-SCHED-019)
	PatternLearner PatternLearnerConfig
	// MissedEventReplayMaxDelay is the maximum lag between a missed scheduled
	// time and process restart for which the scheduler will replay the event.
	// Default: 1 hour. Zero or negative values fall back to the default.
	// (REQ-SCHED-022)
	MissedEventReplayMaxDelay time.Duration
}

// effectiveMissedReplayDelay returns MissedEventReplayMaxDelay or the default
// 1-hour threshold when unset.
func (cfg SchedulerConfig) effectiveMissedReplayDelay() time.Duration {
	if cfg.MissedEventReplayMaxDelay <= 0 {
		return time.Hour
	}
	return cfg.MissedEventReplayMaxDelay
}

// PatternLearnerConfig parameterises the daily PatternLearner that proposes
// new ritual times based on observed activity peaks.
type PatternLearnerConfig struct {
	// Enabled gates the 03:00 daily learner cron entry. Default: false.
	// When false, the learner is never invoked; ritual times come exclusively
	// from explicit user configuration.
	Enabled bool
	// RollingWindowDays is the number of past days the learner aggregates over.
	// Default: 7.
	RollingWindowDays int
	// DriftThresholdMinutes is the minimum |drift| required to emit a proposal.
	// Default: 30.
	DriftThresholdMinutes int
	// DefaultMorning/Breakfast/Lunch/Dinner/Evening provide fallback clock
	// strings used when DaysObserved < RollingWindowDays. Empty values inherit
	// per-kind built-in defaults applied by effective().
	DefaultMorning   string
	DefaultBreakfast string
	DefaultLunch     string
	DefaultDinner    string
	DefaultEvening   string
}

// effective returns a PatternLearnerConfig with built-in defaults applied to
// any unset fields.
func (c PatternLearnerConfig) effective() PatternLearnerConfig {
	if c.RollingWindowDays <= 0 {
		c.RollingWindowDays = 7
	}
	if c.DriftThresholdMinutes <= 0 {
		c.DriftThresholdMinutes = 30
	}
	if c.DefaultMorning == "" {
		c.DefaultMorning = "07:30"
	}
	if c.DefaultBreakfast == "" {
		c.DefaultBreakfast = "08:00"
	}
	if c.DefaultLunch == "" {
		c.DefaultLunch = "12:30"
	}
	if c.DefaultDinner == "" {
		c.DefaultDinner = "19:00"
	}
	if c.DefaultEvening == "" {
		c.DefaultEvening = "22:00"
	}
	return c
}

// RitualsConfig groups the individual ritual schedule entries.
type RitualsConfig struct {
	// Morning is the morning briefing schedule.
	Morning ClockConfig
	// Meals holds the three meal-time schedules.
	Meals MealsConfig
	// Evening is the evening check-in schedule.
	Evening ClockConfig
}

// MealsConfig groups the three meal-time schedules.
type MealsConfig struct {
	// Breakfast is the post-breakfast reminder schedule.
	Breakfast ClockConfig
	// Lunch is the post-lunch reminder schedule.
	Lunch ClockConfig
	// Dinner is the post-dinner reminder schedule.
	Dinner ClockConfig
}

// ClockConfig holds a single ritual time specification.
type ClockConfig struct {
	// Time is a 24-hour "HH:MM" clock string. Empty string disables this ritual.
	Time string
	// SkipWeekends suppresses emission on Saturdays and Sundays.
	SkipWeekends bool
}

// Validate checks the SchedulerConfig for correctness.
//
//   - Returns an error if Timezone is not a valid IANA location name.
//   - Returns ErrQuietHoursViolation if any ritual clock falls in [23:00, 06:00)
//     and AllowNighttime is false. (REQ-SCHED-014)
func (cfg SchedulerConfig) Validate() error {
	tz := cfg.effectiveTimezone()
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("scheduler: invalid IANA timezone %q: %w", tz, err)
	}

	if !cfg.AllowNighttime {
		clocks := cfg.allRitualClocks()
		for _, c := range clocks {
			if c == "" {
				continue
			}
			if err := checkQuietHours(c); err != nil {
				return err
			}
		}
	}
	return nil
}

// allRitualClocks returns all non-empty clock strings from the Rituals config.
func (cfg SchedulerConfig) allRitualClocks() []string {
	return []string{
		cfg.Rituals.Morning.Time,
		cfg.Rituals.Meals.Breakfast.Time,
		cfg.Rituals.Meals.Lunch.Time,
		cfg.Rituals.Meals.Dinner.Time,
		cfg.Rituals.Evening.Time,
	}
}

// checkQuietHours returns ErrQuietHoursViolation when the clock string falls
// within [23:00, 06:00) local time. Exactly 06:00 is NOT in the quiet zone.
func checkQuietHours(clock string) error {
	parts := strings.SplitN(clock, ":", 2)
	if len(parts) != 2 {
		return nil // parsing errors are handled elsewhere
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}
	// Quiet zone: hour >= 23 OR hour < 6 (i.e. [23:00, 06:00), 06:00 excluded).
	if h >= 23 || h < 6 {
		return fmt.Errorf("%w (configured time: %s)", ErrQuietHoursViolation, clock)
	}
	return nil
}

// effectiveTimezone returns the Timezone field if non-empty, otherwise "Asia/Seoul".
func (cfg SchedulerConfig) effectiveTimezone() string {
	if cfg.Timezone == "" {
		return "Asia/Seoul"
	}
	return cfg.Timezone
}

// Location parses and returns the *time.Location for this config.
// Returns an error if the effective timezone is not a valid IANA name.
func (cfg SchedulerConfig) Location() (*time.Location, error) {
	tz := cfg.effectiveTimezone()
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("scheduler: invalid IANA timezone %q: %w", tz, err)
	}
	return loc, nil
}
