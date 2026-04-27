# SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd; run phase 결정 시 변경 가능)
- Harness: standard (file_count<10 예상, security/payment 아님 — daemon wiring helper + RPC handler glue)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `cmd/goosed/wire.go` 신규 helper, 의존 SPEC 의 인터페이스/타입은 implemented FROZEN 또는 planned 상태 그대로 보존
- Branch base: feature/SPEC-CMDCTX-FOLLOWUPS-batch-plan (현재 작업 중 batch)
- Parent SPEC 참고: SPEC-GOOSE-CMDCTX-001 v0.1.1 (PR #52, c018ec5 / 6593705 implemented FROZEN) — `ContextAdapter` 자산을 본 SPEC 이 daemon 진입점에서 wiring

## 작업 단위 분리 (ID 충돌 회피)

- 별도 SPEC: `SPEC-GOOSE-DAEMON-WIRE-001` (planned, P1, "goosed Production Daemon Wire-up — CORE × CONFIG × HOOK × TOOLS × SKILLS × CONTEXT × QUERY") — 5 레지스트리 + adapter + drain consumer 등록의 13-step bootstrap 을 다룸. 본 SPEC 의 작업과 분리.
- 본 SPEC: `SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001` — DAEMON-WIRE-001 의 step 10 직후, step 11 직전에 추가되는 4 sub-step (10.5 alias / 10.6 LoopController / 10.7 ContextAdapter / 10.8 Dispatcher) 의 wiring 만 담당.
- DAEMON-WIRE-001 의 SPEC 본문은 본 SPEC 작업 중 **변경하지 않는다**.

## Phase Log

- 2026-04-27 plan phase 시작
  - 부모 자산 확인:
    - `internal/command/adapter/adapter.go` — `*ContextAdapter`, `adapter.New(Options{...})` (CMDCTX-001 v0.1.1 FROZEN)
    - `internal/command/adapter/controller.go` — `LoopController` interface (CMDCTX-001 FROZEN, 4 메서드)
    - `internal/command/dispatcher.go` — `command.NewDispatcher` (COMMAND-001 implemented FROZEN)
    - `internal/llm/router/registry.go` — `router.DefaultRegistry()` (ROUTER-001 FROZEN)
  - 의존 SPEC 6 종 status 확인:
    - SPEC-GOOSE-DAEMON-WIRE-001 (planned, P1) — 13-step bootstrap framework
    - SPEC-GOOSE-CMDCTX-001 (implemented, FROZEN, v0.1.1) — adapter / Options
    - SPEC-GOOSE-CMDLOOP-WIRE-001 (planned, P0) — LoopControllerImpl 구현체
    - SPEC-GOOSE-ALIAS-CONFIG-001 (planned, P2) — aliasconfig.LoadDefault
    - SPEC-GOOSE-COMMAND-001 (implemented, FROZEN) — dispatcher
    - SPEC-GOOSE-ROUTER-001 (implemented, FROZEN) — DefaultRegistry
  - 본 SPEC 산출물: research.md, spec.md, progress.md (본 파일)
  - 다음 단계: plan-auditor 사이클은 본 위임 범위 외. 사용자 검토 후 /moai run 진입 시점은 DAEMON-WIRE-001 + CMDLOOP-WIRE-001 두 SPEC 의 implementation 완료를 선행 요건으로 한다 (ALIAS-CONFIG-001 부재는 graceful, 비-block).

## 산출물 요약

| 파일 | 라인 수 | 목적 |
|------|------|------|
| research.md | ~360 | wiring surface analysis, 13-step 안에서 step 10.5–10.8 의 위치, RPC ProcessUserInput ctx 전파, single-session 가정 유지, drain 순서 결정, alias graceful policy, panic recovery, EX_CONFIG fail-fast — 결정 D-1~D-10 |
| spec.md | ~470 | EARS 17 REQ + 11 AC, 데이터 모델 (wiring helper 시그니처 + 4 sub-step Go sketch + RPC error 매핑 표), 의존성 6 종, 위험 R-1~R-8, Exclusions 12 개 |
| progress.md | ~70 | phase log (본 파일) |

## REQ / AC 통계 (v0.1.0 기준)

- 총 REQ: **17**
  - Ubiquitous: 5 (REQ-DINTEG-001 ~ 005)
  - Event-Driven: 4 (REQ-DINTEG-010 ~ 013)
  - State-Driven: 2 (REQ-DINTEG-020 ~ 021)
  - Unwanted: 4 (REQ-DINTEG-030 ~ 033)
  - Optional: 2 (REQ-DINTEG-040, 041)
- 총 AC: **11** (AC-DINTEG-001 ~ 011)
  - 각 REQ 최소 1 개 매핑. AC-DINTEG-001 / 002 / 004 / 009 는 다중 REQ 커버.
- 커버리지 매트릭스: spec.md §5.1 참고

## 사용자 결정 보류 항목 (run phase 진입 전 확인 필요)

| ID | 결정 사항 | 본 SPEC 의 권장안 | 영향 |
|----|--------|----------------|------|
| D-1 | wiring helper 위치 (옵션 A: `cmd/goosed/main.go` 동일 파일 / B: `cmd/goosed/wire.go` 분리) | **옵션 B** (test 용이성, main.go 비대화 회피) | 코드 위치 |
| D-2 | DrainCoordinator 호출 순서 의미론 (LIFO vs FIFO) | implementation 시점 read-back 후 결정. 등록 순서를 의미론에 맞춰 inversion | drain consumer 등록 순서 |
| D-3 | RPC framework 가정 (Connect-gRPC vs vanilla gRPC vs HTTP+JSON) | TRANSPORT-001 결정에 위임. 본 SPEC §6.5 의 codes.* 매핑은 framework-agnostic | error 매핑 표 minor update |
| D-4 | LoopController.Drain 메서드 부재 시 처리 (CMDLOOP-WIRE-001 v0.1.0 SPEC 본문에 미정의) | graceful no-op fallback (drain consumer 가 즉시 nil 반환). CMDLOOP-WIRE-001 v0.2.0 amendment 에서 Drain 추가 후 본 SPEC 의 verification 강화 | AC-DINTEG-004 verification 강도 |
| D-5 | wiring helper 의 testable injection point 설계 (build tag vs interface 추상화 vs functional options) | functional options + nil-tolerant default | AC-DINTEG-007 fake stub 주입 방식 |
| D-6 | plan_mode metadata key naming (`plan_mode` vs `x-plan-mode` vs `goose-plan-mode`) | **`plan_mode`** (snake_case, framework-agnostic) | RPC response 호환성 |

## 다음 단계 (제안)

- [ ] plan-auditor iter 1 (선택): EARS 형식 검증, single-session 가정 명시 확인, REQ-AC 매트릭스 완전성, R-1~R-8 mitigation 적정성 점검
- [ ] 사용자 ratify: AskUserQuestion 으로 D-1 ~ D-6 결정 + 진행 여부
- [ ] /moai run 진입 선행 요건:
  - SPEC-GOOSE-DAEMON-WIRE-001 implementation 완료 (helper call site 가 main.go 에 마련됨)
  - SPEC-GOOSE-CMDLOOP-WIRE-001 implementation 완료 (`cmdctrl.New` 와 `Drain` 메서드 가용)
  - SPEC-GOOSE-ALIAS-CONFIG-001 implementation 권장 (부재해도 graceful 이지만 AC-DINTEG-003 verification 을 위해 권장)
- [ ] /moai run SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (선행 요건 충족 시):
  - Phase 2B TDD 예상 task: T-001 (wiring helper 시그니처 + 빈 stub) → T-002 (alias load 통합) → T-003 (loopCtrl + drain 등록) → T-004 (ctxAdapter + dispatcher) → T-005 (RPC handler glue + plan_mode metadata) → T-006 (panic recovery + error 매핑) → T-007 (StateServing 전 거부) → T-008 (overlay opt-in) → T-009 (race + integration test)
  - 예상 산출물:
    - `cmd/goosed/wire.go` (~150 LOC, wiring helper 본체)
    - `cmd/goosed/main.go` (~3 LOC 변경, helper call site 추가 + dispatcher / adapter 를 RPC service struct 에 주입)
    - `cmd/goosed/integration_test.go` (~300 LOC, AC-DINTEG-001~011 verification)
    - 옵션 A 채택 시 `cmd/goosed/wire_test.go` 별도 (helper 단위 테스트)
  - 커버리지 ≥ 90%, race detector pass, AC-DINTEG-001~011 모두 GREEN

## 주의 사항

- **DAEMON-WIRE-001 본문 변경 금지** — 본 SPEC 은 DAEMON-WIRE-001 v0.1.0 의 13-step 정본을 변경하지 않는다. implementation 단계에서 main.go 에 1 줄 helper call site 를 추가하는 것은 wiring 의 자연스러운 결과이며 DAEMON-WIRE-001 의 OUT OF SCOPE 영역에 해당.
- **CMDCTX-001 본문 변경 금지** — `*ContextAdapter`, `Options`, `SlashCommandContext` interface 는 v0.1.1 FROZEN 그대로 사용.
- **CMDLOOP-WIRE-001 본문 변경 금지** — `LoopController` interface 는 본 SPEC 의 wiring 대상. 인터페이스 자체는 그 SPEC 의 v0.1.0 정본 유지.
- **single-session 가정 명시** — REQ-DINTEG-005 / Exclusions §10 #1 / Risks R-4 에서 반복 명시. multi-session 으로 진화하려면 별도 SPEC.
- **본 SPEC 디렉토리만 작업** — 다른 동시 작성 중인 SPEC (CLI / DAEMON-WIRE / ALIAS-CONFIG / CMDLOOP-WIRE / CMDCTX-PERMISSIVE-ALIAS 등) 의 디렉토리는 read-only 분석에만 사용, write 금지.
- **CWD 유지** — 본 SPEC 작성은 main repo, 브랜치 `feature/SPEC-CMDCTX-FOLLOWUPS-batch-plan` 에서만 진행. 별도 worktree 미생성.
- **코드 주석 영어 (CLAUDE.local.md §2.5)** — 본 SPEC 의 §6 데이터 모델 sketch 안의 Go 주석은 영어로 작성. SPEC 본문 산문은 한국어.
