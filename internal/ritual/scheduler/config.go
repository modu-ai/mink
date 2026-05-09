// Package scheduler — config types and validation for SPEC-GOOSE-SCHEDULER-001.
package scheduler

import (
	"fmt"
	"time"
)

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
// Returns an error if Timezone is not a valid IANA location name.
// REQ-SCHED-001: timezone must be IANA-valid.
func (cfg SchedulerConfig) Validate() error {
	tz := cfg.effectiveTimezone()
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("scheduler: invalid IANA timezone %q: %w", tz, err)
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
