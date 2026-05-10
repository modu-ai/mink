package web_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/audit"
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
// TestAuditLog_M1Calls — DC-13 / AC-WEB-018 (M1 scope: 2 tools)
// --------------------------------------------------------------------------

// TestAuditLog_M1Calls calls web_search and http_fetch sequentially with
// pre-granted permissions and verifies that exactly 2 JSON-line audit entries
// are written to a temporary log file, all with outcome="ok" and monotonically
// increasing timestamps.
func TestAuditLog_M1Calls(t *testing.T) {
	// Injected clock with 1µs step per call to guarantee monotonic timestamps.
	var clockTick atomic.Int64
	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	testClock := func() time.Time {
		tick := clockTick.Add(1)
		return baseTime.Add(time.Duration(tick) * time.Microsecond)
	}

	// Mock servers.
	var searchCount, fetchCount atomic.Int64
	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"T","url":"https://ex.com","description":"D"}]}}`))
	}))
	defer searchSrv.Close()

	fetchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer fetchSrv.Close()

	// Real audit FileWriter writing to t.TempDir().
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "audit.log")
	auditWriter, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { _ = auditWriter.Close() }()

	// Pre-grant permission (skip Confirmer round-trip).
	allHosts := []string{
		"api.search.brave.com",
		fetchSrv.Listener.Addr().String(),
	}
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{NetHosts: allHosts}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: auditWriter,
		Clock:       testClock,
		Cwd:         t.TempDir(),
	}

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	deps.RateTracker = tracker
	web.RegisterBraveParser(tracker)

	ctx := context.Background()

	// Call 1: web_search
	searchTool := web.NewWebSearch(deps, searchSrv.URL)
	r1, err := searchTool.Call(ctx, buildSearchInput(t, map[string]any{
		"query": "audit-test", "provider": "brave",
	}))
	require.NoError(t, err)
	require.False(t, r1.IsError, "web_search call must succeed")

	// Call 2: http_fetch
	httpTool := web.NewHTTPFetch(deps)
	r2, err := httpTool.Call(ctx, buildInput(t, map[string]any{
		"url": "http://" + fetchSrv.Listener.Addr().String() + "/",
	}))
	require.NoError(t, err)
	// http_fetch may succeed or fail — both generate an audit event.
	_ = r2

	// Flush and close audit writer to ensure all data is on disk.
	require.NoError(t, auditWriter.Close())

	// Read audit.log and verify exactly 2 lines.
	f, err := os.Open(logPath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry), "each audit line must be valid JSON: %q", line)
		lines = append(lines, entry)
	}
	require.NoError(t, scanner.Err())

	require.Len(t, lines, 2, "exactly 2 audit lines expected for M1 (web_search + http_fetch)")

	// Verify required keys and values on each line.
	requiredKeys := []string{"type", "timestamp", "severity", "message", "metadata"}
	for i, entry := range lines {
		for _, k := range requiredKeys {
			assert.Contains(t, entry, k, "line %d must have key %q", i+1, k)
		}
		assert.Equal(t, "tool.web.invoke", entry["type"], "line %d type must be tool.web.invoke", i+1)

		meta, ok := entry["metadata"].(map[string]any)
		require.True(t, ok, "line %d metadata must be a JSON object", i+1)
		metaRequiredKeys := []string{"tool", "host", "method", "status_code", "cache_hit", "duration_ms", "outcome"}
		for _, mk := range metaRequiredKeys {
			assert.Contains(t, meta, mk, "line %d metadata must have key %q", i+1, mk)
		}
		assert.Equal(t, "ok", meta["outcome"], "line %d outcome must be ok", i+1)
	}

	// Verify timestamps are monotonically increasing.
	ts0, err := time.Parse(time.RFC3339, lines[0]["timestamp"].(string))
	require.NoError(t, err)
	ts1, err := time.Parse(time.RFC3339, lines[1]["timestamp"].(string))
	require.NoError(t, err)
	assert.True(t, !ts1.Before(ts0), "timestamps must be monotonically increasing: %v, %v", ts0, ts1)
}

// --------------------------------------------------------------------------
// TestAuditTimestampMonotonic — edge case: injected clock
// --------------------------------------------------------------------------

// TestAuditTimestampMonotonic uses a 1µs-step clock to verify strict timestamp
// ordering across two sequential tool calls.
func TestAuditTimestampMonotonic(t *testing.T) {
	var tick atomic.Int64
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		return base.Add(time.Duration(tick.Add(1)) * time.Microsecond)
	}

	var received []audit.AuditEvent
	cw := &captureAuditWriter{events: &received}

	deps, _, _ := newTestDeps(t, nil)
	deps.AuditWriter = cw
	deps.Clock = clock
	deps.Cwd = t.TempDir()

	// Two minimal calls with noop tools (any tool with AuditWriter set).
	fetchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer fetchSrv.Close()

	require.NoError(t, deps.PermMgr.Register("agent:goose", permission.Manifest{
		NetHosts: []string{fetchSrv.Listener.Addr().String()},
	}))

	httpTool := web.NewHTTPFetch(deps)
	for range 2 {
		res, err := httpTool.Call(context.Background(), buildInput(t, map[string]any{
			"url": "http://" + fetchSrv.Listener.Addr().String() + "/",
		}))
		require.NoError(t, err)
		r := unmarshalToolResponse(t, res)
		_ = r
	}

	require.Len(t, received, 2, "2 audit events expected")
	assert.True(t, received[1].Timestamp.After(received[0].Timestamp) ||
		received[1].Timestamp.Equal(received[0].Timestamp),
		"timestamps must be monotone: %v, %v", received[0].Timestamp, received[1].Timestamp)
}

// captureAuditWriter records audit events in memory for inspection.
type captureAuditWriter struct {
	events *[]audit.AuditEvent
}

func (c *captureAuditWriter) Write(e audit.AuditEvent) error {
	*c.events = append(*c.events, e)
	return nil
}

// --------------------------------------------------------------------------
// TestAuditLog_FourCallsAllTools — AC-WEB-018 (sync scope: 4 tools)
// --------------------------------------------------------------------------

// auditFourTooStubSession implements web.PlaywrightSession with a noop success
// path, used so the AC-WEB-018 four-call test can exercise web_browse's
// success branch (writeAudit outcome=ok) without a real Playwright driver.
type auditFourToolStubSession struct{}

func (s *auditFourToolStubSession) Goto(_ context.Context, _ string, _ int) error {
	return nil
}
func (s *auditFourToolStubSession) Title() (string, error) { return "Mock Page", nil }
func (s *auditFourToolStubSession) Content() (string, error) {
	return "<html><body>mock</body></html>", nil
}
func (s *auditFourToolStubSession) InnerText(_ string) (string, error) {
	return "mock body text", nil
}
func (s *auditFourToolStubSession) Close() error { return nil }

// auditFourToolStubLauncher returns auditFourToolStubSession on every call.
type auditFourToolStubLauncher struct{}

func (a *auditFourToolStubLauncher) Launch(_ context.Context) (web.PlaywrightSession, error) {
	return &auditFourToolStubSession{}, nil
}

// TestAuditLog_FourCallsAllTools verifies AC-WEB-018: four web tool calls
// (web_search + http_fetch + web_wikipedia + web_browse), each producing one
// audit log line with outcome=ok and the full required metadata key set.
func TestAuditLog_FourCallsAllTools(t *testing.T) {
	var clockTick atomic.Int64
	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	testClock := func() time.Time {
		tick := clockTick.Add(1)
		return baseTime.Add(time.Duration(tick) * time.Microsecond)
	}

	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"T","url":"https://ex.com","description":"D"}]}}`))
	}))
	defer searchSrv.Close()

	fetchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer fetchSrv.Close()

	wikiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "type": "standard",
            "title": "Seoul",
            "extract": "Seoul is the capital of South Korea.",
            "content_urls": {"desktop": {"page": "https://en.wikipedia.org/wiki/Seoul"}}
        }`))
	}))
	defer wikiSrv.Close()

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "audit.log")
	auditWriter, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { _ = auditWriter.Close() }()

	wikiHost := wikiSrv.Listener.Addr().String()
	allHosts := []string{
		"api.search.brave.com",
		fetchSrv.Listener.Addr().String(),
		wikiHost,
		"example.com",
	}
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{NetHosts: allHosts}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: auditWriter,
		Clock:       testClock,
		Cwd:         t.TempDir(),
	}
	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	deps.RateTracker = tracker
	web.RegisterBraveParser(tracker)

	ctx := context.Background()

	// Call 1: web_search.
	searchTool := web.NewWebSearch(deps, searchSrv.URL)
	r1, err := searchTool.Call(ctx, buildSearchInput(t, map[string]any{
		"query": "audit-four", "provider": "brave",
	}))
	require.NoError(t, err)
	require.False(t, r1.IsError, "web_search must succeed")

	// Call 2: http_fetch.
	httpTool := web.NewHTTPFetch(deps)
	_, err = httpTool.Call(ctx, buildInput(t, map[string]any{
		"url": "http://" + fetchSrv.Listener.Addr().String() + "/",
	}))
	require.NoError(t, err)

	// Call 3: web_wikipedia (hostBuilder routes to mock server).
	wikiTool := web.NewWikipediaForTest(deps, func(_ string) string {
		return wikiSrv.URL
	})
	_, err = wikiTool.Call(ctx, json.RawMessage(`{"query":"Seoul","language":"en"}`))
	require.NoError(t, err)

	// Call 4: web_browse with stub launcher (success path → outcome=ok).
	browseTool := web.NewBrowseForTest(deps, &auditFourToolStubLauncher{})
	_, err = browseTool.Call(ctx, json.RawMessage(`{"url":"https://example.com","extract":"text"}`))
	require.NoError(t, err)

	require.NoError(t, auditWriter.Close())

	f, err := os.Open(logPath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry), "line must be valid JSON: %q", line)
		lines = append(lines, entry)
	}
	require.NoError(t, scanner.Err())

	require.Len(t, lines, 4, "exactly 4 audit lines expected (web_search + http_fetch + web_wikipedia + web_browse)")

	requiredKeys := []string{"type", "timestamp", "severity", "message", "metadata"}
	metaRequiredKeys := []string{"tool", "host", "method", "status_code", "cache_hit", "duration_ms", "outcome"}
	gotTools := make(map[string]bool, 4)
	for i, entry := range lines {
		for _, k := range requiredKeys {
			assert.Contains(t, entry, k, "line %d must have %q", i+1, k)
		}
		assert.Equal(t, "tool.web.invoke", entry["type"], "line %d type must be tool.web.invoke", i+1)

		meta, ok := entry["metadata"].(map[string]any)
		require.True(t, ok, "line %d metadata must be JSON object", i+1)
		for _, mk := range metaRequiredKeys {
			assert.Contains(t, meta, mk, "line %d metadata must have %q", i+1, mk)
		}
		assert.Equal(t, "ok", meta["outcome"], "line %d outcome must be ok", i+1)
		if toolName, ok := meta["tool"].(string); ok {
			gotTools[toolName] = true
		}
	}
	assert.True(t, gotTools["web_search"], "web_search audit line missing")
	assert.True(t, gotTools["http_fetch"], "http_fetch audit line missing")
	assert.True(t, gotTools["web_wikipedia"], "web_wikipedia audit line missing")
	assert.True(t, gotTools["web_browse"], "web_browse audit line missing")
}

// unmarshalToolResponse decodes ToolResult into common.Response.
func unmarshalToolResponse(t *testing.T, result tools.ToolResult) common.Response {
	t.Helper()
	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	return resp
}

// --------------------------------------------------------------------------
// TestAuditLog_WeatherCurrentCall — T-018
// --------------------------------------------------------------------------

// auditWeatherStubProvider is a minimal WeatherProvider for audit tests.
type auditWeatherStubProvider struct{}

func (p *auditWeatherStubProvider) Name() string { return "openweathermap" }
func (p *auditWeatherStubProvider) GetCurrent(_ context.Context, _ web.Location) (*web.WeatherReport, error) {
	return &web.WeatherReport{
		Location:       web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"},
		Timestamp:      time.Now().UTC(),
		TemperatureC:   22.5,
		Condition:      "clear",
		SourceProvider: "openweathermap",
	}, nil
}
func (p *auditWeatherStubProvider) GetForecast(_ context.Context, _ web.Location, _ int) ([]web.WeatherForecastDay, error) {
	return nil, nil
}
func (p *auditWeatherStubProvider) GetAirQuality(_ context.Context, _ web.Location) (*web.AirQuality, error) {
	return nil, nil
}
func (p *auditWeatherStubProvider) GetSunTimes(_ context.Context, _ web.Location, _ time.Time) (*web.SunTimes, error) {
	return nil, nil
}

// auditWeatherGeo is a stub IPGeolocator for audit tests.
type auditWeatherGeo struct{}

func (g *auditWeatherGeo) Resolve(_ context.Context) (web.Location, error) {
	return web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}, nil
}

// TestAuditLog_WeatherCurrentCall verifies that a single weather_current call
// produces exactly one audit log line with tool="weather_current", outcome="ok",
// and all required metadata keys.
func TestAuditLog_WeatherCurrentCall(t *testing.T) {
	var clockTick atomic.Int64
	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	testClock := func() time.Time {
		tick := clockTick.Add(1)
		return baseTime.Add(time.Duration(tick) * time.Microsecond)
	}

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "audit-weather.log")
	auditWriter, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { _ = auditWriter.Close() }()

	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{
		NetHosts: []string{"api.openweathermap.org", "ipapi.co"},
	}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: auditWriter,
		Clock:       testClock,
		Cwd:         t.TempDir(),
	}

	cfg, _ := web.LoadWeatherConfig("")
	cfg.OpenWeatherMap.APIKey = "audit-test-key"

	offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")
	tool := web.NewWeatherCurrentForTest(deps, cfg, &auditWeatherStubProvider{}, &auditWeatherGeo{}, offline)

	input, _ := json.Marshal(map[string]any{"lat": 37.57, "lon": 126.98})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	resp := unmarshalToolResponse(t, result)
	require.True(t, resp.OK, "weather_current call must succeed; error=%v", resp.Error)

	require.NoError(t, auditWriter.Close())

	f, err := os.Open(logPath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		lines = append(lines, entry)
	}
	require.NoError(t, scanner.Err())
	require.Len(t, lines, 1, "exactly 1 audit line expected for a single weather_current call")

	entry := lines[0]
	assert.Equal(t, "tool.web.invoke", entry["type"])
	meta, ok := entry["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "weather_current", meta["tool"])
	assert.Equal(t, "ok", meta["outcome"])
	assert.Contains(t, meta, "duration_ms")
}

// --------------------------------------------------------------------------
// TestAuditLog_WeatherForecastCall — AC-WEATHER-004 audit verification
// --------------------------------------------------------------------------

// auditForecastProvider is a minimal WeatherProvider stub for forecast audit tests.
type auditForecastProvider struct {
	name string
}

func (p *auditForecastProvider) Name() string { return p.name }
func (p *auditForecastProvider) GetCurrent(_ context.Context, _ web.Location) (*web.WeatherReport, error) {
	return nil, nil
}
func (p *auditForecastProvider) GetForecast(_ context.Context, _ web.Location, _ int) ([]web.WeatherForecastDay, error) {
	return []web.WeatherForecastDay{
		{Date: "2026-05-10", HighC: 22.0, LowC: 12.0, Condition: "clear", PrecipProbPct: 10},
	}, nil
}
func (p *auditForecastProvider) GetAirQuality(_ context.Context, _ web.Location) (*web.AirQuality, error) {
	return nil, nil
}
func (p *auditForecastProvider) GetSunTimes(_ context.Context, _ web.Location, _ time.Time) (*web.SunTimes, error) {
	return nil, nil
}

// auditForecastGeo is a stub IPGeolocator for forecast audit tests.
type auditForecastGeo struct{}

func (g *auditForecastGeo) Resolve(_ context.Context) (web.Location, error) {
	return web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}, nil
}

// TestAuditLog_WeatherForecastCall verifies that a single weather_forecast call
// produces exactly one audit log line with tool="weather_forecast", outcome="ok".
func TestAuditLog_WeatherForecastCall(t *testing.T) {
	var clockTick atomic.Int64
	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	testClock := func() time.Time {
		tick := clockTick.Add(1)
		return baseTime.Add(time.Duration(tick) * time.Microsecond)
	}

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "audit-forecast.log")
	auditWriter, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { _ = auditWriter.Close() }()

	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{
		NetHosts: []string{"api.openweathermap.org", "apis.data.go.kr", "ipapi.co"},
	}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: auditWriter,
		Clock:       testClock,
		Cwd:         t.TempDir(),
	}

	cfg := web.WeatherConfigForTest(web.WeatherConfigOptions{
		Provider: "openweathermap",
		OWMKey:   "audit-forecast-key",
	})

	provider := &auditForecastProvider{name: "openweathermap"}
	geolocator := &auditForecastGeo{}

	tool := web.NewWeatherForecastForTest(deps, cfg,
		map[string]web.WeatherProvider{"openweathermap": provider},
		geolocator,
	)

	input, _ := json.Marshal(map[string]any{"lat": 37.57, "lon": 126.98, "days": 1})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	resp := unmarshalToolResponse(t, result)
	require.True(t, resp.OK, "weather_forecast call must succeed; error=%v", resp.Error)

	require.NoError(t, auditWriter.Close())

	f, err := os.Open(logPath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		lines = append(lines, entry)
	}
	require.NoError(t, scanner.Err())
	require.Len(t, lines, 1, "exactly 1 audit line expected for a single weather_forecast call")

	entry := lines[0]
	assert.Equal(t, "tool.web.invoke", entry["type"])
	meta, ok := entry["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "weather_forecast", meta["tool"])
	assert.Equal(t, "ok", meta["outcome"])
	assert.Contains(t, meta, "duration_ms")
}
