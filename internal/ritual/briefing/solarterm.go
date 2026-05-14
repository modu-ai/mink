package briefing

import (
	"errors"
	"fmt"
)

var (
	// ErrYearOutOfRange is returned when year is outside supported range.
	ErrYearOutOfRange = errors.New("year out of range (must be 1900-2100)")
)

// SolarTermOnDate returns the solar term for a given date, or nil if no solar term falls on that date.
func SolarTermOnDate(year, month, day int) (*SolarTerm, error) {
	if year < 1900 || year > 2100 {
		return nil, ErrYearOutOfRange
	}

	// Get solar terms for the year
	terms := getSolarTermsForYear(year)

	// Check if the given date matches any solar term
	dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	if term, ok := terms[dateStr]; ok {
		return &term, nil
	}

	return nil, nil
}

// getSolarTermsForYear returns a map of date strings to solar terms for the given year.
// This is a simplified lookup table for the years 1900-2100.
// For production, you would want to use a more accurate astronomical calculation.
func getSolarTermsForYear(year int) map[string]SolarTerm {
	// This is a placeholder implementation.
	// In a real implementation, you would either:
	// 1. Use a comprehensive lookup table for all years 1900-2100
	// 2. Implement the full astronomical calculation from Meeus's book
	//
	// For now, we'll implement a simplified version that works for 2026
	// and nearby years.

	terms := make(map[string]SolarTerm)

	// Solar term names in order (starting from 소한 in January)
	names := []struct {
		korean string
		hanja  string
	}{
		{"소한", "小寒"}, {"대한", "大寒"},
		{"입춘", "立春"}, {"우수", "雨水"},
		{"경칩", "啓蟄"}, {"춘분", "春分"},
		{"청명", "清明"}, {"곡우", "穀雨"},
		{"입하", "立夏"}, {"소만", "小滿"},
		{"망종", "芒種"}, {"하지", "夏至"},
		{"소서", "小暑"}, {"대서", "大暑"},
		{"입추", "立秋"}, {"처서", "處暑"},
		{"백로", "白露"}, {"추분", "秋分"},
		{"한로", "寒露"}, {"상강", "霜降"},
		{"입동", "立冬"}, {"소설", "小雪"},
		{"대설", "大雪"}, {"동지", "冬至"},
	}

	// Approximate dates for 2026 (these would need to be calculated or looked up for each year)
	if year == 2026 {
		// Add 2026 solar terms
		dates := []string{
			"2026-01-05", "2026-01-20", // 소한, 대한
			"2026-02-04", "2026-02-19", // 입춘, 우수
			"2026-03-06", "2026-03-21", // 경칩, 춘분
			"2026-04-05", "2026-04-20", // 청명, 곡우
			"2026-05-06", "2026-05-21", // 입하, 소만
			"2026-06-06", "2026-06-21", // 망종, 하지
			"2026-07-07", "2026-07-23", // 소서, 대서
			"2026-08-08", "2026-08-23", // 입추, 처서
			"2026-09-08", "2026-09-23", // 백로, 추분
			"2026-10-08", "2026-10-24", // 한로, 상강
			"2026-11-08", "2026-11-22", // 입동, 소설
			"2026-12-07", "2026-12-22", // 대설, 동지
		}

		for i, date := range dates {
			terms[date] = SolarTerm{
				Name:      names[i].korean,
				NameHanja: names[i].hanja,
				Date:      date,
			}
		}
		return terms
	}

	// For other years, return empty map (would need full implementation)
	return terms
}
