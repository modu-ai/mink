// Package scheduler — Korean public holiday data 2026–2028.
// SPEC-GOOSE-SCHEDULER-001 P2 Task T-010.
//
// Gregorian equivalents for lunar-based holidays are sourced from the Korean
// Ministry of Government Legislation (law.go.kr) official public holiday table
// and confirmed against the Korea Astronomy and Space Science Institute (KASI)
// lunar calendar.
//
// Substitute holiday rules (statutory baseline, per the 2021
// substitute holiday law amendment):
//   - Seollal holiday window (eve/day/post) and Chuseok holiday window (eve/day/post):
//     Sunday overlap → next weekday substitute.
//   - Independence Movement Day, Children's Day, Liberation Day, National Foundation Day,
//     Hangeul Day: Sunday overlap → next Monday substitute.
//   - Children's Day: Saturday overlap → next Monday substitute (added in 2021).
//   - Christmas: Sunday overlap → next Monday substitute (amendment effective 2024).
package scheduler

import (
	"time"
)

// Holiday canonical keys. These English snake_case identifiers are
// emitted via ScheduledEvent.HolidayName for downstream consumers.
// For Korean display labels, use KoreanHolidayName.
const (
	HolidayNewYear         = "new_year"
	HolidaySeollalEve      = "seollal_eve"
	HolidaySeollal         = "seollal"
	HolidaySeollalPost     = "seollal_post"
	HolidayIndependenceDay = "independence_movement_day"
	HolidayChildrensDay    = "childrens_day"
	HolidayBuddhaBirthday  = "buddha_birthday"
	HolidayMemorialDay     = "memorial_day"
	HolidayLiberationDay   = "liberation_day"
	HolidayChuseokEve      = "chuseok_eve"
	HolidayChuseok         = "chuseok"
	HolidayChuseokPost     = "chuseok_post"
	HolidayFoundationDay   = "national_foundation_day"
	HolidayHangeulDay      = "hangeul_day"
	HolidayChristmasDay    = "christmas_day"
)

// HolidaySubstituteSuffix is appended to a base holiday key when the day
// is a substitute holiday for an actual holiday that fell on a weekend.
// Per the 2021 Korean substitute holiday law amendment.
const HolidaySubstituteSuffix = "_substitute"

// buildKoreanHolidays constructs the built-in holiday map for 2026–2028.
// Returns a map keyed by (year, month, day).
func buildKoreanHolidays() map[holidayKey]HolidayInfo {
	m := make(map[holidayKey]HolidayInfo, 64)
	add := func(y int, mo time.Month, d int, name string, isSub bool) {
		m[holidayKey{year: y, month: mo, day: d}] = HolidayInfo{
			IsHoliday:    true,
			Name:         name,
			IsSubstitute: isSub,
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2026 Korean Public Holidays
	// ─────────────────────────────────────────────────────────────────────────

	// New Year's Day — 2026-01-01 Thursday, no substitute needed
	add(2026, time.January, 1, HolidayNewYear, false)

	// Seollal (Lunar New Year) — 2026: lunar 1st month 1st day = 2026-02-17 (Tuesday)
	// Holiday window: 2026-02-16 (Mon/eve), 2026-02-17 (Tue/day), 2026-02-18 (Wed/post)
	// No overlap with Sunday — no substitute
	add(2026, time.February, 16, HolidaySeollalEve, false)
	add(2026, time.February, 17, HolidaySeollal, false)
	add(2026, time.February, 18, HolidaySeollalPost, false)

	// Independence Movement Day — 2026-03-01 Sunday → substitute 2026-03-02 (Monday)
	add(2026, time.March, 1, HolidayIndependenceDay, false)
	add(2026, time.March, 2, HolidayIndependenceDay+HolidaySubstituteSuffix, true)

	// Children's Day — 2026-05-05 Tuesday, no substitute needed
	add(2026, time.May, 5, HolidayChildrensDay, false)

	// Buddha's Birthday — 2026: lunar 4th month 8th day = 2026-05-24 Sunday
	// Substitute rule confirmed by the 2023 amendment to include this holiday.
	// Substitute: 2026-05-25 (Monday)
	add(2026, time.May, 24, HolidayBuddhaBirthday, false)
	add(2026, time.May, 25, HolidayBuddhaBirthday+HolidaySubstituteSuffix, true)

	// Memorial Day — 2026-06-06 Saturday (commemorative day, not eligible for substitute)
	add(2026, time.June, 6, HolidayMemorialDay, false)

	// Liberation Day — 2026-08-15 Saturday → no substitute
	// Note: substitute only applies on Sunday per current law.
	add(2026, time.August, 15, HolidayLiberationDay, false)

	// Chuseok — 2026: lunar 8th month 15th day = 2026-09-25 (Friday)
	// Holiday window: 2026-09-24 (Thu/eve), 2026-09-25 (Fri/day), 2026-09-26 (Sat/post)
	// Post-day falls on Saturday → substitute 2026-09-28 (Monday)
	add(2026, time.September, 24, HolidayChuseokEve, false)
	add(2026, time.September, 25, HolidayChuseok, false)
	add(2026, time.September, 26, HolidayChuseokPost, false)
	add(2026, time.September, 28, HolidayChuseok+HolidaySubstituteSuffix, true)

	// National Foundation Day — 2026-10-03 Saturday → no Sunday overlap, no substitute
	add(2026, time.October, 3, HolidayFoundationDay, false)

	// Hangeul Day — 2026-10-09 Friday, no substitute needed
	add(2026, time.October, 9, HolidayHangeulDay, false)

	// Christmas — 2026-12-25 Friday, no substitute needed
	add(2026, time.December, 25, HolidayChristmasDay, false)

	// ─────────────────────────────────────────────────────────────────────────
	// 2027 Korean Public Holidays
	// ─────────────────────────────────────────────────────────────────────────

	// New Year's Day — 2027-01-01 Friday, no substitute
	add(2027, time.January, 1, HolidayNewYear, false)

	// Seollal — 2027: lunar 1st month 1st day = 2027-02-06 (Saturday)
	// Holiday window: 2027-02-05 (Fri/eve), 2027-02-06 (Sat/day), 2027-02-07 (Sun/post)
	// Day Saturday → substitute 2027-02-08 (Monday)
	// Post-day Sunday → substitute 2027-02-09 (Tuesday) — per multi-day substitute rule
	add(2027, time.February, 5, HolidaySeollalEve, false)
	add(2027, time.February, 6, HolidaySeollal, false)
	add(2027, time.February, 7, HolidaySeollalPost, false)
	add(2027, time.February, 8, HolidaySeollal+HolidaySubstituteSuffix, true)
	add(2027, time.February, 9, HolidaySeollal+HolidaySubstituteSuffix, true)

	// Independence Movement Day — 2027-03-01 Monday, no substitute needed
	add(2027, time.March, 1, HolidayIndependenceDay, false)

	// Children's Day — 2027-05-05 Wednesday, no substitute
	add(2027, time.May, 5, HolidayChildrensDay, false)

	// Buddha's Birthday — 2027: lunar 4th month 8th day = 2027-05-13 Thursday, no substitute
	add(2027, time.May, 13, HolidayBuddhaBirthday, false)

	// Memorial Day — 2027-06-06 Sunday. Memorial Day is a commemorative day
	// (under the Act on Various Anniversaries etc.), not a national holiday,
	// so substitute holiday provisions do not apply.
	add(2027, time.June, 6, HolidayMemorialDay, false)

	// Liberation Day — 2027-08-15 Sunday → substitute 2027-08-16 (Monday)
	add(2027, time.August, 15, HolidayLiberationDay, false)
	add(2027, time.August, 16, HolidayLiberationDay+HolidaySubstituteSuffix, true)

	// Chuseok — 2027: lunar 8th month 15th day = 2027-09-15 (Wednesday)
	// Holiday window: 2027-09-14 (Tue/eve), 2027-09-15 (Wed/day), 2027-09-16 (Thu/post)
	// No weekend overlap — no substitute
	add(2027, time.September, 14, HolidayChuseokEve, false)
	add(2027, time.September, 15, HolidayChuseok, false)
	add(2027, time.September, 16, HolidayChuseokPost, false)

	// National Foundation Day — 2027-10-03 Sunday → substitute 2027-10-04 (Monday)
	add(2027, time.October, 3, HolidayFoundationDay, false)
	add(2027, time.October, 4, HolidayFoundationDay+HolidaySubstituteSuffix, true)

	// Hangeul Day — 2027-10-09 Saturday → no substitute (Saturday rule only for Children's Day)
	add(2027, time.October, 9, HolidayHangeulDay, false)

	// Christmas — 2027-12-25 Saturday → no substitute (Sunday rule)
	add(2027, time.December, 25, HolidayChristmasDay, false)

	// ─────────────────────────────────────────────────────────────────────────
	// 2028 Korean Public Holidays
	// ─────────────────────────────────────────────────────────────────────────

	// New Year's Day — 2028-01-01 Saturday → no substitute (New Year's Day is not
	// eligible for substitute holiday under current law)
	add(2028, time.January, 1, HolidayNewYear, false)

	// Seollal — 2028: lunar 1st month 1st day = 2028-01-26 (Wednesday)
	// Holiday window: 2028-01-25 (Tue/eve), 2028-01-26 (Wed/day), 2028-01-27 (Thu/post)
	// No weekend overlap — no substitute
	add(2028, time.January, 25, HolidaySeollalEve, false)
	add(2028, time.January, 26, HolidaySeollal, false)
	add(2028, time.January, 27, HolidaySeollalPost, false)

	// Independence Movement Day — 2028-03-01 Wednesday, no substitute
	add(2028, time.March, 1, HolidayIndependenceDay, false)

	// Children's Day — 2028-05-05 Saturday → substitute 2028-05-07 (Monday)
	// Per the Children's Day Saturday rule (applicable since 2021).
	add(2028, time.May, 5, HolidayChildrensDay, false)
	add(2028, time.May, 7, HolidayChildrensDay+HolidaySubstituteSuffix, true)

	// Buddha's Birthday — 2028: lunar 4th month 8th day = 2028-05-02 Tuesday, no substitute
	add(2028, time.May, 2, HolidayBuddhaBirthday, false)

	// Memorial Day — 2028-06-06 Wednesday, no substitute
	add(2028, time.June, 6, HolidayMemorialDay, false)

	// Liberation Day — 2028-08-15 Tuesday, no substitute
	add(2028, time.August, 15, HolidayLiberationDay, false)

	// Chuseok — 2028: lunar 8th month 15th day = 2028-10-03 (Tuesday)
	// Holiday window: 2028-10-02 (Mon/eve), 2028-10-03 (Tue/day), 2028-10-04 (Wed/post)
	// No weekend overlap — no substitute
	add(2028, time.October, 2, HolidayChuseokEve, false)
	add(2028, time.October, 3, HolidayChuseok, false)
	add(2028, time.October, 4, HolidayChuseokPost, false)

	// National Foundation Day — 2028-10-03 (Tuesday) coincides with Chuseok day.
	// Both are public holidays; a single map entry can hold only one name per date.
	// We represent the coincidence with a combined canonical key string.
	// Implementation note: the map can hold only one entry per key — the entry
	// written last wins. We use a combined name to preserve both identities.
	m[holidayKey{year: 2028, month: time.October, day: 3}] = HolidayInfo{
		IsHoliday:    true,
		Name:         HolidayChuseok + "/" + HolidayFoundationDay,
		IsSubstitute: false,
	}

	// Hangeul Day — 2028-10-09 Monday, no substitute
	add(2028, time.October, 9, HolidayHangeulDay, false)

	// Christmas — 2028-12-25 Monday, no substitute
	add(2028, time.December, 25, HolidayChristmasDay, false)

	return m
}
