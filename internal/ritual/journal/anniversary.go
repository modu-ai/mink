package journal

import (
	"context"
	"time"
)

// ImportantDate represents a significant calendar date for a user,
// loaded from the IDENTITY-001 service.
type ImportantDate struct {
	// Type is the event category, e.g. "wedding", "birthday", "bereavement".
	Type string
	// Name is the human-readable label, e.g. "결혼기념일".
	Name string
	// Date is the original event date. Year is the year of occurrence.
	Date time.Time
}

// IdentityClient is the interface to the IDENTITY-001 service used to retrieve
// important dates. Tests supply a mock implementation.
type IdentityClient interface {
	// GetImportantDates returns the important dates registered for userID.
	GetImportantDates(ctx context.Context, userID string) ([]ImportantDate, error)
}

// AnniversaryDetector checks whether today (±1 day) coincides with any of
// the user's important dates loaded from IDENTITY-001.
//
// @MX:ANCHOR: [AUTO] Anniversary detection gateway; called by orchestrator and tests — fan_in >= 3
// @MX:REASON: Used by orchestrator.Prompt, orchestrator tests, and anniversary tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-008, AC-007
type AnniversaryDetector struct {
	identity IdentityClient
}

// NewAnniversaryDetector constructs a detector backed by the given identity client.
func NewAnniversaryDetector(identity IdentityClient) *AnniversaryDetector {
	return &AnniversaryDetector{identity: identity}
}

// anniversaryWindowDays is the ±N day window used for date matching.
const anniversaryWindowDays = 1

// Check returns the important dates that fall within ±anniversaryWindowDays of today.
// The year on ImportantDate.Date is the original occurrence year; the month and day
// are compared against today's month and day.
//
// Returns nil, nil when the identity client returns no dates or none match.
func (d *AnniversaryDetector) Check(ctx context.Context, userID string, today time.Time) ([]ImportantDate, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	dates, err := d.identity.GetImportantDates(ctx, userID)
	if err != nil {
		return nil, err
	}

	var matches []ImportantDate
	for _, date := range dates {
		if dateInWindow(date.Date, today, anniversaryWindowDays) {
			matches = append(matches, date)
		}
	}
	return matches, nil
}

// dateInWindow reports whether the month and day of candidate fall within
// windowDays of the month and day of today (ignoring year).
// The comparison is done by replacing candidate's year with today's year.
func dateInWindow(candidate, today time.Time, windowDays int) bool {
	// Replace the candidate year with today's year so we compare month/day only.
	sameYear := time.Date(today.Year(), candidate.Month(), candidate.Day(),
		0, 0, 0, 0, today.Location())
	diff := today.YearDay() - sameYear.YearDay()
	if diff < 0 {
		diff = -diff
	}
	// Handle year wrap-around (e.g. candidate Dec 31, today Jan 1).
	daysInYear := 365
	if isLeapYear(today.Year()) {
		daysInYear = 366
	}
	if diff > daysInYear/2 {
		diff = daysInYear - diff
	}
	return diff <= windowDays
}

// isLeapYear reports whether year is a Gregorian leap year.
func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
