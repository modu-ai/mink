# SPEC-GENIE-QUERY-001 — Research & Porting Analysis

> **목적**: Claude Code TypeScript의 async generator 기반 query loop를 Go로 이식할 때 보존해야 할 설계 원칙, 재사용 가능한 로직, 재작성이 필요한 영역을 정리한다. `.moai/project/research/claude-core.md`의 분석 결과를 본 SPEC 요구사항과 1:1 매핑한다.
> **작성일**: 2026-04-21
> **범위**: `internal/query/`, `internal/query/loop/`, `internal/message/`, `internal/permissions/` 4개 패키지.

---

## 1. 레포 현재 상태 스캔

```
$ ls /Users/goos/MoAI/AgentOS/
.claude  .moai  CLAUDE.md  README.md
claude-code-source-map/   # TypeScript 참조 (bootstrap/, entrypoints/, bridge/)
hermes-agent-main/        # Python 참조 (agent/, tools/, cli.py)
```

- `internal/query/`, `internal/message/`, `internal/permissions/` → **전부 부재**. Phase 0에서 신규 작성.
- CORE-001이 수립할 `internal/core/`, `cmd/genied/`만 선행 존재 가정.

**결론**: 본 SPEC의 GREEN 단계는 4개 패키지를 **zero-to-one 신규 작성**이며, Claude Code TypeScript는 **언어 상이로 직접 포트 대상이 아니다**. 상태 머신 토폴로지와 continue site 분기 로직만 번역한다.

---

## 2. claude-core.md §6 설계 원칙 → SPEC 요구사항 매핑

`.moai/project/research/claude-core.md` §6이 명시한 10개 핵심 설계 원칙을 본 SPEC의 어느 REQ가 구현하는지 명시한다.

| # | Claude Code 설계 원칙 | 본 SPEC REQ | Go 이디엄 |
|---|---|---|---|
| 1 | One QueryEngine per conversation — 세션 생명주기 = engine 생명주기 | REQ-QUERY-001, REQ-QUERY-004 | 구조체 + `New()` 팩토리 1회, `SubmitMessage` 다회 |
| 2 | Streaming mandatory — SDK 호출자는 응답을 즉시 받아야 함 | REQ-QUERY-002, REQ-QUERY-005, REQ-QUERY-016 | unbuffered `<-chan SDKMessage` 반환, 10ms 마감 |
| 3 | Tool call permission gate is async — `canUseTool()` await 가능 | REQ-QUERY-006, REQ-QUERY-013 | `CanUseTool.Check` blocking OK + Ask 분기는 inbox 채널로 suspend/resume |
| 4 | Message arrays mutable within a turn — 다음 턴에는 새 스냅샷 | REQ-QUERY-003, REQ-QUERY-015 | Continue site에서만 `State` 재할당, 외부 mutation 금지 |
| 5 | Budget tracking cumulative across compactions — task_budget.remaining은 compaction 경계마다 누적 | REQ-QUERY-011, AC-QUERY-011 | `TaskBudget.Remaining`은 compaction 전후 보존 |
| 6 | Continue sites explicit — 복원 경로는 state를 새로 할당 | REQ-QUERY-003, §6.3 표 | `Continue{Reason, NextState}` 구조체 + 3개 경로(after_compact/after_retry/after_tool_results) |
| 7 | Task types are union, not polymorphic — discriminated union | REQ-QUERY-002 (SDKMessage 매핑) | `SDKMessage{Type, Payload any}` + type-switch 분기 (TASK-001에서 별도 처리) |
| 8 | Async agents identity-scoped — InProcessTeammate agentId+teamName + AsyncLocalStorage | REQ-QUERY-020 | `TeammateIdentity` optional 필드 + `context.WithValue`로 loop scope 전파 (SUBAGENT-001 확장) |
| 9 | Coordinator mode changes tool visibility — `isCoordinatorMode()` → worker 도구만 | REQ-QUERY-012, AC-QUERY-010 | `CoordinatorMode bool` + tool filter at LLM call payload construction |
| 10 | Context memoized per session — `getSystemContext()` + `getUserContext()` 한 번만 계산 | (CONTEXT-001로 위임) | 본 SPEC은 memoized 결과를 `QueryParams`로 주입받기만 |

10개 중 9개가 본 SPEC이 직접 담당, 1개(Context memoized)는 CONTEXT-001로 위임. 상호 의존성을 §7 "의존성" 섹션과 중복 문서화한다.

---

## 3. claude-core.md §4 포팅 매핑표 → Go 패키지 결정

claude-core.md §4가 제시한 포팅 경로를 Go 패키지 결정으로 고정한다.

| Claude Code (TS) | GENIE (Go) 패키지 | 결정 근거 |
|---|---|---|
| `QueryEngine` 클래스 | `internal/query/engine.go:QueryEngine` | 상태 소유자 분리 |
| `query()` + `queryLoop()` | `internal/query/loop/loop.go:queryLoop` goroutine | engine은 조정자, loop는 실행자 |
| `Message[]` | `internal/message/message.go:[]Message` | 재사용 타입 (전 패키지 공유) |
| `ToolUseContext` | `internal/context/tool_use.go:ToolUseContext` | **CONTEXT-001 소유**, 본 SPEC은 import |
| `canUseTool()` | `internal/permissions/can_use_tool.go:CanUseTool` 인터페이스 | 권한 결정 분리, SUBAGENT-001 확장 |
| `runTools()` | `internal/tools.Executor` 인터페이스 호출 | TOOLS-001 소유, 본 SPEC은 consumer |
| `Task types` (5종) | `internal/task/*` | TASK-001 (별도 SPEC). 본 SPEC은 미다룸 |
| `TeammateIdentity` | `internal/teammate/identity.go` | SUBAGENT-001 소유, 본 SPEC은 optional 필드 |
| `getSystemContext()` | `internal/context/system.go` | CONTEXT-001 |
| `upstreamproxy` | `internal/llm/proxy/` | ADAPTER-001 |

**본 SPEC이 "생성하는" 패키지**: `internal/query/`, `internal/query/loop/`, `internal/message/`, `internal/permissions/` (4개).
**본 SPEC이 "소비만 하는" 패키지**: `internal/context/`, `internal/tools/`, `internal/teammate/` (3개 — 각자 SPEC).

---

## 4. claude-core.md §8 고리스크 영역 → 완화 전략

§8이 지적한 4가지 고리스크 영역을 본 SPEC의 요구사항/리스크 섹션으로 흡수한다.

| § 8 리스크 | 본 SPEC 위치 | 완화 |
|---|---|---|
| Circular async generator (processUserInput이 model/prompt/messages 변경 가능) | §3.2 OUT ("processUserInput은 passthrough"), COMMAND-001로 위임 | Phase 0 동안 model hot-swap 금지. COMMAND-001이 도입 시 continue site 재검토 |
| Mutable message array (mutableMessages가 후속 iteration에 영향) | REQ-QUERY-015, Risk R2 | 단일 goroutine state ownership, `go test -race` 필수 |
| Budget undercount (compaction 후 task_budget.remaining 미추적) | REQ-QUERY-011, AC-QUERY-011, Risk R5 | `after_compact` continue site가 누적값 보존, `maxOutputTokensRecoveryCount`는 reset |
| Tool permission race (canUseTool 실행 중 AppState.toolPermissionContext 변경) | REQ-QUERY-013, Risk R2 | Permission inbox 채널로 직렬화. loop만이 state를 읽고 씀 |

---

## 5. Go 이디엄 선택 (상세 근거)

### 5.1 AsyncGenerator → goroutine + channel

| TS 패턴 | Go 이디엄 | 선택 이유 |
|---|---|---|
| `async function* queryLoop()` + `yield msg` | goroutine + `out chan<- SDKMessage` + `select { case out <- msg: ; case <-ctx.Done(): return }` | Go의 관용. `range ch` 소비, `close(ch)` EOF |
| `yield* nested()` | helper 함수가 동일 `out` 채널에 직접 send | 중첩 generator 없는 단순 모델 |
| `for await (chunk of stream)` | `for chunk := range chunkCh` | LLMCall이 반환하는 chunk 채널을 loop가 range |
| `AbortController.signal` | `context.Context`의 `Done()` + `Err()` | stdlib 계약 |
| `AsyncLocalStorage.run(scope, fn)` | `context.WithValue(parent, teammateKey, identity)` | 요청 스코프 값 |

### 5.2 Streaming 인터페이스: `<-chan SDKMessage` 채택 이유

- **Unbuffered 채널**: REQ-QUERY-002의 "buffering 금지" 조건을 컴파일러가 강제. 소비자가 늦으면 loop도 늦는 자연스런 backpressure.
- **close(ch)로 EOF**: Terminal 메시지 송신 후 `close(out)` → 소비자 `for range` 자동 종료.
- **select와 자연 통합**: `select { case msg := <-ch: ... ; case <-ctx.Done(): ... }`
- **대안 검토 (SPEC §6.4 참조)**:
  - `io.ReadCloser`(JSON-L): in-process에서 인코딩 왕복 낭비. gRPC transport (TRANSPORT-001)가 채널을 JSON-L로 직렬화하는 게 정석.
  - `Iterator[SDKMessage]` (Go 1.23 range-over-func): Go 1.22 최소 호환성 유지 필요.

### 5.3 State ownership: 단일 goroutine

**원칙**: State 읽기·쓰기는 `queryLoop` goroutine만 수행. 외부는 `SubmitMessage`(input) / `ResolvePermission`(input) / `<-chan SDKMessage`(output) 세 채널만 접촉.

**대안 거부**:
- `sync.RWMutex` 보호 State: race 원천은 막지만 복잡도 급증. Continue site 규약을 해치는 mutation이 분산됨.
- `atomic.Value` with immutable State snapshot: 구현 단순하나, State가 큼(messages[] 포함) → 매 turn 전체 복사는 낭비.

**채택**: 단일 owner + permission inbox 채널. Claude Code TS의 `AsyncLocalStorage.run` 스코프와 의미상 동등.

### 5.4 Tokenizer 라이브러리 (본 SPEC 범위)

본 SPEC은 token counting을 직접 수행하지 **않는다**. `Compactor.ShouldCompact(State) bool`이 결정. 따라서 tokenizer 선택(tiktoken-go vs 직접 구현 vs KoE)은 **CONTEXT-001의 오픈 이슈**.

단, 본 SPEC의 `TaskBudget.Remaining` 차감 계산은 provider가 반환하는 `usage.input_tokens + usage.output_tokens`를 그대로 사용 (ADAPTER-001의 책임).

### 5.5 로거 — `go.uber.org/zap` (CORE-001 결정 계승)

- `zap.Logger`는 `QueryEngineConfig.Logger`로 주입. Loop는 `.With("turn", state.TurnCount, "trace_id", ...)`로 파생.
- `slog` 대안은 stdlib이지만 allocation 성능 약점. CORE-001 결정을 따른다.

---

## 6. 참조 가능한 외부 자산 분석

### 6.1 Claude Code TypeScript (`./claude-code-source-map/`)

본 SPEC 관련 파일 (grep으로 확인):

```
claude-code-source-map/
├── bootstrap/state.ts           # (CORE-001 관련, 본 SPEC은 참조 없음)
├── QueryEngine 관련 파일들       # claude-core.md §3의 TS 시그니처가 출처
```

- claude-core.md §3의 TS 시그니처는 **본 SPEC §6.2 Go 시그니처의 1차 참조 원본**이다.
- `entrypoints/init.ts:gracefulShutdownSync(exitCode)` 패턴은 CORE-001에서 이미 흡수됨. 본 SPEC은 `ctx.Done()` 전파를 통해 CORE-001의 graceful shutdown에 참여.
- **직접 포트 대상 없음**. 패턴만 번역.

### 6.2 Hermes Agent Python (`./hermes-agent-main/`)

- `hermes-agent-main/agent/` 내부의 `model_tools.py`, `context_compressor.py`, `insights.py`는 **후속 SPEC(TOOLS-001, COMPRESSOR-001, INSIGHTS-001)의 원형**. 본 SPEC은 소비자 인터페이스만 정의.
- `cli.py` (409KB 단일 파일) — 파싱 난해. 설계 참조 가치 제한.
- **직접 포트 대상 없음**.

### 6.3 MoAI-ADK-Go

- 본 레포에 미러 없음. 외부 레포라고만 문서 기술. 본 SPEC은 독립 작성.

---

## 7. 외부 의존성 합계

| 모듈 | 버전 | 본 SPEC 사용 | 결정 근거 |
|------|-----|-----------|---------|
| `go.uber.org/zap` | v1.27+ | ✅ 구조화 로깅 | CORE-001 결정 계승 |
| `github.com/stretchr/testify` | v1.9+ | ✅ 테스트 assertion | CORE-001 결정 계승 |
| Go stdlib `context` | 1.22+ | ✅ cancellation, values | AsyncLocalStorage 대체 |
| Go stdlib `sync` | 1.22+ | ✅ `sync.Mutex`(SubmitMessage 직렬화만) | 최소주의 |
| Go stdlib `encoding/json` | 1.22+ | ✅ Message/ContentBlock 직렬화 | 표준 |

**의도적 미사용** (Phase 0):
- **tiktoken-go**: token counting은 CONTEXT-001/ADAPTER-001의 책임. 본 SPEC 직접 미사용.
- **`uber-go/zap/zaptest`** vs `testify`: 본 SPEC은 `testify` + `zaptest.NewLogger`로 충분.
- **`github.com/gorilla/websocket`**: TRANSPORT-001의 범위.
- **goroutine pool 라이브러리**(ants, tunny): loop는 세션당 1 goroutine. 풀 불필요.

---

## 8. 테스트 전략 (TDD RED → GREEN)

### 8.1 Unit 테스트 (25~35개)

**Message/SDKMessage 레이어** (`internal/message/`):

- `TestMessage_Normalize_MergesConsecutiveUser` — 연속 user 메시지 병합 (claude-core.md §7 normalize)
- `TestMessage_Normalize_StripsSignatureFromAssistant` — signature 제거
- `TestSDKMessage_TypeSwitchExhaustive` — 9개 Type 전부 payload 매핑 검증
- `TestStreamEvent_DeltaOrdering` — index 순서 보존

**Permissions 레이어** (`internal/permissions/`):

- `TestCanUseTool_AllowBypassesGate` — Allow behavior는 즉시 통과
- `TestCanUseTool_DenyProducesErrorResult` — Deny는 `is_error:true` 결과 합성
- `TestCanUseTool_AskSuspendsLoop` — Ask는 inbox 채널 대기

**QueryEngine 레이어** (`internal/query/`):

- `TestQueryEngine_New_ValidConfig_Succeeds`
- `TestQueryEngine_New_MissingRequiredField_Fails` (LLMCall, Tools, CanUseTool)
- `TestQueryEngine_SubmitMessage_Returns_Within_10ms` — REQ-QUERY-016
- `TestQueryEngine_SubmitMessage_ReturnsReceiveOnlyChannel` — type check
- `TestQueryEngine_Concurrent_SubmitMessage_IsSerialized` — mu 검증

**QueryLoop 레이어** (`internal/query/loop/`):

- `TestState_ContinueSite_AfterCompact_PreservesTaskBudget` — 누적값 보존
- `TestState_ContinueSite_AfterRetry_IncrementsCounter`
- `TestState_ContinueSite_AfterToolResults_IncrementsTurnCount`
- `TestContinueSite_OnlyThreeReasons` — Continue.Reason 값 검증

### 8.2 Integration 테스트 (12~18개, build tag `integration`)

각 AC에 1:1 대응하는 Go test:

| AC | Test 함수 | Build tag |
|---|---|---|
| AC-QUERY-001 | `TestQueryEngine_SubmitMessage_StreamsImmediately` | integration |
| AC-QUERY-002 | `TestQueryLoop_ToolCallAllow_FullRoundtrip` | integration |
| AC-QUERY-003 | `TestQueryLoop_PermissionDeny_SynthesizesErrorResult` | integration |
| AC-QUERY-004 | `TestQueryEngine_TwoSubmitMessages_ShareMessages` | integration |
| AC-QUERY-005 | `TestQueryLoop_MaxOutputTokens_Retries3ThenFails` | integration |
| AC-QUERY-006 | `TestQueryLoop_BudgetExhausted` | integration |
| AC-QUERY-007 | `TestQueryLoop_MaxTurnsReached` | integration |
| AC-QUERY-008 | `TestQueryLoop_AbortsOnContextCancel` | integration |
| AC-QUERY-009 | `TestQueryLoop_ToolResultBudgetReplacement` | integration |
| AC-QUERY-010 | `TestQueryLoop_CoordinatorModeToolFilter` | integration |
| AC-QUERY-011 | `TestQueryLoop_CompactBoundaryYieldedOnCompact` | integration |
| AC-QUERY-012 | `TestQueryLoop_FallbackModelChain` | integration |

### 8.3 Stub 전략

- `StubLLMCall(chunks []StreamEvent, terminal Terminal) LLMCallFunc` — fixture 기반 LLM 응답.
- `StubExecutor(toolName string, fn func(input) ToolResult) tools.Executor` — deterministic tool.
- `StubCanUseTool(behaviors map[string]Behavior) permissions.CanUseTool`.
- `StubCompactor(triggerAtTurn int, summarize func) context.Compactor`.

### 8.4 Race detector

`go test -race ./internal/query/...` 필수. 특히 REQ-QUERY-015 검증: 외부에서 State를 touch하는 코드 경로가 없음을 race detector로 증명.

### 8.5 커버리지 목표

- `internal/query/`: 90%+
- `internal/query/loop/`: 92%+ (continue site 분기 모두 커버)
- `internal/message/`: 85%+
- `internal/permissions/`: 90%+
- 전체 가중 평균: 88%+ (quality.yaml TDD 요구 85% 초과)

---

## 9. 오픈 이슈

1. **Streaming 인터페이스 최종 확정**: 본 SPEC은 `<-chan SDKMessage` 채택. TRANSPORT-001이 gRPC server streaming으로 wrap할 때 back-pressure 전략(drop vs block)은 TRANSPORT-001에서 결정.
2. **Tool 병렬 실행**: Phase 0은 순차. TOOLS-001에서 `parallel_tool_use` 플래그 도입 시 본 SPEC의 continue site에 새 reason `after_parallel_tools` 추가 여부 재검토.
3. **max_output_tokens 재시도 카운터 reset 시점**: 본 SPEC은 "compaction 경계에서 reset"로 결정 (R5). 대안: "session 전체 누적 3회". 테스트 AC-QUERY-005의 fixture로 선택 검증.
4. **Middleware hook 순서**: `PostSamplingHooks`는 FIFO 적용. 원문 Claude Code는 pipeline composition이 더 복잡할 수 있음 — HOOK-001에서 재검토.
5. **Fallback model의 budget 계산**: 같은 turn 내 primary 실패 후 fallback 호출 시 `task_budget`을 2회 차감? 1회? 본 SPEC은 "성공한 호출만 차감"으로 잠정. ROUTER-001과 합의 필요.
6. **`TeammateIdentity` 전파 경로**: 본 SPEC은 systemPrompt header + SDKMessage 메타 두 경로. Subagent가 Sub-sub-agent를 spawn하는 시나리오는 SUBAGENT-001에서 규정.

---

## 10. 구현 규모 예상

| 영역 | 파일 수 | 신규 LoC | 테스트 LoC |
|---|---|---|---|
| `internal/query/` | 3 | 350 | 600 |
| `internal/query/loop/` | 4 | 700 | 900 |
| `internal/message/` | 5 | 400 | 450 |
| `internal/permissions/` | 3 | 150 | 200 |
| **합계** | **15** | **~1,600** | **~2,150** |

테스트 비율: 57% (TDD 권장 1:1 이상 충족).

---

## 11. 결론

- **상속 자산**: TypeScript source map은 설계 참조, 직접 포트 없음. 패턴 번역이 전부.
- **핵심 결정**:
  - Streaming: **unbuffered `<-chan SDKMessage`** (io.ReadCloser 거부).
  - State: **단일 queryLoop goroutine 소유** (mutex 보호 거부).
  - Continue site: **3개 reason**(after_compact / after_retry / after_tool_results)으로 명시.
  - Compactor/Executor/LLMCall: **인터페이스로 주입**, 본 SPEC은 호출자.
- **Go 버전**: 1.22+ (CORE-001과 정합).
- **리스크**: R1~R7(SPEC §8)로 관리. 가장 높은 리스크는 R2(State race) — race detector로 방어.
- **다음 단계 선행 요건**: SPEC-GENIE-CONTEXT-001의 `Compactor` 인터페이스 서명이 본 SPEC §6.2와 동일해야 함. 두 SPEC이 동시 작성되므로 인터페이스 교차 검증이 GREEN 직전 작업.

본 SPEC이 통과하면 **이후 모든 agentic 기능(tools/skills/mcp/subagent/hook/command/cli)이 의존할 수 있는 단일 agentic core가 확보된다**.

---

**End of research.md**
