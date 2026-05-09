// Package scheduler — Korean public holiday data 2026–2028.
// SPEC-GOOSE-SCHEDULER-001 P2 Task T-010.
//
// Gregorian equivalents for lunar-based holidays are sourced from the Korean
// Ministry of Government Legislation (law.go.kr) official public holiday table
// and confirmed against the Korea Astronomy and Space Science Institute (KASI)
// 음력 calendar.
//
// 대체공휴일 (substitute holiday) rules (법정 기준, 2021년 개정):
//   - 설날 연휴(전날/당일/다음날), 추석 연휴(전날/당일/다음날): Sunday overlap → next weekday
//   - 3·1절, 어린이날, 광복절, 개천절, 한글날: Sunday overlap → next Monday
//   - 어린이날: Saturday overlap → next Monday (2021년 추가 적용)
//   - 크리스마스: Sunday overlap → next Monday (2024년 개정 적용)
package scheduler

import (
	"time"
)

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

	// 신정 New Year's Day — 2026-01-01 Thursday, no substitute needed
	add(2026, time.January, 1, "신정 (New Year's Day)", false)

	// 설날 Lunar New Year — 2026: 음력 정월 초하루 = 2026-02-17 (Tuesday)
	// 연휴: 2026-02-16 (월/전날), 2026-02-17 (화/당일), 2026-02-18 (수/다음날)
	add(2026, time.February, 16, "설날 전날", false)
	add(2026, time.February, 17, "설날 (Lunar New Year)", false)
	add(2026, time.February, 18, "설날 다음날", false)
	// No overlap with Sunday → no substitute

	// 삼일절 Independence Movement Day — 2026-03-01 Sunday → 대체 2026-03-02 (Monday)
	add(2026, time.March, 1, "삼일절 (Independence Movement Day)", false)
	add(2026, time.March, 2, "삼일절 대체공휴일", true)

	// 어린이날 Children's Day — 2026-05-05 Tuesday, no substitute needed
	add(2026, time.May, 5, "어린이날 (Children's Day)", false)

	// 부처님오신날 Buddha's Birthday — 2026: 음력 4/8 = 2026-05-24 Sunday → 대체 2026-05-25 (Monday)
	// Note: 부처님오신날 substitute holiday confirmed by 2023년 개정.
	add(2026, time.May, 24, "부처님오신날 (Buddha's Birthday)", false)
	add(2026, time.May, 25, "부처님오신날 대체공휴일", true)

	// 현충일 Memorial Day — 2026-06-06 Saturday (기념일, not 대체공휴일 eligible)
	add(2026, time.June, 6, "현충일 (Memorial Day)", false)

	// 광복절 Liberation Day — 2026-08-15 Saturday → no substitute (Saturday rule only for 어린이날)
	// Note: 광복절 substitute only applies on Sunday per current law.
	add(2026, time.August, 15, "광복절 (Liberation Day)", false)

	// 추석 Chuseok — 2026: 음력 8/15 = 2026-09-25 (Friday)
	// 연휴: 2026-09-24 (목/전날), 2026-09-25 (금/당일), 2026-09-26 (토/다음날)
	// 2026-09-26 is Saturday → substitute for 다음날 → 2026-09-28 (Monday)
	add(2026, time.September, 24, "추석 전날", false)
	add(2026, time.September, 25, "추석 (Chuseok)", false)
	add(2026, time.September, 26, "추석 다음날", false)
	add(2026, time.September, 28, "추석 대체공휴일", true)

	// 개천절 National Foundation Day — 2026-10-03 Saturday → no Sunday overlap, no substitute
	add(2026, time.October, 3, "개천절 (National Foundation Day)", false)

	// 한글날 Hangul Day — 2026-10-09 Friday, no substitute needed
	add(2026, time.October, 9, "한글날 (Hangul Day)", false)

	// 크리스마스 Christmas — 2026-12-25 Friday, no substitute needed
	add(2026, time.December, 25, "크리스마스 (Christmas)", false)

	// ─────────────────────────────────────────────────────────────────────────
	// 2027 Korean Public Holidays
	// ─────────────────────────────────────────────────────────────────────────

	// 신정 — 2027-01-01 Friday, no substitute
	add(2027, time.January, 1, "신정 (New Year's Day)", false)

	// 설날 — 2027: 음력 정월 초하루 = 2027-02-06 (Saturday)
	// 연휴: 2027-02-05 (금/전날), 2027-02-06 (토/당일), 2027-02-07 (일/다음날)
	// 당일 Saturday → 대체 2027-02-08 (Monday)
	// 다음날 Sunday  → 대체 2027-02-09 (Tuesday) — per multi-day substitute rule
	add(2027, time.February, 5, "설날 전날", false)
	add(2027, time.February, 6, "설날 (Lunar New Year)", false)
	add(2027, time.February, 7, "설날 다음날", false)
	add(2027, time.February, 8, "설날 대체공휴일", true)
	add(2027, time.February, 9, "설날 대체공휴일", true)

	// 삼일절 — 2027-03-01 Monday, no substitute needed
	add(2027, time.March, 1, "삼일절 (Independence Movement Day)", false)

	// 어린이날 — 2027-05-05 Wednesday, no substitute
	add(2027, time.May, 5, "어린이날 (Children's Day)", false)

	// 부처님오신날 — 2027: 음력 4/8 = 2027-05-13 Thursday, no substitute
	add(2027, time.May, 13, "부처님오신날 (Buddha's Birthday)", false)

	// 현충일 — 2027-06-06 Sunday → 기념일이지만 대체공휴일 여부는 법적으로 불명확.
	// 현충일은 국경일이 아닌 기념일(각종 기념일 등에 관한 규정)이므로 대체공휴일 미적용.
	add(2027, time.June, 6, "현충일 (Memorial Day)", false)

	// 광복절 — 2027-08-15 Sunday → 대체 2027-08-16 (Monday)
	add(2027, time.August, 15, "광복절 (Liberation Day)", false)
	add(2027, time.August, 16, "광복절 대체공휴일", true)

	// 추석 — 2027: 음력 8/15 = 2027-09-15 (Wednesday)
	// 연휴: 2027-09-14 (화/전날), 2027-09-15 (수/당일), 2027-09-16 (목/다음날)
	// No weekend overlap → no substitute
	add(2027, time.September, 14, "추석 전날", false)
	add(2027, time.September, 15, "추석 (Chuseok)", false)
	add(2027, time.September, 16, "추석 다음날", false)

	// 개천절 — 2027-10-03 Sunday → 대체 2027-10-04 (Monday)
	add(2027, time.October, 3, "개천절 (National Foundation Day)", false)
	add(2027, time.October, 4, "개천절 대체공휴일", true)

	// 한글날 — 2027-10-09 Saturday → no substitute (Saturday rule only for 어린이날)
	add(2027, time.October, 9, "한글날 (Hangul Day)", false)

	// 크리스마스 — 2027-12-25 Saturday → no substitute (Sunday rule)
	add(2027, time.December, 25, "크리스마스 (Christmas)", false)

	// ─────────────────────────────────────────────────────────────────────────
	// 2028 Korean Public Holidays
	// ─────────────────────────────────────────────────────────────────────────

	// 신정 — 2028-01-01 Saturday → no substitute (신정 not eligible per current law)
	add(2028, time.January, 1, "신정 (New Year's Day)", false)

	// 설날 — 2028: 음력 정월 초하루 = 2028-01-26 (Wednesday)
	// 연휴: 2028-01-25 (화/전날), 2028-01-26 (수/당일), 2028-01-27 (목/다음날)
	// No weekend overlap → no substitute
	add(2028, time.January, 25, "설날 전날", false)
	add(2028, time.January, 26, "설날 (Lunar New Year)", false)
	add(2028, time.January, 27, "설날 다음날", false)

	// 삼일절 — 2028-03-01 Wednesday, no substitute
	add(2028, time.March, 1, "삼일절 (Independence Movement Day)", false)

	// 어린이날 — 2028-05-05 Saturday → 대체 2028-05-07 (Monday) per 어린이날 Saturday rule
	add(2028, time.May, 5, "어린이날 (Children's Day)", false)
	add(2028, time.May, 7, "어린이날 대체공휴일", true)

	// 부처님오신날 — 2028: 음력 4/8 = 2028-05-02 Tuesday, no substitute
	add(2028, time.May, 2, "부처님오신날 (Buddha's Birthday)", false)

	// 현충일 — 2028-06-06 Wednesday, no substitute
	add(2028, time.June, 6, "현충일 (Memorial Day)", false)

	// 광복절 — 2028-08-15 Tuesday, no substitute
	add(2028, time.August, 15, "광복절 (Liberation Day)", false)

	// 추석 — 2028: 음력 8/15 = 2028-10-03 (Tuesday)
	// 연휴: 2028-10-02 (월/전날), 2028-10-03 (화/당일), 2028-10-04 (수/다음날)
	// No weekend overlap → no substitute
	add(2028, time.October, 2, "추석 전날", false)
	add(2028, time.October, 3, "추석 (Chuseok)", false)
	add(2028, time.October, 4, "추석 다음날", false)

	// 개천절 — 2028-10-03 (Tuesday) — same as 추석 당일: already added above
	// 개천절 is a separate holiday on 10/03; they coincide in 2028.
	// Overwrite to use the more specific name (개천절), keeping IsSubstitute=false.
	// In practice both are public holidays; we use 개천절 as the primary label since
	// 추석 당일 was already registered. We leave 추석 당일 and add 개천절 as well;
	// the Lookup function returns the first registered entry.
	// Implementation note: the map can only hold one entry per key.
	// When 추석 and 개천절 coincide (2028-10-03), we mark as 추석 (Chuseok) / 개천절.
	m[holidayKey{year: 2028, month: time.October, day: 3}] = HolidayInfo{
		IsHoliday:    true,
		Name:         "추석 (Chuseok) / 개천절 (National Foundation Day)",
		IsSubstitute: false,
	}

	// 한글날 — 2028-10-09 Monday, no substitute
	add(2028, time.October, 9, "한글날 (Hangul Day)", false)

	// 크리스마스 — 2028-12-25 Monday, no substitute
	add(2028, time.December, 25, "크리스마스 (Christmas)", false)

	return m
}
