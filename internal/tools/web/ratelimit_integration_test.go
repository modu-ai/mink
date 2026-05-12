package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// TestRateLimitExhausted — DC-08 / AC-WEB-008
// --------------------------------------------------------------------------

// TestRateLimitExhausted forces Brave's RequestsMin bucket to exhausted
// (UsagePct >= 100%) and verifies that web_search returns ratelimit_exhausted
// without calling the mock provider.
func TestRateLimitExhausted(t *testing.T) {
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte(`{"web":{"results":[]}}`))
	}))
	defer srv.Close()

	now := time.Now()

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	web.RegisterBraveParser(tracker)

	// Synthesize an exhausted state by parsing headers that set
	// Limit=100, Remaining=0, Reset=15 (seconds).
	// This yields UsagePct = 100%.
	syntheticHeaders := map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     "15",
	}
	require.NoError(t, tracker.Parse("brave", syntheticHeaders, now))

	state := tracker.State("brave")
	require.GreaterOrEqual(t, state.RequestsMin.UsagePct(), 100.0,
		"RequestsMin must be exhausted (UsagePct >= 100)")

	deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
	deps.RateTracker = tracker
	deps.Clock = func() time.Time { return now }
	deps.Cwd = t.TempDir()

	tool := web.NewWebSearch(deps, srv.URL)
	result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
		"query": "exhausted", "provider": "brave",
	}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "ratelimit_exhausted", resp.Error.Code)
	assert.True(t, resp.Error.Retryable, "ratelimit_exhausted must be retryable")
	assert.Greater(t, resp.Error.RetryAfterSeconds, 0,
		"retry_after_seconds must be > 0 when reset seconds remain")

	assert.Equal(t, int64(0), callCount.Load(),
		"provider must NOT be called when ratelimit is exhausted")
}

// --------------------------------------------------------------------------
// TestRateLimit429WithHeader — ratelimit 429 response triggers exhaustion
// --------------------------------------------------------------------------

// TestRateLimit429WithHeader simulates a 429 response with Retry-After: 60,
// calls tracker.Parse with those headers, then verifies the next web_search
// call returns ratelimit_exhausted.
func TestRateLimit429WithHeader(t *testing.T) {
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte(`{"web":{"results":[]}}`))
	}))
	defer srv.Close()

	now := time.Now()

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	web.RegisterBraveParser(tracker)

	// Parse headers representing a 429-like exhaustion with Retry-After: 60.
	headers429 := map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     "60",
	}
	require.NoError(t, tracker.Parse("brave", headers429, now))

	state := tracker.State("brave")
	require.GreaterOrEqual(t, state.RequestsMin.UsagePct(), 100.0,
		"after 429-like headers, RequestsMin must be exhausted")

	// Verify retry_after_seconds reflects the 60-second reset.
	remainSecs := state.RequestsMin.RemainingSecondsNow(now)
	require.Greater(t, remainSecs, 0.0, "remaining seconds must be > 0")

	deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
	deps.RateTracker = tracker
	deps.Clock = func() time.Time { return now }
	deps.Cwd = t.TempDir()

	tool := web.NewWebSearch(deps, srv.URL)
	result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
		"query": "ratelimited", "provider": "brave",
	}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "ratelimit_exhausted", resp.Error.Code)
	assert.True(t, resp.Error.Retryable)

	// retry_after_seconds must be a non-negative integer (math.Ceil of remaining).
	assert.GreaterOrEqual(t, resp.Error.RetryAfterSeconds, 0)

	assert.Equal(t, int64(0), callCount.Load(),
		"provider must NOT be called when ratelimit is exhausted")

	// ISSUE-04: verify retry_after_seconds is integer (never a float).
	raw := result.Content
	var rawJSON map[string]any
	require.NoError(t, json.Unmarshal(raw, &rawJSON))
	errObj, ok := rawJSON["error"].(map[string]any)
	require.True(t, ok, "error field must be a JSON object")
	retryAfterRaw, hasRetry := errObj["retry_after_seconds"]
	require.True(t, hasRetry, "retry_after_seconds must be present")
	// JSON numbers unmarshal as float64; verify it has no fractional part.
	retryFloat, isFloat := retryAfterRaw.(float64)
	require.True(t, isFloat, "retry_after_seconds must be a JSON number")
	assert.Equal(t, retryFloat, float64(int(retryFloat)),
		"retry_after_seconds must be an integer, got %v", retryFloat)
}

// --------------------------------------------------------------------------
// TestRateLimitExhausted_Weather — T-019 / AC-WEATHER-008
// --------------------------------------------------------------------------

// rlWeatherStubProvider is a minimal WeatherProvider for ratelimit tests.
type rlWeatherStubProvider struct {
	callCount atomic.Int64
}

func (p *rlWeatherStubProvider) Name() string { return "openweathermap" }
func (p *rlWeatherStubProvider) GetCurrent(_ context.Context, _ web.Location) (*web.WeatherReport, error) {
	p.callCount.Add(1)
	return &web.WeatherReport{SourceProvider: "openweathermap"}, nil
}
func (p *rlWeatherStubProvider) GetForecast(_ context.Context, _ web.Location, _ int) ([]web.WeatherForecastDay, error) {
	return nil, nil
}
func (p *rlWeatherStubProvider) GetAirQuality(_ context.Context, _ web.Location) (*web.AirQuality, error) {
	return nil, nil
}
func (p *rlWeatherStubProvider) GetSunTimes(_ context.Context, _ web.Location, _ time.Time) (*web.SunTimes, error) {
	return nil, nil
}

// rlWeatherGeo is a stub IPGeolocator for ratelimit tests.
type rlWeatherGeo struct{}

func (g *rlWeatherGeo) Resolve(_ context.Context) (web.Location, error) {
	return web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}, nil
}

// TestRateLimitExhausted_Weather forces the "openweathermap" provider to
// exhausted state and verifies that weather_current returns ratelimit_exhausted
// without calling the provider.
func TestRateLimitExhausted_Weather(t *testing.T) {
	now := time.Now()

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	web.RegisterWeatherParser(tracker)

	// Synthesize exhausted state.
	syntheticHeaders := map[string]string{
		"X-RateLimit-Limit":     "60",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     "30",
	}
	require.NoError(t, tracker.Parse("openweathermap", syntheticHeaders, now))

	state := tracker.State("openweathermap")
	require.GreaterOrEqual(t, state.RequestsMin.UsagePct(), 100.0,
		"RequestsMin must be exhausted (UsagePct >= 100)")

	deps, _, _ := newTestDeps(t, []string{"api.openweathermap.org"})
	deps.RateTracker = tracker
	deps.Clock = func() time.Time { return now }
	deps.Cwd = t.TempDir()

	provider := &rlWeatherStubProvider{}
	offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")

	cfg, _ := web.LoadWeatherConfig("")
	cfg.OpenWeatherMap.APIKey = "rl-test-key"

	tool := web.NewWeatherCurrentForTest(deps, cfg, provider, &rlWeatherGeo{}, offline)
	input, _ := json.Marshal(map[string]any{"lat": 37.57, "lon": 126.98})

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "ratelimit_exhausted", resp.Error.Code)
	assert.True(t, resp.Error.Retryable, "ratelimit_exhausted must be retryable")
	assert.Greater(t, resp.Error.RetryAfterSeconds, 0,
		"retry_after_seconds must be > 0 when reset seconds remain")

	assert.Equal(t, int64(0), provider.callCount.Load(),
		"provider must NOT be called when rate limit is exhausted")
}
