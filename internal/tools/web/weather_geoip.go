package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/modu-ai/mink/internal/tools/web/common"
)

const (
	ipapiEndpoint = "https://ipapi.co/json/"
	ipapiHost     = "ipapi.co"
	geoipCacheTTL = time.Hour
)

// ipapiResponse mirrors the relevant fields of the ipapi.co JSON response.
// Other fields are intentionally ignored.
type ipapiResponse struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	City        string  `json:"city"`
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	Timezone    string  `json:"timezone"`
	Error       bool    `json:"error"`
	Reason      string  `json:"reason"`
}

// geoipCacheEntry holds a resolved location and its expiry time.
type geoipCacheEntry struct {
	loc       Location
	expiresAt time.Time
}

// IPAPIGeolocator implements IPGeolocator using the ipapi.co free-tier API.
// Results are cached in memory for geoipCacheTTL (1 hour) to avoid redundant
// outbound calls from the same GOOSE session.
//
// Permission and blocklist checks for ipapi.co are performed by the caller
// (weather_current.go) before Resolve is invoked (T-022).
//
// @MX:ANCHOR: [AUTO] IPAPIGeolocator — IP-based location resolver with 1h in-memory cache
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-007 — fan_in >= 3 (weather_current, weather_geoip_test, init)
type IPAPIGeolocator struct {
	deps    *common.Deps
	baseURL string // injectable for testing; production uses ipapiEndpoint

	mu    sync.Mutex
	cache *geoipCacheEntry
}

// NewIPAPIGeolocator returns a production IPGeolocator backed by ipapi.co.
func NewIPAPIGeolocator(deps *common.Deps) IPGeolocator {
	return &IPAPIGeolocator{deps: deps, baseURL: ipapiEndpoint}
}

// NewIPAPIGeolocatorForTest returns an IPGeolocator with an injectable base URL
// so tests can redirect requests to an httptest.Server.
func NewIPAPIGeolocatorForTest(deps *common.Deps, baseURL string) IPGeolocator {
	return &IPAPIGeolocator{deps: deps, baseURL: baseURL}
}

// Resolve returns a Location from the caller's public IP.
// Results are cached in-memory for 1 hour. On expiry the live API is called.
//
// @MX:WARN: [AUTO] Holds mu while updating cache; kept minimal to avoid contention
// @MX:REASON: concurrent weather_current calls all call Resolve; lock scope must stay narrow
func (g *IPAPIGeolocator) Resolve(ctx context.Context) (Location, error) {
	now := g.deps.Now()

	// Fast path: check in-memory cache.
	g.mu.Lock()
	if g.cache != nil && now.Before(g.cache.expiresAt) {
		loc := g.cache.loc
		g.mu.Unlock()
		return loc, nil
	}
	g.mu.Unlock()

	// Slow path: call ipapi.co.
	loc, err := g.fetchFromAPI(ctx)
	if err != nil {
		return Location{}, ErrGeolocationFailed
	}

	// Update cache.
	g.mu.Lock()
	g.cache = &geoipCacheEntry{loc: loc, expiresAt: now.Add(geoipCacheTTL)}
	g.mu.Unlock()

	return loc, nil
}

// fetchFromAPI performs the outbound GET to ipapi.co and parses the response.
func (g *IPAPIGeolocator) fetchFromAPI(ctx context.Context) (Location, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL, nil)
	if err != nil {
		return Location{}, fmt.Errorf("geoip: build request: %w", err)
	}
	req.Header.Set("User-Agent", common.UserAgent())
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Location{}, fmt.Errorf("geoip: fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Location{}, fmt.Errorf("geoip: HTTP %d from %s", resp.StatusCode, ipapiHost)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if err != nil {
		return Location{}, fmt.Errorf("geoip: read body: %w", err)
	}

	var apiResp ipapiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return Location{}, fmt.Errorf("geoip: decode JSON: %w", err)
	}
	if apiResp.Error {
		return Location{}, fmt.Errorf("geoip: API error: %s", apiResp.Reason)
	}

	displayName := apiResp.City
	if displayName == "" {
		displayName = apiResp.CountryName
	}

	return Location{
		Lat:         apiResp.Latitude,
		Lon:         apiResp.Longitude,
		DisplayName: displayName,
		Country:     apiResp.CountryCode,
		Timezone:    apiResp.Timezone,
	}, nil
}

// InvalidateCache clears the in-memory cache entry, forcing the next Resolve
// call to hit the live API. Primarily used in tests.
func (g *IPAPIGeolocator) InvalidateCache() {
	g.mu.Lock()
	g.cache = nil
	g.mu.Unlock()
}
