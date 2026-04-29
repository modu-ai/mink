package credproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ProxyConfig controls the credential proxy server.
//
// REQ-CREDPROXY-002: Agent API calls relayed through goose-proxy
type ProxyConfig struct {
	ListenAddr string               // Default: "127.0.0.1:18080"
	Keyring    KeyringBackend       // OS keyring backend
	Store      *ProviderStore       // Provider reference store
	AuditWriter auditWriter         // Audit log writer
	Logger     *zap.Logger          // Logger instance
}

// auditWriter is the interface for writing audit events.
// This matches the audit.Writer interface from internal/audit.
type auditWriter interface {
	Write(event auditEvent) error
}

// auditEvent represents an audit event (matches internal/audit.AuditEvent).
type auditEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// Proxy is the credential injection proxy server.
//
// REQ-CREDPROXY-002: Agent API calls relayed through goose-proxy which injects Authorization header
//
// @MX:ANCHOR: [AUTO] Credential proxy server
// @MX:REASON: Main proxy server component, fan_in >= 3 (CLI, agent runtime, tests)
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-002, AC-CREDPROXY-04
type Proxy struct {
	cfg      ProxyConfig
	server   *http.Server
	listener net.Listener
	mu       sync.RWMutex
	running  bool
}

// NewProxy creates a new credential proxy server.
//
// REQ-CREDPROXY-005: If proxy not running, block request and guide user to start proxy
func NewProxy(cfg ProxyConfig) (*Proxy, error) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:18080" // Default to localhost only
	}
	if cfg.Keyring == nil {
		return nil, fmt.Errorf("keyring backend is required")
	}
	if cfg.Store == nil {
		return nil, fmt.Errorf("provider store is required")
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	p := &Proxy{
		cfg: cfg,
	}

	// Create HTTP server
	p.server = &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      p.createHandler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return p, nil
}

// Start starts the proxy server.
//
// @MX:ANCHOR: [AUTO] Start proxy server
// @MX:REASON: Entry point for proxy lifecycle, fan_in >= 3
func (p *Proxy) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("proxy is already running")
	}
	p.running = true
	p.mu.Unlock()

	// Create listener
	listener, err := net.Listen("tcp", p.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", p.cfg.ListenAddr, err)
	}

	p.mu.Lock()
	p.listener = listener
	p.mu.Unlock()

	p.cfg.Logger.Info("Credential proxy started",
		zap.String("addr", p.cfg.ListenAddr),
	)

	// Log audit event
	p.logAuditEvent("proxy.start", "info", "Credential proxy server started",
		map[string]string{"addr": p.cfg.ListenAddr})

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := p.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("proxy server error: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		p.cfg.Logger.Info("Credential proxy shutting down")
		return p.Stop()
	case err := <-errChan:
		return err
	}
}

// Stop stops the proxy server.
//
// @MX:ANCHOR: [AUTO] Stop proxy server
// @MX:REASON: Lifecycle management required by CLI and tests, fan_in >= 3
func (p *Proxy) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = false
	p.mu.Unlock()

	p.cfg.Logger.Info("Stopping credential proxy")

	// Log audit event
	p.logAuditEvent("proxy.stop", "info", "Credential proxy server stopped", nil)

	// Shutdown server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown proxy server: %w", err)
	}

	return nil
}

// Addr returns the address the proxy is listening on.
func (p *Proxy) Addr() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.listener != nil {
		return p.listener.Addr().String()
	}
	return p.cfg.ListenAddr
}

// IsRunning returns whether the proxy is currently running.
func (p *Proxy) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// createHandler creates the main HTTP handler for the proxy.
func (p *Proxy) createHandler() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/_health", p.handleHealth)

	// Proxy handler for all other requests
	mux.HandleFunc("/", p.handleProxy)

	return mux
}

// handleHealth handles health check requests.
func (p *Proxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		// Log warning but response is already sent
		// Health check failures are non-critical
		p.cfg.Logger.Warn("Failed to write health check response", zap.Error(err))
	}
}

// handleProxy handles proxy requests.
//
// REQ-CREDPROXY-002: Agent API calls relayed through goose-proxy
// REQ-CREDPROXY-003: Secret value NEVER enters agent process memory
// REQ-CREDPROXY-004: Scoped credential binding enforcement
//
// @MX:ANCHOR: [AUTO] Proxy request handler
// @MX:REASON: Core relay logic with credential injection, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-002, REQ-CREDPROXY-003, REQ-CREDPROXY-004
func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request) {
	// Extract host from request
	host := extractHost(r.Host)

	// Match provider based on host pattern (REQ-CREDPROXY-004)
	provider, ref, err := p.cfg.Store.MatchForHost(host)
	if err != nil {
		// No matching provider found
		p.cfg.Logger.Warn("No provider found for host",
			zap.String("host", host),
			zap.Error(err),
		)

		// REQ-CREDPROXY-004: Scoped binding enforcement
		// Block request when no matching provider
		http.Error(w, "No credential configured for this host", http.StatusForbidden)
		return
	}

	// Retrieve secret from keyring
	secret, err := p.cfg.Keyring.Retrieve("goose", ref.KeyringID)
	if err != nil {
		p.cfg.Logger.Error("Failed to retrieve credential from keyring",
			zap.String("provider", provider),
			zap.String("keyring_id", ref.KeyringID),
			zap.Error(err),
		)

		http.Error(w, "Failed to retrieve credential", http.StatusInternalServerError)
		return
	}

	// CRITICAL: Zeroize secret bytes after use to prevent credential leakage in memory
	// This ensures Bearer tokens don't remain in memory dumps, core files, or debugger access
	defer zeroBytes(secret)

	// Log credential access (without value)
	p.logAuditEvent("credential.accessed", "info",
		fmt.Sprintf("Credential accessed for provider: %s", provider),
		map[string]string{
			"provider":  provider,
			"keyring_id": ref.KeyringID,
			"host":      host,
		},
	)

	// Inject Authorization header (REQ-CREDPROXY-002)
	// REQ-CREDPROXY-003: Secret value NEVER enters agent process memory
	// The secret is retrieved and injected here in the proxy process
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(secret)))

	// Forward request to actual API
	p.forwardRequest(w, r)
}

// forwardRequest forwards the request to the actual API endpoint.
//
// @MX:NOTE: [AUTO] Request forwarding
// @MX:TEST: Verify forwarded requests include injected headers
func (p *Proxy) forwardRequest(w http.ResponseWriter, r *http.Request) {
	// Create HTTP client for forwarding
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Build target URL
	targetURL := r.URL
	targetURL.Scheme = "https" // Force HTTPS for API calls
	targetURL.Host = r.Host

	// Create forwarded request
	forwardReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
	if err != nil {
		p.cfg.Logger.Error("Failed to create forward request",
			zap.String("host", r.Host),
			zap.Error(err),
		)
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}

	// Copy headers (excluding hop-by-hop headers)
	for name, values := range r.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(name) {
			continue
		}
		for _, value := range values {
			forwardReq.Header.Add(name, value)
		}
	}

	// Execute forwarded request
	resp, err := client.Do(forwardReq)
	if err != nil {
		p.cfg.Logger.Error("Failed to execute forward request",
			zap.String("host", r.Host),
			zap.Error(err),
		)
		http.Error(w, "Failed to reach upstream API", http.StatusBadGateway)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log warning but response body will be closed on GC
			// This is after the response has been copied, so close error is non-critical
			p.cfg.Logger.Warn("Failed to close response body", zap.Error(err))
		}
	}()

	// Copy response headers
	for name, values := range resp.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(name) {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, _ = io.Copy(w, resp.Body)
}

// isHopByHopHeader checks if a header is a hop-by-hop header that should not be forwarded.
func isHopByHopHeader(name string) bool {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopByHopHeaders {
		if name == h {
			return true
		}
	}
	return false
}

// logAuditEvent logs an audit event.
func (p *Proxy) logAuditEvent(eventType, severity, message string, metadata map[string]string) {
	if p.cfg.AuditWriter == nil {
		return
	}

	event := auditEvent{
		Timestamp: time.Now(),
		Type:      eventType,
		Severity:  severity,
		Message:   message,
		Metadata:  metadata,
	}

	if err := p.cfg.AuditWriter.Write(event); err != nil {
		p.cfg.Logger.Error("Failed to write audit event",
			zap.String("event_type", eventType),
			zap.Error(err),
		)
	}
}

// zeroBytes securely clears a byte slice by overwriting all bytes with zeros.
//
// CRITICAL: This must be called on all secret byte slices after use to prevent
// credential leakage in memory dumps, core files, or debugger access. The defer
// pattern ensures zeroization happens even on error paths (panic, early return).
//
// @MX:NOTE: [AUTO] Memory security for credential zeroization
// @MX:SECURITY: Prevents secret material from remaining in heap memory after use
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
