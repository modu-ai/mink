# SPEC-GOOSE-PLANMODE-CMD-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd, run phase 결정 시 변경 가능)
- Harness: minimal (file_count<=3 예상, 단일 Go 파일 + 테스트 + interface 정의, security/payment 아님)
- Scale-Based Mode: Small (size: 소(S))
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `internal/command/builtin/plan.go` 신규, `command.PlanModeSetter` interface 신규. 의존 SPEC 자산 (COMMAND-001, CMDCTX-001) 은 implemented FROZEN 그대로 사용 (호출만).
- Branch base: feature/SPEC-CMDCTX-FOLLOWUPS-batch-plan (현재 브랜치, 작업 중)
- Parent SPEC: SPEC-GOOSE-CMDCTX-001 (PR #52, c018ec5 / 6593705) — `ContextAdapter.SetPlanMode(bool)` 자산을 본 SPEC 이 사용자 진입점으로 노출
- Sibling SPEC (별개, 본 SPEC 의 builtin 패턴 source-of-truth): SPEC-GOOSE-COMMAND-001 (PR #50) — `command.Command` 인터페이스 + `internal/command/builtin/` 빌트인 명령 7개

### Phase Log

- 2026-04-27 plan phase 시작
  - 부모 자산 확인:
    - `internal/command/adapter/adapter.go:86-88` — `(*ContextAdapter).SetPlanMode(active bool)` (FROZEN, PR #52)
    - `internal/command/adapter/adapter.go:178-193` — `(*ContextAdapter).PlanModeActive() bool` (FROZEN, PR #52, atomic.Bool 기반)
    - `internal/command/context.go:31-33` — `SlashCommandContext.PlanModeActive() bool` (FROZEN, PR #50)
    - `internal/command/dispatcher.go:114` — `if cmd.Metadata().Mutates && sctx.PlanModeActive()` 차단 분기 (FROZEN, PR #50)
    - `internal/command/builtin/builtin.go:42-61` — `Register(reg registrar)` 빌트인 등록 (FROZEN, PR #50)
    - `internal/command/builtin/clear.go`, `model.go`, `compact.go`, `status.go`, `version.go`, `exit.go`, `help.go` — 빌트인 명령 패턴 (FROZEN, 본 SPEC 의 reference)
  - 신규 산출물 계획:
    - `internal/command/plan_mode_setter.go` (~12 LoC) — `PlanModeSetter` interface 정의
    - `internal/command/builtin/plan.go` (~70 LoC) — `planCommand` struct + Execute 분기
    - `internal/command/builtin/plan_test.go` (~120 LoC) — unit test (table-driven), AC 17개 커버
    - `internal/command/adapter/plan_mode_setter_assertion_test.go` (~10 LoC) — `var _ command.PlanModeSetter = (*adapter.ContextAdapter)(nil)` 컴파일 단언 (테스트 파일에서)
  - 변경 산출물 계획:
    - `internal/command/builtin/builtin.go` (+2 LoC) — `mustRegister(&planCommand{})` + `RegisterAlias("planmode", "plan")`
  - 의존 SPEC:
    - SPEC-GOOSE-COMMAND-001 (implemented PR #50, FROZEN) — 호출만, 변경 부재
    - SPEC-GOOSE-CMDCTX-001 (implemented PR #52 v0.1.1, FROZEN) — SetPlanMode 호출만, production 코드 변경 부재
    - SPEC-GOOSE-CMDCTX-CLI-INTEG-001 (planned) — 본 SPEC 의 trigger 와 indicator 결합 (REQ-PMC-019, 단일 방향 의존, 본 SPEC 의 implementation 은 CLI-INTEG 과 독립)
  - design 결정 기록:
    - 옵션 A (SlashCommandContext 본문 확장) — **거부** (FROZEN 위반)
    - 옵션 B (신규 `command.PlanModeSetter` narrow interface + type assertion) — **채택** (REQ-PMC-001 ~ REQ-PMC-002, REQ-PMC-013)
    - 옵션 C (`*adapter.ContextAdapter` 직접 의존) — **거부** (builtin 패턴 비대칭, REQ-PMC-014)
    - `Metadata.Mutates` — **false** (REQ-PMC-004): plan-mode-active 상태에서 `/plan off` 자체가 차단되면 deadlock. plan command 는 메타-제어로 변형 명령이 아님.
    - 컴파일 단언 위치 — **테스트 파일** (`adapter_test` 패키지) (§6.4 옵션 C): production 코드 변경 부재.
  - 다음 단계: plan-auditor 사이클은 본 위임 범위 외. 사용자 검토 후 /moai run 분기 결정.

### 산출물 요약

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| research.md | ~430 | builtin 패턴 분석, 옵션 A/B/C 비교, Metadata.Mutates 결정 근거, 인자 분기 사용자 모델, 위험 영역 5개 |
| spec.md | ~610 | EARS 20 REQ + 17 AC, planCommand 설계, PlanModeSetter interface, builtin.Register 변경, 의존성, Exclusions 13개 |
| progress.md | ~70 | phase log (본 파일) |

### REQ / AC 통계 (v0.1.0 기준)

- 총 REQ: 20
  - Ubiquitous: 6 (REQ-PMC-001 ~ 006)
  - Event-Driven: 5 (REQ-PMC-007 ~ 011)
  - State-Driven: 1 (REQ-PMC-012)
  - Unwanted: 4 (REQ-PMC-013 ~ 016)
  - Optional: 4 (REQ-PMC-017 ~ 020)
- 총 AC: 17 (각 REQ 최소 1개 매핑, REQ-PMC-019/020 은 결합 동작 명세로 cross-SPEC 검증 위임)
- 커버리지 매트릭스: spec.md §5 참고

### 사용자 결정 보류 항목 (run phase 진입 전 확인 필요)

| ID | 결정 사항 | 본 SPEC 의 권장안 | 영향 |
|----|--------|----------------|------|
| D-001 | PlanModeSetter 인터페이스 정의 위치 (A: `command/context.go` 본문 추가 / B: 별도 `command/plan_mode_setter.go`) | **옵션 B** (별도 파일, 본 SPEC 산출물 명확 분리) | COMMAND-001 의 implemented 파일 변경 인상 회피 |
| D-002 | 컴파일 타임 단언 위치 (A: `adapter/adapter.go` 에 한 줄 추가 / B: 본 SPEC 의 빌트인 plan.go / C: 테스트 파일) | **옵션 C** (`adapter_test` 패키지의 assertion 전용 테스트 파일) | CMDCTX-001 production 코드 변경 부재, REQ-PMC-014 builtin → adapter import 부재 동시 만족 |
| D-003 | `/planmode` alias 등록 여부 | **YES** (사용자 친화성, 기존 alias 패턴 (`quit`, `?`) 준수) | builtin.Register 의 1라인 추가 |
| D-004 | `/plan` (인자 없음) 의 default 동작 (A: status 출력 / B: usage 출력 / C: toggle) | **옵션 A** (status, REQ-PMC-010): 사용자가 가장 자주 쓸 의도 — "지금 plan mode 인가?" | 사용자 UX |
| D-005 | `/plan` 의 한국어 / 다국어 메시지 | **NO** (영문 only, CLAUDE.local.md §2.5 코드 주석 영어 정책 준수) | 후속 SPEC (i18n) 으로 분리 |
| D-006 | type assertion 실패 시 메시지 보강 (REQ-PMC-018 의 텍스트) | **현재 텍스트 유지** ("plan: this session does not support plan mode toggling") | 사용자 가시 환경에서는 발생 빈도 매우 낮음. 후속 보강 가능. |

### 다음 단계 (제안)

- [ ] plan-auditor iter 1 (선택): EARS 형식 검증, 의존 SPEC FROZEN 변경 부재 확인 (COMMAND-001 / CMDCTX-001 본문 변경 0건), REQ-AC 매트릭스 완전성, R-001 ~ R-007 mitigation 적정성 점검, Mutates=false 결정의 정합성 (deadlock 방지) 확인, 옵션 B 채택의 PlanModeSetter 위치 / 단언 위치 결정 ratify
- [ ] 사용자 ratify: AskUserQuestion 통해 D-001 ~ D-006 결정 + 진행 여부
- [ ] /moai run SPEC-GOOSE-PLANMODE-CMD-001 (사용자 승인 시)
  - Phase 2B TDD 예상 task: T-001(PlanModeSetter interface) → T-002(컴파일 단언 테스트) → T-003(planCommand skeleton + Metadata) → T-004~T-007(Execute 분기 — status/on/off/toggle/usage) → T-008(nil sctx graceful) → T-009(PlanModeSetter 미구현 graceful) → T-010(builtin.Register 등록) → T-011~T-012(dispatcher integration test) → T-013~T-014(정적 분석)
  - 예상 산출물: `internal/command/plan_mode_setter.go` (~12 LoC) + `internal/command/builtin/plan.go` (~70 LoC) + `internal/command/builtin/plan_test.go` (~120 LoC) + `internal/command/adapter/plan_mode_setter_assertion_test.go` (~10 LoC) + `internal/command/builtin/builtin.go` (+2 LoC)
  - 커버리지 ≥ 85%, race detector pass
  - 예상 LoC: 신규 ~212 LoC, 변경 +2 LoC, 총 ~214 LoC

### 주의 사항

- 본 SPEC 은 `command.SlashCommandContext` interface (COMMAND-001 PR #50) 의 본문을 **변경하지 않는다**. FROZEN.
- 본 SPEC 은 `command.Dispatcher` / `command.Command` / `command.Registry` (COMMAND-001 PR #50) 의 본체를 **변경하지 않는다**. FROZEN. 단, `builtin.Register(reg)` 의 호출 명단에 1 빌트인 + 1 alias 추가는 expansion (호출 사이트 추가, 시그니처 / contract 변경 아님).
- 본 SPEC 은 `adapter.ContextAdapter` (CMDCTX-001 PR #52) 의 본체를 **변경하지 않는다**. FROZEN. 컴파일 타임 단언은 테스트 파일 (`adapter_test` 패키지) 에서 수행 — production 코드 변경 부재.
- 본 SPEC 의 `command.PlanModeSetter` interface 는 본 SPEC 의 산출물. CMDCTX-001 의 `SetPlanMode(active bool)` 시그니처 (FROZEN) 를 미러링. CMDCTX-001 의 시그니처 변경 시 본 SPEC 의 amendment 동시 진행.
- TUI prompt prefix / statusbar 의 plan mode indicator 렌더링은 본 SPEC 범위 외 — CMDCTX-CLI-INTEG-001 의 의무 (REQ-PMC-019 결합 동작 명세).
- plan mode 활성화 시 LLM 동작 변경 (system prompt 추가, tool use 제한, 응답 형식 변경) 은 본 SPEC 범위 외 — 후속 SPEC (가칭 SPEC-GOOSE-PLANMODE-LLM-BEHAVIOR-001).
- plan mode 의 daemon RPC 노출은 본 SPEC 범위 외 — DAEMON-INTEG SPEC.
- 본 SPEC 의 implementation 은 CMDCTX-CLI-INTEG-001 implemented 와 **독립** — 본 SPEC 자체는 `internal/command/` 내부 변경만으로 완결되며, dispatcher 의 정상 호출 경로 (`dispatcher.go:134-136`) 와 ContextAdapter 의 SlashCommandContext 구현 (PR #52) 만으로 동작 검증 가능.
- 다른 동시 작성 중인 SPEC (CMDCTX-CLI-INTEG-001, CMDCTX-DAEMON-INTEG-001 등) 의 디렉토리는 건드리지 않는다 (병렬 작업 보호).
