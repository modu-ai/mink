---
id: SPEC-GOOSE-PROMPT-CACHE-001
version: 0.1.0
status: completed
completed: 2026-04-27
created_at: 2026-04-21
updated_at: 2026-04-27
author: manager-spec
priority: P1
issue_number: null
phase: 1
size: 소(S)
lifecycle: spec-anchored
labels: [phase-1, cache, anthropic, prompt-cache, llm, priority/p1-high]
---

# SPEC-GOOSE-PROMPT-CACHE-001 — Prompt Caching (system_and_3 Strategy, 4 Breakpoint, TTL)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-llm.md §6 + ROADMAP v2.0 Phase 1 기반) | manager-spec |

---

## 1. 개요 (Overview)

Anthropic의 **prompt caching** 기능을 활용하기 위한 caching breakpoint 계획 레이어를 정의한다. Hermes Agent의 `system_and_3` 전략(hermes-llm.md §6)을 Go로 포팅하여, 주어진 message 배열에 최대 4개의 `cache_control: ephemeral` 마커를 삽입하는 `internal/llm/cache` 패키지를 구현한다.

본 SPEC이 통과한 시점에서 `PromptCachePlanner`는:

- `Plan(messages, strategy, ttl)`을 호출하면 마커가 삽입되어야 할 위치의 인덱스 slice(최대 4)를 반환하고,
- `system_and_3` 전략은 (1) system prompt(messages[0])에 1/4, (2) 비-system 메시지 중 마지막 3개에 2~4/4를 순서대로 배치하고,
- TTL은 `5m`(기본) 또는 `1h`(옵션)로 설정 가능하며,
- ADAPTER-001의 Anthropic 어댑터가 이 계획을 소비하여 content block에 `{"cache_control":{"type":"ephemeral","ttl":"1h"}}`를 주입한다.

본 SPEC은 **breakpoint 계획 알고리즘과 제약**만 규정한다. 실제 Anthropic API 호출, content block 직렬화, cache hit 응답 메타 해석은 ADAPTER-001의 Anthropic 어댑터.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- ROADMAP v2.0 Phase 1 row 09는 PROMPT-CACHE-001을 `ROUTER-001` 후속 P1로 배치. ADAPTER-001이 Anthropic 호출 경로에서 이 plan을 소비.
- `.moai/project/research/hermes-llm.md` §6은 Hermes `system_and_3` 전략의 알고리즘과 효과(multi-turn에서 입력 토큰 비용 ~75% 절감)를 제시.
- Anthropic API는 단일 요청에 **최대 4개** `cache_control` breakpoint를 허용. 잘못 배치하면 cache miss로 비용 낭비.
- MINK의 multi-turn agentic loop(QUERY-001의 queryLoop)는 동일 system prompt + tool schema를 매 turn 반복 전송. Caching 없으면 턴마다 동일 내용을 재청구.

### 2.2 상속 자산

- **Hermes Agent Python**: `prompt_cache.py`의 `plan_cache_markers`, `CacheStrategy.system_and_3`.
- **Anthropic prompt caching 문서**: https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching

### 2.3 범위 경계

- **IN**: `CacheStrategy` enum, `BreakpointPlanner.Plan()`, `CacheMarker` 구조체, system_and_3 알고리즘, TTL 선택, message 수에 따른 breakpoint 축소.
- **OUT**: 실제 API 호출, content block 직렬화(ADAPTER-001/Anthropic), cache hit 메트릭 수집(후속 메트릭 SPEC), non-Anthropic provider caching(OpenAI/Gemini의 자체 caching 사용은 별도 SPEC).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/llm/cache/` 패키지: `BreakpointPlanner`, `CacheStrategy`, `CacheMarker`, `Plan`, `TTL`.
2. `CacheStrategy` 열거형 — `SystemOnly`(1/4), `SystemAnd3`(기본, 1+3/4), `None`.
3. `TTL` 열거형 — `TTLDefault`(5분, Anthropic 기본), `TTL1Hour`.
4. `Plan(messages []Message, strategy CacheStrategy, ttl TTL) (*CachePlan, error)`:
   - `CachePlan.Markers` = 삽입 위치 + TTL 정보가 담긴 slice (길이 0~4).
5. `system_and_3` 알고리즘 (hermes-llm.md §6 인용):
   - Marker 1: `messages[0]`이 system role이면 그 위치
   - Marker 2~4: `messages[1..]` 중 마지막 3개의 non-system message 각각의 끝 content block
   - 총 breakpoint 개수는 (system 존재 1) + min(3, non-system 개수)
6. 메시지 수 적을 때 축소: non-system이 2개면 marker는 1+2=3개, 1개면 1+1=2개, 0개면 1개(system만).
7. 동일 위치 중복 삽입 방지: 각 marker의 `(message_index, content_block_index)`가 유일.
8. `CacheMarker`는 계획 정보만 담고 실제 직렬화 형태는 ADAPTER-001이 생성.

### 3.2 OUT OF SCOPE

- **실제 Anthropic API 호출 및 content block 변환**: ADAPTER-001.
- **Cache hit 응답 메타 파싱**(`usage.cache_read_input_tokens` 등): ADAPTER-001 + 메트릭 SPEC.
- **OpenAI/Gemini/기타 provider caching**: 각 provider 별 구현은 ADAPTER-001 또는 후속 SPEC에서 처리.
- **Cache key 계산/중복 제거**: Anthropic 서버 측 책임.
- **TTL 동적 선택**(대화 길이에 따라 5분→1시간 자동): 후속 최적화 SPEC. 본 SPEC은 호출자가 명시.
- **Breakpoint 5개 이상 요청**: Anthropic 제한 준수. 요청 시 에러.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-PC-001 [Ubiquitous]** — The `BreakpointPlanner` **shall** be stateless; concurrent `Plan()` calls **shall** produce identical outputs for identical inputs.

**REQ-PC-002 [Ubiquitous]** — A `CachePlan` **shall** contain at most 4 `CacheMarker` entries (Anthropic limit); `Plan()` **shall** return `ErrBreakpointLimit` if internal logic would produce >4.

**REQ-PC-003 [Ubiquitous]** — Each `CacheMarker.MessageIndex` **shall** refer to a valid index in the input `messages` slice; `ContentBlockIndex` points to the last content block of that message.

### 4.2 Event-Driven

**REQ-PC-004 [Event-Driven]** — **When** `Plan(messages, SystemAnd3, ttl)` is invoked and `messages[0].Role == "system"`, the planner **shall** place marker 1 at `{MessageIndex: 0, ContentBlockIndex: last}`, then place subsequent markers on the last 3 non-system messages from the end.

**REQ-PC-005 [Event-Driven]** — **When** `Plan` is invoked with `strategy == SystemOnly`, the planner **shall** place exactly one marker at the system message (or return empty plan if no system message).

**REQ-PC-006 [Event-Driven]** — **When** `Plan` is invoked with `strategy == None`, the planner **shall** return a plan with `len(Markers) == 0` without error.

### 4.3 State-Driven

**REQ-PC-007 [State-Driven]** — **While** the input `messages` has no system message (i.e., `messages[0].Role != "system"`), the planner using `SystemAnd3` **shall** place markers only on the last 3 non-system messages (plan length 0~3).

**REQ-PC-008 [State-Driven]** — **While** the input `messages` has fewer than 3 non-system messages, the planner **shall** place markers on all available non-system messages (plan length = 1 + n where n is non-system count, capped at 4).

### 4.4 Unwanted Behavior

**REQ-PC-009 [Unwanted]** — The planner **shall not** produce two markers referring to the same `(MessageIndex, ContentBlockIndex)` tuple.

**REQ-PC-010 [Unwanted]** — **If** input `messages` is nil or empty, **then** `Plan()` **shall** return `*CachePlan{Markers: []}` without error (no-op plan).

**REQ-PC-011 [Unwanted]** — The planner **shall not** mutate the input `messages` slice; all reads are non-destructive.

### 4.5 Optional

**REQ-PC-012 [Optional]** — **Where** a message has multiple content blocks (e.g., tool_use + text), the marker **shall** reference the **last** content block index; this maximizes the cached prefix length per Anthropic's documentation.

**REQ-PC-013 [Optional]** — **Where** `CachePlanOptions.MinMessageTokens` is set, messages with estimated tokens below the threshold **shall not** receive a marker; tokens are estimated via the `TokenCounter` interface injected by ADAPTER-001 (if nil, this option is ignored).

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-PC-001 — system_and_3 기본 케이스**
- **Given** messages 6개: `[system, user, assistant, user, assistant, user]`, strategy = SystemAnd3, ttl = TTLDefault
- **When** `Plan(messages, SystemAnd3, TTLDefault)`
- **Then** `plan.Markers`는 정확히 4개, 인덱스 `[0, 3, 4, 5]`, 각 Marker.TTL == "5m"

**AC-PC-002 — non-system 2개만 있을 때**
- **Given** messages `[system, user, assistant]` (non-system 2개)
- **When** `Plan(SystemAnd3, TTLDefault)`
- **Then** `len(plan.Markers) == 3`, 인덱스 `[0, 1, 2]`

**AC-PC-003 — system 없음**
- **Given** messages `[user, assistant, user]` (system 없음)
- **When** `Plan(SystemAnd3, TTL1Hour)`
- **Then** `len(plan.Markers) == 3`, 인덱스 `[0, 1, 2]`, 각 TTL == "1h"

**AC-PC-004 — SystemOnly 전략**
- **Given** messages 6개 (system 포함)
- **When** `Plan(SystemOnly, TTLDefault)`
- **Then** `len(plan.Markers) == 1`, 인덱스 `[0]`

**AC-PC-005 — None 전략**
- **Given** messages 6개
- **When** `Plan(None, TTLDefault)`
- **Then** `len(plan.Markers) == 0`, error 없음

**AC-PC-006 — 빈 messages**
- **Given** messages `[]`
- **When** `Plan(SystemAnd3, TTLDefault)`
- **Then** `len(plan.Markers) == 0`, error 없음

**AC-PC-007 — 다중 content block 메시지**
- **Given** `messages[5]`이 3개 content block 포함 (tool_use + text + text)
- **When** `Plan`
- **Then** 해당 메시지의 marker `ContentBlockIndex == 2` (마지막 block)

**AC-PC-008 — 1시간 TTL**
- **Given** strategy=SystemAnd3, ttl=TTL1Hour
- **When** `Plan`
- **Then** 모든 marker의 `TTL == "1h"`

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/llm/cache/
├── prompt.go          # BreakpointPlanner + Plan() + CacheStrategy
├── prompt_test.go
├── marker.go          # CacheMarker 구조체
├── ttl.go             # TTL enum + String()
└── errors.go          # ErrBreakpointLimit
```

### 6.2 핵심 타입

```go
// internal/llm/cache/prompt.go

type CacheStrategy int

const (
    StrategyNone       CacheStrategy = iota
    StrategySystemOnly
    StrategySystemAnd3
)

type TTL string

const (
    TTLDefault TTL = "5m"  // Anthropic 기본
    TTL1Hour   TTL = "1h"
)

type CacheMarker struct {
    MessageIndex      int
    ContentBlockIndex int
    TTL               TTL
}

type CachePlan struct {
    Strategy CacheStrategy
    Markers  []CacheMarker
}

type PlanOptions struct {
    TokenCounter     TokenCounter // optional (ADAPTER-001 주입)
    MinMessageTokens int           // 이 토큰 이하 메시지에 marker 삽입 생략
}

type TokenCounter interface {
    Count(msg message.Message) int
}

type BreakpointPlanner struct{}

func NewPlanner() *BreakpointPlanner

func (p *BreakpointPlanner) Plan(
    messages []message.Message,
    strategy CacheStrategy,
    ttl TTL,
    opts ...PlanOption,
) (*CachePlan, error)
```

### 6.3 system_and_3 알고리즘

```
Plan(messages, SystemAnd3, ttl):
  plan = CachePlan{Strategy: SystemAnd3, Markers: []}

  // 1. system marker
  if len(messages) > 0 AND messages[0].Role == "system":
    cbIdx = lastContentBlockIndex(messages[0])
    plan.Markers.append(CacheMarker{0, cbIdx, ttl})

  // 2. 마지막 3개 non-system 메시지
  nonSystemIndices = []
  for i := len(messages)-1; i >= 0; i--:
    if messages[i].Role != "system":
      nonSystemIndices.append(i)
    if len(nonSystemIndices) == 3:
      break

  // 복원: 마지막 non-system 3개를 오래된 순으로 정렬
  sort(nonSystemIndices, ASC)

  for idx in nonSystemIndices:
    cbIdx = lastContentBlockIndex(messages[idx])
    // 중복 방지(REQ-PC-009)
    if NOT plan.Markers.contains(idx, cbIdx):
      plan.Markers.append(CacheMarker{idx, cbIdx, ttl})

  // 3. 축소 (REQ-PC-002)
  if len(plan.Markers) > 4:
    return nil, ErrBreakpointLimit  // 이론상 발생 불가능하나 guard

  return plan, nil
```

### 6.4 TTL 결정

본 SPEC은 TTL을 **호출자가 명시**한다. 선택 가이드:

| 상황 | TTL |
|------|-----|
| 5~30분 이내 후속 요청 예상 | `TTLDefault` (5m) |
| 장기 세션(IDE 상주 agent) | `TTL1Hour` |

동적 선택(대화 길이 기반) 확장은 후속 SPEC.

### 6.5 ADAPTER-001 연계 (예시)

```go
// ADAPTER-001/anthropic/adapter.go (본 SPEC이 아닌 후속)

func (a *AnthropicAdapter) BuildRequest(msgs []message.Message) anthropic.Request {
    plan, _ := a.cachePlanner.Plan(msgs, cache.StrategySystemAnd3, cache.TTLDefault)

    apiMsgs := make([]anthropic.Message, len(msgs))
    for i, m := range msgs {
        apiMsgs[i] = a.convertMessage(m)
    }

    // Apply cache markers
    for _, marker := range plan.Markers {
        msg := &apiMsgs[marker.MessageIndex]
        cb := &msg.Content[marker.ContentBlockIndex]
        cb.CacheControl = &anthropic.CacheControl{
            Type: "ephemeral",
            TTL:  string(marker.TTL), // "5m" | "1h" (1h가 API에서 유효한지 확인 필요)
        }
    }
    // ...
}
```

### 6.6 TDD 진입 순서

1. **RED #1**: `TestPlanner_SystemAnd3_FullMessages` — AC-PC-001.
2. **RED #2**: `TestPlanner_SystemAnd3_FewMessages` — AC-PC-002.
3. **RED #3**: `TestPlanner_SystemAnd3_NoSystem` — AC-PC-003.
4. **RED #4**: `TestPlanner_SystemOnly` — AC-PC-004.
5. **RED #5**: `TestPlanner_None` — AC-PC-005.
6. **RED #6**: `TestPlanner_EmptyMessages` — AC-PC-006.
7. **RED #7**: `TestPlanner_MultipleContentBlocks_UsesLastIndex` — AC-PC-007.
8. **RED #8**: `TestPlanner_TTL1Hour_Propagates` — AC-PC-008.
9. **GREEN**: 최소 구현(순수 함수).
10. **REFACTOR**: 인덱스 중복 검사 helper 추출.

### 6.7 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| Tested | 테이블 주도 테스트, 모든 strategy × 메시지 구성 조합 |
| Readable | 순수 함수 스타일, side effect 없음, enum 명시 |
| Unified | go fmt + golangci-lint, strategy enum 일관성 |
| Secured | 메시지 content 읽기만, mutation 없음 |
| Trackable | Plan 결과에 Strategy 필드 포함, ADAPTER-001이 trace 로그로 연결 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `message.Message` 타입 정의 |
| 선행 SPEC | SPEC-GOOSE-ROUTER-001 | ADAPTER-001이 Route에 따라 caching 적용 여부 결정 |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-001 | Anthropic 어댑터가 CachePlan을 소비 |
| 외부 | Go 1.22+ | |
| 외부 | `github.com/stretchr/testify` | |

**라이브러리 결정**: 순수 함수 구현, **외부 라이브러리 추가 없음**.

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Anthropic이 breakpoint 한계를 5로 늘리거나 TTL 형식 변경 | 중 | 낮 | 본 SPEC은 max 4 + TTL enum으로 유연. 변경 시 enum 추가만 |
| R2 | system_and_3이 짧은 대화에서 over-caching (비용 낭비) | 낮 | 낮 | `MinMessageTokens` 옵션(REQ-PC-013)으로 작은 메시지 제외 가능 |
| R3 | 연속 turn에서 동일 message index에 marker 중복 | 중 | 중 | REQ-PC-009 및 테스트 AC-PC-001로 보장 |
| R4 | ADAPTER-001이 구현되기 전까지 본 SPEC 검증 지연 | 중 | 낮 | 본 SPEC은 순수 결정 로직이므로 stub test만으로 검증 가능 |
| R5 | Non-Anthropic provider에 잘못 적용 | 중 | 중 | ADAPTER-001이 provider == "anthropic" 분기에서만 Plan 호출. 본 SPEC은 provider agnostic |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/hermes-llm.md` §6 Prompt Caching, §9 Go 포팅 매핑, §10 SPEC 도출
- `.moai/specs/ROADMAP.md` §4 Phase 1 row 09
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` — adapter가 provider 분기
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` — `message.Message` 타입

### 9.2 외부 참조

- **Anthropic Prompt Caching**: https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching
- **Hermes Agent Python**: `./hermes-agent-main/agent/prompt_cache.py`
- **Anthropic 1h TTL beta**: https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching#cache-ttl

### 9.3 부속 문서

- `./research.md` — 알고리즘 edge case, TTL 선택 가이드, 테스트 fixture

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **실제 Anthropic API 호출을 구현하지 않는다**. ADAPTER-001.
- 본 SPEC은 **content block 직렬화(JSON)를 포함하지 않는다**. ADAPTER-001이 provider별 SDK로 변환.
- 본 SPEC은 **cache hit 메타 파싱을 포함하지 않는다**(`usage.cache_read_input_tokens`). 메트릭 SPEC.
- 본 SPEC은 **OpenAI/Gemini/기타 provider의 자체 caching을 지원하지 않는다**. Anthropic 전용 계획.
- 본 SPEC은 **TTL 동적 선택을 포함하지 않는다**(대화 길이/빈도 기반). 후속 최적화.
- 본 SPEC은 **cache key 계산을 포함하지 않는다**. Anthropic 서버 측 책임.
- 본 SPEC은 **gRPC 또는 CLI 설정 인터페이스를 포함하지 않는다**. CONFIG-001의 `LLMConfig.Caching` 섹션은 호출자(ADAPTER-001) 소비.

---

**End of SPEC-GOOSE-PROMPT-CACHE-001**
