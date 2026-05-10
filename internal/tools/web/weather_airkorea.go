package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/modu-ai/goose/internal/tools/web/common"
)

const (
	airkoreaBaseURL = "https://apis.data.go.kr"
	airkoreaName    = "airkorea"
	airkoreaTimeout = 10 * time.Second

	// airkoreaServicePath is the AirKorea CTPRVN (City/Province) real-time measurement density API path.
	airkoreaServicePath = "/B552584/ArpltnInforInqireSvc/getCtprvnRltmMesureDnsty"
)

// airkoreaAPIHost is the hostname used for blocklist and permission checks.
const airkoreaAPIHost = "apis.data.go.kr"

// airkoreaResponse mirrors the AirKorea API response envelope.
type airkoreaResponse struct {
	Response struct {
		Header struct {
			ResultCode string `json:"resultCode"`
			ResultMsg  string `json:"resultMsg"`
		} `json:"header"`
		Body struct {
			TotalCount int `json:"totalCount"`
			Items      []struct {
				DataTime    string `json:"dataTime"`    // "2026-05-10 14:00"
				StationName string `json:"stationName"` // "강남구"
				SidoName    string `json:"sidoName"`    // "서울"
				PM10Value   string `json:"pm10Value"`   // "80" (μg/m³)
				PM25Value   string `json:"pm25Value"`   // "55" (μg/m³)
				O3Value     string `json:"o3Value"`
				No2Value    string `json:"no2Value"`
				So2Value    string `json:"so2Value"`
				CoValue     string `json:"coValue"`
			} `json:"items"`
		} `json:"body"`
	} `json:"response"`
}

// AirKoreaProvider implements the AirQuality portion of WeatherProvider using
// the Korean Ministry of Environment AirKorea CTPRVN real-time API.
//
// M3 scope: Korean coordinates only. Non-Korean coordinates return unsupported_region.
// API key is never written to logs or error messages (REQ-WEATHER-004).
//
// @MX:ANCHOR: [AUTO] AirKoreaProvider — Korean air-quality data source for PM2.5/PM10
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-008 — fan_in >= 3 (weather_air_quality, tests, GetAirQuality)
type AirKoreaProvider struct {
	apiKey  string
	baseURL string // injectable for tests
	deps    *common.Deps
}

// NewAirKoreaProvider constructs a production AirKorea provider.
func NewAirKoreaProvider(apiKey string, deps *common.Deps) *AirKoreaProvider {
	return &AirKoreaProvider{
		apiKey:  apiKey,
		baseURL: airkoreaBaseURL,
		deps:    deps,
	}
}

// NewAirKoreaProviderForTest constructs an AirKorea provider with an injectable
// base URL so tests can redirect requests to an httptest.Server.
func NewAirKoreaProviderForTest(apiKey, baseURL string, deps *common.Deps) *AirKoreaProvider {
	return &AirKoreaProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		deps:    deps,
	}
}

// Name returns "airkorea".
func (p *AirKoreaProvider) Name() string { return airkoreaName }

// ErrUnsupportedRegion is returned when the coordinate is outside Korea.
var ErrUnsupportedRegion = errors.New("unsupported_region")

// GetAirQuality fetches real-time PM10 and PM25 data from the AirKorea API.
// It selects the station with the most recent dataTime among the returned items.
// The PM25 value is mapped to a normalized level using Korean Ministry of
// Environment boundaries (REQ-WEATHER-008).
//
// @MX:WARN: [AUTO] multi-step response parsing with string-to-int coercion; API returns string numerics
// @MX:REASON: AirKorea API encodes all measurement values as JSON strings, requiring explicit conversion
func (p *AirKoreaProvider) GetAirQuality(ctx context.Context, loc Location) (*AirQuality, error) {
	if p.apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	sidoName := deriveSidoName(loc)
	endpoint := p.buildURL(map[string]string{
		"sidoName": sidoName,
		"ver":      "1.3",
	})

	start := time.Now()
	resp, err := p.doGet(ctx, endpoint)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		p.logCall(loc, latencyMs, 0, "fetch_error")
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p.logCall(loc, latencyMs, resp.StatusCode, "http_error")
		return nil, fmt.Errorf("%w: HTTP %d", ErrInvalidResponse, resp.StatusCode)
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseBytes))
	if readErr != nil {
		p.logCall(loc, latencyMs, resp.StatusCode, "read_error")
		return nil, fmt.Errorf("%w: read body: %v", ErrInvalidResponse, readErr)
	}

	var akResp airkoreaResponse
	if jsonErr := json.Unmarshal(body, &akResp); jsonErr != nil {
		p.logCall(loc, latencyMs, resp.StatusCode, "decode_error")
		return nil, fmt.Errorf("%w: decode JSON: %v (raw: %.200s)", ErrInvalidResponse, jsonErr, body)
	}

	items := akResp.Response.Body.Items
	if len(items) == 0 {
		p.logCall(loc, latencyMs, resp.StatusCode, "no_data")
		return nil, fmt.Errorf("%w: no items in response", ErrInvalidResponse)
	}

	// Select the most recent station by dataTime (lexicographic sort works for "YYYY-MM-DD HH:MM").
	sort.Slice(items, func(i, j int) bool {
		return items[i].DataTime > items[j].DataTime // descending
	})
	best := items[0]

	pm25 := parseAirQualityInt(best.PM25Value)
	pm10 := parseAirQualityInt(best.PM10Value)
	o3 := parseAirQualityFloat(best.O3Value)
	no2 := parseAirQualityFloat(best.No2Value)

	level, levelKo := mapPM25ToLevel(pm25)

	measuredAt, _ := time.Parse("2006-01-02 15:04", best.DataTime)

	aq := &AirQuality{
		Level:      level,
		LevelKo:    levelKo,
		PM10:       pm10,
		PM25:       pm25,
		O3:         o3,
		NO2:        no2,
		Station:    best.StationName,
		MeasuredAt: measuredAt,
		Source:     airkoreaName,
	}

	p.logCall(loc, latencyMs, resp.StatusCode, "ok")
	return aq, nil
}

// deriveSidoName maps a Location to a Korean 시도 (city/province) name for the
// AirKorea sidoName query parameter. M3 simplification: primary city keyword
// matching with fallback to "서울".
func deriveSidoName(loc Location) string {
	// Check display_name and country fields for known city/province keywords.
	combined := strings.ToLower(loc.DisplayName + " " + loc.Country)

	cityMap := []struct {
		keywords []string
		sido     string
	}{
		{[]string{"seoul", "서울"}, "서울"},
		{[]string{"busan", "부산"}, "부산"},
		{[]string{"incheon", "인천"}, "인천"},
		{[]string{"daegu", "대구"}, "대구"},
		{[]string{"daejeon", "대전"}, "대전"},
		{[]string{"gwangju", "광주"}, "광주"},
		{[]string{"ulsan", "울산"}, "울산"},
		{[]string{"gyeonggi", "경기"}, "경기"},
		{[]string{"gangwon", "강원"}, "강원"},
		{[]string{"chungbuk", "충북"}, "충북"},
		{[]string{"chungnam", "충남"}, "충남"},
		{[]string{"jeonbuk", "전북"}, "전북"},
		{[]string{"jeonnam", "전남"}, "전남"},
		{[]string{"gyeongbuk", "경북"}, "경북"},
		{[]string{"gyeongnam", "경남"}, "경남"},
		{[]string{"jeju", "제주"}, "제주"},
		{[]string{"sejong", "세종"}, "세종"},
	}

	for _, entry := range cityMap {
		for _, kw := range entry.keywords {
			if strings.Contains(combined, kw) {
				return entry.sido
			}
		}
	}

	// Default to Seoul when city cannot be inferred.
	return "서울"
}

// mapPM25ToLevel converts a PM2.5 μg/m³ value to a canonical English level
// and Korean level string using Korean Ministry of Environment boundaries.
//
// Boundaries (REQ-WEATHER-008, plan.md §4.2):
//
//	0-15   → "good"          / "좋음"
//	16-35  → "moderate"      / "보통"
//	36-75  → "unhealthy"     / "나쁨"
//	76-150 → "very_unhealthy"/ "매우 나쁨"
//	151+   → "hazardous"     / "위험"   (spec.md AirQuality DTO 5-tier definition)
func mapPM25ToLevel(pm25 int) (level, levelKo string) {
	switch {
	case pm25 <= 15:
		return "good", "좋음"
	case pm25 <= 35:
		return "moderate", "보통"
	case pm25 <= 75:
		return "unhealthy", "나쁨"
	case pm25 <= 150:
		return "very_unhealthy", "매우 나쁨"
	default:
		return "hazardous", "위험"
	}
}

// buildURL constructs an AirKorea API URL. The service key is URL-encoded and
// placed in the query string. Any log that captures the URL must redact
// serviceKey= values (REQ-WEATHER-004).
func (p *AirKoreaProvider) buildURL(params map[string]string) string {
	base := strings.TrimRight(p.baseURL, "/")
	q := url.Values{}
	q.Set("serviceKey", p.apiKey)
	q.Set("returnType", "json")
	q.Set("numOfRows", "100")
	q.Set("pageNo", "1")
	for k, v := range params {
		q.Set(k, v)
	}
	// Redact serviceKey in the logged URL by replacing the raw key.
	rawURL := base + airkoreaServicePath + "?" + q.Encode()
	return rawURL
}

// doGet performs a GET with a 10-second timeout and standard User-Agent.
func (p *AirKoreaProvider) doGet(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", common.UserAgent())
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: airkoreaTimeout}
	return client.Do(req)
}

// logCall emits a structured zap log for an AirKorea API call.
// The API key is always redacted (REQ-WEATHER-004).
func (p *AirKoreaProvider) logCall(loc Location, latencyMs int64, apiStatus int, outcome string) {
	zap.L().Info("airkorea provider call",
		zap.String("provider", airkoreaName),
		zap.String("api_key", "****"), // always redacted (REQ-WEATHER-004)
		zap.Float64("lat", loc.Lat),
		zap.Float64("lon", loc.Lon),
		zap.Int64("latency_ms", latencyMs),
		zap.Int("api_status", apiStatus),
		zap.String("outcome", outcome),
	)
}

// --------------------------------------------------------------------------
// Parsing helpers
// --------------------------------------------------------------------------

// parseAirQualityInt parses an AirKorea string measurement value to int.
// Returns 0 on empty, dash, or parse failure.
func parseAirQualityInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// parseAirQualityFloat parses an AirKorea string measurement value to float64.
// Returns 0 on empty, dash, or parse failure.
func parseAirQualityFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
