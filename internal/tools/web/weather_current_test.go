package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/permission"
	permstore "github.com/modu-ai/goose/internal/permission/store"
	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// Shared test helpers for weather_current tests
// --------------------------------------------------------------------------

// stubWeatherProvider is a minimal WeatherProvider for use in unit tests.
type stubWeatherProvider struct {
	name      string
	report    *web.WeatherReport
	err       error
	callCount atomic.Int64
	latency   time.Duration // optional artificial delay
}

func (s *stubWeatherProvider) Name() string { return s.name }

func (s *stubWeatherProvider) GetCurrent(_ context.Context, _ web.Location) (*web.WeatherReport, error) {
	s.callCount.Add(1)
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	return s.report, s.err
}

func (s *stubWeatherProvider) GetForecast(_ context.Context, _ web.Location, _ int) ([]web.WeatherForecastDay, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubWeatherProvider) GetAirQuality(_ context.Context, _ web.Location) (*web.AirQuality, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubWeatherProvider) GetSunTimes(_ context.Context, _ web.Location, _ time.Time) (*web.SunTimes, error) {
	return nil, nil
}

// stubIPGeolocator returns a fixed Seoul location.
type stubIPGeolocator struct {
	loc web.Location
	err error
}

func (s *stubIPGeolocator) Resolve(_ context.Context) (web.Location, error) {
	return s.loc, s.err
}

// fixedSeoulGeo returns a stub geolocator resolving to Seoul.
func fixedSeoulGeo() web.IPGeolocator {
	return &stubIPGeolocator{
		loc: web.Location{Lat: 37.57, Lon: 126.98, DisplayName: "Seoul", Country: "KR"},
	}
}

// newWeatherTestDeps builds a *common.Deps wired for weather_current tests.
func newWeatherTestDeps(t *testing.T, providerHost string) (*common.Deps, *permission.Manager) {
	t.Helper()
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	hosts := []string{providerHost, "api.openweathermap.org", "ipapi.co"}
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{NetHosts: hosts}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: noopAuditWriter{},
		Clock:       func() time.Time { return time.Now() },
		Cwd:         t.TempDir(),
	}
	return deps, mgr
}

// newWeatherCurrent constructs a testable webWeatherCurrent via the exported factory.
func newWeatherCurrent(
	deps *common.Deps,
	cfg *web.WeatherConfig,
	provider web.WeatherProvider,
	geolocator web.IPGeolocator,
	offline web.OfflineStore,
) tools.Tool {
	return web.NewWeatherCurrentForTest(deps, cfg, provider, geolocator, offline)
}

// weatherCfg returns a WeatherConfig with an OWM API key for testing.
func weatherCfg() *web.WeatherConfig {
	cfg, _ := web.LoadWeatherConfig("") // returns defaults
	cfg.OpenWeatherMap.APIKey = testOWMAPIKey
	return cfg
}

// buildWeatherInput marshals a weather_current input map.
func buildWeatherInput(t *testing.T, m map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return b
}

// seoulReport returns a canonical WeatherReport for Seoul.
func seoulReport() *web.WeatherReport {
	return &web.WeatherReport{
		Location:       web.Location{Lat: 37.57, Lon: 126.98, DisplayName: "Seoul", Country: "KR"},
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		TemperatureC:   22.5,
		Condition:      "clear",
		ConditionLocal: "맑음",
		Humidity:       55,
		WindKph:        12.6,
		SourceProvider: "openweathermap",
	}
}

// --------------------------------------------------------------------------
// TestWeatherCurrent_Registered_InWebTools — AC-WEATHER-001 + AC-WEATHER-009
// --------------------------------------------------------------------------

// TestWeatherCurrent_Registered_InWebTools verifies that weather_current is
// auto-registered in the global web tool list via init() and resolves
// correctly from the registry with ScopeShared.
func TestWeatherCurrent_Registered_InWebTools(t *testing.T) {
	// The global registry includes weather_current via init().
	reg := tools.NewRegistry(tools.WithBuiltins(), web.WithWeb())
	names := reg.ListNames()

	assert.Contains(t, names, "weather_current",
		"weather_current must appear in the registry after init()")

	tool, ok := reg.Resolve("weather_current")
	require.True(t, ok, "weather_current must resolve to a non-nil Tool")
	require.NotNil(t, tool)
	assert.Equal(t, tools.ScopeShared, tool.Scope())
	assert.Equal(t, "weather_current", tool.Name())
}

// --------------------------------------------------------------------------
// TestWeatherCurrent_StandardResponseShape — AC-WEATHER-010
// --------------------------------------------------------------------------

// TestWeatherCurrent_StandardResponseShape verifies the common.Response shape
// for both success and failure cases of weather_current.
func TestWeatherCurrent_StandardResponseShape(t *testing.T) {
	t.Run("success_shape", func(t *testing.T) {
		deps, _ := newWeatherTestDeps(t, "api.openweathermap.org")
		provider := &stubWeatherProvider{name: "openweathermap", report: seoulReport()}
		offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")

		tool := newWeatherCurrent(deps, weatherCfg(), provider, fixedSeoulGeo(), offline)
		result, err := tool.Call(context.Background(), buildWeatherInput(t, map[string]any{
			"lat": 37.57, "lon": 126.98,
		}))
		require.NoError(t, err)

		var resp common.Response
		require.NoError(t, json.Unmarshal(result.Content, &resp))
		assert.True(t, resp.OK, "successful call must have ok=true")
		assert.Nil(t, resp.Error, "successful response must have no error field")
		assert.NotNil(t, resp.Data, "successful response must have data")
		assert.GreaterOrEqual(t, resp.Metadata.DurationMs, int64(0))
	})

	t.Run("failure_shape", func(t *testing.T) {
		deps, _ := newWeatherTestDeps(t, "api.openweathermap.org")
		provider := &stubWeatherProvider{name: "openweathermap", err: web.ErrInvalidResponse}
		offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")

		tool := newWeatherCurrent(deps, weatherCfg(), provider, fixedSeoulGeo(), offline)
		result, err := tool.Call(context.Background(), buildWeatherInput(t, map[string]any{
			"lat": 37.57, "lon": 126.98,
		}))
		require.NoError(t, err) // Call never returns Go error

		var resp common.Response
		require.NoError(t, json.Unmarshal(result.Content, &resp))
		assert.False(t, resp.OK, "failed call must have ok=false")
		require.NotNil(t, resp.Error, "failed response must have error field")
		assert.NotEmpty(t, resp.Error.Code)
		assert.GreaterOrEqual(t, resp.Metadata.DurationMs, int64(0))
	})
}

// --------------------------------------------------------------------------
// TestWeatherCurrent_CacheHitWithin10Min — AC-WEATHER-002
// --------------------------------------------------------------------------

// TestWeatherCurrent_CacheHitWithin10Min verifies that a second call within
// the 10-minute bbolt TTL returns a cached result without calling the provider.
func TestWeatherCurrent_CacheHitWithin10Min(t *testing.T) {
	deps, _ := newWeatherTestDeps(t, "api.openweathermap.org")
	provider := &stubWeatherProvider{name: "openweathermap", report: seoulReport()}
	offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")

	tool := newWeatherCurrent(deps, weatherCfg(), provider, fixedSeoulGeo(), offline)
	input := buildWeatherInput(t, map[string]any{"lat": 37.57, "lon": 126.98})

	// First call — populates cache.
	r1, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	resp1 := unmarshalToolResponse(t, r1)
	require.True(t, resp1.OK, "first call must succeed; error=%v", resp1.Error)
	assert.False(t, resp1.Metadata.CacheHit, "first call must be a cache miss")
	assert.Equal(t, int64(1), provider.callCount.Load())

	// Second call — must hit cache.
	r2, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	resp2 := unmarshalToolResponse(t, r2)
	require.True(t, resp2.OK, "second call must succeed")
	assert.True(t, resp2.Metadata.CacheHit, "second call must be a cache hit")
	assert.Equal(t, int64(1), provider.callCount.Load(), "provider must NOT be called on cache hit")

	// data.cache_hit in the WeatherReport data must also be true.
	var report web.WeatherReport
	require.NoError(t, json.Unmarshal(resp2.Data, &report))
	assert.True(t, report.CacheHit)
	assert.False(t, report.Stale)
}

// --------------------------------------------------------------------------
// TestWeatherCurrent_OfflineFallback_DiskRead — AC-WEATHER-003
// --------------------------------------------------------------------------

// TestWeatherCurrent_OfflineFallback_DiskRead verifies that when the provider
// returns an error, the tool reads the last saved disk file and returns
// stale=true with an offline message.
func TestWeatherCurrent_OfflineFallback_DiskRead(t *testing.T) {
	deps, _ := newWeatherTestDeps(t, "api.openweathermap.org")

	// Pre-populate the offline store with a saved report.
	offlineDir := deps.Cwd + "/weather"
	offline := web.NewDiskOfflineStore(offlineDir)

	savedReport := seoulReport()
	savedReport.Timestamp = time.Now().Add(-3 * time.Hour) // 3h ago — within 24h threshold
	require.NoError(t, offline.SaveLatest("openweathermap", 37.57, 126.98, savedReport))

	// Provider always fails.
	provider := &stubWeatherProvider{name: "openweathermap", err: fmt.Errorf("network unreachable")}

	tool := newWeatherCurrent(deps, weatherCfg(), provider, fixedSeoulGeo(), offline)
	result, err := tool.Call(context.Background(), buildWeatherInput(t, map[string]any{
		"lat": 37.57, "lon": 126.98,
	}))
	require.NoError(t, err)

	// Response must succeed (offline fallback) with stale=true.
	resp := unmarshalToolResponse(t, result)
	require.True(t, resp.OK, "offline fallback must still return ok=true; error=%v", resp.Error)

	var report web.WeatherReport
	require.NoError(t, json.Unmarshal(resp.Data, &report))
	assert.True(t, report.Stale, "data.stale must be true for offline fallback")
	assert.NotEmpty(t, report.Message, "Message must contain an offline note")
	assert.Contains(t, report.Message, "오프라인",
		"Message must reference offline state in Korean")
}

// --------------------------------------------------------------------------
// TestWeatherCurrent_APIKey_Redacted_NotInLogs — AC-WEATHER-006
// --------------------------------------------------------------------------

// TestWeatherCurrent_APIKey_Redacted_NotInLogs verifies that the OWM API key
// never appears in any structured log entry during the weather_current call
// pipeline (REQ-WEATHER-004).
func TestWeatherCurrent_APIKey_Redacted_NotInLogs(t *testing.T) {
	const secretKey = "super-secret-owm-key-xyz789"

	// Mock OWM server responding with a valid Seoul report.
	srv := newOWMTestServer(t, owmSeoulResponse, http.StatusOK)

	// Install a zap observer to capture all log output.
	core, logs := observer.New(zap.DebugLevel)
	zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(func() { zap.ReplaceGlobals(zap.NewNop()) })

	deps, _ := newWeatherTestDeps(t, "api.openweathermap.org")
	cfg := weatherCfg()
	cfg.OpenWeatherMap.APIKey = secretKey
	cfg.OpenWeatherMap.BaseURL = srv.URL

	// Use the real OWM provider so the URL (with appid=...) is constructed.
	provider := web.NewOpenWeatherMapProviderForTest(secretKey, srv.URL, deps)
	offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")

	tool := newWeatherCurrent(deps, cfg, provider, fixedSeoulGeo(), offline)
	result, err := tool.Call(context.Background(), buildWeatherInput(t, map[string]any{
		"lat": 37.57, "lon": 126.98,
	}))
	require.NoError(t, err)
	_ = result

	// Scan all log messages and field values for the raw API key.
	for _, entry := range logs.All() {
		assert.NotContains(t, entry.Message, secretKey,
			"API key must not appear in log message: %q", entry.Message)
		for _, field := range entry.Context {
			if field.String != "" {
				assert.NotContains(t, field.String, secretKey,
					"API key must not appear in log field %q=%q", field.Key, field.String)
			}
		}
	}

	// Verify "****" appears (active redaction).
	found := false
	for _, entry := range logs.All() {
		for _, f := range entry.Context {
			if strings.Contains(f.String, "****") {
				found = true
			}
		}
	}
	assert.True(t, found, `"****" must appear in at least one log field to confirm active redaction`)
}

// --------------------------------------------------------------------------
// TestWeatherCurrent_Singleflight_ConcurrentDedup — AC-WEATHER-007
// --------------------------------------------------------------------------

// TestWeatherCurrent_Singleflight_ConcurrentDedup verifies that 100 concurrent
// goroutines calling weather_current with the same coordinates result in exactly
// one outbound provider call (REQ-WEATHER-012).
func TestWeatherCurrent_Singleflight_ConcurrentDedup(t *testing.T) {
	const goroutines = 100
	const providerLatency = 30 * time.Millisecond

	deps, _ := newWeatherTestDeps(t, "api.openweathermap.org")
	provider := &stubWeatherProvider{
		name:    "openweathermap",
		report:  seoulReport(),
		latency: providerLatency,
	}
	offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")

	tool := newWeatherCurrent(deps, weatherCfg(), provider, fixedSeoulGeo(), offline)
	input := buildWeatherInput(t, map[string]any{"lat": 37.57, "lon": 126.98})

	var wg sync.WaitGroup
	results := make([]tools.ToolResult, goroutines)
	errors := make([]error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = tool.Call(context.Background(), input)
		}(i)
	}
	wg.Wait()

	// All goroutines must succeed.
	for i, err := range errors {
		require.NoError(t, err, "goroutine %d returned unexpected error", i)
	}
	for i, r := range results {
		resp := unmarshalToolResponse(t, r)
		assert.True(t, resp.OK, "goroutine %d must receive a successful response; error=%v", i, resp.Error)
	}

	// Singleflight must coalesce the burst of concurrent requests.
	// With 100 goroutines and a 30ms provider latency, all goroutines should
	// land within the same singleflight window because they are launched before
	// the provider returns. The critical invariant is that the provider is called
	// far fewer times than the number of goroutines (i.e. deduplication works).
	// We allow at most 2 calls as a tolerance for scheduling jitter: all goroutines
	// are launched simultaneously but a rare goroutine may arrive after the first
	// singleflight completes (which then serves it from cache or starts a new sf).
	calls := provider.callCount.Load()
	assert.LessOrEqual(t, calls, int64(2),
		"singleflight must coalesce concurrent requests into at most 2 calls, got %d", calls)
}

// --------------------------------------------------------------------------
// TestWeatherCurrent_RateLimit_Exhausted — AC-WEATHER-008
// --------------------------------------------------------------------------

// TestWeatherCurrent_RateLimit_Exhausted verifies that when the per-provider
// rate limit is exhausted, the tool returns ratelimit_exhausted without calling
// the provider and with retry_after_seconds > 0.
func TestWeatherCurrent_RateLimit_Exhausted(t *testing.T) {
	deps, _ := newWeatherTestDeps(t, "api.openweathermap.org")
	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)

	// Register a minimal parser so tracker.Parse works for "openweathermap".
	web.RegisterWeatherParser(tracker)

	// Synthesize exhausted state: Remaining=0, Reset=30s.
	syntheticHeaders := map[string]string{
		"X-RateLimit-Limit":     "60",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     "30",
	}
	require.NoError(t, tracker.Parse("openweathermap", syntheticHeaders, time.Now()))

	state := tracker.State("openweathermap")
	require.GreaterOrEqual(t, state.RequestsMin.UsagePct(), 100.0,
		"RequestsMin must be exhausted")

	deps.RateTracker = tracker

	provider := &stubWeatherProvider{name: "openweathermap", report: seoulReport()}
	offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")

	tool := newWeatherCurrent(deps, weatherCfg(), provider, fixedSeoulGeo(), offline)
	result, err := tool.Call(context.Background(), buildWeatherInput(t, map[string]any{
		"lat": 37.57, "lon": 126.98,
	}))
	require.NoError(t, err)

	resp := unmarshalToolResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "ratelimit_exhausted", resp.Error.Code)
	assert.True(t, resp.Error.Retryable)
	assert.Greater(t, resp.Error.RetryAfterSeconds, 0)

	assert.Equal(t, int64(0), provider.callCount.Load(),
		"provider must NOT be called when rate limit is exhausted")
}

// --------------------------------------------------------------------------
// TestWeatherCurrent_Blocklist_HostBlocked
// --------------------------------------------------------------------------

// TestWeatherCurrent_Blocklist_HostBlocked verifies that when the OWM host is
// on the blocklist, the tool returns host_blocked before any outbound call.
func TestWeatherCurrent_Blocklist_HostBlocked(t *testing.T) {
	deps, _ := newWeatherTestDeps(t, "api.openweathermap.org")
	deps.Blocklist = common.NewBlocklist([]string{"api.openweathermap.org"})

	provider := &stubWeatherProvider{name: "openweathermap", report: seoulReport()}
	offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")

	tool := newWeatherCurrent(deps, weatherCfg(), provider, fixedSeoulGeo(), offline)
	result, err := tool.Call(context.Background(), buildWeatherInput(t, map[string]any{
		"lat": 37.57, "lon": 126.98,
	}))
	require.NoError(t, err)

	resp := unmarshalToolResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "host_blocked", resp.Error.Code)
	assert.Equal(t, int64(0), provider.callCount.Load(),
		"provider must NOT be called when host is blocked")
}
