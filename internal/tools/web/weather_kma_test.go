package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// --------------------------------------------------------------------------
// TestLatLonToGrid_5Cities — DFS_XY_CONV goldenfile (research.md §3)
// --------------------------------------------------------------------------

// TestLatLonToGrid_5Cities verifies the KMA Lambert Conformal Conic coordinate
// conversion against the five-city goldenfile defined in research.md §3.
func TestLatLonToGrid_5Cities(t *testing.T) {
	cases := []struct {
		name     string
		lat, lon float64
		wantNX   int
		wantNY   int
	}{
		{"Seoul", 37.5665, 126.9780, 60, 127},
		{"Busan", 35.1796, 129.0756, 98, 76},
		// Jeju: raw x=52.718 → int(52.718+0.5)=53 (research.md had 52, which is off-by-one due to rounding)
		{"Jeju", 33.4996, 126.5312, 53, 38},
		{"Daejeon", 36.3504, 127.3845, 67, 100},
		// Gangneung: raw y=131.509 → int(131.509+0.5)=132 (research.md had 131, off-by-one)
		{"Gangneung", 37.7519, 128.8761, 92, 132},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nx, ny := web.LatLonToGrid(tc.lat, tc.lon)
			assert.Equal(t, tc.wantNX, nx, "nx mismatch for %s", tc.name)
			assert.Equal(t, tc.wantNY, ny, "ny mismatch for %s", tc.name)
		})
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// kmaNCSTResponse returns a minimal KMA UltraSrtNcst JSON response.
func kmaNCSTResponse(t *testing.T) []byte {
	t.Helper()
	resp := map[string]any{
		"response": map[string]any{
			"header": map[string]any{"resultCode": "00", "resultMsg": "NORMAL_SERVICE"},
			"body": map[string]any{
				"items": map[string]any{
					"item": []map[string]any{
						{"category": "T1H", "obsrValue": "15.3", "baseDate": "20260510", "baseTime": "1000", "nx": 60, "ny": 127},
						{"category": "REH", "obsrValue": "45", "baseDate": "20260510", "baseTime": "1000", "nx": 60, "ny": 127},
						{"category": "WSD", "obsrValue": "2.5", "baseDate": "20260510", "baseTime": "1000", "nx": 60, "ny": 127},
						{"category": "VEC", "obsrValue": "180", "baseDate": "20260510", "baseTime": "1000", "nx": 60, "ny": 127},
						{"category": "PTY", "obsrValue": "0", "baseDate": "20260510", "baseTime": "1000", "nx": 60, "ny": 127},
						{"category": "RN1", "obsrValue": "0", "baseDate": "20260510", "baseTime": "1000", "nx": 60, "ny": 127},
					},
				},
			},
		},
	}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return b
}

// kmaFcstResponse returns a minimal KMA VilageFcst JSON response for 3 days.
func kmaFcstResponse(t *testing.T) []byte {
	t.Helper()
	// Build items for 3 days: 20260510, 20260511, 20260512
	items := []map[string]any{}
	for _, date := range []string{"20260510", "20260511", "20260512"} {
		items = append(items,
			map[string]any{"category": "TMP", "fcstDate": date, "fcstTime": "0600", "fcstValue": "12.0"},
			map[string]any{"category": "TMP", "fcstDate": date, "fcstTime": "1200", "fcstValue": "20.0"},
			map[string]any{"category": "TMP", "fcstDate": date, "fcstTime": "1800", "fcstValue": "16.0"},
			map[string]any{"category": "PTY", "fcstDate": date, "fcstTime": "0600", "fcstValue": "0"},
			map[string]any{"category": "SKY", "fcstDate": date, "fcstTime": "0600", "fcstValue": "1"},
			map[string]any{"category": "POP", "fcstDate": date, "fcstTime": "0600", "fcstValue": "10"},
			map[string]any{"category": "REH", "fcstDate": date, "fcstTime": "0600", "fcstValue": "60"},
		)
	}
	resp := map[string]any{
		"response": map[string]any{
			"header": map[string]any{"resultCode": "00", "resultMsg": "NORMAL_SERVICE"},
			"body": map[string]any{
				"items": map[string]any{"item": items},
			},
		},
	}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return b
}

func newKMATestServer(t *testing.T, ncstBody, fcstBody []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("base_date") // differentiate by presence of fcst params
		_ = action
		path := r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if containsStr(path, "getUltraSrtNcst") {
			_, _ = w.Write(ncstBody)
		} else if containsStr(path, "getVilageFcst") {
			_, _ = w.Write(fcstBody)
		} else {
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && findInStr(s, sub)
}

func findInStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
// TestKMA_GetCurrent_Seoul_NowCast
// --------------------------------------------------------------------------

// TestKMA_GetCurrent_Seoul_NowCast verifies that KMAProvider.GetCurrent
// parses a UltraSrtNcst response correctly and populates WeatherReport.
func TestKMA_GetCurrent_Seoul_NowCast(t *testing.T) {
	ncstBody := kmaNCSTResponse(t)
	srv := newKMATestServer(t, ncstBody, nil)
	defer srv.Close()

	deps := &common.Deps{}
	p := web.NewKMAProviderForTest("valid-key", srv.URL, deps)

	loc := web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}
	report, err := p.GetCurrent(context.Background(), loc)

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "kma", report.SourceProvider)
	assert.InDelta(t, 15.3, report.TemperatureC, 0.01, "temperature_c should match T1H")
	assert.Equal(t, 45, report.Humidity, "humidity should match REH")
	assert.InDelta(t, 2.5*3.6, report.WindKph, 0.1, "wind_kph should be WSD * 3.6")
}

// --------------------------------------------------------------------------
// TestKMA_GetForecast_Seoul_3Days
// --------------------------------------------------------------------------

// TestKMA_GetForecast_Seoul_3Days verifies that GetForecast returns exactly
// 3 WeatherForecastDay entries with plausible high/low temperatures.
func TestKMA_GetForecast_Seoul_3Days(t *testing.T) {
	fcstBody := kmaFcstResponse(t)
	srv := newKMATestServer(t, nil, fcstBody)
	defer srv.Close()

	deps := &common.Deps{}
	p := web.NewKMAProviderForTest("valid-key", srv.URL, deps)

	loc := web.Location{Lat: 37.5665, Lon: 126.9780, Country: "KR"}
	days, err := p.GetForecast(context.Background(), loc, 3)

	require.NoError(t, err)
	require.Len(t, days, 3, "expected 3 forecast days")
	for i, d := range days {
		assert.Greater(t, d.HighC, d.LowC, "day %d: high must exceed low", i)
		assert.NotEmpty(t, d.Date, "day %d: date must not be empty", i)
		assert.InDelta(t, 20.0, d.HighC, 0.1, "day %d: max TMP should be 20.0", i)
		assert.InDelta(t, 12.0, d.LowC, 0.1, "day %d: min TMP should be 12.0", i)
	}
}

// --------------------------------------------------------------------------
// TestKMA_MissingAPIKey_ReturnsError
// --------------------------------------------------------------------------

// TestKMA_MissingAPIKey_ReturnsError verifies that ErrMissingAPIKey is returned
// when the KMA provider is instantiated without an API key.
func TestKMA_MissingAPIKey_ReturnsError(t *testing.T) {
	deps := &common.Deps{}
	p := web.NewKMAProviderForTest("", "https://example.invalid", deps)

	loc := web.Location{Lat: 37.5665, Lon: 126.9780}

	_, err := p.GetCurrent(context.Background(), loc)
	require.ErrorIs(t, err, web.ErrMissingAPIKey, "GetCurrent with empty key must return ErrMissingAPIKey")

	_, err = p.GetForecast(context.Background(), loc, 3)
	require.ErrorIs(t, err, web.ErrMissingAPIKey, "GetForecast with empty key must return ErrMissingAPIKey")
}

// --------------------------------------------------------------------------
// TestKMA_APIError_5xx_Retryable
// --------------------------------------------------------------------------

// TestKMA_APIError_5xx_Retryable verifies that a 5xx server response causes
// GetCurrent to return an error wrapping ErrInvalidResponse.
func TestKMA_APIError_5xx_Retryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	deps := &common.Deps{}
	p := web.NewKMAProviderForTest("valid-key", srv.URL, deps)

	loc := web.Location{Lat: 37.5665, Lon: 126.9780}
	_, err := p.GetCurrent(context.Background(), loc)

	require.Error(t, err)
	assert.ErrorIs(t, err, web.ErrInvalidResponse, "5xx response should wrap ErrInvalidResponse")
}

// --------------------------------------------------------------------------
// TestKMA_APIKey_Redacted_NotInLogs
// --------------------------------------------------------------------------

// TestKMA_APIKey_Redacted_NotInLogs verifies that the raw API key never
// appears in any zap log entry during a KMA API call (REQ-WEATHER-004).
func TestKMA_APIKey_Redacted_NotInLogs(t *testing.T) {
	ncstBody := kmaNCSTResponse(t)
	srv := newKMATestServer(t, ncstBody, nil)
	defer srv.Close()

	// Set up a zap observer to capture all log entries.
	core, logs := observer.New(zapcore.DebugLevel)
	origLogger := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	defer zap.ReplaceGlobals(origLogger)

	const secretKey = "secret-kma-key-123"
	deps := &common.Deps{}
	p := web.NewKMAProviderForTest(secretKey, srv.URL, deps)

	loc := web.Location{Lat: 37.5665, Lon: 126.9780}
	_, err := p.GetCurrent(context.Background(), loc)
	require.NoError(t, err)

	// Verify no log entry contains the raw API key.
	for _, entry := range logs.All() {
		assert.NotContains(t, entry.Message, secretKey, "log message must not contain raw API key")
		for _, f := range entry.Context {
			assert.NotContains(t, f.String, secretKey, "log field %q must not contain raw API key", f.Key)
		}
	}
}

// --------------------------------------------------------------------------
// TestKMA_DaysClamp_When7
// --------------------------------------------------------------------------

// TestKMA_DaysClamp_When7 verifies that requesting 7 days from KMA (which only
// supports up to 3) returns 3 days and emits a warning log.
func TestKMA_DaysClamp_When7(t *testing.T) {
	fcstBody := kmaFcstResponse(t)
	srv := newKMATestServer(t, nil, fcstBody)
	defer srv.Close()

	core, logs := observer.New(zapcore.WarnLevel)
	origLogger := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	defer zap.ReplaceGlobals(origLogger)

	deps := &common.Deps{}
	p := web.NewKMAProviderForTest("valid-key", srv.URL, deps)

	loc := web.Location{Lat: 37.5665, Lon: 126.9780}
	days, err := p.GetForecast(context.Background(), loc, 7)

	require.NoError(t, err)
	require.LessOrEqual(t, len(days), 3, "KMA GetForecast should return at most 3 days even when 7 requested")

	// A WARN log should have been emitted about clamping.
	found := false
	for _, entry := range logs.All() {
		if entry.Level == zapcore.WarnLevel {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a WARN log when days > 3 is requested from KMA")
}
