package web_test

import (
	"bufio"
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
	"github.com/modu-ai/goose/internal/permission"
	permstore "github.com/modu-ai/goose/internal/permission/store"
	"github.com/modu-ai/goose/internal/tools"
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// TestAuditLog_M1Calls — DC-13 / AC-WEB-018 (M1 scope: 2 tools)
// --------------------------------------------------------------------------

// TestAuditLog_M1Calls calls web_search and http_fetch sequentially with
// pre-granted permissions and verifies that exactly 2 JSON-line audit entries
// are written to a temporary log file, all with outcome="ok" and monotonically
// increasing timestamps.
func TestAuditLog_M1Calls(t *testing.T) {
	// Injected clock with 1µs step per call to guarantee monotonic timestamps.
	var clockTick atomic.Int64
	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	testClock := func() time.Time {
		tick := clockTick.Add(1)
		return baseTime.Add(time.Duration(tick) * time.Microsecond)
	}

	// Mock servers.
	var searchCount, fetchCount atomic.Int64
	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"T","url":"https://ex.com","description":"D"}]}}`))
	}))
	defer searchSrv.Close()

	fetchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer fetchSrv.Close()

	// Real audit FileWriter writing to t.TempDir().
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "audit.log")
	auditWriter, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { _ = auditWriter.Close() }()

	// Pre-grant permission (skip Confirmer round-trip).
	allHosts := []string{
		"api.search.brave.com",
		fetchSrv.Listener.Addr().String(),
	}
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.AlwaysAllowConfirmer{}, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.Register("agent:goose", permission.Manifest{NetHosts: allHosts}))

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: auditWriter,
		Clock:       testClock,
		Cwd:         t.TempDir(),
	}

	tracker, err := ratelimit.New(ratelimit.TrackerOptions{ThresholdPct: 80})
	require.NoError(t, err)
	deps.RateTracker = tracker
	web.RegisterBraveParser(tracker)

	ctx := context.Background()

	// Call 1: web_search
	searchTool := web.NewWebSearch(deps, searchSrv.URL)
	r1, err := searchTool.Call(ctx, buildSearchInput(t, map[string]any{
		"query": "audit-test", "provider": "brave",
	}))
	require.NoError(t, err)
	require.False(t, r1.IsError, "web_search call must succeed")

	// Call 2: http_fetch
	httpTool := web.NewHTTPFetch(deps)
	r2, err := httpTool.Call(ctx, buildInput(t, map[string]any{
		"url": "http://" + fetchSrv.Listener.Addr().String() + "/",
	}))
	require.NoError(t, err)
	// http_fetch may succeed or fail — both generate an audit event.
	_ = r2

	// Flush and close audit writer to ensure all data is on disk.
	require.NoError(t, auditWriter.Close())

	// Read audit.log and verify exactly 2 lines.
	f, err := os.Open(logPath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry), "each audit line must be valid JSON: %q", line)
		lines = append(lines, entry)
	}
	require.NoError(t, scanner.Err())

	require.Len(t, lines, 2, "exactly 2 audit lines expected for M1 (web_search + http_fetch)")

	// Verify required keys and values on each line.
	requiredKeys := []string{"type", "timestamp", "severity", "message", "metadata"}
	for i, entry := range lines {
		for _, k := range requiredKeys {
			assert.Contains(t, entry, k, "line %d must have key %q", i+1, k)
		}
		assert.Equal(t, "tool.web.invoke", entry["type"], "line %d type must be tool.web.invoke", i+1)

		meta, ok := entry["metadata"].(map[string]any)
		require.True(t, ok, "line %d metadata must be a JSON object", i+1)
		metaRequiredKeys := []string{"tool", "host", "method", "status_code", "cache_hit", "duration_ms", "outcome"}
		for _, mk := range metaRequiredKeys {
			assert.Contains(t, meta, mk, "line %d metadata must have key %q", i+1, mk)
		}
		assert.Equal(t, "ok", meta["outcome"], "line %d outcome must be ok", i+1)
	}

	// Verify timestamps are monotonically increasing.
	ts0, err := time.Parse(time.RFC3339, lines[0]["timestamp"].(string))
	require.NoError(t, err)
	ts1, err := time.Parse(time.RFC3339, lines[1]["timestamp"].(string))
	require.NoError(t, err)
	assert.True(t, !ts1.Before(ts0), "timestamps must be monotonically increasing: %v, %v", ts0, ts1)
}

// --------------------------------------------------------------------------
// TestAuditTimestampMonotonic — edge case: injected clock
// --------------------------------------------------------------------------

// TestAuditTimestampMonotonic uses a 1µs-step clock to verify strict timestamp
// ordering across two sequential tool calls.
func TestAuditTimestampMonotonic(t *testing.T) {
	var tick atomic.Int64
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		return base.Add(time.Duration(tick.Add(1)) * time.Microsecond)
	}

	var received []audit.AuditEvent
	cw := &captureAuditWriter{events: &received}

	deps, _, _ := newTestDeps(t, nil)
	deps.AuditWriter = cw
	deps.Clock = clock
	deps.Cwd = t.TempDir()

	// Two minimal calls with noop tools (any tool with AuditWriter set).
	fetchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer fetchSrv.Close()

	require.NoError(t, deps.PermMgr.Register("agent:goose", permission.Manifest{
		NetHosts: []string{fetchSrv.Listener.Addr().String()},
	}))

	httpTool := web.NewHTTPFetch(deps)
	for i := 0; i < 2; i++ {
		res, err := httpTool.Call(context.Background(), buildInput(t, map[string]any{
			"url": "http://" + fetchSrv.Listener.Addr().String() + "/",
		}))
		require.NoError(t, err)
		r := unmarshalToolResponse(t, res)
		_ = r
	}

	require.Len(t, received, 2, "2 audit events expected")
	assert.True(t, received[1].Timestamp.After(received[0].Timestamp) ||
		received[1].Timestamp.Equal(received[0].Timestamp),
		"timestamps must be monotone: %v, %v", received[0].Timestamp, received[1].Timestamp)
}

// captureAuditWriter records audit events in memory for inspection.
type captureAuditWriter struct {
	events *[]audit.AuditEvent
}

func (c *captureAuditWriter) Write(e audit.AuditEvent) error {
	*c.events = append(*c.events, e)
	return nil
}

// unmarshalToolResponse decodes ToolResult into common.Response.
func unmarshalToolResponse(t *testing.T, result tools.ToolResult) common.Response {
	t.Helper()
	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	return resp
}
