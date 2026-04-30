# SPEC-GOOSE-OBS-METRICS-001 Progress

- Started: 2026-04-30 (plan phase)
- Status: planned
- Mode: TBD (run phase 시점에 quality.yaml development_mode 확인 — 현재 default tdd)
- Harness: standard (file_count<10 예상, 단일 Go domain — `internal/observability/metrics/`, security/payment 아님, observability 1차 도입)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: **완전 greenfield** — `internal/observability/` 디렉토리 자체가 신규. 기존 자산 amend 없음.
- Branch base: main (현재 main HEAD `db5c81a`, 본 SPEC 작업 시점 main 직접 분기 또는 feature/SPEC-OBS-METRICS-001 신설)
- Parent SPEC: 없음 (1차 도입)
- Prerequisite SPEC: **없음** — 본 SPEC 자체가 prerequisite 역할 (consumer 의 BLOCKER 해소 source)
- Sibling SPEC: 없음 (다른 amendment 와 충돌 없음)
- Informed SPEC (consumer / downstream):
  - **SPEC-GOOSE-CMDCTX-TELEMETRY-001** (planned, P3) — **본 SPEC implemented 시 BLOCKER 해소 대상** (1순위 consumer)
  - SPEC-GOOSE-CMDCTX-CLI-INTEG-001 (planned) — CLI 진입점에서 sink 주입 wiring
  - SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (planned) — daemon 진입점에서 sink 주입 wiring + `/debug/vars` 자동 노출
- Phase 2 후속 SPEC (TBD): SPEC-GOOSE-OBS-METRICS-OTEL-001 (OTel SDK 어댑터, beta 또는 distributed tracing 도입 시)
- Phase 3 후속 SPEC (TBD): SPEC-GOOSE-OBS-METRICS-PROM-001 (Prometheus 어댑터, 사용자 요청 시)

---

## Phase Log

### 2026-04-30 plan phase 시작

#### 선행 인프라 검증 (1차 도입 확인)

- `internal/` 트리에서 `metric*` / `telemetr*` / `observ*` 디렉토리: **0건**.
- `internal/` 의 Go 코드에서 `go.opentelemetry.io/otel`, `github.com/prometheus`, `expvar.`, `statsd` direct import: **0건**.
- `go.mod` direct require 의 metrics 라이브러리: **0건**.
- `go.mod` indirect 의 OTel: **있음** (`go.opentelemetry.io/otel v1.43.0`, `otel/metric v1.43.0`, `otel/trace v1.43.0`, `connectrpc.com/otelconnect v0.9.0` 경유).
- `.moai/specs/` 의 metric/telemetry/observ SPEC: 본 SPEC 작성 전 1건(`SPEC-GOOSE-CMDCTX-TELEMETRY-001` planned consumer).
- **결론: 본 레포는 metrics 인프라의 1차 도입. 본 SPEC 이 source of truth.**

#### Backend 비교 / 결정 (research.md 본문 §2 참고)

- 옵션 1: **stdlib `expvar` + 자체 sink wrapper**
  - direct require 추가 0건, indirect 추가 0건.
  - histogram / label first-class 미지원 → 자체 구현.
  - alpha v0.1.x 적합도: **높음**.
- 옵션 2: `go.opentelemetry.io/otel` SDK
  - direct 3-5개, indirect 10-20개 추가.
  - lifecycle / exporter 운영 인프라 부담 큼.
  - alpha 적합도: **낮음**. → Phase 2 별도 SPEC.
- 옵션 3: `github.com/prometheus/client_golang`
  - direct 1개, indirect 5-10개 추가.
  - CLI 단발 호출 부적합 (pull scraping).
  - alpha 적합도: **중**. → Phase 3 별도 SPEC.

**결정: Phase 1 = 옵션 1 (expvar) + sink interface**. 의존성 0 추가, alpha 무게 최소화, Phase 2/3 어댑터 추가 시 consumer 영향 0.

#### 부모 자산 확인 (read-only, 본 SPEC 이 amend 안 함)

- `internal/command/adapter/adapter.go` — consumer SPEC TELEMETRY-001 의 amendment 대상. 본 SPEC 직접 수정 없음.
- `internal/command/adapter/errors.go` — `ErrUnknownModel`, `ErrLoopControllerUnavailable` 정의. consumer SPEC 의 emission 분류 입력.
- `go.mod` — direct require 블록 변동 없음 (expvar 는 stdlib).

#### 신규 산출물 (run phase 예상)

1. `internal/observability/metrics/metrics.go` (~80 LOC)
   - `Sink`, `Counter`, `Histogram`, `Gauge` 4 interface + `Labels` 타입.
   - 모든 exported 심볼 godoc + emit 정책 (cardinality, PII) 명시.
   - `@MX:ANCHOR` (vendor-neutral surface, fan_in ≥ 3 예상).

2. `internal/observability/metrics/expvar/expvar.go` (~200 LOC)
   - stdlib `expvar` 기반 default `Sink` 구현.
   - counter: `expvar.Int.Add(1)` mapping.
   - gauge: `expvar.Float.Set/Add` mapping.
   - histogram: 자체 구현 (bucket array + `sync.Mutex`, `+Inf` overflow bucket).
   - cardinality cap (100) + silent drop + 1회 warn log.
   - `@MX:NOTE` (stdlib expvar `init()` 에서 `/debug/vars` 자동 등록).
   - `@MX:WARN` (cardinality cap 초과 silent drop, `@MX:REASON` 운영자 surprise 방지 가이드).

3. `internal/observability/metrics/noop/noop.go` (~50 LOC)
   - 모든 메서드 no-op 인 `Sink` 구현.
   - consumer 의 default 주입용.
   - `@MX:NOTE` (zero-cost fallback, `≤ 5ns/op` benchmark target).

4. `internal/observability/metrics/expvar/expvar_test.go` (~250 LOC)
   - 12 AC 검증 (AC-001 ~ AC-014, AC-018 제외 — consumer SPEC 시점에 검증).
   - race test (`go test -race`).
   - benchmark (`BenchmarkExpvarSink_CounterInc`, `BenchmarkExpvarSink_HistogramObserve`).

5. `internal/observability/metrics/noop/noop_test.go` (~80 LOC)
   - no-op 검증 + zero-cost 벤치마크 (`BenchmarkNoopSink_CounterInc`).

6. `internal/observability/metrics/contract_test.go` (~40 LOC, 선택)
   - interface contract assertion (`var _ metrics.Sink = (*expvarSink)(nil)`, `var _ metrics.Sink = noopSink{}`).

#### 수정 산출물

- 없음 (1차 도입). 기존 패키지 수정 0건.
- `go.mod` / `go.sum`: 변동 없음 (stdlib `expvar` 사용).

#### 의존 SPEC 의 wiring 패치 (본 SPEC 외, 후속 SPEC 의 책임으로 인계)

- **TELEMETRY-001 의 `internal/command/adapter/metrics.go`**: 본 SPEC implemented 시 신규 작성 — `type MetricsSink = metrics.Sink` (alias), `import "github.com/modu-ai/goose/internal/observability/metrics"`.
- **CLI-INTEG-001 의 `internal/cli/app.go`**: env `GOOSE_METRICS_ENABLED` 검사 후 `metricsexpvar.New(logger)` 또는 `metricsnoop.New()` 주입.
- **DAEMON-INTEG-001 의 daemon 진입점**: 동일 패턴 + `/debug/vars` HTTP endpoint 자동 노출 확인.

#### REQ / AC 카운트

- **REQ 총 20개** (REQ-OBS-METRICS-001 ~ REQ-OBS-METRICS-020):
  - Ubiquitous (§4.1): 5개 (REQ-001 ~ REQ-005)
  - Event-Driven (§4.2): 4개 (REQ-006 ~ REQ-009)
  - State-Driven (§4.3): 3개 (REQ-010 ~ REQ-012)
  - Unwanted (§4.4): 5개 (REQ-013 ~ REQ-017)
  - Optional (§4.5): 3개 (REQ-018 ~ REQ-020)
- **AC 총 18개** (AC-OBS-METRICS-001 ~ AC-OBS-METRICS-018):
  - 패키지 구조 / interface 검증 (§5.1): 5개 (AC-001 ~ AC-005)
  - 동작 검증 expvar (§5.2): 5개 (AC-006 ~ AC-010)
  - 동작 검증 noop (§5.3): 2개 (AC-011 ~ AC-012)
  - concurrent / race 검증 (§5.4): 2개 (AC-013 ~ AC-014)
  - 정적 / 빌드 검증 (§5.5): 3개 (AC-015 ~ AC-017)
  - contract / 호환성 검증 (§5.6): 1개 (AC-018, consumer SPEC 시점에 해소)
- **REQ-AC 매핑**: 모든 normative REQ (Ubiquitous + Event-Driven + State-Driven + Unwanted) 가 1개 이상 AC 와 cross-reference. Optional REQ (§4.5) 는 본 SPEC 시점 AC 없음 (도입 시점에 AC 추가).

#### Risks 요약 (spec.md §9 참고)

- **R1 (중)**: expvar histogram 자체 구현이 production-ready 수준 미달 (percentile 미지원) — Phase 2 OTel 어댑터에서 해결.
- R2 (중): cardinality cap 100 이 운영 시 부족 — v0.2.0 amendment 로 env `GOOSE_METRICS_LABEL_CAP` 도입.
- R3 (낮음): consumer TELEMETRY-001 의 `MetricsSink` 와 본 SPEC `metrics.Sink` 의 시그니처 불일치 — §7.3 contract 일치 표로 plan 시점 검증, type alias 패턴으로 호환.
- R4 (낮음): expvar `init()` 의 `/debug/vars` 자동 등록 보안 — godoc 명시 + 별도 SECURITY SPEC 으로 분리.
- R5 (낮음): Phase 2 OTel 어댑터 도입 시 mapping 불가 — research.md §4.1 mapping 표로 호환성 사전 검증.
- R6 (낮음): factory hot-path overhead — NFR-004/005 임계값 측정, caller handle caching 권장.

#### blocker

- **본 SPEC 자체에는 BLOCKER 없음** (1차 도입, 의존성 없음).
- 본 SPEC 이 consumer SPEC TELEMETRY-001 의 BLOCKER (R1, 1급) 를 해소하는 역할.

---

## 다음 단계 (run phase 진입 조건)

### Step 1: plan-auditor 검증

- spec.md 의 EARS 5 패턴 준수 확인 (Ubiquitous / Event-Driven / State-Driven / Unwanted / Optional 모두 사용).
- §Exclusions 13개 항목 → 1개 이상 ✓ (HARD 요구 충족).
- REQ 20 / AC 18 카운트 검증.
- consumer SPEC TELEMETRY-001 의 §3.1 #1 contract 와 본 SPEC §6.1 `Sink` interface 의 일치 검증 (§7.3 표).
- bias 점검: backend 결정의 정당성 (research.md §2.4 매트릭스 정량 비교 기반).

### Step 2: TELEMETRY-001 SPEC 의 BLOCKER 해소 amendment 준비

본 SPEC implemented 후 즉시 적용 (별도 commit):

- `.moai/specs/SPEC-GOOSE-CMDCTX-TELEMETRY-001/spec.md` 의 §R1 (BLOCKER) 완화: `🔴 높음 (blocker)` → `🟡 중 (해소됨, OBS-METRICS-001 implemented)`.
- §2.2 의 "**SPEC-GOOSE-OBS-METRICS-001** (**TBD — 부재**): ... " → "**SPEC-GOOSE-OBS-METRICS-001** (implemented, vX.Y.Z, sink interface 제공): ..."
- §3.1 #1 의 "`internal/command/adapter/metrics.go` (신규 파일) 또는 선행 SPEC 의 sink 를 그대로 alias" → "`internal/command/adapter/metrics.go` 가 `type MetricsSink = metrics.Sink` (alias) 패턴으로 본 SPEC 의 sink import."

### Step 3: run phase 진입 시

1. **테스트 우선 작성 (RED)** (TDD mode):
   - `metrics_test.go` — interface contract assertion test.
   - `expvar/expvar_test.go` — 10 AC 테스트 (Counter/Histogram/Gauge/cardinality cap/concurrent).
   - `noop/noop_test.go` — no-op 동작 + zero-cost 벤치마크.
   - 모두 fail 확인.

2. **구현 (GREEN)**:
   - `internal/observability/metrics/metrics.go` 작성 (interface + Labels + godoc).
   - `internal/observability/metrics/expvar/expvar.go` 작성 (Sink 구현 + bucket histogram + cardinality cap).
   - `internal/observability/metrics/noop/noop.go` 작성 (모든 메서드 no-op).
   - 모든 테스트 pass 확인.

3. **벤치마크 / refactor (REFACTOR)**:
   - NFR-003 (`BenchmarkNoopSink_CounterInc ≤ 5ns/op`) 검증.
   - NFR-004 (`BenchmarkExpvarSink_CounterInc ≤ 100ns/op`) 검증.
   - NFR-005 (`BenchmarkExpvarSink_HistogramObserve ≤ 200ns/op`) 검증.
   - 임계값 초과 시: lock-free path 추가 또는 sync.Pool handle 재사용.

4. **품질 gate**:
   - `go vet ./internal/observability/metrics/...` 통과.
   - `golangci-lint run ./internal/observability/metrics/...` warning 0건.
   - `go test -race ./internal/observability/metrics/...` 통과.
   - `go test -cover ./internal/observability/metrics/...` ≥ 85%.
   - `git diff go.mod go.sum` 변동 없음 확인 (NFR-008/009).

5. **MX 태그**:
   - `Sink` interface — `@MX:ANCHOR` (fan_in ≥ 3 예상, vendor-neutral surface).
   - cardinality cap 로직 — `@MX:WARN` + `@MX:REASON` (silent drop 동작).
   - expvar `/debug/vars` 자동 등록 — `@MX:NOTE`.
   - noop 패키지 — `@MX:NOTE` (zero-cost fallback).

### Step 4: PR 머지 후 sync

- 본 SPEC status `planned → implemented`.
- HISTORY 에 implementation commit hash 1줄 추가.
- consumer SPEC TELEMETRY-001 의 BLOCKER 해소 amendment commit (Step 2).
- consumer SPEC TELEMETRY-001 의 run phase 진입 가능 — `internal/command/adapter/` 의 6 메서드 emission wiring 진행.

### Step 5: Phase 2 후속 SPEC 작성 시점 결정

본 SPEC implemented 이후, 다음 trigger 시 Phase 2 SPEC `SPEC-GOOSE-OBS-METRICS-OTEL-001` (TBD) 작성:
- beta release 직전 (production observability 본격 도입).
- distributed tracing 도입 결정 시.
- 사용자 / 운영자가 OTLP collector / Grafana / Datadog 통합 요청 시.
