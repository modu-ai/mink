package locale

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectionSourceConstants(t *testing.T) {
	assert.Equal(t, DetectionSource("os"), SourceOS)
	assert.Equal(t, DetectionSource("ip"), SourceIPGeolocation)
	assert.Equal(t, DetectionSource("user_override"), SourceUserOverride)
	assert.Equal(t, DetectionSource("default"), SourceDefault)
}

func TestFormalityModeConstants(t *testing.T) {
	assert.Equal(t, FormalityMode("formal"), FormalityFormal)
	assert.Equal(t, FormalityMode("casual"), FormalityCasual)
}

func TestLocaleContextFields(t *testing.T) {
	lc := LocaleContext{
		Country:           "KR",
		PrimaryLanguage:   "ko-KR",
		Timezone:          "Asia/Seoul",
		Currency:          "KRW",
		MeasurementSystem: "metric",
		CalendarSystem:    "gregorian",
		DetectedMethod:    SourceOS,
	}

	assert.Equal(t, "KR", lc.Country)
	assert.Equal(t, "ko-KR", lc.PrimaryLanguage)
	assert.Equal(t, "Asia/Seoul", lc.Timezone)
	assert.Equal(t, "KRW", lc.Currency)
	assert.Equal(t, "metric", lc.MeasurementSystem)
	assert.Equal(t, "gregorian", lc.CalendarSystem)
	assert.Equal(t, SourceOS, lc.DetectedMethod)
	assert.Nil(t, lc.Conflict)
	assert.Empty(t, lc.SecondaryLanguage)
	assert.Empty(t, lc.TimezoneAlternatives)
}

func TestLocaleConflict(t *testing.T) {
	conflict := &LocaleConflict{OSCountry: "KR", IPCountry: "US"}
	lc := LocaleContext{
		Country:  "KR",
		Conflict: conflict,
	}
	assert.Equal(t, "KR", lc.Conflict.OSCountry)
	assert.Equal(t, "US", lc.Conflict.IPCountry)
}

func TestSentinelErrors(t *testing.T) {
	assert.NotNil(t, ErrNoOSLocale)
	assert.NotNil(t, ErrInvalidLocaleEnv)
	assert.NotNil(t, ErrGeoDBStale)
	assert.NotNil(t, ErrGeoDBAbsent)

	assert.Contains(t, ErrNoOSLocale.Error(), "locale:")
	assert.Contains(t, ErrInvalidLocaleEnv.Error(), "locale:")
}

func TestCulturalContextFields(t *testing.T) {
	cc := CulturalContext{
		FormalityDefault: FormalityFormal,
		HonorificSystem:  "korean_jondaetmal",
		NameOrder:        "family_first",
		AddressFormat:    "east_asian",
		WeekendDays:      []string{"Sat", "Sun"},
		FirstDayOfWeek:   "Monday",
		LegalFlags:       []string{"pipa"},
	}
	assert.Equal(t, FormalityFormal, cc.FormalityDefault)
	assert.Equal(t, "korean_jondaetmal", cc.HonorificSystem)
	assert.Equal(t, []string{"Sat", "Sun"}, cc.WeekendDays)
	assert.Equal(t, []string{"pipa"}, cc.LegalFlags)
}
