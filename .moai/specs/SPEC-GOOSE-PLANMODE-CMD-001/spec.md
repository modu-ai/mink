---
id: SPEC-GOOSE-PLANMODE-CMD-001
version: 0.1.0
status: completed
created_at: 2026-04-27
updated_at: 2026-05-04
completed: 2026-05-04
author: manager-spec
priority: P3
issue_number: null
phase: 2
size: 소(S)
lifecycle: spec-anchored
labels: [area/cli, area/runtime, type/feature, priority/p3-low]
---

# SPEC-GOOSE-PLANMODE-CMD-001 — `/plan` 빌트인 명령으로 ContextAdapter.SetPlanMode 트리거

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. PR #52 (SPEC-GOOSE-CMDCTX-001 v0.1.1 implemented, FROZEN) 의 `adapter.ContextAdapter.SetPlanMode(active bool)` API 를 사용자 진입점으로 노출하는 빌트인 명령 신규 SPEC. PR #50 (SPEC-GOOSE-COMMAND-001 implemented, FROZEN) 의 빌트인 명령 패턴 (`internal/command/builtin/`) 을 그대로 따름. 옵션 B 채택 (신규 narrow interface `command.PlanModeSetter` + type assertion). COMMAND-001 / CMDCTX-001 본문 변경 없음 — 본 SPEC 은 expansion. | manager-spec |

---

## 1. 개요 (Overview)

PR #52 (SPEC-GOOSE-CMDCTX-001 v0.1.1) 머지로 `adapter.ContextAdapter.SetPlanMode(active bool)` 가 implemented (FROZEN) 되었으나, **사용자가 plan mode 를 활성화/비활성화할 수 있는 진입점은 없다**. plan mode 는 현재 (a) subagent 의 `TeammateIdentity{PlanModeRequired:true}` ctx 주입 (CMDCTX-001 §6.5) 으로만 활성화 가능하며, (b) interactive TUI / 비대화 ask 모드의 사용자 본인은 토글할 수 없다.

본 SPEC 은 `/plan` (canonical) / `/planmode` (alias) 빌트인 slash command 를 추가하여 사용자가 dispatcher 경로로 `ContextAdapter.SetPlanMode` 를 호출할 수 있게 한다.

본 SPEC 수락 시점에서:

- `internal/command/builtin/plan.go` (신규) 가 존재하고 `planCommand` struct 가 `command.Command` 인터페이스를 구현한다.
- `command.PlanModeSetter` 인터페이스 (신규) 가 `internal/command/` 패키지에 정의되고, `*adapter.ContextAdapter` 가 자동으로 이를 만족한다 (구조적 타이핑).
- `internal/command/builtin/builtin.go` 의 `Register(reg)` 에 `mustRegister(&planCommand{})` 와 `reg.RegisterAlias("planmode", "plan")` 두 라인이 추가된다.
- `/plan on` / `/plan off` / `/plan toggle` / `/plan status` / `/plan` (인자 없음, status alias) 입력에 대해 dispatcher 가 plan command 를 실행하고 `sctx.(command.PlanModeSetter).SetPlanMode(...)` 를 호출한다.
- `/plan` 의 `Metadata.Mutates = false` 으로 설정되어 plan-mode-active 상태에서도 dispatcher 의 차단 분기 (REQ-CMD-011) 에 걸리지 않는다 (deadlock 방지).
- COMMAND-001 / CMDCTX-001 의 본문은 변경되지 않는다.

본 SPEC 은 **plan mode 토글 진입점 추가** 만 다룬다. TUI 의 prompt prefix 변경 (CMDCTX-CLI-INTEG-001 범위), plan mode 상태의 RPC 노출 (DAEMON-INTEG 범위), plan mode 가 활성화될 때의 LLM 동작 변경 (별도 SPEC) 은 모두 범위 외.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **CMDCTX-001 의 SetPlanMode dead API 위험**: PR #52 머지로 `SetPlanMode(active bool)` 가 implemented 되었지만 호출 사이트가 0개 — 사용자 진입점 없이 정의된 mutator 는 지속적으로 dead code 검사에 걸리거나 규모 확장 시 의도가 흐려진다.
- **TRUST 5 Trackable 결손 (사용자 진입점)**: `PlanModeActive()` 가 dispatcher 의 차단 분기에 사용되지만, 사용자 입장에서 그 flag 를 토글하는 명시적 명령이 없으면 invariant 가 외부에서 검증 불가능.
- **빌트인 명령 패턴의 자연스러운 확장**: COMMAND-001 의 `/clear`, `/compact`, `/model` 등 7 builtin 은 모두 sctx 의 OnXxx 메서드를 호출. plan mode 토글도 동일 패턴으로 노출하는 것이 일관성 측면에서 자연스럽다.
- **CMDCTX-CLI-INTEG-001 와의 결합**: CLI-INTEG SPEC 의 §Optional REQ-CLIINT-013 (plan mode indicator 표시) 가 본 SPEC 의 trigger 와 결합하여 사용자에게 visible 한 plan mode UX 제공.

### 2.2 상속 자산 (FROZEN by reference)

- **SPEC-GOOSE-COMMAND-001** (implemented, PR #50, FROZEN, v1.0.0): `command.Command` 인터페이스, `command.Metadata` (Mutates 필드 포함), `command.Args`, `command.Result`, `command.Lister`, `command.Registry.Register`, `command.Registry.RegisterAlias`, `command.Source` (SourceBuiltin), `command.Dispatcher.ProcessUserInput`, `command.SctxContextKey`, `command.SlashCommandContext` (6 메서드), `builtin.extractSctx` 헬퍼, `builtin.Register` 함수. 본 SPEC 은 이들을 **호출만 한다**. 변경하지 않는다. COMMAND-001 spec.md 본문 (REQ-CMD-001 ~ REQ-CMD-019, AC-CMD-001 ~ AC-CMD-013) 변경 금지.
- **SPEC-GOOSE-CMDCTX-001** (implemented, PR #52, v0.1.1, FROZEN): `adapter.ContextAdapter.SetPlanMode(active bool)`, `adapter.ContextAdapter.PlanModeActive()`, `adapter.ContextAdapter` 의 `SlashCommandContext` 구현 단언 (`var _ command.SlashCommandContext = (*ContextAdapter)(nil)`). 본 SPEC 은 SetPlanMode 를 호출만 한다. CMDCTX-001 spec.md 본문 변경 금지. 단, `*ContextAdapter` 가 본 SPEC 의 신규 `command.PlanModeSetter` 인터페이스를 자동으로 만족함은 Go 의 구조적 타이핑 결과 — CMDCTX-001 의 `SetPlanMode` 시그니처 (FROZEN) 를 본 SPEC 의 인터페이스가 미러링.
- **SPEC-GOOSE-CMDCTX-CLI-INTEG-001** (planned, batch B): TUI 의 prompt prefix / statusbar 에 plan mode indicator 표시. 본 SPEC 의 `/plan on` 후 PlanModeActive() 가 true 를 반환하면 CLI-INTEG 의 indicator 가 활성화. 본 SPEC 은 trigger 만 제공, indicator 자체는 CLI-INTEG 범위.

### 2.3 범위 경계 (한 줄)

본 SPEC 은 **`/plan` 빌트인 명령 추가 + `command.PlanModeSetter` narrow interface 추가** 만 한다. plan mode 의 LLM 동작 변경, TUI indicator 렌더링, daemon RPC 노출, plan mode 의 다른 trigger 경로 (signal, env var, config file) 는 모두 범위 외.

---

## 3. 목표 (Goals)

### 3.1 IN SCOPE

| ID | 항목 | 목적 |
|----|------|-----|
| G-1 | `internal/command/builtin/plan.go` 신규 — `planCommand` struct 정의 | `/plan` 빌트인 명령 구현 |
| G-2 | `command.PlanModeSetter` 인터페이스 추가 | sctx 가 SetPlanMode 능력을 노출하는 narrow interface |
| G-3 | `*adapter.ContextAdapter` 가 `command.PlanModeSetter` 를 자동 만족 (구조적 타이핑) | 추가 코드 0, 컴파일 타임 단언으로 검증 |
| G-4 | `/plan` / `/plan on` / `/plan off` / `/plan toggle` / `/plan status` 인자 분기 | 사용자 모델 |
| G-5 | `Metadata.Mutates = false` 설정 | plan-mode-active 상태에서도 토글 가능 (deadlock 방지) |
| G-6 | dispatcher 자동 발견 (builtin.Register 에 1라인 추가) | 빌트인 명령의 standard 등록 패턴 |
| G-7 | `/planmode` alias 등록 | 사용자 친화성 |
| G-8 | sctx == nil 또는 sctx 가 PlanModeSetter 미구현 시 graceful LocalReply | nil-safe |
| G-9 | 잘못된 인자 (`/plan foo`, `/plan on extra`) → usage LocalReply | 사용자 가이드 |
| G-10 | unit test (plan_test.go) + dispatcher integration test | TRUST 5 Tested, ≥ 85% coverage |

### 3.2 OUT OF SCOPE (Exclusions §10 에서 상세화)

- TUI 의 prompt prefix / statusbar plan mode indicator 렌더링 (CMDCTX-CLI-INTEG-001 범위)
- plan mode 활성화 시 LLM 동작 변경 (별도 SPEC, 가칭 SPEC-GOOSE-PLANMODE-LLM-BEHAVIOR-001)
- plan mode 상태의 daemon RPC 노출 (`AgentService/PlanMode` 가칭, DAEMON-INTEG 범위)
- plan mode 의 다른 trigger 경로 (signal, env var, config file)
- `command.SlashCommandContext` 인터페이스 자체 확장 (옵션 A 거부, COMMAND-001 FROZEN)
- `*adapter.ContextAdapter` 의 직접 의존성 주입 (옵션 C 거부, builtin 패턴 비대칭)
- COMMAND-001 / CMDCTX-001 본문 변경
- plan mode 의 영구 저장 (다음 세션 재시작 시 복원) — 본 SPEC 의 default 는 process-lifetime
- subagent 의 `TeammateIdentity{PlanModeRequired:true}` ctx 주입 경로 (CMDCTX-001 §6.5 가 이미 처리)

---

## 4. 요구사항 (Requirements, EARS)

### 4.1 Ubiquitous Requirements (always active)

**REQ-PMC-001**: 시스템은 `internal/command/` 패키지에 `command.PlanModeSetter` 인터페이스를 정의하여야 한다 (`shall define command.PlanModeSetter interface`). 인터페이스 시그니처: `SetPlanMode(active bool)`. 본 인터페이스는 단일 메서드 narrow interface 로, plan mode 토글 능력을 노출하는 sctx 구현체가 만족할 수 있다.

**REQ-PMC-002**: 시스템은 `*adapter.ContextAdapter` 가 `command.PlanModeSetter` 인터페이스를 자동으로 만족함을 컴파일 타임에 단언하여야 한다 (`shall assert *adapter.ContextAdapter satisfies PlanModeSetter at compile time`). 단언 위치: `internal/command/adapter/adapter.go` 또는 본 SPEC 의 plan.go (둘 중 한 곳, §6.4 결정).

**REQ-PMC-003**: 시스템은 `internal/command/builtin/plan.go` 에 `planCommand` struct 를 정의하여 `command.Command` 인터페이스 (Name, Metadata, Execute) 를 구현하여야 한다 (`shall implement Command interface in builtin/plan.go`).

**REQ-PMC-004**: 시스템은 `planCommand.Metadata()` 에서 `Description` 은 plan mode 동작을 설명하는 1줄 영문 텍스트, `ArgumentHint` 는 `[on|off|toggle|status]` 형식, `Mutates` 는 false, `Source` 는 `command.SourceBuiltin` 을 반환하여야 한다 (`shall return Mutates=false metadata`). Mutates=false 의 근거: REQ-CMD-011 (dispatcher 의 plan-mode 차단) 에서 plan command 자체가 차단되면 사용자가 plan mode 에서 빠져나올 수 없는 deadlock 발생. plan mode 토글은 메타-제어로 변형 명령이 아님.

**REQ-PMC-005**: 시스템은 `planCommand.Name()` 에서 lowercase canonical 문자열 `"plan"` 을 반환하여야 한다 (`shall return canonical name "plan"`).

**REQ-PMC-006**: 시스템은 `internal/command/builtin/builtin.go` 의 `Register(reg)` 함수에 `mustRegister(&planCommand{})` 한 라인과 `reg.RegisterAlias("planmode", "plan")` 한 라인을 추가하여야 한다 (`shall register planCommand and alias planmode in builtin.Register`). 이는 `Register` 의 시그니처 / 동작 / contract 변경이 아닌 등록 명단의 확장이다.

### 4.2 Event-Driven Requirements (when X then Y)

**REQ-PMC-007**: WHEN 사용자가 `/plan on` 또는 `/plan ON` (대소문자 무관) 을 입력하면 THEN 시스템은 (a) `extractSctx(ctx)` 로 sctx 획득, (b) sctx 가 nil 이 아니면 `sctx.(command.PlanModeSetter)` type assertion 시도, (c) assertion 성공 시 `SetPlanMode(true)` 호출, (d) `Result{Kind: ResultLocalReply, Text: "plan mode: on"}` 반환하여야 한다 (`shall call SetPlanMode(true) on /plan on`). 이미 plan mode 가 active 인 경우 텍스트는 `"plan mode: on (already active)"`.

**REQ-PMC-008**: WHEN 사용자가 `/plan off` 또는 `/plan OFF` 를 입력하면 THEN 시스템은 `SetPlanMode(false)` 호출 후 `Result{Kind: ResultLocalReply, Text: "plan mode: off"}` 반환하여야 한다 (`shall call SetPlanMode(false) on /plan off`). 이미 inactive 인 경우 텍스트는 `"plan mode: off (already inactive)"`.

**REQ-PMC-009**: WHEN 사용자가 `/plan toggle` 을 입력하면 THEN 시스템은 (a) `sctx.PlanModeActive()` 호출하여 현재 상태 획득, (b) `sctx.(command.PlanModeSetter).SetPlanMode(!current)` 호출, (c) 새 상태에 따라 `"plan mode: on"` 또는 `"plan mode: off"` LocalReply 반환하여야 한다 (`shall toggle plan mode on /plan toggle`).

**REQ-PMC-010**: WHEN 사용자가 `/plan status` 또는 `/plan` (인자 없음) 을 입력하면 THEN 시스템은 (a) `sctx.PlanModeActive()` 호출, (b) `Result{Kind: ResultLocalReply, Text: "plan mode: " + ("on"|"off")}` 반환하여야 한다 (`shall return current state on /plan status or /plan with no args`). 본 동작은 read-only — SetPlanMode 미호출.

**REQ-PMC-011**: WHEN 사용자가 잘못된 첫 인자 (`/plan foo`, `/plan unknown`) 또는 인자 2개 이상 (`/plan on extra`) 을 입력하면 THEN 시스템은 `Result{Kind: ResultLocalReply, Text: "usage: /plan [on|off|toggle|status]"}` 반환하여야 한다 (`shall return usage on invalid args`). SetPlanMode 미호출.

### 4.3 State-Driven Requirements (while X)

**REQ-PMC-012**: WHILE plan mode 가 active (`sctx.PlanModeActive() == true`) 인 동안 THEN 시스템은 `/plan` 자체를 dispatcher 의 plan-mode 차단 분기 (REQ-CMD-011) 에 걸리지 않도록 처리하여야 한다 (`shall not block /plan command in plan mode`). 이는 `Metadata.Mutates = false` (REQ-PMC-004) 로 자동 보장됨 — `dispatcher.go:114` 의 `if cmd.Metadata().Mutates && sctx.PlanModeActive()` 조건이 plan command 에 대해 false.

### 4.4 Unwanted Behavior (shall not / shall reject)

**REQ-PMC-013**: 시스템은 `command.SlashCommandContext` 인터페이스 자체에 `SetPlanMode` 메서드를 추가하지 않아야 한다 (`shall not modify SlashCommandContext interface`). 옵션 A 거부 — COMMAND-001 FROZEN. 모든 sctx 구현체 (mockSctx 포함) 가 강제로 SetPlanMode 를 구현해야 하는 부담 회피.

**REQ-PMC-014**: 시스템은 `planCommand` struct 가 `*adapter.ContextAdapter` 를 직접 의존하지 않아야 한다 (`shall not depend on *adapter.ContextAdapter directly`). 옵션 C 거부 — builtin 패턴 비대칭 + import cycle 위험. 모든 sctx 접근은 `extractSctx(ctx)` + type assertion 패턴.

**REQ-PMC-015**: 시스템은 sctx == nil 또는 sctx 가 `command.PlanModeSetter` 미구현 시 panic 하지 않아야 한다 (`shall not panic on nil sctx or missing PlanModeSetter`). graceful LocalReply 반환 (REQ-PMC-016, REQ-PMC-017).

**REQ-PMC-016**: 시스템은 `/plan` 의 Execute 가 internal error 를 반환하지 않아야 한다 (`shall not return error from Execute`). 모든 분기 (정상, 잘못된 인자, sctx nil, PlanModeSetter 미구현) 는 LocalReply 로 처리. 단, 향후 sctx.PlanModeActive() 가 panic 발생 시 (정상 ContextAdapter 에서는 발생 안 함) 그 panic 은 dispatcher 의 일반 panic 처리 경로로 전파됨 (recover 안 함).

### 4.5 Optional Requirements (where possible)

**REQ-PMC-017**: WHERE sctx 가 nil 이면 시스템은 `Result{Kind: ResultLocalReply, Text: "plan: context unavailable"}` 반환하여야 한다 (`may return graceful message on nil sctx`). 이 분기는 dispatcher 의 정상 호출 경로 (`dispatcher.go:134-136`) 에서는 발생하지 않음 — dispatcher 가 항상 sctx 를 ctx 에 주입. 외부 직접 호출 또는 테스트 시에만 발생.

**REQ-PMC-018**: WHERE sctx 가 nil 이 아니지만 `command.PlanModeSetter` 미구현이면 시스템은 `Result{Kind: ResultLocalReply, Text: "plan: this session does not support plan mode toggling"}` 반환하여야 한다 (`may return graceful message on missing PlanModeSetter`). 이 분기는 ContextAdapter 가 정상 wiring 된 환경에서는 발생하지 않음 — 테스트 mock 또는 future read-only adapter 시에만 발생.

**REQ-PMC-019**: WHERE TUI / CLI 가 CMDCTX-CLI-INTEG-001 의 plan mode indicator 를 구현했으면, 본 SPEC 의 `/plan on` 호출 후 indicator 가 자동으로 활성화되어야 한다 (`may activate indicator after /plan on when CLI-INTEG implemented`). 본 SPEC 은 SetPlanMode trigger 만 제공. indicator 렌더링은 CLI-INTEG 의무. 본 REQ 는 결합 동작 명세 — 본 SPEC 자체의 검증 의무는 없음 (CLI-INTEG 의 AC-CLIINT-012 가 검증).

**REQ-PMC-020**: WHERE structured logger (zap) 가 ContextAdapter 에 주입되어 있으면, SetPlanMode 호출 시 ContextAdapter 가 debug level 로 로그 기록할 수 있다 (`may log SetPlanMode invocation when logger is provided`). 본 SPEC 은 ContextAdapter 의 logger 동작을 강제하지 않음 — CMDCTX-001 의 logger contract 위임. 본 REQ 는 결합 동작 명세.

---

## 5. REQ ↔ AC 매트릭스

| REQ | AC | 관련 함수 / 위치 |
|-----|----|----------|
| REQ-PMC-001 | AC-PMC-001 (인터페이스 컴파일 단언) | command/context.go 또는 command/plan_mode_setter.go |
| REQ-PMC-002 | AC-PMC-002 (var _ assertion) | command/adapter/adapter.go 또는 builtin/plan.go |
| REQ-PMC-003 | AC-PMC-003 (planCommand 컴파일) | builtin/plan.go |
| REQ-PMC-004 | AC-PMC-004 (Metadata 필드값) | planCommand.Metadata() |
| REQ-PMC-005 | AC-PMC-005 (Name() == "plan") | planCommand.Name() |
| REQ-PMC-006 | AC-PMC-013 (Register 추가 라인) | builtin/builtin.go Register |
| REQ-PMC-007 | AC-PMC-006 (/plan on) | planCommand.Execute |
| REQ-PMC-008 | AC-PMC-007 (/plan off) | planCommand.Execute |
| REQ-PMC-009 | AC-PMC-008 (/plan toggle) | planCommand.Execute |
| REQ-PMC-010 | AC-PMC-009 (/plan status, /plan) | planCommand.Execute |
| REQ-PMC-011 | AC-PMC-010 (잘못된 인자) | planCommand.Execute |
| REQ-PMC-012 | AC-PMC-011 (Mutates=false 차단 부재) | dispatcher integration test |
| REQ-PMC-013 | AC-PMC-014 (정적 분석 — SlashCommandContext 본문 변경 부재) | command/context.go |
| REQ-PMC-014 | AC-PMC-015 (정적 분석 — adapter import 부재) | builtin/plan.go imports |
| REQ-PMC-015 | AC-PMC-012 (nil sctx graceful) | planCommand.Execute |
| REQ-PMC-016 | AC-PMC-016 (Execute error 부재) | planCommand.Execute |
| REQ-PMC-017 | AC-PMC-012 | planCommand.Execute |
| REQ-PMC-018 | AC-PMC-017 (PlanModeSetter 미구현 graceful) | planCommand.Execute |
| REQ-PMC-019 | (CLI-INTEG-001 AC-CLIINT-012 위임) | TUI |
| REQ-PMC-020 | (CMDCTX-001 logger contract 위임) | ContextAdapter |

총 20 REQ / 17 AC. 모든 REQ 는 최소 1개 AC 또는 위임된 검증 경로를 가짐. REQ-PMC-019, REQ-PMC-020 은 결합 동작 명세 — 검증 의무 위임.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃 (FROZEN 위에 add-on)

COMMAND-001 §6.5 의 패키지 레이아웃을 read-only 로 가정. 본 SPEC 이 추가/변경:

```
internal/command/
├── command.go                  # FROZEN (변경 없음)
├── context.go                  # ⬅︎ 변경 또는 ⬇︎ 신규 파일 (PlanModeSetter 정의)
├── plan_mode_setter.go         # ⬅︎ (대안) 신규 ~10 LoC — PlanModeSetter 단독 파일
├── dispatcher.go               # FROZEN (변경 없음)
├── errors.go                   # FROZEN (변경 없음)
├── ...
├── adapter/
│   └── adapter.go              # ⬅︎ 변경 (옵션, 1라인) — var _ command.PlanModeSetter 단언
└── builtin/
    ├── builtin.go              # ⬅︎ 변경 (+2 LoC) — mustRegister + RegisterAlias
    ├── plan.go                 # ⬅︎ 신규 (~70 LoC, 본 SPEC)
    └── plan_test.go            # ⬅︎ 신규 (~120 LoC, 본 SPEC)
```

PlanModeSetter 위치 결정: §6.4 참조.

### 6.2 PlanModeSetter 인터페이스 정의

```go
// internal/command/plan_mode_setter.go (신규 파일, ~12 LoC)
// SPEC: SPEC-GOOSE-PLANMODE-CMD-001
package command

// PlanModeSetter is an OPTIONAL capability that a SlashCommandContext
// implementation MAY provide. The /plan builtin uses a type assertion to
// detect support and toggles plan mode when supported.
//
// This interface is intentionally separate from SlashCommandContext to
// avoid forcing every implementation (mocks, fakes, alternative adapters)
// to implement SetPlanMode. *adapter.ContextAdapter satisfies this
// interface implicitly via its existing SetPlanMode(bool) method.
//
// @MX:NOTE: [AUTO] Narrow interface for the /plan builtin's type assertion.
// @MX:SPEC: SPEC-GOOSE-PLANMODE-CMD-001 REQ-PMC-001
type PlanModeSetter interface {
    SetPlanMode(active bool)
}
```

위치 결정 (§6.4): 별도 `plan_mode_setter.go` 파일이 권장. context.go 에 추가하면 COMMAND-001 의 implemented 파일 변경 인상 — 별도 파일은 본 SPEC 의 산출물이 명확히 분리됨.

### 6.3 planCommand 구현

```go
// internal/command/builtin/plan.go (신규)
// SPEC: SPEC-GOOSE-PLANMODE-CMD-001
package builtin

import (
    "context"
    "fmt"
    "strings"

    "github.com/modu-ai/goose/internal/command"
)

// Compile-time assertion: *adapter.ContextAdapter (CMDCTX-001) satisfies
// command.PlanModeSetter via its existing SetPlanMode(bool) method.
// We do NOT import adapter package here (REQ-PMC-014); the assertion lives
// in adapter.go or a dedicated assertion file.

// planCommand implements /plan [on|off|toggle|status].
//
// @MX:NOTE: [AUTO] The /plan builtin toggles ContextAdapter.SetPlanMode via
// type assertion on command.PlanModeSetter. Mutates=false: the /plan command
// itself must remain reachable while plan mode is active to allow the user
// to disable plan mode.
// @MX:SPEC: SPEC-GOOSE-PLANMODE-CMD-001 REQ-PMC-003, REQ-PMC-004, REQ-PMC-012
type planCommand struct{}

func (p *planCommand) Name() string { return "plan" }

func (p *planCommand) Metadata() command.Metadata {
    return command.Metadata{
        Description:  "Toggle plan mode (read-only mode for inspection).",
        ArgumentHint: "[on|off|toggle|status]",
        Mutates:      false, // REQ-PMC-004: must NOT be Mutates=true (deadlock risk).
        Source:       command.SourceBuiltin,
    }
}

// Execute parses the first positional argument and toggles plan mode via
// the PlanModeSetter type assertion on the injected SlashCommandContext.
// REQ-PMC-007 ~ REQ-PMC-011, REQ-PMC-015 ~ REQ-PMC-018.
func (p *planCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
    sctx := extractSctx(ctx)
    if sctx == nil {
        // REQ-PMC-017: nil sctx graceful.
        return command.Result{
            Kind: command.ResultLocalReply,
            Text: "plan: context unavailable",
        }, nil
    }

    sub := ""
    if len(args.Positional) > 0 {
        sub = strings.ToLower(args.Positional[0])
    }

    // REQ-PMC-011: more than one positional argument is invalid.
    if len(args.Positional) > 1 {
        return usageReply(), nil
    }

    switch sub {
    case "", "status":
        // REQ-PMC-010: read-only status query.
        return statusReply(sctx.PlanModeActive()), nil

    case "on":
        return p.applySet(sctx, true)

    case "off":
        return p.applySet(sctx, false)

    case "toggle":
        return p.applySet(sctx, !sctx.PlanModeActive())

    default:
        // REQ-PMC-011: unknown sub-command.
        return usageReply(), nil
    }
}

// applySet performs the type-asserted SetPlanMode call.
// Returns a graceful LocalReply when the sctx does not implement PlanModeSetter.
// REQ-PMC-018.
func (p *planCommand) applySet(sctx command.SlashCommandContext, target bool) (command.Result, error) {
    setter, ok := sctx.(command.PlanModeSetter)
    if !ok {
        return command.Result{
            Kind: command.ResultLocalReply,
            Text: "plan: this session does not support plan mode toggling",
        }, nil
    }

    current := sctx.PlanModeActive()
    setter.SetPlanMode(target)

    var text string
    switch {
    case target && current:
        text = "plan mode: on (already active)"
    case target && !current:
        text = "plan mode: on"
    case !target && !current:
        text = "plan mode: off (already inactive)"
    case !target && current:
        text = "plan mode: off"
    }

    return command.Result{
        Kind: command.ResultLocalReply,
        Text: text,
    }, nil
}

func statusReply(active bool) command.Result {
    state := "off"
    if active {
        state = "on"
    }
    return command.Result{
        Kind: command.ResultLocalReply,
        Text: fmt.Sprintf("plan mode: %s", state),
    }
}

func usageReply() command.Result {
    return command.Result{
        Kind: command.ResultLocalReply,
        Text: "usage: /plan [on|off|toggle|status]",
    }
}
```

### 6.4 Compile-time assertion 위치 결정

옵션:
- A) `internal/command/adapter/adapter.go` 에 추가:
  ```go
  var _ command.PlanModeSetter = (*ContextAdapter)(nil)
  ```
  단점: CMDCTX-001 의 implemented 파일 변경 — 그 자체로 implemented 파일에 한 줄 추가는 amendment 인지 expansion 인지 모호.

- B) `internal/command/builtin/plan.go` 에 추가:
  ```go
  // (이 파일은 adapter package import 가 위 §6.3 import 명단에 없음)
  ```
  단점: builtin → adapter import → REQ-PMC-014 (adapter 직접 의존 금지) 위반.

- C) `internal/command/adapter/plan_mode_setter_assertion_test.go` (테스트 파일) 에 추가:
  ```go
  package adapter_test

  import (
      "github.com/modu-ai/goose/internal/command"
      "github.com/modu-ai/goose/internal/command/adapter"
  )

  // Compile-time assertion: *ContextAdapter satisfies command.PlanModeSetter.
  // REQ-PMC-002.
  var _ command.PlanModeSetter = (*adapter.ContextAdapter)(nil)
  ```
  장점: 테스트 패키지에 두면 production 코드 변경 없음. 단언만 추가. CMDCTX-001 본문 / production 코드 변경 부재. 옵션 A/B 의 단점 모두 회피.

**채택: 옵션 C** — `internal/command/adapter/plan_mode_setter_assertion_test.go` (또는 본 SPEC 의 테스트 파일 중 하나) 에 컴파일 타임 단언 추가. CMDCTX-001 의 production 코드 변경 부재. 본 SPEC 의 테스트 산출물이 단언 소유.

### 6.5 builtin.Register 변경

```go
// internal/command/builtin/builtin.go (변경된 부분만, +2 LoC)
func Register(reg registrar) {
    mustRegister := func(c command.Command) {
        if err := reg.Register(c, command.SourceBuiltin); err != nil {
            panic("builtin registration failed: " + err.Error())
        }
    }

    mustRegister(newHelpCommand(reg))
    mustRegister(&clearCommand{})
    mustRegister(&exitCommand{})
    mustRegister(&modelCommand{})
    mustRegister(&compactCommand{})
    mustRegister(&statusCommand{})
    mustRegister(&versionCommand{})
    mustRegister(&planCommand{})              // ⬅︎ 신규 (REQ-PMC-006)

    reg.RegisterAlias("quit", "exit")
    reg.RegisterAlias("?", "help")
    reg.RegisterAlias("planmode", "plan")     // ⬅︎ 신규 (REQ-PMC-006)
}
```

### 6.6 의존성 — 의존 SPEC 진척도별 fallback

| 의존 SPEC | 시점 | 본 SPEC 동작 |
|---------|-----|-----------|
| COMMAND-001 implemented (PR #50) | 항상 | 본 SPEC 의 빌드 가능. extractSctx, Command interface, Registry 호출. |
| CMDCTX-001 implemented (PR #52) | 항상 | `*adapter.ContextAdapter` 가 PlanModeSetter 자동 만족. 본 SPEC 의 type assertion 정상. |
| CMDCTX-CLI-INTEG-001 missing | runtime | 본 SPEC 의 plan command 는 정상 동작. TUI indicator 만 미표시 — 사용자는 LocalReply 텍스트로 상태 확인. |
| CMDCTX-CLI-INTEG-001 implemented | runtime | 본 SPEC 의 SetPlanMode 호출 후 indicator 자동 활성화. REQ-PMC-019 결합 동작. |

### 6.7 race-clean 보장

- planCommand: stateless struct (필드 0). race 없음.
- Execute: ctx 기반 sctx 추출 → ContextAdapter 의 SetPlanMode 는 atomic.Bool 기반 (CMDCTX-001 §6.6 보장). race-clean.
- 동시 다중 plan command 호출 (예: 두 사용자 입력 또는 dispatcher 의 동시 호출) → ContextAdapter 의 atomic.Bool 이 race-free.

### 6.8 graceful degradation

- sctx == nil → "plan: context unavailable" (REQ-PMC-017)
- sctx 가 PlanModeSetter 미구현 → "plan: this session does not support plan mode toggling" (REQ-PMC-018)
- 잘못된 인자 → "usage: /plan [on|off|toggle|status]" (REQ-PMC-011)
- 인자 2개 이상 → 위와 동일

### 6.9 TDD 진입 순서 (RED → GREEN → REFACTOR)

| 순서 | 작업 | 검증 AC |
|------|------|------|
| T-001 | `internal/command/plan_mode_setter.go` 신규 — PlanModeSetter 인터페이스 정의 (compile only) | AC-PMC-001 |
| T-002 | `internal/command/adapter/plan_mode_setter_assertion_test.go` 신규 — `var _ command.PlanModeSetter = (*adapter.ContextAdapter)(nil)` | AC-PMC-002 |
| T-003 | `internal/command/builtin/plan.go` skeleton — Name() + Metadata() (Mutates=false) | AC-PMC-003, AC-PMC-004, AC-PMC-005 |
| T-004 | Execute — 인자 없음 / "status" → status reply | AC-PMC-009 |
| T-005 | Execute — "on" / "off" 분기 | AC-PMC-006, AC-PMC-007 |
| T-006 | Execute — "toggle" 분기 | AC-PMC-008 |
| T-007 | Execute — 잘못된 인자 / 인자 2개 이상 → usage reply | AC-PMC-010 |
| T-008 | Execute — sctx == nil graceful | AC-PMC-012 |
| T-009 | Execute — sctx 가 PlanModeSetter 미구현 시 graceful (mockSctx 사용) | AC-PMC-017 |
| T-010 | builtin.Register 에 planCommand + planmode alias 추가 | AC-PMC-013 |
| T-011 | dispatcher integration test — `/plan on` → ContextAdapter.PlanModeActive() == true | (cross-SPEC, manager-tdd 검증) |
| T-012 | dispatcher integration test — `/plan` (Mutates=false) plan-mode-active 상태에서도 통과 | AC-PMC-011 |
| T-013 | 정적 분석 — `internal/command/builtin/plan.go` 가 adapter 패키지 import 안 함 | AC-PMC-015 |
| T-014 | 정적 분석 — `internal/command/context.go` 에 SetPlanMode 메서드 추가 부재 | AC-PMC-014 |
| T-015 | 코드 커버리지 ≥ 85%, race detector pass | TRUST 5 |

### 6.10 TRUST 5 매핑

| 차원 | 본 SPEC 적용 |
|------|-----------|
| Tested | 17 AC, ≥ 85% coverage (planCommand.Execute), unit test (plan_test.go), dispatcher integration test, race detector pass |
| Readable | godoc on planCommand, statusReply, usageReply, applySet, English code comments per language.yaml |
| Unified | gofmt + golangci-lint clean, COMMAND-001 의 builtin 패턴 준수 (clearCommand 등 미러링) |
| Secured | nil sctx graceful, type assertion graceful, 인자 검증 (2개 이상 거부), strings.ToLower 정규화 |
| Trackable | conventional commits, SPEC-GOOSE-PLANMODE-CMD-001 trailer, MX:NOTE on planCommand + PlanModeSetter |

### 6.11 의존성 결정 (라이브러리)

기존 자산만 사용. 신규 외부 의존성 없음.
- `context` (stdlib) — Execute 시그니처
- `fmt` (stdlib) — LocalReply 포매팅
- `strings` (stdlib) — 인자 정규화
- `github.com/modu-ai/goose/internal/command` (COMMAND-001) — Command, Metadata, Args, Result
- `github.com/modu-ai/goose/internal/command/adapter` (CMDCTX-001) — **테스트 파일에서만** import (`adapter_test` 패키지의 컴파일 단언). production 코드 (plan.go) 는 adapter import 없음 (REQ-PMC-014).
- `github.com/stretchr/testify` (테스트 only) — assertions

---

## 7. 의존성 (Dependencies)

| SPEC | status (요청 시점) | 본 SPEC 의 사용 방식 |
|------|--------|----------------|
| SPEC-GOOSE-COMMAND-001 | implemented (PR #50, FROZEN, v1.0.0) | `command.Command`, `command.Metadata`, `command.Args`, `command.Result`, `command.Registry`, `command.Source` (SourceBuiltin), `command.Dispatcher.ProcessUserInput`, `command.SctxContextKey`, `command.SlashCommandContext`, `builtin.extractSctx`, `builtin.Register` 자산 그대로 사용. **변경 금지**. 본 SPEC 의 `mustRegister(&planCommand{})` + `RegisterAlias("planmode", "plan")` 추가는 등록 명단의 expansion 으로 분류 — Register 함수의 시그니처 / contract 변경 아님. |
| SPEC-GOOSE-CMDCTX-001 | implemented (PR #52, v0.1.1, FROZEN) | `adapter.ContextAdapter.SetPlanMode(active bool)` 시그니처 + `adapter.ContextAdapter` 의 SlashCommandContext 구현. 본 SPEC 의 `command.PlanModeSetter` 인터페이스가 그 시그니처를 미러링하여 *ContextAdapter 가 자동 만족. **변경 금지**. CMDCTX-001 의 production 코드 (adapter.go) 변경 없음. 컴파일 타임 단언은 본 SPEC 의 테스트 파일 (adapter_test 패키지) 에서 수행. |
| SPEC-GOOSE-CMDCTX-CLI-INTEG-001 | planned (batch B) | TUI prompt prefix / statusbar 의 plan mode indicator. 본 SPEC 의 `/plan on` 호출 후 PlanModeActive() == true → CLI-INTEG 의 indicator 활성화. 본 SPEC 은 trigger 만 제공. CLI-INTEG 의 AC-CLIINT-012 가 indicator 검증. |
| SPEC-GOOSE-DAEMON-INTEG-001 | planned (별도) | daemon 측 ContextAdapter / Dispatcher wiring + plan mode RPC 노출. 본 SPEC 범위 외. |

---

## 8. 정합성 / 비기능 요구사항

| 항목 | 기준 |
|------|------|
| 코드 주석 | 영어 (CLAUDE.local.md §2.5) |
| SPEC 본문 | 한국어 |
| 테스트 커버리지 | ≥ 85% (LSP quality gate, run phase). main.go 제외. |
| race detector | `go test -race -count=10 ./internal/command/builtin/...` PASS |
| golangci-lint | 0 issues |
| gofmt | clean |
| 정적 분석 (REQ-PMC-014 검증) | `grep -E '"github.com/modu-ai/goose/internal/command/adapter"' internal/command/builtin/plan.go` 결과 0건 |
| 정적 분석 (REQ-PMC-013 검증) | `grep -E 'SetPlanMode' internal/command/context.go` 결과 0건 (SlashCommandContext interface 본문에 SetPlanMode 미추가) |
| LSP 진단 | 0 errors / 0 type errors / 0 lint errors (run phase) |

---

## 9. Risks (위험 영역)

| ID | 위험 | 영향 | 완화 |
|----|------|------|------|
| R-001 | PlanModeSetter 인터페이스의 시그니처 (`SetPlanMode(active bool)`) 가 미래에 변경 (예: `SetPlanMode(active bool, reason string)`) 되어 본 SPEC 의 호출 사이트가 깨질 가능성 | plan command Execute 컴파일 실패 | CMDCTX-001 의 SetPlanMode (FROZEN, v0.1.1) 시그니처가 본 인터페이스의 source-of-truth. 시그니처 변경은 CMDCTX-001 의 amendment 필요 — 그 시점에 본 SPEC 의 0.2.0 amendment 도 동시 진행. 본 SPEC §Exclusions §10 #10 에서 명시. |
| R-002 | type assertion (`sctx.(command.PlanModeSetter)`) 실패 시 사용자 가이드 부족 | "plan: this session does not support plan mode toggling" 메시지를 보고 사용자가 다음 행동 모름 | ContextAdapter 가 정상 wiring 된 환경 (cli.App.New 호출 후) 에서는 항상 PlanModeSetter 만족 — assertion 실패는 테스트 외 발생 빈도 매우 낮음. 발생 시 메시지에 후속 가이드 추가 가능 (REQ-PMC-018 의 텍스트 보강은 본 SPEC 의 v0.1.1 patch 로 가능). |
| R-003 | dispatcher 가 sctx==nil 인 경우 plan command Execute 가 "plan: context unavailable" 반환 | 사용자 혼란 (왜 이 메시지가 나오나) | dispatcher 의 정상 호출 경로 (`dispatcher.go:134-136`) 에서는 sctx 항상 주입 — 발생 안 함. 외부 직접 호출 (테스트 환경) 만 발생. AC-PMC-012 가 검증. 사용자 가시 환경에서는 sctx == nil 분기 미발생. |
| R-004 | builtin.Register 의 `RegisterAlias("planmode", "plan")` 호출이 이미 다른 canonical 에 매핑된 "planmode" 와 충돌 | builtin Register panic | 현재 빌트인 별칭 명단 (PR #50): `quit` (→ exit), `?` (→ help). "planmode" 충돌 없음. 단, custom command (SourceCustom) 가 application startup 의 builtin 등록 이후에 "planmode" 등록 시도 시 충돌 가능 — Registry 가 해당 케이스를 어떻게 처리하는지는 COMMAND-001 의 책임. AC-PMC-013 가 startup 시점 unique 등록 검증. |
| R-005 | Mutates=false 결정의 정합성 (사용자 직관 위반 가능성) | 일부 사용자가 plan mode 진입 후 다른 변형 명령 (`/clear`, `/model`) 도 자동 차단되는 것을 보고 `/plan off` 자체도 차단된다고 추측 | 본 SPEC §4.2 REQ-PMC-007 / REQ-PMC-008 의 Execute 가 정상 동작하면 즉시 검증 가능 (`/plan off` → "plan mode: off"). 사용자 직관 위반 시 가이드 메시지 보강 가능 (예: `/plan off` 시 "plan mode disabled. /clear and /model now allowed."). 본 SPEC 0.1.0 의 default 는 단순 메시지 — 사용자 피드백 후 0.1.1 patch 로 보강 가능. |
| R-006 | help 명령 (`/help plan`) 출력에 plan command 의 ArgumentHint 가 정확히 표시되지 않을 가능성 | `/help` 사용자가 plan command 사용법을 모름 | `helpCommand.Execute` (PR #50, FROZEN) 가 ArgumentHint 를 출력 — `/help` 결과에 `/plan [on\|off\|toggle\|status]` 자동 표시. 추가 작업 불필요. AC-PMC-013 가 통합 검증 (registry 에 plan 등록되면 /help 에 자동 표시). |
| R-007 | 사용자가 `/plan on` 입력 시점과 PlanModeActive() 가 true 반환 시점 사이의 race | 다른 dispatcher 호출이 그 사이에 mutating 명령을 실행 | ContextAdapter.SetPlanMode 가 atomic.Bool.Store (CMDCTX-001 §6.6) — synchronous, race-free. plan command Execute → setter.SetPlanMode(true) 가 return 직후 다음 dispatcher 호출은 PlanModeActive() == true 관찰. race 없음. |

---

## 10. Exclusions (What NOT to Build)

본 SPEC 은 다음을 **수행하지 않는다**. 각 항목은 후속 SPEC 또는 명시적 결정으로 분리.

1. **TUI 의 prompt prefix / statusbar plan mode indicator 렌더링**
   - 위임: SPEC-GOOSE-CMDCTX-CLI-INTEG-001 REQ-CLIINT-013
   - 본 SPEC 은 SetPlanMode trigger 만 제공. indicator 자체는 CLI-INTEG 의 의무.

2. **plan mode 활성화 시 LLM 동작 변경 (시스템 프롬프트 추가, tool use 제한, 응답 형식 변경 등)**
   - 위임: 후속 SPEC (가칭 SPEC-GOOSE-PLANMODE-LLM-BEHAVIOR-001)
   - 본 SPEC 은 flag toggle 만. PlanModeActive() == true 일 때 LLM 이 어떻게 동작해야 하는지는 별도 SPEC.

3. **plan mode 상태의 daemon RPC 노출 (`AgentService/PlanMode` get/set 가칭)**
   - 위임: SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 (planned)
   - 본 SPEC 은 client-side dispatcher 경로만. daemon 측 trigger / query 는 별도 SPEC.

4. **plan mode 의 다른 trigger 경로**
   - SIGUSR1 같은 signal 기반 토글
   - 환경 변수 (`MINK_PLAN_MODE=1`)
   - config file (`~/.config/goose/config.yaml` 의 `plan_mode: true`)
   - CLI flag (`goose --plan-mode chat`)
   - 본 SPEC 의 default 는 slash command 경로 only. 다른 trigger 경로는 후속 SPEC.

5. **plan mode 의 영구 저장 (다음 세션 재시작 시 복원)**
   - 결정: 본 SPEC 의 default 는 process-lifetime. 재시작 시 plan mode = off.
   - 후속 SPEC 에서 영구 저장 옵션 가능.

6. **subagent 의 `TeammateIdentity{PlanModeRequired:true}` ctx 주입 경로**
   - 위임: CMDCTX-001 §6.5 가 이미 처리
   - 본 SPEC 은 사용자 (top-level orchestrator) 의 SetPlanMode 토글만. subagent 의 plan mode 는 ctx 주입 (TeammateIdentity) 경로로 별도 활성화 — 두 경로는 ContextAdapter.PlanModeActive() 의 OR 결합.

7. **`command.SlashCommandContext` 인터페이스 자체 확장 (옵션 A 거부)**
   - REQ-PMC-013 에서 명시 거부. COMMAND-001 FROZEN 위반.

8. **`*adapter.ContextAdapter` 의 직접 의존성 주입 (옵션 C 거부)**
   - REQ-PMC-014 에서 명시 거부. builtin 패턴 비대칭.

9. **`/plan` 의 단축 별칭 `/p`**
   - 결정: 미래 명령과 충돌 위험 (`/ping`, `/proceed` 등). `/planmode` alias 만 허용.

10. **PlanModeSetter 인터페이스의 시그니처 변경 (예: `SetPlanMode(active bool, reason string)`)**
    - 결정: 본 SPEC 0.1.0 은 `SetPlanMode(active bool)` 만. 시그니처 확장 필요 시 본 SPEC 의 amendment + CMDCTX-001 의 amendment 동시 진행.

11. **COMMAND-001 / CMDCTX-001 본문 변경**
    - 본 SPEC 은 expansion. COMMAND-001 / CMDCTX-001 의 spec.md 본문은 변경하지 않는다.
    - CMDCTX-001 의 production 코드 (adapter.go) 도 변경하지 않는다 (컴파일 단언은 테스트 파일에서).

12. **plan command 의 LocalReply 메시지 i18n / 다국어**
    - 결정: 본 SPEC 0.1.0 은 영문 텍스트만 (CLAUDE.local.md §2.5 코드 주석 영어 정책 준수). 다국어 지원은 후속 SPEC (가칭 SPEC-GOOSE-I18N-COMMAND-001).

13. **plan command 의 사용자 정의 텍스트 (custom command 가 plan 을 override)**
    - 결정: builtin 우선. custom 의 동일 이름 명령은 Registry 의 precedence 정책 (COMMAND-001 §6.5) 위임.

---

## 11. 참조 (References)

- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` (implemented, PR #50, FROZEN) — Command interface, Metadata.Mutates, Dispatcher.ProcessUserInput, builtin.Register 명단, Source.SourceBuiltin
- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` v0.1.1 (implemented, PR #52, FROZEN) — ContextAdapter.SetPlanMode, ContextAdapter.PlanModeActive, atomic.Bool race-clean 보장
- `.moai/specs/SPEC-GOOSE-CMDCTX-CLI-INTEG-001/spec.md` (planned, batch B) §4.3 REQ-CLIINT-013 — TUI plan mode indicator (결합)
- `internal/command/dispatcher.go:114` — `if cmd.Metadata().Mutates && sctx.PlanModeActive()` 차단 분기
- `internal/command/context.go:31-33` — `PlanModeActive() bool` 메서드 (FROZEN)
- `internal/command/adapter/adapter.go:86-88` — `func (a *ContextAdapter) SetPlanMode(active bool)` (FROZEN)
- `internal/command/builtin/clear.go`, `model.go`, `compact.go`, `status.go` — 빌트인 명령 패턴 참고 (FROZEN)
- `CLAUDE.local.md §2.5` — 코드 주석 영어 정책

---

## 12. Acceptance Criteria

### AC-PMC-001 — PlanModeSetter 인터페이스 정의

`internal/command/plan_mode_setter.go` 또는 `internal/command/context.go` 에 다음 인터페이스가 정의됨:

```go
type PlanModeSetter interface {
    SetPlanMode(active bool)
}
```

검증 방법: `go build ./internal/command/...` 에러 없음 + 코드 리뷰. 정적 분석: `grep -A2 'type PlanModeSetter' internal/command/*.go` 매치 1건.

### AC-PMC-002 — *ContextAdapter 의 PlanModeSetter 단언

`internal/command/adapter/` 의 테스트 파일 (예: `plan_mode_setter_assertion_test.go`) 또는 `internal/command/builtin/plan_test.go` 에 다음 단언:

```go
var _ command.PlanModeSetter = (*adapter.ContextAdapter)(nil)
```

컴파일 성공 → ContextAdapter 가 PlanModeSetter 를 자동 만족 (CMDCTX-001 의 SetPlanMode 메서드 시그니처 매치).

검증 방법: `go build ./internal/command/adapter/...` + `go test -count=1 ./internal/command/adapter/...` 에러 없음.

### AC-PMC-003 — planCommand Command interface 구현

`internal/command/builtin/plan.go` 의 `planCommand` 가 `command.Command` 인터페이스 (Name, Metadata, Execute) 를 구현. 컴파일 단언:

```go
var _ command.Command = (*planCommand)(nil)
```

검증 방법: `go build ./internal/command/builtin/...` 에러 없음 + 코드 리뷰.

### AC-PMC-004 — Metadata 필드값

`(&planCommand{}).Metadata()` 가 다음 필드값 반환:
- `Description`: non-empty 영문 (예: "Toggle plan mode (read-only mode for inspection).")
- `ArgumentHint`: `"[on|off|toggle|status]"`
- `Mutates`: `false` ⬅︎ 가장 중요
- `Source`: `command.SourceBuiltin`

검증 방법: unit test —
```go
md := (&planCommand{}).Metadata()
assert.False(t, md.Mutates)
assert.Equal(t, "[on|off|toggle|status]", md.ArgumentHint)
assert.Equal(t, command.SourceBuiltin, md.Source)
assert.NotEmpty(t, md.Description)
```

### AC-PMC-005 — Name() 반환값

`(&planCommand{}).Name() == "plan"`.

검증 방법: unit test — `assert.Equal(t, "plan", (&planCommand{}).Name())`.

### AC-PMC-006 — `/plan on` 동작

dispatcher 가 `/plan on` 입력을 처리 시 (a) ContextAdapter.SetPlanMode(true) 호출, (b) `Result{Kind: ResultLocalReply, Text: "plan mode: on"}` 반환. 사전 상태가 plan-mode-off 이면 텍스트 "plan mode: on", 사전 상태가 plan-mode-on 이면 텍스트 "plan mode: on (already active)".

검증 방법: unit test with ContextAdapter (실제 인스턴스 또는 mock with PlanModeSetter):
```go
a := adapter.New(adapter.Options{})
ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)
result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{"on"}})
assert.NoError(t, err)
assert.Equal(t, command.ResultLocalReply, result.Kind)
assert.Equal(t, "plan mode: on", result.Text)
assert.True(t, a.PlanModeActive())
```

### AC-PMC-007 — `/plan off` 동작

위와 동일 패턴. 입력 `"off"` → SetPlanMode(false), 텍스트 "plan mode: off" 또는 "plan mode: off (already inactive)".

검증 방법: unit test (a.SetPlanMode(true) 사전 호출 후 plan command 실행).

### AC-PMC-008 — `/plan toggle` 동작

입력 `"toggle"` → 사전 PlanModeActive() 반전. 사전 false → 사후 true (텍스트 "plan mode: on"), 사전 true → 사후 false (텍스트 "plan mode: off").

검증 방법: unit test — `/plan toggle` 두 번 실행 후 PlanModeActive() 가 false → true → false 순으로 변화.

### AC-PMC-009 — `/plan status` / `/plan` (인자 없음) 동작

입력 `""` 또는 `"status"` → SetPlanMode 미호출 (read-only), `Result.Text == "plan mode: " + ("on"|"off")`.

검증 방법: unit test —
```go
a := adapter.New(adapter.Options{})
a.SetPlanMode(true)
ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)

result, _ := (&planCommand{}).Execute(ctx, command.Args{Positional: nil})
assert.Equal(t, "plan mode: on", result.Text)
assert.True(t, a.PlanModeActive())  // unchanged

result, _ = (&planCommand{}).Execute(ctx, command.Args{Positional: []string{"status"}})
assert.Equal(t, "plan mode: on", result.Text)
```

### AC-PMC-010 — 잘못된 인자 / 인자 2개 이상

입력 `/plan foo`, `/plan unknown`, `/plan on extra` → SetPlanMode 미호출, `Result.Text == "usage: /plan [on|off|toggle|status]"`.

검증 방법: table-driven unit test:
```go
tests := []struct {
    name string
    args []string
}{
    {"unknown sub", []string{"foo"}},
    {"random word", []string{"unknown"}},
    {"too many args", []string{"on", "extra"}},
    {"too many args 2", []string{"toggle", "now"}},
}
for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
        a := adapter.New(adapter.Options{})
        ctx := context.WithValue(context.Background(), command.SctxContextKey(), a)
        result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: tc.args})
        assert.NoError(t, err)
        assert.Contains(t, result.Text, "usage:")
        assert.False(t, a.PlanModeActive())  // unchanged
    })
}
```

### AC-PMC-011 — Mutates=false plan-mode-active 차단 부재

dispatcher integration test — ContextAdapter.SetPlanMode(true) 사전 호출 후 `/plan off` 입력. dispatcher 의 `if Metadata.Mutates && PlanModeActive()` 차단 분기를 통과 (Mutates=false 이므로). plan command Execute 가 정상 호출되어 SetPlanMode(false) 실행. 결과 PlanModeActive() == false.

검증 방법: integration test:
```go
reg := command.NewRegistry(...)
builtin.Register(reg)
a := adapter.New(adapter.Options{})
a.SetPlanMode(true)
disp := command.NewDispatcher(reg, command.Config{}, nil)

processed, err := disp.ProcessUserInput(context.Background(), "/plan off", a)
assert.NoError(t, err)
assert.Equal(t, command.ProcessLocal, processed.Kind)
assert.False(t, a.PlanModeActive())  // SetPlanMode(false) executed
```

### AC-PMC-012 — sctx == nil graceful

`(&planCommand{}).Execute(context.Background(), command.Args{Positional: []string{"on"}})` (sctx 미주입) → no error, `Result.Text == "plan: context unavailable"`.

검증 방법: unit test — `extractSctx(ctx) == nil` 시나리오. dispatcher 의 정상 호출 경로에서는 발생 안 함, 외부 직접 호출 케이스만.

### AC-PMC-013 — builtin.Register 후 plan command 등록

`builtin.Register(reg)` 호출 후 (a) `reg.Resolve("plan")` 가 `*planCommand` 반환, (b) `reg.Resolve("planmode")` 도 동일 (alias 매핑), (c) `reg.List()` 결과에 plan 포함, (d) `reg.ListNamed()` 결과에 `{Name: "plan", Metadata: {Mutates: false, ...}}` 포함.

검증 방법: integration test:
```go
reg := command.NewRegistry(...)
builtin.Register(reg)

cmd, ok := reg.Resolve("plan")
assert.True(t, ok)
assert.Equal(t, "plan", cmd.Name())

cmd2, ok := reg.Resolve("planmode")
assert.True(t, ok)
assert.Same(t, cmd, cmd2)  // alias to same command

names := []string{}
for _, m := range reg.ListNamed() {
    names = append(names, m.Name)
}
assert.Contains(t, names, "plan")
```

### AC-PMC-014 — 정적 분석: SlashCommandContext 본문에 SetPlanMode 부재 (REQ-PMC-013)

`grep -E '^\s*SetPlanMode' internal/command/context.go` 결과 0건 (SlashCommandContext 인터페이스 본문에 SetPlanMode 메서드 미추가).

검증 방법: CI 정적 분석 단계 + 코드 리뷰. COMMAND-001 의 SlashCommandContext 본문 (PR #50) 변경 부재 확인.

### AC-PMC-015 — 정적 분석: builtin/plan.go 가 adapter 패키지 import 부재 (REQ-PMC-014)

`grep -E '"github.com/modu-ai/goose/internal/command/adapter"' internal/command/builtin/plan.go` 결과 0건. plan command 의 production 코드는 adapter package 직접 의존 없음. type assertion 은 `command.PlanModeSetter` interface 통해서만.

검증 방법: CI 정적 분석 + 코드 리뷰. (테스트 파일은 예외 — `*_test.go` 는 adapter import 가능, REQ-PMC-014 production 코드 한정.)

### AC-PMC-016 — Execute error 부재 (REQ-PMC-016)

planCommand.Execute 의 모든 분기 (정상 on/off/toggle/status, 잘못된 인자, sctx nil, PlanModeSetter 미구현) 에서 `error == nil`. 컴파일러가 error 반환 가능성을 알지만 현재 본 구현체는 항상 nil 반환.

검증 방법: unit test — 모든 분기에서 `err == nil` 단언. 정적 분석 (선택): `grep -E 'return\s+command\.Result\{[^}]*\},\s*(err|fmt\.Errorf)' internal/command/builtin/plan.go` 결과 0건.

### AC-PMC-017 — sctx 가 PlanModeSetter 미구현 시 graceful (REQ-PMC-018)

mock sctx (PlanModeActive 만 구현, SetPlanMode 미구현) 사용 시 `/plan on` → no error, `Result.Text == "plan: this session does not support plan mode toggling"`.

검증 방법: unit test:
```go
type mockSctxNoSetter struct{}
func (m *mockSctxNoSetter) OnClear() error { return nil }
func (m *mockSctxNoSetter) OnModelChange(command.ModelInfo) error { return nil }
func (m *mockSctxNoSetter) OnCompactRequest(int) error { return nil }
func (m *mockSctxNoSetter) ResolveModelAlias(string) (*command.ModelInfo, error) { return nil, command.ErrUnknownModel }
func (m *mockSctxNoSetter) SessionSnapshot() command.SessionSnapshot { return command.SessionSnapshot{} }
func (m *mockSctxNoSetter) PlanModeActive() bool { return false }

m := &mockSctxNoSetter{}
ctx := context.WithValue(context.Background(), command.SctxContextKey(), command.SlashCommandContext(m))
result, err := (&planCommand{}).Execute(ctx, command.Args{Positional: []string{"on"}})
assert.NoError(t, err)
assert.Contains(t, result.Text, "does not support plan mode toggling")
```

(mockSctxNoSetter 는 SlashCommandContext 6 메서드만 구현, PlanModeSetter 미만족 — type assertion 실패 분기 검증.)

---

Version: 0.1.0
Last Updated: 2026-04-27
REQ coverage: REQ-PMC-001 ~ REQ-PMC-020 (총 20)
AC coverage: AC-PMC-001 ~ AC-PMC-017 (총 17)
