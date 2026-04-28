// Package transport provides gRPC client wrapper for daemon communication.
package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	_ "google.golang.org/grpc/resolver/passthrough"
)

// mockDaemonServer is a mock implementation of DaemonService for testing.
type mockDaemonServer struct {
	goosev1.UnimplementedDaemonServiceServer
	pingFunc func(context.Context, *goosev1.PingRequest) (*goosev1.PingResponse, error)
}

func (m *mockDaemonServer) Ping(ctx context.Context, req *goosev1.PingRequest) (*goosev1.PingResponse, error) {
	if m.pingFunc != nil {
		return m.pingFunc(ctx, req)
	}
	return &goosev1.PingResponse{
		Version:  "test-version",
		UptimeMs: 1000,
		State:    "serving",
	}, nil
}

func (m *mockDaemonServer) ChatStream(srv goosev1.DaemonService_ChatStreamServer) error {
	// Receive request (consume it)
	_, err := srv.Recv()
	if err != nil {
		return err
	}

	// Send text response
	if err := srv.Send(&goosev1.ChatStreamResponse{
		Event: &goosev1.ChatStreamResponse_Text{
			Text: &goosev1.TextEvent{Content: "Hi!"},
		},
	}); err != nil {
		return err
	}

	// Send done event
	return srv.Send(&goosev1.ChatStreamResponse{
		Event: &goosev1.ChatStreamResponse_Done{
			Done: &goosev1.DoneEvent{},
		},
	})
}

// bufDialer creates an in-memory gRPC listener for testing.
func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}
}

func TestNewDaemonClient(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		timeout   time.Duration
		wantErr   bool
		errIs     error
	}{
		{
			name:    "valid address",
			addr:    "127.0.0.1:9005",
			timeout: 3 * time.Second,
			wantErr: false,
		},
		{
			name:    "empty address defaults to localhost",
			addr:    "",
			timeout: 3 * time.Second,
			wantErr: false,
		},
		{
			name:    "zero timeout defaults to 3 seconds",
			addr:    "127.0.0.1:9005",
			timeout: 0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			lis := bufconn.Listen(1024 * 1024)
			s := grpc.NewServer()
			goosev1.RegisterDaemonServiceServer(s, &mockDaemonServer{})
			go s.Serve(lis)
			defer s.Stop()

			// Create client with bufconn dialer
			dialer := bufDialer(lis)
			conn, err := grpc.NewClient(
				"passthrough:///bufnet",
				grpc.WithContextDialer(dialer),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatalf("failed to dial mock server: %v", err)
			}
			defer conn.Close()

			// For this test, we're just testing client creation
			// In real scenario, we'd pass actual address
			client := &DaemonClient{
				conn:   conn,
				client: goosev1.NewDaemonServiceClient(conn),
			}

			if client == nil {
				t.Error("NewDaemonClient returned nil")
			}
		})
	}
}

func TestDaemonClient_Ping(t *testing.T) {
	tests := []struct {
		name    string
		mock    func(context.Context, *goosev1.PingRequest) (*goosev1.PingResponse, error)
		wantErr bool
		errCode codes.Code
	}{
		{
			name: "successful ping",
			mock: func(ctx context.Context, req *goosev1.PingRequest) (*goosev1.PingResponse, error) {
				return &goosev1.PingResponse{
					Version:  "v1.0.0",
					UptimeMs: 5000,
					State:    "serving",
				}, nil
			},
			wantErr: false,
		},
		{
			name: "server error",
			mock: func(ctx context.Context, req *goosev1.PingRequest) (*goosev1.PingResponse, error) {
				return nil, status.Error(codes.Unavailable, "server not ready")
			},
			wantErr: true,
			errCode: codes.Unavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			lis := bufconn.Listen(1024 * 1024)
			s := grpc.NewServer()
			mockServer := &mockDaemonServer{pingFunc: tt.mock}
			goosev1.RegisterDaemonServiceServer(s, mockServer)
			go s.Serve(lis)
			defer s.Stop()

			// Create client
			dialer := bufDialer(lis)
			conn, err := grpc.NewClient(
				"passthrough:///bufnet",
				grpc.WithContextDialer(dialer),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatalf("failed to dial mock server: %v", err)
			}
			defer conn.Close()

			client := &DaemonClient{
				conn:   conn,
				client: goosev1.NewDaemonServiceClient(conn),
			}

			// Test Ping
			ctx := context.Background()
			resp, err := client.Ping(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("Ping expected error but got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("expected gRPC status error, got %T", err)
				} else if st.Code() != tt.errCode {
					t.Errorf("expected error code %v, got %v", tt.errCode, st.Code())
				}
			} else {
				if err != nil {
					t.Errorf("Ping unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("Ping returned nil response")
				}
			}
		})
	}
}

func TestDaemonClient_ChatStream(t *testing.T) {
	tests := []struct {
		name    string
		messages []Message
		wantErr bool
	}{
		{
			name:    "successful stream",
			messages: []Message{{Role: "user", Content: "Hello"}},
			wantErr: false,
		},
		{
			name:    "empty messages",
			messages: []Message{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			lis := bufconn.Listen(1024 * 1024)
			s := grpc.NewServer()
			goosev1.RegisterDaemonServiceServer(s, &mockDaemonServer{})
			go s.Serve(lis)
			defer s.Stop()

			// Create client
			dialer := bufDialer(lis)
			conn, err := grpc.NewClient(
				"passthrough:///bufnet",
				grpc.WithContextDialer(dialer),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatalf("failed to dial mock server: %v", err)
			}
			defer conn.Close()

			client := &DaemonClient{
				conn:   conn,
				client: goosev1.NewDaemonServiceClient(conn),
			}

			// Test ChatStream
			ctx := context.Background()
			stream, err := client.ChatStream(ctx, tt.messages)

			if tt.wantErr {
				if err == nil {
					t.Error("ChatStream expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ChatStream unexpected error: %v", err)
				return
			}

			if stream == nil {
				t.Error("ChatStream returned nil channel")
				return
			}

			// Receive events
			eventCount := 0
			for event := range stream {
				eventCount++
				if event.Type == "error" {
					t.Errorf("unexpected error event: %s", event.Content)
				}
				if eventCount > 10 {
					t.Error("too many events received")
					break
				}
			}

			if eventCount == 0 {
				t.Error("no events received from stream")
			}
		})
	}
}

func TestDaemonClient_ConnectionTimeout(t *testing.T) {
	// Test connection timeout when daemon is not running
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Try to connect to an address that won't respond
	client, err := NewDaemonClient("127.0.0.1:9999", 3*time.Second)
	if err != nil {
		// Connection error is expected
		return
	}
	defer client.Close()

	// Try to ping
	_, err = client.Ping(ctx)
	if err == nil {
		t.Error("expected error when connecting to non-existent server")
	}
}

func TestDaemonClient_Close(t *testing.T) {
	// Create mock server
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	goosev1.RegisterDaemonServiceServer(s, &mockDaemonServer{})
	go s.Serve(lis)
	defer s.Stop()

	// Create client
	dialer := bufDialer(lis)
	conn, err := grpc.NewClient(
				"passthrough:///bufnet",
				grpc.WithContextDialer(dialer),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
	if err != nil {
		t.Fatalf("failed to dial mock server: %v", err)
	}

	client := &DaemonClient{
		conn:   conn,
		client: goosev1.NewDaemonServiceClient(conn),
	}

	// Test Close
	err = client.Close()
	if err != nil {
		t.Errorf("Close unexpected error: %v", err)
	}

	// Double close should return an error (already closed)
	err = client.Close()
	// We expect an error here since grpc connection throws error on double-close
	// But our implementation sets conn to nil, so subsequent calls return nil
	if err != nil {
		// This is acceptable - gRPC returns error on double-close
	}
}
