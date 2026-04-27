# SPEC-GOOSE-CMDCTX-TELEMETRY-001 — Research

> 본 SPEC 은 SPEC-GOOSE-CMDCTX-001 (implemented v0.1.1, FROZEN-by-amendment) 의 `ContextAdapter` 6개 메서드(`OnClear`, `OnCompactRequest`, `OnModelChange`, `ResolveModelAlias`, `SessionSnapshot`, `PlanModeActive`)에 호출 카운트 / latency / 실패율 metrics emission 을 추가하기 위한 wiring SPEC 이다. 본 research.md 는 (a) 본 레포의 metrics 인프라 존재 여부를 정밀 검증하고, (b) 검증 결과를 토대로 본 SPEC 의 implementation 가능 여부를 판정하며, (c) emission point 와 metric schema 를 제안한다.

---

## 0. 결론 요약 (Executive Summary)

| 항목 | 결과 |
|------|------|
| 본 레포의 metrics 인프라 존재 여부 | **❌ 부재** (선행 SPEC 작성 필요) |
| 선행 metrics SPEC (METRICS-* / OBS-* / TELEMETRY-*) 존재 | **❌ 없음** |
| 직접 의존성 (go.mod direct require) 의 metric 라이브러리 | **❌ 없음** |
| 간접 의존성 (indirect, transitively pulled) | ✅ `go.opentelemetry.io/otel v1.43.0`, `otel/metric v1.43.0` (단, 코드에서 직접 import 안 함) |
| 본 SPEC 의 즉시 implementation 가능 여부 | **❌ 불가** — 선행 metrics 인프라 SPEC (예: `SPEC-GOOSE-OBS-METRICS-001`) 우선 작성/구현 필요 |
| 본 SPEC 의 의의 | metrics 인프라가 준비되면 ContextAdapter 측 wiring 의 spec contract 로 작동. 또한 임시 대안(Logger.Warn 기반 emission)으로 P3 우선순위에서 부분 implementation 가능 |

본 SPEC 은 plan only 단계로 작성하되, §RISKS R1 에서 "선행 SPEC 필요" 를 1급 risk 로 명시하고, §6 데이터 모델은 추상 `MetricsSink` interface 로 정의해서 metrics 백엔드(OTel/Prometheus/expvar)와 무관한 형태로 유지한다.

---

## 1. 선행 인프라 검증 (CRITICAL — 본 SPEC implementation 가능 여부 결정)

### 1.1 검색 절차

본 research 는 다음 절차로 본 레포의 metrics 인프라 존재 여부를 검증했다.

#### 1.1.1 패키지 디렉토리 검색

```bash
find /Users/goos/MoAI/AI-Goose/internal -type d \
  -iname "*metric*" -o -iname "*telemetr*" -o -iname "*observ*"
```

결과: **0건**.

`internal/` 트리에서 `metrics`, `telemetry`, `observability` 디렉토리는 존재하지 않는다.

#### 1.1.2 import 기반 검색 — OpenTelemetry / Prometheus / expvar / statsd

```bash
grep -rn "go.opentelemetry.io/otel" internal --include="*.go"
grep -rn "github.com/prometheus" internal --include="*.go"
grep -rn "expvar\." internal --include="*.go"
grep -rn "statsd" internal --include="*.go"
```

결과: **모두 0건** (test 파일 포함).

본 레포의 `internal/` Go 코드는 어떤 metrics 라이브러리도 직접 import 하지 않는다.

#### 1.1.3 `go.mod` direct require 검증

```
require (
    ... (직접 의존성 — metrics 관련 0건)
)

require (
    ... (간접 의존성)
    go.opentelemetry.io/otel v1.43.0 // indirect
    go.opentelemetry.io/otel/metric v1.43.0 // indirect
    go.opentelemetry.io/otel/trace v1.43.0 // indirect
    go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.68.0 // indirect
    connectrpc.com/otelconnect v0.9.0 // indirect
)
```

OTel 패키지가 indirect 로 잡혀 있는 이유는 `connectrpc/otelconnect` 가 transport 레이어 instrumentation 을 위해 pull 한 것이며, 본 레포 코드에서 직접 사용되지 않는다.

따라서 **본 레포는 metrics 라이브러리를 도입한 적이 없다**.

#### 1.1.4 SPEC 디렉토리 검색 — 선행 metrics SPEC 존재 여부

```bash
ls .moai/specs/ | grep -iE "(metric|telemetr|observ)"
```

결과: **0건**.

추가 검색 (SPEC-GOOSE-INSIGHTS-001, SPEC-GOOSE-HEALTH-001 등 유관 후보):

| SPEC ID | 상태 | 본 SPEC 과의 관계 |
|---------|------|-------------------|
| SPEC-GOOSE-INSIGHTS-001 | planned (분석/리포트 산출) | 본 SPEC 과 무관 — runtime metrics emission 미정의 |
| SPEC-GOOSE-HEALTH-001 | planned (health endpoint) | 본 SPEC 과 무관 — alive/ready 신호만 |
| SPEC-GOOSE-JOURNAL-001 | planned (journal log) | 본 SPEC 과 무관 — append-only event log, not aggregated metric |
| SPEC-GOOSE-TRAJECTORY-001 | planned (trajectory recording) | 본 SPEC 과 무관 — 세션 단위 record, not real-time metric |
| **SPEC-GOOSE-OBS-METRICS-* / TELEMETRY-* / METRICS-*** | **❌ 부재** | **본 SPEC 의 prerequisite 이지만 미작성** |

#### 1.1.5 결정

본 레포에는 **metrics 인프라가 부재**하다. 본 SPEC 은 다음 두 경로 중 하나를 선택해야 한다:

- **경로 A (권장)**: 선행 SPEC `SPEC-GOOSE-OBS-METRICS-001` 을 별도 신규 작성하고, 본 SPEC 은 그것에 의존하는 wiring 으로 위치 정의. 본 SPEC 의 implementation 은 prerequisite 가 implemented 된 이후에만 가능.
- **경로 B (임시 대안)**: 본 SPEC 의 §3.2 Optional 절에 `Logger.Warn` 기반 임시 emission 을 두고, P3 우선순위에서 부분 implementation 진행. 단, 이는 진정한 metric (counter/histogram aggregation) 이 아니라 structured log line 일 뿐임을 명시.

본 SPEC 은 §RISKS R1 에서 두 경로를 동등하게 제시하고, run phase 시점에 user 가 선택하도록 한다. plan phase 의 본 research 에서는 spec contract 만 정의한다.

### 1.2 metrics 인프라 sketch (선행 SPEC 의 전제로서)

선행 SPEC 작성 시 참고할 수 있도록, metrics 인프라의 일반적인 구조를 sketch 한다. 본 SPEC 은 이 sketch 를 직접 정의하지 않는다 — 단지 본 SPEC 의 `MetricsSink` interface 가 호환되어야 하는 대상으로 참조한다.

#### 1.2.1 추상 sink interface (vendor-neutral)

```go
// 가상 패키지: github.com/modu-ai/goose/internal/obs/metrics
package metrics

type Sink interface {
    Counter(name string, labels Labels) Counter
    Histogram(name string, labels Labels) Histogram
    Gauge(name string, labels Labels) Gauge
}

type Counter interface {
    Inc()
    Add(delta float64)
}

type Histogram interface {
    Observe(value float64)
}

type Gauge interface {
    Set(value float64)
    Add(delta float64)
}

type Labels map[string]string
```

이 interface 는 OTel `metric.Meter`, Prometheus `CounterVec`/`HistogramVec`, expvar `Map` 모두 위에 어댑터를 씌워 구현 가능하다.

#### 1.2.2 backend 후보

| 백엔드 | 장점 | 단점 |
|--------|------|------|
| OpenTelemetry (OTel) | 표준 / vendor-neutral / 이미 indirect 로 jar | 직접 require 추가 필요, exporter 별도 (OTLP/Prometheus/Console) |
| Prometheus client_golang | 단순 / push-pull / 성숙 | OTel 에 비해 vendor lock-in, 간접 의존성 미존재 |
| expvar (stdlib) | 의존성 없음 / 즉시 사용 가능 | aggregation/exporter 부재, 단순 `/debug/vars` HTTP handler 만 제공 |
| 자체 sink (in-memory ring buffer) | 의존성 없음 | scale 못함, debugging 용 |

선행 SPEC 의 권장 선택지는 **OTel + console exporter (개발) / OTLP (프로덕션)**. 이미 indirect 로 들어와 있으므로 `go.mod` direct 화 비용이 낮다. 단, 본 SPEC 은 그 결정을 강제하지 않는다 — `MetricsSink` interface 는 모든 후보와 호환된다.

#### 1.2.3 emission 지점 패턴

ContextAdapter 와 같은 layer 에서 metrics 를 emission 할 때 권장 패턴:

```go
// pseudo-code (run phase 에 실제 코드로 변환)
func (a *ContextAdapter) OnClear() error {
    start := time.Now()
    if a.metrics != nil {
        a.metrics.Counter("cmdctx.method.calls",
            metrics.Labels{"method": "OnClear"}).Inc()
    }
    defer func() {
        if a.metrics != nil {
            a.metrics.Histogram("cmdctx.method.duration_ms",
                metrics.Labels{"method": "OnClear"}).Observe(
                float64(time.Since(start).Milliseconds()))
        }
    }()

    if a.loopCtrl == nil {
        if a.metrics != nil {
            a.metrics.Counter("cmdctx.method.errors", metrics.Labels{
                "method":     "OnClear",
                "error_type": "ErrLoopControllerUnavailable",
            }).Inc()
        }
        return ErrLoopControllerUnavailable
    }

    err := a.loopCtrl.RequestClear(a.effectiveCtx())
    if err != nil && a.metrics != nil {
        a.metrics.Counter("cmdctx.method.errors", metrics.Labels{
            "method":     "OnClear",
            "error_type": classifyError(err),
        }).Inc()
    }
    return err
}
```

본 SPEC 은 이 패턴을 권장하되, `time.Now()` overhead 와 nil-check overhead 는 hot path 비용으로 §RISKS R3 에 명시한다.

---

## 2. 측정 대상과 메트릭 schema

### 2.1 측정 대상 6개 메서드

`ContextAdapter` 가 구현하는 `command.SlashCommandContext` 의 6개 메서드 모두가 측정 대상이다.

| 메서드 | 호출 빈도 (예상) | latency 민감도 | error 분류 가치 |
|--------|------------------|----------------|------------------|
| `OnClear` | 낮음 (사용자 명시 호출) | 중 (LoopController 위임) | 높음 (debugging value) |
| `OnCompactRequest(target int)` | 중 (사용자 + auto-compact trigger) | 높음 (compactor latency) | 높음 |
| `OnModelChange(info)` | 매우 낮음 (사용자 명시 호출) | 낮음 | 중 |
| `ResolveModelAlias(alias)` | 높음 (매 user input 의 dispatcher path) | 높음 (hot path) | 중 (ErrUnknownModel 분류) |
| `SessionSnapshot()` | 중 (`/status` 명령 + telemetry sampling) | 중 (`os.Getwd` IO) | 낮음 |
| `PlanModeActive()` | 매우 높음 (매 dispatcher decision) | 매우 높음 (atomic.Load + context lookup) | 매우 낮음 (단순 bool) |

`PlanModeActive` 와 `ResolveModelAlias` 는 hot path 에 위치하므로 emission overhead 가 user-visible latency 에 영향을 줄 수 있다. §RISKS R3 와 §3.2 Optional 절의 fast-path 권장 사항을 참고.

### 2.2 메트릭 종류 (3종)

#### 2.2.1 Counter — `cmdctx.method.calls`

- 타입: counter (monotonic increasing)
- 라벨: `method` ∈ {OnClear, OnCompactRequest, OnModelChange, ResolveModelAlias, SessionSnapshot, PlanModeActive}
- 의미: 각 메서드가 호출된 누적 횟수
- emission 시점: 메서드 진입 직후 (이른 emission, error path 와 무관하게)

#### 2.2.2 Counter — `cmdctx.method.errors`

- 타입: counter (monotonic increasing)
- 라벨:
  - `method` ∈ {OnClear, OnCompactRequest, OnModelChange, ResolveModelAlias} — error 를 반환하는 메서드 4종만. `SessionSnapshot` 과 `PlanModeActive` 는 error 반환 안 함.
  - `error_type` ∈ {`ErrUnknownModel`, `ErrLoopControllerUnavailable`, `other`}
- 의미: 각 메서드의 에러 누적 횟수, error_type 으로 분류
- emission 시점: error 가 반환되기 직전
- error_type 분류 규칙:
  - `errors.Is(err, ErrUnknownModel)` → `"ErrUnknownModel"`
  - `errors.Is(err, ErrLoopControllerUnavailable)` → `"ErrLoopControllerUnavailable"`
  - 그 외 → `"other"` (label cardinality 폭발 방지)

#### 2.2.3 Histogram — `cmdctx.method.duration_ms`

- 타입: histogram (분포)
- 라벨: `method` (위와 동일 6종)
- 의미: 각 메서드 실행 시간 (millisecond)
- emission 시점: defer 로 메서드 exit 시
- bucket 권장값: 선행 metrics SPEC 이 정의 (예: OTel 기본 [0, 5, 10, 25, 50, 100, 250, 500, 1000] ms). 본 SPEC 은 bucket 을 강제하지 않는다.
- p50 / p95 / p99 sampling 은 sink (백엔드) 측 구현 책임 — 본 SPEC 은 단순히 raw observation 만 emit 한다.

### 2.3 cardinality 분석

| label | 값 cardinality | 곱 cardinality |
|-------|----------------|------------------|
| method (calls/duration) | 6 | 6 |
| (method, error_type) (errors) | 4 × 3 = 12 | 12 |

총 metric series: 6 (calls) + 12 (errors) + 6 (duration) = **24개 series** (MetricsSink 의 baseline metrics 외 추가분).

cardinality 가 매우 낮으므로 sink 백엔드 부담은 무시 가능.

---

## 3. emission 위치 — adapter wrapper 패턴 (CMDCTX-001 amendment)

### 3.1 결정: 메서드 내부 inline emission (decorator 미사용)

본 SPEC 은 `ContextAdapter` 의 6개 메서드 본문에 inline 으로 emission 코드를 추가한다. 이유:

1. **decorator 패턴 (별도 wrapping struct) 대안의 단점**:
   - `ContextAdapter` 는 여러 곳에서 instantiate 된다 (CLI 진입점, daemon, test). decorator 를 도입하면 모든 instantiation site 를 wrapping 해야 한다.
   - decorator 가 자체 nil-check 등을 갖춰야 해서 코드 복잡도 증가.
   - `WithContext(ctx)` 가 shallow copy 를 반환하는데 decorator 가 그 사이에 끼면 plan-mode pointer sharing 이 깨질 위험.

2. **inline 의 단점**:
   - adapter.go 본문이 길어진다 (현재 201 LOC → 약 280-320 LOC 예상).
   - 6개 메서드에 동일 패턴 (start time / nil-check / emission / defer) 이 반복.

3. **decision**: inline 으로 가되, 반복 코드를 `instrument(method string, fn func() error) error` 헬퍼로 묶는다. 이 헬퍼는 `adapter.go` 또는 `metrics.go` 새 파일에 놓는다.

### 3.2 helper 패턴 sketch

```go
// internal/command/adapter/metrics.go (신규)
// instrumentVoid wraps a void-returning method with metrics emission.
func (a *ContextAdapter) instrumentVoid(method string, fn func()) {
    if a.metrics == nil {
        fn()
        return
    }
    a.metrics.Counter("cmdctx.method.calls",
        metrics.Labels{"method": method}).Inc()
    start := time.Now()
    defer func() {
        a.metrics.Histogram("cmdctx.method.duration_ms",
            metrics.Labels{"method": method}).Observe(
            float64(time.Since(start).Milliseconds()))
    }()
    fn()
}

// instrumentErr wraps an error-returning method.
func (a *ContextAdapter) instrumentErr(method string, fn func() error) error {
    if a.metrics == nil {
        return fn()
    }
    a.metrics.Counter("cmdctx.method.calls",
        metrics.Labels{"method": method}).Inc()
    start := time.Now()
    defer func() {
        a.metrics.Histogram("cmdctx.method.duration_ms",
            metrics.Labels{"method": method}).Observe(
            float64(time.Since(start).Milliseconds()))
    }()
    err := fn()
    if err != nil {
        a.metrics.Counter("cmdctx.method.errors", metrics.Labels{
            "method":     method,
            "error_type": classifyError(err),
        }).Inc()
    }
    return err
}
```

`OnClear` 의 변경:

```go
func (a *ContextAdapter) OnClear() error {
    return a.instrumentErr("OnClear", func() error {
        if a.loopCtrl == nil {
            return ErrLoopControllerUnavailable
        }
        return a.loopCtrl.RequestClear(a.effectiveCtx())
    })
}
```

기존 nil 동작 보존됨. emission 추가는 `metrics == nil` fast-path 로 zero-cost (nil sink 일 때 helper 가 즉시 fn() 호출하고 반환).

### 3.3 panic safety — defer recover

`metrics` sink 가 잘못 구현되어 panic 을 발생시키면 ContextAdapter 메서드의 정상 동작이 깨진다. 이는 §3.2 Unwanted REQ 로 보장:

```go
func (a *ContextAdapter) safeEmit(fn func()) {
    defer func() {
        if r := recover(); r != nil {
            if a.logger != nil {
                a.logger.Warn("metrics emission panic", "panic", r)
            }
        }
    }()
    fn()
}
```

helper 는 emission 호출을 모두 `safeEmit` 로 감싼다. fn() (실제 메서드 본문) 은 wrapping 안 함 — fn 의 panic 은 caller 에게 전파.

### 3.4 CMDCTX-001 v0.2.0 amendment 영향 (CRITICAL)

본 SPEC 의 implementation 은 다음을 동시에 발생시킨다:

1. **adapter.go 변경**: 약 80-120 LOC 추가 (metrics field, instrument helper, safeEmit).
2. **adapter.Options 확장**: `Metrics MetricsSink` 필드 추가.
3. **CMDCTX-001 spec.md frontmatter 갱신**: `version: 0.1.1 → 0.2.0` (또는 다른 amendment SPEC 와 합쳐 `0.3.0` 으로 갈 가능성). `updated_at` 갱신. HISTORY 항목 1줄 추가.
4. **CMDCTX-001 §6 데이터 모델 §6.x Options 절에 Metrics 필드 명시 추가**.
5. **CMDCTX-001 §Exclusions 의 placeholder 갱신** (현재 `(planned, see TBD-METRICS-SPEC)` 같은 placeholder 가 있다면 본 SPEC ID 로 치환).

CMDCTX-001 v0.1.1 의 모든 19 AC 는 보존된다. 신규 AC 는 본 SPEC 에 거주.

이 amendment 가 sibling amendment SPEC (`SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001`, `SPEC-GOOSE-CMDCTX-HOTRELOAD-001` (아직 미작성), 본 SPEC) 과 동일한 base version (`v0.1.1`) 위에서 동시에 진행되면 머지 충돌 가능. §RISKS R2 참고.

---

## 4. 의존 SPEC 의 wiring surface

### 4.1 SPEC-GOOSE-CMDCTX-001 (implemented v0.1.1, FROZEN-by-amendment)

본 SPEC 의 amendment 대상이다. v0.2.0 (또는 v0.3.0) 으로 bump 된다.

본 SPEC 이 보존해야 하는 자산:

- `ContextAdapter` 의 6개 메서드 시그니처: 변경 금지.
- `Options` struct: 필드 추가만 가능, 기존 필드 변경/삭제 금지.
- `WithContext(ctx)` 의 shallow copy + `*atomic.Bool` planMode pointer sharing invariant: 변경 금지. metrics field 도 shallow copy 로 child 와 공유.
- `New(opts Options)` 의 nil graceful 동작: 변경 금지. `opts.Metrics == nil` 이면 emission skip.

### 4.2 선행 metrics SPEC (TBD — 본 SPEC 의 prerequisite)

이름 후보: `SPEC-GOOSE-OBS-METRICS-001` (권장) 또는 `SPEC-GOOSE-METRICS-001`.

선행 SPEC 이 정의해야 하는 surface:

- `metrics.Sink` interface (또는 동등 이름).
- `Counter` / `Histogram` / `Gauge` interface.
- `Labels` 타입.
- nil sink 를 허용하는 default 구현 (no-op).
- 최소 한 개의 backend 어댑터 (OTel / expvar 중 선택).

본 SPEC 의 `MetricsSink` (§6.1) 는 선행 SPEC 의 `metrics.Sink` 를 그대로 alias 하거나 본 SPEC 의 패키지 내에서 import 한다.

### 4.3 SPEC-GOOSE-CMDCTX-CLI-INTEG-001 (planned)

CLI 진입점에서 `adapter.New(Options{...})` 를 호출하는 SPEC. 본 SPEC 이 Options 에 `Metrics` 필드를 추가하면, CLI-INTEG-001 의 wiring 코드도 metrics sink 를 주입해야 한다. CLI-INTEG-001 의 §6 Options 패치 절에 본 SPEC 이 코멘트로 dependency 명시되면 충분.

### 4.4 SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (planned)

위와 동일 — daemon 진입점에서도 metrics sink 주입.

---

## 5. 본 SPEC implementation 가능 시점과 우선순위

### 5.1 P3 우선순위 정당성

- `cmdctx.*` metric 은 debug/observability 가치는 있지만 **사용자에게 직접 보이지 않는 데이터**.
- 선행 metrics 인프라 SPEC 이 없으므로 본 SPEC 의 implementation 은 **prerequisite 충족 후에만** 가능.
- 본 SPEC 의 부재는 다른 SPEC 의 진행을 막지 않음 (CMDCTX-001 / CLI-INTEG-001 / DAEMON-INTEG-001 모두 metrics 없이도 동작).

따라서 priority `P3` (low) 가 적절.

### 5.2 implementation 순서 권장

1. (선행) `SPEC-GOOSE-OBS-METRICS-001` 작성 + 구현 (별도 작업, 본 SPEC 외 범위).
2. CMDCTX-001 의 다른 amendment SPEC (PERMISSIVE-ALIAS, HOTRELOAD 등) 가 머지된 후 본 SPEC 진행 (amendment 충돌 회피).
3. 본 SPEC implementation:
   - adapter.go 에 metrics field + helper 추가.
   - adapter_test.go 에 metrics emission unit test 추가 (fake sink 사용).
   - CMDCTX-001 v0.x.0 amendment 동시 적용.
4. 후속 SPEC (CLI-INTEG-001 / DAEMON-INTEG-001) 의 wiring 코드에서 sink 주입.

### 5.3 임시 대안 — Logger 기반 emission (경로 B)

선행 metrics SPEC 의 작성/구현이 지연될 경우, `Logger.Warn` 기반 임시 대안으로 부분 구현 가능:

```go
// 임시 대안 — metrics sink 가 nil 일 때 Logger 로 structured event 출력
func (a *ContextAdapter) emitFallback(method string, durMs int64, err error) {
    if a.logger == nil {
        return
    }
    if err == nil {
        a.logger.Info("cmdctx.method.call",
            "method", method, "duration_ms", durMs)
    } else {
        a.logger.Warn("cmdctx.method.error",
            "method", method, "duration_ms", durMs,
            "error_type", classifyError(err))
    }
}
```

단, 이 대안은:

- aggregation 안 됨 (counter/histogram 의미 없음).
- log volume 폭발 위험 (`PlanModeActive` 가 매 dispatcher call 마다 호출되므로 log line 폭주 → debug 레벨 또는 sampling 필요).
- 진정한 metric 이 아닌 structured event log.

따라서 §3.2 Optional 절에서만 언급하고, 본 SPEC 의 normative 본체에서는 `MetricsSink` interface 만을 정의한다.

---

## 6. 테스트 전략

### 6.1 unit test (adapter_test.go 확장)

선행 metrics SPEC 이 정의한 `metrics.Sink` 의 fake 구현을 사용 (또는 본 SPEC 내부에 `fakeMetricsSink` 정의).

```go
type fakeMetricsSink struct {
    mu       sync.Mutex
    calls    map[string]int            // key: "metricName|labelsCanonical"
    obs      map[string][]float64       // key: histogram name
}

// fakeMetricsSink 은 모든 emission 을 in-memory 에 기록.
// 테스트에서 sink.calls["cmdctx.method.calls|method=OnClear"] == 1 등 검증.
```

#### 6.1.1 테스트 케이스 (계획)

| 테스트 ID | 검증 내용 |
|-----------|-----------|
| `TestMetrics_OnClear_CountsAndDuration` | OnClear 호출 시 calls counter +1, duration histogram observe 1회 |
| `TestMetrics_OnClear_NilLoopCtrl_ErrorCounter` | loopCtrl=nil 일 때 errors{error_type=ErrLoopControllerUnavailable} +1 |
| `TestMetrics_ResolveModelAlias_Unknown_ErrorCounter` | unknown alias 시 errors{error_type=ErrUnknownModel} +1 |
| `TestMetrics_PlanModeActive_HotPath` | PlanModeActive 100회 호출 시 calls=100, errors=0 |
| `TestMetrics_NilSink_NoOp` | metrics=nil 일 때 모든 메서드 정상 동작, panic 없음 |
| `TestMetrics_PanicInSink_DoesNotBreakMethod` | sink.Counter 가 panic 던져도 메서드는 정상 결과 반환 (defer recover) |
| `TestMetrics_WithContext_ChildSharesSink` | WithContext(ctx) child 가 부모와 동일 sink instance 공유 |
| `TestMetrics_DurationOrder` | duration 이 합리적 (ms 단위, 0 이상, monotonic clock 사용) |

### 6.2 race detector

- adapter.go 의 metrics emission 은 sink 가 thread-safe 임을 가정.
- race_test.go 에 N goroutine × 6 메서드 동시 호출 시나리오 추가:

```go
// race_test.go (확장)
func TestRace_Metrics_ConcurrentEmission(t *testing.T) {
    a := New(Options{Metrics: &fakeMetricsSink{...}, ...})
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            a.PlanModeActive()
            _ = a.ResolveModelAlias("...")
        }()
    }
    wg.Wait()
    // counts 검증
}
```

### 6.3 integration test — 보류

선행 metrics SPEC 이 OTel exporter 를 정의하면 OTLP 수신 fixture 와 결합한 integration test 를 추가할 수 있다. 본 SPEC 의 범위는 **adapter 측 emission 만** 이므로 integration 은 후속 SPEC.

---

## 7. 본 SPEC 의 §Exclusions 도출

본 SPEC 의 exclusions (§spec.md 에 명시):

1. **metrics 백엔드 자체** — OTel SDK 초기화, exporter 설정, Prometheus push gateway 등은 선행 SPEC `SPEC-GOOSE-OBS-METRICS-001` (TBD) 의 책임.
2. **distributed tracing** — span propagation, trace ID 주입은 본 SPEC 범위 외. 별도 `SPEC-GOOSE-TRACING-001` (TBD) 가 다룬다.
3. **metric query/aggregation API** — Prometheus PromQL, OTLP query 등 backend 측 query 는 본 SPEC 무관.
4. **alerting 정의** — slow call alert (예: p99 > 100ms 시 alarm) 는 backend 측 alert manager 가 정의. 본 SPEC 은 raw histogram 만 emit.
5. **다른 패키지 (router / loop / subagent) 의 metrics emission** — 본 SPEC 은 `internal/command/adapter` 만 다룸. router 의 ProviderRegistry latency 등은 별도 SPEC.
6. **CLI-INTEG-001 / DAEMON-INTEG-001 의 sink 주입 wiring** — 그쪽 SPEC 본문에서 다룸. 본 SPEC 은 Options.Metrics 필드를 추가하고 그 contract 만 정의.

---

## 8. 본 research 의 한계와 후속 연구 항목

| 한계 | 후속 연구 |
|------|-----------|
| metrics 인프라 부재 — 본 SPEC 의 implementation 은 prerequisite SPEC 에 의존 | `SPEC-GOOSE-OBS-METRICS-001` 별도 작성 (본 SPEC 외 범위) |
| OTel vs expvar vs Prometheus 의 결정 | 선행 SPEC 에서 결정 |
| histogram bucket 값 결정 | 선행 SPEC 또는 본 SPEC v0.2.0 amendment |
| label cardinality 폭발 방지 정책 (`error_type=other` fallback) | §2.2.2 의 3-tier 분류로 대응 — 만약 추가 error 타입이 미래 도입되면 본 SPEC 에 amendment 필요 |
| `PlanModeActive` 의 high-frequency emission 이 hot path latency 에 영향 | benchmark 필요 — `BenchmarkPlanModeActive_WithMetrics` 추가 (run phase) |

---

## 9. 결론

본 research 의 핵심 결정:

1. ✅ **metrics 인프라 부재 확정** — `internal/`, `go.mod` direct, `.moai/specs/` 모두에서 0건.
2. ✅ **본 SPEC 의 plan-only 작성 진행** — RISKS R1 에 prerequisite 부재를 1급 risk 로 명시.
3. ✅ **MetricsSink interface 추상화로 vendor-neutral 유지** — 선행 SPEC 이 OTel/expvar/Prometheus 어느 것을 선택해도 본 SPEC 호환.
4. ✅ **CMDCTX-001 v0.2.0 (또는 v0.3.0) amendment** 가 본 SPEC implementation 시점에 동시 발생.
5. ✅ **6개 메서드 모두 instrument** — calls counter, errors counter (4개 메서드만), duration histogram.
6. ✅ **inline + helper 패턴** 으로 emission, decorator 미사용. nil sink fast-path + panic safety.
7. ⚠️ **임시 대안 (Logger.Warn)** 은 §3.2 Optional 에만 언급, 본 SPEC normative 외.
8. ⚠️ **hot path overhead** (PlanModeActive, ResolveModelAlias) 는 RISKS R3 에 명시, run phase benchmark 로 검증.

본 SPEC 은 plan only 단계이며, run phase 진입은 선행 metrics SPEC 의 implemented status 충족 후로 미룬다.
