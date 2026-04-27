---
id: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001
version: 0.1.0
status: planned
created_at: 2026-04-27
updated_at: 2026-04-27
author: manager-spec
priority: P1
issue_number: null
phase: 2
size: 중(M)
lifecycle: spec-anchored
labels: [area/router, area/credential, type/feature, priority/p1-high]
---

# SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 — OnModelChange 후 credential pool swap wiring

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. CMDCTX-001 (implemented, v0.1.1) 의 `OnModelChange` → `LoopController.RequestModelChange` 위임 경로에 CREDPOOL-001 (implemented, v0.3.0) 의 credential pool API 결합 wiring SPEC 신설. CMDLOOP-WIRE-001 (planned, batch A) 의 `LoopControllerImpl` 에 `CredentialPoolResolver` 옵션 dependency 를 후방 호환 추가하는 형태. CMDCTX-001 §3.2 OUT-OF-SCOPE #4 (OnModelChange 후 OAuth refresh / credential pool swap) 의 wiring 을 본 SPEC 이 채운다. | manager-spec |

---

## 1. 개요 (Overview)

CMDCTX-001 의 `OnModelChange` 는 `LoopController.RequestModelChange` 를 호출하여 model 식별자를 LoopController 에 전달하지만, **그 model 변경이 실제로 활성 provider 의 credential 까지 swap 하지 않는다.** CMDCTX-001 §3.2 OUT-OF-SCOPE #4 가 명시:

> 4. **OnModelChange 후 active provider 의 OAuth refresh / credential pool swap** — CREDPOOL-001 후속 wiring.

본 SPEC 은 그 빈 자리를 채운다. CREDPOOL-001 (v0.3.0, implemented) 의 credential pool API 와 CMDLOOP-WIRE-001 (planned, batch A) 의 `LoopControllerImpl.RequestModelChange` 핸들러를 결합한다. 본 SPEC 수락 시점에서:

- `internal/query/cmdctrl/` (CMDLOOP-WIRE-001 채택 시 패키지) 에 `CredentialPoolResolver` 인터페이스가 신규 정의되고 1 메서드 (`PoolFor(provider string) *credential.CredentialPool`) 를 노출한다.
- `LoopControllerImpl` 에 `WithCredentialPoolResolver(r CredentialPoolResolver)` 옵션이 추가된다 (후방 호환, nil-tolerant).
- `LoopControllerImpl.RequestModelChange(ctx, info)` 가 신규 provider 의 credential pool 가용성을 사전 검증하고, 가용 entry 가 0 이면 swap 자체를 거부한다 (`ErrCredentialUnavailable`).
- 신규 provider pool 의 첫 `Select(ctx)` 호출이 CREDPOOL-001 의 `triggerRefreshLocked` 자동 OAuth refresh 의미론을 그대로 활용한다. 본 SPEC 은 별도의 refresh API 를 호출하지 않는다.
- (Optional) `WithPreWarmRefresh(true)` 옵션이 enable 되면 swap 후 background goroutine 에서 첫 `Select(ctx)` 를 비동기 호출하여 OAuth refresh 를 사전 트리거한다.
- 모든 nil 의존성 (resolver / pool / engine) 에 대해 graceful degradation 한다 (panic 금지).
- CMDCTX-001 의 `ContextAdapter`, `LoopController` 인터페이스, COMMAND-001 의 dispatcher, CREDPOOL-001 의 CredentialPool 자체는 **변경하지 않는다**.

본 SPEC 은 **CMDLOOP-WIRE-001 의 `LoopControllerImpl` 에만 후방 호환 추가** 를 한다. 다른 모든 의존 SPEC 자산은 read-only 사용.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **CMDCTX-001 OUT-OF-SCOPE #4 의 명시적 후속**: CMDCTX-001 spec.md §3.2 #4 가 본 SPEC 을 명시적으로 호출. 본 SPEC 부재 시 `/model anthropic/claude-opus-4-7` 명령은 model 식별자만 swap 하고 credential 은 이전 provider 의 것을 그대로 사용하여 LLM 호출 단계에서 mismatch / 401 오류 발생.
- **CMDLOOP-WIRE-001 의 RequestModelChange 가 atomic swap 만 수행**: CMDLOOP-WIRE-001 §6.3 의 알고리즘은 `c.activeModel.Store(&info)` 한 줄로 끝난다. credential 검증 / refresh 트리거는 별도 SPEC 책임이라고 명시 (Exclusions §10 #1).
- **OAuth 토큰 만료의 silent 실패 방지**: 신규 provider 의 모든 credential 이 만료된 상황에서 swap 이 성공하면 첫 SubmitMessage 에서 `ErrExhausted` 로 실패. 사용자는 model 명령이 성공했다고 오인. 본 SPEC 의 사전 검증으로 swap 자체를 거부 → 사용자가 즉시 인지.

### 2.2 상속 자산 (FROZEN)

- **SPEC-GOOSE-CREDPOOL-001** (implemented, v0.3.0, FROZEN):
  - `internal/llm/credential/pool.go` 의 `*CredentialPool` API (`Select`, `Size`, `Reload`, `Reset`, `MarkExhaustedAndRotate` 등). 본 SPEC 은 `Select(ctx)` 와 `Size()` 만 호출한다.
  - `internal/llm/credential/refresher.go` 의 `Refresher` 인터페이스. 본 SPEC 은 read-only 의존. 구현체는 ADAPTER-001 / CREDENTIAL-PROXY-001 SPEC 책임.
  - `internal/llm/credential/strategy.go` 의 4 strategy (RoundRobin / LRU / Weighted / Priority). 본 SPEC 은 직교.
  - `internal/llm/credential/factory.go` 의 `NewPoolsFromConfig` — `map[providerName]*CredentialPool` build. 본 SPEC 의 `CredentialPoolResolver` 구현체가 이 map 을 read-only 로 사용 (CLI 진입점이 wire).
- **SPEC-GOOSE-CMDCTX-001** (implemented, v0.1.1, FROZEN):
  - `internal/command/adapter/adapter.go:140-168` 의 `OnModelChange` → `LoopController.RequestModelChange` 위임. 본 SPEC 은 read-only 참조.
  - `internal/command/adapter/controller.go` 의 `LoopController` 인터페이스 4 메서드. **변경 금지.**
  - `command.ModelInfo` 타입. 본 SPEC 의 `extractProvider(info.ID)` 입력.
- **SPEC-GOOSE-CMDLOOP-WIRE-001** (planned, batch A, v0.1.0):
  - `internal/query/cmdctrl/` (D-001 옵션 C 채택 가정) 의 `LoopControllerImpl`.
  - 본 SPEC 은 그 `New(...)` 시그니처에 `WithCredentialPoolResolver(r)` 옵션 추가 + `RequestModelChange` 본문에 nil-tolerant credResolver 호출 추가 (~10 LOC).
  - **CMDLOOP-WIRE-001 가 implemented 가 아니므로** 본 SPEC 의 run phase 진입은 그 SPEC 의 implemented 이후. spec frontmatter `phase: 2` 는 plan/run 분리 의도가 아닌 implementation phase 분류.

### 2.3 범위 경계 (한 줄)

본 SPEC 은 **OnModelChange → RequestModelChange 경로에 credential pool 검증 + 자동 refresh 트리거 wiring 만** 한다. CredentialPool 자체의 정책 / OAuth flow UI / Refresher 구현체 / dispatcher 변경은 모두 범위 외.

---

## 3. 목표 (Goals)

### 3.1 IN SCOPE

| ID | 항목 | 목적 |
|----|------|-----|
| G-1 | `CredentialPoolResolver` 인터페이스 신규 정의 (1 메서드) | provider name → `*CredentialPool` 의 read-only lookup 추상화 |
| G-2 | `LoopControllerImpl` 에 `WithCredentialPoolResolver(r)` 옵션 추가 | 후방 호환 dependency injection |
| G-3 | `LoopControllerImpl.RequestModelChange` 본문에 nil-tolerant 사전 검증 | F-3/F-4 fast-fail, swap 거부 의미론 |
| G-4 | `ErrCredentialUnavailable` sentinel error 신규 정의 | 사용자/dispatcher 에 actionable 에러 전달 |
| G-5 | `extractProvider(info.ID) string` 헬퍼 | model ID 에서 provider name 추출, defensive |
| G-6 | (Optional) `WithPreWarmRefresh(bool)` 옵션 + background refresh goroutine | 첫 SubmitMessage 의 refresh latency 절약 |
| G-7 | 단위 테스트 (fake resolver, fake pool stub), race 테스트, coverage ≥ 90% | TRUST 5 Tested |
| G-8 | nil resolver / nil pool / 잘못된 model ID / pool exhausted 시 graceful degradation | TRUST 5 Secured |

### 3.2 OUT OF SCOPE (§10 Exclusions 에서 정식화)

- CredentialPool 자체의 정책 수정 (cooldown, retry, refresh margin)
- OAuth flow UI (브라우저 OAuth 페이지 열기)
- Refresher 인터페이스의 구현체 (실제 OAuth refresh 로직)
- provider→pool 매핑 wiring (config → pools, factory.go 책임)
- LLM 요청 실패 경로의 credential rotation
- model swap 시점의 dispatcher / OnModelChange 의미론 변경
- multi-provider credential 공유 / cross-provider rotation
- config hot-reload (`Reload(ctx)`)

---

## 4. 요구사항 (Requirements, EARS)

### 4.1 Ubiquitous Requirements (시스템 상시 불변)

**REQ-CCWIRE-001**: 시스템은 `CredentialPoolResolver` 인터페이스를 정의하고 1 메서드 `PoolFor(provider string) *credential.CredentialPool` 을 노출해야 한다 (`shall expose CredentialPoolResolver interface with PoolFor method`). 반환값이 nil 이면 해당 provider 가 credential pool 을 보유하지 않거나 등록되지 않음을 의미한다.

**REQ-CCWIRE-002**: 시스템은 `LoopControllerImpl` 의 `New(...)` 또는 등가 생성자에 `WithCredentialPoolResolver(r CredentialPoolResolver)` 옵션을 추가해야 한다 (`shall accept CredentialPoolResolver as optional dependency`). 옵션 미사용 시 resolver 는 nil 이며 본 SPEC 의 모든 검증 경로는 no-op (CMDLOOP-WIRE-001 의 기존 의미론 보존).

**REQ-CCWIRE-003**: 시스템은 `OnModelChange` → `LoopController.RequestModelChange` 위임 경로의 시그니처를 변경하지 않아야 한다 (`shall not change LoopController interface signature`). CMDCTX-001 의 controller.go FROZEN 보존.

**REQ-CCWIRE-004**: 시스템은 신규 추가되는 모든 코드 경로에서 panic 을 일으키지 않아야 한다 (`shall not panic`). nil resolver, nil pool, 빈 info.ID, ctx 취소 등 모든 입력에 대해 sentinel error 또는 nil 반환.

**REQ-CCWIRE-005**: 시스템은 `extractProvider(info.ID)` 헬퍼를 정의해야 하며, `strings.SplitN(info.ID, "/", 2)` 결과의 첫 번째 요소를 반환해야 한다 (`shall extract provider name from "provider/model" format`). `info.ID == ""` 또는 `/` 미포함이면 빈 문자열 반환.

**REQ-CCWIRE-006**: 시스템은 `ErrCredentialUnavailable` sentinel error 를 정의하고 `errors.Is` 호환이어야 한다 (`shall define ErrCredentialUnavailable sentinel`). CMDLOOP-WIRE-001 의 `ErrEngineUnavailable` / `ErrInvalidModelInfo` 와 동일 패키지(`internal/query/cmdctrl/errors.go`)에 위치.

**REQ-CCWIRE-007**: 시스템의 본 SPEC 이 추가하는 검증 경로는 CredentialPool 의 어떤 mutating 메서드 (`MarkExhausted`, `Release`, `MarkExhaustedAndRotate`, `Reload`, `Reset`) 도 호출하지 않아야 한다 (`shall use only read-only or self-mutating CredentialPool APIs (Select, Size)`). `Select` 는 lease 획득을 동반하지만, 본 SPEC 의 swap-time 호출 경로는 즉시 `Release` (또는 lease 상태를 사용하지 않음 — §6.4 참조).

### 4.2 Event-Driven Requirements (when X then Y)

**REQ-CCWIRE-008**: WHEN `RequestModelChange(ctx, info)` 가 호출되고 resolver != nil 이면 THEN 시스템은 model 식별자 atomic swap **이전에** `provider := extractProvider(info.ID)`, `pool := resolver.PoolFor(provider)`, `pool.Size()` 순서로 사전 검증을 수행해야 한다 (`shall pre-validate credential pool before activeModel swap`).

**REQ-CCWIRE-009**: WHEN `pool == nil` 이면 THEN 시스템은 `ErrCredentialUnavailable` 을 반환하고 `activeModel` 을 변경하지 않아야 한다 (`shall reject swap with ErrCredentialUnavailable on nil pool`).

**REQ-CCWIRE-010**: WHEN `pool.Size()` 의 `available == 0` 이면 THEN 시스템은 `ErrCredentialUnavailable` 을 반환하고 `activeModel` 을 변경하지 않아야 한다 (`shall reject swap with ErrCredentialUnavailable on zero available credentials`).

**REQ-CCWIRE-011**: WHEN 사전 검증을 모두 통과하면 THEN 시스템은 CMDLOOP-WIRE-001 의 기존 atomic swap 의미론 (`activeModel.Store(&info)`) 을 그대로 수행해야 한다 (`shall perform activeModel atomic swap after validation passes`). 즉, 본 SPEC 은 swap 의미론을 변경하지 않는다 — 거부 또는 그대로 진행.

**REQ-CCWIRE-012**: WHEN `info.ID == ""` 또는 `extractProvider(info.ID) == ""` 이면 THEN 시스템은 CMDLOOP-WIRE-001 의 `ErrInvalidModelInfo` 를 반환해야 한다 (`shall reuse ErrInvalidModelInfo for empty provider`). 별도 신규 sentinel 정의 금지 — CMDLOOP-WIRE-001 와 일관.

**REQ-CCWIRE-013**: WHEN `WithPreWarmRefresh(true)` 가 옵션으로 설정되었고 사전 검증 + swap 이 모두 성공한 직후 THEN 시스템은 background goroutine 에서 `pool.Select(ctx)` 를 1회 호출하고, 결과의 lease 를 즉시 `pool.Release` 로 반환해야 한다 (`shall optionally pre-warm OAuth refresh in background after successful swap`). 이 호출은 CREDPOOL-001 의 `triggerRefreshLocked` 자동 refresh 의미론을 활용한다.

### 4.3 State-Driven Requirements (while X)

**REQ-CCWIRE-014**: WHILE `LoopControllerImpl.credResolver == nil` (옵션 미설정 또는 nil 주입) 일 때 `RequestModelChange` 가 호출되면 THEN 시스템은 본 SPEC 이 추가하는 모든 검증 경로를 skip 하고 CMDLOOP-WIRE-001 의 기존 의미론을 그대로 수행해야 한다 (`shall skip all CCWIRE validation when credResolver is nil`). 후방 호환 보장.

**REQ-CCWIRE-015**: WHILE 사전 검증 단계에서 ctx 가 cancelled 상태라면 THEN 시스템은 `ctx.Err()` 를 반환하고 어떤 부수 효과 (activeModel 변경, preWarm goroutine spawn, pool 호출) 도 일으키지 않아야 한다 (`shall return ctx.Err() and abort on cancelled ctx`). CMDLOOP-WIRE-001 의 REQ-CMDLOOP-012 와 일관.

**REQ-CCWIRE-016**: WHILE preWarm background goroutine 이 실행 중인 상태에서 추가 `RequestModelChange` 가 다른 provider 로 호출되면 THEN 기존 goroutine 은 ctx 검사 또는 자체 종료까지 그대로 진행되며, 본 SPEC 은 이전 goroutine 의 결과를 무시한다 (`shall not synchronize concurrent preWarm goroutines`). preWarm 은 best-effort. lease 획득 시 즉시 release 하므로 leak 없음.

### 4.4 Unwanted Behavior (방지)

**REQ-CCWIRE-017**: 시스템은 사전 검증 실패 시 `activeModel` 을 변경해서는 안 된다 (`shall not mutate activeModel on validation failure`). atomic.Pointer.Store 호출 자체가 발생하지 않아야 한다.

**REQ-CCWIRE-018**: 시스템은 swap 성공 후 background preWarm 이 실패하더라도 `RequestModelChange` 의 반환값을 nil 에서 error 로 변경해서는 안 된다 (`shall not propagate preWarm errors to caller`). preWarm 은 best-effort optional. 실패는 logger 에 debug 또는 warn 로 기록하고 상위 흐름에 영향 주지 않음.

**REQ-CCWIRE-019**: 시스템은 본 SPEC 이 추가하는 코드 경로에서 CredentialPool 의 mutating 메서드 (`MarkExhausted`, `MarkExhaustedAndRotate`, `MarkError`, `Release`, `Reload`, `Reset`) 를 호출해서는 안 된다 (`shall not call CredentialPool mutating methods`). 예외: preWarm 의 `Select` 호출 직후 `Release` 1회 — 이는 lease 획득의 짝 (REQ-CCWIRE-013 명시).

**REQ-CCWIRE-020**: 시스템은 CMDCTX-001 의 `internal/command/adapter/` 패키지, COMMAND-001 의 dispatcher, CREDPOOL-001 의 `internal/llm/credential/` 패키지의 어떤 파일도 변경해서는 안 된다 (`shall not modify FROZEN packages`). 변경 허용 범위는 `internal/query/cmdctrl/` (CMDLOOP-WIRE-001 가 도입한 패키지) 에 한정.

### 4.5 Optional Requirements (선택적)

**REQ-CCWIRE-021**: WHERE `WithPreWarmRefresh(true)` 가 enable 되어 있으면 시스템은 swap 성공 후 background goroutine 에서 `pool.Select(ctx)` → `pool.Release(cred)` 를 수행해야 한다 (`shall pre-warm refresh when option enabled`). 기본값은 false (preWarm 비활성).

**REQ-CCWIRE-022**: WHERE structured logger 가 `LoopControllerImpl` 에 주입되어 있으면 시스템은 본 SPEC 의 검증 결과 (pool nil / available 0 / preWarm 시작/완료/실패) 를 debug level 로 기록해야 한다 (`shall log validation outcomes when logger is provided`). logger nil 이면 silent.

**REQ-CCWIRE-023**: WHERE `LoopControllerImpl.Close()` 또는 등가 종료 hook 이 향후 도입되면 시스템은 활성 preWarm goroutine 의 종료를 대기해야 한다 (`shall optionally wait for preWarm goroutines on close`). 본 SPEC 단계에서는 sync.WaitGroup 을 추가하되, Close hook 이 없으면 호출되지 않는다 (defer-only, no explicit wait).

---

## 5. REQ ↔ AC 매트릭스

| REQ | AC | 관련 메서드 / 동작 |
|-----|----|----------------|
| REQ-CCWIRE-001 | AC-CCWIRE-001 | 인터페이스 컴파일 단언 |
| REQ-CCWIRE-002 | AC-CCWIRE-002 | New + WithCredentialPoolResolver 옵션 |
| REQ-CCWIRE-003 | AC-CCWIRE-013 (정적 분석) | controller.go diff 검증 |
| REQ-CCWIRE-004 | AC-CCWIRE-014 | panic-free 모든 경로 |
| REQ-CCWIRE-005 | AC-CCWIRE-003 | extractProvider 단위 테스트 |
| REQ-CCWIRE-006 | AC-CCWIRE-004 | sentinel error errors.Is |
| REQ-CCWIRE-007 | AC-CCWIRE-013 (정적 분석) | mutation grep |
| REQ-CCWIRE-008 | AC-CCWIRE-005 | 검증 순서 |
| REQ-CCWIRE-009 | AC-CCWIRE-006 | nil pool 거부 |
| REQ-CCWIRE-010 | AC-CCWIRE-007 | available == 0 거부 |
| REQ-CCWIRE-011 | AC-CCWIRE-008 | swap 성공 |
| REQ-CCWIRE-012 | AC-CCWIRE-009 | 빈 provider |
| REQ-CCWIRE-013 | AC-CCWIRE-010 | preWarm 호출 |
| REQ-CCWIRE-014 | AC-CCWIRE-011 | nil resolver no-op |
| REQ-CCWIRE-015 | AC-CCWIRE-012 | ctx cancelled |
| REQ-CCWIRE-016 | AC-CCWIRE-016 | 동시 preWarm |
| REQ-CCWIRE-017 | AC-CCWIRE-006, AC-CCWIRE-007 | activeModel 미변경 검증 |
| REQ-CCWIRE-018 | AC-CCWIRE-017 | preWarm 실패 무시 |
| REQ-CCWIRE-019 | AC-CCWIRE-013 | mutation 정적 분석 |
| REQ-CCWIRE-020 | AC-CCWIRE-018 | FROZEN 패키지 diff 검증 |
| REQ-CCWIRE-021 | AC-CCWIRE-010, AC-CCWIRE-016 | preWarm 시나리오 |
| REQ-CCWIRE-022 | AC-CCWIRE-019 | logger 호출 |
| REQ-CCWIRE-023 | AC-CCWIRE-020 (Optional) | sync.WaitGroup 존재 |

총 REQ 23 / 총 AC 20. 모든 REQ 가 최소 1개의 AC 로 검증된다.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/query/cmdctrl/                # CMDLOOP-WIRE-001 가 도입
├── controller.go                       # CMDLOOP-WIRE-001 (LoopControllerImpl 본체) — 수정 (옵션 + 본문 ~15 LOC)
├── controller_test.go                  # CMDLOOP-WIRE-001 — 신규 테스트 추가
├── errors.go                           # CMDLOOP-WIRE-001 의 ErrEngineUnavailable / ErrInvalidModelInfo + 본 SPEC 의 ErrCredentialUnavailable 추가
├── credresolver.go                     # ⬅︎ 본 SPEC 신규: CredentialPoolResolver 인터페이스, extractProvider 헬퍼
├── credresolver_test.go                # ⬅︎ 본 SPEC 신규
└── ... (CMDLOOP-WIRE-001 기타 파일)
```

본 SPEC 이 추가하는 신규 파일: `credresolver.go` + `credresolver_test.go`.
본 SPEC 이 수정하는 파일: `controller.go` (옵션 추가 ~5 LOC + RequestModelChange 본문 ~15 LOC), `errors.go` (sentinel 1개 추가).

### 6.2 핵심 타입 (Go 시그니처)

```go
// Package cmdctrl — credential pool wiring extension.
//
// @MX:SPEC: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001
package cmdctrl

import (
    "context"
    "errors"
    "strings"

    "github.com/modu-ai/goose/internal/llm/credential"
)

// CredentialPoolResolver is the read-only abstraction the LoopControllerImpl
// uses to look up the active provider's credential pool during model swap.
// Implementations are expected to wrap a map[providerName]*CredentialPool
// produced by credential.NewPoolsFromConfig (CREDPOOL-001 OI-06).
//
// PoolFor returns nil when the provider has no pool registered (provider has
// no credentials configured, or the provider name is unknown). Callers MUST
// treat a nil return as a non-recoverable swap error and reject the swap.
//
// @MX:ANCHOR: Boundary between LoopController and credential pool registry.
// @MX:REASON: Single source of truth for provider→pool lookup. fan_in >= 2
// (RequestModelChange + future SubmitMessage credential refresh trigger).
type CredentialPoolResolver interface {
    PoolFor(provider string) *credential.CredentialPool
}

// extractProvider parses the provider name from a "provider/model" formatted
// model identifier. Returns an empty string when info.ID is empty or does not
// contain "/". Defensive check; callers should already have validated via
// CMDCTX-001 ResolveModelAlias upstream.
func extractProvider(modelID string) string {
    if modelID == "" {
        return ""
    }
    parts := strings.SplitN(modelID, "/", 2)
    if len(parts) < 2 || parts[0] == "" {
        return ""
    }
    return parts[0]
}

// ErrCredentialUnavailable is returned by RequestModelChange when the target
// provider has no pool registered or the pool has zero available credentials.
// The activeModel is not swapped on this error. errors.Is compatible.
var ErrCredentialUnavailable = errors.New("cmdctrl: credential pool unavailable for target provider")

// WithCredentialPoolResolver returns an Option that injects a
// CredentialPoolResolver into LoopControllerImpl. Passing nil is a no-op
// (equivalent to not setting the option). Backward compatible.
func WithCredentialPoolResolver(r CredentialPoolResolver) Option {
    return func(c *LoopControllerImpl) {
        c.credResolver = r
    }
}

// WithPreWarmRefresh returns an Option that toggles background OAuth refresh
// pre-warming after a successful model swap. Default: false.
func WithPreWarmRefresh(enabled bool) Option {
    return func(c *LoopControllerImpl) {
        c.preWarmRefresh = enabled
    }
}
```

`Option` 타입과 `New(...)` 의 variadic option 시그니처는 CMDLOOP-WIRE-001 가 도입. 본 SPEC 은 그 패턴을 따른다. (CMDLOOP-WIRE-001 의 D-001 / D-002 결정 확정 후 정식 시그니처 확인 필요 — R-001 참조.)

### 6.3 RequestModelChange 변경 본문 (CMDLOOP-WIRE-001 §6.3 의 확장)

```go
// LoopControllerImpl.RequestModelChange — CMDLOOP-WIRE-001 §6.3 의 알고리즘에
// 본 SPEC 의 사전 검증 + 옵션 preWarm 을 추가한다. 시그니처 불변.
func (c *LoopControllerImpl) RequestModelChange(ctx context.Context, info command.ModelInfo) error {
    if ctx == nil {
        ctx = context.Background()
    }
    if err := ctx.Err(); err != nil {
        return err
    }
    if info.ID == "" {
        return ErrInvalidModelInfo // CMDLOOP-WIRE-001 sentinel
    }

    // ⬇︎ 본 SPEC 신규: 사전 검증 (resolver nil 시 skip, 후방 호환)
    var pool *credential.CredentialPool
    if c.credResolver != nil {
        provider := extractProvider(info.ID)
        if provider == "" {
            // info.ID 가 "provider/model" 형식 아님 — defensive
            return ErrInvalidModelInfo
        }
        pool = c.credResolver.PoolFor(provider)
        if pool == nil {
            if c.logger != nil {
                c.logger.Debug("RequestModelChange: pool unavailable",
                    zap.String("provider", provider))
            }
            return ErrCredentialUnavailable
        }
        _, available := pool.Size()
        if available == 0 {
            if c.logger != nil {
                c.logger.Debug("RequestModelChange: pool exhausted",
                    zap.String("provider", provider))
            }
            return ErrCredentialUnavailable
        }
    }
    // ⬆︎ 본 SPEC 신규 끝

    // ⬇︎ CMDLOOP-WIRE-001 기존: atomic swap
    c.activeModel.Store(&info)
    if c.logger != nil {
        c.logger.Debug("RequestModelChange enqueued", zap.String("id", info.ID))
    }
    // ⬆︎ CMDLOOP-WIRE-001 기존 끝

    // ⬇︎ 본 SPEC Optional: preWarm
    if c.preWarmRefresh && pool != nil {
        c.preWarmCount.Add(1) // 테스트 가시성 (sync.WaitGroup 또는 atomic counter)
        go c.preWarmRefreshAsync(ctx, pool, info.ID)
    }
    // ⬆︎ 본 SPEC Optional 끝

    return nil
}

// preWarmRefreshAsync is a best-effort goroutine that triggers OAuth refresh
// on the new pool by performing a single Select+Release cycle. Errors are
// logged but never propagated.
func (c *LoopControllerImpl) preWarmRefreshAsync(ctx context.Context, pool *credential.CredentialPool, modelID string) {
    defer c.preWarmCount.Add(-1)
    cred, err := pool.Select(ctx)
    if err != nil {
        if c.logger != nil {
            c.logger.Debug("preWarm refresh skipped", zap.String("model", modelID), zap.Error(err))
        }
        return
    }
    // Release immediately — the only goal of preWarm is to trigger
    // CredentialPool.triggerRefreshLocked for OAuth refresh margin entries.
    if err := pool.Release(cred); err != nil && c.logger != nil {
        c.logger.Debug("preWarm release failed", zap.Error(err))
    }
}
```

### 6.4 lease 처리 정책

`Select(ctx)` 는 lease 를 획득한다 (`pool.go:266 cred.leased = true`). 본 SPEC 의 두 호출 경로:

1. **사전 검증 시 (REQ-CCWIRE-008)**: `Size()` 만 호출. lease 획득 없음. 가용 entry 수만 확인. 정확도 trade-off: race 윈도우에서 다른 SubmitMessage 가 동시에 Select 하여 entry 가 lease 됐을 가능성 — 이 경우 swap 은 통과하고 첫 SubmitMessage 가 `ErrExhausted` 반환. 이는 본 SPEC 범위 외 (ADAPTER-001 의 rotation 흐름 책임).

2. **preWarm 시 (REQ-CCWIRE-013)**: `Select(ctx)` → 즉시 `Release(cred)`. lease 는 일시적 (수 마이크로초). 다른 SubmitMessage 가 동시에 진행 중이면 preWarm 의 Select 가 다른 entry 를 lease 함 (RoundRobin / LRU 의 자연스러운 분산). 충돌 없음.

**선택**: 사전 검증을 `Select` 까지 시도하지 않는 이유 — `Select` 는 mutating (UsageCount++, lease 획득). 본 SPEC 은 read-only 검증만 의도. `Size()` 는 RLock 기반 read-only.

### 6.5 race-clean 보장

| 경합 시나리오 | 보호 메커니즘 |
|------------|------------|
| dispatcher RequestModelChange A ↔ B (서로 다른 provider) | activeModel.Store 는 atomic. resolver.PoolFor 는 read-only map lookup (resolver 구현체 책임 — 본 SPEC 은 nil 또는 immutable 가정). race-free. |
| RequestModelChange ↔ 동시 SubmitMessage (다른 goroutine) | activeModel.Load 는 atomic.Pointer. SubmitMessage 가 swap 직후 새 ID 를 관찰. CMDLOOP-WIRE-001 의 의미론과 동일. |
| preWarm goroutine ↔ SubmitMessage 의 Select 경합 | CredentialPool.mu 가 모든 Select 호출 직렬화. race-free. |
| preWarm goroutine ↔ ctx 취소 | ctx.Done 채널을 Select 가 검사 (`pool.go:239-243`). 즉시 종료. |

### 6.6 graceful degradation

- `c.credResolver == nil` → 모든 검증 skip (REQ-CCWIRE-014). 기존 의미론 보존.
- `pool == nil` → `ErrCredentialUnavailable` 반환 (REQ-CCWIRE-009).
- `pool.Size()` 의 available == 0 → `ErrCredentialUnavailable` (REQ-CCWIRE-010).
- `info.ID == ""` 또는 `extractProvider == ""` → `ErrInvalidModelInfo` (REQ-CCWIRE-012).
- ctx cancelled → `ctx.Err()` (REQ-CCWIRE-015).
- preWarm 의 `Select` 실패 → debug 로그 후 무시 (REQ-CCWIRE-018).

### 6.7 logger 통합

logger 가 nil 이면 본 SPEC 의 모든 로깅 경로는 silent. logger 가 제공되면:

- `RequestModelChange: pool unavailable` (debug, with provider field)
- `RequestModelChange: pool exhausted` (debug)
- `preWarm refresh started` (debug)
- `preWarm refresh skipped` (debug, with error)
- `preWarm release failed` (debug, with error)

본 SPEC 의 로깅은 모두 debug level. warn / error 발생 경로 없음 (best-effort optional refresh).

---

## 7. 의존성 (Dependencies)

| SPEC | status | 본 SPEC 의 사용 방식 |
|------|--------|----------------|
| SPEC-GOOSE-CREDPOOL-001 | implemented (FROZEN, v0.3.0) | `*credential.CredentialPool.Select(ctx)`, `Size()`, `Release(cred)` 만 호출. 다른 메서드 미호출. `Refresher` 인터페이스 read-only 의존 (구현체 호출 없음). 패키지 변경 금지. |
| SPEC-GOOSE-CMDCTX-001 | implemented (FROZEN, v0.1.1) | `command.ModelInfo` 타입 사용. `ContextAdapter` / `LoopController` 인터페이스 read-only 참조. 패키지 변경 금지. |
| SPEC-GOOSE-CMDLOOP-WIRE-001 | planned (batch A, v0.1.0) | `LoopControllerImpl` 본체에 `WithCredentialPoolResolver` 옵션 추가 + `RequestModelChange` 본문 확장. 후방 호환. **CMDLOOP-WIRE-001 가 implemented 되어야 본 SPEC run phase 가능.** 본 SPEC 의 ratify 단계에서 D-006 (CMDLOOP-WIRE-001 frontmatter version bump 합의) 결정 필요. |
| SPEC-GOOSE-COMMAND-001 | implemented (FROZEN) | dispatcher / `command.ModelInfo` / `SlashCommandContext` read-only 참조. 변경 금지. |
| SPEC-GOOSE-ROUTER-001 | implemented (FROZEN) | optional read-only 참조 (provider 식별자 검증 보강). 본 SPEC 은 직접 의존하지 않으며, resolver 구현체가 사용할 수 있음 (CLI-001 책임). |

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
| 정적 분석 — adapter 비변경 | `git diff main..HEAD -- internal/command/adapter/ internal/llm/credential/` 결과 변경 없음 (AC-CCWIRE-018) |
| 정적 분석 — mutation 미사용 | `grep -rE 'pool\.(MarkExhausted|MarkExhaustedAndRotate|MarkError|Reload|Reset)\(' internal/query/cmdctrl/ --include='*.go' \| grep -v '_test.go'` 결과 0건 |
| LSP 진단 | 0 errors / 0 type errors / 0 lint errors (run phase) |

---

## 9. Risks (위험 영역)

| ID | 위험 | 영향 | 완화 |
|----|------|------|------|
| R-001 | CREDPOOL-001 의 정확한 API surface 가 v0.3.0 implemented 와 다를 가능성 | 본 SPEC 의 wiring 가정 무너짐 | research.md §3 에서 `pool.go` 의 line 단위 검증 완료 (2026-04-27 기준: `Select` line 237, `Size` line 372, `Release` line 324). run phase Phase 1 (ANALYZE) 에서 재검증. drift 발견 시 spec 0.1.1 patch + plan-auditor 사이클. |
| R-002 | OAuth refresh 가 `LoopController.RequestModelChange` 응답 latency 에 영향 | UI 응답성 악화 | 옵션 A 채택 (research.md §6.1) — refresh 는 첫 `Select` 시 자동, swap 자체에는 영향 없음. preWarm 도 background goroutine. swap latency = `Size()` 1회 (RLock) + atomic Store. |
| R-003 | 다중 provider 간 credential rotation 정책 충돌 | 동일 KeyringID 가 두 provider 에 등록 시 race | 현재 factory.go 가 provider 별 독립 pool 만 build (research.md §5.1). 본 SPEC 은 multi-pool 공유를 가정하지 않는다. resolver 구현체가 동일 pool 을 두 provider 에 매핑하는 경우 사용자 책임 — 본 SPEC 은 그 의미론 정의 안 함. |
| R-004 | preWarm goroutine 의 ctx 누수 (사용자가 다른 model 로 즉시 swap) | goroutine leak | preWarm 은 swap 시점 ctx 를 받음. Select 가 ctx.Done 검사 (CREDPOOL-001 line 239-243). leak 없음. 추가로 sync.WaitGroup 또는 atomic counter 로 활성 goroutine 수 추적 (REQ-CCWIRE-023). |
| R-005 | CMDLOOP-WIRE-001 의 `LoopControllerImpl` 변경이 그 SPEC 의 FROZEN 정책 위반인가 | run phase 진입 차단 | 후방 호환 옵션 dependency 추가 (시그니처 불변, nil-tolerant). 본 SPEC ratify 단계의 D-006 결정 — CMDLOOP-WIRE-001 frontmatter version bump (0.1.x → 0.2.0) 합의. |
| R-006 | `Size()` 의 available 카운트가 사전 검증과 실제 SubmitMessage 사이 race 윈도우에서 변경 | swap 통과 후 SubmitMessage 가 ErrExhausted | 본 SPEC 범위 외 (ADAPTER-001 의 rotation 흐름이 처리). 본 SPEC 은 best-effort 사전 검증 — TOCTOU 완벽 방어 불가능 + 시도하지 않음. AC-CCWIRE-007 에 명시. |
| R-007 | resolver 구현체 (CLI-001 책임) 의 PoolFor 가 thread-safe 하지 않으면 race | 사용자 보고 가능한 panic / 잘못된 pool 반환 | resolver 의 thread-safety 는 그 구현체 책임. 본 SPEC 은 nil 만 graceful 처리. 인터페이스 godoc 에 thread-safe 요구사항 명시 (REQ-CCWIRE-001). |
| R-008 | preWarm 실패가 silent 로 발생하면 사용자가 OAuth 만료 인지 못함 | 디버깅 어려움 | 첫 SubmitMessage 가 `ErrExhausted` 발생 시 dispatcher 가 사용자에게 actionable 메시지. 본 SPEC 의 preWarm 은 best-effort + debug 로그. warn level 로 격상 옵션은 후속 SPEC. |
| R-009 | `ErrCredentialUnavailable` 이 dispatcher 에러 표시 와 호환되지 않음 | 사용자 메시지 누락 | sentinel error 정의 + `errors.Is`. CMDCTX-001 OnModelChange 의 return value 가 그대로 dispatcher 의 RunResult.Err 로 전달됨 (CMDCTX-001 spec.md §6.5 RunResult 참조). 호환성 검증 — AC-CCWIRE-004 / AC-CCWIRE-018. |

---

## 10. Exclusions (What NOT to Build)

본 SPEC 은 다음을 **수행하지 않는다**. 각 항목은 후속 SPEC 또는 다른 SPEC 책임으로 분리.

1. **CredentialPool 자체의 정책 수정 (cooldown, retry, refresh margin, 4 strategy 변경)**
   - 위임: SPEC-GOOSE-CREDPOOL-001 (FROZEN, v0.3.0). 정책 변경은 그 SPEC 의 후속 patch (가칭 SPEC-GOOSE-CREDPOOL-002) 책임.

2. **OAuth flow UI (브라우저 OAuth 페이지 열기, 사용자 로그인 흐름)**
   - 위임: SPEC-GOOSE-CLI-INTEG-001 / SPEC-GOOSE-DAEMON-INTEG-001 (가칭, 본 SPEC 머지 후 별도 plan 필요).

3. **Refresher 인터페이스의 구현체 (실제 OAuth refresh 로직, goose-proxy 경유)**
   - 위임: SPEC-GOOSE-ADAPTER-001 / SPEC-GOOSE-CREDENTIAL-PROXY-001 (implemented). 본 SPEC 은 인터페이스만 read-only 의존.

4. **provider→pool 매핑 wiring (config → pools)**
   - 위임: SPEC-GOOSE-CREDPOOL-001 OI-06 (`factory.NewPoolsFromConfig`, implemented). resolver 구현체가 그 결과를 read-only 로 사용하는 wiring 은 SPEC-GOOSE-CLI-001 / SPEC-GOOSE-DAEMON-WIRE-001 책임.

5. **LLM 요청 실패 경로의 credential rotation (`MarkExhaustedAndRotate`)**
   - 위임: SPEC-GOOSE-ADAPTER-001 (implemented). LLM 요청 시 429/402 응답 후 rotation 흐름은 그 SPEC 책임. 본 SPEC 은 swap 시점만 다룬다.

6. **model swap 시점의 dispatcher / OnModelChange / SlashCommandContext 의미론 변경**
   - 위임: 없음. CMDCTX-001 / COMMAND-001 FROZEN. 변경 불가.

7. **Multi-provider credential 공유 / cross-provider rotation**
   - 위임: 후속 SPEC (가칭 SPEC-GOOSE-CREDPOOL-MULTIPROV-001, 본 SPEC 머지 후 별도 plan 필요). 현재 factory 는 provider 별 독립 pool 만 build.

8. **config hot-reload (`Reload(ctx)` 호출)**
   - 위임: 후속 SPEC (가칭 SPEC-GOOSE-CONFIG-RELOAD-001, 본 SPEC 머지 후 별도 plan 필요). 본 SPEC 의 resolver 는 immutable 가정.

9. **만료된 entry 의 재로그인 (운영자 수동 영구 고갈 해제)**
   - 위임: CREDPOOL-001 의 `Reset(id)` API + CLI 진입점 (SPEC-GOOSE-CLI-001).

10. **CMDLOOP-WIRE-001 의 `RequestModelChange` 시그니처 변경 / 새 sentinel error 추가 (`ErrInvalidModelInfo` 외)**
    - 위임: 없음. CMDLOOP-WIRE-001 의 시그니처는 본 SPEC 도 변경하지 않는다. 단, 그 SPEC 의 `New` 시그니처에 옵션 추가는 본 SPEC 범위 (후방 호환).

11. **preWarm goroutine 의 동기화 (Close hook 등) 의 완전한 lifecycle 관리**
    - 본 SPEC 의 REQ-CCWIRE-023 은 Optional. 명시적 Close 가 도입되는 시점까지 sync.WaitGroup 만 추가하고 Wait 호출은 없음. 완전한 lifecycle 은 후속 SPEC.

12. **Telemetry / metrics emission (swap 카운트, preWarm 실패율 등)**
    - 위임: 후속 observability SPEC (가칭).

---

## 11. 참조 (References)

- `internal/llm/credential/pool.go:60-76` — `New(source, strategy, opts...)` 생성자
- `internal/llm/credential/pool.go:237-273` — `Select(ctx)` (자동 refresh trigger 포함)
- `internal/llm/credential/pool.go:279-320` — `triggerRefreshLocked` (OAuth refresh margin 의미론)
- `internal/llm/credential/pool.go:324-337` — `Release(cred)`
- `internal/llm/credential/pool.go:372-379` — `Size()`
- `internal/llm/credential/refresher.go:12-16` — `Refresher` 인터페이스
- `internal/llm/credential/strategy.go` — 4 strategy 구현
- `internal/llm/credential/factory.go:46-82` — `NewPoolsFromConfig`
- `internal/command/adapter/adapter.go:140-168` — `ContextAdapter.OnModelChange` (read-only)
- `internal/command/adapter/controller.go:19-51` — `LoopController` 인터페이스 (FROZEN)
- `.moai/specs/SPEC-GOOSE-CREDPOOL-001/spec.md` — REQ-CREDPOOL-005/013/015/018 (자동 refresh 의미론)
- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` — §3.2 OUT-SCOPE #4 (본 SPEC 의 호출자), REQ-CMDCTX-008
- `.moai/specs/SPEC-GOOSE-CMDLOOP-WIRE-001/spec.md` — §6.3 RequestModelChange 알고리즘, sentinel `ErrInvalidModelInfo`
- `CLAUDE.local.md §2.5` — 코드 주석 영어 정책
- 본 SPEC `research.md` — wiring surface analysis, hook point options, decisions D-001 ~ D-007

---

## 12. Acceptance Criteria

### AC-CCWIRE-001 — CredentialPoolResolver 인터페이스 컴파일 단언

`internal/query/cmdctrl/credresolver.go` 에 `CredentialPoolResolver` 인터페이스가 정의되고, 1 메서드 `PoolFor(provider string) *credential.CredentialPool` 를 노출한다. 테스트 코드의 `var _ CredentialPoolResolver = (*fakeResolver)(nil)` 가 컴파일 성공.

검증 방법: 컴파일 (`go build ./internal/query/cmdctrl/...`).

### AC-CCWIRE-002 — WithCredentialPoolResolver 옵션 wiring

`cmdctrl.New(engine, logger, cmdctrl.WithCredentialPoolResolver(fakeResolver))` 가 컴파일되고, 결과 `LoopControllerImpl.credResolver == fakeResolver`. 옵션 미사용 시 `c.credResolver == nil` (zero-value).

검증 방법: 단위 테스트 (reflect 또는 unexported 필드 접근 — 테스트 헬퍼 사용 가능).

### AC-CCWIRE-003 — extractProvider 단위 테스트

| input | expected output |
|-------|----------------|
| `""` | `""` |
| `"anthropic"` (no slash) | `""` |
| `"/model"` (empty provider) | `""` |
| `"anthropic/claude-opus-4-7"` | `"anthropic"` |
| `"openai/gpt-4o"` | `"openai"` |
| `"a/b/c"` (extra slashes) | `"a"` |
| `"anthropic/"` (trailing slash) | `"anthropic"` |

검증 방법: 표 기반 단위 테스트.

### AC-CCWIRE-004 — ErrCredentialUnavailable sentinel

`var ErrCredentialUnavailable = errors.New("...")` 정의 존재. `errors.Is(err, cmdctrl.ErrCredentialUnavailable)` 가 wrap 된 에러에서 true 반환. CMDCTX-001 의 `OnModelChange` 가 이 에러를 받아 그대로 dispatcher 에 전달 (RunResult.Err 에 포함).

검증 방법: 단위 테스트 + 통합 테스트 (adapter + cmdctrl + fakeResolver).

### AC-CCWIRE-005 — 검증 순서 (사전 검증 → swap)

`fakeResolver.PoolFor("openai")` 가 호출 카운터 증가 + nil 반환하도록 설정. `c.RequestModelChange(ctx, command.ModelInfo{ID: "openai/gpt-4o"})` 호출. 결과: PoolFor 호출 카운터 == 1, `c.activeModel.Load() == nil` (swap 미발생, 호출 전 상태 유지).

검증 방법: 단위 테스트.

### AC-CCWIRE-006 — nil pool 시 swap 거부

위와 동일 setup. `RequestModelChange` 의 반환 에러가 `errors.Is(err, ErrCredentialUnavailable) == true`. `c.activeModel.Load() == nil` (호출 전 상태 유지).

검증 방법: 단위 테스트.

### AC-CCWIRE-007 — available == 0 시 swap 거부

`fakePool.SizeReturns(total: 3, available: 0)` 설정 (모든 entry 만료 또는 leased). `RequestModelChange` 반환 에러가 `ErrCredentialUnavailable`. `c.activeModel.Load() == nil`.

검증 방법: 단위 테스트.

### AC-CCWIRE-008 — 사전 검증 통과 후 swap 성공

`fakePool.SizeReturns(total: 3, available: 2)` 설정. `RequestModelChange(ctx, command.ModelInfo{ID: "anthropic/claude-opus-4-7"})` 호출. 결과: nil 에러 반환, `c.activeModel.Load().ID == "anthropic/claude-opus-4-7"`.

검증 방법: 단위 테스트.

### AC-CCWIRE-009 — 빈 provider name 거부

`RequestModelChange(ctx, command.ModelInfo{ID: "no-slash"})` 또는 `ID: ""`. resolver != nil 인 상태. 결과: `errors.Is(err, ErrInvalidModelInfo) == true` (CMDLOOP-WIRE-001 의 sentinel 재사용). `c.activeModel.Load() == nil`. PoolFor 호출 카운터 == 0 (provider 추출 실패 시 resolver 호출 안 함).

검증 방법: 단위 테스트.

### AC-CCWIRE-010 — preWarm 호출 검증 (Optional)

`cmdctrl.New(engine, logger, WithCredentialPoolResolver(fakeResolver), WithPreWarmRefresh(true))` 로 생성. `fakePool.SelectReturns(&credential.PooledCredential{ID: "cred-1"}, nil)`, `fakePool.ReleaseReturns(nil)`. `RequestModelChange(ctx, anthropicInfo)` 호출. 결과: nil 반환 (즉시), 100ms 이내 `fakePool.SelectCallCount() == 1` 및 `fakePool.ReleaseCallCount() == 1`.

검증 방법: 단위 테스트 (eventually 패턴, sync.WaitGroup 또는 atomic counter polling).

### AC-CCWIRE-011 — nil resolver 시 no-op (후방 호환)

`cmdctrl.New(engine, logger)` (resolver 옵션 없음). `RequestModelChange(ctx, command.ModelInfo{ID: "anthropic/claude-opus-4-7"})` 호출. 결과: nil 에러 반환, `c.activeModel.Load().ID == "anthropic/claude-opus-4-7"`. 어떤 resolver / pool 도 호출되지 않음 (resolver 미주입).

검증 방법: 단위 테스트 (CMDLOOP-WIRE-001 의 RequestModelChange 기존 의미론과 100% 동일성 검증).

### AC-CCWIRE-012 — ctx cancelled 거부

`ctx, cancel := context.WithCancel(parent); cancel()`. resolver != nil. `RequestModelChange(ctx, info)` 호출. 결과: `errors.Is(err, context.Canceled) == true`. `c.activeModel.Load() == nil`. PoolFor 호출 카운터 == 0 (ctx 검사가 PoolFor 보다 선행).

검증 방법: 단위 테스트.

### AC-CCWIRE-013 — mutation 정적 분석 (REQ-CCWIRE-007/019)

```bash
grep -rE 'pool\.(MarkExhausted|MarkExhaustedAndRotate|MarkError|Reload|Reset)\(' \
  internal/query/cmdctrl/ --include='*.go' | grep -v '_test.go'
```

결과 0건. 본 SPEC 이 추가하는 코드는 CredentialPool 의 mutating 메서드를 호출하지 않는다. 예외: preWarm 의 `pool.Release` (lease 짝, REQ-CCWIRE-013 명시). 별도 grep:

```bash
grep -rE 'pool\.Release\(' internal/query/cmdctrl/ --include='*.go' | grep -v '_test.go'
```

결과는 1건 (preWarm goroutine 내부) 만 허용.

검증 방법: CI 정적 분석 단계 + 코드 리뷰.

### AC-CCWIRE-014 — panic-free 모든 경로

다음 모든 입력 조합에 panic 미발생:
- nil resolver
- nil pool (resolver.PoolFor returns nil)
- 빈 info.ID
- ctx == nil
- ctx 이미 cancelled
- preWarm 의 Select 실패
- preWarm 의 Release 실패
- 동시 1000 goroutine RequestModelChange (race detector pass)

검증 방법: 단위 테스트 + race 테스트.

### AC-CCWIRE-015 — race detector clean

`go test -race -count=10 ./internal/query/cmdctrl/...` PASS. 100 goroutines × 1000 iter (RequestModelChange + 동시 Snapshot + RequestClear 무작위 호출) 시나리오에서 race 미검출.

검증 방법: race 테스트 (`controller_race_test.go` 신규 또는 CMDLOOP-WIRE-001 의 race 테스트 확장).

### AC-CCWIRE-016 — 동시 preWarm goroutine 무동기화

WithPreWarmRefresh(true) 상태에서 5개 goroutine 이 서로 다른 provider 로 RequestModelChange 호출. 결과: 5개 background preWarm goroutine spawn. 모두 완료까지 race 미검출. 마지막 swap 의 activeModel 이 visible (last-write-wins).

검증 방법: 단위 테스트 + race detector.

### AC-CCWIRE-017 — preWarm 실패 무시

`fakePool.SelectReturns(nil, errors.New("simulated"))` 설정. WithPreWarmRefresh(true). RequestModelChange 호출. 결과: nil 반환 (즉시). 100ms 후 logger.DebugCount >= 1 (preWarm skipped 로그). RequestModelChange 자체의 반환값에는 영향 없음.

검증 방법: 단위 테스트.

### AC-CCWIRE-018 — FROZEN 패키지 변경 부재

`git diff main..HEAD -- internal/command/adapter/ internal/llm/credential/ internal/command/dispatcher.go` 결과 변경 없음. 본 SPEC 의 모든 변경은 `internal/query/cmdctrl/` 에 한정.

검증 방법: CI 정적 검증 + 코드 리뷰.

### AC-CCWIRE-019 — logger 호출 검증

`fakeLogger` (zap.Logger or test recorder) 주입. 다음 시나리오에서 debug count 증가:
- pool == nil → debug 1회 ("pool unavailable")
- available == 0 → debug 1회 ("pool exhausted")
- preWarm 시작 → debug 1회 (옵션 — REQ-CCWIRE-022)
- preWarm 실패 → debug 1회

logger 가 nil 일 때는 호출 없음 (silent).

검증 방법: 단위 테스트 with fake logger.

### AC-CCWIRE-020 — sync.WaitGroup 또는 atomic counter 존재 (Optional REQ-CCWIRE-023)

`LoopControllerImpl` 가 활성 preWarm goroutine 수를 추적하는 필드 (`preWarmCount atomic.Int32` 또는 `preWarmWg sync.WaitGroup`) 를 보유. 테스트가 그 값을 polling 하여 0 으로 수렴하는지 검증.

검증 방법: 단위 테스트 (eventually 0).

---

## 13. TDD 진입 순서 (RED → GREEN → REFACTOR) — run phase 참고

| 순서 | 작업 | 검증 |
|------|------|-----|
| T-001 | `credresolver.go` 신규 — `CredentialPoolResolver` 인터페이스 + `extractProvider` 헬퍼 + `ErrCredentialUnavailable` sentinel | AC-CCWIRE-001, AC-CCWIRE-003, AC-CCWIRE-004 |
| T-002 | `controller.go` 변경 — `Option` 패턴에 `WithCredentialPoolResolver` / `WithPreWarmRefresh` 추가, `LoopControllerImpl` 에 `credResolver` / `preWarmRefresh` / `preWarmCount` 필드 추가 | AC-CCWIRE-002 |
| T-003 | `controller.go RequestModelChange` 본문 확장 — 사전 검증 (resolver nil → skip, provider 추출, PoolFor, Size) | AC-CCWIRE-005, AC-CCWIRE-006, AC-CCWIRE-007, AC-CCWIRE-008, AC-CCWIRE-009, AC-CCWIRE-011, AC-CCWIRE-012 |
| T-004 | `controller.go preWarmRefreshAsync` 신규 helper | AC-CCWIRE-010, AC-CCWIRE-016, AC-CCWIRE-017, AC-CCWIRE-020 |
| T-005 | logger 통합 (debug 호출 추가) | AC-CCWIRE-019 |
| T-006 | race + nil paths 단위 테스트 + 통합 테스트 (adapter + cmdctrl + fakeResolver) | AC-CCWIRE-014, AC-CCWIRE-015 |
| T-007 | 정적 분석 검증 (mutation grep, FROZEN 패키지 diff) | AC-CCWIRE-013, AC-CCWIRE-018 |

### TRUST 5 매핑

| 차원 | 본 SPEC 적용 |
|------|-----------|
| Tested | 20 AC, ≥ 90% coverage, race detector pass |
| Readable | godoc on every exported type, English code comments per `language.yaml` |
| Unified | gofmt + golangci-lint clean |
| Secured | nil dependency graceful degradation, no panic paths, swap 거부 정책으로 silent failure 방지 |
| Trackable | conventional commits, SPEC ID in commit body, MX:ANCHOR on `CredentialPoolResolver` (boundary), MX:NOTE on `extractProvider` |

---

Version: 0.1.0
Last Updated: 2026-04-27
REQ coverage: REQ-CCWIRE-001 ~ REQ-CCWIRE-023 (총 23)
AC coverage: AC-CCWIRE-001 ~ AC-CCWIRE-020 (총 20)
