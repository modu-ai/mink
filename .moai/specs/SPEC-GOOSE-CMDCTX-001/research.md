# SPEC-GOOSE-CMDCTX-001 — Research & Wiring Surface Analysis

> **목적**: PR #50 (SPEC-GOOSE-COMMAND-001)이 정의한 `SlashCommandContext` 인터페이스의 구현체(adapter)를 wiring하기 위한 자산 조사. 6개 메서드 각각의 호출자 진입점과 위임 대상 도메인을 매핑하고, 위험 영역(goroutine race / nil 위임체 / plan mode lock 순서)을 식별한다.
> **작성일**: 2026-04-27
> **선행 SPEC**: SPEC-GOOSE-COMMAND-001 (implemented), SPEC-GOOSE-ROUTER-001 (implemented), SPEC-GOOSE-CONTEXT-001 (implemented), SPEC-GOOSE-SUBAGENT-001 (implemented)

---

## 1. 레포 상태 (Wiring Gap)

PR #50 (커밋 6593705 / de2da57) 이후 다음 상태가 관측된다.

```
internal/command/
├── context.go          # SlashCommandContext interface (소비자) — 본 SPEC이 채울 대상 없음
├── dispatcher.go       # ProcessUserInput에서 sctx 인자로 인터페이스 받음
├── builtin/
│   ├── clear.go        # sctx.OnClear() 호출
│   ├── compact.go      # sctx.OnCompactRequest(target) 호출
│   ├── model.go        # sctx.ResolveModelAlias(alias) + sctx.OnModelChange(info) 호출
│   ├── status.go       # sctx.SessionSnapshot() 호출
│   └── builtin.go      # sctx 추출 헬퍼 (extractSctx)
└── ...
```

`SlashCommandContext` 의 **구현체는 존재하지 않는다**. dispatcher 호출자(예상되는 cmd/goose 또는 query loop 진입점)는 `nil`을 전달하거나 fake stub을 직접 정의해서 builtin 테스트만 통과하는 상태이다.

본 SPEC이 채워야 할 산출물:

```
internal/command/adapter/   (또는 internal/command/contextadapter/)
├── adapter.go              # ContextAdapter struct + SlashCommandContext 구현
├── adapter_test.go         # table-driven test (fake registry/loop/subagent)
└── ...
```

`internal/command/` 본 패키지에 두면 import cycle 위험(`router`, `context`, `subagent`이 `command`를 역방향으로 참조하는 일은 없으나, 안전을 위해 별도 서브패키지 권장).

---

## 2. SlashCommandContext 메서드 매트릭스

| 메서드 | dispatcher 호출 진입점 | 호출 빈도 | 위임 대상 도메인 | 위임 대상 타입/함수 | 부작용 |
|--------|---------------------|---------|--------------|---------------------|------|
| `OnClear() error` | `builtin/clear.go:Execute` | `/clear` 입력 1회 | QUERY-001 loop | `loop.State.Messages = nil` + `TurnCount=0` 리셋 시그널 | mutates loop state |
| `OnModelChange(info ModelInfo) error` | `builtin/model.go:Execute` | `/model <alias>` 1회 | ROUTER-001 + QUERY-001 loop | `*router.ProviderRegistry` lookup + active model swap | next submitMessage 적용 |
| `OnCompactRequest(target int) error` | `builtin/compact.go:Execute` | `/compact [N]` 1회 | CONTEXT-001 | `context.DefaultCompactor` 트리거 (`AutoCompactTracking.ReactiveTriggered = true`) | next loop iteration에서 compact |
| `ResolveModelAlias(alias) (*ModelInfo, error)` | `builtin/model.go:Execute` (사전 호출) | `/model` 입력 1회 | ROUTER-001 | `ProviderRegistry.Get(name)` + alias map | 부작용 없음 (read-only) |
| `SessionSnapshot() SessionSnapshot` | `builtin/status.go:Execute` | `/status` 입력 1회 | QUERY-001 loop + os.Getwd | `loop.State.TurnCount`, current model, `os.Getwd()` | 부작용 없음 (snapshot) |
| `PlanModeActive() bool` | `dispatcher.go:ProcessUserInput` (Step 3 plan-mode check) | 모든 slash command 입력마다 | SUBAGENT-001 | `subagent.TeammateIdentityFromContext(ctx).PlanModeRequired` 또는 글로벌 plan flag | 부작용 없음 (read-only) |

### 2.1 호출 빈도 요약

- **고빈도 (모든 입력)**: `PlanModeActive()` — 매 slash command 입력마다 dispatcher가 호출 (mutating command 차단 게이트)
- **저빈도 (사용자 명령 1회)**: 나머지 5개 메서드 — 사용자가 명시적 명령(`/clear`, `/compact`, `/model`, `/status`)을 입력했을 때만 호출

**함의**: `PlanModeActive()` 는 hot path에 있으므로 lock 경합 / atomic load 정도가 적절. 나머지는 mutex로 동기화해도 비용 무시 가능.

---

## 3. 위임 대상 SPEC별 인터페이스 surface

### 3.1 SPEC-GOOSE-CONTEXT-001 (compactor + loop state)

`internal/context/compactor.go`:

```go
type DefaultCompactor struct {
    Summarizer        Summarizer
    ProtectedHead     int
    ProtectedTail     int
    MaxMessageCount   int
    TokenLimit        int64
    HistorySnipOnly   bool
    Logger            *zap.Logger
}

func (c *DefaultCompactor) ShouldCompact(s loop.State) bool
func (c *DefaultCompactor) Compact(s loop.State) (loop.State, query.CompactBoundary, error)
```

`internal/query/loop/state.go`:

```go
type State struct {
    Messages []message.Message
    TurnCount int
    TaskBudgetRemaining int
    TokenLimit int64
    MaxMessageCount int
    AutoCompactTracking AutoCompactTracking
    // ... (REQ-QUERY-015: 외부 goroutine이 직접 변경해서는 안 된다)
}

type AutoCompactTracking struct {
    ReactiveTriggered bool  // SPEC-GOOSE-CONTEXT-001 REQ-CTX-017
}
```

**핵심 제약 (REQ-QUERY-015)**: `loop.State`는 queryLoop goroutine이 단독 소유. 외부에서 직접 mutate 금지.

**위임 패턴**: ContextAdapter는 `loop.State`를 직접 조작할 수 없으므로 **시그널 채널** 또는 **command queue**를 통해 loop에 요청을 전달해야 한다. 가장 간단한 패턴:

```go
type LoopController interface {
    // RequestClear는 다음 iteration에서 Messages를 nil, TurnCount를 0으로 리셋한다.
    RequestClear(ctx context.Context) error
    // RequestReactiveCompact는 다음 iteration에서 ReactiveTriggered=true로 설정한다.
    RequestReactiveCompact(ctx context.Context, target int) error
    // Snapshot returns a read-only view (copied) of the current State.
    Snapshot() loop.State
}
```

이 인터페이스는 본 SPEC이 정의하고, 실제 구현은 QUERY-001/CONTEXT-001 후속 wiring SPEC이 채운다 (또는 본 SPEC 구현 단계에서 `internal/query/`에 함께 추가).

### 3.2 SPEC-GOOSE-ROUTER-001 (provider/model registry)

`internal/llm/router/registry.go`:

```go
type ProviderMeta struct {
    Name string
    DisplayName string
    SuggestedModels []string
    AdapterReady bool
    // ...
}

type ProviderRegistry struct { /* ... */ }

func (r *ProviderRegistry) Get(name string) (*ProviderMeta, bool)
func (r *ProviderRegistry) List() []*ProviderMeta
func DefaultRegistry() *ProviderRegistry
```

**위임 패턴**: `ResolveModelAlias("anthropic/claude-opus-4-7")` 형태의 입력을 받으면

1. `provider/model` 분리 (`/` 구분 또는 alias map lookup)
2. `registry.Get(provider)` 호출
3. `meta.SuggestedModels` 또는 별도 alias 테이블에서 `model` 매칭
4. 매칭 성공 시 `*ModelInfo{ID: provider+"/"+model, DisplayName: meta.DisplayName + " " + model}` 반환
5. 실패 시 `command.ErrUnknownModel` (현재 정의되어 있지 않다면 본 SPEC에서 sentinel 추가)

`OnModelChange(info)` 는 ROUTER-001의 active model swap을 트리거. 현재 ROUTER-001 spec에는 "active model" 개념이 명시적이지 않으므로, **QUERY-001 loop가 보유한 model field**를 변경하는 시그널로 위임 (LoopController에 `RequestModelChange(info)` 추가).

### 3.3 SPEC-GOOSE-SUBAGENT-001 (plan mode)

`internal/subagent/types.go`:

```go
type TeammateIdentity struct {
    AgentID string
    AgentName string
    PlanModeRequired bool  // REQ-SA-022(a)
    ParentSessionID string
}

func TeammateIdentityFromContext(ctx context.Context) (TeammateIdentity, bool)
```

`internal/subagent/permission.go` (REQ-SA-022):

```go
// permission.go 에 plan mode registry
func registerPlanMode(agentID string) *planModeEntry
func deregisterPlanMode(agentID string)
func PlanModeApprove(parentCtx context.Context, agentID string) error
```

**위임 패턴**: `PlanModeActive()` 는 두 가지 소스를 봐야 한다.

1. **Sub-agent 경로**: 현재 ctx에서 `TeammateIdentityFromContext(ctx)` 추출 → `id.PlanModeRequired` 반환
2. **Top-level 경로** (orchestrator 자체가 plan mode): SUBAGENT-001 plan mode registry는 sub-agent 단위라 top-level orchestrator의 plan mode flag가 별도로 필요. 현재 코드베이스에 없다면 본 SPEC이 새 flag (`adapter.planMode atomic.Bool`)를 정의.

**dispatcher → adapter 호출 시 ctx 전달**: 현재 `Dispatcher.ProcessUserInput(ctx, input, sctx)`는 ctx와 sctx를 분리해서 받는다. adapter가 ctx 기반 lookup을 하려면 dispatcher가 ctx를 sctx 메서드에 전달하거나, adapter가 별도 set/get API를 가져야 한다. 

[HARD] **결정 필요**: `PlanModeActive(ctx context.Context)` 시그니처로 변경할지, 아니면 adapter가 lifetime 시작 시점에 ctx를 등록할지. 본 SPEC §6에서 후자(adapter가 mutate 가능한 plan mode flag 보유)를 채택하는 것을 권장 — 현재 `SlashCommandContext.PlanModeActive() bool` 시그니처를 유지하고 adapter 내부 `atomic.Bool` 사용.

---

## 4. 호출자/구현 위임처 매트릭스 (요약 표)

| `SlashCommandContext` 메서드 | 호출자 (dispatcher 경로) | 위임 대상 SPEC | 위임 대상 타입.메서드 | 부작용 매개체 |
|---|---|---|---|---|
| `OnClear()` | `/clear` builtin | CONTEXT-001 + QUERY-001 | `LoopController.RequestClear(ctx)` | command queue → loop next iteration |
| `OnCompactRequest(target)` | `/compact` builtin | CONTEXT-001 | `LoopController.RequestReactiveCompact(ctx, target)` | `loop.State.AutoCompactTracking.ReactiveTriggered = true` |
| `OnModelChange(info)` | `/model` builtin (ResolveModelAlias 성공 후) | ROUTER-001 + QUERY-001 | `LoopController.RequestModelChange(ctx, info)` | next submitMessage가 새 model 사용 |
| `ResolveModelAlias(alias)` | `/model` builtin | ROUTER-001 | `*router.ProviderRegistry.Get(provider)` + alias parse | 없음 (read-only) |
| `SessionSnapshot()` | `/status` builtin | QUERY-001 + os | `LoopController.Snapshot()` + `os.Getwd()` | 없음 (read-only) |
| `PlanModeActive()` | `dispatcher.go:ProcessUserInput` Step 3 | SUBAGENT-001 + adapter own state | `atomic.Bool.Load()` (adapter local) ⊕ `subagent.TeammateIdentityFromContext(ctx).PlanModeRequired` | 없음 (read-only) |

---

## 5. 위험 영역 (Risk Surface)

### 5.1 R1 — Goroutine race: OnClear ↔ query loop 동시 실행

**시나리오**: 사용자가 모델 응답이 진행 중일 때 `/clear` 입력.

- query loop는 `submitMessage`에서 LLM 호출 응답을 기다리는 중 (`State.Messages` 추가 중)
- 동시에 dispatcher가 `OnClear()` 호출 → loop의 `Messages`를 reset 시도

**완화**:
- adapter는 직접 mutate 금지 — 반드시 `LoopController` 인터페이스를 통과
- `LoopController` 구현체는 채널 또는 mutex 기반 command queue
- loop는 매 iteration 시작 시 queue를 drain (REQ-QUERY-015 준수)

**테스트**: race detector 필수 (`go test -race`)

### 5.2 R2 — nil 위임체 처리

dispatcher는 sctx를 nil로 받을 수 있도록 builtin이 모두 `nil` 가드를 가지고 있다 (e.g., `clear.go:25`: `if sctx != nil`). adapter 자체의 의존성(`*ProviderRegistry`, `LoopController`)이 nil인 경우의 거동도 정의해야 한다.

- nil registry → `ResolveModelAlias` 는 `ErrUnknownModel` 반환
- nil LoopController → `OnClear/OnCompactRequest/OnModelChange/SessionSnapshot` 은 sentinel error 반환
- nil subagent context → `PlanModeActive` 는 false 반환 (안전 기본값: plan mode 아님 = 모든 명령 허용. 단 보수적으로 true 반환을 선택할 수도 있음 — §6에서 결정)

[HARD] **결정**: nil 의존성에 대해 **panic 금지**, 항상 graceful degradation.

### 5.3 R3 — Plan mode lock 순서 (deadlock 위험)

`PlanModeActive()`가 hot path(매 입력)에서 호출되며, 동시에 SUBAGENT-001의 `PlanModeApprove(agentID)`가 별도 goroutine에서 진행될 수 있다.

- `PlanModeApprove` 는 plan mode registry mutex를 잡음
- adapter가 `subagent.TeammateIdentityFromContext(ctx).PlanModeRequired` 를 읽는 것은 ctx value lookup이므로 lock 없음 (ok)
- adapter 자체의 `atomic.Bool` 사용 시 순서 문제 없음

**완화**: lock-free atomic 우선. mutex 사용 시 단 하나의 mutex만 잡고 즉시 해제.

### 5.4 R4 — alias 해석 모호성 (provider/model 구분자 충돌)

ROUTER-001은 `provider/model` 형태의 ID를 가정 (e.g., `openrouter` provider의 `openai/gpt-4o` SuggestedModel). 사용자가 `/model openai/gpt-4o` 입력 시:

- "provider=openai, model=gpt-4o"로 해석할 수도
- "provider=openrouter, model=openai/gpt-4o"로 해석할 수도

**완화**: alias 테이블을 명시적으로 정의 (config 또는 default registry의 SuggestedModels에서 build). 본 SPEC §6에서 결정 필요.

### 5.5 R5 — CWD resolution failure

`SessionSnapshot.CWD` 는 `os.Getwd()` 호출. 디렉토리가 삭제된 상태(rare on macOS/linux)에서 에러를 반환할 수 있다.

**완화**: `SessionSnapshot()` 시그니처는 error를 반환하지 않으므로, `os.Getwd()` 실패 시 `"<unknown>"` 또는 `"<cwd-error>"` placeholder 사용. (Optional REQ로 spec.md에 명시.)

---

## 6. 설계 결정 (Decisions)

| 질문 | 결정 | 근거 |
|------|------|-----|
| adapter 패키지 위치 | `internal/command/adapter/` | `internal/command/`와 같은 부모, import cycle 안전 |
| loop와의 통신 방식 | `LoopController` 인터페이스 (본 SPEC이 정의) | REQ-QUERY-015 준수 (loop state 직접 mutate 금지) |
| `LoopController` 구현체 | 본 SPEC 범위 외 (다른 SPEC에서 wiring) | 본 SPEC은 인터페이스 + ContextAdapter 까지 |
| nil 의존성 거동 | graceful error (panic 금지) | TRUST 5 Secured (예측 가능성) |
| plan mode 상태 source | adapter local `atomic.Bool` + ctx lookup 결합 | 양쪽 경로(top-level/sub-agent) 모두 커버 |
| alias 해석 | exact match → SuggestedModels 검색 → ErrUnknownModel | ROUTER-001 read-only 사용 (해당 SPEC을 변경하지 않음) |
| CWD 실패 fallback | `"<unknown>"` 문자열 | Optional REQ |

---

## 7. 의존 SPEC을 변경하지 않음을 보장하는 경계

본 SPEC은 다음 SPEC들의 인터페이스를 **read-only로 사용**한다 (FROZEN, implemented status 유지):

- **SPEC-GOOSE-COMMAND-001**: `SlashCommandContext` 인터페이스, `ModelInfo`, `SessionSnapshot` 타입을 본 SPEC이 구현. 인터페이스 자체는 수정 금지.
- **SPEC-GOOSE-ROUTER-001**: `*router.ProviderRegistry`, `Get/List/DefaultRegistry` 사용. 새로운 메서드 추가 금지.
- **SPEC-GOOSE-CONTEXT-001**: `loop.State`, `loop.AutoCompactTracking` 타입 read-only 참조. mutate는 본 SPEC이 정의하는 `LoopController` 경유.
- **SPEC-GOOSE-SUBAGENT-001**: `TeammateIdentityFromContext`, `TeammateIdentity.PlanModeRequired` read-only. plan mode registry mutate 금지.

본 SPEC이 새로 정의하는 것:

- `internal/command/adapter.ContextAdapter` (struct)
- `internal/command/adapter.LoopController` (interface — 다른 SPEC이 구현)
- `internal/command/adapter.AliasResolver` (선택적 helper)
- `command.ErrUnknownModel` (sentinel — 이미 존재하면 재사용, 없으면 추가)

---

## 8. 테스트 전략 개요

- **table-driven test**: 6개 메서드 각각에 대해 정상/비정상 케이스를 표로 enumerate
- **fake registry**: `internal/command/adapter/fakes_test.go` 에 `fakeRegistry` (alias map만 holding) + `fakeLoopController` (channel 기반 command queue) + `fakeSubagentCtx`
- **race detector**: 모든 테스트 `go test -race` 통과
- **coverage 목표**: ≥ 90% (parent SPEC인 COMMAND-001은 91.2% 달성)

---

## 9. 구현 순서 추정 (TDD 진입)

T-001: ContextAdapter struct + LoopController interface 정의 + 빈 구현
T-002: ResolveModelAlias (read-only, registry only) → 가장 단순, RED→GREEN 빠름
T-003: SessionSnapshot (LoopController.Snapshot + os.Getwd)
T-004: PlanModeActive (atomic.Bool + ctx lookup)
T-005: OnClear / OnCompactRequest (LoopController write 경로)
T-006: OnModelChange (ResolveModelAlias 결과 + LoopController.RequestModelChange)
T-007: nil/error 경로 + race test

---

## 10. 미해결 질문 (Open Questions)

1. `LoopController.RequestModelChange` 의 실제 구현 SPEC은 어디에 두는가? — 본 SPEC 범위 외, 후속 wiring SPEC (예: SPEC-GOOSE-CMDLOOP-WIRE-001) 또는 QUERY-001 추가 SPEC.
2. alias 테이블 정의 방식 — 정적 (default registry SuggestedModels에서 빌드) vs 동적 (config 파일 로드). 본 SPEC은 정적 + Optional config override.
3. `PlanModeActive` 의 nil ctx 기본값 — false (plan mode 아님 = 명령 허용) vs true (보수적 차단). 본 SPEC은 false 권장 (LSP-style "fail open" 보다는 사용자 명령에 friction이 적음).

---

**Version**: 1.0.0
**Last Updated**: 2026-04-27
