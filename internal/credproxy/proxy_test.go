package credproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockAuditWriter is a mock audit writer for testing.
type mockAuditWriter struct {
	events []auditEvent
	mu     sync.Mutex
}

func newMockAuditWriter() *mockAuditWriter {
	return &mockAuditWriter{
		events: make([]auditEvent, 0),
	}
}

func (m *mockAuditWriter) Write(event auditEvent) error {
	m.mu.Lock()
	m.events = append(m.events, event)
	m.mu.Unlock()
	return nil
}

func (m *mockAuditWriter) getEvents() []auditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]auditEvent(nil), m.events...)
}

func (m *mockAuditWriter) clear() {
	m.mu.Lock()
	m.events = make([]auditEvent, 0)
	m.mu.Unlock()
}

// TestNewProxy tests creating a new proxy server.
func TestNewProxy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	logger := zap.NewNop()
	auditWriter := newMockAuditWriter()

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0", // Use :0 to get random available port
		Keyring:     keyring,
		Store:       store,
		AuditWriter: auditWriter,
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)
	assert.NotNil(t, proxy)
	assert.Equal(t, "127.0.0.1:0", proxy.cfg.ListenAddr)
	assert.False(t, proxy.IsRunning())
}

// TestNewProxyValidation tests proxy configuration validation.
func TestNewProxyValidation(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()

	tests := []struct {
		name    string
		cfg     ProxyConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing keyring",
			cfg: ProxyConfig{
				ListenAddr: "127.0.0.1:18080",
				Store:      &ProviderStore{},
				Logger:     logger,
			},
			wantErr: true,
			errMsg:  "keyring backend is required",
		},
		{
			name: "missing store",
			cfg: ProxyConfig{
				ListenAddr: "127.0.0.1:18080",
				Keyring:    &fileKeyring{},
				Logger:     logger,
			},
			wantErr: true,
			errMsg:  "provider store is required",
		},
		{
			name: "missing logger",
			cfg: ProxyConfig{
				ListenAddr: "127.0.0.1:18080",
				Keyring:    &fileKeyring{},
				Store:      &ProviderStore{},
			},
			wantErr: true,
			errMsg:  "logger is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := NewProxy(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, proxy)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, proxy)
			}
		})
	}
}

// TestProxyHealthCheck tests the health check endpoint.
func TestProxyHealthCheck(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)
	logger := zap.NewNop()

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0",
		Keyring:     keyring,
		Store:       store,
		AuditWriter: newMockAuditWriter(),
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Create test request
	req := httptest.NewRequest("GET", "/_health", nil)
	w := httptest.NewRecorder()

	// Serve the request
	proxy.createHandler().ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

// TestProxyHandleProxyNoProvider tests proxy request without matching provider.
func TestProxyHandleProxyNoProvider(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)
	logger := zap.NewNop()
	auditWriter := newMockAuditWriter()

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0",
		Keyring:     keyring,
		Store:       store,
		AuditWriter: auditWriter,
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Create test request to unknown host
	req := httptest.NewRequest("GET", "http://unknown.host.com/path", nil)
	req.Host = "unknown.host.com"
	w := httptest.NewRecorder()

	// Serve the request
	proxy.handleProxy(w, req)

	// Check response - should be forbidden
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "No credential configured for this host")
}

// TestProxyHandleProxyWithProvider tests proxy request with matching provider.
func TestProxyHandleProxyWithProvider(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	err := keyring.Store("goose", "test-key", []byte("test-secret"))
	require.NoError(t, err)

	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)
	err = store.Add("test-provider", ProviderRef{
		KeyringID:   "test-key",
		HostPattern: "api.test.com",
		Provider:    "test",
	})
	require.NoError(t, err)

	logger := zap.NewNop()
	auditWriter := newMockAuditWriter()

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0",
		Keyring:     keyring,
		Store:       store,
		AuditWriter: auditWriter,
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Create test request
	req := httptest.NewRequest("GET", "http://api.test.com/path", nil)
	req.Host = "api.test.com"
	w := httptest.NewRecorder()

	// Serve the request
	proxy.handleProxy(w, req)

	// The request should be forwarded (we can't test actual forwarding without a real server)
	// But we can verify the Authorization header was injected
	assert.Equal(t, "Bearer test-secret", req.Header.Get("Authorization"))

	// Verify audit event was written
	events := auditWriter.getEvents()
	assert.Len(t, events, 1)
	assert.Equal(t, "credential.accessed", events[0].Type)
	assert.Contains(t, events[0].Message, "test-provider")
}

// TestProxyStartStop tests starting and stopping the proxy.
func TestProxyStartStop(t *testing.T) {
	// Cannot use t.Parallel() due to timing-sensitive nature

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)
	logger := zap.NewNop()

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0", // Use :0 for random port
		Keyring:     keyring,
		Store:       store,
		AuditWriter: newMockAuditWriter(),
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Start proxy in background
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = proxy.Start(ctx)
	}()

	// Wait for proxy to start
	time.Sleep(500 * time.Millisecond)

	// Verify proxy is running
	assert.True(t, proxy.IsRunning())
	assert.NotEmpty(t, proxy.Addr())

	// Stop the proxy by canceling context and calling Stop
	cancel()
	err = proxy.Stop()
	require.NoError(t, err)

	// Give it time to shut down
	time.Sleep(500 * time.Millisecond)

	// Verify proxy is stopped
	assert.False(t, proxy.IsRunning())
}

// TestProxyDoubleStart tests that starting an already running proxy fails.
func TestProxyDoubleStart(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)
	logger := zap.NewNop()

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0",
		Keyring:     keyring,
		Store:       store,
		AuditWriter: newMockAuditWriter(),
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Start proxy in background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	started := make(chan struct{})
	go func() {
		close(started)
		_ = proxy.Start(ctx)
	}()

	// Wait for proxy to start
	<-started
	time.Sleep(200 * time.Millisecond)

	// Try to start again - should fail
	err = proxy.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Cleanup
	_ = proxy.Stop()
}

// TestProxyStopWhenNotRunning tests stopping a proxy that's not running.
func TestProxyStopWhenNotRunning(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)
	logger := zap.NewNop()

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0",
		Keyring:     keyring,
		Store:       store,
		AuditWriter: newMockAuditWriter(),
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Stop without starting - should succeed
	err = proxy.Stop()
	assert.NoError(t, err)
	assert.False(t, proxy.IsRunning())
}

// TestProxyIsRunning tests the IsRunning method.
func TestProxyIsRunning(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)
	logger := zap.NewNop()

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0",
		Keyring:     keyring,
		Store:       store,
		AuditWriter: newMockAuditWriter(),
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Initially not running
	assert.False(t, proxy.IsRunning())

	// Start proxy
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	started := make(chan struct{})
	go func() {
		close(started)
		_ = proxy.Start(ctx)
	}()

	// Wait for proxy to start
	<-started
	time.Sleep(200 * time.Millisecond)

	// Now running
	assert.True(t, proxy.IsRunning())

	// Stop proxy
	_ = proxy.Stop()

	// No longer running
	assert.False(t, proxy.IsRunning())
}

// TestIsHopByHopHeader tests hop-by-hop header detection.
func TestIsHopByHopHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		expected bool
	}{
		{"connection", "Connection", true},
		{"keep-alive", "Keep-Alive", true},
		{"proxy-auth", "Proxy-Authenticate", true},
		{"te", "Te", true},
		{"transfer-encoding", "Transfer-Encoding", true},
		{"upgrade", "Upgrade", true},
		{"content-type", "Content-Type", false},
		{"authorization", "Authorization", false},
		{"x-custom", "X-Custom-Header", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, isHopByHopHeader(tt.header))
		})
	}
}

// TestProxyAddr tests Addr method.
func TestProxyAddr(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0",
		Keyring:     keyring,
		Store:       store,
		AuditWriter: newMockAuditWriter(),
		Logger:      zap.NewNop(),
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Before start, addr is empty
	assert.Equal(t, "127.0.0.1:0", proxy.Addr())

	// Start and check
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = proxy.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	addr := proxy.Addr()
	assert.NotEmpty(t, addr, "Addr should return listening address after Start")
	assert.Contains(t, addr, "127.0.0.1", "Addr should contain localhost")

	cancel()
	time.Sleep(50 * time.Millisecond)
}
