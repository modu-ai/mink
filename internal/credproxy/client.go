package credproxy

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyClient is used by the agent to route requests through the credential proxy.
//
// REQ-CREDPROXY-002: Agent API calls relayed through goose-proxy
// REQ-CREDPROXY-005: If proxy not running, block request and guide user to start proxy
//
// @MX:ANCHOR: [AUTO] Agent-side proxy client
// @MX:REASON: Used by agent runtime for all credential-based requests, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-002, REQ-CREDPROXY-005, AC-CREDPROXY-04
type ProxyClient struct {
	proxyAddr string
	enabled   bool
	mu        sync.RWMutex
}

// NewProxyClient creates a new proxy client.
//
// REQ-CREDPROXY-002: Relay requests through goose-proxy
func NewProxyClient(proxyAddr string) *ProxyClient {
	if proxyAddr == "" {
		proxyAddr = "127.0.0.1:18080" // Default proxy address
	}

	return &ProxyClient{
		proxyAddr: proxyAddr,
		enabled:   true,
	}
}

// IsAvailable checks if the proxy server is available.
//
// REQ-CREDPROXY-005: If proxy not running, block request and guide user
//
// @MX:ANCHOR: [AUTO] Check proxy availability
// @MX:REASON: Critical for REQ-CREDPROXY-005 enforcement, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-005, AC-CREDPROXY-04
func (c *ProxyClient) IsAvailable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.enabled {
		return false
	}

	// Try to connect to proxy
	conn, err := net.DialTimeout("tcp", c.proxyAddr, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// HTTPClient returns an HTTP client configured to use the proxy.
//
// REQ-CREDPROXY-004: Scoped credential binding — secret only injected for matching host patterns
//
// @MX:ANCHOR: [AUTO] Create proxied HTTP client
// @MX:REASON: Entry point for agents to make proxied requests, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-002, REQ-CREDPROXY-004
func (c *ProxyClient) HTTPClient() (*http.Client, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.enabled {
		return nil, fmt.Errorf("proxy client is disabled")
	}

	// Check if proxy is available
	if !c.IsAvailable() {
		// REQ-CREDPROXY-005: Block request and guide user to start proxy
		return nil, &ProxyUnavailableError{
			ProxyAddr: c.proxyAddr,
			Message:   "credential proxy is not running. Start it with: goose proxy start",
		}
	}

	// Create HTTP client with proxy transport
	transport := &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) {
			return url.Parse("http://" + c.proxyAddr)
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return client, nil
}

// Disable disables the proxy client.
// When disabled, IsAvailable() returns false and HTTPClient() returns an error.
func (c *ProxyClient) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
}

// Enable enables the proxy client.
func (c *ProxyClient) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
}

// ProxyUnavailableError is returned when the proxy server is not available.
//
// REQ-CREDPROXY-005: If proxy not running, block request and guide user to start proxy
type ProxyUnavailableError struct {
	ProxyAddr string
	Message   string
}

// Error implements the error interface.
func (e *ProxyUnavailableError) Error() string {
	return fmt.Sprintf("proxy unavailable at %s: %s", e.ProxyAddr, e.Message)
}

// Unwrap returns the underlying error.
func (e *ProxyUnavailableError) Unwrap() error {
	return nil
}
