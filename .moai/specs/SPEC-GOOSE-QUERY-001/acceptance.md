---
spec_id: SPEC-GOOSE-QUERY-001
version: 0.1.0
status: planned
created_at: 2026-04-24
updated_at: 2026-04-25
author: manager-spec
methodology: TDD
labels: []
---

# SPEC-GOOSE-QUERY-001 — 수용 기준 (Acceptance Criteria)

> **본 문서의 역할**: `spec.md §5` 의 AC-QUERY-001 ~ 016 를 **테스트 실행 단위**로 확장한다. 각 AC 는 Given/When/Then, 매핑 REQ, 예상 Go 테스트 시그니처, 스텁 요구사항, edge case 를 포함한다.
> **경로 규약**: 테스트 파일은 `internal/query/...`, `internal/query/loop/...`, `internal/message/...`, `internal/permissions/...` 하위. integration 대상은 build tag `integration`.

---

## AC-QUERY-001 — 정상 1턴 (tool 없음) end-to-end

- **매핑 REQ**: REQ-QUERY-002 (streaming mandatory), REQ-QUERY-005 (SubmitMessage 즉시 반환), REQ-QUERY-011 (turnCount 증가 및 정상 종료)
- **Given**: `QueryEngine` 이 `StubLLMCall` 로 설정되고, stub 이 `StreamEvent{delta:"ok"}` 1개 + `message_stop` 1개로 종료하는 단일 assistant 응답을 반환.
- **When**: 테스트가 `SubmitMessage(ctx, "hi")` 를 호출하고 반환 채널을 `for msg := range out` 로 drain.
- **Then**: 관찰 순서는 `user_ack` → `stream_request_start` → `stream_event{delta:"ok"}` → `message{role:"assistant"}` → `terminal{success:true}`. 채널 close 됨. `engine.state.TurnCount == 1` (spec.md §6.3 turnCount 증분 모델 경로 A: tool 없는 assistant terminal 완료 시 1 증분).
- **테스트 파일 / 시그니처**:
  - `internal/query/engine_test.go` → `func TestQueryEngine_SubmitMessage_StreamsImmediately(t *testing.T)` (build tag `integration`)
- **스텁 요구사항**:
  - `StubLLMCall` (research.md §8.3): chunks = `[]StreamEvent{{Delta:"ok"}}`, terminal `stop`
  - `StubCanUseTool`: unused (tool 없음)
  - `StubExecutor`: unused
- **Edge case 보강**:
  - 빈 prompt (`SubmitMessage(ctx, "")`) — user message 가 여전히 append 되고 terminal 정상 도달해야 함. 서브테스트 `t.Run("empty_prompt", ...)`.
  - stub 이 delta 0개만 반환(토큰 0) — `message_stop` 직전 빈 assistant 메시지 생성 여부 검증.

---

## AC-QUERY-002 — 1턴 with 1 tool call (permission Allow)

- **매핑 REQ**: REQ-QUERY-006 (CanUseTool Allow 경로), REQ-QUERY-011 (turn 증가), REQ-QUERY-003 (continue site `after_tool_results`)
- **Given**: stub LLM 의 첫 응답이 `tool_use{name:"echo", input:{"x":1}}` 블록 포함, 두 번째 응답은 `stop`. `StubCanUseTool` 이 항상 `Allow`. `StubExecutor.Run("echo", {"x":1})` → `ToolResult{content:{"x":1}}`.
- **When**: `SubmitMessage(ctx, "call echo")` 호출 후 drain.
- **Then**: 순서 `tool_use` → `permission_check{allow}` → `tool_result{content:{"x":1}}` → 두 번째 `assistant message` → `terminal{success:true}`. `engine.state.TurnCount == 2` (spec.md §6.3 turnCount 증분 모델: 경로 B 1회 tool roundtrip 완료 후 `after_tool_results` 에서 +1, 이어진 경로 A assistant terminal 완료 후 +1, 합 2). `permission_check{allow}` 는 `SDKMsgPermissionCheck` 타입으로 spec.md §6.2 enum 에 정의되어 있으며 REQ-QUERY-006 Allow 분기에서 yield 된다.
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_ToolCallAllow_FullRoundtrip(t *testing.T)` (build tag `integration`)
- **스텁 요구사항**:
  - `StubLLMCall`: 시퀀스 2개 (1st: tool_use, 2nd: stop)
  - `StubExecutor` with `echo` deterministic handler
  - `StubCanUseTool` 항상 Allow
- **Edge case 보강**:
  - `tool_use` input 이 빈 객체(`{}`) 일 때 executor 도 empty result 처리.
  - 동일 응답 내 tool_use 블록 2개 (spec.md Exclusions 10번: 순차 실행) — `TestQueryLoop_MultipleToolBlocks_Sequential` 서브테스트.

---

## AC-QUERY-003 — Tool permission deny 처리

- **매핑 REQ**: REQ-QUERY-006 (Deny 경로), REQ-QUERY-014 경계 (terminal 유발 금지)
- **Given**: stub LLM 이 `tool_use{name:"rm_rf"}` 를 반환, `StubCanUseTool` 이 `Decision{Behavior:Deny, Reason:"destructive"}` 반환, 이어지는 LLM 응답은 `stop`.
- **When**: `SubmitMessage(ctx, "please delete everything")`.
- **Then**: 채널 순서에 `permission_check{tool_use_id, behavior:"deny", reason:"destructive"}` SDKMessage 가 yield 된 후, `StubExecutor.Run` 은 호출되지 않는다. 다음 LLM call payload 의 messages[] 에 `ToolResult{is_error:true, content:"denied: destructive"}` 포함. 최종 `terminal{success:true}` (대화 계속 진행).
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_PermissionDeny_SynthesizesErrorResult(t *testing.T)` (integration)
- **스텁 요구사항**:
  - `StubExecutor` 가 `Run` 호출 시 즉시 `t.Fatal` 하도록 감시자 역할 (fail-on-call guard)
  - `StubCanUseTool` Deny 반환
- **Edge case 보강**:
  - Deny reason 이 빈 문자열인 경우 `content:"denied: "` 가 아닌 `content:"denied"` fallback 처리.
  - 동일 turn 에서 tool_use 블록 2개 중 하나만 Deny — allowed 블록만 실행되고 denied 는 error 결과 합성.

---

## AC-QUERY-004 — 2턴 연속 대화 (messages 배열 유지)

- **매핑 REQ**: REQ-QUERY-004 (multi SubmitMessage 누적), REQ-QUERY-001 (engine 1:1 대화)
- **Given**: 동일 engine 인스턴스. stub LLM 이 각 호출에 단순 stop 으로 응답.
- **When**: `SubmitMessage(ctx, "first")` drain 후 `SubmitMessage(ctx, "second")` 호출.
- **Then**: 두 번째 호출이 stub LLM 에 전달하는 첫 API payload 의 `messages[]` 에 1턴차 user + assistant message 가 포함. `engine.state.TurnCount` 누적. `TaskBudget.Remaining` 누적 차감.
- **테스트 파일 / 시그니처**:
  - `internal/query/engine_test.go` → `func TestQueryEngine_TwoSubmitMessages_ShareMessages(t *testing.T)` (integration)
- **스텁 요구사항**:
  - `StubLLMCall` 이 호출된 payload 를 `[]LLMCallRecord` 로 기록하는 recorder 모드 제공
  - testify `require` 로 2번째 call 의 messages 길이 ≥ 3 (user + assistant + user) 검증
- **Edge case 보강**:
  - 첫 SubmitMessage 가 `ctx.Cancel` 로 조기 종료된 경우 두 번째 SubmitMessage 의 messages 에 부분 assistant 가 포함되는지 명시 — 본 SPEC 은 "부분 assistant 는 포함하지 않음" 고정.
  - 두 번째 SubmitMessage 가 첫 번째 drain 완료 전에 호출될 경우 `sync.Mutex` 로 직렬화(별도 `TestQueryEngine_Concurrent_SubmitMessage_IsSerialized`).

---

## AC-QUERY-005 — max_output_tokens 재시도 ≤ 3회

- **매핑 REQ**: REQ-QUERY-008 (max_output_tokens 재시도), REQ-QUERY-003 (continue site `after_retry`)
- **Given**: `StubLLMCall` 이 첫 3회 호출은 `TerminalReason:"max_output_tokens"` 로 종료, 4회차는 `"ok"` 로 정상 응답.
- **When**: `SubmitMessage(ctx, "retry me")` drain.
- **Then**: `State.MaxOutputTokensRecoveryCount` 가 3 까지 증가 후 정상 응답 수신. 최종 `terminal{success:true}`. 동일 stub 이 4회 모두 `max_output_tokens` 반환 시나리오 서브테스트에서는 `terminal{success:false, error:"max_output_tokens_exhausted"}`.
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_MaxOutputTokens_Retries3ThenFails(t *testing.T)` (integration, 2개 서브테스트: `recover_on_4th`, `exhaust_after_3`)
- **스텁 요구사항**:
  - `StubLLMCall` 에 `terminalQueue []string` 주입 (N회차 terminal reason 지정)
  - counter 를 engine 내부 state 로부터 검증할 수 있는 test helper (`engine.State() State` — test-only getter, build tag `testing`)
- **Edge case 보강**:
  - 3회 재시도 중간에 `ctx.Cancel` → abort 우선 (REQ-QUERY-010) 경로가 재시도 경로를 오버라이드하는지 확인.
  - compaction 직후 재시도 counter reset 검증은 별도 `TestQueryLoop_RetryCounter_ResetsAfterCompact`.

---

## AC-QUERY-006 — task_budget 소진 → budget_exceeded terminal

- **매핑 REQ**: REQ-QUERY-011 (budget terminal)
- **Given**: `QueryEngineConfig.TaskBudget = TaskBudget{Total:50, Remaining:50}`. stub LLM 이 1턴차에 `usage.total = 60` 소비 (총 예산 초과 소비 유발).
- **When**: `SubmitMessage(ctx, "burn budget")` drain.
- **Then**: 1턴 완료 시점에 `Remaining == -10` (음수) 으로 차감된 후, 2턴차 iteration 시작 시 REQ-QUERY-011 의 `remaining <= 0` gate 가 발동하여 `terminal{success:false, error:"budget_exceeded"}`.
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_BudgetExhausted(t *testing.T)` (integration)
- **스텁 요구사항**:
  - `StubLLMCall` 이 응답마다 `usage{input_tokens, output_tokens}` 를 제어 가능하게 노출
  - `StubExecutor` 는 tool_use 없을 수도 있으므로 optional
- **Edge case 보강**:
  - `Total:0` (예산 0) 으로 구성 시 첫 turn 에서 즉시 terminal. 서브테스트.
  - `Remaining` 이 음수로 과소비된 경우(예: stub 이 Total 초과 보고) 동일하게 `budget_exceeded`.

---

## AC-QUERY-007 — max_turns 도달 → max_turns terminal

- **매핑 REQ**: REQ-QUERY-011 (max_turns 는 success:true)
- **Given**: `QueryEngineConfig.MaxTurns = 2`. stub LLM 이 매 응답마다 `tool_use{name:"loop"}` 반환 → 무한 tool loop 가능성. `StubExecutor` 가 echo 형식 결과 반환.
- **When**: `SubmitMessage(ctx, "infinite loop")` drain.
- **Then**: 2턴 실행 후 `terminal{success:true, error:"max_turns"}` (REQ-QUERY-011 규정상 success=true, bounded 도달 = 정상 한계).
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_MaxTurnsReached(t *testing.T)` (integration)
- **스텁 요구사항**:
  - `StubLLMCall` 이 항상 tool_use 반환
  - `StubCanUseTool` Allow
  - `StubExecutor` 가 deterministic result 반환 (tool_use_id 와 짝 맞춤)
- **Edge case 보강**:
  - `MaxTurns = 0` → SubmitMessage 즉시 `terminal{success:true, error:"max_turns"}` 로 반환 (0-turn bound).
  - `MaxTurns = 1` + first response 가 pure text → turn 종료 후 자연스러운 success.

---

## AC-QUERY-008 — Abort via context cancellation (500ms 마감)

- **매핑 REQ**: REQ-QUERY-010 (ctx cancel 500ms abort), REQ-QUERY-015 (state ownership 유지)
- **Given**: stub LLM 이 chunk 간 200ms 대기. 호출자가 `ctx, cancel := context.WithTimeout(parent, 500*time.Millisecond)` 구성.
- **When**: `SubmitMessage(ctx, "slow")` 호출 후 500ms 경과.
- **Then**: 출력 채널 close. 마지막 메시지 `terminal{success:false, error:"aborted"}`. 전체 처리 시간 ≤ 1000ms (500ms 마감 + 여유).
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_AbortsOnContextCancel(t *testing.T)` (integration)
- **스텁 요구사항**:
  - `StubLLMCall` 에 `perChunkDelay time.Duration` 옵션
  - 테스트 유틸 `drainWithDeadline(t, ch, 1*time.Second)` — deadline 초과 시 fail
- **Edge case 보강**:
  - **cancel 직전 in-flight tool call 처리**: stub executor 가 300ms 동안 blocking 하는 tool 을 실행 중에 ctx cancel → tool 결과는 버리고 abort terminal 먼저 yield. 별도 서브테스트 `t.Run("cancel_during_tool_exec", ...)`.
  - Ask permission 이 pending 중 cancel → permission inbox 해제 + abort terminal.
  - 이미 종료된 ctx 로 SubmitMessage 호출 — 즉시 abort terminal (loop spawn 거의 no-op).

---

## AC-QUERY-009 — Tool result budget replacement

- **매핑 REQ**: REQ-QUERY-007 (tool result cap), REQ-QUERY-011 (budget 누적은 별도)
- **Given**: `StubExecutor` 가 1 MiB JSON (`bytes.Repeat([]byte("x"), 1<<20)` 기반 valid JSON) 을 반환. `QueryEngineConfig.TaskBudget.ToolResultCap = 4096` (4 KiB).
- **When**: tool round-trip 1회.
- **Then**: 다음 LLM payload 의 messages[] tool_result content 가 `{tool_use_id, truncated:true, bytes_original:1048576, bytes_kept:4096}` 형태로 치환. 원본 1 MiB 가 LLM 에 전달되지 않음(payload recorder 검증). 로그에 replacement 기록.
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_ToolResultBudgetReplacement(t *testing.T)` (integration)
- **스텁 요구사항**:
  - `StubExecutor` 가 대용량 content 를 deterministic 생성
  - `StubLLMCall` payload recorder 가 content size 측정
  - `zaptest.NewLogger(t)` 로 로그 확인
- **Edge case 보강**:
  - `ToolResultCap == 0` (무제한) → 치환 미발생, 원본 그대로 전달.
  - content 가 정확히 cap 과 같을 때 경계(`==`) 처리 — 치환 발생하지 않아야 함(strict `>`).
  - content 가 multi-byte UTF-8 으로 byte cap 중간을 자르는 경우 치환 summary 의 `bytes_kept` 는 byte 기준 (spec.md 원문 따름).

---

## AC-QUERY-010 — Coordinator mode tool visibility 제한

- **매핑 REQ**: REQ-QUERY-012 (CoordinatorMode tool filter), REQ-QUERY-017 (tool_not_found 합성)
- **Given**: `QueryEngineConfig.CoordinatorMode = true`. Tools: `A{scope:"leader_only"}`, `B{scope:"worker_shareable"}`. stub LLM 이 payload 검사 가능.
- **When**: `SubmitMessage` 첫 호출의 LLM payload inspection.
- **Then**: payload `tools[]` 에 `B` 만 포함, `A` 제외. stub LLM 이 응답으로 `tool_use{name:"A"}` 반환 시나리오 → REQ-QUERY-017 에 따라 `ToolResult{is_error:true, content:"tool_not_found: A"}` 합성 + 대화 계속.
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_CoordinatorModeToolFilter(t *testing.T)` (integration, 2개 서브테스트: `filter_at_payload`, `llm_calls_filtered_tool`)
- **스텁 요구사항**:
  - `tools.Registry` stub (이 SPEC 내 interface 형태로만; 실 구현은 TOOLS-001) — 각 tool 의 `scope` manifest 필드 지원
  - `StubLLMCall` payload recorder
- **Edge case 보강**:
  - `CoordinatorMode=false` (기본) 시 두 tool 모두 payload 에 포함 — 네거티브 케이스.
  - tool 스코프 필드가 manifest 에 없는 경우 기본값 `worker_shareable` 로 해석 (정책 고정).

---

## AC-QUERY-011 — Compaction boundary yield

- **매핑 REQ**: REQ-QUERY-009 (compaction boundary 스트림에 yield), REQ-QUERY-003 (continue site `after_compact`), REQ-QUERY-008 (counter reset 은 R5 보조 검증)
- **Given**: `StubCompactor.ShouldCompact` 가 turn 3 시작 시점에서 `true`. `StubCompactor.Compact` 가 기존 messages 10개를 summary 1개로 치환.
- **When**: `SubmitMessage` drain (충분한 turn 을 유발하기 위해 stub LLM 이 turn 1-2 에서 tool_use 반환 → turn 3 시작 시 ShouldCompact true).
- **Then**: turn 3 시작 전 `sdk_message{type:"compact_boundary", payload:{turn:3, messages_before:10, messages_after:1}}` yield. 이후 iteration 은 치환된 State 로 진행. `TaskBudget.Remaining` 누적값 보존(compaction 전후 차감 0 추가).
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_CompactBoundaryYieldedOnCompact(t *testing.T)` (integration)
- **스텁 요구사항**:
  - `StubCompactor` (research.md §8.3): `ShouldCompact(state State) bool`, `Compact(state State) (State, CompactBoundary, error)` 제어 가능
  - `StubLLMCall` 이 pre-compact / post-compact payload 를 모두 기록
- **Edge case 보강**:
  - Compactor 가 에러 반환 시 compaction skip + 로그 경고, 정상 iteration 지속(fail-soft 정책).
  - `ShouldCompact` 가 연속 2 iteration true 반환 시 각각 별도 boundary yield.
  - counter reset 검증 보조: `TestQueryLoop_RetryCounter_ResetsAfterCompact` 서브테스트가 R5 를 잠금.

---

## AC-QUERY-012 — Fallback model chain

- **매핑 REQ**: REQ-QUERY-019 (fallback model)
- **Given**: `QueryEngineConfig.FallbackModels = ["model-B"]`. `StubLLMCall` 이 primary 호출에서 HTTP 529(Overloaded) 를 per-call retry 예산 소진까지 반환. fallback `model-B` 호출에서는 정상 응답.
- **When**: `SubmitMessage(ctx, "please")`.
- **Then**: 동일 turn 내 primary 실패 → fallback 재호출 → 성공 응답 수신 → `terminal{success:true}`. `zaptest` 로그에 "fallback used" 구조화 필드 존재.
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_FallbackModelChain(t *testing.T)` (integration)
- **스텁 요구사항**:
  - `StubLLMCall` 이 model 이름별로 다른 시퀀스 반환 (`map[string][]StubLLMResponse`)
  - `zaptest.NewLogger(t)` + `observer.New()` 로 로그 엔트리 검증
- **Edge case 보강**:
  - FallbackModels 가 비어있을 때 primary 529 → 즉시 terminal `error:"provider_overloaded"` 또는 유사. 네거티브 케이스.
  - FallbackModels 가 2개 이상일 때 순차 시도, 모두 실패 시 terminal 에 마지막 에러 surface.
  - primary 성공 시 fallback 호출 0회 (로그 absent).

---

## AC-QUERY-013 — Ask permission suspend / resume

- **매핑 REQ**: REQ-QUERY-013 (permission 대기 중 iteration suspend), REQ-QUERY-006 (Ask 분기)
- **Given**: `QueryEngine` 이 `StubLLMCall` 로 설정되고, stub 의 첫 응답이 `tool_use{name:"deleteFile", input:{"path":"/tmp/x"}}` 블록을 포함. `StubCanUseTool` 이 `Decision{Behavior:Ask, Reason:"destructive op"}` 반환. `StubLLMCall` 에 payload recorder 활성 (호출 건수 + payload 기록). 테스트는 별도 goroutine 에서 `engine.ResolvePermission(toolUseID, Allow)` 를 호출할 수 있도록 wiring.
- **When**: 메인 goroutine 이 `SubmitMessage(ctx, "del file")` 을 호출하여 채널 drain 시작. `permission_request{tool_use_id, tool_name:"deleteFile", input:{"path":"/tmp/x"}}` `SDKMessage` 수신 후, 별도 goroutine 에서 50ms 후 `engine.ResolvePermission(toolUseID, Allow)` 호출. 이후 계속 drain.
- **Then**:
  - (a) resolve 호출 전까지 payload recorder 의 LLM call 건수는 1건으로 고정 (loop 가 추가 LLM 호출을 시도하지 않음 확증).
  - (b) `StubExecutor.Run("deleteFile", ...)` 는 resolve 후에만 호출됨 (resolve 전 호출 시 `t.Fatal`).
  - (c) resolve 후 loop 재개 → tool 실행 → `tool_result` 합성 → 두 번째 LLM call 수행.
  - (d) 두 번째 LLM call payload 의 messages[] 마지막 user 블록에 `tool_result{tool_use_id, content:...}` 포함.
  - (e) 최종 `terminal{success:true}`.
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_AskPermission_SuspendResume(t *testing.T)` (build tag `integration`)
- **스텁 요구사항**:
  - `StubCanUseTool` Ask 분기 (reason 포함)
  - `StubLLMCall` payload recorder + call counter
  - `StubExecutor` fail-on-early-call guard (resolve 전 `Run` 호출 시 `t.Fatal`)
- **Edge case 보강**:
  - `ResolvePermission(toolUseID, Deny)` 로 해결 시 `ToolResult{is_error:true, content:"denied: destructive op"}` 합성 → 대화 계속 진행 → 최종 success. 서브테스트 `t.Run("resolve_deny", ...)`.
  - resolve 이전에 `ctx` cancel → abort terminal 우선, tool 실행 0회. 서브테스트 `t.Run("cancel_while_pending", ...)`.
  - 동일 turn 에 Ask permission 2건 대기 → inbox 에 순차 delivery (FIFO). 서브테스트 `t.Run("multiple_asks_fifo", ...)`.

---

## AC-QUERY-014 — SubmitMessage 10ms 마감

- **매핑 REQ**: REQ-QUERY-016 (SubmitMessage 10ms 상시 불변식), REQ-QUERY-002 (streaming)
- **Given**: `StubLLMCall` 이 goroutine 내부에서 `time.Sleep(100*time.Millisecond)` 로 connection dial 을 시뮬레이션. `QueryEngine.New(cfg)` 으로 엔진 생성 완료된 상태 (초기화 비용 배제).
- **When**: 테스트가 아래 루프를 N=1000 회 반복:
  ```go
  t0 := time.Now()
  ch, err := engine.SubmitMessage(ctx, "hi")
  t1 := time.Now()
  samples = append(samples, t1.Sub(t0))
  // drain ch in background; let it settle before next iteration
  ```
- **Then**:
  - 모든 반복에서 `t1 - t0 ≤ 10ms` (strict ceiling).
  - p99(`samples`) ≤ 10ms, p50 ≤ 1ms 보장.
  - stub 의 100ms 지연이 끝나면 채널이 정상적으로 메시지를 수신하여 `terminal{success:true}` 로 종료 (SubmitMessage 반환과 실제 LLM 호출이 goroutine 으로 분리되어 있음을 확증).
- **테스트 파일 / 시그니처**:
  - `internal/query/engine_test.go` → `func TestQueryEngine_SubmitMessage_Returns10ms(t *testing.T)` (build tag `integration`)
- **스텁 요구사항**:
  - `StubLLMCall` 에 `initialDelay time.Duration` 옵션 (goroutine 내 sleep)
  - 성능 측정 헬퍼: `percentile(samples, 0.99)`
- **Edge case 보강**:
  - `StubLLMCall` 이 즉시 에러 반환하는 경우에도 SubmitMessage 자체는 10ms 이내 반환 (에러는 terminal 메시지로 전달). 서브테스트 `t.Run("fast_error", ...)`.
  - N=1000 중 1% 이내의 OS scheduling jitter 는 허용 가능하지만 p99 ≤ 10ms 엄수. 실패 시 CI 재시도 금지 (성능 회귀 명시적 신호).

---

## AC-QUERY-015 — PostSamplingHooks FIFO chain

- **매핑 REQ**: REQ-QUERY-018 (Optional hook middleware, FIFO)
- **Given**: `QueryEngineConfig.PostSamplingHooks = []MessageHook{h1, h2}` 로 등록:
  - `h1`: 입력 `Message.content` 말미에 `" [h1]"` 를 append 하여 반환
  - `h2`: 입력 `Message.content` 말미에 `" [h2]"` 를 append 하여 반환
  스텁 LLM 이 `Message{role:"assistant", content:"ok"}` yield.
- **When**: `SubmitMessage(ctx, "hi")` 호출 후 drain. 다음 LLM call (turn 2) 을 유도하기 위해 첫 응답 뒤 tool_use 없이 stub 이 두 번째 user 입력 시뮬레이션 또는 `engine.state.Messages` 직접 inspection (test-only getter 사용).
- **Then**:
  - 내부 `State.Messages` 에 저장된 assistant message 의 content 가 `"ok [h1] [h2]"` (FIFO 순서 적용).
  - `h1`, `h2` 를 바꿔 `[]MessageHook{h2, h1}` 로 등록하면 결과가 `"ok [h2] [h1]"` 로 변함 (순서 민감성 검증).
- **테스트 파일 / 시그니처**:
  - `internal/query/loop/loop_test.go` → `func TestQueryLoop_PostSamplingHooks_FifoChain(t *testing.T)` (build tag `integration`, 2개 서브테스트: `fifo_h1_then_h2`, `fifo_h2_then_h1`)
- **스텁 요구사항**:
  - test-only `engine.State() State` getter (build tag `testing`) 또는 `StubLLMCall` payload recorder 로 후속 call 의 messages[] 확인
  - 두 개의 순수 함수형 hook (`h1`, `h2`)
- **Edge case 보강**:
  - `PostSamplingHooks` 가 nil 또는 빈 slice 일 때 content 변형 0 (네거티브).
  - hook 이 원본 Message 를 mutate 하지 않고 새 Message 를 반환하는 경우에도 체인 결과 동일.
  - hook 중 하나가 에러 반환 시 loop 가 `StopFailureHooks` 경유 terminal 로 전이할지 여부는 본 AC 범위 밖 (HOOK-001 에서 정의).

---

## AC-QUERY-016 — TeammateIdentity 주입 (system header + SDKMessage metadata)

- **매핑 REQ**: REQ-QUERY-020 (TeammateIdentity injection, 2경로)
- **Given**: `QueryEngineConfig.TeammateIdentity = &TeammateIdentity{AgentID:"spec-ga-01", TeamName:"alpha"}` 구성. `StubLLMCall` 에 payload recorder 활성. 스텁이 assistant text 1회 + stop 으로 응답.
- **When**: `SubmitMessage(ctx, "hi")` 호출 후 채널 drain.
- **Then**:
  - (a) payload recorder 가 기록한 첫 outbound LLM call 의 system 파트에 구조화 헤더 `{"agent_id":"spec-ga-01","team_name":"alpha"}` 포함 (JSON 파싱 또는 규약된 문자열 포맷 검증).
  - (b) drain 된 모든 `SDKMessage` 의 `meta` 필드(또는 동등 metadata 경로) 에 `{agent_id:"spec-ga-01", team_name:"alpha"}` 포함 (user_ack, stream_request_start, stream_event, message, terminal 전부).
- **테스트 파일 / 시그니처**:
  - `internal/query/engine_test.go` → `func TestQueryEngine_TeammateIdentity_InjectedEverywhere(t *testing.T)` (build tag `integration`)
- **스텁 요구사항**:
  - `StubLLMCall` payload recorder
  - `SDKMessage` 의 metadata 접근자 (`Meta() map[string]any` 또는 payload struct 내 명시 필드)
- **Edge case 보강**:
  - `TeammateIdentity == nil` (기본) → system header 주입 0, SDKMessage metadata 에 teammate 필드 부재. 네거티브 서브테스트 `t.Run("nil_identity", ...)`.
  - `AgentID` 만 세팅되고 `TeamName` 이 빈 문자열 → 빈 문자열 그대로 주입 (silent default 없음).
  - 동일 engine 에서 `SubmitMessage` 2회 연속 호출 시 두 번째 호출의 payload 와 SDKMessage 에도 동일 identity 주입 (누적 일관성).

---

## 성능 / 품질 게이트

AC 16개를 관통하는 비기능 기준. 각 항목은 specific 테스트 혹은 CI 단계로 검증.

| 항목 | 근거 REQ | 검증 방법 |
|-----|--------|---------|
| **SubmitMessage 10ms 마감** | REQ-QUERY-016 | **AC-QUERY-014** `TestQueryEngine_SubmitMessage_Returns10ms` (N=1000, p99 ≤ 10ms) — stub LLM 초기화 100ms sleep 상황에서도 `time.Since(start) < 10ms` |
| **Abort 500ms 마감** | REQ-QUERY-010 | `TestQueryLoop_AbortsOnContextCancel` — `ctx` cancel 후 terminal yield 까지 `< 500ms` 측정 |
| **Race detector 무경보** | REQ-QUERY-015 | `go test -race -count=5 ./internal/query/... ./internal/message/... ./internal/permissions/...` (CI `test-race` job, plan.md §8) |
| **Coverage ≥ 85% 가중 평균** | spec.md §6.7 Tested | `go test -coverprofile=... ./internal/...` → 가중 평균 ≥ 85%, `internal/query` ≥ 90%, `internal/query/loop` ≥ 92%, `internal/message` ≥ 85%, `internal/permissions` ≥ 90% |
| **Lint 무경고** | spec.md §6.7 Unified | `golangci-lint run` (errcheck, govet, staticcheck, ineffassign, gocyclo) exit 0 |
| **채널 close 단일 소유자** | REQ-QUERY-002 / 010 | grep 으로 `close(` 호출이 `internal/query/loop/loop.go` 외부에 없음을 CI 에서 확인 |
| **Integration 16개 GREEN** | AC-QUERY-001~016 | `go test -tags=integration ./internal/query/...` exit 0 (AC-013 Ask suspend/resume, AC-014 10ms 마감, AC-015 PostSamplingHooks FIFO, AC-016 TeammateIdentity 포함) |
| **MX 태그 `@MX:TODO` 잔존 0** | plan.md §6 | sync 단계 스캔 (`moai hook mx-scan`) exit 0 |

---

## Definition of Done (수용 기준 관점)

1. 위 AC-QUERY-001 ~ 016 각각에 명시된 Go 테스트 함수가 존재하고 `go test -tags=integration` 로 GREEN (감사 review-1 D1/D2/D3 으로 AC-013~016 신설).
2. 각 AC 의 edge case 서브테스트가 최소 1개 GREEN.
3. "성능 / 품질 게이트" 표의 8개 항목이 모두 PASS.
4. plan.md §11 의 Definition of Done 1 ~ 6 과 교차 검증되어 불일치 없음.

---

**End of acceptance.md**
