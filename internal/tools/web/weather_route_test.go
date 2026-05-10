package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// Route-level unit tests
// --------------------------------------------------------------------------

// TestAutoRoute_KRCountryUsesKMA is the key AC-WEATHER-004 test.
// It constructs a webWeatherForecast tool with two stubbed providers (OWM + KMA)
// and verifies that a Seoul,KR request routes to KMA exclusively.
func TestAutoRoute_KRCountryUsesKMA(t *testing.T) {
	var owmCount, kmaCount atomic.Int64

	owmProvider := &routeTestOWMProvider{callCount: &owmCount}
	kmaProvider := &routeTestKMAProvider{callCount: &kmaCount}

	deps := &common.Deps{Cwd: t.TempDir()}
	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "auto",
		KMAKey:   "valid-kma-key",
		OWMKey:   "valid-owm-key",
	})

	// Inject a geolocator that returns Seoul,KR coordinates.
	geolocator := &stubGeolocator{loc: web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}}

	tool := web.NewWeatherForecastForTest(deps, cfg,
		map[string]web.WeatherProvider{
			"openweathermap": owmProvider,
			"kma":            kmaProvider,
		},
		geolocator,
	)

	// Call with location string so inferCountry can extract "KR" from "Seoul,KR".
	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR", "days": 3})
	result, err := tool.Call(context.Background(), input)

	require.NoError(t, err)
	require.False(t, result.IsError, "expected successful forecast call, got error")

	assert.Equal(t, int64(1), kmaCount.Load(), "KMA outbound counter must be exactly 1")
	assert.Equal(t, int64(0), owmCount.Load(), "OWM outbound counter must be 0")
}

// TestAutoRoute_NonKRUsesOWM verifies that a non-KR country routes to OWM.
func TestAutoRoute_NonKRUsesOWM(t *testing.T) {
	var owmCount, kmaCount atomic.Int64

	owmProvider := &routeTestOWMProvider{callCount: &owmCount}
	kmaProvider := &routeTestKMAProvider{callCount: &kmaCount}

	deps := &common.Deps{Cwd: t.TempDir()}
	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "auto",
		KMAKey:   "valid-kma-key",
		OWMKey:   "valid-owm-key",
	})

	geolocator := &stubGeolocator{loc: web.Location{Lat: 35.6762, Lon: 139.6503, Country: "JP"}}

	tool := web.NewWeatherForecastForTest(deps, cfg,
		map[string]web.WeatherProvider{
			"openweathermap": owmProvider,
			"kma":            kmaProvider,
		},
		geolocator,
	)

	// "Tokyo,JP" — inferCountry returns "JP", routing to OWM.
	input, _ := json.Marshal(map[string]any{"location": "Tokyo,JP", "days": 3})
	result, err := tool.Call(context.Background(), input)

	require.NoError(t, err)
	_ = result

	assert.Equal(t, int64(0), kmaCount.Load(), "KMA must not be called for non-KR location")
	assert.Equal(t, int64(1), owmCount.Load(), "OWM must be called for non-KR location")
}

// TestAutoRoute_KMAKeyMissingFallback verifies that when provider=auto,
// KR location, and KMA API key is empty, the tool falls back to OWM
// (REQ-WEATHER-011 silent fallback).
func TestAutoRoute_KMAKeyMissingFallback(t *testing.T) {
	var owmCount, kmaCount atomic.Int64

	owmProvider := &routeTestOWMProvider{callCount: &owmCount}
	kmaProvider := &routeTestKMAProvider{callCount: &kmaCount}

	deps := &common.Deps{Cwd: t.TempDir()}
	// KMAKey is intentionally empty.
	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "auto",
		KMAKey:   "",
		OWMKey:   "valid-owm-key",
	})

	geolocator := &stubGeolocator{loc: web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}}

	// Both providers present, but cfg.KMA.APIKey is empty → routing must silently pick OWM.
	tool := web.NewWeatherForecastForTest(deps, cfg,
		map[string]web.WeatherProvider{
			"openweathermap": owmProvider,
			"kma":            kmaProvider,
		},
		geolocator,
	)

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR", "days": 3})
	result, err := tool.Call(context.Background(), input)

	require.NoError(t, err)
	_ = result

	assert.Equal(t, int64(0), kmaCount.Load(), "KMA must not be called when key is missing")
	assert.Equal(t, int64(1), owmCount.Load(), "OWM must be called as fallback when KMA key is absent")
}

// TestForceKMA_NoKey_ReturnsError verifies that provider=kma + empty key
// returns missing_api_key error on the first Call().
func TestForceKMA_NoKey_ReturnsError(t *testing.T) {
	deps := &common.Deps{Cwd: t.TempDir()}
	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "kma",
		KMAKey:   "", // intentionally empty
		OWMKey:   "owm-key",
	})

	geolocator := &stubGeolocator{loc: web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}}

	tool := web.NewWeatherForecastForTest(deps, cfg, map[string]web.WeatherProvider{}, geolocator)

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR", "days": 3})
	result, err := tool.Call(context.Background(), input)

	require.NoError(t, err) // Call() returns error code in response, not in err
	assert.True(t, result.IsError, "result must be IsError when KMA key is missing")

	// Verify the error code in the response body.
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(string(result.Content)), &resp))
	ok, _ := resp["ok"].(bool)
	assert.False(t, ok)

	errObj, _ := resp["error"].(map[string]any)
	require.NotNil(t, errObj)
	assert.Equal(t, "missing_api_key", errObj["code"])
}

// --------------------------------------------------------------------------
// Weather Forecast basic tests
// --------------------------------------------------------------------------

// TestWeatherForecast_Registered_InWebTools verifies that weather_forecast
// is registered in the global web tool registry (AC-WEATHER-009 M2).
func TestWeatherForecast_Registered_InWebTools(t *testing.T) {
	registered := web.RegisteredWebToolsForTest()
	names := make([]string, 0, len(registered))
	for _, tool := range registered {
		names = append(names, tool.Name())
	}
	assert.Contains(t, names, "weather_forecast", "weather_forecast must be registered in web tools")
}

// TestWeatherForecast_StandardResponseShape verifies the TOOLS-WEB-001
// standard response shape for weather_forecast (AC-WEATHER-010).
func TestWeatherForecast_StandardResponseShape(t *testing.T) {
	fcstBody := buildForecastKMAResponse(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fcstBody)
	}))
	defer srv.Close()

	deps := &common.Deps{Cwd: t.TempDir()}
	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "kma",
		KMAKey:   "valid-key",
	})
	kmaProvider := web.NewKMAProviderForTest("valid-key", srv.URL, deps)
	geolocator := &stubGeolocator{loc: web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}}

	tool := web.NewWeatherForecastForTest(deps, cfg,
		map[string]web.WeatherProvider{"kma": kmaProvider},
		geolocator,
	)

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR", "days": 3})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	require.NotEmpty(t, result.Content)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &resp))

	assert.Contains(t, resp, "ok", "response must have 'ok' field")
	assert.Contains(t, resp, "metadata", "response must have 'metadata' field")
	assert.True(t, resp["ok"].(bool), "'ok' must be true for successful call")
}

// TestWeatherForecast_DaysOutOfRange verifies that days=0 and days=8 return
// invalid_input (schema guard).
func TestWeatherForecast_DaysOutOfRange(t *testing.T) {
	deps := &common.Deps{Cwd: t.TempDir()}
	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "openweathermap",
		OWMKey:   "key",
	})
	geolocator := &stubGeolocator{loc: web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}}
	tool := web.NewWeatherForecastForTest(deps, cfg, map[string]web.WeatherProvider{}, geolocator)

	for _, badDays := range []int{0, 8} {
		input, _ := json.Marshal(map[string]any{"location": "Seoul,KR", "days": badDays})
		result, err := tool.Call(context.Background(), input)
		require.NoError(t, err, "days=%d: Call must not return error", badDays)

		var resp map[string]any
		require.NoError(t, json.Unmarshal([]byte(string(result.Content)), &resp))
		assert.False(t, resp["ok"].(bool), "days=%d: ok must be false", badDays)
		errObj := resp["error"].(map[string]any)
		assert.Equal(t, "invalid_input", errObj["code"], "days=%d: error code must be invalid_input", badDays)
	}
}

// TestWeatherForecast_BlocklistPriority verifies that a blocklisted host
// is rejected before the provider is called.
func TestWeatherForecast_BlocklistPriority(t *testing.T) {
	var callCount atomic.Int64
	owmProvider := &routeTestOWMProvider{callCount: &callCount}

	// Block the OWM API host explicitly.
	blocklist := common.NewBlocklist([]string{"api.openweathermap.org"})
	deps := &common.Deps{Cwd: t.TempDir(), Blocklist: blocklist}
	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "openweathermap",
		OWMKey:   "key",
	})
	geolocator := &stubGeolocator{loc: web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}}

	tool := web.NewWeatherForecastForTest(deps, cfg,
		map[string]web.WeatherProvider{"openweathermap": owmProvider},
		geolocator,
	)

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR", "days": 3})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	assert.True(t, result.IsError, "blocklisted host must return error result")
	assert.Equal(t, int64(0), callCount.Load(), "outbound call must not happen when host is blocked")
}

// TestWeatherForecast_RateLimit_Exhausted verifies ratelimit_exhausted behavior
// for weather_forecast (mirrors AC-WEATHER-008 for forecast).
func TestWeatherForecast_RateLimit_Exhausted(t *testing.T) {
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		http.Error(w, "should not be called", http.StatusForbidden)
	}))
	defer srv.Close()

	// Build an exhausted tracker for the openweathermap provider.
	now := time.Now()
	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	web.RegisterWeatherParser(tracker)
	syntheticHeaders := map[string]string{
		"X-RateLimit-Limit":     "60",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     "15",
	}
	require.NoError(t, tracker.Parse("openweathermap", syntheticHeaders, now))
	state := tracker.State("openweathermap")
	require.GreaterOrEqual(t, state.RequestsMin.UsagePct(), 100.0, "tracker must be exhausted")

	deps := &common.Deps{Cwd: t.TempDir(), RateTracker: tracker}
	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "openweathermap",
		OWMKey:   "key",
	})
	owmProvider := &routeTestOWMProvider{callCount: &callCount}
	geolocator := &stubGeolocator{loc: web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}}

	tool := web.NewWeatherForecastForTest(deps, cfg,
		map[string]web.WeatherProvider{"openweathermap": owmProvider},
		geolocator,
	)

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR", "days": 3})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	assert.True(t, result.IsError, "exhausted rate limit must return error result")
	assert.Equal(t, int64(0), callCount.Load(), "outbound call must not happen when ratelimit exhausted")

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(string(result.Content)), &resp))
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "ratelimit_exhausted", errObj["code"])
}

// --------------------------------------------------------------------------
// Shared helpers for route tests
// --------------------------------------------------------------------------

// buildForecastKMAResponse returns a minimal VilageFcst-style JSON body.
func buildForecastKMAResponse(t *testing.T) []byte {
	t.Helper()
	return kmaFcstResponse(t)
}

// routeTestOWMProvider is a stub WeatherProvider that always returns success.
// It is used in routing tests where we only need to verify which provider is called.
type routeTestOWMProvider struct {
	callCount *atomic.Int64
}

func (p *routeTestOWMProvider) Name() string { return "openweathermap" }
func (p *routeTestOWMProvider) GetCurrent(_ context.Context, _ web.Location) (*web.WeatherReport, error) {
	return &web.WeatherReport{SourceProvider: "openweathermap"}, nil
}
func (p *routeTestOWMProvider) GetForecast(_ context.Context, _ web.Location, _ int) ([]web.WeatherForecastDay, error) {
	p.callCount.Add(1)
	return []web.WeatherForecastDay{}, nil
}
func (p *routeTestOWMProvider) GetAirQuality(_ context.Context, _ web.Location) (*web.AirQuality, error) {
	return nil, nil
}
func (p *routeTestOWMProvider) GetSunTimes(_ context.Context, _ web.Location, _ time.Time) (*web.SunTimes, error) {
	return nil, nil
}

// routeTestKMAProvider is a stub KMAProvider for routing tests.
type routeTestKMAProvider struct {
	callCount *atomic.Int64
}

func (p *routeTestKMAProvider) Name() string { return "kma" }
func (p *routeTestKMAProvider) GetCurrent(_ context.Context, _ web.Location) (*web.WeatherReport, error) {
	return &web.WeatherReport{SourceProvider: "kma"}, nil
}
func (p *routeTestKMAProvider) GetForecast(_ context.Context, _ web.Location, _ int) ([]web.WeatherForecastDay, error) {
	p.callCount.Add(1)
	return []web.WeatherForecastDay{}, nil
}
func (p *routeTestKMAProvider) GetAirQuality(_ context.Context, _ web.Location) (*web.AirQuality, error) {
	return nil, nil
}
func (p *routeTestKMAProvider) GetSunTimes(_ context.Context, _ web.Location, _ time.Time) (*web.SunTimes, error) {
	return nil, nil
}

// stubGeolocator implements web.IPGeolocator returning a fixed location.
type stubGeolocator struct {
	loc web.Location
}

func (s *stubGeolocator) Resolve(_ context.Context) (web.Location, error) {
	return s.loc, nil
}
