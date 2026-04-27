# internal/mcp

**MCP (Model Context Protocol) Client/Server 패키지** — 외부 MCP 서버 연동 및 프로토콜 구현

## 개요

본 패키지는 AI.GOOSE의 **MCP(Model Context Protocol) 클라이언트/서버**를 구현합니다. 외부 MCP 서버에 연결하여 tool, resource, prompt를 검색하고 실행합니다. JSON-RPC 2.0 기반의 표준 MCP 프로토콜을 지원하며, stdio 및 SSE transport를 제공합니다.

## 핵심 구성 요소

### MCP Client

외부 MCP 서버에 연결하는 클라이언트:

```go
type Client struct {
    transport Transport
    servers   map[string]*ServerConfig
}

func NewClient(configs []*ServerConfig) (*Client, error)
func (c *Client) Connect(ctx context.Context, serverName string) error
func (c *Client) ListTools(ctx context.Context, server string) ([]Tool, error)
func (c *Client) CallTool(ctx context.Context, server string, name string, args map[string]any) (*CallResult, error)
func (c *Client) ListResources(ctx context.Context, server string) ([]Resource, error)
func (c *Client) Close() error
```

### MCP Server

GOOSE 자체를 MCP 서버로 노출:

```go
type Server struct {
    tools     []Tool
    resources []Resource
    handlers  map[string]HandlerFunc
}

func (s *Server) RegisterTool(tool Tool, handler HandlerFunc)
func (s *Server) Serve(transport Transport) error
```

### Transport

MCP 통신 transport 추상화:

```go
type Transport interface {
    Send(ctx context.Context, msg JSONRPCMessage) error
    Receive(ctx context.Context) (JSONRPCMessage, error)
    Close() error
}
```

지원 transport:
- **stdio**: stdin/stdout 파이프 (subprocess 기반)
- **SSE**: Server-Sent Events (HTTP 기반)

### Authentication

MCP 서버 인증 관리:

```go
type AuthConfig struct {
    Type     AuthType // none, api_key, oauth, bearer
    Token    string
    Header   string   // custom header name
}

func (a *AuthConfig) Apply(req *http.Request) error
```

### Types

MCP 프로토콜 타입 정의:

```go
type Tool struct {
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    InputSchema map[string]any `json:"inputSchema"`
}

type CallResult struct {
    Content []Content `json:"content"`
    IsError bool      `json:"isError,omitempty"`
}

type Resource struct {
    URI         string         `json:"uri"`
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    MimeType    string         `json:"mimeType,omitempty"`
}
```

## 서브패키지

| 패키지 | 설명 |
|--------|------|
| `transport/` | stdio, SSE transport 구현 |

## 파일 구조

```
internal/mcp/
├── client.go              # MCP 클라이언트
├── server.go              # MCP 서버
├── adapter.go             # 내부 tool ↔ MCP tool 변환
├── auth.go                # 인증 관리
├── credentials.go         # 자격 증명 저장
├── types.go               # MCP 프로토콜 타입
├── validation.go          # 입력 검증
├── errors.go              # 에러 정의
├── transport_factory.go   # Transport 팩토리
├── transport/             # Transport 구현체
└── testdata/              # 테스트 fixture
```

## 테스트

```bash
go test ./internal/mcp/...
```

현재 테스트 커버리지: **85%+** (client, server, auth, transport)

## 관련 SPEC

- **SPEC-GOOSE-MCP-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-TOOLS-001**: MCP tool을 내부 tool로 변환
- **SPEC-GOOSE-TRANSPORT-001**: Transport 계층

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-MCP-001
