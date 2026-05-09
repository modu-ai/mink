// Package scheduler — fire-event log schema helper. SPEC-GOOSE-SCHEDULER-001 P4b T-029.
package scheduler

import (
	"go.uber.org/zap"
)

// EmitFireLog writes a single INFO-level structured log entry that conforms
// to the REQ-SCHED-004 schema: exactly the seven fields
// {event, scheduled_at, actual_at, tz, holiday, backoff_applied, skipped}.
//
// reason is intentionally NOT in the canonical schema. It is logged at DEBUG
// level via a separate call when a skip occurs and a reason is meaningful.
//
// @MX:NOTE: [AUTO] Canonical fire-log entry — every scheduler dispatch and skip
//
//	must route through this helper to keep the schema stable.
//
// @MX:SPEC: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-004
func EmitFireLog(logger *zap.Logger, ev ScheduledEvent, skipped bool, reason string) {
	if logger == nil {
		return
	}
	logger.Info("ritual_fire",
		zap.String("event", string(ev.Event)),
		zap.Time("scheduled_at", ev.ScheduledAt),
		zap.Time("actual_at", ev.FiredAt),
		zap.String("tz", ev.Timezone),
		zap.Bool("holiday", ev.IsHoliday),
		zap.Bool("backoff_applied", ev.BackoffApplied),
		zap.Bool("skipped", skipped),
	)
	// Reason, when set, is emitted as a separate DEBUG entry to avoid mutating
	// the canonical 7-field schema verified by AC-SCHED-010.
	if skipped && reason != "" {
		logger.Debug("ritual_skip_reason",
			zap.String("event", string(ev.Event)),
			zap.String("reason", reason),
		)
	}
}
