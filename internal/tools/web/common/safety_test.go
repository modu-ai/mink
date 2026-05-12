package common_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/mink/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponseSizeCap verifies DC-06 / AC-WEB-006: responses exceeding 10 MB
// are detected and an error is returned. Both Content-Length and chunked
// transfer scenarios are covered.
func TestResponseSizeCap(t *testing.T) {
	t.Run("ContentLength exceeds cap", func(t *testing.T) {
		// 12 MB body served with Content-Length header
		const bodySize = 12 * 1024 * 1024
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "12582912")
			w.WriteHeader(http.StatusOK)
			// Send the body in chunks to avoid allocating 12 MB in test
			chunk := bytes.Repeat([]byte("x"), 1024)
			for i := 0; i < bodySize/1024; i++ {
				_, _ = w.Write(chunk)
			}
		}))
		defer srv.Close()

		resp, err := http.Get(srv.URL) //nolint:noctx
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		_, readErr := common.LimitedRead(resp.Body)
		assert.ErrorIs(t, readErr, common.ErrResponseTooLarge)
	})

	t.Run("ChunkedTransfer exceeds cap", func(t *testing.T) {
		// 12 MB body with no Content-Length (chunked)
		const bodySize = 12 * 1024 * 1024
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Do not set Content-Length — Go's net/http will use chunked encoding
			w.WriteHeader(http.StatusOK)
			chunk := bytes.Repeat([]byte("y"), 1024)
			flusher, ok := w.(http.Flusher)
			for i := 0; i < bodySize/1024; i++ {
				_, _ = w.Write(chunk)
				if ok && i%100 == 0 {
					flusher.Flush()
				}
			}
		}))
		defer srv.Close()

		resp, err := http.Get(srv.URL) //nolint:noctx
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		_, readErr := common.LimitedRead(resp.Body)
		assert.ErrorIs(t, readErr, common.ErrResponseTooLarge)
	})

	t.Run("Body exactly at cap is accepted", func(t *testing.T) {
		// exactly 10 MB — should succeed
		const bodySize = 10 * 1024 * 1024
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			chunk := bytes.Repeat([]byte("z"), 1024)
			for i := 0; i < bodySize/1024; i++ {
				_, _ = w.Write(chunk)
			}
		}))
		defer srv.Close()

		resp, err := http.Get(srv.URL) //nolint:noctx
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		data, readErr := common.LimitedRead(resp.Body)
		require.NoError(t, readErr)
		assert.Equal(t, bodySize, len(data))
	})
}

// TestBlocklistPriority verifies DC-09 / AC-WEB-009: blocklisted hosts are
// rejected before any permission check. Glob patterns are also supported.
func TestBlocklistPriority(t *testing.T) {
	t.Run("ExactMatch blocks host", func(t *testing.T) {
		bl := common.NewBlocklist([]string{"evil.com"})
		assert.True(t, bl.IsBlocked("evil.com"))
		assert.False(t, bl.IsBlocked("good.com"))
	})

	t.Run("GlobSubdomain matches sub.evil.com", func(t *testing.T) {
		bl := common.NewBlocklist([]string{"*.evil.com"})
		assert.True(t, bl.IsBlocked("sub.evil.com"))
		assert.True(t, bl.IsBlocked("deep.sub.evil.com"))
		assert.False(t, bl.IsBlocked("evil.com")) // glob does not match apex
		assert.False(t, bl.IsBlocked("notevil.com"))
	})

	t.Run("EmptyBlocklist allows everything", func(t *testing.T) {
		bl := common.NewBlocklist(nil)
		assert.False(t, bl.IsBlocked("anything.com"))
	})

	t.Run("MultipleEntries", func(t *testing.T) {
		bl := common.NewBlocklist([]string{"blocked.org", "*.bad.net"})
		assert.True(t, bl.IsBlocked("blocked.org"))
		assert.True(t, bl.IsBlocked("sub.bad.net"))
		assert.False(t, bl.IsBlocked("good.org"))
	})
}

// TestRedirectGuard verifies that the custom CheckRedirect function used in
// http.Client enforces a configurable redirect cap.
func TestRedirectGuard(t *testing.T) {
	t.Run("ExactlyAtCap succeeds", func(t *testing.T) {
		// 5 redirects with cap=5 should NOT trigger error
		guard := common.NewRedirectGuard(5)
		// Simulate 5 requests (the initial + 5 redirects = index 4 in via slice)
		viaSlice := make([]*http.Request, 5) // len 5 means 5 redirects have occurred
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		err := guard(req, viaSlice)
		assert.NoError(t, err)
	})

	t.Run("OverCap returns ErrTooManyRedirects", func(t *testing.T) {
		guard := common.NewRedirectGuard(5)
		viaSlice := make([]*http.Request, 6) // 6 redirects exceeded cap
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		err := guard(req, viaSlice)
		assert.ErrorIs(t, err, common.ErrTooManyRedirects)
	})

	t.Run("ZeroCap stops on first redirect", func(t *testing.T) {
		guard := common.NewRedirectGuard(0)
		viaSlice := make([]*http.Request, 1)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		err := guard(req, viaSlice)
		assert.ErrorIs(t, err, common.ErrTooManyRedirects)
	})

	t.Run("MaxCap10 succeeds", func(t *testing.T) {
		guard := common.NewRedirectGuard(10)
		viaSlice := make([]*http.Request, 10)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		err := guard(req, viaSlice)
		assert.NoError(t, err)
	})

	t.Run("CapOf11 returns ErrRedirectCapTooHigh", func(t *testing.T) {
		// max_redirects=11 should be rejected at construction time (schema limit is 10)
		assert.Panics(t, func() {
			_ = common.NewRedirectGuard(11)
		})
	})

	t.Run("ReadBody returns io.ReadCloser that satisfies LimitReader", func(t *testing.T) {
		small := io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 100)))
		data, err := common.LimitedRead(small)
		require.NoError(t, err)
		assert.Len(t, data, 100)
	})
}
