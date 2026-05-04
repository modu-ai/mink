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
}

// New constructs a Bridge implementation. The server is created in the
// stopped state; call Start to begin accepting connections.
func New(cfg Config) (Bridge, error) {
	if err := verifyLoopbackBind(cfg.BindAddr); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 5 * time.Second
	}
	return &bridgeServer{
		cfg:      cfg,
		registry: NewRegistry(),
	}, nil
}

// bridgeServer is the default Bridge implementation.
//
// M0 wires only the listener lifecycle. The HTTP handler is an empty
// http.NewServeMux placeholder; M2 attaches the real routing table via
// BuildMux (REQ-BR-003).
type bridgeServer struct {
	cfg      Config
	registry *Registry

	mu       sync.Mutex
	state    serverState
	listener net.Listener
	httpSrv  *http.Server
	serveErr chan error
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

	mux := http.NewServeMux() // M2 will replace with BuildMux.
	srv := &http.Server{
		Handler:           mux,
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
