// SPEC-GOOSE-CLI-001 Phase B1 — PingClientAdapter tests.
// Reuses mockDaemonConnectServer + newTestServer from connect_test.go.
package transport

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1"
)

// RED #1: PingClientAdapter writes a byte-identical status line for a
// successful Ping response.
func TestPingClientAdapter_Ping_Success(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&mockDaemonConnectServer{
		pingFunc: func(_ context.Context, _ *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error) {
			return connect.NewResponse(&goosev1.PingResponse{
				Version:  "v0.2.0",
				UptimeMs: 12345,
				State:    "serving",
			}), nil
		},
	}, nil, nil, nil)
	defer srv.Close()

	adapter := &PingClientAdapter{
		newClient: func(_ string, opts ...ConnectOption) (*ConnectClient, error) {
			return NewConnectClient(srv.URL, opts...)
		},
	}

	var buf bytes.Buffer
	if err := adapter.Ping(context.Background(), "127.0.0.1:9005", &buf); err != nil {
		t.Fatalf("adapter.Ping: %v", err)
	}

	got := buf.String()
	want := "pong (version=v0.2.0, state=serving, uptime=12345ms)\n"
	if got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}
}

// RED #2: PingClientAdapter propagates context cancellation.
func TestPingClientAdapter_Ping_Timeout(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&mockDaemonConnectServer{
		pingFunc: func(ctx context.Context, _ *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}, nil, nil, nil)
	defer srv.Close()

	adapter := &PingClientAdapter{
		newClient: func(_ string, opts ...ConnectOption) (*ConnectClient, error) {
			return NewConnectClient(srv.URL, opts...)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	err := adapter.Ping(ctx, "127.0.0.1:9005", &buf)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "ping failed") {
		t.Errorf("expected wrapped 'ping failed' error, got: %v", err)
	}
}

// RED #3 (RPC error path): PingClientAdapter wraps connect errors and writes
// nothing to the output stream when Ping fails.
func TestPingClientAdapter_Ping_RPCError(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&mockDaemonConnectServer{
		pingFunc: func(_ context.Context, _ *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error) {
			return nil, connect.NewError(connect.CodeInternal, errors.New("boom"))
		},
	}, nil, nil, nil)
	defer srv.Close()

	adapter := &PingClientAdapter{
		newClient: func(_ string, opts ...ConnectOption) (*ConnectClient, error) {
			return NewConnectClient(srv.URL, opts...)
		},
	}

	var buf bytes.Buffer
	err := adapter.Ping(context.Background(), "127.0.0.1:9005", &buf)
	if err == nil {
		t.Fatal("expected RPC error, got nil")
	}
	if !strings.Contains(err.Error(), "ping failed") {
		t.Errorf("expected wrapped 'ping failed' error, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("writer should be empty on error, got: %q", buf.String())
	}
}

// PingClientAdapter surfaces factory failures to callers.
func TestPingClientAdapter_Ping_FactoryError(t *testing.T) {
	t.Parallel()

	adapter := &PingClientAdapter{
		newClient: func(_ string, _ ...ConnectOption) (*ConnectClient, error) {
			return nil, errors.New("dial blocked")
		},
	}

	var buf bytes.Buffer
	err := adapter.Ping(context.Background(), "127.0.0.1:9005", &buf)
	if err == nil {
		t.Fatal("expected factory error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to connect") {
		t.Errorf("expected 'failed to connect' wrap, got: %v", err)
	}
}

// NewPingClientAdapter returns a non-nil adapter wired to NewConnectClient.
func TestNewPingClientAdapter_DefaultsToNewConnectClient(t *testing.T) {
	t.Parallel()

	adapter := NewPingClientAdapter()
	if adapter == nil {
		t.Fatal("NewPingClientAdapter returned nil")
	}
	if adapter.newClient == nil {
		t.Fatal("default factory must not be nil")
	}
}

// NormalizeDaemonURL prepends http:// only when scheme is missing.
func TestNormalizeDaemonURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"127.0.0.1:9005", "http://127.0.0.1:9005"},
		{"localhost:8080", "http://localhost:8080"},
		{"http://127.0.0.1:9005", "http://127.0.0.1:9005"},
		{"https://daemon.example:443", "https://daemon.example:443"},
	}

	for _, tc := range cases {
		if got := NormalizeDaemonURL(tc.in); got != tc.want {
			t.Errorf("NormalizeDaemonURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
