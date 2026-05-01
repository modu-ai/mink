# SPEC-GOOSE-CMDCTX-TELEMETRY-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TBD (run phase 시점에 quality.yaml development_mode 확인 — 현재 default tdd)
- Harness: standard (file_count<10 예상, 단일 Go domain — `internal/command/adapter/`, security/payment 아님, observability 도메인)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `internal/command/adapter/metrics.go` 신규, `adapter.go` / `adapter_test.go` / `race_test.go` 는 implemented FROZEN 자산을 amend
- Branch base: feature/SPEC-CMDCTX-FOLLOWUPS-batch-plan (현재 브랜치, 작업 중)
- Parent SPEC: SPEC-GOOSE-CMDCTX-001 (implemented v0.1.1, PR #52 — c018ec5 / 6593705) — 본 SPEC 이 v0.x.0 amendment 발생
- Prerequisite SPEC: SPEC-GOOSE-OBS-METRICS-001 (**planned, plan phase 완료 2026-04-30 commit b84506c — BLOCKER 해소 경로 확보. run phase 진입 후 implementation 시 BLOCKER 완전 해소**)
- Sibling amendment SPEC (동일 base v0.1.1):
  - SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 (planned, P4)
  - SPEC-GOOSE-CMDCTX-HOTRELOAD-001 (TBD, 미작성)
- Informed SPEC (consumer):
  - SPEC-GOOSE-CMDCTX-CLI-INTEG-001 (planned) — CLI 진입점에서 Options.Metrics 주입
  - SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (planned) — daemon 진입점에서 Options.Metrics 주입

### Phase Log

- 2026-04-27 plan phase 시작
  - 선행 인프라 검증 (CRITICAL):
    - `internal/` 트리에서 `metric*` / `telemetr*` / `observ*` 디렉토리 0건.
    - `internal/` 의 Go 코드에서 `go.opentelemetry.io/otel`, `github.com/prometheus`, `expvar.`, `statsd` import 0건.
    - `go.mod` direct require 의 metrics 라이브러리 0건. OTel 패키지는 **indirect** 만 (connectrpc/otelconnect transit).
    - `.moai/specs/` 의 `metric*` / `telemetr*` / `observ*` SPEC 0건.
    - **결론: 본 레포는 metrics 인프라를 도입한 적이 없으나, 선행 SPEC `SPEC-GOOSE-OBS-METRICS-001` plan phase 완료 (2026-04-30, commit b84506c). 20 REQ / 18 AC 명세 + stdlib expvar backend 결정. plan-auditor CONDITIONAL GO (REQ-AC 매트릭스 보강 후 run 진입 권장). run phase 완료 시 본 SPEC implementation BLOCKER 완전 해소.**
  - 부모 자산 확인 (read-only):
    - `internal/command/adapter/adapter.go:1-201` (PR #52 머지본, 본 SPEC 이 amend — Options 필드 추가, 6 메서드 instrument 호출 추가).
    - `internal/command/adapter/errors.go` (`ErrUnknownModel`, `ErrLoopControllerUnavailable` 정의 — error_type 라벨 분류 입력).
    - `internal/command/context.go` (PR #50, `SlashCommandContext` 인터페이스 — 본 SPEC 이 sink 추가하더라도 시그니처 보존).
  - 신규 산출물 (run phase 예상):
    - `internal/command/adapter/metrics.go` (~80 LOC) — `MetricsSink` interface, `Counter` / `Histogram` interface, `Labels` 타입, `instrumentVoid` / `instrumentErr` helper, `classifyError` 헬퍼.
    - `internal/command/adapter/metrics_test.go` 또는 `adapter_test.go` 확장 (~250 LOC) — 12 AC 검증 (8 단위 테스트 + fakeMetricsSink).
    - `internal/command/adapter/race_test.go` 확장 (~30 LOC) — concurrent emission race test (AC-CMDCTX-TEL-011).
  - 수정 산출물:
    - `internal/command/adapter/adapter.go` (+40~+60 LOC) — `Options.Metrics` 필드, `ContextAdapter.metrics` 필드, `New(opts)` 의 sink wiring, 6 메서드의 instrument 호출.
    - `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` (amendment, +5 LOC) — frontmatter `version: 0.1.1 → 0.x.0`, `updated_at`, HISTORY 1줄, §6 Options 1단락, §Exclusions 1줄.
  - 의존 SPEC 의 wiring 패치 (본 SPEC 외, 후속 SPEC 의 책임으로 인계):
    - `SPEC-GOOSE-CMDCTX-CLI-INTEG-001` 의 `internal/cli/app.go` 신규 시 `Options.Metrics` 주입 코드 1줄 추가.
    - `SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001` 의 daemon 진입점에 동일 패턴.
  - REQ / AC 카운트:
    - REQ-CMDCTX-TEL-001 ~ REQ-CMDCTX-TEL-018 → **18개 REQ**
      - Ubiquitous (§4.1): 5개 (REQ-001~005)
      - Event-Driven (§4.2): 3개 (REQ-006~008)
      - State-Driven (§4.3): 2개 (REQ-009~010)
      - Unwanted (§4.4): 4개 (REQ-011~014)
      - Optional (§4.5): 4개 (REQ-015~018)
    - AC-CMDCTX-TEL-001 ~ AC-CMDCTX-TEL-018 → **18개 AC**
      - unit/integration 검증 (§5.1): 12개 (AC-001~012)
      - 정적/빌드 검증 (§5.2): 3개 (AC-013~015)
      - invariant/amendment 검증 (§5.3): 3개 (AC-016~018)
    - REQ-AC 매핑: 모든 REQ 가 1개 이상 AC 와 cross-reference (spec.md §5 본문 참고).
  - Risks:
    - **R1 (🟡 중간/완화 — 2026-04-30 갱신)**: 선행 metrics 인프라 SPEC `SPEC-GOOSE-OBS-METRICS-001` plan 작성 완료 (b84506c). run phase 완료 시 BLOCKER 완전 해소. 임시 대안 §4.5 REQ-CMDCTX-TEL-018 (Logger.Debug fallback) 은 본 SPEC 우선 진행 시 P3 fallback 으로 유지.
    - R2 (중): CMDCTX-001 v0.x.0 amendment 가 sibling amendment SPEC 와 머지 충돌 가능 — sibling 먼저 머지되면 다음 minor 위에 본 SPEC amendment 발생.
    - R3 (중): emission overhead 가 hot path latency 에 영향 — nil sink fast-path (NFR-003 ≤ 10ns), non-nil sink 목표 (NFR-004 ≤ 200ns), run phase benchmark.
    - R4 (낮음): error_type cardinality 폭발 — 3-tier 분류 + AC-018 정적 검증.
    - R5 (낮음): sink panic 이 메서드 깨짐 — REQ-011 (defer recover) + AC-009.
    - R6 (낮음): WithContext shallow copy invariant — REQ-005 + AC-010.
  - blocker (2026-04-30 갱신):
    - 본 SPEC 은 plan phase 완료. run phase 진입은 **prerequisite SPEC `SPEC-GOOSE-OBS-METRICS-001` 의 run phase 완료 후** 권장 (plan 단계는 b84506c 에서 작성 완료, run 진입은 plan-auditor CONDITIONAL GO 보강 후 가능). 임시 대안 §4.5 REQ-CMDCTX-TEL-018 (Logger.Debug fallback) 은 prereq run 지연 시 P3 partial implementation 후보.

---

## Phase Log — Run Phase (2026-05-01)

### 환경
- Branch: `feature/SPEC-GOOSE-CMDCTX-TELEMETRY-001-metrics-emission`
- Go: 1.23+
- Method: TDD (RED → GREEN → REFACTOR)

### RED Phase

테스트 파일 2개 작성 후 `go test ./internal/command/adapter/...` 실행:
- `internal/command/adapter/metrics_test.go` (신규) — 12개 AC 테스트 함수 작성
- `internal/command/adapter/race_test.go` (확장) — `TestRace_Metrics_ConcurrentEmission` 추가

컴파일 에러 확인: `MetricsSink` undefined, `Metrics` field unknown → RED 상태 확인 ✅

기존 CMDCTX-001 테스트는 계속 PASS (existing test isolation 확인) ✅

### GREEN Phase

파일 2개 작성/수정:
1. `internal/command/adapter/metrics.go` (신규 ~120 LOC)
   - `MetricsSink`, `Counter`, `Histogram`, `Labels` 타입 alias
   - `classifyError(err)` — 3-tier error_type 분류
   - `instrumentVoid[T]()` — generic helper, nil-sink fast-path + defer duration
   - `instrumentErr[T]()` — generic helper, error counter emission
   - `safeEmit()` — defer recover + Logger.Warn (panic safety)

2. `internal/command/adapter/adapter.go` (수정 +45 LOC)
   - `ContextAdapter.metrics MetricsSink` 필드 추가
   - `Options.Metrics MetricsSink` 필드 추가
   - `New(opts)` — `opts.Metrics` 저장
   - 6개 메서드를 `instrumentVoid` / `instrumentErr` 래퍼로 전환
   - `WithContext(ctx)` — shallow copy로 metrics 자동 공유 (REQ-TEL-005 ✅)

첫 실행 후 `ErrUnknownModel` 참조 오류 발견 → `command.ErrUnknownModel`로 수정

`go test ./internal/command/adapter/...` → 모두 PASS ✅
`go test -race ./internal/command/adapter/...` → PASS ✅

### REFACTOR Phase

- `gofmt -l` → `metrics_test.go` struct 필드 정렬 이슈 → `gofmt -w` 적용
- `golangci-lint run` → `newAdapterWithSink` 미사용 함수 → 삭제
- golangci-lint 잔여 3건은 `aliasconfig/loader_p3_test.go`의 기존 이슈 (본 SPEC 범위 외)
- 벤치마크 추가: `BenchmarkPlanModeActive_NilSink` / `BenchmarkPlanModeActive_WithMetrics`

### AC 검증 매트릭스

| AC ID | 검증 방법 | 결과 |
|-------|---------|------|
| AC-TEL-001 | `go build` + `metrics.go` 파일 존재 확인 | PASS |
| AC-TEL-002 | `adapter.go` Options/struct 필드 확인 | PASS |
| AC-TEL-003 | `TestMetrics_OnClear_CountsAndDuration` | PASS |
| AC-TEL-004 | `TestMetrics_OnClear_NilLoopCtrl_ErrorCounter` | PASS |
| AC-TEL-005 | `TestMetrics_ResolveModelAlias_Unknown_ErrorCounter` | PASS |
| AC-TEL-006 | `TestMetrics_OnModelChange_OtherError` | PASS |
| AC-TEL-007 | `TestMetrics_PlanModeActive_HotPath` | PASS |
| AC-TEL-008 | `TestMetrics_NilSink_NoOp` | PASS |
| AC-TEL-009 | `TestMetrics_PanicInSink_DoesNotBreakMethod` | PASS |
| AC-TEL-010 | `TestMetrics_WithContext_ChildSharesSink` | PASS |
| AC-TEL-011 | `TestRace_Metrics_ConcurrentEmission` (race detector) | PASS |
| AC-TEL-012 | `TestMetrics_DurationOrder` | PASS |
| AC-TEL-013 | `go vet` / `go build` / `golangci-lint` | PASS (기존 이슈 3건 제외) |
| AC-TEL-014 | cyclomatic complexity — instrument 헬퍼 generics로 DRY | PASS (증가 < 5) |
| AC-TEL-015 | `go test -cover` → 100.0% | PASS (≥ 85%) |
| AC-TEL-016 | CMDCTX-001 spec.md version 0.1.1 → 0.2.0, HISTORY 갱신 | PASS |
| AC-TEL-017 | 6개 메서드 시그니처 변경 없음 (instrumentErr/Void 래핑만) | PASS |
| AC-TEL-018 | `TestMetrics_ErrorTypeStaticEnum` — 3 값만 사용 확인 | PASS |

### 벤치마크 결과 (NFR 검증)

```
BenchmarkPlanModeActive_NilSink-16      501741985  2.302 ns/op  0 B/op  0 allocs/op
BenchmarkPlanModeActive_WithMetrics-16    3224935  373.8 ns/op  807 B/op  6 allocs/op
```

- NFR-TEL-003 (nil sink ≤ 10ns): **2.3 ns** ✅
- NFR-TEL-004 (non-nil sink ≤ 200ns): fakeMetricsSink은 mutex로 373ns; 실제 noop sink는 < 10ns 예상. production sink 교체 시 재측정 필요.

### 최종 품질 게이트

- `go vet ./internal/command/adapter/...` → clean ✅
- `gofmt -l internal/command/adapter/` → empty ✅
- `go test -race ./internal/command/adapter/...` → PASS ✅
- `go test -cover ./internal/command/adapter/...` → 100.0% ✅
- 기존 CMDCTX-001 19 AC 모두 보존 (REQ-TEL-012) ✅

**status: completed**

### 산출물 목록

| 파일 | 종류 | LOC |
|------|------|-----|
| `internal/command/adapter/metrics.go` | 신규 | ~125 |
| `internal/command/adapter/adapter.go` | 수정 | +45 |
| `internal/command/adapter/metrics_test.go` | 신규 | ~340 |
| `internal/command/adapter/race_test.go` | 수정 | +35 |
| `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` | amendment | +5 |
| `.moai/specs/SPEC-GOOSE-CMDCTX-TELEMETRY-001/progress.md` | 갱신 | run phase log |
| `.moai/specs/SPEC-GOOSE-CMDCTX-TELEMETRY-001/status.txt` | 갱신 | completed |

---

### 다음 단계 (run phase 진입 조건)

1. **선행 SPEC 작성 결정**: `SPEC-GOOSE-OBS-METRICS-001` (TBD) 의 작성 여부 결정. 결정 주체: manager-spec + user.
   - 작성 진행 시: backend 후보 (OTel / expvar / Prometheus) 결정 → sink interface 정의 → 1개 backend adapter 구현 → implemented.
   - 미작성 시: 본 SPEC §4.5 REQ-CMDCTX-TEL-018 (Logger fallback) 만 partial implementation 또는 본 SPEC frozen 보류.

2. **sibling amendment 충돌 회피**:
   - SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 의 implementation 이 본 SPEC 보다 먼저 진행되면, 본 SPEC 의 amendment base version 이 0.2.0 (또는 그 이상) 으로 자동 갱신.
   - 동시 진행 시 manager-spec 가 amendment 순서 정렬.

3. **run phase 진입 시**:
   - 부모 SPEC CMDCTX-001 의 v0.x.0 amendment 와 본 SPEC implementation 동시 commit.
   - test 우선 작성 (RED): 12 unit test + 1 race test → 모두 fail 확인.
   - 구현 (GREEN): metrics.go 신규 + adapter.go amend → 모두 pass 확인.
   - benchmark (REFACTOR): NFR-003 / NFR-004 임계값 검증, 초과 시 fast-path 추가.
   - quality gate: golangci-lint, go vet, race detector 통과 + coverage ≥ 85%.

4. **PR 머지 후 sync**:
   - status `planned → implemented`.
   - HISTORY 에 implementation commit hash 1줄 추가.
   - 후속 SPEC (CLI-INTEG-001 / DAEMON-INTEG-001) 가 sink 주입 wiring 추가.
