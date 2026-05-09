// Package scheduler — thin cron.Cron wrapper for SPEC-GOOSE-SCHEDULER-001.
package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	cron "github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// newCronEngine constructs a *cron.Cron with the provided location and logger.
// Panics and still-running-overlap jobs are handled via middleware chains.
func newCronEngine(loc *time.Location, logger *zap.Logger) *cron.Cron {
	cl := cronZapLogger{log: logger}
	return cron.New(
		cron.WithLocation(loc),
		cron.WithChain(
			cron.Recover(cl),
			cron.SkipIfStillRunning(cl),
		),
	)
}

// cronZapLogger adapts a *zap.Logger to the cron.Logger interface.
type cronZapLogger struct {
	log *zap.Logger
}

// Info implements cron.Logger.
func (l cronZapLogger) Info(msg string, keysAndValues ...interface{}) {
	fields := kvToZapFields(keysAndValues)
	l.log.Info(msg, fields...)
}

// Error implements cron.Logger.
func (l cronZapLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	fields := kvToZapFields(keysAndValues)
	fields = append(fields, zap.Error(err))
	l.log.Error(msg, fields...)
}

// kvToZapFields converts alternating key-value pairs from cron to zap.Field slice.
func kvToZapFields(kv []interface{}) []zap.Field {
	fields := make([]zap.Field, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, _ := kv[i].(string)
		fields = append(fields, zap.Any(key, kv[i+1]))
	}
	return fields
}

// parseClock parses a strict "HH:MM" 24-hour time string into hour and minute integers.
// Returns a descriptive wrapped error on any invalid input including empty string.
func parseClock(s string) (hour, minute int, err error) {
	parts, err := splitClock(s)
	if err != nil {
		return 0, 0, err
	}
	return parseHourMinute(parts)
}

// clockToCronExpr converts hour and minute to a robfig/cron "M H * * *" expression.
// The returned string is valid for cron.AddFunc.
func clockToCronExpr(hour, minute int) string {
	return fmt.Sprintf("%d %d * * *", minute, hour)
}

// validateHourMinute checks that hour is in [0,23] and minute is in [0,59].
func validateHourMinute(hour, minute int) error {
	if hour < 0 || hour > 23 {
		return fmt.Errorf("scheduler: hour %d out of range [0,23]", hour)
	}
	if minute < 0 || minute > 59 {
		return fmt.Errorf("scheduler: minute %d out of range [0,59]", minute)
	}
	return nil
}

// parseHourMinute is the shared inner parser used by parseClock.
func parseHourMinute(parts []string) (int, int, error) {
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("scheduler: invalid hour %q: %w", parts[0], err)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("scheduler: invalid minute %q: %w", parts[1], err)
	}
	if err := validateHourMinute(h, m); err != nil {
		return 0, 0, err
	}
	return h, m, nil
}

// splitClock splits the HH:MM string into parts (does not validate content).
func splitClock(s string) ([]string, error) {
	if s == "" {
		return nil, fmt.Errorf("scheduler: clock string must not be empty")
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("scheduler: clock string %q must have format HH:MM", s)
	}
	return parts, nil
}
