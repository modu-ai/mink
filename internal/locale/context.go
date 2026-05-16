package locale

import (
	"errors"
	"time"
)

// DetectionSource identifies how a LocaleContext was obtained.
type DetectionSource string

const (
	// SourceOS means the locale was resolved from OS environment variables or APIs.
	SourceOS DetectionSource = "os"
	// SourceIPGeolocation means the locale was resolved via IP geolocation (MaxMind or ipapi.co).
	SourceIPGeolocation DetectionSource = "ip"
	// SourceUserOverride means a user-supplied override in config.yaml was returned verbatim.
	SourceUserOverride DetectionSource = "user_override"
	// SourceDefault means every detection path failed and the en-US default was used.
	SourceDefault DetectionSource = "default"
)

// FormalityMode describes the default register used when addressing the user.
type FormalityMode string

const (
	// FormalityFormal uses polite/formal register by default (e.g., Korean 존댓말, German Sie).
	FormalityFormal FormalityMode = "formal"
	// FormalityCasual uses first-name or informal register by default (e.g., US/AU/GB).
	FormalityCasual FormalityMode = "casual"
)

// LocaleConflict records a discrepancy between OS-detected and IP-detected country.
// ONBOARDING-001 reads this field to surface a disambiguation dialog.
type LocaleConflict struct {
	// OSCountry is the ISO 3166-1 alpha-2 country resolved from OS APIs.
	OSCountry string `yaml:"os" json:"os"`
	// IPCountry is the ISO 3166-1 alpha-2 country resolved from IP geolocation.
	IPCountry string `yaml:"ip" json:"ip"`
}

// Accuracy indicates how a LocaleContext was sourced.
//
// - "high":   browser GPS + reverse geocoding (city-level)
// - "medium": IP geolocation (country-level)
// - "manual": user explicit override (4-preset radio or free-form)
// - "":       legacy data (pre-amendment-v0.2) — treated as manual by readers
//
// SPEC: SPEC-MINK-LOCALE-001 amendment-v0.2 §6.11
type Accuracy string

const (
	// AccuracyHigh indicates browser GPS + reverse geocoding was used (city-level).
	AccuracyHigh Accuracy = "high"
	// AccuracyMedium indicates IP geolocation was used (country-level).
	AccuracyMedium Accuracy = "medium"
	// AccuracyManual indicates a user explicit override was applied.
	AccuracyManual Accuracy = "manual"
)

// LocaleContext is the canonical per-user locale state produced by Detect().
// All fields are determined at detection time and persisted via the locale:
// section of ~/.mink/config.yaml.
//
// @MX:ANCHOR: [AUTO] Canonical cross-package locale data type — see package doc for change policy.
// @MX:REASON: Persisted to config.yaml; field renames break existing installations.
type LocaleContext struct {
	// Country is the ISO 3166-1 alpha-2 code (e.g., "KR", "JP", "US").
	Country string `yaml:"country" json:"country"`

	// PrimaryLanguage is the BCP 47 tag for the user's primary language (e.g., "ko-KR").
	PrimaryLanguage string `yaml:"primary_language" json:"primary_language"`

	// SecondaryLanguage is an optional BCP 47 tag for a secondary language (e.g., "en-US").
	// Non-empty only when set explicitly via user override or ONBOARDING-001 input.
	SecondaryLanguage string `yaml:"secondary_language,omitempty" json:"secondary_language,omitempty"`

	// Timezone is an IANA Time Zone Database identifier (e.g., "Asia/Seoul").
	Timezone string `yaml:"timezone" json:"timezone"`

	// TimezoneAlternatives lists all IANA zones for the detected country when the
	// country has multiple time zones and OS TZ env was not set. Populated only for
	// multi-timezone countries (US, RU, BR, CA, AU). ONBOARDING-001 uses this for
	// disambiguation; absence indicates a single-timezone country or OS-determined zone.
	TimezoneAlternatives []string `yaml:"timezone_alternatives,omitempty" json:"timezone_alternatives,omitempty"`

	// Currency is the ISO 4217 code (e.g., "KRW", "USD", "EUR").
	Currency string `yaml:"currency" json:"currency"`

	// MeasurementSystem is "metric" or "imperial". US/LR/MM → imperial; all others → metric.
	MeasurementSystem string `yaml:"measurement_system" json:"measurement_system"`

	// CalendarSystem is the primary calendar in use (e.g., "gregorian", "hijri", "thai_buddhist").
	CalendarSystem string `yaml:"calendar_system" json:"calendar_system"`

	// DetectedMethod records the source of the resolved context.
	DetectedMethod DetectionSource `yaml:"detected_method" json:"detected_method"`

	// Conflict is non-nil when OS country and IP country disagree.
	// ONBOARDING-001 reads this to offer a disambiguation dialog.
	Conflict *LocaleConflict `yaml:"conflict,omitempty" json:"conflict,omitempty"`

	// DetectedAt records when Detect() was called.
	// omitzero (Go 1.24+) is required because time.Time is a nested struct and
	// omitempty alone has no effect on it.
	DetectedAt time.Time `yaml:"detected_at,omitempty" json:"detected_at,omitzero"`

	// Accuracy indicates how the LocaleContext was sourced (amendment-v0.2 §6.11).
	// Empty string means legacy data (pre-amendment-v0.2); readers treat it as manual.
	Accuracy Accuracy `yaml:"accuracy,omitempty" json:"accuracy,omitempty"`
}

// CulturalContext is derived deterministically from a LocaleContext.Country value
// via the static countryToCultural mapping table in cultural.go.
// Identical country inputs always produce identical CulturalContext outputs (REQ-LC-003).
type CulturalContext struct {
	// FormalityDefault is the default register for addressing this user.
	FormalityDefault FormalityMode `yaml:"formality_default" json:"formality_default"`

	// HonorificSystem identifies the honorific tradition in use (e.g., "korean_jondaetmal").
	// The LLM applies the appropriate grammatical forms; this field provides the instruction.
	HonorificSystem string `yaml:"honorific_system" json:"honorific_system"`

	// NameOrder is "given_first" (e.g., US) or "family_first" (e.g., KR, JP).
	NameOrder string `yaml:"name_order" json:"name_order"`

	// AddressFormat is "western" or "east_asian" (address rendered smallest-to-largest unit).
	AddressFormat string `yaml:"address_format" json:"address_format"`

	// WeekendDays lists the days considered weekend (e.g., ["Sat", "Sun"], ["Fri", "Sat"]).
	WeekendDays []string `yaml:"weekend_days" json:"weekend_days"`

	// FirstDayOfWeek is "Sunday", "Monday", or "Saturday" per ISO 8601 / CLDR weekData.
	FirstDayOfWeek string `yaml:"first_day_of_week" json:"first_day_of_week"`

	// LegalFlags lists applicable privacy legal frameworks (e.g., "gdpr", "pipa", "ccpa").
	// These are hints; actual compliance logic lives in the consuming SPEC.
	LegalFlags []string `yaml:"legal_flags" json:"legal_flags"`
}

// Sentinel errors returned by locale package functions.
var (
	// ErrNoOSLocale indicates that no usable OS locale could be detected from
	// environment variables or OS-specific APIs.
	ErrNoOSLocale = errors.New("locale: no OS locale detected")

	// ErrInvalidLocaleEnv indicates that an environment variable value failed
	// security validation (e.g., contains shell injection characters).
	ErrInvalidLocaleEnv = errors.New("locale: environment variable failed security validation")

	// ErrGeoDBStale indicates that the MaxMind GeoLite2 DB is older than 90 days.
	ErrGeoDBStale = errors.New("locale: GeoIP DB is stale (>90 days)")

	// ErrGeoDBAbsent indicates that the MaxMind GeoLite2 DB file was not found.
	ErrGeoDBAbsent = errors.New("locale: GeoIP DB file not found")
)
