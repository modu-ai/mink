---
id: SPEC-GOOSE-MCP-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 2
size: 대(L)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-MCP-001 — MCP Client/Server (stdio · WebSocket · SSE, OAuth 2.1, Deferred Loading)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-primitives §3 + ROADMAP v2.0 Phase 2 기반) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 **Model Context Protocol (MCP) 통합**을 정의한다. Claude Code와 동일하게 goosed는 **이중 역할**을 수행한다:

1. **MCP 클라이언트** — 외부 MCP 서버(e.g., github, filesystem, playwright)에 연결하여 해당 서버가 노출한 tools/resources/prompts를 `QueryEngine`의 tool inventory에 동적 편입.
2. **MCP 서버** — `createSdkMcpServer()` API로 사용자가 작성한 Go 프로그램이 MCP 서버로 동작하여 외부 호스트(Claude Desktop, Cursor 등)에 tools를 노출.

본 SPEC이 통과한 시점에서 `internal/mcp` 패키지는:

- **3 transport**(stdio, WebSocket, SSE)를 추상 인터페이스 `Transport`로 단일화하고,
- `MCPClient.ConnectToServer(cfg)`로 연결 풀(memoize) + **deferred tool manifest loading**(ToolSearch)을 수행하며,
- OAuth 2.1 + PKCE를 지원하는 원격 MCP 서버에 대해 브라우저 콜백 플로우를 중재하고,
- 이름 충돌을 `mcp__{serverName}__{toolName}` 접두사로 해결하며,
- `resources/list`, `resources/read`, `tools/list`, `tools/call`, `prompts/list` 5개 메서드를 처리하고,
- `MCPServer` 측에서는 `sdkmcp.NewServer().Tool(name, handler).Prompt(...)` 빌더 API로 외부 노출을 제공한다.

본 SPEC은 **JSON-RPC 2.0 wire 포맷의 직접 구현을 포함하지 않으며**, `modelcontextprotocol/go-sdk`(공식 Go SDK) 위에 GOOSE의 인터페이스를 올리는 래퍼 층을 규정한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Phase 2의 4 primitive 중 MCP는 **외부 생태계와의 단일 접점**. `TOOLS-001`(Phase 3)이 tool registry를 구성할 때 MCP 서버가 노출한 tool을 포함해야 하고, `SUBAGENT-001`이 agent-scoped MCP 서버를 초기화해야 한다.
- `.moai/project/research/claude-primitives.md` §3이 Claude Code의 MCP 아키텍처(이중 역할, 3 transport, OAuth, deferred loading, 이름 충돌 해결)를 제시한다. 본 SPEC은 그 구조를 Go로 포팅.
- `TRANSPORT-001`이 gRPC server streaming을 확보한 직후 필요. MCP의 stdio transport는 TRANSPORT-001의 추상 레이어와 분리된 별도 bytestream이지만, 인증·세션 관리 패턴은 재사용.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code TypeScript**: `tools/MCPTool/`, `mcpWebSocket*`, `mcpAuth*`. 언어 상이 직접 포트 없음 — 로직과 전송 선택만 번역.
- **Anthropic MCP 공식 스펙**: https://spec.modelcontextprotocol.io/ (버전 `2025-03-26`). 본 SPEC은 이 스펙 준수.
- **`modelcontextprotocol/go-sdk`** (공식 Go 구현체): 본 SPEC의 핵심 외부 의존성. wire 프로토콜, JSON-RPC 2.0, schema validation은 이 SDK가 담당.

### 2.3 범위 경계

- **IN**: `MCPClient`, `MCPServer` 인터페이스, `Transport` 추상화 + stdio/WebSocket/SSE 3구현, 연결 풀 + memoize, Deferred tool loading, OAuth 2.1 + PKCE 플로우(브라우저 콜백 포함), 이름 충돌 해결, `MCPTool`/`MCPResource`/`MCPPrompt` 타입, credential 저장(`~/.goose/mcp-credentials/`), `mcp.json` 사용자 설정 파일 스키마, 세션 재연결 정책.
- **OUT**: JSON-RPC 2.0 wire 레이어(go-sdk 위임), Tool 실행 결과 처리(→ TOOLS-001), Prompt → Skill 변환(→ SKILLS-001이 consumer), Hook 이벤트 dispatch(→ HOOK-001), MCP 서버 자체의 비즈니스 로직(사용자가 작성), Marketplace / MCPB 파일 로더(→ PLUGIN-001), gRPC 브릿지(→ TRANSPORT-001이 별도).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/mcp/` 패키지 생성.
2. 인터페이스: `MCPClient`, `MCPServer`, `Transport`, `AuthProvider`.
3. `Transport` 구현: `StdioTransport`, `WebSocketTransport`, `SSETransport`.
4. `MCPClient`:
   - `ConnectToServer(ctx, cfg MCPServerConfig) (ServerSession, error)` — memoized 연결.
   - `ListTools(ctx, session) ([]MCPTool, error)` — deferred (late binding).
   - `CallTool(ctx, session, name, input) (ToolResult, error)`.
   - `ListResources(ctx, session) ([]MCPResource, error)`.
   - `ReadResource(ctx, session, uri) (ResourceContent, error)`.
   - `ListPrompts(ctx, session) ([]MCPPrompt, error)`.
5. `MCPServer` 빌더 API: `NewServer(info)`, `.Tool(name, schema, handler)`, `.Prompt(name, args, template)`, `.Resource(uri, handler)`, `.Serve(ctx, transport)`.
6. OAuth 2.1 + PKCE: `AuthFlow.Start() → authURL`, `AuthFlow.HandleCallback(code, state) → tokens`, `TokenStore.Save/Load/Refresh`.
7. Credential 저장: `~/.goose/mcp-credentials/{server-id}.json` (file mode 0600).
8. `mcp.json` 스키마 — 사용자 설정 파일(global `~/.goose/mcp.json` + project `.goose/mcp.json`).
9. 이름 충돌 해결: `mcp__{serverName}__{toolName}`; 동일 server 내 동일 tool 이름은 에러.
10. 세션 재연결: exponential backoff + max 5 retry.
11. `PromptToSkill(MCPPrompt) (*skill.SkillDefinition, error)` 어댑터 — SKILLS-001 레지스트리에 MCP prompt를 편입.

### 3.2 OUT OF SCOPE

- **JSON-RPC 2.0 wire 실체**: `modelcontextprotocol/go-sdk`가 구현. 본 SPEC은 SDK 래핑만.
- **Tool 실행 로직**: `CallTool`은 MCP 호출만; 결과 해석/errror wrap은 TOOLS-001.
- **Prompt 실행**: `ListPrompts`는 metadata만. 실제 prompt를 QueryEngine에 주입하는 경로는 `PromptToSkill` → SKILLS-001 registry → QueryEngine consumer 체인.
- **Marketplace / MCPB 번들**: PLUGIN-001.
- **gRPC bridge(MCP → gRPC)**: TRANSPORT-001 후속.
- **Telemetry / quota**: 추후 SPEC.
- **Streaming tool 결과**: v1.0 이후. 현재 spec은 unary 응답만.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-MCP-001 [Ubiquitous]** — Every tool exposed by an external MCP server **shall** appear in GOOSE's tool inventory with the namespaced name `mcp__{serverName}__{toolName}`; collisions within the same server **shall** cause `ConnectToServer` to return `ErrDuplicateMCPToolName` for that server only.

**REQ-MCP-002 [Ubiquitous]** — The `MCPClient.ConnectToServer` function **shall** memoize successful connections keyed by `MCPServerConfig.ID` (hash of `Name + Transport + URI`); a second call with identical config **shall** return the existing `ServerSession` without re-establishing the transport.

**REQ-MCP-003 [Ubiquitous]** — MCP credentials (OAuth tokens) **shall** be stored under `~/.goose/mcp-credentials/{server-id}.json` with file mode `0600`; the process **shall** refuse to read a credential file whose mode exceeds `0600` and **shall** log a security warning.

**REQ-MCP-004 [Ubiquitous]** — All transport implementations (stdio / WebSocket / SSE) **shall** conform to the same `Transport` interface: `SendRequest(ctx, req) (resp, error)`, `Notify(ctx, msg) error`, `OnMessage(handler)`, `Close() error`.

### 4.2 Event-Driven (이벤트 기반)

**REQ-MCP-005 [Event-Driven]** — **When** `MCPClient.ConnectToServer(cfg)` is invoked for the first time with `cfg.Transport == "stdio"`, the client **shall** (a) spawn the subprocess defined by `cfg.Command` + `cfg.Args`, (b) pipe stdin/stdout, (c) perform the MCP `initialize` handshake with protocol version `2025-03-26`, (d) store the resulting `ServerSession` in the memoization cache, and (e) return the session.

**REQ-MCP-006 [Event-Driven]** — **When** `MCPClient.ListTools(ctx, session)` is called for the first time on a session, the client **shall** issue the MCP `tools/list` JSON-RPC call (deferred loading) and cache the result; subsequent calls within the same session **shall** return the cached list until `InvalidateToolCache(session)` is called.

**REQ-MCP-007 [Event-Driven]** — **When** the MCP server requires OAuth 2.1 authentication, `AuthFlow.Start()` **shall** (a) generate a PKCE code_verifier (32-byte random) + code_challenge (SHA256 base64url), (b) construct the authorization URL with `response_type=code`, `code_challenge`, `code_challenge_method=S256`, (c) open the user's browser to the URL, (d) wait on a local callback listener, and (e) exchange the received `code` for access + refresh tokens.

**REQ-MCP-008 [Event-Driven]** — **When** an access token expires (indicated by `401` response from the MCP server), the client **shall** attempt refresh using `refresh_token`; **if** refresh succeeds, the original request **shall** be retried transparently; **if** refresh fails with `invalid_grant`, `ListTools`/`CallTool` **shall** return `ErrReauthRequired` and **shall not** retry further.

**REQ-MCP-009 [Event-Driven]** — **When** a MCP server disconnects(transport error), the client **shall** attempt reconnection with exponential backoff (1s, 2s, 4s, 8s, 16s) up to 5 attempts; in-flight requests **shall** be failed with `ErrTransportReset` after the first backoff interval elapses without a successful reconnect.

**REQ-MCP-010 [Event-Driven]** — **When** `MCPServer.Serve(ctx, transport)` is invoked, the server **shall** (a) begin reading requests from the transport, (b) dispatch each request to the registered handler based on `method`, (c) serialize the handler's result as MCP response, (d) write to the transport, and (e) continue until `ctx.Done()` or the transport closes.

### 4.3 State-Driven (상태 기반)

**REQ-MCP-011 [State-Driven]** — **While** a `ServerSession` is in state `Connected` and `tools/list` result is cached, subsequent `ListTools` calls **shall** return from cache within 1ms; cache invalidation **shall** only occur via explicit `InvalidateToolCache` or session disconnection.

**REQ-MCP-012 [State-Driven]** — **While** an OAuth auth flow is pending (`AuthFlow.Start()` was called but `HandleCallback` has not completed), any `ListTools`/`CallTool` call on the same session **shall** block until the flow completes or timeout (60s); on timeout, `ErrAuthFlowTimeout` is returned.

**REQ-MCP-013 [State-Driven]** — **While** the user has configured `mcp.json` with `prompts: true` for a server, the `ListPrompts` call **shall** be invoked on first connection; the resulting prompts **shall** be passed to `PromptToSkill` and registered in the `skill.Registry` with `TriggerInline` + `ID = mcp__{server}__{prompt}`.

### 4.4 Unwanted Behavior (방지)

**REQ-MCP-014 [Unwanted]** — The stdio transport **shall not** leak child process handles; when the `ServerSession` is closed, the child process **shall** be sent SIGTERM, then SIGKILL after 5s grace period.

**REQ-MCP-015 [Unwanted]** — The WebSocket transport **shall not** accept self-signed TLS certificates unless the user has explicitly set `mcp.json:servers.<id>.tls.insecure = true`; default is strict validation using the system CA pool.

**REQ-MCP-016 [Unwanted]** — The server **shall not** expose tools whose name contains `/`, `:`, or double-underscore `__`; such names are reserved for namespacing and **shall** cause `MCPServer.Tool(name, ...)` to return `ErrReservedToolName`.

**REQ-MCP-017 [Unwanted]** — The `OAuth` flow **shall not** accept a callback whose `state` parameter does not match the original request's state; mismatched state **shall** cause `HandleCallback` to return `ErrOAuthStateMismatch` without exchanging the code.

**REQ-MCP-018 [Unwanted]** — The MCP client **shall not** silently downgrade protocol version; **if** the server reports a protocol version not in `SupportedProtocolVersions` (currently `["2025-03-26"]`), `ConnectToServer` **shall** return `ErrUnsupportedProtocolVersion`.

### 4.5 Optional (선택적)

**REQ-MCP-019 [Optional]** — **Where** `mcp.json:servers.<id>.env` is non-empty, the stdio transport **shall** inject those environment variables into the child process's environment, merged on top of the parent environment.

**REQ-MCP-020 [Optional]** — **Where** the SSE transport is used, the client **shall** support server-initiated notifications via the `message` event stream in addition to request/response pairs; notifications are dispatched to `Transport.OnMessage` handlers.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-MCP-001 — stdio MCP 서버 연결 + initialize 핸드셰이크**
- **Given** 테스트 fixture MCP 서버가 stdio로 기동, MCP protocol `2025-03-26` 지원
- **When** `ConnectToServer(ctx, {Name:"fx", Transport:"stdio", Command:"./fx-mcp"})`
- **Then** session이 `Connected` 상태, `initialize` 응답의 `protocolVersion == "2025-03-26"`, 에러 없음

**AC-MCP-002 — Deferred tool loading**
- **Given** fixture MCP 서버가 2개 tool(`search`, `fetch`) 제공
- **When** `ConnectToServer` 즉시에는 tool list fetch를 하지 않고, 이후 `ListTools`를 호출
- **Then** 첫 `ListTools`는 wire 트래픽 발생(`tools/list` 요청), 결과 `[mcp__fx__search, mcp__fx__fetch]`; 두 번째 `ListTools`는 wire 트래픽 없이 캐시 반환

**AC-MCP-003 — 이름 네임스페이싱**
- **Given** 서버 `fx`와 `gh` 모두 `search` tool 제공
- **When** 둘 다 connect → `ListTools` aggregate
- **Then** tool 목록에 `mcp__fx__search`와 `mcp__gh__search`가 둘 다 포함, 충돌 없음

**AC-MCP-004 — 단일 서버 내 tool 충돌**
- **Given** 서버 `fx`가 동일 이름 `search`를 2번 노출(변조된 fixture)
- **When** `ConnectToServer` 후 `ListTools`
- **Then** `ErrDuplicateMCPToolName` 반환, 해당 서버만 등록 실패, 다른 서버는 영향 없음

**AC-MCP-005 — OAuth 2.1 + PKCE**
- **Given** fixture OAuth 서버 + MCP 서버 combo, 브라우저 자동화 fixture(`mockBrowser`)
- **When** `ConnectToServer(... Auth: "oauth2")` → `AuthFlow.Start()` → mockBrowser가 콜백 호출 → `HandleCallback`
- **Then** `~/.goose/mcp-credentials/fx.json`에 `access_token`+`refresh_token` 저장(file mode 0600), 세션은 Connected

**AC-MCP-006 — Token 만료 시 자동 refresh**
- **Given** fixture 서버가 `401`을 첫 호출에 반환하고, refresh 후 두 번째 호출은 성공
- **When** `CallTool(ctx, session, "mcp__fx__search", {...})`
- **Then** 내부적으로 refresh 수행, 외부 호출자에게는 단일 응답 반환(재시도 투명), zap 로그에 refresh 이벤트 기록

**AC-MCP-007 — 재연결 백오프**
- **Given** 서버가 10초간 다운, 이후 복구
- **When** 다운 구간에 `CallTool` 호출
- **Then** 백오프 1s/2s/4s/... 시도, 총 5회 이후 `ErrTransportReset`; 테스트 fixture가 3회차에 복구되는 경우에는 정상 응답 반환

**AC-MCP-008 — WebSocket 기본 strict TLS**
- **Given** MCP 서버가 self-signed 인증서, `mcp.json:tls.insecure` 미설정
- **When** `ConnectToServer`
- **Then** `ErrTLSValidation` 반환, credential 저장 없음

**AC-MCP-009 — MCPServer 빌더로 tool 노출**
- **Given** `NewServer("test-srv").Tool("echo", schema, handler).Serve(ctx, stdio)`
- **When** 테스트 클라이언트가 동일 stdio로 `tools/list` 호출
- **Then** 응답에 `echo` tool의 name/schema가 포함, `tools/call(echo, {"x":1})`이 handler의 결과를 반환

**AC-MCP-010 — Reserved tool name 거부**
- **Given** 사용자 코드가 `.Tool("mcp__evil", ...)` 호출
- **When** 빌더 API
- **Then** `ErrReservedToolName` 반환, 서버는 구동되지 않음

**AC-MCP-011 — Protocol version 불일치**
- **Given** 서버가 `protocolVersion: "2024-01-01"`을 `initialize` 응답에서 반환
- **When** `ConnectToServer`
- **Then** `ErrUnsupportedProtocolVersion` 반환, 세션 생성 안 됨

**AC-MCP-012 — Prompt → Skill 변환**
- **Given** MCP 서버가 prompt `{name: "greet", arguments: [{name:"lang"}]}` 제공, `mcp.json:prompts=true`
- **When** `ConnectToServer` 후 `PromptToSkill` invocation
- **Then** `skill.Registry.Get("mcp__fx__greet")` 존재, `Trigger == TriggerInline`, `Frontmatter.ArgumentHint == "lang"`

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/
└── mcp/
    ├── client.go            # MCPClient 구조체 + ConnectToServer, ListTools, CallTool
    ├── server.go            # MCPServer 빌더 API + Serve
    ├── transport/
    │   ├── transport.go     # Transport 인터페이스
    │   ├── stdio.go         # StdioTransport
    │   ├── websocket.go     # WebSocketTransport
    │   └── sse.go           # SSETransport
    ├── auth.go              # AuthProvider + OAuth2.1 + PKCE
    ├── credentials.go       # ~/.goose/mcp-credentials/ I/O
    ├── config.go            # mcp.json 스키마 + 로더
    ├── validation.go        # 스키마 검증 (reserved names, collisions)
    ├── registry.go          # ServerSession 레지스트리 + memoize
    └── adapter.go           # PromptToSkill, MCPTool→internal 변환
```

### 6.2 핵심 Go 타입

```go
// Transport 추상화: stdio/WebSocket/SSE 공통 인터페이스.
type Transport interface {
    SendRequest(ctx context.Context, req JSONRPCRequest) (JSONRPCResponse, error)
    Notify(ctx context.Context, msg JSONRPCNotification) error
    OnMessage(handler func(JSONRPCMessage))
    Close() error
}

type MCPServerConfig struct {
    ID        string            // 연결 memoize key (해시)
    Name      string
    Transport string            // "stdio" | "websocket" | "sse"
    Command   string            // stdio only
    Args      []string
    Env       map[string]string
    URI       string            // websocket/sse only
    TLS       *TLSConfig
    Auth      *AuthConfig       // "none" | "oauth2" | "bearer"
    Prompts   bool              // true → PromptToSkill 실행
}

type ServerSession struct {
    ID              string
    Config          MCPServerConfig
    Transport       Transport
    State           SessionState     // Connected | Reconnecting | Disconnected | AuthPending
    ProtocolVersion string
    tools           []MCPTool
    toolsLoaded     bool
    mu              sync.RWMutex
    logger          *zap.Logger
}

type MCPTool struct {
    Name        string            // namespaced: "mcp__{server}__{tool}"
    Description string
    InputSchema json.RawMessage   // JSON Schema
    ServerID    string
}

type MCPResource struct {
    URI         string
    Name        string
    Description string
    MimeType    string
}

type MCPPrompt struct {
    Name        string
    Description string
    Arguments   []PromptArgument
    Template    string
}

// 클라이언트 API.
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

// 서버 API (빌더).
type MCPServer struct {
    info     ServerInfo
    tools    map[string]ToolHandler
    resources map[string]ResourceHandler
    prompts  map[string]PromptHandler
}

func NewServer(info ServerInfo) *MCPServer
func (s *MCPServer) Tool(name string, schema json.RawMessage, h ToolHandler) *MCPServer
func (s *MCPServer) Resource(uri string, h ResourceHandler) *MCPServer
func (s *MCPServer) Prompt(name string, args []PromptArgument, h PromptHandler) *MCPServer
func (s *MCPServer) Serve(ctx context.Context, t Transport) error

// OAuth 플로우.
type AuthFlow struct {
    ClientID     string
    AuthURL      string
    TokenURL     string
    RedirectURI  string
    Scopes       []string
    verifier     string    // PKCE code_verifier
    state        string    // CSRF state
}

func (f *AuthFlow) Start(ctx context.Context) (authURL string, err error)
func (f *AuthFlow) HandleCallback(code, state string) (*TokenSet, error)

type TokenSet struct {
    AccessToken  string
    RefreshToken string
    ExpiresAt    time.Time
    Scope        string
}
```

### 6.3 Transport 선택 규칙

| Config | Transport | 사용 사례 |
|--------|-----------|----------|
| `command` 지정 | stdio | 로컬 subprocess MCP 서버 (filesystem, github-mcp 등) |
| `uri: wss://...` | WebSocket | 원격 MCP 서버 (authenticated, long-lived) |
| `uri: https://... + events: sse` | SSE | 원격 streaming read-only 서버 |

### 6.4 Deferred Loading 규약

1. `ConnectToServer`는 transport 연결 + `initialize`까지만 수행. `tools/list`는 호출하지 않음.
2. 첫 `ListTools`/`CallTool` 시점에 `tools/list`를 invoke, 결과 캐시.
3. `CallTool(name)` 호출 시 `tools` 캐시를 체크하여 `name`이 등록되지 않은 경우 `ErrToolNotFound` (late error이지만 허용된 지연).

이유: 50개 MCP 서버 연결 시 50개 list call을 eager 수행하면 초기 지연 폭증. Claude Code의 ToolSearch 패턴 계승.

### 6.5 OAuth 2.1 + PKCE

1. `code_verifier` = 32 bytes crypto/rand → base64url.
2. `code_challenge` = SHA256(verifier) → base64url.
3. Authorization URL: `{authURL}?response_type=code&client_id=...&redirect_uri=http://localhost:{port}/callback&code_challenge={challenge}&code_challenge_method=S256&state={random}`.
4. `exec.Command("open"|"xdg-open", url)` — OS 브라우저.
5. Local HTTP listener on random port 수신 `code` + `state`.
6. 검증: `state` 매칭 필수 (REQ-MCP-017).
7. Token exchange: POST `{tokenURL}` with `grant_type=authorization_code`, `code`, `code_verifier`, `redirect_uri`.
8. 저장: `~/.goose/mcp-credentials/{server_id}.json` file mode 0600.

### 6.6 mcp.json 스키마

```json
{
  "servers": {
    "github": {
      "transport": "stdio",
      "command": "mcp-github",
      "args": ["--config", "~/.github-token"],
      "env": {"GITHUB_API": "https://api.github.com"},
      "prompts": true
    },
    "anthropic-cloud": {
      "transport": "websocket",
      "uri": "wss://mcp.anthropic.com/v1",
      "auth": {"type": "oauth2", "clientId": "...", "authURL": "...", "tokenURL": "...", "scopes": ["mcp:read", "mcp:tools"]},
      "tls": {"insecure": false}
    }
  }
}
```

Schema validation은 CONFIG-001의 loader에서 1차(struct tag), 본 SPEC의 `validation.go`에서 2차(reserved names, duplicate IDs, transport-specific field presence).

### 6.7 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| MCP 프로토콜 (JSON-RPC 2.0) | `github.com/modelcontextprotocol/go-sdk` | 공식 Go SDK. wire 레이어 + schema 검증 위임 |
| WebSocket | `github.com/coder/websocket` (前 `nhooyr.io/websocket`) | context 통합, 유지보수 활성 |
| SSE | stdlib `net/http` + 직접 reader | 외부 의존 최소화 |
| JSON | stdlib `encoding/json` | |
| OAuth | 직접 구현 + PKCE (외부 OAuth 라이브러리 회피) | 최소 scope, 투명성 |
| TLS 검증 | stdlib `crypto/tls` | |
| 로깅 | `go.uber.org/zap` | CORE-001 계승 |

**의도적 미사용**:
- `golang.org/x/oauth2`: PKCE 지원이 제한적, mcp 특화 플로우와 충돌.
- `gorilla/websocket`: maintenance mode, `coder/websocket` 권장.

### 6.8 TDD 진입 순서

1. **RED #1** — `TestTransport_StdioHandshake` (AC-MCP-001)
2. **RED #2** — `TestMCPClient_ListTools_Deferred` (AC-MCP-002)
3. **RED #3** — `TestMCPClient_NameNamespacing` (AC-MCP-003)
4. **RED #4** — `TestMCPClient_DuplicateToolName_Error` (AC-MCP-004)
5. **RED #5** — `TestOAuthFlow_PKCEExchange` (AC-MCP-005)
6. **RED #6** — `TestMCPClient_TokenRefresh_Transparent` (AC-MCP-006)
7. **RED #7** — `TestMCPClient_Reconnect_ExponentialBackoff` (AC-MCP-007)
8. **RED #8** — `TestMCPClient_TLS_StrictDefault` (AC-MCP-008)
9. **RED #9** — `TestMCPServer_Builder_Tool` (AC-MCP-009)
10. **RED #10** — `TestMCPServer_ReservedToolName_Error` (AC-MCP-010)
11. **RED #11** — `TestMCPClient_ProtocolVersionMismatch` (AC-MCP-011)
12. **RED #12** — `TestAdapter_PromptToSkill` (AC-MCP-012)
13. **GREEN** — go-sdk 래퍼 + transport 3구현 + OAuth.
14. **REFACTOR** — transport 중복 제거, session state 머신 추출.

### 6.9 TRUST 5 매핑

| 차원 | 본 SPEC 달성 방법 |
|-----|-----------------|
| **T**ested | 35+ unit test, 12 integration test (AC 1:1), httptest/exec.Cmd fixture |
| **R**eadable | 패키지 세분화(client/server/transport/auth/credentials/config), `Transport` 인터페이스로 계층 분리 |
| **U**nified | `go fmt`, `golangci-lint` (gosec 추가 — OAuth/TLS 검증) |
| **S**ecured | file mode 0600 enforcement, strict TLS default, PKCE 필수, state mismatch 거부, reserved names 차단 |
| **T**rackable | 세션 ID/server ID 기반 zap 구조화 로그, OAuth 플로우 단계별 로그, transport reset 이벤트 기록 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-TRANSPORT-001 | gRPC transport 기반 인터셉터 패턴(logging/recovery) 공유 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `mcp.json` loader 통합, feature gate |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap 로거, context 루트, graceful shutdown |
| 후속 SPEC | SPEC-GOOSE-TOOLS-001 | MCP tool을 tool registry에 편입하는 consumer |
| 후속 SPEC | SPEC-GOOSE-SKILLS-001 | `PromptToSkill` 결과 등록 |
| 후속 SPEC | SPEC-GOOSE-SUBAGENT-001 | agent-scoped MCP 서버 초기화 |
| 후속 SPEC | SPEC-GOOSE-PLUGIN-001 | plugin manifest `mcpServers:` 로딩 |
| 외부 | Go 1.22+ | context, generics |
| 외부 | `github.com/modelcontextprotocol/go-sdk` | 공식 MCP SDK |
| 외부 | `github.com/coder/websocket` | WebSocket transport |
| 외부 | `go.uber.org/zap` | 로깅 |
| 외부 | `github.com/stretchr/testify` | 테스트 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | `modelcontextprotocol/go-sdk`가 API breaking change | 중 | 고 | semver pin, 본 SPEC 인터페이스가 SDK 일대일 포팅 아니라 abstract layer 유지. v1.0 release 이전 바운드는 go.mod에서 명시 |
| R2 | OAuth 콜백 리스너가 포트 충돌로 실패 | 중 | 중 | `net.Listen(":0")`으로 OS 자동 할당, `redirect_uri`에 실제 포트 주입 |
| R3 | stdio subprocess가 deadlock(stdin/stdout 버퍼링) | 고 | 중 | bufio + 별도 고루틴, 주기적 keepalive, SIGTERM/SIGKILL grace period 5s |
| R4 | TLS self-signed 서버에서 사용자 설정 실수로 exposure | 중 | 고 | 기본 strict. `tls.insecure=true`는 설정 시 zap WARN 로그 + 경고 메시지 |
| R5 | Token 저장소가 멀티 프로세스 race | 낮 | 중 | file lock(`golang.org/x/sys/unix.Flock` linux/mac only) 또는 atomic rename. mvp는 single-process 가정 |
| R6 | 이름 충돌 해결 규칙이 tool 50개+ 시 prompt 길이 폭증 | 낮 | 낮 | tool 이름에 `mcp__` 접두사가 평균 8자 추가. 50개 기준 +400 tokens. 허용 |
| R7 | Deferred loading으로 tool 존재 여부 late error | 중 | 낮 | `CallTool` 첫 호출 시 자동 `ListTools` prefetch 옵션 제공(`cfg.EagerToolFetch`) |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/claude-primitives.md` §3 MCP System, §3.1 이중 역할, §3.2 Transport & 프로토콜, §3.3 OAuth, §3.4 Deferred Loading
- `.moai/specs/ROADMAP.md` §4 Phase 2 row 12 (MCP-001)
- `.moai/specs/SPEC-GOOSE-TRANSPORT-001/spec.md` — gRPC 패턴 (인터셉터 참고)
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` — 설정 로더

### 9.2 외부 참조

- MCP 공식 스펙: https://spec.modelcontextprotocol.io/ (버전 `2025-03-26`)
- `modelcontextprotocol/go-sdk`: https://github.com/modelcontextprotocol/go-sdk
- OAuth 2.1 draft: https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/
- PKCE RFC 7636: https://datatracker.ietf.org/doc/html/rfc7636
- `coder/websocket` (前 nhooyr.io/websocket): https://github.com/coder/websocket

### 9.3 부속 문서

- `./research.md` — claude-primitives.md §3 인용, go-sdk API 조사, OAuth 2.1 PKCE 상세
- `../SPEC-GOOSE-TRANSPORT-001/spec.md`
- `../SPEC-GOOSE-TOOLS-001/spec.md` (Phase 3)

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **JSON-RPC 2.0 wire 프로토콜을 직접 구현하지 않는다**. `modelcontextprotocol/go-sdk` 위임.
- 본 SPEC은 **MCP tool의 실행 semantics를 정의하지 않는다**. `CallTool`은 투명한 proxy. 결과 해석/wrap은 TOOLS-001.
- 본 SPEC은 **Marketplace / MCPB 번들 로딩을 포함하지 않는다**. PLUGIN-001.
- 본 SPEC은 **MCP resource의 캐싱 정책을 정의하지 않는다**. 매 `ReadResource`는 wire 호출. 캐싱은 후속 최적화 SPEC.
- 본 SPEC은 **Streaming tool 결과를 지원하지 않는다**. v1.0 이후.
- 본 SPEC은 **OAuth 1.0 / Bearer 이외의 인증 방식을 지원하지 않는다**. mTLS, API key rotation 등은 별도 SPEC.
- 본 SPEC은 **MCP 서버의 권한/정책 시스템을 정의하지 않는다**. 호스트(CallTool caller) 측 정책은 HOOK-001의 `useCanUseTool` 플로우가 담당.
- 본 SPEC은 **MCP 서버 비즈니스 로직을 포함하지 않는다**. `MCPServer` 빌더 API는 껍데기; handler 내부 구현은 사용자 책임.
- 본 SPEC은 **MCP protocol version 업그레이드 자동 협상을 지원하지 않는다**. 단일 지원 버전(`2025-03-26`)만; 불일치 시 에러.
- 본 SPEC은 **Telemetry / quota / rate limit을 구현하지 않는다**. 후속 SPEC.

---

**End of SPEC-GOOSE-MCP-001**
