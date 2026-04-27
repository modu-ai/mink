# SPEC-GOOSE-PLANMODE-CMD-001 — Research

> 본 SPEC 은 PR #52 (SPEC-GOOSE-CMDCTX-001 v0.1.1, implemented) 가 노출하는 `adapter.ContextAdapter.SetPlanMode(active bool)` API 를 사용자 진입점으로 노출하기 위한 **빌트인 명령 추가 SPEC** 이다. 본 research.md 는 SPEC-GOOSE-COMMAND-001 (implemented, FROZEN) 의 빌트인 명령 패턴을 분석하고, FROZEN amendment 정책 하에서의 신규 builtin 추가 가능성, dispatcher → builtin → ContextAdapter 호출 경로의 3가지 design 옵션 (A/B/C), Metadata.Mutates 결정 근거를 정리한다.

---

## 1. 문제 정의

### 1.1 현재 상태 (PR #52 머지 직후)

| 항목 | 상태 | 출처 |
|------|------|------|
| `adapter.ContextAdapter.SetPlanMode(active bool)` API | implemented (FROZEN) | `internal/command/adapter/adapter.go:86-88` (PR #52) |
| `command.SlashCommandContext.PlanModeActive() bool` | implemented (FROZEN) | `internal/command/context.go:31-33` (PR #50) |
| dispatcher 의 plan-mode 차단 (Metadata.Mutates=true 명령) | implemented (FROZEN) | `internal/command/dispatcher.go:114` |
| `errors.ErrPlanModeBlocked` | implemented (FROZEN) | `internal/command/errors.go:27-29` |
| 빌트인 명령 (`/clear`, `/compact`, `/exit`, `/help`, `/model`, `/status`, `/version`) | implemented (FROZEN) | `internal/command/builtin/` (PR #50, 7개) |
| **사용자가 plan mode 를 토글할 수 있는 진입점** | **missing** | 본 SPEC 이 채울 빈 자리 |

핵심 결손: `ContextAdapter.SetPlanMode` 는 noun-verb 형태의 mutator API 만 노출되고, 어떤 사용자 입력 경로 (slash command / cli flag / RPC / TUI key) 도 그 호출 사이트가 없다. 결과적으로 plan mode flag 는 (a) 서브에이전트의 `TeammateIdentity{PlanModeRequired:true}` ctx 주입 (CMDCTX-001 §6.5) 만으로 활성화 가능하며, (b) interactive TUI / 비대화 ask 모드의 사용자 본인은 plan mode 를 토글할 수 없다.

### 1.2 본 SPEC 의 단일 책임

`/plan` (canonical name) 또는 `/planmode` (alias) 빌트인 slash command 를 추가하여, 사용자가 interactive TUI / 비대화 ask 모드에서 `ContextAdapter.SetPlanMode(true|false)` 를 호출할 수 있게 한다. 본 SPEC 은:

- COMMAND-001 의 빌트인 명령 패턴 (`internal/command/builtin/`) 을 그대로 따름
- COMMAND-001 의 본문 (REQ-CMD-001 ~ REQ-CMD-019, AC-CMD-001 ~ AC-CMD-013) 은 **변경하지 않음**
- CMDCTX-001 의 본문 (REQ-CMDCTX-001 ~ REQ-CMDCTX-018, AC-CMDCTX-001 ~ AC-CMDCTX-019) 은 **변경하지 않음**
- 신규 builtin 파일 추가만 — `internal/command/builtin/plan.go` (또는 등가)
- 신규 small interface 추가 — `command.PlanModeSetter` (옵션 B 채택, §3 참조)

---

## 2. COMMAND-001 빌트인 명령 패턴 분석

### 2.1 디렉토리 구조 (FROZEN, 변경 금지)

```
internal/command/
├── command.go              # Command interface, Metadata, Args, Result
├── context.go              # SlashCommandContext, ModelInfo, SessionSnapshot
├── dispatcher.go           # Dispatcher, ProcessUserInput, sctx 주입
├── errors.go               # ErrUnknownModel, ErrPlanModeBlocked, ...
├── registry.go             # Registry (alias 지원)
├── source.go               # Source enum (Builtin/Custom/Skill)
└── builtin/
    ├── builtin.go          # Register(reg) — 7개 builtin 등록 + 별칭
    ├── clear.go            # /clear
    ├── compact.go          # /compact
    ├── exit.go             # /exit (alias: quit)
    ├── help.go             # /help (alias: ?)
    ├── model.go            # /model <alias>
    ├── status.go           # /status
    └── version.go          # /version
```

본 SPEC 의 추가 파일:

```
internal/command/
└── builtin/
    └── plan.go             # ⬅︎ 신규 (~70 LoC, 본 SPEC)
    └── plan_test.go        # ⬅︎ 신규 (~120 LoC, 본 SPEC)
```

### 2.2 빌트인 명령의 공통 구현 패턴

각 빌트인 명령은 `command.Command` 인터페이스 (3 메서드) 를 구현하는 unexported struct:

```go
// internal/command/builtin/clear.go (참고용 — PR #50 implemented)
type clearCommand struct{}

func (c *clearCommand) Name() string { return "clear" }
func (c *clearCommand) Metadata() command.Metadata {
    return command.Metadata{
        Description: "Clear the current conversation history.",
        Source:      command.SourceBuiltin,
    }
}
func (c *clearCommand) Execute(ctx context.Context, _ command.Args) (command.Result, error) {
    sctx := extractSctx(ctx)  // builtin/builtin.go:19-26 helper
    if sctx != nil {
        if err := sctx.OnClear(); err != nil {
            return command.Result{}, fmt.Errorf("OnClear: %w", err)
        }
    }
    return command.Result{Kind: command.ResultLocalReply, Text: "conversation cleared"}, nil
}
```

핵심 패턴:
- struct 는 unexported (`clearCommand`, `modelCommand`, ...)
- `Name()` 은 lowercase canonical name 반환
- `Metadata()` 는 fresh struct 반환 (mutable state 없음)
- `Execute()` 는 ctx 에서 `extractSctx(ctx)` 로 sctx 획득 후 sctx 의 메서드 호출
- nil sctx 시 graceful (LocalReply 반환, 에러 없음)
- mutator 동작 (OnClear 등) 은 sctx 메서드 통해 호출 — 직접 의존성 없음

### 2.3 Register 자동 등록

```go
// internal/command/builtin/builtin.go (참고용 — PR #50)
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

    reg.RegisterAlias("quit", "exit")
    reg.RegisterAlias("?", "help")
}
```

본 SPEC 의 추가 (단일 라인):
```go
mustRegister(&planCommand{})              // 신규
reg.RegisterAlias("planmode", "plan")     // 신규 (옵션, §6 결정 사항)
```

`Register` 함수 자체의 시그니처는 변경되지 않음. 단순히 `mustRegister` 호출 라인 1개와 `RegisterAlias` 라인 1개 추가. 이는 COMMAND-001 의 본문 변경이 아닌, 호출 사이트의 추가일 뿐.

### 2.4 dispatcher → builtin 호출 경로 (FROZEN)

```
User input "/plan on"
  → Dispatcher.ProcessUserInput(ctx, input, sctx)
    → parser.Parse → name="plan", rawArgs="on"
    → registry.Resolve("plan") → planCommand
    → if Metadata().Mutates && sctx.PlanModeActive() → block (REQ-CMD-011)
    → context.WithValue(ctx, sctxInjectionKey{}, sctx)
    → planCommand.Execute(execCtx, Args{Positional:["on"]})
      → extractSctx(execCtx) → sctx (= ContextAdapter)
      → sctx.SetPlanMode(true)  // ← 본 SPEC 이 추가하는 호출
    → Result{Kind: ResultLocalReply, Text: "plan mode: on"}
```

이 경로의 모든 함수 (`ProcessUserInput`, `parser.Parse`, `registry.Resolve`, `extractSctx`) 는 PR #50 implemented (FROZEN). 본 SPEC 은 이들을 **호출만 한다**.

---

## 3. Design Decision — SetPlanMode 호출 경로

`ContextAdapter.SetPlanMode(bool)` 은 `command.SlashCommandContext` 인터페이스의 **일부가 아니다**. dispatcher 가 sctx 로서 노출하는 것은 6개 메서드 (OnClear, OnModelChange, OnCompactRequest, ResolveModelAlias, SessionSnapshot, PlanModeActive). 따라서 빌트인 plan command 가 sctx 를 통해 SetPlanMode 를 호출하려면 별도 경로가 필요하다.

3가지 옵션을 비교한다.

### 3.1 옵션 A — SlashCommandContext 인터페이스 확장 (비추천)

`command.SlashCommandContext` 에 `SetPlanMode(active bool)` 메서드 추가.

```go
// 가상 코드 (반대 입장)
type SlashCommandContext interface {
    OnClear() error
    OnModelChange(info ModelInfo) error
    OnCompactRequest(target int) error
    ResolveModelAlias(alias string) (*ModelInfo, error)
    SessionSnapshot() SessionSnapshot
    PlanModeActive() bool
    SetPlanMode(active bool)  // ⬅︎ 신규 메서드
}
```

장점:
- 가장 단순: plan command 의 Execute 에서 `sctx.SetPlanMode(...)` 직접 호출
- 기존 builtin 패턴 (sctx.OnXxx) 과 대칭

단점 (치명적):
- COMMAND-001 의 SlashCommandContext 인터페이스 본문 변경 — **FROZEN 위반**
- 모든 sctx 구현체 (테스트용 fakes 포함) 가 새 메서드 구현 필요 — back-compat 깨짐
- `dispatcher_test.go:51`, `builtin_test.go:45` 의 `mockSctx` 도 변경 필요
- COMMAND-001 v0.1.0 → v0.2.0 amendment 필요. plan-auditor cycle 재실행. SPEC scope 확장.

**결정: 거부**. FROZEN 정책 위반.

### 3.2 옵션 B — 신규 PlanModeSetter 인터페이스 + type assertion (권장)

`command` 패키지에 새 작은 인터페이스 정의:

```go
// internal/command/context.go 에 추가 — 또는 별도 plan_mode_setter.go
//
// PlanModeSetter is an OPTIONAL capability that a SlashCommandContext
// implementation MAY provide. The plan command uses a type assertion to
// detect support and toggles plan mode when supported.
//
// This interface is intentionally separate from SlashCommandContext to
// avoid forcing every implementation (mocks, fakes, alternative adapters)
// to implement SetPlanMode.
type PlanModeSetter interface {
    SetPlanMode(active bool)
}
```

plan command 의 Execute 패턴:

```go
// internal/command/builtin/plan.go
func (p *planCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
    sctx := extractSctx(ctx)
    if sctx == nil {
        return command.Result{Kind: command.ResultLocalReply, Text: "plan: context unavailable"}, nil
    }

    setter, ok := sctx.(command.PlanModeSetter)
    if !ok {
        return command.Result{
            Kind: command.ResultLocalReply,
            Text: "plan: this session does not support plan mode toggling",
        }, nil
    }

    // ... arg parsing → setter.SetPlanMode(...)
}
```

장점:
- COMMAND-001 의 SlashCommandContext 본문 변경 없음 — FROZEN 유지
- CMDCTX-001 의 ContextAdapter.SetPlanMode 시그니처가 이미 `SetPlanMode(bool)` 이므로 자동으로 PlanModeSetter 만족 — 추가 코드 0
- sctx 구현체 중 SetPlanMode 미구현 (예: 테스트 mock, future alternative adapter) 도 영향 없음 (graceful fallback)
- 인터페이스가 작아서 테스트가 쉬움
- COMMAND-001 의 expansion 으로 분류 가능 — 신규 식별자 추가는 변경이 아닌 확장

단점 (관리 가능):
- type assertion 실패 시 사용자에게 무엇을 가이드할지 결정 필요 (§3.4 참조)
- PlanModeSetter 인터페이스의 소속이 COMMAND-001 인지 본 SPEC 인지 명확화 필요 (§3.5 참조)

**결정: 채택**.

### 3.3 옵션 C — ContextAdapter 직접 의존성 주입 (비대칭)

`planCommand` 가 `*adapter.ContextAdapter` 를 직접 보유:

```go
// 가상 코드
type planCommand struct {
    adapter *adapter.ContextAdapter  // ⬅︎ 직접 의존
}

func newPlanCommand(a *adapter.ContextAdapter) *planCommand {
    return &planCommand{adapter: a}
}
```

장점:
- 메서드 호출 직관적
- nil-safe (생성 시점에 nil 체크 가능)

단점 (구조적):
- 다른 builtin (clear, model, compact 등) 은 모두 `extractSctx(ctx)` 패턴을 사용 — **builtin 패턴 비대칭**
- `internal/command/builtin/` 가 `internal/command/adapter/` 에 의존 → import cycle 위험
  (현재 adapter → command 단방향이지만 builtin → adapter 추가 시 양방향)
- `Register(reg)` 의 시그니처에 adapter 추가 필요 → COMMAND-001 의 `func Register(reg registrar)` 본문 변경 → FROZEN 위반
- testability 저하: 단순 mock sctx 로 테스트 불가, ContextAdapter 의 fakes 필요

**결정: 거부**. builtin 패턴 비대칭 + FROZEN 위반.

### 3.4 옵션 B 채택 시 type assertion 실패 동작

ContextAdapter 가 sctx 의 100% 구현체이고 SetPlanMode 를 항상 가지므로, 정상 운영 환경에서 assertion 은 실패하지 않는다. 단, 다음 경우 실패 가능:

1. 테스트 환경: `mockSctx` 같은 sctx 만 구현하고 SetPlanMode 미구현 mock
2. 미래 대안 adapter 가 SetPlanMode 의도적으로 미구현 (예: read-only adapter)

이 경우 plan command 의 동작:
- assertion 실패 → LocalReply 반환: `"plan: this session does not support plan mode toggling"`
- error 미반환 (graceful)
- 단위 테스트에서 검증 가능 (REQ-PMC-009)

### 3.5 PlanModeSetter 인터페이스의 소속

옵션 B 채택 시 `command.PlanModeSetter` 가 COMMAND-001 의 일부인지 본 SPEC 의 일부인지 결정이 필요.

**채택 결정**: PlanModeSetter 는 **본 SPEC 의 산출물**. 단, **물리적 위치는 `internal/command/` 패키지** (sctx 와 같은 패키지). 이는 COMMAND-001 의 expansion 으로 분류 — 신규 public 식별자 추가는 본문 변경이 아님.

근거:
- `command.PlanModeSetter` 는 plan command 만 사용하는 narrow interface
- COMMAND-001 의 SlashCommandContext / Dispatcher / Result 같은 핵심 자산은 변경되지 않음
- 인터페이스 추가는 Go 의 구조적 타이핑 특성상 backward-compatible (기존 구현체에 영향 없음)
- COMMAND-001 의 spec.md HISTORY 에 amendment 표시 없음 (FROZEN 유지)
- 본 SPEC 이 신규 인터페이스의 owner — 후속 SPEC 이 이를 변경하려면 본 SPEC 의 amendment 필요

대안 검토: PlanModeSetter 를 본 SPEC 의 별도 패키지 (`internal/command/planmode/`) 에 두는 안.
- 거부 사유: 작은 인터페이스 1개를 위한 신규 패키지는 과한 분리. type assertion (`sctx.(planmode.Setter)`) 은 cross-package 시 import 추가 필요.

---

## 4. Metadata.Mutates 결정

`command.Metadata.Mutates` 필드는 dispatcher 의 plan-mode 차단 정책 (REQ-CMD-011) 과 결합:

```go
// internal/command/dispatcher.go:114 (FROZEN)
if cmd.Metadata().Mutates && sctx != nil && sctx.PlanModeActive() {
    msg := fmt.Sprintf("command '%s' disabled in plan mode", name)
    return ProcessedInput{
        Kind:     ProcessLocal,
        Messages: []message.SDKMessage{newSystemMessage(msg)},
    }, nil
}
```

질문: `/plan` 명령은 `Mutates=true` 인가?

### 4.1 의미론적 검토

`/plan` 명령은 plan mode flag 자체를 토글한다. 변형 명령 (`/clear`, `/compact`, `/model`) 의 Mutates=true 는 "세션 상태 (대화 내역, 모델, 토큰 카운트) 를 변형한다" 는 의미. plan mode 토글은 이런 의미의 변형이 아니라 **메타-제어** (모드 자체의 토글).

### 4.2 정책 검토

`Mutates=true` 로 설정 시:
- 사용자가 `/plan on` 으로 plan mode 진입 → 이제 plan mode active
- 사용자가 `/plan off` 시도 → dispatcher 가 차단 → "command 'plan' disabled in plan mode"
- **사용자가 plan mode 에서 빠져나올 수 없게 됨** — 함정 (deadlock)

이는 명백히 의도하지 않은 동작이다.

### 4.3 결정

`/plan` 의 `Metadata.Mutates = false`. 근거:
- plan mode 진입 후 빠져나오는 경로 (`/plan off`) 가 항상 가능해야 함
- plan mode 의 의도는 "변형 명령 차단" — `/plan` 자체는 차단 대상이 아님
- `/help`, `/status`, `/version` 도 `Mutates=false` (read-only / 메타-정보) — 일관성

부수 효과: dispatcher 의 `if cmd.Metadata().Mutates && sctx.PlanModeActive() { block }` 분기가 plan command 에는 적용되지 않음. plan command 의 Execute 가 plan-mode-active 상태에서도 항상 실행됨. 이는 정상.

---

## 5. 인자 분기 — `/plan on` / `/plan off` / `/plan toggle` / `/plan status`

### 5.1 사용자 모델

빌트인 명령의 공통 사용자 모델:
- `/clear`: 인자 없음 (immediate effect)
- `/compact [target]`: 인자 0~1개
- `/model <alias>`: 인자 정확히 1개 필수
- `/help [name]`: 인자 0~1개
- `/status`: 인자 없음
- `/version`: 인자 없음
- `/exit`: 인자 없음

`/plan` 의 사용자 모델:
- `/plan on` — plan mode 활성화
- `/plan off` — plan mode 비활성화
- `/plan toggle` — 현재 상태 반전
- `/plan status` — 현재 상태 조회 (읽기 전용)
- `/plan` (인자 없음) — `/plan status` 의 alias (사용자가 가장 자주 쓸 행동: "지금 plan mode 인가?")

### 5.2 인자 검증

- `/plan` (인자 없음) → status 와 동일
- `/plan on` / `/plan off` / `/plan toggle` / `/plan status` → 정상
- `/plan ON` (대문자) → lowercase 정규화 후 정상 (Go 의 strings.ToLower)
- `/plan foo` / `/plan bar` → 잘못된 인자: LocalReply 로 usage 메시지 반환
- `/plan on extra` (인자 2개 이상) → 잘못된 인자: LocalReply 로 usage 메시지 반환

### 5.3 출력 형식

| 입력 | 출력 텍스트 |
|------|----------|
| `/plan` | `plan mode: <on\|off>` |
| `/plan status` | `plan mode: <on\|off>` |
| `/plan on` (이전 off) | `plan mode: on` |
| `/plan on` (이미 on) | `plan mode: on (already active)` |
| `/plan off` (이전 on) | `plan mode: off` |
| `/plan off` (이미 off) | `plan mode: off (already inactive)` |
| `/plan toggle` (off → on) | `plan mode: on` |
| `/plan toggle` (on → off) | `plan mode: off` |
| `/plan foo` | `usage: /plan [on\|off\|toggle\|status]` |

---

## 6. 별칭 (alias) 결정

별칭 옵션:
- `/planmode` → `/plan` 의 alias
- `/p` → 너무 짧음, 미래 명령과 충돌 위험
- 별칭 없음

채택: `/planmode` → `/plan` alias 추가. 사용자 친화성 (사용자가 `/plan` 또는 `/planmode` 둘 다 자연스럽게 시도 가능).

`builtin.Register` 호출 추가:
```go
reg.RegisterAlias("planmode", "plan")
```

기존 별칭 `reg.RegisterAlias("quit", "exit")`, `reg.RegisterAlias("?", "help")` 와 동일 패턴.

---

## 7. TUI 사용자 가시성 (CMDCTX-CLI-INTEG-001 와의 결합)

CMDCTX-CLI-INTEG-001 (planned) 의 REQ-CLIINT-013:
> WHILE TUI 모드에서 `app.Adapter.PlanModeActive()` 가 true 를 반환하는 동안 THEN 시스템은 readline / textarea prompt prefix 또는 statusbar 에 plan mode indicator (예: `[plan]`) 를 표시해야 한다

본 SPEC 은 SetPlanMode trigger 만 제공한다. TUI 의 prompt prefix 변경 / statusbar 렌더링은 CMDCTX-CLI-INTEG-001 의 의무. 본 SPEC 의 §Optional 에서 "TUI 사용자 가시성 (prompt prefix 변경 — CMDCTX-CLI-INTEG-001 와 연계)" 으로 명시.

본 SPEC 은 plan command 가 LocalReply 로 `plan mode: on` / `plan mode: off` 메시지를 반환하므로, TUI 가 CMDCTX-CLI-INTEG-001 implemented 이전이라도 사용자는 텍스트 출력으로 plan mode 상태를 확인 가능 (graceful baseline).

---

## 8. dispatcher 통합 검토 (FROZEN 변경 부재 확인)

### 8.1 dispatcher.go 변경 부재

`internal/command/dispatcher.go` (PR #50, FROZEN) 의 본문은 변경되지 않음. plan command 가 dispatcher 의 `cmd.Metadata().Mutates && sctx.PlanModeActive()` 차단 분기를 우회하는 것은 `Mutates=false` 설정만으로 충분 (§4 결정). dispatcher 의 차단 로직은 그대로 유지.

### 8.2 builtin/builtin.go 변경 (호출 사이트만)

`Register(reg)` 함수 본체에 2 라인 추가:
```go
mustRegister(&planCommand{})              // 신규
reg.RegisterAlias("planmode", "plan")     // 신규
```

이는 `builtin.Register` 의 시그니처 / 동작 / contract 변경이 아닌 **등록 명단의 확장**. COMMAND-001 §6.5 에서 명시한 "7개 builtin" 이 "8개 builtin" 으로 늘어나지만, COMMAND-001 의 SPEC 에서 "7개 명단" 자체가 invariant 가 아닌 **현재 시점의 산출물 목록**임을 §6.5 주석 (`"// Commands registered (7):"`) 에서 확인 가능. 본 SPEC 으로 COMMAND-001 spec.md 의 numbering 변경은 불필요 (현재 7 → 미래 8 은 자연스러운 expansion).

### 8.3 dispatcher_test.go / builtin_test.go 의 mockSctx 변경 부재

`mockSctx` 는 6 메서드만 구현. 신규 PlanModeSetter 인터페이스를 구현하지 않음. 이는 의도된 동작:
- 기존 테스트는 SetPlanMode 호출 경로를 다루지 않음 → 영향 없음
- 새 plan_test.go 에서 mockSctx 의 변형 (mockSctxWithSetter) 또는 ContextAdapter 직접 사용으로 검증

**결론**: 옵션 B 채택 시 COMMAND-001 / CMDCTX-001 의 본문 변경 없음. 본 SPEC 은 expansion 으로 분류.

---

## 9. 위험 영역 (Risks Preview)

### 9.1 R-1: PlanModeSetter 인터페이스의 소속 이후 분쟁

본 SPEC 이 정의하는 `command.PlanModeSetter` 가 미래 SPEC 에서 변경되거나 (예: `SetPlanMode(active bool, reason string)` 로 시그니처 변경) 본 SPEC 의 호출 사이트가 깨질 가능성.

완화: 본 SPEC §Exclusions 에서 명시 — "PlanModeSetter 의 시그니처 변경은 본 SPEC 의 amendment 필요". CMDCTX-001 의 `ContextAdapter.SetPlanMode(bool)` 시그니처는 v0.1.1 implemented 시점에 FROZEN 이므로, PlanModeSetter 가 그 시그니처를 미러링 (`SetPlanMode(active bool)`) 하는 한 변경 위험은 낮음.

### 9.2 R-2: type assertion 실패 시 사용자 혼란

`sctx.(command.PlanModeSetter)` 가 false 를 반환하는 환경 (테스트 mock, future read-only adapter) 에서 사용자가 `/plan on` 입력 시 받는 응답: `"plan: this session does not support plan mode toggling"`. 사용자가 이 메시지를 보고 무엇을 해야 할지 모를 수 있음.

완화: 메시지에 후속 가이드 추가 — `"plan: this session does not support plan mode toggling. Run goosed first or restart the CLI."`. 단, ContextAdapter 가 정상 wiring 된 환경에서는 항상 PlanModeSetter 를 만족하므로, 이 분기는 테스트 외 발생 빈도 낮음.

### 9.3 R-3: dispatcher 가 sctx==nil 인 경우

dispatcher.go 는 `sctx != nil` 체크 후 plan-mode 차단 (line 114). plan command 의 Execute 도 `if sctx == nil` 가드 후 graceful return. 단, `extractSctx(ctx)` 가 nil 반환 시 (= dispatcher 가 sctx 를 ctx 에 주입하지 않은 경우) plan command 가 `plan: context unavailable` 출력. 이는 dispatcher 의 정상 호출 경로 (line 134-136) 에서는 발생하지 않음 — 외부 호출자가 dispatcher 를 우회하고 plan command 를 직접 호출한 경우만.

완화: 정상 경로에서는 발생 안 함 (dispatcher 가 항상 sctx 주입). 외부 직접 호출은 dispatcher 우회의 사용자 책임. 본 SPEC 의 AC 에서는 dispatcher 호출 시나리오만 검증.

### 9.4 R-4: 별칭 등록 실패

`reg.RegisterAlias("planmode", "plan")` 호출 시 (Registry.RegisterAlias 의 동작) 만약 "planmode" 가 이미 다른 canonical 에 매핑되어 있으면 실패 가능. 현재 PR #50 implemented 의 별칭 명단은 `quit`, `?` 둘만 — 충돌 없음. 단, custom command 가 이미 "planmode" 를 등록한 경우 builtin Register 가 panic.

완화: builtin 은 application startup 시점에 가장 먼저 Register — custom command 등록 이전. panic 시 process 종료 (fail-fast, 빌드/배포 단계에서 발견). AC-PMC-013 가 검증.

### 9.5 R-5: `/plan` canonical 자체와 사용자 자연어의 충돌

사용자가 plan phase 의 `/moai plan ...` 같은 multi-segment 명령을 시도하다가 빌트인 `/plan` 가 우선 매칭될 우려. COMMAND-001 의 parser 는 첫 토큰을 명령 이름으로 사용 (parser.Parse). `/moai plan ...` 같은 multi-segment 는 첫 토큰 "moai" 가 명령 이름이 되므로 빌트인 plan 과 충돌하지 않음. 단, `/plan moai ...` 같은 입력은 plan command 의 인자로 처리됨 (`/plan foo` 처럼 잘못된 인자 → usage 메시지).

완화: 잘못된 인자 시 명확한 usage 메시지 (REQ-PMC-007). 사용자가 원래 의도한 명령으로 재시도 가능.

---

## 10. TDD 접근 (RED → GREEN → REFACTOR)

| 순서 | 작업 | 검증 AC |
|------|------|------|
| T-001 | `internal/command/context.go` (또는 신규 파일) 에 `PlanModeSetter` 인터페이스 정의 | AC-PMC-001 |
| T-002 | `*adapter.ContextAdapter` 가 `command.PlanModeSetter` 를 구현함을 컴파일 타임 단언 (`var _ command.PlanModeSetter = (*adapter.ContextAdapter)(nil)`) | AC-PMC-002 |
| T-003 | `internal/command/builtin/plan.go` 에 `planCommand` struct + `Name()` + `Metadata()` (Mutates=false) | AC-PMC-003, AC-PMC-004 |
| T-004 | `planCommand.Execute` — 인자 없음 시 status 출력 | AC-PMC-005 |
| T-005 | `planCommand.Execute` — `on` / `off` / `toggle` / `status` 인자 분기 | AC-PMC-006, AC-PMC-007, AC-PMC-008 |
| T-006 | 잘못된 인자 (`/plan foo`, `/plan on extra`) → usage LocalReply | AC-PMC-009 |
| T-007 | sctx==nil graceful (LocalReply, no error) | AC-PMC-010 |
| T-008 | sctx 가 PlanModeSetter 미구현 시 graceful (LocalReply, no error) | AC-PMC-011 |
| T-009 | `internal/command/builtin/builtin.go` Register 에 `mustRegister(&planCommand{})` + `RegisterAlias("planmode", "plan")` 추가 | AC-PMC-012, AC-PMC-013 |
| T-010 | dispatcher 통합 테스트 — `/plan on` → ContextAdapter.PlanModeActive() == true | AC-PMC-014 |
| T-011 | dispatcher 통합 테스트 — `/plan` (Mutates=false) 가 plan-mode-active 상태에서도 차단되지 않음 | AC-PMC-015 |
| T-012 | 코드 커버리지 ≥ 85%, race detector pass | TRUST 5 Tested |

---

## 11. 의존성 정리 (라이브러리 / SPEC)

기존 자산만 사용. 신규 외부 의존성 없음.

| 의존 | 형태 | 사용 위치 |
|------|------|--------|
| `internal/command` (COMMAND-001) | FROZEN, 호출만 | Command interface, Metadata, extractSctx 헬퍼 |
| `internal/command/builtin` (COMMAND-001) | FROZEN, 호출 + 등록 명단 추가 | builtin.go Register |
| `internal/command/adapter` (CMDCTX-001) | FROZEN, type assertion 대상 | ContextAdapter.SetPlanMode |
| `context` (stdlib) | 표준 | Execute 시그니처 |
| `fmt` (stdlib) | 표준 | LocalReply 텍스트 포매팅 |
| `strings` (stdlib) | 표준 | 인자 정규화 (ToLower) |
| `github.com/stretchr/testify` | 테스트 only | unit test assertions |

테스트 라이브러리:
- `internal/command/builtin/plan_test.go` — testify, ContextAdapter 직접 사용 또는 mockSctx + PlanModeSetter mock
- `internal/command/dispatcher_test.go` — 기존 mockSctx 영향 없음
- 옵션: `internal/command/integration_test.go` — dispatcher + ContextAdapter + planCommand end-to-end (선택)

---

## 12. 결론

본 SPEC 은 **신규 builtin 명령 1개 + 신규 narrow interface 1개** 을 추가하는 small SPEC (size: 소(S)) 이다. 영향 범위:

- 신규 파일: `internal/command/builtin/plan.go` (~70 LoC), `internal/command/builtin/plan_test.go` (~120 LoC)
- 변경 파일: `internal/command/builtin/builtin.go` (+2 LoC)
- 신규 인터페이스: `command.PlanModeSetter` — `internal/command/context.go` 또는 별도 파일 (~10 LoC)

COMMAND-001 / CMDCTX-001 의 본문은 **변경하지 않음**. 본 SPEC 은 expansion 으로 분류. 사용자 진입점:
- TUI: `/plan on`, `/plan off`, `/plan toggle`, `/plan status`, `/plan` 직접 입력
- ask 비대화 모드: `goose ask "/plan status"` 같은 사전 입력 (CMDCTX-CLI-INTEG-001 의 dispatcher 호출 패턴 위에)
- 별칭: `/planmode` → `/plan`

본 SPEC 은 CMDCTX-CLI-INTEG-001 implemented 이후 **동시 implementation 가능** (CLI-INTEG 의 dispatcher wiring 이 plan command 의 호출 경로를 활성화). 단, 본 SPEC 자체의 implementation (plan.go 작성) 은 CLI-INTEG implementation 과 독립 — `internal/command/` 패키지 내부 변경만으로 완결.
