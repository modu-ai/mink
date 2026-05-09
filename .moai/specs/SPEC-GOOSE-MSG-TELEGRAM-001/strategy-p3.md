---
id: SPEC-GOOSE-MSG-TELEGRAM-001
artifact: strategy-p3
version: "0.1.0"
created_at: 2026-05-09
---

# SPEC-GOOSE-MSG-TELEGRAM-001 — P3 Strategy (Outbound + Markdown V2 + File Attach + Callback + Agent Wiring + OS Keyring)

본 문서는 plan.md §2 Phase 3 진입 직전 **추가 구현 결정이 필요한 5개 영역**만 다룬다.
plan.md (Layered Architecture, Test 전략, Rollout) 와 analyze-p2.md (audit/store/credproxy 결정) 가 다룬 내용은 중복하지 않는다.

P3 Task 매핑: T1 markdown / T2 sender / T3 tool 등록 / T4 file attach / T5 callback / T6 agent 어댑터 / T7 integration / T8 OS keyring.

사용자 확정 사항:
- Agent 연결: handler 직접 호출 (in-process, gRPC stack 우회)
- OS Keyring: zalando/go-keyring 1차 후보 (charmbracelet/x/exp 부재 시 확정)
- SPEC sync: post-P3 sync phase 일괄 (plan/spec/progress 수정 금지)

---

## Section A — ChatHandler Interface Extraction & Wiring

### A.1 현재 상태 (실측)

`grep -rn "RegisterAgentServiceServer" internal/` 결과 **호출 0건**. 즉 `internal/transport/grpc/gen/goosev1/agent_grpc.pb.go:116` 의 등록 함수는 존재하지만, 데몬은 아직 AgentService 를 자기 자신의 gRPC 서버에 등록하지 않았다. `internal/transport/grpc/server.go:159-176` (`AgentService_ServiceDesc`) 는 사용되지 않는 구조체 상태.

`internal/transport/grpc/server.go:160` 에는 `DaemonService` 만 등록되어 있으며, `AgentService`/`ToolService`/`ConfigService` 는 모두 미등록. CLI (`internal/cli/transport/connect.go:144`) 가 호출하는 `c.agent.Chat(...)` 도 현재 데몬에 도달하면 `Unimplemented` 응답을 받게 되어 있다.

`internal/agent/runner.go` 에 `AgentRunner.RunTask(ctx, task)` 가 존재하지만 `query.QueryEngine` 을 매 호출마다 생성하는 outer orchestrator. 데몬 부팅 시 instantiation 0건.

→ 즉 P3 의 "handler 직접 호출" 은 두 작업을 동시에 수행한다:
1. AgentService.Chat 의 in-process 핸들러를 데몬 wire-up 시퀀스에 신규 등록
2. 같은 handler 인스턴스를 telegram 채널에 in-process 로 주입

본 결정은 BRIDGE-001 의 "AgentService/Query unary RPC" 의도 자체를 부정하지 않는다. CLI 클라이언트 경로 (Connect-gRPC, `c.agent.Chat`) 는 동일 handler 가 gRPC server.go 를 통해 외부에 노출되는 경로로 P3 와 함께 활성화하면 된다 (또는 P4 로 이월; 본 SPEC scope 에서는 in-process 만 충족).

### A.2 Narrow Interface 제안

`internal/messaging/telegram/bridge_handler.go:21-23` 의 `AgentQuery` 인터페이스는 그대로 유지한다.

```go
type AgentQuery interface {
    Query(ctx context.Context, text string) (string, error)
}
```

P3 의 추가 결정은 어댑터를 어디에 두느냐다. 두 옵션:

- 옵션 (a) — `goosev1.AgentServiceServer` 인터페이스를 직접 만족하는 핸들러를 별도 패키지에 두고, 그 인스턴스를 두 경로에 공유:
  - `internal/agent/chathandler/` 신규 패키지 (또는 `internal/agent/service.go`)
  - `Chat(ctx, *goosev1.AgentChatRequest) (*goosev1.AgentChatResponse, error)` 메서드를 가진 `Handler` 타입 정의
  - gRPC 서버 등록: `goosev1.RegisterAgentServiceServer(s.grpcSrv, handler)` (server.go newServerInternal 내부)
  - telegram 어댑터: `internal/messaging/telegram/agent_adapter.go` 가 `Handler` 를 받아 `AgentQuery` 로 narrow 변환
- 옵션 (b) — 데몬 내부에서 사용할 신규 도메인 인터페이스 (`AgentChatHandler`) 를 정의하고, 그 위에 (i) gRPC server adapter (ii) telegram adapter 둘 다 얹기:
  - `internal/agent/chat.go` — `type ChatService interface { Chat(ctx, ChatRequest) (ChatResponse, error) }` (proto 의존 없음)
  - gRPC server.go 에서 proto request → ChatRequest 변환 후 위임
  - telegram 어댑터에서 ChatRequest 생성 (proto 의존 없음)

**추천: 옵션 (b)**. 근거:
1. telegram 패키지가 `goosev1` 을 직접 import 하지 않게 된다 (domain isolation, proto schema 변화 보호).
2. 테스트 친화: `internal/agent/chat_mock.go` 를 두면 telegram + grpc server 양쪽 unit test 가 같은 mock 을 공유.
3. analyze-p2.md §6 SPEC divergence 표의 "Query→Chat" 항목을 sync phase 에서 정리할 때 인터페이스 계층이 더 명확하다.

리스크: (a) 보다 코드 양이 약 30~50줄 증가 (proto ↔ domain 매핑 어댑터). 그러나 telegram 패키지 import graph 정화 가치가 더 크다.

### A.3 의존성 (실측)

`AgentRunner.NewAgentRunner` 는 `EngineConfigFactory` 를 요구하며, 이는 `query.QueryEngineConfig` 를 생성한다. 데몬 부팅 시 LLM provider/tool registry/permission gate 가 모두 ready 상태여야 한다. 현재 `cmd/goosed/main.go runWithContext` Step 5~10 시점에서:

- Step 5.5 `providerRegistry := router.DefaultRegistry()` — provider ready
- Step 5~7 `wireRegistries` — toolsRegistry/skillRegistry ready
- Step 10.5~10.8 `wireSlashCommandSubsystem` — `LoopController`/`ContextAdapter`/`Dispatcher` ready

`ChatService` 인스턴스는 Step 10.8 직후에 구성 가능하다. `EngineConfigFactory` 가 의존하는 것:
- providerRegistry (LLM 호출 라우팅)
- toolsRegistry (tool execution)
- aliasMap (model alias resolution)
- ChatHistoryStore — TBD (현재 데몬에 정착된 store 없음 — P3-T6 에서 in-memory placeholder 사용)

### A.4 Bootstrap 시퀀스

`cmd/goosed/main.go runWithContext` 에 Step 10.9 / 11.5 를 신설:

```
Step 1~10.8  (기존 변경 없음)
Step 10.9    [NEW] chatService := agentchat.NewChatService(agentchat.Config{
                    ProviderRegistry: providerRegistry,
                    ToolsRegistry:    toolsRegistry,
                    AliasMap:         aliasMap,
                    Logger:           logger,
                  })
Step 10.95   [NEW] goosev1.RegisterAgentServiceServer(grpcServer, agentchatGrpc.NewServer(chatService))
                  — gRPC server.go 의 newServerInternal 내부 또는 main.go 에서 server 인스턴스
                    노출 후 main 측 등록 (현재 server.go 가 internal grpc.Server 를 expose 하지 않으므로
                    server.go 의 Config 에 RegisterAgentService bool 필드 추가하거나
                    NewServer(... services ...AgentServer) 시그니처 확장 필요)
Step 11      health server (기존)
Step 11.5    [NEW] if telegram configured:
                    deps := telegram.Deps{
                        ...,
                        Agent: telegram.NewAgentAdapter(chatService),
                    }
                    go telegram.Start(rootCtx, deps)  (rootCtx cancel 시 graceful)
Step 12~13   기존
```

**부팅 순서 보장**:
- chatService 가 Step 10.9 에서 ready 되어야 telegram poller 가 query 실행 가능
- telegram.Start (Step 11.5) 가 chatService 를 전제로만 호출됨 → ordering 자연 보장
- chatService 자체는 stateless wrapper (provider/tools/alias 만 보유) → Step 10.9 에 위치 적합

**graceful shutdown 통합**:
- `rt.Drain.RegisterDrainConsumer` 에 telegram poller drain 등록 (max 5초 timeout)
- chatService 는 stateless 이므로 별도 drain 불필요. 단, in-flight `Chat` 호출이 ctx cancel 로 전파되어야 한다 (현재 BridgeQueryHandler 가 30초 timeout 이미 보유)

### A.5 P3 신규/수정 파일 목록 (Section A 한정)

| 경로 | 마커 | 비고 |
|-----|------|-----|
| `internal/agent/chat.go` 또는 `internal/agent/chathandler/service.go` | [NEW] | ChatService 도메인 인터페이스 + 구현 |
| `internal/transport/grpc/agent_chat_server.go` | [NEW] | proto ↔ domain 어댑터 (AgentServiceServer 만족) |
| `internal/transport/grpc/server.go` | [MODIFY] | newServerInternal 시그니처에 chatService 주입, RegisterAgentServiceServer 호출 추가 |
| `internal/messaging/telegram/agent_adapter.go` | [NEW] | ChatService → AgentQuery narrow 변환 |
| `internal/messaging/telegram/bootstrap.go` | [MODIFY] | NoOpAgentQuery deprecate (legacy fallback 보존), Deps.Agent 가 nil 일 때 Start 가 명시적 에러 (또는 panic) |
| `cmd/goosed/main.go` | [MODIFY] | Step 10.9 chatService 생성 + Step 11.5 telegram.Start goroutine |

NoOpAgentQuery 는 P3 종료 시 **삭제하지 않고** export 만 유지 (test/dev 용). bootstrap.go:33 의 `@MX:TODO P3` 주석은 P3 sender T6 완료 시 제거.

---

## Section B — TOOLS-001 Registry 실측

### B.1 현재 구현 위치 (실측)

- `internal/tools/registry.go` — `Registry` 구조체, `Register(t Tool, src Source) error` API. RWMutex 보호, JSON Schema draft 2020-12 컴파일 (`jsonschema/v6`), `additionalProperties: false` strict 옵션.
- `internal/tools/tool.go` — `Tool` 인터페이스 (`Name()/Schema()/Scope()/Call(ctx, json.RawMessage) (ToolResult, error)`).
- `internal/tools/builtin/` — 6개 built-in tool. 각 tool 파일의 `init()` 에서 `tools.RegisterBuiltin(t)` 호출 → 전역 슬라이스 누적 → `WithBuiltins()` Option 으로 일괄 등록.
- `internal/tools/web/register.go` — 웹 tool 의 동일 패턴 (`globalWebTools` + `WithWeb()` Option).
- `internal/tools/executor.go` — `Run(ctx, ExecRequest)` 가 schema validation → preapproval → CanUseTool gate → tool.Call 실행.
- `internal/tools/permission/matcher.go` — `Preapproved` 패턴 매칭 (cwd-relative file glob).
- `internal/permissions/can_use_tool.go` — `CanUseTool.Check(ctx, ToolPermissionContext) Decision` (Allow/Deny/Ask 분기).
- spec.md "TOOLS-001" 명세는 본 구현과 **일치**한다 (네이밍만 다를 뿐 구조 동일).

### B.2 telegram_send_message 등록 패턴

웹 tool (`register.go:21-25`) 와 동일 패턴 채택 추천:

```go
// internal/messaging/telegram/tool_register.go
package telegram

import (
    "sync"
    "github.com/modu-ai/goose/internal/tools"
)

var (
    globalMessagingToolsMu sync.Mutex
    globalMessagingTools   []tools.Tool
)

func RegisterMessagingTool(t tools.Tool) { /* append under lock */ }

// WithMessaging returns a tools.Option that registers all messaging tools into the registry.
func WithMessaging() tools.Option { /* iterate + r.Register */ }
```

P3 신규 파일 `internal/messaging/telegram/tool.go` 에서:

```go
type telegramSendMessageTool struct { sender *Sender }
func (t *telegramSendMessageTool) Name() string { return "telegram_send_message" }
func (t *telegramSendMessageTool) Schema() json.RawMessage { return telegramSendMessageSchema }
func (t *telegramSendMessageTool) Scope() tools.Scope { return tools.ScopeShared }
func (t *telegramSendMessageTool) Call(ctx, input) (tools.ToolResult, error) { /* sender.Send */ }
```

**중요한 wiring 차이**: web tool 은 `init()` 에서 등록되지만, telegram tool 은 sender 인스턴스 (config + client + store + audit 의존) 가 필요하므로 **runtime 등록**이어야 한다. 옵션:

- 옵션 (i) — `WithMessaging(sender)` 가 sender 를 인자로 받아 wiring 시점에 tool 인스턴스 생성 후 등록 (init 불사용)
- 옵션 (ii) — `init()` 에서 placeholder tool 등록 후 sender 가 `Bind(s *Sender)` 로 후크인

**추천: 옵션 (i)**. 근거:
1. `init()` 사용 시 sender nil 상태에서 Call 호출되면 nil deref 위험.
2. wire-up 시퀀스가 main.go 에서 명시적 → 디버깅/추적 용이.
3. test 에서 sender mock 주입이 자연스러움 (init 우회 불필요).

`cmd/goosed/wire.go wireRegistries` 호출 직후 main.go 에 다음 추가:

```go
if telegramConfigured {
    if err := tools.WithMessaging(telegramSender)(toolsRegistry); err != nil {
        logger.Warn("telegram tool registration failed", zap.Error(err))
    }
}
```

`tools.Option` 은 함수형 패턴이므로 추가 호출이 가능하다 (registry.go:69 `Option func(*Registry)`).

### B.3 Permission Gate 진입점 (실측)

`internal/tools/executor.go:101-122` 의 Run() 흐름:
1. `e.matcher.Preapproved(toolName, input, permCfg)` — `internal/tools/permission/matcher.go` GlobMatcher
2. preapproved=false 면 `e.canUseTool.Check(ctx, ctx)` → Decision (Allow/Deny/Ask)
3. **Ask 는 현재 deny 와 동일 처리** (executor.go:117 `// Ask: HOOK-001/CLI-001이 처리. 지금은 deny와 동일하게 처리.`)

CLI-TUI-002 modal 은 아직 `CanUseTool` 의 Ask 결과를 처리하는 구현이 없는 상태. 즉 P3 시점에 `telegram_send_message` 가 `Decision.Ask` 를 반환하면 사용자 응답 없이 즉시 deny 된다.

**P3 호환 어댑터 전략**:
- `telegram_send_message` 를 default `Allow` 로 등록하되, sender.Send() 내부에서 **per-call allowed_users 검증**을 수행 (REQ-MTGM-N02 가 이미 강제 — analyze-p2.md §3 의 store.ListAllowed 통과 필요).
- TOOLS-001 modal (`Ask`) 가 P4 또는 후속 SPEC 에서 활성되면, 같은 tool 의 permission policy 를 `default_ask` 로 변경하기만 하면 된다 (`permission.Config` 의 yaml 변경, 코드 변경 불필요).
- 즉 **이중 게이트**: registry preapproval (코드 default) + sender allowed_users 검증 (도메인 보안). 후자는 항상 활성.

acceptance.md AC-MTGM-005 E2 ("Permission gate 에서 사용자 Deny → tool error `permission_denied`") 는 P3 에서 부분적으로만 만족된다. 정확히는:
- E2 가 활성 modal 가정 → P4 까지는 N/A 로 마킹 (sync phase 에서 acceptance.md 에 [DEFERRED P4 — modal 미구현] 주석 추가)
- 대신 P3 에서 `unauthorized_chat_id` (E1) 는 sender allowed_users 검증으로 완전히 GREEN

### B.4 telegram_send_message JSON Schema (Telegram Bot API 6.x 기준)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["chat_id", "text"],
  "properties": {
    "chat_id": {
      "oneOf": [
        { "type": "integer" },
        { "type": "string", "pattern": "^@?[A-Za-z0-9_]{5,32}$" }
      ]
    },
    "text": { "type": "string", "minLength": 1, "maxLength": 4096 },
    "parse_mode": { "type": "string", "enum": ["MarkdownV2", "HTML", "Plain"], "default": "MarkdownV2" },
    "reply_to_message_id": { "type": "integer", "minimum": 1 },
    "inline_keyboard": {
      "type": "array",
      "maxItems": 1,
      "items": {
        "type": "array",
        "maxItems": 8,
        "items": {
          "type": "object",
          "additionalProperties": false,
          "required": ["text", "callback_data"],
          "properties": {
            "text": { "type": "string", "minLength": 1, "maxLength": 64 },
            "callback_data": { "type": "string", "minLength": 1, "maxLength": 64 }
          }
        }
      }
    },
    "attachments": {
      "type": "array",
      "maxItems": 10,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["type"],
        "properties": {
          "type": { "type": "string", "enum": ["image", "document"] },
          "path": { "type": "string" },
          "url":  { "type": "string", "format": "uri" }
        },
        "oneOf": [
          { "required": ["path"] },
          { "required": ["url"] }
        ]
      }
    },
    "silent": { "type": "boolean", "default": false }
  }
}
```

확인 사항:
- `chat_id` integer 또는 channel username (`@channel` 형태). private chat 만 IN-scope 이지만 schema 자체는 둘 다 허용 후 sender 측에서 private 검증.
- `inline_keyboard` 1단 강제 (`maxItems: 1` for outer array — REQ-MTGM-E03 / OUT-9 정합).
- `callback_data` 64 bytes 제약 (Telegram Bot API §inline-keyboard).
- `attachments` `oneOf path/url` 강제 — 동시 지정 금지.
- registry config `StrictSchema: true` 활성 시 `additionalProperties: false` 통과 필수 (registry.go:380).

### B.5 spec.md 와의 적합성

spec.md §3.1 Area 3 의 schema 와 본 §B.4 의 차이:
- spec 본문: `inline_keyboard: [[{text, callback_data}]] (optional, 1단)` — 정확히 §B.4 와 일치 (`maxItems: 1` 으로 강제).
- spec 본문: `attachments: [{type: image|document, path | url}]` — `oneOf path/url` 보강.
- spec 본문 `silent: bool (default: false — disable_notification)` — Telegram API 의 `disable_notification` 으로 매핑됨, sender 에서 변환.

divergence 없음. P3 에서는 본 §B.4 schema 를 그대로 등록.

---

## Section C — File Attachment Lifecycle

### C.1 Telegram getFile API 응답 (Bot API 6.x)

`POST /bot<token>/getFile` body: `{"file_id": "<id>"}` → response:

```json
{
  "ok": true,
  "result": {
    "file_id": "AgACAgIAAxkBAAEC...",
    "file_unique_id": "AQADBgADBecJEHo",
    "file_size": 51234,
    "file_path": "photos/file_42.jpg"
  }
}
```

다운로드 URL: `https://api.telegram.org/file/bot<TOKEN>/<file_path>` (note: `/file/` prefix, **API endpoint URL 과 다름**).

`internal/messaging/telegram/getupdates.go:43` 의 `httpPostJSON` 헬퍼는 `https://api.telegram.org/bot<TOKEN>/<method>` 만 지원. P3 에서 download path 용 별도 헬퍼 (`httpGetFile(ctx, baseURL, token, filePath, dst io.Writer)`) 추가 필요.

`go-telegram/bot/models.File` (이미 indirect dep 으로 v1.20.0 보유) 가 응답 스트럭처를 제공한다. 단 `client.go:7-9` 가 그 모델을 이미 사용 중이므로 추가 import 비용 없음.

### C.2 다운로드 디렉토리 + 보안

경로: `~/.goose/messaging/telegram/inbox/<message_id>.<ext>` (spec.md §3.1 Area 4 명시).

보안 강제:
1. **확장자 화이트리스트** — Telegram message 는 `photo` (PhotoSize array, jpg 확정) / `document` (mime_type 필드 보유). `document` 의 `file_name` 에서 추출한 `ext` 가 화이트리스트 (`{".jpg", ".jpeg", ".png", ".gif", ".webp", ".pdf", ".txt", ".zip", ".docx", ".csv"}`) 외이면 다운로드 거부 + audit `attachment_skipped: ext_blocked` 기록.
2. **파일명 sanitization** — `file_name` 사용 금지, 항상 `<message_id>.<ext>` 강제. `..` / `/` / NUL 검증 불필요 (자체 생성).
3. **크기 제한** — `file_size > 50_000_000` 이면 다운로드 거부 (Telegram bot inbound 제약과 일치, AC-MTGM-007 E1).
4. **inbox 디렉토리 권한** — `os.MkdirAll(inboxDir, 0o700)` (config.go:104 setup 패턴 동일).

### C.3 Cleanup 메커니즘 비교

옵션 (a) — per-file `time.AfterFunc(30*time.Minute, deleteFn)`:
- 장점: 코드 간단, 파일 마다 정확한 30분
- 단점: 다운로드 N개 시 timer goroutine N개 (lightweight 이지만 metric noise), daemon 재시작 시 timer 잃음 → 재시작 시점 cleanup 누락

옵션 (b) — 중앙 janitor goroutine (1분 tick + mtime sweep):
- 장점: goroutine 1개, daemon 재시작 시 mtime 기반이므로 재기동 시 자동 회수, AC-MTGM-007 E3 ("daemon 재시작 후에도 30분 미만 파일 보존") 자연 만족
- 단점: cleanup 정밀도 1분 윈도우 (30분 ± 1분), 첫 sweep 까지 지연 가능

**추천: 옵션 (b)**. 근거:
1. AC-MTGM-007 E3 가 mtime-based sweep 을 명시적으로 요구 → 옵션 (a) 만으로는 미충족.
2. goroutine 단일 → drain consumer 등록 단순 (`rt.Drain.RegisterDrainConsumer`).
3. 30분 cleanup 의 본질은 "PII 노출 시간 제한" 이며 ±1분 정밀도 부족이 보안 위험을 만들지 않는다.
4. daemon 외부에서 사용자가 `~/.goose/messaging/telegram/inbox/` 직접 삭제해도 영향 없음 (idempotent).

구현 위치: `internal/messaging/telegram/inbox.go` [NEW] — `Janitor` 구조체 + `Run(ctx)` 메서드. bootstrap.Start 에서 별도 goroutine 으로 기동.

```go
type Janitor struct {
    inboxDir  string
    ttl       time.Duration  // default 30min, configurable for tests
    tickEvery time.Duration  // default 1min, 5sec in tests
    logger    *zap.Logger
}

func (j *Janitor) Run(ctx context.Context) error {
    ticker := time.NewTicker(j.tickEvery)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return ctx.Err()
        case <-ticker.C:  j.sweepOnce(time.Now())
        }
    }
}
```

### C.4 동시성 — 중복 다운로드

같은 `message_id` 가 다른 update 로 두 번 도착하는 케이스 (Telegram 측 duplicate 또는 offset 영속 직후 daemon kill):

- `os.OpenFile(path, O_CREATE|O_EXCL|O_WRONLY, 0o600)` 사용 → 기존 파일 존재 시 fail-fast
- fail-fast 시 idempotent skip: audit `attachment_idempotent_skip: true`, 기존 파일 그대로 사용 (mtime 갱신 없음)
- BridgeQuery 본문에는 기존 파일 경로 그대로 포함

이는 AC-MTGM-003 E2 ("offset 영속 직후 daemon kill -9 → 마지막 update 1개 중복 처리") 의 방어선과 일치.

### C.5 BridgeQuery 본문에 attachment 통합 방식

`bridge_handler.go:175` 의 현재 `agent.Query(ctx, text)` 시그니처는 단일 string. P3 에서 두 가지 옵션:

- 옵션 (i) — `AgentQuery` 인터페이스 확장: `Query(ctx, text, attachments []string) (string, error)`
- 옵션 (ii) — 본문에 prefix 합성: `text` 앞에 `[attachments: path1, path2]\n\n<original>` 합쳐서 단일 string 으로 전달

**추천: 옵션 (i)**. 근거:
1. agent (chathandler/service.go) 가 attachment path 를 별도 인자로 받으면 LLM provider 의 multimodal 입력 (예 : Anthropic vision) 으로 자연스럽게 매핑 가능 (P4+ 확장 여지).
2. prefix 합성은 LLM 이 attachment 를 일반 텍스트 마커로 오해할 위험.
3. analyze-p2.md §6 의 "AgentQuery 인터페이스 변경" 으로 SPEC sync 표에 1줄 추가하면 충분.

A.2 에서 정의한 ChatService 도메인 인터페이스의 `ChatRequest` 에 `[]Attachment` 필드 추가:

```go
type Attachment struct {
    Path     string  // local path within ~/.goose/messaging/telegram/inbox/
    MimeType string  // e.g. image/jpeg
    SizeBytes int64
}
```

P3 시점에는 ChatService 가 attachment 를 무시하고 text 만 LLM 에 전달 (placeholder). multimodal 활성은 별도 SPEC.

---

## Section D — Callback Query Branch

### D.1 Callback Query Payload (Telegram Bot API 6.x)

`getUpdates` 응답의 update 객체에 `callback_query` 필드 (현재 `internal/messaging/telegram/client.go:42-45` Update struct 에 미정의):

```json
{
  "update_id": 123,
  "callback_query": {
    "id": "1234567890",
    "from": { "id": 555, "username": "alice", "is_bot": false },
    "message": {
      "message_id": 42,
      "chat": { "id": 555, "type": "private" },
      "text": "이전 메시지 본문"
    },
    "chat_instance": "abc123",
    "data": "opt_a"
  }
}
```

### D.2 Update 모델 확장

`client.go:42-45` 의 `Update` 와 `convertUpdates` (client.go:164-184) 는 P3 에서 callback 추가 분기 필요:

```go
type Update struct {
    UpdateID      int
    Message       *InboundMessage   // 기존
    CallbackQuery *CallbackQuery    // [NEW]
}

type CallbackQuery struct {
    ID         string  // for answerCallbackQuery
    FromUserID int64
    ChatID     int64   // from CallbackQuery.Message.Chat.ID
    MessageID  int     // for context
    Data       string  // ≤ 64 bytes (validated by sender on outbound side)
    ReceivedAt time.Time
}
```

`convertUpdates` 내부 분기 추가: `u.CallbackQuery != nil` 일 때 위 매핑 수행. `go-telegram/bot/models` 의 `Update.CallbackQuery` 가 이미 정의되어 있다 (indirect dep v1.20.0).

### D.3 answerCallbackQuery 호출 의무 (REQ-MTGM-N04)

`POST /bot<token>/answerCallbackQuery` body: `{"callback_query_id": "<id>"}` (text/show_alert 옵션은 P3 에서 미사용).

타이밍: handler 진입 후 즉시 호출 (Telegram 60초 안에 호출 안 하면 사용자 폰에 "loading spinner" 가 남는다). bridge_handler.go 의 callback 분기는 다음 순서:
1. `client.AnswerCallbackQuery(ctx, callbackQuery.ID)` — fire-and-log (실패해도 흐름 진행)
2. audit RecordInbound (direction=in, content_hash=sha256(callback_data))
3. callback_data 변환 → ChatService.Chat 호출
4. 응답 sendMessage (또는 inline keyboard 가 attached 메시지를 editMessageText)

`Client` 인터페이스 (client.go:51-61) 에 메서드 추가:
```go
AnswerCallbackQuery(ctx context.Context, callbackQueryID string) error
```

### D.4 callback_data 변환

옵션 (a) — raw string passthrough:
- BridgeQuery 본문 = `callback_data` 그대로
- 단순. agent 가 의미를 해석할 책임

옵션 (b) — JSON 디코딩 + 구조화:
- callback_data 를 JSON ({"action": "opt_a", "context": "..."}) 으로 인코딩 후 디코딩
- 64 bytes 제약 안에서 JSON 은 너무 좁음 (key 중복 비용)

**추천: 옵션 (a) — raw string passthrough**. 근거:
1. callback_data 64 bytes 제약 → JSON 구조화 시 useful payload < 30 bytes.
2. agent 측 ChatHandler 가 LLM context 로 받아서 자연어 해석 가능 ("user clicked opt_a" 형태로 prefix 합성).
3. 실제로 outbound 측 agent 가 inline_keyboard 를 보냈을 때 callback_data 문자열을 직접 정한 인스턴스이므로 의미 해석 책임도 같은 agent 가 진다.

BridgeQuery 본문 합성 패턴:

```
[callback_query] data="opt_a" message_id=42
```

agent 가 이 prefix 를 인식하도록 ChatService 의 system prompt 에 명시 (또는 별도 ChatRequest field `Source: "callback_query"`).

### D.5 chat_id 추출

`callback_query.message.chat.id` 우선 (사용자가 button 을 누른 메시지의 chat).
fallback `callback_query.from.id` (private chat 의 경우 두 값 동일하지만, 그룹 chat — OUT-1 — 인 경우 message.chat.id 가 그룹 ID. 본 SPEC 은 private only 이므로 동일).

bridge_handler.go 의 callback 분기는 chat_id 매핑 조회 (store.GetUserMapping) 를 inbound message 분기와 동일하게 수행. blocked user (`mapping.Allowed=false`) 의 callback 도 silent drop (REQ-MTGM-N05 일관).

### D.6 Audit 표현

inbound message 와 동일 schema 재사용:
- `EventType: messaging.inbound`
- `direction: in`
- `chat_id`, `message_id` (callback_query.message.message_id 사용)
- `content_hash: sha256(callback_data)` — REQ-MTGM-N06 일관
- metadata: `{"source": "callback_query", "callback_id": "<id>"}` (raw callback_id 는 PII 아님 — Telegram session-scoped)

acceptance.md AC-MTGM-010 의 "audit log 에 callback_query entry 기록 (`callback_data: opt_a`)" 항목은 PII 보호 정책상 raw `callback_data` 를 평문으로 기록할 수 없다. P3 에서는 `content_hash` 만 기록하고, sync phase 에서 acceptance.md 본 항목을 수정 (divergence 표 추가 — Section F 후보).

### D.7 callback expired 처리 (REQ-MTGM-N04)

Telegram 측 60초 timeout 은 사용자 client 의 spinner 동작에 관한 것. 데몬 입장에서는:
- 정상 흐름: poller 가 update 즉시 수신 (long poll 30초) → handler 가 즉시 answerCallbackQuery 호출
- abnormal: `update_id` 는 즉시 받았지만 daemon backoff/restart 로 처리가 60초+ 지연 → answerCallbackQuery 가 Telegram 측 400 (`query is too old`) 반환

처리: `answerCallbackQuery` 응답 코드 검사 → 400 일 때 audit `callback_expired: true` 기록 + 이후 흐름은 계속 진행 (사용자에게 응답 메시지 도착, 단 spinner 는 timeout). REQ-MTGM-N04 가 "처리하지 않는다" 라고 명시했으나 실 운영 가치 측면에서 **응답은 진행**, **audit 만 expired 마킹** 으로 의미 보전. SPEC sync 시 본 절을 §4.4 에 명시 (Section F 후보).

---

## Section E — OS Keyring Library 선정 + Adapter

### E.1 후보 평가

| 기준 | zalando/go-keyring | 99designs/keyring | charmbracelet/x/exp/once |
|------|--------------------|-----|-----|
| 최신 release | v0.2.8 (2026-03-23) | v1.2.2 (2022-12-19) | N/A — keyring 추상화 미제공 |
| License | MIT | MIT | MIT |
| 별 수 | 1.2k | 651 | (parent repo) |
| macOS Keychain | OK | OK | — |
| Windows Cred Manager | OK | OK | — |
| Linux Secret Service | OK (dbus 필수) | OK | — |
| Linux 백엔드 다양성 | dbus 만 | dbus / pass / file (encrypted) / keyctl | — |
| Test mock | `MockInit()` (in-memory provider replace) | 없음 | — |
| 의존성 | godbus/dbus/v5 (Linux) | 더 무거움 (jose/golang-jwt 등) | — |
| API 단순도 | `Set/Get/Delete(service, user, password)` 3개 | 다중 backend factory + Item struct | — |

`charmbracelet/x/exp/once` 는 sync.Once 보강 라이브러리이며 keyring 추상화를 제공하지 않는다 — 후보에서 제외 (사용자 옵션 제시 단계의 confused 후보였던 것으로 보임).

### E.2 추천

**zalando/go-keyring v0.2.8 채택**. 근거:
1. 최근 유지보수 활성 (2026-03-23 release) — 99designs/keyring (2022-12) 대비 4년+ 격차.
2. 의존성 최소 (`godbus/dbus/v5` 만; 이미 indirect 가 아님 — 신규 추가) — 99designs 는 jose/jwt 등 대형 의존을 끌어옴.
3. `MockInit()` 가 테스트 친화 (in-memory provider 교체 1줄). 99designs 는 `FileBackend` 나 `MockBackend` 중 backend factory 를 선택해야 함 — 복잡.
4. P3 시점에서 매크로 backend 다양성 (pass, keyctl) 은 yagni — 사용자 OS 3개 (mac/win/linux) 만 커버하면 SPEC scope 충족.

리스크: Linux dbus 부재 환경 (server 환경, headless CI) 에서 `Set/Get` 이 에러. 대응은 §E.4 GitHub Actions 전략에서 다룸.

### E.3 Adapter Interface

`internal/messaging/telegram/keyring.go:15-22` 의 현재 `Keyring` 인터페이스:

```go
type Keyring interface {
    Store(service, key string, value []byte) error
    Retrieve(service, key string) ([]byte, error)
}
```

zalando/go-keyring API:
- `keyring.Set(service, user string, password string) error`
- `keyring.Get(service, user string) (string, error)`
- `keyring.Delete(service, user string) error`

매핑은 trivial (`value []byte` ↔ `password string`). 신규 파일 `internal/messaging/telegram/keyring_os.go`:

```go
//go:build !nokeyring

package telegram

import zk "github.com/zalando/go-keyring"

type OSKeyring struct{}

func NewOSKeyring() *OSKeyring { return &OSKeyring{} }

func (OSKeyring) Store(service, key string, value []byte) error {
    return zk.Set(service, key, string(value))
}
func (OSKeyring) Retrieve(service, key string) ([]byte, error) {
    s, err := zk.Get(service, key)
    if err != nil { return nil, err }
    return []byte(s), nil
}
```

`Delete` 는 Keyring 인터페이스에 추가하지 않음 (사용자 직접 삭제: `security delete-generic-password ...` 또는 `keyring revoke` 별도 CLI — P4 후보).

build tag `!nokeyring` 으로 격리하여 dbus 부재 환경에서 build 에서 제거 가능.

### E.4 go.mod 추가 + GitHub Actions 대응

`go get github.com/zalando/go-keyring@v0.2.8` 으로 require 추가. 새 indirect: `github.com/godbus/dbus/v5`.

GitHub Actions Linux runner (`ubuntu-latest`) 는 dbus + secret-service 가 default 설치되어 있지 않다. 옵션:

- 옵션 (a) — `go test -tags=nokeyring ./...` 로 OS keyring 코드 제외, MemoryKeyring 으로 테스트
- 옵션 (b) — Linux runner 에 `apt install gnome-keyring dbus-x11` 후 `dbus-launch gnome-keyring-daemon` 으로 keyring 활성

**추천: 옵션 (a)**. 근거:
1. CI 의 목적은 코드 변경의 회귀 검출이지 OS 통합 검증이 아니다. OS 통합은 manual smoke test (acceptance.md AC-MTGM-001 E4) 가 담당.
2. 옵션 (b) 는 runner 별 환경 설치 + flaky.
3. CI 의 "Go (build / vet / gofmt / test -race)" status check (CLAUDE.local.md §1.3) 가 build tag `nokeyring` 로 실행되도록 `.github/workflows/<go>.yml` 의 test step 에 `-tags=nokeyring` 추가하면 통과.

신규 파일 구조:
- `keyring.go` (기존) — `Keyring` interface + `MemoryKeyring` (build tag 없음, 모든 환경)
- `keyring_os.go` [NEW] — `OSKeyring` 구현 (`//go:build !nokeyring`)
- `keyring_nokeyring.go` [NEW] — `OSKeyring` placeholder (`//go:build nokeyring`) — 호출 시 `errors.New("keyring disabled at build time")` 반환

Production 빌드 (`go build`) 는 default tag set 으로 OS keyring 사용. CI 빌드는 `-tags=nokeyring` 으로 disable.

### E.5 분석-P2 §4 의 @MX:TODO P3 마커 해소

`internal/messaging/telegram/keyring.go:67-71`:

```go
// @MX:TODO P3 — evaluate OS keyring integration (e.g. zalando/go-keyring or
// charmbracelet/x/exp/once) for production bot token storage.
// credproxy (internal/credproxy) is an HTTP proxy pattern, not a KV keyring,
// and is therefore not a fit for direct telegram token retrieval.
// See analyze-p2.md §4 for the full rationale.
```

P3-T8 GREEN 시 본 주석 블록 삭제 + `@MX:NOTE` 로 대체:

```go
// @MX:NOTE: [AUTO] OS keyring backend wired via OSKeyring (keyring_os.go),
// fallback MemoryKeyring for tests and -tags=nokeyring builds. Selection rule:
// production main.go uses OSKeyring; tests inject MemoryKeyring via Deps.
```

`messaging_telegram.go:97`, `messaging_telegram.go:288` 의 두 위치에서 현재 `telegram.NewMemoryKeyring()` 가 default. P3-T8 에서:
- setup command default → `telegram.NewOSKeyring()`
- start command default → `telegram.NewOSKeyring()`
- test 들은 `kr` 명시 주입 (이미 dependency injection 되어 있음 — 변경 불필요)

CLI flag `--keyring memory` (또는 환경변수 `GOOSE_TELEGRAM_KEYRING=memory`) 로 사용자가 의도적으로 in-memory 선택 가능 — Linux headless 환경 회피용.

---

## Section F — Newly Discovered Divergence (sync phase 입력)

P3 구현 진입 직전 코드베이스 조사 결과 spec/plan/acceptance 와 어긋나는 항목 (analyze-p2.md §6 의 후속):

| spec/plan 내용 | 실제 / P3 결정 | 영향 |
|---|---|---|
| spec.md §3.1 Area 3 "TOOLS-001 의 permission gate (CLI-TUI-002 modal)" | CLI-TUI-002 modal 미구현, executor.go:117 가 Ask=Deny 처리. P3 는 sender 측 allowed_users 검증으로 우회 (이중 게이트) | acceptance.md AC-MTGM-005 E2 (`permission_denied` modal) 를 P4 로 [DEFERRED] 마킹 |
| acceptance.md AC-MTGM-010 "audit log 에 callback_query entry 기록 (`callback_data: opt_a`)" | REQ-MTGM-N06 PII 보호 정책상 callback_data 를 평문 기록 금지. content_hash 만 기록 | acceptance.md AC-MTGM-010 본문 수정: "audit entry content_hash + metadata.source=callback_query" |
| spec.md §4.4 REQ-MTGM-N04 "callback timeout 초과 callback 응답 처리하지 않는다" | answerCallbackQuery 만 fail (Telegram 400). 응답 메시지 (BridgeQuery 결과) 는 정상 진행 + audit `callback_expired: true` | spec.md REQ-MTGM-N04 본문에 "answerCallbackQuery 만 미수행, 응답 메시지는 진행" 명시 |
| spec.md §3.1 Area 3 schema "attachments: [{type, path \| url}]" | path/url oneOf 강제 (XOR) — schema 에 명시 | spec.md schema 표기에 oneOf 강제 추가 |
| spec.md §3.1 Area 1 "setup 미실행 시 ... messaging 모듈 비활성" | ChatService 가 데몬 부팅 Step 10.9 에 wired 되며 telegram 모듈만 Step 11.5 에서 conditional. ChatService 자체는 항상 활성 (CLI gRPC 노출과 공유) | spec.md §2.2 의 [MODIFY] 표에 `cmd/goosed/main.go` Step 10.9 (chatService) 추가 |
| BRIDGE-001 의 `AgentService/Query` (spec.md §6 의존성 표) | gRPC 메서드명은 `Chat` (analyze-p2.md §2 기록 됨), P3 에서 narrow domain interface `ChatService` 도입 | spec.md §6 의존성 표 항목명 `AgentService/Chat (in-process via ChatService)` 로 갱신 |

위 6개 divergence 는 **P3 종료 후 sync phase** 에서 manager-docs 가 spec.md / plan.md / acceptance.md 에 일괄 반영. P3 진행 중 본 파일들 수정 금지 (사용자 확정 §3 항목).

---

## Appendix — P3 우선순위 요약 (Time Estimation 금지, 의존 순서만)

T1 markdown.go (independent) → T2 sender.go (markdown 의존) → T3 tool.go (sender 의존, ChatService independent)
T8 OS keyring (independent, T4 와 병행 가능) → T6 agent 어댑터 (ChatService 신설 + bootstrap MODIFY)
T4 file attach (Update 모델 확장 + janitor) → T5 callback (Update 모델 추가 의존)
T7 integration test (T1~T6 모두 GREEN 후 마지막)

부팅 시퀀스 (cmd/goosed/main.go):
- Step 10.9: ChatService 신설 (T6 선결)
- Step 10.95: gRPC AgentService 등록 (T6 후속, optional — 본 SPEC scope 내 또는 P4)
- Step 11.5: telegram.Start goroutine (T1~T8 GREEN 후)
