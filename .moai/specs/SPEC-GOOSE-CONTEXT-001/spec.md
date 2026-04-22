---
id: SPEC-GOOSE-CONTEXT-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GOOSE-CONTEXT-001 — Context Window 관리 및 Compaction 전략

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-core §7 + QUERY-001 인터페이스 합의 기반) | manager-spec |

---

## 1. 개요 (Overview)

`QueryEngine`(SPEC-GOOSE-QUERY-001)의 매 iteration이 API 호출 전에 의존하는 **context 계층**을 정의한다. 본 SPEC은 다음 세 요소를 묶어 하나의 일관된 계약으로 제공한다:

1. **Context 소스 조립** — `SystemContext`(git/cacheBreaker) + `UserContext`(CLAUDE.md + currentDate) + `ToolUseContext`(per-iteration). 모두 session 생명주기 동안 memoized.
2. **Token 윈도우 추정** — `TokenCountWithEstimation(messages)`, `CalculateTokenWarningState(used, limit)`. context window 사용률 80% 도달 시 warning state 트리거.
3. **Compaction 3단 전략** — `AutoCompact`(예방적 LLM 요약), `ReactiveCompact`(다음 메시지 예측 기반 사전 압축), `Snip`(꼬리 절단; protected head/tail + redacted_thinking 보존). `QueryEngine`의 continue site가 호출할 `Compactor` 인터페이스로 결속.

본 SPEC은 코드 본체(`internal/context/`)의 **인터페이스 계약과 관찰 가능 행동**을 규정하고, `QueryEngine`의 iteration 루프가 호출하는 3개 함수(`ShouldCompact`, `Compact`, `Get*Context`)의 서명을 확정한다. LLM-기반 요약의 실제 호출은 SPEC-GOOSE-COMPRESSOR-001이 구현하고, 본 SPEC은 `Summarizer` 인터페이스의 consumer로만 참여한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- SPEC-GOOSE-QUERY-001의 continue site `after_compact` 경로는 `Compactor.ShouldCompact/Compact`를 호출하도록 설계되었다. 이 인터페이스가 구현되지 않으면 QUERY-001의 AC-QUERY-011, AC-QUERY-006은 통과할 수 없다.
- `.moai/project/research/claude-core.md` §7이 Claude Code의 `getSystemContext()`, `getUserContext()` 메모이제이션과 compaction 3단 전략을 명시한다. 본 SPEC은 그 포팅 경로를 Go로 확정한다.
- CORE-001이 `goosed` 데몬을 띄우고 QUERY-001이 loop 골격을 만든 직후 즉시 필요. Phase 0의 마지막 퍼즐 조각 중 하나.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code TypeScript**: `getSystemContext = memoize(async () => ({gitStatus, cacheBreaker}))`, `getUserContext = memoize(async () => ({claudeMd, currentDate}))`, `autoCompact/reactiveCompact/snip` 함수군. 언어 상이 직접 포트 없음.
- **Hermes Agent Python**: `hermes-agent-main/agent/context_compressor.py` — LLM 요약 기반 compaction의 원형. COMPRESSOR-001이 주 계승자. 본 SPEC은 compactor의 caller 인터페이스를 정의.
- **tiktoken-go** 등 외부 토크나이저: 본 SPEC이 사용 여부 결정 (오픈 이슈 §11에서 정리).

### 2.3 범위 경계

- **IN**: `SystemContext`/`UserContext`/`ToolUseContext` 구조체와 Getter, Session-level memoization, CLAUDE.md walk, Git status 조합, Token estimation (근사), Warning state, `Compactor` 인터페이스 구현, Snip 전략 구체 구현(protected window + redacted_thinking 보존), AutoCompact/ReactiveCompact 호출 orchestration, `CompactBoundary` payload 생성, Message normalize (consecutive user 병합, signature strip).
- **OUT**: LLM 요약 실제 호출 (→ COMPRESSOR-001), Token counting의 정확한 tokenizer 구현 (→ 본 SPEC은 근사치, 정확한 값은 ADAPTER-001의 provider response `usage`), CLAUDE.md 스키마 검증, Git 저장소 부재 시 에러 처리(→ CORE-001의 graceful 경로 계승), HISTORY_SNIP feature flag 자체 구현(→ CONFIG-001의 feature gate 계승).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/context/` 패키지 생성.
2. `SystemContext`, `UserContext`, `ToolUseContext` 구조체 및 Getter 함수.
3. Session-scoped memoization (`sync.Once` + `atomic.Pointer` 기반, `cacheBreaker`로 무효화).
4. `CLAUDE.md` 파일 탐색 (cwd → root까지 walk-up + `--add-dir` 경로 추가 검색).
5. Git status 조합 (`git status --porcelain` + `git branch --show-current` + `git log -1`) 및 4KB truncation.
6. `currentDate` 주입 (UTC ISO 8601).
7. `TokenCountWithEstimation(messages) int64`: characters/4 + tool_use/tool_result overhead 근사.
8. `CalculateTokenWarningState(used, limit int64) WarningLevel`: Green (<60%), Yellow (60-80%), Orange (80-92%), Red (>92%).
9. `Compactor` 인터페이스 구현체 `DefaultCompactor`:
   - `ShouldCompact(s query.State) bool`
   - `Compact(s query.State) (query.State, CompactBoundary, error)`
   - 내부에서 `Strategy` 선택 (`AutoCompact` > `ReactiveCompact` > `Snip` 우선순위).
10. `Snip` 전략 완전 구현 (protected head N=3, protected tail M=5, redacted_thinking 블록 절대 보존).
11. `AutoCompact`, `ReactiveCompact`는 `Summarizer` 인터페이스 호출로 위임 (본 SPEC은 orchestration + fallback만 구현; Summarizer가 주입되지 않으면 `Snip`으로 fallback).
12. `CompactBoundary` payload struct (turn, strategy, messages_before, messages_after, tokens_before, tokens_after, task_budget_preserved).
13. Message `Normalize([]Message) []Message`: consecutive user 병합, signature strip.
14. Context invalidation: `InvalidateUserContext()`, `InvalidateSystemContext()` — SubmitMessage 외부 이벤트(예: CLAUDE.md 수정 감지) 대응.

### 3.2 OUT OF SCOPE (명시적 제외)

- **LLM 요약 자체**: `Summarizer.Summarize(ctx, messages) (SummaryMessage, error)` 인터페이스 호출만. 실제 LLM 호출·프롬프트·모델 선택은 COMPRESSOR-001.
- **정확한 tokenizer**: 본 SPEC의 `TokenCountWithEstimation`은 휴리스틱. Provider가 반환하는 `usage.input_tokens`가 정확한 ground truth이며, 그 값은 ADAPTER-001이 `QueryEngine`에 전달.
- **SystemPromptInjection의 내용**: `QueryEngineConfig.SystemPrompt` 값은 CLI-001/CONFIG-001이 주입. 본 SPEC은 "invalidation trigger"로 인지만.
- **File watcher**: CLAUDE.md 변경 감지(inotify/fsnotify)는 본 SPEC 제외. `InvalidateUserContext()` 호출은 외부 이벤트 핸들러(Phase 2+)가 결정.
- **Compaction 결과의 적절성 평가**: reflect/rollback은 REFLECT-001/ROLLBACK-001.
- **Multi-session context 공유**: 본 SPEC의 memoization은 session-local. Cross-session cache는 MEMORY-001.
- **redacted_thinking 블록의 해석·재생성**: Anthropic extended thinking 블록은 opaque. 본 SPEC은 **보존만** 보장 (삭제 금지).

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CTX-001 [Ubiquitous]** — The `SystemContext` getter **shall** be idempotent within a session; repeated calls without explicit invalidation **shall** return the identical in-memory struct (pointer equality OR deep-equal on all fields).

**REQ-CTX-002 [Ubiquitous]** — The `UserContext` getter **shall** include `currentDate` formatted as UTC ISO 8601 (`2006-01-02T15:04:05Z`) computed at first-call time; subsequent calls within the same session **shall not** recompute unless `InvalidateUserContext()` has been called.

**REQ-CTX-003 [Ubiquitous]** — The `DefaultCompactor` **shall** preserve `redacted_thinking` content blocks across every compaction strategy; no snip or summarization pass **shall** drop or mutate such blocks.

**REQ-CTX-004 [Ubiquitous]** — The `TokenCountWithEstimation` function **shall** produce a deterministic integer for a fixed messages input (no randomness, no time-of-day dependence).

### 4.2 Event-Driven (이벤트 기반)

**REQ-CTX-005 [Event-Driven]** — **When** `GetSystemContext(ctx)` is called for the first time in a session, the function **shall** invoke `git branch --show-current`, `git status --porcelain`, and `git log -1 --format=%h %s` with a combined timeout of 2 seconds, concatenate the results, truncate to 4096 bytes, and cache the result under the session's `SystemContext`.

**REQ-CTX-006 [Event-Driven]** — **When** `GetUserContext(ctx, cwd, addDirs)` is called, the function **shall** walk from `cwd` to its filesystem root looking for `CLAUDE.md`, additionally search each entry in `addDirs`, concatenate all discovered files in document order, prepend `currentDate`, and cache under `UserContext`.

**REQ-CTX-007 [Event-Driven]** — **When** `Compactor.ShouldCompact(state)` is called, the function **shall** return `true` if and only if `TokenCountWithEstimation(state.Messages) / state.TokenLimit >= 0.80` OR `state.AutoCompactTracking.ReactiveTriggered == true` OR `len(state.Messages) > state.MaxMessageCount`.

**REQ-CTX-008 [Event-Driven]** — **When** `Compactor.Compact(state)` is invoked, the compactor **shall** (a) select a strategy in the order `AutoCompact → ReactiveCompact → Snip`, (b) if the selected strategy requires `Summarizer` but `Summarizer == nil`, fall back to `Snip`, (c) produce a new `State` with mutated `Messages` and preserved `TaskBudget.Remaining`, (d) return a `CompactBoundary` struct containing before/after metrics.

**REQ-CTX-009 [Event-Driven]** — **When** `Snip` executes, the strategy **shall** keep the first `ProtectedHead` messages (default 3) and the last `ProtectedTail` messages (default 5), drop messages in between, insert a single synthetic `<moai-snip-marker>` `Message` with `role:"system"` describing the number of dropped messages, and preserve every content block of type `redacted_thinking` from dropped messages by attaching them to the snip marker as auxiliary content.

**REQ-CTX-010 [Event-Driven]** — **When** compaction completes, the new `State.TaskBudget.Remaining` **shall** equal the pre-compaction `State.TaskBudget.Remaining` (unchanged); compaction itself **shall not** debit task budget, only LLM-summary calls performed by `Summarizer` may (and those are accounted by ADAPTER-001 in the surrounding turn).

### 4.3 State-Driven (상태 기반)

**REQ-CTX-011 [State-Driven]** — **While** `WarningLevel` derived from `CalculateTokenWarningState` is `Red` (>92% of limit), `Compactor.ShouldCompact` **shall** return `true` regardless of other conditions; this overrides REQ-CTX-007's 80% threshold.

**REQ-CTX-012 [State-Driven]** — **While** `Summarizer` interface is registered (`Compactor.Summarizer != nil`), the `AutoCompact` and `ReactiveCompact` strategies **shall** be eligible for selection; otherwise only `Snip` is selected.

### 4.4 Unwanted Behavior (방지)

**REQ-CTX-013 [Unwanted]** — The `DefaultCompactor.Compact` **shall not** return a `State` whose `Messages` is empty; at minimum the `<moai-snip-marker>` plus `ProtectedTail` messages **shall** remain.

**REQ-CTX-014 [Unwanted]** — **If** `Summarizer.Summarize` returns an error, **then** `AutoCompact` or `ReactiveCompact` **shall** log the error and fall back to `Snip` without surfacing the error to the caller (compaction must always succeed in some form).

**REQ-CTX-015 [Unwanted]** — The `GetSystemContext` function **shall not** fail the session if git commands time out or the directory is not a git repository; **if** git is unavailable, **then** the returned `SystemContext.GitStatus` **shall** be `"(no git)"` and the session **shall** continue.

### 4.5 Optional (선택적)

**REQ-CTX-016 [Optional]** — **Where** environment variable `GOOSE_HISTORY_SNIP=1` is set, the `Snip` strategy **shall** be preferred over `AutoCompact`/`ReactiveCompact` even when `Summarizer` is available; this feature gate allows deterministic snip-only mode for debugging.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-CTX-001 — SystemContext memoization**
- **Given** 테스트용 session + fake git executor counter
- **When** `GetSystemContext(ctx)`를 2회 호출
- **Then** git 실행 횟수는 1회, 두 호출 결과는 pointer-equal, `SystemContext.GitStatus`는 non-empty

**AC-CTX-002 — UserContext walks CLAUDE.md up to root**
- **Given** `/tmp/test/a/b/c`에 cwd 설정, `/tmp/test/a/CLAUDE.md`와 `/tmp/test/a/b/CLAUDE.md` 2개 파일 생성
- **When** `GetUserContext(ctx, "/tmp/test/a/b/c", nil)`
- **Then** 결과 `UserContext.ClaudeMd`는 두 파일 내용을 문서 순서대로 포함, `currentDate`는 `time.Now().UTC()` 근사(±1초), 두 번째 호출은 파일 IO 없이 캐시 반환

**AC-CTX-003 — Token estimation 근사 정확도**
- **Given** 알려진 문자열(예: 4,000자 영문 + 100자 한글 혼합) 1개 user message
- **When** `TokenCountWithEstimation([]Message{msg})`
- **Then** 반환값이 `providerGroundTruth ± 5%` 범위 (ground truth는 테스트 fixture에 cp949/utf-8 영/한 혼합 기준값)

**AC-CTX-004 — Warning state 80% 임계값 트리거**
- **Given** `limit=100_000`
- **When** `CalculateTokenWarningState(used=80_001, limit=100_000)`
- **Then** `WarningLevel == Orange` (80-92% 구간); `used=92_001` → `Red`; `used=60_001` → `Yellow`; `used=59_999` → `Green`

**AC-CTX-005 — AutoCompact 인터페이스 호출 (Summarizer mock)**
- **Given** `DefaultCompactor{Summarizer: stubSummarizer}`, stub이 항상 `SummaryMessage{Content:"...summary..."}`를 반환. State에 25개 message + token 사용량 90_000/100_000
- **When** `Compactor.Compact(state)`
- **Then** 결과 State의 messages는 `[snipMarker OR summary, ...ProtectedTail 5개]` 형태, `CompactBoundary.Strategy == "AutoCompact"`, Summarizer가 1회 호출됨

**AC-CTX-006 — Snip 전략의 protected window 및 redacted_thinking 보존**
- **Given** 20개 messages, 이 중 messages[5]와 messages[12]가 `redacted_thinking` 블록을 포함. ProtectedHead=3, ProtectedTail=5
- **When** Snip 실행
- **Then** 결과는 `[m0, m1, m2, snipMarker(with 2 redacted_thinking blocks), m15, m16, m17, m18, m19]`; redacted_thinking 블록 2개 모두 snipMarker의 auxiliary content에 보존됨

**AC-CTX-007 — Compaction 후 task_budget 보존**
- **Given** `State.TaskBudget.Remaining = 1234`, compaction 필요
- **When** `Compactor.Compact(state)`
- **Then** 결과 `State.TaskBudget.Remaining == 1234` (compaction 자체가 예산을 쓰지 않음)

**AC-CTX-008 — Summarizer 미등록 시 Snip fallback**
- **Given** `DefaultCompactor{Summarizer: nil}`, state가 AutoCompact 조건 충족
- **When** `Compact`
- **Then** `CompactBoundary.Strategy == "Snip"`, Summarizer 미호출, 결과 State 정상

**AC-CTX-009 — Summarizer 에러 시 Snip fallback**
- **Given** Summarizer가 `return errors.New("llm unavailable")`
- **When** `Compact`
- **Then** ERROR 로그 1회, `CompactBoundary.Strategy == "Snip"`, 호출자에게는 에러 전파되지 않음

**AC-CTX-010 — Context invalidation**
- **Given** `GetUserContext` 1회 호출 완료 (캐시됨), systemPromptInjection 변경 시뮬레이션 → `InvalidateUserContext()` 호출
- **When** `GetUserContext` 재호출
- **Then** 파일 IO가 다시 발생 (cache miss), 새 `UserContext` 반환, 이후 호출은 다시 캐시됨

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/
└── context/
    ├── system.go                 # GetSystemContext + memoize
    ├── user.go                   # GetUserContext + CLAUDE.md walk
    ├── tool_use.go               # ToolUseContext struct
    ├── tokens.go                 # TokenCountWithEstimation + WarningLevel
    ├── compactor.go              # DefaultCompactor implementation
    ├── strategy_snip.go          # Snip strategy
    ├── strategy_auto.go          # AutoCompact orchestration
    ├── strategy_reactive.go      # ReactiveCompact orchestration
    ├── summarizer.go             # Summarizer interface (COMPRESSOR-001 impl)
    ├── boundary.go               # CompactBoundary struct
    ├── normalize.go              # Message normalize helpers
    └── *_test.go
```

### 6.2 핵심 타입 (Go 시그니처 제안)

```go
// internal/context/system.go

type SystemContext struct {
    GitStatus    string    // 최대 4KB, 부재 시 "(no git)"
    CacheBreaker string    // build version or session id
    ComputedAt   time.Time
}

// memoize: session-local sync.Once + atomic.Pointer
// ctx 만료 시 git 명령 타임아웃
func GetSystemContext(ctx context.Context) (*SystemContext, error)
func InvalidateSystemContext()


// internal/context/user.go

type UserContext struct {
    ClaudeMd    string    // 모든 CLAUDE.md concat
    CurrentDate string    // ISO 8601 UTC
    ComputedAt  time.Time
}

func GetUserContext(ctx context.Context, cwd string, addDirs []string) (*UserContext, error)
func InvalidateUserContext()


// internal/context/tool_use.go

// ToolUseContext는 iteration 마다 새로 생성되는 mutable 구조체.
// QueryEngine.queryLoop가 continue site에서 교체.
type ToolUseContext struct {
    TurnIndex      int
    InvocationIDs  []string
    ReadFiles      []string  // 현 iteration에서 읽은 파일 경로
    WrittenFiles   []string
    PermissionCtx  ToolPermissionContext
}


// internal/context/tokens.go

type WarningLevel int
const (
    WarningGreen WarningLevel = iota  // <60%
    WarningYellow                     // 60-80%
    WarningOrange                     // 80-92%
    WarningRed                        // >92%
)

// 근사치 계산. provider.usage.input_tokens가 정답이지만
// compaction 트리거 판단용으로 근사치 사용.
func TokenCountWithEstimation(messages []message.Message) int64

func CalculateTokenWarningState(used, limit int64) WarningLevel


// internal/context/summarizer.go

// Summarizer는 COMPRESSOR-001이 구현. 본 SPEC은 consumer.
type Summarizer interface {
    Summarize(
        ctx context.Context,
        messages []message.Message,
        targetTokens int64,
    ) (message.Message, error)  // role:"system", content:summary
}


// internal/context/compactor.go

type DefaultCompactor struct {
    Summarizer         Summarizer          // optional; nil일 경우 Snip only
    ProtectedHead      int                 // default 3
    ProtectedTail      int                 // default 5
    MaxMessageCount    int                 // default 500
    TokenLimit         int64               // session token window
    HistorySnipOnly    bool                // REQ-CTX-016 feature gate
    Logger             *zap.Logger
}

func (c *DefaultCompactor) ShouldCompact(s loop.State) bool
func (c *DefaultCompactor) Compact(s loop.State) (loop.State, CompactBoundary, error)


// internal/context/boundary.go

type CompactBoundary struct {
    Turn                int
    Strategy            string  // "AutoCompact" | "ReactiveCompact" | "Snip"
    MessagesBefore      int
    MessagesAfter       int
    TokensBefore        int64
    TokensAfter         int64
    TaskBudgetPreserved int64   // REQ-CTX-010 검증용 투명성 필드
    DroppedThinkingCount int    // 보존된 redacted_thinking 블록 수
}
```

### 6.3 QUERY-001과의 인터페이스 정합

SPEC-GOOSE-QUERY-001 §6.2의 `QueryEngineConfig.Compactor`는 다음 메서드 셋을 요구:

```go
type Compactor interface {
    ShouldCompact(s loop.State) bool
    Compact(s loop.State) (loop.State, CompactBoundary, error)
}
```

`DefaultCompactor`가 이 인터페이스를 구현한다. 두 SPEC의 GREEN 단계 직전 **인터페이스 교차 검증 테스트**(`compactor_contract_test.go`)를 통해 서명 일치 보장.

### 6.4 Token Estimation 알고리즘 (MVP)

**전략**: "characters/4 + overhead" 근사.

```
tokens(message) =
    len(utf8.chars(textContent)) / 4              // 기본 텍스트
  + 12 * len(toolUseBlocks)                        // tool_use 블록당 12 토큰
  + len(utf8.chars(toolResultContent)) / 4         // tool_result 내용
  + 8 * len(redactedThinkingBlocks)                // opaque 블록당 8 토큰 근사
  + 4                                              // role/boundary overhead
```

- 한글·일문·중문은 UTF-8 3바이트이지만 tokenizer는 일반적으로 1.5-2 토큰/문자. `/4`(문자 기준)는 영문에 최적, CJK에는 undercount. **본 SPEC은 "±5% 내 근사"를 목표**(AC-CTX-003)로 하며, 정확한 값은 ADAPTER-001의 provider response `usage.input_tokens`로 보정.
- 향후 tiktoken-go 도입 시 interface 호환으로 교체 가능 (§11 오픈 이슈 1).

### 6.5 CLAUDE.md Walk 알고리즘

```
1. from = cwd
2. results = []
3. while from != "/" and from != "":
    if exists(from + "/CLAUDE.md"):
        results = [read(from + "/CLAUDE.md")] + results   // prepend
    from = parent(from)
4. for dir in addDirs:
    if exists(dir + "/CLAUDE.md"):
        results = results + [read(dir + "/CLAUDE.md")]
5. return join(results, "\n\n---\n\n")
```

- Root 도달 후 중단.
- `addDirs`는 `QueryEngineConfig` 또는 CLI `--add-dir` 플래그에서 주입.
- 순환 심볼릭 링크 방어: walk-up은 OS 파일시스템이 자연히 root에서 중단시키므로 별도 사이클 검출 불필요.

### 6.6 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestGetSystemContext_MemoizesGitCommand` — AC-CTX-001 → 실패.
2. **RED #2**: `TestGetUserContext_WalksUpAndConcatenates` — AC-CTX-002 → 실패.
3. **RED #3**: `TestTokenCountWithEstimation_Within5Percent` — AC-CTX-003.
4. **RED #4**: `TestCalculateTokenWarningState_Thresholds` — AC-CTX-004.
5. **RED #5**: `TestSnip_PreservesProtectedWindow` — AC-CTX-006.
6. **RED #6**: `TestSnip_PreservesRedactedThinking` — REQ-CTX-003.
7. **RED #7**: `TestCompactor_AutoCompactCallsSummarizer` — AC-CTX-005.
8. **RED #8**: `TestCompactor_TaskBudgetPreserved` — AC-CTX-007, REQ-CTX-010.
9. **RED #9**: `TestCompactor_NilSummarizer_FallsBackToSnip` — AC-CTX-008.
10. **RED #10**: `TestCompactor_SummarizerError_FallsBackToSnip` — AC-CTX-009.
11. **RED #11**: `TestInvalidateUserContext_ForcesRecompute` — AC-CTX-010.
12. **RED #12**: `TestCompactor_RedLevel_OverridesThreshold` — REQ-CTX-011.
13. **GREEN**: 최소 구현.
14. **REFACTOR**: strategy 모듈 분리(snip/auto/reactive 각 파일), Summarizer 인터페이스 seam 정리.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, redacted_thinking 보존 property-based test, CJK 문자열 fixture |
| **R**eadable | strategy 파일 분리 (snip/auto/reactive), 상수명 ProtectedHead/Tail 명시 |
| **U**nified | `go fmt` + `golangci-lint`, 모든 compaction 경로가 동일 CompactBoundary 반환 |
| **S**ecured | redacted_thinking 블록은 opaque — 삭제 금지(REQ-CTX-003). Git 명령은 cwd 내부에서만 실행, user input은 CLAUDE.md 내용에 그대로 포함(raw) |
| **T**rackable | 모든 Compact 호출에 structured log (`strategy`, `messages_before/after`, `tokens_before/after`) |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap 로거, context 루트 |
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `Compactor` 인터페이스 서명, `loop.State`, `message.Message` |
| 후속 SPEC | SPEC-GOOSE-COMPRESSOR-001 | `Summarizer.Summarize` 실 구현 (LLM 요약) |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-001 | provider `usage.input_tokens`로 token count 보정 |
| 후속 SPEC | SPEC-GOOSE-CONFIG-001 | `GOOSE_HISTORY_SNIP`, `TokenLimit`, `ProtectedHead/Tail` 설정값 |
| 외부 | Go 1.22+ | `sync.Once`, `atomic.Pointer[T]` generics |
| 외부 | `go.uber.org/zap` v1.27+ | 구조화 로깅 |
| 외부 | Git (CLI, runtime dependency) | `git status`, `git branch`, `git log` 실행 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Token estimation 근사치가 CJK에서 크게 undercount하여 compaction 지연 → context overflow | 중 | 고 | Warning Red(>92%)에서 REQ-CTX-011로 강제 compact. 후속으로 tiktoken-go 도입 검토 (§11 오픈 이슈 1) |
| R2 | redacted_thinking 블록의 auxiliary content 포맷이 Anthropic API와 맞지 않아 다음 호출 실패 | 중 | 고 | Snip marker는 `role:"system"`으로 두고 redacted_thinking을 독립 content block으로 attach. ADAPTER-001과 integration test로 검증 |
| R3 | CLAUDE.md walk가 심볼릭 링크 순환에 빠짐 | 낮 | 중 | OS filesystem이 root에서 자연 중단. 명시적 cycle detection 불필요 |
| R4 | Git 명령 타임아웃(2초) 내에 끝나지 않는 대형 레포에서 session start 지연 | 낮 | 낮 | 타임아웃 시 `GitStatus="(no git)"`로 graceful (REQ-CTX-015) |
| R5 | Summarizer가 task_budget을 소비하는데 본 SPEC이 "compaction은 예산 불변"이라 모순 | 중 | 중 | 해결: Summarizer 호출은 **외부 turn**으로 본다. 호출자(QueryEngine)가 turn 차감, 본 SPEC의 REQ-CTX-010은 "compaction 자체"는 예산 불변 (Summarizer가 소비한 토큰은 그 turn에 이미 계상됨) |
| R6 | AutoCompact 후에도 token count가 여전히 80% 이상 → 무한 compact loop | 낮 | 고 | `ShouldCompact` 판단 시 "직전 compaction 이후 새 메시지 ≥1"을 요구 (QUERY-001 continue site가 보장) |
| R7 | ProtectedTail이 tool_result로만 구성되어 context가 무의미 | 중 | 중 | Snip marker가 "직전 요약"을 synthetic message로 삽입하여 의미 복원. COMPRESSOR-001 도입 시 auto-summary로 대체 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/claude-core.md` §3 (TS 인터페이스: getSystemContext/getUserContext memoize), §4 (포팅 매핑), §6 원칙 5·10, §7 SPEC-GOOSE-CONTEXT-001 초안, §8 리스크(budget undercount)
- `.moai/project/structure.md` §1 (`internal/memory/` vs 본 SPEC `internal/context/` 경계), §4 (모듈 책임)
- `.moai/project/tech.md` §3.1 (Go 런타임)
- `.moai/specs/ROADMAP.md` §4 Phase 0 #03, §13 원칙 2·5
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` — Compactor 인터페이스 요구 원천

### 9.2 외부 참조

- **Claude Code TypeScript**: `./claude-code-source-map/bootstrap/state.ts` — SystemContext memoize 원형 (패턴만)
- **Hermes Agent Python**: `./hermes-agent-main/agent/context_compressor.py` — COMPRESSOR-001의 주 참조; 본 SPEC은 interface consumer
- **Anthropic API docs**: extended thinking, `redacted_thinking` 블록 opaque 보존 요구
- **Go `sync.Once`, `atomic.Pointer`**: https://pkg.go.dev/sync, https://pkg.go.dev/sync/atomic

### 9.3 부속 문서

- `./research.md` — claude-core.md §7 분석 상세, tokenizer 결정 근거, 테스트 전략
- `../SPEC-GOOSE-QUERY-001/spec.md` — 인터페이스 원천
- `../SPEC-GOOSE-QUERY-001/research.md` — State.TaskBudget 계약 원천
- `../ROADMAP.md` — 전체 Phase 계획

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **LLM 요약을 직접 호출하지 않는다**. `Summarizer.Summarize` 인터페이스 호출만. 실제 LLM 통신은 COMPRESSOR-001 + ADAPTER-001.
- 본 SPEC은 **정확한 tokenizer를 구현하지 않는다**. `TokenCountWithEstimation`은 근사 ±5%. 정확한 값은 provider response `usage.input_tokens`가 ground truth.
- 본 SPEC은 **CLAUDE.md 스키마 검증을 수행하지 않는다**. 내용은 raw concat.
- 본 SPEC은 **File watcher(fsnotify)를 포함하지 않는다**. `InvalidateUserContext()` 호출은 외부 이벤트 핸들러의 책임.
- 본 SPEC은 **Compaction 결과의 품질 평가·rollback을 수행하지 않는다**. REFLECT-001/ROLLBACK-001.
- 본 SPEC은 **Cross-session context cache를 구현하지 않는다**. MEMORY-001.
- 본 SPEC은 **redacted_thinking 블록의 내용을 해석·재생성하지 않는다**. opaque 보존만.
- 본 SPEC은 **HISTORY_SNIP feature gate의 전역 설정 로드를 구현하지 않는다**. `DefaultCompactor.HistorySnipOnly` 필드로 받아서 사용만 하며, env 파싱은 CONFIG-001.

---

**End of SPEC-GOOSE-CONTEXT-001**
