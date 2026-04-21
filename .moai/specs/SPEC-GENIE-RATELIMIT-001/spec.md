---
id: SPEC-GENIE-RATELIMIT-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 소(S)
lifecycle: spec-anchored
---

# SPEC-GENIE-RATELIMIT-001 — Rate Limit Tracker (RPM/TPM/RPH/TPH 4 Bucket)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-llm.md §5 + ROADMAP v2.0 Phase 1 기반) | manager-spec |

---

## 1. 개요 (Overview)

GENIE-AGENT Phase 1의 **provider 응답 헤더 기반 rate limit 추적 레이어**를 정의한다. Hermes Agent의 `rate_limit_tracker.py`(~243 LoC)를 Go로 포팅하여, 각 provider의 HTTP 응답 헤더(`x-ratelimit-*`)를 파싱하고 4-bucket(`requests_min/hour`, `tokens_min/hour`) 상태를 유지하며 80% 임계치 초과 시 경고를 발화하는 `internal/llm/ratelimit` 패키지를 구현한다.

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
- v4.0 UX 목표(genie ask 중 "⚠️ Anthropic rate limit 82% used, resets in 34s" 같은 안내)의 데이터 원천.

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
4. Provider별 헤더 파싱기 3종 — `OpenAIParser`(OpenAI-compat), `AnthropicParser`(anthropic-* prefix), `OpenRouterParser`(OpenAI-compat). `NousParser`는 metadata-only(실 구현은 Nous Portal 응답 후속).
5. `Tracker.Parse(provider, headers, now)`로 상태 갱신.
6. `Tracker.State(provider) RateLimitState` 읽기(copy 반환).
7. 80% 임계치 도달 시 `Event{Provider, BucketType, UsagePct, ResetIn}`을 옵서버로 발화. 옵서버는 zap WARN 로그 기본 구현 + 사용자 정의 hook.
8. 경고 쿨다운: 동일 provider×bucket 조합에 대해 30초 내 중복 경고 억제.
9. Human-readable display: `Tracker.Display(provider) string` — "requests_min: 120/1000 (12%), tokens_min: 50K/200K (25%), reset in 34s".

### 3.2 OUT OF SCOPE

- **호출 전 throttle wait**: 단순히 상태를 갱신/조회만. 호출 전 `remaining > 0` 판단은 ADAPTER-001 선택 사항.
- **429 자동 재시도**: CREDPOOL의 `MarkExhaustedAndRotate`로 위임.
- **다중 인스턴스 공유 상태**: 단일 `genied` 프로세스 가정.
- **Prometheus/OpenTelemetry export**: 후속 메트릭 SPEC.
- **Nous Portal "Agent Key" TTL 기반 고유 limit 해석**: Phase 1 범위 외.
- **gRPC quotas**: 본 SPEC은 HTTP provider 전용.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-RL-001 [Ubiquitous]** — The `Tracker` **shall** maintain one `RateLimitState` per provider, keyed by canonical provider name; state snapshots returned via `State()` **shall** be copies (read-only by contract).

**REQ-RL-002 [Ubiquitous]** — Each `RateLimitBucket` **shall** expose `Limit int`, `Remaining int`, `ResetSeconds float64`, `CapturedAt time.Time`; derived `Used() = max(0, Limit - Remaining)` and `UsagePct() = (Used/Limit)*100` (zero-limit returns 0).

**REQ-RL-003 [Ubiquitous]** — The `Tracker` **shall** be safe for concurrent `Parse`/`State`/`Display` calls from multiple goroutines using `sync.RWMutex`.

### 4.2 Event-Driven

**REQ-RL-004 [Event-Driven]** — **When** `Parse(provider, headers, now)` is invoked, the tracker **shall** (a) resolve the Parser registered for `provider`, (b) extract 4 buckets from headers per the parser's rules, (c) atomically replace the provider's `RateLimitState`, (d) evaluate 80% threshold per bucket, (e) emit `Event` to observers for each bucket that newly exceeded 80% since last parse.

**REQ-RL-005 [Event-Driven]** — **When** a bucket's `UsagePct() >= 80.0` and the same bucket emitted an Event less than 30 seconds ago, the tracker **shall** suppress the Event (cooldown applied).

**REQ-RL-006 [Event-Driven]** — **When** a header value cannot be parsed (malformed integer, unknown reset format), the tracker **shall** log a DEBUG message and treat that bucket as `{0, 0, 0, now}` without failing the entire parse.

### 4.3 State-Driven

**REQ-RL-007 [State-Driven]** — **While** `Bucket.CapturedAt + Bucket.ResetSeconds < now`, the bucket is considered stale; `RemainingSecondsNow()` **shall** return `0.0`, and `UsagePct()` is reported but flagged as stale in `Display()`.

**REQ-RL-008 [State-Driven]** — **While** no parse has occurred for a provider, `State(provider)` **shall** return a zero-value `RateLimitState` with `IsEmpty() == true`; callers **shall** treat this as "no rate limit information yet".

### 4.4 Unwanted Behavior

**REQ-RL-009 [Unwanted]** — The tracker **shall not** mutate input `headers` map; all reads are non-destructive.

**REQ-RL-010 [Unwanted]** — **If** a parser is not registered for the requested provider, **then** `Parse()` **shall** return `ErrParserNotRegistered{provider: ...}` without modifying any state.

**REQ-RL-011 [Unwanted]** — The tracker **shall not** panic on nil headers, nil observer list, or zero-time `now`; it logs at DEBUG and returns.

### 4.5 Optional

**REQ-RL-012 [Optional]** — **Where** `TrackerOptions.Observers` is non-empty, each observer **shall** receive Events in registration order; errors returned by observers are logged but do not halt dispatch.

**REQ-RL-013 [Optional]** — **Where** `TrackerOptions.ThresholdPct` is configured, that value **shall** be used in place of the 80% default; value must be between 50.0 and 100.0 inclusive.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-RL-001 — 정상 파싱 (OpenAI 헤더)**
- **Given** 빈 Tracker, OpenAI parser 등록, 헤더 `{"x-ratelimit-limit-requests":"1000", "x-ratelimit-remaining-requests":"800", "x-ratelimit-reset-requests":"60"}` (+ tokens 동등)
- **When** `Parse("openai", headers, now)`
- **Then** `State("openai").RequestsMin == {Limit:1000, Remaining:800, ResetSeconds:60, CapturedAt:now}`, `UsagePct() == 20.0`, Event 발화 없음

**AC-RL-002 — 80% 초과 시 Event 발화**
- **Given** OpenAI parser, 옵서버 spy
- **When** `Parse`가 `requests_min` remaining 150/1000으로 갱신(사용률 85%)
- **Then** spy가 `Event{Provider:"openai", BucketType:"requests_min", UsagePct:85.0, ResetIn:xx}` 1회 수신, zap WARN 로그 1건

**AC-RL-003 — 쿨다운 적용된 중복 억제**
- **Given** AC-RL-002 상황 직후
- **When** 10초 후 동일 상황으로 다시 `Parse`
- **Then** 옵서버 추가 호출 없음 (30초 쿨다운)

**AC-RL-004 — Anthropic prefix 정규화**
- **Given** Anthropic parser, 헤더 `{"anthropic-ratelimit-requests-limit":"500", "anthropic-ratelimit-requests-remaining":"400", "anthropic-ratelimit-requests-reset":"2026-04-21T12:00:00Z"}` (ISO 8601 reset)
- **When** `Parse("anthropic", headers, now)`
- **Then** `State("anthropic").RequestsMin.ResetSeconds`가 `(reset - now).Seconds()`로 정확히 계산됨

**AC-RL-005 — Malformed 헤더 graceful**
- **Given** OpenAI parser, 헤더 `{"x-ratelimit-limit-requests":"abc"}` (잘못된 int)
- **When** `Parse`
- **Then** 에러 반환 없음, 해당 bucket은 zero-value, DEBUG 로그 1건, 다른 bucket은 정상 파싱

**AC-RL-006 — Stale bucket 표시**
- **Given** State에 `{ResetSeconds: 60, CapturedAt: now-120s}`
- **When** `Display("openai")`
- **Then** 출력 문자열에 "stale" 또는 "reset passed" 표시, `RemainingSecondsNow() == 0`

**AC-RL-007 — 병렬 Parse 경쟁**
- **Given** 동일 provider에 대해 100개 goroutine이 각기 다른 remaining 값으로 Parse
- **When** 모두 완료
- **Then** race detector 통과, 최종 `State` 일관성(마지막 완료된 Parse의 값이 유지), Event 순서 무관

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

1. **RED #1**: `TestBucket_UsagePct_ZeroLimit` — 경계 조건.
2. **RED #2**: `TestOpenAIParser_HappyPath` — AC-RL-001.
3. **RED #3**: `TestTracker_ThresholdEventEmittedOnce` — AC-RL-002.
4. **RED #4**: `TestTracker_CooldownSuppressesDuplicate` — AC-RL-003.
5. **RED #5**: `TestAnthropicParser_ISO8601ResetNormalized` — AC-RL-004.
6. **RED #6**: `TestParser_MalformedHeader_ZeroValue` — AC-RL-005.
7. **RED #7**: `TestBucket_StaleDetection` — AC-RL-006.
8. **RED #8**: `TestTracker_ConcurrentParse_RaceDetectorPasses` — AC-RL-007.
9. **GREEN**: 최소 구현.
10. **REFACTOR**: 파서 공통 코드(`CaseInsensitiveGet`, `parseIntOrZero`, `parseDurationSeconds`) 추출.

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
| 선행 SPEC | SPEC-GENIE-CORE-001 | zap logger |
| 선행 SPEC | SPEC-GENIE-CREDPOOL-001 | 429 시 호출자가 tracker state를 참조 후 `MarkExhaustedAndRotate(retry_after)` 결정 |
| 후속 SPEC | SPEC-GENIE-ADAPTER-001 | HTTP 응답 완료 후 `Tracker.Parse(provider, headers, now)` 호출 |
| 외부 | Go 1.22+ | |
| 외부 | `go.uber.org/zap` | |
| 외부 | stdlib `net/http`, `time` | |
| 외부 | `github.com/stretchr/testify` | |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Provider가 헤더 스펙을 변경 (예: OpenAI가 `reset`을 ISO로 변경) | 중 | 중 | Parser 단위 분리, 변경 시 해당 parser만 수정. 통합 테스트가 snapshot 고정 |
| R2 | 80% 임계치가 일부 사용자에겐 과도/부족 | 중 | 낮 | `ThresholdPct` 설정(REQ-RL-013) |
| R3 | WarnCooldown 30초가 짧은 간격 호출에서 event storm | 낮 | 낮 | 옵션화, 테스트로 검증 |
| R4 | 동시성 시 `lastWarn` map race | 고 | 중 | `sync.RWMutex` + Parse 내부에서 write lock |
| R5 | 시계 되감김 → `RemainingSecondsNow()` 음수 | 낮 | 낮 | `max(0, ...)` 적용 |
| R6 | Nous Portal 헤더 차이 | 중 | 낮 | Phase 1은 3 parser만(OpenAI/Anthropic/OpenRouter). Nous는 후속 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/hermes-llm.md` §5 Rate Limit Tracker, §9 Go 포팅 매핑, §10 SPEC 도출
- `.moai/specs/ROADMAP.md` §4 Phase 1 row 08
- `.moai/specs/SPEC-GENIE-CREDPOOL-001/spec.md` — 429 경로 통합
- `.moai/specs/SPEC-GENIE-ROUTER-001/spec.md` — ProviderRegistry 이름 정규화 공유

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
- 본 SPEC은 **Prometheus/OTel export를 포함하지 않는다**. 후속 메트릭 SPEC.
- 본 SPEC은 **다중 genied 인스턴스 공유 상태를 포함하지 않는다**. 단일 프로세스 가정.
- 본 SPEC은 **Nous Portal Agent Key TTL 추적을 구현하지 않는다**. 후속.
- 본 SPEC은 **gRPC 서비스 quotas를 다루지 않는다**. HTTP provider 전용.
- 본 SPEC은 **overage(limit 초과) 예측/비용 알림을 포함하지 않는다**. 메트릭 SPEC.

---

**End of SPEC-GENIE-RATELIMIT-001**
