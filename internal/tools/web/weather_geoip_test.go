package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGeoIPTestServer returns an httptest.Server that responds with Seoul
// coordinates for every GET request. The call counter is returned so tests
// can assert on the number of outbound calls.
func newGeoIPTestServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"latitude": 37.57,
			"longitude": 126.98,
			"city": "Seoul",
			"country_code": "KR",
			"country_name": "South Korea",
			"timezone": "Asia/Seoul",
			"error": false
		}`)
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

// TestGeoIP_Resolve_Seoul verifies that the geolocator resolves coordinates
// from the mock API response.
func TestGeoIP_Resolve_Seoul(t *testing.T) {
	srv, count := newGeoIPTestServer(t)
	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	g := web.NewIPAPIGeolocatorForTest(deps, srv.URL)

	loc, err := g.Resolve(context.Background())
	require.NoError(t, err)

	assert.InDelta(t, 37.57, loc.Lat, 0.01)
	assert.InDelta(t, 126.98, loc.Lon, 0.01)
	assert.Equal(t, "Seoul", loc.DisplayName)
	assert.Equal(t, "KR", loc.Country)
	assert.Equal(t, "Asia/Seoul", loc.Timezone)
	assert.Equal(t, int64(1), count.Load(), "exactly one API call expected")
}

// TestGeoIP_CacheHitWithin1Hour verifies that a second Resolve call within
// the 1-hour TTL window returns the cached result without hitting the API again.
func TestGeoIP_CacheHitWithin1Hour(t *testing.T) {
	srv, count := newGeoIPTestServer(t)

	now := time.Now()
	deps := &common.Deps{Clock: func() time.Time { return now }}
	g := web.NewIPAPIGeolocatorForTest(deps, srv.URL)

	// First call — populates cache.
	loc1, err := g.Resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), count.Load())

	// Second call within TTL — must hit cache, not the server.
	loc2, err := g.Resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), count.Load(), "second call must be cache hit; no extra API call expected")

	assert.Equal(t, loc1.Lat, loc2.Lat)
	assert.Equal(t, loc1.Country, loc2.Country)
}

// TestGeoIP_FetchFailed_ReturnsGeolocationError verifies that a network error
// from the mock server surfaces as ErrGeolocationFailed.
func TestGeoIP_FetchFailed_ReturnsGeolocationError(t *testing.T) {
	// Immediately-closed server simulates a connection failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return 500 to simulate server-side failure.
		w.WriteHeader(http.StatusInternalServerError)
	}))
	srv.Close() // close immediately to force connection refused

	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	g := web.NewIPAPIGeolocatorForTest(deps, srv.URL)

	_, err := g.Resolve(context.Background())
	assert.ErrorIs(t, err, web.ErrGeolocationFailed,
		"network failure must surface as ErrGeolocationFailed")
}

// TestGeoIP_TTLExpiry_RefreshesCache verifies that after the 1-hour TTL
// expires the geolocator makes a fresh API call.
func TestGeoIP_TTLExpiry_RefreshesCache(t *testing.T) {
	srv, count := newGeoIPTestServer(t)

	// Start clock before the TTL boundary.
	base := time.Now()
	var mu sync.Mutex
	clockTime := base
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return clockTime
	}

	deps := &common.Deps{Clock: clock}
	g := web.NewIPAPIGeolocatorForTest(deps, srv.URL)

	// First call — fills cache.
	_, err := g.Resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), count.Load())

	// Advance clock by 1h+1s to expire the cache entry.
	mu.Lock()
	clockTime = base.Add(time.Hour + time.Second)
	mu.Unlock()

	// Second call after TTL — must hit the server again.
	_, err = g.Resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), count.Load(), "expired cache must trigger a fresh API call")
}

// TestGeoIP_APIError_Response verifies that an API response with error:true
// surfaces as ErrGeolocationFailed.
func TestGeoIP_APIError_Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"error": true, "reason": "Reserved IP Address"}`)
	}))
	t.Cleanup(srv.Close)

	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	g := web.NewIPAPIGeolocatorForTest(deps, srv.URL)

	_, err := g.Resolve(context.Background())
	assert.ErrorIs(t, err, web.ErrGeolocationFailed)
}

// Ensure IPAPIGeolocator implements IPGeolocator interface.
var _ web.IPGeolocator = (*web.IPAPIGeolocator)(nil)
