---
spec_id: SPEC-GOOSE-QUERY-001
version: 0.1.0
status: Planned
created: 2026-04-24
updated: 2026-04-25
author: manager-spec
methodology: TDD (RED → GREEN → REFACTOR, per `.moai/config/sections/quality.yaml`)
---

# SPEC-GOOSE-QUERY-001 — 실행 계획 (Implementation Plan)

> **본 문서의 역할**: `spec.md`의 "무엇(What)을 왜(Why)"를 전제로, **어떻게(How)** 만들지를 파일·순서·검증 관점에서 고정한다. `spec.md §6`의 설계 결정은 재설명하지 않고 참조한다.

---

## 1. 개요

- **목표**: `QueryEngine` + `queryLoop` + `message` + `permissions` 4개 신규 패키지를 TDD(RED → GREEN → REFACTOR) 로 zero-to-one 작성.
- **선행 상태**: SPEC-GOOSE-CORE-001 완료(zap logger, context root, `goosed` 데몬 기동). `internal/query`, `internal/query/loop`, `internal/message`, `internal/permissions` 전부 부재.
- **성공 정의**: AC-QUERY-001 ~ 016(16개, 감사 review-1 D1/D2/D3 으로 AC-013~016 신설) 전부 GREEN + `go test -race` 무경보 + `golangci-lint` 무경고 + 가중 평균 coverage ≥ 85% (`internal/query` 90%+, `internal/query/loop` 92%+, research.md §8.5 기준).
- **비범위 재확인**: Compactor 본체, LLM HTTP, Tool 실행, subagent fork, slash command, hook dispatcher, credential pool — spec.md §3.2 및 Exclusions 섹션 그대로 승계.

---

## 2. 구현 단계별 작업 분해 (Task Breakdown)

spec.md §6.6 (RED #1 ~ #12) 를 파일 단위로 분해하고 의존성을 명시한다. TDD 사이클은 task 내부에서 RED → GREEN → REFACTOR 순으로 진행.

### 단계 S0 — 스켈레톤 & 스텁 기반 구축 (의존성: CORE-001)

| Task ID | 역할 | 신규 파일 | 의존 task | 내용 |
|---------|-----|----------|----------|------|
| T0.1 | 타입 골격 | `internal/message/message.go`, `internal/message/sdk_message.go`, `internal/message/stream_event.go` | — | `Message`, `ContentBlock`, `ToolResult`, `SDKMessage{Type,Payload}`, `StreamEvent` 타입 선언만 (메서드 미구현). Type enum 10개 (spec.md §6.2, `SDKMsgPermissionCheck` 포함). |
| T0.2 | 권한 타입 | `internal/permissions/behavior.go`, `internal/permissions/context.go`, `internal/permissions/can_use_tool.go` | — | `Behavior` enum (Allow/Deny/Ask), `Decision`, `ToolPermissionContext`, `CanUseTool` 인터페이스. |
| T0.3 | 설정 스켈레톤 | `internal/query/config.go` | T0.1, T0.2 | `QueryEngineConfig`, `QueryParams`, `TaskBudget`, `MessageHook`, `FailureHook`, `LLMCallFunc`, `Compactor` 인터페이스 선언 (spec.md §6.2 그대로). |
| T0.4 | 테스트 스텁 | `internal/query/testsupport/stubs.go` (build tag `!integration` 기본 노출, testing 전용) | T0.1~T0.3 | `StubLLMCall`, `StubExecutor`, `StubCanUseTool`, `StubCompactor` — research.md §8.3 규격. |

**S0 완료 기준**: `go build ./internal/...` 통과, 어떤 테스트도 아직 GREEN 아님.

### 단계 S1 — Message layer 단독 검증 (RED #7 ~ normalize 계열)

| Task ID | Test (RED) | 구현 (GREEN) | 의존 task | 매핑 REQ |
|---------|-----------|-------------|----------|---------|
| T1.1 | `TestMessage_Normalize_MergesConsecutiveUser`, `TestMessage_Normalize_StripsSignatureFromAssistant` (research.md §8.1) | `internal/message/normalize.go` | T0.1 | REQ-QUERY-003 (State 변환 진입점) |
| T1.2 | `TestSDKMessage_TypeSwitchExhaustive` | `SDKMessage` payload 구조체 10종 + exhaustive type-switch helper | T0.1 | REQ-QUERY-002, REQ-QUERY-007 매핑 |
| T1.3 | `TestStreamEvent_DeltaOrdering` | `internal/message/stream_event.go` 순서 보존 | T0.1 | REQ-QUERY-002 |
| T1.4 | `TestToolUseSummary_Formatter` | `internal/message/summary.go` (ToolUseSummaryMessage 포맷 spec.md §6.2) | T0.1 | REQ-QUERY-007 |

**S1 완료 기준**: `internal/message` 단위 테스트 GREEN, coverage ≥ 85%.

### 단계 S2 — Permissions layer 단독 검증

| Task ID | Test (RED) | 구현 (GREEN) | 의존 task | 매핑 REQ |
|---------|-----------|-------------|----------|---------|
| T2.1 | `TestCanUseTool_AllowBypassesGate` | `StubCanUseTool` Allow 경로 + contract test | T0.2, T0.4 | REQ-QUERY-006 |
| T2.2 | `TestCanUseTool_DenyProducesErrorResult` | Deny → `ToolResult{is_error:true}` 합성 helper | T0.2 | REQ-QUERY-006, REQ-QUERY-014 경계 |
| T2.3 | `TestCanUseTool_AskSuspendsLoop` (단독 mock) | inbox 채널 기반 suspend helper | T0.2 | REQ-QUERY-013 |

**S2 완료 기준**: `internal/permissions` coverage ≥ 90%.

### 단계 S3 — QueryEngine 라이프사이클 (RED #1, REQ-QUERY-001/004/016)

| Task ID | Test (RED) | 구현 (GREEN) | 의존 task | 매핑 REQ / AC |
|---------|-----------|-------------|----------|--------------|
| T3.1 | `TestQueryEngine_New_ValidConfig_Succeeds`, `TestQueryEngine_New_MissingRequiredField_Fails` | `internal/query/engine.go` `New(cfg)` + 유효성 검증 (LLMCall / Tools / CanUseTool / Logger 필수) | T0.3 | REQ-QUERY-001 |
| T3.2 | `TestQueryEngine_SubmitMessage_ReturnsReceiveOnlyChannel` (타입 레벨) | `SubmitMessage` 시그니처 + unbuffered `chan SDKMessage` 반환 | T3.1 | REQ-QUERY-002, REQ-QUERY-016 |
| T3.3 | **RED #14** = `TestQueryEngine_SubmitMessage_Returns10ms` (AC-QUERY-014, N=1000 p99) + `TestQueryEngine_SubmitMessage_Returns_Within_10ms` (unit, stub: `LLMCall` 이 100ms sleep 후 응답) | goroutine spawn, 연결 dial 을 goroutine 내부로 이동 | T3.2 | REQ-QUERY-016, AC-QUERY-014, R7 검증 |
| T3.4 | `TestQueryEngine_Concurrent_SubmitMessage_IsSerialized` | `sync.Mutex` on `SubmitMessage` (spec.md §6.2 `mu sync.Mutex` 필드) | T3.3 | REQ-QUERY-001 |
| T3.5 | `TestQueryEngine_SubmitMessage_StreamsImmediately` (= **RED #1**, AC-QUERY-001) | queryLoop 최소 구현(assistant text 1회 → Terminal success) | T3.3, T0.4 | AC-QUERY-001 |

**S3 완료 기준**: AC-QUERY-001 GREEN. 이 지점에서 engine + message + 최소 loop 가 처음으로 end-to-end로 스트리밍.

### 단계 S4 — Continue site 3종 (REQ-QUERY-003 / §6.3)

spec.md §6.3 의 3개 continue site를 **별도 파일 `internal/query/loop/continue_site.go`** 로 분리. TDD 로 경로별 검증.

| Task ID | Test (RED) | 구현 (GREEN) | 의존 task | 매핑 REQ / AC |
|---------|-----------|-------------|----------|--------------|
| T4.1 | `TestState_ContinueSite_AfterToolResults_IncrementsTurnCount`, **RED #2** = `TestQueryLoop_ToolCallAllow_FullRoundtrip` (AC-QUERY-002) | `internal/query/loop/loop.go` 기본 iteration + `continue_site.go` `after_tool_results` | T3.5, T2.1 | REQ-QUERY-006, REQ-QUERY-011 |
| T4.2 | **RED #3** = `TestQueryLoop_PermissionDeny_SynthesizesErrorResult` (AC-QUERY-003) | permissions `Deny` 분기 합성 | T4.1, T2.2 | REQ-QUERY-006, AC-QUERY-003 |
| T4.3 | **RED #5** = `TestQueryLoop_MaxOutputTokens_Retries3ThenFails` (AC-QUERY-005) + `TestState_ContinueSite_AfterRetry_IncrementsCounter` | `internal/query/loop/retry.go` + `continue_site.go` `after_retry` (counter ≤ 3, compaction 경계에서 reset) | T4.1 | REQ-QUERY-008, R5 |
| T4.4 | **RED #11** = `TestQueryLoop_CompactBoundaryYieldedOnCompact` (AC-QUERY-011) + `TestState_ContinueSite_AfterCompact_PreservesTaskBudget` | `continue_site.go` `after_compact` — Compactor 인터페이스 호출 + `CompactBoundary` yield | T4.1, T0.3 | REQ-QUERY-009, AC-QUERY-011 |
| T4.5 | `TestContinueSite_OnlyThreeReasons` (defensive) | Continue.Reason string 상수 3개만 존재 검증 | T4.1 | REQ-QUERY-003 방어 |

**S4 완료 기준**: AC-QUERY-002, 003, 005, 011 GREEN. spec.md §6.3 표의 3개 경로 모두 테스트로 커버.

### 단계 S5 — 상태 기반 종료 조건 (REQ-QUERY-011)

| Task ID | Test (RED) | 구현 (GREEN) | 의존 task | 매핑 REQ / AC |
|---------|-----------|-------------|----------|--------------|
| T5.1 | **RED #6** = `TestQueryLoop_BudgetExhausted` (AC-QUERY-006) | `TaskBudget.Remaining` 차감 + budget terminal | T4.3 | REQ-QUERY-011 |
| T5.2 | **RED #7** = `TestQueryLoop_MaxTurnsReached` (AC-QUERY-007) | `turnCount == maxTurns` 시 `Terminal{success:true, error:"max_turns"}` | T4.1 | REQ-QUERY-011 |
| T5.3 | **RED #4** = `TestQueryEngine_TwoSubmitMessages_ShareMessages` (AC-QUERY-004) | messages/turnCount/budget 누적을 engine 인스턴스 내 보존 | T3.5, T5.1 | REQ-QUERY-004 |

**S5 완료 기준**: AC-QUERY-004, 006, 007 GREEN.

### 단계 S6 — Abort & 예외 경로 (REQ-QUERY-010 / 014)

| Task ID | Test (RED) | 구현 (GREEN) | 의존 task | 매핑 REQ / AC |
|---------|-----------|-------------|----------|--------------|
| T6.1 | **RED #8** = `TestQueryLoop_AbortsOnContextCancel` (AC-QUERY-008) — 500ms 마감 측정 | `select { case out <- msg: ; case <-ctx.Done(): }` 전파 + `close(out)` + permission inbox 해제 | T4.1 | REQ-QUERY-010, R2 |
| T6.2 | `TestQueryLoop_ProviderError_NonRetriable` + `TestQueryLoop_StopFailureHooks_Invoked` | provider 4xx (except 429) 시 `StopFailureHooks` 호출 + `SDKMsgError` yield + `Terminal{success:false}` | T4.1 | REQ-QUERY-014 |
| T6.3 | `TestQueryLoop_UnknownToolName_Synthesizes404` | spec.md `tool_not_found` 합성 경로, terminal 유발 금지 | T4.1 | REQ-QUERY-017 |

**S6 완료 기준**: AC-QUERY-008 GREEN. 예외 경로 coverage 확보.

### 단계 S7 — Tool result budget + Coordinator filter + Fallback

| Task ID | Test (RED) | 구현 (GREEN) | 의존 task | 매핑 REQ / AC |
|---------|-----------|-------------|----------|--------------|
| T7.1 | **RED #9** = `TestQueryLoop_ToolResultBudgetReplacement` (AC-QUERY-009) | 1MB tool result → `{tool_use_id, truncated, bytes_original, bytes_kept}` 치환 | T4.1 | REQ-QUERY-007, AC-QUERY-009 |
| T7.2 | **RED #10** = `TestQueryLoop_CoordinatorModeToolFilter` (AC-QUERY-010) | LLM 호출 payload 구축 시 `scope:"leader_only"` 도구 제거 | T3.3 | REQ-QUERY-012 |
| T7.3 | **RED #12** = `TestQueryLoop_FallbackModelChain` (AC-QUERY-012) | primary 5xx/429 예산 소진 후 `FallbackModels[0]` 재호출 + 성공 시 Terminal success + fallback 로그 기록 | T4.1, T6.2 | REQ-QUERY-019 |

**S7 완료 기준**: AC-QUERY-009, 010, 012 GREEN.

### 단계 S8 — Middleware / Teammate / Permission inbox 완성 (REQ-QUERY-013/018/020, AC-QUERY-013/015/016)

| Task ID | Test (RED) | 구현 (GREEN) | 의존 task | 매핑 REQ / AC |
|---------|-----------|-------------|----------|--------------|
| T8.1 | **RED #15** = `TestQueryLoop_PostSamplingHooks_FifoChain` (AC-QUERY-015) + `TestQueryLoop_PostSamplingHooks_FIFO_MutateMessage` (unit) | `PostSamplingHooks` FIFO 적용 훅 체인 | T4.1 | REQ-QUERY-018, AC-QUERY-015 |
| T8.2 | **RED #13** = `TestQueryLoop_AskPermission_SuspendResume` (AC-QUERY-013) + `TestQueryEngine_ResolvePermission_ResumesLoop` (unit) | `ResolvePermission(toolUseID, Behavior)` inbox 전달 + loop 재개 + payload recorder 로 suspend 기간 LLM call 0 확증 | T4.2 | REQ-QUERY-013, AC-QUERY-013 |
| T8.3 | **RED #16** = `TestQueryEngine_TeammateIdentity_InjectedEverywhere` (AC-QUERY-016) + `TestQueryLoop_TeammateIdentity_InjectedIntoSystemPromptAndMetadata` (unit) | `TeammateIdentity` 전파 2경로 (system header + SDKMessage meta) | T3.3 | REQ-QUERY-020, AC-QUERY-016 |

**S8 완료 기준**: optional/suspend 계열 REQ 전부 GREEN. AC-013/015/016 integration test 포함.

### 단계 S9 — REFACTOR & 품질 게이트

| Task ID | 내용 | 산출 / 검증 |
|---------|-----|-----------|
| T9.1 | `continue_site.go` 를 단일 책임 파일로 추출 (이미 S4에서 분리되어 있음; 여기서는 dispatch 테이블 정리) | diff 에서 `loop.go` LoC 감소 + 분기 복잡도 ↓ |
| T9.2 | `StubLLMCall` 등 testsupport 를 공통 fixture builder로 리팩터 | 테스트 LoC 감소, race detector 여전히 통과 |
| T9.3 | 전체 `go test -race ./internal/...` + `golangci-lint run` | 두 명령 모두 exit 0 |
| T9.4 | coverage 집계 (`go test -coverprofile`) | 패키지별 타깃 달성 확인 (S9 §5 참조) |
| T9.5 | MX 태그 스윕 (§5 목록 기반) | 태그 누락 없음 |

---

## 3. 기술 스택 & 라이브러리 (production-stable 고정)

spec.md §7 의존성 표를 실행 관점에서 재확인. 본 SPEC 범위에서는 **CORE-001 결정을 그대로 승계하고 신규 외부 의존성 0**.

| 항목 | 선택 | 고정 버전 | 비고 |
|-----|------|---------|------|
| Go toolchain | stdlib `context`, `sync`, `encoding/json`, `testing` | **Go 1.22+** (go.mod 선언) | spec.md §7 외부 의존성 |
| 구조화 로깅 | `go.uber.org/zap` | **v1.27+** (CORE-001 pin 유지) | `zaptest.NewLogger(t)` 를 테스트에서 주입 |
| 테스트 assertion | `github.com/stretchr/testify/assert`, `.../require` | **v1.9+** | mock은 사용하지 않음 (스텁 직접 작성) |
| 동시성 | stdlib `sync.Mutex` | — | `SubmitMessage` 직렬화 단일 목적. `sync.RWMutex`/`atomic.Value` 거부 근거: research.md §5.3 |
| 의도적 **미사용** | tiktoken-go, goroutine pool, websocket, mockery | — | 본 SPEC 범위 밖 (research.md §7) |

`go.sum` 신규 변경 예상 범위: 없음(모두 CORE-001에서 기 도입). 만약 현재 `go.mod` 에 미등록 상태라면 S0 단계 첫 PR 에서 tidy 후 commit.

---

## 4. 리스크 완화 전략 (어느 task 에서 어떻게 검증할지)

spec.md §8 의 R1~R7 을 실행 task 에 귀속시킨다.

| # | 리스크 요약 | 검증 task | 검증 방법 |
|---|-----------|----------|---------|
| R1 | Channel backpressure가 UI 느릴 때 loop blocking | T3.3, T6.1 | unbuffered 전제 유지. `TestQueryEngine_SubmitMessage_Returns_Within_10ms` 에서 drain goroutine 분리 패턴을 테스트 픽스처로 시연. 옵션화 여부는 TRANSPORT-001 로 미룸(본 SPEC은 unbuffered 고정) |
| R2 | State race (특히 permission resume 시) | T3.4, T4.1, T8.2, T9.3 | 모든 Run task 에서 `go test -race` 필수. `ResolvePermission` 은 inbox 채널로만 loop 에 전달(외부 mutation 경로 0). S9 최종 race run 으로 종합 검증 |
| R3 | Compactor 인터페이스 mismatch (CONTEXT-001 과) | T4.4 | spec.md §6.2 `Compactor` 시그니처를 본 SPEC 에서 **먼저** 고정. CONTEXT-001 작성 시 cross-check checklist 항목. 테스트는 `StubCompactor` 로 격리 |
| R4 | tool_use 병렬 실행 정책 미결정 | S7 전체 | Phase 0 은 순차. spec.md Exclusions 10번째 조항 그대로. `TestQueryLoop_MultipleToolBlocks_SequentialExecution` 를 T7.1 하위에서 보조 검증 |
| R5 | max_output_tokens counter reset 시점 | T4.3, T4.4 | `after_compact` continue site 에서 counter → 0. `TestQueryLoop_RetryCounter_ResetsAfterCompact` 를 T4.4 에 추가 편성 |
| R6 | Fallback chain 과 credential pool 충돌 | T7.3 | 본 SPEC `FallbackModels` 는 **모델 alias**만. credential 선택은 LLMCall 구현체(ADAPTER-001) 가 담당. `StubLLMCall` 로 fallback 경로만 단위 검증 |
| R7 | 10ms SubmitMessage 마감 | T3.3 | 초기화 비용을 spawn 된 goroutine 내부로 이동. 테스트 fixture 에서 stub dial 을 100ms sleep 으로 세팅하고 SubmitMessage 반환이 10ms 이내인지 timer 로 측정 |

추가 방어:
- `TestState_ExternalMutation_Forbidden` — 외부 코드가 `engine.state` 에 접근할 수 없도록 unexported 필드임을 go vet 수준에서 확인 (컴파일 타임 방어).
- S9 종료 시점 `go test -race -count=5 ./internal/query/...` 를 수행해 flaky 케이스 조기 감지.

---

## 5. 파일 생성/수정 목록

spec.md §6.1 패키지 레이아웃을 실행 관점에서 task 매핑과 함께 확정.

### 5.1 신규 파일 (15개, research.md §10 규모와 정합)

| 경로 | 역할 | 생성 task |
|-----|------|---------|
| `internal/query/engine.go` | `QueryEngine` 구조체, `New`, `SubmitMessage`, `ResolvePermission` | T3.1 ~ T3.5 |
| `internal/query/engine_test.go` | engine 단위 테스트 (20+ 케이스) | T3.x 전반 |
| `internal/query/config.go` | `QueryEngineConfig`, `QueryParams`, `TaskBudget`, 인터페이스 타입 | T0.3 |
| `internal/query/testsupport/stubs.go` | Stub LLMCall/Executor/CanUseTool/Compactor | T0.4 |
| `internal/query/loop/loop.go` | `queryLoop` goroutine 본체 + 메인 iteration | T3.5, T4.1 |
| `internal/query/loop/continue_site.go` | 3개 Continue site 분기 (after_compact/after_retry/after_tool_results) | T4.1, T4.3, T4.4 |
| `internal/query/loop/retry.go` | `max_output_tokens` 재시도 로직 (≤3) | T4.3 |
| `internal/query/loop/state.go` | `State`, `Continue`, `Terminal` 타입 | T3.5 |
| `internal/query/loop/loop_test.go` | loop 통합 테스트 (AC 1:1 대응) | S4 ~ S8 |
| `internal/message/message.go` | `Message`, `ContentBlock`, `ToolResult` | T0.1, T1.x |
| `internal/message/sdk_message.go` | `SDKMessage`, 10개 Type enum, payload 구조체 | T0.1, T1.2 |
| `internal/message/stream_event.go` | `StreamEvent` delta | T0.1, T1.3 |
| `internal/message/normalize.go` | 연속 user merge, signature strip | T1.1 |
| `internal/message/summary.go` | `ToolUseSummaryMessage` 포맷 | T1.4 |
| `internal/permissions/{behavior,context,can_use_tool}.go` | 권한 타입 + 인터페이스 | T0.2, T2.x |

### 5.2 수정 대상 파일

- `go.mod` — `go.uber.org/zap`, `testify` 가 CORE-001 단계에서 이미 등록되어 있다면 변경 없음. 미등록 시 S0 에서 `go mod tidy`.
- `.moai/state/last-session-state.json` — 본 SPEC 범위 외 (runtime artifact).

### 5.3 생성하지 않는 파일 (명시)

- `internal/context/*` (CONTEXT-001)
- `internal/tools/*` (TOOLS-001)
- `internal/llm/*` / `internal/adapter/*` (ADAPTER-001)
- `internal/teammate/*` (SUBAGENT-001)
- `internal/hook/*` (HOOK-001)
- `cmd/goose/**`, `cmd/goosed/**` 신규 파일 — CORE-001/CLI-001 범위

---

## 6. MX 태그 계획

본 SPEC 산출 코드에 부여할 `@MX` 태그를 미리 식별. (실 태그 작성은 GREEN/REFACTOR 진행 중에 수행; `sync` 단계에서 최종 검증.)

### 6.1 `@MX:ANCHOR` — 도메인 불변 + 고 fan_in

| 지점 | 파일 / 심볼 | 이유 |
|-----|-----------|------|
| A1 | `QueryEngine.SubmitMessage` (`internal/query/engine.go`) | 모든 상위 레이어(CLI, TRANSPORT, SUBAGENT)의 단일 진입점. fan_in ≥ 3 예상 |
| A2 | `queryLoop` (`internal/query/loop/loop.go`) | agentic core의 상태 머신 본체. continue site 재할당 불변식의 중심 |
| A3 | `continue_site.go` 3개 분기 상수 (`ReasonAfterCompact`, `ReasonAfterRetry`, `ReasonAfterToolResults`) | REQ-QUERY-003 의 "오직 이 3곳" 계약을 코드로 잠그는 지점 |
| A4 | `CanUseTool.Check` (`internal/permissions/can_use_tool.go`) | 모든 tool 실행의 단일 gate (Security 불변) |

### 6.2 `@MX:WARN` — concurrency/goroutine 위험

| 지점 | 이유 | 동반 `@MX:REASON` |
|-----|------|------------------|
| W1 | `queryLoop` goroutine spawn 지점 (`engine.go SubmitMessage`) | spawned goroutine 이 `State` 를 단독 소유; 외부에서 touch 하면 race | "REQ-QUERY-015: state ownership은 loop goroutine 단일. 외부 mutation 금지" |
| W2 | permission inbox 채널 (`engine.go permInbox`) | 여러 goroutine 에서 send, loop 만 receive; buffering 정책에 주의 | "REQ-QUERY-013: Ask 분기 재개 단일 경로" |
| W3 | `out chan<- SDKMessage` close 지점 (`loop.go`) | close 책임 단일화 (loop만 close). 이중 close 패닉 방지 | "REQ-QUERY-002/010: close 단일 소유자" |
| W4 | `context.Done()` select 분기 (abort 전파) | ctx 취소 시 500ms 내 cleanup 마감 (REQ-QUERY-010) | "REQ-QUERY-010: 500ms abort" |

### 6.3 `@MX:NOTE` — 맥락/의도 전달

| 지점 | 내용 |
|-----|------|
| N1 | `TaskBudget.Remaining` 차감 위치 (`continue_site.go after_tool_results`) | "compaction 경계에서도 누적 보존, max_output_tokens counter만 reset" |
| N2 | `FallbackModels` 순회 (`loop.go`) | "본 SPEC은 모델 alias 수준. credential/provider 전환은 ADAPTER-001/ROUTER-001" |
| N3 | `TeammateIdentity` 주입 (`engine.go`, `loop.go`) | "systemPrompt header + SDKMessage metadata 두 경로. 세부 확장은 SUBAGENT-001" |
| N4 | `PostSamplingHooks` FIFO 호출부 | "middleware 순서는 FIFO 로 고정. HOOK-001 에서 composition 재검토 가능" |

### 6.4 `@MX:TODO` — GREEN 완료 시까지 존재하는 잔여 작업

- RED 단계 전체에서 최소 1회 활용: "fail now; resolve in GREEN" 형식.
- GREEN 후 전부 제거; 품질 게이트(`go vet` custom check 혹은 sync 단계 스캔)에서 잔존 시 실패로 취급.

---

## 7. TRUST 5 달성 경로 (실행 체크리스트)

spec.md §6.7 을 실행 체크리스트로 변환. 각 항목은 특정 task 종료 시 PASS 로 표시해야 S9 진입 가능.

### Tested

- [ ] `internal/query` ≥ 90% coverage — T3.x ~ T8.x 누적 (측정: S9 T9.4)
- [ ] `internal/query/loop` ≥ 92% coverage — continue site 3경로 × state-terminal 경로 조합 모두 테스트
- [ ] `internal/message` ≥ 85% coverage — T1.x
- [ ] `internal/permissions` ≥ 90% coverage — T2.x
- [ ] 가중 평균 ≥ 85% (quality.yaml 최소치 충족)
- [ ] Integration test 16개(AC-QUERY-001~016) 전부 GREEN (build tag `integration`)
- [ ] `go test -race -count=5 ./internal/query/...` 무경보 (S9 T9.3)

### Readable

- [ ] 패키지당 파일 LoC 평균 ≤ 200, 단일 함수 cyclomatic complexity ≤ 10
- [ ] continue site 이름은 코드 상수(`ReasonAfterCompact` 등)로 명시, 문자열 하드코딩 금지
- [ ] `godoc` 스타일 주석: `QueryEngine`, `queryLoop`, `CanUseTool`, `Continue`, `Terminal` 최소 수준
- [ ] 한글 주석은 `code_comments: ko` 정책상 허용이지만, 본 SPEC 대상 코드는 **코드 주석 영어** 통일(CORE-001 convention 계승 — 재확인 필요 시 기본 영어, 정책 충돌 시 운영 설정 따름)

### Unified

- [ ] `gofmt -s` + `goimports` 무변경
- [ ] `golangci-lint run` 무경고 (errcheck, govet, staticcheck, ineffassign, gocyclo 활성)
- [ ] 채널 close 책임 단일화: `queryLoop` 만 `close(out)` 수행 (grep 으로 검증)
- [ ] 에러 래핑 일관: `fmt.Errorf("...: %w", err)` 패턴

### Secured

- [ ] `CanUseTool` 외 경로에서 `tools.Executor.Run` 호출 경로 0 (grep 검증)
- [ ] `tool_not_found` 는 terminal 유발 금지 — AC 수준 검증 (REQ-QUERY-017)
- [ ] Tool result 1MB 치환 검증 — AC-QUERY-009
- [ ] Permission Ask 분기 중 추가 LLM token 소비 0 — AC 수준 검증 (REQ-QUERY-013)

### Trackable

- [ ] 모든 `SDKMessage` 에 `{turn, iteration, trace_id}` 메타 포함 (로깅/메타 field 로)
- [ ] zap logger 파생: `logger.With("turn", ..., "trace_id", ...)` 사용
- [ ] conventional commit + trailer (SPEC / REQ / AC) 규약 준수 (CLAUDE.local.md §2.2)
- [ ] sync 단계 commit 에 MX 태그 추가/수정 요약 기재

---

## 8. Quality Gate (merge 차단 기준)

spec.md §6.7 및 `.moai/config/sections/quality.yaml`(TDD 기본) 기반. 아래 항목 중 하나라도 실패 시 merge 금지.

| 게이트 | 도구 | 기준 | 실행 시점 |
|-------|-----|------|---------|
| Build | `go build ./...` | exit 0 | 모든 task 종료 시 |
| Race | `go test -race ./internal/query/... ./internal/message/... ./internal/permissions/...` | exit 0, data race 0 | T9.3 |
| Lint | `golangci-lint run ./internal/query/... ./internal/message/... ./internal/permissions/...` | 0 warnings | T9.3 |
| Unit coverage | `go test -coverprofile=... ./internal/...` | 가중 평균 ≥ 85%, `internal/query` ≥ 90%, `internal/query/loop` ≥ 92% | T9.4 |
| Integration | `go test -tags=integration ./internal/query/...` | AC-QUERY-001 ~ 016 GREEN (감사 D1/D2/D3 으로 013~016 추가) | T9.3 |
| Format | `gofmt -s -l ./internal/query ./internal/message ./internal/permissions` | 출력 없음 | T9.3 |
| MX tag sweep | `moai hook mx-scan` (또는 sync 단계 agent) | `@MX:TODO` 잔존 0 (RED 잔재 금지) | T9.5 |
| No forbidden import | grep `"github.com/stretchr/testify/mock"` | 미발견 (mock 라이브러리 금지; 스텁 직접 작성 정책) | T9.3 |
| CI workflow | `.github/workflows/ci.yml` build / vet / gofmt / test -race 단계 (최근 커밋 b4559f9 도입) | 전부 green | PR 단계 |

추가:
- SubmitMessage 10ms 마감은 `TestQueryEngine_SubmitMessage_Returns_Within_10ms` 가 성능 회귀 방지 단위로 포함.
- Abort 500ms 마감은 `TestQueryLoop_AbortsOnContextCancel` 이 1초 타임박스로 검증.

---

## 9. 일정/우선순위 (priority-based, 시간 추정 금지)

시간 예측은 규약상 금지. 단계 우선순위와 의존성만 기술한다.

- **Priority P0 (선행 차단 요인 해소)**: S0 → S1 → S2 → S3. 이 4단계가 끝나야 agentic core의 최소 "assistant text 1턴 스트리밍" 이 가능.
- **Priority P0 (핵심 경로)**: S4 (continue site 3종) + S5 (terminal 조건) — AC-QUERY-001~007 / 011 커버.
- **Priority P1 (안정성 강화)**: S6 (abort/예외), S7 (budget/coordinator/fallback).
- **Priority P2 (optional/확장점)**: S8 (hooks, permission resume, teammate).
- **Priority P2 (품질 고정)**: S9 (REFACTOR + gates).

병렬화 여지:
- S1(Message) 과 S2(Permissions) 는 서로 독립 → 병렬 가능.
- S3(Engine) 은 S1/S2 의 인터페이스만 의존 → S1 정상화 후 즉시 착수.
- S6/S7 은 S4/S5 완료 이후에는 상호 독립 — 별도 커밋/PR 로 병렬 가능.

---

## 10. 오픈 이슈 핸들링 (research.md §9 재고지)

| 이슈 | 본 SPEC의 잠정 결정 | 재검토 시점 |
|-----|-----------------|-----------|
| Streaming back-pressure 정책 (drop vs block) | unbuffered(block) 고정 | TRANSPORT-001 단계 |
| tool 병렬 실행 | 순차만 | TOOLS-001 단계 |
| retry counter reset 시점 | `after_compact` 에서 reset | T4.4 테스트로 고정 |
| Middleware hook 순서 | FIFO | HOOK-001 단계 |
| Fallback budget 계산 | 성공한 호출만 차감 | ROUTER-001 단계 |
| TeammateIdentity 전파 경로 | systemPrompt header + SDKMessage meta 2경로 | SUBAGENT-001 단계 |

위 결정은 본 SPEC GREEN 시점 테스트로 잠긴다. 변경 시 SPEC HISTORY 에 기록.

---

## 11. 종료 조건 (Definition of Done)

1. spec.md §5 의 AC-QUERY-001 ~ 016 전부 integration test GREEN (감사 review-1 D1/D2/D3 으로 AC-013/014/015/016 신설).
2. §8 Quality Gate 9개 항목 모두 PASS.
3. §6 MX 태그 계획 중 `@MX:ANCHOR` 4개 + `@MX:WARN` 4개 모두 해당 심볼/라인에 부착, `@MX:TODO` 0개 잔존.
4. `plan.md`(본 문서), `acceptance.md`, `spec-compact.md`, `research.md`, `spec.md` 가 `.moai/specs/SPEC-GOOSE-QUERY-001/` 하위에 공존.
5. conventional commit + SPEC/REQ/AC trailer 포함 커밋으로 `feature/SPEC-GOOSE-QUERY-001-plan` → main PR 생성 (merge 전략: squash).
6. SPEC HISTORY 섹션에 "0.2.0 GREEN 완료" 엔트리 추가.

---

**End of plan.md**
