package web

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/modu-ai/goose/internal/tools/web/common"
)

// routeProvider determines the provider name for a given config and location.
// It implements the auto-routing logic defined in REQ-WEATHER-006 and
// REQ-WEATHER-011.
//
// Rules:
//   - provider == "openweathermap"  → always "openweathermap"
//   - provider == "kma"            → "kma" (ErrMissingAPIKey propagated upstream when key empty)
//   - provider == "auto":
//   - loc.Country == "KR" + kma.api_key non-empty → "kma"
//   - loc.Country == "KR" + kma.api_key empty     → "openweathermap" + audit log
//   - loc.Country != "KR"                          → "openweathermap"
//
// @MX:ANCHOR: [AUTO] routeProvider — central routing logic for weather provider selection
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-006 — fan_in >= 3 (weather_current, weather_forecast, tests)
func routeProvider(cfg *WeatherConfig, loc Location) string {
	switch cfg.Provider {
	case "openweathermap":
		return "openweathermap"
	case "kma":
		// Caller is responsible for checking key presence; we return "kma"
		// unconditionally and let selectProvider/ErrMissingAPIKey surface the error.
		return "kma"
	default: // "auto" or any unrecognized value defaults to auto
		if loc.Country == "KR" {
			if cfg.KMA.APIKey != "" {
				return "kma"
			}
			// KMA key missing: silent fallback to OWM (REQ-WEATHER-011).
			zap.L().Info("kma key missing; falling back to openweathermap",
				zap.String("reason", "kma_key_missing_fallback_owm"),
				zap.String("country", loc.Country),
			)
			return "openweathermap"
		}
		return "openweathermap"
	}
}

// selectProvider resolves the active WeatherProvider for a call.
// It calls routeProvider to get the provider name, then looks it up in the
// providers map. Returns ErrMissingAPIKey when provider=="kma" and apiKey=="".
func selectProvider(cfg *WeatherConfig, loc Location, providers map[string]WeatherProvider, deps *common.Deps) (WeatherProvider, error) {
	name := routeProvider(cfg, loc)

	// Special case: explicit kma with empty key should fail fast.
	if name == "kma" && cfg.KMA.APIKey == "" {
		return nil, fmt.Errorf("%w: weather.kma.api_key is required when provider=kma", ErrMissingAPIKey)
	}

	p, ok := providers[name]
	if !ok || p == nil {
		// Build the provider on demand using production factories.
		switch name {
		case "kma":
			p = NewKMAProvider(cfg.KMA.APIKey, deps)
		default:
			p = NewOpenWeatherMapProvider(cfg.OpenWeatherMap.APIKey, deps)
		}
	}
	return p, nil
}
