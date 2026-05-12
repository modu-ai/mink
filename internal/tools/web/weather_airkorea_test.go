package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

// airkoreaSeoulResponse is a minimal AirKorea CTPRVN real-time response
// representing a Seoul station reporting PM2.5=55 and PM10=80.
const airkoreaSeoulResponse = `{
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
          "pm25Value": "55",
          "o3Value": "0.025",
          "no2Value": "0.030",
          "so2Value": "0.005",
          "coValue": "0.5"
        }
      ]
    }
  }
}`

// TestAirKorea_GetAirQuality_Seoul_PM25_55 verifies that the provider parses
// the AirKorea response correctly and returns PM25=55, PM10=80.
func TestAirKorea_GetAirQuality_Seoul_PM25_55(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(airkoreaSeoulResponse))
	}))
	defer srv.Close()

	deps := &common.Deps{}
	provider := web.NewAirKoreaProviderForTest("test-key", srv.URL, deps)

	loc := web.Location{Lat: 37.57, Lon: 126.98, DisplayName: "Seoul,KR", Country: "KR"}
	aq, err := provider.GetAirQuality(context.Background(), loc)
	require.NoError(t, err)
	require.NotNil(t, aq)

	assert.Equal(t, 55, aq.PM25)
	assert.Equal(t, 80, aq.PM10)
	assert.Equal(t, "unhealthy", aq.Level, "PM25=55 must map to unhealthy (36-75 boundary)")
	assert.Equal(t, "나쁨", aq.LevelKo)
	assert.Equal(t, "강남구", aq.Station)
	assert.Equal(t, "airkorea", aq.Source)
}

// TestAirKorea_MissingAPIKey_ReturnsError verifies that an empty API key
// produces ErrMissingAPIKey without making any outbound request.
func TestAirKorea_MissingAPIKey_ReturnsError(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deps := &common.Deps{}
	provider := web.NewAirKoreaProviderForTest("", srv.URL, deps) // empty key

	loc := web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}
	_, err := provider.GetAirQuality(context.Background(), loc)

	require.Error(t, err)
	assert.ErrorIs(t, err, web.ErrMissingAPIKey, "empty API key must return ErrMissingAPIKey")
	assert.False(t, called, "no outbound request must be made when API key is missing")
}

// TestAirKorea_APIError_5xx_Retryable verifies that a 5xx response from the
// AirKorea API surfaces as an error (ErrInvalidResponse wrapping).
func TestAirKorea_APIError_5xx_Retryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	deps := &common.Deps{}
	provider := web.NewAirKoreaProviderForTest("test-key", srv.URL, deps)

	loc := web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}
	_, err := provider.GetAirQuality(context.Background(), loc)

	require.Error(t, err, "5xx must return an error")
}

// TestAirKorea_APIKey_Redacted_NotInLogs verifies that the raw API key never
// appears in zap log output (REQ-WEATHER-004).
func TestAirKorea_APIKey_Redacted_NotInLogs(t *testing.T) {
	const secretKey = "secret-airkorea-key-xyz987"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(airkoreaSeoulResponse))
	}))
	defer srv.Close()

	// Capture all zap log entries.
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	// Replace global logger for the duration of the test.
	restore := zap.ReplaceGlobals(logger)
	defer restore()

	deps := &common.Deps{}
	provider := web.NewAirKoreaProviderForTest(secretKey, srv.URL, deps)
	loc := web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}
	_, _ = provider.GetAirQuality(context.Background(), loc)

	// Verify no log entry contains the raw secret key.
	for _, entry := range logs.All() {
		assert.NotContains(t, entry.Message, secretKey,
			"log message must not contain raw API key")
		for _, field := range entry.Context {
			if field.Type == zapcore.StringType {
				assert.NotContains(t, field.String, secretKey,
					"log field %q must not contain raw API key", field.Key)
			}
		}
	}
}

// TestAirKorea_InvalidJSON_Response verifies that a malformed JSON response
// returns an error without panicking.
func TestAirKorea_InvalidJSON_Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response": {"body": {INVALID JSON}}`))
	}))
	defer srv.Close()

	deps := &common.Deps{}
	provider := web.NewAirKoreaProviderForTest("test-key", srv.URL, deps)
	loc := web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}

	_, err := provider.GetAirQuality(context.Background(), loc)
	require.Error(t, err, "malformed JSON must return error")
}

// TestAirKorea_ParsesMostRecentStation verifies that when multiple stations
// are returned with different dataTime values, the provider selects the most
// recent station.
func TestAirKorea_ParsesMostRecentStation(t *testing.T) {
	multiStationResponse := `{
  "response": {
    "header": {"resultCode": "00", "resultMsg": "NORMAL_SERVICE"},
    "body": {
      "totalCount": 3,
      "items": [
        {
          "dataTime": "2026-05-10 12:00",
          "stationName": "종로구",
          "sidoName": "서울",
          "pm10Value": "60",
          "pm25Value": "30",
          "o3Value": "0.020",
          "no2Value": "0.025"
        },
        {
          "dataTime": "2026-05-10 14:00",
          "stationName": "강남구",
          "sidoName": "서울",
          "pm10Value": "80",
          "pm25Value": "55",
          "o3Value": "0.025",
          "no2Value": "0.030"
        },
        {
          "dataTime": "2026-05-10 13:00",
          "stationName": "마포구",
          "sidoName": "서울",
          "pm10Value": "70",
          "pm25Value": "40",
          "o3Value": "0.022",
          "no2Value": "0.028"
        }
      ]
    }
  }
}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(multiStationResponse))
	}))
	defer srv.Close()

	deps := &common.Deps{}
	provider := web.NewAirKoreaProviderForTest("test-key", srv.URL, deps)
	loc := web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}

	aq, err := provider.GetAirQuality(context.Background(), loc)
	require.NoError(t, err)
	require.NotNil(t, aq)

	// Most recent dataTime is "2026-05-10 14:00" → 강남구 (PM25=55, PM10=80).
	assert.Equal(t, "강남구", aq.Station, "must select the station with the latest dataTime")
	assert.Equal(t, 55, aq.PM25)
	assert.Equal(t, 80, aq.PM10)
	assert.Equal(t, time.Date(2026, 5, 10, 14, 0, 0, 0, time.UTC), aq.MeasuredAt)
}

// TestAirKorea_PM25LevelBoundaries exercises PM25 boundary mapping indirectly
// via AirKoreaProvider by setting pm25Value in the mock response and checking
// the returned AirQuality.Level and LevelKo values.
func TestAirKorea_PM25LevelBoundaries(t *testing.T) {
	cases := []struct {
		pm25      string
		wantLevel string
		wantKo    string
	}{
		{"15", "good", "좋음"},
		{"16", "moderate", "보통"},
		{"35", "moderate", "보통"},
		{"36", "unhealthy", "나쁨"},
		{"75", "unhealthy", "나쁨"},
		{"76", "very_unhealthy", "매우 나쁨"},
		{"150", "very_unhealthy", "매우 나쁨"},
		{"151", "hazardous", "위험"},
		{"200", "hazardous", "위험"},
	}

	for _, tc := range cases {
		t.Run("pm25="+tc.pm25, func(t *testing.T) {
			responseBody, _ := json.Marshal(map[string]any{
				"response": map[string]any{
					"header": map[string]any{"resultCode": "00", "resultMsg": "NORMAL_SERVICE"},
					"body": map[string]any{
						"totalCount": 1,
						"items": []map[string]any{
							{
								"dataTime":    "2026-05-10 14:00",
								"stationName": "강남구",
								"sidoName":    "서울",
								"pm10Value":   "80",
								"pm25Value":   tc.pm25,
								"o3Value":     "0.025",
								"no2Value":    "0.030",
							},
						},
					},
				},
			})

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(responseBody)
			}))
			defer srv.Close()

			deps := &common.Deps{}
			provider := web.NewAirKoreaProviderForTest("test-key", srv.URL, deps)
			loc := web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"}

			aq, err := provider.GetAirQuality(context.Background(), loc)
			require.NoError(t, err)
			require.NotNil(t, aq)
			assert.Equal(t, tc.wantLevel, aq.Level, "pm25=%s level", tc.pm25)
			assert.Equal(t, tc.wantKo, aq.LevelKo, "pm25=%s level_ko", tc.pm25)
		})
	}
}
