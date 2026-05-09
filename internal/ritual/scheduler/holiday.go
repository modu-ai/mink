// Package scheduler — holiday calendar types for SPEC-GOOSE-SCHEDULER-001 P2.
package scheduler

import (
	"strings"
	"time"
)

// HolidayInfo holds the result of a holiday lookup for a given date.
type HolidayInfo struct {
	// IsHoliday reports whether the date is a public holiday.
	IsHoliday bool
	// Name is the holiday name; empty if not a holiday.
	Name string
	// IsSubstitute reports whether the day is a substitute holiday for an actual holiday that fell on a weekend.
	IsSubstitute bool
}

// HolidayCalendar is the interface for looking up public holidays.
type HolidayCalendar interface {
	// Lookup returns HolidayInfo for the given date.
	// The date's year/month/day in its own Location is used; time-of-day is ignored.
	Lookup(date time.Time) HolidayInfo
}

// CustomHolidayOverride adds a user-defined holiday for a specific calendar date.
type CustomHolidayOverride struct {
	// Date is the date to mark as a holiday (time-of-day is ignored).
	Date time.Time
	// Name is the holiday name shown in HolidayInfo.Name.
	Name string
}

// KoreanHolidayProvider implements HolidayCalendar for South Korean public holidays.
// It supports substitute holidays and optional user overrides.
//
// @MX:ANCHOR: [AUTO] KoreanHolidayProvider is the primary holiday lookup entry point
// @MX:REASON: SPEC-GOOSE-SCHEDULER-001 REQ-SCHED-009 — fan_in >= 3 (Lookup, New, WithOverrides, Scheduler wiring)
type KoreanHolidayProvider struct {
	// entries holds the pre-computed year/month/day → HolidayInfo map.
	entries map[holidayKey]HolidayInfo
	// overrides holds user-supplied extra holidays indexed by year/month/day.
	overrides map[holidayKey]HolidayInfo
}

// holidayKey is the map key for holiday lookup: year × month × day.
type holidayKey struct {
	year  int
	month time.Month
	day   int
}

func keyFrom(t time.Time) holidayKey {
	y, m, d := t.Date()
	return holidayKey{year: y, month: m, day: d}
}

// NewKoreanHolidayProvider constructs a KoreanHolidayProvider with the built-in
// Korean holiday calendar (2026–2028).
func NewKoreanHolidayProvider() *KoreanHolidayProvider {
	return NewKoreanHolidayProviderWithOverrides(nil)
}

// NewKoreanHolidayProviderWithOverrides constructs a KoreanHolidayProvider with the
// built-in calendar plus the supplied user overrides.
// Override dates ADD to (not replace) the base calendar; if both sources apply on
// the same date, the override's Name wins.
func NewKoreanHolidayProviderWithOverrides(overrides []CustomHolidayOverride) *KoreanHolidayProvider {
	p := &KoreanHolidayProvider{
		entries:   buildKoreanHolidays(),
		overrides: make(map[holidayKey]HolidayInfo, len(overrides)),
	}
	for _, o := range overrides {
		k := keyFrom(o.Date)
		p.overrides[k] = HolidayInfo{IsHoliday: true, Name: o.Name}
	}
	return p
}

// KoreanHolidayName returns the Korean display label for a canonical
// holiday key (as emitted via ScheduledEvent.HolidayName). Returns the
// input key unchanged if no mapping exists. Suitable for log/UI rendering
// in Korean locale.
//
// @MX:NOTE: [AUTO] Stable mapping table for canonical-to-Korean i18n. Do not
// remove entries — downstream consumers may pin specific keys.
func KoreanHolidayName(key string) string {
	switch key {
	case HolidayNewYear:
		return "신정"
	case HolidaySeollalEve:
		return "설날 전날"
	case HolidaySeollal:
		return "설날"
	case HolidaySeollalPost:
		return "설날 다음날"
	case HolidayIndependenceDay:
		return "삼일절"
	case HolidayChildrensDay:
		return "어린이날"
	case HolidayBuddhaBirthday:
		return "부처님오신날"
	case HolidayMemorialDay:
		return "현충일"
	case HolidayLiberationDay:
		return "광복절"
	case HolidayChuseokEve:
		return "추석 전날"
	case HolidayChuseok:
		return "추석"
	case HolidayChuseokPost:
		return "추석 다음날"
	case HolidayFoundationDay:
		return "개천절"
	case HolidayHangeulDay:
		return "한글날"
	case HolidayChristmasDay:
		return "성탄절"
	}
	// Substitute fallback: if key ends with _substitute, look up base key and append label.
	if base, ok := strings.CutSuffix(key, HolidaySubstituteSuffix); ok {
		if name := KoreanHolidayName(base); name != base {
			return name + " 대체공휴일"
		}
	}
	return key
}

// Lookup returns HolidayInfo for the given date.
// It checks user overrides first (name wins), then built-in entries.
func (p *KoreanHolidayProvider) Lookup(date time.Time) HolidayInfo {
	k := keyFrom(date)

	// User override: if present, it is always a holiday. Name overrides built-in.
	if ov, ok := p.overrides[k]; ok {
		// Still mark IsSubstitute from base if applicable.
		if base, ok2 := p.entries[k]; ok2 {
			ov.IsSubstitute = base.IsSubstitute
		}
		return ov
	}

	if h, ok := p.entries[k]; ok {
		return h
	}
	return HolidayInfo{}
}
