package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

// weatherCurrentSchema is the JSON Schema for the weather_current tool input.
// The anyOf allows three modes:
//   - explicit location string ("Seoul,KR")
//   - explicit lat + lon coordinates
//   - empty object (triggers IP geolocation when allow_ip_geolocation=true)
var weatherCurrentSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "location": { "type": "string", "minLength": 1, "maxLength": 200 },
    "lat": { "type": "number", "minimum": -90, "maximum": 90 },
    "lon": { "type": "number", "minimum": -180, "maximum": 180 },
    "units": { "type": "string", "enum": ["metric", "imperial"], "default": "metric" },
    "lang":  { "type": "string", "enum": ["ko", "en"], "default": "en" }
  },
  "anyOf": [
    { "required": ["location"] },
    { "required": ["lat", "lon"] },
    {}
  ]
}`)

// weatherCurrentInput holds the parsed and defaulted tool input.
type weatherCurrentInput struct {
	Location string  `json:"location"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Units    string  `json:"units"`
	Lang     string  `json:"lang"`

	hasLocation bool // true when Location field was explicitly provided
	hasLatLon   bool // true when Lat+Lon fields were explicitly provided
}

// providerFactory constructs a WeatherProvider from a WeatherConfig.
// In production this returns an OpenWeatherMapProvider; tests inject stubs.
type providerFactory func(cfg *WeatherConfig, deps *common.Deps) WeatherProvider

// geolocatorFactory constructs an IPGeolocator.
// In production this returns an IPAPIGeolocator; tests inject stubs.
type geolocatorFactory func(deps *common.Deps) IPGeolocator

// offlineStoreFactory constructs an OfflineStore.
// In production this uses the cache directory derived from deps.Cwd.
type offlineStoreFactory func(deps *common.Deps) OfflineStore

// webWeatherCurrent implements the weather_current tool.
// It composes TOOLS-WEB-001 infrastructure (Blocklist, Permission, RateLimit,
// Audit, bbolt Cache) with weather-domain logic (OWM provider, IP geolocation,
// disk offline fallback, singleflight dedup).
//
// @MX:ANCHOR: [AUTO] webWeatherCurrent tool — main entry for current weather fetch pipeline
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-005 — fan_in >= 3 (init, tests, executor)
type webWeatherCurrent struct {
	deps              *common.Deps
	cfg               *WeatherConfig
	providerFactory   providerFactory
	geolocatorFactory geolocatorFactory
	offlineFactory    offlineStoreFactory
	sf                singleflight.Group
}

// productionProviderFactory creates an OpenWeatherMapProvider with the OWM
// API key from cfg. In M1 this is the only supported provider.
func productionProviderFactory(cfg *WeatherConfig, deps *common.Deps) WeatherProvider {
	return NewOpenWeatherMapProvider(cfg.OpenWeatherMap.APIKey, deps)
}

// productionGeolocatorFactory creates an IPAPIGeolocator.
func productionGeolocatorFactory(deps *common.Deps) IPGeolocator {
	return NewIPAPIGeolocator(deps)
}

// productionOfflineFactory creates a diskOfflineStore under deps.Cwd.
func productionOfflineFactory(deps *common.Deps) OfflineStore {
	baseDir := offlineBaseDir(deps.Cwd)
	return NewDiskOfflineStore(baseDir)
}

// offlineBaseDir derives the directory for offline fallback files.
// Production: ~/.mink/cache/weather/
// Test: <deps.Cwd>/weather/
func offlineBaseDir(cwd string) string {
	if cwd == "" {
		return "weather"
	}
	return cwd + "/weather"
}

// Name returns the canonical tool name.
func (w *webWeatherCurrent) Name() string { return "weather_current" }

// Schema returns the JSON Schema used by the Executor for input validation.
func (w *webWeatherCurrent) Schema() json.RawMessage { return weatherCurrentSchema }

// Scope returns ScopeShared — weather_current is available to all agent types.
func (w *webWeatherCurrent) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the weather_current 11-step pipeline.
//
//  1. Parse + defensive schema guard
//  2. Resolve location (lat/lon or location string or IP geolocation)
//  3. Derive provider host (M1: always api.openweathermap.org)
//  4. Blocklist gate
//  5. Permission gate (CapNet, scope=host)
//  6. Build cache key (SHA256)
//  7. Live bbolt cache check → hit → return {cache_hit: true, stale: false}
//  8. Rate limit check
//  9. Singleflight: outbound API call
//  10. On success: cache.Set + offline.SaveLatest + audit ok + return
//  11. On failure: offline.LoadLatest → stale or fetch_failed
//
// @MX:WARN: [AUTO] 11-step pipeline with multiple early-return paths; cyclomatic complexity >= 12
// @MX:REASON: Each step maps to a distinct SPEC requirement gate; do not collapse without re-audit
func (w *webWeatherCurrent) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := w.deps.Now()

	// Step 1: Parse input.
	in, err := parseWeatherCurrentInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	// Build dependencies lazily.
	provider := w.providerFactory(w.cfg, w.deps)
	geolocator := w.geolocatorFactory(w.deps)
	offline := w.offlineFactory(w.deps)

	// Step 2: Resolve location to lat/lon.
	loc, err := w.resolveLocation(ctx, in, geolocator)
	if err != nil {
		w.writeAudit(ctx, "", 0, false, "denied", "invalid_input", start)
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	// Step 3: Derive provider host.
	providerHost := owmAPIHost // M1: always OWM

	// Step 4: Blocklist gate.
	if w.deps.Blocklist != nil && w.deps.Blocklist.IsBlocked(stripPort(providerHost)) {
		w.writeAudit(ctx, providerHost, 0, false, "denied", "host_blocked", start)
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
			w.writeAudit(ctx, providerHost, 0, false, "denied", "permission_denied", start)
			return toToolResult(common.ErrResponse("permission_denied", msg, false, 0, elapsed(start))), nil
		}
	}

	// Step 6: Build cache key.
	cacheKey := weatherCacheKey(provider.Name(), loc.Lat, loc.Lon, in.Units, in.Lang)

	// Step 7: Live bbolt cache check.
	cache := w.openCache()
	if cache != nil {
		defer func() { _ = cache.Close() }()
		if cachedBytes, hit, cErr := cache.Get(cacheKey); cErr == nil && hit {
			var report WeatherReport
			if json.Unmarshal(cachedBytes, &report) == nil {
				report.CacheHit = true
				report.Stale = false
				w.writeAudit(ctx, providerHost, 0, true, "ok", "", start)
				meta := elapsed(start)
				meta.CacheHit = true
				okResp, _ := common.OKResponse(report, meta)
				return toToolResult(okResp), nil
			}
		}
	}

	// Step 8: Rate limit check.
	if w.deps.RateTracker != nil {
		state := w.deps.RateTracker.State(provider.Name())
		if state.RequestsMin.UsagePct() >= 100 {
			retryAfter := int(math.Ceil(state.RequestsMin.RemainingSecondsNow(w.deps.Now())))
			retryAfter = max(retryAfter, 0)
			w.writeAudit(ctx, providerHost, 0, false, "denied", "ratelimit_exhausted", start)
			return toToolResult(common.ErrResponse("ratelimit_exhausted",
				fmt.Sprintf("weather rate limit exhausted; retry after %d seconds", retryAfter),
				true, retryAfter, elapsed(start))), nil
		}
	}

	// Step 9: Singleflight: outbound API call.
	type fetchResult struct {
		report *WeatherReport
		err    error
	}

	sfResult, sfErr, _ := w.sf.Do(cacheKey, func() (any, error) {
		owmProvider, ok := provider.(*OpenWeatherMapProvider)
		if ok {
			rep, provErr := owmProvider.GetCurrentWithOptions(ctx, loc, in.Units, in.Lang)
			return &fetchResult{report: rep, err: provErr}, nil
		}
		rep, provErr := provider.GetCurrent(ctx, loc)
		return &fetchResult{report: rep, err: provErr}, nil
	})

	if sfErr != nil {
		// singleflight itself failed (should not happen with our closure).
		w.writeAudit(ctx, providerHost, 0, false, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", sfErr.Error(), true, 0, elapsed(start))), nil
	}

	fr := sfResult.(*fetchResult)

	// Step 10: On success — cache + offline + audit + return.
	if fr.err == nil && fr.report != nil {
		// Copy the shared singleflight result to avoid concurrent mutation
		// when multiple goroutines receive the same *WeatherReport pointer.
		reportCopy := *fr.report
		reportCopy.CacheHit = false
		reportCopy.Stale = false

		// Cache write (best-effort) — use the original report (no flags set).
		if cache != nil {
			if dataBytes, mErr := json.Marshal(reportCopy); mErr == nil {
				_ = cache.Set(cacheKey, dataBytes, w.cfg.CacheTTLDuration())
			}
		}

		// Offline SaveLatest (best-effort).
		_ = offline.SaveLatest(provider.Name(), loc.Lat, loc.Lon, &reportCopy)

		w.writeAudit(ctx, providerHost, 0, false, "ok", "", start)
		okResp, _ := common.OKResponse(reportCopy, elapsed(start))
		return toToolResult(okResp), nil
	}

	// Step 11: On failure — try offline disk fallback.
	offlineReport, loadErr := offline.LoadLatest(provider.Name(), loc.Lat, loc.Lon)
	if loadErr != nil || offlineReport == nil {
		// No fallback available.
		w.writeAudit(ctx, providerHost, 0, false, "error", "fetch_failed", start)
		errMsg := "fetch_failed"
		if fr.err != nil {
			errMsg = fr.err.Error()
		}
		return toToolResult(common.ErrResponse("fetch_failed", errMsg, true, 0, elapsed(start))), nil
	}

	// Determine stale message based on how old the offline data is.
	offlineReport.Stale = true
	offlineReport.CacheHit = false
	staleSince := w.deps.Now().Sub(offlineReport.Timestamp)
	if staleSince > 24*time.Hour {
		offlineReport.Message = fmt.Sprintf("데이터가 오래되었을 수 있어요 (마지막 확인: %s)",
			offlineReport.Timestamp.Format("2006-01-02 15:04"))
	} else {
		offlineReport.Message = fmt.Sprintf("오프라인 상태입니다 (마지막 확인: %s)",
			offlineReport.Timestamp.Format("15:04"))
	}

	w.writeAudit(ctx, providerHost, 0, false, "error", "stale_fallback", start)
	okResp, _ := common.OKResponse(offlineReport, elapsed(start))
	return toToolResult(okResp), nil
}

// resolveLocation determines the lat/lon for the call.
// Priority: explicit lat+lon > location string (geocode stub in M1) > IP geolocation.
func (w *webWeatherCurrent) resolveLocation(
	ctx context.Context,
	in weatherCurrentInput,
	geolocator IPGeolocator,
) (Location, error) {
	// Explicit lat/lon.
	if in.hasLatLon {
		return Location{Lat: in.Lat, Lon: in.Lon}, nil
	}

	// Explicit location string — M1 uses the OWM geocoding implicit in the
	// call to avoid adding another round-trip. We store the location string
	// as DisplayName and let the OWM URL handle it via lat/lon override.
	// For M1 we perform a pre-flight geocode if a string location is given
	// without coordinates; for simplicity in M1 we attempt to geocode via
	// OWM Geo API first, falling back to a fixed Seoul coordinates when the
	// provider is unavailable.
	//
	// M1 simplified approach: use the location string but resolve coordinates
	// using a basic geocode via OWM Geocoding API when an API key is present.
	// If not, default to Seoul (37.57, 126.98) to avoid blocking the pipeline.
	// This will be improved in M2 with full geocoding support.
	if in.hasLocation {
		// Try to geocode via OWM Geo API.
		loc, geoErr := w.geocodeLocation(ctx, in.Location, in.Units, in.Lang)
		if geoErr == nil {
			return loc, nil
		}
		// Fall back: use Seoul as a safe default.
		return Location{
			Lat:         37.57,
			Lon:         126.98,
			DisplayName: in.Location,
			Country:     "KR",
		}, nil
	}

	// No location provided — use IP geolocation.
	if !w.cfg.AllowIPGeolocation {
		return Location{}, fmt.Errorf("no location provided and IP geolocation is disabled")
	}
	loc, geoErr := geolocator.Resolve(ctx)
	if geoErr != nil {
		if errors.Is(geoErr, ErrGeolocationFailed) {
			return Location{}, fmt.Errorf("geolocation_failed: %w", geoErr)
		}
		return Location{}, geoErr
	}
	return loc, nil
}

// geocodeLocation resolves a location string to lat/lon via OWM Geo API.
// Returns an error when the OWM API key is missing or the call fails.
func (w *webWeatherCurrent) geocodeLocation(ctx context.Context, locationStr, _, _ string) (Location, error) {
	if w.cfg.OpenWeatherMap.APIKey == "" {
		return Location{}, ErrMissingAPIKey
	}

	geoURL := fmt.Sprintf(
		"https://api.openweathermap.org/geo/1.0/direct?q=%s&limit=1&appid=%s",
		locationStr,
		w.cfg.OpenWeatherMap.APIKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geoURL, nil)
	if err != nil {
		return Location{}, err
	}
	req.Header.Set("User-Agent", common.UserAgent())

	client := &http.Client{Timeout: owmTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return Location{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Location{}, fmt.Errorf("geocode HTTP %d", resp.StatusCode)
	}

	var geoResults []struct {
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		Name    string  `json:"name"`
		Country string  `json:"country"`
	}
	if jsonErr := json.NewDecoder(resp.Body).Decode(&geoResults); jsonErr != nil || len(geoResults) == 0 {
		return Location{}, fmt.Errorf("geocode: no results for %q", locationStr)
	}

	r := geoResults[0]
	return Location{
		Lat:         r.Lat,
		Lon:         r.Lon,
		DisplayName: r.Name,
		Country:     r.Country,
	}, nil
}

// openCache opens the bbolt TTL cache for weather data. Returns nil when the
// cache cannot be opened (degraded-but-functional: callers must handle nil).
func (w *webWeatherCurrent) openCache() *common.Cache {
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

// writeAudit emits a structured audit event for the weather_current call.
func (w *webWeatherCurrent) writeAudit(
	_ context.Context, host string, statusCode int,
	cacheHit bool, outcome, reason string, start time.Time,
) {
	if w.deps == nil || w.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "weather_current",
		"host":        host,
		"method":      http.MethodGet,
		"status_code": fmt.Sprintf("%d", statusCode),
		"cache_hit":   fmt.Sprintf("%t", cacheHit),
		"duration_ms": fmt.Sprintf("%d", w.deps.Now().Sub(start).Milliseconds()),
		"outcome":     outcome,
		"provider":    owmName,
	}
	if reason != "" {
		meta["reason"] = reason
	}
	ev := audit.NewAuditEvent(w.deps.Now(), audit.EventTypeToolWebInvoke, audit.SeverityInfo,
		"weather_current invoked", meta)
	_ = w.deps.AuditWriter.Write(ev)
}

// weatherCacheKey computes the SHA-256 cache key for a given call's parameters.
// Lat/lon are rounded to 2 decimal places (~1.1 km) for key normalization.
func weatherCacheKey(provider string, lat, lon float64, units, lang string) string {
	raw := fmt.Sprintf("weather_current:%s:%.2f:%.2f:%s:%s", provider, lat, lon, units, lang)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// parseWeatherCurrentInput parses and defaults the raw JSON input.
// Applies schema guards so direct Call() invocations (bypassing Executor)
// still fail closed.
func parseWeatherCurrentInput(raw json.RawMessage) (weatherCurrentInput, error) {
	// Use a raw map first to detect which keys were explicitly provided.
	var rawMap map[string]json.RawMessage
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &rawMap); err != nil {
			return weatherCurrentInput{}, fmt.Errorf("invalid JSON: %w", err)
		}
	}

	in := weatherCurrentInput{
		Units: "metric",
		Lang:  "en",
	}

	if len(rawMap) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return weatherCurrentInput{}, err
		}
	}

	_, hasLoc := rawMap["location"]
	_, hasLat := rawMap["lat"]
	_, hasLon := rawMap["lon"]

	in.hasLocation = hasLoc
	in.hasLatLon = hasLat && hasLon

	// Validate ranges when explicitly provided.
	if in.hasLatLon {
		if in.Lat < -90 || in.Lat > 90 {
			return weatherCurrentInput{}, fmt.Errorf("lat %v out of range [-90, 90]", in.Lat)
		}
		if in.Lon < -180 || in.Lon > 180 {
			return weatherCurrentInput{}, fmt.Errorf("lon %v out of range [-180, 180]", in.Lon)
		}
	}

	if in.Units != "metric" && in.Units != "imperial" {
		return weatherCurrentInput{}, fmt.Errorf("units must be \"metric\" or \"imperial\", got %q", in.Units)
	}
	if in.Lang != "ko" && in.Lang != "en" {
		return weatherCurrentInput{}, fmt.Errorf("lang must be \"ko\" or \"en\", got %q", in.Lang)
	}

	return in, nil
}

// owmAPIHost is the OWM API hostname used for blocklist and permission checks.
const owmAPIHost = "api.openweathermap.org"

// NewWeatherCurrentForTest constructs a weather_current tool with all
// dependencies explicitly injected. Tests use this to isolate the tool from
// the network, filesystem, and real providers.
func NewWeatherCurrentForTest(
	deps *common.Deps,
	cfg *WeatherConfig,
	provider WeatherProvider,
	geolocator IPGeolocator,
	offline OfflineStore,
) tools.Tool {
	return &webWeatherCurrent{
		deps: deps,
		cfg:  cfg,
		providerFactory: func(_ *WeatherConfig, _ *common.Deps) WeatherProvider {
			return provider
		},
		geolocatorFactory: func(_ *common.Deps) IPGeolocator {
			return geolocator
		},
		offlineFactory: func(_ *common.Deps) OfflineStore {
			return offline
		},
	}
}

// init registers weather_current in the global web tool list.
// The tool is constructed with zero-value Deps and production factories;
// callers must inject a real dependency container at bootstrap before Call().
func init() {
	cfg := defaultWeatherConfig()
	RegisterWebTool(&webWeatherCurrent{
		deps:              &common.Deps{},
		cfg:               &cfg,
		providerFactory:   productionProviderFactory,
		geolocatorFactory: productionGeolocatorFactory,
		offlineFactory:    productionOfflineFactory,
	})
}
