package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/permission"
	permstore "github.com/modu-ai/mink/internal/permission/store"
	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// Shared test helpers
// --------------------------------------------------------------------------

// noopAuditWriter satisfies common.AuditWriter by discarding all events.
type noopAuditWriter struct{}

func (noopAuditWriter) Write(_ audit.AuditEvent) error { return nil }

// countingConfirmer tracks Ask call count and returns a fixed Decision.
type countingConfirmer struct {
	count    int
	decision permission.Decision
	err      error
}

func (c *countingConfirmer) Ask(_ context.Context, _ permission.PermissionRequest) (permission.Decision, error) {
	c.count++
	return c.decision, c.err
}

// newTestDeps builds a *common.Deps wired with real permission.Manager (backed
// by MemoryStore) and a fixed clock. When netHosts is non-empty the subject
// "agent:goose" is pre-registered so Manager.Check succeeds.
func newTestDeps(t *testing.T, netHosts []string) (*common.Deps, *permission.Manager, *countingConfirmer) {
	t.Helper()

	confirmer := &countingConfirmer{
		decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow},
	}

	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())

	mgr, err := permission.New(store, confirmer, nil, nil, nil)
	require.NoError(t, err)

	if len(netHosts) > 0 {
		require.NoError(t, mgr.Register("agent:goose", permission.Manifest{NetHosts: netHosts}))
	}

	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: noopAuditWriter{},
		Clock:       func() time.Time { return time.Now() },
	}
	return deps, mgr, confirmer
}

// buildInput marshals an http_fetch input map to json.RawMessage.
func buildInput(t *testing.T, m map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return b
}

// unmarshalResponse decodes ToolResult.Content into common.Response.
func unmarshalResponse(t *testing.T, result tools.ToolResult) common.Response {
	t.Helper()
	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	return resp
}

// --------------------------------------------------------------------------
// TestHTTPFetch_RedirectCap — DC-05 / AC-WEB-005
// --------------------------------------------------------------------------

// TestHTTPFetch_RedirectCap verifies redirect cap behavior:
// default 5 = success, 6 = too_many_redirects, 0 = immediate fail,
// max_redirects=10 = success, max_redirects=11 = schema_validation_failed.
func TestHTTPFetch_RedirectCap(t *testing.T) {
	t.Run("default_5_success", func(t *testing.T) {
		srv := buildRedirectChain(t, 5)
		deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
		deps.Cwd = t.TempDir()

		tool := web.NewHTTPFetch(deps)
		result, err := tool.Call(context.Background(), buildInput(t, map[string]any{"url": srv.URL + "/r0"}))
		require.NoError(t, err)

		resp := unmarshalResponse(t, result)
		assert.True(t, resp.OK, "5 redirects with default max_redirects=5 must succeed; error=%v", resp.Error)
	})

	t.Run("6_redirects_fail", func(t *testing.T) {
		srv := buildRedirectChain(t, 6)
		deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
		deps.Cwd = t.TempDir()

		tool := web.NewHTTPFetch(deps)
		result, err := tool.Call(context.Background(), buildInput(t, map[string]any{"url": srv.URL + "/r0"}))
		require.NoError(t, err)

		resp := unmarshalResponse(t, result)
		assert.False(t, resp.OK)
		require.NotNil(t, resp.Error)
		assert.Equal(t, "too_many_redirects", resp.Error.Code)
	})

	t.Run("max_redirects_0_immediate_fail", func(t *testing.T) {
		srv := buildRedirectChain(t, 1)
		deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
		deps.Cwd = t.TempDir()

		tool := web.NewHTTPFetch(deps)
		result, err := tool.Call(context.Background(), buildInput(t, map[string]any{
			"url":           srv.URL + "/r0",
			"max_redirects": 0,
		}))
		require.NoError(t, err)

		resp := unmarshalResponse(t, result)
		assert.False(t, resp.OK)
		require.NotNil(t, resp.Error)
		assert.Equal(t, "too_many_redirects", resp.Error.Code)
	})

	t.Run("max10_success", func(t *testing.T) {
		srv := buildRedirectChain(t, 10)
		deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
		deps.Cwd = t.TempDir()

		tool := web.NewHTTPFetch(deps)
		result, err := tool.Call(context.Background(), buildInput(t, map[string]any{
			"url":           srv.URL + "/r0",
			"max_redirects": 10,
		}))
		require.NoError(t, err)

		resp := unmarshalResponse(t, result)
		assert.True(t, resp.OK, "10 redirects with max_redirects=10 must succeed; error=%v", resp.Error)
	})

	t.Run("schema_fail_max_11", func(t *testing.T) {
		// max_redirects=11 exceeds the schema maximum (10).
		// Must be tested through Executor so schema validation fires.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
		deps.Cwd = t.TempDir()

		web.ClearWebToolsForTest()
		defer web.RestoreWebToolsForTest()
		web.RegisterWebTool(web.NewHTTPFetch(deps))

		reg := tools.NewRegistry(web.WithWeb())
		exec := tools.NewExecutor(tools.ExecutorConfig{Registry: reg})

		result := exec.Run(context.Background(), tools.ExecRequest{
			ToolName: "http_fetch",
			Input:    buildInput(t, map[string]any{"url": srv.URL, "max_redirects": 11}),
		})
		assert.True(t, result.IsError)
		assert.Contains(t, string(result.Content), "schema_validation_failed")
	})
}

// buildRedirectChain creates a test server with n sequential redirects ending
// at a 200 OK response. Paths: /r0 → /r1 → … → /r{n} (200 OK).
func buildRedirectChain(t *testing.T, n int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for i := 0; i < n; i++ {
		i := i
		mux.HandleFunc(fmt.Sprintf("/r%d", i), func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fmt.Sprintf("/r%d", i+1), http.StatusFound)
		})
	}
	mux.HandleFunc(fmt.Sprintf("/r%d", n), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --------------------------------------------------------------------------
// TestHTTPFetch_MethodAllowlist — DC-10 / AC-WEB-010
// --------------------------------------------------------------------------

// TestHTTPFetch_MethodAllowlist verifies that only GET and HEAD are schema-valid.
// POST/PUT/DELETE/PATCH must be rejected by Executor schema validation.
func TestHTTPFetch_MethodAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	t.Cleanup(srv.Close)

	deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
	deps.Cwd = t.TempDir()

	web.ClearWebToolsForTest()
	defer web.RestoreWebToolsForTest()
	web.RegisterWebTool(web.NewHTTPFetch(deps))

	reg := tools.NewRegistry(web.WithWeb())
	exec := tools.NewExecutor(tools.ExecutorConfig{Registry: reg})

	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		method := method
		t.Run("reject_"+method, func(t *testing.T) {
			result := exec.Run(context.Background(), tools.ExecRequest{
				ToolName: "http_fetch",
				Input:    buildInput(t, map[string]any{"url": srv.URL, "method": method}),
			})
			assert.True(t, result.IsError, "method %s should fail", method)
			assert.Contains(t, string(result.Content), "schema_validation_failed",
				"method %s should produce schema_validation_failed", method)
		})
	}

	for _, method := range []string{"GET", "HEAD"} {
		method := method
		t.Run("allow_"+method, func(t *testing.T) {
			result := exec.Run(context.Background(), tools.ExecRequest{
				ToolName: "http_fetch",
				Input:    buildInput(t, map[string]any{"url": srv.URL, "method": method}),
			})
			assert.NotContains(t, string(result.Content), "schema_validation_failed",
				"method %s must not produce schema_validation_failed", method)
		})
	}
}

// --------------------------------------------------------------------------
// TestHTTPFetch_BlocklistPriority — DC-09 / AC-WEB-009
// --------------------------------------------------------------------------

// TestHTTPFetch_BlocklistPriority verifies that blocklisted hosts are rejected
// before permission check (Confirmer.Ask must not be called).
func TestHTTPFetch_BlocklistPriority(t *testing.T) {
	var fetchCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetchCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	host := srv.Listener.Addr().String()
	// Blocklist compares plain hostnames (port stripped before lookup), so
	// register the hostname only — registering "host:port" would silently
	// fail to match after the bypass-fix in extractURLHost call sites.
	hostname, _, splitErr := net.SplitHostPort(host)
	require.NoError(t, splitErr)
	deps, _, confirmer := newTestDeps(t, []string{host})
	deps.Cwd = t.TempDir()
	deps.Blocklist = common.NewBlocklist([]string{hostname})

	tool := web.NewHTTPFetch(deps)
	result, err := tool.Call(context.Background(), buildInput(t, map[string]any{
		"url": "http://" + host + "/path",
	}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "host_blocked", resp.Error.Code)
	assert.Equal(t, 0, confirmer.count, "Confirmer.Ask must not be called for blocklisted host")
	assert.Equal(t, int64(0), fetchCount.Load(), "target endpoint must not be reached for blocklisted host")
}

// --------------------------------------------------------------------------
// TestHTTPFetch_SizeLimit — DC-06 / AC-WEB-006
// --------------------------------------------------------------------------

// TestHTTPFetch_SizeLimit verifies that a 12 MB body returns response_too_large.
func TestHTTPFetch_SizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		// Stream 12 MB of zeros.
		const bodySize = 12 * 1024 * 1024
		_, _ = io.Copy(w, io.LimitReader(zeroReader{}, bodySize))
	}))
	t.Cleanup(srv.Close)

	deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
	deps.Cwd = t.TempDir()

	tool := web.NewHTTPFetch(deps)
	result, err := tool.Call(context.Background(), buildInput(t, map[string]any{"url": srv.URL}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "response_too_large", resp.Error.Code)
}

// zeroReader produces an infinite stream of zero bytes.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// --------------------------------------------------------------------------
// TestHTTPFetch_RobotsDisallow — DC-04 / AC-WEB-004
// --------------------------------------------------------------------------

// TestHTTPFetch_RobotsDisallow verifies that a path disallowed by robots.txt is
// rejected before any data fetch occurs.
func TestHTTPFetch_RobotsDisallow(t *testing.T) {
	var dataFetchCount atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintln(w, "User-agent: *")
			_, _ = fmt.Fprintln(w, "Disallow: /private")
			return
		}
		dataFetchCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data`))
	}))
	t.Cleanup(srv.Close)

	deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
	deps.Cwd = t.TempDir()

	// Inject a real RobotsChecker so robots.txt enforcement is active.
	robotsChecker, err := common.NewRobotsChecker(time.Now)
	require.NoError(t, err)
	deps.RobotsChecker = robotsChecker

	tool := web.NewHTTPFetch(deps)
	result, callErr := tool.Call(context.Background(), buildInput(t, map[string]any{
		"url": srv.URL + "/private/page",
	}))
	require.NoError(t, callErr)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "robots_disallow", resp.Error.Code)
	assert.Equal(t, int64(0), dataFetchCount.Load(), "data endpoint must not be fetched after robots_disallow")
}

// --------------------------------------------------------------------------
// TestHTTPFetch_StandardResponseShape — DC-11 / AC-WEB-012
// --------------------------------------------------------------------------

// TestHTTPFetch_StandardResponseShape verifies that success and failure responses
// both unmarshal to the exact {ok, data|error, metadata} top-level structure.
func TestHTTPFetch_StandardResponseShape(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`hello world`))
		}))
		t.Cleanup(srv.Close)

		deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
		deps.Cwd = t.TempDir()

		tool := web.NewHTTPFetch(deps)
		result, err := tool.Call(context.Background(), buildInput(t, map[string]any{"url": srv.URL}))
		require.NoError(t, err)

		var resp common.Response
		require.NoError(t, json.Unmarshal(result.Content, &resp))
		assert.True(t, resp.OK)
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Data)

		raw := map[string]json.RawMessage{}
		require.NoError(t, json.Unmarshal(result.Content, &raw))
		assert.Contains(t, raw, "ok")
		assert.Contains(t, raw, "data")
		assert.Contains(t, raw, "metadata")
		assert.NotContains(t, raw, "error")
	})

	t.Run("failure_host_blocked", func(t *testing.T) {
		deps, _, _ := newTestDeps(t, nil)
		deps.Cwd = t.TempDir()
		deps.Blocklist = common.NewBlocklist([]string{"blocked.example.com"})

		tool := web.NewHTTPFetch(deps)
		result, err := tool.Call(context.Background(), buildInput(t, map[string]any{
			"url": "http://blocked.example.com/x",
		}))
		require.NoError(t, err)

		var resp common.Response
		require.NoError(t, json.Unmarshal(result.Content, &resp))
		assert.False(t, resp.OK)
		require.NotNil(t, resp.Error)
		assert.NotEmpty(t, resp.Error.Code)
		assert.NotEmpty(t, resp.Error.Message)

		raw := map[string]json.RawMessage{}
		require.NoError(t, json.Unmarshal(result.Content, &raw))
		assert.Contains(t, raw, "ok")
		assert.Contains(t, raw, "error")
		assert.Contains(t, raw, "metadata")
		assert.NotContains(t, raw, "data")
	})
}

// --------------------------------------------------------------------------
// TestHTTPFetch_RegisterBeforeCheck — DC-14
// --------------------------------------------------------------------------

// TestHTTPFetch_RegisterBeforeCheck verifies that calling Tool.Call without a
// prior Manager.Register results in a permission failure (ErrSubjectNotReady path).
func TestHTTPFetch_RegisterBeforeCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	t.Cleanup(srv.Close)

	// Build deps with a real manager but do NOT register any subject.
	confirmer := &countingConfirmer{
		decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow},
	}
	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, confirmer, nil, nil, nil)
	require.NoError(t, err)

	// Deliberately omit mgr.Register so "agent:goose" is unknown.
	deps := &common.Deps{
		PermMgr:     mgr,
		AuditWriter: noopAuditWriter{},
		Cwd:         t.TempDir(),
		Clock:       func() time.Time { return time.Now() },
	}

	tool := web.NewHTTPFetch(deps)
	result, err := tool.Call(context.Background(), buildInput(t, map[string]any{"url": srv.URL}))
	require.NoError(t, err)

	resp := unmarshalResponse(t, result)
	assert.False(t, resp.OK, "must fail when subject is not registered")
	require.NotNil(t, resp.Error)
	// Error code must indicate permission failure due to unregistered subject.
	assert.True(t,
		resp.Error.Code == "permission_denied" ||
			resp.Error.Code == "subject_not_ready" ||
			strings.Contains(resp.Error.Message, "not registered"),
		"expected permission failure, got code=%q message=%q", resp.Error.Code, resp.Error.Message,
	)
}
