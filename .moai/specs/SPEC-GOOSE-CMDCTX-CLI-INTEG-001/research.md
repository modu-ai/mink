# SPEC-GOOSE-CMDCTX-CLI-INTEG-001 — Research

> 본 SPEC 은 PR #50 / #52 (COMMAND-001, CMDCTX-001) 머지로 정의된 `command.Dispatcher` + `adapter.ContextAdapter` 와 SPEC-GOOSE-CLI-001 의 사용자 대면 CLI 진입점을 wiring 하는 **integration SPEC** 이다. 본 research.md 는 SPEC-GOOSE-CLI-001 (planned, FROZEN-by-reference) 의 cobra/bubbletea 구조를 read-only 로 분석하고, ContextAdapter 인스턴스화 시점, dispatcher 주입 패턴, 매 user input 별 `WithContext(ctx)` 자식 어댑터 사용 패턴, alias config 와의 결합, interactive vs 비대화 모드 분기를 정리한다.

---

## 1. 문제 정의

### 1.1 현재 상태 (PR #52 머지 직후)

| 항목 | 상태 | 출처 |
|------|------|------|
| `command.Dispatcher` (`ProcessUserInput(ctx, input, sctx)`) | implemented (FROZEN) | PR #50 SPEC-GOOSE-COMMAND-001 |
| `command.SlashCommandContext` 인터페이스 | implemented (FROZEN) | `internal/command/context.go` |
| `adapter.ContextAdapter` (`SlashCommandContext` 구현) | implemented (FROZEN) | `internal/command/adapter/adapter.go` (PR #52) |
| `adapter.LoopController` 인터페이스 | implemented (FROZEN, 정의만) | `internal/command/adapter/controller.go:19-51` |
| `cmdctrl.LoopControllerImpl` (`LoopController` 구현체) | **planned** | SPEC-GOOSE-CMDLOOP-WIRE-001 (batch A) |
| `aliasconfig.LoadDefault()` (alias 데이터 소스) | **planned** | SPEC-GOOSE-ALIAS-CONFIG-001 (batch A) |
| `cmdctx.PermissiveAlias` 모드 (strict / lenient) | **planned** | SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 (batch A) |
| `cmd/goose/main.go` cobra root + `internal/cli/` | **planned** | SPEC-GOOSE-CLI-001 v0.2.0 |
| **CLI 진입점에서 ContextAdapter instantiate + dispatcher 주입** | **missing** | 본 SPEC 이 채울 빈 자리 |

핵심 결손: `cmd/goose/main.go` 의 cobra root 에서 (a) `adapter.New(Options{...})` 를 호출해서 단일 ContextAdapter 인스턴스를 만들고, (b) `command.NewDispatcher(...)` 를 호출해서 dispatcher 를 만들고, (c) interactive (TUI) / 비대화 (`ask`, `ask --stdin`) 양쪽 모드에서 user input 마다 `dispatcher.ProcessUserInput(ctx, input, adapter.WithContext(ctx))` 를 호출하는 wiring 코드가 없다.

### 1.2 본 SPEC 의 단일 책임

CLI-001 의 cobra root + bubbletea TUI 구조 (FROZEN-by-reference) 의 진입점에 **adapter + dispatcher wiring 어드온** 을 더한다. CLI-001 의 본문 (REQ-CLI-001 ~ REQ-CLI-025, AC-CLI-001 ~ AC-CLI-016) 은 변경하지 않는다. 본 SPEC 은 CLI-001 의 슈퍼셋이 아니라 **wiring 어드온** 이다.

---

## 2. 의존 SPEC 의 wiring surface 분석

### 2.1 SPEC-GOOSE-CLI-001 (planned) 진입 지점

CLI-001 §6.1 (제안 패키지 레이아웃):

```
cmd/
├── goose/main.go                # ~15 LoC
└── goosed/main.go               # ~40 LoC

internal/cli/
├── rootcmd.go                   # cobra root + persistent flags
├── commands/
│   ├── ask.go                   # unary / stdin (비대화)
│   ├── chat.go                  # → tui.Run (interactive)
│   ├── ping.go
│   ├── version.go
│   ├── session.go
│   ├── config.go
│   ├── tool.go
│   ├── plugin.go
│   └── daemon.go
├── tui/
│   ├── model.go                 # bubbletea Model struct
│   ├── update.go                # tea.Msg dispatch + handleSubmit
│   ├── view.go
│   ├── stream.go
│   └── keybindings.go
├── transport/
│   ├── client.go                # Connect-Go client factory
│   ├── dial.go
│   └── stream.go
└── session/
    └── file.go                  # jsonl read/write
```

본 SPEC 이 추가/수정 대상으로 보는 위치:

- `cmd/goose/main.go`: 변경 없음 (~15 LoC, `cli.Execute()` 호출).
- `internal/cli/rootcmd.go`: **단일 진입점**. cobra `PersistentPreRunE` 에서 ContextAdapter + Dispatcher 1회 생성, 자식 cmd 에 전달.
- `internal/cli/app.go` (신규 권장): `App` struct 가 adapter / dispatcher / client 를 보유. 자식 cmd 가 `*App` 를 통해 wiring 의존성 접근.
- `internal/cli/tui/update.go` `handleSubmit`: 매 user input 마다 `dispatcher.ProcessUserInput(ctx, line, app.Adapter.WithContext(ctx))`. CLI-001 §6.5 의 `m.sctx()` 는 `tuiSlashContext` stub 인데, 본 SPEC 은 이를 `app.Adapter.WithContext(ctx)` 호출로 대체한다 (CLI-001 본문은 stub 이라 명시).
- `internal/cli/commands/ask.go`: 비대화 모드. `--stdin` 또는 인자 message 에 대해 (CLI-001 REQ-CLI-007) prompt 본체는 daemon 으로 전송되지만, 시작 시 `/clear` 같은 leading slash command 가 들어올 수 있는지는 CLI-001 §6.5 에 명시되지 않음 → 본 SPEC 은 **`ask` 모드에서도 dispatcher.ProcessUserInput 을 첫 라인에 적용** 하여 `/clear`, `/model alias`, `/exit` 같은 local-only 명령은 네트워크 호출 없이 처리 (CLI-001 REQ-CLI-021 일관성).

### 2.2 SPEC-GOOSE-CMDCTX-001 (implemented, FROZEN, v0.1.1) 자산

CMDCTX-001 §6.2 가 노출하는 surface (변경 금지):

```go
// internal/command/adapter/adapter.go
type Options struct {
    Registry       *router.ProviderRegistry
    LoopController LoopController
    AliasMap       map[string]string
    GetwdFn        func() (string, error)
    Logger         Logger
}

func New(opts Options) *ContextAdapter
func (a *ContextAdapter) SetPlanMode(active bool)
func (a *ContextAdapter) WithContext(ctx context.Context) *ContextAdapter
```

본 SPEC 은 이 3개 메서드만 사용한다. `New(Options)` 1회 (애플리케이션 lifetime 단위), `WithContext(ctx)` 매 user input 별, `SetPlanMode` 는 plan mode 진입 시 (후속 SPEC `/plan` slash command 가 trigger).

### 2.3 SPEC-GOOSE-CMDLOOP-WIRE-001 (planned, batch A) 의 surface

CMDLOOP-WIRE-001 §6.2 (옵션 C 권장):

```go
// internal/query/cmdctrl/controller.go
package cmdctrl

func New(engine *query.QueryEngine, logger *zap.Logger) *LoopControllerImpl
// var _ adapter.LoopController = (*LoopControllerImpl)(nil)
```

본 SPEC 은 이 `cmdctrl.New(engine, logger)` 의 반환 값을 `adapter.New(Options{LoopController: ...})` 에 주입한다. CMDLOOP-WIRE-001 의 status 가 planned 이므로 **본 SPEC 의 implementation 은 CMDLOOP-WIRE-001 implemented 이후로 차단된다** (R-001).

### 2.4 SPEC-GOOSE-ALIAS-CONFIG-001 (planned, batch A) 의 surface

ALIAS-CONFIG-001 §4.1:

```go
// internal/command/adapter/aliasconfig/loader.go
package aliasconfig

type LoadOptions struct {
    AliasFile string  // GOOSE_ALIAS_FILE env or --alias-file flag override
    Strict    bool    // --strict-alias flag override
    Registry  *router.ProviderRegistry
    Logger    adapter.Logger
}

func Load(path string) (map[string]string, error)
func LoadDefault(opts LoadOptions) (map[string]string, error)
func Validate(m map[string]string, registry *router.ProviderRegistry, strict bool) error
```

본 SPEC 은 `LoadDefault(opts)` 의 반환 map 을 `adapter.New(Options{AliasMap: ...})` 에 주입한다. ALIAS-CONFIG-001 의 status 가 planned 이므로 본 SPEC 도 차단된다 (R-002). 단, 본 SPEC 은 ALIAS-CONFIG-001 부재 시 `nil` 또는 `map[string]string{}` 빈 맵 fallback 으로 진행 가능 (REQ-CMDCTX-017 가 alias map 을 optional 로 명시).

### 2.5 SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 (planned, batch A) 의 surface

PERMISSIVE-ALIAS-001 은 ResolveModelAlias 의 strict / lenient mode 토글을 추가한다. 본 SPEC 은 그 토글을 **CLI flag (`--strict-alias` 또는 config 파일)** 로 노출하는 책임을 진다. 이는 ALIAS-CONFIG-001 의 `LoadOptions.Strict` 와 통합된다.

---

## 3. 매 user input 별 wiring 패턴

### 3.1 CMDCTX-001 §6.3 의 권장 패턴 (한 줄 요약)

```go
// CMDCTX-001 §6.3 pseudocode
adapter := adapter.New(adapter.Options{...})
dispatcher := command.NewDispatcher(reg, command.Config{}, logger)
// every user input:
dispatcher.ProcessUserInput(ctx, input, adapter.WithContext(ctx))
```

→ 본 SPEC 은 이 패턴을 **CLI-001 의 4개 user-input 진입점** 에 일관되게 적용:

| 진입점 | 위치 | input 소스 |
|------|-----|---------|
| TUI handleSubmit | `internal/cli/tui/update.go` | textarea Enter |
| ask cmd 인자 | `internal/cli/commands/ask.go` | cobra Args[0] |
| ask --stdin | `internal/cli/commands/ask.go` | os.Stdin pipe |
| session load (resume) | `internal/cli/commands/session.go` | jsonl 첫 user message (조건부 — 보통 resume 은 dispatcher 우회) |

### 3.2 ctx 소스

- **TUI**: `cmd.Context()` 에서 derive 한 `streamCtx` (Ctrl-C cancellable, REQ-CLI-009).
- **ask**: `cmd.Context()` 에서 derive 한 `askCtx` (CLI-001 REQ-CLI-007 의 30초 timeout, `--timeout` override).
- **session load**: `cmd.Context()`.

### 3.3 WithContext 의 역할

`adapter.WithContext(ctx)` 는 child adapter 를 반환 (CMDCTX-001 §6.5):

- `planMode *atomic.Bool` 는 부모와 공유 → `SetPlanMode(true)` 가 모든 자식에서 즉시 visible.
- `ctxHook = ctx` 로 child adapter 의 `PlanModeActive()` 가 ctx 의 `TeammateIdentity.PlanModeRequired` 를 참조 가능.
- registry, loopCtrl, aliasMap, logger, getwdFn 는 부모와 공유.

→ **본 SPEC 은 매 user input 마다 새 child adapter 를 만든다**. 이 child 는 일회용. shallow copy + 1 atomic flag 공유이므로 비용 낮음 (~10ns per call).

---

## 4. ContextAdapter 인스턴스화 시점

### 4.1 후보 위치 비교

| 위치 | 장점 | 단점 |
|------|-----|-----|
| A: `cmd/goose/main.go` | 단일 진입점, ~15 LoC 유지 | cobra cmd 트리 진입 전이라 flag 파싱 미완료 — `--alias-file`, `--strict-alias` 등 영향 |
| B: `internal/cli/rootcmd.go` `PersistentPreRunE` | flag 파싱 완료 후 1회 실행, 모든 자식 cmd 가 보장된 adapter 보유 | rootcmd 의존성이 폭증할 수 있음 — `App` struct 분리로 완화 |
| C: 각 cmd 에서 lazy init | 필요한 cmd 만 adapter 생성 (예: `version`, `ping` 은 불필요) | cobra 의 lifecycle 에 lazy init 매번 호출 → registry / cmdctrl 도 매번 인스턴스화. 과도한 비용 |

→ **B 권장**. `App` struct (`internal/cli/app.go` 신규) 가 adapter / dispatcher / client 를 보유하고 `rootcmd.PersistentPreRunE` 가 1회 초기화. `version`, `ping`, `tool list` 같은 가벼운 cmd 는 `App.Adapter` 를 참조만 하고 호출 안 함 (no-op).

### 4.2 인스턴스 lifetime

- **App lifetime**: process start → process exit. 단일 인스턴스.
- **TUI 모드**: bubbletea `tea.Program` 가 종료된 후에도 adapter 는 살아 있을 수 있으나 대부분 즉시 process exit.
- **ask 모드**: stream 종료 후 process exit 직전까지 adapter 살아 있음.
- **planMode flag**: process 내내 단일 진실 공급원. TUI 에서 `/plan` 입력 → `app.Adapter.SetPlanMode(true)` 호출 → 모든 child adapter (handleSubmit 마다 생성) 가 즉시 관찰.

### 4.3 nil 의존성에 대한 graceful degradation

CMDCTX-001 REQ-CMDCTX-014 / REQ-CMDCTX-015:
- nil registry → `ResolveModelAlias` 항상 `ErrUnknownModel` 반환, panic 없음.
- nil loopCtrl → `OnClear/OnCompactRequest/OnModelChange` → `ErrLoopControllerUnavailable` 반환, panic 없음.

본 SPEC 의 책임: **CLI 진입점에서 registry 와 loopCtrl 가 nil 이지 않도록 wiring**. 단,
- registry 부재 (router.DefaultRegistry() 자체가 빈 맵) 시: 명확한 stderr 메시지 + exit 1 (CLI 본인의 의무, REQ-CLI-002 prefix `goose:`).
- loopCtrl 부재 (CMDLOOP-WIRE-001 미구현 시): `ask` / `chat` 모드에서 CMDLOOP-WIRE-001 implemented 후 활성화 메시지 출력 + 일부 명령은 dispatcher 우회 (예: `/help`, `/exit` 은 local-only 로 동작 가능).

---

## 5. interactive (TUI) vs 비대화 (ask) 모드 분기

### 5.1 TUI 모드 — bubbletea handleSubmit 통합

CLI-001 §6.5 의 `m.sctx()` 는 stub (`tuiSlashContext`). 본 SPEC 은 stub 을 **`app.Adapter.WithContext(streamCtx)`** 호출로 대체:

```go
// internal/cli/tui/update.go (intent — full code in spec.md §6.3)
func (m Model) handleSubmit() (Model, tea.Cmd) {
    line := m.input.Value()
    if line == "" { return m, nil }

    // ctx is the stream context (Ctrl-C cancellable)
    ctx := m.streamCtx

    // Build child adapter with this ctx; expose SlashCommandContext
    sctx := m.app.Adapter.WithContext(ctx)

    processed, err := m.app.Dispatcher.ProcessUserInput(ctx, line, sctx)
    // ... CLI-001 §6.5 의 switch processed.Kind 그대로
}
```

### 5.2 비대화 모드 (`ask`, `ask --stdin`)

- **input 이 `/` 로 시작하지 않는 경우** (일반 prompt): dispatcher 우회 가능 — 직접 daemon `ChatStream` 으로 전송. 단, **slash 분기 없는 단일 흐름 일관성** 을 위해 본 SPEC 은 dispatcher 를 **항상 호출** 하도록 권장. 일반 prompt 는 dispatcher 가 `ProcessProceed` 로 분기하여 expanded prompt 를 daemon 으로 전송 (CLI-001 REQ-CLI-010 의 일반화).
- **input 이 `/` 로 시작하는 경우**: dispatcher 가 local 처리 (`/clear`, `/help`, `/version`, `/exit`) 또는 `ProcessProceed` 분기. 비대화 모드에서 `/clear` 같은 mutating local command 는 stdout 에 `[system] context cleared` 출력 후 exit 0.

### 5.3 readline / TTY 감지

- **TTY 감지**: `golang.org/x/term.IsTerminal(int(os.Stdin.Fd()))` 로 판정. `goose` (인자 없음) + TTY → TUI. `goose` (인자 없음) + non-TTY → stdin pipe 로 ask 모드 fallback (CLI-001 REQ-CLI-006 의 명시적 변형).
- **readline prompt prefix**: plan mode 활성 시 prompt 변경 (REQ §4.3 State-Driven 에서 정의). bubbletea `textarea` 의 prompt 필드 또는 statusbar 에 `[plan]` indicator 표시.

### 5.4 SIGINT (Ctrl+C) 처리 결정

**핵심 질문**: SIGINT 가 활성 stream 을 cancel 하는 것 외에 (CLI-001 REQ-CLI-009), `LoopController.RequestClear` 도 trigger 해야 하는가?

분석:
- **NO 권장 (본 SPEC 의 default)**: Ctrl-C 의 직관 = "이번 turn 만 중단", 컨텍스트 보존. RequestClear 는 `/clear` slash command 의 명시적 의미. 둘을 결합하면 사용자 의도 위반.
- 사용자가 명시적으로 `/clear` 입력 → dispatcher 가 OnClear 호출 → adapter 가 LoopController.RequestClear 위임. 이 경로만 활성.
- Ctrl-C → CLI-001 REQ-CLI-009 그대로 (stream cancel + viewport `[aborted]`).

본 SPEC 은 SIGINT 와 LoopController.RequestClear 를 **연결하지 않는다** (Exclusions §10).

---

## 6. alias config 와의 결합 (ALIAS-CONFIG-001 통합)

### 6.1 데이터 흐름

```
process start
  ├─ rootcmd parses flags (--alias-file, --strict-alias)
  ├─ aliasconfig.LoadDefault(LoadOptions{
  │     AliasFile: viper.GetString("alias-file"),  // env GOOSE_ALIAS_FILE 우선
  │     Strict:    viper.GetBool("strict-alias"),
  │     Registry:  router.DefaultRegistry(),
  │     Logger:    zapAdapter,
  │  })
  │     → returns map[string]string (or empty map on error)
  │
  ├─ adapter.New(Options{
  │     Registry:       router.DefaultRegistry(),
  │     LoopController: cmdctrl.New(engine, logger),
  │     AliasMap:       aliasMap,
  │     Logger:         zapAdapter,
  │  })
  │
  └─ App ready
```

### 6.2 ALIAS-CONFIG-001 부재 시 fallback

ALIAS-CONFIG-001 이 implemented 되지 않은 시점에 본 SPEC 만 implemented 되면:
- `aliasconfig.LoadDefault` 함수 자체가 부재 → 컴파일 실패.
- 우회: `Options.AliasMap = nil` 또는 빈 맵 으로 진행. CMDCTX-001 의 `resolveAlias` (FROZEN) 가 alias 미발견 시 canonical lookup fallback 하므로 `provider/model` 형태 입력은 정상 동작, alias 형태 입력 (`opus`) 만 `ErrUnknownModel`.

→ 본 SPEC 의 implementation 은 ALIAS-CONFIG-001 implemented 이후 진행 권장 (R-002). 그 전에는 빈 맵 fallback 으로 컴파일/test 가능.

### 6.3 `--alias-file` flag 와 `--strict-alias` flag

CLI flag 추가 (rootcmd.go):

```go
rootCmd.PersistentFlags().String("alias-file", "", "Path to alias YAML file (overrides $GOOSE_ALIAS_FILE)")
rootCmd.PersistentFlags().Bool("strict-alias", false, "Reject aliases not registered in ProviderRegistry")
```

→ 본 SPEC 의 REQ §4.5 Optional 에서 명시.

---

## 7. CLI app struct 설계 (제안)

```go
// internal/cli/app.go (신규, 본 SPEC 산출물)
package cli

import (
    "github.com/modu-ai/goose/internal/cli/transport"
    "github.com/modu-ai/goose/internal/command"
    "github.com/modu-ai/goose/internal/command/adapter"
)

// App holds the wiring dependencies for the goose CLI.
// Single instance per process, initialized in rootcmd.PersistentPreRunE.
//
// @MX:ANCHOR: CLI wiring root.
// @MX:REASON: Single source of truth for adapter, dispatcher, and transport client.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-CLI-INTEG-001
type App struct {
    Adapter    *adapter.ContextAdapter
    Dispatcher *command.Dispatcher
    Client     *transport.Client
    Logger     *zap.Logger
}

func New(cfg Config) (*App, error) {
    // 1. registry from router.DefaultRegistry()
    // 2. transport client (Connect-Go) — CLI-001 §6.3
    // 3. cmdctrl.New(engine ref, logger) — but engine lives in daemon, so client-side controller is a stub or omitted
    //    (see §8 below)
    // 4. aliasconfig.LoadDefault(...) — ALIAS-CONFIG-001
    // 5. adapter.New(Options{...})
    // 6. command.NewDispatcher(registry, command.Config{}, logger)
}
```

### 7.1 client-side LoopController 의 특수성

CLI 는 client-side. `LoopController.RequestClear` 등은 daemon 의 query loop 에 영향을 줘야 한다. CLI 가 직접 `cmdctrl.New(engine, logger)` 를 호출할 수 없다 — engine 은 daemon-side 객체.

**해결책 옵션** (사용자 결정 보류):

| 옵션 | 설명 | 장단점 |
|------|------|------|
| α | client-side LoopController 는 daemon 의 RPC (예: `AgentService/Clear`, `AgentService/Compact`, `AgentService/SwitchModel`) 를 호출하는 wrapper | proto 확장 필요 (CLI-001 §6.2 의 agent.proto 추가). depth 큼. |
| β | client-side 에서 dispatcher 가 local-only 명령만 처리 (`/help`, `/exit`, `/version`); mutating 명령 (`/clear`, `/compact`, `/model`) 은 daemon 으로 전달 → daemon-side dispatcher 가 처리 | dispatcher 가 client+daemon 양쪽에 존재. 책임 분리 명확. CLI-001 REQ-CLI-010 의 자연스러운 확장. |
| γ | DAEMON-WIRE-001 (별도 SPEC) 에서 daemon-side 진입점 wiring 처리, CLI-001 은 local-only 명령만 (β의 변형) | 본 SPEC 범위가 client-side wiring 으로만 좁아짐 — depth 작음. 권장. |

→ **옵션 γ 권장**. 본 SPEC 은 **client-side wiring** 만 담당:
- CLI 의 dispatcher 는 local-only 명령 (`/help`, `/exit`, `/version`, `/status`) 처리.
- mutating 명령 (`/clear`, `/compact`, `/model`) 은 dispatcher 의 `ProcessProceed` 로 분기되어 daemon 으로 prompt 형태로 전달, daemon-side dispatcher 가 처리 (DAEMON-WIRE-001 책임).
- 결과적으로 CLI-side ContextAdapter 의 `LoopController` 는 **stub** (모든 메서드가 nil 반환 또는 RPC wrapper) 이거나 **omitted** (Options.LoopController = nil → REQ-CMDCTX-015 의 graceful degradation 으로 모든 mutating 메서드가 ErrLoopControllerUnavailable).

본 SPEC 은 옵션 γ + omitted 패턴을 default 로 채택. 사용자 결정에 따라 옵션 α (proto 확장) 로 변경 가능.

### 7.2 client-side adapter 의 책임 (옵션 γ)

| 메서드 | client-side 동작 |
|-------|---------------|
| `OnClear` | daemon 에 `/clear` prompt 전송 (또는 ErrLoopControllerUnavailable 반환 — DAEMON-WIRE-001 가 처리) |
| `OnCompactRequest` | 동일 |
| `OnModelChange` | 동일 |
| `ResolveModelAlias` | **client-side 에서 처리** — registry + aliasMap lookup. `/model opus` → canonical "anthropic/claude-opus-4-7" 변환 후 daemon 에 전달 |
| `SessionSnapshot` | daemon 의 `AgentService/Status` RPC 결과 또는 LoopSnapshot{} (zero-value) |
| `PlanModeActive` | client-side `*atomic.Bool` + ctx — TUI 의 plan mode 표시에 사용 |

→ ResolveModelAlias 와 PlanModeActive 는 client-side meaningful, 나머지는 daemon-side wiring 위임.

---

## 8. 의존 SPEC 진척도 의존성 (Build order)

| 단계 | SPEC | Status (요청 시점) | 본 SPEC 과의 관계 |
|------|------|------------|---------------|
| 1 | SPEC-GOOSE-COMMAND-001 | implemented (PR #50) | dispatcher / SlashCommandContext interface 자산 |
| 2 | SPEC-GOOSE-CMDCTX-001 | implemented (PR #52, v0.1.1) | ContextAdapter / Options / WithContext 자산 |
| 3 | SPEC-GOOSE-CLI-001 | planned | 진입점 (cobra root, bubbletea TUI, ask cmd) |
| 4 | SPEC-GOOSE-CMDLOOP-WIRE-001 | planned (batch A) | LoopControllerImpl — 옵션 γ 채택 시 client-side 에서는 불필요 |
| 5 | SPEC-GOOSE-ALIAS-CONFIG-001 | planned (batch A) | aliasconfig.LoadDefault — alias map 데이터 소스 |
| 6 | SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 | planned (batch A) | strict / lenient mode toggle — `--strict-alias` flag 와 결합 |
| 7 | **SPEC-GOOSE-CMDCTX-CLI-INTEG-001** (본 SPEC) | planned | 5 / 6 의 산출물 + 1 / 2 의 자산을 3 의 진입점에 wiring |
| 8 | SPEC-GOOSE-DAEMON-WIRE-001 | planned (별도) | daemon-side wiring (옵션 γ 의 mutating 명령 처리) |

→ 본 SPEC 의 implementation 은 **CLI-001 implemented 이후** 진행 가능. CMDLOOP-WIRE-001 / ALIAS-CONFIG-001 / PERMISSIVE-ALIAS-001 은 **선택적 의존** (없어도 nil/empty fallback 으로 컴파일/test 가능).

---

## 9. 위험 영역 (Risks)

| ID | 위험 | 영향 | 완화 |
|----|------|------|------|
| R-001 | CLI-001 (planned, FROZEN-by-reference) 의 실제 구조가 본 SPEC 가정과 다를 가능성 | 본 SPEC 의 wiring 위치 (rootcmd PersistentPreRunE / app.go) 가 무효 | run phase Phase 1 (ANALYZE) 에서 CLI-001 implemented 여부 재확인. CLI-001 implemented 이전에는 본 SPEC implementation 차단. CLI-001 §6.1 패키지 레이아웃이 변경되면 본 SPEC 0.1.1 patch + plan-auditor 사이클. |
| R-002 | ALIAS-CONFIG-001 (planned) 부재 시 컴파일 실패 가능성 | 본 SPEC implementation 차단 | empty alias map fallback 으로 진행. ALIAS-CONFIG-001 implemented 후 단순 wire-up 으로 전환. 본 SPEC 0.1.0 은 빈 맵 default 로 컴파일/test 가능. |
| R-003 | client-side LoopController 의 책임 분담 (옵션 α / β / γ) 사용자 결정 필요 | 본 SPEC 의 surface 가 사용자 선택에 따라 변동 | 본 SPEC §7.1 / §7.2 에서 옵션 γ 권장 + 옵션 α / β 의 영향 명시. plan-auditor / ratify 단계에서 사용자 결정. 결정 변경 시 spec 0.1.1 patch. |
| R-004 | TUI 모드와 ask 비대화 모드의 dispatcher 호출 일관성 | 일부 명령이 한 모드에서만 동작 | `ask` 모드에서도 dispatcher 첫 호출 (REQ-CLI-021 일관성). 비대화에서 mutating local command (`/clear`) 의 의미는 trivial (즉시 exit 0 + stdout 메시지). |
| R-005 | SIGINT (Ctrl-C) 처리 — RequestClear trigger 여부 결정 | 사용자 직관 위반 시 UX 후퇴 | 본 SPEC 의 default = NO (Ctrl-C 는 stream cancel only, RequestClear 는 `/clear` slash command 만 trigger). Exclusions §10 에 명시. 사용자 결정에 따라 변경 가능. |
| R-006 | CLI 진입점 다양성 (TUI / ask / ask --stdin / session load) 별 wiring 일관성 | 한 진입점만 wiring 되고 다른 곳에 빠짐 | 본 SPEC §3.1 의 표가 4 진입점 모두 명시. AC 가 각 진입점을 검증. |
| R-007 | bubbletea `tea.Program` 의 ctx 라이프사이클이 stream ctx 와 다른 경우 | WithContext 호출 시점이 부적절 | TUI handleSubmit 에서 stream ctx (Ctrl-C cancellable) 를 명시적으로 derive 하고 그 ctx 를 WithContext 에 전달. CLI-001 REQ-CLI-009 ctx 일관. |
| R-008 | `--strict-alias` flag 가 PERMISSIVE-ALIAS-001 의 surface 와 호환되지 않을 가능성 | flag 명시적 사양 부합 실패 | PERMISSIVE-ALIAS-001 의 strict 토글 인터페이스 (Options 또는 메서드) 와 본 SPEC 의 flag 매핑이 1:1 일치하도록 명시. 불일치 시 PERMISSIVE-ALIAS-001 implemented 후 본 SPEC 0.1.1 patch. |
| R-009 | session load (resume) 시 dispatcher 우회 여부 결정 | 미정 동작 | 본 SPEC 의 default = dispatcher 우회 (resume 은 user input 이 아닌 historical messages). AC 에서 명시. |

---

## 10. 본 SPEC 이 변경하지 않는 자산 (FROZEN by reference)

본 SPEC 은 다음 자산을 **read-only / wiring-only** 로 사용한다. 본문 (REQ / AC / 코드) 변경 금지:

- `internal/command/context.go` (COMMAND-001 SlashCommandContext, ModelInfo, SessionSnapshot)
- `internal/command/dispatcher.go` (COMMAND-001 Dispatcher, ProcessUserInput)
- `internal/command/adapter/adapter.go` (CMDCTX-001 ContextAdapter, Options, New, WithContext, SetPlanMode)
- `internal/command/adapter/controller.go` (CMDCTX-001 LoopController interface, LoopSnapshot)
- `internal/command/errors.go` (COMMAND-001 ErrUnknownModel)
- `internal/llm/router/registry.go` (ROUTER-001 ProviderRegistry)
- SPEC-GOOSE-CLI-001 spec.md / research.md / DEPRECATED.md (별개 SPEC, 본 SPEC 은 어드온)
- SPEC-GOOSE-CMDLOOP-WIRE-001 spec.md (planned, batch A)
- SPEC-GOOSE-ALIAS-CONFIG-001 spec.md (planned, batch A)
- SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 spec.md (planned, batch A)

---

## 11. 본 SPEC 이 신규 추가하는 자산 (산출물)

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| `internal/cli/app.go` (신규) | ~100 | App struct (adapter, dispatcher, client 보유), App.New 생성자 |
| `internal/cli/rootcmd.go` (변경, ~30 LoC 추가) | — | PersistentPreRunE 에 App.New 호출 추가, persistent flag (`--alias-file`, `--strict-alias`) 추가 |
| `internal/cli/tui/update.go` (변경, ~10 LoC) | — | handleSubmit 의 m.sctx() 를 m.app.Adapter.WithContext(ctx) 로 교체 |
| `internal/cli/tui/model.go` (변경, ~5 LoC) | — | Model 에 `app *App` 필드 추가 |
| `internal/cli/commands/ask.go` (변경, ~20 LoC) | — | dispatcher 호출 추가 (ProcessLocal / ProcessProceed 분기) |
| `internal/cli/app_test.go` (신규) | ~150 | App.New 단위 테스트 (nil deps graceful, alias map 주입 검증) |
| `internal/cli/tui/update_test.go` (변경) | — | handleSubmit 의 dispatcher 호출 검증 (teatest harness) |

신규 외부 의존성: 없음. 기존 CMDCTX-001 / COMMAND-001 / ROUTER-001 / CLI-001 자산만 wiring.

---

## 12. TRUST 5 매핑

| 차원 | 본 SPEC 적용 |
|------|-----------|
| Tested | App.New 단위 테스트, TUI handleSubmit teatest, ask cmd subprocess test, race detector pass, ≥ 85% coverage |
| Readable | godoc on App struct + New, English code comments per language.yaml |
| Unified | gofmt + golangci-lint clean, viper / cobra 기존 패턴 준수 |
| Secured | nil deps graceful (CMDCTX-001 invariant 위임), --alias-file 경로 traversal 방지 (ALIAS-CONFIG-001 위임) |
| Trackable | conventional commits, SPEC-GOOSE-CMDCTX-CLI-INTEG-001 trailer, MX:ANCHOR on App struct |

---

## 13. 참조

- `.moai/specs/SPEC-GOOSE-CLI-001/spec.md` v0.2.0 §6.1 (패키지 레이아웃), §6.5 (slash command 프리-디스패치)
- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` v0.1.1 §6.2 (Options struct), §6.3 (의존성 주입 패턴), §6.5 (PlanModeActive 결합)
- `.moai/specs/SPEC-GOOSE-CMDLOOP-WIRE-001/spec.md` (planned) §6.2 (cmdctrl.New 시그니처)
- `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` (planned) §4.1 (Loader, LoadDefault)
- `.moai/specs/SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001/spec.md` (planned)
- `CLAUDE.local.md §2.5` (코드 주석 영어 정책)
- CLI-001 §6.5 의 stub `tuiSlashContext` — 본 SPEC 이 대체 대상
