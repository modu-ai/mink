---
id: SPEC-GOOSE-CMDCTX-CLI-INTEG-001
version: 0.1.0
status: completed
created_at: 2026-04-27
updated_at: 2026-05-04
completed: 2026-05-04
author: manager-spec
priority: P1
issue_number: null
phase: 2
size: 중(M)
lifecycle: spec-anchored
labels: [area/cli, area/runtime, type/feature, priority/p1-high]
---

# SPEC-GOOSE-CMDCTX-CLI-INTEG-001 — CLI 진입점에서 ContextAdapter / Dispatcher Wiring (Integration Add-on to CLI-001)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. PR #50 (COMMAND-001 implemented) + PR #52 (CMDCTX-001 implemented v0.1.1) 머지로 정의된 dispatcher / ContextAdapter / SlashCommandContext 자산을 SPEC-GOOSE-CLI-001 (planned, FROZEN-by-reference) 의 cobra root + bubbletea TUI + ask cmd 진입점에 wiring 하는 integration add-on SPEC. CLI-001 본문은 변경하지 않는다. CMDLOOP-WIRE-001 / ALIAS-CONFIG-001 / CMDCTX-PERMISSIVE-ALIAS-001 (모두 planned, batch A) 와 결합. SIGINT 처리는 RequestClear 와 분리 (Exclusions §10). | manager-spec |

---

## 1. 개요 (Overview)

PR #50 (SPEC-GOOSE-COMMAND-001) 와 PR #52 (SPEC-GOOSE-CMDCTX-001 v0.1.1) 머지로 다음 자산이 implemented (FROZEN) 상태이다:

- `command.Dispatcher` + `command.SlashCommandContext` 인터페이스 (`internal/command/`)
- `adapter.ContextAdapter` + `adapter.New(Options)` + `adapter.WithContext(ctx)` (`internal/command/adapter/`)

그러나 **이 자산을 사용자 대면 CLI 진입점 (cobra root, bubbletea TUI, `ask` cmd) 에 instantiate / wire 하는 코드는 아직 없다**. SPEC-GOOSE-CLI-001 (planned v0.2.0) 은 cobra/bubbletea 의 패키지 레이아웃을 정의하지만 dispatcher / ContextAdapter wiring 은 §6.5 의 stub `tuiSlashContext` 외에 명시하지 않는다.

본 SPEC 수락 시점에서:

- `internal/cli/app.go` (신규) 에 `App` struct 가 존재하고 `Adapter`, `Dispatcher`, `Client` 를 보유한다.
- `internal/cli/rootcmd.go` 의 `PersistentPreRunE` 에서 `App.New(cfg)` 가 1회 호출되어 단일 인스턴스가 자식 cmd 트리에 전파된다.
- `internal/cli/tui/update.go` 의 `handleSubmit` 가 매 user input 마다 `app.Dispatcher.ProcessUserInput(ctx, line, app.Adapter.WithContext(ctx))` 를 호출한다.
- `internal/cli/commands/ask.go` 가 비대화 모드 첫 input 에 대해 동일 dispatcher 호출 패턴을 적용한다.
- `--alias-file` / `--strict-alias` persistent flag 가 `aliasconfig.LoadDefault(LoadOptions)` 의 입력으로 매핑된다.
- alias config / LoopController / strict-alias 토글 부재 시 nil/empty fallback 으로 panic 없이 동작한다 (CMDCTX-001 의 graceful degradation 위임).

본 SPEC 은 **CLI 진입점에서의 wiring add-on** 이다. CLI-001 의 본문 (REQ-CLI-001 ~ REQ-CLI-025, AC-CLI-001 ~ AC-CLI-016) 은 **변경하지 않는다**. 본 SPEC 은 CLI-001 의 슈퍼셋이 아니라 그 위에 얹는 어드온이다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **PR #52 후 dispatcher 활성화 불가**: CMDCTX-001 의 ContextAdapter 가 implemented 상태이지만, 어떤 CLI 진입점도 `adapter.New(Options{...})` 를 호출하지 않는다. 결과적으로 dispatcher 가 instantiate 되지 않고 어떤 slash command 도 동작하지 않는다.
- **CLI-001 의 §6.5 stub 보완**: CLI-001 v0.2.0 §6.5 의 `m.sctx()` 는 `tuiSlashContext` stub 으로 정의되었으나, 본 SPEC 이 그 stub 을 `app.Adapter.WithContext(ctx)` 로 대체한다. CLI-001 본문은 stub 임을 명시했으므로 본 SPEC 은 그 stub 의 실 wiring 을 채운다.
- **TRUST 5 Trackable 결손**: CLI 진입점 wiring 이 SPEC 으로 명시되지 않으면 누락된 wiring 이 추적 불가능. 본 SPEC 으로 명시화.
- **CMDLOOP-WIRE-001 / ALIAS-CONFIG-001 / PERMISSIVE-ALIAS-001 의 산출물 회수**: batch A SPEC 들이 implemented 되어도 CLI 가 그 산출물을 instantiate / wire 하지 않으면 dead code. 본 SPEC 이 그 회수 경로.

### 2.2 상속 자산 (FROZEN by reference)

- **SPEC-GOOSE-COMMAND-001** (implemented, PR #50, FROZEN): `command.Dispatcher`, `command.SlashCommandContext`, `command.ModelInfo`, `command.SessionSnapshot`, `command.ErrUnknownModel`. 본 SPEC 은 이 자산을 instantiate / 호출 만 한다. 변경하지 않는다.
- **SPEC-GOOSE-CMDCTX-001** (implemented, PR #52, v0.1.1, FROZEN): `adapter.ContextAdapter`, `adapter.Options`, `adapter.New`, `adapter.WithContext`, `adapter.SetPlanMode`, `adapter.LoopController` 인터페이스, `adapter.LoopSnapshot`. 본 SPEC 은 `New(Options{...})` 와 `WithContext(ctx)` 만 호출한다. 변경하지 않는다.
- **SPEC-GOOSE-CLI-001** (planned, v0.2.0, FROZEN-by-reference): `cmd/mink/main.go`, `internal/cli/rootcmd.go`, `internal/cli/tui/`, `internal/cli/commands/ask.go`, `internal/cli/transport/`. 본 SPEC 은 이 패키지 레이아웃을 read-only 로 가정하고 wiring add-on 만 추가한다. CLI-001 의 REQ / AC / 본문 변경 금지.
- **SPEC-GOOSE-CMDLOOP-WIRE-001** (planned, batch A): `cmdctrl.New(engine, logger)` 시그니처. 본 SPEC 은 §7.1 의 옵션 γ 를 채택하여 client-side 에서는 LoopController 를 omit 또는 stub 으로 처리. CMDLOOP-WIRE-001 의 산출물은 daemon-side wiring (DAEMON-WIRE-001) 에서 직접 회수.
- **SPEC-GOOSE-ALIAS-CONFIG-001** (planned, batch A): `aliasconfig.LoadDefault(LoadOptions)` 시그니처. 본 SPEC 은 그 반환 map 을 `adapter.Options.AliasMap` 에 주입.
- **SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001** (planned, batch A): strict / lenient mode 토글. 본 SPEC 의 `--strict-alias` flag 와 결합.

### 2.3 범위 경계 (한 줄)

본 SPEC 은 **CLI 진입점에서 ContextAdapter + Dispatcher 인스턴스화 / 주입** 만 wiring 한다. daemon mode wiring (DAEMON-INTEG SPEC), bubbletea TUI 구조 (CLI-001), interactive prompt 디자인 (CLI-001) 은 모두 범위 외.

---

## 3. 목표 (Goals)

### 3.1 IN SCOPE

| ID | 항목 | 목적 |
|----|------|-----|
| G-1 | `internal/cli/app.go` 신규 — `App` struct 정의 (Adapter, Dispatcher, Client 보유) | 단일 wiring 진실 공급원 |
| G-2 | `App.New(cfg) (*App, error)` 생성자 | adapter / dispatcher 인스턴스화 (process 시작 시 1회) |
| G-3 | `internal/cli/rootcmd.go` `PersistentPreRunE` 에 `App.New` 호출 추가 | 자식 cmd 트리에 *App 전파 |
| G-4 | `--alias-file` / `--strict-alias` persistent flag 추가 | `aliasconfig.LoadDefault` / PERMISSIVE-ALIAS-001 토글 매핑 |
| G-5 | `internal/cli/tui/update.go` handleSubmit 의 `m.sctx()` stub 을 `app.Adapter.WithContext(ctx)` 로 대체 | 매 user input 별 child adapter 주입 |
| G-6 | `internal/cli/commands/ask.go` 의 첫 input 에 dispatcher 호출 추가 | 비대화 모드 일관성 (REQ-CLI-021) |
| G-7 | nil registry / nil loopCtrl / 빈 alias map fallback 그대로 동작 | CMDCTX-001 graceful degradation 위임 |
| G-8 | App.New 단위 테스트 + TUI handleSubmit teatest + ask cmd subprocess test | TRUST 5 Tested |

### 3.2 OUT OF SCOPE (Exclusions §10 에서 상세화)

- daemon mode wiring (DAEMON-INTEG / DAEMON-WIRE-001 후속 SPEC)
- bubbletea TUI 디자인 / 키바인딩 / 렌더링 (CLI-001 범위)
- LoopController 의 client-side 구현 (옵션 γ 채택, omit/stub)
- proto 확장 (mutating slash command → daemon RPC) — 옵션 α 거부
- SIGINT (Ctrl-C) → RequestClear trigger
- alias config 파일 hot-reload (HOTRELOAD-001 후속)
- `/plan` slash command 자체 (COMMAND-001 후속, SetPlanMode trigger 별도)
- CLI-001 의 REQ / AC / 본문 변경

---

## 4. 요구사항 (Requirements, EARS)

### 4.1 Ubiquitous Requirements (always active)

**REQ-CLIINT-001**: 시스템은 process lifetime 동안 단일 `*adapter.ContextAdapter` 인스턴스를 보유해야 한다 (`shall hold a single ContextAdapter instance per process lifetime`). 모든 user-input 진입점은 동일 인스턴스의 `WithContext(ctx)` 자식을 사용한다.

**REQ-CLIINT-002**: 시스템은 process lifetime 동안 단일 `*command.Dispatcher` 인스턴스를 보유해야 한다 (`shall hold a single Dispatcher instance per process lifetime`).

**REQ-CLIINT-003**: 시스템은 `App` struct (`internal/cli/app.go`) 에서 `Adapter *adapter.ContextAdapter`, `Dispatcher *command.Dispatcher`, `Client *transport.Client` 3개 필드를 노출해야 한다 (`shall expose Adapter, Dispatcher, Client fields on App struct`).

**REQ-CLIINT-004**: 시스템은 `App.New(cfg Config) (*App, error)` 생성자가 (a) `router.DefaultRegistry()` 로 registry 획득, (b) `aliasconfig.LoadDefault(...)` 또는 빈 맵 fallback 으로 alias map 획득, (c) `adapter.New(Options{...})` 호출, (d) `command.NewDispatcher(...)` 호출 의 4단계를 순서대로 실행하도록 해야 한다 (`shall execute registry → alias map → adapter → dispatcher in order`).

**REQ-CLIINT-005**: 시스템은 모든 user-input 진입점 (TUI handleSubmit, ask cmd, ask --stdin) 에서 dispatcher 호출 패턴이 `app.Dispatcher.ProcessUserInput(ctx, input, app.Adapter.WithContext(ctx))` 으로 일관되어야 한다 (`shall invoke dispatcher with WithContext(ctx) child adapter consistently across all entry points`).

**REQ-CLIINT-006**: 시스템은 `App.New` 가 nil registry / nil loopCtrl / 빈 alias map 입력에 대해 panic 없이 graceful degradation 해야 한다 (`shall not panic on nil registry, nil loopCtrl, or empty alias map`). nil deps 의 시맨틱 해석은 CMDCTX-001 REQ-CMDCTX-014 / -015 에 위임.

**REQ-CLIINT-007**: 시스템은 `App.New` 가 alias config 로드 실패 (파일 부재, 권한 없음, 파싱 에러) 에 대해 빈 맵 + warn log 로 fallback 하고 `App.New` 자체는 nil error 를 반환해야 한다 (`shall fallback to empty alias map with warn log on config load failure`). registry 부재 (router.DefaultRegistry() 자체가 빈 맵) 는 별개 — REQ-CLIINT-013 참조.

### 4.2 Event-Driven Requirements (when X then Y)

**REQ-CLIINT-008**: WHEN process 가 시작되어 cobra root 의 `PersistentPreRunE` 가 실행되면 THEN 시스템은 `App.New(cfg)` 를 정확히 1회 호출하고 그 결과를 cobra cmd 트리에 (Context 또는 closure 를 통해) 전파해야 한다 (`shall invoke App.New exactly once during PersistentPreRunE`).

**REQ-CLIINT-009**: WHEN bubbletea TUI 의 `handleSubmit` 이 user input 라인을 받으면 THEN 시스템은 (a) `streamCtx := derive(cmd.Context())` 를 cancellable context 로 derive, (b) `sctx := app.Adapter.WithContext(streamCtx)` 호출, (c) `app.Dispatcher.ProcessUserInput(streamCtx, line, sctx)` 호출, (d) 결과의 `Kind` 에 따라 (CLI-001 §6.5 의 switch 문) 분기해야 한다 (`shall invoke dispatcher with child adapter on every TUI submit`).

**REQ-CLIINT-010**: WHEN `mink ask "<message>"` 또는 `mink ask --stdin` 이 invoke 되면 THEN 시스템은 첫 input 에 대해 dispatcher 호출 패턴 (`ProcessUserInput(ctx, input, app.Adapter.WithContext(ctx))`) 을 적용해야 한다 (`shall invoke dispatcher on first input in ask mode`). dispatcher 결과:
- `ProcessLocal`: stdout 에 메시지 출력 후 exit 0.
- `ProcessExit`: exit 0.
- `ProcessProceed`: expanded prompt 를 daemon 의 `AgentService/ChatStream` 에 전송 (CLI-001 REQ-CLI-007 경로).
- `ProcessAbort`: stderr 메시지 + exit 1.

**REQ-CLIINT-011**: WHEN `--alias-file <path>` flag 가 명시되면 THEN 시스템은 `aliasconfig.LoadDefault(LoadOptions{AliasFile: path, ...})` 를 호출하여 그 파일을 우선 사용해야 한다 (`shall use --alias-file path when specified`). 명시되지 않으면 환경 변수 `MINK_ALIAS_FILE` 우선, 그 다음 default `$MINK_HOME/aliases.yaml` (ALIAS-CONFIG-001 위임).

**REQ-CLIINT-012**: WHEN `--strict-alias` flag 가 true 로 명시되면 THEN 시스템은 `aliasconfig.LoadDefault(LoadOptions{Strict: true, ...})` 를 호출하여 unregistered canonical 에 대해 error 반환을 강제해야 한다 (`shall enforce strict alias validation when --strict-alias is true`). 기본값 false (lenient — PERMISSIVE-ALIAS-001 의 default 동작).

### 4.3 State-Driven Requirements (while X)

**REQ-CLIINT-013**: WHILE TUI 모드에서 `app.Adapter.PlanModeActive()` 가 true 를 반환하는 동안 THEN 시스템은 readline / textarea prompt prefix 또는 statusbar 에 plan mode indicator (예: `[plan]`) 를 표시해야 한다 (`shall display plan mode indicator while plan mode is active`). 비대화 모드 (`ask`) 에서는 미표시.

**REQ-CLIINT-014**: WHILE registry 가 빈 맵 (router.DefaultRegistry() 가 어떤 provider 도 등록하지 않은 상태) 인 동안 THEN 시스템은 `App.New` 시 stderr 에 명확한 진단 메시지 (`goose: provider registry is empty; --model commands will fail`) 를 출력하고 exit 0 (graceful degradation 으로 진행) 해야 한다 (`shall warn on empty registry but proceed`). exit 1 이 아닌 이유: `version`, `ping` 같은 registry 에 의존하지 않는 cmd 가 있다.

### 4.4 Unwanted Behavior (shall not / shall reject)

**REQ-CLIINT-015**: 시스템은 `App.New` 가 multiple call (process 내) 되지 않도록 해야 한다 (`shall not allow App.New to be called more than once per process`). cobra `PersistentPreRunE` 의 sync.Once 또는 동등 보장으로 단일 호출 강제. 두 번째 호출은 첫 번째 결과를 반환 (idempotent).

**REQ-CLIINT-016**: 시스템은 dispatcher 호출 시 `WithContext(nil)` 또는 `WithContext(context.TODO())` 같은 비명시적 ctx 를 전달하지 않아야 한다 (`shall not pass nil or context.TODO() to WithContext`). 모든 진입점은 cobra cmd.Context() 또는 그 derived ctx 를 명시적으로 사용.

**REQ-CLIINT-017**: 시스템은 SIGINT (Ctrl-C) 수신 시 `LoopController.RequestClear` 를 trigger 하지 않아야 한다 (`shall not trigger RequestClear on SIGINT`). SIGINT 의 의미는 CLI-001 REQ-CLI-009 그대로 (현재 stream cancel + viewport `[aborted]`). RequestClear 는 명시적 `/clear` slash command 만 trigger.

**REQ-CLIINT-018**: 시스템은 `App.New` 실행 중 발생하는 panic 을 process 종료로 전파해야 한다 (`shall propagate App.New panics to process exit`). 의도적으로 recover 하지 않음 — wiring 실패는 fail-fast.

### 4.5 Optional Requirements (where possible)

**REQ-CLIINT-019**: WHERE `--alias-file` flag 가 제공되면 시스템은 ALIAS-CONFIG-001 의 `LoadOptions.AliasFile` 으로 그 값을 매핑해야 한다 (`shall map --alias-file flag to LoadOptions.AliasFile`). flag 미제공 시 ALIAS-CONFIG-001 의 default 동작 (env / MINK_HOME).

**REQ-CLIINT-020**: WHERE `--strict-alias` flag 가 제공되면 시스템은 ALIAS-CONFIG-001 의 `LoadOptions.Strict` 와 PERMISSIVE-ALIAS-001 의 strict toggle 양쪽에 동일 값으로 매핑해야 한다 (`shall map --strict-alias flag to both LoadOptions.Strict and PERMISSIVE-ALIAS-001 toggle`). flag 미제공 시 default false (lenient).

**REQ-CLIINT-021**: WHERE structured logger (zap) 가 주입되면 시스템은 (a) App.New 의 4단계 진행, (b) 각 dispatcher 호출의 ProcessKind, (c) PlanModeActive 토글 변경 을 debug level 로 기록해야 한다 (`shall log wiring lifecycle events when logger is provided`). logger nil 이면 silent.

**REQ-CLIINT-022**: WHERE CMDLOOP-WIRE-001 이 implemented 되어 있으면 시스템은 `cmdctrl.New(engine, logger)` 를 호출하여 `Options.LoopController` 에 주입할 수 있다 (`may invoke cmdctrl.New when implemented`). 단, 본 SPEC 의 default 는 옵션 γ (client-side 에서 omit, daemon-side 에서 처리) 이므로 `Options.LoopController = nil` 도 정상 경로. 사용자 결정에 따라 활성화.

---

## 5. REQ ↔ AC 매트릭스

| REQ | AC | 관련 진입점 / 함수 |
|-----|----|----------|
| REQ-CLIINT-001 | AC-CLIINT-001 (단일 인스턴스) | App.New, App.Adapter |
| REQ-CLIINT-002 | AC-CLIINT-001 | App.Dispatcher |
| REQ-CLIINT-003 | AC-CLIINT-002 (struct fields) | app.go 컴파일 |
| REQ-CLIINT-004 | AC-CLIINT-003 (4단계 순서) | App.New |
| REQ-CLIINT-005 | AC-CLIINT-004, AC-CLIINT-005, AC-CLIINT-006 | TUI / ask / ask --stdin |
| REQ-CLIINT-006 | AC-CLIINT-007 (nil deps graceful) | App.New |
| REQ-CLIINT-007 | AC-CLIINT-008 (alias config 로드 실패) | App.New |
| REQ-CLIINT-008 | AC-CLIINT-009 (PersistentPreRunE 1회) | rootcmd |
| REQ-CLIINT-009 | AC-CLIINT-004 (TUI dispatcher 호출) | tui/update.go handleSubmit |
| REQ-CLIINT-010 | AC-CLIINT-005, AC-CLIINT-006 | commands/ask.go |
| REQ-CLIINT-011 | AC-CLIINT-010 | rootcmd flag → ALIAS-CONFIG-001 |
| REQ-CLIINT-012 | AC-CLIINT-011 | rootcmd flag → PERMISSIVE-ALIAS-001 |
| REQ-CLIINT-013 | AC-CLIINT-012 (plan mode indicator) | TUI statusbar |
| REQ-CLIINT-014 | AC-CLIINT-013 (registry empty warn) | App.New |
| REQ-CLIINT-015 | AC-CLIINT-014 (single call) | App.New idempotent |
| REQ-CLIINT-016 | AC-CLIINT-015 (정적 분석 / no nil ctx) | dispatcher 호출 사이트 |
| REQ-CLIINT-017 | AC-CLIINT-016 (SIGINT 동작) | tui handler |
| REQ-CLIINT-018 | AC-CLIINT-017 (panic propagate) | App.New |
| REQ-CLIINT-019 | AC-CLIINT-010 | flag 매핑 |
| REQ-CLIINT-020 | AC-CLIINT-011 | flag 매핑 |
| REQ-CLIINT-021 | AC-CLIINT-018 (logger debug) | App.New + handleSubmit |
| REQ-CLIINT-022 | AC-CLIINT-019 (CMDLOOP optional) | App.New |

총 22 REQ / 19 AC. 모든 REQ 는 최소 1개 AC 로 검증된다.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃 (CLI-001 §6.1 위에 add-on)

CLI-001 §6.1 의 패키지 레이아웃을 read-only 로 가정. 본 SPEC 이 추가하는 신규/변경:

```
internal/cli/
├── app.go                       # ⬅︎ 신규 (본 SPEC, ~100 LoC)
├── app_test.go                  # ⬅︎ 신규 (~150 LoC)
├── rootcmd.go                   # ⬅︎ 변경 (+30 LoC: PersistentPreRunE + 2 flag)
├── commands/
│   └── ask.go                   # ⬅︎ 변경 (+20 LoC: dispatcher 호출)
└── tui/
    ├── model.go                 # ⬅︎ 변경 (+5 LoC: Model.app *App 필드)
    └── update.go                # ⬅︎ 변경 (+10 LoC: handleSubmit 의 m.sctx() 대체)
```

### 6.2 데이터 모델 — `App` struct

```go
// internal/cli/app.go
package cli

import (
    "context"
    "errors"
    "os"

    "github.com/modu-ai/goose/internal/cli/transport"
    "github.com/modu-ai/goose/internal/command"
    "github.com/modu-ai/goose/internal/command/adapter"
    "github.com/modu-ai/goose/internal/command/adapter/aliasconfig" // ALIAS-CONFIG-001
    "github.com/modu-ai/goose/internal/llm/router"
    "go.uber.org/zap"
)

// App holds the wiring dependencies for the mink CLI.
//
// One instance per process. Initialized in rootcmd.PersistentPreRunE via App.New.
// All user-input entry points (TUI handleSubmit, ask cmd, session load) call
// app.Dispatcher.ProcessUserInput(ctx, input, app.Adapter.WithContext(ctx))
// to ensure consistent slash-command pre-dispatch behavior.
//
// @MX:ANCHOR: CLI wiring root.
// @MX:REASON: Single source of truth for adapter, dispatcher, transport client.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-CLI-INTEG-001 REQ-CLIINT-001/002/003
type App struct {
    Adapter    *adapter.ContextAdapter
    Dispatcher *command.Dispatcher
    Client     *transport.Client
    Logger     *zap.Logger
}

// Config carries the persistent flags + env values needed for App.New.
// Populated in rootcmd.PersistentPreRunE from cobra/viper.
type Config struct {
    AliasFile   string // --alias-file flag, falls back to MINK_ALIAS_FILE env, then default
    StrictAlias bool   // --strict-alias flag, default false
    DaemonAddr  string // --daemon-addr flag, falls back to CONFIG-001
    Logger      *zap.Logger
}

// New initializes the App with the four-stage wiring (REQ-CLIINT-004).
// Order:
//   1. registry := router.DefaultRegistry()
//   2. aliasMap := aliasconfig.LoadDefault(...) — fallback to empty map on failure
//   3. adapter := adapter.New(adapter.Options{...})
//   4. dispatcher := command.NewDispatcher(registry, command.Config{}, logger)
//   5. transport client (CLI-001 §6.3 — Connect-Go)
//
// nil deps are tolerated per CMDCTX-001 REQ-CMDCTX-014/-015 (graceful degradation).
// Panics propagate to process exit (REQ-CLIINT-018, fail-fast).
func New(cfg Config) (*App, error) {
    // Stage 1: registry
    registry := router.DefaultRegistry()
    if cfg.Logger != nil && registry != nil && registry.Empty() {
        // REQ-CLIINT-014: warn but proceed
        fmt.Fprintln(os.Stderr, "goose: provider registry is empty; --model commands will fail")
    }

    // Stage 2: alias map (graceful fallback to empty map)
    aliasMap, err := aliasconfig.LoadDefault(aliasconfig.LoadOptions{
        AliasFile: cfg.AliasFile,
        Strict:    cfg.StrictAlias,
        Registry:  registry,
        Logger:    cfg.Logger,
    })
    if err != nil {
        // REQ-CLIINT-007: warn + empty map fallback, App.New itself returns nil
        if cfg.Logger != nil {
            cfg.Logger.Warn("alias config load failed; using empty map", zap.Error(err))
        }
        aliasMap = map[string]string{}
    }

    // Stage 3: ContextAdapter
    a := adapter.New(adapter.Options{
        Registry:       registry,
        LoopController: nil, // REQ-CLIINT-022: client-side omit (option γ); CMDLOOP-WIRE-001
                              //                  is consumed daemon-side (DAEMON-WIRE-001).
        AliasMap:       aliasMap,
        GetwdFn:        nil,  // defaults to os.Getwd
        Logger:         zapLoggerAdapter(cfg.Logger),
    })

    // Stage 4: Dispatcher
    d := command.NewDispatcher(registry, command.Config{}, cfg.Logger)

    // Stage 5: Connect-Go transport client (CLI-001 §6.3)
    tc, err := transport.Dial(context.Background(), cfg.DaemonAddr, /* timeout */)
    if err != nil {
        // Transport error is recoverable — `version`/`ping` still work without daemon.
        // App.New returns the partial App with Client=nil; callers check before stream calls.
        if cfg.Logger != nil {
            cfg.Logger.Debug("daemon unreachable at App.New; will retry per-cmd", zap.Error(err))
        }
        tc = nil
    }

    return &App{
        Adapter:    a,
        Dispatcher: d,
        Client:     tc,
        Logger:     cfg.Logger,
    }, nil
}
```

### 6.3 wiring 패턴 — TUI handleSubmit

CLI-001 §6.5 의 stub `m.sctx()` 를 본 SPEC 이 대체:

```go
// internal/cli/tui/update.go (변경된 부분만)

// CLI-001 §6.5 의 stub 제거:
// func (m Model) sctx() command.SlashCommandContext { ... tuiSlashContext stub ... }
//
// 본 SPEC 이 대체:
func (m Model) handleSubmit() (Model, tea.Cmd) {
    line := m.input.Value()
    if line == "" {
        return m, nil
    }

    // streamCtx is Ctrl-C cancellable; derived from m.app's process ctx.
    streamCtx, cancel := context.WithCancel(m.app.processCtx)
    m.streamCancel = cancel

    // REQ-CLIINT-009: child adapter scoped to this submit's ctx.
    sctx := m.app.Adapter.WithContext(streamCtx)

    processed, err := m.app.Dispatcher.ProcessUserInput(streamCtx, line, sctx)
    if err != nil {
        return m.handleErr(errMsg{err})
    }

    switch processed.Kind {
    case command.ProcessLocal:
        m.session.AppendSystem(processed.Messages)
        m.input.Reset()
        return m, nil
    case command.ProcessExit:
        return m, tea.Quit
    case command.ProcessProceed:
        return m.startStream(streamCtx, processed.Prompt)
    case command.ProcessAbort:
        m.input.Reset()
        return m, nil
    }
    return m, nil
}
```

### 6.4 wiring 패턴 — ask cmd (비대화)

```go
// internal/cli/commands/ask.go (변경된 부분만)
func runAsk(cmd *cobra.Command, args []string, app *cli.App) error {
    // 1. Read input from args[0] or stdin (CLI-001 §3.1 #3)
    input := readAskInput(cmd, args)

    // 2. REQ-CLIINT-010: dispatcher pre-dispatch on first input
    ctx := cmd.Context()
    sctx := app.Adapter.WithContext(ctx)
    processed, err := app.Dispatcher.ProcessUserInput(ctx, input, sctx)
    if err != nil {
        return fmt.Errorf("dispatcher: %w", err)
    }

    switch processed.Kind {
    case command.ProcessLocal:
        // Print local response (e.g., /help, /version) to stdout, exit 0.
        for _, msg := range processed.Messages {
            fmt.Println(msg)
        }
        return nil
    case command.ProcessExit:
        return nil
    case command.ProcessProceed:
        // Send expanded prompt to daemon ChatStream (CLI-001 REQ-CLI-007).
        return runChatStream(cmd, app, processed.Prompt)
    case command.ProcessAbort:
        return errors.New("aborted")
    }
    return nil
}
```

### 6.5 rootcmd PersistentPreRunE wiring

```go
// internal/cli/rootcmd.go (변경된 부분만)
var (
    flagAliasFile   string
    flagStrictAlias bool
)

func init() {
    rootCmd.PersistentFlags().StringVar(&flagAliasFile, "alias-file", "",
        "Path to alias YAML file (overrides $MINK_ALIAS_FILE)")
    rootCmd.PersistentFlags().BoolVar(&flagStrictAlias, "strict-alias", false,
        "Reject aliases whose canonical model is not registered in ProviderRegistry")
    // ... 기존 CLI-001 의 다른 persistent flag (--config, --daemon-addr, etc.) 그대로
}

var appOnce sync.Once
var appInstance *cli.App

func PersistentPreRunE(cmd *cobra.Command, args []string) error {
    var initErr error
    appOnce.Do(func() {
        cfg := cli.Config{
            AliasFile:   flagAliasFile,
            StrictAlias: flagStrictAlias,
            DaemonAddr:  flagDaemonAddr, // existing CLI-001 flag
            Logger:      buildLogger(),
        }
        appInstance, initErr = cli.New(cfg)
    })
    if initErr != nil {
        return initErr
    }
    // Inject *App into cmd.Context() for child cmds.
    ctx := cli.WithApp(cmd.Context(), appInstance)
    cmd.SetContext(ctx)
    return nil
}
```

### 6.6 의존 SPEC 진척도 별 fallback

| 의존 SPEC | 시점 | 본 SPEC 동작 |
|---------|-----|-----------|
| ALIAS-CONFIG-001 missing | `aliasconfig` 패키지 자체가 없는 경우 | 본 SPEC 컴파일 차단. ALIAS-CONFIG-001 implemented 후 진행. |
| ALIAS-CONFIG-001 implemented, 파일 부재 | runtime | empty map fallback (REQ-CLIINT-007). |
| PERMISSIVE-ALIAS-001 missing | `--strict-alias` flag 가 ALIAS-CONFIG-001 의 LoadOptions.Strict 만 통과 | LoadOptions.Strict 만 적용. PERMISSIVE-ALIAS-001 implemented 후 본 SPEC 0.1.1 patch 로 dual-toggle 활성. |
| CMDLOOP-WIRE-001 missing | runtime | LoopController = nil (옵션 γ default). mutating slash command (`/clear`, `/compact`, `/model`) 는 ErrLoopControllerUnavailable 또는 daemon 위임 (DAEMON-WIRE-001 후속). 본 SPEC 의 current scope 에서는 영향 없음. |
| CLI-001 implemented before this SPEC | precondition | 본 SPEC implementation 진행 가능. |
| CLI-001 still planned | precondition | 본 SPEC implementation 차단. SPEC 자체 작성은 가능 (현재 상태). |

### 6.7 race-clean 보장

- `App` struct: `appOnce sync.Once` 로 단일 초기화 보장. 이후 read-only.
- `app.Adapter`: CMDCTX-001 REQ-CMDCTX-005 (race-clean) 위임.
- `app.Dispatcher`: COMMAND-001 의 race 보장 위임.
- `WithContext(ctx)`: shallow copy + 1 atomic flag pointer 공유 (CMDCTX-001 §6.6) — race-free.
- TUI / ask / session load 의 동시 호출은 각자 독립 child adapter 사용 — 경합 없음.

### 6.8 graceful degradation

- registry 빈 맵 → stderr warn, App.New nil error (REQ-CLIINT-014).
- alias config 로드 실패 → empty map + warn log, App.New nil error (REQ-CLIINT-007).
- transport.Dial 실패 → Client=nil + debug log, App.New nil error. `version`/`ping` 같은 client 미의존 cmd 는 정상 동작. `ask`/`chat` 은 호출 시점에 별도 에러 (CLI-001 REQ-CLI-008).
- nil cfg.Logger → silent (모든 logger 호출이 nil-guarded).
- WithContext(nil) 시도 → 본 SPEC 의 모든 호출 사이트는 cmd.Context() 또는 derived ctx 를 명시적으로 사용 (REQ-CLIINT-016 정적 분석).

### 6.9 TDD 진입 순서 (RED → GREEN → REFACTOR)

| 순서 | 작업 | 검증 AC |
|------|------|------|
| T-001 | `App` struct + `Config` struct 정의 (compile only) | AC-CLIINT-002 |
| T-002 | `App.New` stage 1 (registry) + nil-deps unit test | AC-CLIINT-007, AC-CLIINT-013 |
| T-003 | `App.New` stage 2 (alias map fallback) | AC-CLIINT-008 |
| T-004 | `App.New` stage 3 (adapter) + AliasMap 주입 검증 | AC-CLIINT-003 |
| T-005 | `App.New` stage 4 (dispatcher) | AC-CLIINT-001 |
| T-006 | `App.New` stage 5 (transport client + nil fallback) | AC-CLIINT-013 |
| T-007 | rootcmd PersistentPreRunE + sync.Once | AC-CLIINT-009, AC-CLIINT-014 |
| T-008 | flag 매핑 (`--alias-file` / `--strict-alias`) | AC-CLIINT-010, AC-CLIINT-011 |
| T-009 | TUI handleSubmit dispatcher 호출 (teatest) | AC-CLIINT-004 |
| T-010 | ask cmd dispatcher 호출 (subprocess test) | AC-CLIINT-005, AC-CLIINT-006 |
| T-011 | plan mode indicator 표시 (TUI teatest) | AC-CLIINT-012 |
| T-012 | SIGINT 동작 검증 (RequestClear 미트리거) | AC-CLIINT-016 |
| T-013 | logger debug 호출 검증 (fakeLogger) | AC-CLIINT-018 |
| T-014 | CMDLOOP-WIRE-001 optional wiring (조건부 컴파일 또는 nil-default) | AC-CLIINT-019 |
| T-015 | 정적 분석 — WithContext(nil) / WithContext(context.TODO()) 부재 | AC-CLIINT-015 |

### 6.10 TRUST 5 매핑

| 차원 | 본 SPEC 적용 |
|------|-----------|
| Tested | 19 AC, ≥ 85% coverage (App.New, handleSubmit, ask cmd), TUI teatest harness, ask subprocess test, race detector pass |
| Readable | godoc on `App` struct + New, English code comments per language.yaml |
| Unified | gofmt + golangci-lint clean, viper / cobra 기존 패턴 준수 |
| Secured | nil deps graceful (CMDCTX-001 invariant 위임), --alias-file 경로 traversal 방지 (ALIAS-CONFIG-001 위임), SIGINT 분리 |
| Trackable | conventional commits, SPEC-GOOSE-CMDCTX-CLI-INTEG-001 trailer, MX:ANCHOR on App struct |

### 6.11 의존성 결정 (라이브러리)

기존 자산만 사용. 신규 외부 의존성 없음.
- `github.com/spf13/cobra` (CLI-001 계승)
- `github.com/modu-ai/goose/internal/command` (COMMAND-001)
- `github.com/modu-ai/goose/internal/command/adapter` (CMDCTX-001)
- `github.com/modu-ai/goose/internal/command/adapter/aliasconfig` (ALIAS-CONFIG-001, planned)
- `github.com/modu-ai/goose/internal/llm/router` (ROUTER-001)
- `github.com/modu-ai/goose/internal/cli/transport` (CLI-001)
- `go.uber.org/zap` (CORE-001)
- `github.com/charmbracelet/x/exp/teatest` (CLI-001 의 teatest harness 재사용)
- `github.com/stretchr/testify` (테스트 only)

---

## 7. 의존성 (Dependencies)

| SPEC | status (요청 시점) | 본 SPEC 의 사용 방식 |
|------|--------|----------------|
| SPEC-GOOSE-COMMAND-001 | implemented (PR #50, FROZEN) | `command.Dispatcher`, `SlashCommandContext`, `ProcessUserInput`, `ErrUnknownModel` 자산 그대로 사용. 변경 금지. |
| SPEC-GOOSE-CMDCTX-001 | implemented (PR #52, v0.1.1, FROZEN) | `adapter.New(Options)`, `adapter.WithContext(ctx)`, `adapter.SetPlanMode` 호출. `internal/command/adapter/` 변경 금지. |
| SPEC-GOOSE-CLI-001 | **planned** (v0.2.0) | cobra root + bubbletea TUI + ask cmd 패키지 레이아웃 read-only 가정. 본 SPEC 은 add-on (rootcmd.go, ask.go, tui/update.go, tui/model.go 변경 + app.go 신규). CLI-001 의 REQ / AC / 본문은 변경 금지. **본 SPEC 의 implementation 은 CLI-001 implemented 이후 가능** (R-001). |
| SPEC-GOOSE-CMDLOOP-WIRE-001 | planned (batch A) | `cmdctrl.New(engine, logger)` 시그니처 read-only 참조. 본 SPEC 의 default (옵션 γ) 는 `Options.LoopController = nil` — client-side 에서 omit. CMDLOOP-WIRE-001 implemented 후 옵션 α (RPC wrapper) 변경 가능, 본 SPEC 0.1.1 patch. |
| SPEC-GOOSE-ALIAS-CONFIG-001 | planned (batch A) | `aliasconfig.LoadDefault(LoadOptions)` 호출. ALIAS-CONFIG-001 implemented 이전에는 컴파일 차단. fallback 으로 빈 맵 사용 가능 — 단, aliasconfig 패키지 자체가 없으면 컴파일 실패. **본 SPEC 의 implementation 은 ALIAS-CONFIG-001 implemented 이후 권장** (R-002). |
| SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 | planned (batch A) | strict / lenient toggle 시그니처. `--strict-alias` flag 가 그 toggle 에 매핑. PERMISSIVE-ALIAS-001 부재 시 LoadOptions.Strict 만 활성. |
| SPEC-GOOSE-DAEMON-WIRE-001 | planned (별도) | daemon-side dispatcher / adapter wiring. 본 SPEC 의 옵션 γ 채택 시 mutating slash command 처리는 DAEMON-WIRE-001 가 담당. 본 SPEC 의 client-side 에서는 prompt 형태로 전달만. |

---

## 8. 정합성 / 비기능 요구사항

| 항목 | 기준 |
|------|------|
| 코드 주석 | 영어 (CLAUDE.local.md §2.5) |
| SPEC 본문 | 한국어 |
| 테스트 커버리지 | ≥ 85% (LSP quality gate, run phase). main.go 제외. |
| race detector | App.New + handleSubmit 동시 호출 race-clean (`go test -race -count=10` PASS) |
| golangci-lint | 0 issues |
| gofmt | clean |
| 정적 분석 | `grep -rE 'WithContext\((nil\|context\.TODO\(\))\)' internal/cli/` 결과 0건 (REQ-CLIINT-016) |
| LSP 진단 | 0 errors / 0 type errors / 0 lint errors (run phase) |

---

## 9. Risks (위험 영역)

| ID | 위험 | 영향 | 완화 |
|----|------|------|------|
| R-001 | CLI-001 (planned) 가 본 SPEC implementation 시점에 implemented 되지 않은 경우 | 본 SPEC 의 wiring 위치 (rootcmd / app.go / tui/update.go) 가 무효 | 선행 의존성으로 명시. plan-auditor / ratify 단계에서 CLI-001 status 재확인. CLI-001 implemented 이후 진행. CLI-001 §6.1 패키지 레이아웃 변경 시 본 SPEC 0.1.1 patch + plan-auditor 사이클. |
| R-002 | ALIAS-CONFIG-001 / CMDLOOP-WIRE-001 / PERMISSIVE-ALIAS-001 (모두 planned, batch A) 의 implemented 시점이 본 SPEC 보다 늦을 가능성 | 본 SPEC 의 일부 surface 가 빈 fallback 으로 동작 — 사용자 가시 기능 (`/model opus`, `/clear`) 미동작 | empty map + nil LoopController fallback 으로 컴파일 가능. batch A SPEC 들이 implemented 된 후 본 SPEC 의 wiring 이 자동으로 활성화 (LoadOptions / cmdctrl.New 호출 추가). 본 SPEC 0.1.0 은 fallback 가정으로 작성. |
| R-003 | bubbletea `tea.Program` 의 ctx 라이프사이클이 stream ctx 와 다른 경우 | WithContext(ctx) 호출 시 ctx 가 적절하지 않을 수 있음 — plan mode indicator 잘못 표시 | TUI handleSubmit 에서 stream ctx (Ctrl-C cancellable) 를 명시적으로 derive 하고 WithContext 에 전달. CLI-001 REQ-CLI-009 ctx 일관성 보장. AC-CLIINT-004 가 검증. |
| R-004 | client-side LoopController omit (옵션 γ) 으로 인해 일부 mutating slash command (`/clear`) 가 client-side 에서 ErrLoopControllerUnavailable 반환 | 사용자 UX 후퇴 — `/clear` 가 즉시 동작하지 않음 | DAEMON-WIRE-001 가 daemon-side dispatcher 를 wiring 하면 mutating command 가 prompt 형태로 daemon 에 전달되어 처리. 그 전까지는 ErrLoopControllerUnavailable 명확한 stderr 메시지 + 사용자 가이드 (`run goosed and use /clear in TUI`). |
| R-005 | SIGINT 처리 분리 결정의 사용자 직관 위반 | 일부 사용자가 Ctrl-C 로 컨텍스트 클리어 기대 | 본 SPEC §4.4 REQ-CLIINT-017 에서 명시적 분리. 사용자 결정에 따라 변경 가능 — 후속 SPEC 으로. 본 SPEC 0.1.0 의 default 는 stream cancel only. AC-CLIINT-016 가 검증. |
| R-006 | sync.Once 가 cobra 의 PersistentPreRunE 호출 패턴과 호환되지 않을 가능성 (subcommand 별 PreRun 호출 등) | App 이 중복 초기화 또는 미초기화 | sync.Once 는 process-global. cobra 의 PreRun chain 은 root → subcommand 순. PersistentPreRunE 는 cmd 트리 전체에서 1회 (parent 의 PersistentPreRunE 가 child 에서 다시 실행되지 않는 cobra 동작). AC-CLIINT-014 가 dual subcommand invocation 시나리오로 검증. |
| R-007 | `App.Client` 가 nil (transport.Dial 실패) 인 상태에서 `ask`/`chat` cmd 호출 | 런타임 nil pointer panic | `runAsk` / `runChat` 진입 시 nil check 후 CLI-001 REQ-CLI-008 의 exit 69 분기. 본 SPEC §6.8 graceful degradation 명시. |

---

## 10. Exclusions (What NOT to Build)

본 SPEC 은 다음을 **수행하지 않는다**. 각 항목은 후속 SPEC 또는 명시적 결정으로 분리.

1. **daemon mode wiring (daemon-side ContextAdapter / Dispatcher 인스턴스화)**
   - 위임: SPEC-GOOSE-DAEMON-WIRE-001 (별도 SPEC)
   - 본 SPEC 은 client-side (`cmd/goose/`, `internal/cli/`) 만 다룬다. `cmd/goosed/` 와 `internal/daemon/` 의 wiring 은 별도.

2. **interactive prompt 디자인 / 키바인딩 / 렌더링**
   - 위임: SPEC-GOOSE-CLI-001
   - 본 SPEC 은 bubbletea 의 textarea / viewport / statusbar 컴포넌트 자체를 변경하지 않는다. handleSubmit 의 dispatcher 호출 라인 + Model 의 `app *App` 필드만 추가.

3. **bubbletea TUI 자체의 구현**
   - 위임: SPEC-GOOSE-CLI-001
   - 본 SPEC 의 변경 범위는 update.go 의 handleSubmit 함수 내 dispatcher 호출 + model.go 의 필드 1개. tui/view.go / tui/keybindings.go / tui/stream.go 등은 변경 금지.

4. **LoopController 의 client-side 구현 (옵션 α — proto 확장)**
   - 위임: 후속 SPEC (SPEC-GOOSE-CLI-LOOPCTL-RPC-001 가칭)
   - 본 SPEC 의 default 는 옵션 γ (omit). 사용자가 옵션 α 를 원하면 본 SPEC 0.2.0 또는 별도 SPEC 으로 분기.

5. **proto 확장 (`AgentService/Clear`, `AgentService/Compact`, `AgentService/SwitchModel` RPC)**
   - 위임: SPEC-GOOSE-CLI-001 (proto 확장 책임) + SPEC-GOOSE-CLI-LOOPCTL-RPC-001 (가칭)
   - 본 SPEC 은 proto 변경하지 않는다.

6. **SIGINT (Ctrl-C) → LoopController.RequestClear trigger**
   - 결정: 분리 (REQ-CLIINT-017, AC-CLIINT-016). SIGINT 의 의미는 stream cancel only (CLI-001 REQ-CLI-009).
   - 사용자가 명시적 의미 통합을 원하면 후속 SPEC 또는 본 SPEC 0.2.0.

7. **alias config 파일 hot-reload (파일 watch → 런타임 map 교체)**
   - 위임: SPEC-GOOSE-HOTRELOAD-001 (작성 중)
   - 본 SPEC 은 process start 시 1회 로드만.

8. **`/plan` slash command 자체 (SetPlanMode trigger 의 사용자 진입점)**
   - 위임: SPEC-GOOSE-COMMAND-001 후속 (가칭 SPEC-GOOSE-CMD-PLAN-001)
   - 본 SPEC 은 SetPlanMode 호출 사이트만 (TUI handleSubmit 에서 dispatcher 가 SetPlanMode 위임 대상으로 호출 가능). `/plan` 명령 자체의 정의는 별도.

9. **CLI-001 의 REQ / AC / 본문 변경**
   - 본 SPEC 은 add-on. CLI-001 본문 변경 금지. CLI-001 §6.5 의 stub `tuiSlashContext` 가 본 SPEC 에 의해 대체된다는 사실은 CLI-001 implemented 시점에 자연스럽게 (CLI-001 의 stub 명시 덕분에) 적용 가능.

10. **CMDCTX-001 의 ContextAdapter / Options / WithContext / SetPlanMode 의 본체 변경**
    - 본 SPEC 은 호출만 한다. CMDCTX-001 (FROZEN) 변경 금지.

11. **COMMAND-001 의 Dispatcher / SlashCommandContext / ProcessUserInput 의 본체 변경**
    - 본 SPEC 은 호출만 한다. COMMAND-001 (FROZEN) 변경 금지.

12. **session load / resume 시 dispatcher 첫 호출**
    - 결정: dispatcher 우회 (resume 은 historical messages 재생, user input 이 아님). AC-CLIINT-005 의 적용 대상 외. 후속 SPEC 에서 변경 가능.

13. **`--alias-file` 의 경로 traversal 방지 / YAML 파싱 보안**
    - 위임: ALIAS-CONFIG-001 (Loader 의 책임)
    - 본 SPEC 은 flag 값을 그대로 LoadOptions.AliasFile 에 전달. 검증은 ALIAS-CONFIG-001 의 의무.

---

## 11. 참조 (References)

- `.moai/specs/SPEC-GOOSE-CLI-001/spec.md` v0.2.0 §6.1 (패키지 레이아웃), §6.5 (slash command 프리-디스패치 — `tuiSlashContext` stub 본 SPEC 이 대체)
- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` v0.1.1 §6.2 (Options struct), §6.3 (의존성 주입 패턴), §6.5 (PlanModeActive 결합)
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` (implemented, PR #50) — Dispatcher / SlashCommandContext 인터페이스
- `.moai/specs/SPEC-GOOSE-CMDLOOP-WIRE-001/spec.md` (planned, batch A) §6.2 (cmdctrl.New 시그니처)
- `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` (planned, batch A) §4.1 (Loader, LoadDefault)
- `.moai/specs/SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001/spec.md` (planned, batch A) — strict / lenient toggle
- `.moai/specs/SPEC-GOOSE-DAEMON-WIRE-001/spec.md` (planned 별도) — daemon-side wiring (옵션 γ 의 mutating 명령 처리)
- `CLAUDE.local.md §2.5` — 코드 주석 영어 정책

---

## 12. Acceptance Criteria

### AC-CLIINT-001 — 단일 인스턴스 invariant

`App.New` 가 process 내 다중 호출되어도 `app1.Adapter == app2.Adapter` 그리고 `app1.Dispatcher == app2.Dispatcher` 임 (sync.Once 의 idempotent 보장).

검증 방법: unit test — `appOnce.Do(...)` 후 두 번째 `cli.New(cfg)` 호출이 첫 번째 결과를 반환 (또는 PersistentPreRunE 직접 호출 시뮬레이션).

### AC-CLIINT-002 — App struct 컴파일 단언

`internal/cli/app.go` 컴파일 성공. `var _ struct {
    Adapter *adapter.ContextAdapter
    Dispatcher *command.Dispatcher
    Client *transport.Client
} = cli.App{}` 같은 type-shape 단언 성립.

검증 방법: `go build ./internal/cli/...` 에러 없음 + 코드 리뷰.

### AC-CLIINT-003 — App.New 4단계 순서 (registry → alias → adapter → dispatcher)

`App.New(cfg)` 호출 시 (a) `router.DefaultRegistry()` 가 먼저 평가되고, (b) `aliasconfig.LoadDefault(...)` 가 그 registry 를 인자로 받고, (c) `adapter.New(Options{Registry, AliasMap})` 가 위 두 결과를 사용, (d) `command.NewDispatcher(registry, ...)` 가 같은 registry 를 사용. 호출 순서를 trace fake (callOrder slice) 로 검증.

검증 방법: unit test with mocked stages.

### AC-CLIINT-004 — TUI handleSubmit dispatcher 호출

bubbletea Model 의 handleSubmit 호출 시 (a) `app.Adapter.WithContext(streamCtx)` 가 정확히 1회 호출되고 그 결과 child adapter 가 (b) `app.Dispatcher.ProcessUserInput(streamCtx, line, child)` 의 sctx 인자로 전달됨. teatest harness 로 검증.

검증 방법: teatest — `m.input.SetValue("hello"); m.handleSubmit()` 후 fakeDispatcher.LastCall.SCtx 가 child adapter 인지 확인 (parent 와 ID 다른 인스턴스).

### AC-CLIINT-005 — ask cmd dispatcher 호출 (인자 모드)

`mink ask "hello"` subprocess 실행 시 dispatcher.ProcessUserInput 이 1회 호출되고 input == "hello", sctx 는 `app.Adapter.WithContext(askCtx)` 의 결과. ProcessProceed 결과 시 daemon ChatStream 으로 전달.

검증 방법: subprocess test (`os/exec.Cmd` + stub dispatcher with call recorder).

### AC-CLIINT-006 — ask cmd dispatcher 호출 (--stdin 모드)

`echo "hello" | mink ask --stdin` subprocess 실행 시 dispatcher.ProcessUserInput 이 1회 호출되고 input == "hello".

검증 방법: subprocess test with stdin pipe.

### AC-CLIINT-007 — nil deps graceful

`App.New(Config{Logger: nil, AliasFile: "/nonexistent.yaml", DaemonAddr: "127.0.0.1:0"})` 호출 시 (a) panic 없음, (b) `app.Adapter != nil`, (c) `app.Dispatcher != nil`, (d) `app.Client == nil` (Dial 실패), (e) `app.Adapter.AliasMap` 은 빈 맵 (alias config 로드 실패 fallback).

검증 방법: unit test.

### AC-CLIINT-008 — alias config 로드 실패 fallback

`aliasconfig.LoadDefault` 가 error 반환 (모킹) 시 `App.New` 는 nil error 반환, `app.Adapter` 의 alias map 은 빈 맵, fakeLogger.WarnCount >= 1 (config load failed warn).

검증 방법: unit test with mocked aliasconfig.LoadDefault.

### AC-CLIINT-009 — PersistentPreRunE 1회 호출

cobra root 가 두 개의 subcommand (`ping`, `version`) 를 가질 때, 각각 호출 시 PersistentPreRunE 의 `appOnce.Do(...)` 내부 함수가 process 전체에서 1회만 실행됨.

검증 방법: integration test — subprocess 내 ping 후 version 연속 호출 (단일 process 시뮬레이션은 unit test 로, sync.Once 검증).

### AC-CLIINT-010 — `--alias-file` flag 매핑

`goose --alias-file=/tmp/test.yaml ping` 실행 시 `aliasconfig.LoadDefault` 가 호출될 때 `LoadOptions.AliasFile == "/tmp/test.yaml"`.

검증 방법: subprocess test with stub aliasconfig (build tag) 또는 unit test with App.New 모킹.

### AC-CLIINT-011 — `--strict-alias` flag 매핑

`goose --strict-alias ping` 실행 시 `aliasconfig.LoadDefault` 의 `LoadOptions.Strict == true`.

검증 방법: 위와 동일 패턴.

### AC-CLIINT-012 — plan mode indicator (TUI)

`app.Adapter.SetPlanMode(true)` 호출 후 TUI Model 의 statusbar 또는 input prompt 에 plan mode indicator (예: `[plan]`) 표시. teatest snapshot 비교.

검증 방법: teatest — SetPlanMode(true) 후 `m.View()` 에 "[plan]" substring 존재.

### AC-CLIINT-013 — empty registry warn

`router.DefaultRegistry().Empty() == true` 인 상태에서 `App.New(Config{})` 호출 시 stderr 에 `goose: provider registry is empty` 메시지 + App.New nil error.

검증 방법: unit test — empty registry mock + capture stderr.

### AC-CLIINT-014 — App.New idempotent (sync.Once 검증)

두 goroutine 에서 동시에 PersistentPreRunE 시뮬레이션 (`appOnce.Do(initFn)` 동시 호출) 시 initFn 은 정확히 1회 실행. 두 goroutine 의 결과는 동일 `*App` 포인터.

검증 방법: race test — 100 goroutines × 동시 PersistentPreRunE 호출 → fakeInitCounter == 1.

### AC-CLIINT-015 — WithContext(nil) 부재 정적 분석

`grep -rE 'WithContext\((nil|context\.TODO\(\))\)' internal/cli/ --include='*.go' | grep -v '_test.go'` 결과 0건.

검증 방법: CI 정적 분석 단계 + 코드 리뷰.

### AC-CLIINT-016 — SIGINT → RequestClear 미트리거

TUI 에서 Ctrl-C 입력 후 `fakeLoopController.RequestClearCallCount == 0`. stream cancel + viewport `[aborted]` 만 발생 (CLI-001 REQ-CLI-009).

검증 방법: teatest — Ctrl-C 키 이벤트 → fakeLoopController 카운터 검증.

### AC-CLIINT-017 — App.New panic propagate

`App.New` 내 stage 3 (adapter.New) 가 panic 을 일으키도록 모킹 (예: nil Logger interface 가 아닌 nil concrete type). panic 이 process exit 으로 전파됨 (recover 없음).

검증 방법: unit test — `defer func() { recover() }()` 로 panic 을 잡아 검증.

### AC-CLIINT-018 — logger debug 호출

`fakeLogger` 주입 후 `App.New(cfg)` 호출 → fakeLogger.DebugCount >= 1 (stage 진행 로그). 이어 `app.Dispatcher.ProcessUserInput(...)` 호출 → fakeLogger.DebugCount 가 추가 증가.

검증 방법: unit test with fake zap logger.

### AC-CLIINT-019 — CMDLOOP-WIRE-001 optional wiring

`Options.LoopController = nil` (옵션 γ default) 으로 App.New 호출 시 정상 동작. CMDLOOP-WIRE-001 implemented 후 `Options.LoopController = cmdctrl.New(...)` 로 변경 시에도 동일 컴파일 + 동일 dispatcher 호출 패턴.

검증 방법: 두 build tag (`!cmdloop` / `cmdloop`) 또는 nil/non-nil 인자 분기 test.

---

Version: 0.1.0
Last Updated: 2026-04-27
REQ coverage: REQ-CLIINT-001 ~ REQ-CLIINT-022 (총 22)
AC coverage: AC-CLIINT-001 ~ AC-CLIINT-019 (총 19)
