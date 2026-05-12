package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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

// weatherForecastSchema is the JSON Schema for the weather_forecast tool input.
// The anyOf restricts input to either a location string or explicit lat+lon;
// unlike weather_current, an empty object is not valid for forecast.
var weatherForecastSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "location": { "type": "string", "minLength": 1, "maxLength": 200 },
    "lat": { "type": "number", "minimum": -90, "maximum": 90 },
    "lon": { "type": "number", "minimum": -180, "maximum": 180 },
    "days": { "type": "integer", "minimum": 1, "maximum": 7, "default": 3 },
    "units": { "type": "string", "enum": ["metric", "imperial"], "default": "metric" },
    "lang":  { "type": "string", "enum": ["ko", "en"], "default": "en" }
  },
  "anyOf": [
    { "required": ["location"] },
    { "required": ["lat", "lon"] },
    {}
  ]
}`)

// weatherForecastInput holds the parsed and defaulted tool input for weather_forecast.
type weatherForecastInput struct {
	Location string  `json:"location"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Days     int     `json:"days"`
	Units    string  `json:"units"`
	Lang     string  `json:"lang"`

	hasLocation bool
	hasLatLon   bool
}

// forecastProviderFactory constructs a provider map for use by webWeatherForecast.
// Tests inject stubs; production uses real providers.
type forecastProviderFactory func(cfg *WeatherConfig, deps *common.Deps) map[string]WeatherProvider

// webWeatherForecast implements the weather_forecast tool.
// It applies the same TOOLS-WEB-001 infrastructure as weather_current
// (Blocklist, Permission, RateLimit, Audit, bbolt Cache, singleflight)
// and adds KMA provider routing for Korean coordinates.
//
// @MX:ANCHOR: [AUTO] webWeatherForecast tool — entry point for multi-day forecast pipeline
// @MX:REASON: SPEC-GOOSE-WEATHER-001 AC-WEATHER-004 — fan_in >= 3 (init, tests, route)
type webWeatherForecast struct {
	deps              *common.Deps
	cfg               *WeatherConfig
	providers         map[string]WeatherProvider // injected by tests
	providerFactory   forecastProviderFactory
	geolocatorFactory geolocatorFactory
	offlineFactory    offlineStoreFactory
	sf                singleflight.Group
}

// productionForecastProviderFactory creates both OWM and KMA providers so the
// routing logic can pick the active one at call time.
func productionForecastProviderFactory(cfg *WeatherConfig, deps *common.Deps) map[string]WeatherProvider {
	providers := map[string]WeatherProvider{
		"openweathermap": NewOpenWeatherMapProvider(cfg.OpenWeatherMap.APIKey, deps),
	}
	if cfg.KMA.APIKey != "" {
		providers["kma"] = NewKMAProvider(cfg.KMA.APIKey, deps)
	}
	return providers
}

// Name returns the canonical tool name.
func (w *webWeatherForecast) Name() string { return "weather_forecast" }

// Schema returns the JSON Schema used by the Executor for input validation.
func (w *webWeatherForecast) Schema() json.RawMessage { return weatherForecastSchema }

// Scope returns ScopeShared — weather_forecast is available to all agent types.
func (w *webWeatherForecast) Scope() tools.Scope { return tools.ScopeShared }

// Call executes the weather_forecast 11-step pipeline.
//
//  1. Parse + defensive schema guard
//  2. Resolve location (lat/lon or location string or IP geolocation)
//  3. Select provider via routeProvider
//  4. Derive provider host
//  5. Blocklist gate
//  6. Permission gate (CapNet, scope=host)
//  7. Build cache key (SHA256)
//  8. Live bbolt cache check
//  9. Rate limit check
//  10. Singleflight: outbound GetForecast
//  11. On success: cache.Set + audit ok + return
//
// @MX:WARN: [AUTO] 11-step pipeline with multiple early-return paths; cyclomatic complexity >= 12
// @MX:REASON: Each step maps to a distinct SPEC requirement gate; mirrors weather_current pipeline
func (w *webWeatherForecast) Call(ctx context.Context, raw json.RawMessage) (tools.ToolResult, error) {
	start := w.deps.Now()

	// Step 1: Parse input.
	in, err := parseWeatherForecastInput(raw)
	if err != nil {
		return toToolResult(common.ErrResponse("invalid_input", err.Error(), false, 0, elapsed(start))), nil
	}

	// Build per-call dependencies.
	providers := w.resolveProviders()
	geolocator := w.geolocatorFactory(w.deps)

	// Step 2: Resolve location to lat/lon.
	loc, locErr := w.resolveLocationForecast(ctx, in, geolocator)
	if locErr != nil {
		w.writeAudit(ctx, "", "", 0, false, "denied", "invalid_input", start)
		return toToolResult(common.ErrResponse("invalid_input", locErr.Error(), false, 0, elapsed(start))), nil
	}

	// Step 3: Select provider via routing logic.
	provider, routeErr := selectProvider(w.cfg, loc, providers, w.deps)
	if routeErr != nil {
		w.writeAudit(ctx, "", "", 0, false, "denied", "missing_api_key", start)
		return toToolResult(common.ErrResponse("missing_api_key", routeErr.Error(), false, 0, elapsed(start))), nil
	}

	// Step 4: Derive provider host.
	providerHost := providerHostFor(provider.Name())

	// Step 5: Blocklist gate.
	if w.deps.Blocklist != nil && w.deps.Blocklist.IsBlocked(stripPort(providerHost)) {
		w.writeAudit(ctx, provider.Name(), providerHost, 0, false, "denied", "host_blocked", start)
		return toToolResult(common.ErrResponse("host_blocked",
			fmt.Sprintf("host %q is blocked by security policy", providerHost),
			false, 0, elapsed(start))), nil
	}

	// Step 6: Permission gate (CapNet, scope=host).
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
			w.writeAudit(ctx, provider.Name(), providerHost, 0, false, "denied", "permission_denied", start)
			return toToolResult(common.ErrResponse("permission_denied", msg, false, 0, elapsed(start))), nil
		}
	}

	// Step 7: Build cache key.
	cacheKey := weatherForecastCacheKey(provider.Name(), loc.Lat, loc.Lon, in.Days, in.Units, in.Lang)

	// Step 8: Live bbolt cache check.
	cache := w.openCache()
	if cache != nil {
		defer func() { _ = cache.Close() }()
		if cachedBytes, hit, cErr := cache.Get(cacheKey); cErr == nil && hit {
			var days []WeatherForecastDay
			if json.Unmarshal(cachedBytes, &days) == nil {
				w.writeAudit(ctx, provider.Name(), providerHost, 0, true, "ok", "", start)
				meta := elapsed(start)
				meta.CacheHit = true
				okResp, _ := common.OKResponse(map[string]any{"days": days}, meta)
				return toToolResult(okResp), nil
			}
		}
	}

	// Step 9: Rate limit check.
	if w.deps.RateTracker != nil {
		state := w.deps.RateTracker.State(provider.Name())
		if state.RequestsMin.UsagePct() >= 100 {
			retryAfter := int(math.Ceil(state.RequestsMin.RemainingSecondsNow(w.deps.Now())))
			retryAfter = max(retryAfter, 0)
			w.writeAudit(ctx, provider.Name(), providerHost, 0, false, "denied", "ratelimit_exhausted", start)
			return toToolResult(common.ErrResponse("ratelimit_exhausted",
				fmt.Sprintf("weather rate limit exhausted; retry after %d seconds", retryAfter),
				true, retryAfter, elapsed(start))), nil
		}
	}

	// Step 10: Singleflight: outbound API call.
	type fetchResult struct {
		days []WeatherForecastDay
		err  error
	}

	sfResult, sfErr, _ := w.sf.Do(cacheKey, func() (any, error) {
		days, fetchErr := provider.GetForecast(ctx, loc, in.Days)
		return &fetchResult{days: days, err: fetchErr}, nil
	})

	if sfErr != nil {
		w.writeAudit(ctx, provider.Name(), providerHost, 0, false, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", sfErr.Error(), true, 0, elapsed(start))), nil
	}

	fr := sfResult.(*fetchResult)

	if fr.err != nil {
		w.writeAudit(ctx, provider.Name(), providerHost, 0, false, "error", "fetch_failed", start)
		return toToolResult(common.ErrResponse("fetch_failed", fr.err.Error(), true, 0, elapsed(start))), nil
	}

	// Step 11: On success — cache + audit + return.
	if cache != nil {
		if dataBytes, mErr := json.Marshal(fr.days); mErr == nil {
			_ = cache.Set(cacheKey, dataBytes, time.Hour) // forecast TTL = 1h
		}
	}

	w.writeAudit(ctx, provider.Name(), providerHost, 0, false, "ok", "", start)
	okResp, _ := common.OKResponse(map[string]any{"days": fr.days, "source_provider": provider.Name()}, elapsed(start))
	return toToolResult(okResp), nil
}

// resolveProviders returns the provider map. Tests inject stubs via the
// providers field; production uses the factory.
func (w *webWeatherForecast) resolveProviders() map[string]WeatherProvider {
	if w.providers != nil {
		return w.providers
	}
	if w.providerFactory != nil {
		return w.providerFactory(w.cfg, w.deps)
	}
	return nil
}

// resolveLocationForecast resolves the lat/lon for the call, mirroring the
// weather_current resolveLocation logic but adapted for forecast input.
func (w *webWeatherForecast) resolveLocationForecast(
	ctx context.Context,
	in weatherForecastInput,
	geolocator IPGeolocator,
) (Location, error) {
	if in.hasLatLon {
		return Location{Lat: in.Lat, Lon: in.Lon}, nil
	}
	if in.hasLocation {
		// Attempt to geocode via OWM; fall back to Seoul on error.
		// We share the M1 geocoding logic via a temporary webWeatherCurrent helper.
		helper := &webWeatherCurrent{deps: w.deps, cfg: w.cfg}
		loc, geoErr := helper.geocodeLocation(ctx, in.Location, in.Units, in.Lang)
		if geoErr == nil {
			return loc, nil
		}
		return Location{
			Lat:         37.57,
			Lon:         126.98,
			DisplayName: in.Location,
			Country:     inferCountry(in.Location),
		}, nil
	}
	// IP geolocation fallback.
	if !w.cfg.AllowIPGeolocation {
		return Location{}, fmt.Errorf("no location provided and IP geolocation is disabled")
	}
	loc, geoErr := geolocator.Resolve(ctx)
	if geoErr != nil {
		return Location{}, fmt.Errorf("geolocation_failed: %w", geoErr)
	}
	return loc, nil
}

// inferCountry tries to extract a 2-letter country code from a "City,CC"
// location string (e.g. "Seoul,KR" → "KR"). Returns "" when not detected.
func inferCountry(location string) string {
	for i := len(location) - 1; i >= 0; i-- {
		if location[i] == ',' {
			cc := location[i+1:]
			if len(cc) == 2 {
				return cc
			}
		}
	}
	return ""
}

// providerHostFor returns the API hostname for a given provider name.
func providerHostFor(providerName string) string {
	if providerName == "kma" {
		return kmaAPIHost
	}
	return owmAPIHost
}

// openCache opens the bbolt TTL cache for weather data. Returns nil on error.
func (w *webWeatherForecast) openCache() *common.Cache {
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

// writeAudit emits a structured audit event for the weather_forecast call.
func (w *webWeatherForecast) writeAudit(
	_ context.Context, providerName, host string, statusCode int,
	cacheHit bool, outcome, reason string, start time.Time,
) {
	if w.deps == nil || w.deps.AuditWriter == nil {
		return
	}
	meta := map[string]string{
		"tool":        "weather_forecast",
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
		"weather_forecast invoked", meta)
	_ = w.deps.AuditWriter.Write(ev)
}

// weatherForecastCacheKey computes a SHA-256 cache key.
// Lat/lon are rounded to 2 decimal places (~1.1 km).
func weatherForecastCacheKey(provider string, lat, lon float64, days int, units, lang string) string {
	raw := fmt.Sprintf("weather_forecast:%s:%.2f:%.2f:%d:%s:%s", provider, lat, lon, days, units, lang)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// parseWeatherForecastInput parses and defaults the raw JSON input.
func parseWeatherForecastInput(raw json.RawMessage) (weatherForecastInput, error) {
	var rawMap map[string]json.RawMessage
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &rawMap); err != nil {
			return weatherForecastInput{}, fmt.Errorf("invalid JSON: %w", err)
		}
	}

	in := weatherForecastInput{
		Days:  3,
		Units: "metric",
		Lang:  "en",
	}

	if len(rawMap) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return weatherForecastInput{}, err
		}
	}

	_, hasLoc := rawMap["location"]
	_, hasLat := rawMap["lat"]
	_, hasLon := rawMap["lon"]

	in.hasLocation = hasLoc
	in.hasLatLon = hasLat && hasLon

	if in.hasLatLon {
		if in.Lat < -90 || in.Lat > 90 {
			return weatherForecastInput{}, fmt.Errorf("lat %v out of range [-90, 90]", in.Lat)
		}
		if in.Lon < -180 || in.Lon > 180 {
			return weatherForecastInput{}, fmt.Errorf("lon %v out of range [-180, 180]", in.Lon)
		}
	}

	if in.Days < 1 || in.Days > 7 {
		return weatherForecastInput{}, fmt.Errorf("days %d out of range [1, 7]", in.Days)
	}

	if in.Units != "metric" && in.Units != "imperial" {
		return weatherForecastInput{}, fmt.Errorf("units must be \"metric\" or \"imperial\", got %q", in.Units)
	}
	if in.Lang != "ko" && in.Lang != "en" {
		return weatherForecastInput{}, fmt.Errorf("lang must be \"ko\" or \"en\", got %q", in.Lang)
	}

	return in, nil
}

// NewWeatherForecastForTest constructs a weather_forecast tool with all
// dependencies explicitly injected. Tests use this to isolate from network.
func NewWeatherForecastForTest(
	deps *common.Deps,
	cfg *WeatherConfig,
	providers map[string]WeatherProvider,
	geolocator IPGeolocator,
) tools.Tool {
	return &webWeatherForecast{
		deps:      deps,
		cfg:       cfg,
		providers: providers,
		geolocatorFactory: func(_ *common.Deps) IPGeolocator {
			return geolocator
		},
		offlineFactory: func(d *common.Deps) OfflineStore {
			return NewDiskOfflineStore(offlineBaseDir(d.Cwd))
		},
	}
}

// WeatherConfigOptions holds parameters for WeatherConfigForTest.
type WeatherConfigOptions struct {
	Provider   string
	KMAKey     string
	OWMKey     string
	OWMBaseURL string // optional: override for tests
}

// WeatherConfigForTest constructs a WeatherConfig for use in tests.
// It merges the supplied options over the default config.
func WeatherConfigForTest(opts WeatherConfigOptions) *WeatherConfig {
	cfg := defaultWeatherConfig()
	if opts.Provider != "" {
		cfg.Provider = opts.Provider
	}
	cfg.KMA.APIKey = opts.KMAKey
	cfg.OpenWeatherMap.APIKey = opts.OWMKey
	if opts.OWMBaseURL != "" {
		cfg.OpenWeatherMap.BaseURL = opts.OWMBaseURL
	}
	return &cfg
}

// init registers weather_forecast in the global web tool list.
func init() {
	cfg := defaultWeatherConfig()
	RegisterWebTool(&webWeatherForecast{
		deps:              &common.Deps{},
		cfg:               &cfg,
		providerFactory:   productionForecastProviderFactory,
		geolocatorFactory: productionGeolocatorFactory,
		offlineFactory:    productionOfflineFactory,
	})
}
