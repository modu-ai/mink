---
id: SPEC-GENIE-ERROR-CLASS-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 4
size: 소(S)
lifecycle: spec-anchored
---

# SPEC-GENIE-ERROR-CLASS-001 — Error Classifier (14 FailoverReason, Retry 전략)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-learning.md §5 + Hermes `error_classifier.py` 28KB 기반) | manager-spec |

---

## 1. 개요 (Overview)

GENIE-AGENT **자기진화 파이프라인의 보조 레이어**를 정의한다. LLM 어댑터(ADAPTER-001)에서 발생한 모든 오류를 **14종 FailoverReason enum**으로 정확 분류하고, 각 오류에 대해 **`retryable` / `should_compress` / `should_rotate_credential` / `should_fallback` 4개의 회복 신호**를 계산한다. 본 분류 결과는 ROUTER-001의 모델 전환, CREDPOOL-001의 credential 회전, CONTEXT-001의 긴급 compaction, COMPRESSOR-001의 trajectory 재가공 트리거로 사용된다.

본 SPEC이 통과한 시점에서:

- `Classifier.Classify(ctx, err, meta ErrorMeta) ClassifiedError`가 **5단계 파이프라인**(provider 특화 → HTTP 상태 → error code → message 패턴 → transport 휴리스틱)을 순서대로 실행하고,
- 14 `FailoverReason` 중 가장 구체적인 하나로 분류되며(매칭 없으면 `Unknown` with `retryable=true`),
- `retryable`은 ADAPTER-001이 같은 credential로 재시도할지 결정, `should_rotate_credential`은 CREDPOOL-001이 다음 key로 이동할지, `should_compress`는 CONTEXT-001이 context window 축소할지, `should_fallback`은 ROUTER-001이 fallback 모델 chain 호출할지 판단한다.
- Anthropic 특화 오류(`thinking_signature`, `long_context_tier`)와 OpenAI 특화 오류(`insufficient_quota`, `context_length_exceeded`)가 provider별 특화 분기에서 먼저 매칭되어 일반 HTTP 코드 매칭보다 우선한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 모든 실제 LLM 호출 경로(ADAPTER-001, COMPRESSOR-001의 Summarizer, FUTURE Skill 호출)의 **단일 오류 해석 소스**가 필요하다. 각 어댑터가 독자 분류를 가지면 retry/fallback 정책이 파편화된다.
- `.moai/project/research/hermes-learning.md` §5가 Hermes `error_classifier.py` 28KB의 14종 분류 체계를 90% 재사용 대상으로 지정했다.
- CREDPOOL-001의 rotation 전략(4가지)은 "어떤 오류에서 rotate할지"를 본 SPEC의 `should_rotate_credential` 플래그에 의존한다.
- TRAJECTORY-001이 실패 궤적을 `failed/` 디렉토리에 저장할 때 `TrajectoryMetadata.FailureReason`은 본 SPEC의 분류 결과를 문자열화한 것이다 — INSIGHTS-001이 실패 유형별 집계에 사용.
- 로드맵 v2.0 §4 Phase 4 #22.

### 2.2 상속 자산

- **Hermes Agent Python** (`./hermes-agent-main/agent/error_classifier.py` 28KB): 14 FailoverReason enum, 5단계 파이프라인(provider → status → error code → message → transport), Anthropic/OpenAI 특화 패턴. 본 SPEC의 GREEN 단계는 분류 표와 패턴 정규식을 90% 재사용.
- **Claude Code TypeScript**: 계승 대상 아님(provider별 분리된 분류만 있음).

### 2.3 범위 경계

- **IN**: `FailoverReason` enum 14종, `ClassifiedError` 구조체, `Classifier` 인터페이스 + 기본 구현, 5단계 파이프라인, provider 특화 패턴(Anthropic/OpenAI/Google 각 2-3개), HTTP status → reason 매핑 표, message 패턴 정규식, transport 휴리스틱(timeout + token budget), Fallback reason `Unknown`.
- **OUT**: 실제 retry 수행(ADAPTER-001), credential rotation(CREDPOOL-001), context compaction(CONTEXT-001), fallback chain 실행(ROUTER-001), rate limit bucket 추적(RATELIMIT-001), 오류 로깅 포맷(logger consumer 책임).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/evolve/errorclass/` 패키지: `FailoverReason` enum, `ClassifiedError`, `ErrorMeta`, `Classifier` 인터페이스.
2. `internal/evolve/errorclass/reasons.go`: 14개 `FailoverReason` 상수 + `String()` / `UnmarshalText()` / `MarshalText()`.
3. `internal/evolve/errorclass/classifier.go`: 기본 `Classifier` 구현 + 5단계 파이프라인.
4. `internal/evolve/errorclass/patterns.go`: provider 특화 패턴 + message regex + error code 매핑 표.
5. `internal/evolve/errorclass/http_status.go`: HTTP status → reason 매핑.
6. `internal/evolve/errorclass/transport.go`: transport 휴리스틱(`ReadTimeout` / `ConnectTimeout` / server disconnect 감지).
7. 각 `FailoverReason`별 4-flag 기본값 표(`retryable`, `should_compress`, `should_rotate_credential`, `should_fallback`).
8. `ErrorMeta` 입력: `Provider string`, `Model string`, `StatusCode int`, `ApproxTokens int`, `ContextLength int`, `MessageCount int`, `RawError error`.
9. 사용자 확장: `ClassifierOptions.ExtraPatterns []ProviderPattern`으로 신규 provider 대응.

### 3.2 OUT OF SCOPE (명시적 제외)

- **실제 재시도 수행**: ADAPTER-001.
- **Credential rotation 실행**: CREDPOOL-001 (본 SPEC은 `should_rotate_credential` bool만 제공).
- **Context compaction 실행**: CONTEXT-001 / COMPRESSOR-001 (본 SPEC은 `should_compress` bool만 제공).
- **Fallback chain 실행**: ROUTER-001 (본 SPEC은 `should_fallback` bool만 제공).
- **Rate limit bucket 추적**: RATELIMIT-001은 429 오류에서 Retry-After 헤더 파싱 — 본 SPEC은 `reason=RateLimit`까지만.
- **비LLM 오류 분류**: 파일 시스템, 네트워크 스택, DB 오류는 대상 아님. `Unknown + retryable=false`로 반환.
- **오류 집계 통계**: INSIGHTS-001 담당.
- **UI 표시 / 사용자 메시지 번역**: CLI-001 담당.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-ERRCLASS-001 [Ubiquitous]** — The `Classifier.Classify` method **shall** always return a `ClassifiedError` with exactly one of the 14 `FailoverReason` values (including `Unknown` as fallback); nil reasons **shall not** occur.

**REQ-ERRCLASS-002 [Ubiquitous]** — Each of the 14 `FailoverReason` values **shall** have a deterministic default 4-flag profile (`retryable`, `should_compress`, `should_rotate_credential`, `should_fallback`) documented in the source code as a lookup table.

**REQ-ERRCLASS-003 [Ubiquitous]** — The `Classifier` **shall** execute the 5-stage pipeline in strict order: (1) provider-specific, (2) HTTP status, (3) error code, (4) message regex, (5) transport heuristic; a match at any stage **shall** short-circuit subsequent stages.

**REQ-ERRCLASS-004 [Ubiquitous]** — `ClassifiedError.RawError` **shall** always preserve the original `error` unwrapping chain (i.e. `errors.Unwrap(classified.RawError)` returns the innermost error).

### 4.2 Event-Driven (이벤트 기반)

**REQ-ERRCLASS-005 [Event-Driven]** — **When** `meta.Provider == "anthropic"` and the error message contains the substring `"thinking_signature"`, the classifier **shall** return `FailoverReason.ThinkingSignature` with `retryable=false, should_fallback=true` (Anthropic-specific protocol error, no recovery within same provider).

**REQ-ERRCLASS-006 [Event-Driven]** — **When** `meta.StatusCode == 401`, the classifier **shall** return `FailoverReason.Auth` with `retryable=true, should_rotate_credential=true` (temporary auth failure — likely token refresh needed).

**REQ-ERRCLASS-007 [Event-Driven]** — **When** `meta.StatusCode == 403` and message matches `/(permission|forbidden|not.*allowed)/i`, the classifier **shall** return `FailoverReason.AuthPermanent` with `retryable=false, should_rotate_credential=true, should_fallback=true`.

**REQ-ERRCLASS-008 [Event-Driven]** — **When** `meta.StatusCode == 429`, the classifier **shall** return `FailoverReason.RateLimit` with `retryable=true, should_rotate_credential=true` (try next key before giving up).

**REQ-ERRCLASS-009 [Event-Driven]** — **When** `meta.StatusCode == 402` or message matches `/(insufficient.?quota|billing|credit.*exhausted)/i`, the classifier **shall** return `FailoverReason.Billing` with `retryable=false, should_rotate_credential=true, should_fallback=true`.

**REQ-ERRCLASS-010 [Event-Driven]** — **When** `meta.StatusCode == 413` or message matches `/(payload.*too.*large|request.*body.*too.*large)/i`, the classifier **shall** return `FailoverReason.PayloadTooLarge` with `retryable=true, should_compress=true`.

**REQ-ERRCLASS-011 [Event-Driven]** — **When** `meta.StatusCode == 400` and message matches `/(context.*length.*exceed|maximum.*context|token.*limit)/i`, the classifier **shall** return `FailoverReason.ContextOverflow` with `retryable=true, should_compress=true`.

**REQ-ERRCLASS-012 [Event-Driven]** — **When** `meta.StatusCode == 503` or `529`, the classifier **shall** return `FailoverReason.Overloaded` with `retryable=true, should_fallback=true`.

**REQ-ERRCLASS-013 [Event-Driven]** — **When** `meta.StatusCode == 500` or `502`, the classifier **shall** return `FailoverReason.ServerError` with `retryable=true, should_fallback=true`.

**REQ-ERRCLASS-014 [Event-Driven]** — **When** `meta.StatusCode == 404` and message matches `/(model.*not.*found|no.*such.*model)/i`, the classifier **shall** return `FailoverReason.ModelNotFound` with `retryable=false, should_fallback=true`.

**REQ-ERRCLASS-015 [Event-Driven]** — **When** the underlying error is `context.DeadlineExceeded` or wraps `net.Error.Timeout() == true`, the classifier **shall** return `FailoverReason.Timeout` with `retryable=true`.

**REQ-ERRCLASS-016 [Event-Driven]** — **When** the error is a transport disconnect AND `meta.ApproxTokens > meta.ContextLength * 0.6` OR `meta.ApproxTokens > 120_000` OR `meta.MessageCount > 200`, the classifier **shall** return `FailoverReason.ContextOverflow` (heuristic: server likely disconnected due to context bloat) with `retryable=true, should_compress=true`.

### 4.3 State-Driven (상태 기반)

**REQ-ERRCLASS-017 [State-Driven]** — **While** the input `err` is nil, the classifier **shall** return `ClassifiedError{Reason: Unknown, Retryable: false, Message: "nil error"}` without executing the pipeline.

**REQ-ERRCLASS-018 [State-Driven]** — **While** `meta.Provider` is in the set of supported providers (`anthropic`, `openai`, `google`, `xai`, `deepseek`, `ollama`), stage 1 (provider-specific) patterns **shall** be consulted; otherwise stage 1 is skipped.

### 4.4 Unwanted Behavior (방지)

**REQ-ERRCLASS-019 [Unwanted]** — The classifier **shall not** panic on malformed error types (nil-deref, invalid regex inputs); all pattern matching **shall** be wrapped with `recover()` and on panic return `FailoverReason.Unknown`.

**REQ-ERRCLASS-020 [Unwanted]** — The classifier **shall not** modify `meta` (read-only input); the result **shall not** retain references to `meta.RawError`'s internal mutable fields beyond the function return.

**REQ-ERRCLASS-021 [Unwanted]** — The classifier **shall not** set both `retryable=true` AND `should_fallback=true` simultaneously for the same `FailoverReason` unless the reason lookup table explicitly defines both (Billing, Overloaded, ServerError, ThinkingSignature) — the combination represents "try again but also prepare fallback".

**REQ-ERRCLASS-022 [Unwanted]** — **If** stage 2 (HTTP status) matches a status code but message content contradicts the default reason (e.g. 429 with message "actually OK"), the classifier **shall** still proceed to stage 4 (message regex) to override the reason; HTTP status is a hint, not final.

### 4.5 Optional (선택적)

**REQ-ERRCLASS-023 [Optional]** — **Where** `ClassifierOptions.ExtraPatterns` is non-empty, the classifier **shall** consult extra patterns at the start of stage 1 before built-in provider patterns; this allows new providers to be onboarded without code change.

**REQ-ERRCLASS-024 [Optional]** — **Where** `ClassifierOptions.OverrideFlags` map contains a reason, the 4-flag defaults for that reason **shall** be replaced by the override (allowing policy tuning per deployment).

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then.

**AC-ERRCLASS-001 — 14 FailoverReason 열거형 완전성**
- **Given** `FailoverReason` enum
- **When** 테스트가 `AllFailoverReasons()` slice 호출
- **Then** 정확히 14개 반환: `Auth, AuthPermanent, Billing, RateLimit, Overloaded, ServerError, ContextOverflow, PayloadTooLarge, ModelNotFound, Timeout, FormatError, ThinkingSignature, TransportError, Unknown`. 각 reason에 대한 `.String()` 호출이 snake_case 문자열 반환(`"auth"`, `"auth_permanent"`, ...)

**AC-ERRCLASS-002 — Anthropic thinking_signature 우선 분기**
- **Given** `meta.Provider="anthropic"`, `err=errors.New("thinking_signature mismatch between request and response")`
- **When** `Classify(err, meta)`
- **Then** `Reason == ThinkingSignature`, `Retryable=false`, `ShouldFallback=true`. HTTP status가 400이라도 stage 1에서 short-circuit

**AC-ERRCLASS-003 — HTTP 401 → Auth retryable+rotate**
- **Given** `meta.StatusCode=401`, `err=errors.New("invalid api key")`
- **When** `Classify`
- **Then** `Reason == Auth`, `Retryable=true`, `ShouldRotateCredential=true`, `ShouldFallback=false`, `ShouldCompress=false`

**AC-ERRCLASS-004 — HTTP 402 billing → fallback**
- **Given** `meta.StatusCode=402`, `err=errors.New("insufficient_quota")`
- **When** `Classify`
- **Then** `Reason == Billing`, `Retryable=false`, `ShouldRotateCredential=true`, `ShouldFallback=true`

**AC-ERRCLASS-005 — HTTP 413 payload → compress**
- **Given** `meta.StatusCode=413`, `err=errors.New("request body too large")`
- **When** `Classify`
- **Then** `Reason == PayloadTooLarge`, `Retryable=true`, `ShouldCompress=true`

**AC-ERRCLASS-006 — 400 + context_length_exceeded message**
- **Given** `meta.StatusCode=400`, `err=errors.New("context length exceeded: got 150000 tokens, max is 128000")`
- **When** `Classify`
- **Then** `Reason == ContextOverflow`, `Retryable=true`, `ShouldCompress=true`. stage 2(HTTP 400은 ambiguous) → stage 4(message regex)가 우선 override(REQ-022 검증)

**AC-ERRCLASS-007 — 429 rate limit + rotate**
- **Given** `meta.StatusCode=429`
- **When** `Classify`
- **Then** `Reason == RateLimit`, `Retryable=true`, `ShouldRotateCredential=true`

**AC-ERRCLASS-008 — 503 overloaded → fallback**
- **Given** `meta.StatusCode=503`, `err=errors.New("service unavailable")`
- **When** `Classify`
- **Then** `Reason == Overloaded`, `Retryable=true`, `ShouldFallback=true`

**AC-ERRCLASS-009 — 529 anthropic overloaded**
- **Given** `meta.StatusCode=529`, `meta.Provider="anthropic"`
- **When** `Classify`
- **Then** `Reason == Overloaded` (Anthropic의 비표준 529는 overloaded 동의어)

**AC-ERRCLASS-010 — context.DeadlineExceeded → Timeout**
- **Given** `err=context.DeadlineExceeded`, `meta.StatusCode=0`
- **When** `Classify`
- **Then** `Reason == Timeout`, `Retryable=true`

**AC-ERRCLASS-011 — Transport 휴리스틱: 큰 컨텍스트 → ContextOverflow**
- **Given** `err=errors.New("server disconnected")`, `meta.StatusCode=0`, `meta.ApproxTokens=125_000`, `meta.ContextLength=200_000` (not yet 60%)
- **When** `Classify`
- **Then** `Reason == ContextOverflow` (125_000 > 120_000 임계치 만족), `ShouldCompress=true`

**AC-ERRCLASS-012 — 404 model not found → fallback**
- **Given** `meta.StatusCode=404`, `err=errors.New("model 'gpt-5-turbo-nonexistent' not found")`
- **When** `Classify`
- **Then** `Reason == ModelNotFound`, `Retryable=false`, `ShouldFallback=true`

**AC-ERRCLASS-013 — nil 오류 안전 처리**
- **Given** `err=nil`, `meta` any
- **When** `Classify`
- **Then** `Reason == Unknown`, `Retryable=false`, `Message == "nil error"`, 패닉 없음

**AC-ERRCLASS-014 — 알 수 없는 오류 fallback**
- **Given** `err=errors.New("strange ufo error 🛸")`, `meta.StatusCode=0`, `meta.Provider="unknown_cloud"`
- **When** `Classify`
- **Then** `Reason == Unknown`, `Retryable=true` (기본적으로 한 번은 시도, REQ-ERRCLASS-022에 따라 신중히)

**AC-ERRCLASS-015 — 파이프라인 순서(provider 우선)**
- **Given** `meta.Provider="anthropic"`, `meta.StatusCode=429`, `err=errors.New("thinking_signature mismatch")`
- **When** `Classify`
- **Then** `Reason == ThinkingSignature` (stage 1이 stage 2 HTTP 429보다 우선)

**AC-ERRCLASS-016 — 패닉 방어**
- **Given** 주입된 malicious pattern이 regex 평가 중 panic 유발 (테스트에서 인위적 주입)
- **When** `Classify`
- **Then** `Reason == Unknown`, `Retryable=false`, `Message == "classification panic recovered"`, 프로세스 계속 실행

**AC-ERRCLASS-017 — ExtraPatterns 주입**
- **Given** `ClassifierOptions.ExtraPatterns=[{Provider:"mistral", Pattern:/model_overloaded/, Reason:Overloaded}]`
- **When** `Classify(meta.Provider="mistral", err="our model is temporarily overloaded")`
- **Then** `Reason == Overloaded` (built-in 표에 "mistral" 없지만 Extra로 매칭)

**AC-ERRCLASS-018 — OverrideFlags 정책 변경**
- **Given** `ClassifierOptions.OverrideFlags[Timeout] = {Retryable:false, ShouldFallback:true}` (회사 정책: timeout은 재시도 금지, 바로 fallback)
- **When** `Classify(err=context.DeadlineExceeded)`
- **Then** `Reason == Timeout`, `Retryable=false`, `ShouldFallback=true` (기본값 override됨)

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/
└── evolve/
    └── errorclass/
        ├── reasons.go              # 14 FailoverReason enum + String/Marshal
        ├── reasons_test.go
        ├── classifier.go           # Classifier interface + default impl
        ├── classifier_test.go
        ├── patterns.go             # Provider patterns + message regex + error code
        ├── patterns_test.go
        ├── http_status.go          # HTTP status → reason map
        ├── transport.go            # Timeout + server disconnect heuristic
        ├── defaults.go             # 14 reason별 4-flag default 표
        └── options.go              # ClassifierOptions (ExtraPatterns, OverrideFlags)
```

### 6.2 핵심 타입 (Go 시그니처)

```go
// internal/evolve/errorclass/reasons.go

type FailoverReason int

const (
    Unknown FailoverReason = iota  // 0: fallback
    Auth                           // 1:  401 일시적
    AuthPermanent                  // 2:  403 또는 key revoked
    Billing                        // 3:  402 / insufficient_quota
    RateLimit                      // 4:  429
    Overloaded                     // 5:  503 / 529
    ServerError                    // 6:  500 / 502
    ContextOverflow                // 7:  400 context_length_exceeded or transport heuristic
    PayloadTooLarge                // 8:  413
    ModelNotFound                  // 9:  404 model_not_found
    Timeout                        // 10: read/connect timeout
    FormatError                    // 11: 400 invalid JSON / malformed request
    ThinkingSignature              // 12: Anthropic 특화 protocol error
    TransportError                 // 13: network stack error (not timeout)
)

func (r FailoverReason) String() string  // "auth", "auth_permanent", ...
func (r FailoverReason) MarshalText() ([]byte, error)
func (r *FailoverReason) UnmarshalText(b []byte) error

func AllFailoverReasons() []FailoverReason  // 14 values (exclude Unknown or include? spec: include)


// internal/evolve/errorclass/classifier.go

type ClassifiedError struct {
    Reason                 FailoverReason
    StatusCode             int
    Retryable              bool
    ShouldCompress         bool
    ShouldRotateCredential bool
    ShouldFallback         bool
    Message                string   // human-readable summary
    MatchedBy              string   // "stage1_provider" | "stage2_http" | "stage3_code" | "stage4_message" | "stage5_transport" | "fallback"
    RawError               error    // 보존 (errors.Unwrap 가능)
}

type ErrorMeta struct {
    Provider      string
    Model         string
    StatusCode    int
    ApproxTokens  int
    ContextLength int
    MessageCount  int
    RawError      error
}

type Classifier interface {
    Classify(ctx context.Context, err error, meta ErrorMeta) ClassifiedError
}

type defaultClassifier struct {
    opts ClassifierOptions
}

func New(opts ClassifierOptions) Classifier

// 5-stage pipeline
func (c *defaultClassifier) Classify(ctx context.Context, err error, meta ErrorMeta) ClassifiedError {
    if err == nil {
        return ClassifiedError{Reason: Unknown, Message: "nil error"}
    }
    defer panicGuard(&result)  // REQ-019

    // Stage 1: Provider-specific
    if r, ok := c.matchProviderSpecific(err, meta); ok {
        return c.build(r, "stage1_provider", err, meta)
    }
    // Stage 2: HTTP status
    if r, ok := matchHTTPStatus(meta.StatusCode); ok {
        // Stage 4 override check (REQ-022)
        if rOverride, ok := matchMessageRegex(err.Error()); ok && rOverride != r {
            return c.build(rOverride, "stage4_message", err, meta)
        }
        return c.build(r, "stage2_http", err, meta)
    }
    // Stage 3: Error code (body.error.code)
    if r, ok := matchErrorCode(err); ok {
        return c.build(r, "stage3_code", err, meta)
    }
    // Stage 4: Message regex
    if r, ok := matchMessageRegex(err.Error()); ok {
        return c.build(r, "stage4_message", err, meta)
    }
    // Stage 5: Transport heuristic
    if r, ok := matchTransport(err, meta); ok {
        return c.build(r, "stage5_transport", err, meta)
    }
    // Fallback
    return c.build(Unknown, "fallback", err, meta)
}


// internal/evolve/errorclass/defaults.go

// defaultFlags는 14 reason의 4-flag 기본 정책.
var defaultFlags = map[FailoverReason]struct {
    Retryable              bool
    ShouldCompress         bool
    ShouldRotateCredential bool
    ShouldFallback         bool
}{
    Auth:              {true,  false, true,  false},
    AuthPermanent:     {false, false, true,  true},
    Billing:           {false, false, true,  true},
    RateLimit:         {true,  false, true,  false},
    Overloaded:        {true,  false, false, true},
    ServerError:       {true,  false, false, true},
    ContextOverflow:   {true,  true,  false, false},
    PayloadTooLarge:   {true,  true,  false, false},
    ModelNotFound:     {false, false, false, true},
    Timeout:           {true,  false, false, false},
    FormatError:       {false, false, false, false},
    ThinkingSignature: {false, false, false, true},
    TransportError:    {true,  false, false, false},
    Unknown:           {true,  false, false, false},
}


// internal/evolve/errorclass/patterns.go

type ProviderPattern struct {
    Provider string                     // "anthropic", "openai", ...
    Pattern  *regexp.Regexp
    Reason   FailoverReason
}

// BuiltinProviderPatterns는 Hermes §5 원본 기반.
var BuiltinProviderPatterns = []ProviderPattern{
    {Provider: "anthropic", Pattern: regexp.MustCompile(`thinking_signature`), Reason: ThinkingSignature},
    {Provider: "anthropic", Pattern: regexp.MustCompile(`long_context_tier`),  Reason: ContextOverflow},
    {Provider: "openai",    Pattern: regexp.MustCompile(`insufficient_quota`), Reason: Billing},
    {Provider: "openai",    Pattern: regexp.MustCompile(`context_length_exceeded`), Reason: ContextOverflow},
    // ... 추가
}

var messagePatterns = []struct {
    Pattern *regexp.Regexp
    Reason  FailoverReason
}{
    {regexp.MustCompile(`(?i)context.*length.*exceed`), ContextOverflow},
    {regexp.MustCompile(`(?i)maximum.*context`),         ContextOverflow},
    {regexp.MustCompile(`(?i)token.*limit`),             ContextOverflow},
    {regexp.MustCompile(`(?i)payload.*too.*large`),      PayloadTooLarge},
    {regexp.MustCompile(`(?i)insufficient.?quota`),      Billing},
    {regexp.MustCompile(`(?i)credit.*exhausted`),        Billing},
    {regexp.MustCompile(`(?i)rate.?limit`),              RateLimit},
    {regexp.MustCompile(`(?i)model.*not.*found`),        ModelNotFound},
    {regexp.MustCompile(`(?i)no.*such.*model`),          ModelNotFound},
    {regexp.MustCompile(`(?i)permission|forbidden`),     AuthPermanent},
    // ... 총 15-20개
}


// internal/evolve/errorclass/http_status.go

func matchHTTPStatus(status int) (FailoverReason, bool) {
    switch status {
    case 401:        return Auth, true
    case 402:        return Billing, true
    case 403:        return AuthPermanent, true
    case 404:        return ModelNotFound, true
    case 413:        return PayloadTooLarge, true
    case 429:        return RateLimit, true
    case 500, 502:   return ServerError, true
    case 503, 529:   return Overloaded, true
    case 400:        // ambiguous, defer to stage 4
        return Unknown, false
    }
    return Unknown, false
}


// internal/evolve/errorclass/transport.go

func matchTransport(err error, meta ErrorMeta) (FailoverReason, bool) {
    if errors.Is(err, context.DeadlineExceeded) {
        return Timeout, true
    }
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Timeout() {
        return Timeout, true
    }
    // Server disconnect heuristic
    msg := err.Error()
    isDisconnect := strings.Contains(msg, "connection reset") ||
                    strings.Contains(msg, "server disconnected") ||
                    strings.Contains(msg, "EOF")
    if isDisconnect {
        // Context bloat heuristic (REQ-ERRCLASS-016)
        if meta.ApproxTokens > int(float64(meta.ContextLength)*0.6) ||
           meta.ApproxTokens > 120_000 ||
           meta.MessageCount > 200 {
            return ContextOverflow, true
        }
        return TransportError, true
    }
    return Unknown, false
}


// internal/evolve/errorclass/options.go

type ClassifierOptions struct {
    ExtraPatterns  []ProviderPattern
    OverrideFlags  map[FailoverReason]FlagProfile
}

type FlagProfile struct {
    Retryable              bool
    ShouldCompress         bool
    ShouldRotateCredential bool
    ShouldFallback         bool
}
```

### 6.3 14 FailoverReason × 4-flag 기본값 표

| Reason | Retryable | ShouldCompress | ShouldRotateCredential | ShouldFallback | 주된 트리거 |
|---|:-:|:-:|:-:|:-:|---|
| `Auth`               | ✓ | ·  | ✓ | ·  | 401 |
| `AuthPermanent`      | ·  | ·  | ✓ | ✓ | 403 + permission msg |
| `Billing`            | ·  | ·  | ✓ | ✓ | 402 / insufficient_quota |
| `RateLimit`          | ✓ | ·  | ✓ | ·  | 429 |
| `Overloaded`         | ✓ | ·  | ·  | ✓ | 503 / 529 |
| `ServerError`        | ✓ | ·  | ·  | ✓ | 500 / 502 |
| `ContextOverflow`    | ✓ | ✓ | ·  | ·  | 400 context_length / transport bloat |
| `PayloadTooLarge`    | ✓ | ✓ | ·  | ·  | 413 |
| `ModelNotFound`      | ·  | ·  | ·  | ✓ | 404 model_not_found |
| `Timeout`            | ✓ | ·  | ·  | ·  | context.DeadlineExceeded |
| `FormatError`        | ·  | ·  | ·  | ·  | 400 invalid JSON |
| `ThinkingSignature`  | ·  | ·  | ·  | ✓ | Anthropic 특화 |
| `TransportError`     | ✓ | ·  | ·  | ·  | connection reset, EOF (no bloat) |
| `Unknown`            | ✓ | ·  | ·  | ·  | fallback |

### 6.4 5단계 파이프라인 의사코드

```
Classify(err, meta):
    if err == nil: return Unknown{nil error}
    
    # Stage 1: Provider-specific
    for pattern in ExtraPatterns + BuiltinProviderPatterns:
        if meta.Provider == pattern.Provider and pattern.Pattern.match(err.Error()):
            return build(pattern.Reason, "stage1")
    
    # Stage 2: HTTP status (with Stage 4 override check)
    if reason, ok := matchHTTPStatus(meta.StatusCode); ok:
        # REQ-022: stage 4 regex가 더 구체적이면 override
        if overrideReason, ok := matchMessageRegex(err.Error()); ok and overrideReason != reason:
            return build(overrideReason, "stage4")
        return build(reason, "stage2")
    
    # Stage 3: Error code (body.error.code)
    if reason, ok := matchErrorCode(err); ok:
        return build(reason, "stage3")
    
    # Stage 4: Message regex
    if reason, ok := matchMessageRegex(err.Error()); ok:
        return build(reason, "stage4")
    
    # Stage 5: Transport heuristic
    if reason, ok := matchTransport(err, meta); ok:
        return build(reason, "stage5")
    
    return build(Unknown, "fallback")
```

### 6.5 Integration 예시

```go
// ADAPTER-001에서 사용 예시
resp, err := provider.Call(ctx, req)
if err != nil {
    classified := classifier.Classify(ctx, err, errorclass.ErrorMeta{
        Provider:      provider.Name(),
        Model:         req.Model,
        StatusCode:    extractStatus(err),
        ApproxTokens:  req.EstimateTokens(),
        ContextLength: req.ModelContextLength(),
        MessageCount:  len(req.Messages),
        RawError:      err,
    })
    switch {
    case classified.ShouldRotateCredential:
        credPool.MarkExhausted(currentKey)
    case classified.ShouldCompress:
        context.RequestCompaction()
    case classified.ShouldFallback:
        router.AdvanceToFallback()
    }
    if classified.Retryable {
        retryWithBackoff(req)
    }
}
```

### 6.6 TDD 진입 순서

1. **RED #1**: `TestAllFailoverReasons_14Items` — AC-ERRCLASS-001.
2. **RED #2**: `TestClassify_NilError` — AC-ERRCLASS-013.
3. **RED #3**: `TestClassify_Anthropic_ThinkingSignature_TakesPriority` — AC-ERRCLASS-002, AC-ERRCLASS-015.
4. **RED #4**: `TestClassify_HTTP_401_Auth` — AC-ERRCLASS-003.
5. **RED #5**: `TestClassify_HTTP_402_Billing` — AC-ERRCLASS-004.
6. **RED #6**: `TestClassify_HTTP_413_PayloadCompress` — AC-ERRCLASS-005.
7. **RED #7**: `TestClassify_400_ContextMessage_OverridesGenericBadRequest` — AC-ERRCLASS-006, REQ-022.
8. **RED #8**: `TestClassify_HTTP_429_RateLimit` — AC-ERRCLASS-007.
9. **RED #9**: `TestClassify_HTTP_503_And_529_Overloaded` — AC-ERRCLASS-008, AC-ERRCLASS-009.
10. **RED #10**: `TestClassify_DeadlineExceeded_Timeout` — AC-ERRCLASS-010.
11. **RED #11**: `TestClassify_TransportDisconnect_BigContext_Overflow` — AC-ERRCLASS-011.
12. **RED #12**: `TestClassify_404_ModelNotFound` — AC-ERRCLASS-012.
13. **RED #13**: `TestClassify_UnknownFallbackRetryable` — AC-ERRCLASS-014.
14. **RED #14**: `TestClassify_PanicRecovered` — AC-ERRCLASS-016.
15. **RED #15**: `TestOptions_ExtraPatterns` — AC-ERRCLASS-017.
16. **RED #16**: `TestOptions_OverrideFlags` — AC-ERRCLASS-018.
17. **GREEN**: 5단계 파이프라인 + pattern 표 + defaults 표.
18. **REFACTOR**: pattern 표를 data-driven(test에서 case slice 사용), stage 함수 분리.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, 18 AC 전부 단위 테스트, 14 reason 각각 positive + negative 케이스 |
| **R**eadable | 4-flag 기본값 표(§6.3) + 5단계 파이프라인(§6.4)이 데이터 구조로 명시 |
| **U**nified | `golangci-lint`, reason enum의 `String()` snake_case 일관성 |
| **S**ecured | Regex RE2(backtracking 없음), panic guard(REQ-019), message는 공격자 입력이므로 regex 적용 전 size cap |
| **T**rackable | `MatchedBy` 필드로 어느 stage에서 분류됐는지 기록, zap 로그에 reason + stage |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GENIE-ADAPTER-001 | 오류 발생처. HTTP status / provider 이름 주입자 |
| 선행 SPEC | SPEC-GENIE-CORE-001 | zap 로거, context 루트 |
| 후속 SPEC | SPEC-GENIE-CREDPOOL-001 | `ShouldRotateCredential` 소비 |
| 후속 SPEC | SPEC-GENIE-ROUTER-001 | `ShouldFallback` 소비 |
| 후속 SPEC | SPEC-GENIE-CONTEXT-001 | `ShouldCompress` 소비 |
| 후속 SPEC | SPEC-GENIE-RATELIMIT-001 | `Reason=RateLimit` 소비 후 Retry-After 처리 |
| 후속 SPEC | SPEC-GENIE-TRAJECTORY-001 | `Reason.String()`이 `TrajectoryMetadata.FailureReason` 값 |
| 후속 SPEC | SPEC-GENIE-INSIGHTS-001 | 실패 reason 집계 |
| 외부 | Go 1.22+ | regexp, errors.Is/As, net.Error |
| 외부 | `go.uber.org/zap` v1.27+ | CORE-001 계승 |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 신규 provider 오류 포맷(예: Groq, Mistral)이 기존 패턴에 없음 | 고 | 중 | `ExtraPatterns` 주입 인터페이스(REQ-023). `.moai/config/errorclass.yaml`에 선언적 추가 경로 |
| R2 | Message regex false positive (사용자 프롬프트가 "insufficient_quota" 포함) | 낮 | 중 | regex는 error message에만 적용(사용자 프롬프트 무관). stage 1-3이 먼저 매칭되므로 영향 제한 |
| R3 | `matchHTTPStatus` 400 → 단계 4 override가 무한 루프 가능성 | 낮 | 낮 | 명확한 단방향 파이프라인, stage 2→4 override는 단 1회만 |
| R4 | Panic guard가 실제 버그를 숨김 | 중 | 중 | Panic 시 zap error 레벨 로그 + `MatchedBy="panic_guard"` 기록. 프로덕션 모니터링으로 조기 발견 |
| R5 | `ContextOverflow` 휴리스틱(60% / 120K / 200 msgs)이 실제 provider 한도와 괴리 | 중 | 중 | 임계치를 `ClassifierOptions.TransportThresholds`로 주입 가능하게 확장 |
| R6 | `ShouldFallback=true` + `Retryable=true` 동시가 호출자 혼란 | 중 | 낮 | REQ-021로 정책 문서화. 호출자는 "retry first, then fallback on exhaustion" 순서 |
| R7 | `Unknown` reason이 너무 자주 나와 silent fallback | 중 | 중 | INSIGHTS-001이 `Unknown` 비율을 alert. 15% 초과 시 신규 패턴 조사 |
| R8 | `FormatError` vs `ContextOverflow`의 400 모호함 | 중 | 낮 | stage 4 regex가 더 구체적(`context.*length`), 일반 400은 `FormatError`로 분류 |
| R9 | Ollama(로컬) 오류가 HTTP status 없는 경우 | 중 | 낮 | stage 5 transport 휴리스틱이 담당. `TransportError` 또는 `Timeout` 분류 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-learning.md` §5 Error Classifier (14 FailoverReason + 분류 파이프라인 원문)
- `.moai/project/learning-engine.md` §12.2 Error 유형 분류 요구
- `.moai/specs/ROADMAP.md` §4 Phase 4 #22
- `.moai/specs/SPEC-GENIE-TRAJECTORY-001/spec.md` — `TrajectoryMetadata.FailureReason` 소비자

### 9.2 외부 참조

- **Hermes `error_classifier.py`** (28KB): 14 FailoverReason 원본
- **Anthropic API error reference**: https://docs.anthropic.com/en/api/errors — thinking_signature, overloaded(529)
- **OpenAI API error codes**: https://platform.openai.com/docs/guides/error-codes — insufficient_quota, context_length_exceeded
- **RFC 9110 (HTTP Semantics)**: 401/403/404/413/429/500/502/503 정의
- **Go `net.Error`**: https://pkg.go.dev/net#Error — Timeout() 인터페이스

### 9.3 부속 문서

- `./research.md` — Hermes 28KB → Go 500 LoC 이식 매핑, 14 reason 결정 근거, regex 테스트 표
- `../SPEC-GENIE-ADAPTER-001/spec.md` — 선행(error 발생처)
- `../SPEC-GENIE-CREDPOOL-001/spec.md` — 후속(rotation 소비)
- `../SPEC-GENIE-ROUTER-001/spec.md` — 후속(fallback 소비)

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **실제 재시도 수행을 구현하지 않는다**. `Retryable` bool만 제공. ADAPTER-001.
- 본 SPEC은 **credential rotation 실행을 구현하지 않는다**. `ShouldRotateCredential` bool만. CREDPOOL-001.
- 본 SPEC은 **context compaction 실행을 구현하지 않는다**. `ShouldCompress` bool만. CONTEXT-001/COMPRESSOR-001.
- 본 SPEC은 **fallback chain 실행을 구현하지 않는다**. `ShouldFallback` bool만. ROUTER-001.
- 본 SPEC은 **Retry-After 헤더 파싱을 포함하지 않는다**. RATELIMIT-001 위임.
- 본 SPEC은 **오류 집계 / 시각화를 포함하지 않는다**. INSIGHTS-001 위임.
- 본 SPEC은 **사용자 메시지 번역 / UI 표시를 포함하지 않는다**. CLI-001 위임.
- 본 SPEC은 **비 LLM 오류(DB, 파일, 네트워크 스택)를 분류하지 않는다**. `Unknown + retryable=false` 반환.
- 본 SPEC은 **오류 로깅 포맷을 강제하지 않는다**. logger consumer 책임.
- 본 SPEC은 **retry budget / circuit breaker를 구현하지 않는다**. ADAPTER-001의 호출자 책임.

---

**End of SPEC-GENIE-ERROR-CLASS-001**
