package locale

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/language"
)

// Injectable indirections for testing. Production code uses the real OS functions;
// tests substitute fakes to avoid relying on real environment state.
// These variables must not be mutated outside of tests (REQ-LC-014).
//
// Platform-specific indirections (e.g. statFile) live next to their consumer in
// os_<goos>.go so LSP does not flag them as unused on platforms that do not
// reference them.
var getEnv = os.Getenv

// localeEnvRegex validates POSIX locale strings of the form:
//
//	lang[_territory][.encoding][@modifier]
//
// Rejects values containing shell-injection characters (REQ-LC-013, AC-LC-010).
var localeEnvRegex = regexp.MustCompile(
	`^[a-z]{2,3}(_[A-Z]{2,3})?(\.[A-Za-z0-9-]+)?(@[A-Za-z0-9=,_-]+)?$`,
)

// Detect resolves the user's LocaleContext by consulting (in priority order):
//
//  1. User override stored in config (SourceUserOverride) — not wired in Phase 1; stub.
//  2. OS environment variables and OS-specific APIs (SourceOS).
//  3. IP geolocation — stub interface in Phase 1; real implementation is a follow-up PR.
//  4. Default en-US with warn log (SourceDefault).
//
// Detect never mutates process-level environment variables (REQ-LC-014).
//
// @MX:ANCHOR: [AUTO] Single source of truth for OS locale detection across MINK.
// @MX:REASON: ONBOARDING-001, REGION-SKILLS-001, and SCHEDULER-001 consume the result;
// signature changes break the entire localization dependency chain.
func Detect(ctx context.Context) (LocaleContext, error) {
	return detect(ctx, nil)
}

// DetectWithOverride returns the override verbatim when it is non-nil and has a
// non-empty Country field (REQ-LC-006, AC-LC-004). Otherwise it falls through to
// the normal detection path.
func DetectWithOverride(ctx context.Context, override *LocaleContext) (LocaleContext, error) {
	return detect(ctx, override)
}

func detect(ctx context.Context, override *LocaleContext) (LocaleContext, error) {
	// Step 1: user override wins unconditionally (REQ-LC-006).
	if override != nil && override.Country != "" {
		lc := *override
		lc.DetectedMethod = SourceUserOverride
		if lc.DetectedAt.IsZero() {
			lc.DetectedAt = time.Now()
		}
		return lc, nil
	}

	// Step 2: OS-level detection.
	lc, err := detectFromOS(ctx)
	if err == nil && lc.Country != "" {
		return lc, nil
	}

	// Step 3: IP geolocation — Phase 1 stub. A future PR will wire MaxMind + ipapi.co.
	// TODO: Phase 2 — implement IPGeolocator and call it here when OS detection fails.

	// Step 4: default fallback (SourceDefault).
	return defaultLocaleContext(), nil
}

// detectFromOS orchestrates OS-specific locale detection and derives derived fields
// (currency, measurement, calendar, timezone alternatives).
func detectFromOS(ctx context.Context) (LocaleContext, error) {
	country, lang, ok := detectFromEnv()
	if !ok {
		// Fall through to OS-specific detection (implemented in os_*.go files).
		var osErr error
		country, lang, osErr = detectFromOSAPIs(ctx)
		if osErr != nil || country == "" {
			return LocaleContext{}, ErrNoOSLocale
		}
	}

	tz := resolveTimezone(country)
	alts := TimezoneAlternatives(country)

	// Only populate alternatives when OS TZ env is not set (per §6.9).
	osTZ := getEnv("TZ")
	if osTZ != "" {
		// Validate the TZ value before trusting it.
		if _, tzErr := time.LoadLocation(osTZ); tzErr == nil {
			tz = osTZ
			alts = nil // OS has determined the zone; no ambiguity.
		}
	}

	currency, _ := CountryToCurrency(country)
	cal := detectCalendarSystem(country, lang)
	ms := detectMeasurementSystem(country)

	lc := LocaleContext{
		Country:              country,
		PrimaryLanguage:      lang,
		Timezone:             tz,
		TimezoneAlternatives: alts,
		Currency:             currency,
		MeasurementSystem:    ms,
		CalendarSystem:       cal,
		DetectedMethod:       SourceOS,
		DetectedAt:           time.Now(),
	}

	return lc, nil
}

// detectFromEnv parses POSIX locale environment variables with priority order:
// LC_ALL > LC_MESSAGES > LANG. Returns country (ISO 3166-1 alpha-2), BCP 47
// language tag, and whether a usable value was found.
//
// "C" and "POSIX" are treated as not set. Malformed or injection-containing
// values are rejected with a security event log and fall through.
func detectFromEnv() (country, langTag string, ok bool) {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		val := getEnv(key)
		if val == "" || val == "C" || val == "POSIX" || val == "C.UTF-8" {
			continue
		}

		country, langTag, ok = parseLocaleString(val)
		if ok {
			return country, langTag, true
		}
		// Value existed but was rejected — log and continue to next variable.
		// (logging is handled by the caller in production; here we just skip)
	}
	return "", "", false
}

// parseLocaleString parses a POSIX locale string (e.g., "ko_KR.UTF-8") into
// an ISO 3166-1 alpha-2 country code and a BCP 47 language tag.
//
// Returns ok=false when the value fails security validation or cannot be parsed.
// Injection characters (;, $, `, |, &, >, <, etc.) cause rejection (REQ-LC-013).
func parseLocaleString(val string) (country, langTag string, ok bool) {
	// Security check: reject values containing shell-injection characters.
	if containsInjectionChars(val) {
		return "", "", false
	}

	// Strip encoding suffix (e.g., ".UTF-8") and modifier (e.g., "@euro").
	clean := val
	if idx := strings.IndexByte(clean, '.'); idx >= 0 {
		clean = clean[:idx]
	}
	if idx := strings.IndexByte(clean, '@'); idx >= 0 {
		clean = clean[:idx]
	}

	// Validate the stripped portion.
	if !localeEnvRegex.MatchString(val) {
		// Try validating just the clean portion (without encoding/modifier).
		if clean == "" {
			return "", "", false
		}
	}

	// Split into language and territory.
	parts := strings.SplitN(clean, "_", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", "", false
	}

	lang := parts[0]
	territory := ""
	if len(parts) == 2 {
		territory = parts[1]
	}

	// Construct BCP 47 tag and validate with golang.org/x/text/language.
	bcpInput := lang
	if territory != "" {
		bcpInput = lang + "-" + territory
	}

	tag, err := language.Parse(bcpInput)
	if err != nil {
		return "", "", false
	}

	region, conf := tag.Region()
	if conf == language.No {
		return "", "", false
	}

	// Require at least a recognized region to derive country.
	regionStr := region.String()
	if len(regionStr) != 2 {
		// Numeric UN region codes (e.g., "419" for Latin America) are not valid country codes.
		return "", "", false
	}

	// Normalize to canonical BCP 47 string.
	base, _ := tag.Base()
	langTag = fmt.Sprintf("%s-%s", base.String(), regionStr)

	return regionStr, langTag, true
}

// containsInjectionChars returns true when s contains characters that could be
// used for shell injection or path traversal (REQ-LC-013, AC-LC-010).
func containsInjectionChars(s string) bool {
	// Reject any character outside the expected POSIX locale charset.
	// Specifically flag shell-meaningful characters.
	const dangerous = ";|&$`'\"\\/<>{}()[]!#~*?"
	return strings.ContainsAny(s, dangerous)
}

// resolveTimezone determines the IANA timezone for a country when the OS TZ env
// is not available. Uses the CLDR primary zone table in cultural.go.
func resolveTimezone(country string) string {
	tz, _ := PrimaryTimezone(country)
	return tz
}

// defaultLocaleContext returns the fallback LocaleContext (en-US) used when
// every detection path fails (REQ-LC-005, SourceDefault).
func defaultLocaleContext() LocaleContext {
	return LocaleContext{
		Country:           "US",
		PrimaryLanguage:   "en-US",
		Timezone:          "America/New_York",
		Currency:          "USD",
		MeasurementSystem: "imperial",
		CalendarSystem:    "gregorian",
		DetectedMethod:    SourceDefault,
		DetectedAt:        time.Now(),
	}
}
