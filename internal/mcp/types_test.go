package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONRPCRequest_MarshalRoundTrip — JSON-RPC 2.0 요청 마샬/언마샬 검증
// AC-MCP-015 (transport interface 공통 시그니처) 의 기반 타입 검증
func TestJSONRPCRequest_MarshalRoundTrip(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/list",
		Params:  json.RawMessage(`{"cursor":null}`),
	}

	b, err := json.Marshal(req)
	require.NoError(t, err)

	var got JSONRPCRequest
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, req.JSONRPC, got.JSONRPC)
	assert.Equal(t, req.Method, got.Method)
	// ID는 JSON number로 마샬된다 — float64로 역마샬됨을 수용
	assert.NotNil(t, got.ID)
}

// TestJSONRPCResponse_ErrorFieldPresent — 에러 응답 구조 검증
func TestJSONRPCResponse_ErrorFieldPresent(t *testing.T) {
	b := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`)
	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal(b, &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code)
	assert.Equal(t, "method not found", resp.Error.Message)
}

// TestJSONRPCNotification_NoID — 알림에는 ID가 없어야 한다
func TestJSONRPCNotification_NoID(t *testing.T) {
	notif := JSONRPCNotification{
		JSONRPC: JSONRPCVersion,
		Method:  "$/cancelRequest",
		Params:  json.RawMessage(`{"id":42}`),
	}
	b, err := json.Marshal(notif)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Equal(t, "$/cancelRequest", m["method"])
	_, hasID := m["id"]
	assert.False(t, hasID, "알림 메시지에는 id 필드가 없어야 한다")
}

// TestJSONRPCMessage_TypeDiscrimination — IsRequest/IsNotification/IsResponse 판별
func TestJSONRPCMessage_TypeDiscrimination(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Method: "tools/list"}
		assert.True(t, msg.IsRequest())
		assert.False(t, msg.IsNotification())
		assert.False(t, msg.IsResponse())
	})
	t.Run("notification", func(t *testing.T) {
		msg := JSONRPCMessage{JSONRPC: "2.0", Method: "notifications/progress"}
		assert.False(t, msg.IsRequest())
		assert.True(t, msg.IsNotification())
		assert.False(t, msg.IsResponse())
	})
	t.Run("response", func(t *testing.T) {
		msg := JSONRPCMessage{JSONRPC: "2.0", ID: 1, Result: json.RawMessage(`{}`)}
		assert.False(t, msg.IsRequest())
		assert.False(t, msg.IsNotification())
		assert.True(t, msg.IsResponse())
	})
}

// TestJSONRPCError_ErrorInterface — JSONRPCError가 error 인터페이스를 만족하는지 검증
func TestJSONRPCError_ErrorInterface(t *testing.T) {
	e := &JSONRPCError{Code: ErrCodeInternal, Message: "internal error"}
	assert.Equal(t, "internal error", e.Error())
}

// TestServerSession_CapabilityCheck — capability 확인 메서드 검증 (REQ-MCP-021)
func TestServerSession_CapabilityCheck(t *testing.T) {
	s := &ServerSession{
		ServerCapabilities: map[string]bool{
			"tools": true,
		},
	}
	assert.True(t, s.HasCapability("tools"))
	assert.False(t, s.HasCapability("prompts"))
	assert.False(t, s.HasCapability("resources"))
}

// TestServerSession_StateTransition — 상태 전이 동시성 안전성 검증
func TestServerSession_StateTransition(t *testing.T) {
	s := &ServerSession{State: SessionConnected}
	assert.Equal(t, SessionConnected, s.GetState())
	s.SetState(SessionDisconnected)
	assert.Equal(t, SessionDisconnected, s.GetState())
}

// TestMCPServerConfig_DefaultTimeout — RequestTimeout 기본값 검증 (REQ-MCP-022)
func TestMCPServerConfig_DefaultTimeout(t *testing.T) {
	// DefaultRequestTimeout은 30초여야 한다
	assert.Equal(t, DefaultRequestTimeout.Seconds(), float64(30))
}

// TestSupportedProtocolVersions — 지원 프로토콜 버전 목록 검증 (REQ-MCP-018)
func TestSupportedProtocolVersions(t *testing.T) {
	assert.Contains(t, SupportedProtocolVersions, "2025-03-26")
}
