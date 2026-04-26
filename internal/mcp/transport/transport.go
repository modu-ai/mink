// Package transport는 MCP JSON-RPC 2.0 transport 추상화와 구현체들을 제공한다.
// stdio/WebSocket/SSE 세 가지 transport를 단일 Transport 인터페이스로 추상화한다.
// 이 패키지는 내부 패키지 의존성을 갖지 않는 독립적인 패키지이다.
//
// SPEC: SPEC-GOOSE-MCP-001 v0.2.0, REQ-MCP-004
package transport

import (
	"context"
	"encoding/json"
)

// --- JSON-RPC 2.0 타입 ---

// JSONRPCVersion은 JSON-RPC 2.0 버전 문자열이다.
const JSONRPCVersion = "2.0"

// Request는 JSON-RPC 2.0 요청 메시지이다.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response는 JSON-RPC 2.0 응답 메시지이다.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Notification은 ID가 없는 JSON-RPC 2.0 알림 메시지이다.
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Message는 요청, 응답, 알림을 통합하는 union 타입이다.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// IsRequest는 메시지가 요청인지 반환한다.
func (m Message) IsRequest() bool {
	return m.Method != "" && m.ID != nil
}

// IsNotification은 메시지가 알림인지 반환한다.
func (m Message) IsNotification() bool {
	return m.Method != "" && m.ID == nil
}

// IsResponse는 메시지가 응답인지 반환한다.
func (m Message) IsResponse() bool {
	return m.Method == "" && (m.Result != nil || m.Error != nil)
}

// Error는 JSON-RPC 2.0 에러 객체이다.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// JSON-RPC 2.0 표준 에러 코드.
const (
	ErrCodeParse            = -32700
	ErrCodeInvalidRequest   = -32600
	ErrCodeMethodNotFound   = -32601
	ErrCodeInvalidParams    = -32602
	ErrCodeInternal         = -32603
	ErrCodeRequestCancelled = -32800
)

// ErrTransportClosed는 닫힌 transport에 연산 시 반환된다.
type ErrTransportClosedType struct{}

func (e ErrTransportClosedType) Error() string { return "transport is closed" }

// ErrTransportClosed는 닫힌 transport에 연산 시 반환되는 에러이다.
var ErrTransportClosed = ErrTransportClosedType{}

// TLSValidationError는 TLS 검증 실패 에러 타입이다.
// transport 패키지 소비자가 이 타입을 type assertion으로 확인할 수 있다.
type TLSValidationError struct {
	Cause error
}

func (e TLSValidationError) Error() string {
	if e.Cause != nil {
		return "TLS certificate validation failed: " + e.Cause.Error()
	}
	return "TLS certificate validation failed"
}

func (e TLSValidationError) Unwrap() error { return e.Cause }

// ErrTLSValidation는 TLS 검증 실패 에러이다.
var ErrTLSValidation = TLSValidationError{}

// Transport는 MCP JSON-RPC 메시지 전송 인터페이스이다.
// REQ-MCP-004: stdio/WebSocket/SSE 공통 시그니처
//
// @MX:ANCHOR: [AUTO] Transport — MCP transport 공통 추상화 인터페이스
// @MX:REASON: REQ-MCP-004, AC-MCP-015 — 세 구현체 모두 이 인터페이스를 만족해야 함. fan_in >= 3
type Transport interface {
	SendRequest(ctx context.Context, req Request) (Response, error)
	Notify(ctx context.Context, msg Notification) error
	OnMessage(handler func(Message))
	Close() error
}

// 컴파일 타임 인터페이스 검증.
var (
	_ Transport = (*StdioTransport)(nil)
	_ Transport = (*WebSocketTransport)(nil)
	_ Transport = (*SSETransport)(nil)
)

// StdioTransport는 stdio subprocess를 통한 MCP transport 구현이다.
// REQ-MCP-005, REQ-MCP-014, REQ-MCP-019
//
// @MX:WARN: [AUTO] goroutine 사용 — readLoop + stderr goroutine이 내부에서 실행됨
// @MX:REASON: REQ-MCP-005, REQ-MCP-014 — subprocess IO 비동기 처리. Close() 시 종료 보장
type StdioTransport struct {
	inner *stdioBase
}

// WebSocketTransport는 WebSocket을 통한 MCP transport 구현이다.
// REQ-MCP-015
//
// @MX:WARN: [AUTO] goroutine 사용 — WebSocket read goroutine이 내부에서 실행됨
// @MX:REASON: REQ-MCP-004, REQ-MCP-015 — read goroutine은 conn.Close() 시 종료됨
type WebSocketTransport struct {
	inner *wsBase
}

// SSETransport는 SSE(Server-Sent Events)를 통한 MCP transport 구현이다.
// REQ-MCP-020
//
// @MX:WARN: [AUTO] goroutine 사용 — SSE event stream read goroutine이 내부에서 실행됨
// @MX:REASON: REQ-MCP-020 — SSE stream goroutine은 Close() 시 context cancel로 종료됨
type SSETransport struct {
	inner *sseBase
}
