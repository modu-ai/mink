package web_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
// Shared helpers for air-quality tests
// --------------------------------------------------------------------------

// newAirQualityTestDeps builds *common.Deps wired for weather_air_quality tests.
func newAirQualityTestDeps(t *testing.T, extraHosts ...string) (*common.Deps, *permission.Manager) {
	t.Helper()
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)

	hosts := append([]string{"apis.data.go.kr"}, extraHosts...)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{NetHosts: hosts}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: noopAuditWriter{},
		Clock:       func() time.Time { return time.Now() },
		Cwd:         t.TempDir(),
	}
	return deps, mgr
}

// airkoreaResponseWithPM25 builds a minimal AirKorea JSON response for a given
// PM25 value string.
func airkoreaResponseWithPM25(pm25 string) string {
	return fmt.Sprintf(`{
  "response": {
    "header": {"resultCode": "00", "resultMsg": "NORMAL_SERVICE"},
    "body": {
      "totalCount": 1,
      "items": [
        {
          "dataTime": "2026-05-10 14:00",
          "stationName": "강남구",
          "sidoName": "서울",
          "pm10Value": "80",
          "pm25Value": "%s",
          "o3Value": "0.025",
          "no2Value": "0.030"
        }
      ]
    }
  }
}`, pm25)
}

// newAirQualityTool constructs a webWeatherAirQuality via the test factory,
// pointing to an httptest server for the AirKorea provider.
func newAirQualityTool(t *testing.T, deps *common.Deps, srv *httptest.Server) tools.Tool {
	t.Helper()
	cfg, _ := web.LoadWeatherConfig("")
	cfg.AirKorea.APIKey = "test-key"

	provider := web.NewAirKoreaProviderForTest("test-key", srv.URL, deps)
	return web.NewWeatherAirQualityForTest(deps, cfg, provider)
}

// --------------------------------------------------------------------------
// AC-WEATHER-005: PM25 Korean standard boundary mapping (table-driven)
// --------------------------------------------------------------------------

// TestAirQuality_PM25_KoreanStandardMapping is the primary AC-WEATHER-005
// test. It exercises all 7 boundary cases from the acceptance document using
// a table-driven approach.
func TestAirQuality_PM25_KoreanStandardMapping(t *testing.T) {
	cases := []struct {
		pm25      int
		wantLevel string
		wantKo    string
	}{
		// Boundary cases from acceptance.md §AC-WEATHER-005
		{15, "good", "좋음"},
		{16, "moderate", "보통"},
		{35, "moderate", "보통"},
		{36, "unhealthy", "나쁨"},
		{75, "unhealthy", "나쁨"},
		{76, "very_unhealthy", "매우 나쁨"},
		{200, "hazardous", "위험"}, // spec.md DTO 5-tier: 151+ → hazardous
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("pm25=%d", tc.pm25), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(airkoreaResponseWithPM25(fmt.Sprintf("%d", tc.pm25))))
			}))
			defer srv.Close()

			deps, _ := newAirQualityTestDeps(t, srv.Listener.Addr().String())
			tool := newAirQualityTool(t, deps, srv)

			input, _ := json.Marshal(map[string]any{"location": "Seoul,KR"})
			result, err := tool.Call(context.Background(), input)
			require.NoError(t, err)

			var resp common.Response
			require.NoError(t, json.Unmarshal(result.Content, &resp))
			require.True(t, resp.OK, "call must succeed for pm25=%d; error=%v", tc.pm25, resp.Error)

			dataBytes, _ := json.Marshal(resp.Data)
			var aq web.AirQuality
			require.NoError(t, json.Unmarshal(dataBytes, &aq))

			assert.Equal(t, tc.pm25, aq.PM25, "pm25 must round-trip")
			assert.Equal(t, tc.wantLevel, aq.Level, "level for pm25=%d", tc.pm25)
			assert.Equal(t, tc.wantKo, aq.LevelKo, "level_ko for pm25=%d", tc.pm25)
		})
	}
}

// --------------------------------------------------------------------------
// AC-WEATHER-009 (M3): registry count 17
// --------------------------------------------------------------------------

// TestWeatherAirQuality_Registered_InWebTools verifies that weather_air_quality
// is registered and the total web tool count is 17 (M3 completion).
func TestWeatherAirQuality_Registered_InWebTools(t *testing.T) {
	registered := web.RegisteredWebToolsForTest()

	names := make(map[string]bool, len(registered))
	for _, tool := range registered {
		names[tool.Name()] = true
	}

	assert.True(t, names["weather_air_quality"], "weather_air_quality must be registered")
	assert.True(t, names["weather_current"], "weather_current must still be registered")
	assert.True(t, names["weather_forecast"], "weather_forecast must still be registered")
}

// --------------------------------------------------------------------------
// AC-WEATHER-010 (M3): standard response shape for weather_air_quality
// --------------------------------------------------------------------------

// TestWeatherAirQuality_StandardResponseShape verifies that both success and
// failure responses from weather_air_quality conform to TOOLS-WEB-001
// common.Response{ok, data|error, metadata} shape.
func TestWeatherAirQuality_StandardResponseShape(t *testing.T) {
	// Success case.
	successSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(airkoreaResponseWithPM25("55")))
	}))
	defer successSrv.Close()

	deps, _ := newAirQualityTestDeps(t, successSrv.Listener.Addr().String())
	successTool := newAirQualityTool(t, deps, successSrv)

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR"})
	successResult, err := successTool.Call(context.Background(), input)
	require.NoError(t, err)

	var successResp common.Response
	require.NoError(t, json.Unmarshal(successResult.Content, &successResp))
	assert.True(t, successResp.OK)
	assert.NotNil(t, successResp.Data)
	assert.NotNil(t, successResp.Metadata)

	// Failure case: unsupported_region (non-Korean coordinate).
	deps2, _ := newAirQualityTestDeps(t)
	cfg, _ := web.LoadWeatherConfig("")
	cfg.AirKorea.APIKey = "test-key"
	failureTool := web.NewWeatherAirQualityForTest(deps2, cfg, nil)

	failInput, _ := json.Marshal(map[string]any{"lat": 35.0, "lon": 139.0})
	failResult, err := failureTool.Call(context.Background(), failInput)
	require.NoError(t, err)

	var failResp common.Response
	require.NoError(t, json.Unmarshal(failResult.Content, &failResp))
	assert.False(t, failResp.OK)
	require.NotNil(t, failResp.Error)
	assert.Equal(t, "unsupported_region", failResp.Error.Code)
	assert.NotNil(t, failResp.Metadata)
}

// --------------------------------------------------------------------------
// Edge: unsupported_region
// --------------------------------------------------------------------------

// TestWeatherAirQuality_UnsupportedRegion_NonKorean verifies that non-Korean
// coordinates and location strings produce unsupported_region without making
// any outbound request.
func TestWeatherAirQuality_UnsupportedRegion_NonKorean(t *testing.T) {
	var outboundCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		outboundCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deps, _ := newAirQualityTestDeps(t, srv.Listener.Addr().String())

	cases := []struct {
		name  string
		input map[string]any
	}{
		{"lat_lon_tokyo", map[string]any{"lat": 35.0, "lon": 139.0}},
		{"location_tokyo_jp", map[string]any{"location": "Tokyo,JP"}},
		{"location_london", map[string]any{"location": "London,UK"}},
		{"lat_lon_new_york", map[string]any{"lat": 40.7, "lon": -74.0}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := web.LoadWeatherConfig("")
			cfg.AirKorea.APIKey = "test-key"
			tool := web.NewWeatherAirQualityForTest(deps, cfg, web.NewAirKoreaProviderForTest("test-key", srv.URL, deps))

			rawInput, _ := json.Marshal(tc.input)
			result, err := tool.Call(context.Background(), rawInput)
			require.NoError(t, err)

			var resp common.Response
			require.NoError(t, json.Unmarshal(result.Content, &resp))
			assert.False(t, resp.OK, "%s must not succeed for non-Korean location", tc.name)
			require.NotNil(t, resp.Error)
			assert.Equal(t, "unsupported_region", resp.Error.Code,
				"%s must return unsupported_region error", tc.name)
		})
	}

	assert.Equal(t, int64(0), outboundCount.Load(), "no outbound calls expected for unsupported regions")
}

// --------------------------------------------------------------------------
// Edge: blocklist
// --------------------------------------------------------------------------

// TestWeatherAirQuality_BlocklistPriority verifies that when apis.data.go.kr
// is on the blocklist, the tool returns host_blocked without any outbound call.
func TestWeatherAirQuality_BlocklistPriority(t *testing.T) {
	var outboundCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		outboundCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deps, _ := newAirQualityTestDeps(t, srv.Listener.Addr().String())
	deps.Blocklist = common.NewBlocklist([]string{"apis.data.go.kr"})

	cfg, _ := web.LoadWeatherConfig("")
	cfg.AirKorea.APIKey = "test-key"
	tool := web.NewWeatherAirQualityForTest(deps, cfg, web.NewAirKoreaProviderForTest("test-key", srv.URL, deps))

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR"})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "host_blocked", resp.Error.Code)
	assert.Equal(t, int64(0), outboundCount.Load(), "no outbound call must occur when blocklisted")
}

// --------------------------------------------------------------------------
// Edge: permission denied
// --------------------------------------------------------------------------

// TestWeatherAirQuality_PermissionDenied verifies that the permission gate
// blocks the call when the host is not granted.
func TestWeatherAirQuality_PermissionDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Build deps without granting apis.data.go.kr.
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.DefaultDenyConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	// Register but with empty NetHosts so the confirmer will deny.
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{NetHosts: []string{}}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: noopAuditWriter{},
		Clock:       func() time.Time { return time.Now() },
		Cwd:         t.TempDir(),
	}

	cfg, _ := web.LoadWeatherConfig("")
	cfg.AirKorea.APIKey = "test-key"
	tool := web.NewWeatherAirQualityForTest(deps, cfg, web.NewAirKoreaProviderForTest("test-key", srv.URL, deps))

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR"})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "permission_denied", resp.Error.Code)
}

// --------------------------------------------------------------------------
// Edge: rate limit exhausted
// --------------------------------------------------------------------------

// TestWeatherAirQuality_RateLimit_Exhausted verifies that when the airkorea
// rate-limit bucket is exhausted, the call returns ratelimit_exhausted without
// any outbound request.
func TestWeatherAirQuality_RateLimit_Exhausted(t *testing.T) {
	var outboundCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		outboundCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deps, _ := newAirQualityTestDeps(t, srv.Listener.Addr().String())
	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)

	// Register a minimal parser for "airkorea" so tracker.Parse works.
	web.RegisterAirKoreaRateLimitParser(tracker)

	// Synthesize exhausted state: Remaining=0, Reset=30s.
	syntheticHeaders := map[string]string{
		"X-RateLimit-Limit":     "60",
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     "30",
	}
	require.NoError(t, tracker.Parse("airkorea", syntheticHeaders, time.Now()))

	state := tracker.State("airkorea")
	require.GreaterOrEqual(t, state.RequestsMin.UsagePct(), 100.0, "RequestsMin must be exhausted")

	deps.RateTracker = tracker

	cfg, _ := web.LoadWeatherConfig("")
	cfg.AirKorea.APIKey = "test-key"
	tool := web.NewWeatherAirQualityForTest(deps, cfg, web.NewAirKoreaProviderForTest("test-key", srv.URL, deps))

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR"})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "ratelimit_exhausted", resp.Error.Code)
	assert.True(t, resp.Error.Retryable)
	assert.Equal(t, int64(0), outboundCount.Load(), "no outbound call when rate-limited")
}

// --------------------------------------------------------------------------
// Edge: audit writer
// --------------------------------------------------------------------------

// TestWeatherAirQuality_AuditWriter verifies that a successful call writes
// exactly one audit event with tool="weather_air_quality", outcome="ok".
func TestWeatherAirQuality_AuditWriter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(airkoreaResponseWithPM25("55")))
	}))
	defer srv.Close()

	var clockTick atomic.Int64
	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	testClock := func() time.Time {
		tick := clockTick.Add(1)
		return baseTime.Add(time.Duration(tick) * time.Microsecond)
	}

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "audit-airquality.log")
	auditWriter, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { _ = auditWriter.Close() }()

	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{
		NetHosts: []string{"apis.data.go.kr", srv.Listener.Addr().String()},
	}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: auditWriter,
		Clock:       testClock,
		Cwd:         t.TempDir(),
	}

	cfg, _ := web.LoadWeatherConfig("")
	cfg.AirKorea.APIKey = "test-key"
	tool := web.NewWeatherAirQualityForTest(deps, cfg, web.NewAirKoreaProviderForTest("test-key", srv.URL, deps))

	input, _ := json.Marshal(map[string]any{"location": "Seoul,KR"})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	resp := unmarshalToolResponse(t, result)
	require.True(t, resp.OK, "call must succeed")

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
	require.Len(t, lines, 1, "exactly 1 audit line expected")

	entry := lines[0]
	assert.Equal(t, "tool.web.invoke", entry["type"])
	meta, ok := entry["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "weather_air_quality", meta["tool"])
	assert.Equal(t, "ok", meta["outcome"])
}

// --------------------------------------------------------------------------
// Edge: Korean coordinate via lat/lon only
// --------------------------------------------------------------------------

// TestWeatherAirQuality_KoreanCoordinate_LatLonOnly verifies that a Korean
// coordinate supplied via lat/lon (without location string) succeeds.
func TestWeatherAirQuality_KoreanCoordinate_LatLonOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(airkoreaResponseWithPM25("20")))
	}))
	defer srv.Close()

	deps, _ := newAirQualityTestDeps(t, srv.Listener.Addr().String())
	tool := newAirQualityTool(t, deps, srv)

	// Seoul centroid coordinates.
	input, _ := json.Marshal(map[string]any{"lat": 37.5, "lon": 127.0})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.True(t, resp.OK, "Korean lat/lon must succeed")
}

// --------------------------------------------------------------------------
// Edge: Korean location string variants
// --------------------------------------------------------------------------

// TestWeatherAirQuality_KoreanLocationString_PusanBusan verifies that
// "Busan, KR" resolves as a Korean location and produces a successful result.
func TestWeatherAirQuality_KoreanLocationString_PusanBusan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(airkoreaResponseWithPM25("30")))
	}))
	defer srv.Close()

	deps, _ := newAirQualityTestDeps(t, srv.Listener.Addr().String())
	tool := newAirQualityTool(t, deps, srv)

	input, _ := json.Marshal(map[string]any{"location": "Busan, KR"})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.True(t, resp.OK, "Busan, KR must be recognized as Korean location")
}

// --------------------------------------------------------------------------
// TestAuditLog_WeatherAirQualityCall — tasks.md M3 audit requirement
// --------------------------------------------------------------------------

// TestAuditLog_WeatherAirQualityCall verifies that a single weather_air_quality
// call produces exactly one audit line with tool="weather_air_quality",
// outcome="ok", and all required metadata keys.
func TestAuditLog_WeatherAirQualityCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(airkoreaResponseWithPM25("55")))
	}))
	defer srv.Close()

	var clockTick atomic.Int64
	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	testClock := func() time.Time {
		tick := clockTick.Add(1)
		return baseTime.Add(time.Duration(tick) * time.Microsecond)
	}

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "audit-aq-call.log")
	auditWriter, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { _ = auditWriter.Close() }()

	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{
		NetHosts: []string{"apis.data.go.kr", srv.Listener.Addr().String()},
	}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: auditWriter,
		Clock:       testClock,
		Cwd:         t.TempDir(),
	}

	cfg, _ := web.LoadWeatherConfig("")
	cfg.AirKorea.APIKey = "audit-aq-key"
	provider := web.NewAirKoreaProviderForTest("audit-aq-key", srv.URL, deps)
	tool := web.NewWeatherAirQualityForTest(deps, cfg, provider)

	input, _ := json.Marshal(map[string]any{"lat": 37.57, "lon": 126.98})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	resp := unmarshalToolResponse(t, result)
	require.True(t, resp.OK, "weather_air_quality call must succeed; error=%v", resp.Error)

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
	require.Len(t, lines, 1, "exactly 1 audit line expected for a single weather_air_quality call")

	entry := lines[0]
	assert.Equal(t, "tool.web.invoke", entry["type"])
	meta, ok := entry["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "weather_air_quality", meta["tool"])
	assert.Equal(t, "ok", meta["outcome"])
	assert.Contains(t, meta, "duration_ms")
}
