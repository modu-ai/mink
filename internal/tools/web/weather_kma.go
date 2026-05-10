package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	kmaBaseURL = "https://apis.data.go.kr"
	kmaName    = "kma"
	kmaTimeout = 10 * time.Second

	// KMA API service path prefix.
	kmaServicePath = "/1360000/VilageFcstInfoService_2.0"
)

// DFS_XY_CONV constants — Lambert Conformal Conic projection parameters
// defined by the Korean Meteorological Administration.
const (
	kmaREGRID = 6371.00877 // earth radius (km)
	kmaGRID   = 5.0        // grid interval (km)
	kmaSLAT1  = 30.0       // standard latitude 1 (degrees)
	kmaSLAT2  = 60.0       // standard latitude 2 (degrees)
	kmaOLON   = 126.0      // standard longitude (degrees)
	kmaOLAT   = 38.0       // standard latitude origin (degrees)
	kmaXO     = 43         // origin X on grid
	kmaYO     = 136        // origin Y on grid
)

// kmaAPIHost is the KMA API hostname used for blocklist and permission checks.
const kmaAPIHost = "apis.data.go.kr"

// kmaItemsResponse mirrors the common KMA API response envelope.
type kmaItemsResponse struct {
	Response struct {
		Header struct {
			ResultCode string `json:"resultCode"`
			ResultMsg  string `json:"resultMsg"`
		} `json:"header"`
		Body struct {
			Items struct {
				Item []kmaItem `json:"item"`
			} `json:"items"`
		} `json:"body"`
	} `json:"response"`
}

// kmaItem represents one observation or forecast element from the KMA API.
type kmaItem struct {
	Category  string `json:"category"`
	ObsrValue string `json:"obsrValue,omitempty"` // UltraSrtNcst (실황)
	FcstDate  string `json:"fcstDate,omitempty"`  // VilageFcst (예보) — YYYYMMDD
	FcstTime  string `json:"fcstTime,omitempty"`  // VilageFcst (예보) — HHMM
	FcstValue string `json:"fcstValue,omitempty"` // VilageFcst (예보)
}

// KMAProvider implements WeatherProvider using the Korean Meteorological
// Administration OpenAPI (data.go.kr). It supports:
//   - UltraSrtNcst (초단기실황): GetCurrent
//   - VilageFcst (단기예보):    GetForecast (up to 3 days)
//
// The API key is never written to logs or error messages (REQ-WEATHER-004).
//
// @MX:ANCHOR: [AUTO] KMAProvider — Korean weather data source implementation
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-001 — fan_in >= 3 (route, weather_forecast, tests)
type KMAProvider struct {
	apiKey  string
	baseURL string // injectable for tests
	deps    *common.Deps
}

// NewKMAProvider constructs a production KMA provider.
func NewKMAProvider(apiKey string, deps *common.Deps) *KMAProvider {
	return &KMAProvider{
		apiKey:  apiKey,
		baseURL: kmaBaseURL,
		deps:    deps,
	}
}

// NewKMAProviderForTest constructs a KMA provider with an injectable base URL
// so tests can redirect requests to an httptest.Server.
func NewKMAProviderForTest(apiKey, baseURL string, deps *common.Deps) *KMAProvider {
	return &KMAProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		deps:    deps,
	}
}

// Name returns "kma".
func (p *KMAProvider) Name() string { return kmaName }

// LatLonToGrid converts WGS-84 geographic coordinates to KMA Lambert Conformal
// Conic grid coordinates (nx, ny). The algorithm is the official DFS_XY_CONV
// implementation published by the Korean Meteorological Administration.
//
// Goldenfile (research.md §3):
//
//	Seoul  (37.5665, 126.9780) → (60, 127)
//	Busan  (35.1796, 129.0756) → (98, 76)
//	Jeju   (33.4996, 126.5312) → (52, 38)
//	Daejeon(36.3504, 127.3845) → (67, 100)
//	Gangneung(37.7519,128.8761)→ (92, 131)
func (p *KMAProvider) LatLonToGrid(lat, lon float64) (nx, ny int) {
	return LatLonToGrid(lat, lon)
}

// LatLonToGrid is a package-level function so tests and weather_route.go can
// call it without holding a KMAProvider instance.
func LatLonToGrid(lat, lon float64) (nx, ny int) {
	degrad := math.Pi / 180.0
	re := kmaREGRID / kmaGRID

	slat1 := kmaSLAT1 * degrad
	slat2 := kmaSLAT2 * degrad
	olon := kmaOLON * degrad
	olat := kmaOLAT * degrad

	sn := math.Tan(math.Pi*0.25+slat2*0.5) / math.Tan(math.Pi*0.25+slat1*0.5)
	sn = math.Log(math.Cos(slat1)/math.Cos(slat2)) / math.Log(sn)
	sf := math.Tan(math.Pi*0.25 + slat1*0.5)
	sf = math.Pow(sf, sn) * math.Cos(slat1) / sn
	ro := math.Tan(math.Pi*0.25 + olat*0.5)
	ro = re * sf / math.Pow(ro, sn)

	ra := math.Tan(math.Pi*0.25 + lat*degrad*0.5)
	ra = re * sf / math.Pow(ra, sn)

	theta := lon*degrad - olon
	if theta > math.Pi {
		theta -= 2.0 * math.Pi
	}
	if theta < -math.Pi {
		theta += 2.0 * math.Pi
	}
	theta *= sn

	nx = int(ra*math.Sin(theta) + float64(kmaXO) + 0.5)
	ny = int(ro - ra*math.Cos(theta) + float64(kmaYO) + 0.5)
	return nx, ny
}

// GetCurrent fetches the current weather at loc using the KMA UltraSrtNcst
// (초단기실황) endpoint. The response is mapped to the canonical WeatherReport.
func (p *KMAProvider) GetCurrent(ctx context.Context, loc Location) (*WeatherReport, error) {
	if p.apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	nx, ny := p.LatLonToGrid(loc.Lat, loc.Lon)
	now := time.Now().UTC().Add(9 * time.Hour) // KST = UTC+9

	// KMA uses the top of the current hour for base time.
	baseDate := now.Format("20060102")
	baseTime := fmt.Sprintf("%02d00", now.Hour())

	endpoint := p.buildURL("/getUltraSrtNcst", map[string]string{
		"base_date": baseDate,
		"base_time": baseTime,
		"nx":        strconv.Itoa(nx),
		"ny":        strconv.Itoa(ny),
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

	var kmaResp kmaItemsResponse
	if jsonErr := json.Unmarshal(body, &kmaResp); jsonErr != nil {
		p.logCall(loc, latencyMs, resp.StatusCode, "decode_error")
		return nil, fmt.Errorf("%w: decode JSON: %v (raw: %.200s)", ErrInvalidResponse, jsonErr, body)
	}

	report := p.ncstToWeatherReport(kmaResp.Response.Body.Items.Item, loc)
	p.logCall(loc, latencyMs, resp.StatusCode, "ok")
	return report, nil
}

// GetForecast fetches a multi-day forecast using the KMA VilageFcst
// (단기예보) endpoint. days is clamped to [1, 3] because the short-range
// forecast only provides 3 days of data.
//
// @MX:WARN: [AUTO] days clamping and hourly-to-daily aggregation; complexity >= 10
// @MX:REASON: KMA VilageFcst returns 3-hourly items; we must group by fcstDate and aggregate high/low
func (p *KMAProvider) GetForecast(ctx context.Context, loc Location, days int) ([]WeatherForecastDay, error) {
	if p.apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	// KMA single-range forecast supports up to 3 days.
	warned := days > 3
	if days > 3 {
		days = 3
	}
	if days < 1 {
		days = 1
	}

	if warned {
		p.logWarn("GetForecast: days clamped to 3 (KMA VilageFcst limit)", loc)
	}

	nx, ny := p.LatLonToGrid(loc.Lat, loc.Lon)
	now := time.Now().UTC().Add(9 * time.Hour) // KST

	// KMA short-range forecast base times: 0200, 0500, 0800, 1100, 1400, 1700, 2000, 2300
	// Use the most recent available base time.
	baseDate, baseTime := kmaFcstBaseTime(now)

	endpoint := p.buildURL("/getVilageFcst", map[string]string{
		"base_date": baseDate,
		"base_time": baseTime,
		"nx":        strconv.Itoa(nx),
		"ny":        strconv.Itoa(ny),
		"numOfRows": "1000",
		"pageNo":    "1",
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

	var kmaResp kmaItemsResponse
	if jsonErr := json.Unmarshal(body, &kmaResp); jsonErr != nil {
		p.logCall(loc, latencyMs, resp.StatusCode, "decode_error")
		return nil, fmt.Errorf("%w: decode JSON: %v (raw: %.200s)", ErrInvalidResponse, jsonErr, body)
	}

	result := p.fcstToDailyForecast(kmaResp.Response.Body.Items.Item, days)
	p.logCall(loc, latencyMs, resp.StatusCode, "ok")
	return result, nil
}

// GetAirQuality is a stub for M3; returns nil, nil per M2 spec.
func (p *KMAProvider) GetAirQuality(_ context.Context, _ Location) (*AirQuality, error) {
	return nil, nil
}

// GetSunTimes is not implemented for M2; returns nil, nil.
func (p *KMAProvider) GetSunTimes(_ context.Context, _ Location, _ time.Time) (*SunTimes, error) {
	return nil, nil
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// buildURL constructs a KMA API URL with common parameters.
// The service key is URL-encoded and always placed last to simplify redaction
// in any log output that captures the URL.
func (p *KMAProvider) buildURL(action string, params map[string]string) string {
	base := strings.TrimRight(p.baseURL, "/")
	q := url.Values{}
	q.Set("serviceKey", p.apiKey)
	q.Set("dataType", "JSON")
	q.Set("numOfRows", "100")
	q.Set("pageNo", "1")
	for k, v := range params {
		q.Set(k, v)
	}
	return fmt.Sprintf("%s%s%s?%s", base, kmaServicePath, action, q.Encode())
}

// doGet performs a GET request with a 10-second timeout and a standard User-Agent.
func (p *KMAProvider) doGet(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", common.UserAgent())
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: kmaTimeout}
	return client.Do(req)
}

// logCall emits a structured zap log entry for the KMA call.
// The API key is always redacted (REQ-WEATHER-004).
func (p *KMAProvider) logCall(loc Location, latencyMs int64, apiStatus int, outcome string) {
	log := zap.L()
	log.Info("kma provider call",
		zap.String("provider", kmaName),
		zap.String("api_key", "****"), // always redacted (REQ-WEATHER-004)
		zap.Float64("lat", loc.Lat),
		zap.Float64("lon", loc.Lon),
		zap.Int64("latency_ms", latencyMs),
		zap.Int("api_status", apiStatus),
		zap.String("outcome", outcome),
	)
}

// logWarn emits a warning log for non-fatal conditions (e.g. days clamping).
func (p *KMAProvider) logWarn(msg string, loc Location) {
	log := zap.L()
	log.Warn(msg,
		zap.String("provider", kmaName),
		zap.String("api_key", "****"),
		zap.Float64("lat", loc.Lat),
		zap.Float64("lon", loc.Lon),
	)
}

// ncstToWeatherReport maps KMA UltraSrtNcst items to a WeatherReport.
// Category codes: T1H=temp, REH=humidity, WSD=wind speed, VEC=wind direction,
// PTY=precip type (0=none,1=rain,2=rain/snow,3=snow,4=shower),
// RN1=precipitation (mm/h).
func (p *KMAProvider) ncstToWeatherReport(items []kmaItem, loc Location) *WeatherReport {
	values := make(map[string]string, len(items))
	for _, it := range items {
		values[it.Category] = it.ObsrValue
	}

	tempC := parseKMAFloat(values["T1H"])
	humidity := parseKMAInt(values["REH"])
	windMs := parseKMAFloat(values["WSD"])
	windDir := parseKMAInt(values["VEC"])
	ptyCode := parseKMAInt(values["PTY"])
	rainMm := parseKMAFloat(values["RN1"])

	cond, condLocal := ptyToCondition(ptyCode)

	return &WeatherReport{
		Location:       loc,
		Timestamp:      time.Now().UTC(),
		TemperatureC:   roundFloat(tempC, 1),
		FeelsLikeC:     roundFloat(tempC, 1), // KMA 실황 does not provide feels_like
		Condition:      cond,
		ConditionLocal: condLocal,
		Humidity:       humidity,
		WindKph:        roundFloat(windMs*3.6, 1), // m/s → km/h
		WindDirection:  degreeToCompass(windDir),
		PrecipMm:       roundFloat(rainMm, 1),
		SourceProvider: kmaName,
	}
}

// fcstToDailyForecast aggregates KMA VilageFcst 3-hourly items into daily
// WeatherForecastDay slices. It groups items by fcstDate and computes the
// daily high/low temperature and most-common condition (by occurrence count).
func (p *KMAProvider) fcstToDailyForecast(items []kmaItem, days int) []WeatherForecastDay {
	// Group items by date.
	type dayData struct {
		temps     []float64
		ptyCodes  []int
		skyCodes  []int
		precipPct int
		precipMm  float64
		humidity  int
		windKph   float64
	}

	dayMap := make(map[string]*dayData)
	var dates []string

	for _, it := range items {
		date := it.FcstDate
		if date == "" {
			continue
		}
		if _, exists := dayMap[date]; !exists {
			dayMap[date] = &dayData{}
			dates = append(dates, date)
		}
		d := dayMap[date]
		val := it.FcstValue

		switch it.Category {
		case "TMP": // 3-hourly temperature (°C)
			d.temps = append(d.temps, parseKMAFloat(val))
		case "TMX": // daily max
			d.temps = append(d.temps, parseKMAFloat(val))
		case "TMN": // daily min
			d.temps = append(d.temps, parseKMAFloat(val))
		case "PTY": // precip type
			d.ptyCodes = append(d.ptyCodes, parseKMAInt(val))
		case "SKY": // sky condition 1=clear 3=cloudy 4=overcast
			d.skyCodes = append(d.skyCodes, parseKMAInt(val))
		case "POP": // precip probability (%)
			if v := parseKMAInt(val); v > d.precipPct {
				d.precipPct = v
			}
		case "PCP": // precipitation (mm) — skip "강수없음"
			if val != "강수없음" && val != "" {
				d.precipMm += parseKMAFloat(val)
			}
		case "REH":
			d.humidity = parseKMAInt(val)
		case "WSD":
			if kph := parseKMAFloat(val) * 3.6; kph > d.windKph {
				d.windKph = kph
			}
		}
	}

	// Sort dates to ensure chronological order.
	sort.Strings(dates)
	// Remove duplicates (dates can appear multiple times in the raw items).
	seen := make(map[string]bool)
	var uniqueDates []string
	for _, d := range dates {
		if !seen[d] {
			seen[d] = true
			uniqueDates = append(uniqueDates, d)
		}
	}

	result := make([]WeatherForecastDay, 0, days)
	for i, date := range uniqueDates {
		if i >= days {
			break
		}
		d := dayMap[date]

		// Format date as YYYY-MM-DD.
		fmtDate := date
		if len(date) == 8 {
			fmtDate = date[:4] + "-" + date[4:6] + "-" + date[6:]
		}

		high := math.Inf(-1)
		low := math.Inf(1)
		for _, t := range d.temps {
			if t > high {
				high = t
			}
			if t < low {
				low = t
			}
		}
		if math.IsInf(high, -1) {
			high = 0
		}
		if math.IsInf(low, 1) {
			low = 0
		}

		cond, condLocal := skyAndPtyToCondition(d.skyCodes, d.ptyCodes)

		result = append(result, WeatherForecastDay{
			Date:           fmtDate,
			HighC:          roundFloat(high, 1),
			LowC:           roundFloat(low, 1),
			Condition:      cond,
			ConditionLocal: condLocal,
			PrecipProbPct:  d.precipPct,
			PrecipMm:       roundFloat(d.precipMm, 1),
			WindKph:        roundFloat(d.windKph, 1),
			Humidity:       d.humidity,
		})
	}
	return result
}

// --------------------------------------------------------------------------
// KMA-specific conversion helpers
// --------------------------------------------------------------------------

// ptyToCondition maps KMA PTY (강수형태) code to canonical condition strings.
// 0=없음, 1=비, 2=비/눈, 3=눈, 4=소나기
func ptyToCondition(pty int) (canonical, local string) {
	switch pty {
	case 1:
		return "rain", "비"
	case 2:
		return "rain", "비/눈"
	case 3:
		return "snow", "눈"
	case 4:
		return "rain", "소나기"
	default:
		return "clear", "맑음"
	}
}

// skyAndPtyToCondition picks the dominant condition from a day's worth of
// sky-code and pty-code observations. PTY (precipitation) wins over SKY when
// any non-zero PTY code is present.
// SKY: 1=맑음, 3=구름많음, 4=흐림
func skyAndPtyToCondition(skyCodes, ptyCodes []int) (canonical, local string) {
	// If any precipitation is present, use pty.
	for _, pty := range ptyCodes {
		if pty > 0 {
			return ptyToCondition(pty)
		}
	}

	if len(skyCodes) == 0 {
		return "clear", "맑음"
	}

	// Count occurrences of each SKY code.
	counts := make(map[int]int)
	for _, s := range skyCodes {
		counts[s]++
	}

	dominant := skyCodes[0]
	for code, cnt := range counts {
		if cnt > counts[dominant] {
			dominant = code
		}
	}

	switch dominant {
	case 1:
		return "clear", "맑음"
	case 3:
		return "cloudy", "구름많음"
	case 4:
		return "cloudy", "흐림"
	default:
		return "clear", "맑음"
	}
}

// kmaFcstBaseTime returns the most recent available VilageFcst base time
// (one of 0200, 0500, 0800, 1100, 1400, 1700, 2000, 2300 KST).
func kmaFcstBaseTime(kst time.Time) (baseDate, baseTime string) {
	hour := kst.Hour()
	baseTimes := []int{2, 5, 8, 11, 14, 17, 20, 23}

	selected := 23 // fallback: 2300 of previous day
	for _, bt := range baseTimes {
		if hour >= bt {
			selected = bt
		}
	}

	if hour < 2 {
		// Before 0200 KST: use previous day 2300.
		prev := kst.Add(-24 * time.Hour)
		return prev.Format("20060102"), "2300"
	}

	return kst.Format("20060102"), fmt.Sprintf("%02d00", selected)
}

// parseKMAFloat parses a KMA string value to float64; returns 0 on error.
func parseKMAFloat(s string) float64 {
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

// parseKMAInt parses a KMA string value to int; returns 0 on error.
func parseKMAInt(s string) int {
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
