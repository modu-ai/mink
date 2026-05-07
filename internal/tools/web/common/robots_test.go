package common_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRobotsDisallow verifies DC-04 / AC-WEB-004: a host that disallows a
// path for the goose-agent User-Agent returns false from IsAllowed.
func TestRobotsDisallow(t *testing.T) {
	const robotsTxt = "User-agent: *\nDisallow: /private\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, robotsTxt)
	}))
	defer srv.Close()

	clock := func() time.Time { return time.Now() }
	checker, err := common.NewRobotsChecker(clock)
	require.NoError(t, err)

	// Host is served by the test server; we construct the robots URL manually.
	allowed, err := checker.IsAllowed(srv.URL, "/private", common.UserAgent())
	require.NoError(t, err)
	assert.False(t, allowed, "disallowed path should return false")

	allowed2, err := checker.IsAllowed(srv.URL, "/public", common.UserAgent())
	require.NoError(t, err)
	assert.True(t, allowed2, "allowed path should return true")
}

// TestRobotsExempt_SearchProvider verifies that provider API base URLs are
// exempt from robots.txt checking (they are a known search API endpoint).
func TestRobotsExempt_SearchProvider(t *testing.T) {
	clock := func() time.Time { return time.Now() }
	checker, err := common.NewRobotsChecker(clock)
	require.NoError(t, err)

	// Search API endpoints are exempted; no HTTP call should be made.
	allowed, err := checker.IsAllowedExempt("https://api.search.brave.com", "/res/v1/web/search", common.UserAgent())
	require.NoError(t, err)
	assert.True(t, allowed, "search provider API is exempt from robots.txt")
}

// TestRobotsCachePair verifies that a second call to the same host reuses the
// cached robots.txt without making another fetch.
func TestRobotsCachePair(t *testing.T) {
	var fetchCount int64
	const robotsTxt = "User-agent: *\nAllow: /\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&fetchCount, 1)
		_, _ = fmt.Fprint(w, robotsTxt)
	}))
	defer srv.Close()

	clock := func() time.Time { return time.Now() }
	checker, err := common.NewRobotsChecker(clock)
	require.NoError(t, err)

	_, err = checker.IsAllowed(srv.URL, "/any", common.UserAgent())
	require.NoError(t, err)

	_, err = checker.IsAllowed(srv.URL, "/other", common.UserAgent())
	require.NoError(t, err)

	assert.Equal(t, int64(1), atomic.LoadInt64(&fetchCount),
		"second call must use LRU cache (fetch count stays at 1)")
}

// TestRobotsNonExemptPath verifies that IsAllowedExempt falls through to
// the regular robots check for non-exempt URLs.
func TestRobotsNonExemptPath(t *testing.T) {
	const robotsTxt = "User-agent: *\nDisallow: /blocked\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, robotsTxt)
	}))
	defer srv.Close()

	clock := func() time.Time { return time.Now() }
	checker, err := common.NewRobotsChecker(clock)
	require.NoError(t, err)

	// Non-exempt URL should fall through to actual robots.txt check.
	allowed, err := checker.IsAllowedExempt(srv.URL, "/blocked", common.UserAgent())
	require.NoError(t, err)
	assert.False(t, allowed, "non-exempt URL must respect robots.txt")
}

// TestRobotsFetchError verifies that a fetch error is treated as "allow all"
// (graceful degradation).
func TestRobotsFetchError(t *testing.T) {
	clock := func() time.Time { return time.Now() }
	checker, err := common.NewRobotsChecker(clock)
	require.NoError(t, err)

	// Use an unreachable URL; the checker must not return an error.
	allowed, err := checker.IsAllowed("http://127.0.0.1:1", "/any", common.UserAgent())
	require.NoError(t, err)
	assert.True(t, allowed, "fetch error should default to allow")
}

// TestRobotsInvalidBaseURL verifies that an unparseable base URL is handled
// gracefully (treated as allow, no panic).
func TestRobotsInvalidBaseURL(t *testing.T) {
	clock := func() time.Time { return time.Now() }
	checker, err := common.NewRobotsChecker(clock)
	require.NoError(t, err)

	// ":// not a valid URL" — should not panic
	allowed, err := checker.IsAllowed("://not-valid", "/page", common.UserAgent())
	require.NoError(t, err)
	assert.True(t, allowed)
}

// TestRobotsCacheTTLExpiry verifies that a cached entry older than 24h is
// re-fetched on the next call.
func TestRobotsCacheTTLExpiry(t *testing.T) {
	var fetchCount int64
	const robotsTxt = "User-agent: *\nAllow: /\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&fetchCount, 1)
		_, _ = fmt.Fprint(w, robotsTxt)
	}))
	defer srv.Close()

	now := time.Now()
	clock := func() time.Time { return now }
	checker, err := common.NewRobotsChecker(clock)
	require.NoError(t, err)

	// Prime the cache.
	_, err = checker.IsAllowed(srv.URL, "/any", common.UserAgent())
	require.NoError(t, err)
	assert.Equal(t, int64(1), atomic.LoadInt64(&fetchCount))

	// Advance past 24h TTL.
	now = now.Add(25 * time.Hour)
	_, err = checker.IsAllowed(srv.URL, "/any", common.UserAgent())
	require.NoError(t, err)
	assert.Equal(t, int64(2), atomic.LoadInt64(&fetchCount), "expired entry should trigger re-fetch")
}

// TestRobotsSelfFetch verifies DC-15: when the robots checker fetches
// /robots.txt for a host, it must not recursively check robots.txt for that
// same path (no infinite loop). The test must complete within a short timeout.
func TestRobotsSelfFetch(t *testing.T) {
	const robotsTxt = "User-agent: *\nAllow: /\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, robotsTxt)
	}))
	defer srv.Close()

	clock := func() time.Time { return time.Now() }
	checker, err := common.NewRobotsChecker(clock)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Requesting the robots.txt path itself must not cause recursion.
		_, _ = checker.IsAllowed(srv.URL, "/robots.txt", common.UserAgent())
	}()

	select {
	case <-done:
		// completed without deadlock
	case <-time.After(100 * time.Millisecond):
		t.Fatal("TestRobotsSelfFetch: timed out — possible infinite recursion")
	}
}
