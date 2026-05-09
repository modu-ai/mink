// Package scheduler implements proactive ritual time emission for GOOSE
// Phase 7 Daily Companion. SPEC-GOOSE-SCHEDULER-001 P1.
package scheduler

import (
	"time"

	"github.com/modu-ai/goose/internal/hook"
)

// Re-export the 5 ritual HookEvent constants for ergonomic access from this package.
// Callers can use scheduler.MorningBriefingTime instead of hook.EvMorningBriefingTime.
var (
	// MorningBriefingTime is emitted at the user's configured morning briefing time.
	MorningBriefingTime = hook.EvMorningBriefingTime
	// PostBreakfastTime is emitted after the user's configured breakfast time.
	PostBreakfastTime = hook.EvPostBreakfastTime
	// PostLunchTime is emitted after the user's configured lunch time.
	PostLunchTime = hook.EvPostLunchTime
	// PostDinnerTime is emitted after the user's configured dinner time.
	PostDinnerTime = hook.EvPostDinnerTime
	// EveningCheckInTime is emitted at the user's configured evening check-in time.
	EveningCheckInTime = hook.EvEveningCheckInTime
)

// ScheduledEvent carries contextual data emitted with each ritual HookEvent.
// IsHoliday and HolidayName are populated by P2 (HolidayCalendar); P1 leaves them zero.
// BackoffApplied and DelayHint are populated by P3 (BackoffManager).
type ScheduledEvent struct {
	// Event is the hook event type emitted.
	Event hook.HookEvent
	// FiredAt is the wall-clock time at which the scheduler fired the event.
	FiredAt time.Time
	// ScheduledAt is the intended trigger time for this event.
	ScheduledAt time.Time
	// Timezone is the IANA timezone name used for scheduling.
	Timezone string
	// UserLocalDate is the local date in YYYY-MM-DD format at fire time.
	UserLocalDate string
	// IsHoliday indicates whether the fire date is a public holiday (populated in P2).
	IsHoliday bool
	// HolidayName is the name of the holiday if IsHoliday is true (populated in P2).
	HolidayName string
	// BackoffApplied is true when max_defer_count was reached and this is a force-emit.
	// (REQ-SCHED-021, P3)
	BackoffApplied bool
	// DelayHint is the total accumulated defer time when force-emit occurs.
	// Computed as defer_count * active_window. (REQ-SCHED-021, P3)
	DelayHint time.Duration
	// IsReplay indicates whether this event was replayed by Scheduler.Start
	// after a process restart that missed the scheduled time. (REQ-SCHED-022, P4b)
	IsReplay bool
	// DelayMinutes is the elapsed minutes between ScheduledAt and FiredAt for a
	// replayed event. Zero when IsReplay is false.
	DelayMinutes int
}

// RitualTime associates a ritual event with a scheduling specification.
type RitualTime struct {
	// Event is the hook event to emit.
	Event hook.HookEvent
	// Clock is the 24-hour "HH:MM" time string.
	Clock string
	// SkipWeekends suppresses emission on Saturdays and Sundays.
	SkipWeekends bool
	// SkipHolidays suppresses emission on public holidays (evaluated by P2 HolidayCalendar).
	SkipHolidays bool
}

// RegisteredEvents returns the 5 ritual HookEvent constants in declared order.
// The returned slice is always length 5.
func RegisteredEvents() []hook.HookEvent {
	return []hook.HookEvent{
		hook.EvMorningBriefingTime,
		hook.EvPostBreakfastTime,
		hook.EvPostLunchTime,
		hook.EvPostDinnerTime,
		hook.EvEveningCheckInTime,
	}
}
