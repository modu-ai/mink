package locale

// reverse.go provides GPS coordinate reverse geocoding via Nominatim (OpenStreetMap).
//
// SPEC: SPEC-MINK-LOCALE-001 amendment-v0.2 Item 4
// REQ: REQ-LC-020 (high-accuracy GPS path)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Sentinel errors returned by ReverseGeocode.
var (
	// ErrReverseTimeout is returned when the Nominatim call exceeds 3s.
	ErrReverseTimeout = errors.New("locale: reverse geocode timeout")

	// ErrReverseHTTP is returned when Nominatim responds with a non-2xx status code.
	ErrReverseHTTP = errors.New("locale: reverse geocode non-2xx response")

	// ErrReverseParse is returned when the response body cannot be decoded as JSON
	// or when the required country_code field is absent.
	ErrReverseParse = errors.New("locale: reverse geocode response decode failed")

	// ErrReverseInvalidCoords is returned when lat/lng fall outside valid ranges.
	// No external HTTP call is made when this error is returned.
	ErrReverseInvalidCoords = errors.New("locale: invalid lat/lng coordinates")
)

// reverseGeocodeHTTPClient is the HTTP client used by ReverseGeocode. Tests may
// replace this variable with a custom client (pointing to an httptest.Server)
// before calling ReverseGeocode. The replacement must be restored via t.Cleanup.
//
// @MX:WARN: [AUTO] Package-level mutable variable used for test injection of HTTP transport.
// @MX:REASON: Replacing a package-level var is the standard Go pattern for injecting
// HTTP transport in unit tests without changing the exported function signature.
// Mirror of ipLookupHTTPClient in iplookup.go.
var reverseGeocodeHTTPClient = &http.Client{Timeout: 3 * time.Second}

// reverseGeocodeBaseURL is the base URL for the Nominatim reverse geocoding endpoint.
// Tests may replace this to point at an httptest.Server. Must be restored via t.Cleanup.
//
// @MX:WARN: [AUTO] Package-level mutable variable used for Nominatim base URL injection in tests.
// @MX:REASON: Same test-injection pattern as ipAPIBaseURL; allows offline unit tests without
// hitting the real Nominatim service.
var reverseGeocodeBaseURL = "https://nominatim.openstreetmap.org"

// nominatimResponse mirrors the relevant fields from a Nominatim reverse geocoding
// JSON response. Only the address sub-object is needed.
type nominatimResponse struct {
	Address nominatimAddress `json:"address"`
}

// nominatimAddress holds the address fields returned by Nominatim.
type nominatimAddress struct {
	// CountryCode is the ISO 3166-1 alpha-2 code in lowercase (e.g., "kr").
	// Must be uppercased before returning to callers.
	CountryCode string `json:"country_code"`
}

// ReverseGeocodeInput holds the GPS coordinates to reverse-geocode.
type ReverseGeocodeInput struct {
	// Lat is the WGS-84 latitude in decimal degrees (-90.0 to +90.0).
	Lat float64

	// Lng is the WGS-84 longitude in decimal degrees (-180.0 to +180.0).
	Lng float64
}

// ReverseGeocodeResult contains the locale information derived from GPS coordinates.
type ReverseGeocodeResult struct {
	// Country is the ISO 3166-1 alpha-2 code in UPPERCASE (e.g., "KR").
	Country string

	// Language is the BCP 47 primary language tag (best-effort; empty when
	// the country is not in the built-in countryToLanguage table).
	Language string

	// Timezone is the IANA timezone identifier derived via PrimaryTimezone().
	// Falls back to "UTC" for unknown countries (matching PrimaryTimezone behaviour).
	Timezone string
}

// countryToLanguage maps a small set of ISO 3166-1 alpha-2 country codes to their
// primary BCP 47 language tag. Coverage targets the four MINK locale presets
// (KR/US/FR/DE) plus common countries. Unlisted countries return Language="".
var countryToLanguage = map[string]string{
	"KR": "ko",
	"JP": "ja",
	"CN": "zh",
	"US": "en",
	"GB": "en",
	"AU": "en",
	"CA": "en",
	"NZ": "en",
	"SG": "en",
	"ZA": "en",
	"DE": "de",
	"AT": "de",
	"CH": "de", // simplified: de is the plurality language
	"FR": "fr",
	"BE": "fr", // simplified: majority language
	"ES": "es",
	"MX": "es",
	"AR": "es",
	"IT": "it",
	"PT": "pt",
	"BR": "pt",
	"RU": "ru",
	"PL": "pl",
	"NL": "nl",
	"SE": "sv",
	"NO": "no",
	"TR": "tr",
	"IN": "hi",
	"SA": "ar",
	"AE": "ar",
	"EG": "ar",
	"TH": "th",
	"VN": "vi",
	"ID": "id",
	"PH": "fil",
	"MY": "ms",
	"IL": "he",
}

// primaryLanguageForCountry returns the best-effort BCP 47 language tag for the
// given uppercase ISO 3166-1 alpha-2 country code. Returns "" for unknown countries.
func primaryLanguageForCountry(country string) string {
	return countryToLanguage[country]
}

// ReverseGeocode converts GPS coordinates to a country, language, and timezone
// using the Nominatim OpenStreetMap reverse geocoding API.
//
// Validation errors (invalid lat/lng) are returned without making any HTTP call.
// Timeout, non-2xx, and parse failures each return the corresponding sentinel error.
//
// @MX:ANCHOR: [AUTO] GPS reverse geocoding entry point for LOCALE-001 amendment-v0.2 high-accuracy path.
// @MX:REASON: Called by handler.go /install/api/locale/probe GPS branch; indirected via
// reverseGeocodeHTTPClient and reverseGeocodeBaseURL for test injection.
func ReverseGeocode(ctx context.Context, in ReverseGeocodeInput) (ReverseGeocodeResult, error) {
	// Validate coordinate ranges before any I/O.
	if in.Lat < -90.0 || in.Lat > 90.0 {
		return ReverseGeocodeResult{}, ErrReverseInvalidCoords
	}
	if in.Lng < -180.0 || in.Lng > 180.0 {
		return ReverseGeocodeResult{}, ErrReverseInvalidCoords
	}

	// Build the Nominatim reverse geocoding URL.
	// zoom=3 returns country-level granularity (sufficient for locale detection).
	url := fmt.Sprintf(
		"%s/reverse?format=json&lat=%g&lon=%g&zoom=3&accept-language=en",
		reverseGeocodeBaseURL, in.Lat, in.Lng,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ReverseGeocodeResult{}, fmt.Errorf("%w: build request: %v", ErrReverseHTTP, err)
	}

	// Nominatim usage policy requires a descriptive User-Agent.
	req.Header.Set("User-Agent", "MINK/0.3 (https://github.com/modu-ai/mink)")
	req.Header.Set("Accept", "application/json")

	resp, err := reverseGeocodeHTTPClient.Do(req)
	if err != nil {
		// Distinguish context deadline from HTTP client-level timeout.
		if ctx.Err() != nil {
			return ReverseGeocodeResult{}, ErrReverseTimeout
		}
		// http.Client.Timeout fires as a url.Error with Timeout() == true.
		return ReverseGeocodeResult{}, ErrReverseTimeout
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ReverseGeocodeResult{}, fmt.Errorf("%w: status %d", ErrReverseHTTP, resp.StatusCode)
	}

	var body nominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ReverseGeocodeResult{}, fmt.Errorf("%w: %v", ErrReverseParse, err)
	}

	// country_code is required; an empty value means Nominatim could not resolve
	// the coordinates to a country (e.g., open ocean). Treat as a parse failure.
	if strings.TrimSpace(body.Address.CountryCode) == "" {
		return ReverseGeocodeResult{}, fmt.Errorf("%w: empty country_code in response", ErrReverseParse)
	}

	country := strings.ToUpper(body.Address.CountryCode)

	// Derive IANA timezone via the existing PrimaryTimezone lookup (cultural.go).
	// Falls back to "UTC" for unknown countries.
	tz, _ := PrimaryTimezone(country)

	return ReverseGeocodeResult{
		Country:  country,
		Language: primaryLanguageForCountry(country),
		Timezone: tz,
	}, nil
}
