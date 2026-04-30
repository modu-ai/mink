# SPEC-GOOSE-OBS-METRICS-001 — Research

> 본 문서는 metrics 인프라 도입에 앞선 사전 조사. backend 후보 3종(stdlib `expvar`, OpenTelemetry SDK, Prometheus client_golang) 의 정량 비교, 본 레포(goose) 현황, 도입 비용/이점 매트릭스, 권장안을 담는다.

작성일: 2026-04-30
작성자: manager-spec
대상 SPEC: SPEC-GOOSE-OBS-METRICS-001 (planned, P2)
informed: SPEC-GOOSE-CMDCTX-TELEMETRY-001 (planned, BLOCKER 해소 대상)

---

## 1. 본 레포(goose) metrics 인프라 현황

### 1.1 정량 검증 (2026-04-30 기준, `main` HEAD `db5c81a`)

| 항목 | 결과 | 명령 |
|------|------|------|
| `internal/` 하위 `metric*` 디렉토리 | **0건** | `find /Users/goos/MoAI/AI-Goose/internal -type d -name 'metric*'` |
| `internal/` 하위 `telemetr*` 디렉토리 | **0건** | `find ... -name 'telemetr*'` |
| `internal/` 하위 `observ*` 디렉토리 | **0건** | `find ... -name 'observ*'` |
| `go.mod` direct require 의 `expvar` / `prometheus/client_golang` | **0건** | `grep -E "expvar\|prometheus/client_golang" go.mod` |
| `go.mod` direct require 의 `go.opentelemetry.io/otel*` | **0건** | direct require 블록에 없음 |
| `go.mod` indirect 의 OTel | **있음** | `go.opentelemetry.io/otel v1.43.0 // indirect`, `otel/metric v1.43.0`, `otel/trace v1.43.0`, `connectrpc.com/otelconnect v0.9.0 // indirect`, `otelhttp v0.68.0 // indirect` |
| `internal/` 코드 내 OTel direct import | **0건** | `internal/` 코드 어디에서도 `go.opentelemetry.io/otel` import 없음 |
| `.moai/specs/` 의 `metric*` / `telemetr*` / `observ*` SPEC | **2건** | `SPEC-GOOSE-CMDCTX-TELEMETRY-001` (planned, consumer), 본 SPEC `SPEC-GOOSE-OBS-METRICS-001` (planned, 본 작성) |

### 1.2 결론 — 본 레포는 metrics infrastructure 가 부재하다

- direct dependency 0건 — 본 SPEC 이 첫 도입.
- 그러나 **OTel 패키지가 indirect 로 이미 들어와 있음** (`connectrpc/otelconnect` 경유). 즉, 이미 OTel 모듈 그래프는 빌드 산출물에 존재하나 `internal/` 코드에서 활용하지 않음. → 만약 OTel 을 backend 로 채택하면 indirect 가 direct 로 승격될 뿐, 신규 모듈 추가는 0.
- consumer SPEC `SPEC-GOOSE-CMDCTX-TELEMETRY-001` 은 sink interface 부재로 현재 BLOCKER. 본 SPEC 이 `Sink` interface 를 확정하면 consumer SPEC 의 BLOCKER 해소.

---

## 2. Backend 후보 3종 비교

### 2.1 옵션 1: stdlib `expvar` + 자체 sink wrapper

**개요**: Go 표준 라이브러리의 `expvar` 패키지는 프로세스 변수(`Int`, `Float`, `Map`)를 `/debug/vars` HTTP endpoint 로 자동 노출한다. 본 SPEC 은 그 위에 `Sink` interface wrapper 를 둔다.

**의존성**:
- direct require 추가: **0건** (stdlib).
- indirect 추가: **0건**.
- go.mod 변화 없음.

**기능**:
- counter (단조 증가): `expvar.Int` 의 `Add(1)`.
- gauge (set/add): `expvar.Float` 의 `Set(v)` / `Add(d)`.
- histogram (분포 관찰): **stdlib 직접 미지원** — 자체 구현 필요 (bucket array + sync.Mutex 또는 atomic). 또는 `expvar.Func` 으로 외부 통계값 노출.
- label / dimension: **미지원** — 직접 metric name 에 인코딩 (`cmdctx.method.calls.OnClear`) 또는 별도 map 관리.
- HTTP 노출: `expvar` 는 `init()` 에서 `http.DefaultServeMux` 의 `/debug/vars` 에 자동 등록 — 이미 daemon 이 `net/http` 를 import 하면 노출 자동.
- 외부 scraping: 가능하나 `/debug/vars` 의 JSON 형식은 Prometheus / OTel 표준 아님. 별도 exporter 필요.

**장점**:
- 의존성 제로 — 본 SPEC 이 가장 가볍게 도입 가능.
- 학습 비용 거의 0 — Go 개발자라면 `expvar` 는 stdlib.
- alpha 단계(v0.1.x) 의 무게 / 복잡도 최소화.
- panic / shutdown 등 lifecycle 부담 없음.
- `Sink` interface 를 이후 OTel 또는 Prometheus 어댑터로 교체할 수 있는 abstraction 유지.

**단점**:
- histogram / label 의 first-class 미지원 — bucket / dimension 직접 구현 부담.
- Prometheus / OTLP 직접 scraping 불가 — production-ready exporter 필요 시 별도 작업.
- 실시간 dashboard 통합(Grafana 등)이 OTLP / Prometheus 대비 불편.

**적합 시점**: alpha 단계, "metric 이 있긴 하다" 수준의 internal 디버깅 / 운영자 점검용. v0.1.x release 까지의 무게 / 복잡도 최소화 목표에 부합.

---

### 2.2 옵션 2: `go.opentelemetry.io/otel` SDK

**개요**: OpenTelemetry 는 metrics / traces / logs 를 통합하는 vendor-neutral 표준. `otel/metric` API 와 `otel/sdk/metric` SDK 를 통해 OTLP / Prometheus / stdout exporter 를 선택할 수 있다.

**의존성**:
- direct require 추가 (예상):
  - `go.opentelemetry.io/otel` (이미 indirect, direct 로 승격).
  - `go.opentelemetry.io/otel/metric` (이미 indirect, 승격).
  - `go.opentelemetry.io/otel/sdk` (신규 direct).
  - `go.opentelemetry.io/otel/sdk/metric` (신규 direct).
  - exporter 선택 시 추가: `otel/exporters/otlp/otlpmetric/otlpmetricgrpc` 또는 `otel/exporters/prometheus`.
- indirect 추가: 다수 (gRPC, protobuf, OTLP wire 의존). go.mod direct/indirect 합산 ~10-20 라인 증가 예상.
- 빌드 산출물 크기 영향: SDK + exporter 포함 시 binary size +5~10MB 추정 (production 빌드).

**기능**:
- counter / up_down_counter / histogram / gauge: API 1급 지원.
- label (attributes): `attribute.KeyValue` 로 dimension 자유롭게 부여, cardinality 관리 OTel collector 측 가능.
- exemplar / aggregation / view: production observability 표준 기능.
- exporter 선택: OTLP/gRPC, OTLP/HTTP, Prometheus pull, stdout — 운영 환경에 따라 교체.
- distributed tracing 과의 통합 (trace_id 라벨 자동 주입 등) — 향후 `SPEC-GOOSE-TRACING-001` 도입 시 자연스럽게 연결.

**장점**:
- 산업 표준 — OTLP 는 OpenTelemetry CNCF graduated 표준.
- vendor-neutral — Datadog / New Relic / Honeycomb / Grafana / Prometheus 모두 OTLP 수신 가능.
- traces / logs 로 자연스러운 확장 — 본 SPEC 이후 `SPEC-GOOSE-TRACING-001` 도입 시 어댑터 재작성 불필요.
- `connectrpc/otelconnect` 가 이미 indirect 로 들어와 있어 OTel 모듈 그래프 친숙도 있음.

**단점**:
- 의존 트리 큼 — alpha 단계의 무게 부담.
- 학습 곡선 — `Meter`, `MeterProvider`, `View`, `Reader` 등 추상화 다수.
- SDK 초기화 / shutdown lifecycle 관리 필요 — 잘못하면 metric 누락 / shutdown 시 hang.
- production exporter (OTLP collector / Prometheus scraping) 운영 인프라 별도 필요. v0.1.x alpha 단계에서 운영자가 collector 운영하지 않을 가능성.

**적합 시점**: beta 이후 (production observability 가 본격 가치 발휘) 또는 distributed tracing 동시 도입 시.

---

### 2.3 옵션 3: `github.com/prometheus/client_golang`

**개요**: Prometheus 의 Go client. counter / gauge / histogram / summary 4종을 first-class 로 노출하고 `/metrics` HTTP endpoint 로 pull-scraping 한다.

**의존성**:
- direct require 추가:
  - `github.com/prometheus/client_golang` (예: v1.20+).
- indirect 추가: `prometheus/client_model`, `prometheus/common`, `prometheus/procfs` 등 ~5-10 라인. OTel 보다 작음.
- 빌드 산출물 크기 영향: binary size +2~3MB 추정.

**기능**:
- counter / gauge / histogram / summary: API 1급.
- label: `prometheus.Labels{}` 로 dimension 부여. cardinality 는 client 측 자체 관리 (Prometheus server 측 cardinality 폭발 방지 책임은 호출자).
- HTTP 노출: `promhttp.Handler()` 를 `/metrics` 에 등록.
- exporter: pull-scraping 모델 — Prometheus server 가 정해진 주기로 scrape.

**장점**:
- 산업 표준 — Prometheus 는 Kubernetes / SRE 생태계의 de facto.
- 학습 곡선 낮음 — `prometheus.NewCounterVec` / `Inc()` / `Observe()` 직관적.
- 의존 트리 OTel 보다 작음.
- production 인프라가 Prometheus + Grafana 인 환경이라면 가장 자연스러움.

**단점**:
- Pull 모델 — daemon / long-running process 에 적합하나, 단발 CLI 호출에는 부적합 (scrape 전 종료).
- vendor lock-in 정도가 OTel 보다 큼 — Prometheus 외 backend 로 이전하려면 어댑터 재작성.
- traces / logs 통합 별도.
- cardinality 폭발 방지 책임이 client 코드 측 (잘못된 label 사용 시 server 메모리 폭발).

**적합 시점**: daemon 운영 환경에서 Prometheus + Grafana 가 이미 구축되어 있을 때, 빠른 도입 우선.

---

### 2.4 정량 비교 매트릭스

| 항목 | expvar | OTel SDK | Prometheus client_golang |
|------|--------|----------|--------------------------|
| **direct require 신규 추가** | 0개 | 3-5개 | 1개 |
| **indirect 신규 추가 (예상)** | 0개 | 10-20개 | 5-10개 |
| **binary size 증가 (예상)** | 0MB | +5~10MB | +2~3MB |
| **histogram first-class** | ✗ (직접 구현) | ✓ | ✓ |
| **label/dimension first-class** | ✗ (name 인코딩) | ✓ | ✓ |
| **HTTP exporter** | `/debug/vars` (auto) | OTLP / Prometheus / stdout | `/metrics` |
| **pull vs push** | pull (HTTP) | both | pull |
| **CLI 단발 호출 적합** | △ (HTTP server 필요) | △ (push exporter 필요) | ✗ (pull, scrape 전 종료) |
| **daemon long-running 적합** | ✓ | ✓ | ✓ |
| **vendor lock-in** | 낮음 (자체 wrapper) | 매우 낮음 (CNCF 표준) | 중 (Prometheus 종속) |
| **학습 곡선** | 낮음 | 높음 | 낮음 |
| **lifecycle 관리 부담** | 거의 없음 | 높음 (SDK init/shutdown) | 중 (registry/handler) |
| **traces/logs 통합** | ✗ | ✓ (OTel 통합) | △ (별도 OTLP 어댑터) |
| **alpha v0.1.x 단계 적합도** | **높음** | **낮음** | **중** |
| **production beta+ 단계 적합도** | 낮음 | **높음** | 중-높음 |

### 2.5 도입 비용 / 이점 평가 (v0.1.x alpha 기준)

| 옵션 | 비용 | 이점 | net 평가 |
|------|------|------|----------|
| expvar | 매우 낮음 (의존 0, lifecycle 관리 0) | 기본적 counter/gauge 노출, sink interface abstraction 유지로 미래 OTel/Prometheus 어댑터 추가 길 열림 | **+** alpha 단계 최적 |
| OTel SDK | 높음 (의존 트리, lifecycle, exporter 운영 인프라) | 산업 표준, traces 동시 도입 시 시너지 | **-** alpha 단계 과도, beta 에서 재평가 |
| Prometheus | 중간 (의존, HTTP handler, cardinality 책임) | Prometheus 환경 즉시 호환 | **±** 운영 인프라가 Prometheus 가 이미 있다면 +, 아니면 - |

---

## 3. Phase 분리 전략 (권장안)

### 3.1 Phase 1 (본 SPEC, P2): `Sink` interface + expvar 어댑터 + noop 어댑터

- **목표**: consumer SPEC `SPEC-GOOSE-CMDCTX-TELEMETRY-001` 의 BLOCKER 해소. 가장 가벼운 backend 로 sink interface contract 검증.
- **선택 backend**: 옵션 1 (expvar).
- **신규 산출물**:
  - `internal/observability/metrics/` — interface 패키지 (Sink, Counter, Histogram, Gauge, Labels).
  - `internal/observability/metrics/expvar/` — expvar 기반 default 구현.
  - `internal/observability/metrics/noop/` — no-op 구현 (consumer 가 nil 대신 기본값으로 사용).
- **emit 정책**: label cardinality 제한 (max 100 unique combinations per metric, 초과 시 drop), PII 금지, env `GOOSE_METRICS_ENABLED` (default: false in alpha, true in beta+).

### 3.2 Phase 2 (별도 SPEC, TBD): OTel adapter 추가

- **트리거**: beta release 또는 distributed tracing 도입 결정 시.
- **신규 산출물**: `internal/observability/metrics/otel/` — OTel `Meter` 어댑터.
- **호환성**: 본 SPEC 의 Sink interface 가 backend-agnostic 이어야 하므로, Phase 2 에서 consumer 코드 수정 없이 backend 만 swap 가능.
- **별도 SPEC ID 후보**: `SPEC-GOOSE-OBS-METRICS-OTEL-001` (TBD).

### 3.3 Phase 3 (선택, TBD): Prometheus adapter

- **트리거**: 사용자 요청 시. Prometheus 환경에서 운영하는 user 가 존재할 때.
- **신규 산출물**: `internal/observability/metrics/prometheus/`.
- **별도 SPEC ID 후보**: `SPEC-GOOSE-OBS-METRICS-PROM-001` (TBD).

### 3.4 Phase 분리의 정당화

- **단계적 검증**: Phase 1 의 expvar 기반 도입으로 sink interface 의 contract 가 실제 사용 패턴에서 검증된 후, Phase 2/3 에서 무거운 backend 도입.
- **alpha 단계 무게 최소화**: alpha 사용자에게 OTel collector 운영 / Prometheus server 운영을 강제하지 않음.
- **abstraction 유지**: Sink interface 가 consumer 코드와 backend 를 분리. backend 교체 비용 = 어댑터 1개 작성 + 1줄 wiring.
- **regression 방지**: Phase 1 의 Sink interface contract 가 Phase 2/3 도입 시에도 보존된다(v0.x.0 minor amendment 가능, breaking change 회피).

---

## 4. Sink Interface 설계 결정

### 4.1 인터페이스 시그니처 (확정)

```go
// internal/observability/metrics/metrics.go (Phase 1 신규)
package metrics

// Sink is a vendor-neutral metrics emission surface.
// Implementations MUST be thread-safe and MUST gracefully handle high
// cardinality by dropping new label combinations beyond the configured cap.
type Sink interface {
    Counter(name string, labels Labels) Counter
    Histogram(name string, labels Labels, buckets []float64) Histogram
    Gauge(name string, labels Labels) Gauge
}

// Counter is a monotonically-increasing handle.
type Counter interface {
    Inc()
    Add(delta float64)
}

// Histogram is a value-distribution observer with implementation-defined buckets.
type Histogram interface {
    Observe(value float64)
}

// Gauge is a settable instantaneous-value handle.
type Gauge interface {
    Set(value float64)
    Add(delta float64)
}

// Labels is a static dimension map. Dynamic values (err.Error(), user IDs,
// prompt content) MUST NOT appear here — caller responsibility per
// REQ-OBS-METRICS-005 (PII / cardinality firewall).
type Labels map[string]string
```

### 4.2 설계 근거

- **3-instrument 최소집합**: counter, histogram, gauge — Prometheus / OTel / expvar 모두 first-class 또는 직접 구현 가능. summary 는 OTel 가 권장하지 않으므로 제외.
- **factory 메서드 + handle 패턴**: 호출자가 hot path 에서 매번 lookup 하지 않도록 handle 을 1회 획득 후 재사용 가능.
- **buckets parameter 명시**: histogram 의 bucket 은 호출자(consumer)가 도메인 지식으로 결정해야 함. backend 가 강제 default 를 두지 않음. nil/empty 슬라이스 시 backend default 사용.
- **Labels = static map**: 동적 값 금지(REQ §4 / Exclusions §1). 카디널리티 폭발 방지의 1차 방어선.
- **noop fallback**: consumer 가 nil 체크 없이 항상 sink 를 호출할 수 있도록, default 주입은 noop 구현. nil sink 도 허용하나(graceful skip), noop 권장.

### 4.3 consumer 적용 (TELEMETRY-001 BLOCKER 해소)

```go
// CMDCTX-TELEMETRY-001 의 adapter.Options 에 본 SPEC sink 직접 적용
import "github.com/modu-ai/goose/internal/observability/metrics"

type Options struct {
    // ... 기존 필드
    Metrics metrics.Sink // 본 SPEC sink. nil 또는 metrics/noop.New() 권장.
}
```

TELEMETRY-001 의 `MetricsSink` interface 는 본 SPEC 의 `metrics.Sink` 를 직접 alias 또는 import 한다. 별도 interface 정의 불필요.

---

## 5. emit 정책 결정

### 5.1 cardinality 제한 (HARD)

- **per-metric cap**: 단일 metric name 당 max **100** unique label combinations.
- **초과 시 동작**: drop (silent), 단 expvar backend 의 `expvar.Map` 는 1회 warn log 발생 (`metrics/expvar` 책임).
- **cap 값 선정 근거**: alpha 단계 운영 안전 마진. 일반적으로 metric 당 수십 개 이하의 dimension combination 이 healthy. 100 초과 시 cardinality 폭발 신호.
- **cap 변경**: 본 SPEC v0.2.0 amendment 시 환경변수 `GOOSE_METRICS_LABEL_CAP` 도입 가능 (Phase 2 권장).

### 5.2 PII 금지 (HARD)

- label 값에 다음을 **금지**:
  - user prompt 본문 / model output 본문.
  - credential 값 (API key, OAuth token, password).
  - file path 내 user home directory (`/Users/...`, `~/...`) — replace with `<HOME>`.
  - email / 전화번호 / 사용자 식별자 원본.
- consumer 책임 — 본 SPEC sink 는 PII 검출 로직 없음. 단, Phase 2 에서 OTel SDK 도입 시 OTel collector 측 redaction processor 활용 가능.
- 위반 시: code review 차단 (TRUST 5 — Secured), 또는 정적 분석 (label 값에 동적 input 사용 금지) 도입.

### 5.3 enable / disable 토글

- env `GOOSE_METRICS_ENABLED`:
  - default in alpha (v0.1.x): `false`.
  - default in beta+ : `true`.
  - 명시적 `false` 시 sink wiring 단계에서 noop 구현 주입.
  - 명시적 `true` 시 expvar (Phase 1) / OTel (Phase 2 도입 후) 구현 주입.
- consumer 코드는 env 검사하지 않음 — wiring 진입점(CLI / daemon) 에서만 검사 후 적절한 sink 인스턴스 주입.

### 5.4 expvar HTTP endpoint 노출

- expvar 의 `/debug/vars` endpoint:
  - daemon 모드: 자동 노출 (이미 `net/http` import 시).
  - CLI 단발 호출: HTTP server 미실행 → endpoint 무용. 단, 프로세스 수명 동안 in-memory counter 는 누적되어 debugger / runtime print 로 접근 가능.
- security: `/debug/vars` 는 internal 전용. production daemon 의 경우 admin port (별도 listener) 또는 unix socket 으로 격리. Phase 1 본 SPEC 은 endpoint 보안 분리를 강제하지 않음(별도 SPEC).

---

## 6. 사용자 결정 권장 — Phase 1 backend 선택

### 6.1 권장: 옵션 1 (expvar) + sink interface

**근거**:
- 본 레포는 alpha v0.1.x 단계 — 무게 / 복잡도 최소화 우선.
- consumer SPEC TELEMETRY-001 의 BLOCKER 해소가 본 SPEC 의 1차 목표 — sink interface 만 확정되면 consumer 진행 가능.
- expvar 는 stdlib — 의존 추가 0건, lifecycle 관리 부담 없음.
- sink interface 가 backend-agnostic 이므로 Phase 2 에서 OTel 어댑터 추가 시 consumer 코드 수정 0건.

### 6.2 비권장: 옵션 2 (OTel SDK) Phase 1 직접 도입

- alpha 단계에서 OTel collector 운영을 user 에게 강제하기 부담.
- SDK lifecycle 관리 / exporter 설정 등 추가 작업이 본 SPEC scope 를 키움.
- Phase 2 에서 도입해도 consumer 영향 0 — Phase 1 에서 무리할 이유 없음.

### 6.3 비권장: 옵션 3 (Prometheus) Phase 1 직접 도입

- CLI 단발 호출에 부적합 (pull scraping 전 종료).
- 사용자의 Prometheus 환경 가정이 alpha 단계에서 너무 강함.
- 필요 시 Phase 3 별도 SPEC 으로 도입.

### 6.4 의사결정 흐름

```
Q1: 본 레포 v0.1.x 알파에서 metrics 인프라가 필요한가?
├─ YES → Q2
└─ NO → 본 SPEC 보류, consumer SPEC TELEMETRY-001 도 보류.

Q2: 어떤 backend 가 alpha 단계에 적합한가?
├─ expvar (권장) → 본 SPEC Phase 1 진행.
├─ OTel → 본 SPEC scope 확장 + alpha 단계 무게 부담 수용 시 진행.
└─ Prometheus → daemon 환경 + Prometheus 인프라 기대 시 진행.

Q3: Phase 2 (OTel adapter) 는 언제 도입하는가?
├─ beta 이후, 또는 distributed tracing 도입 시.
```

**manager-spec 권장 결론**: **옵션 1 (expvar) Phase 1 도입**. 본 SPEC 의 spec.md 는 이 결정에 따라 작성된다.

---

## 7. Risk 요약

| ID | Risk | Mitigation |
|----|------|-----------|
| R1 | Phase 1 expvar 만으로 production observability 부족 | Phase 2 OTel 도입 별도 SPEC, sink interface 보존으로 consumer 영향 0 |
| R2 | sink interface 가 OTel / Prometheus 와 호환 안 됨 (factory 시그니처) | 3 후보 backend 모두 counter/histogram/gauge 1급 — interface 가 자연스럽게 매핑. histogram bucket 만 backend 별 차이 → bucket 슬라이스 명시로 해결 |
| R3 | label cardinality 폭발 | per-metric cap 100, drop on overflow, PII 금지 (HARD) |
| R4 | expvar `/debug/vars` 가 production daemon 에서 외부 노출 보안 | Phase 1 scope 외 — 별도 SPEC 에서 admin port / unix socket 분리 |
| R5 | env `GOOSE_METRICS_ENABLED` 가 wiring 시점에 누락 | wiring 진입점(CLI / daemon) 에서 noop fallback 강제 — sink 가 nil 인 경우도 허용하나 noop 권장 |

---

## 8. 다음 단계

1. spec.md 작성 — 본 research 의 권장안(옵션 1 expvar)을 EARS REQ / AC 로 변환.
2. plan-auditor 검증 — REQ/AC EARS 준수, Exclusions 명시, BLOCKER 해소 경로 확인.
3. run phase 진입:
   - `internal/observability/metrics/` 패키지 신규.
   - expvar / noop 어댑터 구현.
   - consumer SPEC TELEMETRY-001 의 wiring 검증 (별도 SPEC run phase).
4. Phase 2 SPEC `SPEC-GOOSE-OBS-METRICS-OTEL-001` (TBD) 는 본 SPEC implemented 후 작성.

---

Version: 0.1.0
Last Updated: 2026-04-30
Decision: Phase 1 backend = stdlib `expvar` + sink interface.
