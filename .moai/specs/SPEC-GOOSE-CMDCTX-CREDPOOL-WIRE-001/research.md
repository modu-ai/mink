# SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 Research — OnModelChange 후 credential pool swap wiring

> **연구 단계** (plan phase, read-only). 본 문서는 SPEC-GOOSE-CREDPOOL-001 (v0.3.0, implemented), SPEC-GOOSE-CMDCTX-001 (v0.1.1, implemented), SPEC-GOOSE-CMDLOOP-WIRE-001 (v0.1.0, planned) 의 코드/스펙 자산을 read-only로 분석하여 본 SPEC 의 wiring surface 를 식별한다. 어떤 의존 SPEC 도 변경하지 않는다.

---

## 1. 연구 목적 (Research Goals)

| ID | 목적 | 산출 |
|----|------|------|
| RG-1 | `internal/llm/credential/` 의 공개 API surface 매핑 | §3 API Surface Map |
| RG-2 | OnModelChange / RequestModelChange 흐름과 credential swap 의 결합 가능성 분석 | §4 Hook Point Analysis |
| RG-3 | 4 strategy (RoundRobin / LRU / Weighted / Priority) 의 model swap 시 행동 분석 | §5 Strategy Compatibility Matrix |
| RG-4 | OAuth refresh trigger 시점 결정: 동기적 (swap 시점) vs 비동기적 (다음 SubmitMessage 직전) | §6 Refresh Timing Decision |
| RG-5 | 신규 provider credential 부재 시 fallback 정책 식별 | §7 Failure Mode Analysis |
| RG-6 | 본 SPEC 의 IN / OUT 경계 명료화 (CREDPOOL-001 / CMDLOOP-WIRE-001 책임 분리) | §8 Scope Boundary |

---

## 2. 의존 SPEC 자산 매트릭스 (FROZEN)

| SPEC | status | 자산 위치 | 본 SPEC 의 사용 |
|------|--------|---------|------------|
| SPEC-GOOSE-CREDPOOL-001 | implemented (v0.3.0, FROZEN) | `internal/llm/credential/` 패키지 전체 | `*CredentialPool` API read-only 호출 (`Select`, `MarkExhaustedAndRotate`, `Reload`, `Reset`, `Size`, `AcquireLease`). `Refresher` 인터페이스 read-only 참조. **CredentialPool 코드/Refresher 인터페이스 변경 없음.** |
| SPEC-GOOSE-CMDCTX-001 | implemented (v0.1.1, FROZEN) | `internal/command/adapter/adapter.go`, `controller.go` | `ContextAdapter.OnModelChange` → `LoopController.RequestModelChange` 의 위임 경로 read-only 참조. **adapter.go / controller.go 변경 없음.** |
| SPEC-GOOSE-CMDLOOP-WIRE-001 | planned (v0.1.0, batch A) | `internal/query/cmdctrl/` (planned) — `LoopControllerImpl.RequestModelChange` | 본 SPEC 은 그 핸들러 안에서 credential pool 호출을 결합하는 **신규 협업자** 역할. CMDLOOP-WIRE-001 가 implemented 되어야 본 SPEC 의 run phase 가능. |
| SPEC-GOOSE-ROUTER-001 | implemented (FROZEN) | `internal/llm/router/registry.go` | provider 식별자 (`ModelInfo.ID == "provider/model"`) → provider name 추출. `*router.ProviderRegistry.Get(name)` 으로 provider metadata 확인. read-only. |
| SPEC-GOOSE-CONFIG-001 | implemented (FROZEN) | `internal/config/` — `LLMConfig.Providers[*].Credentials` | provider→pool 매핑 자료 source. `factory.go` 의 `NewPoolsFromConfig` 가 build 한 `map[string]*CredentialPool` 을 본 SPEC 은 read-only 로 받는다. |

---

## 3. CredentialPool API Surface Map (CREDPOOL-001 read-only)

### 3.1 풀 단위 API

| 메서드 | 시그니처 | 본 SPEC 사용 의도 |
|------|--------|-------------|
| `Select(ctx)` | `func (*CredentialPool) Select(ctx context.Context) (*PooledCredential, error)` | 신규 provider 활성화 시 첫 lease 획득. 만료 임박 OAuth 엔트리는 자동 refresh 트리거 (`triggerRefreshLocked` via `Refresher`). |
| `MarkExhaustedAndRotate(ctx, id, statusCode, retryAfter)` | `(*PooledCredential, error)` | 본 SPEC 범위 외. LLM 요청 실패 경로 (Adapter 측). 본 SPEC 은 호출하지 않는다. |
| `Release(cred)` | `error` | 본 SPEC 범위 외. SubmitMessage 흐름 책임. |
| `MarkExhausted(cred, cooldown)` | `error` | 본 SPEC 범위 외. |
| `MarkError(cred, err)` | `error` | 본 SPEC 범위 외. |
| `Reload(ctx)` | `error` | swap 직후 신규 provider pool 의 source 가 외부 변경됐을 가능성에 대비한 optional refresh hook. **본 SPEC 은 호출하지 않는다 (R-002 참조)**. |
| `Reset(id)` | `error` | 본 SPEC 범위 외 (운영자 수동 영구 고갈 해제). |
| `Size()` | `(total, available int)` | 신규 provider pool 의 가용 entry 수 사전 검증 (fast-fail 경로). |
| `PersistState(ctx)` | `error` | 본 SPEC 범위 외. |

### 3.2 Refresher 인터페이스 (read-only 의존)

```go
// internal/llm/credential/refresher.go
type Refresher interface {
    Refresh(ctx context.Context, cred *PooledCredential) error
}
```

- 구현체는 ADAPTER-001 / CREDENTIAL-PROXY-001 SPEC 이 제공.
- `CredentialPool` 은 `Select` 의 `triggerRefreshLocked` 경로에서 `expires_at - refreshMargin < now` 조건 만족 시 자동 호출 (`pool.go:279-320`).
- **본 SPEC 의 의미**: pool swap 후 첫 `Select(ctx)` 호출이 자동으로 refresh trigger 를 일으킨다. 본 SPEC 은 이 의미론에 의존하며, **별도 refresh API 를 호출하지 않는다.** (§6 참조)

### 3.3 PooledCredential 의 OAuth 의미론

- `ExpiresAt time.Time` — `IsZero()` 이면 영구 유효 (API key). `!IsZero()` 이면 OAuth 토큰.
- `available()` (`pool.go:175-202`) 는 `!c.ExpiresAt.IsZero() && !now.Before(c.ExpiresAt)` 이면 선택 제외. 만료 토큰은 자동으로 pool 후보에서 빠진다.
- `triggerRefreshLocked` 가 `expires_at - refreshMargin < now` 인 엔트리는 미리 refresh 시도. 즉, **swap 시점에 만료 임박 entry 가 있으면 refresh 가 자동 발동된다.**

---

## 4. OnModelChange → Credential Swap Hook Point 분석

### 4.1 현재 (CMDCTX-001 / CMDLOOP-WIRE-001 종합) flow

```
사용자: /model anthropic/claude-opus-4-7
  ↓
dispatcher (PR #50, FROZEN)
  ↓ ResolveModelAlias("anthropic/claude-opus-4-7") → ModelInfo{ID, DisplayName}
ContextAdapter.OnModelChange(info)              [CMDCTX-001 adapter.go:140-168]
  ↓ delegates to
LoopController.RequestModelChange(ctx, info)    [CMDCTX-001 controller.go interface]
  ↓ implemented by (CMDLOOP-WIRE-001 planned)
LoopControllerImpl.RequestModelChange(ctx, info)
  ↓
  c.activeModel.Store(&info)                     [CMDLOOP-WIRE-001 §6.3]
  return nil
```

이 흐름은 model 식별자만 atomic swap. credential pool 은 변경되지 않는다.

### 4.2 본 SPEC 이 추가하는 hook point

본 SPEC 은 **새 인터페이스 `CredentialPoolResolver`** 를 도입하고, `LoopControllerImpl` (CMDLOOP-WIRE-001 implemented 시점) 의 `RequestModelChange` 핸들러에 **선택적 dependency 로 주입** 한다. CMDLOOP-WIRE-001 의 시그니처는 변경하지 않으며, 새 dependency 는 `cmdctrl.New(...)` 의 옵션 파라미터로만 추가된다.

```
LoopControllerImpl.RequestModelChange(ctx, info)
  ├─ activeModel.Store(&info)                    [기존: CMDLOOP-WIRE-001]
  └─ if credResolver != nil:                     [신규: 본 SPEC]
        provider := extractProvider(info.ID)
        pool := credResolver.PoolFor(provider)
        if pool == nil:
          return ErrCredentialUnavailable        [REQ-CCWIRE-?]
        // 사전 검증: pool 가용성
        total, avail := pool.Size()
        if avail == 0:
          return ErrCredentialUnavailable
        // 첫 Select 가 OAuth refresh 트리거를 자동 발동시킨다.
        // pre-warm 옵션 (REQ-CCWIRE-?): 비동기 background refresh 큐 enqueue
        if preWarmEnabled:
          go c.preWarmAsync(ctx, pool)
        return nil
```

### 4.3 Hook 주입 방식 (옵션 비교)

| 옵션 | 위치 | 장단점 |
|------|------|------|
| **A (권장)** | 본 SPEC 이 신규 인터페이스 `CredentialPoolResolver` 정의 + `LoopControllerImpl` 옵션 파라미터로 주입 | CMDLOOP-WIRE-001 의 `RequestModelChange` 시그니처 불변. 옵션 dependency 만 추가 (nil-tolerant). FROZEN 위반 없음. |
| B | `ContextAdapter.OnModelChange` 내부에서 직접 pool 호출 | CMDCTX-001 의 adapter.go 변경 필요 → FROZEN 위반. **거부.** |
| C | dispatcher (PR #50) 내부에서 pool 호출 | COMMAND-001 의 dispatcher 변경 필요 → FROZEN 위반. **거부.** |
| D | 별도 middleware 패턴 (intercepting LoopController) | 인터페이스 wrapping 추가 → 복잡도 증가. 본 SPEC 의 단순한 swap 의미론에 과도. **차선.** |

→ **옵션 A 채택.** §6.3 (spec.md) 에서 정식화.

### 4.4 본 SPEC 이 LoopControllerImpl 을 변경한다는 의미

CMDLOOP-WIRE-001 가 batch A 에서 plan 되어 아직 implemented 가 아니다. 본 SPEC 은 **CMDLOOP-WIRE-001 implemented 직후** 에만 run 가능. 변경 범위는:

- `cmdctrl.New(engine, logger)` → `cmdctrl.New(engine, logger, cmdctrl.WithCredentialPoolResolver(r))` 옵션 추가. 후방 호환.
- `LoopControllerImpl.RequestModelChange` 본문에 nil-tolerant credResolver 호출 추가 (~10 LOC).

**이 변경이 CMDLOOP-WIRE-001 의 FROZEN 정책 위반인지** 는 본 SPEC 의 ratify 단계에서 사용자 결정 필요 (R-006). 후방 호환 추가이므로 frontmatter version bump 만으로 충분할 가능성이 높다.

---

## 5. Strategy Compatibility Matrix (CREDPOOL-001 4 전략)

| 전략 | model swap 시점 행동 | 본 SPEC 영향 |
|------|------------------|------------|
| RoundRobin (`pool.go:25-44`) | 다음 `Select` 호출 시 카운터 기반 순환 | 영향 없음 — pool 단위 카운터, swap 무관 |
| LRU (`pool.go:50-70`) | UsageCount 가 가장 낮은 entry 선택 | 영향 없음 — pool 단위 통계 |
| Weighted (`pool.go:76-119`) | Weight 비례 무작위 | 영향 없음 |
| Priority (`pool.go:125-166`) | Priority max → 동일 시 RR 타이브레이크 | 영향 없음 |

**결론**: 4 전략 모두 model swap 의미론과 직교 (orthogonal). 본 SPEC 은 전략 selection 정책에 개입하지 않는다.

### 5.1 다중 provider 간 strategy 충돌 가능성

각 provider 는 **자체 `*CredentialPool` 인스턴스** 를 가지며 (factory.go `NewPoolsFromConfig` 가 `map[providerName]*CredentialPool` 반환), 전략 인스턴스도 풀별로 독립. 따라서 swap 시 신규 provider pool 의 strategy 가 이전 pool 의 strategy 와 다르더라도 충돌 없음.

**예외**: 두 provider 가 **동일 한 PooledCredential 을 공유** 하는 시나리오 — 현재 factory.go 가 이를 허용하지 않으므로 (provider 별 source 로 분기, `pool_test.go` 시나리오 부재), 본 SPEC 은 가정하지 않는다.

---

## 6. Refresh Timing Decision

### 6.1 옵션 비교

| 옵션 | 트리거 시점 | 장단점 |
|------|---------|------|
| **A (권장)** | 신규 pool 의 첫 `Select(ctx)` 호출 시 자동 (`triggerRefreshLocked` 경유) | 별도 refresh 호출 없음. CREDPOOL-001 의 자동 refresh margin 의미론 그대로 사용. swap 자체의 latency 영향 없음. 단점: 첫 SubmitMessage 가 refresh latency 를 흡수. |
| B | swap 즉시 동기적 refresh 호출 | refresh latency 가 swap 응답 시간에 반영 → `RequestModelChange` 의 latency 보장 위반. 사용자 경험 악화. **거부.** |
| C | swap 후 background goroutine 에서 비동기 refresh (pre-warm) | swap 자체는 빠르다. 첫 SubmitMessage 가 refresh 와 race 할 수 있음 — 그러나 `Select` 의 `triggerRefreshLocked` 가 이미 mutex 보호 + 재호출 안전 (`refreshFailCounts` idempotent), race 무해. 첫 SubmitMessage 가 운 좋게 background refresh 완료 후 호출되면 latency 절약. |

→ **옵션 A 가 기본, 옵션 C 를 Optional REQ 로 추가.** spec.md §6.4 에서 정식화.

### 6.2 신규 provider 의 OAuth 토큰 만료 시나리오

신규 pool 의 모든 entry 가 만료 (`!ExpiresAt.IsZero() && !now.Before(ExpiresAt)`) 인 경우:
- `available()` 가 빈 슬라이스 반환 → `Select` 가 `ErrExhausted` 반환.
- **본 SPEC 의 처리**: `pool.Size()` 의 `available` 카운트로 사전 감지 가능. 0 이면 `ErrCredentialUnavailable` 즉시 반환 (REQ-CCWIRE-?).
- `triggerRefreshLocked` 는 만료된 entry 를 skip 한다 (`Status == CredExhausted` 또는 만료 후 cooldown 미만료). 즉, 만료된 토큰은 자동 refresh 되지 않는다 (refresh 는 만료 임박 entry 만 대상).

**fallback 정책**: 본 SPEC 은 신규 pool 의 가용 entry 가 0 이면 swap 자체를 거부 (REQ-CCWIRE-?). model 식별자는 atomic swap 되지 않는다. 사용자는 다른 provider 를 선택하거나 OAuth 재로그인을 수행해야 한다.

### 6.3 CMDLOOP-WIRE-001 의 atomic swap 과의 순서

```
RequestModelChange(ctx, info):
  1. pre-validate credential availability (본 SPEC)
     ├─ if credResolver == nil: skip (no-op, 후방 호환)
     ├─ pool = credResolver.PoolFor(provider)
     ├─ if pool == nil: return ErrCredentialUnavailable
     └─ if pool.Size().available == 0: return ErrCredentialUnavailable
  2. activeModel.Store(&info)              [CMDLOOP-WIRE-001 기존]
  3. (optional) preWarm 비동기 큐 enqueue  [본 SPEC REQ-CCWIRE-?]
  4. return nil
```

**선후 결정**: pre-validate (1) 가 swap (2) 보다 먼저. 이유: pool 가용성 부재 시 swap 자체를 거부해야 한다 (사용자가 잘못된 model 식별자로 갇히는 것 방지). REQ-CCWIRE-? 가 정식화.

---

## 7. Failure Mode Analysis

| ID | 시나리오 | 본 SPEC 처리 |
|----|--------|----------|
| F-1 | `credResolver == nil` (CLI 진입점이 dependency 미주입) | no-op. 기존 CMDLOOP-WIRE-001 의미론 유지 (model 식별자 swap 만). 후방 호환 보장. |
| F-2 | `info.ID` 가 `provider/model` 형식 아님 | 이미 CMDCTX-001 ResolveModelAlias 가 거부 (REQ-CMDCTX-009). 본 SPEC 도달 전 차단. defensive: extractProvider 에서 빈 문자열이면 `ErrInvalidModelInfo` 반환 (CMDLOOP-WIRE-001 §6.3 sentinel 재사용). |
| F-3 | provider name 이 `credResolver.PoolFor` 에 등록되지 않음 (provider 가 credential 미설정) | `ErrCredentialUnavailable` 반환. swap 거부. activeModel 변경 없음. |
| F-4 | provider pool 의 모든 entry 만료 (`available == 0`) | `ErrCredentialUnavailable` 반환. swap 거부. |
| F-5 | provider pool 의 일부 entry 만 만료 임박 (refresh 대상) | swap 성공. 첫 SubmitMessage 시 자동 refresh (CREDPOOL-001 `triggerRefreshLocked`). |
| F-6 | provider pool 의 entry refresh 가 실패 (`refreshFailCounts >= 3`) | swap 시점에는 정상 (다른 entry 가 가용). 실제 LLM 요청 시 `Select` 가 `ErrExhausted` 반환 → ADAPTER 의 `MarkExhaustedAndRotate` 흐름으로 처리 (본 SPEC 범위 외). |
| F-7 | preWarm goroutine 이 ctx 취소 후에도 살아있음 | preWarm 은 ctx 를 받아서 `Select(ctx)` 호출. ctx 취소 시 즉시 종료 (CREDPOOL-001 `Select` line 239-243). leak 없음. |
| F-8 | 동시 RequestModelChange (서로 다른 provider) → race | activeModel.Store 는 atomic. credResolver.PoolFor 는 read-only map lookup (immutable after `New`). race-free. |

---

## 8. Scope Boundary

### 8.1 IN SCOPE (본 SPEC 이 정의)

| ID | 항목 |
|----|------|
| IN-1 | `CredentialPoolResolver` 인터페이스 신규 정의 (1 메서드: `PoolFor(provider string) *credential.CredentialPool`) |
| IN-2 | CMDLOOP-WIRE-001 implemented 의 `LoopControllerImpl` 에 `WithCredentialPoolResolver(r)` 옵션 추가 (~5 LOC) |
| IN-3 | `LoopControllerImpl.RequestModelChange` 본문에 nil-tolerant credResolver 호출 추가 (~10 LOC) |
| IN-4 | `ErrCredentialUnavailable` sentinel error 신규 정의 |
| IN-5 | `extractProvider(info.ID) string` 헬퍼 — `ModelInfo.ID` 에서 첫 `/` 앞부분 추출. `info.ID == ""` 또는 `/` 없음이면 `""` 반환. |
| IN-6 | (Optional) preWarm 비동기 background refresh 옵션 — `WithPreWarmRefresh(true)` 가 enable 시 swap 후 background goroutine 에서 `Select(ctx)` 1회 (refresh 트리거만) |
| IN-7 | 단위 테스트 (fake CredentialPoolResolver, fake CredentialPool stub), race detector pass, coverage ≥ 90% |

### 8.2 OUT OF SCOPE (본 SPEC 이 명시적으로 제외 — Exclusions §10 에서 정식화)

| ID | 항목 | 위임 대상 |
|----|------|-------|
| OUT-1 | CredentialPool 자체의 정책 수정 (cooldown, retry, refresh margin 등) | CREDPOOL-001 책임. 본 SPEC 변경 불가. |
| OUT-2 | OAuth flow UI (사용자가 brave/firefox 로 OAuth 페이지 열기) | CLI-INTEG / DAEMON-INTEG SPEC 범위 |
| OUT-3 | Refresher 인터페이스의 구현체 (실제 OAuth refresh 로직) | ADAPTER-001 / CREDENTIAL-PROXY-001 SPEC |
| OUT-4 | provider→pool 매핑 자체의 wiring (config → pools) | CONFIG-001 / factory.go (이미 implemented) |
| OUT-5 | 만료된 entry 의 재로그인 (운영자 수동 영구 고갈 해제) | CREDPOOL-001 `Reset(id)` API + CLI 진입점 |
| OUT-6 | 다중 provider 간 credential 공유 / cross-provider rotation | 후속 SPEC. 현재 factory 가 provider 별 독립 pool 보장. |
| OUT-7 | `Reload(ctx)` 호출 (config 동적 갱신) | 후속 SPEC (config hot-reload) |
| OUT-8 | LLM 요청 실패 경로의 credential rotation (`MarkExhaustedAndRotate`) | ADAPTER-001 SPEC 범위 |
| OUT-9 | model swap 시점의 dispatcher / OnModelChange 의미론 변경 | CMDCTX-001 / COMMAND-001 FROZEN. 변경 불가. |

---

## 9. 의존 SPEC 정합성 사전 검증 (planning 단계 — read-only)

### 9.1 CREDPOOL-001 (v0.3.0, implemented)

- ✅ `Select(ctx)` / `Size()` / `Reload(ctx)` / `Reset(id)` 모두 `pool.go` 에 존재 (line 237/372/383/392).
- ✅ `Refresher` 인터페이스 `refresher.go` line 12-16.
- ✅ 4 strategy `strategy.go` line 25/50/76/125 존재.
- ✅ `factory.NewPoolsFromConfig` `factory.go` line 46 — `map[string]*CredentialPool` 반환.
- ✅ `ErrExhausted` / `ErrNotFound` 정의 `pool.go` line 11/14.

### 9.2 CMDCTX-001 (v0.1.1, implemented)

- ✅ `ContextAdapter.OnModelChange` 존재 (`adapter.go:140-168` per progress.md).
- ✅ `LoopController.RequestModelChange(ctx, info command.ModelInfo) error` 인터페이스 (`controller.go:19-51` per CMDCTX-001 progress).
- ✅ `command.ModelInfo` 타입 정의 (`SlashCommandContext` 의 일부).

### 9.3 CMDLOOP-WIRE-001 (v0.1.0, planned)

- ⏳ 아직 implemented 아님. 본 SPEC 의 run phase 진입 전제 조건: CMDLOOP-WIRE-001 가 먼저 implemented.
- 본 SPEC 의 spec.md §7 에 명시 의존.
- ⚠️ CMDLOOP-WIRE-001 의 D-001 (패키지 위치) / D-002 (`PreIteration` 훅) 결정에 따라 본 SPEC 의 import path 가 달라진다 (옵션 C 권장 시 `internal/query/cmdctrl`). 현재 권장안 가정.

### 9.4 ROUTER-001 (implemented, FROZEN)

- ✅ `*router.ProviderRegistry.Get(name)` 가 존재 (CMDCTX-001 spec.md §6.2 import).
- 본 SPEC 은 provider 존재 검증을 위해 사용 가능 (optional, F-3 보강용). 현재 단계에서는 `credResolver.PoolFor` 의 nil 결과로 충분히 처리 — Registry 조회 추가 의존성 없음.

---

## 10. 위험 영역 (Risk Surface) — spec.md §9 의 input

| ID | 위험 | 영향 | 1차 완화 |
|----|------|------|------|
| R-001 | CREDPOOL-001 의 정확한 API surface 가 v0.3.0 implemented 와 다를 가능성 (drift) | 본 SPEC 의 wiring 가정 무너짐 | 본 research.md §3 에서 `pool.go` 의 line 단위 검증 완료 (2026-04-27 기준). run phase Phase 1 (ANALYZE) 에서 재검증. |
| R-002 | swap 시 `Reload(ctx)` 를 호출할 필요가 있는가 | 호출 시 source.Load() 가 디스크/네트워크 I/O 유발 → swap latency 악화 | 본 SPEC 은 호출하지 않는다 (§3.1). pool 의 lifecycle 은 CREDPOOL-001 / factory 책임. swap 은 active credential 선택만 변경. |
| R-003 | OAuth refresh 가 LoopController.RequestModelChange 응답 latency 에 영향 | UI 응답성 악화 | 옵션 A 채택 (§6.1) — refresh 는 첫 `Select` 시 자동, swap 자체에는 영향 없음. 옵션 C 의 preWarm 도 background goroutine. |
| R-004 | 다중 provider 간 credential rotation 정책 충돌 | 동일 KeyringID 가 두 provider 에 등록 시 race | 현재 factory.go 가 provider 별 독립 pool 만 build (§5.1) → 충돌 없음. 본 SPEC 은 multi-pool 공유를 가정하지 않는다. |
| R-005 | preWarm goroutine 의 ctx 누수 (사용자가 다른 model 로 즉시 swap) | goroutine leak | preWarm 은 swap 시점 ctx 를 받음 + ctx.Done 검사. 추가로 sync.WaitGroup 으로 LoopControllerImpl.Close 시 drain 가능 (Optional). 본 SPEC 의 EARS 에서 명시. |
| R-006 | CMDLOOP-WIRE-001 의 `LoopControllerImpl` 변경이 그 SPEC 의 FROZEN 위반인가 | run phase 진입 차단 | 후방 호환 옵션 dependency 만 추가 (시그니처 불변). 사용자 결정 필요. CMDLOOP-WIRE-001 implemented 후 본 SPEC plan 재검증 권장. |
| R-007 | `ErrCredentialUnavailable` 이 dispatcher / OnModelChange 의 에러 표시 방식과 호환되는가 | 사용자 에러 메시지 누락 | sentinel error 로 정의 + `errors.Is` 호환. dispatcher 의 에러 처리 (CMDCTX-001 OnModelChange return value) 에 그대로 전달됨. |
| R-008 | CredentialPoolResolver 의 nil 에 대한 테스트 누락 | 후방 호환 결손 | spec.md §AC 에 nil-resolver 시나리오 명시. fake CredentialPoolResolver + nil case 둘 다 검증. |

---

## 11. 후속 SPEC 와의 관계

| 후속 SPEC (가칭) | 관계 |
|------------|------|
| SPEC-GOOSE-CLI-001 / SPEC-GOOSE-DAEMON-WIRE-001 | CLI 진입점에서 `cmdctrl.New(engine, logger, cmdctrl.WithCredentialPoolResolver(resolver))` wiring 작성. 본 SPEC 산출물의 사용자. |
| SPEC-GOOSE-ADAPTER-001 / SPEC-GOOSE-CREDENTIAL-PROXY-001 | Refresher 구현체 제공. 본 SPEC 은 Refresher 인터페이스만 read-only 의존. |
| SPEC-GOOSE-ALIAS-CONFIG-001 (planned) | model alias 의 config 파일 로드. 본 SPEC 의 동작과 직교. ResolveModelAlias 결과의 ID 가 본 SPEC 입력. |
| (후속) SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-002 (가칭) | hot-reload, multi-pool 공유, 동적 strategy 변경 등 본 SPEC 의 OUT-7/OUT-6 항목 |

---

## 12. 결정 사항 후보 (사용자 ratify 단계 — spec.md 작성 후 재확인)

| ID | 결정 사항 | 본 research 의 권장 | 영향 |
|----|--------|---------------|------|
| D-001 | Hook 주입 방식 (옵션 A/B/C/D) | **A** — `CredentialPoolResolver` 옵션 dependency | LoopControllerImpl 의 New 시그니처에 옵션 추가. 후방 호환. |
| D-002 | Refresh trigger 시점 | **옵션 A 기본 + 옵션 C optional** — 자동 refresh on 첫 Select; preWarm 은 Optional REQ | swap latency 보존, 첫 SubmitMessage 가 refresh latency 흡수 |
| D-003 | 신규 pool 의 `available == 0` 시 fallback | **swap 거부 + ErrCredentialUnavailable** | activeModel 변경 안 됨. 사용자가 OAuth 재로그인 또는 다른 provider 선택해야 함. |
| D-004 | preWarm 의 implementation | **별도 goroutine + ctx 검사**. `LoopControllerImpl.Close` 시 drain 은 Optional. | Optional REQ |
| D-005 | `extractProvider` 의 알고리즘 | `strings.SplitN(info.ID, "/", 2)[0]`. 빈 문자열이면 `""` 반환. | 기존 CMDCTX-001 ResolveModelAlias 와 일관 |
| D-006 | CMDLOOP-WIRE-001 의 `New` 시그니처 변경에 대한 그 SPEC frontmatter version bump 여부 | **변경 (0.1.x → 0.2.0)**. 후방 호환 옵션 추가지만 외부 surface 변경. CMDLOOP-WIRE-001 implemented 후 본 SPEC 머지 시 동시 patch. | CMDLOOP-WIRE-001 의 ratify 단계에서 합의 필요 |
| D-007 | `ErrCredentialUnavailable` 의 위치 | `internal/query/cmdctrl/errors.go` (CMDLOOP-WIRE-001 의 ErrEngineUnavailable / ErrInvalidModelInfo 와 동일 패키지). | sentinel error 그룹 응집 |

---

## 13. 다음 단계

1. spec.md 작성 (EARS 형식, 본 research 의 §6/7/8/10 입력).
2. plan-auditor iter1 (선택): EARS 형식 / FROZEN SPEC 변경 부재 / REQ-AC 매트릭스 / R-001 ~ R-008 mitigation.
3. 사용자 ratify: AskUserQuestion 으로 D-001 ~ D-007 결정.
4. CMDLOOP-WIRE-001 implemented 대기.
5. /moai run SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 (사용자 승인 시).

---

Version: 0.1.0
Last Updated: 2026-04-27
