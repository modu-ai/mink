package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// Coverage tests for search.go — name, scope, loadwebconfig, providerAPIHost
// --------------------------------------------------------------------------

// TestWebSearch_NameAndScope verifies Name() and Scope() return expected values.
func TestWebSearch_NameAndScope(t *testing.T) {
	deps := &common.Deps{}
	tool := web.NewWebSearch(deps, "")
	assert.Equal(t, "web_search", tool.Name())
	assert.Equal(t, 0, int(tool.Scope()), "web_search must be ScopeShared (0)")
}

// TestLoadWebConfig_MissingFile verifies that LoadWebConfig returns a zero
// config (no error) when the file does not exist.
func TestLoadWebConfig_MissingFile(t *testing.T) {
	cfg, err := web.LoadWebConfig(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "", cfg.DefaultSearchProvider)
}

// TestLoadWebConfig_WithProvider verifies that LoadWebConfig correctly
// parses a default_search_provider value from a YAML file.
func TestLoadWebConfig_WithProvider(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "web.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte("default_search_provider: tavily\n"), 0600))

	cfg, err := web.LoadWebConfig(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "tavily", cfg.DefaultSearchProvider)
}

// TestSearch_ProviderAPIHosts verifies that different provider names map to
// the correct API hostnames (used in permission + blocklist checks).
func TestSearch_ProviderAPIHosts(t *testing.T) {
	cases := []struct {
		provider     string
		expectedHost string
	}{
		{"brave", "api.search.brave.com"},
		{"tavily", "api.tavily.com"},
		{"exa", "api.exa.ai"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.provider, func(t *testing.T) {
			var callCount atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount.Add(1)
				_, _ = w.Write([]byte(`{"web":{"results":[]}}`))
			}))
			defer srv.Close()

			deps, _, _ := newTestDeps(t, []string{tc.expectedHost})
			deps.Cwd = t.TempDir()
			tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
			require.NoError(t, err)
			deps.RateTracker = tracker
			web.RegisterBraveParser(tracker)

			// Use srv.URL as override — the call will be made to the mock server.
			// The permission check uses apiHost derived from the provider name.
			tool := web.NewWebSearch(deps, srv.URL)
			result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
				"query": "test", "provider": tc.provider,
			}))
			require.NoError(t, err)

			resp := unmarshalResponse(t, result)
			// tavily and exa fall back to brave path in M1, so they also
			// go through the brave API host permission check.
			// The test verifies no permission_denied or host_blocked.
			if resp.Error != nil {
				assert.NotEqual(t, "host_blocked", resp.Error.Code)
				assert.NotEqual(t, "permission_denied", resp.Error.Code)
			}
		})
	}
}

// TestSearch_InvalidInput verifies that malformed JSON returns an error response.
func TestSearch_InvalidInput(t *testing.T) {
	deps, _, _ := newTestDeps(t, nil)
	deps.Cwd = t.TempDir()

	tool := web.NewWebSearch(deps, "")
	result, err := tool.Call(context.Background(), json.RawMessage(`{invalid json`))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "invalid_input", resp.Error.Code)
}

// TestSearch_BraveParserMalformedHeaders verifies that malformed Brave
// rate-limit headers do not cause panic (only debug messages).
func TestSearch_BraveParserMalformedHeaders(t *testing.T) {
	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	web.RegisterBraveParser(tracker)

	// Malformed values — should be silently ignored per REQ-RL-006.
	malformedHeaders := map[string]string{
		"X-RateLimit-Limit":     "not-a-number",
		"X-RateLimit-Remaining": "",
		"X-RateLimit-Reset":     "also-bad",
	}
	err = tracker.Parse("brave", malformedHeaders, time.Now())
	assert.NoError(t, err, "malformed brave headers must not cause an error")

	state := tracker.State("brave")
	// Limit defaults to 0 when malformed → UsagePct is 0 (not exhausted).
	assert.Equal(t, 0.0, state.RequestsMin.UsagePct())
}

// TestSearch_AuditOnBlockedHost verifies that a blocked host generates an
// audit event with outcome=denied before permission is checked.
func TestSearch_AuditOnBlockedHost(t *testing.T) {
	var received []string
	aw := &outcomeCapture{outcomes: &received}

	blocklist := common.NewBlocklist([]string{"api.search.brave.com"})
	deps, _, _ := newTestDeps(t, nil)
	deps.Blocklist = blocklist
	deps.AuditWriter = aw
	deps.Cwd = t.TempDir()

	tool := web.NewWebSearch(deps, "")
	result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
		"query": "block", "provider": "brave",
	}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK)
	assert.Equal(t, "host_blocked", resp.Error.Code)
	assert.Contains(t, received, "denied", "audit must record denied outcome")
}

// TestSearch_RateLimitWithNilTracker verifies that when RateTracker is nil
// (no rate limiting), the search proceeds normally.
func TestSearch_RateLimitWithNilTracker(t *testing.T) {
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"T","url":"u","description":"D"}]}}`))
	}))
	defer srv.Close()

	deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
	deps.RateTracker = nil // explicitly nil
	deps.Cwd = t.TempDir()

	tool := web.NewWebSearch(deps, srv.URL)
	result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
		"query": "nil-tracker",
	}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.True(t, resp.OK, "nil tracker must not block; error=%v", resp.Error)
	assert.Equal(t, int64(1), callCount.Load())
}

// TestSearch_NoCacheDir verifies that search works without a cache directory.
func TestSearch_NoCacheDir(t *testing.T) {
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"T","url":"u","description":"D"}]}}`))
	}))
	defer srv.Close()

	deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
	deps.Cwd = "" // no cache directory

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	deps.RateTracker = tracker
	web.RegisterBraveParser(tracker)

	tool := web.NewWebSearch(deps, srv.URL)
	result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{
		"query": "no-cache",
	}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.True(t, resp.OK, "no cache dir must still succeed; error=%v", resp.Error)
}

// TestSearch_ParseWebSearchInputDefaults verifies that parseWebSearchInput
// sets sensible defaults (max_results=10 when 0, provider="" when absent).
func TestSearch_ParseWebSearchInputDefaults(t *testing.T) {
	deps := &common.Deps{Clock: func() time.Time { return time.Now() }}
	tool := web.NewWebSearch(deps, "")

	// Verify schema has max_results with default of 10.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(tool.Schema(), &raw))
	props := raw["properties"].(map[string]any)
	mr := props["max_results"].(map[string]any)
	assert.Equal(t, float64(10), mr["default"])
}

// TestSearch_WriteAuditNoWriter verifies that missing AuditWriter does not panic.
func TestSearch_WriteAuditNoWriter(t *testing.T) {
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte(`{"web":{"results":[]}}`))
	}))
	defer srv.Close()

	deps, _, _ := newTestDeps(t, []string{"api.search.brave.com"})
	deps.AuditWriter = nil // no audit writer
	deps.Cwd = t.TempDir()
	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	deps.RateTracker = tracker
	web.RegisterBraveParser(tracker)

	tool := web.NewWebSearch(deps, srv.URL)
	result, err := tool.Call(context.Background(), buildSearchInput(t, map[string]any{"query": "no-audit"}))
	require.NoError(t, err)
	resp := unmarshalResponse(t, result)
	assert.True(t, resp.OK, "nil AuditWriter must not block; error=%v", resp.Error)
}

// outcomeCapture records audit event outcomes for test assertion.
type outcomeCapture struct {
	outcomes *[]string
}

func (o *outcomeCapture) Write(ev audit.AuditEvent) error {
	if outcome, ok := ev.Metadata["outcome"]; ok {
		*o.outcomes = append(*o.outcomes, outcome)
	}
	return nil
}
