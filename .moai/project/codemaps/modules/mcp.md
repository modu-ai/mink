# mcp 패키지 — MCP Server/Client

**위치**: internal/mcp/, internal/mcp/transport/  
**파일**: 20개 (server.go, client.go, transport/*)  
**상태**: ✅ Active (SPEC-GOOSE-MCP-001)

---

## 목적

Model Context Protocol (stdio/WebSocket/SSE 3-transport). Claude와 외부 시스템 통합.

---

## 공개 API

### MCPServer
```go
type Server struct {
    resources   map[string]Resource
    tools       map[string]Tool
    prompts     map[string]Prompt
    transport   Transport
}

func (s *Server) HandleRequest(req Request) Response
    // 1. Dispatch by method
    // 2. Execute handler
    // 3. Return response
```

### MCPClient
```go
type Client struct {
    transport Transport
}

func (c *Client) ListResources(ctx context.Context) ([]Resource, error)
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error)
```

---

## 3-Transport

```go
type Transport interface {
    ReadRequest() (json.RawMessage, error)
    WriteResponse(json.RawMessage) error
    Close() error
}

// 1. Stdio: stdin/stdout (parent process communication)
// 2. WebSocket: tcp/ws (network)
// 3. SSE: http/sse (server-sent events)
```

---

## OAuth 2.1 Negotiation

```
Client initiates:
  1. Send /oauth/authorize request
  2. Get authorization URL
  3. User grants permission
  4. Exchange code for token
  5. Server stores token
  6. Client authenticated
```

---

## Capability Negotiation

```
Handshake:
  1. Client asks: "What capabilities?"
  2. Server responds: resources=[], tools=[...], prompts=[...]
  3. Client filters by supported version
  4. Both agree on subset
  5. Continue with agreed capabilities
```

---

**Version**: mcp v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~280
