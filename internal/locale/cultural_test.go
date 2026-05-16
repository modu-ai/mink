package locale

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// TestResolveCulturalContext_KR tests AC-LC-006: Korean cultural context.
func TestResolveCulturalContext_KR(t *testing.T) {
	cc := ResolveCulturalContext("KR")
	assert.Equal(t, FormalityFormal, cc.FormalityDefault)
	assert.Equal(t, "korean_jondaetmal", cc.HonorificSystem)
	assert.Equal(t, "family_first", cc.NameOrder)
	assert.Equal(t, []string{"Sat", "Sun"}, cc.WeekendDays)
	assert.Equal(t, "Monday", cc.FirstDayOfWeek)
	assert.Equal(t, []string{"pipa"}, cc.LegalFlags)
}

// TestResolveCulturalContext_SA tests AC-LC-007: Saudi Arabia cultural context.
func TestResolveCulturalContext_SA(t *testing.T) {
	cc := ResolveCulturalContext("SA")
	assert.Equal(t, "arabic_formal_familiar", cc.HonorificSystem)
	assert.Equal(t, []string{"Fri", "Sat"}, cc.WeekendDays)
	assert.Equal(t, "Saturday", cc.FirstDayOfWeek)
	// calendar system is in the cultural entry but not in CulturalContext struct —
	// it is returned as part of LocaleContext.CalendarSystem via detectCalendarSystem.
}

// TestResolveCulturalContext_Determinism tests AC-LC-014: same input → same output 100x.
func TestResolveCulturalContext_Determinism(t *testing.T) {
	first := ResolveCulturalContext("KR")
	for i := 0; i < 99; i++ {
		cc := ResolveCulturalContext("KR")
		// Deep equality via YAML serialization (bytes must match).
		firstYAML, _ := yaml.Marshal(first)
		ccYAML, _ := yaml.Marshal(cc)
		assert.Equal(t, firstYAML, ccYAML, "iteration %d produced a different result", i+1)
	}
}

// TestResolveCulturalContext_SliceIsolation verifies that modifying the returned
// slices does not affect subsequent calls (aliasing prevention).
func TestResolveCulturalContext_SliceIsolation(t *testing.T) {
	cc1 := ResolveCulturalContext("KR")
	cc1.WeekendDays[0] = "MODIFIED"
	cc1.LegalFlags[0] = "MODIFIED"

	cc2 := ResolveCulturalContext("KR")
	assert.Equal(t, "Sat", cc2.WeekendDays[0], "slice aliasing detected in WeekendDays")
	assert.Equal(t, "pipa", cc2.LegalFlags[0], "slice aliasing detected in LegalFlags")
}

// TestResolveCulturalContext_Table tests a representative set of countries.
func TestResolveCulturalContext_Table(t *testing.T) {
	cases := []struct {
		country        string
		formality      FormalityMode
		honorific      string
		nameOrder      string
		firstDayOfWeek string
		hasLegalFlags  bool
	}{
		{"JP", FormalityFormal, "japanese_keigo", "family_first", "Monday", true},
		{"CN", FormalityFormal, "chinese_jing", "family_first", "Monday", true},
		{"US", FormalityCasual, "none", "given_first", "Sunday", true},
		{"DE", FormalityFormal, "german_sie_du", "given_first", "Monday", true},
		{"FR", FormalityFormal, "french_tu_vous", "given_first", "Monday", true},
		{"GB", FormalityCasual, "none", "given_first", "Monday", true},
		{"BR", FormalityFormal, "portuguese_senhor", "given_first", "Sunday", true},
		{"RU", FormalityFormal, "russian_vy", "given_first", "Monday", true},
		{"VN", FormalityFormal, "vietnamese_anh_em", "family_first", "Monday", true},
		{"TH", FormalityFormal, "thai_khun", "given_first", "Sunday", true},
		{"AU", FormalityCasual, "none", "given_first", "Monday", true},
	}

	for _, tc := range cases {
		t.Run(tc.country, func(t *testing.T) {
			cc := ResolveCulturalContext(tc.country)
			assert.Equal(t, tc.formality, cc.FormalityDefault, "formality mismatch for %s", tc.country)
			assert.Equal(t, tc.honorific, cc.HonorificSystem, "honorific mismatch for %s", tc.country)
			assert.Equal(t, tc.nameOrder, cc.NameOrder, "name_order mismatch for %s", tc.country)
			assert.Equal(t, tc.firstDayOfWeek, cc.FirstDayOfWeek, "first_day_of_week mismatch for %s", tc.country)
			if tc.hasLegalFlags {
				assert.NotEmpty(t, cc.LegalFlags, "expected legal flags for %s", tc.country)
			}
		})
	}
}

// TestResolveCulturalContext_Unknown verifies that unlisted countries use the default.
func TestResolveCulturalContext_Unknown(t *testing.T) {
	cc := ResolveCulturalContext("XX")
	assert.Equal(t, FormalityCasual, cc.FormalityDefault)
	assert.Equal(t, "none", cc.HonorificSystem)
	assert.Equal(t, "given_first", cc.NameOrder)
	assert.Empty(t, cc.LegalFlags)
}

// TestCountryToCurrency_Table tests AC-LC-001 currency derivation for 30 priority countries.
func TestCountryToCurrency_Table(t *testing.T) {
	cases := []struct {
		country  string
		currency string
	}{
		{"US", "USD"},
		{"KR", "KRW"},
		{"JP", "JPY"},
		{"CN", "CNY"},
		{"GB", "GBP"},
		{"DE", "EUR"},
		{"FR", "EUR"},
		{"IT", "EUR"},
		{"ES", "EUR"},
		{"NL", "EUR"},
		{"CA", "CAD"},
		{"AU", "AUD"},
		{"BR", "BRL"},
		{"MX", "MXN"},
		{"IN", "INR"},
		{"RU", "RUB"},
		{"ZA", "ZAR"},
		{"EG", "EGP"},
		{"SA", "SAR"},
		{"AE", "AED"},
		{"TR", "TRY"},
		{"ID", "IDR"},
		{"TH", "THB"},
		{"VN", "VND"},
		{"PH", "PHP"},
		{"SG", "SGD"},
		{"MY", "MYR"},
		{"NZ", "NZD"},
		{"CH", "CHF"},
		{"SE", "SEK"},
		{"NO", "NOK"},
	}

	for _, tc := range cases {
		t.Run(tc.country, func(t *testing.T) {
			got, ok := CountryToCurrency(tc.country)
			assert.True(t, ok, "expected true for known country %s", tc.country)
			assert.Equal(t, tc.currency, got)
		})
	}
}

// TestCountryToCurrency_Fallback verifies that unknown countries return USD with ok=false.
func TestCountryToCurrency_Fallback(t *testing.T) {
	got, ok := CountryToCurrency("XX")
	assert.False(t, ok)
	assert.Equal(t, "USD", got)
}

// TestPrimaryTimezone_SingleTZ verifies single-timezone country returns the correct zone.
func TestPrimaryTimezone_SingleTZ(t *testing.T) {
	cases := []struct {
		country string
		tz      string
	}{
		{"KR", "Asia/Seoul"},
		{"JP", "Asia/Tokyo"},
		{"DE", "Europe/Berlin"},
		{"FR", "Europe/Paris"},
		{"IN", "Asia/Kolkata"},
	}
	for _, tc := range cases {
		t.Run(tc.country, func(t *testing.T) {
			tz, ok := PrimaryTimezone(tc.country)
			assert.True(t, ok)
			assert.Equal(t, tc.tz, tz)
		})
	}
}

// TestPrimaryTimezone_MultiTZ verifies multi-timezone countries return the primary zone.
func TestPrimaryTimezone_MultiTZ(t *testing.T) {
	cases := []struct {
		country string
		primary string
	}{
		{"US", "America/New_York"},
		{"RU", "Europe/Moscow"},
		{"BR", "America/Sao_Paulo"},
		{"CA", "America/Toronto"},
		{"AU", "Australia/Sydney"},
	}
	for _, tc := range cases {
		t.Run(tc.country, func(t *testing.T) {
			tz, ok := PrimaryTimezone(tc.country)
			assert.True(t, ok)
			assert.Equal(t, tc.primary, tz)
		})
	}
}

// TestPrimaryTimezone_Unknown verifies unknown country returns UTC with ok=false.
func TestPrimaryTimezone_Unknown(t *testing.T) {
	tz, ok := PrimaryTimezone("XX")
	assert.False(t, ok)
	assert.Equal(t, "UTC", tz)
}

// TestTimezoneAlternatives_US verifies AC-LC-018: US has 6 alternatives.
func TestTimezoneAlternatives_US(t *testing.T) {
	alts := TimezoneAlternatives("US")
	assert.Len(t, alts, 6)
	assert.Contains(t, alts, "America/New_York")
	assert.Contains(t, alts, "America/Chicago")
	assert.Contains(t, alts, "America/Denver")
	assert.Contains(t, alts, "America/Los_Angeles")
	assert.Contains(t, alts, "America/Anchorage")
	assert.Contains(t, alts, "Pacific/Honolulu")
}

// TestTimezoneAlternatives_SingleTZ verifies single-timezone countries return nil.
func TestTimezoneAlternatives_SingleTZ(t *testing.T) {
	assert.Nil(t, TimezoneAlternatives("KR"))
	assert.Nil(t, TimezoneAlternatives("JP"))
	assert.Nil(t, TimezoneAlternatives("XX"))
}

// TestDetectMeasurementSystem verifies imperial vs metric classification.
func TestDetectMeasurementSystem(t *testing.T) {
	cases := []struct {
		country string
		system  string
	}{
		{"US", "imperial"},
		{"LR", "imperial"},
		{"MM", "imperial"},
		{"KR", "metric"},
		{"GB", "metric"},
		{"DE", "metric"},
		{"XX", "metric"},
	}
	for _, tc := range cases {
		t.Run(tc.country, func(t *testing.T) {
			assert.Equal(t, tc.system, detectMeasurementSystem(tc.country))
		})
	}
}

// TestDetectCalendarSystem verifies calendar system derivation.
func TestDetectCalendarSystem(t *testing.T) {
	assert.Equal(t, "gregorian", detectCalendarSystem("KR", "ko-KR"))
	assert.Equal(t, "gregorian", detectCalendarSystem("US", "en-US"))
	assert.Equal(t, "hijri", detectCalendarSystem("SA", "ar-SA"))
	assert.Equal(t, "thai_buddhist", detectCalendarSystem("TH", "th-TH"))
	assert.Equal(t, "gregorian", detectCalendarSystem("XX", "en-XX"))
}
