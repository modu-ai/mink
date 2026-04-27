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
- Prerequisite SPEC (BLOCKER): SPEC-GOOSE-OBS-METRICS-001 (**TBD — 부재, 본 SPEC implementation 의 blocker**)
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
    - **결론: 본 레포는 metrics 인프라를 도입한 적이 없다. 선행 SPEC `SPEC-GOOSE-OBS-METRICS-001` (TBD) 가 본 SPEC implementation 의 blocker.**
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
    - **R1 (🔴 높음/blocker)**: 선행 metrics 인프라 SPEC `SPEC-GOOSE-OBS-METRICS-001` 부재 → 본 SPEC implementation 불가. 권장 경로: prerequisite SPEC 별도 신규 작성. 임시 대안: §4.5 REQ-CMDCTX-TEL-018 (Logger.Debug fallback) — P3 우선순위에서만 권장.
    - R2 (중): CMDCTX-001 v0.x.0 amendment 가 sibling amendment SPEC 와 머지 충돌 가능 — sibling 먼저 머지되면 다음 minor 위에 본 SPEC amendment 발생.
    - R3 (중): emission overhead 가 hot path latency 에 영향 — nil sink fast-path (NFR-003 ≤ 10ns), non-nil sink 목표 (NFR-004 ≤ 200ns), run phase benchmark.
    - R4 (낮음): error_type cardinality 폭발 — 3-tier 분류 + AC-018 정적 검증.
    - R5 (낮음): sink panic 이 메서드 깨짐 — REQ-011 (defer recover) + AC-009.
    - R6 (낮음): WithContext shallow copy invariant — REQ-005 + AC-010.
  - blocker:
    - 본 SPEC 은 plan phase 완료. run phase 진입은 **prerequisite SPEC `SPEC-GOOSE-OBS-METRICS-001` 의 implemented status 충족 후** 또는 §4.5 REQ-CMDCTX-TEL-018 임시 대안만 부분 구현 (manager-spec 협의 필요).

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
