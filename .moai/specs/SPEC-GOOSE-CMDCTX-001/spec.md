---
id: SPEC-GOOSE-CMDCTX-001
version: 0.1.1
status: implemented
created_at: 2026-04-27
updated_at: 2026-04-27
author: manager-spec
priority: P1
issue_number: null
phase: 2
size: 중(M)
lifecycle: spec-anchored
labels: [area/cli, area/runtime, area/router, type/feature, priority/p1-high]
---

# SPEC-GOOSE-CMDCTX-001 — Slash Command Context Adapter (Wiring)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. PR #50 (SPEC-GOOSE-COMMAND-001 implemented)에서 정의된 `SlashCommandContext` 인터페이스의 구현체(adapter) wiring SPEC 신설. 6개 메서드를 ROUTER-001 / CONTEXT-001 / SUBAGENT-001 에 위임. | manager-spec |
| 0.1.1 | 2026-04-27 | plan-auditor iter1 FAIL 결함 수정: M1 (ErrUnknownModel stale claim 정정 — PR #50 internal/command/errors.go:23-25 에 이미 정의됨), M2 (planMode *atomic.Bool 포인터 indirection 으로 변경 — sync/atomic.Bool noCopy 위반 해소), M3 (AC-CMDCTX-019 신설 — adapter 비-mutation invariant 정적 분석), M4 (REQ-CMDCTX-016 §4.4 Unwanted → §4.1 Ubiquitous 재배치), M5 (AC-CMDCTX-016 확장 — logger.Warn 호출 검증 추가), N2 (Exclusions #7-9 placeholder 명시) | manager-spec |

---

## 1. 개요 (Overview)

PR #50 (SPEC-GOOSE-COMMAND-001) 머지로 `internal/command/context.go` 에 `SlashCommandContext` 인터페이스가 정의되었으나, **그 인터페이스의 구현체(adapter)는 wiring되지 않았다**. 본 SPEC은 그 빈 자리(런타임 어댑터)를 채운다.

본 SPEC 수락 시점에서:

- `internal/command/adapter/` (또는 등가 경로)에 `ContextAdapter` 구조체가 존재하고 `command.SlashCommandContext` 를 구현한다.
- `OnClear` / `OnCompactRequest` 는 `LoopController` 인터페이스(본 SPEC이 정의)를 통해 QUERY-001 loop / CONTEXT-001 compactor에 위임된다.
- `OnModelChange` / `ResolveModelAlias` 는 `*router.ProviderRegistry` (SPEC-GOOSE-ROUTER-001) 를 read-only로 조회한다.
- `PlanModeActive` 는 SUBAGENT-001 의 `TeammateIdentityFromContext` 와 adapter local atomic flag를 결합해서 판정한다.
- `SessionSnapshot` 은 `LoopController.Snapshot()` 결과 + `os.Getwd()` 로 구성한다.
- 모든 메서드는 `nil` 의존성에 대해 panic 없이 graceful degradation 한다.

본 SPEC은 **adapter layer만** 정의한다. `LoopController` 인터페이스의 구현체(QUERY-001 loop 측 wiring)는 후속 SPEC 또는 본 SPEC 의 범위 외 작업.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **PR #50 후 사용 불가능 상태**: COMMAND-001은 dispatcher가 `sctx SlashCommandContext` 를 인자로 받지만, 실제 호출자(미래의 `cmd/goose` 진입점 또는 query loop의 user-input 라우터)는 nil 또는 fake stub을 넘기는 임시 상태이다. 어떤 빌트인 명령(`/clear`, `/compact`, `/model`, `/status`)도 의미 있는 부작용을 일으키지 않는다.
- **CLI-001 / DAEMON-WIRE-001 가 본 SPEC에 의존**: CLI 진입점이 dispatcher를 사용하려면 `SlashCommandContext` 구현체를 제공해야 한다.
- **TRUST 5 Trackable 결손**: 인터페이스만 있고 구현체가 없으면 "wiring 미완료" 라는 implicit 결손이 추적되지 않는다. SPEC으로 명시.

### 2.2 상속 자산

- **SPEC-GOOSE-COMMAND-001** (implemented, FROZEN): `internal/command/context.go` 의 `SlashCommandContext` 인터페이스와 `ModelInfo`, `SessionSnapshot` 타입을 본 SPEC이 구현. 인터페이스 자체는 수정하지 않는다.
- **SPEC-GOOSE-ROUTER-001** (implemented, FROZEN): `*router.ProviderRegistry` 의 `Get/List/DefaultRegistry` API를 read-only 로 사용.
- **SPEC-GOOSE-CONTEXT-001** (implemented, FROZEN): `loop.State`, `AutoCompactTracking`, `DefaultCompactor` 의 의미론을 read-only 로 참조. mutate는 본 SPEC이 정의하는 `LoopController` 경유.
- **SPEC-GOOSE-SUBAGENT-001** (implemented, FROZEN): `TeammateIdentityFromContext`, `TeammateIdentity.PlanModeRequired` read-only 참조. plan mode registry mutate 금지.

### 2.3 범위 경계 (한 줄)

- **IN**: `ContextAdapter` 구현, `LoopController` 인터페이스 정의, alias 해석 헬퍼, 6개 메서드 unit test, race detector 통과.
- **OUT**: `LoopController` 의 실제 구현(QUERY-001 측 wiring), CLI 진입점에서 adapter를 instantiate 하는 코드, 모델 alias config 파일 로드.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/command/adapter/` 패키지 (또는 등가 경로).
   - `ContextAdapter` struct — `command.SlashCommandContext` 구현체.
   - `LoopController` interface — adapter가 QUERY-001 loop 와 통신하는 추상화.
   - `AliasResolver` 헬퍼 — `provider/model` 형태 alias 를 `ProviderRegistry` 와 매칭.
   - `Options` / `New(...)` 생성자 — registry, loop controller, optional alias map 주입.

2. 6개 `SlashCommandContext` 메서드 구현:
   - `OnClear() error` → `LoopController.RequestClear(ctx)` 위임.
   - `OnCompactRequest(target int) error` → `LoopController.RequestReactiveCompact(ctx, target)` 위임.
   - `OnModelChange(info ModelInfo) error` → `LoopController.RequestModelChange(ctx, info)` 위임.
   - `ResolveModelAlias(alias string) (*ModelInfo, error)` → registry + alias 테이블 lookup.
   - `SessionSnapshot() SessionSnapshot` → `LoopController.Snapshot()` + `os.Getwd()`.
   - `PlanModeActive() bool` → adapter local `*atomic.Bool` (포인터 공유 — WithContext children 간 단일 진실 공급원) ⊕ ctx-based `TeammateIdentity.PlanModeRequired`.

3. nil/error 경로 처리:
   - nil registry / nil loop controller: graceful error 반환 (panic 금지).
   - 미등록 alias: `command.ErrUnknownModel` (PR #50 SPEC-GOOSE-COMMAND-001 에서 `internal/command/errors.go:23-25` 에 이미 정의된 sentinel — 본 SPEC은 재사용).
   - `os.Getwd()` 실패: `"<unknown>"` placeholder.

4. 테스트:
   - table-driven unit test (`adapter_test.go`).
   - fake `LoopController` (channel 기반 command queue).
   - race detector 필수 (`go test -race`).
   - coverage ≥ 90%.

### 3.2 OUT OF SCOPE (명시적 제외)

- `LoopController` 의 실제 구현 (`internal/query/loop/` 또는 `internal/query/` 측 wiring) — 후속 SPEC.
- `cmd/goose` 진입점에서 `ContextAdapter` 를 instantiate 하고 dispatcher에 전달하는 코드 — CLI-001 / DAEMON-WIRE-001 범위.
- 모델 alias 의 config 파일 로드 (e.g., `~/.goose/aliases.yaml`) — 후속 SPEC.
- `OnModelChange` 후 active provider 의 OAuth refresh / credential pool swap — CREDPOOL-001 후속 wiring.
- Plan mode top-level orchestrator flag 의 외부 setter API (e.g., `/plan` slash command) — COMMAND-001 후속 SPEC.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CMDCTX-001** — The `ContextAdapter` **shall** implement `command.SlashCommandContext` interface (compile-time assertion via `var _ command.SlashCommandContext = (*ContextAdapter)(nil)`).

**REQ-CMDCTX-002** — The `ContextAdapter.ResolveModelAlias` method **shall** look up the provided alias against the injected `*router.ProviderRegistry` and an internal alias table, returning `(*ModelInfo, nil)` on hit or `(nil, command.ErrUnknownModel)` on miss without mutating any external state.

**REQ-CMDCTX-003** — The `ContextAdapter.SessionSnapshot` method **shall** return a `SessionSnapshot` value with `TurnCount` from the loop controller snapshot, `Model` from the current active model identifier, and `CWD` from `os.Getwd()` (or `"<unknown>"` on error).

**REQ-CMDCTX-004** — The `ContextAdapter.PlanModeActive` method **shall** return `true` if and only if the adapter's internal `*atomic.Bool` flag (shared with WithContext children) is set OR the calling context (when ctx-aware variant is used) carries a `TeammateIdentity` with `PlanModeRequired == true`. Default behavior (no ctx hint, flag unset) **shall** return `false`.

**REQ-CMDCTX-005** — All `ContextAdapter` methods **shall** be safe for concurrent invocation from multiple goroutines (race-free as verified by `go test -race`).

**REQ-CMDCTX-016** — The adapter **shall not** mutate `loop.State` directly. All loop state changes **shall** be routed through `LoopController` interface methods (preserves SPEC-GOOSE-CONTEXT-001 / SPEC-GOOSE-QUERY-001 REQ-QUERY-015 invariant: loop state is single-goroutine-owned).

### 4.2 Event-Driven (이벤트 기반)

**REQ-CMDCTX-006** — **When** `/clear` is dispatched and `OnClear()` is invoked, the `ContextAdapter` **shall** call `LoopController.RequestClear(ctx)` exactly once and return its error result.

**REQ-CMDCTX-007** — **When** `/compact [N]` is dispatched and `OnCompactRequest(target)` is invoked, the `ContextAdapter` **shall** call `LoopController.RequestReactiveCompact(ctx, target)` exactly once, where `target` is the user-supplied positive integer or 0 for "use compactor default".

**REQ-CMDCTX-008** — **When** `/model <alias>` is dispatched and `ResolveModelAlias(alias)` returns success, **then** the subsequent `OnModelChange(info)` **shall** call `LoopController.RequestModelChange(ctx, info)` exactly once and return its error result.

**REQ-CMDCTX-009** — **When** `ResolveModelAlias(alias)` is called with an alias of the form `provider/model`, the adapter **shall** split on the first `/`, look up the provider by name, then verify the model is in either the provider's `SuggestedModels` list or the configured alias table.

**REQ-CMDCTX-010** — **When** `SessionSnapshot()` is invoked and `os.Getwd()` returns a non-nil error, the adapter **shall** populate `CWD` with the placeholder string `"<unknown>"` and **shall not** return an error from `SessionSnapshot` (the interface signature does not allow it).

### 4.3 State-Driven (상태 기반)

**REQ-CMDCTX-011** — **While** the adapter's plan mode flag is set to `true` (e.g., via top-level orchestrator entering plan mode), invocations of `OnClear()` and `OnCompactRequest(target)` **shall** still execute (delegated to `LoopController`); **the dispatcher itself** is responsible for blocking mutating commands per `command.Metadata.Mutates` (REQ-CMD-011 of SPEC-GOOSE-COMMAND-001). The adapter's `PlanModeActive()` only reports the state; it does not enforce.

**REQ-CMDCTX-012** — **While** the calling context carries a `TeammateIdentity` with `PlanModeRequired == true`, every call to `PlanModeActive()` made within that context **shall** return `true` regardless of the adapter's local flag.

**REQ-CMDCTX-013** — **While** the adapter's `LoopController` dependency is non-nil and `RequestClear/RequestReactiveCompact/RequestModelChange` returns `nil` error, the adapter method's return value **shall** be `nil`. The adapter does not introspect the loop's eventual application of the request — it is a fire-and-forget signal.

### 4.4 Unwanted Behavior (방지)

**REQ-CMDCTX-014** — **If** the `*router.ProviderRegistry` injected at construction is `nil`, **then** `ResolveModelAlias` **shall** return `(nil, command.ErrUnknownModel)` for every input and **shall not** panic.

**REQ-CMDCTX-015** — **If** the `LoopController` dependency injected at construction is `nil`, **then** `OnClear / OnCompactRequest / OnModelChange` **shall** return a sentinel error `ErrLoopControllerUnavailable` and **shall not** panic. `SessionSnapshot()` **shall** return a `SessionSnapshot{TurnCount: 0, Model: "<unknown>", CWD: cwdOrFallback}` value.

(REQ-CMDCTX-016 은 §4.1 Ubiquitous 로 이동됨 — adapter 의 비-mutation 은 시스템 상시 불변이므로 Unwanted Behavior 보다 Ubiquitous 가 적절. v0.1.1 부터.)

### 4.5 Optional (선택적)

**REQ-CMDCTX-017** — **Where** an alias map config file (e.g., `~/.goose/aliases.yaml`) is provided to `New(...)` constructor, **then** the adapter **shall** consult that map first before falling back to `ProviderRegistry.SuggestedModels` lookup. (Implementation may stub this and return `nil` unless config is wired.)

**REQ-CMDCTX-018** — **Where** `os.Getwd()` is unavailable or blocked (e.g., chrooted environment, deleted CWD), the adapter **shall** populate `SessionSnapshot.CWD` with `"<unknown>"` and **shall** log a warning at WARN level (if a logger is injected). Logging is best-effort.

---

## 5. 수용 기준 (Acceptance Criteria)

| AC ID | 검증 대상 REQ | Given-When-Then |
|-------|---------------|-----------------|
| **AC-CMDCTX-001** | REQ-CMDCTX-001 | **Given** 컴파일된 `internal/command/adapter` 패키지 **When** `var _ command.SlashCommandContext = (*adapter.ContextAdapter)(nil)` 구문이 컴파일됨 **Then** 빌드 성공 |
| **AC-CMDCTX-002** | REQ-CMDCTX-002, REQ-CMDCTX-009 | **Given** `ContextAdapter` 가 `DefaultRegistry()` 로 초기화 **When** `ResolveModelAlias("anthropic/claude-opus-4-7")` 호출 **Then** `(*ModelInfo, nil)` 반환, ID="anthropic/claude-opus-4-7", DisplayName 비어있지 않음 |
| **AC-CMDCTX-003** | REQ-CMDCTX-002 | **Given** 위와 동일 **When** `ResolveModelAlias("nonexistent/foo")` 호출 **Then** `(nil, command.ErrUnknownModel)` 반환, panic 없음 |
| **AC-CMDCTX-004** | REQ-CMDCTX-003 | **Given** fake `LoopController.Snapshot()` 가 `loop.State{TurnCount: 7, ...}` 반환 **When** `SessionSnapshot()` 호출 **Then** 결과 `.TurnCount == 7`, `.CWD` 가 현재 working dir 또는 `"<unknown>"` |
| **AC-CMDCTX-005** | REQ-CMDCTX-006 | **Given** fake `LoopController` 의 `RequestClear` 호출 카운터 0 **When** `OnClear()` 호출 **Then** 카운터 1 증가, 반환 에러 nil |
| **AC-CMDCTX-006** | REQ-CMDCTX-007 | **Given** fake `LoopController` 의 `RequestReactiveCompact` 호출 인자 capture **When** `OnCompactRequest(50000)` 호출 **Then** capture 된 target == 50000 |
| **AC-CMDCTX-007** | REQ-CMDCTX-007 | **Given** 위와 동일 **When** `OnCompactRequest(0)` 호출 **Then** capture 된 target == 0 (compactor 기본값 사용 시그널) |
| **AC-CMDCTX-008** | REQ-CMDCTX-008 | **Given** fake `LoopController` 의 `RequestModelChange` 호출 카운터 0 **When** `OnModelChange(ModelInfo{ID:"anthropic/claude-opus-4-7", ...})` 호출 **Then** 카운터 1, capture 된 ID 일치 |
| **AC-CMDCTX-009** | REQ-CMDCTX-004, REQ-CMDCTX-012 | **Given** ctx 에 `TeammateIdentity{PlanModeRequired: true}` 주입, adapter local flag false **When** ctx-aware `PlanModeActive` 변형 호출 (또는 `WithContext` 사용) **Then** `true` 반환 |
| **AC-CMDCTX-010** | REQ-CMDCTX-004 | **Given** adapter local flag false, ctx 에 TeammateIdentity 없음 **When** `PlanModeActive()` 호출 **Then** `false` 반환 |
| **AC-CMDCTX-011** | REQ-CMDCTX-014 | **Given** `New(nil registry, fakeLoop, ...)` 로 생성 **When** `ResolveModelAlias("anthropic/claude-opus-4-7")` 호출 **Then** `(nil, command.ErrUnknownModel)` 반환, panic 없음 |
| **AC-CMDCTX-012** | REQ-CMDCTX-015 | **Given** `New(registry, nil loopController, ...)` 로 생성 **When** `OnClear()` 호출 **Then** `ErrLoopControllerUnavailable` sentinel 반환, panic 없음 |
| **AC-CMDCTX-013** | REQ-CMDCTX-015 | **Given** 위와 동일 **When** `SessionSnapshot()` 호출 **Then** `SessionSnapshot{TurnCount: 0, Model: "<unknown>", CWD: ...}` 반환 |
| **AC-CMDCTX-014** | REQ-CMDCTX-005, REQ-CMDCTX-016 | **Given** ContextAdapter 인스턴스, 100 goroutine 동시에 6개 메서드 무작위 호출 (1000 iteration each) **When** `go test -race -count=10` 실행 **Then** race condition 0건, panic 0건 |
| **AC-CMDCTX-015** | REQ-CMDCTX-013 | **Given** fake LoopController.RequestClear 가 nil 반환 **When** `OnClear()` 호출 **Then** 반환 에러 nil; **Given** fake가 sentinel error 반환 **When** 위와 동일 호출 **Then** 그 에러가 그대로 전파 |
| **AC-CMDCTX-016** | REQ-CMDCTX-010, REQ-CMDCTX-018 | **Given** mock `getwdFn` 이 에러 반환하도록 주입(`fakeGetwdFn override`) **And** logger 가 `fakeWarnLogger` 로 주입됨 **When** `SessionSnapshot()` 호출 **Then** 반환값 `.CWD == "<unknown>"` **And** 함수 자체는 panic 없이 정상 반환 **And** `fakeWarnLogger.WarnCount >= 1` **And** Warn 메시지에 `os.Getwd` 가 반환한 원본 에러가 포함됨 (REQ-CMDCTX-018 logger 검증). logger 가 nil 인 경우 best-effort 에 따라 Warn 호출 생략은 허용. |
| **AC-CMDCTX-017** | REQ-CMDCTX-017 | **Given** `New(...)` 에 alias map `{"opus": "anthropic/claude-opus-4-7"}` 주입 **When** `ResolveModelAlias("opus")` 호출 **Then** `(*ModelInfo{ID:"anthropic/claude-opus-4-7"}, nil)` 반환 |
| **AC-CMDCTX-018** | REQ-CMDCTX-011 | **Given** adapter plan mode flag true, fakeLoop carrier **When** `OnClear()` 호출 **Then** `LoopController.RequestClear` 호출 1회 발생 (adapter는 차단하지 않음 — 차단은 dispatcher 책임) |
| **AC-CMDCTX-019** | REQ-CMDCTX-016 | **Given** `internal/command/adapter/` 패키지 코드 **When** 정적 분석 실행: `grep -rE 'loop\.State\.[A-Z][A-Za-z]*\s*=' internal/command/adapter/ --include='*.go' \| grep -v '_test.go'` **Then** 매칭 0건 (export 필드 직접 할당 부재) **And** REQ-CMDCTX-016 가 입증된다 — 모든 loop.State 변경은 LoopController 메서드 경유 |

**커버리지 매트릭스**:

| REQ | AC들 |
|-----|------|
| REQ-CMDCTX-001 | AC-CMDCTX-001 |
| REQ-CMDCTX-002 | AC-CMDCTX-002, AC-CMDCTX-003, AC-CMDCTX-011 |
| REQ-CMDCTX-003 | AC-CMDCTX-004, AC-CMDCTX-013 |
| REQ-CMDCTX-004 | AC-CMDCTX-009, AC-CMDCTX-010 |
| REQ-CMDCTX-005 | AC-CMDCTX-014 |
| REQ-CMDCTX-006 | AC-CMDCTX-005, AC-CMDCTX-015 |
| REQ-CMDCTX-007 | AC-CMDCTX-006, AC-CMDCTX-007 |
| REQ-CMDCTX-008 | AC-CMDCTX-008 |
| REQ-CMDCTX-009 | AC-CMDCTX-002 |
| REQ-CMDCTX-010 | AC-CMDCTX-016 |
| REQ-CMDCTX-011 | AC-CMDCTX-018 |
| REQ-CMDCTX-012 | AC-CMDCTX-009 |
| REQ-CMDCTX-013 | AC-CMDCTX-015 |
| REQ-CMDCTX-014 | AC-CMDCTX-011 |
| REQ-CMDCTX-015 | AC-CMDCTX-012, AC-CMDCTX-013 |
| REQ-CMDCTX-016 | AC-CMDCTX-014 (race), AC-CMDCTX-019 (정적 분석 — adapter 비-mutation invariant) |
| REQ-CMDCTX-017 | AC-CMDCTX-017 |
| REQ-CMDCTX-018 | AC-CMDCTX-016 |

총 18 REQ / 19 AC. 모든 REQ 가 최소 1개의 AC로 검증된다.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/command/
├── context.go                  # SlashCommandContext (FROZEN, COMMAND-001 자산)
├── dispatcher.go               # FROZEN, COMMAND-001 자산
├── builtin/                    # FROZEN, COMMAND-001 자산
└── adapter/                    # ⬅︎ 본 SPEC 신규
    ├── adapter.go              # ContextAdapter struct + 6 메서드 구현
    ├── adapter_test.go         # table-driven test
    ├── controller.go           # LoopController interface 정의
    ├── controller_test.go      # interface 컴파일 assertion
    ├── alias.go                # AliasResolver helper
    ├── alias_test.go
    ├── errors.go               # ErrLoopControllerUnavailable sentinel
    └── fakes_test.go           # fakeLoopController, fakeRegistry helpers
```

`command.ErrUnknownModel` 은 PR #50 (SPEC-GOOSE-COMMAND-001)에서 `internal/command/errors.go:23-25` 에 이미 정의됨. 본 SPEC은 **재사용**.

### 6.2 핵심 타입 (Go 시그니처)

```go
// Package adapter wires SlashCommandContext to the runtime: router registry,
// query loop controller, and subagent plan-mode awareness.
package adapter

import (
    "context"
    "errors"
    "os"
    "sync/atomic"

    "github.com/modu-ai/goose/internal/command"
    "github.com/modu-ai/goose/internal/llm/router"
    "github.com/modu-ai/goose/internal/query/loop"
    "github.com/modu-ai/goose/internal/subagent"
)

// LoopController is the abstraction the adapter uses to communicate with the
// query loop without violating the loop's single-owner invariant
// (SPEC-GOOSE-QUERY-001 REQ-QUERY-015).
//
// @MX:ANCHOR: Boundary between command adapter and query loop.
// @MX:REASON: All side-effecting slash commands route through this interface.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-001 REQ-CMDCTX-006/007/008/016
type LoopController interface {
    // RequestClear signals the loop to reset Messages and TurnCount on the
    // next iteration. Fire-and-forget; returns error only if the request
    // could not be enqueued.
    RequestClear(ctx context.Context) error

    // RequestReactiveCompact signals the loop to set
    // AutoCompactTracking.ReactiveTriggered = true on the next iteration.
    // target == 0 means "use compactor default".
    RequestReactiveCompact(ctx context.Context, target int) error

    // RequestModelChange signals the loop to swap the active model on the
    // next submitMessage. The adapter does not validate info; ResolveModelAlias
    // does that upstream.
    RequestModelChange(ctx context.Context, info command.ModelInfo) error

    // Snapshot returns a deep-enough copy of the loop state for read-only
    // inspection by /status. Called from any goroutine.
    Snapshot() LoopSnapshot
}

// LoopSnapshot is the read-only view of loop state surfaced to the adapter.
type LoopSnapshot struct {
    TurnCount   int
    Model       string
    TokenCount  int64
    TokenLimit  int64
}

// ContextAdapter implements command.SlashCommandContext by composing
// router (read-only), loop controller (write), and subagent plan-mode
// awareness (read-only).
//
// @MX:ANCHOR: Concrete SlashCommandContext implementation.
// @MX:REASON: Single instance per CLI/daemon process.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-001 REQ-CMDCTX-001
type ContextAdapter struct {
    registry   *router.ProviderRegistry  // may be nil → REQ-CMDCTX-014
    loopCtrl   LoopController            // may be nil → REQ-CMDCTX-015
    aliasMap   map[string]string         // optional, may be empty
    // planMode is a *atomic.Bool (pointer indirection) so that WithContext
    // children share the same underlying flag without copying the atomic
    // (sync/atomic.Bool carries a noCopy guard; copying triggers go vet
    // copylocks). SetPlanMode on the parent is observed by all children.
    planMode   *atomic.Bool              // top-level orchestrator plan flag, shared
    getwdFn    func() (string, error)    // injectable for testing
    logger     Logger                    // optional, may be nil
    // ctxHook is the optional context that carries TeammateIdentity for
    // PlanModeActive checks. Set via WithContext for sub-agent calls.
    ctxHook    context.Context
}

// Options is the constructor parameter bag.
type Options struct {
    Registry       *router.ProviderRegistry
    LoopController LoopController
    AliasMap       map[string]string
    GetwdFn        func() (string, error) // defaults to os.Getwd
    Logger         Logger
}

// New constructs a ContextAdapter with the given options. nil dependencies
// are tolerated (graceful degradation per REQ-CMDCTX-014, -015).
// New always allocates a fresh *atomic.Bool for planMode so that WithContext
// children share state with the parent.
//
//     return &ContextAdapter{
//         planMode: new(atomic.Bool),
//         ...
//     }
func New(opts Options) *ContextAdapter

// SetPlanMode toggles the top-level orchestrator plan-mode flag.
// Called by future /plan command implementation (out of scope here).
// Because planMode is *atomic.Bool, all WithContext children observe the
// same flag value (REQ-CMDCTX-005, REQ-CMDCTX-011).
func (a *ContextAdapter) SetPlanMode(active bool)

// WithContext returns a new ContextAdapter that uses the provided ctx for
// PlanModeActive lookups. The original adapter is not modified.
// The returned clone is a shallow copy: registry, loopCtrl, aliasMap, logger,
// getwdFn, and the *atomic.Bool planMode pointer are all shared. Only
// ctxHook differs. This is safe because the only mutable shared state
// (planMode) is accessed via atomic operations.
//
//     clone := *a              // shallow copy is safe — atomic.Bool is via pointer
//     clone.ctxHook = ctx
//     return &clone
func (a *ContextAdapter) WithContext(ctx context.Context) *ContextAdapter

// SlashCommandContext implementation.
var _ command.SlashCommandContext = (*ContextAdapter)(nil)

func (a *ContextAdapter) OnClear() error
func (a *ContextAdapter) OnCompactRequest(target int) error
func (a *ContextAdapter) OnModelChange(info command.ModelInfo) error
func (a *ContextAdapter) ResolveModelAlias(alias string) (*command.ModelInfo, error)
func (a *ContextAdapter) SessionSnapshot() command.SessionSnapshot
func (a *ContextAdapter) PlanModeActive() bool

// Errors.
var ErrLoopControllerUnavailable = errors.New("adapter: LoopController is nil")

// Logger is the minimal logging interface the adapter needs.
type Logger interface {
    Warn(msg string, fields ...any)
}
```

### 6.3 의존성 주입 패턴

CLI / daemon 진입점(본 SPEC 범위 외)이 다음과 같이 wiring:

```go
// pseudocode — actual wiring lives in CLI-001 / DAEMON-WIRE-001
reg := router.DefaultRegistry()
loopCtrl := query.NewLoopController(engine)  // 후속 SPEC

adapter := adapter.New(adapter.Options{
    Registry:       reg,
    LoopController: loopCtrl,
    AliasMap:       loadAliasMap(),  // optional
    Logger:         zapAdapter,
})

dispatcher := command.NewDispatcher(reg, command.Config{}, logger)
// every user input:
dispatcher.ProcessUserInput(ctx, input, adapter.WithContext(ctx))
```

### 6.4 alias 해석 알고리즘

```
ResolveModelAlias(alias):
  1. if registry == nil:
       return nil, ErrUnknownModel
  2. if alias is in aliasMap:
       canonical := aliasMap[alias]
       fall through with canonical
     else:
       canonical := alias
  3. parts := strings.SplitN(canonical, "/", 2)
  4. if len(parts) != 2:
       return nil, ErrUnknownModel
  5. provider, model := parts[0], parts[1]
  6. meta, ok := registry.Get(provider)
     if !ok:
       return nil, ErrUnknownModel
  7. if model NOT in meta.SuggestedModels:
       (allow-list strict mode: return ErrUnknownModel)
       (alternative: permissive mode: still return ModelInfo since registry
        does not enumerate every model — config-driven)
       Default: strict.
  8. return &ModelInfo{
       ID:          provider + "/" + model,
       DisplayName: meta.DisplayName + " " + model,
     }, nil
```

### 6.5 PlanModeActive 결합 알고리즘

```
PlanModeActive():
  1. if a.planMode != nil && a.planMode.Load() == true: return true
     // (a.planMode 는 New(...) 가 항상 new(atomic.Bool) 로 채우므로 일반 경로에서는
     //  nil 이 아니다. defensive check 만 표기.)
  2. if a.ctxHook != nil:
       id, ok := subagent.TeammateIdentityFromContext(a.ctxHook)
       if ok && id.PlanModeRequired: return true
  3. return false

WithContext(ctx):
  clone := *a              // shallow copy: planMode pointer 는 그대로 공유
  clone.ctxHook = ctx
  return &clone

SetPlanMode(active):
  a.planMode.Store(active)  // 부모/자식 adapter 모두에서 즉시 관찰됨
```

### 6.6 race 안전성

- `planMode *atomic.Bool` — pointer indirection. Load/Store atomic, no mutex. `WithContext` children share the same pointer; `SetPlanMode` on parent is observed by all children atomically. **이 포인터 indirection 은 `sync/atomic.Bool` 의 `noCopy` 가드를 위반하지 않기 위함이다** (값 타입으로 둘 경우 `WithContext` 의 shallow-copy 가 `go vet copylocks` 경고를 유발).
- `aliasMap` — read-only after `New(...)`; no concurrent mutation
- `registry` — read-only (ROUTER-001 invariant)
- `loopCtrl` — interface-level concurrency contract (구현체 책임)
- `ctxHook` — `WithContext` 가 새 인스턴스를 반환하므로 immutable per-invocation. clone 은 shallow copy 이지만 mutable shared state 는 `*atomic.Bool` 한 개 뿐이며 atomic operation 으로만 접근.

**상태 공유 invariant**: WithContext 로 파생된 모든 child adapter 는 부모와 `planMode *atomic.Bool` 포인터를 공유한다. 따라서 `parent.SetPlanMode(true)` 호출은 모든 자식의 `PlanModeActive()` 호출에서 즉시(atomic ordering 보장 내) 관찰된다. 이는 plan-mode 토글이 단일 진실 공급원(single source of truth)임을 보장한다.

### 6.7 TDD 진입 순서 (RED → GREEN → REFACTOR)

| 순서 | 작업 | 검증 |
|------|------|-----|
| T-001 | 타입 정의 (struct, interface, errors) | 컴파일 |
| T-002 | `ResolveModelAlias` (read-only) | AC-CMDCTX-002, -003, -011 |
| T-003 | `SessionSnapshot` | AC-CMDCTX-004, -013, -016 |
| T-004 | `PlanModeActive` | AC-CMDCTX-009, -010 |
| T-005 | `OnClear` / `OnCompactRequest` | AC-CMDCTX-005, -006, -007, -012, -015 |
| T-006 | `OnModelChange` | AC-CMDCTX-008 |
| T-007 | race test + nil paths | AC-CMDCTX-014, AC-CMDCTX-018 |

### 6.8 TRUST 5 매핑

| 차원 | 본 SPEC 적용 |
|------|-----------|
| Tested | 18 AC, ≥ 90% coverage, race detector pass |
| Readable | godoc on every exported type, English code comments per `language.yaml` |
| Unified | gofmt + golangci-lint clean |
| Secured | nil dependency graceful degradation, no panic paths |
| Trackable | conventional commits, SPEC ID in commit body, MX:ANCHOR on `LoopController` |

### 6.9 의존성 결정 (라이브러리)

- `sync/atomic` (stdlib) — atomic.Bool
- `context` (stdlib)
- `os` (stdlib) — Getwd
- `errors` (stdlib)
- `github.com/modu-ai/goose/internal/command` — interface, ModelInfo, SessionSnapshot
- `github.com/modu-ai/goose/internal/llm/router` — ProviderRegistry
- `github.com/modu-ai/goose/internal/query/loop` — State, AutoCompactTracking (read-only types only)
- `github.com/modu-ai/goose/internal/subagent` — TeammateIdentityFromContext
- `github.com/stretchr/testify/assert` — 기존 사용 패턴 따름 (테스트 only)

신규 외부 의존성 없음.

---

## 7. 의존성 (Dependencies)

| 종류 | 대상 SPEC | 관계 |
|------|---------|------|
| 소비자 | SPEC-GOOSE-COMMAND-001 (implemented) | 본 SPEC이 정의하는 `ContextAdapter` 가 `SlashCommandContext` 를 구현 |
| 위임 대상 | SPEC-GOOSE-ROUTER-001 (implemented) | `*router.ProviderRegistry` read-only 사용 |
| 위임 대상 | SPEC-GOOSE-CONTEXT-001 (implemented) | `loop.State`, `AutoCompactTracking` read-only 참조 |
| 위임 대상 | SPEC-GOOSE-SUBAGENT-001 (implemented) | `TeammateIdentityFromContext`, `TeammateIdentity.PlanModeRequired` read-only |
| 후속 의존자 | (가칭) SPEC-GOOSE-CMDLOOP-WIRE-001 | `LoopController` interface 의 실제 구현 |
| 후속 의존자 | SPEC-GOOSE-CLI-001 / SPEC-GOOSE-DAEMON-WIRE-001 | adapter 인스턴스화 + dispatcher 주입 |

본 SPEC은 의존 SPEC들을 **변경하지 않는다** (모두 implemented status, FROZEN). 본 SPEC이 추가하는 것:

- `internal/command/adapter/` 패키지 (신규)
- `command.ErrUnknownModel` 은 PR #50 (SPEC-GOOSE-COMMAND-001)에서 `internal/command/errors.go:23-25` 에 이미 정의됨. 본 SPEC은 **재사용** (추가/수정 없음).
- `command.SlashCommandContext` 인터페이스 시그니처 자체는 수정 금지

---

## 8. Acceptance Test 전략

### 8.1 표 기반 (table-driven) 단위 테스트

```go
// adapter_test.go (의도 시그니처)
func TestContextAdapter_ResolveModelAlias(t *testing.T) {
    cases := []struct {
        name       string
        registry   *router.ProviderRegistry
        aliasMap   map[string]string
        input      string
        wantID     string
        wantErr    error
    }{
        {"happy_provider_slash_model", router.DefaultRegistry(), nil, "anthropic/claude-opus-4-7", "anthropic/claude-opus-4-7", nil},
        {"alias_map_hit", router.DefaultRegistry(), map[string]string{"opus": "anthropic/claude-opus-4-7"}, "opus", "anthropic/claude-opus-4-7", nil},
        {"unknown_provider", router.DefaultRegistry(), nil, "nonexistent/foo", "", command.ErrUnknownModel},
        {"unknown_model", router.DefaultRegistry(), nil, "anthropic/not-a-model", "", command.ErrUnknownModel},
        {"nil_registry", nil, nil, "anthropic/claude-opus-4-7", "", command.ErrUnknownModel},
        {"malformed_alias", router.DefaultRegistry(), nil, "no-slash", "", command.ErrUnknownModel},
        {"empty_alias", router.DefaultRegistry(), nil, "", "", command.ErrUnknownModel},
    }
    for _, tc := range cases { /* ... */ }
}
```

### 8.2 Fake LoopController

```go
// fakes_test.go
type fakeLoopController struct {
    mu                   sync.Mutex
    clearCount           int
    compactRequests      []int
    modelChanges         []command.ModelInfo
    snapshot             LoopSnapshot
    nextErr              error
}

func (f *fakeLoopController) RequestClear(ctx context.Context) error
func (f *fakeLoopController) RequestReactiveCompact(ctx context.Context, target int) error
func (f *fakeLoopController) RequestModelChange(ctx context.Context, info command.ModelInfo) error
func (f *fakeLoopController) Snapshot() LoopSnapshot
```

### 8.3 Race detector

```bash
go test -race -count=10 ./internal/command/adapter/...
```

`AC-CMDCTX-014` 의 100 goroutine × 1000 iter 테스트 포함. Coverage 측정은 `-race` 모드 별도 실행 (race + cover 동시는 부정확).

### 8.4 Coverage 목표

- 라인 커버리지: ≥ 90%
- branch 커버리지 (gocov-xml): ≥ 85%
- 6 메서드 모두 happy path + nil path + error path 검증

### 8.5 Lint / Format 게이트

- `gofmt -l . | grep . && exit 1` — clean
- `golangci-lint run ./internal/command/adapter/...` — 0 issues
- godoc on every exported identifier (Go convention)

---

## 9. 리스크 & 완화 (Risks & Mitigations)

| 리스크 | 영향 | 완화 |
|--------|----|------|
| R1 — `LoopController` 구현이 본 SPEC과 별도 SPEC에 정의됨 → 통합 시점에 시그니처 불일치 | 중 | 본 SPEC이 인터페이스를 명확히 고정. 후속 SPEC은 이 인터페이스를 그대로 구현. compile-time assertion으로 검증. |
| R2 — `ResolveModelAlias` 의 strict mode 가 사용자 친화성을 해침 (SuggestedModels 에 없는 정당한 모델 거부) | 중 | Optional REQ-CMDCTX-017 이 alias map override 경로 제공. 후속 wiring SPEC에서 permissive mode 옵션 추가 가능. |
| R3 — `PlanModeActive` 의 ctx-aware 변형 미사용 시 sub-agent plan mode 누수 | 중 | `WithContext` 패턴을 dispatcher 호출 시점에 강제 (테스트로 회귀 방지). |
| R4 — race detector 의 false positive 또는 미검출 | 저 | `-count=10` 반복, atomic 우선 사용으로 mutex contention 자체 회피. |
| R5 — `os.Getwd()` 가 macOS 의 deleted CWD 에서 hang | 저 | `getwdFn` 주입 가능 → CLI 측에서 timeout wrapper 가능. 본 SPEC scope 외. |

---

## 10. 참고 (References)

### 10.1 프로젝트 문서

- `internal/command/context.go` — `SlashCommandContext` 인터페이스 원본 (FROZEN)
- `internal/command/dispatcher.go` — dispatcher 호출 진입점
- `internal/command/builtin/*.go` — 6 빌트인 명령
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` — 부모 SPEC
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` — registry / model resolution
- `.moai/specs/SPEC-GOOSE-CONTEXT-001/spec.md` — compactor + loop state
- `.moai/specs/SPEC-GOOSE-SUBAGENT-001/spec.md` — plan mode (REQ-SA-022)
- `internal/llm/router/registry.go` — ProviderRegistry
- `internal/context/compactor.go` — DefaultCompactor
- `internal/query/loop/state.go` — loop.State (REQ-QUERY-015)
- `internal/subagent/types.go` — TeammateIdentity

### 10.2 부속 문서

- `research.md` (본 디렉토리) — Wiring surface analysis
- `progress.md` (본 디렉토리) — phase log

---

## Exclusions (What NOT to Build)

본 SPEC이 **명시적으로 제외**하는 항목 (어느 후속 SPEC이 채워야 하는지 명시):

1. **`LoopController` 의 실제 구현체** — `internal/query/loop/` 또는 `internal/query/` 측 wiring. 후속 SPEC (가칭 `SPEC-GOOSE-CMDLOOP-WIRE-001`).
2. **CLI/daemon 진입점에서 ContextAdapter instantiate** — `cmd/goose` 실행 진입점. SPEC-GOOSE-CLI-001 / SPEC-GOOSE-DAEMON-WIRE-001 범위.
3. **모델 alias config 파일 로드** — `~/.goose/aliases.yaml` 또는 등가. 후속 config SPEC.
4. **OnModelChange 후 OAuth refresh / credential pool swap** — SPEC-GOOSE-CREDPOOL-001 후속 wiring.
5. **Plan mode top-level orchestrator setter** — `/plan` slash command 또는 등가 진입점. COMMAND-001 후속.
6. **Telemetry / metrics emission** — adapter 호출 카운트 / latency 수집. 후속 observability SPEC.
7. **Permissive alias mode** — SuggestedModels 에 없는 모델 허용 옵션. 본 SPEC은 strict only. 후속 SPEC (TBD-SPEC-ID, 본 SPEC 머지 후 별도 plan 필요).
8. **Hot-reload of registry / aliasMap** — `New(...)` 시점 immutable. 후속 SPEC (TBD-SPEC-ID, 본 SPEC 머지 후 별도 plan 필요).
9. **Multi-session adapter** — 단일 프로세스 단일 세션 가정. 다중 세션 multiplexing 은 후속 SPEC (TBD-SPEC-ID, 본 SPEC 머지 후 별도 plan 필요).
10. **Dispatcher 인터페이스 변경** — `SlashCommandContext` 시그니처 자체는 SPEC-GOOSE-COMMAND-001 가 소유. 본 SPEC은 구현만.
