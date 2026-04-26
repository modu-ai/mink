package mcp

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPServer_Serve는 Serve 메서드의 기본 동작을 검증한다.
func TestMCPServer_Serve(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	_, _ = srv.Tool("echo", nil, func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
		return input, nil
	})

	// serveMock은 Serve가 OnMessage를 호출하는지 검증하는 mock transport
	mock := &mockTransport{}
	mock.response = JSONRPCResponse{JSONRPC: JSONRPCVersion}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Serve는 ctx.Done()이나 transport 닫힘 시 반환해야 한다
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx, mock)
	}()

	select {
	case err := <-errCh:
		// ctx timeout으로 종료
		assert.Error(t, err) // context.DeadlineExceeded
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Serve should return after ctx cancel")
	}

	// OnMessage 핸들러가 등록됨을 확인
	mock.mu.Lock()
	handlerCount := len(mock.handlers)
	mock.mu.Unlock()
	assert.Equal(t, 1, handlerCount, "Serve는 OnMessage 핸들러를 1개 등록해야 함")
}

// TestMCPServer_Serve_HandlerDispatch는 Serve가 요청을 dispatch하는지 검증한다.
func TestMCPServer_Serve_HandlerDispatch(t *testing.T) {
	var callCount int32 // atomic 사용
	srv := NewServer(ServerInfo{Name: "test"})
	_, _ = srv.Tool("echo", nil, func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
		atomic.AddInt32(&callCount, 1)
		return input, nil
	})

	mock := &mockTransport{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go srv.Serve(ctx, mock)
	time.Sleep(10 * time.Millisecond) // Serve 시작 대기

	// OnMessage 핸들러를 통해 요청 주입
	mock.mu.Lock()
	handlers := make([]func(JSONRPCMessage), len(mock.handlers))
	copy(handlers, mock.handlers)
	mock.mu.Unlock()

	if len(handlers) > 0 {
		params, _ := json.Marshal(map[string]any{"name": "echo", "arguments": json.RawMessage(`{}`)})
		handlers[0](JSONRPCMessage{
			JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: params,
		})
		time.Sleep(20 * time.Millisecond)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "echo handler가 호출되어야 함")
	}
}

// TestMCPServer_Serve_NonRequestMessage는 비-요청 메시지를 Serve가 무시하는지 검증한다.
func TestMCPServer_Serve_NonRequestMessage(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test"})
	mock := &mockTransport{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go srv.Serve(ctx, mock)
	time.Sleep(10 * time.Millisecond)

	mock.mu.Lock()
	handlers := make([]func(JSONRPCMessage), len(mock.handlers))
	copy(handlers, mock.handlers)
	mock.mu.Unlock()

	if len(handlers) > 0 {
		// 알림 (ID 없음) → 무시해야 함
		handlers[0](JSONRPCMessage{JSONRPC: "2.0", Method: "notifications/initialized"})
	}
	// 패닉 없이 완료
}

// TestDefaultTransportFactory_Stdio는 defaultTransportFactory의 stdio 경로를 검증한다.
func TestDefaultTransportFactory_Stdio(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess test skipped in short mode")
	}

	// Command가 없으면 에러 반환
	_, err := defaultTransportFactory(context.Background(), MCPServerConfig{
		Transport: "stdio",
		Command:   "", // 없음
	})
	require.Error(t, err)
}

// TestDefaultTransportFactory_WebSocket는 defaultTransportFactory의 websocket 경로를 검증한다.
func TestDefaultTransportFactory_WebSocket(t *testing.T) {
	// URI 없음 → 에러
	_, err := defaultTransportFactory(context.Background(), MCPServerConfig{
		Transport: "websocket",
		URI:       "",
	})
	require.Error(t, err)
}

// TestDefaultTransportFactory_SSE는 defaultTransportFactory의 sse 경로를 검증한다.
func TestDefaultTransportFactory_SSE(t *testing.T) {
	// URI 없음 → 에러
	_, err := defaultTransportFactory(context.Background(), MCPServerConfig{
		Transport: "sse",
		URI:       "",
	})
	require.Error(t, err)
}
