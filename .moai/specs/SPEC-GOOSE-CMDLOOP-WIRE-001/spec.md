---
id: SPEC-GOOSE-CMDLOOP-WIRE-001
version: 0.1.0
status: planned
created_at: 2026-04-27
updated_at: 2026-04-27
author: manager-spec
priority: P0
issue_number: null
phase: 2
size: 중(M)
lifecycle: spec-anchored
labels: [area/runtime, area/router, type/feature, priority/p0-critical]
---

# SPEC-GOOSE-CMDLOOP-WIRE-001 — LoopController 구현체 wiring (Slash Command → Query Loop)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. PR #52 (SPEC-GOOSE-CMDCTX-001 implemented) 머지로 `internal/command/adapter/` 패키지가 신설되어 `LoopController` 인터페이스가 정의되었으나 그 인터페이스의 **실제 구현체**가 아직 없음. 본 SPEC 은 query loop 측 wiring 구현 SPEC 을 신설. | manager-spec |

---

## 1. 개요 (Overview)

PR #52 (SPEC-GOOSE-CMDCTX-001) 머지로 `internal/command/adapter/controller.go` 에 `LoopController` 인터페이스가 정의되었으며, `ContextAdapter` 가 이 인터페이스에 의존해서 slash command (`/clear`, `/compact`, `/model`, `/status`) 의 부수 효과를 위임한다. 그러나 **이 인터페이스의 구현체는 아직 없다**. 결과적으로 dispatcher → adapter → `ErrLoopControllerUnavailable` 경로만 활성이고, 어떤 slash command 도 의미 있는 부수 효과를 일으키지 못한다.

본 SPEC 수락 시점에서:

- `internal/query/cmdctrl/` (또는 사용자 결정한 등가 경로 — §6.2) 패키지에 `LoopControllerImpl` 구조체가 존재하고 `adapter.LoopController` 를 구현한다.
- `RequestClear(ctx)` 는 다음 query loop iteration 진입 시 `state.Messages = nil`, `state.TurnCount = 0` 을 적용한다.
- `RequestReactiveCompact(ctx, target)` 는 다음 iteration 시작 시 `state.AutoCompactTracking.ReactiveTriggered = true` 를 set 한다. `target` 파라미터는 본 SPEC 에서 무시한다 (Exclusions §10 참조).
- `RequestModelChange(ctx, info)` 는 다음 `SubmitMessage` 진입 직전 atomic 으로 활성 model 식별자를 교체한다. **provider re-dial / credential pool swap 은 본 SPEC 범위 외**.
- `Snapshot()` 은 모든 goroutine 에서 race-clean 하게 `LoopSnapshot{TurnCount, Model, TokenCount, TokenLimit}` 의 read-only 복사본을 반환한다.
- 외부 goroutine 이 `loop.State` 를 직접 mutate 하지 않는다 (REQ-QUERY-015 single-owner invariant 보존).
- 모든 메서드는 `nil` ctx / engine 부재 / queue 가득 등 비정상 입력에 panic 없이 graceful degradation 한다.

본 SPEC 은 **LoopController 구현체** 만 정의한다. CMDCTX-001 의 인터페이스(`adapter.LoopController`) 변경, ContextAdapter 변경, dispatcher 변경, CLI 진입점 wiring 은 본 SPEC 범위 외.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **PR #52 후 dispatcher 활성화 불가**: CMDCTX-001 의 `ContextAdapter` 는 `LoopController` 가 nil 이면 모든 메서드가 `ErrLoopControllerUnavailable` 를 반환한다 (`adapter.go:111-115, 121-127, 133-139`). 즉, 아무리 dispatcher 를 wire 해도 본 SPEC 산출물 없이는 slash command 가 의미 있는 동작을 하지 못한다.
- **CLI / DAEMON-WIRE-001 의 명시적 의존**: CLI 진입점 SPEC 은 `adapter.New(Options{LoopController: ???})` 의 `???` 자리에 본 SPEC 의 `LoopControllerImpl` 인스턴스를 주입할 예정.
- **TRUST 5 Trackable 결손**: 인터페이스만 있고 구현체가 없는 상태가 SPEC 으로 명시되지 않으면 추적 불가능한 implicit 결손. 본 SPEC 으로 명시화.

### 2.2 상속 자산 (FROZEN)

- **SPEC-GOOSE-CMDCTX-001** (implemented, FROZEN, v0.1.1):
  - `internal/command/adapter/controller.go` 의 `LoopController` 인터페이스 (4 메서드)
  - `internal/command/adapter/controller.go` 의 `LoopSnapshot` 구조체 (4 필드)
  - 본 SPEC 은 이 인터페이스/타입 을 **구현** 한다. 변경하지 않는다.
- **SPEC-GOOSE-CONTEXT-001** (implemented, FROZEN):
  - `internal/context/compactor.go` 의 `DefaultCompactor.ShouldCompact` / `Compact` 의미론
  - `loop.State.AutoCompactTracking.ReactiveTriggered = true` 가 set 되면 ShouldCompact 가 즉시 true 반환
  - 본 SPEC 은 이 의미론을 **read-only 로 사용**. 변경하지 않는다.
- **SPEC-GOOSE-QUERY-001** (implemented, FROZEN, v0.1.3):
  - `internal/query/loop/state.go` 의 `loop.State` 구조
  - `internal/query/engine.go` 의 `QueryEngine.SubmitMessage` 진입점
  - REQ-QUERY-015: state 단일 소유자 invariant
  - 본 SPEC 은 이 invariant 를 **보존** 한다. state mutation 은 loop goroutine 단독.
  - **단**, `loop.LoopConfig` 에 `PreIteration` nil-tolerant 필드 1 개를 추가할 수 있다 (§6.4 옵션 C). 후방 호환 변경. QUERY-001 의 frontmatter version bump 가 필요한지 사용자 결정.

### 2.3 범위 경계 (한 줄)

본 SPEC 은 **LoopController 의 구현체만 wiring** 한다. 인터페이스 변경 / 새 slash command / model 의 actual provider re-dial / credential refresh 는 모두 범위 외.

---

## 3. 목표 (Goals)

### 3.1 IN SCOPE

| ID | 항목 | 목적 |
|----|------|-----|
| G-1 | `internal/query/cmdctrl/` (또는 등가) 에 `LoopControllerImpl` 구조체 신설 | adapter.LoopController 구현체 제공 |
| G-2 | `RequestClear` / `RequestReactiveCompact` / `RequestModelChange` / `Snapshot` 4 메서드 구현 | dispatcher → adapter → 본 SPEC → loop 경로 활성화 |
| G-3 | request enqueue → loop iteration drain 패턴 (lock-free atomic 기반) | REQ-QUERY-015 single-owner invariant 보존 |
| G-4 | `Snapshot()` 의 race-clean read-only 보장 | concurrent dispatcher 호출 안전 |
| G-5 | nil ctx / engine 부재 / 비정상 입력에 graceful degradation | TRUST 5 Secured |
| G-6 | race detector clean (`go test -race -count=10`) | concurrency 검증 |
| G-7 | 단위 테스트 + race 테스트 + 통합 테스트 (실제 DefaultCompactor 와 함께) | TRUST 5 Tested |

### 3.2 OUT OF SCOPE (Exclusions §10 에서 상세화)

- model swap 시 OAuth refresh / credential pool swap
- compact target 파라미터의 실제 처리 (compactor 가 target 을 받아 사용)
- engine lifecycle (Close, restart, shutdown)
- CLI 진입점 wiring (adapter instantiate, dispatcher wire)
- 새 slash command 추가
- LoopController 인터페이스 자체의 변경

---

## 4. 요구사항 (Requirements, EARS)

### 4.1 Ubiquitous Requirements (always active)

**REQ-CMDLOOP-001**: 시스템은 `adapter.LoopController` 인터페이스의 4개 메서드 (`RequestClear`, `RequestReactiveCompact`, `RequestModelChange`, `Snapshot`) 를 모두 구현해야 한다 (`shall implement`).

**REQ-CMDLOOP-002**: 시스템은 외부 goroutine (dispatcher 호출자) 이 `loop.State` 를 직접 변경하지 않도록 해야 한다 (`shall preserve REQ-QUERY-015 single-owner invariant`). 모든 state mutation 은 loop goroutine 의 iteration 진입 직후 또는 engine 의 `SubmitMessage` 진입 직전에만 발생한다.

**REQ-CMDLOOP-003**: 시스템은 `LoopControllerImpl` 의 모든 read 경로 (`Snapshot`) 가 lock-free 또는 RWMutex.RLock 기반으로 race-clean 해야 한다 (`shall be race-clean under -race -count=10`).

**REQ-CMDLOOP-004**: 시스템은 `RequestClear` / `RequestReactiveCompact` / `RequestModelChange` 의 enqueue 경로가 호출당 일정한 wall-clock 비용 (lock-free atomic 또는 1회 atomic.Store) 으로 완료되어야 한다 (`shall complete in O(1)`).

**REQ-CMDLOOP-005**: 시스템은 `LoopControllerImpl` 의 모든 메서드가 `nil` 수신자 (`*LoopControllerImpl == nil`) 또는 nil ctx 에 대해 panic 없이 동작해야 한다 (`shall not panic on nil receiver or nil ctx`).

**REQ-CMDLOOP-006**: 시스템은 `LoopControllerImpl` 가 `var _ adapter.LoopController = (*LoopControllerImpl)(nil)` 컴파일 타임 단언 (compile-time assertion) 을 통과해야 한다 (`shall satisfy adapter.LoopController interface at compile time`).

**REQ-CMDLOOP-007**: 시스템은 `LoopControllerImpl.Snapshot()` 이 반환하는 `LoopSnapshot` 의 모든 필드가 호출 시점 기준 가장 최근에 완료된 turn 의 값이거나 0 zero-value 이어야 한다 (`shall return last completed turn snapshot or zero-value`).

### 4.2 Event-Driven Requirements (when X then Y)

**REQ-CMDLOOP-008**: WHEN 외부 goroutine 이 `RequestClear(ctx)` 를 호출하면 THEN 시스템은 다음 query loop iteration 시작 시 `state.Messages = nil`, `state.TurnCount = 0` 을 적용해야 한다 (`shall apply Messages=nil, TurnCount=0 on next iteration entry`). 그 외 state 필드 (TaskBudgetRemaining, TokenLimit, MaxMessageCount, MaxOutputTokensRecoveryCount, AutoCompactTracking) 는 보존되어야 한다.

**REQ-CMDLOOP-009**: WHEN 외부 goroutine 이 `RequestReactiveCompact(ctx, target)` 를 호출하면 THEN 시스템은 다음 query loop iteration 시작 시 `state.AutoCompactTracking.ReactiveTriggered = true` 를 set 해야 한다. 결과적으로 `cfg.ShouldCompact(state)` 가 true 를 반환하고, `cfg.Compact(state)` 가 호출되어야 한다 (`shall set ReactiveTriggered=true on next iteration entry`).

**REQ-CMDLOOP-010**: WHEN 외부 goroutine 이 `RequestModelChange(ctx, info)` 를 호출하면 THEN 시스템은 즉시 (호출 반환 전) atomic 하게 활성 model 식별자를 `info` 로 교체해야 한다 (`shall atomically swap active model identifier before return`). 다음 `SubmitMessage` 호출 시 새 식별자가 LLM 호출 클로저에 반영된다.

**REQ-CMDLOOP-011**: WHEN 외부 goroutine 이 `Snapshot()` 을 호출하면 THEN 시스템은 engine 의 마지막 OnComplete 결과를 기반으로 `LoopSnapshot{TurnCount, Model, TokenCount, TokenLimit}` 를 즉시 반환해야 한다 (`shall return synchronously without blocking on in-flight SubmitMessage`).

**REQ-CMDLOOP-012**: WHEN ctx 가 이미 cancelled 되어 있을 때 enqueue 메서드가 호출되면 THEN 시스템은 `ctx.Err()` 를 반환하고 어떤 부수 효과도 일으키지 않아야 한다 (`shall return ctx.Err() and abort enqueue`).

### 4.3 State-Driven Requirements (while X)

**REQ-CMDLOOP-013**: WHILE query loop 가 진행 중 (활성 SubmitMessage 가 있고 종료되지 않음) 일 때 enqueue 메서드가 호출되면 THEN 시스템은 다음 iteration 진입 시 적용을 위해 request 를 보유해야 한다 (`shall hold request until next iteration`).

**REQ-CMDLOOP-014**: WHILE query loop 가 종료 상태 (Terminal yield 후) 또는 활성 SubmitMessage 가 없는 idle 상태일 때 `RequestClear` / `RequestReactiveCompact` 가 호출되면 THEN 시스템은 request 를 다음 SubmitMessage 가 시작되어 첫 iteration 에 도달할 때 적용되도록 보유해야 한다 (`shall persist request across idle-to-active transitions`). request 는 idempotent 하게 atomic flag 로 보유되므로 idle 동안 다중 호출은 단일 적용으로 합쳐진다.

**REQ-CMDLOOP-015**: WHILE 동일 종류의 enqueue 메서드 (예: `RequestClear`) 가 다중 호출되어 아직 적용되지 않은 상태일 때 THEN 시스템은 마지막 호출까지 모두 합쳐서 단일 적용으로 처리해야 한다 (`shall coalesce multiple pending requests of same kind`). 이는 atomic flag 의 idempotent 의미론에 의해 자연스럽게 달성된다.

### 4.4 Unwanted Behavior (shall not / shall reject)

**REQ-CMDLOOP-016**: 시스템은 외부 goroutine 으로부터 `loop.State` 의 어떤 필드도 직접 변경되지 않도록 해야 한다 (`shall not allow external mutation of loop.State`). state mutation 은 (a) loop goroutine 의 iteration 진입 직후 in-place 또는 (b) engine 의 `SubmitMessage` 진입 직전 single-owner 경로에서만 허용된다.

**REQ-CMDLOOP-017**: WHEN `RequestModelChange(ctx, command.ModelInfo{})` (zero-value ModelInfo, ID=="") 가 호출되면 THEN 시스템은 `ErrInvalidModelInfo` 를 반환하고 어떤 부수 효과도 일으키지 않아야 한다 (`shall reject zero-value ModelInfo`).

**REQ-CMDLOOP-018**: 시스템은 어떤 메서드도 panic 을 일으키지 않아야 한다 (`shall not panic`). nil engine, nil ctx, zero-value 입력, 동시 호출 등 모든 경로에서 안전.

### 4.5 Optional Requirements (where possible)

**REQ-CMDLOOP-019**: WHERE structured logger 가 주입되어 있으면 시스템은 각 enqueue 메서드의 호출과 적용 시점을 debug level 로 기록해야 한다 (`shall log enqueue and apply events when logger is provided`). logger 가 nil 이면 silent.

**REQ-CMDLOOP-020**: WHERE engine 이 nil 인 경우 시스템은 `ErrEngineUnavailable` 을 반환하고 graceful degradation 해야 한다 (`shall return ErrEngineUnavailable when engine is nil`). RequestClear/RequestReactiveCompact/RequestModelChange 는 enqueue 자체는 성공할 수 있으나 (atomic flag 만 set), Snapshot 은 zero-value 를 반환한다.

---

## 5. REQ ↔ AC 매트릭스

| REQ | AC | 관련 메서드 |
|-----|----|----------|
| REQ-CMDLOOP-001 | AC-CMDLOOP-006 | 전체 |
| REQ-CMDLOOP-002 | AC-CMDLOOP-016 (정적 분석) | 전체 |
| REQ-CMDLOOP-003 | AC-CMDLOOP-014 (race test) | 전체 |
| REQ-CMDLOOP-004 | AC-CMDLOOP-015 (벤치마크) | enqueue 3종 |
| REQ-CMDLOOP-005 | AC-CMDLOOP-018 | 전체 |
| REQ-CMDLOOP-006 | AC-CMDLOOP-006 | 전체 |
| REQ-CMDLOOP-007 | AC-CMDLOOP-005 | Snapshot |
| REQ-CMDLOOP-008 | AC-CMDLOOP-001 | RequestClear |
| REQ-CMDLOOP-009 | AC-CMDLOOP-002 | RequestReactiveCompact |
| REQ-CMDLOOP-010 | AC-CMDLOOP-003, AC-CMDLOOP-004, AC-CMDLOOP-017 | RequestModelChange |
| REQ-CMDLOOP-011 | AC-CMDLOOP-005 | Snapshot |
| REQ-CMDLOOP-012 | AC-CMDLOOP-007 | enqueue 3종 |
| REQ-CMDLOOP-013 | AC-CMDLOOP-008 | enqueue 3종 |
| REQ-CMDLOOP-014 | AC-CMDLOOP-009 | enqueue 3종 |
| REQ-CMDLOOP-015 | AC-CMDLOOP-010 | RequestClear/Compact |
| REQ-CMDLOOP-016 | AC-CMDLOOP-016 | 정적 분석 |
| REQ-CMDLOOP-017 | AC-CMDLOOP-011 | RequestModelChange |
| REQ-CMDLOOP-018 | AC-CMDLOOP-018 | 전체 |
| REQ-CMDLOOP-019 | AC-CMDLOOP-012 | 전체 |
| REQ-CMDLOOP-020 | AC-CMDLOOP-013 | 전체 |

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 데이터 모델

`LoopControllerImpl` 는 다음 lock-free 필드로 구성:

| 필드 | 타입 | 용도 |
|------|------|------|
| `pendingClear` | `atomic.Bool` | RequestClear → Swap(true). loop drain → Swap(false) 후 적용 |
| `pendingCompact` | `atomic.Bool` | RequestReactiveCompact → Swap(true). loop drain → Swap(false) 후 적용 |
| `activeModel` | `atomic.Pointer[command.ModelInfo]` | RequestModelChange → Store. SubmitMessage drain → Load |
| `engine` | `*query.QueryEngine` (또는 enginer interface) | Snapshot 시 engine.SnapshotState() 호출 |
| `logger` | `*zap.Logger` (optional) | debug level 로 enqueue/apply 기록 |

추가로 sentinel 에러:

```go
var (
    ErrEngineUnavailable = errors.New("query engine is unavailable")
    ErrInvalidModelInfo  = errors.New("invalid model info: ID must be non-empty")
)
```

### 6.2 패키지 위치 (사용자 결정 권장 옵션)

세 옵션을 제시. 본 SPEC 은 **옵션 C 를 권장** 한다. 사용자가 다른 옵션을 선택하면 ratify 단계에서 변경.

| 옵션 | 경로 | 장단점 |
|------|------|------|
| A | `internal/query/loop/cmdctrl/` | loop 내부에 위치 → state 변경이 loop 와 가까움. 단점: loop 패키지가 command 의존성 도입 |
| B | `internal/command/adapter/loopctrl/` | command 도메인 응집. 단점: query/engine 의 unexported 필드 접근 어려움 → engine 측 hook API 필요 |
| **C (권장)** | `internal/query/cmdctrl/` | query 패키지와 동일 모듈 트리, command/loop 양쪽 모두 import 가능, cycle 없음 |

옵션 C 의 import dependency:

```
internal/command/adapter (LoopController interface)
        ▲ implements
internal/query/cmdctrl (LoopControllerImpl, 본 SPEC 신규)
        ├── imports internal/command (ModelInfo)
        ├── imports internal/query/loop (State for snapshot conversion)
        └── imports internal/query (engine reference)

internal/query (engine)
        └── 신규 메서드 (옵션 C-i 또는 C-ii — §6.4 참조)
```

### 6.3 메서드별 알고리즘

#### `RequestClear(ctx context.Context) error`

```
1. if ctx == nil → ctx = context.Background()
2. if ctx.Err() != nil → return ctx.Err()
3. c.pendingClear.Store(true)
4. if logger != nil → logger.Debug("RequestClear enqueued")
5. return nil
```

#### `RequestReactiveCompact(ctx context.Context, target int) error`

```
1. if ctx == nil → ctx = context.Background()
2. if ctx.Err() != nil → return ctx.Err()
3. c.pendingCompact.Store(true)
4. if logger != nil → logger.Debug("RequestReactiveCompact enqueued", "target", target)
5. note: target 은 본 SPEC 에서 무시 (Exclusions §10 #2)
6. return nil
```

#### `RequestModelChange(ctx context.Context, info command.ModelInfo) error`

```
1. if ctx == nil → ctx = context.Background()
2. if ctx.Err() != nil → return ctx.Err()
3. if info.ID == "" → return ErrInvalidModelInfo
4. c.activeModel.Store(&info)
5. if logger != nil → logger.Debug("RequestModelChange enqueued", "id", info.ID)
6. return nil
```

#### `Snapshot() LoopSnapshot`

```
1. if c == nil → return LoopSnapshot{} (zero-value)
2. if c.engine == nil → return LoopSnapshot{} (zero-value)
3. state := c.engine.SnapshotState() // 신규 read-only 메서드, RLock 기반
4. var modelID string
5. if m := c.activeModel.Load(); m != nil → modelID = m.ID
6. else → modelID = engine.cfg.Provider 같은 default (또는 "")
7. return LoopSnapshot{
     TurnCount: state.TurnCount,
     Model: modelID,
     TokenCount: 0,  // §9 R-009: 후속 SPEC 위임
     TokenLimit: state.TokenLimit,
   }
```

### 6.4 loop iteration drain 훅 주입 (사용자 결정 필요)

본 SPEC 은 **옵션 C-i (loop.LoopConfig.PreIteration 추가)** 를 권장. 사용자가 거부하면 옵션 C-ii.

#### 옵션 C-i (권장): `loop.LoopConfig.PreIteration` 추가

`internal/query/loop/loop.go` 에 nil-tolerant 필드 1개 추가:

```go
type LoopConfig struct {
    ...
    // PreIteration 은 iteration 시작 직후, ShouldCompact 검사 이전에 호출되는 훅이다.
    // nil 이면 무시. state in-place mutation 허용 (loop goroutine 단독 소유 보장).
    PreIteration func(state *State)
}
```

`for {` 직후 (line 188) 에 1줄 추가:

```go
for {
    if cfg.PreIteration != nil { cfg.PreIteration(&state) }   // 신규
    if cfg.ShouldCompact != nil && cfg.ShouldCompact(state) { ... }
    ...
}
```

engine.go 의 LoopConfig 조립 시 `PreIteration: c.applyPendingRequests` 주입.

`applyPendingRequests` 정의 (controller.go 내):

```go
func (c *LoopControllerImpl) applyPendingRequests(state *loop.State) {
    if c.pendingClear.Swap(false) {
        state.Messages = nil
        state.TurnCount = 0
    }
    if c.pendingCompact.Swap(false) {
        state.AutoCompactTracking.ReactiveTriggered = true
    }
}
```

→ QUERY-001 의 frontmatter version bump 필요할 수도 (사용자 결정).

#### 옵션 C-ii (대안): engine 이 controller 를 인스턴스 변수로 보유

QUERY-001 의 loop.go / state.go 변경 없이, engine.go 만 변경:

- `QueryEngine` 에 `controller *cmdctrl.LoopControllerImpl` 필드 추가
- `SubmitMessage` 진입 시 `currentState.applyController(e.controller)` 같은 helper 호출
- loop iteration 진입 시점이 아닌 **SubmitMessage 진입 시점** 에서만 drain
- 단점: SubmitMessage 가 진행 중이면 RequestClear 가 그 turn 동안 적용 안 됨 → REQ-CMDLOOP-008 의 "다음 iteration" 의미가 "다음 SubmitMessage" 로 약화. 사용자 수용 가능 여부 결정 필요.

### 6.5 race-clean 보장

| 경합 시나리오 | 보호 메커니즘 |
|------------|------------|
| dispatcher RequestClear ↔ loop applyPendingRequests | atomic.Bool.Swap (lock-free, linearizable) |
| dispatcher RequestModelChange ↔ SubmitMessage activeModel.Load | atomic.Pointer (acquire/release semantics) |
| Snapshot ↔ OnComplete (e.state 쓰기) | engine 이 노출하는 SnapshotState() 가 stateMu RLock 사용 |
| 다중 dispatcher goroutine 동시 enqueue | 각 atomic 연산이 독립 → race 불가 |

### 6.6 graceful degradation

- nil ctx → context.Background() 로 대체
- ctx.Err() != nil → 즉시 ctx.Err() 반환, 부수 효과 없음
- nil engine → Snapshot 은 zero-value 반환, enqueue 는 atomic flag 만 set (적용 시점에 noop)
- nil receiver (`*LoopControllerImpl == nil`) → 모든 메서드가 zero-value/nil error 반환
- info.ID == "" → ErrInvalidModelInfo 즉시 반환

### 6.7 logger 통합

`*zap.Logger` 가 제공되면:

- enqueue 시 `logger.Debug("...enqueued", ...)`
- apply 시 (옵션 C-i 의 applyPendingRequests 내부) `logger.Debug("...applied")`
- error path 에서 `logger.Warn("...failed", "error", err)`

logger 가 nil 이면 silent. 본 SPEC 의 의미론에 영향 없음.

---

## 7. 의존성 (Dependencies)

| SPEC | status | 본 SPEC 의 사용 방식 |
|------|--------|----------------|
| SPEC-GOOSE-CMDCTX-001 | implemented (FROZEN, v0.1.1) | `LoopController` 인터페이스 + `LoopSnapshot` 타입 그대로 구현. `internal/command/adapter/controller.go` 수정 금지. |
| SPEC-GOOSE-CONTEXT-001 | implemented (FROZEN) | `DefaultCompactor.ShouldCompact` 가 `AutoCompactTracking.ReactiveTriggered` 를 참조하는 의미론 read-only 사용. `internal/context/compactor.go` 수정 금지. |
| SPEC-GOOSE-QUERY-001 | implemented (FROZEN, v0.1.3) | `loop.State` / `engine.QueryEngine` / `engine.SubmitMessage` 의미론 read-only 참조. **단**, 옵션 C-i 채택 시 `loop.LoopConfig.PreIteration` 필드 1개 추가 (후방 호환). 이로 인한 frontmatter 갱신 필요 여부는 사용자 결정. |

---

## 8. 정합성 / 비기능 요구사항

| 항목 | 기준 |
|------|------|
| 코드 주석 | 영어 (CLAUDE.local.md §2.5) |
| SPEC 본문 | 한국어 |
| 테스트 커버리지 | ≥ 90% (LSP quality gate, run phase) |
| race detector | `-race -count=10` PASS |
| golangci-lint | 0 issues |
| gofmt | clean |
| 정적 분석 | `grep -rE 'state\.[A-Z][A-Za-z]+\s*=' internal/query/cmdctrl/` 결과 0건 (PreIteration 콜백 인자 mutation 제외 — 명시적 화이트리스트) |
| LSP 진단 | 0 errors / 0 type errors / 0 lint errors (run phase) |

---

## 9. Risks (위험 영역)

| ID | 위험 | 영향 | 완화 |
|----|------|------|------|
| R-001 | QUERY-001 implemented status 불일치 — frontmatter 는 implemented 이지만 본 SPEC 이 가정하는 구체적 코드 구조가 실제와 다를 가능성 | 본 SPEC 의 전제 무너짐 | run phase Phase 1 (ANALYZE) 에서 `internal/query/loop/loop.go`, `state.go`, `engine.go` 의 실제 코드 vs research.md 정합성 재검증. 불일치 발견 시 SPEC 0.1.1 patch + plan-auditor 사이클. |
| R-002 | loop iteration 적용 시점의 atomic swap visibility | 외부 enqueue 가 다음 iteration 에 보이지 않을 수 있음 | atomic.Bool 의 acquire/release semantics (Go memory model) 로 충분. race detector 로 검증. 추가로 race_test.go 에서 enqueue → 1 iteration → state 검증 시나리오 포함. |
| R-003 | `PreIteration` 훅 추가가 QUERY-001 FROZEN 정책 위반으로 해석될 수 있음 | run phase 진입 차단 | 사용자 결정 필요. 본 SPEC §6.4 에서 옵션 C-i / C-ii 를 명시. C-i 가 거부되면 C-ii 로 fallback (의미론 약화 수용). |
| R-004 | request queue 크기 결정 (atomic flag 패턴 채택 시 무관) | 패턴 변경 시 영향 | atomic flag 패턴 (idempotent) 으로 queue full 시나리오 자체를 제거. 만약 사용자가 channel 기반 enqueue 를 원하면 spec 0.2.0 으로 분기. |
| R-005 | Snapshot 의 token 정보 source 부재 (loop.State 에 누적 token 필드 없음) | LoopSnapshot.TokenCount 가 항상 0 | §6.3 Snapshot 알고리즘에서 명시 — TokenCount=0, TokenLimit=state.TokenLimit. 후속 SPEC (CMDLOOP-TOKEN-WIRE-001 가칭) 으로 위임. Exclusions §10 #4. |
| R-006 | 동시 RequestModelChange 에서 last-write-wins 의미론 사용자 기대 불일치 | model 이 이전 호출로 되돌아가는 것처럼 보임 | atomic.Pointer 의 last-write-wins 가 slash command 의미론과 일치 (사용자 의도: 마지막 명령). race_test.go 에서 시나리오 명시. |
| R-007 | engine 측 SnapshotState() 신규 메서드 추가 시 QUERY-001 FROZEN 위반 가능성 | run phase 진입 차단 | 옵션 C-i 와 마찬가지로 후방 호환 추가. 사용자 결정 필요. 거부 시 engine.state 직접 접근 대신 `query.QueryEngine.LastTurnCount()` 같은 더 작은 단위 메서드로 분해. |

---

## 10. Exclusions (What NOT to Build)

본 SPEC 은 다음을 **수행하지 않는다**. 각 항목은 후속 SPEC 또는 명시적 결정으로 분리.

1. **OAuth refresh / credential pool swap on RequestModelChange**
   - 위임: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 (가칭, 본 SPEC 머지 후 별도 plan 필요)
   - 본 SPEC 은 model 식별자 atomic swap 만 담당. 실제 provider re-dial / API key rotation / OAuth 갱신은 범위 외.

2. **`RequestReactiveCompact(target)` 의 target 파라미터 실제 처리**
   - 위임: SPEC-GOOSE-CONTEXT-COMPACT-TARGET-001 (가칭, 본 SPEC 머지 후 별도 plan 필요)
   - 본 SPEC 은 target 을 무시하고 compactor 기본값 (`tokenLimit * 60 / 100`) 을 사용. CONTEXT-001 의 `Compact()` 시그니처 변경이 필요한 작업이므로 분리.

3. **engine lifecycle (Close, restart, shutdown)**
   - 위임: SPEC-GOOSE-QUERY-LIFECYCLE-001 (가칭, 후속 SPEC)
   - 본 SPEC 은 engine non-nil 가정. Close 후 Snapshot 호출은 정의되지 않은 동작.

4. **`LoopSnapshot.TokenCount` 의 실제 채움**
   - 위임: SPEC-GOOSE-CMDLOOP-TOKEN-WIRE-001 (가칭, 후속 SPEC)
   - 본 SPEC 은 TokenCount=0 fix. loop.State 에 누적 token 필드 추가가 필요하므로 QUERY-001 변경 동반 → 분리.

5. **CLI 진입점 wiring (adapter instantiate, dispatcher wire)**
   - 위임: SPEC-GOOSE-CLI-001 / SPEC-GOOSE-DAEMON-WIRE-001
   - 본 SPEC 은 LoopControllerImpl 만 제공. CLI 가 `cmdctrl.New(engine, logger)` 를 호출해서 인스턴스를 만들고 `adapter.New(Options{LoopController: c})` 에 주입하는 wiring 은 후속 SPEC.

6. **새 slash command 추가 (`/turn-budget`, `/permissions`, ...)**
   - 위임: 각 slash command 별 별도 SPEC
   - 본 SPEC 은 기존 4 메서드 (RequestClear/Compact/ModelChange/Snapshot) 만 wiring.

7. **`adapter.LoopController` 인터페이스 자체의 변경 (메서드 추가/시그니처 변경)**
   - 위임: SPEC-GOOSE-CMDCTX-001 follow-up (가칭, 본 SPEC 머지 후 별도 plan 필요)
   - 본 SPEC 은 인터페이스를 **구현** 만 한다. controller.go 수정 금지.

8. **`ContextAdapter` (CMDCTX-001 의 adapter.go) 의 변경**
   - 위임: 없음. CMDCTX-001 가 FROZEN 이므로 adapter.go 변경 불가.
   - 본 SPEC 은 adapter.go 를 read-only 로 사용.

9. **dispatcher (PR #50 SPEC-GOOSE-COMMAND-001) 의 변경**
   - 위임: 없음. COMMAND-001 가 FROZEN.

10. **multi-session / multi-engine wiring (한 controller 가 여러 engine 을 관리)**
    - 위임: 후속 SPEC
    - 본 SPEC 은 1 controller : 1 engine 관계. 다중 engine 은 다중 controller 인스턴스로.

11. **request queue size / backpressure / channel-based enqueue**
    - 위임: 향후 의미론 변경이 필요할 때만. 본 SPEC 은 atomic flag 패턴 (idempotent) 으로 queue 자체 없음.

12. **`PreIteration` 훅이 거부될 경우의 옵션 C-ii fallback path 의미론 변경 명시 (다음 SubmitMessage vs 다음 iteration)**
    - 위임: 사용자 결정에 따라 본 SPEC 0.1.1 patch 또는 후속 SPEC.

---

## 11. 참조 (References)

- `internal/command/adapter/controller.go:19-51` — LoopController 인터페이스 정의 (FROZEN)
- `internal/command/adapter/adapter.go:107-176` — ContextAdapter 의 OnClear/OnCompactRequest/OnModelChange/SessionSnapshot 위임 경로
- `internal/query/loop/state.go` — loop.State 구조 (FROZEN)
- `internal/query/loop/loop.go:188` — iteration 진입점 (옵션 C-i 의 PreIteration 훅 삽입 위치)
- `internal/query/engine.go:80-173` — SubmitMessage 진입점 (model swap 시점, OnComplete 콜백)
- `internal/context/compactor.go:118-147` — ShouldCompact / ReactiveTriggered 의미론
- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` — REQ-CMDCTX-006/007/008/016 (본 SPEC 이 위임받는 부분)
- `.moai/specs/SPEC-GOOSE-CONTEXT-001/spec.md` — REQ-CTX-007/017/018 (compactor 의미론)
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` — REQ-QUERY-015 (single-owner invariant)
- `CLAUDE.local.md §2.5` — 코드 주석 영어 정책

---

## 12. Acceptance Criteria

### AC-CMDLOOP-001 — RequestClear 적용

`RequestClear(ctx)` 호출 후 query loop 가 다음 iteration 에 진입하면, `state.Messages == nil` 이고 `state.TurnCount == 0` 임. 그 외 필드 (TaskBudgetRemaining, TokenLimit 등) 는 호출 직전 값과 동일.

검증 방법: unit test — fake loop 에 PreIteration 훅 주입 → `c.RequestClear(ctx)` → 1 iteration → state 검증.

### AC-CMDLOOP-002 — RequestReactiveCompact 적용

`RequestReactiveCompact(ctx, 0)` 호출 후 query loop 가 다음 iteration 에 진입하면, `state.AutoCompactTracking.ReactiveTriggered == true`. 결과적으로 `DefaultCompactor.ShouldCompact(state)` 가 true 반환.

검증 방법: integration test — 실제 `DefaultCompactor` 사용. `c.RequestReactiveCompact(ctx, 0)` → 1 iteration → `compactor.ShouldCompact(state) == true` 확인.

### AC-CMDLOOP-003 — RequestModelChange atomic swap

`RequestModelChange(ctx, command.ModelInfo{ID: "anthropic/claude-opus-4-7"})` 호출 직후, `c.activeModel.Load().ID == "anthropic/claude-opus-4-7"`. 두 번째 `RequestModelChange(ctx, command.ModelInfo{ID: "openai/gpt-4o"})` 호출 후 `c.activeModel.Load().ID == "openai/gpt-4o"`.

검증 방법: unit test.

### AC-CMDLOOP-004 — RequestModelChange last-write-wins (R-006)

서로 다른 goroutine 에서 동시에 `c.RequestModelChange(ctx, A)` 와 `c.RequestModelChange(ctx, B)` 를 호출. 두 호출 모두 nil 에러 반환. 그 직후 `c.activeModel.Load().ID` 는 `A.ID` 또는 `B.ID` 중 하나 (마지막 store 가 visible). race detector 미검출. 동일 시나리오를 단일 goroutine 내 순차 호출로 수행하면 마지막 호출의 값이 visible (A → B 호출 시 B.ID).

검증 방법: race_test.go 시나리오 + 순차 호출 unit test.

### AC-CMDLOOP-005 — Snapshot 동기 반환

`Snapshot()` 호출은 in-flight `SubmitMessage` 가 있더라도 blocking 없이 100ms 이내 반환. 반환된 `LoopSnapshot.TurnCount` 는 마지막 OnComplete 결과 또는 zero. `LoopSnapshot.Model` 은 마지막 RequestModelChange 결과 또는 ""/default. `LoopSnapshot.TokenCount == 0` (본 SPEC 범위 외, Exclusions §10 #4). `LoopSnapshot.TokenLimit` 은 `state.TokenLimit`.

검증 방법: sync test — goroutine A 가 SubmitMessage 진행 중인 상태에서 goroutine B 가 Snapshot 호출 → 100ms 이내 반환 + 값 검증.

### AC-CMDLOOP-006 — interface 컴파일 단언

`var _ adapter.LoopController = (*LoopControllerImpl)(nil)` 컴파일 성공. `go build ./internal/query/cmdctrl/...` 에러 없음.

검증 방법: 컴파일 (taken as code review check + CI build).

### AC-CMDLOOP-007 — ctx cancelled 거부

`ctx, cancel := context.WithCancel(...); cancel(); c.RequestClear(ctx)` 결과 `err == ctx.Err()` (`context.Canceled`). `c.pendingClear.Load() == false` (부수 효과 없음).

검증 방법: unit test.

### AC-CMDLOOP-008 — 활성 loop 중 enqueue 보유

활성 SubmitMessage 가 있는 동안 `c.RequestClear(ctx)` 호출 시 즉시 nil 반환. `c.pendingClear.Load() == true`. 다음 iteration 진입 시 자동 적용.

검증 방법: integration test.

### AC-CMDLOOP-009 — idle → active 전환 시 보유 유지

활성 SubmitMessage 가 없는 idle 상태에서 `c.RequestClear(ctx)` 호출 후, 새로 `engine.SubmitMessage(ctx, "...")` 호출. 첫 iteration 진입 시 `state.Messages = nil`, `state.TurnCount = 0` 적용.

검증 방법: integration test.

### AC-CMDLOOP-010 — multiple pending coalesce

`c.RequestClear(ctx)` 를 5번 연속 호출. 다음 iteration 진입 시 1번만 적용 (`state.Messages = nil` 1회, `pendingClear == false` 후).

검증 방법: unit test — 5 호출 후 1 iteration → state 검증 + `c.pendingClear.Load() == false`.

### AC-CMDLOOP-011 — zero-value ModelInfo 거부

`c.RequestModelChange(ctx, command.ModelInfo{})` 결과 `errors.Is(err, ErrInvalidModelInfo)`. `c.activeModel.Load() == nil` (또는 호출 전 값 보존).

검증 방법: unit test.

### AC-CMDLOOP-012 — logger 호출 검증

`fakeLogger` 주입 후 `c.RequestClear(ctx)` → fakeLogger.DebugCount >= 1, `c.RequestModelChange(ctx, command.ModelInfo{})` (zero-value) → fakeLogger.WarnCount >= 0 (logger 는 optional, warn 은 비필수).

검증 방법: unit test with fake logger.

### AC-CMDLOOP-013 — nil engine graceful

`c := cmdctrl.New(nil, nil); _ = c.RequestClear(context.Background())` panic 없음. `c.Snapshot() == LoopSnapshot{}` (zero-value).

검증 방법: unit test.

### AC-CMDLOOP-014 — race detector clean

`go test -race -count=10 ./internal/query/cmdctrl/...` PASS. 100 goroutines × 1000 iter 시나리오 (RequestClear/Compact/ModelChange/Snapshot 무작위 호출) 에서 race 미검출.

검증 방법: race_test.go.

### AC-CMDLOOP-015 — O(1) 호출 비용 (벤치마크)

`go test -bench=BenchmarkRequestClear -benchtime=1s ./internal/query/cmdctrl/...` 결과 ns/op 가 100ns 이하 (단일 atomic.Bool.Store 비용 + ctx.Err 검사). RequestReactiveCompact / RequestModelChange 동등.

검증 방법: 벤치마크 (run phase 에서 측정, 임계는 가이드라인).

### AC-CMDLOOP-016 — state mutation 정적 분석 (single-owner invariant)

`grep -rE 'state\.[A-Z][A-Za-z]+\s*=' internal/query/cmdctrl/` 결과 0건. `applyPendingRequests` 의 `state.Messages = nil` / `state.TurnCount = 0` / `state.AutoCompactTracking.ReactiveTriggered = true` 는 PreIteration 콜백의 인자 mutation 이므로 명시적 화이트리스트 (예: `// AC-CMDLOOP-016-WHITELIST` 주석 또는 별도 helper 함수). 화이트리스트 외 모든 직접 할당 0건.

검증 방법: CI 의 정적 분석 단계 + 코드 리뷰.

### AC-CMDLOOP-017 — RequestModelChange 즉시 visibility

`RequestModelChange(ctx, info)` 호출 반환 후 즉시 `c.activeModel.Load().ID == info.ID`. 별도 동기화 없이 다른 goroutine 에서도 관찰 가능 (atomic.Pointer 보장).

검증 방법: unit test.

### AC-CMDLOOP-018 — panic-free 모든 경로

다음 모든 입력 조합에 panic 미발생:

- nil receiver (`*LoopControllerImpl == nil`)
- nil ctx
- nil engine
- zero-value ModelInfo
- 동시 1000 goroutine 호출

검증 방법: unit + race test 통합.

### AC-CMDLOOP-019 — interface 만족 (컴파일 + 동작)

`adapter.New(Options{LoopController: cmdctrl.New(engine, logger)})` 가 컴파일되고, 결과 ContextAdapter 의 OnClear/OnCompactRequest/OnModelChange 가 nil error 반환 (즉, ErrLoopControllerUnavailable 미발생).

검증 방법: 통합 test — adapter + cmdctrl + engine 의 end-to-end 호출.

---

Version: 0.1.0
Last Updated: 2026-04-27
REQ coverage: REQ-CMDLOOP-001 ~ REQ-CMDLOOP-020 (총 20)
AC coverage: AC-CMDLOOP-001 ~ AC-CMDLOOP-019 (총 19)
