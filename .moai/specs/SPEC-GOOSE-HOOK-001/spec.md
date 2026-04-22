---
id: SPEC-GOOSE-HOOK-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 2
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GOOSE-HOOK-001 — Lifecycle Hook System (24 Events + useCanUseTool 권한 플로우)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-primitives §5 + QUERY-001/SKILLS-001 합의 기반) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 **24개 lifecycle hook 이벤트 디스패처**와 **권한 결정 플로우(`useCanUseTool`)**을 정의한다. Claude Code의 hook 시스템을 Go로 포팅하여, `QueryEngine`(QUERY-001)이 tool 실행 전후·컨텍스트 변화·권한 요청·세션 종료 시점에서 사용자 정의 핸들러(plugin hook + inline shell 명령)를 동기/비동기적으로 실행하고, `PreToolUse` 블로킹(`continue=false`)으로 도구 호출을 차단하며, `useCanUseTool`의 3-분기(allow/deny/ask)를 결정한다.

본 SPEC이 통과한 시점에서 `internal/hook` 패키지는:

- 24개 `HookEvent` 상수를 정의하고 각 이벤트의 `HookInput`/`HookOutput` 페이로드 스키마를 타입화하며,
- `HookRegistry`가 plugin hook + inline YAML hook을 atomic하게 등록·교체(`clearThenRegister`)하고,
- `DispatchPreToolUse`가 `continue:false`를 받으면 tool 실행을 차단하고 `permissionDecision`을 upstream(QueryEngine)으로 전달하며,
- `useCanUseTool` 워크플로(classifier → interactive / coordinator / swarmWorker 핸들러 분기)를 구현하고,
- `SkillsRegistry.FileChangedConsumer`(SKILLS-001)를 `FileChanged` 이벤트 핸들러로 등록하여 조건부 Skill 활성화를 트리거한다.

QUERY-001의 continue site(§6.3)에서 본 SPEC의 `Dispatch*` 함수들이 명시적 계약에 따라 호출된다 — 본 SPEC은 **PreToolUse는 QUERY에서 호출**이라는 경계를 엄수한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Phase 2의 4 primitive 중 Hook은 **모든 primitive의 관찰자**. Skills(FileChanged → 조건부 활성화), MCP(PostToolUse → MCP 결과 후처리), Subagent(SubagentStart/SubagentStop) 모두 hook이 없으면 독립 동작 불가.
- `.moai/project/research/claude-primitives.md` §5가 Claude Code의 24 event + `useCanUseTool 40KB` 권한 플로우를 제시한다. 본 SPEC은 그 토폴로지를 Go로 확정.
- QUERY-001의 `PostSamplingHooks`, `StopFailureHooks` 훅 슬롯이 본 SPEC의 dispatcher에 연결되어야 RED→GREEN 가능.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code TypeScript**: `hooks/`, `useCanUseTool` React hook(40KB), `toolPermission/` 디렉토리. Go 포팅 시 React hook 개념은 제거되고, 단순 함수 + 상태 머신으로 번역.
- **MoAI-ADK `.claude/hooks/`**: 이미 `moai/` 하위에 Python 핸들러(stop-handler, session-start-handler 등) 존재. 본 SPEC은 subprocess 실행자로 이들을 호출만.

### 2.3 범위 경계

- **IN**: 24개 `HookEvent` 열거, 각 이벤트의 `HookInput`/`HookOutput` 타입, `HookRegistry` (atomic clearThenRegister), `Dispatch{Pre,Post}ToolUse`, `DispatchSessionStart`, `DispatchFileChanged` 등 주요 dispatcher, Shell command hook 실행자(`ExecCommandHook`), `useCanUseTool` permission flow(classifier/interactive/coordinator/swarmWorker), `PermissionQueueOps`(YOLO classifier approval, auto-mode denial 추적, decision logging), Plugin hook loader entry point.
- **OUT**: Skill system 자체(SKILLS-001), Subagent spawn(SUBAGENT-001), MCP tool 실행(MCP-001), gRPC dispatch(TRANSPORT-001), Plugin manifest 검증(PLUGIN-001), Telemetry aggregation(추후 SPEC), User-facing UI(CLI-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/hook/` 패키지 생성.
2. `HookEvent` enum — 24개 상수.
3. `HookInput`, `HookOutput`, `HookJSONOutput` 타입.
4. `HookHandler` 인터페이스 + 내장 구현:
   - `InlineCommandHandler`(shell command 실행),
   - `InlineFuncHandler`(Go 함수),
   - `PluginHandler`(plugin-loaded subprocess).
5. `HookRegistry`:
   - `Register(event HookEvent, matcher string, h HookHandler)`,
   - `ClearThenRegister(snapshot map[HookEvent][]HookBinding)` — atomic swap,
   - `Handlers(event HookEvent, ctx HookInput) []HookHandler`.
6. 주요 dispatcher 함수:
   - `DispatchPreToolUse(ctx, input) (PreToolUseResult, error)` — blocking-capable.
   - `DispatchPostToolUse(ctx, input) PostToolUseResult` — non-blocking aggregation.
   - `DispatchSessionStart(ctx, input) SessionStartResult` — returns `initialUserMessage`, `watchPaths`, `additionalContext`.
   - `DispatchFileChanged(ctx, changed []string)` — SKILLS-001 consumer 호출.
   - `DispatchUserPromptSubmit`, `DispatchStop`, `DispatchSubagentStop`, `DispatchPermissionRequest` 등.
7. `useCanUseTool` permission flow:
   - `PermissionResult` 타입 (`Allow|Deny|Ask` + `decisionReason`),
   - 핸들러 분기: classifier → interactive / coordinator / swarmWorker,
   - `PermissionQueueOps`: `SetYoloClassifierApproval`, `RecordAutoModeDenial`, `LogPermissionDecision`.
8. Shell command 실행자:
   - `exec.Command(cfg.Shell, "-c", command)`,
   - timeout(기본 30s, hook `async:true`는 별도),
   - stdin으로 `HookInput` JSON 전달, stdout에서 `HookJSONOutput` 파싱.
9. Matcher 시스템 — event별 matcher 문법(`PreToolUse:matcher`는 tool name, `FileChanged:matcher`는 glob pattern).
10. `PluginHookLoader.Load(manifest)` entry point — PLUGIN-001의 호출 대상; 본 SPEC은 인터페이스만 노출.

### 3.2 OUT OF SCOPE

- **Plugin manifest 파싱·검증**: PLUGIN-001. 본 SPEC은 `PluginHookLoader` 인터페이스 소비자.
- **Skill `FileChanged` consumer 본체**: SKILLS-001의 `FileChangedConsumer`. 본 SPEC은 등록·호출만.
- **Subagent 생명주기**: SUBAGENT-001. 본 SPEC은 `SubagentStart/Stop/TeammateIdle` 이벤트 dispatch 인프라만.
- **MCP 서버별 hook**: MCP-001. 본 SPEC은 `PostToolUse`에서 `updatedMCPToolOutput` 수신만 지원.
- **AgentTeams task hook(TaskCreated/TaskCompleted)**: 이벤트 상수 + input 타입만 정의. 실제 dispatch 논리는 SUBAGENT-001.
- **Permission UI (interactive terminal prompt)**: CLI-001. 본 SPEC은 `InteractiveHandler` 인터페이스로 위임.
- **Hook 분산 dispatch(cross-process)**: 단일 프로세스 내에서만.
- **Telemetry 수집기**: `LogPermissionDecision`은 zap 로그로만; telemetry export는 추후 SPEC.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-HK-001 [Ubiquitous]** — The `HookRegistry` **shall** expose exactly 24 named `HookEvent` constants; adding a new event requires a schema change in this SPEC and a semver-minor bump in `internal/hook`.

**REQ-HK-002 [Ubiquitous]** — All hook dispatch calls **shall** execute registered handlers in registration order (FIFO); the registry **shall not** reorder or deduplicate handlers.

**REQ-HK-003 [Ubiquitous]** — The `HookRegistry.ClearThenRegister(snapshot)` operation **shall** be atomic — observers calling `Handlers(event, ctx)` concurrently **shall** observe either the full prior snapshot or the full new snapshot, never a partial merge.

**REQ-HK-004 [Ubiquitous]** — Every hook dispatch **shall** emit a structured zap log entry with `{event, handler_count, outcome, duration_ms}` at INFO level on success and ERROR level on handler failure.

### 4.2 Event-Driven (이벤트 기반)

**REQ-HK-005 [Event-Driven]** — **When** `DispatchPreToolUse(ctx, input)` is invoked, the dispatcher **shall** (a) invoke all registered handlers matching `input.Tool.Name` in FIFO order, (b) stop invoking further handlers immediately upon the first handler returning `HookJSONOutput{Continue: false}`, (c) aggregate `permissionDecision` from that handler, and (d) return `PreToolUseResult{Blocked: true|false, PermissionDecision: ...}`.

**REQ-HK-006 [Event-Driven]** — **When** a shell command hook is executed, the dispatcher **shall** (a) spawn the shell subprocess, (b) pipe `HookInput` as JSON to stdin, (c) wait up to `cfg.Timeout` (default 30s), (d) parse stdout as `HookJSONOutput` JSON, (e) treat non-zero exit code OR malformed JSON OR timeout as `handler_error` — log and continue to the next handler (no block on error).

**REQ-HK-007 [Event-Driven]** — **When** `DispatchSessionStart(ctx, input)` is invoked, the dispatcher **shall** collect `initialUserMessage`, `watchPaths`, `additionalContext` from all handler outputs; multiple handlers' `watchPaths` **shall** be concatenated (dedup by absolute path), and `initialUserMessage` **shall** use the last non-empty value with a zap warn if multiple handlers set it.

**REQ-HK-008 [Event-Driven]** — **When** `DispatchFileChanged(ctx, changed []string)` is invoked, the dispatcher **shall** (a) invoke all handlers registered for `FileChanged` matching any path in `changed`, (b) after the internal handlers complete, call the externally-registered `SkillsFileChangedConsumer` (SKILLS-001's `FileChangedConsumer`) with the same `changed` slice, (c) return the union of activated skill IDs.

**REQ-HK-009 [Event-Driven]** — **When** `useCanUseTool(ctx, toolName, input, permCtx)` is called, the decision flow **shall** (a) run the YOLO classifier(auto-mode approval check), (b) if not auto-approved, dispatch `PermissionRequest` hooks, (c) if any hook returns `behavior: deny`, short-circuit with `PermissionResult{Behavior: Deny, DecisionReason: ...}`, (d) if no handler decides, route to `InteractiveHandler` (CLI-001) → `CoordinatorHandler` (SUBAGENT-001 coordinator mode) → `SwarmWorkerHandler` based on `permCtx.Role`.

**REQ-HK-010 [Event-Driven]** — **When** a hook returns `HookJSONOutput{Async: true, AsyncTimeout: N}`, the dispatcher **shall** spawn a goroutine to execute that handler with a deadline of `N` seconds (default 60s); the main dispatch **shall not** block on async handlers, and their output **shall** be logged but discarded unless the event is `PostToolUse` which appends `additionalContext`.

### 4.3 State-Driven (상태 기반)

**REQ-HK-011 [State-Driven]** — **While** a `PreToolUse` handler is blocking (has returned `Continue: false`), subsequent `PreToolUse` handlers for the same tool invocation **shall not** be invoked; the dispatcher **shall** short-circuit and return immediately with the blocking handler's `permissionDecision`.

**REQ-HK-012 [State-Driven]** — **While** YOLO classifier is set to `auto-approve` for a tool pattern (via `SetYoloClassifierApproval`), `useCanUseTool` calls matching that pattern **shall** return `PermissionResult{Behavior: Allow, DecisionReason: {Type: "yolo_auto"}}` without invoking interactive/coordinator handlers.

**REQ-HK-013 [State-Driven]** — **While** `PluginHookLoader.IsLoading == true`, the registry **shall** reject new `Register` calls with `ErrRegistryLocked`; this prevents partial plugin loads from being observable.

### 4.4 Unwanted Behavior (방지)

**REQ-HK-014 [Unwanted]** — The dispatcher **shall not** allow a hook handler to mutate the input object passed to downstream handlers; each handler **shall** receive a deep copy of `HookInput`.

**REQ-HK-015 [Unwanted]** — The dispatcher **shall not** execute handlers for events whose inputs fail schema validation; an invalid `HookInput` **shall** cause `ErrInvalidHookInput` to be returned immediately without handler invocation.

**REQ-HK-016 [Unwanted]** — Shell command hooks **shall not** be executed with elevated privileges; the dispatcher **shall not** call `sudo` or set capability bits; if a user-provided command requires privileges, the user is responsible for invoking via `sudo -n` inside the command string.

**REQ-HK-017 [Unwanted]** — The `useCanUseTool` flow **shall not** default to `Allow` when all handlers abstain; if no handler produces a `PermissionResult`, the default **shall** be `Ask` routed to `InteractiveHandler` — and in non-interactive environments (`stdout` not TTY), default **shall** be `Deny` with reason `"no_interactive_fallback"`.

**REQ-HK-018 [Unwanted]** — `ClearThenRegister` **shall not** fire any handlers during the swap; if new handlers rely on init-like side effects, they **shall** be idempotent — the registry provides no `OnLoad` hook.

### 4.5 Optional (선택적)

**REQ-HK-019 [Optional]** — **Where** `ENV GOOSE_HOOK_TRACE=1` is set, the dispatcher **shall** emit DEBUG-level trace logs including full `HookInput` + `HookOutput` JSON for every handler invocation.

**REQ-HK-020 [Optional]** — **Where** a handler declares `matcher: "regex:..."` the matcher **shall** be interpreted as a Go `regexp.Regexp`; bare strings **shall** be interpreted as glob patterns.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-HK-001 — 24개 HookEvent 상수 완전성**
- **Given** `internal/hook/types.go`
- **When** 테스트가 `HookEventNames()` 결과를 검사
- **Then** 정확히 24개 문자열이 반환되고, 각각 claude-primitives.md §5.1의 24개 이벤트명과 1:1 매칭 (Setup, SessionStart, SubagentStart, UserPromptSubmit, PreToolUse, PostToolUse, PostToolUseFailure, CwdChanged, FileChanged, WorktreeCreate, WorktreeRemove, PermissionRequest, PermissionDenied, Notification, Elicitation, ElicitationResult, PreCompact, PostCompact, Stop, StopFailure, SubagentStop, TeammateIdle, TaskCreated, TaskCompleted, SessionEnd, ConfigChange, InstructionsLoaded — 28개에서 본 SPEC은 24 핵심만)

**AC-HK-002 — PreToolUse blocking**
- **Given** 2개 핸들러 등록: H1은 `Continue: false + permissionDecision: {approve: false, reason: "unsafe"}`, H2는 호출되지 않아야 함
- **When** `DispatchPreToolUse(ctx, input{Tool: "rm_rf"})`
- **Then** `PreToolUseResult{Blocked: true, PermissionDecision: {Approve: false, Reason: "unsafe"}}`, H2는 호출 0회

**AC-HK-003 — Shell command hook roundtrip**
- **Given** 핸들러 등록: `InlineCommandHandler{Command: "cat | jq -c '.tool'"}` + timeout 5s
- **When** `DispatchPostToolUse(ctx, input{Tool: "test", Output: "..."})`
- **Then** subprocess가 stdin에서 JSON 수신, stdout으로 `"test"` 반환, dispatcher는 그 결과를 수집하여 `PostToolUseResult`에 포함

**AC-HK-004 — Shell command hook 타임아웃**
- **Given** 핸들러 등록: `InlineCommandHandler{Command: "sleep 60", Timeout: 2s}`
- **When** `DispatchPreToolUse`
- **Then** 2s 경과 후 subprocess 종료, handler_error 로그, 다음 핸들러 정상 호출

**AC-HK-005 — SessionStart 결과 집계**
- **Given** 2개 핸들러: H1 = `{watchPaths: ["src/"]}`, H2 = `{watchPaths: ["tests/"], initialUserMessage: "hi"}`
- **When** `DispatchSessionStart`
- **Then** 결과 `SessionStartResult{WatchPaths: ["src/", "tests/"], InitialUserMessage: "hi"}` (단일 메시지 set, 다중이면 warn)

**AC-HK-006 — FileChanged가 SKILLS-001 consumer를 트리거**
- **Given** stub `SkillsFileChangedConsumer`가 등록됨 (받은 paths를 기록), Hook 핸들러 0개
- **When** `DispatchFileChanged(ctx, ["src/foo.ts"])`
- **Then** stub consumer가 정확히 `["src/foo.ts"]`을 받음, dispatcher는 stub 반환값(활성화된 skill ID 목록)을 그대로 반환

**AC-HK-007 — useCanUseTool: YOLO 자동 승인**
- **Given** `SetYoloClassifierApproval("read_file")` 호출됨
- **When** `useCanUseTool(ctx, "read_file", {}, permCtx)`
- **Then** `PermissionResult{Behavior: Allow, DecisionReason: {Type: "yolo_auto"}}`, interactive handler 호출 0회

**AC-HK-008 — useCanUseTool: handler deny 단축회로**
- **Given** `PermissionRequest` 핸들러 1개 등록: 항상 `Deny{reason:"policy"}`
- **When** `useCanUseTool(ctx, "rm_rf", {}, permCtx)`
- **Then** `PermissionResult{Behavior: Deny, DecisionReason:{Reason: "policy"}}`, interactive/coordinator 호출 0회, `RecordAutoModeDenial` 호출됨

**AC-HK-009 — useCanUseTool: 비-TTY 환경 안전 기본값**
- **Given** 핸들러 0개, stdout이 파이프(not TTY), interactive fallback 불가
- **When** `useCanUseTool(ctx, "unknown_tool", {}, permCtx)`
- **Then** `PermissionResult{Behavior: Deny, DecisionReason:{Reason:"no_interactive_fallback"}}`

**AC-HK-010 — ClearThenRegister 원자성**
- **Given** 기존 핸들러 세트 A 등록됨, goroutine 2개가 `Handlers(event)`를 지속 호출 중
- **When** 다른 goroutine이 `ClearThenRegister(snapshotB)` 실행
- **Then** race detector 통과, 모든 observer는 A 또는 B만 관찰(혼재 없음), 최종 상태는 B

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/
└── hook/
    ├── types.go            # HookEvent (24개 상수), HookInput/Output 타입
    ├── registry.go         # HookRegistry (atomic clearThenRegister)
    ├── handlers.go         # HookHandler 인터페이스 + InlineCommand/InlineFunc/Plugin 구현
    ├── permission.go       # useCanUseTool flow + PermissionQueueOps
    ├── dispatchers.go      # Dispatch* 함수군
    ├── plugin_loader.go    # PluginHookLoader 인터페이스 (PLUGIN-001 consumer)
    └── *_test.go
```

### 6.2 핵심 Go 타입

```go
// 24개 이벤트. (claude-primitives §5.1 기반 추림)
type HookEvent string

const (
    EvSetup                HookEvent = "Setup"
    EvSessionStart         HookEvent = "SessionStart"
    EvSubagentStart        HookEvent = "SubagentStart"
    EvUserPromptSubmit     HookEvent = "UserPromptSubmit"
    EvPreToolUse           HookEvent = "PreToolUse"
    EvPostToolUse          HookEvent = "PostToolUse"
    EvPostToolUseFailure   HookEvent = "PostToolUseFailure"
    EvCwdChanged           HookEvent = "CwdChanged"
    EvFileChanged          HookEvent = "FileChanged"
    EvWorktreeCreate       HookEvent = "WorktreeCreate"
    EvWorktreeRemove       HookEvent = "WorktreeRemove"
    EvPermissionRequest    HookEvent = "PermissionRequest"
    EvPermissionDenied     HookEvent = "PermissionDenied"
    EvNotification         HookEvent = "Notification"
    EvPreCompact           HookEvent = "PreCompact"
    EvPostCompact          HookEvent = "PostCompact"
    EvStop                 HookEvent = "Stop"
    EvStopFailure          HookEvent = "StopFailure"
    EvSubagentStop         HookEvent = "SubagentStop"
    EvTeammateIdle         HookEvent = "TeammateIdle"
    EvTaskCreated          HookEvent = "TaskCreated"
    EvTaskCompleted        HookEvent = "TaskCompleted"
    EvSessionEnd           HookEvent = "SessionEnd"
    EvConfigChange         HookEvent = "ConfigChange"
)

// HookInput은 모든 이벤트가 공유하는 공통 구조.
type HookInput struct {
    HookEvent  HookEvent
    ToolUseID  string                 // tool 관련 이벤트에서만
    Tool       *ToolInfo              // PreToolUse/PostToolUse
    Input      map[string]any
    Output     any                    // PostToolUse만
    Error      *HookError             // PostToolUseFailure만
    ChangedPaths []string             // FileChanged만
    SessionID  string
    CustomData map[string]any
}

type HookJSONOutput struct {
    Continue           *bool              `json:"continue,omitempty"`
    SuppressOutput     bool               `json:"suppressOutput,omitempty"`
    Async              bool               `json:"async,omitempty"`
    AsyncTimeout       int                `json:"asyncTimeout,omitempty"`
    PermissionDecision *PermissionDecision `json:"permissionDecision,omitempty"`
    InitialUserMessage string             `json:"initialUserMessage,omitempty"`
    WatchPaths         []string           `json:"watchPaths,omitempty"`
    AdditionalContext  string             `json:"additionalContext,omitempty"`
    UpdatedMCPToolOutput any              `json:"updatedMCPToolOutput,omitempty"`
}

type HookHandler interface {
    Handle(ctx context.Context, input HookInput) (HookJSONOutput, error)
    Matches(input HookInput) bool   // matcher 적용
}

type HookBinding struct {
    Event   HookEvent
    Matcher string              // glob or regex: prefix
    Handler HookHandler
    Source  string              // "inline" | "plugin" | "builtin"
}

// Registry (atomic swap 기반).
type HookRegistry struct {
    current  atomic.Pointer[map[HookEvent][]HookBinding]
    pending  map[HookEvent][]HookBinding   // ClearThenRegister staging
    mu       sync.Mutex                    // staging만 보호
    logger   *zap.Logger
}

func (r *HookRegistry) Register(event HookEvent, matcher string, h HookHandler) error
func (r *HookRegistry) ClearThenRegister(snapshot map[HookEvent][]HookBinding) error
func (r *HookRegistry) Handlers(event HookEvent, input HookInput) []HookHandler

// Permission flow.
type PermissionBehavior int
const (
    PermAllow PermissionBehavior = iota
    PermDeny
    PermAsk
)

type PermissionResult struct {
    Behavior       PermissionBehavior
    DecisionReason *DecisionReason
}

type DecisionReason struct {
    Type   string  // "yolo_auto" | "handler" | "interactive" | "coordinator" | "swarm" | "no_interactive_fallback"
    Reason string
    Details map[string]any
}

type PermissionQueueOps interface {
    SetYoloClassifierApproval(toolPattern string)
    RecordAutoModeDenial(toolName string, reason string)
    LogPermissionDecision(result PermissionResult, toolName string)
}

// Interactive handler는 CLI-001이 구현.
type InteractiveHandler interface {
    PromptUser(ctx context.Context, toolName string, input map[string]any) (PermissionResult, error)
}
```

### 6.3 useCanUseTool 워크플로 (상태 머신)

```
┌─────────────────────────────────────────┐
│ useCanUseTool(toolName, input, permCtx) │
└────────────────┬────────────────────────┘
                 │
        ┌────────▼────────┐
        │ YOLO classifier │ ─── auto-approve? ──► Allow
        └────────┬────────┘
                 │ no
        ┌────────▼────────┐
        │ Dispatch        │
        │ PermissionRequest│ ─── deny? ──► Deny (+ RecordAutoModeDenial)
        └────────┬────────┘
                 │ no decision
        ┌────────▼────────┐
        │ Role-based route│
        │ - coordinator   │───► CoordinatorHandler (SUBAGENT-001)
        │ - swarmWorker   │───► SwarmWorkerHandler (permission bubble up)
        │ - interactive   │───► InteractiveHandler (CLI-001)
        │ - non-TTY       │───► Deny (no_interactive_fallback)
        └────────┬────────┘
                 │
        ┌────────▼────────┐
        │ LogPermissionDecision │
        └─────────────────┘
```

### 6.4 PreToolUse blocking 계약

QUERY-001의 loop에서:

```go
// QueryLoop 내부 (QUERY-001 책임)
res, _ := hook.DispatchPreToolUse(ctx, hookInput)
if res.Blocked {
    // tool 실행 생략, ToolResult를 permissionDecision 기반으로 합성
    toolResult = synthesizeDenial(res.PermissionDecision)
} else {
    // 정상 tool 실행
    toolResult = tools.Executor.Run(...)
}
```

본 SPEC의 `DispatchPreToolUse`는 **순수 함수**. QUERY-001이 그 결과를 해석하여 loop flow를 결정. 이것이 "PreToolUse는 QUERY에서 호출" 경계의 의미.

### 6.5 Shell Command Hook 실행자 (exec fork)

```go
func (h *InlineCommandHandler) Handle(ctx context.Context, input HookInput) (HookJSONOutput, error) {
    cctx, cancel := context.WithTimeout(ctx, h.Timeout)
    defer cancel()

    cmd := exec.CommandContext(cctx, h.Shell, "-c", h.Command)
    stdin, _ := cmd.StdinPipe()
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Start(); err != nil { return HookJSONOutput{}, err }

    inputJSON, _ := json.Marshal(input)
    stdin.Write(inputJSON)
    stdin.Close()

    if err := cmd.Wait(); err != nil {
        return HookJSONOutput{}, fmt.Errorf("hook exit %d: %s", cmd.ProcessState.ExitCode(), stderr.String())
    }

    var out HookJSONOutput
    if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
        return HookJSONOutput{}, fmt.Errorf("malformed hook output: %w", err)
    }
    return out, nil
}
```

### 6.6 Atomic ClearThenRegister

```go
func (r *HookRegistry) ClearThenRegister(snapshot map[HookEvent][]HookBinding) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    // deep copy snapshot
    copy := deepCopy(snapshot)
    r.current.Store(&copy)
    return nil
}
```

`atomic.Pointer[map[...]]` 기반 swap. 읽기는 lock-free.

### 6.7 Matcher 구문

| Prefix | 의미 | 예 |
|--------|-----|---|
| 없음 또는 `glob:` | glob 패턴 | `"src/**/*.ts"`, `"read_*"` |
| `regex:` | Go `regexp.Regexp` | `"regex:^rm_.*"` |
| `*` | wildcard (모든 입력) | `"*"` |

기본은 glob. `filepath.Match` 기반. Regex는 opt-in.

### 6.8 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| glob 매칭 | stdlib `path/filepath.Match` | 단순 충분 |
| regex | stdlib `regexp` | |
| JSON | stdlib `encoding/json` | |
| subprocess | stdlib `os/exec` + `context` | timeout 통합 |
| 로깅 | `go.uber.org/zap` | CORE-001 계승 |
| 파일 변경 감지 (watcher) | `github.com/fsnotify/fsnotify` | FileChanged 이벤트 발생 측(본 SPEC은 consumer만, watcher는 CORE-001 또는 CLI-001) |

### 6.9 TDD 진입 순서

1. **RED #1** — `TestHookEventNames_Exactly24` (AC-HK-001)
2. **RED #2** — `TestDispatchPreToolUse_HandlerBlocks` (AC-HK-002)
3. **RED #3** — `TestInlineCommandHandler_Roundtrip` (AC-HK-003)
4. **RED #4** — `TestInlineCommandHandler_Timeout` (AC-HK-004)
5. **RED #5** — `TestDispatchSessionStart_Aggregation` (AC-HK-005)
6. **RED #6** — `TestDispatchFileChanged_SkillsConsumer` (AC-HK-006)
7. **RED #7** — `TestUseCanUseTool_YOLO_AutoApprove` (AC-HK-007)
8. **RED #8** — `TestUseCanUseTool_HandlerDenyShortCircuit` (AC-HK-008)
9. **RED #9** — `TestUseCanUseTool_NonTTY_Deny` (AC-HK-009)
10. **RED #10** — `TestRegistry_ClearThenRegister_RaceFree` (AC-HK-010)
11. **GREEN** — 최소 구현.
12. **REFACTOR** — dispatcher 공통화(공유 goroutine 관리 코드 추출), permission flow 상태 머신 단순화.

### 6.10 TRUST 5 매핑

| 차원 | 본 SPEC 달성 방법 |
|-----|-----------------|
| **T**ested | 30+ unit test, 10 integration test (AC), race detector 필수 |
| **R**eadable | types / registry / handlers / permission / dispatchers 5파일 분리, 24 event 상수 주석 |
| **U**nified | `go fmt`, `golangci-lint`, 모든 Dispatch 함수 동일 시그니처 계약 |
| **S**ecured | Shell hook no-sudo, 비-TTY 환경 default deny, malformed JSON fail-safe, deep copy input |
| **T**rackable | 모든 dispatch에 `{event, handler_count, outcome, duration_ms}` 로그, `LogPermissionDecision` 기록 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `DispatchPreToolUse`/`DispatchPostSampling` consumer |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap 로거, context 루트, fsnotify watcher |
| 후속 SPEC | SPEC-GOOSE-SKILLS-001 | `FileChangedConsumer` 등록 |
| 후속 SPEC | SPEC-GOOSE-SUBAGENT-001 | `SubagentStart/Stop/TeammateIdle` dispatcher, CoordinatorHandler |
| 후속 SPEC | SPEC-GOOSE-MCP-001 | `PostToolUse.updatedMCPToolOutput` 수신 |
| 후속 SPEC | SPEC-GOOSE-PLUGIN-001 | `PluginHookLoader` 구현 |
| 후속 SPEC | SPEC-GOOSE-CLI-001 | `InteractiveHandler` 구현 |
| 외부 | Go 1.22+ | generics, context |
| 외부 | `go.uber.org/zap` v1.27+ | |
| 외부 | `github.com/stretchr/testify` v1.9+ | |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | PreToolUse 핸들러가 블로킹으로 turn latency 급증 | 중 | 중 | 기본 timeout 30s, 통계적으로 PreToolUse 핸들러는 1~2개 예상. 로그로 duration 관측. |
| R2 | Shell hook의 stdin/stdout deadlock (큰 payload) | 중 | 고 | payload cap 4MB. 초과 시 `ErrHookPayloadTooLarge` |
| R3 | `ClearThenRegister` 중 외부가 handler 참조 유지 → use-after-clear | 낮 | 중 | `Handlers()`가 slice를 **반환할 때 복사** (방어적 copy) |
| R4 | YOLO classifier 설정이 session 간 누수 | 중 | 중 | `PermissionQueueOps`의 state는 per-session, 세션 종료 시 `SessionEnd` hook에서 reset |
| R5 | 24개 이벤트 수가 claude-primitives §5.1의 28+와 불일치 | 중 | 낮 | 본 SPEC은 **핵심 24개만 지원**, Elicitation/ElicitationResult/InstructionsLoaded 등은 추후 SPEC 또는 본 SPEC v0.2에서 확장 |
| R6 | Plugin hook이 악의적 코드 실행 (plugin source 신뢰) | 고 | 고 | `ExecCommandHook`은 shell escape를 피하기 위해 `exec.Command(shell, "-c", cmd)` 고정. privilege escalation 차단 (REQ-HK-016). Plugin 소스 감사는 PLUGIN-001의 책임 |
| R7 | 비-TTY 환경 탐지가 CI 환경에서 false positive | 중 | 낮 | `GOOSE_HOOK_NON_INTERACTIVE=1` env override 제공. CI 설정 권장 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/claude-primitives.md` §5 Hook System, §5.1 24 runtime events, §5.2 Permission Hook flow, §5.3 Hook Callback payload
- `.moai/specs/ROADMAP.md` §4 Phase 2 row 14 (HOOK-001)
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` — `PostSamplingHooks`, `StopFailureHooks` entrypoint
- `.moai/specs/SPEC-GOOSE-SKILLS-001/spec.md` — `FileChangedConsumer` 계약

### 9.2 외부 참조

- Claude Code source map: `./claude-code-source-map/hooks/`, `useCanUseTool`, `toolPermission/` (패턴만)
- `fsnotify/fsnotify`: 파일 변경 감지 (FileChanged 이벤트 소스는 외부)
- Go `os/exec`: https://pkg.go.dev/os/exec
- Go `atomic.Pointer`: https://pkg.go.dev/sync/atomic#Pointer

### 9.3 부속 문서

- `./research.md` — claude-primitives.md §5 원문 + dispatcher 세부 설계
- `../SPEC-GOOSE-QUERY-001/spec.md`
- `../SPEC-GOOSE-SKILLS-001/spec.md`
- `../SPEC-GOOSE-SUBAGENT-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **Plugin manifest의 hook 섹션을 파싱하지 않는다**. `PluginHookLoader` 인터페이스만 노출. 파싱은 PLUGIN-001.
- 본 SPEC은 **fsnotify watcher를 구동하지 않는다**. FileChanged 이벤트 발생원은 CORE-001 또는 CLI-001의 책임; 본 SPEC은 그 이벤트의 dispatcher만 제공.
- 본 SPEC은 **Interactive terminal UI를 구현하지 않는다**. `InteractiveHandler` 인터페이스 위임만. CLI-001.
- 본 SPEC은 **Coordinator/SwarmWorker handler 내부 로직을 구현하지 않는다**. SUBAGENT-001.
- 본 SPEC은 **Skill conditional 활성화 알고리즘을 구현하지 않는다**. `SkillsFileChangedConsumer` 외부 함수 호출만.
- 본 SPEC은 **Elicitation/ElicitationResult/InstructionsLoaded/PostCompact dispatch 세부를 완전 구현하지 않는다**. 이벤트 상수 + 기본 dispatcher만; 세부는 후속 버전.
- 본 SPEC은 **MCP 서버별 hook 라우팅을 구현하지 않는다**. `PostToolUse.updatedMCPToolOutput` payload 전달만.
- 본 SPEC은 **Permission flow의 UI(터미널 prompt)를 구현하지 않는다**. CLI-001.
- 본 SPEC은 **Telemetry 수집기를 포함하지 않는다**. `LogPermissionDecision`은 zap 로그만.
- 본 SPEC은 **Hook handler cross-process dispatch를 지원하지 않는다**. 단일 goosed 프로세스 내.

---

**End of SPEC-GOOSE-HOOK-001**
