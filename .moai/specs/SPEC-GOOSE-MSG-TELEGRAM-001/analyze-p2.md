# SPEC-GOOSE-MSG-TELEGRAM-001 P2 분석 노트

작성일: 2026-05-09
작성자: manager-tdd (P2 구현 전 아키텍처 결정)

---

## 1. audit.Writer 인터페이스 및 EventType 결정

### 확인된 인터페이스

`internal/audit/adapter_permission.go:21`에 `Writer` 인터페이스 확인:

```go
type Writer interface {
    Write(event AuditEvent) error
    Close() error
}
```

`MockWriter`도 동일 파일 `adapter_permission.go:123`에 존재 — 테스트에 바로 사용 가능.

`AuditEvent`는 `internal/audit/event.go:99`에 위치. Metadata 타입은 `map[string]string`.

### 기존 EventType 검토

기존 상수 목록:
- `fs.write`, `fs.read.denied`, `fs.blocked_always` — 파일시스템 이벤트
- `permission.grant`, `permission.revoke`, `permission.denied` — 권한 이벤트
- `sandbox.blocked_syscall`, `credential.accessed` — 보안 이벤트
- `task.plan_approved`, `task.plan_rejected` — 태스크 이벤트
- `tool.web.invoke`, `tool.web.sandbox_warning` — 웹 도구 이벤트
- `goosed.start`, `goosed.stop` — 데몬 이벤트

메시징 채널 인바운드/아웃바운드 이벤트에 맞는 기존 타입 없음.

### 결정: 신규 EventType 2개 추가

```go
EventTypeMessagingInbound  EventType = "messaging.inbound"
EventTypeMessagingOutbound EventType = "messaging.outbound"
```

**근거**: `tool.web.invoke` 패턴과 동일한 네이밍 컨벤션(`domain.direction`). 메시징 도메인은 파일/권한/샌드박스 이벤트와 의미적으로 명확히 구분되므로 기존 타입 재사용 부적절. `messaging` 네임스페이스는 향후 다른 채널(Slack, Discord 등)의 이벤트도 수용 가능.

Severity: `SeverityInfo` (정상 메시지 흐름). 차단/드롭 이벤트는 metadata로 표현.

---

## 2. AgentQuery 인터페이스 결정 (spec.md의 Query vs 실제 Chat)

### 실제 gRPC 메서드 확인

`internal/transport/grpc/gen/goosev1/agent_grpc.pb.go`:

```go
type AgentServiceClient interface {
    Chat(ctx context.Context, in *AgentChatRequest, opts ...grpc.CallOption) (*AgentChatResponse, error)
    ChatStream(...)
}
```

`AgentChatRequest` 필드:
- `Agent string` — GetAgent()로 접근 (필드명은 `Agent`, NOT `AgentName`)
- `Message string` — GetMessage()
- `InitialMessages []*AgentMessage`
- `SessionId string`

`AgentChatResponse` 필드:
- `Content string` — GetContent()
- `TokensIn int64`
- `TokensOut int64`

**spec.md/plan.md의 `Query` 메서드명은 실제 gRPC 계약과 다름** — 이 divergence를 analyze-p2.md에 기록하고 sync phase에서 반영.

### 결정: 옵션 B (narrow interface)

```go
type AgentQuery interface {
    Query(ctx context.Context, text string) (string, error)
}
```

어댑터 구현:

```go
type AgentQueryAdapter struct {
    client    goosev1.AgentServiceClient
    agentName string
}

func (a *AgentQueryAdapter) Query(ctx context.Context, text string) (string, error) {
    resp, err := a.client.Chat(ctx, &goosev1.AgentChatRequest{
        Agent:   a.agentName,
        Message: text,
    })
    if err != nil {
        return "", err
    }
    return resp.GetContent(), nil
}
```

**근거**: narrow interface가 모킹 친화적. BridgeQueryHandler가 grpc 패키지를 직접 의존하지 않아도 됨. 어댑터는 bootstrap 레이어에서만 구성.

---

## 3. 메모리 영속성 결정 (CRITICAL)

### MemoryProvider 인터페이스 검토

`internal/memory/provider.go`의 `MemoryProvider`는 LLM 도구 스타일 인터페이스 (Initialize, GetToolSchemas, HandleRecall 등). KV 스토어가 아님. spec.md/plan.md의 "MEMORY-001 bucket" 추상화는 실제 구현과 무관.

### 옵션 분석

**Option A** (BuiltinProvider files API 재사용): MemoryProvider 인터페이스 우회 필요, 결합도 높음, 테스트 격리 어려움.

**Option B** (독립적 sqlite DB at `~/.goose/messaging/telegram.db`): 블래스트 반경 최소, MemoryProvider 인터페이스 무영향, `t.TempDir()` 사용한 테스트 격리 완벽.

**Option C** (패키지 내부 KV 추상화): Option B와 유사하나 추상화 레이어 추가됨.

### 결정: Option B (독립적 sqlite DB)

- 경로: `~/.goose/messaging/telegram.db`
- 드라이버: `modernc.org/sqlite` — go.mod에 `v1.50.0`으로 이미 등록 확인됨
- 드라이버 임포트: `_ "modernc.org/sqlite"` — `internal/memory/builtin/builtin.go`에서 이미 블랭크 임포트됨. 새 패키지에서도 블랭크 임포트 필요.
- Store 인터페이스: `GetUserMapping`, `PutUserMapping`, `ListAllowed`, `Approve`, `Revoke`, `GetLastOffset`, `PutLastOffset` (7개 메서드 + Close)

**근거**: 메시징 패키지가 memory 패키지에 의존하지 않음. telegram.db는 messaging 도메인 소유의 격리된 데이터. 테스트에서 `t.TempDir()`으로 완전 격리 가능.

---

## 4. credproxy 연동 (MemoryKeyring → CredproxyKeyring)

### credproxy 인터페이스 확인

`internal/credproxy/proxy.go:59`의 `NewProxy(cfg ProxyConfig)`:

```go
type ProxyConfig struct {
    ListenAddr  string
    Store       *ProviderStore
    Keyring     keyring  // fileKeyring, not exported
    AuditWriter auditWriter
}
```

credproxy는 HTTP proxy 패턴 — 에이전트 프로세스의 HTTP 요청을 중간에서 가로채 자격 증명을 주입하는 구조. Telegram 봇 토큰을 단순 get/set 방식으로 조회하는 용도에 맞지 않음. `credproxy.Keyring`은 내부 unexported 인터페이스이며 telegram 패키지에서 직접 사용할 수 없음.

### 결정: MemoryKeyring 유지, @MX:TODO 명확화

credproxy는 개념적으로 다른 패턴 (LLM agent HTTP 요청 proxying). 봇 토큰 조회 목적에 적합한 공개 API 없음.

P1의 `@MX:TODO P2` 마커를 다음으로 업데이트:
```
// @MX:TODO P3 — evaluate OS keyring integration (e.g. zalando/go-keyring or
// charmbracelet/x/exp/once) for production bot token storage.
// credproxy (internal/credproxy) is an HTTP proxy pattern, not a KV keyring,
// and is therefore not a fit for direct telegram token retrieval.
```

---

## 5. 데몬 진입점 (bootstrap wiring)

### goosed 시작 시퀀스 확인

`cmd/goosed/main.go`의 `runWithContext`:
- 13-step wire-up 시퀀스
- 10.5~10.8: Slash command subsystem wiring
- 11: 헬스서버 기동
- 12: StateServing 전환
- 13: 시그널 대기

### AgentServiceClient 접근 가능성

현재 `cmd/goosed/wire.go`에 AgentServiceClient를 직접 구성하는 코드 없음. 데몬은 itself가 AgentService 서버이므로 자기 자신에게 gRPC 클라이언트를 만드는 것은 설계 불일치. 

**결정: 옵션 (a) — @MX:TODO P3으로 AgentQuery 연동 연기**

Bootstrap 레이어에서 `NoOpAgentQuery`를 임시 구현:

```go
type NoOpAgentQuery struct{}

func (n *NoOpAgentQuery) Query(ctx context.Context, text string) (string, error) {
    return "Telegram BRIDGE wiring deferred to P3. (goosed self-gRPC client not yet wired)", nil
}
```

P3에서 데몬 내부 AgentService 핸들러를 직접 호출하는 방식으로 구현. 현재 P2에서는 BridgeQueryHandler의 모든 흐름 (매핑, 감사, 게이트)이 작동하되, agent 응답만 고정 텍스트.

### 통합 포인트

`runWithContext` Step 12와 13 사이 (StateServing 전환 후):

```go
// Optional: Telegram messaging channel (SPEC-GOOSE-MSG-TELEGRAM-001)
go startTelegramChannel(rootCtx, logger)
```

`startTelegramChannel`은 방어적 구현 — 오류 발생 시 로그 경고 후 리턴, 데몬 실패 없음.

---

## 6. SPEC divergence 요약 (sync phase 참조용)

| spec.md/plan.md 내용 | 실제 구현 | 영향 |
|---|---|---|
| `Query(ctx, text)` 메서드 | gRPC는 `Chat()` — AgentQuery 어댑터로 래핑 | 인터페이스 이름 유지, 구현 어댑터 |
| MEMORY-001 bucket abstraction | MemoryProvider는 LLM 도구 인터페이스 — 독립 sqlite | sync 시 spec 업데이트 필요 |
| credproxy.NewProxy keyring wiring | credproxy는 HTTP proxy 패턴 — MemoryKeyring 유지 | @MX:TODO P3으로 이동 |
| BoltDB for mapping persistence | modernc.org/sqlite 사용 (이미 go.mod에 존재) | sync 시 spec 업데이트 |
| AgentName field in AgentChatRequest | 실제 필드명은 `Agent` (not `AgentName`) | 코드는 올바르게 `Agent:` 사용 |
| P2에서 완전한 Agent 연동 | NoOpAgentQuery — P3으로 연기 | sync 시 spec 업데이트 |
