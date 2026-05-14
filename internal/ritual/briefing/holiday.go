package briefing

import (
	"errors"
	"fmt"
)

var (
	// ErrYearOutOfRangeForHoliday is returned when year is outside supported range for holidays.
	ErrYearOutOfRangeForHoliday = errors.New("year out of range (must be 1900-2100)")
)

// LookupKoreanHoliday returns the Korean holiday for a given date, or nil if no holiday falls on that date.
func LookupKoreanHoliday(year, month, day int) (*KoreanHoliday, error) {
	if year < 1900 || year > 2100 {
		return nil, ErrYearOutOfRangeForHoliday
	}

	// Get holidays for the year
	holidays := getKoreanHolidaysForYear(year)

	// Check if the given date matches any holiday
	dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	if holiday, ok := holidays[dateStr]; ok {
		return &holiday, nil
	}

	return nil, nil
}

// getKoreanHolidaysForYear returns a map of date strings to Korean holidays for the given year.
// This is a simplified lookup table for the years 1900-2100.
// For production, you would want to use a comprehensive lunar calendar conversion.
func getKoreanHolidaysForYear(year int) map[string]KoreanHoliday {
	holidays := make(map[string]KoreanHoliday)

	// Solar holidays (fixed Gregorian dates)
	solarHolidays := []struct {
		month     int
		day       int
		name      string
		nameHanja string
	}{
		{1, 1, "신정", "新年"},
		{3, 1, "삼일절", "三一節"},
		{5, 5, "어린이날", "兒童節"},
		{6, 6, "현충일", "顯忠日"},
		{8, 15, "광복절", "光復節"},
		{10, 3, "개천절", "開天節"},
		{10, 9, "한글날", "韓글날"},
		{12, 25, "성탄절", "聖誕節"},
	}

	for _, h := range solarHolidays {
		date := fmt.Sprintf("%04d-%02d-%02d", year, h.month, h.day)
		holidays[date] = KoreanHoliday{
			Name:      h.name,
			NameHanja: h.nameHanja,
			Date:      date,
			IsHoliday: true,
		}
	}

	// Lunar holidays (simplified - would need lunar calendar conversion for accuracy)
	// For 2026, here are the approximate dates:
	if year == 2026 {
		lunarHolidays := []struct {
			month     int
			day       int
			name      string
			nameHanja string
		}{
			{2, 16, "설날", "春節"},       // Lunar New Year's Eve
			{2, 17, "설날", "春節"},       // Lunar New Year's Day
			{2, 18, "설날", "春節"},       // Lunar New Year's Day 2
			{2, 12, "정월대보름", "正月大보름"}, // Lantern Festival
			{5, 24, "석가탄신일", "釋迦誕辰日"}, // Buddha's Birthday
			{9, 20, "추석", "秋夕"},       // Chuseok Eve
			{9, 21, "추석", "秋夕"},       // Chuseok Day
			{9, 22, "추석", "秋夕"},       // Chuseok Day 2
		}

		for _, h := range lunarHolidays {
			date := fmt.Sprintf("%04d-%02d-%02d", year, h.month, h.day)
			holidays[date] = KoreanHoliday{
				Name:      h.name,
				NameHanja: h.nameHanja,
				Date:      date,
				IsHoliday: true,
			}
		}
	}

	return holidays
}
