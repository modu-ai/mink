---
id: SPEC-GENIE-ROUTER-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GENIE-ROUTER-001 — Smart Model Routing + Provider Registry

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-llm.md §4 + ROADMAP v2.0 Phase 1 기반) | manager-spec |

---

## 1. 개요 (Overview)

GENIE-AGENT의 **라우팅 결정 레이어**를 정의한다. Hermes Agent의 `model_router.py` smart routing heuristic(hermes-llm.md §4)을 Go로 포팅하여, 사용자 메시지의 단순성을 판정하고 primary 모델과 cheap 모델 사이의 전환을 결정하는 `internal/llm/router` 패키지를 구현한다.

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
- **When** `Route`
- **Then** `nil, ErrProviderNotRegistered{name:"nonexistent_provider"}`

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
    AuthType        string // "oauth" | "api_key"
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

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GENIE-CORE-001 | zap logger |
| 선행 SPEC | SPEC-GENIE-CONFIG-001 | `LLMConfig.Routing` 섹션 |
| 선행 SPEC | SPEC-GENIE-CREDPOOL-001 | Route.Provider를 풀 provider key로 사용 |
| 후속 SPEC | SPEC-GENIE-ADAPTER-001 | Route를 provider HTTP 호출로 변환 |
| 후속 SPEC | SPEC-GENIE-RATELIMIT-001 | 동적 라우팅 상태 참조(후속 확장) |
| 후속 SPEC | SPEC-GENIE-PROMPT-CACHE-001 | cheap route는 일반적으로 캐시 없음 |
| 후속 SPEC | SPEC-GENIE-INSIGHTS-001 | 라우팅 결정 학습 |
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
- `.moai/specs/SPEC-GENIE-CREDPOOL-001/spec.md` — Route.Provider → CredentialPool key
- `.moai/project/tech.md` §3.2 Go LLM 스택, §9 LLM Provider 지원

### 9.2 외부 참조

- **Hermes Agent Python**: `./hermes-agent-main/agent/model_router.py` — 원형 인용
- **Go regexp package**: https://pkg.go.dev/regexp
- **Go unicode package**: https://pkg.go.dev/unicode — word boundary CJK 처리

### 9.3 부속 문서

- `./research.md` — classifier heuristic 상세 케이스, provider registry 15+ 엔트리 스펙, 테스트 전략

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

**End of SPEC-GENIE-ROUTER-001**
