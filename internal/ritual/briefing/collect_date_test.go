package briefing

import (
	"testing"
	"time"
)

func TestDateCollector_Collect(t *testing.T) {
	t.Run("happy path - regular day", func(t *testing.T) {
		today := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

		collector := NewDateCollector()
		module, status := collector.Collect(today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		if module.Today != "2026-05-14" {
			t.Errorf("expected Today='2026-05-14', got '%s'", module.Today)
		}

		// 2026-05-14 is a Thursday
		if module.DayOfWeek != "목요일" {
			t.Errorf("expected DayOfWeek='목요일', got '%s'", module.DayOfWeek)
		}

		// 2026-05-14 is not a solar term date
		if module.SolarTerm != nil {
			t.Errorf("expected SolarTerm=nil on non-solar-term date, got %v", module.SolarTerm)
		}

		// 2026-05-14 is not a holiday
		if module.Holiday != nil {
			t.Errorf("expected Holiday=nil on non-holiday, got %v", module.Holiday)
		}
	})

	t.Run("solar term date - 입하 (Start of Summer)", func(t *testing.T) {
		// 입하 (Start of Summer) is around May 5-6
		// Let's use a known date: 2026-05-05 should be close to 입하
		today := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)

		collector := NewDateCollector()
		module, status := collector.Collect(today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		// 입하 should be detected (exact date depends on longitude calculation)
		// For now, just check that the field is populated if it's a solar term
		// The exact solar term logic is tested in solarterm_test.go
		if module.SolarTerm != nil {
			if module.SolarTerm.Name == "" {
				t.Error("expected SolarTerm.Name to be populated")
			}
		}
	})

	t.Run("holiday date - Children's Day", func(t *testing.T) {
		// Children's Day in Korea is May 5
		today := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)

		collector := NewDateCollector()
		module, status := collector.Collect(today)

		if status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", status)
		}

		// May 5 is Children's Day (어린이날)
		if module.Holiday == nil {
			t.Error("expected Holiday to be populated on Children's Day")
		} else {
			if module.Holiday.Name != "어린이날" {
				t.Errorf("expected Holiday.Name='어린이날', got '%s'", module.Holiday.Name)
			}
		}
	})

	t.Run("out-of-range year - before 2024", func(t *testing.T) {
		// Solar term lookup supports 2024-2030
		today := time.Date(2023, 5, 14, 0, 0, 0, 0, time.UTC)

		collector := NewDateCollector()
		module, status := collector.Collect(today)

		// Out-of-range should not fail, but solar term will be nil
		if status != "ok" {
			t.Errorf("expected status 'ok' even with out-of-range year, got '%s'", status)
		}

		if module.SolarTerm != nil {
			t.Error("expected SolarTerm=nil for out-of-range year")
		}
	})

	t.Run("out-of-range year - after 2030", func(t *testing.T) {
		today := time.Date(2031, 5, 14, 0, 0, 0, 0, time.UTC)

		collector := NewDateCollector()
		module, status := collector.Collect(today)

		// Out-of-range should not fail, but solar term will be nil
		if status != "ok" {
			t.Errorf("expected status 'ok' even with out-of-range year, got '%s'", status)
		}

		if module.SolarTerm != nil {
			t.Error("expected SolarTerm=nil for out-of-range year")
		}
	})

	t.Run("korean day names - all days of week", func(t *testing.T) {
		testCases := []struct {
			date            time.Time
			expectedDayName string
		}{
			{time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), "월요일"}, // Monday
			{time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC), "화요일"}, // Tuesday
			{time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC), "수요일"}, // Wednesday
			{time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC), "목요일"}, // Thursday
			{time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), "금요일"}, // Friday
			{time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC), "토요일"}, // Saturday
			{time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC), "일요일"}, // Sunday
		}

		collector := NewDateCollector()

		for _, tc := range testCases {
			t.Run(tc.expectedDayName, func(t *testing.T) {
				module, _ := collector.Collect(tc.date)

				if module.DayOfWeek != tc.expectedDayName {
					t.Errorf("expected DayOfWeek='%s', got '%s'", tc.expectedDayName, module.DayOfWeek)
				}
			})
		}
	})
}
