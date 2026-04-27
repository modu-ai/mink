# SPEC-GOOSE-CMDLOOP-WIRE-001 — Research

> 본 SPEC은 PR #52 (CMDCTX-001) 머지로 신설된 `internal/command/adapter/controller.go` 의 `LoopController` 인터페이스에 대한 **실제 구현체** 를 query loop 측에 wiring 하는 작업이다. 본 research.md 는 구현 전 surface analysis 와 위험 영역을 정리한다.

---

## 1. 문제 정의

### 1.1 현재 상태 (PR #52 머지 직후)

| 항목 | 상태 | 출처 |
|------|------|------|
| `command.SlashCommandContext` 인터페이스 | implemented (FROZEN) | `internal/command/context.go` (PR #50) |
| `adapter.ContextAdapter` (Slash → Loop 위임) | implemented (FROZEN) | `internal/command/adapter/adapter.go` (PR #52) |
| `adapter.LoopController` 인터페이스 | **defined only** (FROZEN) | `internal/command/adapter/controller.go:19-38` (PR #52) |
| `LoopController` **구현체** | **missing** | — |
| 결과 | dispatcher → adapter → `ErrLoopControllerUnavailable` | adapter.go:111-115 |

핵심 결손: `ContextAdapter.New(Options{LoopController: ???})` 의 `???` 자리에 들어갈 실 객체가 없다. CLI / DAEMON-WIRE-001 에서 dispatcher 를 활성화하려면 본 SPEC 의 산출물이 필요하다.

### 1.2 요구되는 4개 메서드

`LoopController` 인터페이스 (FROZEN, 변경 금지):

```go
type LoopController interface {
    RequestClear(ctx context.Context) error
    RequestReactiveCompact(ctx context.Context, target int) error
    RequestModelChange(ctx context.Context, info command.ModelInfo) error
    Snapshot() LoopSnapshot
}
```

| 메서드 | 기대 동작 (slash command 의미론) |
|--------|----------|
| `RequestClear` | `state.Messages = nil; state.TurnCount = 0` 을 다음 iteration 시작 시 적용 |
| `RequestReactiveCompact(target)` | `state.AutoCompactTracking.ReactiveTriggered = true` 를 다음 iteration 시작 시 적용. `target == 0` 이면 compactor 기본값 사용 |
| `RequestModelChange(info)` | engine 의 활성 모델을 다음 `submitMessage` 호출 직전 atomic 교체 |
| `Snapshot()` | `LoopSnapshot{TurnCount, Model, TokenCount, TokenLimit}` 의 read-only 복사본 반환 (모든 goroutine 에서 안전) |

### 1.3 핵심 제약 (불변식)

`SPEC-GOOSE-QUERY-001 REQ-QUERY-015`:
> `loop.State` 는 `queryLoop` goroutine 이 단독으로 소유한다. 외부 goroutine 이 직접 변경해서는 안 된다.

→ 본 SPEC 의 모든 메서드는 외부 goroutine 에서 호출된다 (dispatcher 콜백). 따라서 직접 `state` 를 mutate 하면 안 되고, **request enqueue → loop iteration 적용** 패턴이 필수.

---

## 2. 위임 대상 코드 분석

### 2.1 `loop.State` 구조 (FROZEN)

`internal/query/loop/state.go`:

```go
type State struct {
    Messages                     []message.Message
    TurnCount                    int
    MaxOutputTokensRecoveryCount int
    TaskBudgetRemaining          int
    TokenLimit                   int64
    MaxMessageCount              int
    AutoCompactTracking          AutoCompactTracking
}

type AutoCompactTracking struct {
    ReactiveTriggered bool
}
```

본 SPEC 이 mutate 해야 할 필드:

- **clear**: `Messages = nil`, `TurnCount = 0` (그 외는 보존 — `TaskBudgetRemaining` 보존해야 누적 budget 정책 유지)
- **reactive compact**: `AutoCompactTracking.ReactiveTriggered = true`
- **model change**: state 가 아닌 **engine config / external active model 포인터** mutate. state 자체는 영향받지 않음.

### 2.2 `queryLoop` iteration 진입점

`internal/query/loop/loop.go:188`:

```go
for {
    // --- S7: iteration 시작 시 compaction 검사 (after_compact continue site) ---
    if cfg.ShouldCompact != nil && cfg.ShouldCompact(state) { ... }

    // --- S5: budget_exceeded gate ---
    if state.TaskBudgetRemaining <= 0 { ... return }

    // --- max_turns gate ---
    if cfg.MaxTurns > 0 && state.TurnCount >= cfg.MaxTurns { ... return }

    // turn 카운트 증가 + LLM 호출
    state.TurnCount++
    streamCh, err := cfg.CallLLM(ctx)
    ...
}
```

→ iteration 진입점 (line 188 `for {` 직후) 에 **request drain** 훅을 삽입할 수 있다. 단, `ShouldCompact` 검사 **이전에** drain 해야 reactive compact request 가 즉시 발효된다.

### 2.3 `engine.SubmitMessage` 진입점

`internal/query/engine.go:80`:

```go
func (e *QueryEngine) SubmitMessage(ctx context.Context, prompt string) (<-chan message.SDKMessage, error) {
    e.mu.Lock()
    defer e.mu.Unlock()
    ...
    e.stateMu.Lock()
    currentState := e.state
    e.stateMu.Unlock()
    currentState.Messages = append(append([]message.Message(nil), currentState.Messages...), userMsg)
    callLLM := e.buildLLMStreamFuncFromMsgs(currentState.Messages)
    ...
}
```

→ `SubmitMessage` 직전이 model swap 의 자연스러운 atomic 시점. `e.cfg` 또는 별도의 `activeModel atomic.Pointer[ModelInfo]` 를 `buildLLMStreamFuncFromMsgs` 가 참조하도록 변경 가능.

`OnComplete` 콜백 (engine.go:150-154):

```go
OnComplete: func(finalState loop.State) {
    e.stateMu.Lock()
    e.state = finalState
    e.stateMu.Unlock()
},
```

→ `Snapshot()` 은 `e.stateMu.RLock()` 으로 `e.state` 를 읽어 `LoopSnapshot` 으로 변환하면 된다. SubmitMessage 가 진행 중이어도 `stateMu` 가 보호하는 마지막 완료 상태를 얻는다.

### 2.4 `DefaultCompactor.ShouldCompact` 트리거 경로

`internal/context/compactor.go:118-147`:

```go
func (c *DefaultCompactor) ShouldCompact(s loop.State) bool {
    if s.AutoCompactTracking.ReactiveTriggered { return true }
    ...
}
```

→ `ReactiveTriggered = true` 가 set 되면 token 사용률과 무관하게 다음 iteration 진입 시 즉시 compact 된다. compact 후 newState 는 `cfg.Compact` 반환값으로 교체되며, `ReactiveTriggered` 플래그는 compact 전략 코드에서 자연스럽게 reset 되거나 (현재 동작 확인 필요) 본 SPEC 이 reset 책임을 진다.

`target int` 파라미터 처리: `DefaultCompactor.Compact` 는 현재 `target` 을 받지 않고 내부적으로 `tokenLimit * 60 / 100` 를 사용한다 (compactor.go:208-211). `target != 0` 인 경우의 처리 방안:

- 옵션 A: 본 SPEC 에서 `target` 을 `state` 에 저장하는 새 필드 추가 (e.g. `CompactTargetOverride int64`) 후 compactor 가 참조 — CONTEXT-001 변경 필요 → **FROZEN 위배**
- 옵션 B: 본 SPEC 의 LoopControllerImpl 가 `target` 을 무시하고 compactor 기본값만 사용. `target != 0` 은 향후 SPEC 으로 위임 → **권장**

### 2.5 model swap 시점

현재 `engine.cfg.Provider` 는 `QueryEngineConfig` 의 read-only 필드. `buildLLMStreamFuncFromMsgs` 가 이를 참조한다. 본 SPEC 이 model 을 swap 하려면:

- 옵션 1: engine 에 `activeModel atomic.Pointer[command.ModelInfo]` 필드를 추가하고 `RequestModelChange` 가 atomic.Store, `buildLLMStreamFuncFromMsgs` 가 atomic.Load.
- 옵션 2: model change request 를 다른 mutation 들과 동일한 queue 에 enqueue → `SubmitMessage` 진입 시 drain → `cfg` 변경.

옵션 1 이 race-clean 하고 lock-free. 단, model 식별자 → provider 어댑터 매핑은 본 SPEC 의 책임이 아님 (ROUTER-001 / 후속 SPEC). 본 SPEC 은 **model identifier 의 atomic 교체** 만 담당하고, 실제 provider re-dial / credential pool swap 은 후속 SPEC (CMDCTX-CREDPOOL-WIRE-001) 가 담당.

→ **본 SPEC 에서는 model swap 의 atomic 식별자 갱신만 구현, provider re-dial 은 placeholder (string 기록 + log).**

---

## 3. 구현 패키지 위치 결정

### 3.1 두 후보

옵션 A: `internal/query/loop/cmdctrl/` — query loop 내부에 위치
- 장점: state 변경이 loop 와 가까운 곳에서 일어남, import cycle 위험 낮음
- 단점: query loop 패키지가 command 의존성을 갖게 됨 (command.ModelInfo 참조)

옵션 B: `internal/command/adapter/loopctrl/` — command adapter 옆
- 장점: adapter 와 controller 가 한 곳에 모임, command 도메인 응집도
- 단점: query/engine 의 unexported 필드 (e.state, e.stateMu, e.cfg) 접근 어려움 → query 패키지 측에 hook (예: `engine.RegisterController(c LoopController)`) 필요

옵션 C (권장): `internal/query/cmdctrl/` — query 패키지 내부 신규 서브패키지
- 장점:
  - query 패키지 (engine.go) 와 동일 모듈 트리 → engine 의 internal API 노출 최소화 가능
  - command 의 ModelInfo 만 import (역방향 cycle 없음)
  - loop 패키지 (state.go) 도 import 가능
- 단점: 약간의 boilerplate (engine ↔ controller bidirectional reference)

→ **§6 에서 옵션 C 를 채택하되, 사용자 결정용 옵션 A/B 도 명시.**

### 3.2 import dependency graph (옵션 C)

```
internal/command/adapter (LoopController interface)
        ▲
        │ implements
        │
internal/query/cmdctrl (LoopControllerImpl)  ← 본 SPEC 신규
        │
        ├── imports internal/command (ModelInfo)
        ├── imports internal/query/loop (State for snapshot)
        └── imports internal/query (engine reference for stateMu access — via internal API)

internal/query (engine)
        │
        └── exports RegisterControllerHook 또는 등가 (본 SPEC 내부에서만 사용)
```

cycle 없음. `internal/command` 는 `internal/query/*` 를 import 하지 않으므로 backward edge 없음.

---

## 4. Request Enqueue 패턴 분석

### 4.1 후보 패턴

**패턴 A: buffered channel + select drain**

```go
type LoopControllerImpl struct {
    requests chan controlRequest // buffered, cap 8
    engine   *query.QueryEngine
}

type controlRequest struct {
    kind   requestKind // clear | reactive_compact | model_change
    target int
    info   *command.ModelInfo
}

func (c *LoopControllerImpl) RequestClear(ctx context.Context) error {
    select {
    case c.requests <- controlRequest{kind: reqClear}:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    default:
        return ErrRequestQueueFull
    }
}
```

iteration 진입 시 drain:

```go
for {
    drainControlRequests(state, c.requests) // mutate state in-place
    if cfg.ShouldCompact != nil && cfg.ShouldCompact(state) { ... }
    ...
}
```

장점: race-clean (channel 이 동기화), 명시적 backpressure.
단점: `drainControlRequests` 를 loop.go 에 추가해야 함 → `LoopConfig` 에 새 필드 (`PreIterationHook func(state *State)`) 필요.

**패턴 B: atomic flag + mutex-guarded request**

```go
type LoopControllerImpl struct {
    pendingClear   atomic.Bool
    pendingCompact atomic.Bool
    pendingModelMu sync.Mutex
    pendingModel   *command.ModelInfo
}
```

iteration 진입 시 check:

```go
if c.pendingClear.Swap(false) { state.Messages = nil; state.TurnCount = 0 }
if c.pendingCompact.Swap(false) { state.AutoCompactTracking.ReactiveTriggered = true }
```

장점: 가장 단순, 외부 호출자 contention 거의 없음, queue full 시나리오 없음.
단점: 동일 종류 요청이 합쳐짐 (e.g. 두 번의 RequestClear 가 한 번으로 보임) — 하지만 본 SPEC 의미론상 **idempotent** 이므로 무해.

**패턴 C (권장): 하이브리드 — atomic flag (clear/compact) + atomic.Pointer (model)**

```go
type LoopControllerImpl struct {
    pendingClear   atomic.Bool                   // RequestClear → Swap(true)
    pendingCompact atomic.Bool                   // RequestReactiveCompact → Swap(true)
    activeModel    atomic.Pointer[command.ModelInfo] // RequestModelChange → Store
    engine         *query.QueryEngine            // for Snapshot
}
```

장점:
- lock-free, race detector clean
- queue full 시나리오 없음 (idempotent)
- model swap 은 atomic.Pointer 로 마지막 요청만 유효 (slash command 의미론과 일치 — `/model gpt-4` 후 `/model claude-opus` 했을 때 마지막 것만 유효)

단점:
- compact target 무시 (§2.4 옵션 B 와 일치)
- request 카운팅이 어려움 (debug observability 측면) — log 로 보완 가능

→ **패턴 C 채택.**

### 4.2 pre-iteration hook 주입 방법

`LoopConfig` 에 새 필드 추가:

```go
// PreIteration 은 loop iteration 시작 직후, ShouldCompact 검사 이전에 호출되는 훅이다.
// nil 이면 무시. state 를 in-place mutate 할 수 있다 (loop goroutine 단독 소유 보장).
PreIteration func(state *State)
```

engine.go 의 `LoopConfig` 조립 시 `PreIteration: c.applyPendingRequests` 를 주입.

**대안**: `cfg.ShouldCompact` 클로저가 내부적으로 controller 의 pendingCompact 를 swap 하는 sneaky 방법도 가능하지만, `ShouldCompact` 의 의미론을 오염시키므로 **명시적 PreIteration 훅** 이 깨끗.

`LoopConfig` 변경은 QUERY-001 SPEC 에 영향. 그러나 **추가 필드** 이고 nil-tolerant 하면 후방 호환되므로 FROZEN 위배 아님 (§7 참조).

---

## 5. Race-Clean 검증 전략

### 5.1 필수 race scenarios

1. **dispatcher goroutine 이 `RequestClear` 호출 중 loop goroutine 이 iteration 진입하여 `pendingClear.Swap(false)` 를 실행** → atomic.Bool 의 Store-after-Swap 가 1 회 누락될 수 있는가? **No** — atomic.Bool 의 Swap 은 lock-free CAS 보장.

2. **dispatcher goroutine 이 `RequestModelChange(A)` 호출, 즉시 `RequestModelChange(B)` 호출. Loop goroutine 이 그 사이에 `activeModel.Load()` 한다면?** → 마지막 Store 이전 어떤 값이든 반환할 수 있음 (linearizable). slash command 의미론상 사용자 의도는 "마지막 명령 우선" 이므로 OK.

3. **`Snapshot()` 호출 시 `e.stateMu` 가 SubmitMessage 에 의해 잡혀 있다면?** → `Snapshot()` 도 `e.stateMu.RLock()` 으로 보호. `RWMutex` 사용하면 다중 reader 허용.

4. **engine 이 종료 (close) 된 후 Snapshot 호출?** → engine lifecycle SPEC 에 의존. 본 SPEC 은 engine non-nil 가정. lifecycle 은 후속 SPEC.

### 5.2 검증 도구

| 검증 항목 | 도구 | 명령 |
|----------|------|------|
| atomic.Bool / atomic.Pointer race | `go test -race` | `go test -race ./internal/query/cmdctrl/...` |
| concurrent request scenario | 별도 `race_test.go` (CMDCTX-001 의 `race_test.go` 패턴 참고) | 100 goroutines × 1000 iter |
| state mutation 단일 소유자 invariant | static grep (`grep -rE 'state\.[A-Z][A-Za-z]+\s*=' internal/query/cmdctrl/`) | 결과 0건이어야 함 (state mutation 은 PreIteration 콜백 내부에서만, in-place 인자 mutation) |
| LSP 진단 | `gopls` | quality gate |

### 5.3 부수 race-clean 검증 (참고: CMDCTX-001 의 패턴)

`internal/command/adapter/race_test.go` 가 모범. `go test -race -count=10` PASS 를 기준선.

---

## 6. Test Plan 개요 (run phase 참조용)

| Test ID | 시나리오 | 검증 |
|---------|---------|------|
| TC-W001 | nil ctx → ctx.Background fallback | RequestClear/RequestReactiveCompact/RequestModelChange 가 nil ctx 에 panic 없이 동작 |
| TC-W002 | RequestClear 후 1 iteration → state.Messages == nil, state.TurnCount == 0 | unit test, fake loop |
| TC-W003 | RequestReactiveCompact(0) 후 1 iteration → AutoCompactTracking.ReactiveTriggered == true → ShouldCompact 호출 시 true 반환 | integration test, real DefaultCompactor |
| TC-W004 | RequestModelChange(A) 후 RequestModelChange(B) → activeModel.Load() == B | unit test |
| TC-W005 | Snapshot during SubmitMessage in-flight → 직전 OnComplete 결과 반환, 진행 중인 messages 없음 | sync test |
| TC-W006 | 100 goroutine 동시 RequestClear/Compact/ModelChange → race detector clean | race_test.go |
| TC-W007 | engine 미주입 (nil) → graceful degradation, ErrEngineUnavailable 반환 | nil path |
| TC-W008 | RequestModelChange(zero ModelInfo) → ErrInvalidModelInfo 반환 (또는 무조건 accept — 결정 필요, §9 참조) | edge case |
| TC-W009 | adapter.LoopController 인터페이스 만족 (compile-time 확인) | `var _ adapter.LoopController = (*LoopControllerImpl)(nil)` |
| TC-W010 | ContextAdapter wiring 통합 — 실제 dispatcher 호출이 loop state 변경으로 이어짐 | E2E test |

---

## 7. 의존성 정합화 / FROZEN 영향 평가

| 의존 SPEC | status | 본 SPEC 이 변경하는가? | 비고 |
|---------|--------|-----------------|------|
| SPEC-GOOSE-CMDCTX-001 | implemented (FROZEN) | **No** | `LoopController` 인터페이스 변경 금지 (controller.go:19-38 그대로) |
| SPEC-GOOSE-CONTEXT-001 | implemented (FROZEN) | **No** | `DefaultCompactor`, `AutoCompactTracking` 의미론 그대로 사용 |
| SPEC-GOOSE-QUERY-001 | implemented (FROZEN, 2026-04-25 기준 v0.1.3) | **부분 변경 평가 필요** | `LoopConfig` 에 `PreIteration` 필드 추가 — nil-tolerant 후방 호환 (§4.2). engine 에 `RegisterController` 또는 등가 메서드 추가 — 신규 API 이므로 backward compat. 본 SPEC 채택 시 QUERY-001 의 미세 확장으로 다룰지, 별도 minor bump 로 다룰지 사용자 결정 필요. |

**리스크**: QUERY-001 의 v0.1.3 → v0.1.4 frontmatter bump + HISTORY 1줄 추가가 필요할 수 있다. 만약 사용자가 "QUERY-001 코드 한 줄도 건드리지 마라" 라고 결정하면, 본 SPEC 은 옵션 B (engine 에 `LoopController` 자체를 인스턴스 변수로 보유, engine 이 controller 의 drain 메서드를 호출) 로 reroute 해야 한다. 이는 §6.2 에서 명시.

---

## 8. 비교 가능한 모범 사례

### 8.1 charmbracelet/crush 의 유사 wiring

`crush` 는 LSP client wrap 시 `transport.Connection` 을 외부에서 호출하고, 내부 goroutine 이 message 를 process. 본 SPEC 의 패턴 (외부 enqueue → 내부 drain) 과 isomorphic.

### 8.2 Go runtime 의 `runtime.SetFinalizer` 패턴

외부 goroutine 이 atomic flag 를 set, internal goroutine 이 다음 iteration 에서 swap-and-act. lock-free 패턴의 표준.

---

## 9. 위험 영역 / 결정 보류 항목

| ID | 위험 / 결정 보류 | 본 SPEC 의 입장 |
|----|--------------|------------|
| R-001 | QUERY-001 의 `LoopConfig` 에 `PreIteration` 필드 추가 가능 여부 | **사용자 결정 필요**. 후방 호환 추가이지만 FROZEN 정책에 따라 별도 SPEC 이 될 수도. spec.md §6 에서 옵션 A/B/C 명시. |
| R-002 | 구현 패키지 위치 (`cmdctrl/` 의 부모 패키지) | spec.md §6.2 에서 옵션 A/B/C 명시 후 사용자 결정 권유 |
| R-003 | `RequestReactiveCompact(target)` 의 `target != 0` 처리 | **무시** (compactor 기본값 사용). 본 SPEC Exclusions 에 명시. 후속 SPEC (CONTEXT-COMPACT-TARGET-001 가칭) 으로 위임 |
| R-004 | `RequestModelChange` 시 OAuth refresh / credential pool swap | **본 SPEC 범위 외**. CMDCTX-CREDPOOL-WIRE-001 (가칭) 으로 위임. 본 SPEC 은 식별자 atomic swap 만 담당 |
| R-005 | `RequestModelChange(zero ModelInfo)` 거부 여부 | spec.md §4.4 Unwanted 에서 ErrInvalidModelInfo 반환 명시 |
| R-006 | engine lifecycle 미정의 (Close 후 Snapshot 호출) | **본 SPEC 범위 외**. engine non-nil & 활성 가정 |
| R-007 | request queue size (패턴 A 채택 시) | 패턴 C 채택으로 무관 (atomic flag) |
| R-008 | model swap atomic 의 visibility 보장 | atomic.Pointer 의 acquire/release semantics 로 충분 (Go 메모리 모델) |
| R-009 | `Snapshot()` 호출 시 token 정보 source | `loop.State` 에 누적 token 필드 없음. 옵션: (a) `LoopSnapshot.TokenCount/Limit` 을 0/0 으로 두고 후속 SPEC 위임, (b) `state.Messages` 에서 `TokenCountWithEstimation` 사용. **(a) 권장** — 후속 SPEC 으로 분리. |

---

## 10. 산출물 명세 요약

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| spec.md | ~500 | EARS 18-22 REQ + 18-22 AC, 기술 접근, 의존성, Exclusions |
| research.md | (본 파일) ~280 | wiring surface analysis, 결정 보류 항목 |
| progress.md | ~30 | phase log |

run phase 산출물 (예상, 본 SPEC 의 범위 외):

- `internal/query/cmdctrl/controller.go` (~150 LOC)
- `internal/query/cmdctrl/controller_test.go` (~250 LOC)
- `internal/query/cmdctrl/race_test.go` (~100 LOC)
- `internal/query/loop/loop.go` 미세 변경 (PreIteration 훅 추가, +5 LOC) — 옵션 C 채택 시
- `internal/query/engine.go` 미세 변경 (controller 주입 + drain hook 등록, +20 LOC) — 옵션 C 채택 시

---

Version: 0.1.0
Last Updated: 2026-04-27
