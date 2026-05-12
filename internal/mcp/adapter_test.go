package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modu-ai/mink/internal/mcp/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransportImpl은 transport.Transport를 구현하는 mock이다.
type mockTransportImpl struct {
	requests []transport.Request
	response transport.Response
	respFn   func(req transport.Request) transport.Response
	handlers []func(transport.Message)
	closed   bool
}

func (m *mockTransportImpl) SendRequest(_ context.Context, req transport.Request) (transport.Response, error) {
	m.requests = append(m.requests, req)
	if m.respFn != nil {
		return m.respFn(req), nil
	}
	return m.response, nil
}

func (m *mockTransportImpl) Notify(_ context.Context, _ transport.Notification) error {
	return nil
}

func (m *mockTransportImpl) OnMessage(handler func(transport.Message)) {
	m.handlers = append(m.handlers, handler)
}

func (m *mockTransportImpl) Close() error {
	m.closed = true
	return nil
}

// TestTransportAdapter_SendRequest는 transportAdapter.SendRequest를 검증한다.
func TestTransportAdapter_SendRequest(t *testing.T) {
	mock := &mockTransportImpl{
		response: transport.Response{
			JSONRPC: transport.JSONRPCVersion,
			ID:      float64(1),
			Result:  json.RawMessage(`"test"`),
		},
	}

	adapter := wrapTransport(mock)
	req := JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: 1, Method: "test"}
	resp, err := adapter.SendRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, JSONRPCVersion, resp.JSONRPC)
	assert.Equal(t, 1, len(mock.requests))
	assert.Equal(t, "test", mock.requests[0].Method)
}

// TestTransportAdapter_SendRequest_WithError는 에러 응답 변환을 검증한다.
func TestTransportAdapter_SendRequest_WithError(t *testing.T) {
	mock := &mockTransportImpl{
		response: transport.Response{
			JSONRPC: transport.JSONRPCVersion,
			ID:      float64(1),
			Error: &transport.Error{
				Code:    transport.ErrCodeInternal,
				Message: "internal error",
				Data:    json.RawMessage(`"data"`),
			},
		},
	}

	adapter := wrapTransport(mock)
	resp, err := adapter.SendRequest(context.Background(), JSONRPCRequest{Method: "test"})
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	assert.Equal(t, transport.ErrCodeInternal, resp.Error.Code)
	assert.Equal(t, "internal error", resp.Error.Message)
}

// TestTransportAdapter_Notify는 transportAdapter.Notify를 검증한다.
func TestTransportAdapter_Notify(t *testing.T) {
	mock := &mockTransportImpl{}
	adapter := wrapTransport(mock)

	notif := JSONRPCNotification{JSONRPC: JSONRPCVersion, Method: "test/notify"}
	err := adapter.Notify(context.Background(), notif)
	require.NoError(t, err)
}

// TestTransportAdapter_OnMessage는 transportAdapter.OnMessage를 검증한다.
func TestTransportAdapter_OnMessage(t *testing.T) {
	mock := &mockTransportImpl{}
	adapter := wrapTransport(mock)

	received := make(chan JSONRPCMessage, 1)
	adapter.OnMessage(func(msg JSONRPCMessage) {
		received <- msg
	})

	// 핸들러가 등록됨을 확인
	assert.Len(t, mock.handlers, 1)

	// 등록된 핸들러를 호출하여 변환 검증
	mock.handlers[0](transport.Message{
		JSONRPC: transport.JSONRPCVersion,
		Method:  "notifications/test",
		Error: &transport.Error{
			Code:    -1,
			Message: "test error",
		},
	})

	msg := <-received
	assert.Equal(t, "notifications/test", msg.Method)
	require.NotNil(t, msg.Error)
	assert.Equal(t, -1, msg.Error.Code)
}

// TestTransportAdapter_Close는 transportAdapter.Close를 검증한다.
func TestTransportAdapter_Close(t *testing.T) {
	mock := &mockTransportImpl{}
	adapter := wrapTransport(mock)

	err := adapter.Close()
	require.NoError(t, err)
	assert.True(t, mock.closed)
}

// TestFetchToolManifest는 mcpConnectionBridge.FetchToolManifest를 검증한다.
func TestFetchToolManifest(t *testing.T) {
	session := &ServerSession{
		ID: "fetch-test",
		tools: []MCPTool{
			{Name: "mcp__fx__search", Description: "Search"},
			{Name: "mcp__fx__fetch", Description: "Fetch"},
		},
		toolsLoaded: true,
	}

	bridge := &mcpConnectionBridge{session: session}

	// 존재하는 tool
	manifest, err := bridge.FetchToolManifest(nil, "mcp__fx__search")
	require.NoError(t, err)
	assert.Equal(t, "mcp__fx__search", manifest.Name)

	// 존재하지 않는 tool
	_, err = bridge.FetchToolManifest(nil, "nonexistent")
	require.Error(t, err)
}
