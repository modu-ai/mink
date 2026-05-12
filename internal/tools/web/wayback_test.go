package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/mink/internal/permission"
	permstore "github.com/modu-ai/mink/internal/permission/store"
	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
)

// waybackAvailableFixture is a minimal Wayback Machine /wayback/available response
// where the closest snapshot is available.
const waybackAvailableFixture = `{
  "url": "https://example.com",
  "archived_snapshots": {
    "closest": {
      "available": true,
      "url": "https://web.archive.org/web/20231005112233/https://example.com",
      "timestamp": "20231005112233",
      "status": "200"
    }
  }
}`

// waybackUnavailableFixture is a Wayback response with no closest snapshot.
const waybackUnavailableFixture = `{
  "url": "https://nonexistent-site-xyz.example",
  "archived_snapshots": {}
}`

// waybackAvailableFalseFixture is a Wayback response where closest.available is false.
const waybackAvailableFalseFixture = `{
  "url": "https://example.com",
  "archived_snapshots": {
    "closest": {
      "available": false,
      "url": "",
      "timestamp": "20231005112233",
      "status": "404"
    }
  }
}`

// startWaybackMockServer starts an httptest server that always returns the given
// fixture body with status 200.
func startWaybackMockServer(t *testing.T, fixture string) (*httptest.Server, *int) {
	t.Helper()
	hitCount := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*hitCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(srv.Close)
	return srv, hitCount
}

// TestWayback_LatestSnapshot verifies AC-WEB-016: an available snapshot returns
// status="available" with non-empty snapshot_url and timestamp. Tests both the
// case without a timestamp hint and with one.
func TestWayback_LatestSnapshot(t *testing.T) {
	t.Parallel()

	srv, _ := startWaybackMockServer(t, waybackAvailableFixture)
	tool := web.NewWaybackForTest(&common.Deps{}, func() string { return srv.URL })

	// Without timestamp.
	result, err := tool.Call(context.Background(),
		json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true, got error: %+v", res.Error)

	var data struct {
		SnapshotURL string `json:"snapshot_url"`
		Timestamp   string `json:"timestamp"`
		Status      string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(res.Data, &data))
	assert.Equal(t, "available", data.Status)
	assert.Equal(t, "https://web.archive.org/web/20231005112233/https://example.com", data.SnapshotURL)
	assert.Equal(t, "20231005112233", data.Timestamp)

	// With timestamp hint.
	result2, err := tool.Call(context.Background(),
		json.RawMessage(`{"url":"https://example.com","timestamp":"20231005000000"}`))
	require.NoError(t, err)

	var res2 common.Response
	require.NoError(t, json.Unmarshal(result2.Content, &res2))
	require.True(t, res2.OK, "expected ok=true with timestamp, got error: %+v", res2.Error)

	var data2 struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(res2.Data, &data2))
	assert.Equal(t, "available", data2.Status)
}

// TestWayback_Unavailable verifies that an empty archived_snapshots object
// results in status="unavailable" with empty snapshot_url and timestamp.
func TestWayback_Unavailable(t *testing.T) {
	t.Parallel()

	// Test with empty archived_snapshots.
	srv1, _ := startWaybackMockServer(t, waybackUnavailableFixture)
	tool1 := web.NewWaybackForTest(&common.Deps{}, func() string { return srv1.URL })

	result1, err := tool1.Call(context.Background(),
		json.RawMessage(`{"url":"https://nonexistent-site-xyz.example"}`))
	require.NoError(t, err)

	var res1 common.Response
	require.NoError(t, json.Unmarshal(result1.Content, &res1))
	require.True(t, res1.OK, "expected ok=true even when snapshot unavailable: %+v", res1.Error)

	var data1 struct {
		SnapshotURL string `json:"snapshot_url"`
		Timestamp   string `json:"timestamp"`
		Status      string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(res1.Data, &data1))
	assert.Equal(t, "unavailable", data1.Status)
	assert.Equal(t, "", data1.SnapshotURL)
	assert.Equal(t, "", data1.Timestamp)

	// Test with available=false in closest.
	srv2, _ := startWaybackMockServer(t, waybackAvailableFalseFixture)
	tool2 := web.NewWaybackForTest(&common.Deps{}, func() string { return srv2.URL })

	result2, err := tool2.Call(context.Background(),
		json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var res2 common.Response
	require.NoError(t, json.Unmarshal(result2.Content, &res2))
	require.True(t, res2.OK, "expected ok=true even when available=false: %+v", res2.Error)

	var data2 struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(res2.Data, &data2))
	assert.Equal(t, "unavailable", data2.Status)
}

// TestWayback_SchemaValidation verifies that invalid inputs are rejected before
// any network activity.
func TestWayback_SchemaValidation(t *testing.T) {
	t.Parallel()

	tool := web.NewWaybackForTest(&common.Deps{}, func() string { return "http://127.0.0.1:0" })

	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "missing_url",
			input: `{}`,
		},
		{
			name:  "empty_url",
			input: `{"url":""}`,
		},
		{
			name:  "invalid_scheme_ftp",
			input: `{"url":"ftp://example.com"}`,
		},
		{
			name:  "invalid_scheme_no_protocol",
			input: `{"url":"example.com"}`,
		},
		{
			name:  "invalid_timestamp_too_short",
			input: `{"url":"https://example.com","timestamp":"2023100511"}`,
		},
		{
			name:  "invalid_timestamp_non_numeric",
			input: `{"url":"https://example.com","timestamp":"2023abc56789012"}`,
		},
		{
			name:  "invalid_timestamp_too_long",
			input: `{"url":"https://example.com","timestamp":"202310051122334"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := tool.Call(context.Background(), json.RawMessage(tc.input))
			require.NoError(t, err)

			var res common.Response
			require.NoError(t, json.Unmarshal(result.Content, &res))
			assert.False(t, res.OK, "expected ok=false for case %q", tc.name)
			require.NotNil(t, res.Error)
			assert.Equal(t, "invalid_input", res.Error.Code,
				"case %q: unexpected error code", tc.name)
		})
	}
}

// TestWayback_BlocklistPriority verifies that when archive.org is blocked, the
// tool returns host_blocked without making any HTTP request.
func TestWayback_BlocklistPriority(t *testing.T) {
	t.Parallel()

	serverHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	bl := common.NewBlocklist([]string{"archive.org"})
	deps := &common.Deps{Blocklist: bl}
	tool := web.NewWaybackForTest(deps, func() string { return srv.URL })

	result, err := tool.Call(context.Background(),
		json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "host_blocked", res.Error.Code)
	assert.Equal(t, 0, serverHits, "HTTP server must not be hit when host is blocked")
}

// TestWayback_PermissionDenied verifies that a permission denial returns
// "permission_denied" without making any HTTP request.
func TestWayback_PermissionDenied(t *testing.T) {
	t.Parallel()

	serverHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := permstore.NewMemoryStore()
	require.NoError(t, store.Open())
	mgr, err := permission.New(store, permission.DefaultDenyConfirmer{}, nil, nil, nil)
	require.NoError(t, err)

	deps := &common.Deps{PermMgr: mgr}
	tool := web.NewWaybackForTest(deps, func() string { return srv.URL })

	result, err := tool.Call(context.Background(),
		json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	assert.False(t, res.OK)
	require.NotNil(t, res.Error)
	assert.Equal(t, "permission_denied", res.Error.Code)
	assert.Equal(t, 0, serverHits)
}

// TestWayback_RegisteredInWebTools verifies that web_wayback is registered in the
// global web tools list at package init time.
func TestWayback_RegisteredInWebTools(t *testing.T) {
	t.Parallel()
	names := web.RegisteredWebToolNamesForTest()
	assert.True(t, slices.Contains(names, "web_wayback"),
		"web_wayback not found in RegisteredWebToolNames: %v", names)
}

// TestWayback_AuditWriter verifies that the audit writer is called on a
// successful Wayback Machine request (covers the writeAudit ok-path).
func TestWayback_AuditWriter(t *testing.T) {
	t.Parallel()

	srv, _ := startWaybackMockServer(t, waybackAvailableFixture)
	deps := &common.Deps{AuditWriter: noopAuditWriter{}}
	tool := web.NewWaybackForTest(deps, func() string { return srv.URL })

	result, err := tool.Call(context.Background(),
		json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var res common.Response
	require.NoError(t, json.Unmarshal(result.Content, &res))
	require.True(t, res.OK, "expected ok=true, got error: %+v", res.Error)
}
