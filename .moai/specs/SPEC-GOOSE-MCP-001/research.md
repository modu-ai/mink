# SPEC-GOOSE-MCP-001 — Research & Porting Analysis

> **목적**: Claude Code의 MCP(Model Context Protocol) 이중 역할 시스템 → GOOSE Go 포팅 계약. `.moai/project/research/claude-primitives.md` §3의 분석을 본 SPEC REQ와 1:1 매핑한다.
> **작성일**: 2026-04-21
> **범위**: `internal/mcp/` + `internal/mcp/transport/` 2개 하위 패키지.

---

## 1. 레포 현재 상태 스캔

```
/Users/goos/MoAI/AgentOS/
├── claude-code-source-map/   # tools/MCPTool/, mcpWebSocket*, mcpAuth* 확인
├── hermes-agent-main/        # MCP 자체 구현 없음 (cli.py 중심)
└── .moai/specs/              # TRANSPORT-001 (gRPC) 합의 완료
```

- `internal/mcp/` → **전부 부재**. Phase 2 신규.
- Claude Code source map 내 MCP 관련 파일 다수(WebSocket, stdio, auth, tools dispatch). **언어 상이로 직접 포트 불가** — 공식 Go SDK가 primary 의존성.

**결론**: GREEN 단계는 `internal/mcp/` 9개 파일 **zero-to-one** 신규 작성 + `modelcontextprotocol/go-sdk` 위에 GOOSE 인터페이스 레이어.

---

## 2. claude-primitives.md §3 원문 인용 → REQ 매핑

### 2.1 이중 역할 (§3.1)

원문:

> **클라이언트** (Claude Code → 외부 MCP 서버):
> - `mcpClient.ts`: SSE, WebSocket, stdio 전송
> - `connectToServer()` memoize (연결 풀링)
> - `fetchToolsForClient()`: 도구 매니페스트 fetch (deferred)
>
> **서버** (Claude Code → SDK 프로그램):
> - `server/` 번들
> - `createSdkMcpServer()`: tool() + handlers
> - WebSocket over stdio

| 원문 | 본 SPEC REQ | Go 매핑 |
|---|---|---|
| `connectToServer()` memoize | REQ-MCP-002 | `MCPClient.ConnectToServer` + `registry.go` memoize |
| 3 transport (stdio/WS/SSE) | REQ-MCP-004, REQ-MCP-005 | `Transport` 인터페이스 + 3 구현 |
| `fetchToolsForClient()` deferred | REQ-MCP-006, REQ-MCP-011 | `ListTools` + 캐시 |
| `createSdkMcpServer().tool()` | REQ-MCP-010, REQ-MCP-016 | `NewServer().Tool(...)` 빌더 |

### 2.2 Transport & 프로토콜 (§3.2)

원문 TS 스니펫:

```typescript
export class WebSocketTransport implements Transport {
  async request(req) → res
  async notify(msg) → void
  async close() → void
}

// MCP 메시지 타입:
- resources/read, resources/list
- tools/list, tools/call
- prompts/list
```

→ 본 SPEC의 `Transport` Go 인터페이스가 동일 시그니처를 이식:

```go
type Transport interface {
    SendRequest(ctx, req JSONRPCRequest) (JSONRPCResponse, error)
    Notify(ctx, msg JSONRPCNotification) error
    OnMessage(handler func(JSONRPCMessage))
    Close() error
}
```

Go는 non-async이므로 `async`가 제거되고 `context.Context` 인자로 대체. `OnMessage`는 handler 등록형으로 — SSE 같은 서버 iniatiated 알림 대응.

### 2.3 OAuth 흐름 (§3.3)

원문:

```
1. OAuth 플로우 시작 → 브라우저
2. callback_url → code + state
3. Token exchange (백그라운드)
4. 저장: ~/.claude/mcp-credentials/{server-id}
5. 갱신: 자동 refresh_token
```

→ **REQ-MCP-007, REQ-MCP-008**. Go 포팅:

- 경로 `~/.claude/` → `~/.goose/` (프로젝트 convention).
- PKCE는 claude-primitives.md에 명시 안 됨. OAuth 2.1 스펙(draft IETF)을 따라 PKCE 필수 강제.
- Token refresh는 background goroutine 아닌 lazy(401 수신 시) 전략 채택(Go에서 timer goroutine 누수 방지).

### 2.4 Deferred Loading (§3.4)

원문:

```
1. connectToServer() 호출 (도구 매니페스트 미로드)
2. 모델이 "mcp__..." 도구 언급 또는 MCPTool 직접 호출
3. Late binding: fetchToolsForClient()
4. 캐싱: memoize로 재연결 방지
```

→ **REQ-MCP-006**. 50+ MCP 서버 시나리오에서 eager fetch는 초기 지연 폭증. 본 SPEC은 lazy fetch 채택.

**이름 충돌**: `mcp__{serverName}__{toolName}` 접두사 → **REQ-MCP-001**.

---

## 3. Go 포팅 매핑표 (claude-primitives.md §7)

| Claude Code (TS) | GOOSE (Go) | 결정 |
|---|---|---|
| `tools/MCPTool/` | `internal/mcp/adapter.go:PromptToSkill` + tool namespacing | MCPTool은 TOOLS-001 consumer |
| `mcpWebSocket*` | `internal/mcp/transport/websocket.go` | `coder/websocket` 기반 |
| `mcpAuth*` | `internal/mcp/auth.go` + `credentials.go` | PKCE 직접 구현 |
| `connectToServer()` memoize | `MCPClient.ConnectToServer` + `registry.go` | `sync.Map` + config hash |
| `createSdkMcpServer()` | `NewServer(...).Tool(...).Serve(...)` | 빌더 패턴 |

---

## 4. Go 이디엄 선택 (상세 근거)

### 4.1 공식 `modelcontextprotocol/go-sdk` 채택

**채택 이유**:

- JSON-RPC 2.0 wire 레이어를 직접 구현할 경우 ~800 LoC + 정확성 리스크(protocol drift).
- MCP spec `2025-03-26` 업데이트 시 SDK가 흡수.
- Anthropic 공식 지원 → 장기 유지보수 보장.
- GOOSE는 이 SDK 위에 wrapper 층만 유지 → GOOSE 자체 인터페이스 변경 없이 MCP 버전 이동 가능.

**리스크 (R1)**: SDK가 아직 v0.x. breaking change 가능성.

**완화**:

- `go.mod`에 정확한 버전 pin.
- GOOSE 인터페이스(`MCPClient`, `MCPServer`, `Transport`)는 SDK 타입에 종속되지 않도록 추상화.
- SDK 업그레이드는 독립 PR + 통합 테스트 통과 후.

### 4.2 WebSocket 라이브러리: `coder/websocket`

`nhooyr.io/websocket`이 2024년 maintainer 변경으로 `coder/websocket`으로 이전됨. 대안 검토:

| 라이브러리 | 장점 | 단점 | 결정 |
|---|---|---|---|
| `coder/websocket` | context 통합, 활성 유지보수, stdlib-like API | 비교적 신규 | **채택** |
| `gorilla/websocket` | 성숙 | archived, context 통합 약함 | 탈락 |
| stdlib `net/websocket` | — | 낙후됨 | 탈락 |

### 4.3 OAuth 자체 구현 이유

`golang.org/x/oauth2`는:

- PKCE 지원이 ad-hoc (수동 verifier/challenge 주입 필요).
- 자동 refresh의 goroutine 생명주기 불투명.
- MCP 특유의 브라우저 콜백 flow와 맞지 않음.

본 SPEC은 ~200 LoC의 직접 구현을 선택:

- 투명한 flow 단계 (start → callback → exchange).
- Refresh는 401 수신 시점에 명시적 호출(goroutine 없음).
- Credential 저장은 파일 I/O만.

### 4.4 stdio subprocess 관리

- `os/exec.Command` + stdin/stdout `io.Pipe`.
- stderr는 별도 goroutine이 zap 로그로 포워드.
- 종료: `SendSignal(SIGTERM)` → 5s wait → `Kill()`.
- 부모 프로세스 종료 시 잔존 child: `syscall.SysProcAttr{Setpgid: true}` + process group kill (linux/mac).

### 4.5 Memoize 구현

`connectToServer`의 memoize는:

```go
type sessionRegistry struct {
    mu       sync.Mutex
    sessions map[string]*ServerSession
}

func (r *sessionRegistry) getOrConnect(cfg MCPServerConfig) (*ServerSession, error) {
    r.mu.Lock()
    if s, ok := r.sessions[cfg.ID]; ok && s.State == Connected {
        r.mu.Unlock()
        return s, nil
    }
    r.mu.Unlock()
    // slow path: connect (lock-free)
    s, err := connect(cfg)
    if err != nil { return nil, err }
    r.mu.Lock()
    defer r.mu.Unlock()
    r.sessions[cfg.ID] = s
    return s, nil
}
```

Double-check locking — Go memory model 준수.

---

## 5. 참조 가능한 외부 자산 분석

### 5.1 Claude Code TypeScript (`./claude-code-source-map/`)

- `tools/MCPTool/`, `components/mcp*` 파일 다수. 직접 포트 대상 아님 — claude-primitives.md §3 요약이 primary source.
- 특히 `mcpAuth*.ts`가 OAuth flow 세부 단계 참고용.
- **직접 포트 대상 없음**.

### 5.2 `modelcontextprotocol/go-sdk` (외부)

- 공식 Go SDK. 현재 alpha(v0.x).
- 제공 기능: JSON-RPC 2.0, message types, schema validation, base transport helpers.
- GOOSE 위임 영역: wire format, message marshaling, schema check.
- GOOSE 자체 구현: connection lifecycle, OAuth, credentials, mcp.json loader, namespace 충돌 해결, deferred loading 정책.

### 5.3 Anthropic MCP 스펙 (공식)

https://spec.modelcontextprotocol.io/ 버전 `2025-03-26`:

- `initialize` 핸드셰이크
- `tools/list`, `tools/call`
- `resources/list`, `resources/read`, `resources/subscribe`
- `prompts/list`, `prompts/get`
- `notifications/*` (서버 → 클라이언트)

본 SPEC은 이 중 `resources/subscribe`와 `notifications/*`는 OUT (추후 SPEC). `initialize`, `tools/*`, `resources/list|read`, `prompts/list`만 구현.

---

## 6. 외부 의존성 합계

| 모듈 | 버전 | 본 SPEC 사용 | 결정 근거 |
|------|-----|-----------|---------|
| `github.com/modelcontextprotocol/go-sdk` | pin TBD (v0.x alpha) | ✅ JSON-RPC 위임 | 공식 SDK |
| `github.com/coder/websocket` | v1.8+ | ✅ WebSocket transport | nhooyr 후속 |
| `go.uber.org/zap` | v1.27+ | ✅ 로깅 | CORE-001 계승 |
| `github.com/stretchr/testify` | v1.9+ | ✅ 테스트 | CORE-001 계승 |
| Go stdlib `net/http` | 1.22+ | ✅ SSE, OAuth callback | 표준 |
| Go stdlib `os/exec` | 1.22+ | ✅ stdio subprocess | 표준 |
| Go stdlib `crypto/tls`, `crypto/sha256`, `crypto/rand` | 1.22+ | ✅ TLS + PKCE | 표준 |
| Go stdlib `encoding/json` | 1.22+ | ✅ mcp.json + tool schema | 표준 |

**의도적 미사용**:

- `golang.org/x/oauth2`: PKCE flow 제약, 본 SPEC은 직접 구현.
- `gorilla/websocket`: archived, `coder/websocket` 권장.
- `github.com/golang-jwt/jwt`: MCP는 OAuth access token 자체를 opaque로 취급. JWT 파싱 불필요.

---

## 7. 테스트 전략 (TDD RED → GREEN)

### 7.1 Unit 테스트 (30~40개)

**Transport 레이어**:
- `TestStdioTransport_Roundtrip` — fixture 에코 서버 subprocess
- `TestStdioTransport_Close_SendsSIGTERM`
- `TestWebSocketTransport_Roundtrip` — `httptest.Server` + upgrade
- `TestWebSocketTransport_StrictTLS_RejectSelfSigned`
- `TestSSETransport_ServerInitiatedEvents`

**Auth 레이어**:
- `TestPKCE_VerifierChallenge_RoundTrip` — SHA256 + base64url
- `TestAuthFlow_State_Mismatch_Rejected`
- `TestAuthFlow_CodeExchange_Success`
- `TestCredentials_FileMode0600_Enforced`

**Client 레이어**:
- `TestMCPClient_ConnectToServer_Memoizes`
- `TestMCPClient_InvalidProtocolVersion_Rejects`
- `TestMCPClient_ListTools_Cached` — 2번째 호출 wire 없음
- `TestMCPClient_InvalidateCache_NextCallFetches`
- `TestMCPClient_CallTool_NotFound_LateError`
- `TestMCPClient_Reconnect_5RetryLimit`

**Server 레이어**:
- `TestMCPServer_Builder_Tool_Register`
- `TestMCPServer_Builder_ReservedName_Rejected`
- `TestMCPServer_Serve_DispatchesRequests`

**Adapter 레이어**:
- `TestPromptToSkill_ArgumentMapping`
- `TestNamespacing_mcp_serverName_toolName`

### 7.2 Integration 테스트 (AC 1:1, build tag `integration`)

| AC | Test |
|---|---|
| AC-MCP-001 | `TestMCP_Stdio_InitializeHandshake` |
| AC-MCP-002 | `TestMCP_ListTools_Deferred` |
| AC-MCP-003 | `TestMCP_Namespacing_AcrossServers` |
| AC-MCP-004 | `TestMCP_DuplicateToolName_WithinServer` |
| AC-MCP-005 | `TestMCP_OAuth_PKCE_EndToEnd` — mockBrowser |
| AC-MCP-006 | `TestMCP_TokenRefresh_Transparent` |
| AC-MCP-007 | `TestMCP_Reconnect_ExponentialBackoff` |
| AC-MCP-008 | `TestMCP_WSS_SelfSigned_Rejected` |
| AC-MCP-009 | `TestMCPServer_Stdio_ExposedTool` |
| AC-MCP-010 | `TestMCPServer_ReservedToolName` |
| AC-MCP-011 | `TestMCP_ProtocolVersionMismatch` |
| AC-MCP-012 | `TestMCP_PromptToSkill_Registered` |

### 7.3 Fixture 전략

- `testdata/mcp-fixture-server/` — Go로 작성한 minimal MCP server (stdio) — `tools/list` + `tools/call` 에코.
- `testdata/oauth-fixture/` — httptest.Server로 OAuth 2.1 token endpoint 시뮬레이션.
- `mockBrowser` — 사용자 브라우저 오픈 대신 goroutine이 callback URL에 GET.

### 7.4 커버리지 목표

- `internal/mcp/`: 88%+
- `internal/mcp/transport/`: 90%+ (transport 교차 테스트 중요)
- `-race` 필수 (memoize registry, session state machine).

---

## 8. 오픈 이슈

1. **go-sdk API 안정성**: v0.x alpha. 본 SPEC은 wrapper만 의존, 직접 타입 노출 최소화.
2. **OAuth 콜백 listener 포트**: 기본 `:0`(OS 할당). 방화벽 환경에서는 사용자가 `mcp.json:auth.callbackPort` 명시.
3. **Credential rotation 주기**: Refresh는 lazy(401 시). eager rotation(e.g., 만료 5분 전)은 후속 SPEC.
4. **MCP resource subscribe/notifications**: 본 SPEC OUT. Phase 3+ SPEC에서 도입.
5. **MCPServer concurrent handler 제한**: 기본 unbounded. backpressure는 `Serve` 구현에서 worker pool 옵션 제공 여부 추후 결정.
6. **Streaming tool 응답**: MCP spec에 partial result(`tools/call` 진행률)는 명시되지 않음. 추가되면 SDK 업그레이드 후 본 SPEC 후속 버전에서 도입.
7. **Multi-process credential race**: `~/.goose/mcp-credentials/` file lock은 단일 플랫폼. Windows 포팅 시 `golang.org/x/sys/windows`.

---

## 9. 구현 규모 예상

| 영역 | 파일 수 | 신규 LoC | 테스트 LoC |
|---|---|---|---|
| `client.go` | 1 | 350 | 500 |
| `server.go` | 1 | 250 | 350 |
| `transport/` (3 구현 + 인터페이스) | 4 | 500 | 700 |
| `auth.go` + PKCE | 1 | 220 | 350 |
| `credentials.go` | 1 | 120 | 180 |
| `config.go` (mcp.json) | 1 | 150 | 200 |
| `validation.go` | 1 | 120 | 220 |
| `registry.go` (memoize) | 1 | 100 | 200 |
| `adapter.go` (PromptToSkill) | 1 | 80 | 150 |
| **합계** | **12** | **~1,890** | **~2,850** |

테스트 비율: 60% (TDD 1:1+ 충족).

---

## 10. 결론

- **상속 자산**: TypeScript source map 설계 참조만. JSON-RPC wire는 `modelcontextprotocol/go-sdk` 위임.
- **핵심 결정**:
  - 공식 go-sdk 래핑 층 유지 → 본 SPEC 인터페이스는 SDK 타입에 독립.
  - OAuth 직접 구현 (PKCE 투명성 + goroutine 누수 방지).
  - Deferred tool loading으로 50+ 서버 시나리오 초기 지연 회피.
  - `coder/websocket` + stdlib SSE + `os/exec` stdio.
  - 이름 충돌: `mcp__{server}__{tool}` 접두사 엄격 적용.
  - Credential file mode `0600` 강제.
- **Go 버전**: 1.22+ (CORE-001 정합).
- **다음 단계 선행 요건**: TRANSPORT-001의 gRPC 인터셉터 패턴 확정 후 MCP의 내부 로깅/recovery 인터셉터 공유 설계. CONFIG-001의 feature gate로 `mcp.enabled`를 분리.
