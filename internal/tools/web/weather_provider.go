package web

import (
	"context"
	"errors"
	"time"
)

// WeatherProvider is the abstraction over weather data sources.
// M1 ships OpenWeatherMapProvider; KMAProvider arrives in M2.
//
// @MX:ANCHOR: [AUTO] WeatherProvider interface — single seam for all weather data sources
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-001 — fan_in >= 3 (weather_current, weather_forecast M2, tests)
type WeatherProvider interface {
	// Name returns the provider identifier, e.g. "openweathermap" or "kma".
	Name() string

	// GetCurrent fetches the current weather for the given location.
	// Returns ErrMissingAPIKey when the required API key is absent.
	// Returns ErrInvalidResponse when the provider returns unparseable data.
	GetCurrent(ctx context.Context, loc Location) (*WeatherReport, error)

	// GetForecast fetches a multi-day forecast (M2+).
	// days must be in [1, 7].
	GetForecast(ctx context.Context, loc Location, days int) ([]WeatherForecastDay, error)

	// GetAirQuality fetches air-quality data (M3+).
	// For KMA/AirKorea this is PM10/PM2.5; for OWM it uses the AQI endpoint.
	GetAirQuality(ctx context.Context, loc Location) (*AirQuality, error)

	// GetSunTimes fetches sunrise/sunset for loc on date (optional, best-effort).
	GetSunTimes(ctx context.Context, loc Location, date time.Time) (*SunTimes, error)
}

// IPGeolocator resolves the caller's public IP address to a Location.
// The production implementation calls ipapi.co; tests inject a stub.
type IPGeolocator interface {
	// Resolve returns a Location derived from the caller's outbound IP address.
	// Returns ErrGeolocationFailed on network error or unparseable response.
	Resolve(ctx context.Context) (Location, error)
}

// OfflineStore persists the most-recent successful WeatherReport to disk so
// that weather_current can return stale data when the provider is unreachable.
type OfflineStore interface {
	// SaveLatest persists report to the provider+coordinates slot, overwriting
	// any previous value atomically (temp-file + rename).
	SaveLatest(provider string, lat, lon float64, report *WeatherReport) error

	// LoadLatest retrieves the last saved report. Returns ErrNoFallbackAvailable
	// when no file exists for the given slot. Returns ErrNoFallbackAvailable when
	// the on-disk JSON is corrupt (and evicts the corrupt file).
	LoadLatest(provider string, lat, lon float64) (*WeatherReport, error)
}

// Sentinel errors used by WeatherProvider implementations and the weather_current tool.

// ErrMissingAPIKey is returned by a provider when its API key is absent or empty.
var ErrMissingAPIKey = errors.New("weather: missing API key")

// ErrNoFallbackAvailable is returned by OfflineStore.LoadLatest when no disk
// fallback exists for the requested provider + coordinates.
var ErrNoFallbackAvailable = errors.New("weather: no offline fallback available")

// ErrGeolocationFailed is returned by IPGeolocator.Resolve when the lookup
// fails (network error, non-200 response, unparseable JSON).
var ErrGeolocationFailed = errors.New("weather: IP geolocation failed")

// ErrInvalidResponse is returned by a provider when the API returns a response
// that cannot be parsed into a WeatherReport.
var ErrInvalidResponse = errors.New("weather: invalid provider response")
