// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-001, REQ-BR-003, REQ-BR-005
// AC: AC-BR-001, AC-BR-003, AC-BR-005
// M0-T3 — bridgeServer skeleton: loopback bind, listener lifecycle.
// HTTP handlers (mux / WebSocket / SSE / POST) are mounted in M2.

package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// Config controls the bridgeServer at construction time.
type Config struct {
	// BindAddr is the "host:port" form passed to verifyLoopbackBind.
	// Required. Must resolve to a loopback alias (127.0.0.1, ::1, localhost).
	BindAddr string

	// ShutdownTimeout caps the duration of a graceful Stop. Zero defaults to 5s.
	ShutdownTimeout time.Duration

	// HMACSecret keys session-cookie signing. Empty triggers crypto/rand
	// generation inside NewAuthenticator (32 bytes).
	HMACSecret []byte

	// Adapter handles validated InboundMessages. Nil falls back to a noop
	// adapter so a bridge can serve auth/SSE/static traffic before the
	// QueryEngine is wired in.
	Adapter QueryEngineAdapter
}

// New constructs a Bridge implementation. The server is created in the
// stopped state; call Start to begin accepting connections.
//
// Side effects: an Authenticator and RevocationStore are created at
// construction time. They live for the bridgeServer's lifetime.
func New(cfg Config) (Bridge, error) {
	if err := verifyLoopbackBind(cfg.BindAddr); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 5 * time.Second
	}
	auth, err := NewAuthenticator(AuthConfig{HMACSecret: cfg.HMACSecret})
	if err != nil {
		return nil, fmt.Errorf("bridge: authenticator init: %w", err)
	}
	registry := NewRegistry()
	revocation := NewRevocationStore(nil)
	buffer := newOutboundBuffer(nil)
	gate := newFlushGate()
	dispatcher := newOutboundDispatcher(registry, buffer, gate)
	resumer := newResumer(buffer)
	permStore := newPermissionStore(nil)
	permReq := newPermissionRequester(permStore, dispatcher, PermissionTimeout)
	return &bridgeServer{
		cfg:           cfg,
		registry:      registry,
		auth:          auth,
		revocation:    revocation,
		buffer:        buffer,
		gate:          gate,
		dispatcher:    dispatcher,
		resumer:       resumer,
		permissions:   permStore,
		permRequester: permReq,
	}, nil
}

// bridgeServer is the default Bridge implementation. M2 wires BuildMux
// onto the listener so the WebSocket / SSE / inbound endpoints become
// reachable.
type bridgeServer struct {
	cfg           Config
	registry      *Registry
	auth          *Authenticator
	revocation    *RevocationStore
	buffer        *outboundBuffer
	gate          *flushGate
	dispatcher    *outboundDispatcher
	resumer       *resumer
	permissions   *permissionStore
	permRequester *permissionRequester

	mu       sync.Mutex
	state    serverState
	listener net.Listener
	httpSrv  *http.Server
	serveErr chan error
}

// SendOutbound implements Bridge.SendOutbound (M3, REQ-BR-007).
func (s *bridgeServer) SendOutbound(sessionID string, t OutboundType, payload []byte) (uint64, error) {
	return s.dispatcher.SendOutbound(sessionID, t, payload)
}

// RequestPermission implements Bridge.RequestPermission (M3, REQ-BR-008).
func (s *bridgeServer) RequestPermission(ctx context.Context, sessionID string, payload []byte) (bool, error) {
	return s.permRequester.Request(ctx, sessionID, payload)
}

type serverState int

const (
	stateStopped serverState = iota
	stateStarted
)

// ErrAlreadyStarted is returned when Start is invoked on a running server.
var ErrAlreadyStarted = errors.New("bridge: already started")

// Start binds the loopback listener and begins serving. Returns
// ErrAlreadyStarted if called twice without an intervening Stop. Honors ctx
// for the bind step only; serving runs in a background goroutine until Stop.
func (s *bridgeServer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.state == stateStarted {
		s.mu.Unlock()
		return ErrAlreadyStarted
	}

	// Re-verify in case Config.BindAddr was mutated post-construction.
	if err := verifyLoopbackBind(s.cfg.BindAddr); err != nil {
		s.mu.Unlock()
		return err
	}

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", s.cfg.BindAddr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("bridge: listener bind failed: %w", err)
	}

	handler := BuildMux(MuxConfig{
		Auth:          s.auth,
		Registry:      s.registry,
		Revocation:    s.revocation,
		Adapter:       s.cfg.Adapter,
		permRequester: s.permRequester,
	})
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	s.listener = ln
	s.httpSrv = srv
	s.serveErr = serveErr
	s.state = stateStarted
	s.mu.Unlock()

	go func() {
		err := srv.Serve(ln)
		// http.ErrServerClosed is the expected signal from Stop.
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
		close(serveErr)
	}()

	return nil
}

// Stop performs a graceful shutdown. Safe to call when the server is already
// stopped (returns nil).
func (s *bridgeServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.state == stateStopped {
		s.mu.Unlock()
		return nil
	}
	srv := s.httpSrv
	serveErr := s.serveErr
	timeout := s.cfg.ShutdownTimeout
	s.mu.Unlock()

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shutdownErr := srv.Shutdown(shutdownCtx)

	// Drain the serve goroutine.
	var serveResult error
	if serveErr != nil {
		serveResult = <-serveErr
	}

	s.mu.Lock()
	s.state = stateStopped
	s.listener = nil
	s.httpSrv = nil
	s.serveErr = nil
	s.mu.Unlock()

	if shutdownErr != nil {
		return fmt.Errorf("bridge: shutdown failed: %w", shutdownErr)
	}
	return serveResult
}

// Sessions returns a snapshot of currently registered sessions.
func (s *bridgeServer) Sessions() []WebUISession {
	return s.registry.Snapshot()
}

// Metrics returns a zero-valued Metrics for M0; populated in M5.
func (s *bridgeServer) Metrics() Metrics {
	return Metrics{
		ActiveSessions: int64(s.registry.Len()),
	}
}

// Addr returns the listener address (only valid while running).
// Useful for tests that bind to port 0 and discover the actual port.
func (s *bridgeServer) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}
