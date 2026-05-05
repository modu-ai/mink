---
id: SPEC-GOOSE-OBS-METRICS-001
version: 0.1.2
status: implemented
created_at: 2026-04-30
updated_at: 2026-05-05
author: manager-spec
priority: P2
issue_number: null
phase: 3
size: 중(M)
lifecycle: spec-anchored
labels: ["area/observability", "area/runtime", "type/feature", "infrastructure", "blocker-unblock"]
parent_spec: null
informed_by: ["SPEC-GOOSE-CMDCTX-TELEMETRY-001"]
---

# SPEC-GOOSE-OBS-METRICS-001 — Metrics Sink Interface 와 expvar Phase 1 Backend

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-30 | 초안 작성. consumer SPEC `SPEC-GOOSE-CMDCTX-TELEMETRY-001` (planned, P3) 의 BLOCKER 해소를 위한 metrics sink interface 와 Phase 1 backend (stdlib `expvar`) 정의. backend 비교 결과 `research.md` 참고. Phase 2 (OTel adapter) / Phase 3 (Prometheus adapter) 는 별도 SPEC. | manager-spec |
| 0.1.1 | 2026-05-04 | plan-audit iteration 2 FAIL fix: §5.7 REQ↔AC 매트릭스 전면 재작성 (15/20 행 분류 오류 정정 — 매트릭스가 sibling/stale REQ 번호와 어긋났음). REQ-004/005 직접 검증 AC-019/AC-020 신규 추가, REQ-016 은 §6.1 godoc + NFR 검증으로 명시. §5 자기주장 블록 정직 재작성. | manager-spec |
| 0.1.2 | 2026-05-05 | **sync phase — status drift 정정 + implementation commit 매핑.** 본 SPEC 의 implementation 본체는 PR #60 (commit `43c46bf`, 2026-05-01 머지) 으로 도달했으나, sync 단계가 누락되어 frontmatter `status: planned` 가 long-standing drift 로 잔존. AC-019/AC-020 augmentation 도 PR #102 (commit `5c4e0a8`, 2026-05-05 머지) 로 main 에 반영 완료. 본 entry 는 (a) status `planned` → `implemented` 전환, (b) implementation commit `43c46bf` (#60) + augmentation commit `5c4e0a8` (#102) 매핑, (c) consumer SPEC `SPEC-GOOSE-CMDCTX-TELEMETRY-001` R1 BLOCKER 가 v0.1.1 amendment 로 이미 해소되었음을 확정 기록. spec 본문/요구사항/AC 변경 없음 — 메타 갱신 only. | manager-docs |

---

## 1. 개요 (Overview)

본 SPEC 은 `github.com/modu-ai/goose` 레포에 metrics 인프라를 신규 도입한다. 본 레포는 현재(2026-04-30 `main` HEAD `db5c81a`) `internal/` 에 metrics / telemetry / observability 디렉토리가 0건이며, direct 의존도 0건이다 (단, `connectrpc/otelconnect` 경유로 OTel 패키지 indirect 만 존재).

본 SPEC 의 목표:

1. **vendor-neutral metrics sink interface** 정의 — counter, histogram, gauge 3종 instrument + Labels 타입.
2. **Phase 1 backend = stdlib `expvar`** 어댑터 + **noop** 어댑터 구현. 의존성 0건 추가.
3. **emit 정책** 확정 — label cardinality cap, PII 금지, env toggle (`GOOSE_METRICS_ENABLED`).
4. **consumer SPEC `SPEC-GOOSE-CMDCTX-TELEMETRY-001` 의 BLOCKER 해소** — 본 SPEC 의 `metrics.Sink` 가 consumer 의 `adapter.Options.Metrics` 에 직접 주입 가능.

본 SPEC 은 **interface contract + Phase 1 default backend** 만 정의한다. OTel SDK 어댑터, Prometheus client_golang 어댑터, distributed tracing, alerting 정의, exporter 운영 인프라는 OUT OF SCOPE.

---

## 2. 배경 (Background)

### 2.1 현황 (research.md §1 기준)

- `internal/` 의 metric / telemetry / observability 디렉토리: **0건**.
- `go.mod` direct require 의 metrics 라이브러리: **0건**.
- `go.mod` indirect 의 OTel: 있음 (`go.opentelemetry.io/otel v1.43.0` 등, `connectrpc/otelconnect v0.9.0` 경유).
- 결론: 본 SPEC 이 metrics 인프라의 1차 도입.

### 2.2 trigger — consumer SPEC BLOCKER

`SPEC-GOOSE-CMDCTX-TELEMETRY-001` (planned, P3) 은 `internal/command/adapter/` 의 6개 메서드(`OnClear`, `OnCompactRequest`, `OnModelChange`, `ResolveModelAlias`, `SessionSnapshot`, `PlanModeActive`)에 calls / errors / duration metrics 를 emission 한다. 그 SPEC §3.1 #1 은 `MetricsSink` interface 를 "선행 SPEC 의 sink 를 import" 한다고 명시 — 그 선행 SPEC 이 본 SPEC. 본 SPEC 부재 시 consumer SPEC 은 BLOCKER (R1, 1급 risk).

본 SPEC 이 sink interface 를 확정하면 consumer SPEC 의 BLOCKER 해소.

### 2.3 backend 결정 — Phase 1 = expvar (research.md §6 결론)

세 후보(stdlib `expvar`, OTel SDK, Prometheus client_golang)의 도입 비용 / 이점 매트릭스 (research.md §2.4) 결과:

- **expvar**: direct require 0개, indirect 0개, lifecycle 관리 거의 없음, alpha 단계 적합도 **높음**.
- **OTel SDK**: direct require 3-5개, indirect 10-20개, lifecycle 부담 높음, alpha 적합도 낮음. → Phase 2 별도 SPEC.
- **Prometheus**: direct 1개, indirect 5-10개, CLI 단발 호출 부적합 (pull). → Phase 3 별도 SPEC.

**Phase 1 권장: expvar + sink interface**. 의존성 0 추가, alpha 무게 최소, sink interface 가 backend-agnostic 이므로 Phase 2/3 어댑터 추가 시 consumer 영향 0.

### 2.4 범위 경계 (한 줄)

- **IN**: `Sink` interface 패키지 (`internal/observability/metrics/`), expvar 기본 구현, noop 구현, env toggle, label cardinality cap, PII 금지 가이드라인, consumer SPEC TELEMETRY-001 호환성.
- **OUT**: OTel SDK 어댑터, Prometheus 어댑터, distributed tracing, exporter 인프라, alerting 정의, dashboard, scraping endpoint security, consumer wiring (TELEMETRY-001 / CLI-INTEG / DAEMON-INTEG 의 책임).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC 이 정의 / 구현하는 것)

1. **`internal/observability/metrics/` 패키지 신규** — 3종 interface (`Sink`, `Counter`, `Histogram`, `Gauge`) + `Labels` 타입.

2. **`internal/observability/metrics/expvar/` 패키지 신규** — stdlib `expvar` 기반 default `Sink` 구현.
   - counter: `expvar.Int.Add(1)` mapping.
   - gauge: `expvar.Float.Set(v)` / `Add(d)`.
   - histogram: 자체 구현 (bucket array + `sync.Mutex` 또는 atomic, `Observe(v)` 시 적합 bucket 의 counter 증가).
   - HTTP endpoint: stdlib `expvar` 가 `init()` 에서 `/debug/vars` 자동 등록 (별도 작업 불필요, 단 daemon 의 `net/http` import 가정).

3. **`internal/observability/metrics/noop/` 패키지 신규** — 모든 메서드가 no-op 인 `Sink` 구현. consumer 의 default 주입용.

4. **emit 정책 (HARD)**:
   - label cardinality cap: per-metric **100** unique label combinations. 초과 시 silent drop + 1회 warn log (expvar backend 만).
   - PII 금지: label 값에 user prompt 본문 / model output / credential / file path 의 `$HOME` / email 절대 금지. 정적 가이드라인 + code review 차단 (TRUST 5 — Secured).
   - env toggle: `GOOSE_METRICS_ENABLED`.
     - alpha (v0.1.x) default: `false`.
     - beta+ default: `true`.
     - wiring 진입점에서 검사 후 expvar / noop sink 분기.

5. **consumer SPEC TELEMETRY-001 호환성**:
   - 본 SPEC 의 `metrics.Sink` 가 consumer 의 `adapter.Options.Metrics MetricsSink` 에 직접 사용 가능.
   - consumer SPEC 의 `MetricsSink` interface 는 본 SPEC 의 `metrics.Sink` 를 alias 또는 직접 import.
   - factory 시그니처 (`Counter(name, labels)`, `Histogram(name, labels, buckets)`) 가 consumer 의 emission 패턴과 호환.

6. **테스트**:
   - `metrics_test.go` — interface contract 검증 (구현체별 contract test).
   - `expvar/expvar_test.go` — counter/histogram/gauge 동작 + cardinality cap + concurrent safety (`go test -race`).
   - `noop/noop_test.go` — 모든 메서드 no-op 검증, panic 없음.
   - line coverage ≥ 85% (TRUST 5 standard).

7. **godoc**:
   - 모든 exported 심볼 godoc 작성.
   - emit 정책 (cardinality, PII) 을 `Sink` interface godoc 에 명시.
   - SPEC ID cross-reference (`@MX:SPEC SPEC-GOOSE-OBS-METRICS-001`).

8. **MX 태그**:
   - `Sink` interface — `@MX:ANCHOR` (vendor-neutral surface, fan_in ≥ 3 예상: TELEMETRY-001 / 향후 router emission / 향후 loop emission).
   - cardinality cap 로직 — `@MX:WARN` (`@MX:REASON`: silent drop 동작이 운영자에게 surprise 가능).
   - expvar `/debug/vars` 자동 등록 — `@MX:NOTE` (init 부수효과).

### 3.2 OUT OF SCOPE (별도 SPEC 또는 운영 책임)

1. **OTel SDK 어댑터** — `SPEC-GOOSE-OBS-METRICS-OTEL-001` (TBD, Phase 2). 본 SPEC implemented 후 작성.
2. **Prometheus client_golang 어댑터** — `SPEC-GOOSE-OBS-METRICS-PROM-001` (TBD, Phase 3, 사용자 요청 시).
3. **distributed tracing** — `SPEC-GOOSE-TRACING-001` (TBD).
4. **consumer 패키지의 emission wiring** — TELEMETRY-001 / 향후 router / loop / subagent / query / credential SPEC 의 책임. 본 SPEC 은 sink contract 만.
5. **CLI / daemon 진입점에서의 sink instance 주입** — `SPEC-GOOSE-CMDCTX-CLI-INTEG-001` / `SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001` (planned) 의 책임.
6. **alerting / dashboard / metric query API** — backend 운영 인프라 측 (Grafana, Prometheus alertmanager 등).
7. **exporter 운영** (OTLP collector 운영, Prometheus pull-scraping 설정) — Phase 2/3 도입 후 별도 운영 SPEC.
8. **`/debug/vars` endpoint 의 production security 분리** (admin port, unix socket) — 별도 `SPEC-GOOSE-OBS-METRICS-SECURITY-001` (TBD).
9. **histogram bucket 의 도메인별 default 결정** — 본 SPEC 은 `buckets []float64` parameter 만 정의. consumer SPEC 가 도메인 지식으로 bucket 결정.

---

## 4. EARS 요구사항 (Requirements)

EARS 5 패턴: Ubiquitous (항상), Event-Driven (WHEN), State-Driven (WHILE/IF), Unwanted (SHALL NOT), Optional (WHERE).

### 4.1 Ubiquitous — 항상 활성

**REQ-OBS-METRICS-001**: 본 SPEC 의 `internal/observability/metrics/` 패키지는 `Sink`, `Counter`, `Histogram`, `Gauge` 4 종 interface 와 `Labels` 타입을 export 한다. interface 메서드는 thread-safe 임을 godoc 에 명시한다.

**REQ-OBS-METRICS-002**: 모든 `Sink` 구현체 (expvar, noop, 그리고 향후 OTel/Prometheus) 는 `Counter(name string, labels Labels) Counter`, `Histogram(name string, labels Labels, buckets []float64) Histogram`, `Gauge(name string, labels Labels) Gauge` 3 factory 메서드를 제공한다. 반환된 handle 은 thread-safe 한 호출을 허용한다.

**REQ-OBS-METRICS-003**: 본 SPEC 의 `metrics.Sink` interface 는 consumer SPEC `SPEC-GOOSE-CMDCTX-TELEMETRY-001` 의 `adapter.Options.Metrics` 필드 타입과 호환된다 (직접 alias 또는 import). consumer 가 본 SPEC 의 sink 를 import 한 후 별도 wrapper 없이 주입 가능하다.

**REQ-OBS-METRICS-004**: 모든 `Sink` 구현체는 동일 `(name, labels)` 조합으로 factory 메서드를 다회 호출 시 동일한 handle 을 반환하거나, 또는 동일한 underlying counter/histogram/gauge 에 누적되는 두 handle 을 반환한다. 호출자의 handle caching 패턴을 강제하지 않는다.

**REQ-OBS-METRICS-005**: `Labels` 타입은 정적 dimension map (`map[string]string`) 으로, 호출 시 caller 가 설정한 값을 그대로 보존한다. 본 SPEC 은 동적 값 검출 / 변환 / sanitization 을 수행하지 않는다 (caller 책임).

### 4.2 Event-Driven — 트리거 기반

**REQ-OBS-METRICS-006**: WHEN `Sink.Counter(name, labels)` 가 호출될 때, 동일 `(name, labels)` 조합이 처음이면 새 counter 가 생성되고, 기존 조합이면 기존 counter handle 을 반환한다. 반환된 handle 의 `Inc()` / `Add(delta)` 호출은 underlying counter 값을 atomic 또는 mutex-protected 방식으로 증가시킨다.

**REQ-OBS-METRICS-007**: WHEN `Sink.Histogram(name, labels, buckets)` 가 호출될 때, 동일 `(name, labels)` 조합이 처음이면 새 histogram 이 `buckets` 로 초기화되고, 기존 조합이면 기존 handle 을 반환한다. `buckets == nil` 또는 `len(buckets) == 0` 일 때 backend default bucket 을 사용한다 (expvar 어댑터의 default: `[0.1, 1, 10, 100, 1000]` ms-scale).

**REQ-OBS-METRICS-008**: WHEN env `GOOSE_METRICS_ENABLED` 가 wiring 진입점에서 검사될 때, `"true"` / `"1"` 이면 expvar sink 인스턴스를 주입하고, `"false"` / `"0"` / 미설정이면 noop sink 인스턴스를 주입한다. alpha (v0.1.x) 의 default 미설정 동작은 noop. (wiring 진입점은 본 SPEC 의 책임이 아니나, env 토글 contract 는 본 SPEC 이 정의.)

**REQ-OBS-METRICS-009**: WHEN expvar 어댑터의 `Counter(name, labels)` / `Histogram` / `Gauge` 가 새 `(name, labels)` 조합을 등록할 때, 해당 metric name 의 unique label combination 수가 cardinality cap (100) 을 초과하면, 새 조합 등록은 silent drop (return noop counter/histogram/gauge handle) 되며 1회 warn log (`Logger.Warn("metrics cardinality cap exceeded", "metric", name, "cap", 100)`) 가 발생한다.

### 4.3 State-Driven — 조건 기반

**REQ-OBS-METRICS-010**: WHILE 임의의 `Sink` 구현체가 사용되는 동안, factory 메서드(`Counter`/`Histogram`/`Gauge`) 와 handle 메서드(`Inc`/`Add`/`Observe`/`Set`) 는 concurrent 호출에 대해 race-free 하게 동작한다. `go test -race ./internal/observability/metrics/...` 가 통과한다.

**REQ-OBS-METRICS-011**: IF `Sink` 가 `noop.Sink` 인스턴스이면, 모든 factory 메서드는 동일한 noop handle (또는 매번 새 noop handle) 을 반환하며, 모든 handle 메서드는 부수효과 없이 즉시 반환한다 (≤ 5ns benchmark target).

**REQ-OBS-METRICS-012**: IF `Sink` 가 nil 이면, consumer 의 wiring 코드 (본 SPEC scope 외) 는 noop sink 로 fallback 해야 한다. 본 SPEC 의 `Sink` interface 자체는 nil receiver 를 다루지 않는다 (caller 책임).

### 4.4 Unwanted — 금지 / 방지

**REQ-OBS-METRICS-013**: `Labels` 의 값에 다음 형태의 데이터를 포함하는 emission 은 본 SPEC 의 contract 위반이다 (caller 측 정적 분석 / code review 차단 책임):
- user prompt 본문 또는 model output 본문 (전체 또는 일부).
- credential 값 (API key, OAuth token, password, session cookie).
- file path 내 user home directory 원본 (`/Users/...`, `/home/...`, `~/...`) — replace with `<HOME>` 권장.
- email 주소, 전화번호, 사용자 식별자(user ID, device ID 등) 의 raw 값.

**REQ-OBS-METRICS-014**: 본 SPEC 의 `Sink` 구현체는 caller 가 제공한 `(name, labels)` 조합을 변형(normalization, hashing, redaction) 하지 않는다. 즉, REQ-OBS-METRICS-013 위반 데이터가 backend 에 도달하면 그대로 노출된다 (caller 책임 강제).

**REQ-OBS-METRICS-015**: expvar 어댑터의 cardinality cap (100) 초과 시 panic 을 발생시키지 않는다. silent drop + 1회 warn log 만 허용된다 (REQ-OBS-METRICS-009).

**REQ-OBS-METRICS-016**: 본 SPEC 의 어떤 구현체도 backend SDK 의 lifecycle (init / shutdown / flush) 을 caller 에게 요구하지 않는다. expvar 어댑터는 lifecycle 관리 0 (stdlib `expvar` 가 process-wide singleton). noop 어댑터도 lifecycle 0. (Phase 2 OTel 어댑터는 별도 SPEC 에서 lifecycle contract 정의.)

**REQ-OBS-METRICS-017**: 본 SPEC 은 consumer 패키지 (예: `internal/command/adapter`, `internal/router`, `internal/loop`) 에서 metrics emission 코드를 작성하지 않는다. 본 SPEC 의 산출물은 `internal/observability/metrics/` 트리만이다.

### 4.5 Optional — 권장사항 (운영 환경에 따라)

**REQ-OBS-METRICS-018**: WHERE daemon 모드로 실행되고 `net/http` HTTP server 가 활성화된 경우, expvar 어댑터의 `/debug/vars` endpoint 가 자동으로 노출된다. 본 SPEC 은 endpoint 의 access control / authentication 을 정의하지 않는다 (별도 SPEC 책임).

**REQ-OBS-METRICS-019**: WHERE 운영자가 cardinality cap (100) 을 변경하고 싶다면, 본 SPEC v0.2.0 amendment 시 환경변수 `GOOSE_METRICS_LABEL_CAP` 도입 가능하다. v0.1.0 에서는 cap 이 hardcoded.

**REQ-OBS-METRICS-020**: WHERE histogram bucket 을 도메인별로 customize 하고 싶다면, caller (consumer SPEC) 가 `Histogram(name, labels, buckets)` 호출 시 도메인 적합 bucket 슬라이스를 명시한다. 본 SPEC 은 bucket 의 도메인별 default 를 강제하지 않으며, expvar 어댑터의 fallback default (`[0.1, 1, 10, 100, 1000]`) 만 정의한다.

---

## 5. 수락 기준 (Acceptance Criteria)

### 5.1 패키지 구조 / interface 검증

**AC-OBS-METRICS-001**: `internal/observability/metrics/metrics.go` (또는 `sink.go`) 가 존재하고, `Sink`, `Counter`, `Histogram`, `Gauge` 4 interface 와 `Labels` 타입이 정의되어 있다. 모든 exported 심볼은 godoc 을 보유한다. (REQ-OBS-METRICS-001)

**AC-OBS-METRICS-002**: `Sink` interface 의 시그니처는 `Counter(name string, labels Labels) Counter`, `Histogram(name string, labels Labels, buckets []float64) Histogram`, `Gauge(name string, labels Labels) Gauge` 3 메서드만이다. 시그니처 외 추가 메서드 없음. (REQ-OBS-METRICS-002)

**AC-OBS-METRICS-003**: `Counter` interface 는 `Inc()` 와 `Add(delta float64)` 2 메서드, `Histogram` interface 는 `Observe(value float64)` 1 메서드, `Gauge` interface 는 `Set(value float64)` 와 `Add(delta float64)` 2 메서드만 갖는다. (REQ-OBS-METRICS-002)

**AC-OBS-METRICS-004**: `internal/observability/metrics/expvar/expvar.go` 가 존재하고, `New() metrics.Sink` 또는 `NewSink() metrics.Sink` factory 함수가 expvar 기반 `Sink` 구현체를 반환한다. (REQ-OBS-METRICS-002, §3.1 #2)

**AC-OBS-METRICS-005**: `internal/observability/metrics/noop/noop.go` 가 존재하고, `New() metrics.Sink` factory 가 모든 메서드 no-op 인 `Sink` 구현체를 반환한다. (REQ-OBS-METRICS-011, §3.1 #3)

### 5.2 동작 검증 — expvar 어댑터

**AC-OBS-METRICS-006**: `TestExpvarSink_Counter_Increments` 테스트는 (a) `Counter("test.counter", nil)` 의 `Inc()` 100회 호출 후 expvar 의 `test.counter` 변수 값이 정확히 100, (b) `Add(2.5)` 호출 후 값이 102.5 임을 검증한다. (REQ-OBS-METRICS-006)

**AC-OBS-METRICS-007**: `TestExpvarSink_Histogram_BucketsObserve` 테스트는 (a) `Histogram("test.hist", nil, []float64{1, 10, 100})` 에 `Observe(0.5)`, `Observe(5)`, `Observe(50)`, `Observe(500)` 호출 후 각 bucket 의 누적 카운트가 expected `[1, 1, 1, 1]` (cumulative: `[1, 2, 3, 4]`) 임을 검증한다. (b) `buckets == nil` 일 때 default bucket `[0.1, 1, 10, 100, 1000]` 이 적용됨을 검증한다. (REQ-OBS-METRICS-007)

**AC-OBS-METRICS-008**: `TestExpvarSink_Gauge_SetAndAdd` 테스트는 (a) `Gauge("test.gauge", nil).Set(42.0)` 후 expvar 값이 42.0, (b) 이어 `Add(8.0)` 후 값이 50.0, (c) `Add(-30.0)` 후 값이 20.0 임을 검증한다. (REQ-OBS-METRICS-002)

**AC-OBS-METRICS-009**: `TestExpvarSink_Labels_NameMangling` 테스트는 `Counter("cmdctx.method.calls", Labels{"method": "OnClear"})` 와 `Counter("cmdctx.method.calls", Labels{"method": "OnCompact"})` 가 서로 다른 expvar 변수에 매핑되며, 동일 `(name, labels)` 재호출 시 동일 underlying counter 에 누적됨을 검증한다. (REQ-OBS-METRICS-006, REQ-OBS-METRICS-004)

**AC-OBS-METRICS-010**: `TestExpvarSink_CardinalityCap_DropsAndWarns` 테스트는 동일 metric name 에 101 개 unique label combination 으로 `Counter` 를 등록 시도할 때, 처음 100 개는 정상 등록되고 101 번째는 noop counter 를 반환하며 warn log 가 정확히 1회 발생함을 검증한다. (REQ-OBS-METRICS-009, REQ-OBS-METRICS-015)

### 5.3 동작 검증 — noop 어댑터

**AC-OBS-METRICS-011**: `TestNoopSink_AllOps_NoSideEffect` 테스트는 noop sink 의 `Counter().Inc()` / `Counter().Add()` / `Histogram().Observe()` / `Gauge().Set()` / `Gauge().Add()` 가 panic 없이 정상 반환되며, 외부 관찰 가능한 부수효과 (expvar 변수 등록 등) 가 발생하지 않음을 검증한다. (REQ-OBS-METRICS-011)

**AC-OBS-METRICS-012**: `BenchmarkNoopSink_CounterInc` 벤치마크는 noop counter 의 `Inc()` 호출이 ≤ 5ns/op 임을 검증한다. (REQ-OBS-METRICS-011 의 zero-cost target)

### 5.4 concurrent / race 검증

**AC-OBS-METRICS-013**: `TestExpvarSink_ConcurrentInc_RaceFree` 테스트는 100 goroutine 이 동일 `Counter("concurrent", nil).Inc()` 를 1000회씩 호출 후 누적 값이 정확히 100,000 임을 검증한다. `go test -race ./internal/observability/metrics/expvar/...` 통과. (REQ-OBS-METRICS-010)

**AC-OBS-METRICS-014**: `TestExpvarSink_ConcurrentHistogram_RaceFree` 테스트는 100 goroutine 이 동일 histogram 에 `Observe(rand.Float64() * 100)` 를 100회씩 호출 후 모든 bucket 의 카운트 합이 10,000 임을 검증한다. (REQ-OBS-METRICS-010)

### 5.5 정적 / 빌드 검증

**AC-OBS-METRICS-015**: `go vet ./internal/observability/metrics/...` 통과. `go build ./internal/observability/metrics/...` 통과. `golangci-lint run ./internal/observability/metrics/...` warning 0건.

**AC-OBS-METRICS-016**: `internal/observability/metrics/...` 패키지 트리의 line coverage 가 ≥ 85% (TRUST 5 standard).

**AC-OBS-METRICS-017**: `go.mod` 의 direct require 블록에 `expvar` 또는 그 외 metrics 라이브러리 신규 추가 없음 (`expvar` 는 stdlib). indirect 의 OTel 패키지는 `go mod tidy` 후 변동 없음 (`connectrpc/otelconnect` 외 추가 indirect 없음).

### 5.6 contract / 호환성 검증

**AC-OBS-METRICS-018**: consumer SPEC `SPEC-GOOSE-CMDCTX-TELEMETRY-001` 의 `MetricsSink` interface 정의가 본 SPEC 의 `metrics.Sink` interface 와 호환됨을 다음 방법으로 **필수 검증** (run phase 시 (b) 옵션이 검증 경로):
- (b) **[필수, 검증 경로]** 별도 컴파일 검증 테스트 (`internal/observability/metrics/contract_test.go`) 가 `var _ MetricsSinkContract = (*expvarSink)(nil)` 패턴으로 contract assertion (plan-audit 2026-04-30 권장에 따라 "선택" → "필수" 격상).
- (a) **[informational, downstream]** consumer SPEC TELEMETRY-001 의 `internal/command/adapter/metrics.go` 가 `type MetricsSink = metrics.Sink` (alias) 또는 `import "github.com/modu-ai/goose/internal/observability/metrics"` 후 직접 사용. consumer SPEC implementation 시점에 해소되며, 본 SPEC 의 run phase 테스트 스위트는 (b) 만 수행한다.
- 본 SPEC 은 sink contract 가 consumer 의 6 메서드 emission 패턴 (calls counter + errors counter + duration histogram) 을 모두 지원함을 spec.md 본문 + research.md §4.3 으로 보장한다. (REQ-OBS-METRICS-003)

### 5.7 REQ-004/005 직접 검증 AC (2026-05-04 신규)

**AC-OBS-METRICS-019**: `TestExpvarSink_HandleReuse_SameKey` 테스트는 동일 `(name, labels)` 조합으로 factory 메서드를 다회 호출 시 (a) 반환된 두 handle 이 동일 underlying counter 에 누적됨을 검증한다 (`h1 := sink.Counter("x", L1); h2 := sink.Counter("x", L1); h1.Inc(); h2.Inc(); expvar.Get("x") == 2`), 또는 (b) 동일 handle 인스턴스를 반환함을 검증한다. 어느 쪽이든 호출자의 handle caching 패턴 미강제 contract 를 만족한다. (REQ-OBS-METRICS-004 직접 매핑)

**AC-OBS-METRICS-020**: `TestExpvarSink_Labels_PostCallMutationInvariance` 테스트는 (a) `labels := Labels{"method": "OnClear"}` 로 `Counter("x", labels)` 등록 후, (b) caller 가 `labels["method"] = "Mutated"` 로 입력 map 을 변형하더라도, (c) 등록된 series 의 label key/value 가 caller 의 mutation 에 의해 영향받지 않음을 검증한다 (sink 가 caller-provided labels 를 정적으로 보존하는지 확인). 변형 가능성이 있는 caller-side 데이터에 대해 sink 가 snapshot 또는 immutable 처리를 채택했음을 확인. (REQ-OBS-METRICS-005 직접 매핑)

### 5.8 REQ ↔ AC 커버리지 매트릭스 (2026-05-04 전면 재작성, plan-audit iteration 2 FAIL fix)

본 매트릭스는 §4 (L138-L188) 의 normative REQ 정의를 line-by-line 재독한 후 작성한다. 이전 iteration 의 매트릭스가 sibling SPEC 의 REQ 번호와 어긋나 15/20 행 분류 오류를 포함했던 회귀 결함을 정정한다.

| REQ ID | EARS Category | Topic | Mapped ACs | Coverage Status |
|---|---|---|---|---|
| REQ-OBS-METRICS-001 | Ubiquitous | Sink/Counter/Histogram/Gauge 4 interface + Labels type export, thread-safe godoc | AC-001, AC-002 | Direct (interface 존재 + signature 검증) |
| REQ-OBS-METRICS-002 | Ubiquitous | Sink 3 factory 메서드 (Counter/Histogram/Gauge) + thread-safe handle | AC-002, AC-003, AC-004, AC-005, AC-008 | Direct (signature + handle 동작) |
| REQ-OBS-METRICS-003 | Ubiquitous | metrics.Sink ↔ TELEMETRY-001 adapter.Options.Metrics 호환 | AC-018 | Direct (contract assertion 필수) |
| REQ-OBS-METRICS-004 | Ubiquitous | handle reuse — 동일 (name, labels) factory 다회 호출 시 동일/누적 handle | AC-019 | Direct (2026-05-04 신규 AC) |
| REQ-OBS-METRICS-005 | Ubiquitous | Labels 정적 보존 — caller 입력 그대로 보존, sink 측 변형 없음 | AC-020, AC-009 (보조) | Direct (2026-05-04 신규 AC) + 보조 검증 |
| REQ-OBS-METRICS-006 | Event-Driven | WHEN Counter() — 신규 (name,labels) 등록 또는 기존 handle 반환, atomic Inc/Add | AC-006, AC-009 | Direct (counter increment + label distinguish) |
| REQ-OBS-METRICS-007 | Event-Driven | WHEN Histogram() — bucket init 또는 reuse, nil 시 default bucket | AC-007 | Direct (bucket observe + default fallback) |
| REQ-OBS-METRICS-008 | Event-Driven | WHEN env GOOSE_METRICS_ENABLED 검사 — expvar/noop sink 분기 | (deferred) | Deferred to SPEC-GOOSE-CMDCTX-CLI-INTEG-001 / DAEMON-INTEG-001 (env wiring 진입점은 본 SPEC scope 외, REQ-008 마지막 문장 명시) |
| REQ-OBS-METRICS-009 | Event-Driven | WHEN expvar 신규 (name,labels) 등록 시 cardinality cap 초과 → silent drop + 1 warn log | AC-010 | Direct |
| REQ-OBS-METRICS-010 | State-Driven | WHILE Sink 사용 중 factory+handle 메서드 race-free, -race detector PASS | AC-013, AC-014 | Direct (counter race + histogram race) |
| REQ-OBS-METRICS-011 | State-Driven | IF Sink == noop, 모든 메서드 no-op + ≤5ns benchmark | AC-005, AC-011, AC-012 | Direct (factory + no-side-effect + benchmark) |
| REQ-OBS-METRICS-012 | State-Driven | IF Sink == nil, caller wiring noop fallback (caller 책임) | (informational) | Caller responsibility — wiring SPEC scope. 본 SPEC interface 자체는 nil receiver 미지원 |
| REQ-OBS-METRICS-013 | Unwanted | Labels 값에 PII (prompt/credential/path/email/user-id) 포함 금지 | (informational, deferred) | Caller responsibility — 정적 분석 도입은 별도 SPEC (Exclusions #10, TBD `SPEC-GOOSE-OBS-METRICS-LINT-001`). Run phase 는 godoc + code review 로 enforce |
| REQ-OBS-METRICS-014 | Unwanted | Sink 구현체는 (name, labels) 변형(normalization/hashing/redaction) 금지 | AC-009 (간접), AC-019/020 (보조) | Indirect — labels 가 그대로 보존됨을 다른 AC 가 입증. 명시적 non-normalization 검증은 godoc contract |
| REQ-OBS-METRICS-015 | Unwanted | expvar cardinality cap 초과 시 panic 금지 (silent drop + log 만 허용) | AC-010 | Direct (test 가 panic 부재 + warn log 1회 검증) |
| REQ-OBS-METRICS-016 | Unwanted | 어떤 구현체도 backend SDK lifecycle (init/shutdown/flush) 을 caller 에게 요구하지 않음 | (godoc/NFR-verified) | Verified by §6.1 Sink interface 정의에 Close()/Shutdown()/Flush() 메서드 부재 + godoc 명시 ("MUST NOT require lifecycle calls"). Test 불필요 — interface signature 부재가 contract |
| REQ-OBS-METRICS-017 | Unwanted | 본 SPEC 의 산출물은 `internal/observability/metrics/` 트리만; consumer 패키지 emission code 작성 안 함 | (NFR/scope) | Scope discipline — §10 Deliverables 표가 산출물 트리 enumerate. PR diff review 로 enforce |
| REQ-OBS-METRICS-018 | Optional | WHERE daemon + net/http active, /debug/vars 자동 노출 (access control 본 SPEC scope 외) | (deferred) | Deferred to SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (HTTP server 활성화 컨텍스트). 본 SPEC 은 expvar `init()` 의 부수효과 godoc 명시만 |
| REQ-OBS-METRICS-019 | Optional | WHERE 운영자가 cap 변경 원함, v0.2.0 `GOOSE_METRICS_LABEL_CAP` env amendment 가능 (v0.1.0 hardcoded) | (informational, future) | Future amendment — v0.1.0 cap 100 hardcoded. 본 SPEC 검증 대상 아님 |
| REQ-OBS-METRICS-020 | Optional | WHERE caller 도메인별 bucket customize 원함, Histogram(name, labels, buckets) parameter 로 명시 | AC-007 | Direct (b) — buckets nil 시 default fallback 검증; non-nil 시 caller 책임 |

**커버리지 결론**:
- 직접 검증 AC 매핑 보유 normative REQ: REQ-001, 002, 003, 004, 005, 006, 007, 009, 010, 011, 015, 020 (12개). 모두 1개 이상의 specific AC 가 검증 책임을 짊.
- godoc/NFR/scope-verified REQ: REQ-014 (간접 + godoc), REQ-016 (interface signature 부재 + godoc), REQ-017 (Deliverables enumerate + PR review).
- Out-of-scope deferred REQ: REQ-008 (env wiring → CLI/DAEMON-INTEG SPEC), REQ-018 (HTTP exposure → DAEMON-INTEG SPEC).
- Caller-responsibility informational REQ: REQ-012 (nil sink fallback), REQ-013 (PII firewall — 정적 분석 SPEC TBD).
- Future amendment REQ: REQ-019 (v0.2.0 amendment).

AC-006 의 REQ 매핑 정정: 이전 iteration 매트릭스가 AC-006 을 REQ-004/005/016 의 검증으로 over-claim 했으나, AC-006 (`TestExpvarSink_Counter_Increments`) 은 counter increment monotonicity (REQ-006) 만 검증한다. handle reuse (REQ-004) 와 Labels caller-preservation (REQ-005) 은 본 iteration 신규 추가된 AC-019/AC-020 가 직접 검증하며, lifecycle 0 (REQ-016) 은 §6.1 interface signature 부재로 verify 한다.

**plan-audit iteration 2 defect 해소 상태 (정직 보고)**:
- D1 (§5.7 매트릭스 systematic mislabel, critical): **해소** — 본 §5.8 매트릭스를 §4 line-by-line 재독으로 전면 재작성. 모든 (REQ topic, EARS category) 정정.
- D2 (자기주장 블록 false-claim, critical): **해소** — 본 절을 정직 보고로 재작성. AC-006 의 REQ-004/005/016 over-mapping 철회.
- D3 (AC-006 over-mapping, major): **해소** — REQ-004 → AC-019, REQ-005 → AC-020 분리. AC-006 은 REQ-006 단일 매핑.
- D4 (REQ-008 env-toggle AC fabrication, major): **해소** — 매트릭스에 "Deferred to CLI-INTEG-001 / DAEMON-INTEG-001" 명시.
- D5 (AC-018 ambiguity, minor): **해소** — §5.6 AC-018 본문에서 (b) "검증 경로", (a) "informational, downstream" 으로 우선순위 명확화.

**iteration 1 prior defect 회귀 검증**:
- Prior D1 (REQ-AC 매트릭스 부재) → iteration 2 에서 매트릭스 추가했으나 mislabel 회귀 → iteration 3 에서 정정 매트릭스로 진정 해소.
- Prior D2 (REQ-004/005/016 직접 매핑 부재) → iteration 2 에서 false-claim 회귀 → iteration 3 에서 AC-019/AC-020 신규 + REQ-016 godoc/NFR 명시로 진정 해소.
- Prior D3 (AC-018 [필수] 격상) → iteration 2 에서 해소 유지.

---

## 6. 데이터 모델 (Data Model)

### 6.1 `Sink` interface 정의

```go
// internal/observability/metrics/metrics.go (신규)
//
// Package metrics provides a vendor-neutral metrics emission surface used
// across goose's runtime. Implementations include:
//   - expvar (default Phase 1 backend, stdlib-only)
//   - noop (zero-cost fallback)
// Future Phase 2/3 SPECs will add OpenTelemetry and Prometheus adapters.
package metrics

// Sink is the abstract metrics emission interface.
//
// Implementations MUST:
//   - Be thread-safe across all factory and handle methods.
//   - Cap label cardinality per metric name (silent drop on overflow).
//   - Avoid panics regardless of caller input (cardinality, label values).
//
// Implementations MUST NOT:
//   - Mutate, normalize, hash, or redact caller-provided labels.
//   - Require lifecycle calls (init/shutdown/flush) from callers.
//
// Caller responsibility (NOT enforced by Sink):
//   - PII firewall: caller must not include user prompts, model outputs,
//     credentials, raw user identifiers, or absolute home paths in labels.
//   - Cardinality discipline: caller should keep dynamic label combinations
//     bounded; Sink will silently drop overflow and emit a single warn log.
//
// @MX:ANCHOR: vendor-neutral metrics surface, fan_in >= 3 expected
//             (cmdctx adapter, future router emission, future loop emission).
// @MX:SPEC: SPEC-GOOSE-OBS-METRICS-001 REQ-OBS-METRICS-002
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

// Histogram is a value-distribution observer.
// Buckets are implementation-defined when caller passes nil/empty.
type Histogram interface {
    Observe(value float64)
}

// Gauge is a settable instantaneous-value handle.
type Gauge interface {
    Set(value float64)
    Add(delta float64)
}

// Labels is a static dimension map. PII and high-cardinality dynamic
// values (err.Error(), user IDs, prompt content) MUST NOT appear here
// per REQ-OBS-METRICS-013 (caller responsibility).
type Labels map[string]string
```

### 6.2 expvar 어댑터 골격

```go
// internal/observability/metrics/expvar/expvar.go (신규)
package expvar

import (
    stdexpvar "expvar"
    "log/slog"
    "sync"

    "github.com/modu-ai/goose/internal/observability/metrics"
)

const defaultLabelCap = 100

// New returns an expvar-backed metrics.Sink with the default cardinality cap (100).
// All metrics are exposed via stdlib expvar's /debug/vars endpoint.
//
// @MX:NOTE: stdlib expvar registers its handler at init() in net/http programs.
//           CLI-only programs without HTTP server still accumulate counters
//           in-memory (useful for end-of-process debugging).
// @MX:SPEC: SPEC-GOOSE-OBS-METRICS-001 REQ-OBS-METRICS-018
func New(logger *slog.Logger) metrics.Sink {
    return &sink{logger: logger, labelCap: defaultLabelCap, /* ... */}
}

// (... 내부 구현 — bucket array histogram, Mutex-protected map<seriesKey>handle, etc.)
```

### 6.3 noop 어댑터 골격

```go
// internal/observability/metrics/noop/noop.go (신규)
package noop

import "github.com/modu-ai/goose/internal/observability/metrics"

// New returns a no-op metrics.Sink. All methods return shared no-op handles.
// Use as default fallback when GOOSE_METRICS_ENABLED is unset/false.
func New() metrics.Sink {
    return noopSink{}
}

type noopSink struct{}

func (noopSink) Counter(string, metrics.Labels) metrics.Counter   { return noopCounter{} }
func (noopSink) Histogram(string, metrics.Labels, []float64) metrics.Histogram { return noopHistogram{} }
func (noopSink) Gauge(string, metrics.Labels) metrics.Gauge       { return noopGauge{} }

type noopCounter struct{}

func (noopCounter) Inc()                {}
func (noopCounter) Add(delta float64)   {}

type noopHistogram struct{}

func (noopHistogram) Observe(value float64) {}

type noopGauge struct{}

func (noopGauge) Set(value float64) {}
func (noopGauge) Add(delta float64) {}
```

### 6.4 metric name + label key naming convention (가이드라인, HARD 아님)

- metric name: `<area>.<subarea>.<measure>` — dot-separated, all lowercase. 예: `cmdctx.method.calls`, `router.dispatch.duration_ms`.
- label key: snake_case, 영문, 2-20자. 예: `method`, `error_type`, `provider`.
- label value: 정적 enum 또는 backend-safe 문자열. dynamic 값 금지 (REQ-OBS-METRICS-013).
- 본 가이드라인 위반은 contract 위반 아님 (caller 자유). 단, dashboard / query 호환성 위해 권장.

### 6.5 env toggle contract

| env 값 | 동작 | wiring 진입점 |
|--------|------|---------------|
| `GOOSE_METRICS_ENABLED=true` 또는 `=1` | expvar.New() sink 주입 | CLI / daemon (별도 SPEC) |
| `GOOSE_METRICS_ENABLED=false` 또는 `=0` 또는 미설정 (alpha) | noop.New() sink 주입 | CLI / daemon |
| 미설정 (beta+) | expvar.New() sink 주입 | CLI / daemon |

본 SPEC 은 env 값을 검사하지 않는다. wiring 진입점 SPEC (CLI-INTEG / DAEMON-INTEG) 의 책임.

---

## 7. 통합 (Integration)

### 7.1 SPEC 의존 관계 그래프

```
SPEC-GOOSE-OBS-METRICS-001 (본 SPEC, planned, P2)
  ├─ depends on: 없음 (1차 도입, 의존성 0 추가)
  ├─ informs: SPEC-GOOSE-CMDCTX-TELEMETRY-001 (planned, P3)
  │           → 본 SPEC implemented 시 BLOCKER 해소.
  │             consumer 의 adapter.Options.Metrics 에 metrics.Sink 직접 주입.
  ├─ informs: SPEC-GOOSE-CMDCTX-CLI-INTEG-001 (planned)
  │           → CLI 진입점에서 env 검사 후 sink 인스턴스 주입.
  ├─ informs: SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (planned)
  │           → daemon 진입점에서 동일 패턴 + /debug/vars 자동 노출.
  ├─ Phase 2 후속: SPEC-GOOSE-OBS-METRICS-OTEL-001 (TBD)
  │                → OTel SDK 어댑터, distributed tracing 도입 시.
  └─ Phase 3 후속: SPEC-GOOSE-OBS-METRICS-PROM-001 (TBD)
                   → Prometheus client_golang 어댑터, 사용자 요청 시.
```

### 7.2 consumer SPEC TELEMETRY-001 의 BLOCKER 해소 방식

**Before (현재, BLOCKER)**:
- consumer SPEC §3.1 #1: "`MetricsSink` interface — `internal/command/adapter/metrics.go` (신규 파일) 또는 선행 SPEC 의 sink 를 그대로 alias."
- consumer SPEC §R1: "선행 metrics 인프라 SPEC `SPEC-GOOSE-OBS-METRICS-001` 부재 — 본 SPEC implementation 불가."
- 상태: consumer SPEC plan 완료, run 진입 불가 (BLOCKER R1, 1급).

**After (본 SPEC implemented 시)**:
- consumer SPEC 의 `internal/command/adapter/metrics.go` 가 본 SPEC 의 `metrics.Sink` 를 alias 또는 직접 사용:

  ```go
  // internal/command/adapter/metrics.go (consumer SPEC implementation)
  package adapter

  import "github.com/modu-ai/goose/internal/observability/metrics"

  // MetricsSink aliases metrics.Sink for cmdctx adapter use.
  type MetricsSink = metrics.Sink
  type Counter = metrics.Counter
  type Histogram = metrics.Histogram
  type Labels = metrics.Labels
  ```

- consumer SPEC §R1 (BLOCKER) 해소 — `status: planned → blocked-resolved` 1줄 amendment.
- consumer run phase 진입 가능 (테스트 우선 RED 작성 → expvar/noop sink 주입 후 GREEN).

### 7.3 consumer SPEC §3.1 와의 contract 일치 검증

| consumer 요구 (TELEMETRY-001 §3.1 #1) | 본 SPEC 의 제공 | 호환 |
|--------------------------------------|---------------|-----|
| `Counter(name string, labels Labels) Counter` | `Sink.Counter(name, labels) Counter` | ✓ |
| `Histogram(name string, labels Labels) Histogram` | `Sink.Histogram(name, labels, buckets) Histogram` | ✓ (buckets 추가, consumer 가 nil 전달 시 default) |
| `Counter.Inc()`, `Counter.Add(delta float64)` | 동일 | ✓ |
| `Histogram.Observe(value float64)` | 동일 | ✓ |
| `Labels = map[string]string` | 동일 | ✓ |
| thread-safe 보장 | REQ-OBS-METRICS-010 보장 | ✓ |

→ consumer 의 6 메서드 emission (calls counter + errors counter + duration histogram, cardinality 24 series) 을 본 SPEC sink 가 100% 지원.

### 7.4 wiring 진입점 SPEC 의 책임 분리

- CLI / daemon 진입점에서 다음 wiring 코드를 추가 (별도 SPEC 책임):

  ```go
  // 별도 SPEC: SPEC-GOOSE-CMDCTX-CLI-INTEG-001 / DAEMON-INTEG-001
  import (
      "os"

      "github.com/modu-ai/goose/internal/observability/metrics"
      metricsexpvar "github.com/modu-ai/goose/internal/observability/metrics/expvar"
      metricsnoop "github.com/modu-ai/goose/internal/observability/metrics/noop"
  )

  func selectMetricsSink(logger *slog.Logger) metrics.Sink {
      switch os.Getenv("GOOSE_METRICS_ENABLED") {
      case "true", "1":
          return metricsexpvar.New(logger)
      default:
          return metricsnoop.New()
      }
  }
  ```

- 본 SPEC 의 책임: `metricsexpvar.New(logger)` 와 `metricsnoop.New()` factory 를 노출.
- wiring SPEC 의 책임: env 검사 + 진입점에서 `adapter.Options.Metrics` 주입.

---

## 8. 비-기능 요구사항 (Non-Functional Requirements)

| NFR ID | 항목 | 목표 | 측정 |
|--------|------|------|------|
| NFR-OBS-METRICS-001 | 패키지 line coverage | ≥ 85% | `go test -cover ./internal/observability/metrics/...` |
| NFR-OBS-METRICS-002 | race detector | 통과 | `go test -race ./internal/observability/metrics/...` |
| NFR-OBS-METRICS-003 | noop counter `Inc()` overhead | ≤ 5ns/op | `BenchmarkNoopSink_CounterInc` |
| NFR-OBS-METRICS-004 | expvar counter `Inc()` overhead | ≤ 100ns/op (uncontended) | `BenchmarkExpvarSink_CounterInc` |
| NFR-OBS-METRICS-005 | expvar histogram `Observe()` overhead | ≤ 200ns/op (5 bucket) | `BenchmarkExpvarSink_HistogramObserve` |
| NFR-OBS-METRICS-006 | golangci-lint warning | 0건 | `golangci-lint run ./internal/observability/metrics/...` |
| NFR-OBS-METRICS-007 | godoc 누락 | 모든 exported 심볼 godoc 보유 | `go doc -all` 수동 검토 |
| NFR-OBS-METRICS-008 | go.mod direct require 신규 추가 | 0건 | `git diff go.mod` (require 블록) |
| NFR-OBS-METRICS-009 | go.mod indirect 신규 추가 | 0건 (이미 indirect 인 OTel 변동 없음) | `git diff go.sum` |

NFR-OBS-METRICS-004 / 005 의 임계값은 plan phase 추정. run phase benchmark 결과로 조정 가능 (v0.2.0 amendment).

---

## 9. 위험 (Risks)

| ID | 위험 | 우선순위 | 완화 전략 |
|----|------|----------|-----------|
| R1 | expvar 어댑터의 histogram 자체 구현이 production-ready 수준 미달 (bucket overflow, percentile 미지원) | 중 | (a) bucket overflow: max bucket 초과 값은 별도 `+Inf` bucket counter 에 누적 (Prometheus 관행 모방). (b) percentile 미지원: 본 SPEC scope 외 — Phase 2 OTel 어댑터에서 OTel SDK percentile aggregation 사용. (c) Phase 1 의 expvar histogram 은 운영자 디버깅 / 카운트 수준 관찰용임을 godoc 명시. |
| R2 | cardinality cap (100) 이 실제 운영 시 부족 | 중 | (a) Phase 1 hardcoded 100, 부족 시 v0.2.0 amendment 로 env `GOOSE_METRICS_LABEL_CAP` 도입 (REQ-OBS-METRICS-019). (b) cap 도달 시 warn log 가 운영자에게 신호 — 빠른 감지 가능. |
| R3 | consumer SPEC TELEMETRY-001 의 `MetricsSink` interface 가 본 SPEC `metrics.Sink` 와 미세하게 불일치 (factory 시그니처 차이) | 낮음 | (a) §7.3 contract 일치 검증 표로 plan phase 시점에 호환성 확인. (b) consumer SPEC 의 §3.1 #1 이 "선행 SPEC 의 sink 를 그대로 alias" 로 명시 — 본 SPEC 이 source of truth, consumer 가 type alias 로 import. (c) Histogram 시그니처 차이: 본 SPEC 은 `buckets` parameter 추가, consumer 가 nil 전달 시 default bucket — backward compatible. |
| R4 | expvar `init()` 의 `/debug/vars` HTTP handler 자동 등록이 사용자가 의도하지 않은 endpoint 노출 | 낮음 | (a) godoc 에 명시: "expvar imports register the /debug/vars handler in net/http.DefaultServeMux at init time." (b) production daemon 의 endpoint 보안 분리는 별도 SPEC `SPEC-GOOSE-OBS-METRICS-SECURITY-001` (TBD). (c) CLI 단발 호출에는 `net/http` HTTP server 미실행 → 영향 없음. |
| R5 | Phase 2 OTel 어댑터 도입 시 본 SPEC 의 `Sink` interface 가 OTel `metric.Meter` 와 mapping 불가능 | 낮음 | (a) research.md §4.1 / §6.4 의 mapping 표로 mapping 가능성 검증 — Counter→Counter, Histogram→Histogram, Gauge→ObservableGauge or UpDownCounter 으로 자연스러움. (b) 호환성 테스트: Phase 2 SPEC implementation 시 동일 emit 패턴이 OTel collector 측에서 동일 series 로 수신됨을 검증. |
| R6 | `Sink` interface 의 factory 메서드가 hot path 에서 매번 호출 시 overhead | 낮음 | (a) NFR-004 / 005 의 임계값으로 측정. 초과 시 caller (consumer) 가 handle caching (한 번 획득 후 재사용). (b) 본 SPEC 은 caller 의 caching 패턴을 강제하지 않으나 godoc 에 권장. |

---

## Exclusions (What NOT to Build)

본 SPEC 이 명시적으로 다루지 않는 항목 (별도 SPEC 또는 운영 책임):

1. **OTel SDK 어댑터** (`internal/observability/metrics/otel/`) — Phase 2, `SPEC-GOOSE-OBS-METRICS-OTEL-001` (TBD). 본 SPEC implemented 후 작성. 본 SPEC 의 `Sink` interface 는 OTel mapping 호환 가능하도록 설계.
2. **Prometheus client_golang 어댑터** (`internal/observability/metrics/prometheus/`) — Phase 3, `SPEC-GOOSE-OBS-METRICS-PROM-001` (TBD). 사용자 요청 시.
3. **distributed tracing** (span propagation, trace ID 라벨 자동 주입) — `SPEC-GOOSE-TRACING-001` (TBD).
4. **consumer 패키지의 emission wiring** — TELEMETRY-001 (cmdctx adapter), 향후 router / loop / subagent / query / credential / fsaccess SPEC 의 책임. 본 SPEC 은 sink contract 만.
5. **CLI / daemon 진입점에서의 sink instance 주입** — `SPEC-GOOSE-CMDCTX-CLI-INTEG-001` / `SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001` (planned) 의 책임. env 검사 + factory 호출.
6. **alerting 정의 / dashboard JSON / metric query API** — backend 운영 인프라 측 (Grafana, Prometheus alertmanager, Datadog 등). 본 SPEC scope 외.
7. **exporter 운영 인프라** (OTLP collector 운영, Prometheus pull-scraping 설정, push gateway) — Phase 2/3 후속 SPEC + 운영 SPEC.
8. **`/debug/vars` endpoint 의 production security 분리** (admin port, unix socket, TLS, authentication) — `SPEC-GOOSE-OBS-METRICS-SECURITY-001` (TBD).
9. **histogram bucket 의 도메인별 default 결정** — 본 SPEC 은 `buckets []float64` parameter 만. consumer SPEC 가 도메인 지식으로 결정 (예: TELEMETRY-001 의 latency bucket: `[0.1, 0.5, 1, 5, 10, 50, 100, 500, 1000]` ms).
10. **PII 검출 / sanitization / redaction 로직** — caller 책임 (REQ-OBS-METRICS-013, REQ-OBS-METRICS-014). 본 SPEC sink 는 caller-provided labels 를 변형하지 않는다. 정적 분석 도입은 별도 SPEC.
11. **metric series 이름의 도메인별 catalog 관리** — 본 SPEC 은 naming convention 가이드라인만 (§6.4). catalog 관리 / 문서 자동화는 별도 docs SPEC.
12. **runtime metric 변경 검출 / drift alert** — 본 SPEC scope 외.
13. **multi-process / multi-node aggregation** — Phase 2/3 backend 의 책임.

---

## 10. 산출물 (Deliverables)

run phase 시점의 implementation 산출물 (예상):

| 파일 | 변경 종류 | LOC 예상 |
|------|-----------|----------|
| `internal/observability/metrics/metrics.go` | 신규 | ~80 (interface + Labels + godoc) |
| `internal/observability/metrics/expvar/expvar.go` | 신규 | ~200 (Sink + Counter/Histogram/Gauge 구현 + cardinality cap + bucket histogram) |
| `internal/observability/metrics/expvar/expvar_test.go` | 신규 | ~250 (10+ AC + race test) |
| `internal/observability/metrics/noop/noop.go` | 신규 | ~50 (모든 메서드 no-op) |
| `internal/observability/metrics/noop/noop_test.go` | 신규 | ~80 (no-op 검증 + 벤치마크) |
| `internal/observability/metrics/contract_test.go` | 신규 (선택) | ~40 (interface contract assertion) |
| `.moai/specs/SPEC-GOOSE-OBS-METRICS-001/progress.md` | 갱신 | run phase log |
| `go.mod` / `go.sum` | (변동 없음) | 0 |

총 신규 LOC ~700 (test 포함). size 분류: **중(M)** 적절.

---

## 11. 참조 (References)

- 본 디렉토리: `.moai/specs/SPEC-GOOSE-OBS-METRICS-001/`
  - `research.md` — backend 비교 + Phase 분리 결정 + sink interface 설계 결정.
  - `progress.md` — phase log.
- consumer SPEC: `.moai/specs/SPEC-GOOSE-CMDCTX-TELEMETRY-001/spec.md` (planned, P3, 본 SPEC implemented 시 BLOCKER 해소).
- 부모 SPEC: 없음 (1차 도입).
- Phase 2 후속 (TBD): `SPEC-GOOSE-OBS-METRICS-OTEL-001` — OTel SDK 어댑터.
- Phase 3 후속 (TBD): `SPEC-GOOSE-OBS-METRICS-PROM-001` — Prometheus 어댑터.
- 연관 Security SPEC (TBD): `SPEC-GOOSE-OBS-METRICS-SECURITY-001` — `/debug/vars` endpoint 보안 분리.
- 외부 reference:
  - Go stdlib `expvar`: <https://pkg.go.dev/expvar>
  - OpenTelemetry Go SDK: <https://pkg.go.dev/go.opentelemetry.io/otel> (Phase 2)
  - Prometheus client_golang: <https://pkg.go.dev/github.com/prometheus/client_golang> (Phase 3)
- 코드 참조 (read-only):
  - `go.mod` (current `main` HEAD `db5c81a`, indirect OTel 확인).
  - `internal/command/adapter/adapter.go` (consumer 측 adapter, TELEMETRY-001 의 amendment 대상 — 본 SPEC 직접 수정 안 함).

---

Version: 0.1.2 (implemented, P2, phase 3, size 중(M))
Last Updated: 2026-05-05
Status: implemented — 구현 본체 PR #60 (commit `43c46bf`, 2026-05-01 머지) + AC-019/020 augmentation PR #102 (commit `5c4e0a8`, 2026-05-05 머지) 로 main 도달. v0.1.2 sync 로 frontmatter status drift 정정 완료.
BLOCKER 해소 대상: `SPEC-GOOSE-CMDCTX-TELEMETRY-001` (planned, P3) — 본 SPEC implemented 도달로 R1 BLOCKER 는 v0.1.1 amendment (2026-05-01) 로 해소 완료 (🔴 → 🟡).
