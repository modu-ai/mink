package locale

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_Override_Present verifies that a locale.override section is parsed correctly.
func TestLoad_Override_Present(t *testing.T) {
	yaml := `
locale:
  override:
    country: JP
    primary_language: ja-JP
    timezone: Asia/Tokyo
    currency: JPY
    measurement_system: metric
    calendar_system: gregorian
    detected_method: user_override
  geolocation_enabled: false
  geoip_db_path: /tmp/geo.mmdb
`
	lc, err := Load(strings.NewReader(yaml))
	require.NoError(t, err)
	assert.Equal(t, "JP", lc.Country)
	assert.Equal(t, "ja-JP", lc.PrimaryLanguage)
	assert.Equal(t, "Asia/Tokyo", lc.Timezone)
	assert.Equal(t, "JPY", lc.Currency)
	assert.Equal(t, "metric", lc.MeasurementSystem)
	assert.Equal(t, "gregorian", lc.CalendarSystem)
	assert.Equal(t, DetectionSource("user_override"), lc.DetectedMethod)
}

// TestLoad_NoOverride verifies that absent override returns zero-value LocaleContext.
func TestLoad_NoOverride(t *testing.T) {
	yaml := `
locale:
  geolocation_enabled: true
`
	lc, err := Load(strings.NewReader(yaml))
	require.NoError(t, err)
	assert.Empty(t, lc.Country)
}

// TestLoad_EmptyYAML verifies that an empty reader returns zero-value without error.
func TestLoad_EmptyYAML(t *testing.T) {
	lc, err := Load(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, lc.Country)
}

// TestLoad_MissingLocaleKey verifies that a config without `locale:` key is fine.
func TestLoad_MissingLocaleKey(t *testing.T) {
	yaml := `
other_section:
  foo: bar
`
	lc, err := Load(strings.NewReader(yaml))
	require.NoError(t, err)
	assert.Empty(t, lc.Country)
}

// TestLoad_PartialFields verifies that partial override fields decode correctly.
func TestLoad_PartialFields(t *testing.T) {
	yaml := `
locale:
  override:
    country: KR
    primary_language: ko-KR
`
	lc, err := Load(strings.NewReader(yaml))
	require.NoError(t, err)
	assert.Equal(t, "KR", lc.Country)
	assert.Equal(t, "ko-KR", lc.PrimaryLanguage)
	assert.Empty(t, lc.Timezone) // not set
}

// TestSave_RoundTrip verifies that Save produces YAML that Load can read back.
func TestSave_RoundTrip(t *testing.T) {
	original := LocaleContext{
		Country:           "KR",
		PrimaryLanguage:   "ko-KR",
		Timezone:          "Asia/Seoul",
		Currency:          "KRW",
		MeasurementSystem: "metric",
		CalendarSystem:    "gregorian",
		DetectedMethod:    SourceOS,
		DetectedAt:        time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
	}

	var buf bytes.Buffer
	err := Save(&buf, original)
	require.NoError(t, err)
	require.NotEmpty(t, buf.String())

	loaded, err := Load(&buf)
	require.NoError(t, err)
	assert.Equal(t, original.Country, loaded.Country)
	assert.Equal(t, original.PrimaryLanguage, loaded.PrimaryLanguage)
	assert.Equal(t, original.Timezone, loaded.Timezone)
	assert.Equal(t, original.Currency, loaded.Currency)
	assert.Equal(t, original.MeasurementSystem, loaded.MeasurementSystem)
	assert.Equal(t, original.CalendarSystem, loaded.CalendarSystem)
	assert.Equal(t, original.DetectedMethod, loaded.DetectedMethod)
}

// TestSave_OutputContainsLocaleKey verifies that the YAML output has `locale:` root key.
func TestSave_OutputContainsLocaleKey(t *testing.T) {
	lc := LocaleContext{Country: "US", PrimaryLanguage: "en-US"}
	var buf bytes.Buffer
	err := Save(&buf, lc)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "locale:")
	assert.Contains(t, buf.String(), "override:")
	assert.Contains(t, buf.String(), "country: US")
}

// TestLoadConfig_AllFields verifies that LoadConfig reads all config fields.
func TestLoadConfig_AllFields(t *testing.T) {
	yaml := `
locale:
  override:
    country: JP
    primary_language: ja-JP
  geolocation_enabled: false
  geoip_db_path: /tmp/geo.mmdb
`
	cfg, err := LoadConfig(strings.NewReader(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Override)
	assert.Equal(t, "JP", cfg.Override.Country)
	require.NotNil(t, cfg.GeolocationEnabled)
	assert.False(t, *cfg.GeolocationEnabled)
	assert.Equal(t, "/tmp/geo.mmdb", cfg.GeoIPDBPath)
}

// TestLoadConfig_GeolocationEnabled_True verifies bool field parsing.
func TestLoadConfig_GeolocationEnabled_True(t *testing.T) {
	yaml := `
locale:
  geolocation_enabled: true
`
	cfg, err := LoadConfig(strings.NewReader(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.GeolocationEnabled)
	assert.True(t, *cfg.GeolocationEnabled)
}

// TestLoad_LegacyData_EmptyAccuracy verifies that legacy data (without the accuracy
// field) unmarshals without panic and leaves Accuracy as the zero value ("").
// This ensures backward compatibility for pre-amendment-v0.2 installations.
func TestLoad_LegacyData_EmptyAccuracy(t *testing.T) {
	// Legacy YAML has no accuracy field at all.
	yamlData := `
locale:
  override:
    country: KR
    primary_language: ko-KR
    timezone: Asia/Seoul
    currency: KRW
    measurement_system: metric
    calendar_system: gregorian
    detected_method: user_override
`
	lc, err := Load(strings.NewReader(yamlData))
	require.NoError(t, err)
	assert.Equal(t, "KR", lc.Country)
	// Legacy data must decode without panic and Accuracy must be the zero value.
	assert.Equal(t, Accuracy(""), lc.Accuracy, "legacy data must have empty Accuracy, not panic")
}

// TestSave_RoundTrip_WithAccuracy verifies that Accuracy survives a Save→Load round-trip.
func TestSave_RoundTrip_WithAccuracy(t *testing.T) {
	original := LocaleContext{
		Country:           "KR",
		PrimaryLanguage:   "ko-KR",
		Timezone:          "Asia/Seoul",
		Currency:          "KRW",
		MeasurementSystem: "metric",
		CalendarSystem:    "gregorian",
		DetectedMethod:    SourceOS,
		Accuracy:          AccuracyMedium,
	}

	var buf bytes.Buffer
	err := Save(&buf, original)
	require.NoError(t, err)

	loaded, err := Load(&buf)
	require.NoError(t, err)
	assert.Equal(t, AccuracyMedium, loaded.Accuracy, "Accuracy must survive Save→Load round-trip")
}

// TestLoad_WithTimezoneAlternatives verifies round-trip of timezone_alternatives.
func TestLoad_WithTimezoneAlternatives(t *testing.T) {
	lc := LocaleContext{
		Country:              "US",
		PrimaryLanguage:      "en-US",
		Timezone:             "America/New_York",
		Currency:             "USD",
		MeasurementSystem:    "imperial",
		CalendarSystem:       "gregorian",
		DetectedMethod:       SourceOS,
		TimezoneAlternatives: []string{"America/New_York", "America/Chicago"},
	}

	var buf bytes.Buffer
	err := Save(&buf, lc)
	require.NoError(t, err)

	loaded, err := Load(&buf)
	require.NoError(t, err)
	assert.Equal(t, []string{"America/New_York", "America/Chicago"}, loaded.TimezoneAlternatives)
}
