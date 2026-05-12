package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"github.com/modu-ai/mink/internal/permission"
	permstore "github.com/modu-ai/mink/internal/permission/store"
	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// TestFirstCallConfirm_WebSearch — DC-03 / AC-WEB-003
// --------------------------------------------------------------------------

// TestFirstCallConfirm_WebSearch exercises the first-call confirmation flow:
//   - 1st call: Confirmer.Ask is invoked exactly once, grant is saved.
//   - 2nd same-input call: Ask is not called again (grant cached) AND
//     cache_hit=true (result cached).
//   - DenyCase subtest: Ask returns Deny → permission_denied, no fetch.
func TestFirstCallConfirm_WebSearch(t *testing.T) {
	t.Run("allow_alwaysallow", func(t *testing.T) {
		var fetchCount atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fetchCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"web":{"results":[{"title":"T","url":"https://x.com","description":"D"}]}}`))
		}))
		defer srv.Close()

		confirmer := &countingConfirmer{
			decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow},
		}
		store := permstore.NewMemoryStore()
		require.NoError(t, store.Open())
		mgr, err := permission.New(store, confirmer, nil, nil, nil)
		require.NoError(t, err)
		require.NoError(t, mgr.Register("agent:goose", permission.Manifest{
			NetHosts: []string{"api.search.brave.com"},
		}))

		deps := &common.Deps{
			PermMgr:     mgr,
			AuditWriter: noopAuditWriter{},
			Clock:       func() time.Time { return time.Now() },
			Cwd:         t.TempDir(),
		}

		tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
		require.NoError(t, err)
		deps.RateTracker = tracker
		web.RegisterBraveParser(tracker)

		tool := web.NewWebSearch(deps, srv.URL)
		input := buildSearchInput(t, map[string]any{"query": "perm-test", "provider": "brave"})

		// First call: Ask must be called once.
		result1, err := tool.Call(context.Background(), input)
		require.NoError(t, err)
		resp1 := unmarshalResponse(t, result1)
		require.True(t, resp1.OK, "first call must succeed; error=%v", resp1.Error)
		assert.Equal(t, 1, confirmer.count, "Ask must be called exactly once on first call")
		assert.Equal(t, int64(1), fetchCount.Load(), "provider must be called once on cache miss")

		// Second call: same input → Ask not called again, cache hit.
		result2, err := tool.Call(context.Background(), input)
		require.NoError(t, err)
		resp2 := unmarshalResponse(t, result2)
		require.True(t, resp2.OK)
		assert.Equal(t, 1, confirmer.count, "Ask must not be called on second call (grant cached)")
		assert.True(t, resp2.Metadata.CacheHit, "second call must be a cache hit")
		assert.Equal(t, int64(1), fetchCount.Load(), "provider must NOT be called on cache hit")
	})

	t.Run("deny_case", func(t *testing.T) {
		var fetchCount atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fetchCount.Add(1)
			_, _ = w.Write([]byte(`{"web":{"results":[]}}`))
		}))
		defer srv.Close()

		denyConfirmer := &countingConfirmer{
			decision: permission.Decision{Allow: false, Choice: permission.DecisionDeny},
		}
		store := permstore.NewMemoryStore()
		require.NoError(t, store.Open())
		mgr, err := permission.New(store, denyConfirmer, nil, nil, nil)
		require.NoError(t, err)
		require.NoError(t, mgr.Register("agent:goose", permission.Manifest{
			NetHosts: []string{"api.search.brave.com"},
		}))

		deps := &common.Deps{
			PermMgr:     mgr,
			AuditWriter: noopAuditWriter{},
			Clock:       func() time.Time { return time.Now() },
			Cwd:         t.TempDir(),
		}
		tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
		require.NoError(t, err)
		deps.RateTracker = tracker
		web.RegisterBraveParser(tracker)

		tool := web.NewWebSearch(deps, srv.URL)
		input := buildSearchInput(t, map[string]any{"query": "deny-test", "provider": "brave"})

		result, err := tool.Call(context.Background(), input)
		require.NoError(t, err)
		resp := unmarshalResponse(t, result)

		assert.False(t, resp.OK)
		require.NotNil(t, resp.Error)
		assert.Equal(t, "permission_denied", resp.Error.Code)
		assert.Equal(t, int64(0), fetchCount.Load(), "provider must NOT be called when permission denied")
	})
}

// --------------------------------------------------------------------------
// TestFirstCallConfirm_WeatherCurrent — T-017
// --------------------------------------------------------------------------

// TestFirstCallConfirm_WeatherCurrent exercises the permission confirmation
// flow for weather_current on host=api.openweathermap.org.
// AlwaysAllow: first call asks, second call (via cache) skips ask.
func TestFirstCallConfirm_WeatherCurrent(t *testing.T) {
	// Mock OWM server returning a minimal current-weather response.
	var fetchCount atomic.Int64
	owmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetchCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"weather":[{"main":"Clear","description":"clear sky","icon":"01d"}],
			"main":{"temp":22.5,"feels_like":21.0,"humidity":55,"temp_min":18.0,"temp_max":25.0},
			"wind":{"speed":3.5,"deg":225},
			"clouds":{"all":10},
			"dt":1746871200,
			"sys":{"country":"KR","sunrise":1746840000,"sunset":1746890000},
			"timezone":32400,
			"name":"Seoul",
			"cod":200
		}`))
	}))
	defer owmSrv.Close()

	confirmer := &countingConfirmer{
		decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow},
	}
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, confirmer, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{
		NetHosts: []string{"api.openweathermap.org"},
	}))

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: noopAuditWriter{},
		Clock:       func() time.Time { return time.Now() },
		Cwd:         t.TempDir(),
		RateTracker: tracker,
	}

	cfg, _ := web.LoadWeatherConfig("")
	cfg.OpenWeatherMap.APIKey = "test-perm-key"
	cfg.OpenWeatherMap.BaseURL = owmSrv.URL

	provider := web.NewOpenWeatherMapProviderForTest("test-perm-key", owmSrv.URL, deps)
	offline := web.NewDiskOfflineStore(deps.Cwd + "/weather")
	geolocator := &stubIPGeolocatorPerm{
		loc: web.Location{Lat: 37.57, Lon: 126.98, Country: "KR"},
	}

	tool := web.NewWeatherCurrentForTest(deps, cfg, provider, geolocator, offline)
	input, _ := json.Marshal(map[string]any{"lat": 37.57, "lon": 126.98})

	// First call: permission Ask must be called once.
	result1, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	resp1 := unmarshalResponse(t, result1)
	require.True(t, resp1.OK, "first call must succeed; error=%v", resp1.Error)
	assert.Equal(t, 1, confirmer.count, "Ask must be called once on first call")

	// Second call: same input → permission cached, cache hit.
	result2, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	resp2 := unmarshalResponse(t, result2)
	require.True(t, resp2.OK)
	assert.Equal(t, 1, confirmer.count, "Ask must not be called on second call (grant cached)")
	assert.True(t, resp2.Metadata.CacheHit, "second call must be a cache hit")
}

// stubIPGeolocatorPerm is a minimal IPGeolocator for permission integration test.
type stubIPGeolocatorPerm struct {
	loc web.Location
}

func (s *stubIPGeolocatorPerm) Resolve(_ context.Context) (web.Location, error) {
	return s.loc, nil
}
