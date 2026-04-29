package credproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewProxyClient tests creating a new proxy client.
func TestNewProxyClient(t *testing.T) {
	t.Parallel()

	client := NewProxyClient("127.0.0.1:18080")
	assert.NotNil(t, client)
	assert.Equal(t, "127.0.0.1:18080", client.proxyAddr)
	assert.True(t, client.enabled)
}

// TestNewProxyClientDefaultAddr tests default proxy address.
func TestNewProxyClientDefaultAddr(t *testing.T) {
	t.Parallel()

	client := NewProxyClient("")
	assert.Equal(t, "127.0.0.1:18080", client.proxyAddr)
}

// TestProxyClientIsAvailable tests checking proxy availability.
func TestProxyClientIsAvailable(t *testing.T) {
	t.Parallel()

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Extract address from test server
	addr := server.Listener.Addr().String()

	// Create client pointing to test server
	client := NewProxyClient(addr)

	// Check availability - should be available
	assert.True(t, client.IsAvailable())
}

// TestProxyClientIsAvailableNotRunning tests checking availability when proxy is not running.
func TestProxyClientIsAvailableNotRunning(t *testing.T) {
	t.Parallel()

	// Create client pointing to non-existent proxy
	client := NewProxyClient("127.0.0.1:19999") // Port that's likely not in use

	// Check availability - should not be available
	assert.False(t, client.IsAvailable())
}

// TestProxyClientIsAvailableDisabled tests checking availability when client is disabled.
func TestProxyClientIsAvailableDisabled(t *testing.T) {
	t.Parallel()

	client := NewProxyClient("127.0.0.1:18080")
	client.Disable()

	assert.False(t, client.IsAvailable())
}

// TestProxyClientHTTPClient tests creating an HTTP client.
func TestProxyClientHTTPClient(t *testing.T) {
	t.Parallel()

	// Create a test HTTP server that acts as the proxy
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request is being proxied
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("proxied")); err != nil {
				t.Fatalf("Failed to write response: %v", err)
			}
	}))
	defer proxyServer.Close()

	addr := proxyServer.Listener.Addr().String()

	// Create client pointing to test proxy
	client := NewProxyClient(addr)

	// Create HTTP client
	httpClient, err := client.HTTPClient()
	require.NoError(t, err)
	assert.NotNil(t, httpClient)

	// Make a request through the proxy
	resp, err := httpClient.Get(proxyServer.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestProxyClientHTTPClientUnavailable tests creating HTTP client when proxy is unavailable.
func TestProxyClientHTTPClientUnavailable(t *testing.T) {
	t.Parallel()

	client := NewProxyClient("127.0.0.1:19999") // Port that's likely not in use

	// Try to create HTTP client - should fail
	httpClient, err := client.HTTPClient()
	assert.Error(t, err)
	assert.Nil(t, httpClient)

	// Verify the error type
	proxyErr, ok := err.(*ProxyUnavailableError)
	assert.True(t, ok, "error should be ProxyUnavailableError")
	assert.Contains(t, proxyErr.Error(), "proxy unavailable")
	assert.Contains(t, proxyErr.Message, "credential proxy is not running")
}

// TestProxyClientHTTPClientDisabled tests creating HTTP client when client is disabled.
func TestProxyClientHTTPClientDisabled(t *testing.T) {
	t.Parallel()

	client := NewProxyClient("127.0.0.1:18080")
	client.Disable()

	// Try to create HTTP client - should fail
	httpClient, err := client.HTTPClient()
	assert.Error(t, err)
	assert.Nil(t, httpClient)
	assert.Contains(t, err.Error(), "proxy client is disabled")
}

// TestProxyClientEnableDisable tests enabling and disabling the client.
func TestProxyClientEnableDisable(t *testing.T) {
	t.Parallel()

	client := NewProxyClient("127.0.0.1:18080")

	// Initially enabled
	assert.True(t, client.enabled)

	// Disable
	client.Disable()
	assert.False(t, client.enabled)

	// Enable
	client.Enable()
	assert.True(t, client.enabled)
}

// TestProxyUnavailableError tests the ProxyUnavailableError.
func TestProxyUnavailableError(t *testing.T) {
	t.Parallel()

	err := &ProxyUnavailableError{
		ProxyAddr: "127.0.0.1:18080",
		Message:   "test message",
	}

	assert.Equal(t, "proxy unavailable at 127.0.0.1:18080: test message", err.Error())
	assert.Nil(t, err.Unwrap())
}

// TestProxyClientProxiedRequest tests that requests are properly proxied.
func TestProxyClientProxiedRequest(t *testing.T) {
	// Cannot use t.Parallel() here because we're sharing state

	// Create a test HTTP server that acts as the proxy
	// It should receive requests and forward them
	type serverState struct {
		received bool
		mu       chan struct{}
	}
	state := &serverState{
		received: false,
		mu:       make(chan struct{}, 1),
	}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.mu <- struct{}{}
		state.received = true
		<-state.mu
		// Verify the request was received (Host header might be empty in test proxy)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("response from target")); err != nil {
				t.Fatalf("Failed to write response: %v", err)
			}
	}))
	defer proxyServer.Close()

	addr := proxyServer.Listener.Addr().String()

	// Create client pointing to test proxy
	client := NewProxyClient(addr)

	// Create HTTP client
	httpClient, err := client.HTTPClient()
	require.NoError(t, err)

	// Make a request through the proxy
	resp, err := httpClient.Get(proxyServer.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, state.received, "proxy should have received the request")
}

// TestProxyClientConcurrentRequests tests concurrent requests through the proxy.
func TestProxyClientConcurrentRequests(t *testing.T) {
	t.Parallel()

	// Create a test HTTP server
	var requestCount atomic.Int32
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
				t.Fatalf("Failed to write response: %v", err)
			}
	}))
	defer proxyServer.Close()

	addr := proxyServer.Listener.Addr().String()
	client := NewProxyClient(addr)
	httpClient, err := client.HTTPClient()
	require.NoError(t, err)

	// Launch concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			resp, _ := httpClient.Get(proxyServer.URL)
			if resp != nil {
				if err := resp.Body.Close(); err != nil {
				t.Errorf("Failed to close response body: %v", err)
			}
			}
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all requests were handled
	assert.Equal(t, int32(10), requestCount.Load())
}

// TestProxyClientTimeout tests that the HTTP client respects timeouts.
func TestProxyClientTimeout(t *testing.T) {
	t.Parallel()

	// Create a slow server
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Sleep longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer proxyServer.Close()

	addr := proxyServer.Listener.Addr().String()
	client := NewProxyClient(addr)

	// Create HTTP client with short timeout
	httpClient, err := client.HTTPClient()
	require.NoError(t, err)

	// Override timeout for this test
	if transport, ok := httpClient.Transport.(*http.Transport); ok {
		httpClient.Transport = &http.Transport{
			Proxy: transport.Proxy,
		}
	}
	httpClient.Timeout = 100 * time.Millisecond

	// Make a request - should timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", proxyServer.URL, nil)
	_, err = httpClient.Do(req)
	assert.Error(t, err)
}

// BenchmarkProxyClientHTTPClient benchmarks creating HTTP clients.
func BenchmarkProxyClientHTTPClient(b *testing.B) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer proxyServer.Close()

	addr := proxyServer.Listener.Addr().String()
	client := NewProxyClient(addr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.HTTPClient()
	}
}

// BenchmarkProxyClientRequest benchmarks proxied requests.
func BenchmarkProxyClientRequest(b *testing.B) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
				b.Fatalf("Failed to write response: %v", err)
			}
	}))
	defer proxyServer.Close()

	addr := proxyServer.Listener.Addr().String()
	client := NewProxyClient(addr)
	httpClient, _ := client.HTTPClient()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := httpClient.Get(proxyServer.URL)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
}
