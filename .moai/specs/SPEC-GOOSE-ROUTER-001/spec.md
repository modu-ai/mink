---
id: SPEC-GOOSE-ROUTER-001
version: 1.0.0
status: completed
completed: 2026-04-27
created_at: 2026-04-21
updated_at: 2026-04-27
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 중(M)
lifecycle: spec-anchored
labels: [routing, llm, infrastructure, phase-1]
---

# SPEC-GOOSE-ROUTER-001 — Smart Model Routing + Provider Registry

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-llm.md §4 + ROADMAP v2.0 Phase 1 기반) | manager-spec |
| 1.0.0 | 2026-04-25 | 구현 완료 반영 (commit 103803b, 16/16 REQ + 8/8 AC, coverage 97.2%, race-clean). 감사 리포트 `ROUTER-001-audit.md` 결함 수정: frontmatter `labels` 추가, status `planned`→`implemented`, AC-009~014 추가(REQ-001/012/013/014/015/016 직접 매핑), AC-008 semantic clarification(New() 시점 fail-fast 허용), §6.2 AuthType `"none"` 추가, §3.2/§6.7 구현 드리프트(D-IMPL-1~7) 명시. | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE의 **라우팅 결정 레이어**를 정의한다. Hermes Agent의 `model_router.py` smart routing heuristic(hermes-llm.md §4)을 Go로 포팅하여, 사용자 메시지의 단순성을 판정하고 primary 모델과 cheap 모델 사이의 전환을 결정하는 `internal/llm/router` 패키지를 구현한다.

본 SPEC이 통과한 시점에서 `Router`는:

- `Route(ctx, RoutingRequest)`를 호출하면 `(Route, error)`를 반환하되, `Route`에는 `{model, provider, base_url, routing_reason, signature}`가 담긴다.
- 판정 입력: 마지막 user 메시지의 길이(chars), 단어 수, 개행 수, 코드 블록 존재, URL 포함, 복잡 키워드(debug/implement/refactor/test/analyze/design/architecture/terminal/docker 등).
- 단순 판정 조건을 **모두** 충족 시 `cheap_model_route` 반환(conservative by design — 하나라도 실패하면 primary 유지).
- Provider Registry를 통해 각 provider의 지원 capability(streaming/tools/vision/embed)·권장 모델·base_url을 중앙 관리.
- `CredentialPool`과 협력: routing 결정 후 `CREDPOOL`에서 자격 취득하여 실제 호출은 ADAPTER-001이 담당.

본 SPEC은 **라우팅 결정 규칙 + Provider Registry 스키마**만 규정한다. 실제 LLM 호출, 스트리밍 변환, tool schema 매핑은 ADAPTER-001. 캐시 마커 주입은 PROMPT-CACHE-001.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- ROADMAP v2.0 Phase 1 row 07은 ROUTER-001을 `CREDPOOL-001` 이후에 배치한다. ADAPTER-001과 PROMPT-CACHE-001은 모두 "Router가 결정한 provider/model로 호출"이라는 전제에 의존.
- `.moai/project/research/hermes-llm.md` §4는 Hermes의 `choose_cheap_model_route` 알고리즘과 conservative design 원칙을 Go 포팅 매핑(§9)과 함께 제시.
- v4.0 비용 최적화 목표(§7 "LLM Router 레이턴시 < 10ms")의 주 달성 수단이 본 라우팅 결정. primary 모델(예: Claude Opus)과 cheap 모델(예: Claude Haiku 또는 Ollama local)의 스위칭으로 30-70% 비용 절감 가능.

### 2.2 상속 자산

- **Hermes Agent Python**: `model_router.py`의 `choose_cheap_model_route`, `_COMPLEX_KEYWORDS` set, route signature 생성.
- **MoAI-ADK-Go**: 본 레포에 미러 없음.

### 2.3 범위 경계

- **IN**: 단순성 판정 heuristic(6 기준), Route 결정, Provider Registry(15+ provider 메타데이터), route signature 생성, primary↔cheap 전환 로직, RoutingConfig 스키마.
- **OUT**: 실제 LLM HTTP 호출(ADAPTER-001), credential 취득(CREDPOOL-001), rate limit 추적(RATELIMIT-001), cache marker 주입(PROMPT-CACHE-001), 비용 추적/pricing(후속 메트릭 SPEC), 학습 기반 라우팅(Phase 4 INSIGHTS-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/llm/router/` 패키지: `Router`, `Route`, `RoutingRequest`, `RoutingConfig`, `SimpleClassifier`.
2. 단순성 판정 6 기준 (hermes-llm.md §4 인용):
   - `char_count <= 160`
   - `word_count <= 28`
   - `newline_count <= 2`
   - `NOT has_code_block(msg)`
   - `NOT has_url(msg)`
   - `NOT has_complex_keyword(msg)` (키워드 set §6.3)
3. `choose_cheap_model_route` 로직: 6 기준 **ALL true** → cheap, 하나라도 false → primary.
4. `internal/llm/router/registry.go`: `ProviderRegistry` — 15+ provider 메타데이터(capability, default_model, base_url, auth_type).
5. `Route.Signature`: `(model, provider, base_url, mode, command, args)` 튜플을 canonical string으로 직렬화(트레이싱·캐싱용).
6. `RoutingRequest` 입력: 메시지 배열 + 컨텍스트(`conversation_length`, `has_prior_tool_use` 등).
7. `RoutingConfig`: primary/cheap 모델 및 provider 지정, 키워드 set override, 판정 기준 threshold 조정.
8. Hook 포인트: `RoutingDecisionHook` — 라우팅 결정 후 호출(Phase 4 INSIGHTS에서 사용).
9. `Router`는 **순수 함수에 가까운 상태 없는 객체** (내부 상태 없음, Thread-safe by design).

### 3.2 OUT OF SCOPE

- **실제 LLM HTTP 호출**: ADAPTER-001.
- **Credential 선택·취득**: CREDPOOL-001. Router는 `Route.Provider`를 반환만.
- **Rate limit 고려한 동적 라우팅**: RATELIMIT-001 상태를 Router가 참조할지는 후속 SPEC 결정. Phase 1은 **정적 heuristic만**.
- **학습 기반 라우팅**: Phase 4 INSIGHTS-001에서 사용자 패턴 기반 라우팅 확장 가능.
- **Prompt caching TTL 결정**: PROMPT-CACHE-001.
- **Tool schema 변환**: ADAPTER-001 (provider별).
- **Streaming delta 변환**: ADAPTER-001.
- **Fallback model chain 실행**: QueryEngine(QUERY-001)의 `FallbackModels`와 연계. Router는 chain 자체를 결정하지 않고 primary/cheap 단일 결정만.

### 3.3 Registry Co-ownership Note (SPEC-002 연계)

본 SPEC이 정의한 `ProviderRegistry`(§6, registry.go)는 SPEC-GOOSE-ADAPTER-002에서 **adapter 구현 진행에 따라 공동 소유**된다. 구체적으로:

- SPEC-ROUTER-001 정의: 메타데이터 스키마(`ProviderMeta`), DefaultRegistry 구조, Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama 6개 provider의 `AdapterReady=true` 등록.
- SPEC-ADAPTER-002 확장: OpenRouter, Nous, Mistral, Groq, Qwen, Kimi, GLM, MiniMax 등 추가 9개 provider의 `AdapterReady`를 `false → true`로 전환. 코드 주석 `// SPEC-002 M{N} 구현 완료`로 출처 표기.
- Bonus metadata provider (`cohere`): SPEC-ADAPTER-002 M4에서 추가, ROUTER-001 research.md §3.2 metadata 목록에는 부재하나 "15+ providers" 요구(REQ-ROUTER-003)를 초과 달성하는 범위 내 허용.

ROUTER-001의 **행동 계약(REQ/AC)은 DefaultRegistry 구성에 비의존적**이다 — Registry에 추가 provider가 등록되어도 Router 결정 로직은 그대로 유지되며, AC-008 등 "미등록 provider 거부" 계약도 영향받지 않는다.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-ROUTER-001 [Ubiquitous]** — The `Router` **shall** be stateless; concurrent `Route()` invocations from multiple goroutines **shall** produce identical outputs for identical inputs.

**REQ-ROUTER-002 [Ubiquitous]** — Every `Route` returned **shall** contain a non-empty `Signature` that uniquely identifies the routing decision (model + provider + base_url + mode + command tuple) as a stable canonical string.

**REQ-ROUTER-003 [Ubiquitous]** — The `ProviderRegistry` **shall** enumerate at least the following providers with complete metadata: Anthropic, OpenAI, Google Gemini, xAI, DeepSeek, Ollama (Phase 1 ADAPTER-001 scope); additional providers (OpenRouter, Nous, Mistral, Groq, Qwen, Kimi, GLM, MiniMax) **may** be registered as metadata-only (no adapter yet).

**REQ-ROUTER-004 [Ubiquitous]** — Routing decisions **shall not** mutate the input `RoutingRequest`; Router **shall** treat inputs as immutable.

### 4.2 Event-Driven (이벤트 기반)

**REQ-ROUTER-005 [Event-Driven]** — **When** `Route(ctx, req)` is invoked, the Router **shall** (a) extract the last user message from `req.Messages`, (b) run the `SimpleClassifier.Classify(msg)` heuristic, (c) if classified as simple AND `RoutingConfig.CheapRoute != nil` → return the cheap route, (d) otherwise return the primary route, (e) populate `Route.RoutingReason = "simple_turn" | "complex_task"`, (f) compute and set `Route.Signature`.

**REQ-ROUTER-006 [Event-Driven]** — **When** the input message contains a fenced code block (``` or ~~~), the classifier **shall** set `has_code_block = true`, causing cheap route rejection regardless of other criteria.

**REQ-ROUTER-007 [Event-Driven]** — **When** the input message contains a URL matched by `https?://\S+`, the classifier **shall** set `has_url = true`, causing cheap route rejection.

**REQ-ROUTER-008 [Event-Driven]** — **When** any keyword from `RoutingConfig.ComplexKeywords` (default set §6.3) appears as a whole word (case-insensitive, word boundary) in the message, the classifier **shall** set `has_complex_keyword = true`, causing cheap route rejection.

### 4.3 State-Driven (상태 기반)

**REQ-ROUTER-009 [State-Driven]** — **While** `RoutingConfig.ForceMode` is `"primary"`, the Router **shall** always return the primary route regardless of classifier output; **while** `ForceMode == "cheap"`, Router **shall** return cheap (if defined) or error `ErrCheapRouteUndefined`.

**REQ-ROUTER-010 [State-Driven]** — **While** `RoutingConfig.CheapRoute == nil`, the Router **shall** always return the primary route, even when classifier reports simple_turn; `Route.RoutingReason` **shall** be `"primary_only_configured"` in this case.

### 4.4 Unwanted Behavior (방지)

**REQ-ROUTER-011 [Unwanted]** — **If** the primary provider is not registered in `ProviderRegistry`, **then** `Route()` **shall** return `ErrProviderNotRegistered{name: ...}` without attempting to construct a `Route`.

**REQ-ROUTER-012 [Unwanted]** — The Router **shall not** perform any network I/O or credential access during `Route()`; all routing decisions are based solely on input message + RoutingConfig + Registry metadata.

**REQ-ROUTER-013 [Unwanted]** — The classifier **shall not** produce a `simple_turn` decision for messages containing **multi-line** code (>= 2 consecutive lines with leading whitespace or ```) even if below the character/word thresholds.

**REQ-ROUTER-014 [Unwanted]** — The signature generation **shall not** include credential identifiers, user-identifying tokens, or timestamps; only provider/model/base_url/mode/command tuple. Reproducible signatures for identical route inputs.

### 4.5 Optional (선택적)

**REQ-ROUTER-015 [Optional]** — **Where** `RoutingConfig.RoutingDecisionHooks` is non-empty, each hook **shall** receive `(RoutingRequest, Route)` after decision is computed; hooks **shall not** modify the Route (observational only).

**REQ-ROUTER-016 [Optional]** — **Where** `RoutingConfig.CustomClassifier` is set, the Router **shall** use it in place of the default heuristic; custom classifiers return `ClassifierResult{IsSimple bool, Reasons []string}`.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-ROUTER-001 — 단순 메시지는 cheap route**
- **Given** primary `{model:"claude-opus", provider:"anthropic"}`, cheap `{model:"claude-haiku", provider:"anthropic"}`, 메시지 `"안녕하세요, 오늘 날씨 어때요?"` (단어 6, 문자 25, 코드/URL/복잡키워드 없음)
- **When** `Route(ctx, req)`
- **Then** `route.Model == "claude-haiku"`, `route.RoutingReason == "simple_turn"`, `route.Signature` non-empty

**AC-ROUTER-002 — 복잡 키워드 포함 시 primary**
- **Given** 동일 설정, 메시지 `"debug this function please"` (단어 4, 문자 25, 복잡키워드 "debug" 포함)
- **When** `Route`
- **Then** `route.Model == "claude-opus"`, `route.RoutingReason == "complex_task"`

**AC-ROUTER-003 — 코드 블록 포함 시 primary**
- **Given** 메시지 `"fix this\n```go\nfunc main(){}\n```"`
- **When** `Route`
- **Then** `route.Model == "claude-opus"`, `classifier.Reasons`에 "has_code_block" 포함

**AC-ROUTER-004 — URL 포함 시 primary**
- **Given** 메시지 `"check https://example.com for details"`
- **When** `Route`
- **Then** `route.Model == "claude-opus"`, reason에 "has_url"

**AC-ROUTER-005 — 길이 초과 시 primary**
- **Given** 메시지 `strings.Repeat("a", 200)` (문자 200 > 160)
- **When** `Route`
- **Then** primary route, reason에 "exceeds_char_limit"

**AC-ROUTER-006 — Cheap route 미정의 시 primary + reason 기록**
- **Given** `RoutingConfig.CheapRoute == nil`, 단순 메시지
- **When** `Route`
- **Then** primary route, `route.RoutingReason == "primary_only_configured"`

**AC-ROUTER-007 — Route signature 재현성**
- **Given** 동일 `RoutingRequest` 두 번 입력
- **When** 각각 `Route` 호출
- **Then** `route1.Signature == route2.Signature` (동일 문자열)

**AC-ROUTER-008 — 미등록 provider 거부**
- **Given** primary provider `"nonexistent_provider"`
- **When** `New(cfg, registry, logger)` 또는 `Route(ctx, req)` 호출
- **Then** `ErrProviderNotRegistered{name:"nonexistent_provider"}` 반환
- **Note (구현 정책)**: 본 계약은 **fail-fast at `New()`** 또는 **deferred fail at `Route()`** 중 어느 구현을 채택하든 만족된다. 현재 구현(commit 103803b)은 `New()` 시점에서 primary/cheap provider를 registry 조회하여 즉시 거부(`router.go:L75`) — 더 조기에 오류를 드러내는 엄격한 정책. 기존에 문구가 `Route` 반환으로 좁게 해석될 수 있던 부분을 본 clarification으로 해소.

**AC-ROUTER-009 — 상태 없음 / 동시성 안전성 (REQ-ROUTER-001 매핑)**
- **Given** 단일 `Router` 인스턴스와 동일 `RoutingRequest` 입력
- **When** 100개 goroutine이 동시에 `Route(ctx, req)` 호출
- **Then** 모든 goroutine이 반환한 `Route` 값(특히 `Model`, `Provider`, `RoutingReason`, `Signature`)이 **완전히 동일**하며, `-race` 플래그로 테스트 실행 시 데이터 레이스 미검출
- **REQ 매핑**: REQ-ROUTER-001
- **테스트 방식**: table-driven concurrent test + `go test -race` 필수

**AC-ROUTER-010 — Router는 네트워크 I/O를 수행하지 않음 (REQ-ROUTER-012 매핑)**
- **Given** `Router` 패키지 소스 트리 전체
- **When** 정적 import 분석
- **Then** `net/http`, `net`, `golang.org/x/net`, provider SDK(HTTP 클라이언트) 등 네트워크 I/O 패키지가 import되지 **않음**; 또한 `Route()` 경로에서 파일시스템 쓰기도 없음
- **REQ 매핑**: REQ-ROUTER-012
- **테스트 방식**: 구조적 검증(import list assertion) 또는 코드 리뷰 체크리스트

**AC-ROUTER-011 — 다중 라인 들여쓰기 코드는 복잡으로 분류 (REQ-ROUTER-013 매핑)**
- **Given** 메시지 `"quick question\n    def foo():\n        return 1\n"` (문자/단어 threshold 이하, 펜스 코드 블록 없음, 2줄 이상 선행 공백을 가진 라인)
- **When** `SimpleClassifier.Classify(msg)`
- **Then** `ClassifierResult.IsSimple == false`, `Reasons`에 "has_code_block" 또는 indented-code 상당 사유 포함
- **REQ 매핑**: REQ-ROUTER-013
- **테스트 방식**: `classifier_test.go`의 indented-code 사례 (예: `TestClassifier_IndentedMultilineCode_ClassifiesComplex`)

**AC-ROUTER-012 — Signature에 PII·시간·자격 불포함 (REQ-ROUTER-014 매핑)**
- **Given** 동일 `RoutingRequest`로 1초 간격을 두고 두 번 `Route` 호출
- **When** 각 반환된 `route1.Signature`, `route2.Signature` 비교
- **Then** `route1.Signature == route2.Signature`; 또한 signature 문자열에 대해 regex로 타임스탬프 패턴(ISO-8601, Unix epoch 10/13자리), 이메일, API key prefix(`sk-`, `hf_` 등) 스캔 시 매치 없음
- **REQ 매핑**: REQ-ROUTER-014
- **테스트 방식**: signature_test.go의 timestamp/PII 스캔 테스트

**AC-ROUTER-013 — Hook은 결정 후에만 호출 (REQ-ROUTER-015 매핑)**
- **Given** `RoutingConfig.RoutingDecisionHooks = [hookA, hookB]` (각 hook이 호출 순서·인자를 기록)
- **When** `Route(ctx, req)` 성공 반환
- **Then** (a) 두 hook 모두 정확히 1회 호출, (b) 호출 시점이 Route 구성 **완료 후**(즉 hook이 받은 `*Route`의 `Signature` 필드가 이미 채워져 있음), (c) hook 호출 순서는 등록 순서와 일치
- **REQ 매핑**: REQ-ROUTER-015
- **Note**: 본 AC는 hook 관찰 계약을 검증하되, 포인터 전달로 인한 구조적 mutation 가능성은 코드 리뷰·문서 계약에 위임(§6.7 D-IMPL 항목 참조).

**AC-ROUTER-014 — CustomClassifier 대체 가능 (REQ-ROUTER-016 매핑)**
- **Given** `RoutingConfig.CustomClassifier = fakeCls`이고 `fakeCls.Classify(msg)`는 입력 무관하게 `{IsSimple: true, Reasons: ["custom_always_simple"]}` 반환
- **When** 복잡 키워드 `"debug"`를 포함한 메시지로 `Route` 호출 (기본 classifier라면 complex로 판정했을 입력)
- **Then** `route.Model == cheap.Model` (cheap 경로 채택), `route.ClassifierReasons == ["custom_always_simple"]`
- **REQ 매핑**: REQ-ROUTER-016
- **테스트 방식**: `router_test.go`의 `TestRouter_CustomClassifier_Overrides` 상당 테스트

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/llm/router/
├── router.go            # Router 구조체 + Route()
├── router_test.go
├── classifier.go        # SimpleClassifier + heuristic 6 기준
├── classifier_test.go
├── registry.go          # ProviderRegistry + 15+ provider 등록
├── registry_test.go
├── config.go            # RoutingConfig + ForceMode enum
├── signature.go         # Signature canonical serialization
├── signature_test.go    # signature reproducibility + PII/timestamp 스캔
└── errors.go            # ErrProviderNotRegistered + ErrCheapRouteUndefined
```

### 6.2 핵심 타입 (Go 시그니처 제안)

```go
// internal/llm/router/router.go

type Router struct {
    cfg      RoutingConfig
    registry *ProviderRegistry
    cls      Classifier
    logger   *zap.Logger
}

type RoutingRequest struct {
    // 직전 user 메시지(들). Router는 classifier에 마지막 user role 메시지만 전달.
    Messages             []message.Message
    ConversationLength   int
    HasPriorToolUse      bool
    // Optional context
    Meta                 map[string]any
}

type Route struct {
    Model         string
    Provider      string
    BaseURL       string
    Mode          string // "chat" | "completion" | "embed"
    Command       string // provider-specific command (e.g., "messages.create")
    Args          map[string]any
    RoutingReason string // "simple_turn" | "complex_task" | "primary_only_configured" | "forced:primary" | "forced:cheap"
    Signature     string // canonical tuple string
    ClassifierReasons []string // classifier가 제공한 근거 (observability)
}

func New(cfg RoutingConfig, registry *ProviderRegistry, logger *zap.Logger) (*Router, error)

// Route는 순수 결정; 네트워크/자격 접근 없음.
func (r *Router) Route(ctx context.Context, req RoutingRequest) (*Route, error)
```

```go
// internal/llm/router/classifier.go

type Classifier interface {
    Classify(msg string) ClassifierResult
}

type ClassifierResult struct {
    IsSimple bool
    Reasons  []string // "exceeds_char_limit" | "has_code_block" | ...
}

// SimpleClassifier는 Hermes의 choose_cheap_model_route 로직.
type SimpleClassifier struct {
    MaxChars         int      // 기본 160
    MaxWords         int      // 기본 28
    MaxNewlines      int      // 기본 2
    ComplexKeywords  map[string]struct{} // lowercase set
}

func (c *SimpleClassifier) Classify(msg string) ClassifierResult
```

```go
// internal/llm/router/config.go

type ForceMode string

const (
    ForceModeAuto    ForceMode = "auto"
    ForceModePrimary ForceMode = "primary"
    ForceModeCheap   ForceMode = "cheap"
)

type RouteDefinition struct {
    Model    string
    Provider string
    BaseURL  string            // optional; registry에서 default 조회
    Mode     string            // 기본 "chat"
    Command  string            // 기본 "messages.create"
    Args     map[string]any    // model-specific
}

type RoutingConfig struct {
    Primary              RouteDefinition
    CheapRoute           *RouteDefinition // nullable
    ForceMode            ForceMode
    MaxChars             int
    MaxWords             int
    MaxNewlines          int
    ComplexKeywords      []string // lowercase 허용, 내부에서 set으로 정규화
    CustomClassifier     Classifier // optional
    RoutingDecisionHooks []RoutingDecisionHook
}

type RoutingDecisionHook func(req RoutingRequest, route *Route)
```

```go
// internal/llm/router/registry.go

type ProviderMeta struct {
    Name            string
    DisplayName     string
    DefaultBaseURL  string
    AuthType        string // "oauth" | "api_key" | "none" (e.g., local Ollama)
    SupportsStream  bool
    SupportsTools   bool
    SupportsVision  bool
    SupportsEmbed   bool
    AdapterReady    bool // Phase 1 ADAPTER-001이 구현했는지
    SuggestedModels []string
}

type ProviderRegistry struct {
    providers map[string]*ProviderMeta
}

func NewRegistry() *ProviderRegistry
func (r *ProviderRegistry) Register(meta *ProviderMeta) error
func (r *ProviderRegistry) Get(name string) (*ProviderMeta, bool)
func (r *ProviderRegistry) List() []*ProviderMeta

// DefaultRegistry는 Phase 1에서 15+ provider 메타데이터 사전 등록.
func DefaultRegistry() *ProviderRegistry
```

### 6.3 기본 복잡 키워드 set (Hermes 원형 인용)

hermes-llm.md §4 인용:

```
_COMPLEX_KEYWORDS = {
    "debug", "implement", "refactor", "test", "analyze",
    "design", "architecture", "terminal", "docker", ...
}
```

본 SPEC 기본값(확장):
```go
var defaultComplexKeywords = []string{
    // 코드/개발
    "debug", "implement", "refactor", "test", "analyze",
    "design", "architecture", "fix", "build", "compile",
    // 인프라
    "terminal", "docker", "kubernetes", "deploy", "install",
    // 파일/검색
    "grep", "search", "find", "read", "write", "edit",
    // 데이터
    "query", "migrate", "schema",
}
```

RoutingConfig에서 override 가능(REQ-ROUTER-008).

### 6.4 Signature 직렬화

```go
// signature.go

func makeSignature(r *Route) string {
    // canonical: "model|provider|base_url|mode|command|args_hash"
    argsHash := sha256sum(canonicalJSON(r.Args))[:12]
    return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
        r.Model, r.Provider, r.BaseURL, r.Mode, r.Command, argsHash)
}
```

REQ-ROUTER-014 준수: 시간/사용자 식별자 불포함.

### 6.5 알고리즘 의사코드

```
Route(ctx, req):
  if cfg.ForceMode == "primary":
    return buildRoute(cfg.Primary, "forced:primary")
  if cfg.ForceMode == "cheap":
    if cfg.CheapRoute == nil: return nil, ErrCheapRouteUndefined
    return buildRoute(*cfg.CheapRoute, "forced:cheap")

  lastUser = findLastUserMessage(req.Messages)
  if lastUser == nil:
    return buildRoute(cfg.Primary, "no_user_message")

  result = classifier.Classify(lastUser.Text)

  if result.IsSimple AND cfg.CheapRoute != nil:
    route = buildRoute(*cfg.CheapRoute, "simple_turn")
  elif cfg.CheapRoute == nil:
    route = buildRoute(cfg.Primary, "primary_only_configured")
  else:
    route = buildRoute(cfg.Primary, "complex_task")

  route.ClassifierReasons = result.Reasons
  route.Signature = makeSignature(route)

  for hook in cfg.RoutingDecisionHooks:
    hook(req, route)  # observational

  return route, nil
```

### 6.6 TDD 진입 순서

1. **RED #1**: `TestClassifier_SimpleGreeting_ClassifiesSimple` — AC-ROUTER-001의 classifier 부분.
2. **RED #2**: `TestClassifier_ComplexKeyword_ClassifiesComplex` — AC-ROUTER-002.
3. **RED #3**: `TestClassifier_CodeBlock_ClassifiesComplex` — AC-ROUTER-003.
4. **RED #4**: `TestClassifier_URL_ClassifiesComplex` — AC-ROUTER-004.
5. **RED #5**: `TestClassifier_LongMessage_ClassifiesComplex` — AC-ROUTER-005.
6. **RED #6**: `TestRouter_CheapRouteNil_FallsBackToPrimary` — AC-ROUTER-006.
7. **RED #7**: `TestRouter_Signature_Reproducible` — AC-ROUTER-007.
8. **RED #8**: `TestRouter_UnregisteredProvider_ReturnsError` — AC-ROUTER-008.
9. **GREEN**: 최소 구현 (classifier + registry + router).
10. **REFACTOR**: 키워드 set을 embedded data로 분리, signature 헬퍼 정리.

### 6.7 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| Tested | 테이블 주도 테스트, 6 기준 × boundary cases 48개 classifier test |
| Readable | `Classifier` interface 분리, `ClassifierResult.Reasons` observability |
| Unified | go fmt + golangci-lint, signature canonical format 고정 |
| Secured | REQ-ROUTER-014 signature에 PII/비밀값 미포함 |
| Trackable | 모든 Route 결정에 zap 로그(`{provider, model, reason, signature_prefix}`), `RoutingDecisionHook`로 분석 SPEC(INSIGHTS-001) 연계 준비 |

### 6.8 구현 드리프트 기록 (Audit 2026-04-25, commit 103803b)

SPEC과 구현 간 **minor 드리프트** 목록. 모두 REQ/AC 행동 계약을 위배하지 않는 범위의 설계 선택이며, 향후 유지보수자의 혼란을 줄이기 위해 여기에 기록한다. 감사 리포트 출처: `.moai/reports/plan-audit/mass-20260425/ROUTER-001-audit.md` (Part B §B-5).

| ID | 드리프트 지점 | 설명 | SPEC 측 방침 |
|----|-------------|------|-----------|
| D-IMPL-1 | `router.go:L181` `Args: def.Args` | `Route.Args`가 `RoutingConfig.Primary/CheapRoute.Args` map reference와 공유 — downstream consumer가 실수로 mutate할 경우 config가 오염될 여지. | `Route.Args`는 **read-only 계약**(GoDoc 주석 권장). 필요 시 defensive shallow-copy로 보강. REQ-ROUTER-004 직접 위배는 아님 — "Router가 input을 mutate하지 않음"을 다루므로. |
| D-IMPL-2 | `router.go:L190–L194` `callHooks` | `*Route` 포인터로 hook에 전달 — Go 타입 시스템으로 observational 계약을 강제 불가. | REQ-ROUTER-015는 hook이 "observational only"임을 계약으로 명시. mutation 여부는 hook 작성자 책임. 향후 필요 시 defensive copy 또는 `RoutingDecisionHook func(req RoutingRequest, route Route)` (값 수신)로 계약 강화 가능. |
| D-IMPL-3 | `router.go:L197–L204` `logDecision` signature_prefix | `route.Signature[:min(len, 12)]` 슬라이스로 관찰성 prefix 구성 — 의미 있는 파싱이 아니라 첫 12바이트. | cosmetic; `strings.SplitN(route.Signature, "|", 2)[0]` (모델 이름)으로 교체 시 가독성 향상. 비 계약 영역. |
| D-IMPL-4 | `registry.go:L223–L344` `AdapterReady=true` 확장 | SPEC-002 M1~M5에서 9개 추가 provider를 `AdapterReady=true`로 승격. ROUTER-001 research.md §3.2는 metadata-only(`AdapterReady=false`)로 명시. | §3.3(본 SPEC) Registry Co-ownership Note로 정식화. REQ-ROUTER-003은 "15+ provider 메타데이터" 최소 조건만 요구하므로 확장은 허용. |
| D-IMPL-5 | `registry.go:L183` `cohere` | research.md §3.2 metadata 목록에 없는 bonus provider. | §3.3 참조; "15+" 요구 초과 달성 범위 내 허용. |
| D-IMPL-6 | (해결) `Router.cls` 필드명 | 감사 중 의심되었으나 SPEC §6.2와 구현 모두 `cls Classifier`로 일치 확인됨. | no action. |
| D-IMPL-7 | AC-008 검출 시점 | SPEC AC-008의 문구가 `Route()` 반환으로 읽힐 수 있으나 구현은 `New()` 시점에 fail-fast. | §5 AC-ROUTER-008 Note로 clarification 완료 — `New()` 또는 `Route()` 어느 시점 검출이든 계약 만족. |

**종합**: 위 7건 모두 행동 계약(REQ/AC) 위반 없음. D-IMPL-1/2는 **추가 안전 강화 기회**로 향후 SPEC 개정 시 contract tightening 후보. D-IMPL-4/5는 SPEC-002와의 registry co-ownership으로 설명됨(§3.3). D-IMPL-7은 본 개정으로 해소됨.

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `LLMConfig.Routing` 섹션 |
| 선행 SPEC | SPEC-GOOSE-CREDPOOL-001 | Route.Provider를 풀 provider key로 사용 |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-001 | Route를 provider HTTP 호출로 변환 |
| 후속 SPEC | SPEC-GOOSE-RATELIMIT-001 | 동적 라우팅 상태 참조(후속 확장) |
| 후속 SPEC | SPEC-GOOSE-PROMPT-CACHE-001 | cheap route는 일반적으로 캐시 없음 |
| 후속 SPEC | SPEC-GOOSE-INSIGHTS-001 | 라우팅 결정 학습 |
| 외부 | Go 1.22+ | |
| 외부 | `go.uber.org/zap` v1.27+ | |
| 외부 | `github.com/stretchr/testify` v1.9+ | |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 기본 복잡 키워드 set이 한국어/다국어 메시지에서 동작 안함 | 고 | 중 | `ComplexKeywords` override 가능(REQ-ROUTER-008). Phase 4 INSIGHTS가 언어별 키워드 학습 |
| R2 | 160/28/2 threshold가 사용자 유형별로 부적절 | 중 | 중 | RoutingConfig에서 조정 가능. 기본값은 Hermes 원형 그대로 |
| R3 | Cheap route가 tool call을 지원 안 함(예: 초기 Haiku) | 중 | 고 | ProviderRegistry.SupportsTools 확인 후 tool 있는 대화는 primary fallback. 본 SPEC Phase 1은 단순 heuristic만, 후속 SPEC에서 tool-aware 확장 |
| R4 | classifier heuristic 변경 시 signature 불일치 | 중 | 낮 | signature는 결정 후 Route에 대한 fingerprint이므로 heuristic 변경이 signature 출력에 영향 없음 (model+provider tuple) |
| R5 | 단순 메시지가 실제로는 복잡했다는 사용자 피드백 | 중 | 중 | RoutingDecisionHook로 Phase 4 INSIGHTS가 수집, 키워드 set 보강 제안 |
| R6 | Multi-language 단어 boundary 판정 실패(CJK) | 중 | 중 | word count 시 유니코드 공백 + CJK 구두점 고려. 테스트 케이스 추가 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-llm.md` §2 Provider 매트릭스(15+), §4 Smart Routing 의사코드, §9 Go 포팅 매핑, §10 SPEC 도출
- `.moai/specs/ROADMAP.md` §4 Phase 1 row 07, §13 핵심 설계 원칙 3
- `.moai/specs/SPEC-GOOSE-CREDPOOL-001/spec.md` — Route.Provider → CredentialPool key
- `.moai/project/tech.md` §3.2 Go LLM 스택, §9 LLM Provider 지원

### 9.2 외부 참조

- **Hermes Agent Python**: `./hermes-agent-main/agent/model_router.py` — 원형 인용
- **Go regexp package**: https://pkg.go.dev/regexp
- **Go unicode package**: https://pkg.go.dev/unicode — word boundary CJK 처리

### 9.3 부속 문서

- `./research.md` — classifier heuristic 상세 케이스, provider registry 15+ 엔트리 스펙, 테스트 전략
- `.moai/reports/plan-audit/mass-20260425/ROUTER-001-audit.md` — 독립 감사 리포트 (2026-04-24, 구현 정합률 100% 16/16 REQ · 8/8 AC, coverage 97.2%, race-clean)
- `internal/llm/router/` (commit 103803b) — 현재 구현 트리 (router.go, classifier.go, registry.go, config.go, signature.go, errors.go + 4개 test 파일)

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **실제 LLM HTTP 호출을 포함하지 않는다**. ADAPTER-001.
- 본 SPEC은 **credential 취득을 수행하지 않는다**. CREDPOOL-001의 Select를 호출자가 ROUTER 결정 이후 별도로 호출.
- 본 SPEC은 **rate limit 상태를 고려하지 않는다**. 정적 heuristic만. RATELIMIT-001 결과를 반영하는 동적 라우팅은 후속 SPEC.
- 본 SPEC은 **학습 기반 라우팅을 포함하지 않는다**. INSIGHTS-001(Phase 4)에서 `RoutingDecisionHook`로 수집한 데이터로 확장.
- 본 SPEC은 **프롬프트 캐시 TTL 결정을 포함하지 않는다**. PROMPT-CACHE-001.
- 본 SPEC은 **tool schema 변환을 포함하지 않는다**. ADAPTER-001이 provider별 변환.
- 본 SPEC은 **Fallback model chain 실행을 포함하지 않는다**. QUERY-001의 FallbackModels와 연계.
- 본 SPEC은 **비용 추적/pricing을 포함하지 않는다**. 후속 메트릭 SPEC.
- 본 SPEC은 **multi-round classification(이전 turn 참조)을 지원하지 않는다**. 마지막 user 메시지만 판정 대상.
- 본 SPEC은 **CJK 제외 다국어 토큰화를 최적화하지 않는다**. 기본 unicode space split만.

---

**End of SPEC-GOOSE-ROUTER-001**
