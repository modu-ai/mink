// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-001, REQ-BR-005
// AC: AC-BR-001, AC-BR-005
// M0-T3 — bridgeServer lifecycle: New / Start / Stop / Sessions / Metrics.

package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// freeLoopbackAddr reserves an ephemeral loopback port and returns it as
// "127.0.0.1:NNNN". The reservation is released before return so the caller
// can rebind. There is a small race window, acceptable for tests.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port reserve failed: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()
	return fmt.Sprintf("127.0.0.1:%d", addr.Port)
}

func TestNew_RejectsNonLoopbackBind(t *testing.T) {
	t.Parallel()

	_, err := New(Config{BindAddr: "0.0.0.0:8091"})
	if !errors.Is(err, ErrNonLoopbackBind) {
		t.Fatalf("New(0.0.0.0:8091) err = %v, want ErrNonLoopbackBind", err)
	}
}

func TestNew_AcceptsLoopback(t *testing.T) {
	t.Parallel()

	b, err := New(Config{BindAddr: "127.0.0.1:8091"})
	if err != nil {
		t.Fatalf("New(127.0.0.1:8091) err = %v, want nil", err)
	}
	if b == nil {
		t.Fatalf("New returned nil Bridge")
	}
}

func TestServer_StartStopLifecycle(t *testing.T) {
	t.Parallel()

	addr := freeLoopbackAddr(t)
	b, err := New(Config{BindAddr: addr, ShutdownTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("New err = %v", err)
	}

	ctx := context.Background()
	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start err = %v", err)
	}

	// Probe a path the bridge mux does not register. BuildMux uses
	// http.ServeMux pattern matching; an unknown path returns 404 / 405.
	// We only need proof the listener is bound and serving — not which
	// specific status fires.
	resp, err := http.Get("http://" + addr + "/__unmounted__/probe")
	if err != nil {
		t.Fatalf("HTTP probe failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Errorf("probe status = %d, want 4xx (listener bound + mux active)", resp.StatusCode)
	}

	if err := b.Stop(ctx); err != nil {
		t.Fatalf("Stop err = %v", err)
	}
}

func TestServer_StartTwiceRejected(t *testing.T) {
	t.Parallel()

	addr := freeLoopbackAddr(t)
	b, err := New(Config{BindAddr: addr})
	if err != nil {
		t.Fatalf("New err = %v", err)
	}
	ctx := context.Background()
	if err := b.Start(ctx); err != nil {
		t.Fatalf("first Start err = %v", err)
	}
	defer func() { _ = b.Stop(ctx) }()

	if err := b.Start(ctx); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("second Start err = %v, want ErrAlreadyStarted", err)
	}
}

func TestServer_StopOnStoppedNoop(t *testing.T) {
	t.Parallel()

	addr := freeLoopbackAddr(t)
	b, err := New(Config{BindAddr: addr})
	if err != nil {
		t.Fatalf("New err = %v", err)
	}
	if err := b.Stop(context.Background()); err != nil {
		t.Fatalf("Stop on stopped server err = %v, want nil", err)
	}
}

func TestServer_StartStopRestart(t *testing.T) {
	t.Parallel()

	addr := freeLoopbackAddr(t)
	b, err := New(Config{BindAddr: addr})
	if err != nil {
		t.Fatalf("New err = %v", err)
	}
	ctx := context.Background()
	if err := b.Start(ctx); err != nil {
		t.Fatalf("first Start err = %v", err)
	}
	if err := b.Stop(ctx); err != nil {
		t.Fatalf("first Stop err = %v", err)
	}
	if err := b.Start(ctx); err != nil {
		t.Fatalf("restart Start err = %v", err)
	}
	if err := b.Stop(ctx); err != nil {
		t.Fatalf("restart Stop err = %v", err)
	}
}

func TestServer_SessionsAndMetricsZero(t *testing.T) {
	t.Parallel()

	addr := freeLoopbackAddr(t)
	b, err := New(Config{BindAddr: addr})
	if err != nil {
		t.Fatalf("New err = %v", err)
	}

	if got := b.Sessions(); len(got) != 0 {
		t.Errorf("Sessions() len = %d, want 0", len(got))
	}
	m := b.Metrics()
	if m.ActiveSessions != 0 {
		t.Errorf("Metrics.ActiveSessions = %d, want 0", m.ActiveSessions)
	}
}

func TestServer_AddrAvailableWhileRunning(t *testing.T) {
	t.Parallel()

	addr := freeLoopbackAddr(t)
	b, err := New(Config{BindAddr: addr})
	if err != nil {
		t.Fatalf("New err = %v", err)
	}

	// Type-assert back to *bridgeServer for the Addr inspection helper.
	srv := b.(*bridgeServer)
	if srv.Addr() != nil {
		t.Errorf("Addr() before Start = %v, want nil", srv.Addr())
	}

	ctx := context.Background()
	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start err = %v", err)
	}
	defer func() { _ = b.Stop(ctx) }()

	if srv.Addr() == nil {
		t.Fatalf("Addr() after Start = nil, want non-nil")
	}
	gotAddr := srv.Addr().(*net.TCPAddr)
	expectPort, _ := strconv.Atoi(addr[len("127.0.0.1:"):])
	if gotAddr.Port != expectPort {
		t.Errorf("Addr port = %d, want %d", gotAddr.Port, expectPort)
	}
}
