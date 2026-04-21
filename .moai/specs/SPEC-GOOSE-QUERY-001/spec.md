---
id: SPEC-GOOSE-QUERY-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 대(L)
lifecycle: spec-anchored
---

# SPEC-GOOSE-QUERY-001 — QueryEngine 및 Query Loop (Async Streaming Agentic Core)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-core §1-6 + ROADMAP v2.0 Phase 0 기반) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 **agentic 코어 런타임**을 정의한다. Claude Code의 async generator 기반 query loop를 Go로 포팅하여, 하나의 대화 세션 = 하나의 `QueryEngine` 인스턴스라는 생명주기 계약 위에서 streaming 응답, tool 실행, permission gating, budget tracking, compaction trigger yield를 통합한다.

본 SPEC이 통과한 시점에서 `QueryEngine`은:

- `SubmitMessage(prompt)`를 호출하면 즉시 `<-chan SDKMessage`를 반환(스트리밍 보장, 버퍼링 금지)하고,
- 내부 `queryLoop`가 `normalize → API call → tool_use 파싱 → canUseTool 게이트 → runTools → 결과 스트리밍 → 지속 판정` 루프를 `turnCount <= maxTurns`, `taskBudget.remaining > 0`, `ctx.Err() == nil` 동안 회전하며,
- 재시도(max_output_tokens 최대 3회), compaction 경계(CompactBoundary yield), permission 거부(allow/deny/ask 3-분기), teammate 모드(tool visibility 제약)를 **continue site에서 state 재할당**으로 처리한다.

본 SPEC은 코드 본체(`internal/query/`, `internal/query/loop/`, `internal/message/`, `internal/permissions/`)의 **인터페이스 계약과 관찰 가능 행동**을 규정한다. Compaction 전략의 세부 알고리즘은 SPEC-GOOSE-CONTEXT-001에, Tool 레지스트리 자체 구현은 SPEC-GOOSE-TOOLS-001에 위임한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 후속 Phase 2 (SKILLS/MCP/SUBAGENT/HOOK) 와 Phase 3 (TOOLS/COMMAND/CLI) 모두 `QueryEngine`의 소비자다. 본 SPEC이 Plan·Run 완료되기 전에는 어떠한 "LLM을 호출해서 응답을 돌려주는" 기능도 착수할 수 없다.
- `.moai/project/research/claude-core.md` §1–6이 Claude Code TypeScript async generator 구조의 직접 포트 경로를 제시한다(§4 포팅 매핑 + §5 80% 로직 재사용). 본 SPEC은 그 포팅 계약을 Go 이디엄(goroutine + channel + `context.Context`)으로 확정한다.
- 로드맵 v2.0 §13 핵심 설계 원칙 1·2("One QueryEngine per conversation", "Streaming mandatory")의 구현 가능성을 입증한다.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code TypeScript** (`./claude-code-source-map/`): `QueryEngine.submitMessage`, `queryLoop`, `State`, `Terminal`, `ToolUseContext`. 언어 상이로 직접 포트 아님 — 상태 머신과 continue site 로직만 번역.
- **Hermes Agent Python** (`./hermes-agent-main/`): `cli.py`의 `model_tools.py` tool inventory 패턴. 본 SPEC은 tool registry의 consumer 측만 정의.
- **MoAI-ADK-Go**: 본 레포에 미러 없음. 패턴 참고 전무.

### 2.3 범위 경계

- **IN**: `QueryEngine` 구조체 + 라이프사이클, `SubmitMessage` streaming API, `queryLoop` 상태 머신, canUseTool 인터페이스, Terminal 분기, Abort(context cancellation) 전파, 재시도 카운터, Message/StreamEvent 타입, Middleware hook 진입점, Teammate/Coordinator 모드 tool visibility 스위치.
- **OUT**: Compaction 알고리즘 자체(autoCompact/reactiveCompact/snip 상세 → CONTEXT-001), Tool 레지스트리 구현(→ TOOLS-001), systemPrompt 로딩(→ CONTEXT-001), LLM 프로바이더 어댑터(→ ADAPTER-001), Credential Pool(→ CREDPOOL-001), Hook 이벤트 디스패치(→ HOOK-001), Slash command 파서(→ COMMAND-001), Subagent fork(→ SUBAGENT-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/query/` 패키지: `QueryEngineConfig`, `QueryEngine`, `SubmitMessage` 메서드.
2. `internal/query/loop/` 패키지: `queryLoop` 고루틴 + `State` 구조체 + `Terminal` 결과 + continue site 재할당 규칙.
3. `internal/message/` 패키지: `Message`, `SDKMessage`(discriminated union via `Type` field), `StreamEvent`, `ContentBlock` 타입, normalize 함수, `ToolUseSummaryMessage` 포맷터.
4. `internal/permissions/` 패키지: `CanUseTool` 인터페이스 + `PermissionBehavior` enum (`Allow|Deny|Ask`) + `ToolPermissionContext` 구조체.
5. `QueryParams` 입력 구조체(§6.2).
6. Middleware hook 진입점: `PostSamplingHooks`, `StopFailureHooks` slice — 실제 이벤트 디스패치는 HOOK-001이 등록.
7. Teammate 모드 식별 필드 + coordinator 모드 tool filter 스위치(실 구현은 SUBAGENT-001).
8. `context.Context` 기반 abort 전파(모든 lifetime-bound 리소스 cleanup).
9. `task_budget` 누적 추적(compaction 경계 무관).
10. `maxOutputTokensRecoveryCount` ≤ 3 재시도 로직.
11. Fallback model chain 호출 훅(실제 체인은 ROUTER-001).

### 3.2 OUT OF SCOPE (명시적 제외)

- **Compaction 알고리즘 본체**: 본 SPEC은 CONTEXT-001의 인터페이스를 호출(Interface `Compactor { ShouldCompact(State) bool; Compact(State) (State, CompactBoundary, error) }`)하고 compaction 완료 시 `CompactBoundary` 메시지를 스트림에 yield한다. 내부 알고리즘(protected window 계산, LLM summary 요약 등)은 CONTEXT-001.
- **Tool 실행 본체**: `runTools(ctx, req) ([]ToolResult, error)`는 `internal/tools.Executor` 인터페이스를 호출만 한다. 파일/쉘/MCP 실행은 TOOLS-001.
- **Slash command 파싱**: `processUserInput(prompt)`는 본 SPEC에서 noop (원문 그대로 반환). COMMAND-001이 확장.
- **systemPrompt/userContext 조립**: CONTEXT-001의 `GetSystemContext()`, `GetUserContext()` 결과를 `QueryParams`로 주입받기만 한다.
- **LLM HTTP 호출 실체**: `LLMCall(ctx, req) (<-chan Chunk, error)` 인터페이스만 정의. Anthropic/OpenAI/Ollama 구현은 ADAPTER-001.
- **Credential rotation / rate limit / prompt caching**: CREDPOOL-001, RATELIMIT-001, PROMPT-CACHE-001.
- **Sub-agent fork / teammate mailbox / plan-mode 승인 대기**: SUBAGENT-001. 본 SPEC은 Teammate 식별 필드(`TeammateIdentity` optional)만 `QueryEngineConfig`에 둔다.
- **Trajectory 수집 / 학습 파이프라인**: TRAJECTORY-001 이후.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-QUERY-001 [Ubiquitous]** — The `QueryEngine` **shall** maintain a one-to-one correspondence between its instance lifetime and a single conversation session (no multiplexing across conversations).

**REQ-QUERY-002 [Ubiquitous]** — The `QueryEngine.SubmitMessage` method **shall** return a `<-chan SDKMessage` (receive-only channel) without buffering model output chunks; each `StreamEvent` delta from the underlying LLM call **shall** be forwarded to the channel as soon as it is parsed.

**REQ-QUERY-003 [Ubiquitous]** — The `queryLoop` **shall** mutate its `State` only at explicitly documented continue sites (post-compaction, post-retry, post-tool-results) and **shall not** mutate `State` from within LLM streaming callbacks.

**REQ-QUERY-004 [Ubiquitous]** — The `QueryEngine` **shall** support multiple sequential `SubmitMessage` invocations within the same instance, where each invocation shares the accumulating `messages[]` array and `task_budget` from prior invocations.

### 4.2 Event-Driven (이벤트 기반)

**REQ-QUERY-005 [Event-Driven]** — **When** `SubmitMessage(prompt)` is invoked, the engine **shall** (a) append the user `Message` to `State.messages`, (b) yield a `user_ack` `SDKMessage`, (c) spawn the `queryLoop` goroutine, and (d) return the output channel within 10ms.

**REQ-QUERY-006 [Event-Driven]** — **When** the LLM response contains a `tool_use` content block, the `queryLoop` **shall** invoke `CanUseTool(ctx, toolName, input, ToolPermissionContext)` and dispatch based on the returned `PermissionBehavior`: `Allow` → execute via `tools.Executor`, `Deny` → synthesize a `ToolResult{is_error: true, content: "denied"}`, `Ask` → return a `permission_request` `SDKMessage` and suspend the loop until a resolution arrives on the engine's permission inbox.

**REQ-QUERY-007 [Event-Driven]** — **When** a tool returns a result whose serialized `content` exceeds `taskBudget.toolResultCap` bytes, the `queryLoop` **shall** apply tool-result budget replacement: substitute the content with a pointer summary `{tool_use_id, truncated: true, bytes_original, bytes_kept}` and log the replacement.

**REQ-QUERY-008 [Event-Driven]** — **When** the API returns a `max_output_tokens` terminal event, the `queryLoop` **shall** (a) increment `State.maxOutputTokensRecoveryCount`, (b) if the counter is ≤ 3, re-issue the API call with the same messages array and no modifications, (c) if > 3, transition to `Terminal{success: false, error: "max_output_tokens_exhausted"}`.

**REQ-QUERY-009 [Event-Driven]** — **When** the `Compactor.ShouldCompact(State)` interface returns `true` at the top of an iteration, the `queryLoop` **shall** (a) invoke `Compactor.Compact(State)`, (b) yield a `CompactBoundary` `SDKMessage` with the boundary metadata, and (c) continue the loop with the returned new `State`.

**REQ-QUERY-010 [Event-Driven]** — **When** `ctx.Done()` fires (caller aborts via `context.Context` cancellation), the `queryLoop` **shall** (a) stop consuming LLM chunks, (b) close the output channel, (c) release any pending tool permissions, and (d) return `Terminal{success: false, error: "aborted"}` within 500ms.

### 4.3 State-Driven (상태 기반)

**REQ-QUERY-011 [State-Driven]** — **While** `State.turnCount < maxTurns` and `State.taskBudget.remaining > 0` and `ctx.Err() == nil`, the `queryLoop` **shall** continue iteration; reaching `turnCount == maxTurns` **shall** transition to `Terminal{success: true, error: "max_turns"}` (success because the conversation ran to the bounded limit), and `taskBudget.remaining <= 0` **shall** transition to `Terminal{success: false, error: "budget_exceeded"}`.

**REQ-QUERY-012 [State-Driven]** — **While** `QueryEngineConfig.CoordinatorMode == true`, the `CanUseTool` gate **shall** filter out tools whose manifest declares `scope: "leader_only"`, exposing only tools tagged `scope: "worker_shareable"` to the underlying LLM call's tool list.

**REQ-QUERY-013 [State-Driven]** — **While** a `permission_request` is pending (REQ-QUERY-006 `Ask` branch), the `queryLoop` **shall** suspend iteration without consuming additional LLM tokens and **shall** resume only after a `PermissionDecision` is delivered via the engine's permission inbox channel.

### 4.4 Unwanted Behavior (방지)

**REQ-QUERY-014 [Unwanted]** — **If** the LLM streaming call panics or returns a non-retriable provider error (HTTP 4xx except 429), **then** the `queryLoop` **shall** (a) stop forwarding chunks, (b) invoke `StopFailureHooks` with the error, (c) yield a final `SDKMessage` of type `error` with the error details, and (d) return `Terminal{success: false, error: "<provider_error>"}`.

**REQ-QUERY-015 [Unwanted]** — The `QueryEngine` **shall not** write to `State.messages` from goroutines other than the `queryLoop` goroutine owning the current iteration; all state transitions **shall** occur in the single loop goroutine to eliminate lock-based synchronization.

**REQ-QUERY-016 [Unwanted]** — The `QueryEngine.SubmitMessage` **shall not** block the caller for longer than 10ms before returning the output channel, even if LLM connection setup is slow (connection dial happens inside the `queryLoop` goroutine).

**REQ-QUERY-017 [Unwanted]** — **If** a `tool_use` block references a tool name not present in `QueryEngineConfig.Tools`, **then** the loop **shall** synthesize a `ToolResult{is_error: true, content: "tool_not_found: <name>"}` and **shall not** terminate the conversation.

### 4.5 Optional (선택적)

**REQ-QUERY-018 [Optional]** — **Where** `QueryEngineConfig.PostSamplingHooks` is non-empty, each hook function **shall** receive the sampled `Message` after every LLM response and **may** mutate it before tool parsing (middleware chain, FIFO order).

**REQ-QUERY-019 [Optional]** — **Where** `QueryEngineConfig.FallbackModels` is non-empty and the primary model returns a 5xx or 429 exceeding the per-call retry budget, the engine **shall** transparently retry the same turn against the next model in the chain before surfacing the error.

**REQ-QUERY-020 [Optional]** — **Where** `QueryEngineConfig.TeammateIdentity != nil`, the engine **shall** inject `{agent_id, team_name}` into every outbound LLM system prompt as a structured header and into every Trajectory-bound `SDKMessage` as metadata.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `*_test.go`로 변환 가능한 수준.

**AC-QUERY-001 — 정상 1턴 (tool 없음) end-to-end**
- **Given** `QueryEngine` configured with a stub `LLMCall` that streams one assistant text message "ok" and then ends
- **When** 테스트가 `SubmitMessage("hi")`를 호출하고 반환 채널을 drain
- **Then** 채널 순서대로 `user_ack` → `stream_request_start` → 하나의 `StreamEvent{delta:"ok"}` → 하나의 `Message{role:"assistant"}` → `Terminal{success:true}` 메시지가 관찰되고, 채널이 close되며, `State.turnCount == 1`

**AC-QUERY-002 — 1턴 with 1 tool call**
- **Given** 스텁 LLM이 첫 응답에서 `tool_use{name:"echo", input:{"x":1}}` 블록을 포함하고, 두 번째 응답에서 `stop`을 반환. `CanUseTool`이 항상 `Allow`. `tools.Executor.Run("echo", input)`이 `{"x":1}`를 반환
- **When** `SubmitMessage("call echo")`를 호출하고 drain
- **Then** `tool_use` → `permission_check{allow}` → `tool_result{content:{"x":1}}` → 두 번째 `assistant Message` → `Terminal{success:true}` 순서로 관찰되고, `State.turnCount == 2` (tool round-trip = 1 iteration)

**AC-QUERY-003 — Tool permission deny 처리**
- **Given** 스텁 LLM이 `tool_use{name:"rm_rf"}`를 반환, `CanUseTool`이 `Deny{reason:"destructive"}`를 반환
- **When** `SubmitMessage`
- **Then** tool은 실제 실행되지 않고, `ToolResult{is_error:true, content:"denied: destructive"}`가 다음 API call의 messages에 포함되며, 대화는 계속 진행됨(terminal은 두 번째 assistant turn 후 success)

**AC-QUERY-004 — 2턴 연속 대화 (messages 배열 유지)**
- **Given** 동일 engine 인스턴스
- **When** `SubmitMessage("first")` drain 후 `SubmitMessage("second")`를 호출
- **Then** 두 번째 호출의 첫 API call payload의 `messages[]`에 1턴차 user+assistant 메시지가 모두 포함되고, `State.turnCount`는 누적된 값이며, `task_budget.remaining`은 누적 차감된 값

**AC-QUERY-005 — max_output_tokens 재시도 ≤ 3회**
- **Given** 스텁 LLM이 첫 3회 호출은 `max_output_tokens` 종료를 반환하고, 4회차에 정상 응답을 반환
- **When** `SubmitMessage`
- **Then** `State.maxOutputTokensRecoveryCount`는 3까지 증가 후 정상 응답 수신, `Terminal{success:true}`; 만약 스텁이 4회 모두 max_output_tokens을 반환한다면 `Terminal{success:false, error:"max_output_tokens_exhausted"}`

**AC-QUERY-006 — task_budget 소진 → budget_exceeded terminal**
- **Given** `QueryEngineConfig.TaskBudget = {total: 100}`, 스텁 LLM이 매 turn 60 units를 소비
- **When** `SubmitMessage` drain
- **Then** 2턴차 시작 시점에서 `remaining` 검사가 `-20`을 감지하고 `Terminal{success:false, error:"budget_exceeded"}`

**AC-QUERY-007 — max_turns 도달 → max_turns terminal**
- **Given** `MaxTurns=2`, 스텁 LLM이 항상 `tool_use`를 반환하여 무한 tool loop 형성
- **When** `SubmitMessage` drain
- **Then** 2턴 실행 후 `Terminal{success:true, error:"max_turns"}` (REQ-QUERY-011에 따라 success=true)

**AC-QUERY-008 — Abort via context cancellation**
- **Given** 스텁 LLM이 chunk 간 200ms 대기, `ctx`는 상위에서 `context.WithTimeout(500ms)`
- **When** `SubmitMessage` 호출 후 500ms 경과
- **Then** 채널이 close되고, 마지막 `SDKMessage`가 `Terminal{success:false, error:"aborted"}`이며, 처리 시간은 1초 이내 (REQ-QUERY-010의 500ms 마감)

**AC-QUERY-009 — Tool result budget replacement**
- **Given** 스텁 executor가 1MB JSON을 반환, `TaskBudget.ToolResultCap=4KB`
- **When** tool round-trip
- **Then** 다음 LLM payload의 messages에 포함된 tool_result의 `content`가 `{tool_use_id:..., truncated:true, bytes_original:1048576, bytes_kept:4096}` 형태로 치환되고, 원본 1MB는 전파되지 않음

**AC-QUERY-010 — Coordinator mode tool visibility 제한**
- **Given** `CoordinatorMode=true`, tool `A{scope:"leader_only"}`, tool `B{scope:"worker_shareable"}`
- **When** LLM call payload inspection
- **Then** payload의 `tools[]`에 `B`만 포함되고 `A`는 포함되지 않음. LLM이 잘못된 `tool_use{name:"A"}`를 반환할 경우 REQ-QUERY-017에 따라 `tool_not_found: A` 에러 결과로 처리

**AC-QUERY-011 — Compaction boundary yield**
- **Given** stub `Compactor.ShouldCompact`가 turn 3에서 true를 반환, `Compact`가 기존 messages 10개를 요약 1개로 치환
- **When** `SubmitMessage` drain
- **Then** turn 3 시작 전 `CompactBoundary{turn:3, messages_before:10, messages_after:1}` SDKMessage가 yield되고, 이후 iteration은 치환된 State로 진행됨. `task_budget.remaining`은 compaction 전/후 누적값이 보존됨

**AC-QUERY-012 — Fallback model chain**
- **Given** primary model이 HTTP 529(Overloaded) 반환, `FallbackModels=["model-B"]`
- **When** `SubmitMessage`
- **Then** 동일 turn에서 primary 재시도(예산 소진) → fallback `model-B`로 투명 재시도 → 성공 응답 수신. 외부로 드러나는 `Terminal`은 `success:true`이며, 로그에 fallback 사용이 기록됨

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/
├── query/
│   ├── engine.go                 # QueryEngine 구조체 + SubmitMessage
│   ├── engine_test.go            # Unit tests (20+ cases)
│   ├── config.go                 # QueryEngineConfig + QueryParams
│   └── loop/
│       ├── loop.go               # queryLoop goroutine + State machine
│       ├── continue_site.go      # continue site 분기 로직
│       ├── retry.go              # max_output_tokens 재시도
│       └── loop_test.go
├── message/
│   ├── message.go                # Message, ContentBlock, ToolResult
│   ├── sdk_message.go            # SDKMessage discriminated union
│   ├── stream_event.go           # StreamEvent
│   ├── normalize.go              # consecutive user merge, signature strip
│   └── summary.go                # ToolUseSummaryMessage 포맷
└── permissions/
    ├── can_use_tool.go           # CanUseTool interface + types
    ├── behavior.go               # PermissionBehavior enum
    └── context.go                # ToolPermissionContext
```

### 6.2 핵심 타입 (Go 시그니처 제안)

```go
// internal/query/config.go

// QueryEngineConfig는 세션 시작 시 1회 주입되는 불변 설정.
// 모든 필드는 생성자 이후 읽기 전용.
type QueryEngineConfig struct {
    Cwd               string
    Tools             tools.Registry              // TOOLS-001 제공
    Commands          []command.Definition        // COMMAND-001 제공
    MCPClients        []mcp.Connection            // MCP-001 제공
    Agents            []agent.Definition          // SUBAGENT-001 제공
    CanUseTool        permissions.CanUseTool      // REQ-QUERY-006
    MaxTurns          int
    MaxBudgetUsd      float64
    TaskBudget        *TaskBudget                 // optional; §6.3
    TeammateIdentity  *teammate.Identity          // SUBAGENT-001 optional
    CoordinatorMode   bool
    FallbackModels    []string
    PostSamplingHooks []MessageHook               // hook middleware
    StopFailureHooks  []FailureHook
    LLMCall           LLMCallFunc                 // ADAPTER-001 주입
    Compactor         context.Compactor           // CONTEXT-001 주입
    InitialMessages   []message.Message
    Logger            *zap.Logger                 // CORE-001 공유
}

// QueryEngine은 단일 대화에 1:1 대응.
type QueryEngine struct {
    cfg          QueryEngineConfig
    state        *loop.State
    permInbox    chan permissions.Decision
    mu           sync.Mutex                       // SubmitMessage 직렬화만
}

func New(cfg QueryEngineConfig) (*QueryEngine, error)

// SubmitMessage는 10ms 이내 반환(REQ-QUERY-016).
// 실제 LLM 호출은 spawn된 queryLoop goroutine이 담당.
func (e *QueryEngine) SubmitMessage(
    ctx context.Context,
    prompt string, // 또는 []ContentBlock
    opts ...SubmitOption,
) (<-chan message.SDKMessage, error)

// ResolvePermission은 REQ-QUERY-013의 pending permission을 해결.
// UI/CLI 레이어가 호출.
func (e *QueryEngine) ResolvePermission(
    toolUseID string,
    behavior permissions.Behavior,
) error


// internal/query/loop/state.go

// State는 queryLoop 내부에서만 읽고 쓴다 (REQ-QUERY-015).
// Continue site에서만 새 값 할당(REQ-QUERY-003).
type State struct {
    Messages                      []message.Message
    ToolUseContext                context.ToolUseContext
    AutoCompactTracking           *context.AutoCompactTrackingState
    MaxOutputTokensRecoveryCount  int
    TurnCount                     int
    TaskBudget                    TaskBudget
    Transition                    *Continue
}

// Continue는 continue site에서 next iteration이 수행할 분기를 표현.
type Continue struct {
    Reason    string // "after_compact" | "after_retry" | "after_tool_results"
    NextState *State
}

type TaskBudget struct {
    Total          int64   // 전체 예산 (tokens or cost units)
    Remaining      int64
    ToolResultCap  int64   // bytes per tool result
}

// Terminal은 queryLoop 종료 시 반환되는 최종 상태.
type Terminal struct {
    Success bool
    Error   string  // "max_turns" | "budget_exceeded" | "aborted" | "<provider_error>" | ...
    Text    string  // 마지막 assistant text (UI 편의)
}


// internal/message/sdk_message.go

type SDKMessageType string

const (
    SDKMsgUserAck            SDKMessageType = "user_ack"
    SDKMsgStreamRequestStart SDKMessageType = "stream_request_start"
    SDKMsgStreamEvent        SDKMessageType = "stream_event"
    SDKMsgMessage            SDKMessageType = "message"
    SDKMsgToolUseSummary     SDKMessageType = "tool_use_summary"
    SDKMsgPermissionRequest  SDKMessageType = "permission_request"
    SDKMsgCompactBoundary    SDKMessageType = "compact_boundary"
    SDKMsgError              SDKMessageType = "error"
    SDKMsgTerminal           SDKMessageType = "terminal"
)

// SDKMessage는 discriminated union (Go에서는 Type field + interface).
type SDKMessage struct {
    Type     SDKMessageType
    Payload  any          // 타입별 payload struct (type-switch로 분기)
}


// internal/permissions/can_use_tool.go

type Behavior int
const (
    Allow Behavior = iota
    Deny
    Ask
)

type Decision struct {
    ToolUseID string
    Behavior  Behavior
    Reason    string
}

// CanUseTool은 정책 게이트 인터페이스.
// SUBAGENT-001이 teammate별 정책을 구현.
type CanUseTool interface {
    Check(
        ctx context.Context,
        toolName string,
        input json.RawMessage,
        permCtx ToolPermissionContext,
    ) Decision
}
```

### 6.3 Continue Site 규약

`queryLoop`는 다음 세 지점에서만 `State`를 새로 할당한다 (REQ-QUERY-003):

| Continue Site | 트리거 | 새 State 구성 |
|---|---|---|
| `after_compact` | `Compactor.ShouldCompact == true` | messages 치환, `autoCompactTracking` 갱신, `taskBudget.Remaining` 누적 보존 |
| `after_retry` | `max_output_tokens` 수신, counter ≤ 3 | `maxOutputTokensRecoveryCount++`, messages 불변 |
| `after_tool_results` | tool 실행 완료 | messages에 `user` role tool_result block append, `turnCount++`, `taskBudget.Remaining -= tokens_used` |

나머지 상태 변경(예: `TurnCount` 증가)은 모두 위 3개 경로 내에서 발생한다.

### 6.4 Streaming 인터페이스 결정

본 SPEC은 **`<-chan SDKMessage`** 를 채택한다. 대안 비교:

| 대안 | 장점 | 단점 | 결정 |
|---|---|---|---|
| `<-chan SDKMessage` | Go 이디엄, select 통합, close로 EOF 표현 | 타입 안전성은 unit type switch에 의존 | **채택** |
| `io.ReadCloser` (JSON-L) | HTTP streaming과 자연스러운 매핑 | in-process 사용 시 serialize/deserialize 오버헤드 | 채택 안 함 |
| `Iterator[SDKMessage]` (Go 1.23 range-over-func) | 타입 안전 + 이디엄 | 1.22 호환성 손상. CORE-001이 1.22+ 명시 | 채택 안 함 |

transport 레이어(TRANSPORT-001)가 gRPC server streaming으로 직렬화할 때 채널을 drain하여 proto Message stream으로 변환한다.

### 6.5 AsyncGenerator → Go 매핑 전략

| TS 원문 | Go 매핑 |
|---|---|
| `async function* queryLoop()` | `func queryLoop(ctx, state, out chan<- SDKMessage) Terminal` goroutine |
| `yield message` | `select { case out <- message: ; case <-ctx.Done(): return }` |
| `yield* nestedGenerator()` | 동일 helper가 `out` 채널에 직접 send |
| `AsyncLocalStorage` | `context.Context` 값(`context.WithValue`) |
| `AbortController` | `context.WithCancel` 또는 상위 `WithTimeout` |
| `Promise<Response>` | 동기 호출 + goroutine 내 blocking |

### 6.6 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestQueryEngine_SubmitMessage_StreamsImmediately` — AC-QUERY-001 → 스텁 LLM 1턴 → 실패.
2. **RED #2**: `TestQueryLoop_ToolCallAllow_FullRoundtrip` — AC-QUERY-002 → 실패.
3. **RED #3**: `TestQueryLoop_PermissionDeny_SynthesizesErrorResult` — AC-QUERY-003 → 실패.
4. **RED #4**: `TestQueryEngine_TwoSubmitMessages_ShareMessages` — AC-QUERY-004 → 실패.
5. **RED #5**: `TestQueryLoop_MaxOutputTokens_Retries3Then Fails` — AC-QUERY-005.
6. **RED #6**: `TestQueryLoop_BudgetExhausted` — AC-QUERY-006.
7. **RED #7**: `TestQueryLoop_MaxTurnsReached` — AC-QUERY-007.
8. **RED #8**: `TestQueryLoop_AbortsOnContextCancel` — AC-QUERY-008.
9. **RED #9**: `TestQueryLoop_ToolResultBudgetReplacement` — AC-QUERY-009.
10. **RED #10**: `TestQueryLoop_CoordinatorModeToolFilter` — AC-QUERY-010.
11. **RED #11**: `TestQueryLoop_CompactBoundaryYieldedOnCompact` — AC-QUERY-011.
12. **RED #12**: `TestQueryLoop_FallbackModelChain` — AC-QUERY-012.
13. ... (REQ-QUERY-015 같은 방어 계약은 race detector + state mutation 경로 테스트로).
14. **GREEN**: 모든 RED 확인 후 `internal/query/*`, `internal/message/*`, `internal/permissions/*` 최소 구현.
15. **REFACTOR**: continue site를 별도 파일(`continue_site.go`)로 추출, LLMCall 어댑터 seam 정리, middleware chain 일반화.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, 모든 AC에 integration test, race detector 통과, stub LLM + stub Executor 기반 격리 테스트 |
| **R**eadable | 패키지별 단일 책임(engine vs loop vs message vs permissions), continue site 이름 명시 |
| **U**nified | `go fmt` + `golangci-lint` (errcheck, govet, staticcheck, ineffassign), 채널 close 책임 단일화(queryLoop만) |
| **S**ecured | CanUseTool 게이트가 모든 tool 실행의 단일 진입점, tool_not_found는 terminal 유발하지 않음(DoS 방지) |
| **T**rackable | 모든 SDKMessage에 `{turn, iteration, trace_id}` 메타 포함, zap 구조화 로그 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | `goosed` 데몬 + zap 로거 + context 루트 |
| 후속 SPEC | SPEC-GOOSE-CONTEXT-001 | `Compactor`, `ToolUseContext`, `AutoCompactTrackingState` 실 구현 |
| 후속 SPEC | SPEC-GOOSE-TOOLS-001 | `tools.Registry`, `tools.Executor` 실 구현 |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-001 | `LLMCall` 함수 타입의 실제 Anthropic/OpenAI 구현 |
| 후속 SPEC | SPEC-GOOSE-SUBAGENT-001 | `TeammateIdentity`, `CanUseTool` teammate 정책 |
| 후속 SPEC | SPEC-GOOSE-HOOK-001 | `PostSamplingHooks`, `StopFailureHooks` 이벤트 dispatcher |
| 후속 SPEC | SPEC-GOOSE-COMMAND-001 | `processUserInput`의 slash command 분기 |
| 후속 SPEC | SPEC-GOOSE-TRANSPORT-001 | `<-chan SDKMessage` → gRPC stream 변환 |
| 외부 | Go 1.22+ | generics, `signal.NotifyContext` 등 |
| 외부 | `go.uber.org/zap` v1.27+ | 구조화 로깅 (CORE-001 계승) |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | channel 기반 스트리밍이 backpressure를 소비자 쪽으로 전가, UI가 느리면 loop가 blocked | 중 | 중 | `out` 채널에 작은 buffer(예: 8) 또는 `select default drop` 옵션. 기본은 unbuffered(REQ-QUERY-002 준수), UI-heavy 소비자는 별도 drain goroutine |
| R2 | State mutation이 여러 곳에서 일어나면 race. 특히 Resume-from-permission 시 | 고 | 고 | REQ-QUERY-015 강제. `go test -race` 필수. Permission 해결은 inbox 채널을 통해 loop에 delivered, 외부 mutation 없음 |
| R3 | Compactor 인터페이스가 CONTEXT-001과 맞지 않아 재작업 | 중 | 중 | 본 SPEC에서 인터페이스를 확정(§6.2 `Compactor`), CONTEXT-001이 이를 구현. 두 SPEC 인터페이스 섹션을 교차 참조 |
| R4 | tool_use 병렬 실행 여부 미결정 (Claude Code는 순차) | 중 | 중 | Phase 0은 순차만. 병렬은 TOOLS-001에서 plan. 현재 계약은 "한 응답 내 여러 tool_use 블록은 순차 실행" |
| R5 | max_output_tokens 재시도 카운터가 compaction 경계를 넘어 누적되어야 하는지 불분명 | 낮 | 낮 | `after_compact` continue site에서 counter를 0으로 reset. 근거: compaction 후 context가 변했으므로 새 재시도 예산 부여. 테스트로 검증 |
| R6 | Fallback model chain이 CREDPOOL-001의 credential pool과 충돌 | 중 | 중 | 본 SPEC은 `LLMCall` 인터페이스 호출만. 풀 선택·순회는 ROUTER-001 내부. 본 SPEC의 `FallbackModels`는 provider-level override (모델 alias만) |
| R7 | 10ms SubmitMessage 반환 시간이 stub LLM 초기화와 충돌 | 낮 | 낮 | 초기화는 고루틴 내에서 수행. SubmitMessage는 채널 생성 + goroutine spawn만 (수 마이크로초 소요) |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/claude-core.md` §1 Core Loop, §3 TS 원문 인터페이스, §4 Go 매핑, §5 재사용 평가, §6 설계 원칙 10가지, §7 SPEC-GOOSE-QUERY-001 초안, §8 고리스크 영역
- `.moai/project/structure.md` §1 (패키지 레이아웃), §4 (모듈 책임 매트릭스), §7 (Go 인터페이스 원형)
- `.moai/project/tech.md` §3.1 (Go 런타임 스택)
- `.moai/specs/ROADMAP.md` §4 Phase 0, §13 핵심 설계 원칙 1·2
- `.moai/specs/SPEC-GOOSE-CORE-001/spec.md` — 선행 SPEC (데몬 생명주기)

### 9.2 외부 참조

- **Claude Code TypeScript source map**: `./claude-code-source-map/bootstrap/state.ts`, `entrypoints/init.ts` (패턴만)
- **Go context package**: https://pkg.go.dev/context — cancellation + values
- **Go 1.22 release notes**: https://go.dev/blog/go1.22 — for-range loop var semantics

### 9.3 부속 문서

- `./research.md` — claude-core.md §6 설계 원칙 10개를 본 SPEC 요구사항에 매핑한 상세 표, 테스트 전략, Go 라이브러리 결정
- `../ROADMAP.md` — 전체 Phase 계획
- `../SPEC-GOOSE-CONTEXT-001/spec.md` — 후속 Compactor 구현

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **compaction 알고리즘 본체를 구현하지 않는다**. `Compactor` 인터페이스 호출만. CONTEXT-001이 구현.
- 본 SPEC은 **실제 LLM HTTP 호출을 구현하지 않는다**. `LLMCall` 함수 타입만 정의. ADAPTER-001이 구현.
- 본 SPEC은 **Tool 실행 엔진을 구현하지 않는다**. `tools.Executor` 인터페이스 호출만. TOOLS-001이 구현.
- 본 SPEC은 **Sub-agent fork/worktree/mailbox를 구현하지 않는다**. SUBAGENT-001.
- 본 SPEC은 **Slash command 파서를 구현하지 않는다**. `processUserInput`은 passthrough. COMMAND-001.
- 본 SPEC은 **Hook 이벤트 dispatcher를 구현하지 않는다**. `PostSamplingHooks`/`StopFailureHooks` 엔트리포인트만 제공. HOOK-001.
- 본 SPEC은 **Credential rotation, rate limiting, prompt caching을 구현하지 않는다**. CREDPOOL/RATELIMIT/PROMPT-CACHE-001.
- 본 SPEC은 **Trajectory 수집/학습 파이프라인을 포함하지 않는다**. TRAJECTORY-001+.
- 본 SPEC은 **UI 렌더링(Ink/React)을 포함하지 않는다**. CLI-001.
- 본 SPEC은 **tool 병렬 실행을 허용하지 않는다**. Phase 0에서는 한 응답 내 tool_use 블록을 순차 실행만 (TOOLS-001이 향후 확장).

---

**End of SPEC-GOOSE-QUERY-001**
