---
id: SPEC-GOOSE-QUERY-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: 5
labels: []
---

# SPEC-GOOSE-QUERY-001 — Compact (Run 단계용)

> **용도**: `/moai run` 단계에서 token 절약을 위해 spec.md 에서 Requirements / AC / 파일 목록 / Exclusions 만 추출한 압축본. 서술형 배경·기술적 접근·의존성·리스크 상세는 `spec.md` / `plan.md` / `research.md` 를 조회.

**한 줄 요약**: GOOSE-AGENT agentic 코어 런타임 — 하나의 대화 세션 = 하나의 `QueryEngine` 인스턴스 위에서 streaming 응답, tool 실행, permission gating, budget tracking, compaction trigger yield 를 통합한다.

---

## Requirements (EARS)

### Ubiquitous

- **REQ-QUERY-001**: The `QueryEngine` **shall** maintain a one-to-one correspondence between its instance lifetime and a single conversation session (no multiplexing across conversations).
- **REQ-QUERY-002**: The `QueryEngine.SubmitMessage` method **shall** return a `<-chan SDKMessage` (receive-only channel) without buffering model output chunks; each `StreamEvent` delta from the underlying LLM call **shall** be forwarded to the channel as soon as it is parsed.
- **REQ-QUERY-003**: The `queryLoop` **shall** mutate its `State` only at explicitly documented continue sites (post-compaction, post-retry, post-tool-results) and **shall not** mutate `State` from within LLM streaming callbacks.
- **REQ-QUERY-004**: The `QueryEngine` **shall** support multiple sequential `SubmitMessage` invocations within the same instance, where each invocation shares the accumulating `messages[]` array and `task_budget` from prior invocations.

### Event-Driven

- **REQ-QUERY-005**: **When** `SubmitMessage(prompt)` is invoked, the engine **shall** (a) append the user `Message` to `State.messages`, (b) yield a `user_ack` `SDKMessage`, (c) spawn the `queryLoop` goroutine, and (d) return the output channel within 10ms.
- **REQ-QUERY-006**: **When** the LLM response contains a `tool_use` content block, the `queryLoop` **shall** invoke `CanUseTool(ctx, toolName, input, ToolPermissionContext)` and dispatch based on the returned `PermissionBehavior`: `Allow` → yield `permission_check{tool_use_id, behavior:"allow"}` then execute via `tools.Executor`, `Deny` → yield `permission_check{tool_use_id, behavior:"deny", reason}` then synthesize a `ToolResult{is_error: true, content: "denied: <reason>"}`, `Ask` → yield `permission_request{tool_use_id, tool_name, input}` and suspend the loop until a resolution arrives on the engine's permission inbox.
- **REQ-QUERY-007**: **When** a tool returns a result whose serialized `content` exceeds `taskBudget.toolResultCap` bytes, the `queryLoop` **shall** apply tool-result budget replacement: substitute the content with a pointer summary `{tool_use_id, truncated: true, bytes_original, bytes_kept}` and log the replacement.
- **REQ-QUERY-008**: **When** the API returns a `max_output_tokens` terminal event, the `queryLoop` **shall** (a) increment `State.maxOutputTokensRecoveryCount`, (b) if the counter is ≤ 3, re-issue the API call with the same messages array and no modifications, (c) if > 3, transition to `Terminal{success: false, error: "max_output_tokens_exhausted"}`. At the `after_compact` continue site (REQ-QUERY-009), the `queryLoop` **shall** reset `maxOutputTokensRecoveryCount` to 0.
- **REQ-QUERY-009**: **When** `Compactor.ShouldCompact(State)` returns `true` at the top of an iteration, the `queryLoop` **shall** (a) invoke `Compactor.Compact(State)`, (b) yield a `CompactBoundary` `SDKMessage` with the boundary metadata, and (c) continue the loop with the returned new `State`.
- **REQ-QUERY-010**: **When** `ctx.Done()` fires, the `queryLoop` **shall** (a) stop consuming LLM chunks, (b) release any pending tool permissions, (c) yield a `Terminal{success: false, error: "aborted"}` `SDKMessage` on the output channel, and (d) close the output channel, all within 500ms.

### State-Driven

- **REQ-QUERY-011**: **While** `State.turnCount < maxTurns` and `State.taskBudget.remaining > 0` and `ctx.Err() == nil`, the `queryLoop` **shall** continue iteration; reaching `turnCount == maxTurns` **shall** transition to `Terminal{success: true, error: "max_turns"}`, and `taskBudget.remaining <= 0` **shall** transition to `Terminal{success: false, error: "budget_exceeded"}`.
- **REQ-QUERY-012**: **While** `QueryEngineConfig.CoordinatorMode == true`, the `CanUseTool` gate **shall** filter out tools whose manifest declares `scope: "leader_only"`, exposing only tools tagged `scope: "worker_shareable"` to the underlying LLM call's tool list.
- **REQ-QUERY-013**: **While** a `permission_request` is pending, the `queryLoop` **shall** suspend iteration without consuming additional LLM tokens and **shall** resume only after a `PermissionDecision` is delivered via the engine's permission inbox channel.

### Unwanted / Ubiquitous Prohibition

> **라벨 주기 (감사 review-1 D4)**: REQ-015, REQ-016 은 원래 Ubiquitous prohibition (`shall not` 상시 금지). REQ 번호 안정성을 위해 본 섹션에 두되 라벨을 `[Ubiquitous]` 로 교정. REQ-014, REQ-017 은 진정한 Unwanted 패턴.

- **REQ-QUERY-014 [Unwanted]**: **If** the LLM streaming call panics or returns a non-retriable provider error (HTTP 4xx except 429), **then** the `queryLoop` **shall** (a) stop forwarding chunks, (b) invoke `StopFailureHooks` with the error, (c) yield a final `SDKMessage` of type `error` with the error details, and (d) return `Terminal{success: false, error: "<provider_error>"}`.
- **REQ-QUERY-015 [Ubiquitous]**: The `QueryEngine` **shall not** write to `State.messages` from goroutines other than the `queryLoop` goroutine owning the current iteration; all state transitions **shall** occur in the single loop goroutine to eliminate lock-based synchronization.
- **REQ-QUERY-016 [Ubiquitous]**: The `QueryEngine.SubmitMessage` **shall not** block the caller for longer than 10ms before returning the output channel, even if LLM connection setup is slow (connection dial happens inside the `queryLoop` goroutine).
- **REQ-QUERY-017 [Unwanted]**: **If** a `tool_use` block references a tool name not present in `QueryEngineConfig.Tools`, **then** the loop **shall** synthesize a `ToolResult{is_error: true, content: "tool_not_found: <name>"}` and **shall not** terminate the conversation.

### Optional

- **REQ-QUERY-018**: **Where** `QueryEngineConfig.PostSamplingHooks` is non-empty, each hook function **shall** receive the sampled `Message` after every LLM response and **may** mutate it before tool parsing (middleware chain, FIFO order).
- **REQ-QUERY-019**: **Where** `QueryEngineConfig.FallbackModels` is non-empty and the primary model returns a 5xx or 429 exceeding the per-call retry budget, the engine **shall** transparently retry the same turn against the next model in the chain before surfacing the error.
- **REQ-QUERY-020**: **Where** `QueryEngineConfig.TeammateIdentity != nil`, the engine **shall** inject `{agent_id, team_name}` into every outbound LLM system prompt as a structured header and into every Trajectory-bound `SDKMessage` as metadata.

---

## Acceptance Criteria (Given / When / Then)

### AC-QUERY-001 — 정상 1턴 (tool 없음) end-to-end
- **Given** `QueryEngine` configured with stub `LLMCall` streaming "ok" then ending.
- **When** `SubmitMessage("hi")` 호출 후 채널 drain.
- **Then** 순서대로 `user_ack` → `stream_request_start` → `StreamEvent{delta:"ok"}` → `Message{role:"assistant"}` → `Terminal{success:true}`, 채널 close, `State.turnCount == 1` (§6.3 경로 A: assistant terminal 완료 +1).

### AC-QUERY-002 — 1턴 with 1 tool call
- **Given** 스텁 LLM 첫 응답에 `tool_use{name:"echo", input:{"x":1}}`, 두 번째는 `stop`. `CanUseTool` Allow. `Executor.Run("echo")` → `{"x":1}`.
- **When** `SubmitMessage("call echo")` drain.
- **Then** `tool_use` → `permission_check{allow}` → `tool_result{content:{"x":1}}` → 두 번째 assistant Message → `Terminal{success:true}`, `State.turnCount == 2` (§6.3: 경로 B tool roundtrip +1, 이어진 경로 A assistant terminal +1). `permission_check{allow}` 는 `SDKMsgPermissionCheck` 타입.

### AC-QUERY-003 — Tool permission deny 처리
- **Given** 스텁 LLM `tool_use{name:"rm_rf"}`, `CanUseTool` Deny{reason:"destructive"}.
- **When** `SubmitMessage`.
- **Then** 채널 순서에 `permission_check{tool_use_id, behavior:"deny", reason:"destructive"}` SDKMessage yield 후, tool 미실행, 다음 API call messages 에 `ToolResult{is_error:true, content:"denied: destructive"}` 포함, 두 번째 assistant turn 후 `Terminal{success:true}`.

### AC-QUERY-004 — 2턴 연속 대화 (messages 배열 유지)
- **Given** 동일 engine 인스턴스.
- **When** `SubmitMessage("first")` drain 후 `SubmitMessage("second")`.
- **Then** 두 번째 호출의 첫 API payload messages[] 에 1턴차 user+assistant 포함, `State.turnCount` 누적, `task_budget.remaining` 누적 차감.

### AC-QUERY-005 — max_output_tokens 재시도 ≤ 3회
- **Given** 스텁 LLM 첫 3회 `max_output_tokens` 종료, 4회차 정상.
- **When** `SubmitMessage`.
- **Then** `maxOutputTokensRecoveryCount` 3 까지 증가 후 정상 응답, `Terminal{success:true}`. 4회 모두 실패 시나리오에서는 `Terminal{success:false, error:"max_output_tokens_exhausted"}`.

### AC-QUERY-006 — task_budget 소진
- **Given** `TaskBudget = {total:50}`, 스텁 LLM 1턴차에 60 units 소비 (예산 초과 유발).
- **When** `SubmitMessage` drain.
- **Then** 1턴 완료 시점에 `remaining == -10` (음수) 로 차감된 후, 2턴차 iteration 시작 시 REQ-QUERY-011 의 `remaining <= 0` gate 발동 → `Terminal{success:false, error:"budget_exceeded"}`.

### AC-QUERY-007 — max_turns 도달
- **Given** `MaxTurns=2`, 스텁 LLM 항상 `tool_use` 반환.
- **When** `SubmitMessage` drain.
- **Then** 2턴 실행 후 `Terminal{success:true, error:"max_turns"}` (REQ-QUERY-011: success=true).

### AC-QUERY-008 — Abort via context cancellation
- **Given** 스텁 LLM chunk 간 200ms, `ctx = context.WithTimeout(500ms)`.
- **When** `SubmitMessage` 500ms 경과.
- **Then** 채널 close, 마지막 `Terminal{success:false, error:"aborted"}`, 처리 시간 ≤ 1초.

### AC-QUERY-009 — Tool result budget replacement
- **Given** 스텁 executor 1MB JSON 반환, `TaskBudget.ToolResultCap=4KB`.
- **When** tool round-trip.
- **Then** 다음 LLM payload tool_result `content` 가 `{tool_use_id:..., truncated:true, bytes_original:1048576, bytes_kept:4096}` 치환, 원본 1MB 미전파.

### AC-QUERY-010 — Coordinator mode tool visibility 제한
- **Given** `CoordinatorMode=true`, tool `A{scope:"leader_only"}`, `B{scope:"worker_shareable"}`.
- **When** LLM call payload inspection.
- **Then** payload tools[] 에 `B` 만 포함, `A` 제외. LLM 이 `tool_use{name:"A"}` 반환 시 REQ-QUERY-017 `tool_not_found: A` 합성.

### AC-QUERY-011 — Compaction boundary yield
- **Given** `Compactor.ShouldCompact` turn 3 에서 true, `Compact` 가 messages 10개 → summary 1개.
- **When** `SubmitMessage` drain.
- **Then** turn 3 시작 전 `CompactBoundary{turn:3, messages_before:10, messages_after:1}` yield, 이후 iteration 은 치환 State 로 진행, `task_budget.remaining` 보존.

### AC-QUERY-012 — Fallback model chain
- **Given** primary HTTP 529, `FallbackModels=["model-B"]`.
- **When** `SubmitMessage`.
- **Then** 동일 turn primary 재시도 소진 → fallback `model-B` 투명 재시도 → 성공, 외부 `Terminal{success:true}`, 로그에 fallback 기록.

### AC-QUERY-013 — Ask permission suspend / resume
- **Given** 스텁 LLM 이 `tool_use{name:"deleteFile"}` 반환, `CanUseTool` 이 `Decision{Behavior:Ask, Reason:"destructive op"}` 반환, payload recorder 활성.
- **When** `SubmitMessage("del file")` drain 중 `permission_request` 수신 후 외부에서 `engine.ResolvePermission(toolUseID, Allow)` 호출.
- **Then** (a) resolve 전까지 LLM call 추가 시도 0건 (payload recorder 고정), (b) resolve 후 loop 재개 → tool 실행 → 두 번째 LLM call 수행, (c) 두 번째 payload messages[] 에 `tool_result` 포함, (d) 최종 `Terminal{success:true}`. Edge: Deny 해결 시 `ToolResult{is_error:true, content:"denied: destructive op"}` 합성.

### AC-QUERY-014 — SubmitMessage 10ms 마감
- **Given** 스텁 LLM 이 goroutine 내부 초기화에 100ms 지연. 엔진은 이미 생성 완료 상태.
- **When** N=1000 반복: `t0 := time.Now(); SubmitMessage(ctx, "hi"); t1 := time.Now()`.
- **Then** 모든 반복 `t1 - t0 ≤ 10ms`, p99 ≤ 10ms. 반환된 채널은 이후 stub 100ms 완료 후 정상 메시지 수신 → `Terminal{success:true}`.

### AC-QUERY-015 — PostSamplingHooks FIFO chain
- **Given** `PostSamplingHooks = [h1, h2]` 등록. h1 은 content 말미에 `" [h1]"`, h2 는 `" [h2]"` append. 스텁 LLM `Message{content:"ok"}` yield.
- **When** `SubmitMessage("hi")` drain.
- **Then** `State.Messages` 의 assistant content 가 `"ok [h1] [h2]"` (FIFO). 순서 바꾸면 결과 변함 (순서 민감성 확증).

### AC-QUERY-016 — TeammateIdentity 주입
- **Given** `TeammateIdentity = &{AgentID:"spec-ga-01", TeamName:"alpha"}`, payload recorder 활성.
- **When** `SubmitMessage("hi")` drain.
- **Then** (a) outbound LLM payload system 파트에 `{"agent_id":"spec-ga-01","team_name":"alpha"}` 구조화 헤더 포함, (b) 관찰된 모든 `SDKMessage` 의 meta 필드에 동일 identity 포함. Edge: `nil` identity 면 헤더/meta 부재.

---

## Files to Modify (신규 생성)

```
internal/
├── query/
│   ├── engine.go                 # QueryEngine + SubmitMessage
│   ├── engine_test.go
│   ├── config.go                 # QueryEngineConfig + QueryParams
│   └── loop/
│       ├── loop.go               # queryLoop goroutine + State machine
│       ├── state.go              # State, Continue, Terminal 타입
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

보조 파일(테스트 지원): `internal/query/testsupport/stubs.go` (Stub LLMCall / Executor / CanUseTool / Compactor — research.md §8.3).

---

## Exclusions (What NOT to Build)

- 본 SPEC 은 **compaction 알고리즘 본체를 구현하지 않는다**. `Compactor` 인터페이스 호출만. CONTEXT-001 이 구현.
- 본 SPEC 은 **실제 LLM HTTP 호출을 구현하지 않는다**. `LLMCall` 함수 타입만 정의. ADAPTER-001 이 구현.
- 본 SPEC 은 **Tool 실행 엔진을 구현하지 않는다**. `tools.Executor` 인터페이스 호출만. TOOLS-001 이 구현.
- 본 SPEC 은 **Sub-agent fork/worktree/mailbox 를 구현하지 않는다**. SUBAGENT-001.
- 본 SPEC 은 **Slash command 파서를 구현하지 않는다**. `processUserInput` 은 passthrough. COMMAND-001.
- 본 SPEC 은 **Hook 이벤트 dispatcher 를 구현하지 않는다**. `PostSamplingHooks`/`StopFailureHooks` 엔트리포인트만 제공. HOOK-001.
- 본 SPEC 은 **Credential rotation, rate limiting, prompt caching 을 구현하지 않는다**. CREDPOOL/RATELIMIT/PROMPT-CACHE-001.
- 본 SPEC 은 **Trajectory 수집/학습 파이프라인을 포함하지 않는다**. TRAJECTORY-001+.
- 본 SPEC 은 **UI 렌더링(Ink/React) 을 포함하지 않는다**. CLI-001.
- 본 SPEC 은 **tool 병렬 실행을 허용하지 않는다**. Phase 0 에서는 한 응답 내 tool_use 블록을 순차 실행만 (TOOLS-001 이 향후 확장).

---

**End of spec-compact.md**
