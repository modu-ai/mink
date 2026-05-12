package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

// weatherAirQualitySchema is the JSON Schema for the weather_air_quality tool input.
// anyOf requires either a location string or explicit lat+lon.
var weatherAirQualitySchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "location": { "type": "string", "minLength": 1, "maxLength": 200 },
    "lat": { "type": "number", "minimum": -90, "maximum": 90 },
    "lon": { "type": "number", "minimum": -180, "maximum": 180 }
  },
  "anyOf": [
    { "required": ["location"] },
    { "required": ["lat", "lon"] }
  ]
}`)

// weatherAirQualityInput holds parsed tool input for weather_air_quality.
type weatherAirQualityInput struct {
	Location string  `json:"location"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`

	hasLocation bool
	hasLatLon   bool
}

// airQualityProviderFactory constructs an AirKoreaProvider from config.
// Tests inject stubs via the provider field directly.
type airQualityProviderFactory func(cfg *WeatherConfig, deps *common.Deps) *AirKoreaProvider

// webWeatherAirQuality implements the weather_air_quality tool (M3).
// It applies TOOLS-WEB-001 infrastructure gates (Blocklist, Permission,
// RateLimit, Audit, bbolt Cache, singleflight) and delegates to AirKoreaProvider.
// Only Korean coordinates/locations are supported (REQ-WEATHER-008).
//
// @MX:ANCHOR: [AUTO] webWeatherAirQuality — entry point for Korean air-quality pipeline
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-008 — fan_in >= 3 (init, tests, executor)
type webWeatherAirQuality struct {
	deps            *common.Deps
	cfg             *WeatherConfig
	provider        *AirKoreaProvider // injected by tests
	providerFactory airQualityProviderFactory
	sf              singleflight.Group
}

// productionAirQualityProviderFactory constructs an AirKoreaProvider.
func productionAirQualityProviderFactory(cfg *WeatherConfig, deps *common.Deps) *AirKoreaProvider {
	return NewAirKoreaProvider(cfg.AirKorea.APIKey, deps)
}

// Name returns the canonical tool name.
func (w *webWeatherAirQuality) Name() string { return "weather_air_quality" }

// Schema returns the JSON Schema for input validation.
func (w *webWeatherAirQuality) Schema() json.RawMessage { return weatherAirQualitySchema }

// Scope returns ScopeShared — available to all agent types.
func (w *webWeatherAirQuality) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the weather_air_quality 10-step pipeline.
//
//  1. Parse + defensive schema guard
//  2. Resolve location; validate Korean coordinate/string → unsupported_region if not Korean
//  3. Derive provider host (airkorea: apis.data.go.kr)
//  4. Blocklist gate
//  5. Permission gate (CapNet, scope=host)
//  6. Build cache key (SHA256, 30-min TTL)
//  7. Live bbolt cache check
//  8. Rate limit check
//  9. Singleflight: outbound GetAirQuality
//  10. On success: cache.Set + mapPM25ToLevel + audit ok + return
//
// @MX:WARN: [AUTO] 10-step pipeline with multiple early-return paths; cyclomatic complexity >= 12
// @MX:REASON: Each step maps to a distinct SPEC gate; mirrors weather_current/weather_forecast pipelines
func (w *webWeatherAirQuality) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := w.deps.Now()

	// Step 1: Parse input.
	in, err := parseWeatherAirQualityInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	// Step 2: Resolve Korean location.
	loc, ok := deriveKoreanLocation(in)
	if !ok {
		w.writeAudit(ctx, "", "", 0, false, "denied", "unsupported_region", start)
		return toToolResult(common.ErrResponse("unsupported_region",
			"weather_air_quality only supports Korean coordinates and locations (M3)",
			false, 0, elapsed(start))), nil
	}

	// Step 3: Derive provider host.
	providerHost := airkoreaAPIHost

	// Step 4: Blocklist gate.
	if w.deps.Blocklist != nil && w.deps.Blocklist.IsBlocked(stripPort(providerHost)) {
		w.writeAudit(ctx, airkoreaName, providerHost, 0, false, "denied", "host_blocked", start)
		return toToolResult(common.ErrResponse("host_blocked",
			fmt.Sprintf("host %q is blocked by security policy", providerHost),
			false, 0, elapsed(start))), nil
	}

	// Step 5: Permission gate (CapNet, scope=host).
	if w.deps.PermMgr != nil {
		subjectID := w.deps.SubjectID(ctx)
		req := permission.PermissionRequest{
			SubjectID:   subjectID,
			SubjectType: permission.SubjectAgent,
			Capability:  permission.CapNet,
			Scope:       providerHost,
			RequestedAt: start,
		}
		dec, checkErr := w.deps.PermMgr.Check(ctx, req)
		if checkErr != nil || !dec.Allow {
			msg := "permission denied"
			if checkErr != nil {
				msg = checkErr.Error()
			}
			w.writeAudit(ctx, airkoreaName, providerHost, 0, false, "denied", "permission_denied", start)
			return toToolResult(common.ErrResponse("permission_denied", msg, false, 0, elapsed(start))), nil
		}
	}

	// Step 6: Build cache key (lat/lon rounded to 2dp for ~1.1km resolution).
	cacheKey := airQualityCacheKey(loc.Lat, loc.Lon)

	// Step 7: Live bbolt cache check (30-min TTL).
	cache := w.openCache()
	if cache != nil {
		defer func() { _ = cache.Close() }()
		if cachedBytes, hit, cErr := cache.Get(cacheKey); cErr == nil && hit {
			var aq AirQuality
			if json.Unmarshal(cachedBytes, &aq) == nil {
				w.writeAudit(ctx, airkoreaName, providerHost, 0, true, "ok", "", start)
				meta := elapsed(start)
				meta.CacheHit = true
				okResp, _ := common.OKResponse(aq, meta)
				return toToolResult(okResp), nil
			}
		}
	}

	// Step 8: Rate limit check.
	provider := w.resolveProvider()
	if w.deps.RateTracker != nil {
		state := w.deps.RateTracker.State(airkoreaName)
		if state.RequestsMin.UsagePct() >= 100 {
			retryAfter := int(math.Ceil(state.RequestsMin.RemainingSecondsNow(w.deps.Now())))
			retryAfter = max(retryAfter, 0)
			w.writeAudit(ctx, airkoreaName, providerHost, 0, false, "denied", "ratelimit_exhausted", start)
			return toToolResult(common.ErrResponse("ratelimit_exhausted",
				fmt.Sprintf("air quality rate limit exhausted; retry after %d seconds", retryAfter),
				true, retryAfter, elapsed(start))), nil
		}
	}

	// Step 9: Singleflight: outbound API call.
	type fetchResult struct {
		aq  *AirQuality
		err error
	}

	sfResult, sfErr, _ := w.sf.Do(cacheKey, func() (any, error) {
		aq, fetchErr := provider.GetAirQuality(ctx, loc)
		return &fetchResult{aq: aq, err: fetchErr}, nil
	})

	if sfErr != nil {
		w.writeAudit(ctx, airkoreaName, providerHost, 0, false, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", sfErr.Error(), true, 0, elapsed(start))), nil
	}

	fr := sfResult.(*fetchResult)
	if fr.err != nil {
		w.writeAudit(ctx, airkoreaName, providerHost, 0, false, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", fr.err.Error(), true, 0, elapsed(start))), nil
	}

	// Step 10: Cache + audit + return.
	if cache != nil {
		if dataBytes, mErr := json.Marshal(fr.aq); mErr == nil {
			_ = cache.Set(cacheKey, dataBytes, 30*time.Minute)
		}
	}

	w.writeAudit(ctx, airkoreaName, providerHost, 0, false, "ok", "", start)
	okResp, _ := common.OKResponse(fr.aq, elapsed(start))
	return toToolResult(okResp), nil
}

// resolveProvider returns the AirKoreaProvider. Tests inject stubs via the
// provider field; production uses the factory.
func (w *webWeatherAirQuality) resolveProvider() *AirKoreaProvider {
	if w.provider != nil {
		return w.provider
	}
	if w.providerFactory != nil {
		return w.providerFactory(w.cfg, w.deps)
	}
	return NewAirKoreaProvider("", w.deps)
}

// isKoreanCoordinate returns true when lat/lon falls within the Korean peninsula
// bounding box: 33.0 <= lat <= 39.5 AND 124.0 <= lon <= 132.5.
func isKoreanCoordinate(lat, lon float64) bool {
	return lat >= 33.0 && lat <= 39.5 && lon >= 124.0 && lon <= 132.5
}

// isKoreanLocationString returns true when the location string contains a
// keyword that strongly implies a Korean city or the country code "KR".
func isKoreanLocationString(location string) bool {
	lower := strings.ToLower(location)
	koreanKeywords := []string{
		"kr", "korea", "한국",
		"seoul", "서울",
		"busan", "부산",
		"incheon", "인천",
		"daegu", "대구",
		"daejeon", "대전",
		"gwangju", "광주",
		"ulsan", "울산",
		"jeju", "제주",
		"gyeonggi", "경기",
		"gangwon", "강원",
		"sejong", "세종",
	}
	for _, kw := range koreanKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// deriveKoreanLocation validates that the input refers to a Korean location and
// returns a Location suitable for AirKorea API calls.
// Returns (loc, true) when the input is Korean; (zero, false) otherwise.
func deriveKoreanLocation(in weatherAirQualityInput) (Location, bool) {
	if in.hasLatLon {
		if !isKoreanCoordinate(in.Lat, in.Lon) {
			return Location{}, false
		}
		return Location{
			Lat:     in.Lat,
			Lon:     in.Lon,
			Country: "KR",
		}, true
	}
	if in.hasLocation {
		if !isKoreanLocationString(in.Location) {
			return Location{}, false
		}
		// Use Seoul centroid as default coordinates for location strings; the
		// AirKorea API uses sidoName, not coordinates, so the lat/lon here is
		// only used for cache-key derivation and future geocoding.
		return Location{
			Lat:         37.57,
			Lon:         126.98,
			DisplayName: in.Location,
			Country:     "KR",
		}, true
	}
	return Location{}, false
}

// openCache opens the bbolt TTL cache for air quality data. Returns nil on error.
func (w *webWeatherAirQuality) openCache() *common.Cache {
	if w.deps == nil || w.deps.Cwd == "" {
		return nil
	}
	cacheDir := w.deps.Cwd + "/weather"
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil
	}
	cachePath := cacheDir + "/cache.db"
	cache, err := common.OpenCache(cachePath, w.deps.Now)
	if err != nil {
		return nil
	}
	return cache
}

// writeAudit emits a structured audit event for the weather_air_quality call.
func (w *webWeatherAirQuality) writeAudit(
	_ context.Context, providerName, host string, statusCode int,
	cacheHit bool, outcome, reason string, start time.Time,
) {
	if w.deps == nil || w.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "weather_air_quality",
		"host":        host,
		"method":      http.MethodGet,
		"status_code": fmt.Sprintf("%d", statusCode),
		"cache_hit":   fmt.Sprintf("%t", cacheHit),
		"duration_ms": fmt.Sprintf("%d", w.deps.Now().Sub(start).Milliseconds()),
		"outcome":     outcome,
		"provider":    providerName,
	}
	if reason != "" {
		meta["reason"] = reason
	}
	ev := audit.NewAuditEvent(w.deps.Now(), audit.EventTypeToolWebInvoke, audit.SeverityInfo,
		"weather_air_quality invoked", meta)
	_ = w.deps.AuditWriter.Write(ev)
}

// airQualityCacheKey computes a SHA-256 cache key for an air quality request.
// Lat/lon are rounded to 2dp (~1.1 km) for cache stability.
func airQualityCacheKey(lat, lon float64) string {
	raw := fmt.Sprintf("weather_air_quality:airkorea:%.2f:%.2f", lat, lon)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// parseWeatherAirQualityInput parses and validates the raw JSON tool input.
func parseWeatherAirQualityInput(raw json.RawMessage) (weatherAirQualityInput, error) {
	var rawMap map[string]json.RawMessage
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &rawMap); err != nil {
			return weatherAirQualityInput{}, fmt.Errorf("invalid JSON: %w", err)
		}
	}

	var in weatherAirQualityInput
	if len(rawMap) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return weatherAirQualityInput{}, err
		}
	}

	_, hasLoc := rawMap["location"]
	_, hasLat := rawMap["lat"]
	_, hasLon := rawMap["lon"]

	in.hasLocation = hasLoc
	in.hasLatLon = hasLat && hasLon

	if in.hasLatLon {
		if in.Lat < -90 || in.Lat > 90 {
			return weatherAirQualityInput{}, fmt.Errorf("lat %v out of range [-90, 90]", in.Lat)
		}
		if in.Lon < -180 || in.Lon > 180 {
			return weatherAirQualityInput{}, fmt.Errorf("lon %v out of range [-180, 180]", in.Lon)
		}
	}

	if !in.hasLocation && !in.hasLatLon {
		return weatherAirQualityInput{}, fmt.Errorf("either location or lat+lon must be provided")
	}

	return in, nil
}

// NewWeatherAirQualityForTest constructs a weather_air_quality tool with all
// dependencies explicitly injected for test isolation.
func NewWeatherAirQualityForTest(
	deps *common.Deps,
	cfg *WeatherConfig,
	provider *AirKoreaProvider,
) tools.Tool {
	return &webWeatherAirQuality{
		deps:     deps,
		cfg:      cfg,
		provider: provider,
	}
}

// init registers weather_air_quality in the global web tool list.
func init() {
	cfg := defaultWeatherConfig()
	RegisterWebTool(&webWeatherAirQuality{
		deps:            &common.Deps{},
		cfg:             &cfg,
		providerFactory: productionAirQualityProviderFactory,
	})
}
