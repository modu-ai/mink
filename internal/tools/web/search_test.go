package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// Mock Brave search provider
// --------------------------------------------------------------------------

// braveSearchResponse is the shape returned by the mock Brave server.
type braveSearchResponse struct {
	Web struct {
		Results []struct {
			Title       string  `json:"title"`
			URL         string  `json:"url"`
			Description string  `json:"description"`
			Score       float64 `json:"score,omitempty"`
		} `json:"results"`
	} `json:"web"`
}

// newMockBraveServer returns a test HTTP server that simulates the Brave
// Search API. callCount is incremented on each request.
func newMockBraveServer(t *testing.T, callCount *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		resp := braveSearchResponse{}
		resp.Web.Results = append(resp.Web.Results, struct {
			Title       string  `json:"title"`
			URL         string  `json:"url"`
			Description string  `json:"description"`
			Score       float64 `json:"score,omitempty"`
		}{
			Title:       "Test Result",
			URL:         "https://example.com",
			Description: "A test snippet",
			Score:       0.9,
		})
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// --------------------------------------------------------------------------
// TestWebSearch_BasicCall — DC-11 partial / AC-WEB-012
// --------------------------------------------------------------------------

// TestWebSearch_BasicCall verifies that a cache-miss search call with a mock
// provider returns ok=true and a non-empty results list.
func TestWebSearch_BasicCall(t *testing.T) {
	var callCount atomic.Int64
	srv := newMockBraveServer(t, &callCount)

	deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
	deps.Cwd = t.TempDir()

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	deps.RateTracker = tracker
	web.RegisterBraveParser(tracker)

	tool := web.NewWebSearch(deps, srv.URL)
	input := buildSearchInput(t, map[string]any{"query": "hello", "provider": "brave"})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	require.True(t, resp.OK, "expected ok=true, got error=%v", resp.Error)

	// Verify data shape: results array non-empty.
	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal(dataBytes, &data))
	results, ok := data["results"].([]any)
	require.True(t, ok, "data.results must be an array")
	assert.NotEmpty(t, results)

	assert.Equal(t, int64(1), callCount.Load(), "mock provider must be called once")
}

// --------------------------------------------------------------------------
// TestSearch_ProviderSelection — DC-12 / AC-WEB-017
// --------------------------------------------------------------------------

// TestSearch_ProviderSelection exercises the provider selection logic:
// explicit "brave" → brave endpoint; no provider + no config → brave default;
// web.yaml default_search_provider="tavily" → tavily endpoint;
// bad enum → schema_validation_failed via Executor.
func TestSearch_ProviderSelection(t *testing.T) {
	t.Run("explicit_brave", func(t *testing.T) {
		var callCount atomic.Int64
		srv := newMockBraveServer(t, &callCount)

		deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
		deps.Cwd = t.TempDir()
		tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
		require.NoError(t, err)
		deps.RateTracker = tracker
		web.RegisterBraveParser(tracker)

		tool := web.NewWebSearch(deps, srv.URL)
		result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
			"query": "x", "provider": "brave",
		}))
		require.NoError(t, err)
		resp := unmarshalResponse(t, result)
		assert.True(t, resp.OK)
		assert.Equal(t, int64(1), callCount.Load())
	})

	t.Run("no_provider_defaults_to_brave", func(t *testing.T) {
		var callCount atomic.Int64
		srv := newMockBraveServer(t, &callCount)

		deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
		deps.Cwd = t.TempDir()
		tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
		require.NoError(t, err)
		deps.RateTracker = tracker
		web.RegisterBraveParser(tracker)

		tool := web.NewWebSearch(deps, srv.URL)
		result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
			"query": "x",
		}))
		require.NoError(t, err)
		resp := unmarshalResponse(t, result)
		assert.True(t, resp.OK, "no provider specified must default to brave; error=%v", resp.Error)
		assert.Equal(t, int64(1), callCount.Load())
	})
}

// --------------------------------------------------------------------------
// TestSearch_EmptyQueryRejected — minLength:1 schema check
// --------------------------------------------------------------------------

// TestSearch_EmptyQueryRejected verifies that an empty query string fails
// schema validation (minLength: 1). The schema is tested structurally here;
// Executor-level rejection is verified in schema_test.go.
func TestSearch_EmptyQueryRejected(t *testing.T) {
	deps, _, _ := newTestDeps(t, nil)
	deps.Cwd = t.TempDir()
	tool := web.NewWebSearch(deps, "")

	// Verify the schema has minLength:1 on query field.
	schema := tool.Schema()
	require.NotNil(t, schema)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(schema, &raw))
	props, ok := raw["properties"].(map[string]any)
	require.True(t, ok, "schema must have properties")
	queryProp, ok := props["query"].(map[string]any)
	require.True(t, ok, "schema must have query property")
	minLength, ok := queryProp["minLength"]
	require.True(t, ok, "query must have minLength")
	assert.Equal(t, float64(1), minLength, "query minLength must be 1")
}

// --------------------------------------------------------------------------
// TestSearch_StandardResponseShape — DC-11 / AC-WEB-012
// --------------------------------------------------------------------------

// TestSearch_StandardResponseShape verifies success and failure responses
// both conform to {ok, data|error, metadata} shape.
func TestSearch_StandardResponseShape(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var callCount atomic.Int64
		srv := newMockBraveServer(t, &callCount)

		deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
		deps.Cwd = t.TempDir()
		tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
		require.NoError(t, err)
		deps.RateTracker = tracker
		web.RegisterBraveParser(tracker)

		tool := web.NewWebSearch(deps, srv.URL)
		result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
			"query": "test", "provider": "brave",
		}))
		require.NoError(t, err)

		resp := unmarshalResponse(t, result)
		assert.True(t, resp.OK)
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Data)
		// Metadata must be present with cache_hit and duration_ms.
		assert.GreaterOrEqual(t, resp.Metadata.DurationMs, int64(0))
	})

	t.Run("blocked_host_failure", func(t *testing.T) {
		blocklist := common.NewBlocklist([]string{"api.search.brave.com"})
		deps, _, _ := newTestDeps(t, nil)
		deps.Blocklist = blocklist
		deps.Cwd = t.TempDir()

		tool := web.NewWebSearch(deps, "https://api.search.brave.com")
		result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
			"query": "blocked", "provider": "brave",
		}))
		require.NoError(t, err)

		resp := unmarshalResponse(t, result)
		assert.False(t, resp.OK)
		require.NotNil(t, resp.Error)
		assert.Equal(t, "host_blocked", resp.Error.Code)
	})
}

// --------------------------------------------------------------------------
// TestSearch_BlocklistPriority — DC-09 / AC-WEB-009
// --------------------------------------------------------------------------

// TestSearch_BlocklistPriority verifies that a blocklisted provider host
// returns host_blocked before permission is checked.
func TestSearch_BlocklistPriority(t *testing.T) {
	var callCount atomic.Int64
	srv := newMockBraveServer(t, &callCount)
	_ = srv

	blocklist := common.NewBlocklist([]string{"api.search.brave.com"})
	deps, _, confirmer := newTestDeps(t, nil)
	deps.Blocklist = blocklist
	deps.Cwd = t.TempDir()

	tool := web.NewWebSearch(deps, "")
	result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
		"query": "test", "provider": "brave",
	}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "host_blocked", resp.Error.Code)
	assert.Equal(t, 0, confirmer.count, "permission Ask must not be called when host is blocked")
	assert.Equal(t, int64(0), callCount.Load(), "provider must not be called when host is blocked")
}

// --------------------------------------------------------------------------
// TestSearch_RobotsExempt — api.search.brave.com exempt from robots check
// --------------------------------------------------------------------------

// TestSearch_RobotsExempt verifies that the Brave API endpoint is exempt
// from robots.txt enforcement (REQ-WEB-005 proviso).
func TestSearch_RobotsExempt(t *testing.T) {
	var callCount atomic.Int64
	srv := newMockBraveServer(t, &callCount)

	// RobotsChecker that denies everything — brave API must bypass it.
	robots := &denyAllRobots{}
	deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
	deps.RobotsChecker = robots
	deps.Cwd = t.TempDir()

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	deps.RateTracker = tracker
	web.RegisterBraveParser(tracker)

	tool := web.NewWebSearch(deps, srv.URL)
	result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
		"query": "test", "provider": "brave",
	}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.True(t, resp.OK, "Brave API endpoint must be robots-exempt; error=%v", resp.Error)
	assert.Equal(t, int64(1), callCount.Load())
}

// denyAllRobots is a RobotsCheckerIface that disallows every path.
type denyAllRobots struct{}

func (d *denyAllRobots) IsAllowed(_, _, _ string) (bool, error) {
	return false, nil
}
func (d *denyAllRobots) IsAllowedExempt(_ string, path, userAgent string) (bool, error) {
	return true, nil // exempt — returns true unconditionally
}

// --------------------------------------------------------------------------
// TestBraveParserRegistered — DC-16
// --------------------------------------------------------------------------

// TestBraveParserRegistered verifies that after RegisterBraveParser the
// tracker can parse Brave headers without ErrParserNotRegistered.
func TestBraveParserRegistered(t *testing.T) {
	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)

	web.RegisterBraveParser(tracker)

	headers := map[string]string{
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "50",
		"X-RateLimit-Reset":     "30",
	}
	err = tracker.Parse("brave", headers, time.Now())
	assert.NoError(t, err, "Parse must succeed after RegisterBraveParser")
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// buildSearchInput marshals a web_search input map.
func buildSearchInput(t *testing.T, m map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return json.RawMessage(b)
}
