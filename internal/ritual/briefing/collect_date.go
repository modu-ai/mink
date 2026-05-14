package briefing

import (
	"time"
)

// DateCollector collects date and calendar information for the briefing.
type DateCollector struct {
	// No dependencies - uses pure functions from solarterm.go and holiday.go
}

// NewDateCollector creates a new DateCollector.
func NewDateCollector() *DateCollector {
	return &DateCollector{}
}

// koreanDayNames maps weekday numbers to Korean day names.
// weekday 0 = Sunday, 1 = Monday, ..., 6 = Saturday
var koreanDayNames = []string{
	"일요일", // Sunday
	"월요일", // Monday
	"화요일", // Tuesday
	"수요일", // Wednesday
	"목요일", // Thursday
	"금요일", // Friday
	"토요일", // Saturday
}

// Collect fetches date information and returns a DateModule.
// Status is always "ok" since date calculation cannot fail (out-of-range returns nil fields).
func (c *DateCollector) Collect(today time.Time) (*DateModule, string) {
	module := &DateModule{
		Today:     today.Format("2006-01-02"),
		DayOfWeek: koreanDayNames[today.Weekday()],
		SolarTerm: nil,
		Holiday:   nil,
	}

	// Look up solar term (returns nil if out of range or no match)
	solarTerm, _ := SolarTermOnDate(today.Year(), int(today.Month()), today.Day())
	if solarTerm != nil {
		module.SolarTerm = &SolarTerm{
			Name:      solarTerm.Name,
			NameHanja: solarTerm.NameHanja,
			Date:      solarTerm.Date,
		}
	}

	// Look up Korean holiday (returns nil if no match)
	holiday, _ := LookupKoreanHoliday(today.Year(), int(today.Month()), today.Day())
	if holiday != nil {
		module.Holiday = &KoreanHoliday{
			Name:      holiday.Name,
			NameHanja: holiday.NameHanja,
			Date:      holiday.Date,
			IsHoliday: holiday.IsHoliday,
		}
	}

	return module, "ok"
}
