# SPEC-GENIE-CONTEXT-001 — Research & Porting Analysis

> **목적**: Claude Code TypeScript의 Context 계층(`getSystemContext/getUserContext/autoCompact/reactiveCompact/snip`)을 Go로 이식하면서 보존해야 할 불변식과 Go 이디엄 선택을 문서화한다. SPEC-GENIE-QUERY-001의 `Compactor` 인터페이스 합의와 SPEC-GENIE-COMPRESSOR-001(후속) 사이의 경계도 확정한다.
> **작성일**: 2026-04-21
> **범위**: `internal/context/` 단일 패키지.

---

## 1. 레포 현재 상태 스캔

```
$ ls /Users/goos/MoAI/AgentOS/internal/
(부재)
```

`internal/context/`는 **Phase 0에서 신규 작성**. 참조 가능한 외부 자산:

- `./claude-code-source-map/` (TypeScript 참조)
- `./hermes-agent-main/agent/context_compressor.py` (Python 참조 — COMPRESSOR-001의 주 출처)

**결론**: 본 SPEC은 zero-to-one 작성. TypeScript 원형의 설계 의도만 계승.

---

## 2. claude-core.md §7 원문 분석 → 본 SPEC 매핑

`.moai/project/research/claude-core.md` §7 SPEC-GENIE-CONTEXT-001 초안은 다음과 같이 요약했다:

> "systemPrompt + userContext + systemContext 로드(memoized), normalize messages, tokenCountWithEstimation, calculateTokenWarningState 기반 auto-compact trigger, compaction 적용(autoCompact/reactiveCompact/snip), task_budget 누적 추적, CompactBoundary yield, snip 경계는 redacted_thinking 보존."

이 요약의 각 요소를 본 SPEC 요구사항으로 분해했다:

| claude-core.md §7 항목 | 본 SPEC REQ/AC | Go 구현 포인트 |
|---|---|---|
| systemPrompt 로드 | (OUT — CLI-001/CONFIG-001이 주입) | `QueryEngineConfig.SystemPrompt` 필드 값만 받음 |
| userContext 로드 (memoized) | REQ-CTX-002, REQ-CTX-006, AC-CTX-002 | `GetUserContext()` + `sync.Once` |
| systemContext 로드 (memoized) | REQ-CTX-001, REQ-CTX-005, REQ-CTX-015, AC-CTX-001 | `GetSystemContext()` + `sync.Once` |
| Normalize messages | §3.1 IN #13, `normalize.go` | consecutive user merge + signature strip |
| tokenCountWithEstimation | REQ-CTX-004, AC-CTX-003 | `TokenCountWithEstimation` 근사 ±5% |
| calculateTokenWarningState | REQ-CTX-011, AC-CTX-004 | 4-level enum |
| auto-compact trigger | REQ-CTX-007, REQ-CTX-011 | 80% 임계 + Red 강제 |
| autoCompact/reactiveCompact/snip | REQ-CTX-008, REQ-CTX-012, AC-CTX-005, AC-CTX-006 | 3 strategy 우선순위 |
| task_budget 누적 추적 | REQ-CTX-010, AC-CTX-007 | Compact는 예산 불변 |
| CompactBoundary yield | §6.2 `CompactBoundary` | QUERY-001이 yield, 본 SPEC은 payload 생성 |
| snip redacted_thinking 보존 | REQ-CTX-003, AC-CTX-006 | auxiliary content로 attach |

claude-core.md §7의 모든 요소가 본 SPEC REQ로 흡수되었다. 누락 없음.

---

## 3. claude-core.md §6 설계 원칙 중 본 SPEC 담당

§2에서 본 SPEC이 직접 담당하는 설계 원칙:

| # | 설계 원칙 | 본 SPEC 구현 |
|---|---|---|
| 5 | Budget tracking cumulative across compactions | REQ-CTX-010: Compact는 예산 불변, 호출자(QUERY-001)가 compact 전후 `TaskBudget.Remaining` 누적 관찰 |
| 10 | Context memoized per session | REQ-CTX-001, REQ-CTX-002: `sync.Once` + `atomic.Pointer`, `Invalidate*`로만 무효화 |

원칙 1-4, 6-9는 QUERY-001 소관. 본 SPEC은 2개 원칙을 완전히 책임진다.

---

## 4. claude-core.md §8 고리스크 영역 중 본 SPEC 담당

- **Budget undercount** (compaction 후 task_budget.remaining 미추적) → REQ-CTX-010이 "compaction 자체는 예산 불변"을 단언. QUERY-001과의 경계가 명확: summarizer가 소비한 토큰은 summarizer 호출 turn에서 계상, compaction은 순수 메시지 재구조화.

나머지 3개(circular async generator, mutable message array, tool permission race)는 QUERY-001 소관.

---

## 5. Hermes Agent context_compressor.py 분석

`./hermes-agent-main/agent/context_compressor.py`는 LLM 요약 기반 compaction의 Python 원형이다. 본 SPEC은 이 모듈의 **caller 인터페이스만** 정의하고, 실제 로직은 COMPRESSOR-001이 계승.

### 5.1 Hermes 원형에서 본 SPEC이 추출한 인터페이스 계약

```python
# Hermes context_compressor.py 요약 (실제 파일 참조 후 정제)
def compress_history(
    messages: List[Message],
    target_token_budget: int,
    protect_head: int = 3,
    protect_tail: int = 5,
) -> CompressedResult:
    """
    Returns:
      - summary: LLM-generated summary message
      - preserved_head: messages[:protect_head]
      - preserved_tail: messages[-protect_tail:]
      - dropped_count: int
    """
```

- `protect_head` / `protect_tail` 파라미터 → 본 SPEC의 `ProtectedHead` / `ProtectedTail` 필드 (§6.2).
- `target_token_budget` → 본 SPEC은 `TokenLimit`와 현재 used 계산으로 유도.
- LLM 요약 호출은 Summarizer 인터페이스로 위임 (REQ-CTX-012).

### 5.2 재사용 vs 재작성

| Hermes 요소 | 본 SPEC 처리 |
|---|---|
| protect_head/tail 상수 전략 | **재사용** — 동일 의미, Go 필드로 |
| LLM summary 호출 | **위임** — COMPRESSOR-001 |
| Message serialization | **재작성** — Python dict → Go struct |
| Tokenizer (Kimi) | **재작성** — 본 SPEC은 근사치 + 향후 tiktoken-go |

---

## 6. Go 이디엄 선택 (상세 근거)

### 6.1 Memoization: `sync.Once` + `atomic.Pointer[T]`

**선택**: `sync.Once`로 최초 1회 초기화, `atomic.Pointer` 로 pointer 저장. `InvalidateSystemContext()`는 새 `sync.Once`를 할당 (atomic swap).

```go
type memoizedSystemContext struct {
    once    sync.Once
    value   atomic.Pointer[SystemContext]
    err     atomic.Pointer[error]
}

var _systemCtx atomic.Pointer[memoizedSystemContext]

func init() {
    _systemCtx.Store(&memoizedSystemContext{})
}
```

**대안 거부**:
- `sync.Mutex + bool + *SystemContext`: 간단하지만 invalidation race 복잡.
- `generics Once[T]` (Go 1.22 없음): 1.22 호환.
- Global map: 세션 분리 안 됨. 본 SPEC은 세션당 1 QueryEngine이므로 engine 인스턴스가 memoized struct 소유가 더 자연 — REFACTOR 단계에서 검토.

### 6.2 Git 명령 실행: `exec.CommandContext` + 2초 timeout

```go
ctx, cancel := context.WithTimeout(parent, 2*time.Second)
defer cancel()
out, err := exec.CommandContext(ctx, "git", "status", "--porcelain").Output()
if err != nil { /* return "(no git)" per REQ-CTX-015 */ }
```

- Git 부재/타임아웃은 graceful (REQ-CTX-015).
- 세 명령 병렬 실행(goroutine + WaitGroup)으로 ≤1초 예산 내 완료 노림.

### 6.3 CLAUDE.md Walk: `filepath.Walk` 거부 → 단순 while loop

- `filepath.Walk`는 재귀 내려가기. 본 SPEC은 walk-up (부모 방향) 필요 → 직접 loop.
- 최대 깊이 30 단계 안전장치(무한 loop 방지).

### 6.4 Token 근사 알고리즘

```go
func TokenCountWithEstimation(messages []message.Message) int64 {
    var total int64
    for _, msg := range messages {
        total += 4 // role/boundary overhead
        for _, block := range msg.Content {
            switch block.Type {
            case "text":
                total += int64(utf8.RuneCountInString(block.Text)) / 4
            case "tool_use":
                total += 12 + int64(len(block.InputJSON))/4
            case "tool_result":
                total += int64(utf8.RuneCountInString(block.Content)) / 4
            case "redacted_thinking":
                total += 8
            }
        }
    }
    return total
}
```

- `utf8.RuneCountInString` (bytes/len이 아니라 rune 단위) → CJK undercount 완화. 여전히 ±5% 가능.
- AC-CTX-003의 fixture는 영문/한글 혼합 기준값 1개 고정. 후속 tiktoken-go 도입 시 교체 가능한 구조.

### 6.5 Snip 전략: redacted_thinking 수집 루프

```go
func (s *snipStrategy) Apply(state loop.State) (loop.State, CompactBoundary) {
    head := state.Messages[:s.head]
    tail := state.Messages[len(state.Messages)-s.tail:]
    middle := state.Messages[s.head : len(state.Messages)-s.tail]

    var preserved []message.ContentBlock
    droppedCount := 0
    for _, m := range middle {
        for _, b := range m.Content {
            if b.Type == "redacted_thinking" {
                preserved = append(preserved, b)
            }
        }
        droppedCount++
    }

    marker := message.Message{
        Role: "system",
        Content: append(
            []message.ContentBlock{
                {Type: "text", Text: fmt.Sprintf("<moai-snip-marker>: dropped %d messages", droppedCount)},
            },
            preserved...,
        ),
    }

    newMessages := append(head, marker)
    newMessages = append(newMessages, tail...)

    return loop.State{...newMessages, TaskBudget: state.TaskBudget}, CompactBoundary{...}
}
```

### 6.6 `atomic.Pointer[T]` Go 1.22+ 호환성

- `atomic.Pointer[T]`는 Go 1.19+ 제공. CORE-001의 Go 1.22+ 베이스라인에서 사용 가능.
- 대안 `atomic.Value` (`interface{}` 기반)은 type-assert 비용 있음.

---

## 7. 외부 의존성 합계

| 모듈 | 버전 | 본 SPEC 사용 | 결정 근거 |
|------|-----|-----------|---------|
| `go.uber.org/zap` | v1.27+ | ✅ 구조화 로깅 | CORE-001 결정 계승 |
| `github.com/stretchr/testify` | v1.9+ | ✅ 테스트 | CORE-001 결정 계승 |
| Go stdlib `context` | 1.22+ | ✅ timeout | 표준 |
| Go stdlib `sync`, `sync/atomic` | 1.22+ | ✅ Once + Pointer | 표준 |
| Go stdlib `os/exec` | 1.22+ | ✅ git 명령 | 표준 |
| Go stdlib `unicode/utf8` | 1.22+ | ✅ 문자 카운팅 | 표준 |

**의도적 미사용** (Phase 0):
- **tiktoken-go**(`github.com/pkoukk/tiktoken-go`): MVP에서는 근사치로 충분. 오픈 이슈 §11-1에서 Phase 1+ 도입 시점 평가.
- **fsnotify**: CLAUDE.md 변경 감지는 본 SPEC 제외 (OUT).
- **go-git** (`github.com/go-git/go-git`): git CLI를 직접 exec로 호출 (Hermes 방식과 동일 — git binary presence 가정). go-git 도입은 바이너리 크기 + 의존성 비용 대비 이득 불명.

---

## 8. 테스트 전략 (TDD RED → GREEN)

### 8.1 Unit 테스트 (20~28개)

**SystemContext** (`system_test.go`):

- `TestGetSystemContext_InvokesGitOnce` (AC-CTX-001)
- `TestGetSystemContext_NoGit_ReturnsGraceful` (REQ-CTX-015)
- `TestGetSystemContext_Truncates4KB` — 대형 git status 절단
- `TestInvalidateSystemContext_ForcesRecompute`
- `TestGetSystemContext_Timeout_ReturnsNoGit` — 2초 타임아웃

**UserContext** (`user_test.go`):

- `TestGetUserContext_WalksUpAndConcat` (AC-CTX-002)
- `TestGetUserContext_RespectsAddDirs`
- `TestGetUserContext_Memoizes`
- `TestGetUserContext_CurrentDateIso8601UTC`
- `TestInvalidateUserContext_ForcesReread` (AC-CTX-010)
- `TestGetUserContext_NoClaudeMd_ReturnsEmptyButSucceeds`

**Tokens** (`tokens_test.go`):

- `TestTokenCountWithEstimation_EmptyMessages_Zero`
- `TestTokenCountWithEstimation_EnglishFixture_Within5Percent`
- `TestTokenCountWithEstimation_KoreanFixture_Within5Percent`
- `TestTokenCountWithEstimation_MixedFixture_Within5Percent`
- `TestTokenCountWithEstimation_ToolBlocks_AddsOverhead`
- `TestTokenCountWithEstimation_RedactedThinking_CountedAs8` — REQ-CTX-003 보존은 별개지만 counting에 포함
- `TestCalculateTokenWarningState_Thresholds` (AC-CTX-004)
- `TestCalculateTokenWarningState_BoundaryValues` — 59999/60000/80000/92000 엣지

**Normalize** (`normalize_test.go`):

- `TestNormalize_MergesConsecutiveUser`
- `TestNormalize_StripsSignatureBlocks`
- `TestNormalize_PreservesRedactedThinking`

**Compactor core** (`compactor_test.go`):

- `TestShouldCompact_UnderThreshold_ReturnsFalse`
- `TestShouldCompact_Over80Percent_ReturnsTrue` (REQ-CTX-007)
- `TestShouldCompact_RedLevel_Overrides` (REQ-CTX-011)
- `TestShouldCompact_MaxMessageCount_ReturnsTrue`

### 8.2 Integration 테스트 (10~14개, build tag `integration`)

- `TestSnip_PreservesProtectedHead` — AC-CTX-006 전반부
- `TestSnip_PreservesProtectedTail`
- `TestSnip_PreservesRedactedThinking_Multiple` — 2개 이상 블록
- `TestSnip_MaintainsMinimumMessages` (REQ-CTX-013)
- `TestAutoCompact_CallsSummarizer` (AC-CTX-005)
- `TestAutoCompact_TaskBudgetPreserved` (AC-CTX-007)
- `TestAutoCompact_NilSummarizer_FallsBackToSnip` (AC-CTX-008)
- `TestAutoCompact_SummarizerError_FallsBackToSnip` (AC-CTX-009)
- `TestReactiveCompact_TriggeredByFlag`
- `TestCompact_ReturnsCompactBoundaryWithAllFields`
- `TestCompact_HistorySnipOnlyFeatureGate` (REQ-CTX-016)
- `TestCompactorContract_MatchesQueryEngineInterface` — QUERY-001 `Compactor` 서명 호환

### 8.3 Property-based 테스트 (옵션, 고가치)

- Property: "모든 compaction 후 `len(messages) >= ProtectedTail + 1`" (REQ-CTX-013)
- Property: "모든 compaction 후 redacted_thinking 개수 ≥ 입력 내 redacted_thinking 개수" (REQ-CTX-003)
- 도구: `gopter` 또는 `rapid` (본 SPEC은 테스트 라이브러리 미결정 — testify로도 generated fixture 가능)

### 8.4 Race detector

`go test -race ./internal/context/...` — `sync.Once` + `atomic.Pointer` 조합이 동시 Invalidate + Get에서 안전한지 검증.

### 8.5 커버리지 목표

- `internal/context/`: 90%+ (전략 분기 모두 커버)
- 특히 `strategy_snip.go`는 95%+ (redacted_thinking 보존이 critical path)
- 전체: 90%+

---

## 9. QUERY-001과의 인터페이스 계약 검증

QUERY-001 §6.2의 `QueryEngineConfig.Compactor` 필드와 본 SPEC `DefaultCompactor`는 다음 서명으로 정합:

```go
// QUERY-001이 요구하는 인터페이스 (internal/query/config.go에 정의)
package query

type Compactor interface {
    ShouldCompact(s loop.State) bool
    Compact(s loop.State) (loop.State, loop.CompactBoundary, error)
}

// CONTEXT-001이 제공하는 구현체 (internal/context/compactor.go)
package context

func (c *DefaultCompactor) ShouldCompact(s loop.State) bool
func (c *DefaultCompactor) Compact(s loop.State) (loop.State, loop.CompactBoundary, error)

var _ query.Compactor = (*DefaultCompactor)(nil)  // 컴파일 시 contract 검증
```

- `loop.State`, `loop.CompactBoundary`는 QUERY-001의 `internal/query/loop/` 패키지 소유.
- 본 SPEC은 `loop` 패키지를 import (역방향 의존 없음).
- `compactor_contract_test.go`가 `var _ query.Compactor = ...` assertion으로 인터페이스 호환성 빌드타임 검증.

**Circular import 방지**: `loop.State`는 `internal/query/loop/`, `Compactor` 인터페이스는 `internal/query/`(상위), `DefaultCompactor` 구현은 `internal/context/`(하위). 의존 방향은 `context → query/loop ← query`로 단방향.

---

## 10. 구현 규모 예상

| 파일 | 신규 LoC | 테스트 LoC |
|---|---|---|
| `system.go` | 80 | 150 |
| `user.go` | 100 | 180 |
| `tool_use.go` | 40 | 60 |
| `tokens.go` | 80 | 200 |
| `compactor.go` | 120 | 250 |
| `strategy_snip.go` | 100 | 250 |
| `strategy_auto.go` | 80 | 180 |
| `strategy_reactive.go` | 60 | 120 |
| `summarizer.go` | 20 | — |
| `boundary.go` | 30 | 40 |
| `normalize.go` | 60 | 120 |
| **합계** | **~770** | **~1,550** |

테스트 비율: 67% (TDD 적합).

---

## 11. 오픈 이슈

1. **Tokenizer 최종 선택**: Phase 0 MVP는 `/4` 근사. Phase 1 ADAPTER-001에서 provider response `usage` 도입 후, Phase 2+에서 `tiktoken-go` 또는 자체 BPE 구현 결정. 본 SPEC의 `TokenCountWithEstimation`은 인터페이스 고정이므로 교체 용이.
2. **Protected window 상수**: head=3, tail=5가 최적인지 미검증. Claude Code TS 값을 계승했으나, 실제 워크로드(LLM 응답 턴 길이, tool_use 비율)에 따라 조정 필요. 초기값 고정, 관찰 후 config로 노출 가능.
3. **`atomic.Pointer` 세션 분리**: 현재 설계는 package-level global memoization. 동일 프로세스에 여러 `QueryEngine` 인스턴스가 있을 경우 (SUBAGENT-001 도입 시) session-local 필요. REFACTOR 단계에서 `QueryEngine` 인스턴스가 context 소유로 전환.
4. **Summarizer 에러 재시도 정책**: REQ-CTX-014는 "즉시 Snip fallback". 대안: 1-2회 재시도 후 fallback. COMPRESSOR-001 도입 후 재검토.
5. **CompactBoundary.TokensBefore/After 계산 비용**: 매 Compact마다 전체 messages 토큰 재계산. 큰 세션에서 느릴 수 있음. 증분 추적 고려 (Phase 1+).
6. **redacted_thinking auxiliary content 포맷**: Anthropic API가 snip marker에 붙은 redacted_thinking 블록을 어떻게 해석하는지 integration test 필요. ADAPTER-001과 연계하여 확인.
7. **HISTORY_SNIP env 파싱**: 본 SPEC은 `DefaultCompactor.HistorySnipOnly` 필드로 받음. CONFIG-001이 env 파싱 후 주입. Phase 0 MVP에서는 테스트 bypass만.

---

## 12. 결론

- **상속 자산**: Claude Code TS 설계 의도 + Hermes Python `context_compressor.py` 알고리즘 아이디어. 직접 포트 없음.
- **핵심 결정**:
  - Memoization: **`sync.Once` + `atomic.Pointer`** (Go 1.19+).
  - Token 근사: **`/4 + overhead` 휴리스틱** (MVP, ±5% 목표).
  - Compaction 3 strategy 우선순위: **AutoCompact > ReactiveCompact > Snip**.
  - Snip 보호: **head=3, tail=5, redacted_thinking 무조건 보존**.
  - Summarizer 미가용 시: **Snip fallback** (항상 성공).
- **QUERY-001과의 인터페이스**: `compactor_contract_test.go`로 빌드타임 검증.
- **리스크**: R2(redacted_thinking 포맷 호환성)이 가장 높음 — ADAPTER-001 integration test로 방어.
- **다음 단계**: 본 SPEC과 QUERY-001이 GREEN 단계에서 같은 컨벤션으로 `loop.State` 접근해야 함. 두 SPEC implementer 간 주간 sync 권장.

본 SPEC이 통과하면 **QUERY-001의 continue site `after_compact`가 실제로 작동하는 데 필요한 모든 인터페이스가 확보된다**. COMPRESSOR-001은 Summarizer 구현체만 추가하면 AutoCompact/ReactiveCompact가 자동 활성화.

---

**End of research.md**
