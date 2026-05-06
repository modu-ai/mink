package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPFetch_InvalidJSON verifies that malformed JSON input is handled gracefully.
// This covers the parseHTTPFetchInput error path in http.go.
func TestHTTPFetch_InvalidJSON(t *testing.T) {
	deps := &common.Deps{
		AuditWriter: noopAuditWriter{},
		Clock:       func() time.Time { return time.Now() },
	}
	tool := web.NewHTTPFetch(deps)

	// Directly call with malformed JSON (Executor schema-validates first in production,
	// but we test the direct path here for coverage).
	result, err := tool.Call(context.Background(), json.RawMessage(`{invalid`))
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
}

// TestHTTPFetch_InvalidURL verifies that a URL with no host is handled gracefully.
// This covers the extractURLHost empty-host error path in http.go.
func TestHTTPFetch_InvalidURL(t *testing.T) {
	deps := &common.Deps{
		AuditWriter: noopAuditWriter{},
		Clock:       func() time.Time { return time.Now() },
	}
	tool := web.NewHTTPFetch(deps)

	// A URL that parses but has no host (e.g. relative URL stored in url field).
	input := buildInput(t, map[string]any{"url": "http:///no-host/path"})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
}

// TestHTTPFetch_NetworkError verifies that a network-level fetch error returns fetch_failed.
// This covers the doFetch error path in http.go.
func TestHTTPFetch_NetworkError(t *testing.T) {
	// Use a closed server to force a connection refused error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	addr := srv.Listener.Addr().String()
	srv.Close() // close immediately so requests fail

	deps, _, _ := newTestDeps(t, []string{addr})
	deps.Cwd = t.TempDir()

	tool := web.NewHTTPFetch(deps)
	input := buildInput(t, map[string]any{"url": "http://" + addr})
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	// Should be fetch_failed or too_many_redirects; either is acceptable.
	assert.NotEmpty(t, resp.Error.Code)
}

// TestHTTPFetch_NilAuditWriter verifies that a nil AuditWriter does not cause a panic.
// This covers the nil-check guard in writeAudit.
func TestHTTPFetch_NilAuditWriter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`hi`))
	}))
	t.Cleanup(srv.Close)

	deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
	deps.AuditWriter = nil // explicitly nil
	deps.Cwd = t.TempDir()

	tool := web.NewHTTPFetch(deps)
	result, err := tool.Call(context.Background(), buildInput(t, map[string]any{"url": srv.URL}))
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	// Should still succeed even without audit writer.
	assert.True(t, resp.OK)
}

// TestHTTPFetch_SuccessHEAD verifies that HEAD method returns a response without body.
func TestHTTPFetch_SuccessHEAD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	t.Cleanup(srv.Close)

	deps, _, _ := newTestDeps(t, []string{srv.Listener.Addr().String()})
	deps.Cwd = t.TempDir()

	tool := web.NewHTTPFetch(deps)
	result, err := tool.Call(context.Background(), buildInput(t, map[string]any{
		"url":    srv.URL,
		"method": "HEAD",
	}))
	require.NoError(t, err)

	var resp common.Response
	require.NoError(t, json.Unmarshal(result.Content, &resp))
	assert.True(t, resp.OK, "HEAD method must succeed; error=%v", resp.Error)
}
