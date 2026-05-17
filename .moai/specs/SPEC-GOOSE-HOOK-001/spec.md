---
id: SPEC-GOOSE-HOOK-001
version: 0.3.1
status: completed
completed: 2026-04-27
created_at: 2026-04-21
updated_at: 2026-04-27
author: manager-spec
priority: P0
issue_number: null
phase: 2
size: 중(M)
lifecycle: spec-anchored
labels: [hook, dispatcher, permission, phase-2, goose-agent]
---

# SPEC-GOOSE-HOOK-001 — Lifecycle Hook System (24 Events + useCanUseTool 권한 플로우)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-primitives §5 + QUERY-001/SKILLS-001 합의 기반) | manager-spec |
| 0.2.0 | 2026-04-25 | plan-auditor 2차 감사 반영 — D1 (iter-1, critical) AC-HK-001 24개 일관화 (§5), D2 (iter-1, critical) frontmatter `created_at`/`labels` 채움 (MP-3), D6 (iter-1, major) REQ-HK-006에 exit code 2 compatibility 조항 추가 (§4.2), D10 (iter-1, minor → upgraded) `DispatchPermissionDenied` 공식화 및 REQ-HK-009 확장 (§3.1 / §4.2 / §6.2), D8 (iter-1, major) REQ-HK-021 shell hook subprocess isolation 신설 (§4.4), D9 (iter-1, minor) REQ-HK-022 4 MiB payload cap 신설 (§4.4) + §6.5 stdin goroutine write 정합. 미해결: D3 (iter-1, critical, MP-2 EARS), D4 (iter-1, major, traceability), D5 (iter-1, major, implementation leaks), D7 (iter-1, major, Unwanted EARS labeling), D11/D12/D13 (iter-1, minor). | manager-spec |
| 0.3.0 | 2026-04-25 | plan-auditor 3차 감사 반영 (RITUAL iter3 패턴) — **D3 (iter-2, critical, MP-2)** Format Declaration 제거 + 모든 AC를 EARS 패턴으로 직접 변환 (§5), **D4 (iter-2, major)** uncovered 12 REQ에 AC-HK-011~024 신설 (§5), **D5 (iter-2, major)** REQ 본문에서 구현 식별자(zap, ErrRegistryLocked, ErrInvalidHookInput, MINK_HOOK_TRACE, 정규식, env-var 리스트, rlimit 상수, ErrHookPayloadTooLarge) 제거하여 §6/§6.11로 이동, **D7 (iter-2, major)** REQ-HK-014/015/016/017/018/022 Unwanted EARS 패턴(`If ... then ... shall`) 적용, **D11 (iter-1, minor)** §6.2 Registry에 `SetSkillsFileChangedConsumer` API 정의, **D12 (iter-1, minor)** §6.2에 `Role` enum 정의, **D13 (iter-1, minor)** AC-HK-014로 PluginLoader 상태 검증 (DispatchPermissionRequest 분기는 AC-HK-008과 함께 묶음), **D14 (iter-2, minor)** §4.4와 §4.5 경계에 REQ 번호 비단조 배치 사유 명시 (재배치 금지 제약), **D15 (iter-2, minor)** REQ-HK-021(b) `WorkspaceRoot` resolver 책임을 CORE-001로 명시 + fallback 동작 정의, **D16 (iter-2, minor)** REQ-HK-022 "exceeds 4 MiB"를 "JSON byte length is strictly greater than 4 MiB" 명시, **D17 (iter-2, minor)** REQ-HK-021(c) `cfg.Timeout ≤ 0` 엣지케이스 정의 (default 30s), **D18 (iter-2, minor)** v0.2.0 HISTORY 결함번호 매핑 정정. | manager-spec |
| 0.3.1 | 2026-04-25 | Status sync — `internal/hook/` 패키지 구현 완료를 frontmatter에 반영. PR #11 (commit 6ac25c8): `dispatchers.go` (Dispatcher + 24 hook events), `registry.go` (HookRegistry + SetSkillsFileChangedConsumer), `handlers.go` (InlineCommandHandler with WorkspaceRootResolver field), `permission.go` (DefaultPermissionQueue), `isolation_*.go` (REQ-HK-021 4 isolation guarantees: env scrub / cwd pin / rlimit / FD hygiene), `types.go` (24 events + Role enum + ErrHookSessionUnresolved/ErrHookPayloadTooLarge), `plugin_loader.go` (PluginLoader stub). status: planned → implemented. SPEC 본문 변경 없음. | manager-spec |

---

## 1. 개요 (Overview)

AI.MINK의 **24개 lifecycle hook 이벤트 디스패처**와 **권한 결정 플로우(`useCanUseTool`)**을 정의한다. Claude Code의 hook 시스템을 Go로 포팅하여, `QueryEngine`(QUERY-001)이 tool 실행 전후·컨텍스트 변화·권한 요청·세션 종료 시점에서 사용자 정의 핸들러(plugin hook + inline shell 명령)를 동기/비동기적으로 실행하고, `PreToolUse` 블로킹(`continue=false`)으로 도구 호출을 차단하며, `useCanUseTool`의 3-분기(allow/deny/ask)를 결정한다.

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
   - `DispatchUserPromptSubmit`, `DispatchStop`, `DispatchSubagentStop`, `DispatchPermissionRequest` 등.  `DispatchPermissionRequest`는 §6.3 의 `useCanUseTool` 워크플로 내부에서만 호출되며, 검증은 AC-HK-008 (deny short-circuit 분기에서 호출) + AC-HK-009 (abstain → non-TTY default deny 분기에서 호출 0회) 의 조합으로 다룬다.
   - `DispatchPermissionDenied(ctx, result PermissionResult)` — **EvPermissionDenied 이벤트의 공식 dispatcher**. `useCanUseTool` 플로우가 `Deny`로 귀결될 때 내부에서 자동 호출되며, 외부 관찰자(감사 로그 소비자, 텔레메트리 consumer, 후속 SUBAGENT-001 coordinator)가 이 이벤트에 핸들러를 등록할 수 있다. enum 상수 `EvPermissionDenied`(§6.2)와 dispatcher 간 1:1 대응을 보장한다.
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

**REQ-HK-004 [Ubiquitous]** — Every hook dispatch **shall** emit a structured log entry containing the fields `event`, `handler_count`, `outcome`, and `duration_ms` at INFO level on success and at ERROR level on handler failure. The concrete logger (zap) and exact field encoding are defined in §6.10.

### 4.2 Event-Driven (이벤트 기반)

**REQ-HK-005 [Event-Driven]** — **When** `DispatchPreToolUse(ctx, input)` is invoked, the dispatcher **shall** (a) invoke all registered handlers matching `input.Tool.Name` in FIFO order, (b) stop invoking further handlers immediately upon the first handler returning `HookJSONOutput{Continue: false}`, (c) aggregate `permissionDecision` from that handler, and (d) return `PreToolUseResult{Blocked: true|false, PermissionDecision: ...}`.

**REQ-HK-006 [Event-Driven]** — **When** a shell command hook is executed, the dispatcher **shall** (a) spawn the shell subprocess, (b) pipe `HookInput` as JSON to stdin, (c) wait up to `cfg.Timeout` (default 30s), (d) parse stdout as `HookJSONOutput` JSON if present, (e) treat **exit code 2** as a **blocking signal** equivalent to `HookJSONOutput{Continue: false}` (Claude Code hook convention compatibility) — the dispatcher **shall** capture stderr up to 4KiB and surface it as `PermissionDecision.Reason` (for `PreToolUse`) or as `AdditionalContext` (for other events), and **shall not** treat exit code 2 as `handler_error`, (f) treat non-zero exit codes **other than 2** OR malformed JSON on a zero-exit OR timeout as `handler_error` — log and continue to the next handler (no block on error), (g) a stdout JSON `{"continue": false}` on exit code 0 remains a valid blocking channel and takes precedence over exit code semantics when both are present.

> Rationale: Existing MoAI-ADK hook scripts at `.claude/hooks/moai/*.py` (see research.md §3.2) follow the Claude Code convention of exiting with code 2 to signal "block with stderr shown to user". Without this mapping, those scripts would be misclassified as errors when executed through `InlineCommandHandler`, defeating the consumption-target goal stated in research.md.

**REQ-HK-007 [Event-Driven]** — **When** `DispatchSessionStart(ctx, input)` is invoked, the dispatcher **shall** collect `initialUserMessage`, `watchPaths`, `additionalContext` from all handler outputs; multiple handlers' `watchPaths` **shall** be concatenated (dedup by absolute path), and `initialUserMessage` **shall** use the last non-empty value with a zap warn if multiple handlers set it.

**REQ-HK-008 [Event-Driven]** — **When** `DispatchFileChanged(ctx, changed []string)` is invoked, the dispatcher **shall** (a) invoke all handlers registered for `FileChanged` matching any path in `changed`, (b) after the internal handlers complete, call the externally-registered `SkillsFileChangedConsumer` (SKILLS-001's `FileChangedConsumer`) with the same `changed` slice, (c) return the union of activated skill IDs.

**REQ-HK-009 [Event-Driven]** — **When** `useCanUseTool(ctx, toolName, input, permCtx)` is called, the decision flow **shall** (a) run the YOLO classifier (auto-mode approval check), (b) if not auto-approved, dispatch `PermissionRequest` hooks, (c) if any hook returns `behavior: deny`, short-circuit with `PermissionResult{Behavior: Deny, DecisionReason: ...}` **and shall invoke `DispatchPermissionDenied(ctx, result)` before returning** so that `EvPermissionDenied` observers see the denial, (d) if the final decision resolved by any branch below is `Deny`, the dispatcher **shall** likewise invoke `DispatchPermissionDenied` exactly once per call, (e) if no handler decides, route to `InteractiveHandler` (CLI-001) → `CoordinatorHandler` (SUBAGENT-001 coordinator mode) → `SwarmWorkerHandler` based on `permCtx.Role`.

**REQ-HK-010 [Event-Driven]** — **When** a hook returns `HookJSONOutput{Async: true, AsyncTimeout: N}`, the dispatcher **shall** spawn a goroutine to execute that handler with a deadline of `N` seconds (default 60s); the main dispatch **shall not** block on async handlers, and their output **shall** be logged but discarded unless the event is `PostToolUse` which appends `additionalContext`.

### 4.3 State-Driven (상태 기반)

**REQ-HK-011 [State-Driven]** — **While** a `PreToolUse` handler is blocking (has returned `Continue: false`), subsequent `PreToolUse` handlers for the same tool invocation **shall not** be invoked; the dispatcher **shall** short-circuit and return immediately with the blocking handler's `permissionDecision`.

**REQ-HK-012 [State-Driven]** — **While** YOLO classifier is set to `auto-approve` for a tool pattern (via `SetYoloClassifierApproval`), `useCanUseTool` calls matching that pattern **shall** return `PermissionResult{Behavior: Allow, DecisionReason: {Type: "yolo_auto"}}` without invoking interactive/coordinator handlers.

**REQ-HK-013 [State-Driven]** — **While** `PluginHookLoader.IsLoading == true`, the registry **shall** reject new `Register` calls with a registry-locked error; this prevents partial plugin loads from being observable. The concrete error type name is defined in §6.2.

### 4.4 Unwanted Behavior (방지)

> **REQ 번호 배치 주석 (D14, iter-2 감사 대응)**: 본 절 §4.4 는 v0.2.0 에서 REQ-HK-021 / REQ-HK-022 가 추가됨에 따라 문서 순서가 014→015→016→017→018→021→022 가 되었고, §4.5 Optional 은 019→020 을 유지한다. 이는 **이미 게시된 REQ 번호의 재배치를 금지하는 SPEC 안정성 제약**(외부 인용·테스트 ID 고정) 때문이며, 의도된 비단조성이다. 향후 추가 Unwanted REQ 는 023+ 에서 시작한다. 모든 REQ ID 의 유일성·완전성(MP-1)은 만족된다.

**REQ-HK-014 [Unwanted]** — **If** a hook handler attempts to mutate the `HookInput` object delivered to it, **then** the dispatcher **shall** ensure the mutation is not observable to downstream handlers; each handler **shall** receive an independent deep copy of `HookInput`.

**REQ-HK-015 [Unwanted]** — **If** a `HookInput` instance fails schema validation prior to dispatch, **then** the dispatcher **shall** return an invalid-input error immediately without invoking any handler. The concrete error type name is defined in §6.2.

**REQ-HK-016 [Unwanted]** — **If** a shell command hook is requested for execution, **then** the dispatcher **shall not** elevate privileges (no `sudo` invocation, no capability bit setting); responsibility for any required privilege escalation rests with the user-provided command string itself.

**REQ-HK-017 [Unwanted]** — **If** every registered handler in a `useCanUseTool` flow abstains (returns no `PermissionResult`), **then** the dispatcher **shall not** default to `Allow`; instead the dispatcher **shall** default to `Ask` routed to `InteractiveHandler` when stdout is a TTY, and **shall** default to `Deny` with reason `"no_interactive_fallback"` when stdout is not a TTY.

**REQ-HK-018 [Unwanted]** — **If** `ClearThenRegister` is invoked, **then** the registry **shall not** fire any handler during the swap; the registry **shall not** provide any `OnLoad` callback hook, and any side effects encoded in handler construction are the handler author's responsibility to keep idempotent.

**REQ-HK-021 [Unwanted]** — **If** a shell command hook is about to be executed, **then** the dispatcher **shall not** start the subprocess without applying the following four minimum isolation guarantees: (a) **environment scrub** — environment variable names matching the deny-list defined in §6.11 **shall not** be propagated to the child process; (b) **working directory pin** — the subprocess working directory **shall** default to the session workspace root resolved via the `WorkspaceRoot(sessionID string) string` resolver supplied by SPEC-GOOSE-CORE-001; if the resolver returns an empty string or fails, the dispatcher **shall** fail-closed by returning a session-resolution error rather than falling back to the parent process's current working directory (see §6.11); (c) **resource limits** — where the host OS supports `syscall.Setrlimit` (Linux, macOS), the dispatcher **shall** apply the resource limits defined in §6.11. The CPU-time limit **shall** be derived from `cfg.Timeout`; if `cfg.Timeout ≤ 0`, the dispatcher **shall** treat the timeout as the system default (30 s) and emit a WARN-level log entry; on platforms without rlimit support the dispatcher **shall** log a single WARN-level entry per session and proceed; (d) **FD hygiene** — every parent-process file descriptor opened before subprocess start **shall** be marked close-on-exec so that hook subprocesses cannot inherit it. These four guarantees are the minimum isolation contract; stronger sandboxing (seccomp, cgroups, WASM) is deferred to a separate SPEC (see §8 R6). The concrete deny-list, rlimit constants, and resolver fallback are defined in §6.11 ("Isolation Defaults").

**REQ-HK-022 [Unwanted]** — **If** the JSON serialization of a `HookInput` instance is strictly greater than 4 × 1024 × 1024 bytes (i.e. `len(json.Marshal(input)) > 4 MiB`; exactly 4 MiB is permitted), **then** the dispatcher **shall** return a payload-too-large error before any subprocess is started, emit a WARN-level structured log entry containing the fields `event`, `handler_id`, and `payload_bytes`, and proceed to the next handler. Writes to subprocess stdin **shall** be performed on a dedicated goroutine so that a slow-reading subprocess cannot deadlock the dispatcher; that goroutine **shall** be cancelled when the parent `context.Context` is done. The concrete error type name is defined in §6.2.

### 4.5 Optional (선택적)

**REQ-HK-019 [Optional]** — **Where** a trace-enable environment variable is set in the parent process environment, the dispatcher **shall** emit DEBUG-level trace logs including the full `HookInput` and `HookOutput` JSON for every handler invocation. The concrete environment variable name and accepted values are defined in §6.11.

**REQ-HK-020 [Optional]** — **Where** a handler declares `matcher: "regex:..."` the matcher **shall** be interpreted as a Go `regexp.Regexp`; bare strings **shall** be interpreted as glob patterns.

---

## 5. 수용 기준 (Acceptance Criteria)

> **EARS 직접 변환 (D3, iter-2 critical 대응).** 본 절의 모든 AC는 §4 의 다섯 EARS 패턴(Ubiquitous / Event-Driven / State-Driven / Unwanted / Optional) 중 하나로 작성된다. 각 AC 는 (1) `[EARS 패턴]` 라벨, (2) 단일 EARS 문장(`When/While/If/Where ... the [system] shall [observable response]`), (3) 검증 대상 REQ 식별자(`Verifies REQ-HK-XXX`), 그리고 보조적으로 (4) RED-phase 테스트에 바인딩되는 구체 시나리오 블록을 포함한다. 보조 시나리오 블록은 EARS 문장의 검증 절차(test fixture)일 뿐 EARS 컴플라이언스의 일부가 아니다 — 컴플라이언스는 (2) 한 줄로만 평가된다. **24 개의 AC 가 22 개의 REQ 를 1:N 매핑으로 모두 커버한다** (REQ-HK-021 은 4-clause 구조 때문에 AC-HK-021/022/023 의 3 개로 split; REQ-HK-005 와 REQ-HK-011 은 단일 AC-HK-002 로 묶음; REQ-HK-009 는 AC-HK-008 + AC-HK-009 의 2 개로 split; 그 외 REQ 는 1:1).

---

**AC-HK-001 — 24-element HookEvent enum completeness**
- **EARS [Ubiquitous]**: The `HookEventNames()` function **shall** return exactly 24 distinct strings whose set is equal to the 24 `Ev*` constants enumerated in §6.2 and whose set excludes `Elicitation`, `ElicitationResult`, and `InstructionsLoaded`.
- **Verifies**: REQ-HK-001
- **Test scenario**: Given `internal/hook/types.go`; When the test invokes `HookEventNames()`; Then the result is the set `{Setup, SessionStart, SubagentStart, UserPromptSubmit, PreToolUse, PostToolUse, PostToolUseFailure, CwdChanged, FileChanged, WorktreeCreate, WorktreeRemove, PermissionRequest, PermissionDenied, Notification, PreCompact, PostCompact, Stop, StopFailure, SubagentStop, TeammateIdle, TaskCreated, TaskCompleted, SessionEnd, ConfigChange}`; And presence of any of `Elicitation`, `ElicitationResult`, `InstructionsLoaded` causes test failure.

**AC-HK-002 — PreToolUse blocking and second-handler suppression**
- **EARS [Event-Driven]**: When `DispatchPreToolUse(ctx, input)` is invoked with two handlers registered for `input.Tool.Name` in FIFO order where the first handler returns `HookJSONOutput{Continue: ptrFalse, PermissionDecision: {Approve: false, Reason: "unsafe"}}`, the dispatcher **shall** return `PreToolUseResult{Blocked: true, PermissionDecision: {Approve: false, Reason: "unsafe"}}` and **shall not** invoke the second handler.
- **Verifies**: REQ-HK-005, REQ-HK-011
- **Test scenario**: Given two handlers H1, H2 bound to tool `rm_rf` where H1 returns `Continue: false`; When `DispatchPreToolUse(ctx, input{Tool: "rm_rf"})` is called; Then H2 invocation count equals 0 and the result satisfies the EARS clause above.

**AC-HK-003 — Shell command hook stdin/stdout roundtrip on exit code 0**
- **EARS [Event-Driven]**: When `DispatchPostToolUse(ctx, input)` is invoked with one `InlineCommandHandler` whose command exits with code 0 after consuming `HookInput` JSON from stdin and emitting valid `HookJSONOutput` JSON on stdout, the dispatcher **shall** include the parsed `HookJSONOutput` value of that handler in the returned `PostToolUseResult` aggregate.
- **Verifies**: REQ-HK-006 (clauses a–d, g)
- **Test scenario**: Given an `InlineCommandHandler{Command: "cat | jq -c '.tool'", Timeout: 5s}`; When `DispatchPostToolUse(ctx, input{Tool: "test", Output: "..."})` is called; Then the subprocess receives the marshalled `HookInput` on stdin, exits 0 with `"test"` on stdout, and the dispatcher's `PostToolUseResult` contains the parsed value.

**AC-HK-004 — Shell command hook timeout produces handler_error and continues**
- **EARS [Event-Driven]**: When a shell command hook execution exceeds `cfg.Timeout`, the dispatcher **shall** terminate the subprocess, emit a `handler_error` log entry, and proceed to invoke the next registered handler in FIFO order without blocking the outer dispatch.
- **Verifies**: REQ-HK-006 (clause f, timeout branch)
- **Test scenario**: Given two handlers H1 = `InlineCommandHandler{Command: "sleep 60", Timeout: 2s}` and H2 = a no-op handler; When `DispatchPreToolUse(ctx, input)` is called; Then H1 is killed at the 2 s deadline, a `handler_error` log entry is emitted, and H2 is invoked exactly once.

**AC-HK-005 — DispatchSessionStart aggregates watchPaths and last initialUserMessage**
- **EARS [Event-Driven]**: When `DispatchSessionStart(ctx, input)` is invoked with multiple handlers each returning a partial `SessionStartResult`, the dispatcher **shall** return a result whose `WatchPaths` is the absolute-path-deduplicated concatenation across handler outputs and whose `InitialUserMessage` is the last non-empty value (with a WARN log emitted when more than one handler sets `InitialUserMessage`).
- **Verifies**: REQ-HK-007
- **Test scenario**: Given H1 returning `{WatchPaths: ["src/"]}` and H2 returning `{WatchPaths: ["tests/"], InitialUserMessage: "hi"}`; When `DispatchSessionStart` is called; Then the result equals `{WatchPaths: ["src/", "tests/"], InitialUserMessage: "hi"}` and no WARN is emitted (single setter).

**AC-HK-006 — DispatchFileChanged invokes the registered SkillsFileChangedConsumer**
- **EARS [Event-Driven]**: When `DispatchFileChanged(ctx, changed)` is invoked with no internal handlers registered for `FileChanged` and an externally-registered `SkillsFileChangedConsumer` set via `SetSkillsFileChangedConsumer(fn)` (§6.2), the dispatcher **shall** invoke that consumer exactly once with the same `changed` slice and **shall** return the consumer's returned set of activated skill IDs unchanged.
- **Verifies**: REQ-HK-008
- **Test scenario**: Given a stub consumer that records its `paths` argument and returns `["skill-a"]`, registered via `SetSkillsFileChangedConsumer(stub)`, and zero internal `FileChanged` handlers; When `DispatchFileChanged(ctx, ["src/foo.ts"])` is called; Then the stub records exactly `["src/foo.ts"]` and the dispatcher returns `["skill-a"]`.

**AC-HK-007 — useCanUseTool YOLO classifier auto-approve**
- **EARS [State-Driven]**: While `SetYoloClassifierApproval("read_file")` has been invoked and not yet cleared, the function `useCanUseTool(ctx, "read_file", input, permCtx)` **shall** return `PermissionResult{Behavior: Allow, DecisionReason: {Type: "yolo_auto"}}` without invoking any `PermissionRequest`, `InteractiveHandler`, `CoordinatorHandler`, or `SwarmWorkerHandler`.
- **Verifies**: REQ-HK-012
- **Test scenario**: Given the YOLO classifier set for tool pattern `read_file`; When `useCanUseTool(ctx, "read_file", {}, permCtx)` is called; Then the result matches the EARS clause and the spy counters for the four downstream handlers all equal 0.

**AC-HK-008 — useCanUseTool handler-deny short-circuits and dispatches PermissionDenied**
- **EARS [Event-Driven]**: When `useCanUseTool(ctx, toolName, input, permCtx)` is invoked with a `PermissionRequest` handler that returns `PermissionResult{Behavior: Deny, DecisionReason: {Reason: "policy"}}`, the dispatcher **shall** (1) return that `PermissionResult` to the caller, (2) invoke `DispatchPermissionDenied(ctx, result)` exactly once before returning, (3) call `RecordAutoModeDenial(toolName, "policy")`, and (4) **shall not** invoke `InteractiveHandler` or `CoordinatorHandler`.
- **Verifies**: REQ-HK-009 (clauses b, c, d)
- **Test scenario**: Given a `PermissionRequest` handler that always returns Deny, plus spies for `DispatchPermissionDenied`, `RecordAutoModeDenial`, `InteractiveHandler`, `CoordinatorHandler`; When `useCanUseTool(ctx, "rm_rf", {}, permCtx)` is called; Then the four assertions hold per the EARS clause.

**AC-HK-009 — useCanUseTool non-TTY safe-default is Deny**
- **EARS [Unwanted]**: If `useCanUseTool(ctx, toolName, input, permCtx)` is invoked when stdout is not a TTY, no handler produces a `PermissionResult`, and no YOLO approval covers `toolName`, then the dispatcher **shall** return `PermissionResult{Behavior: Deny, DecisionReason: {Reason: "no_interactive_fallback"}}` and **shall** invoke `DispatchPermissionDenied(ctx, result)` exactly once before returning.
- **Verifies**: REQ-HK-009 (clause d), REQ-HK-017
- **Test scenario**: Given no `PermissionRequest` handler, stdout piped to a non-TTY file, no YOLO entry; When `useCanUseTool(ctx, "unknown_tool", {}, permCtx)` is called; Then the result matches the EARS clause and the `DispatchPermissionDenied` spy count equals 1.

**AC-HK-010 — ClearThenRegister atomic swap under concurrent readers**
- **EARS [Ubiquitous]**: While two or more goroutines are concurrently invoking `Handlers(event, input)` against a registry, every observed return value **shall** be either entirely the pre-swap snapshot A or entirely the post-swap snapshot B (never an interleaving of the two), and the final stored snapshot after `ClearThenRegister(B)` returns **shall** equal B.
- **Verifies**: REQ-HK-003
- **Test scenario**: Given snapshot A registered and two reader goroutines spinning on `Handlers(EvPreToolUse, input)`; When a third goroutine invokes `ClearThenRegister(B)`; Then `go test -race` passes, every reader's observed slice equals A or equals B, and the post-swap registry returns B.

---

**AC-HK-011 — Handlers preserve registration order without deduplication**
- **EARS [Ubiquitous]**: When the same event has multiple handlers registered in order H1 → H2 → H3 (including duplicates), `Handlers(event, input)` **shall** return them in exactly that order with no reordering and no deduplication.
- **Verifies**: REQ-HK-002
- **Test scenario**: Given three handlers registered in order H1, H2, H3 for `EvPreToolUse`, including a re-registration of H1 after H3; When `Handlers(EvPreToolUse, input)` is invoked; Then the returned slice is `[H1, H2, H3, H1]` in that exact order.

**AC-HK-012 — Every dispatch emits structured INFO/ERROR log entry**
- **EARS [Ubiquitous]**: When any `Dispatch*` function is invoked, the dispatcher **shall** emit, before returning, exactly one structured log entry whose fields include the keys `event`, `handler_count`, `outcome`, and `duration_ms`, at INFO level when no handler returned a Go error and at ERROR level when at least one handler returned a Go error.
- **Verifies**: REQ-HK-004
- **Test scenario**: Given a captured logger and a single handler that returns nil (success case) or a stub error (failure case); When `DispatchPreToolUse` is invoked; Then the captured log buffer contains exactly one entry with the four required fields and the level matches success/failure as specified.

**AC-HK-013 — Async hook output is non-blocking and discarded outside PostToolUse**
- **EARS [Event-Driven]**: When a hook handler returns `HookJSONOutput{Async: true, AsyncTimeout: N}` for an event other than `PostToolUse`, the dispatcher **shall** return to the caller before the asynchronous handler completes and **shall** discard the asynchronous handler's eventual output (logging only); for `PostToolUse`, the dispatcher **shall** instead append the asynchronous output's `AdditionalContext` to the aggregated result when the goroutine completes within the deadline `N` (default 60 s when `N == 0`).
- **Verifies**: REQ-HK-010
- **Test scenario**: Given a `PreToolUse` async handler with `AsyncTimeout: 5` that sleeps 1 s before emitting `AdditionalContext: "x"`; When `DispatchPreToolUse` is invoked; Then the dispatcher returns within 100 ms (well below the 1 s sleep) and the captured log contains the discarded async output, while the returned `PreToolUseResult` does not include `"x"`.

**AC-HK-014 — Registry rejects Register while PluginHookLoader.IsLoading**
- **EARS [State-Driven]**: While `PluginHookLoader.IsLoading == true`, an invocation of `HookRegistry.Register(event, matcher, handler)` **shall** return the registry-locked error defined in §6.2 without mutating the registry's current snapshot.
- **Verifies**: REQ-HK-013
- **Test scenario**: Given a registry with snapshot A and a `PluginHookLoader` stub whose `IsLoading()` returns true; When `Register(EvPreToolUse, "*", h)` is called; Then the call returns the registry-locked error and a subsequent `Handlers(EvPreToolUse, input)` returns the slice from snapshot A unchanged.

**AC-HK-015 — Handler mutation of HookInput is not observable downstream**
- **EARS [Unwanted]**: If handler H1 modifies any field of the `HookInput` instance it received during `Handle`, then handler H2 invoked next for the same dispatch **shall** observe the original (unmutated) values for every field of its received `HookInput`.
- **Verifies**: REQ-HK-014
- **Test scenario**: Given H1 that mutates `input.Input["k"] = "tampered"` and `input.CustomData["k"] = "tampered"`, followed by H2 that asserts `input.Input["k"] == originalValue` and `input.CustomData["k"] == originalValue`; When `DispatchPreToolUse(ctx, input)` is called; Then H2's assertions pass.

**AC-HK-016 — Schema-invalid HookInput is rejected before any handler runs**
- **EARS [Unwanted]**: If a `HookInput` instance fails the schema validation defined for its `HookEvent` (e.g. `EvFileChanged` with `ChangedPaths == nil`), then the dispatcher **shall** return the invalid-input error defined in §6.2 immediately and **shall not** invoke any registered handler.
- **Verifies**: REQ-HK-015
- **Test scenario**: Given two handlers registered for `EvFileChanged` with hit-counters; When `DispatchFileChanged(ctx, nil)` is invoked with a malformed input; Then the call returns the invalid-input error and both handlers' hit-counters remain at 0.

**AC-HK-017 — Shell command hook does not invoke sudo or set capability bits**
- **EARS [Unwanted]**: If a shell command hook is dispatched, then the dispatcher **shall not** prepend `sudo`, **shall not** invoke `setuid`/`setgid`/`setcap` system calls, and **shall** invoke the user-supplied command exclusively via `exec.Command(cfg.Shell, "-c", command)`.
- **Verifies**: REQ-HK-016
- **Test scenario**: Given an `InlineCommandHandler{Command: "id -u"}` and a shim `cfg.Shell` that records its argv to a test file; When `DispatchPreToolUse` is called; Then the recorded argv equals `[shim, "-c", "id -u"]` exactly (no `sudo` prefix), and a strace-style spy verifies no `setuid`/`setgid`/`setcap` syscalls.

**AC-HK-018 — ClearThenRegister fires no handler during the swap**
- **EARS [Unwanted]**: If `ClearThenRegister(snapshot)` is invoked, then for every handler `H` referenced in either the prior snapshot or the new `snapshot`, `H.Handle` **shall not** be invoked as a side effect of the swap operation itself.
- **Verifies**: REQ-HK-018
- **Test scenario**: Given a prior snapshot A containing handler `Ha` with a call-counter and a new snapshot B containing handler `Hb` with a call-counter; When `ClearThenRegister(B)` is invoked once; Then both counters remain at 0, and only an explicit subsequent `DispatchPreToolUse` increments `Hb`'s counter.

**AC-HK-019 — Trace environment variable enables DEBUG-level dispatch logs**
- **EARS [Optional]**: Where the trace-enable environment variable defined in §6.11 is present in the parent process environment with the activation value defined in §6.11, the dispatcher **shall** emit, in addition to the standard structured log entry, one DEBUG-level log entry per handler invocation that includes the full `HookInput` and `HookOutput` JSON.
- **Verifies**: REQ-HK-019
- **Test scenario**: Given the trace-enable env var set to its activation value and a single handler returning a known `HookJSONOutput`; When `DispatchPreToolUse` is invoked; Then the captured logger contains one DEBUG entry whose `input` and `output` fields parse back to the original `HookInput` and `HookOutput`.

**AC-HK-020 — Matcher prefix `regex:` selects regex semantics; bare strings select glob**
- **EARS [Optional]**: Where a handler is registered with matcher string starting with the prefix `regex:`, the dispatcher **shall** evaluate handler selection by compiling the suffix as `regexp.Regexp` and matching against `input.Tool.Name` (or the equivalent matchable field per §6.7); for any other matcher form (no prefix, `glob:` prefix, or the literal `*`), the dispatcher **shall** evaluate selection via `filepath.Match` glob semantics.
- **Verifies**: REQ-HK-020
- **Test scenario**: Given two handlers registered for `EvPreToolUse` with matchers `"regex:^rm_.*"` and `"read_*"`, plus inputs `Tool.Name = "rm_rf"` and `Tool.Name = "read_file"`; When `Handlers(EvPreToolUse, input)` is invoked for each; Then the regex handler matches only `rm_rf` and the glob handler matches only `read_file`.

**AC-HK-021 — Subprocess environment scrub strips deny-listed variables**
- **EARS [Unwanted]**: If a shell command hook subprocess is started while the parent process environment contains any variable name matching the deny-list defined in §6.11, then the child subprocess's environment **shall not** contain any of those matching variable names.
- **Verifies**: REQ-HK-021 (clause a)
- **Test scenario**: Given the parent environment populated with `ANTHROPIC_API_KEY=xyz`, `OPENAI_API_KEY=abc`, `MINK_AUTH_TOKEN=zzz`, `MY_TOKEN=t`, `PASSWORD=p`, `HARMLESS=keep`; and an `InlineCommandHandler{Command: "env"}`; When the subprocess is started; Then the captured stdout includes `HARMLESS=keep` and excludes every variable whose name matches the §6.11 deny-list.

**AC-HK-022 — Subprocess working directory is pinned to the resolved session workspace root**
- **EARS [Unwanted]**: If a shell command hook subprocess is started with `HookInput.SessionID == sid` and the `WorkspaceRoot` resolver supplied by SPEC-GOOSE-CORE-001 returns a non-empty path `p` for `sid`, then the subprocess's working directory **shall** equal `p` regardless of the parent process's current working directory; if the resolver returns an empty string or fails, the dispatcher **shall** return the session-resolution error defined in §6.11 without starting the subprocess.
- **Verifies**: REQ-HK-021 (clause b), D15 resolution
- **Test scenario**: Given a stub resolver mapping `sid="S1" → "/ws/S1"` and a separate test mapping `sid="S2" → ""`; and an `InlineCommandHandler{Command: "pwd"}`; When the dispatcher invokes the handler with `SessionID="S1"` from a parent CWD of `/tmp`; Then the captured stdout equals `/ws/S1`. When `SessionID="S2"`, the dispatcher returns the session-resolution error and the subprocess is never started.

**AC-HK-023 — Subprocess applies rlimit and FD-hygiene defaults; degrades gracefully on unsupported OS**
- **EARS [Unwanted]**: If a shell command hook subprocess is started on a host OS that supports `syscall.Setrlimit`, then the subprocess **shall** be started with the resource limits defined in §6.11 (memory, file-descriptors, CPU time derived from `cfg.Timeout`) and with all parent-process file descriptors marked close-on-exec; if the host OS does not support `syscall.Setrlimit`, then the dispatcher **shall** emit exactly one WARN-level log entry per session indicating rlimit unavailability and **shall** still apply the close-on-exec marking.
- **Verifies**: REQ-HK-021 (clauses c, d), REQ-HK-021 cfg.Timeout edge case (D17)
- **Test scenario**: On Linux: given `cfg.Timeout = 30 s` and an `InlineCommandHandler{Command: "ulimit -v -n -t"}`; When the subprocess starts; Then the captured stdout reflects the limits defined in §6.11 (e.g. `RLIMIT_AS = 1 GiB`, `RLIMIT_NOFILE = 128`, `RLIMIT_CPU = 35 s`). Additional case: given `cfg.Timeout = 0`; Then the dispatcher emits a WARN log and uses the §6.11 default (30 s) for CPU time. On unsupported-OS stub: a single WARN-per-session entry is emitted and a sentinel parent FD opened before `cmd.Start()` is verified unreadable from the child via `/proc/self/fd` enumeration (Linux) or `lsof` equivalent.

**AC-HK-024 — Oversized HookInput payload is rejected before subprocess start**
- **EARS [Unwanted]**: If `len(json.Marshal(input))` is strictly greater than 4 × 1024 × 1024 bytes, then the dispatcher **shall** return the payload-too-large error defined in §6.2 before any subprocess is started, **shall** emit a WARN-level structured log entry containing the keys `event`, `handler_id`, `payload_bytes`, and **shall** proceed to invoke the next registered handler in FIFO order.
- **Verifies**: REQ-HK-022, D16 boundary clarification
- **Test scenario**: Given a `HookInput` whose `CustomData` is padded so that `len(json.Marshal(input)) == 4*1024*1024 + 1` (exactly one byte over the limit), plus a second handler H2 with a hit-counter; When `DispatchPreToolUse` is invoked; Then the first handler's subprocess is never started, the dispatcher emits the payload-too-large error and a WARN log with the three required fields, and H2's hit-counter increments to 1. Boundary test: a payload of exactly `4*1024*1024` bytes **shall** be accepted (no error).

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

// SkillsFileChangedConsumer registration API (D11 resolution).
// SKILLS-001은 dispatcher가 FileChanged 이벤트의 내부 핸들러 chain 완료 후
// 호출할 외부 consumer 함수를 다음 메서드로 등록한다. 등록 시맨틱:
//   - replace-only: 두 번째 호출은 첫 번째를 덮어쓴다 (chain 추가 아님).
//   - nil 인자는 등록 해제(unregister)와 동치.
//   - thread-safe: 내부적으로 mu mutex 또는 atomic.Pointer로 보호.
//   - DispatchFileChanged의 호출 시점에 nil이면 호출 생략 + 빈 결과 반환.
type SkillsFileChangedConsumer func(ctx context.Context, changed []string) []string
func (r *HookRegistry) SetSkillsFileChangedConsumer(fn SkillsFileChangedConsumer)

// Registry-locked error (REQ-HK-013, AC-HK-014의 "registry-locked error").
var ErrRegistryLocked = errors.New("hook: registry is locked while plugin loader is loading")

// Invalid HookInput error (REQ-HK-015, AC-HK-016의 "invalid-input error").
var ErrInvalidHookInput = errors.New("hook: HookInput failed schema validation")

// Payload-too-large error (REQ-HK-022, AC-HK-024의 "payload-too-large error").
var ErrHookPayloadTooLarge = errors.New("hook: HookInput JSON exceeds 4 MiB limit")

// Session-resolution error (REQ-HK-021 b clause, AC-HK-022의 "session-resolution error").
var ErrHookSessionUnresolved = errors.New("hook: WorkspaceRoot resolver returned empty path or failed")

// DispatchPermissionDenied는 EvPermissionDenied enum 상수와 1:1 대응한다.
// useCanUseTool이 Deny로 귀결될 때 자동 호출된다 (REQ-HK-009 c/d절).
// 외부 관찰자(감사 로그 소비자, 후속 SUBAGENT-001 coordinator, 텔레메트리 consumer)가
// 이 이벤트에 핸들러를 등록할 수 있다. 중복 호출은 허용되지 않는다 — useCanUseTool 한 번당 최대 1회.
func DispatchPermissionDenied(ctx context.Context, result PermissionResult) DispatchResult

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

// Role enum (D12 resolution): permCtx.Role 값 집합.
// 본 SPEC은 fallback 동작까지 포함한 4가지를 명시한다.
// SUBAGENT-001 이 추가 Role 값을 도입하는 경우, 본 SPEC v0.4+ 에서 enum 확장으로 동기화한다.
type Role string
const (
    RoleCoordinator  Role = "coordinator"   // SUBAGENT-001 coordinator 모드 — CoordinatorHandler로 라우팅
    RoleSwarmWorker  Role = "swarm_worker"  // SUBAGENT-001 swarm worker 모드 — SwarmWorkerHandler로 라우팅
    RoleInteractive  Role = "interactive"   // 사용자 터미널 세션 — InteractiveHandler로 라우팅 (TTY 가정)
    RoleNonTTY       Role = "non_tty"       // 자동화·CI·파이프라인 환경 — Deny(no_interactive_fallback)로 즉시 귀결
)
// permCtx.Role 이 위 4가지 중 하나도 아닌 경우 (unknown 문자열): RoleInteractive로 fallback하되 §6.10 trace
// 로그에 "unknown_role" WARN 한 번 emit. 빈 문자열은 RoleNonTTY로 처리.

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

본 절은 REQ-HK-006 (exit 2 compatibility), REQ-HK-021 (subprocess isolation), REQ-HK-022 (4 MiB payload cap)의 구조적 계약을 기술한다. 코드는 behavior 계약을 설명하는 pseudo-Go이며 구체 구현은 Run 단계(`internal/hook/handlers.go`)의 책임이다.

구조적 순서:

1. `cctx, cancel := context.WithTimeout(ctx, h.Timeout)` — 타임아웃 바인딩
2. `inputJSON := json.Marshal(input)` — 직렬화 후 `len(inputJSON) > 4*1024*1024`이면 `ErrHookPayloadTooLarge` 즉시 반환 (REQ-HK-022)
3. `cmd := exec.CommandContext(cctx, h.Shell, "-c", h.Command)` — 명령 준비
4. **Env scrub (REQ-HK-021 a)**: `cmd.Env` 를 parent `os.Environ()` 에서 deny-list 필터링한 slice로 설정
5. **CWD pin (REQ-HK-021 b)**: `path, err := resolver.WorkspaceRoot(input.SessionID)` (resolver 는 §6.11.2 정의), `(path != "" && err == nil)` 이면 `cmd.Dir = path`, 그 외에는 `ErrHookSessionUnresolved` 즉시 반환
6. **rlimit + CloseOnExec (REQ-HK-021 c/d)**: `cmd.SysProcAttr` 에 플랫폼별 rlimit 설정 (Linux/macOS) 및 `O_CLOEXEC` 마킹
7. `stdin, _ := cmd.StdinPipe()` 후 `cmd.Start()`
8. **stdin goroutine write (REQ-HK-022)**: 별도 goroutine에서 `stdin.Write(inputJSON); stdin.Close()` — 메인 goroutine이 `cmd.Wait()`에서 블록되더라도 slow-reading child로 인한 pipe 데드락 방지; goroutine은 `cctx.Done()`에서 취소
9. `err := cmd.Wait()` 후 exit code 분기:
   - `exitCode == 0` — stdout을 `HookJSONOutput`으로 파싱, malformed면 `handler_error`
   - `exitCode == 2` — **blocking signal**로 승격: `stderr`(선두 4 KiB)을 `PermissionDecision.Reason` 또는 `AdditionalContext`로 복사한 `HookJSONOutput{Continue: ptrFalse, PermissionDecision: ...}` 합성 후 반환 (REQ-HK-006 e)
   - 기타 non-zero exit — `handler_error`로 로그 후 빈 output 반환 (REQ-HK-006 f)
10. 모든 경로에서 `{event, handler_id, exit_code, duration_ms, payload_bytes}` 구조적 로그 emit (REQ-HK-004)

> 이전 버전 (v0.1.0)의 inline code block은 synchronous `stdin.Write` 로 pipe 데드락 리스크를 내포했다. v0.2.0부터 REQ-HK-022에 의해 별도 goroutine write가 규정된다 (research.md §4.2와도 정합).

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

각 AC 는 1 개의 RED-phase 테스트에 1:1 매핑된다. 본 SPEC 은 22 REQ × 23 AC × 23 테스트 구조로 traceability 를 보장한다 (REQ-HK-021 은 4-clause 구조 때문에 AC-HK-021/022/023 의 3 개로 split, 그 외 모든 REQ 는 1:1 또는 1:N).

기존 ACs (AC-HK-001 ~ 010):

1. **RED #1** — `TestHookEventNames_Exactly24` (AC-HK-001 / REQ-HK-001)
2. **RED #2** — `TestDispatchPreToolUse_HandlerBlocks` (AC-HK-002 / REQ-HK-005, REQ-HK-011)
3. **RED #3** — `TestInlineCommandHandler_Roundtrip` (AC-HK-003 / REQ-HK-006 a-d, g)
4. **RED #4** — `TestInlineCommandHandler_Timeout` (AC-HK-004 / REQ-HK-006 f)
5. **RED #5** — `TestDispatchSessionStart_Aggregation` (AC-HK-005 / REQ-HK-007)
6. **RED #6** — `TestDispatchFileChanged_SkillsConsumer` (AC-HK-006 / REQ-HK-008)
7. **RED #7** — `TestUseCanUseTool_YOLO_AutoApprove` (AC-HK-007 / REQ-HK-012)
8. **RED #8** — `TestUseCanUseTool_HandlerDenyShortCircuit` (AC-HK-008 / REQ-HK-009 b/c/d)
9. **RED #9** — `TestUseCanUseTool_NonTTY_Deny` (AC-HK-009 / REQ-HK-009 d, REQ-HK-017)
10. **RED #10** — `TestRegistry_ClearThenRegister_RaceFree` (AC-HK-010 / REQ-HK-003)

신규 ACs (AC-HK-011 ~ 024, v0.3.0 추가):

11. **RED #11** — `TestRegistry_FIFO_Order_NoDeduplication` (AC-HK-011 / REQ-HK-002)
12. **RED #12** — `TestDispatch_StructuredLog_INFO_ERROR` (AC-HK-012 / REQ-HK-004)
13. **RED #13** — `TestDispatchPreToolUse_AsyncNonBlocking` (AC-HK-013 / REQ-HK-010)
14. **RED #14** — `TestRegistry_RejectRegister_WhileLoading` (AC-HK-014 / REQ-HK-013)
15. **RED #15** — `TestDispatch_DeepCopy_NoMutationLeak` (AC-HK-015 / REQ-HK-014)
16. **RED #16** — `TestDispatch_RejectInvalidSchema_BeforeHandlers` (AC-HK-016 / REQ-HK-015)
17. **RED #17** — `TestShellHook_NoSudo_NoCapability` (AC-HK-017 / REQ-HK-016)
18. **RED #18** — `TestClearThenRegister_NoHandlerFiredDuringSwap` (AC-HK-018 / REQ-HK-018)
19. **RED #19** — `TestDispatch_TraceEnv_DEBUG` (AC-HK-019 / REQ-HK-019)
20. **RED #20** — `TestRegistry_RegexMatcher_VsGlobMatcher` (AC-HK-020 / REQ-HK-020)
21. **RED #21** — `TestShellHook_EnvScrub_DenyList` (AC-HK-021 / REQ-HK-021 a)
22. **RED #22** — `TestShellHook_CWDPin_FromSessionID` (AC-HK-022 / REQ-HK-021 b)
23. **RED #23** — `TestShellHook_RLimit_FDHygiene_Defaults` (AC-HK-023 / REQ-HK-021 c, d, D17 edge)
24. **RED #24** — `TestShellHook_PayloadCap_4MiB_Boundary` (AC-HK-024 / REQ-HK-022, D16 boundary)

25. **GREEN** — 최소 구현.
26. **REFACTOR** — dispatcher 공통화(공유 goroutine 관리 코드 추출), permission flow 상태 머신 단순화.

> 비고: AC-HK-021/022/023 의 3 개는 REQ-HK-021 의 4 clause (a env scrub / b CWD / c rlimit / d FD hygiene) 를 검증하지만, c 와 d 는 같은 `cmd.SysProcAttr` 경로에서 동시 검증 가능하므로 AC-HK-023 에 묶었다. 따라서 REQ-HK-021 은 AC-HK-021/022/023 의 union 으로 완전 커버된다.

### 6.10 구조적 로깅 디테일

REQ-HK-004 가 요구하는 구조적 로그 entry 의 구체 구현은 본 절에서 정의한다.

- 로거: `go.uber.org/zap` v1.27+ (CORE-001 계승). `zap.Logger` 인스턴스는 dispatcher 생성 시 주입.
- 표준 dispatch 로그 필드 (REQ-HK-004): `event` (string, `HookEvent` 값), `handler_count` (int, 호출된 handler 수), `outcome` (`"ok" | "blocked" | "handler_error" | "timeout"`), `duration_ms` (int64).
- Shell hook subprocess 로그 추가 필드 (REQ-HK-004 ∩ REQ-HK-006/021/022): `handler_id` (string), `exit_code` (int), `payload_bytes` (int).
- 권한 결정 로그 (`LogPermissionDecision`): `tool_name`, `behavior` (`"allow"|"deny"|"ask"`), `decision_type`, `decision_reason`.
- 레벨 매핑: 정상 완료 = INFO, handler_error/timeout = ERROR, rlimit 미지원 OS의 1회/세션 경고 = WARN, REQ-HK-019 trace 출력 = DEBUG.

### 6.11 Isolation Defaults (REQ-HK-019 / REQ-HK-021 / REQ-HK-022 구현 상수)

본 절은 §4 의 REQ 본문에서 의도적으로 추상화한 구현 식별자(정규식 패턴, 환경변수 이름, rlimit 상수, 에러 식별자)를 한 곳에 모은다. 본 절의 값은 **EVOLVABLE** 이며 SPEC 본문(§4) 의 contract 를 위반하지 않는 범위에서 minor 버전 업그레이드만으로 조정 가능하다.

#### 6.11.1 환경 변수 deny-list (REQ-HK-021 a)

자식 프로세스로 전파하지 **않는** 환경변수 이름:

| 분류 | 패턴 / 이름 |
|-----|---------|
| 정규식 (case-insensitive) | `(?i)(token\|secret\|password\|apikey\|api_key)` |
| 명시적 이름 | `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `MINK_AUTH_*` (glob; `MINK_AUTH_TOKEN`, `MINK_AUTH_REFRESH` 등 모두 포함) |

매칭은 위 두 규칙의 합집합. `HARMLESS`, `PATH`, `HOME`, `USER`, `TMPDIR` 등 일반 변수는 그대로 전파한다.

#### 6.11.2 작업 디렉토리 resolver (REQ-HK-021 b, D15 해결)

```go
// SPEC-GOOSE-CORE-001 이 제공하는 인터페이스. 본 SPEC 은 consumer.
type WorkspaceRootResolver interface {
    WorkspaceRoot(sessionID string) (string, error)
}
```

- `internal/hook/dispatchers.go` 는 dispatcher 생성 시점에 `WorkspaceRootResolver` 인스턴스를 주입받는다 (DI 또는 functional option 패턴).
- 호출 결과 분기:
  - `(path, nil)` 이고 `path != ""` → `cmd.Dir = path`
  - `("", _)` 또는 `(_, err)` → `ErrHookSessionUnresolved` 반환, 자식 프로세스 시작 금지 (fail-closed; 부모 CWD 로의 fallback 은 명시적으로 금지됨)

#### 6.11.3 rlimit 상수 (REQ-HK-021 c, D17 해결)

| rlimit | 기본값 | cfg.Timeout 의존성 |
|--------|-------|------------------|
| `RLIMIT_AS` (가상 메모리) | 1 GiB (`1 << 30` bytes) | 무관 |
| `RLIMIT_NOFILE` (파일 디스크립터 수) | 128 | 무관 |
| `RLIMIT_CPU` (CPU 시간, 초) | `cfg.Timeout + 5s` (cfg.Timeout > 0 일 때) | 의존 |

`cfg.Timeout ≤ 0` 엣지케이스 (D17 해결):
- dispatcher 는 시스템 default 값 `30s` 를 적용 (`RLIMIT_CPU = 30s + 5s = 35s`).
- 동시에 §6.10 표준 로그에 `outcome="config_warn"` 의 WARN 레벨 entry 한 번 emit.
- config 검증 자체(`cfg.Timeout > 0` 강제)는 **본 SPEC 의 책임이 아님** — config 로더(SPEC-GOOSE-CORE-001) 가 책임지며 본 SPEC 은 방어적 default 만 제공.

플랫폼 분기:
- Linux / macOS / *BSD: `syscall.Setrlimit` 직접 호출.
- Windows / 기타: `syscall.Setrlimit` 미지원 → §6.10 WARN 로그 1회/세션 emit, 자식 프로세스는 그대로 시작 (호스트 OS 의 기본 격리에 의존).

#### 6.11.4 close-on-exec (REQ-HK-021 d)

- 부모 프로세스가 `cmd.Start()` 이전에 연 모든 file descriptor 는 `O_CLOEXEC` 또는 `syscall.CloseOnExec(fd)` 로 마킹.
- Go 1.22+ 의 `os.OpenFile` 는 기본적으로 `O_CLOEXEC` 를 설정하지만, 외부 syscall 직접 호출 또는 `dup`/`socket` 으로 얻은 FD 는 명시적 마킹 필요.

#### 6.11.5 Trace-enable 환경변수 (REQ-HK-019)

- 변수 이름: `MINK_HOOK_TRACE`
- 활성화 값: `"1"`, `"true"`, `"on"` (case-insensitive). 그 외 값(빈 문자열, `"0"`, `"false"`, 미설정)은 비활성.

#### 6.11.6 비-TTY 환경 override (R7 mitigation)

- 변수 이름: `MINK_HOOK_NON_INTERACTIVE`
- 활성화 값: `"1"` 으로 설정 시 stdout TTY 검사를 우회하고 `RoleNonTTY` 로 강제 — CI 환경에서 false-positive 회피용.

### 6.12 TRUST 5 매핑

| 차원 | 본 SPEC 달성 방법 |
|-----|-----------------|
| **T**ested | 30+ unit test, 10 integration test (AC), race detector 필수 |
| **R**eadable | types / registry / handlers / permission / dispatchers 5파일 분리, 24 event 상수 주석 |
| **U**nified | `go fmt`, `golangci-lint`, 모든 Dispatch 함수 동일 시그니처 계약 |
| **S**ecured | Shell hook no-sudo, 비-TTY 환경 default deny, malformed JSON fail-safe, deep copy input, env scrub + CWD pin + rlimit + CloseOnExec (REQ-HK-021), 4 MiB payload cap (REQ-HK-022), exit code 2를 block 채널로 분기하여 기존 Claude Code hook 스크립트 호환 (REQ-HK-006) |
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
| R2 | Shell hook의 stdin/stdout deadlock (큰 payload) | 중 | 고 | **REQ-HK-022로 승격** (4 MiB payload cap + `ErrHookPayloadTooLarge` + goroutine stdin write). 본 risk는 REQ로 흡수되어 v0.2.0에서 해소 예정. |
| R3 | `ClearThenRegister` 중 외부가 handler 참조 유지 → use-after-clear | 낮 | 중 | `Handlers()`가 slice를 **반환할 때 복사** (방어적 copy) |
| R4 | YOLO classifier 설정이 session 간 누수 | 중 | 중 | `PermissionQueueOps`의 state는 per-session, 세션 종료 시 `SessionEnd` hook에서 reset |
| R5 | 24개 이벤트 수가 claude-primitives §5.1의 28+와 불일치 | 중 | 낮 | 본 SPEC은 **핵심 24개만 지원**, Elicitation/ElicitationResult/InstructionsLoaded 등은 추후 SPEC 또는 본 SPEC v0.2에서 확장 |
| R6 | Plugin hook이 악의적 코드 실행 (plugin source 신뢰) | 고 | 고 | `ExecCommandHook`은 shell escape를 피하기 위해 `exec.Command(shell, "-c", cmd)` 고정. privilege escalation 차단 (REQ-HK-016). Plugin 소스 감사는 PLUGIN-001의 책임 |
| R7 | 비-TTY 환경 탐지가 CI 환경에서 false positive | 중 | 낮 | `MINK_HOOK_NON_INTERACTIVE=1` env override 제공. CI 설정 권장 |

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
