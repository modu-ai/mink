# SPEC-GOOSE-CMDCTX-CLI-INTEG-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd, run phase 결정 시 변경 가능)
- Harness: standard (file_count<10 예상, 단일 Go domain — `internal/cli/`, security/payment 아님)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `internal/cli/app.go` 신규, 의존 SPEC 인터페이스/구조는 implemented FROZEN 또는 planned (CLI-001, batch A) 유지
- Branch base: feature/SPEC-CMDCTX-FOLLOWUPS-batch-plan (현재 브랜치, 작업 중)
- Parent SPEC: SPEC-GOOSE-CMDCTX-001 (PR #52, c018ec5 / 6593705) — `ContextAdapter` / `Options` / `WithContext` 자산을 본 SPEC 이 wiring
- Sibling SPEC (별개, 본 SPEC 의 wiring 대상): SPEC-GOOSE-CLI-001 (planned v0.2.0) — cobra root + bubbletea TUI + ask cmd 진입점

### Phase Log

- 2026-04-27 plan phase 시작
  - 부모 자산 확인:
    - `internal/command/adapter/adapter.go:107-176` — ContextAdapter / Options / New / WithContext / SetPlanMode (FROZEN, PR #52)
    - `internal/command/dispatcher.go` — Dispatcher / ProcessUserInput (FROZEN, PR #50)
    - `internal/command/context.go` — SlashCommandContext / ModelInfo / SessionSnapshot (FROZEN, PR #50)
  - wiring 대상 (CLI-001 v0.2.0, planned, FROZEN-by-reference):
    - `cmd/goose/main.go` (~15 LoC, 변경 없음)
    - `internal/cli/rootcmd.go` (PersistentPreRunE + 2 persistent flag 추가)
    - `internal/cli/tui/update.go` handleSubmit (m.sctx() stub 대체)
    - `internal/cli/tui/model.go` (Model.app *App 필드 추가)
    - `internal/cli/commands/ask.go` (dispatcher 호출 추가)
  - 신규 산출물:
    - `internal/cli/app.go` (~100 LoC) — App struct
    - `internal/cli/app_test.go` (~150 LoC) — 단위 테스트
  - 의존 SPEC (모두 planned, batch A 또는 별도):
    - SPEC-GOOSE-CMDLOOP-WIRE-001 (planned, batch A) — 옵션 γ 채택으로 client-side 에서는 nil fallback
    - SPEC-GOOSE-ALIAS-CONFIG-001 (planned, batch A) — implemented 이전 fallback (빈 맵)
    - SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 (planned, batch A) — `--strict-alias` flag 와 결합
  - 다음 단계: plan-auditor 사이클은 본 위임 범위 외. 사용자 검토 후 /moai run 분기 결정.

### 산출물 요약

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| research.md | ~360 | wiring surface analysis, CLI-001 진입점 매핑, 4 진입점 dispatcher 호출 패턴, 옵션 α/β/γ 비교, 위험 영역 식별 |
| spec.md | ~520 | EARS 22 REQ + 19 AC, App struct 설계, App.New 4단계, 의존성, Exclusions 13개 |
| progress.md | ~50 | phase log (본 파일) |

### REQ / AC 통계 (v0.1.0 기준)

- 총 REQ: 22
  - Ubiquitous: 7 (REQ-001 ~ 007)
  - Event-Driven: 5 (REQ-008 ~ 012)
  - State-Driven: 2 (REQ-013, 014)
  - Unwanted: 4 (REQ-015 ~ 018)
  - Optional: 4 (REQ-019 ~ 022)
- 총 AC: 19 (각 REQ 최소 1개 매핑, 일부 REQ 는 다중 AC)
- 커버리지 매트릭스: spec.md §5 참고

### 사용자 결정 보류 항목 (run phase 진입 전 확인 필요)

| ID | 결정 사항 | 본 SPEC 의 권장안 | 영향 |
|----|--------|----------------|------|
| D-001 | client-side LoopController 책임 분담 (옵션 α: proto 확장 RPC wrapper / β: client+daemon dispatcher 분리 / γ: client omit, daemon-side 처리) | **옵션 γ** (client-side omit, DAEMON-WIRE-001 가 mutating 명령 처리) | App.New 의 Options.LoopController 인자, 향후 SPEC 분기 |
| D-002 | App.New 인스턴스 진입 위치 (A: cmd/goose/main.go / B: rootcmd.PersistentPreRunE / C: 각 cmd lazy) | **옵션 B** (PersistentPreRunE + sync.Once) | rootcmd.go 변경 범위 |
| D-003 | ask 모드에서 dispatcher 첫 호출 적용 여부 | **적용** (REQ-CLI-021 일관성, 일반 prompt 도 ProcessProceed 분기로 daemon 전달) | ask cmd 의 분기 복잡도 |
| D-004 | SIGINT (Ctrl-C) 시 LoopController.RequestClear trigger 여부 | **NO** (stream cancel only — CLI-001 REQ-CLI-009 그대로) | UX 직관, REQ-CLIINT-017 |
| D-005 | session load (resume) 시 dispatcher 첫 호출 여부 | **NO** (resume 은 historical messages 재생, user input 아님) | Exclusions §10 #12 |
| D-006 | ALIAS-CONFIG-001 / PERMISSIVE-ALIAS-001 implemented 이전 본 SPEC implementation 진행 여부 | **권장 안 함** (선행 의존). 단, SPEC 자체 작성은 가능 (현재 상태) | implementation timing |

### 다음 단계 (제안)

- [ ] plan-auditor iter 1 (선택): EARS 형식 검증, 의존 SPEC FROZEN 변경 부재 확인, REQ-AC 매트릭스 완전성, R-001 ~ R-007 mitigation 적정성 점검, CLI-001 §6.5 stub 의 명시적 대체 의도 확인
- [ ] 사용자 ratify: AskUserQuestion 통해 D-001 ~ D-006 결정 + 진행 여부
- [ ] /moai run SPEC-GOOSE-CMDCTX-CLI-INTEG-001 (사용자 승인 시, **CLI-001 implemented 이후**)
  - Phase 2B TDD 예상 task: T-001(App struct + Config) → T-002~T-006(App.New 5단계) → T-007(rootcmd PersistentPreRunE + sync.Once) → T-008(flag 매핑) → T-009(TUI handleSubmit teatest) → T-010(ask cmd subprocess test) → T-011(plan mode indicator) → T-012(SIGINT 분리 검증) → T-013(logger debug) → T-014(CMDLOOP optional) → T-015(정적 분석)
  - 예상 산출물: `internal/cli/app.go` (~100 LoC) + `internal/cli/app_test.go` (~150 LoC) + rootcmd.go / tui/update.go / tui/model.go / commands/ask.go 변경 (~70 LoC 추가)
  - 커버리지 ≥ 85%, race detector pass

### 주의 사항

- 본 SPEC 은 `command.Dispatcher` / `command.SlashCommandContext` (COMMAND-001 PR #50) 를 **변경하지 않는다**. FROZEN.
- 본 SPEC 은 `adapter.ContextAdapter` / `adapter.Options` / `adapter.New` / `adapter.WithContext` / `adapter.SetPlanMode` (CMDCTX-001 PR #52) 를 **변경하지 않는다**. FROZEN.
- 본 SPEC 은 `LoopController` 인터페이스 (CMDCTX-001) 를 **변경하지 않는다**. FROZEN.
- 본 SPEC 은 SPEC-GOOSE-CLI-001 의 본문 (REQ-CLI-001 ~ REQ-CLI-025, AC-CLI-001 ~ AC-CLI-016, §6.1 패키지 레이아웃, bubbletea TUI 구조) 을 **변경하지 않는다**. CLI-001 §6.5 의 stub `tuiSlashContext` 가 본 SPEC 의 `app.Adapter.WithContext(ctx)` 로 자연스럽게 대체된다 (CLI-001 본문이 stub 임을 명시했으므로 충돌 없음).
- 본 SPEC 의 implementation 은 **CLI-001 implemented 이후 진행 가능** (R-001). CLI-001 이 planned 인 동안에는 본 SPEC 도 SPEC 작성 단계까지만.
- ALIAS-CONFIG-001 implemented 이전에는 본 SPEC implementation 차단 (R-002, aliasconfig 패키지 부재 시 컴파일 실패). batch A SPEC 들이 implemented 된 후 본 SPEC implementation 진행.
- 다른 동시 작성 중인 SPEC (CLI-001, DAEMON-WIRE-001 등) 의 디렉토리는 건드리지 않는다 (병렬 작업 보호).
- daemon mode wiring (cmd/goosed/, internal/daemon/) 은 본 SPEC 범위 외 — DAEMON-INTEG / DAEMON-WIRE-001 별도 SPEC.
- SIGINT (Ctrl-C) 의 시맨틱은 stream cancel only (CLI-001 REQ-CLI-009). LoopController.RequestClear trigger 는 명시적 `/clear` slash command 만 (REQ-CLIINT-017, AC-CLIINT-016).
