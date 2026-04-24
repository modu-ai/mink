---
id: SPEC-GOOSE-RATELIMIT-001
version: 0.2.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 소(S)
lifecycle: spec-anchored
labels: [rate-limit, llm, provider, tracker, phase-1]
---

# SPEC-GOOSE-RATELIMIT-001 — Rate Limit Tracker (RPM/TPM/RPH/TPH 4 Bucket)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-llm.md §5 + ROADMAP v2.0 Phase 1 기반) | manager-spec |
| 0.2.0 | 2026-04-25 | 감사 리포트 결함 수정: MP-2 EARS Unwanted 패턴 교정(REQ-RL-009/011), REQ-RL-011 3분할, Traceability 고아 REQ 5건에 AC 신설(AC-RL-008~012), REQ-RL-004/005 하드코딩을 `opts.ThresholdPct`/`opts.WarnCooldown`로 일관화, REQ-RL-002/003 구현 세부(sync.RWMutex/공식) 추상화, REQ-RL-007 2분할(007a/007b), AC-RL-004 결정성 보강, AC-RL-006 단일 문자열 고정, AC-RL-007 결정적 invariant화, Exclusions에 circuit breaker 경계 명시, IN SCOPE 명확화(Nous stub 범위 + OpenAI-compat 재사용 대상) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT Phase 1의 **provider 응답 헤더 기반 rate limit 추적 레이어**를 정의한다. Hermes Agent의 `rate_limit_tracker.py`(~243 LoC)를 Go로 포팅하여, 각 provider의 HTTP 응답 헤더(`x-ratelimit-*`)를 파싱하고 4-bucket(`requests_min/hour`, `tokens_min/hour`) 상태를 유지하며 80% 임계치 초과 시 경고를 발화하는 `internal/llm/ratelimit` 패키지를 구현한다.

본 SPEC이 통과한 시점에서 `RateLimitTracker`는:

- `Parse(provider, headers)`로 응답 헤더를 수신하여 4개 bucket의 `{limit, remaining, reset_seconds, captured_at}`을 갱신하고,
- `State(provider)`로 각 provider별 현재 상태 스냅샷을 O(1)로 조회할 수 있으며,
- `UsagePct()`가 80% 이상 bucket에 대해 WARN 로그 1회(쿨다운 적용) + `Event`를 관찰자에게 전달하고,
- provider 간 상이한 헤더 네이밍(OpenAI/Anthropic/OpenRouter/Nous)을 정규화된 내부 구조체로 변환하며,
- `RemainingSecondsNow()`, `Used()`, `UsagePct()` 등 유도 속성을 제공한다.

본 SPEC은 **헤더 파싱 규칙 + 버킷 상태 머신 + 임계치 이벤트**만 규정한다. 실제 backoff/retry는 CREDPOOL-001의 `MarkExhaustedAndRotate` 경로로 처리. 라우팅 변경은 ROUTER-001(정적)·후속 SPEC(동적).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- ROADMAP v2.0 Phase 1 row 08은 RATELIMIT-001을 `CREDPOOL-001` 이후 P0로 배치. 실제 호출을 수행하는 ADAPTER-001이 호출 직후 헤더를 본 tracker에 넘겨 버킷을 갱신하는 구조.
- `.moai/project/research/hermes-llm.md` §5는 Hermes의 `RateLimitBucket` + `RateLimitState` 구조와 헤더 파싱 정책을 Go 포팅 매핑(§9)과 함께 제시. "80% 사용률 경고"는 사용자가 Retry-After 없이도 고갈을 미리 알아차리게 하는 핵심 기능.
- v4.0 UX 목표(goose ask 중 "⚠️ Anthropic rate limit 82% used, resets in 34s" 같은 안내)의 데이터 원천.

### 2.2 상속 자산

- **Hermes Agent Python**: `rate_limit_tracker.py`의 `RateLimitBucket`, `RateLimitState`, `parse_headers`, `format_display`.
- **OpenAI/Anthropic 헤더 스펙**:
  - `x-ratelimit-limit-requests`, `x-ratelimit-remaining-requests`, `x-ratelimit-reset-requests`
  - `x-ratelimit-limit-tokens`, `x-ratelimit-remaining-tokens`, `x-ratelimit-reset-tokens`
  - Anthropic은 `anthropic-ratelimit-*` prefix 사용 (정규화 필요)

### 2.3 범위 경계

- **IN**: 4-bucket 구조, 헤더 파싱 + 정규화(provider별), 80% 임계치 이벤트, human-readable display, 쿨다운된 WARN 로그, Observer 훅.
- **OUT**: 실제 HTTP 호출(ADAPTER-001), 429 처리 자동 wait(호출자가 CREDPOOL `MarkExhaustedAndRotate`로 처리), pre-emptive throttling(호출 전 wait 여부 결정)은 Phase 1 범위 외, prometheus export(후속 메트릭 SPEC).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/llm/ratelimit/` 패키지: `RateLimitBucket`, `RateLimitState`, `Tracker`, `Parser`, `Event`.
2. 4 bucket 정의: `requests_min`, `requests_hour`, `tokens_min`, `tokens_hour`.
3. `Bucket`의 유도 속성: `Used() int`, `UsagePct() float64`, `RemainingSecondsNow() float64`.
4. Provider별 헤더 파싱기 3종 — `OpenAIParser`(OpenAI-compat 표준 구현), `AnthropicParser`(anthropic-* prefix 정규화), `OpenRouterParser`(OpenAI 슬림 래퍼).
5. **OpenAI-compat 재사용 대상**: xAI/DeepSeek/Groq는 별도 Parser를 정의하지 않고 `OpenAIParser`를 `ProviderRegistry`에서 공유 등록한다 (ROUTER-001 provider alias 경유).
6. **NousParser 범위**: Phase 1에서는 **완전 제외**. Nous Portal 응답 포맷이 §2.4 연구 단계에 있으므로 본 SPEC에서 stub조차 등록하지 않는다. Nous provider에 대한 `Parse()` 호출은 REQ-RL-010에 의해 `ErrParserNotRegistered`를 반환한다. (후속 SPEC에서 Nous Portal 응답 확정 후 추가)
7. `Tracker.Parse(provider, headers, now)`로 상태 갱신.
8. `Tracker.State(provider) RateLimitState` 읽기(copy 반환).
9. 설정 가능 임계치(기본 80%) 도달 시 `Event{Provider, BucketType, UsagePct, ResetIn}`을 옵서버로 발화. 옵서버는 zap WARN 로그 기본 구현 + 사용자 정의 hook.
10. 설정 가능 경고 쿨다운(기본 30초): 동일 provider×bucket 조합에 대해 쿨다운 윈도우 내 중복 경고 억제.
11. Human-readable display: `Tracker.Display(provider) string` — "requests_min: 120/1000 (12%), tokens_min: 50K/200K (25%), reset in 34s".

### 3.2 OUT OF SCOPE

- **호출 전 throttle wait**: 단순히 상태를 갱신/조회만. 호출 전 `remaining > 0` 판단은 ADAPTER-001 선택 사항.
- **429 자동 재시도**: CREDPOOL의 `MarkExhaustedAndRotate`로 위임.
- **다중 인스턴스 공유 상태**: 단일 `goosed` 프로세스 가정.
- **Prometheus/OpenTelemetry export**: 후속 메트릭 SPEC.
- **Nous Portal "Agent Key" TTL 기반 고유 limit 해석**: Phase 1 범위 외.
- **gRPC quotas**: 본 SPEC은 HTTP provider 전용.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-RL-001 [Ubiquitous]** — The `Tracker` **shall** maintain one `RateLimitState` per provider, keyed by canonical provider name; state snapshots returned via `State()` **shall** be copies (read-only by contract).

**REQ-RL-002 [Ubiquitous]** — Each `RateLimitBucket` **shall** expose `Limit`, `Remaining`, `ResetSeconds`, `CapturedAt`; the `Used()` derived property **shall** never return a negative value, and the `UsagePct()` derived property **shall** return `0.0` when `Limit == 0`.

**REQ-RL-003 [Ubiquitous]** — The `Tracker` **shall** be safe for concurrent `Parse`, `State`, and `Display` invocations from multiple goroutines; implementation mechanism is specified in §6.

### 4.2 Event-Driven

**REQ-RL-004 [Event-Driven]** — **When** `Parse(provider, headers, now)` is invoked, the tracker **shall** (a) resolve the Parser registered for `provider`, (b) extract 4 buckets from headers per the parser's rules, (c) atomically replace the provider's `RateLimitState`, (d) evaluate the **configured threshold** (`opts.ThresholdPct`, default 80.0) per bucket, (e) emit `Event` to observers for each bucket that newly exceeded the configured threshold since last parse.

**REQ-RL-005 [Event-Driven]** — **When** a bucket's `UsagePct()` is at or above the configured threshold and the same provider×bucket combination emitted an Event within the **configured cooldown window** (`opts.WarnCooldown`, default 30s), the tracker **shall** suppress the Event.

**REQ-RL-006 [Event-Driven]** — **When** a header value cannot be parsed (malformed integer, unknown reset format), the tracker **shall** log a DEBUG message and treat that bucket as `{0, 0, 0, now}` without failing the entire parse.

### 4.3 State-Driven

**REQ-RL-007a [State-Driven]** — **While** `Bucket.CapturedAt + Bucket.ResetSeconds < now` (the bucket is considered stale), `RemainingSecondsNow()` **shall** return `0.0`.

**REQ-RL-007b [State-Driven]** — **While** a bucket is stale (per REQ-RL-007a), `Display()` **shall** emit the literal marker `[STALE]` adjacent to that bucket's rendering; the underlying `UsagePct()` value **shall** continue to be reported unchanged.

**REQ-RL-008 [State-Driven]** — **While** no parse has occurred for a provider, `State(provider)` **shall** return a zero-value `RateLimitState` with `IsEmpty() == true`; callers **shall** treat this as "no rate limit information yet".

### 4.4 Unwanted Behavior

**REQ-RL-009 [Unwanted]** — **If** an internal code path attempts to mutate the input `headers` map during `Parse()`, **then** the tracker **shall** abort the parse and return an error without committing any state change (contract: header reads are non-destructive).

**REQ-RL-010 [Unwanted]** — **If** a parser is not registered for the requested provider, **then** `Parse()` **shall** return `ErrParserNotRegistered{provider: ...}` without modifying any state.

**REQ-RL-011a [Unwanted]** — **If** `Parse()` is invoked with a nil `headers` map, **then** the tracker **shall** log a DEBUG message, leave the provider state unchanged, and return without panic.

**REQ-RL-011b [Unwanted]** — **If** `TrackerOptions.Observers` is nil or empty at the time an Event would be dispatched, **then** the tracker **shall** skip observer dispatch (logger WARN still fires per REQ-RL-004) and return without panic.

**REQ-RL-011c [Unwanted]** — **If** `Parse()` is invoked with a zero-value `now` (i.e., `time.Time{}`), **then** the tracker **shall** log a DEBUG message, leave the provider state unchanged, and return without panic.

### 4.5 Optional

**REQ-RL-012 [Optional]** — **Where** `TrackerOptions.Observers` is non-empty, each observer **shall** receive Events in registration order; errors returned by observers **shall** be logged at WARN but **shall not** halt dispatch to remaining observers.

**REQ-RL-013 [Optional]** — **Where** `TrackerOptions.ThresholdPct` is configured, that value **shall** be used as the threshold in place of the 80.0 default (see REQ-RL-004/005). If the configured value is outside the inclusive range `[50.0, 100.0]`, `New()` **shall** reject construction with a validation error (no silent clamping; no partial construction).

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-RL-001 — 정상 파싱 (OpenAI 헤더)** [covers REQ-RL-001, REQ-RL-002, REQ-RL-004]
- **Given** 빈 Tracker, OpenAI parser 등록, 헤더 `{"x-ratelimit-limit-requests":"1000", "x-ratelimit-remaining-requests":"800", "x-ratelimit-reset-requests":"60"}` (+ tokens 동등)
- **When** `Parse("openai", headers, now)`
- **Then** `State("openai").RequestsMin == {Limit:1000, Remaining:800, ResetSeconds:60, CapturedAt:now}`, `UsagePct() == 20.0`, Event 발화 없음

**AC-RL-002 — 설정 임계치 초과 시 Event 발화 (기본 80%)** [covers REQ-RL-004]
- **Given** OpenAI parser, 옵서버 spy, `ThresholdPct` 기본값(80.0) 사용
- **When** `Parse`가 `requests_min` remaining 150/1000으로 갱신(사용률 85%)
- **Then** spy가 `Event{Provider:"openai", BucketType:"requests_min", UsagePct:85.0, ResetIn:xx}` 1회 수신, zap WARN 로그 1건

**AC-RL-003 — 쿨다운 적용된 중복 억제 (기본 30s)** [covers REQ-RL-005]
- **Given** AC-RL-002 상황 직후, `WarnCooldown` 기본값(30s) 사용
- **When** 10초 후 동일 상황으로 다시 `Parse`
- **Then** 옵서버 추가 호출 없음, 35초 후 다시 `Parse`하면 옵서버 1회 호출

**AC-RL-004 — Anthropic prefix 정규화 (결정적 now 고정)** [covers REQ-RL-001, REQ-RL-004]
- **Given** Anthropic parser, `now = 2026-04-21T11:59:34Z` (고정), 헤더 `{"anthropic-ratelimit-requests-limit":"500", "anthropic-ratelimit-requests-remaining":"400", "anthropic-ratelimit-requests-reset":"2026-04-21T12:00:00Z"}` (ISO 8601 reset, now + 26s)
- **When** `Parse("anthropic", headers, now)`
- **Then** `State("anthropic").RequestsMin.ResetSeconds == 26.0` (허용 오차 `±0.001`)

**AC-RL-005 — Malformed 헤더 graceful** [covers REQ-RL-006]
- **Given** OpenAI parser, 헤더 `{"x-ratelimit-limit-requests":"abc", "x-ratelimit-remaining-requests":"800", "x-ratelimit-reset-requests":"60"}` (limit만 잘못된 int)
- **When** `Parse`
- **Then** 에러 반환 없음, `RequestsMin.Limit == 0` 및 `{0, 0, 0, now}` zero-value, DEBUG 로그 1건, 다른 bucket(tokens 등)은 정상 파싱

**AC-RL-006 — Stale bucket 표시 ([STALE] 고정 마커)** [covers REQ-RL-007a, REQ-RL-007b]
- **Given** State에 `{Limit:1000, Remaining:200, ResetSeconds: 60, CapturedAt: now-120s}`
- **When** `Display("openai")`
- **Then** 출력 문자열에 정확히 `[STALE]` 서브스트링이 포함됨, `RemainingSecondsNow(now) == 0.0`, `UsagePct() == 80.0` (stale이어도 값은 계속 계산됨)

**AC-RL-007 — 병렬 Parse 경쟁 (결정적 invariant)** [covers REQ-RL-003]
- **Given** 동일 provider에 대해 100개 goroutine이 각기 다른 `remaining ∈ {0, 1, ..., 99}` 값(Limit=1000 고정)으로 동시에 `Parse`
- **When** 모든 goroutine 완료
- **Then** (1) race detector 통과, (2) `State("openai").RequestsMin.Limit == 1000`, (3) `State("openai").RequestsMin.Remaining`은 입력 100개 값 중 하나 (`0..99` 범위 내), (4) 어떤 goroutine도 panic하지 않음, (5) torn write 없음(Limit+Remaining이 서로 다른 Parse 호출에서 혼합되지 않음)

**AC-RL-008 — 미파싱 provider의 IsEmpty** [covers REQ-RL-008]
- **Given** 빈 Tracker, OpenAI parser 등록됨, OpenAI에 대한 `Parse()` 호출 한 번도 없음
- **When** `State("openai")` 조회
- **Then** 반환값 `IsEmpty() == true`, 모든 4개 bucket이 zero-value, `Display("openai")`는 사람 가독한 "no rate limit information yet" 상태 표현

**AC-RL-009 — 미등록 Parser에 대한 Parse (ErrParserNotRegistered + 상태 불변)** [covers REQ-RL-010]
- **Given** Tracker에 OpenAI parser만 등록, `State("anthropic")` 사전 조회하여 initial snapshot 저장
- **When** `Parse("anthropic", headers, now)` 호출
- **Then** 반환된 에러가 `ErrParserNotRegistered`와 일치(provider 필드 `"anthropic"`), 이후 `State("anthropic")`이 사전 initial snapshot과 동일(내부 state map에 `"anthropic"` 엔트리가 신설되지 않음), OpenAI state도 영향 없음

**AC-RL-010 — nil 입력에 대한 방어** [covers REQ-RL-011a, REQ-RL-011b, REQ-RL-011c]
- **Case A (nil headers)**: `Parse("openai", nil, now)` 호출 시 panic 없음, DEBUG 로그 1건, `State("openai").IsEmpty() == true` 유지
- **Case B (nil/empty observer list)**: `TrackerOptions.Observers == nil`로 Tracker 생성 후 임계치 초과 `Parse` 시 panic 없음, zap WARN 로그만 발생, observer 호출 없음
- **Case C (zero-time now)**: `Parse("openai", headers, time.Time{})` 호출 시 panic 없음, DEBUG 로그 1건, provider state 불변

**AC-RL-011 — Observer 순서 보존 및 에러 격리** [covers REQ-RL-012]
- **Given** OpenAI parser, 3개 observer 등록 순서: `[obs1, obs2, obs3]`. `obs2`는 `OnRateLimitEvent`에서 에러 반환 또는 panic 대신 에러값 반환
- **When** 임계치 초과 `Parse` 1회
- **Then** 호출 순서 기록은 `[obs1, obs2, obs3]` (obs3가 건너뛰어지지 않음), obs2 에러는 WARN 로그로 남음, Tracker의 `lastWarn`은 정상 갱신(쿨다운 기록됨)

**AC-RL-012 — ThresholdPct 경계 검증** [covers REQ-RL-013]
- **Case A (유효)**: `TrackerOptions{ThresholdPct: 75.0}`로 `New()` 호출 시 에러 없이 Tracker 반환, 75% 사용률에서 Event 발화
- **Case B (유효 경계)**: `ThresholdPct: 50.0` 및 `ThresholdPct: 100.0` 모두 `New()` 성공
- **Case C (하한 위반)**: `ThresholdPct: 49.9`로 `New()` 호출 시 validation 에러 반환, Tracker 인스턴스는 `nil`(부분 생성 금지)
- **Case D (상한 위반)**: `ThresholdPct: 100.1`로 `New()` 호출 시 validation 에러 반환, Tracker 인스턴스 `nil`

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/llm/ratelimit/
├── tracker.go          # Tracker + Parse/State/Display
├── tracker_test.go
├── bucket.go           # RateLimitBucket + RateLimitState + 유도 속성
├── parser.go           # Parser interface + 공통 유틸
├── parser_openai.go    # OpenAI + compat (xAI/DeepSeek/Groq 재사용)
├── parser_anthropic.go # Anthropic 정규화
├── parser_openrouter.go# OpenRouter (OpenAI 슬림 래퍼)
├── event.go            # Event + Observer interface
├── display.go          # Human-readable formatter
└── errors.go           # ErrParserNotRegistered
```

### 6.2 핵심 타입

```go
// internal/llm/ratelimit/bucket.go

type RateLimitBucket struct {
    Limit         int
    Remaining     int
    ResetSeconds  float64
    CapturedAt    time.Time
}

func (b RateLimitBucket) Used() int
func (b RateLimitBucket) UsagePct() float64 // 0.0 when Limit==0
func (b RateLimitBucket) RemainingSecondsNow(now time.Time) float64
func (b RateLimitBucket) IsStale(now time.Time) bool

type RateLimitState struct {
    Provider     string
    RequestsMin  RateLimitBucket
    RequestsHour RateLimitBucket
    TokensMin    RateLimitBucket
    TokensHour   RateLimitBucket
    CapturedAt   time.Time
}

func (s RateLimitState) IsEmpty() bool
```

```go
// internal/llm/ratelimit/tracker.go

type TrackerOptions struct {
    Parsers        map[string]Parser // provider → parser
    Observers      []Observer
    ThresholdPct   float64 // 기본 80.0
    WarnCooldown   time.Duration // 기본 30s
    Logger         *zap.Logger
    Clock          func() time.Time
}

type Tracker struct {
    opts     TrackerOptions
    mu       sync.RWMutex
    states   map[string]*RateLimitState
    lastWarn map[string]map[string]time.Time // provider → bucketType → 마지막 warn 시간
}

func New(opts TrackerOptions) *Tracker

func (t *Tracker) Parse(provider string, headers map[string]string, now time.Time) error

func (t *Tracker) State(provider string) RateLimitState

func (t *Tracker) Display(provider string) string
```

```go
// internal/llm/ratelimit/parser.go

type Parser interface {
    Provider() string
    Parse(headers map[string]string, now time.Time) (RateLimitState, error)
}

// CaseInsensitiveGet은 헤더 lookup 헬퍼 (HTTP 헤더 case-insensitive).
func CaseInsensitiveGet(headers map[string]string, key string) (string, bool)
```

```go
// internal/llm/ratelimit/event.go

type Event struct {
    Provider   string
    BucketType string // "requests_min" | "requests_hour" | "tokens_min" | "tokens_hour"
    UsagePct   float64
    ResetIn    time.Duration
    At         time.Time
}

type Observer interface {
    OnRateLimitEvent(e Event)
}
```

### 6.3 OpenAI 헤더 매핑

```
x-ratelimit-limit-requests      → RequestsMin.Limit
x-ratelimit-remaining-requests  → RequestsMin.Remaining
x-ratelimit-reset-requests      → RequestsMin.ResetSeconds (duration format: "60s", "1m30s")

x-ratelimit-limit-tokens        → TokensMin.Limit
x-ratelimit-remaining-tokens    → TokensMin.Remaining
x-ratelimit-reset-tokens        → TokensMin.ResetSeconds
```

Hour buckets는 OpenAI가 반환하지 않으므로 zero-value 유지.

### 6.4 Anthropic 헤더 매핑

```
anthropic-ratelimit-requests-limit      → RequestsMin.Limit
anthropic-ratelimit-requests-remaining  → RequestsMin.Remaining
anthropic-ratelimit-requests-reset      → ISO 8601 timestamp → (reset - now).Seconds()

anthropic-ratelimit-tokens-limit        → TokensMin.Limit
...
```

Anthropic은 ISO 8601 reset을 사용하므로 파서가 `time.Parse(time.RFC3339, v)` → `Sub(now)` 변환.

### 6.5 OpenRouter 헤더 매핑

OpenAI와 동일. `parser_openrouter.go`는 `parser_openai.go`의 wrapper.

### 6.6 임계치 평가 알고리즘

```
parseAndEmit(provider, state, now):
  for each bucket in {RequestsMin, RequestsHour, TokensMin, TokensHour}:
    pct = bucket.UsagePct()
    if pct >= opts.ThresholdPct:
      lastWarnAt = lastWarn[provider][bucketType] or zero
      if now - lastWarnAt >= opts.WarnCooldown:
        event = Event{provider, bucketType, pct, bucket.RemainingSecondsNow(now), now}
        for obs in opts.Observers:
          obs.OnRateLimitEvent(event)
        logger.Warn("rate_limit_threshold_exceeded", ...)
        lastWarn[provider][bucketType] = now
```

### 6.7 TDD 진입 순서

1. **RED #1**: `TestBucket_UsagePct_ZeroLimit` — 경계 조건 (REQ-RL-002).
2. **RED #2**: `TestOpenAIParser_HappyPath` — AC-RL-001.
3. **RED #3**: `TestTracker_ThresholdEventEmittedOnce` — AC-RL-002.
4. **RED #4**: `TestTracker_CooldownSuppressesDuplicate_ThenFiresAfterWindow` — AC-RL-003.
5. **RED #5**: `TestAnthropicParser_ISO8601ResetNormalized_DeterministicNow` — AC-RL-004.
6. **RED #6**: `TestParser_MalformedHeader_ZeroValue` — AC-RL-005.
7. **RED #7**: `TestBucket_StaleDetection_DisplayContainsSTALEMarker` — AC-RL-006 (REQ-RL-007a/007b).
8. **RED #8**: `TestTracker_ConcurrentParse_RaceDetectorPasses_WithInvariants` — AC-RL-007.
9. **RED #9**: `TestTracker_IsEmptyOnUnseenProvider` — AC-RL-008 (REQ-RL-008).
10. **RED #10**: `TestTracker_ErrParserNotRegistered_NoStateMutation` — AC-RL-009 (REQ-RL-010).
11. **RED #11**: `TestTracker_NilInputs_DoNotPanic` — AC-RL-010 (REQ-RL-011a/011b/011c).
12. **RED #12**: `TestTracker_ObserverOrder_ErrorIsolation` — AC-RL-011 (REQ-RL-012).
13. **RED #13**: `TestTracker_ThresholdPctBounds_ValidationAtNew` — AC-RL-012 (REQ-RL-013).
14. **GREEN**: 최소 구현.
15. **REFACTOR**: 파서 공통 코드(`CaseInsensitiveGet`, `parseIntOrZero`, `parseDurationSeconds`) 추출.

### 6.8 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| Tested | 85%+ 커버리지, 테이블 주도 파서 테스트(provider별 헤더 fixture), race detector |
| Readable | `Parser` 인터페이스 분리, `Bucket` 유도 속성 method receiver |
| Unified | go fmt + golangci-lint, 헤더 lookup은 항상 `CaseInsensitiveGet` 경유 |
| Secured | 헤더 값은 `string` 그대로 저장(민감 정보 아님), 로그에 provider만 기록 |
| Trackable | 모든 Event에 zap 구조화 로그(`{provider, bucket, pct, reset_in}`) |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger |
| 선행 SPEC | SPEC-GOOSE-CREDPOOL-001 | 429 시 호출자가 tracker state를 참조 후 `MarkExhaustedAndRotate(retry_after)` 결정 |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-001 | HTTP 응답 완료 후 `Tracker.Parse(provider, headers, now)` 호출 |
| 외부 | Go 1.22+ | |
| 외부 | `go.uber.org/zap` | |
| 외부 | stdlib `net/http`, `time` | |
| 외부 | `github.com/stretchr/testify` | |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Provider가 헤더 스펙을 변경 (예: OpenAI가 `reset`을 ISO로 변경) | 중 | 중 | Parser 단위 분리, 변경 시 해당 parser만 수정. 통합 테스트가 snapshot 고정 |
| R2 | 기본 임계치(80%)가 일부 사용자에겐 과도/부족 | 중 | 낮 | `opts.ThresholdPct` 설정(REQ-RL-013, 범위 [50,100]) |
| R3 | 기본 쿨다운(30s)이 짧은 간격 호출에서 event storm/과도 억제 | 낮 | 낮 | `opts.WarnCooldown` 옵션화(REQ-RL-005), 테스트로 검증 |
| R4 | 동시성 시 `lastWarn` map race | 고 | 중 | `sync.RWMutex` + Parse 내부에서 write lock |
| R5 | 시계 되감김 → `RemainingSecondsNow()` 음수 | 낮 | 낮 | `max(0, ...)` 적용 |
| R6 | Nous Portal 헤더 차이 | 중 | 낮 | Phase 1은 3 parser만(OpenAI/Anthropic/OpenRouter). Nous는 후속 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/hermes-llm.md` §5 Rate Limit Tracker, §9 Go 포팅 매핑, §10 SPEC 도출
- `.moai/specs/ROADMAP.md` §4 Phase 1 row 08
- `.moai/specs/SPEC-GOOSE-CREDPOOL-001/spec.md` — 429 경로 통합
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` — ProviderRegistry 이름 정규화 공유

### 9.2 외부 참조

- **OpenAI rate limit 문서**: https://platform.openai.com/docs/guides/rate-limits
- **Anthropic rate limit 헤더**: https://docs.anthropic.com/claude/reference/rate-limits
- **OpenRouter**: https://openrouter.ai/docs/limits
- **Hermes Agent Python**: `./hermes-agent-main/agent/rate_limit_tracker.py` — 원형

### 9.3 부속 문서

- `./research.md` — 각 provider 헤더 fixture + 파서 상세 매핑 + 테스트 전략

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **실제 HTTP 호출/재시도/backoff를 구현하지 않는다**. 헤더 파싱/상태 유지만.
- 본 SPEC은 **호출 전 pre-emptive throttle wait을 수행하지 않는다**. 상태 조회는 read-only.
- 본 SPEC은 **429 자동 처리를 포함하지 않는다**. 호출자(ADAPTER-001)가 tracker state 참조 후 CREDPOOL의 `MarkExhaustedAndRotate`로 위임.
- 본 SPEC은 **circuit breaker / failure isolation을 포함하지 않는다**. 연속 429/5xx 시 credential rotation은 **CREDPOOL-001** 소관, 재시도 backoff는 **ADAPTER-001** 소관. 본 Tracker는 "상태를 기록하고 관찰자에게 통지"하는 passive role만 수행한다.
- 본 SPEC은 **Prometheus/OTel export를 포함하지 않는다**. 후속 메트릭 SPEC.
- 본 SPEC은 **다중 goosed 인스턴스 공유 상태를 포함하지 않는다**. 단일 프로세스 가정.
- 본 SPEC은 **Nous Portal 헤더 파싱을 포함하지 않는다**. 현재 Phase 1 parser는 OpenAI/Anthropic/OpenRouter 3종만 등록; Nous는 후속 SPEC에서 추가.
- 본 SPEC은 **Nous Portal Agent Key TTL 추적을 구현하지 않는다**. 후속.
- 본 SPEC은 **gRPC 서비스 quotas를 다루지 않는다**. HTTP provider 전용.
- 본 SPEC은 **overage(limit 초과) 예측/비용 알림을 포함하지 않는다**. 메트릭 SPEC.
- 본 SPEC은 **로컬 quota 관리(token bucket / leaky bucket / sliding window)를 포함하지 않는다**. 본 Tracker는 provider 응답 헤더를 기반으로 한 passive reflector이며, 클라이언트 측 자체 quota 계산 알고리즘은 구현하지 않는다.

---

**End of SPEC-GOOSE-RATELIMIT-001**
