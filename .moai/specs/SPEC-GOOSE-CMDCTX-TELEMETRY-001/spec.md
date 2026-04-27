---
id: SPEC-GOOSE-CMDCTX-TELEMETRY-001
version: 0.1.0
status: planned
created_at: 2026-04-27
updated_at: 2026-04-27
author: manager-spec
priority: P3
issue_number: null
phase: 3
size: 중(M)
lifecycle: spec-anchored
labels: [area/runtime, area/observability, type/feature, priority/p3-low]
---

# SPEC-GOOSE-CMDCTX-TELEMETRY-001 — ContextAdapter 호출 카운트 / Latency Metrics Emission

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. CMDCTX-001 (implemented v0.1.1) 의 ContextAdapter 6개 메서드(`OnClear`, `OnCompactRequest`, `OnModelChange`, `ResolveModelAlias`, `SessionSnapshot`, `PlanModeActive`)에 호출 카운트 / latency / 실패율 metrics emission wiring SPEC 신설. 선행 metrics 인프라 부재 → §RISKS R1 1급 risk 명시. CMDCTX-001 v0.2.0 (또는 v0.3.0) amendment 동시 발생. priority P3 (low) — observability 가치는 있지만 사용자 직접 노출 안 됨. | manager-spec |

---

## 1. 개요 (Overview)

CMDCTX-001 의 `ContextAdapter` 는 `command.SlashCommandContext` 의 6개 메서드를 구현하지만, 그 메서드들의 호출 빈도 / latency / 실패율을 수집하는 metrics emission 이 부재하다. 본 SPEC 은 그 wiring 을 추가한다.

본 SPEC 수락 시점에서:

- `internal/command/adapter/` 에 `MetricsSink` interface 가 정의되거나 (선행 SPEC 의 sink 를 import).
- `adapter.Options` 에 `Metrics MetricsSink` 필드가 존재.
- 6개 메서드 모두 호출 시 `cmdctx.method.calls` counter 가 +1.
- error 를 반환하는 4개 메서드 (`OnClear`, `OnCompactRequest`, `OnModelChange`, `ResolveModelAlias`) 는 error 발생 시 `cmdctx.method.errors{error_type=...}` counter 가 +1.
- 6개 메서드 모두 `cmdctx.method.duration_ms` histogram 에 latency observation.
- `Options.Metrics == nil` 또는 `MetricsSink == nil` 일 때 모든 emission 이 skip 되며 메서드는 정상 동작.
- emission 자체의 panic 이 메서드의 정상 동작을 깨지 않음 (defer recover).
- `WithContext(ctx)` 가 만든 child adapter 도 부모와 동일 sink 인스턴스 공유.

본 SPEC 은 **adapter 측 emission 만** 정의한다. metrics 백엔드 (OTel SDK / Prometheus exporter / expvar handler) 는 선행 SPEC `SPEC-GOOSE-OBS-METRICS-001` (TBD, 부재) 의 책임.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **observability 결손**: `cmdctx.*` layer 는 dispatcher 의 결정 path 에 위치한다. user 가 `/clear` / `/compact` / `/model` 을 얼마나 자주 사용하는지, alias resolution 실패율이 얼마인지, plan mode toggle 빈도가 얼마인지를 모르면 product/UX 결정에 데이터가 없다.
- **debugging 가치**: `ErrLoopControllerUnavailable` 또는 `ErrUnknownModel` 의 누적 카운트는 wiring 결손 / config 실수의 조기 신호.
- **hot path latency 보증**: `PlanModeActive` 와 `ResolveModelAlias` 는 dispatcher hot path. 이 메서드의 latency p95/p99 가 갑자기 늘어나면 (예: registry lock 경합) 즉시 감지해야 한다.

### 2.2 상속 자산

- **SPEC-GOOSE-CMDCTX-001** (implemented v0.1.1, FROZEN-by-amendment): `ContextAdapter`, `Options`, 6개 메서드. 본 SPEC 의 amendment 대상. **본 SPEC 의 implementation 시점에 v0.1.1 → v0.2.0 (또는 다른 amendment 와 합쳐 v0.3.0) 으로 동시 갱신**.
- **SPEC-GOOSE-OBS-METRICS-001** (**TBD — 부재**): 본 SPEC 의 prerequisite. metrics sink interface 와 backend (OTel/expvar/Prometheus 중 1) 정의. **본 SPEC 은 이 SPEC 의 implemented status 충족 후에만 implementation 가능**.

### 2.3 범위 경계 (한 줄)

- **IN**: `MetricsSink` interface (또는 선행 SPEC 의 sink import), `Options.Metrics` 필드, 6개 메서드의 calls / errors / duration emission, nil sink fast-path, panic safety, fake sink 기반 unit test, race detector.
- **OUT**: metrics 백엔드 SDK 초기화, exporter 설정, distributed tracing, 다른 패키지 (router / loop / subagent) 의 emission, alerting 정의, query API.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC 이 정의/구현하는 것)

1. **`MetricsSink` interface** — `internal/command/adapter/metrics.go` (신규 파일) 또는 선행 SPEC 의 sink 를 그대로 alias.
   - `Counter(name string, labels Labels) Counter` — counter handle 반환.
   - `Histogram(name string, labels Labels) Histogram` — histogram handle 반환.
   - `Counter.Inc()`, `Counter.Add(delta float64)`.
   - `Histogram.Observe(value float64)`.
   - `Labels = map[string]string`.
   - 모든 메서드는 thread-safe (선행 SPEC 의 sink 가 보장).

2. **`adapter.Options.Metrics` 필드 추가**:
   ```go
   type Options struct {
       Registry       *router.ProviderRegistry
       LoopController LoopController
       AliasMap       map[string]string
       GetwdFn        func() (string, error)
       Logger         Logger
       Metrics        MetricsSink // [NEW] optional, may be nil
   }
   ```

3. **`adapter.ContextAdapter` 에 `metrics MetricsSink` 필드 추가** + `New(opts)` 의 wiring.

4. **`WithContext(ctx)` 의 invariant 보존**: shallow copy 로 child 가 부모와 동일 `metrics` 인스턴스 공유. 추가 sync 코드 불필요 (sink 자체가 thread-safe).

5. **6개 메서드의 emission wiring**:
   - `OnClear`, `OnCompactRequest`, `OnModelChange`: instrumentErr helper 경유.
   - `ResolveModelAlias`: instrumentErr helper 경유 (returns `(*ModelInfo, error)` 이므로 별도 헬퍼 또는 inline).
   - `SessionSnapshot`: instrumentVoid helper 경유 (return 값만 반환, error 없음).
   - `PlanModeActive`: instrumentVoid helper 경유 (bool 반환, error 없음).

6. **emit metrics**:
   - `cmdctx.method.calls{method=...}` — counter, 메서드 진입 직후 +1.
   - `cmdctx.method.errors{method=..., error_type=...}` — counter, error 반환 직전 +1. error_type ∈ {`ErrUnknownModel`, `ErrLoopControllerUnavailable`, `other`}.
   - `cmdctx.method.duration_ms{method=...}` — histogram, defer 로 메서드 exit 시 observe.

7. **nil sink fast-path**: `a.metrics == nil` 또는 `opts.Metrics == nil` 일 때 emission 코드 skip, 메서드는 emission 없이 정상 동작.

8. **panic safety**: metrics sink 의 `Counter()` / `Histogram()` / `Inc()` / `Observe()` 가 panic 던져도 메서드 본문은 정상 결과 반환 (defer recover).

9. **CMDCTX-001 v0.x.0 amendment**:
   - `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` frontmatter `version: 0.1.1` → `0.2.0` (또는 다른 amendment 와 합쳐 `0.3.0`).
   - `updated_at` 갱신.
   - HISTORY 항목 1줄 추가: "본 SPEC 의 amendment, ContextAdapter 6개 메서드에 metrics emission 추가, Options.Metrics 필드 신설".
   - §6 데이터 모델 §6.x Options 절에 `Metrics MetricsSink` 필드 명시 추가 (필드만, sink interface 정의는 본 SPEC 에).
   - §Exclusions 의 placeholder (`metrics emission`) 제거 (본 SPEC ID 로 갈음).

10. **테스트**:
   - `adapter/metrics_test.go` (신규) 또는 `adapter_test.go` 확장 — fake sink 기반 emission unit test (8개 케이스, §6.1 참고).
   - `race_test.go` 확장 — concurrent emission race detector.
   - `Options.Metrics == nil` 의 모든 메서드에 대한 zero-emission 검증.

### 3.2 OUT OF SCOPE

- **선행 SPEC `SPEC-GOOSE-OBS-METRICS-001` 의 작성/구현** (별도 SPEC).
- **OTel SDK / exporter / Prometheus push gateway / expvar HTTP handler 설정** (선행 SPEC).
- **distributed tracing** (별도 `SPEC-GOOSE-TRACING-001` TBD).
- **다른 패키지의 metrics emission** (router, loop, subagent, query — 별도 SPEC).
- **alerting 정의** (slow call alert, error rate alert — backend alert manager 측).
- **metric query/aggregation API** (PromQL, OTLP query — backend 측).
- **histogram bucket 결정** (선행 SPEC 또는 본 SPEC v0.2.0 amendment).

---

## 4. EARS 요구사항 (Requirements)

EARS 5 패턴: Ubiquitous (항상), Event-Driven (WHEN), State-Driven (WHILE/IF), Unwanted (SHALL NOT), Optional (WHERE).

### 4.1 Ubiquitous — 항상 활성

**REQ-CMDCTX-TEL-001**: `ContextAdapter` 의 모든 6개 메서드는 `Options.Metrics != nil` 일 때 호출 즉시 `cmdctx.method.calls{method=<메서드명>}` counter 를 +1 한다.

**REQ-CMDCTX-TEL-002**: `ContextAdapter` 의 모든 6개 메서드는 `Options.Metrics != nil` 일 때 메서드 exit 시 `cmdctx.method.duration_ms{method=<메서드명>}` histogram 에 실행 시간 (millisecond) 을 1회 observe 한다.

**REQ-CMDCTX-TEL-003**: `ContextAdapter` 의 6개 메서드의 시그니처 (이름, 파라미터, 반환 타입) 는 본 SPEC 의 amendment 로도 변경되지 않는다. 본 SPEC 은 메서드 본문만 확장한다.

**REQ-CMDCTX-TEL-004**: `MetricsSink` interface 는 `Counter(name, labels)` 와 `Histogram(name, labels)` 두 factory 메서드를 노출하고, 반환된 handle 은 `Inc()` / `Add(float64)` (counter) / `Observe(float64)` (histogram) 메서드를 갖는다.

**REQ-CMDCTX-TEL-005**: `WithContext(ctx)` 가 반환한 child adapter 는 부모와 동일한 `metrics MetricsSink` 인스턴스를 공유한다. metrics 필드는 shallow copy 로 복사된다 (REQ-CMDCTX-005 / REQ-CMDCTX-011 의 invariant 와 호환).

### 4.2 Event-Driven — 트리거 기반

**REQ-CMDCTX-TEL-006**: `OnClear`, `OnCompactRequest`, `OnModelChange`, `ResolveModelAlias` 가 `error != nil` 을 반환하기 직전, `Options.Metrics != nil` 일 때, `cmdctx.method.errors{method=<메서드명>, error_type=<분류>}` counter 를 +1 한다.

**REQ-CMDCTX-TEL-007**: error_type 라벨은 다음 분류 규칙으로 결정된다:
- `errors.Is(err, ErrUnknownModel)` 이 true → `"ErrUnknownModel"`.
- `errors.Is(err, ErrLoopControllerUnavailable)` 이 true → `"ErrLoopControllerUnavailable"`.
- 위 둘 모두 false → `"other"` (label cardinality 폭발 방지).

**REQ-CMDCTX-TEL-008**: `New(opts Options)` 호출 시 `opts.Metrics` 가 `MetricsSink` 의 비-nil 구현체이면, 그 인스턴스를 `ContextAdapter.metrics` 필드에 저장한다.

### 4.3 State-Driven — 조건 기반

**REQ-CMDCTX-TEL-009**: `IF Options.Metrics == nil OR ContextAdapter.metrics == nil`, THEN 모든 6개 메서드는 emission 코드를 skip 하고 정상 동작한다 (graceful degradation, REQ-CMDCTX-014 패턴 일관).

**REQ-CMDCTX-TEL-010**: `WHILE` 메서드가 실행 중인 동안, `cmdctx.method.duration_ms` 측정의 시작점은 메서드 진입 직후 (counter +1 직후), 종료점은 return 직전 (defer 실행 시) 이다. counter 의 emission 비용은 측정 범위에 포함된다 (단순화).

### 4.4 Unwanted — 금지/방지

**REQ-CMDCTX-TEL-011**: `MetricsSink` 의 `Counter()` / `Histogram()` / `Inc()` / `Add()` / `Observe()` 호출이 panic 을 발생시켜도, `ContextAdapter` 의 메서드는 panic 을 caller 에게 전파하지 않고 정상 결과를 반환한다 (defer recover). 이때 `Options.Logger != nil` 이면 `Logger.Warn("metrics emission panic", "panic", r)` 를 1회 호출한다.

**REQ-CMDCTX-TEL-012**: `ContextAdapter` 의 6개 메서드 본문 (emission 외) 의 동작은 본 SPEC 의 amendment 로 변경되지 않는다. 즉, CMDCTX-001 v0.1.1 의 19 AC 는 모두 보존된다.

**REQ-CMDCTX-TEL-013**: error_type 라벨에 임의의 동적 값 (예: `err.Error()` 문자열) 을 넣지 않는다. cardinality 폭발 방지를 위해 §4.2 REQ-CMDCTX-TEL-007 의 3-tier 분류만 허용된다.

**REQ-CMDCTX-TEL-014**: emission 은 `Counter().Inc()` / `Histogram().Observe()` 의 1회 호출만 허용. 동일 메서드 호출 내에서 동일 counter 를 다중 increment 하지 않는다.

### 4.5 Optional — 권장사항 (운영 환경에 따라)

**REQ-CMDCTX-TEL-015** (Optional): `WHERE` 운영 환경에 OTel SDK 가 설정되어 있다면, `MetricsSink` 의 backend 는 OTel `metric.Meter` 어댑터로 구성될 수 있다. 본 SPEC 은 backend 를 강제하지 않는다.

**REQ-CMDCTX-TEL-016** (Optional): `WHERE` p95 / p99 latency sampling 이 backend 측에서 지원된다면, `cmdctx.method.duration_ms` histogram 의 bucket 은 운영자가 정의할 수 있다. 본 SPEC 은 bucket 을 강제하지 않는다 (선행 SPEC 또는 backend 기본값 사용).

**REQ-CMDCTX-TEL-017** (Optional): `WHERE` slow call alerting 이 backend 측에서 지원된다면, `cmdctx.method.duration_ms{method=PlanModeActive}` 의 p99 가 임계값 (예: 1ms) 초과 시 alarm 발생을 운영자가 설정할 수 있다. 본 SPEC 은 alert 를 정의하지 않는다.

**REQ-CMDCTX-TEL-018** (Optional, 임시 대안): `WHERE` 선행 metrics SPEC 이 implemented 상태가 아닐 때, `Options.Metrics == nil` 인 상태에서도 `Options.Logger != nil` 이면 메서드 exit 시 `Logger.Debug("cmdctx.method.call", "method", ..., "duration_ms", ...)` 를 호출하는 fallback emission 을 둘 수 있다. 이는 진정한 metric 이 아닌 structured event log 임을 명시. **본 REQ 는 P3 우선순위 / Optional 이며, run phase 에서 user 가 활성화 여부를 결정한다.**

---

## 5. 수락 기준 (Acceptance Criteria)

### 5.1 unit / integration 테스트 검증

**AC-CMDCTX-TEL-001**: `internal/command/adapter/metrics.go` (또는 선행 SPEC sink import) 가 존재하고, `MetricsSink`, `Counter`, `Histogram`, `Labels` 타입이 정의되어 있다. (REQ-CMDCTX-TEL-004)

**AC-CMDCTX-TEL-002**: `adapter.Options` 에 `Metrics MetricsSink` 필드가 추가되어 있고, `adapter.ContextAdapter` 에 `metrics MetricsSink` 필드가 존재한다. `New(opts)` 가 `opts.Metrics` 를 그 필드에 저장한다. (REQ-CMDCTX-TEL-008)

**AC-CMDCTX-TEL-003**: `TestMetrics_OnClear_CountsAndDuration` 테스트는 (a) `OnClear` 호출 시 fake sink 의 `cmdctx.method.calls{method=OnClear}` counter 가 정확히 1, (b) `cmdctx.method.duration_ms{method=OnClear}` histogram 에 정확히 1개 observation 이 기록됨을 검증한다. (REQ-CMDCTX-TEL-001, REQ-CMDCTX-TEL-002)

**AC-CMDCTX-TEL-004**: `TestMetrics_OnClear_NilLoopCtrl_ErrorCounter` 테스트는 `loopCtrl=nil` 인 상태에서 `OnClear()` 가 `ErrLoopControllerUnavailable` 반환 시 fake sink 의 `cmdctx.method.errors{method=OnClear, error_type=ErrLoopControllerUnavailable}` counter 가 +1 됨을 검증한다. (REQ-CMDCTX-TEL-006, REQ-CMDCTX-TEL-007)

**AC-CMDCTX-TEL-005**: `TestMetrics_ResolveModelAlias_Unknown_ErrorCounter` 테스트는 unknown alias 입력 시 `cmdctx.method.errors{method=ResolveModelAlias, error_type=ErrUnknownModel}` counter +1 됨을 검증한다. (REQ-CMDCTX-TEL-007)

**AC-CMDCTX-TEL-006**: `TestMetrics_OnModelChange_OtherError` 테스트는 `LoopController.RequestModelChange` 가 `ErrUnknownModel` / `ErrLoopControllerUnavailable` 외 임의의 error 를 반환할 때, `cmdctx.method.errors{method=OnModelChange, error_type=other}` counter +1 됨을 검증한다. (REQ-CMDCTX-TEL-007 의 `other` fallback)

**AC-CMDCTX-TEL-007**: `TestMetrics_PlanModeActive_HotPath` 테스트는 `PlanModeActive()` 100회 호출 후 fake sink 의 `cmdctx.method.calls{method=PlanModeActive}` counter 값이 정확히 100 이며, `errors` counter 는 PlanModeActive 라벨로 0 임을 검증한다 (PlanModeActive 는 error 반환 안 함). (REQ-CMDCTX-TEL-001, REQ-CMDCTX-TEL-002)

**AC-CMDCTX-TEL-008**: `TestMetrics_NilSink_NoOp` 테스트는 `Options.Metrics = nil` 인 상태에서 6개 메서드 모두 호출 시 panic 없이 정상 동작하며, 별도의 fake sink probe (예: `&panicSink{}` 가 호출되었는지 확인) 가 호출되지 않음을 검증한다. (REQ-CMDCTX-TEL-009)

**AC-CMDCTX-TEL-009**: `TestMetrics_PanicInSink_DoesNotBreakMethod` 테스트는 fake sink 의 `Counter().Inc()` 가 panic 던지는 시나리오에서 `OnClear()` 가 panic 을 caller 에게 전파하지 않고 정상 (또는 underlying error 반환) 동작함을 검증한다. `Options.Logger` 가 주입되어 있다면 `Logger.Warn("metrics emission panic", ...)` 호출이 1회 기록됨도 검증한다. (REQ-CMDCTX-TEL-011)

**AC-CMDCTX-TEL-010**: `TestMetrics_WithContext_ChildSharesSink` 테스트는 `parent := New(Options{Metrics: sink, ...})` 와 `child := parent.WithContext(ctx)` 에 대해, child 의 `OnClear()` 호출이 부모와 동일 sink 의 counter 를 증가시킴을 검증한다 (즉, 동일 sink 인스턴스 공유). (REQ-CMDCTX-TEL-005)

**AC-CMDCTX-TEL-011**: `TestRace_Metrics_ConcurrentEmission` 테스트는 100 goroutine 이 6개 메서드를 동시 호출하는 시나리오에서 `go test -race` 를 통과하며, 모든 counter 의 누적값이 deterministic 하게 일치함을 검증한다 (sink 가 thread-safe 한 fake 일 때). (REQ-CMDCTX-TEL-005)

**AC-CMDCTX-TEL-012**: `TestMetrics_DurationOrder` 테스트는 `OnClear()` 의 `duration_ms` observation 이 0 이상이고, monotonic clock (`time.Since`) 기반임을 검증한다 (negative duration 미발생). (REQ-CMDCTX-TEL-002, REQ-CMDCTX-TEL-010)

### 5.2 정적 / 빌드 검증

**AC-CMDCTX-TEL-013**: `go vet ./internal/command/adapter/...` 통과. `go build ./internal/command/adapter/...` 통과. `golangci-lint run ./internal/command/adapter/...` 통과 (warning 0건).

**AC-CMDCTX-TEL-014**: `internal/command/adapter` 의 패키지 cyclomatic complexity (추가된 instrument 헬퍼 포함) 가 본 SPEC 추가 전 대비 5 미만 증가. (TRUST 5 — Readable)

**AC-CMDCTX-TEL-015**: `internal/command/adapter` 패키지의 line coverage 가 본 SPEC 추가 후 85% 이상 유지. (TRUST 5 — Tested, project-wide standard)

### 5.3 invariant / amendment 검증

**AC-CMDCTX-TEL-016**: `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` 의 frontmatter `version` 이 `0.1.1` 에서 `0.2.0` (또는 다른 amendment 와 합쳐 `0.3.0`) 으로 갱신되었고, `updated_at` 이 본 SPEC implementation 일자로 갱신되었으며, HISTORY 에 본 SPEC 의 amendment 항목 1줄이 추가되었다. CMDCTX-001 의 19 AC 는 모두 텍스트 보존된다 (REQ-CMDCTX-TEL-012).

**AC-CMDCTX-TEL-017**: `internal/command/adapter/adapter.go` 의 6개 메서드 시그니처가 PR #52 머지 시점과 동일함을 `git diff PR#52..HEAD -- internal/command/adapter/adapter.go` 또는 정적 분석으로 검증. (REQ-CMDCTX-TEL-003)

**AC-CMDCTX-TEL-018**: error_type 라벨 값은 정적 분석 (e.g., `grep -rn "error_type" internal/command/adapter/`) 으로 정확히 3개 값 (`ErrUnknownModel`, `ErrLoopControllerUnavailable`, `other`) 만 사용됨을 검증. (REQ-CMDCTX-TEL-013)

---

## 6. 데이터 모델 (Data Model)

### 6.1 `MetricsSink` interface

```go
// internal/command/adapter/metrics.go (신규) 또는 선행 SPEC import
package adapter

// MetricsSink is the abstract metrics emission interface used by ContextAdapter.
// nil sink is permitted and triggers emission skip per REQ-CMDCTX-TEL-009.
//
// Implementations MUST be thread-safe: ContextAdapter spawns child adapters via
// WithContext(ctx) that share the same sink instance and may emit concurrently.
//
// @MX:ANCHOR: [AUTO] vendor-neutral metrics surface for cmdctx layer.
// @MX:REASON: Multiple backends candidates (OTel/Prometheus/expvar) — interface
//             abstraction prevents lock-in. fan_in >= 3 (adapter, tests, future
//             integration code).
// @MX:SPEC: SPEC-GOOSE-CMDCTX-TELEMETRY-001 REQ-CMDCTX-TEL-004
type MetricsSink interface {
    Counter(name string, labels Labels) Counter
    Histogram(name string, labels Labels) Histogram
}

// Counter is a monotonically increasing counter handle.
type Counter interface {
    Inc()
    Add(delta float64)
}

// Histogram is a value distribution observer.
type Histogram interface {
    Observe(value float64)
}

// Labels is a set of key-value tags applied to a metric series.
// Cardinality is bounded by REQ-CMDCTX-TEL-013 (no dynamic err.Error()).
type Labels map[string]string
```

### 6.2 `Options.Metrics` 필드 (CMDCTX-001 v0.x.0 amendment)

```go
type Options struct {
    Registry       *router.ProviderRegistry
    LoopController LoopController
    AliasMap       map[string]string
    GetwdFn        func() (string, error)
    Logger         Logger
    // [NEW] Metrics is the optional metrics emission sink.
    // nil sink triggers graceful skip per REQ-CMDCTX-TEL-009.
    Metrics MetricsSink
}
```

### 6.3 emission spec 표

| 메서드 | calls counter | errors counter (error_type 라벨) | duration histogram |
|--------|---------------|----------------------------------|--------------------|
| `OnClear` | ✅ +1 | ✅ ErrLoopControllerUnavailable / other | ✅ |
| `OnCompactRequest` | ✅ +1 | ✅ ErrLoopControllerUnavailable / other | ✅ |
| `OnModelChange` | ✅ +1 | ✅ ErrLoopControllerUnavailable / other | ✅ |
| `ResolveModelAlias` | ✅ +1 | ✅ ErrUnknownModel / other | ✅ |
| `SessionSnapshot` | ✅ +1 | (error 반환 없음) | ✅ |
| `PlanModeActive` | ✅ +1 | (error 반환 없음) | ✅ |

### 6.4 metric series 카탈로그

| series 이름 | 라벨 | cardinality |
|-------------|------|-------------|
| `cmdctx.method.calls` | `method` ∈ 6값 | 6 |
| `cmdctx.method.errors` | `method` ∈ 4값, `error_type` ∈ 3값 | 12 |
| `cmdctx.method.duration_ms` | `method` ∈ 6값 | 6 |
| **합계 series 수** | | **24** |

cardinality 가 매우 낮으므로 backend 부담 무시 가능.

---

## 7. 통합 (Integration)

### 7.1 SPEC 의존 관계 그래프

```
SPEC-GOOSE-CMDCTX-TELEMETRY-001 (본 SPEC, planned, P3)
  ├─ depends on: SPEC-GOOSE-CMDCTX-001 (implemented v0.1.1)
  │              → 본 SPEC 이 v0.2.0 (또는 v0.3.0) amendment 발생
  ├─ depends on: SPEC-GOOSE-OBS-METRICS-001 (TBD — 부재)
  │              → prerequisite, 본 SPEC implementation 의 blocker
  ├─ amendment-coupled with: SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 (planned, P4)
  │                          → 동일 base v0.1.1 위 amendment, 머지 충돌 가능성
  ├─ amendment-coupled with: SPEC-GOOSE-CMDCTX-HOTRELOAD-001 (TBD)
  │                          → 동일 base, 본 SPEC 후속
  ├─ informs: SPEC-GOOSE-CMDCTX-CLI-INTEG-001 (planned)
  │           → CLI 진입점에서 sink 주입 wiring 필요 (Options.Metrics)
  └─ informs: SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (planned)
              → daemon 진입점에서도 sink 주입 wiring 필요
```

### 7.2 CMDCTX-001 v0.x.0 amendment 변경 요약 (run phase 시점)

`.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` 변경 내용:

1. frontmatter `version: 0.1.1` → `0.2.0` (또는 다른 amendment 와 합쳐 `0.3.0`).
2. frontmatter `updated_at` 갱신.
3. HISTORY 에 다음 1줄 추가:
   ```
   | 0.2.0 | YYYY-MM-DD | SPEC-GOOSE-CMDCTX-TELEMETRY-001 amendment: ContextAdapter 6개 메서드에 metrics emission 추가. Options.Metrics 필드 신설. CMDCTX-001 의 19 AC 는 모두 보존, 신규 18 AC 는 TELEMETRY-001 거주. | manager-spec |
   ```
4. §6 데이터 모델 §6.x Options 절에 `Metrics MetricsSink` 필드 명시 추가 (인터페이스 정의는 본 SPEC 에 거주).
5. §Exclusions 의 placeholder (있다면) 갱신: `metrics emission` → `(see SPEC-GOOSE-CMDCTX-TELEMETRY-001, planned)`.

### 7.3 후속 SPEC 의 wiring 패치 요약

#### 7.3.1 SPEC-GOOSE-CMDCTX-CLI-INTEG-001 (planned)

CLI 진입점에서 `adapter.New(Options{...})` 호출 시 sink 주입:

```go
// internal/cli/app.go (예시 — CLI-INTEG-001 의 책임)
ctxAdapter := adapter.New(adapter.Options{
    Registry:       providerRegistry,
    LoopController: loopCtrl,
    AliasMap:       aliasMap,
    Logger:         logger,
    Metrics:        metricsSink, // [NEW] 선행 SPEC 의 sink 인스턴스
})
```

#### 7.3.2 SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (planned)

daemon 진입점도 동일 패턴 — `Options.Metrics` 에 sink 주입.

---

## 8. 비-기능 요구사항 (Non-Functional Requirements)

| NFR ID | 항목 | 목표 | 측정 |
|--------|------|------|------|
| NFR-CMDCTX-TEL-001 | 패키지 line coverage | ≥ 85% | `go test -cover ./internal/command/adapter/...` |
| NFR-CMDCTX-TEL-002 | race detector | 통과 | `go test -race ./internal/command/adapter/...` |
| NFR-CMDCTX-TEL-003 | nil sink fast-path overhead | ≤ 10ns 추가 | `BenchmarkPlanModeActive_NilSink` (vs 기존 baseline) |
| NFR-CMDCTX-TEL-004 | non-nil sink emission overhead | ≤ 200ns 추가 (PlanModeActive hot path) | `BenchmarkPlanModeActive_WithMetrics` |
| NFR-CMDCTX-TEL-005 | golangci-lint warning | 0건 | `golangci-lint run ./internal/command/adapter/...` |
| NFR-CMDCTX-TEL-006 | godoc 누락 | 모든 exported 심볼 godoc 보유 | `go doc -all` 수동 검토 |

NFR-CMDCTX-TEL-003 / 004 의 임계값은 plan phase 기준 추정. run phase benchmark 결과에 따라 조정 가능 (run phase 에서 amendment 1줄로 갱신).

---

## 9. 위험 (Risks)

| ID | 위험 | 우선순위 | 완화 전략 |
|----|------|----------|-----------|
| **R1** | **선행 metrics 인프라 SPEC `SPEC-GOOSE-OBS-METRICS-001` 부재 — 본 SPEC implementation 불가** | **🔴 높음 (blocker)** | (a) **권장 경로**: prerequisite SPEC 별도 신규 작성. backend 후보 (OTel / expvar / Prometheus) 결정 + sink interface 정의 + 최소 1개 backend adapter 구현. **본 SPEC 의 implementation 은 그 SPEC 의 implemented status 충족 후에만 진행**. (b) **임시 대안 경로**: 본 SPEC §4.5 REQ-CMDCTX-TEL-018 (Optional, Logger 기반 fallback) 만 구현. 진정한 metric 아닌 structured event log. cardinality / aggregation 미지원. P3 우선순위에서만 권장. (c) plan phase 의 본 SPEC 작성은 prerequisite 부재와 무관하게 진행 — spec contract 가 prerequisite SPEC 의 작성에도 입력으로 작용. |
| R2 | CMDCTX-001 v0.x.0 amendment 가 sibling amendment SPEC (PERMISSIVE-ALIAS-001, HOTRELOAD-001 (TBD)) 와 동일 base v0.1.1 위에서 동시 진행 시 머지 충돌 | 중 | (a) sibling amendment SPEC 가 먼저 머지되면 본 SPEC 의 amendment 는 다음 minor (예: v0.2.0 → v0.3.0) 위에 발생. governance 무결성 보존. (b) 본 SPEC §3.1 #9 / §7.2 가 amendment 범위를 명시 (HISTORY 1줄, frontmatter 2 필드, §6 1단락, §Exclusions 1줄). (c) amendment 가 implemented status 를 invalidate 하지 않음을 §7.2 에서 명시. |
| R3 | emission overhead 가 hot path latency 에 영향 (특히 `PlanModeActive` 매 dispatcher decision 마다 호출) | 중 | (a) nil sink fast-path 로 `Options.Metrics == nil` 시 zero-cost (NFR-CMDCTX-TEL-003 ≤ 10ns). (b) non-nil sink overhead 는 NFR-CMDCTX-TEL-004 ≤ 200ns 목표. (c) run phase 에서 `BenchmarkPlanModeActive_WithMetrics` 로 검증. (d) 임계값 초과 시 본 SPEC v0.2.0 amendment 로 fast-path 추가 (예: per-method emission 활성화 플래그). (e) 운영자 측 tuning: `PlanModeActive` emission 만 sampling (예: 1/100) 하는 옵션 — 본 SPEC v0.2.0 amendment 로 추가 가능. |
| R4 | error_type 라벨 cardinality 폭발 (미래 신규 error 타입 도입 시) | 낮음 | §4.4 REQ-CMDCTX-TEL-013 + §4.2 REQ-CMDCTX-TEL-007 의 3-tier 분류. 신규 error 타입 도입 시 본 SPEC v0.2.0 amendment 로 분류 추가 (예: `ErrInvalidAlias` 신설 시 amendment). label 값을 정적 enum 으로 강제 (AC-CMDCTX-TEL-018). |
| R5 | sink panic 이 메서드 정상 동작을 깨면 production 사고 | 낮음 | §4.4 REQ-CMDCTX-TEL-011 (defer recover). AC-CMDCTX-TEL-009 의 panic-injection 테스트로 보장. |
| R6 | `WithContext(ctx)` shallow copy 가 sink 공유 invariant 깨짐 | 낮음 | §4.1 REQ-CMDCTX-TEL-005 + AC-CMDCTX-TEL-010 의 child-shares-sink 테스트로 보장. CMDCTX-001 §6 의 invariant (atomic.Bool pointer sharing) 와 동일 패턴. |

### R1 추가 상세 — 선행 SPEC 부재 시 의사결정 흐름

```
Q1: 선행 SPEC SPEC-GOOSE-OBS-METRICS-001 가 implemented 인가?
├─ YES → 본 SPEC §4.1-4.4 normative REQ 모두 구현. §4.5 REQ-CMDCTX-TEL-018 (Logger fallback) 미구현.
└─ NO →
     Q2: 본 SPEC 을 P3 우선순위에서 임시 대안으로 진행할 것인가?
     ├─ YES → §4.5 REQ-CMDCTX-TEL-018 만 구현 (Logger.Debug fallback). normative REQ 는 prerequisite 충족까지 보류. 본 SPEC frontmatter status 는 partial 또는 별도 단계 정의 (manager-spec 협의).
     └─ NO → 본 SPEC 은 plan phase 에서 정지. 선행 SPEC 작성 후 본 SPEC run phase 진입.
```

권장 경로: 선행 SPEC 작성 → 본 SPEC normative 구현. P3 우선순위 정당화 (사용자 직접 노출 안 됨, 다른 SPEC 진행 막지 않음).

---

## Exclusions (What NOT to Build)

본 SPEC 이 명시적으로 다루지 않는 항목 (별도 SPEC 또는 운영 책임):

1. **metrics 백엔드 자체** (OTel SDK, Prometheus client_golang, expvar HTTP handler) — 선행 SPEC `SPEC-GOOSE-OBS-METRICS-001` (TBD) 의 책임.
2. **OTLP / Prometheus exporter 설정 / push gateway** — 선행 SPEC.
3. **distributed tracing** (span propagation, trace ID 주입) — 별도 `SPEC-GOOSE-TRACING-001` (TBD).
4. **다른 패키지 (router / loop / subagent / query / credential) 의 metrics emission** — 별도 SPEC. 본 SPEC 은 `internal/command/adapter` 만.
5. **alerting 정의** (slow call alert, error rate alert, dashboard JSON) — backend alert manager 측 운영 책임.
6. **metric query/aggregation API** (PromQL endpoint, OTLP query 인터페이스) — backend 측 운영 책임.
7. **histogram bucket 결정** — 선행 SPEC 또는 본 SPEC v0.2.0 amendment.
8. **CMDCTX-001 의 6개 메서드 본문 로직 변경** — 본 SPEC 은 emission wrapper 만 추가, 메서드 본문의 비-emission 로직은 보존 (REQ-CMDCTX-TEL-012).
9. **CLI / daemon 진입점에서의 sink 주입 wiring** — `SPEC-GOOSE-CMDCTX-CLI-INTEG-001` / `SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001` 의 책임. 본 SPEC 은 `Options.Metrics` 필드 contract 만 정의.
10. **사용자에게 직접 노출되는 metric dashboard / UI** — 본 SPEC 무관. backend 측 운영자가 Grafana / Datadog 등으로 구성.

---

## 10. 산출물 (Deliverables)

run phase 시점에 본 SPEC 의 implementation 산출물 (예상):

| 파일 | 변경 종류 | LOC 예상 |
|------|-----------|----------|
| `internal/command/adapter/metrics.go` | 신규 | ~80 (interface + helper + classifyError) |
| `internal/command/adapter/adapter.go` | 수정 | +40 ~ +60 (Options.Metrics, metrics field, 6 메서드 instrument 호출) |
| `internal/command/adapter/metrics_test.go` 또는 `adapter_test.go` 확장 | 신규/수정 | ~250 (8 테스트 케이스 + fakeMetricsSink) |
| `internal/command/adapter/race_test.go` | 수정 | +30 (concurrent emission race test) |
| `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` | amendment | +5 LOC (frontmatter 2필드, HISTORY 1줄, §6 1단락, §Exclusions 1줄) |
| `.moai/specs/SPEC-GOOSE-CMDCTX-TELEMETRY-001/progress.md` | 갱신 | run phase log |

총 신규/수정 ~400-500 LOC 예상 (test 포함). size 분류: **중(M)** 적절.

---

## 11. 참조 (References)

- 본 디렉토리: `.moai/specs/SPEC-GOOSE-CMDCTX-TELEMETRY-001/`
  - `research.md` — 선행 인프라 검증 + emission 설계 결정.
  - `progress.md` — phase log.
- 부모 SPEC: `.moai/specs/SPEC-GOOSE-CMDCTX-001/` (implemented v0.1.1, 본 SPEC 의 amendment 대상).
- 선행 SPEC (TBD, 부재): `SPEC-GOOSE-OBS-METRICS-001` — metrics sink interface 와 backend 정의.
- amendment-coupled SPEC: `SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001` (planned), `SPEC-GOOSE-CMDCTX-HOTRELOAD-001` (TBD).
- informed SPEC: `SPEC-GOOSE-CMDCTX-CLI-INTEG-001` (planned), `SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001` (planned).
- 부모 PR: PR #52 (SPEC-GOOSE-CMDCTX-001 implementation, c018ec5 / 6593705).
- 코드 참조 (read-only):
  - `internal/command/adapter/adapter.go` (PR #52 머지본, 본 SPEC 이 amend).
  - `internal/command/adapter/errors.go` (`ErrUnknownModel`, `ErrLoopControllerUnavailable` 정의).
  - `internal/command/context.go` (PR #50, `SlashCommandContext` / `ModelInfo` / `SessionSnapshot`).

---

Version: 0.1.0 (planned, P3, phase 3, size 중(M))
Last Updated: 2026-04-27
Status: planned — implementation 은 선행 SPEC `SPEC-GOOSE-OBS-METRICS-001` (TBD) implemented 후 시작.
