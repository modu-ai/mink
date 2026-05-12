package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/modu-ai/mink/internal/tools/web/common"
)

const (
	owmBaseURL = "https://api.openweathermap.org"
	owmName    = "openweathermap"
	owmTimeout = 10 * time.Second
)

// owmCurrentResponse mirrors the fields of the OWM /data/2.5/weather endpoint
// that weather_current requires. Unneeded fields are intentionally omitted.
type owmCurrentResponse struct {
	Weather []struct {
		Main        string `json:"main"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"` // m/s when units=metric
		Deg   int     `json:"deg"`
	} `json:"wind"`
	Clouds struct {
		All int `json:"all"` // cloud cover percent
	} `json:"clouds"`
	Rain struct {
		OneH float64 `json:"1h"` // mm in last hour
	} `json:"rain"`
	Snow struct {
		OneH float64 `json:"1h"`
	} `json:"snow"`
	Dt  int64 `json:"dt"` // Unix timestamp
	Sys struct {
		Country string `json:"country"`
		Sunrise int64  `json:"sunrise"`
		Sunset  int64  `json:"sunset"`
	} `json:"sys"`
	Timezone int    `json:"timezone"` // offset seconds from UTC
	Name     string `json:"name"`
	Cod      int    `json:"cod"` // OWM status code; 200 = OK
}

// OpenWeatherMapProvider implements WeatherProvider using the OWM REST API
// v2.5 (current weather). M1 implements GetCurrent only; M2 adds GetForecast.
//
// API key is never written to logs or error messages (REQ-WEATHER-004).
type OpenWeatherMapProvider struct {
	apiKey  string
	baseURL string // injectable for tests; defaults to owmBaseURL
	deps    *common.Deps
}

// NewOpenWeatherMapProvider constructs a production OWM provider.
func NewOpenWeatherMapProvider(apiKey string, deps *common.Deps) *OpenWeatherMapProvider {
	return &OpenWeatherMapProvider{
		apiKey:  apiKey,
		baseURL: owmBaseURL,
		deps:    deps,
	}
}

// NewOpenWeatherMapProviderForTest constructs an OWM provider with an
// injectable base URL so tests can redirect requests to an httptest.Server.
func NewOpenWeatherMapProviderForTest(apiKey, baseURL string, deps *common.Deps) *OpenWeatherMapProvider {
	return &OpenWeatherMapProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		deps:    deps,
	}
}

// Name returns "openweathermap".
func (p *OpenWeatherMapProvider) Name() string { return owmName }

// GetCurrent fetches the current weather at loc using the OWM /data/2.5/weather
// endpoint. lang and units are read from the caller-supplied values when
// populating the URL; defaults are "en" and "metric".
func (p *OpenWeatherMapProvider) GetCurrent(ctx context.Context, loc Location) (*WeatherReport, error) {
	return p.getCurrentWithOptions(ctx, loc, "metric", "en")
}

// GetCurrentWithOptions is the internal variant used by weather_current.go to
// pass caller-supplied units and lang through the provider.
func (p *OpenWeatherMapProvider) GetCurrentWithOptions(ctx context.Context, loc Location, units, lang string) (*WeatherReport, error) {
	return p.getCurrentWithOptions(ctx, loc, units, lang)
}

func (p *OpenWeatherMapProvider) getCurrentWithOptions(ctx context.Context, loc Location, units, lang string) (*WeatherReport, error) {
	if p.apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if units == "" {
		units = "metric"
	}
	if lang == "" {
		lang = "en"
	}

	endpoint := p.buildCurrentURL(loc.Lat, loc.Lon, units, lang)

	start := time.Now()
	resp, err := p.doGet(ctx, endpoint)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		p.logCall(loc, latencyMs, false, false, 0, "fetch_error")
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p.logCall(loc, latencyMs, false, false, resp.StatusCode, "http_error")
		return nil, fmt.Errorf("%w: HTTP %d", ErrInvalidResponse, resp.StatusCode)
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseBytes))
	if readErr != nil {
		p.logCall(loc, latencyMs, false, false, resp.StatusCode, "read_error")
		return nil, fmt.Errorf("%w: read body: %v", ErrInvalidResponse, readErr)
	}

	var owmResp owmCurrentResponse
	if jsonErr := json.Unmarshal(body, &owmResp); jsonErr != nil {
		p.logCall(loc, latencyMs, false, false, resp.StatusCode, "decode_error")
		return nil, fmt.Errorf("%w: decode JSON: %v (raw: %.200s)", ErrInvalidResponse, jsonErr, body)
	}

	report := p.toWeatherReport(owmResp, loc, units)
	p.logCall(loc, latencyMs, false, false, resp.StatusCode, "ok")
	return report, nil
}

// GetForecast is a stub for M1; M2 implements it.
func (p *OpenWeatherMapProvider) GetForecast(_ context.Context, _ Location, _ int) ([]WeatherForecastDay, error) {
	return nil, fmt.Errorf("weather_forecast not implemented in M1")
}

// GetAirQuality is a stub for M1; M3 implements it.
func (p *OpenWeatherMapProvider) GetAirQuality(_ context.Context, _ Location) (*AirQuality, error) {
	return nil, fmt.Errorf("weather_air_quality not implemented in M1")
}

// GetSunTimes returns sunrise/sunset embedded in the current-weather response.
func (p *OpenWeatherMapProvider) GetSunTimes(ctx context.Context, loc Location, _ time.Time) (*SunTimes, error) {
	report, err := p.GetCurrent(ctx, loc)
	if err != nil {
		return nil, err
	}
	if report.SunTimes == nil {
		return nil, nil
	}
	return report.SunTimes, nil
}

// buildCurrentURL composes the OWM /data/2.5/weather URL.
// The API key is included as a query parameter; it is masked in log output.
func (p *OpenWeatherMapProvider) buildCurrentURL(lat, lon float64, units, lang string) string {
	base := strings.TrimRight(p.baseURL, "/")
	return fmt.Sprintf("%s/data/2.5/weather?lat=%f&lon=%f&units=%s&lang=%s&appid=%s",
		base, lat, lon, url.QueryEscape(units), url.QueryEscape(lang), p.apiKey)
}

// doGet performs a GET request with a 10-second timeout and a standard User-Agent.
func (p *OpenWeatherMapProvider) doGet(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", common.UserAgent())
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: owmTimeout}
	return client.Do(req)
}

// toWeatherReport converts an OWM API response to the canonical WeatherReport.
func (p *OpenWeatherMapProvider) toWeatherReport(r owmCurrentResponse, loc Location, units string) *WeatherReport {
	cond := owmConditionToCanonical(r.Weather)
	condLocal := ""
	if len(r.Weather) > 0 {
		condLocal = r.Weather[0].Description
	}

	// OWM returns wind speed in m/s for metric. Convert to km/h.
	windKph := r.Main.Temp // placeholder; reassigned below
	_ = windKph
	windSpeedKph := r.Wind.Speed * 3.6 // m/s → km/h
	if units == "imperial" {
		// Imperial: m/s is still returned (OWM inconsistency); convert mph→kph.
		windSpeedKph = r.Wind.Speed * 1.60934
	}

	// OWM returns temp in Celsius for metric, Fahrenheit for imperial.
	// WeatherReport always stores Celsius.
	tempC := r.Main.Temp
	feelsC := r.Main.FeelsLike
	if units == "imperial" {
		tempC = (r.Main.Temp - 32) * 5 / 9
		feelsC = (r.Main.FeelsLike - 32) * 5 / 9
	}

	precipMm := r.Rain.OneH
	if precipMm == 0 {
		precipMm = r.Snow.OneH
	}

	var sunTimes *SunTimes
	if r.Sys.Sunrise > 0 && r.Sys.Sunset > 0 {
		sunTimes = &SunTimes{
			Sunrise: time.Unix(r.Sys.Sunrise, 0).UTC(),
			Sunset:  time.Unix(r.Sys.Sunset, 0).UTC(),
		}
	}

	// Enrich location with provider data if caller only supplied lat/lon.
	resolvedLoc := loc
	if resolvedLoc.DisplayName == "" {
		resolvedLoc.DisplayName = r.Name
	}
	if resolvedLoc.Country == "" {
		resolvedLoc.Country = r.Sys.Country
	}
	if resolvedLoc.Timezone == "" && r.Timezone != 0 {
		// Store offset as string; tzdata lookup is out of scope for M1.
		resolvedLoc.Timezone = fmt.Sprintf("UTC%+d", r.Timezone/3600)
	}

	return &WeatherReport{
		Location:       resolvedLoc,
		Timestamp:      time.Unix(r.Dt, 0).UTC(),
		TemperatureC:   roundFloat(tempC, 1),
		FeelsLikeC:     roundFloat(feelsC, 1),
		Condition:      cond,
		ConditionLocal: condLocal,
		Humidity:       r.Main.Humidity,
		WindKph:        roundFloat(windSpeedKph, 1),
		WindDirection:  degreeToCompass(r.Wind.Deg),
		CloudCoverPct:  r.Clouds.All,
		PrecipMm:       roundFloat(precipMm, 1),
		UVIndex:        0, // OWM v2.5 does not include UV index
		SunTimes:       sunTimes,
		SourceProvider: owmName,
	}
}

// logCall emits a structured zap log entry for the OWM call.
// The API key is never included (REQ-WEATHER-004).
func (p *OpenWeatherMapProvider) logCall(loc Location, latencyMs int64, cacheHit, stale bool, apiStatus int, outcome string) {
	if p.deps == nil {
		return
	}
	// Use a local zap logger; production bootstrap injects a real logger via
	// common.Deps; for M1 we fall back to a no-op logger when none is wired.
	log := zap.L()
	log.Info("weather provider call",
		zap.String("provider", owmName),
		zap.String("api_key", "****"), // always redacted (REQ-WEATHER-004)
		zap.Float64("lat", loc.Lat),
		zap.Float64("lon", loc.Lon),
		zap.Int64("latency_ms", latencyMs),
		zap.Bool("cache_hit", cacheHit),
		zap.Bool("stale", stale),
		zap.Int("api_status", apiStatus),
		zap.String("outcome", outcome),
	)
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// owmConditionToCanonical maps OWM weather condition codes to the canonical
// set defined in the SPEC: "clear" | "cloudy" | "rain" | "snow" | "thunderstorm" | "mist".
func owmConditionToCanonical(weather []struct {
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}) string {
	if len(weather) == 0 {
		return "unknown"
	}
	switch strings.ToLower(weather[0].Main) {
	case "clear":
		return "clear"
	case "clouds":
		return "cloudy"
	case "rain", "drizzle":
		return "rain"
	case "snow":
		return "snow"
	case "thunderstorm":
		return "thunderstorm"
	case "mist", "smoke", "haze", "dust", "fog", "sand", "ash", "squall", "tornado":
		return "mist"
	default:
		return "unknown"
	}
}

// degreeToCompass converts a meteorological wind direction degree (0-360)
// to a 16-point compass rose abbreviation (N, NNE, NE, ENE, …).
func degreeToCompass(deg int) string {
	// Normalize to [0, 360).
	d := ((deg % 360) + 360) % 360
	// 16 points, each spanning 22.5°. We snap to the nearest 8-point subset
	// matching the SPEC: "N" | "NE" | "E" | "SE" | "S" | "SW" | "W" | "NW".
	dirs := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	idx := int(math.Round(float64(d)/45)) % 8
	return dirs[idx]
}

// roundFloat rounds f to prec decimal places.
func roundFloat(f float64, prec int) float64 {
	factor := math.Pow10(prec)
	return math.Round(f*factor) / factor
}
