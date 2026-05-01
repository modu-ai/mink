# SPEC-GOOSE-OBS-METRICS-001 — Task Decomposition

**SPEC**: SPEC-GOOSE-OBS-METRICS-001
**Mode**: TDD (RED-GREEN-REFACTOR per task)
**Harness**: standard (greenfield, single Go domain, file_count < 12)
**Source plan**: manager-strategy report (2026-04-30, UltraThink applied)
**Approved by**: GOOS행님 (Plan approval gate, 2026-04-30)

## Logger Decision

- spec.md §6.2 표기: `*slog.Logger` (참고용, plan-phase 추정)
- **확정**: `*zap.Logger` (사용자 결정, 2026-04-30)
  - 근거: `internal/` 20+ 파일이 zap 사용, slog는 3개 (ollama/agent/cmdctrl). 일관성 우선.
  - Test 검증: `zaptest.NewObservedCore` 로 warn log emission 확인.
  - HISTORY 갱신: implemented 전환 시 1줄 정정.

## Task Breakdown (TDD cycles)

| Task ID | Description | REQ | AC | Dependencies | Planned Files | Status |
|---------|-------------|-----|----|--------------|---------------|--------|
| T-001 | Define `Sink`/`Counter`/`Histogram`/`Gauge` interfaces + `Labels` type. RED: `metrics_test.go` interface contract test (compile-time `var _` + reflection method-set check). GREEN: define interfaces with English godoc + `@MX:ANCHOR` on `Sink`. REFACTOR: trim godoc. | REQ-001..003, REQ-005 | AC-001, AC-002, AC-003 | none | `internal/observability/metrics/metrics.go`, `internal/observability/metrics/metrics_test.go` | pending |
| T-002 | Implement noop adapter. RED: `noop/noop_test.go` `TestNoopSink_AllOps_NoSideEffect`. GREEN: empty bodies for all methods, shared singletons. REFACTOR: zero-allocation guarantee. `@MX:NOTE` on zero-cost. | REQ-011, REQ-012 | AC-005, AC-011 | T-001 | `internal/observability/metrics/noop/noop.go`, `internal/observability/metrics/noop/noop_test.go` | pending |
| T-003 | Contract assertion test (audit D3 [필수]). `var _ metrics.Sink = (*expvarSink)(nil)` + `var _ metrics.Sink = noopSink{}` + reflection method-set check. **Note**: T-003 작성 시점에 expvarSink 미존재로 컴파일 fail은 의도된 RED. T-006 closure 시 GREEN. | REQ-003, REQ-016, REQ-020 | AC-018 [필수] | T-002 | `internal/observability/metrics/contract_test.go` | pending |
| T-004 | expvar Counter + cardinality cap framework. RED: `TestExpvarSink_Counter_Increments`, `TestExpvarSink_Labels_NameMangling`, `TestExpvarSink_CardinalityCap_DropsAndWarns`. GREEN: `expvarSink.Counter()`, `seriesKey()` helper, per-metric `sync.Map` + `atomic.Int64` cap counter, warn-once via `sync.Once`. `@MX:WARN` + `@MX:REASON` on cap branch. | REQ-002, REQ-004, REQ-006, REQ-009, REQ-014 | AC-004, AC-006, AC-009, AC-010 | T-003 | `internal/observability/metrics/expvar/expvar.go`, `internal/observability/metrics/expvar/expvar_test.go`, `internal/observability/metrics/expvar/series_key.go` | pending |
| T-005 | expvar Gauge implementation. RED: `TestExpvarSink_Gauge_SetAndAdd`. GREEN: `expvarGauge` wrapping `expvar.Float` with `Set/Add`. Reuses cap framework from T-004. | REQ-002 | AC-008 | T-004 | `internal/observability/metrics/expvar/expvar.go` (extend) | pending |
| T-006 | expvar Histogram (가장 복잡). RED: `TestExpvarSink_Histogram_BucketsObserve` — bucket boundary cases + nil/empty fallback to `defaultBuckets [0.1,1,10,100,1000]` + `+Inf` overflow bucket. GREEN: sorted bucket array + parallel `[]atomic.Int64`, `sort.SearchFloat64s` placement, `expvar.Func` JSON exposure. Validate buckets at construction (panic-free, fall back to default on bad input). godoc: percentile not supported, use Phase 2 OTel adapter. T-003 contract closes here. | REQ-002, REQ-007 | AC-007 | T-005 | `internal/observability/metrics/expvar/expvar.go` (extend) | pending |
| T-007 | Concurrent / race tests. `TestExpvarSink_ConcurrentInc_RaceFree` (100 × 1000 = 100,000), `TestExpvarSink_ConcurrentHistogram_RaceFree` (100 × 100 = 10,000). `go test -race` clean. | REQ-010 | AC-013, AC-014 | T-006 | `internal/observability/metrics/expvar/expvar_concurrent_test.go` | pending |
| T-008 | Static + build gates. `go vet`, `golangci-lint run` (0 warnings), `git diff go.mod go.sum` (empty). | REQ-014 | AC-015, AC-017 | T-007 | (verification only, no new files) | pending |
| T-009 | NFR benchmarks + coverage. `BenchmarkNoopSink_CounterInc` (≤5ns), `BenchmarkExpvarSink_CounterInc` (≤100ns), `BenchmarkExpvarSink_HistogramObserve` (≤200ns), `go test -cover` ≥85%. Document handle-caching recommendation in godoc. | NFR-003/004/005, NFR-001 | AC-012, AC-016 | T-008 | `internal/observability/metrics/noop/noop_bench_test.go`, `internal/observability/metrics/expvar/expvar_bench_test.go` | pending |

**Total tasks: 9** | **Total new files: 11** | **Estimated LoC: ~700**

## File Inventory (project-root-relative)

NEW (11 files):
- `internal/observability/metrics/metrics.go` (~80 LoC)
- `internal/observability/metrics/metrics_test.go` (~60 LoC)
- `internal/observability/metrics/contract_test.go` (~50 LoC)
- `internal/observability/metrics/expvar/expvar.go` (~200 LoC)
- `internal/observability/metrics/expvar/series_key.go` (~30 LoC)
- `internal/observability/metrics/expvar/expvar_test.go` (~250 LoC)
- `internal/observability/metrics/expvar/expvar_concurrent_test.go` (~80 LoC)
- `internal/observability/metrics/expvar/expvar_bench_test.go` (~50 LoC)
- `internal/observability/metrics/noop/noop.go` (~50 LoC)
- `internal/observability/metrics/noop/noop_test.go` (~60 LoC)
- `internal/observability/metrics/noop/noop_bench_test.go` (~30 LoC)

MODIFIED: **none**

`go.mod` / `go.sum`: **byte-identical to baseline** (NFR-008/009, AC-017).

## HARD Constraints

1. 모든 코드 주석 영어 (godoc, inline, MX 태그 description) — CLAUDE.local.md §2.5
2. `go.mod` 신규 direct/indirect require 0건
3. AC-018 contract test 필수 (audit D3)
4. MX 태그 4종 배치:
   - `@MX:ANCHOR` on `Sink` interface
   - `@MX:WARN` + `@MX:REASON` on cardinality cap overflow branch
   - `@MX:NOTE` on `expvar.New()` (init side-effect /debug/vars)
   - `@MX:NOTE` on `noop.New()` (zero-cost fallback)
5. Logger: `*zap.Logger` (사용자 결정 2026-04-30)
6. Histogram default buckets: `[0.1, 1, 10, 100, 1000]` ms-scale
7. `+Inf` overflow bucket (Prometheus convention, R1 mitigation)

## Acceptance Criteria Mapping (18 AC)

| AC | Spec Section | T-XXX | Verification Method |
|----|-------------|-------|---------------------|
| AC-001 | §5.1 패키지 구조 | T-001 | File existence + godoc |
| AC-002 | §5.1 Sink signature | T-001 | Compile + reflection |
| AC-003 | §5.1 Counter/Histogram/Gauge methods | T-001 | Reflection method-set |
| AC-004 | §5.1 expvar.New() factory | T-004 | Test creates sink |
| AC-005 | §5.1 noop.New() factory | T-002 | Test creates sink |
| AC-006 | §5.2 Counter Inc/Add | T-004 | TestExpvarSink_Counter_Increments |
| AC-007 | §5.2 Histogram buckets | T-006 | TestExpvarSink_Histogram_BucketsObserve |
| AC-008 | §5.2 Gauge Set/Add | T-005 | TestExpvarSink_Gauge_SetAndAdd |
| AC-009 | §5.2 Labels mangling | T-004 | TestExpvarSink_Labels_NameMangling |
| AC-010 | §5.2 cardinality cap | T-004 | TestExpvarSink_CardinalityCap_DropsAndWarns |
| AC-011 | §5.3 noop no side effect | T-002 | TestNoopSink_AllOps_NoSideEffect |
| AC-012 | §5.3 noop ≤5ns | T-009 | BenchmarkNoopSink_CounterInc |
| AC-013 | §5.4 concurrent counter | T-007 | TestExpvarSink_ConcurrentInc_RaceFree |
| AC-014 | §5.4 concurrent histogram | T-007 | TestExpvarSink_ConcurrentHistogram_RaceFree |
| AC-015 | §5.5 vet/build/lint | T-008 | go vet + golangci-lint |
| AC-016 | §5.5 coverage ≥85% | T-009 | go test -cover |
| AC-017 | §5.5 go.mod stable | T-008 | git diff |
| AC-018 [필수] | §5.6 contract test | T-003 + T-006 | var _ assertions + reflection |

---

Version: 1.0.0
Created: 2026-04-30 (Phase 1.5)
