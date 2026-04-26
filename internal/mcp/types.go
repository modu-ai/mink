// Package mcp는 AI.GOOSE의 Model Context Protocol(MCP) 클라이언트/서버를 구현한다.
// JSON-RPC 2.0 위에 stdio/WebSocket/SSE 3종 transport, OAuth 2.1 + PKCE,
// capability negotiation, $/cancelRequest, tool registry sync를 제공한다.
//
// SPEC: SPEC-GOOSE-MCP-001 v0.2.0
package mcp

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
)

// --- JSON-RPC 2.0 타입 ---

// JSONRPCVersion은 JSON-RPC 2.0 버전 문자열이다.
const JSONRPCVersion = "2.0"

// MCP 지원 프로토콜 버전.
// REQ-MCP-018: 이 목록에 없는 버전은 ErrUnsupportedProtocolVersion을 반환한다.
var SupportedProtocolVersions = []string{"2025-03-26"}

// JSONRPCRequest는 JSON-RPC 2.0 요청 메시지이다.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"` // string | int | null
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse는 JSON-RPC 2.0 응답 메시지이다.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCNotification은 JSON-RPC 2.0 알림 메시지(ID 없음)이다.
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCMessage는 요청, 응답, 알림을 통합하는 union 타입이다.
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// IsRequest는 메시지가 요청인지 반환한다.
func (m JSONRPCMessage) IsRequest() bool {
	return m.Method != "" && m.ID != nil
}

// IsNotification은 메시지가 알림인지 반환한다.
func (m JSONRPCMessage) IsNotification() bool {
	return m.Method != "" && m.ID == nil
}

// IsResponse는 메시지가 응답인지 반환한다.
func (m JSONRPCMessage) IsResponse() bool {
	return m.Method == "" && (m.Result != nil || m.Error != nil)
}

// JSONRPCError는 JSON-RPC 2.0 에러 객체이다.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return e.Message
}

// JSON-RPC 2.0 표준 에러 코드.
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
	// MCP 확장 에러 코드 (-32000 ~ -32099)
	ErrCodeRequestCancelled = -32800
)

// --- MCP 타입 ---

// MCPTool은 MCP 서버가 노출하는 tool 기술자이다. 이름은 네임스페이스가 적용된다.
// REQ-MCP-001: 이름 형식 "mcp__{serverName}__{toolName}"
type MCPTool struct {
	// Name은 네임스페이스가 적용된 이름: "mcp__{server}__{tool}"
	Name        string
	Description string
	InputSchema json.RawMessage
	ServerID    string
}

// MCPResource는 MCP 서버가 노출하는 리소스이다.
type MCPResource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
}

// ResourceContent는 ReadResource 결과이다.
type ResourceContent struct {
	URI      string
	MimeType string
	Text     string
	Blob     []byte
}

// PromptArgument는 MCP prompt의 단일 인수이다.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// MCPPrompt는 MCP 서버가 노출하는 prompt이다.
type MCPPrompt struct {
	Name        string
	Description string
	Arguments   []PromptArgument
	Template    string
}

// ToolResult는 CallTool의 결과이다.
type ToolResult struct {
	Content  json.RawMessage
	IsError  bool
	Metadata map[string]any
}

// --- 세션 상태 ---

// SessionState는 ServerSession의 연결 상태를 나타낸다.
type SessionState int

const (
	// SessionConnected는 연결이 활성화된 상태이다.
	SessionConnected SessionState = iota
	// SessionReconnecting은 재연결 시도 중인 상태이다.
	SessionReconnecting
	// SessionDisconnected는 연결이 종료된 상태이다.
	SessionDisconnected
	// SessionAuthPending은 OAuth 플로우 대기 중인 상태이다.
	// REQ-MCP-012: 이 상태에서 ListTools/CallTool은 60초 블록 후 ErrAuthFlowTimeout
	SessionAuthPending
)

// --- 설정 타입 ---

// TLSConfig는 TLS 연결 설정이다.
// REQ-MCP-015: 기본적으로 strict validation 적용
type TLSConfig struct {
	// Insecure를 true로 설정하면 self-signed 인증서를 허용한다 (보안 경고 출력)
	Insecure bool
}

// AuthConfig는 인증 방식 설정이다.
type AuthConfig struct {
	Type        string // "none" | "oauth2" | "bearer"
	ClientID    string
	AuthURL     string
	TokenURL    string
	Scopes      []string
	BearerToken string
}

// MCPServerConfig는 MCP 서버 연결 설정이다.
//
// @MX:ANCHOR: [AUTO] MCPServerConfig — MCP 서버 연결의 기본 설정 구조체
// @MX:REASON: REQ-MCP-002, REQ-MCP-005 — ConnectToServer의 memoize key 생성, transport 선택에 사용
type MCPServerConfig struct {
	// ID는 memoize key (Name + Transport + URI 해시)
	// REQ-MCP-002: 동일 ID의 두 번째 연결은 기존 세션 반환
	ID        string
	Name      string
	Transport string // "stdio" | "websocket" | "sse"
	Command   string // stdio only
	Args      []string
	// Env는 stdio 자식 프로세스에 주입할 환경 변수이다.
	// REQ-MCP-019: 부모 환경 위에 merge
	Env  map[string]string
	URI  string // websocket/sse only
	TLS  *TLSConfig
	Auth *AuthConfig
	// Prompts가 true이면 ConnectToServer 후 PromptToSkill을 실행한다.
	// REQ-MCP-013
	Prompts bool
	// RequestTimeout은 개별 요청 deadline이다.
	// REQ-MCP-022: 기본 30초
	RequestTimeout time.Duration
}

// DefaultRequestTimeout은 기본 요청 timeout이다.
const DefaultRequestTimeout = 30 * time.Second

// --- 서버 세션 ---

// ServerSession은 MCP 서버와의 단일 연결을 나타낸다.
//
// @MX:ANCHOR: [AUTO] ServerSession — MCP 서버 연결의 상태 보유 구조체
// @MX:REASON: REQ-MCP-005, REQ-MCP-021 — capability cache, 상태 머신의 핵심 데이터 구조
type ServerSession struct {
	// ID는 세션 고유 식별자 (= MCPServerConfig.ID)
	ID     string
	Config MCPServerConfig
	// State는 현재 연결 상태이다.
	State SessionState
	// ProtocolVersion은 initialize 핸드셰이크에서 서버가 반환한 버전이다.
	// REQ-MCP-018: "2025-03-26"만 허용
	ProtocolVersion string
	// ServerCapabilities는 initialize 응답에서 기록한 서버 capability 맵이다.
	// REQ-MCP-021: 이 맵에 없는 capability의 메서드 호출은 ErrCapabilityNotSupported 반환
	ServerCapabilities map[string]bool
	// ClientCapabilities는 initialize 요청에서 선언한 클라이언트 capability 맵이다.
	ClientCapabilities map[string]bool

	// transport는 현재 연결에 사용 중인 Transport 구현이다.
	transport Transport
	// tools는 ListTools의 캐시이다.
	// REQ-MCP-006, REQ-MCP-011: 첫 ListTools에서 채워지고, invalidate 전까지 유지
	tools       []MCPTool
	toolsLoaded bool
	// mu는 세션 상태 보호 RWMutex이다.
	mu     sync.RWMutex
	logger *zap.Logger
}

// GetState는 세션 상태를 안전하게 반환한다.
func (s *ServerSession) GetState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// SetState는 세션 상태를 안전하게 설정한다.
func (s *ServerSession) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

// HasCapability는 서버가 해당 capability를 선언했는지 확인한다.
// REQ-MCP-021
func (s *ServerSession) HasCapability(cap string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ServerCapabilities[cap]
}

// --- 서버 빌더 타입 ---

// ServerInfo는 MCPServer의 메타데이터이다.
type ServerInfo struct {
	Name    string
	Version string
}

// ToolHandler는 MCPServer의 tool 핸들러 함수이다.
type ToolHandler func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)

// ResourceHandler는 MCPServer의 resource 핸들러 함수이다.
type ResourceHandler func(ctx context.Context, uri string) (ResourceContent, error)

// PromptHandler는 MCPServer의 prompt 핸들러 함수이다.
type PromptHandler func(ctx context.Context, args map[string]string) (string, error)

// toolEntry는 MCPServer 내부 tool 등록 항목이다.
type toolEntry struct {
	schema  json.RawMessage
	handler ToolHandler
}

// --- 인터페이스 ---

// Transport는 MCP JSON-RPC 메시지 전송 인터페이스이다.
// REQ-MCP-004: stdio/WebSocket/SSE 공통 시그니처
//
// @MX:ANCHOR: [AUTO] Transport — MCP transport 공통 추상화 인터페이스
// @MX:REASON: REQ-MCP-004, AC-MCP-015 — 세 구현체 모두 이 인터페이스를 만족해야 함
type Transport interface {
	// SendRequest는 요청을 전송하고 응답을 반환한다.
	// ctx 취소 시 ErrTransportClosed 또는 ctx.Err()를 반환한다.
	SendRequest(ctx context.Context, req JSONRPCRequest) (JSONRPCResponse, error)
	// Notify는 응답을 기대하지 않는 알림 메시지를 전송한다.
	Notify(ctx context.Context, msg JSONRPCNotification) error
	// OnMessage는 서버 발송 메시지(알림)의 핸들러를 등록한다.
	// REQ-MCP-020: SSE 서버 발송 알림 처리에 사용
	OnMessage(handler func(JSONRPCMessage))
	// Close는 transport를 닫고 리소스를 해제한다.
	Close() error
}

// MCPClient는 외부 MCP 서버에 연결하는 클라이언트 인터페이스이다.
//
// @MX:ANCHOR: [AUTO] MCPClient — MCP 클라이언트 공개 인터페이스
// @MX:REASON: REQ-MCP-001..023 — 모든 MCP 클라이언트 동작의 단일 진입점
type MCPClient interface {
	ConnectToServer(ctx context.Context, cfg MCPServerConfig) (*ServerSession, error)
	ListTools(ctx context.Context, s *ServerSession) ([]MCPTool, error)
	CallTool(ctx context.Context, s *ServerSession, name string, input json.RawMessage) (ToolResult, error)
	ListResources(ctx context.Context, s *ServerSession) ([]MCPResource, error)
	ReadResource(ctx context.Context, s *ServerSession, uri string) (ResourceContent, error)
	ListPrompts(ctx context.Context, s *ServerSession) ([]MCPPrompt, error)
	Disconnect(s *ServerSession) error
	InvalidateToolCache(s *ServerSession)
}
