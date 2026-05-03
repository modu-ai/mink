package insights

import (
	"errors"
	"time"
)

// ErrInvalidPeriod is returned when From > To in an InsightsPeriod.
var ErrInvalidPeriod = errors.New("insights: period From must be before or equal To")

// InsightsPeriod specifies the time range for an Extract call.
type InsightsPeriod struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Validate returns ErrInvalidPeriod if From is after To.
func (p InsightsPeriod) Validate() error {
	if p.From.After(p.To) {
		return ErrInvalidPeriod
	}
	return nil
}

// Contains reports whether t falls within [From, To] (inclusive).
func (p InsightsPeriod) Contains(t time.Time) bool {
	return !t.Before(p.From) && !t.After(p.To)
}

// Last returns a period covering the last n days ending now (UTC).
func Last(days int) InsightsPeriod {
	now := time.Now().UTC()
	return InsightsPeriod{
		From: now.AddDate(0, 0, -days),
		To:   now,
	}
}

// LastDuration returns a period covering the last d duration ending now (UTC).
func LastDuration(d time.Duration) InsightsPeriod {
	now := time.Now().UTC()
	return InsightsPeriod{
		From: now.Add(-d),
		To:   now,
	}
}

// Between returns a period with explicit From and To bounds.
func Between(from, to time.Time) InsightsPeriod {
	return InsightsPeriod{From: from, To: to}
}

// AllTime returns a period covering all time (zero value From, far-future To).
func AllTime() InsightsPeriod {
	return InsightsPeriod{
		From: time.Time{},
		To:   time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
	}
}
