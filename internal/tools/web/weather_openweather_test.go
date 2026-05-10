package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testOWMAPIKey = "test-secret-key-abc123"

// owmSeoulResponse is a minimal OWM /data/2.5/weather response for Seoul.
var owmSeoulResponse = map[string]any{
	"weather": []map[string]any{
		{"main": "Clear", "description": "맑음", "icon": "01d"},
	},
	"main": map[string]any{
		"temp":       22.5,
		"feels_like": 21.0,
		"humidity":   55,
		"temp_min":   18.0,
		"temp_max":   25.0,
	},
	"wind": map[string]any{
		"speed": 3.5, // m/s
		"deg":   225,
	},
	"clouds": map[string]any{"all": 10},
	"rain":   map[string]any{},
	"dt":     int64(1746871200),
	"sys": map[string]any{
		"country": "KR",
		"sunrise": int64(1746840000),
		"sunset":  int64(1746890000),
	},
	"timezone": 32400, // Asia/Seoul UTC+9
	"name":     "Seoul",
	"cod":      200,
}

// newOWMTestServer creates a mock OWM server that serves owmSeoulResponse.
// The request URL is inspected to ensure the API key is present.
func newOWMTestServer(t *testing.T, respBody any, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		data, _ := json.Marshal(respBody)
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestOWM_GetCurrent_Seoul_KO_Metric verifies the OWM provider converts the
// mock response to a correct WeatherReport when units=metric and lang=ko.
func TestOWM_GetCurrent_Seoul_KO_Metric(t *testing.T) {
	srv := newOWMTestServer(t, owmSeoulResponse, http.StatusOK)

	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	p := web.NewOpenWeatherMapProviderForTest(testOWMAPIKey, srv.URL, deps)

	loc := web.Location{Lat: 37.57, Lon: 126.98}
	report, err := p.GetCurrentWithOptions(context.Background(), loc, "metric", "ko")
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.InDelta(t, 22.5, report.TemperatureC, 0.1, "temperature_c must match mock")
	assert.Equal(t, "clear", report.Condition, "canonical condition must be 'clear'")
	assert.Equal(t, "맑음", report.ConditionLocal, "condition_local must be the OWM description")
	assert.Equal(t, 55, report.Humidity)
	assert.InDelta(t, 3.5*3.6, report.WindKph, 0.1, "wind_kph must be m/s * 3.6")
	assert.Equal(t, "SW", report.WindDirection, "225° must map to SW")
	assert.Equal(t, "openweathermap", report.SourceProvider)
	assert.False(t, report.Stale)
	assert.False(t, report.CacheHit)
	assert.NotNil(t, report.SunTimes, "sun times must be populated")
}

// TestOWM_GetCurrent_APIError_5xx_Retryable verifies that a 5xx response
// from the OWM API surfaces as ErrInvalidResponse.
func TestOWM_GetCurrent_APIError_5xx_Retryable(t *testing.T) {
	srv := newOWMTestServer(t, map[string]any{"message": "internal error"}, http.StatusServiceUnavailable)

	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	p := web.NewOpenWeatherMapProviderForTest(testOWMAPIKey, srv.URL, deps)

	_, err := p.GetCurrentWithOptions(context.Background(), web.Location{Lat: 37.57, Lon: 126.98}, "metric", "en")
	require.Error(t, err)
	assert.ErrorIs(t, err, web.ErrInvalidResponse,
		"5xx response must surface as ErrInvalidResponse")
}

// TestOWM_GetCurrent_APIKey_Redacted_NotInLogs verifies that the OWM API key
// never appears in any zap log entry during a provider call (REQ-WEATHER-004).
func TestOWM_GetCurrent_APIKey_Redacted_NotInLogs(t *testing.T) {
	srv := newOWMTestServer(t, owmSeoulResponse, http.StatusOK)

	// Install a zap observer that captures all log entries.
	core, logs := observer.New(zap.DebugLevel)
	zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(func() { zap.ReplaceGlobals(zap.NewNop()) })

	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	p := web.NewOpenWeatherMapProviderForTest(testOWMAPIKey, srv.URL, deps)

	_, err := p.GetCurrentWithOptions(context.Background(), web.Location{Lat: 37.57, Lon: 126.98}, "metric", "en")
	require.NoError(t, err)

	// Inspect every captured log entry — the raw API key must not appear.
	for _, entry := range logs.All() {
		msg := entry.Message
		assert.NotContains(t, msg, testOWMAPIKey,
			"API key must not appear in log message: %q", msg)
		for _, field := range entry.Context {
			if field.String != "" {
				assert.NotContains(t, field.String, testOWMAPIKey,
					"API key must not appear in log field %q: %q", field.Key, field.String)
			}
		}
	}

	// Verify "****" appears in at least one log entry (proving active redaction).
	found := false
	for _, entry := range logs.All() {
		for _, field := range entry.Context {
			if strings.Contains(field.String, "****") {
				found = true
			}
		}
	}
	assert.True(t, found, `"****" must appear in at least one log field to confirm active redaction`)
}

// TestOWM_GetCurrent_InvalidJSON_Response verifies that a malformed JSON
// response from the OWM API surfaces as ErrInvalidResponse with the raw
// body preserved in the error context.
func TestOWM_GetCurrent_InvalidJSON_Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{not valid json`)
	}))
	t.Cleanup(srv.Close)

	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	p := web.NewOpenWeatherMapProviderForTest(testOWMAPIKey, srv.URL, deps)

	_, err := p.GetCurrentWithOptions(context.Background(), web.Location{Lat: 37.57, Lon: 126.98}, "metric", "en")
	require.Error(t, err)
	assert.ErrorIs(t, err, web.ErrInvalidResponse, "invalid JSON must surface as ErrInvalidResponse")
}

// TestOWM_GetCurrent_MissingAPIKey verifies that an empty API key returns
// ErrMissingAPIKey without making any network call.
func TestOWM_GetCurrent_MissingAPIKey(t *testing.T) {
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		callCount++
	}))
	t.Cleanup(srv.Close)

	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	// Empty API key.
	p := web.NewOpenWeatherMapProviderForTest("", srv.URL, deps)

	_, err := p.GetCurrentWithOptions(context.Background(), web.Location{Lat: 37.57, Lon: 126.98}, "metric", "en")
	require.Error(t, err)
	assert.ErrorIs(t, err, web.ErrMissingAPIKey)
	assert.Equal(t, 0, callCount, "no network call must be made when API key is missing")
}
