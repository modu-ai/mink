---
id: SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001
version: 0.1.0
status: planned
created_at: 2026-04-27
updated_at: 2026-04-27
author: manager-spec
priority: P4
issue_number: null
phase: 3
size: 극소(XS)
lifecycle: spec-anchored
labels: [area/router, type/feature, priority/p4-low]
---

# SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 — Permissive Alias Mode (CMDCTX v0.2 Amendment)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. SPEC-GOOSE-CMDCTX-001 v0.1.1 (implemented, FROZEN) §Risks R2 와 §Exclusions #7 의 후속 amendment SPEC. ContextAdapter `Options` 에 `AliasResolveMode` enum 필드를 추가하여 strict / PermissiveProvider / Permissive 3종 모드를 도입하고, `ResolveModelAlias` 알고리즘 step 7 분기를 추가한다. 본 SPEC 의 implementation 시점에 CMDCTX-001 SPEC 본문도 v0.1.1 → v0.2.0 amendment 가 동시 발생함 (지금은 plan 단계이므로 CMDCTX-001 본문 변경 금지). | manager-spec |

---

## 1. 개요 (Overview)

본 SPEC 은 `SPEC-GOOSE-CMDCTX-001` v0.1.1 (implemented) 에 정의된 `ContextAdapter.ResolveModelAlias` 알고리즘의 strict-only 정책을 완화하는 amendment 이다.

CMDCTX-001 v0.1.1 §6.4 알고리즘은 step 7 에서 `meta.SuggestedModels` 에 등록되지 않은 모델을 무조건 `command.ErrUnknownModel` 로 거부한다. 본 SPEC 은:

- `Options.AliasResolveMode AliasResolveMode` enum 필드를 추가한다.
- 3종 모드(`Strict` / `PermissiveProvider` / `Permissive`) 를 정의한다.
- `Strict` 가 zero-value default 이므로 backward compatibility 가 자동 보장된다.
- `PermissiveProvider` 모드: provider lookup 성공한 경우 SuggestedModels 검증 생략. warn-log emit.
- `Permissive` 모드: PermissiveProvider 와 동일하지만 향후 확장 여지 (provider-unknown 시 별도 정책 가능성). 본 SPEC 의 §6 에서 두 mode 의 동작 차이를 한정한다.
- per-provider opt-in 의 단순 형태로 `aliasMap` 의 wildcard 키 `provider/*` 를 Optional REQ 로 추가한다.

본 SPEC 의 수락은 CMDCTX-001 SPEC 본문의 v0.1.1 → v0.2.0 amendment 와 함께 발생한다 (run phase 시점).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

OpenRouter 등 multi-tenant LLM gateway provider 는 단일 provider 명(`openrouter`) 아래 nested model ID(예: `deepseek/deepseek-r1:free`, `meta-llama/llama-3.3-70b-instruct:free`) 를 동적으로 노출한다. ROUTER-001 의 `ProviderMeta.SuggestedModels` 는 본래 "권장" 모델 힌트만을 노출하도록 설계되었으나, CMDCTX-001 §6.4 step 7 이 이를 strict allow-list 게이트로 사용하여 의미론적 mismatch 를 발생시킨다.

CMDCTX-001 v0.1.1 §9 Risks R2 가 이 문제를 명시적으로 인지하였으며, §Exclusions #7 이 본 SPEC 을 후속 plan 으로 약속하였다. research.md §1.2 / §1.3 참조.

### 2.2 상속 자산

- **SPEC-GOOSE-CMDCTX-001** v0.1.1 (implemented): `ContextAdapter`, `Options`, `ResolveModelAlias`, alias.go. **본 SPEC 의 amendment 대상**. 본 SPEC 의 implementation 시점에 CMDCTX-001 의 frontmatter version 0.1.1 → 0.2.0 으로 동시 갱신된다.
- **SPEC-GOOSE-COMMAND-001** (implemented, FROZEN): `command.ErrUnknownModel` sentinel(`internal/command/errors.go:23-25`). 본 SPEC 은 재사용.
- **SPEC-GOOSE-ROUTER-001** (implemented, FROZEN): `*router.ProviderRegistry`, `ProviderMeta.SuggestedModels`. 본 SPEC 은 read-only 사용. **변경 없음**.

### 2.3 범위 경계 (한 줄)

- **IN**: `AliasResolveMode` enum 신설, `Options.AliasResolveMode` 필드 추가, `ResolveModelAlias` step 7 분기, warn-log, Optional `provider/*` wildcard alias key, AC 신규 4종 + CMDCTX-001 v0.2.0 본문 amendment governance.
- **OUT**: ProviderMeta 변경(옵션 B 미채택), telemetry counter, config hot-reload, mode 4 종 이상 추가, ProviderRegistry mutation, CMDCTX-001 §6.4 외 영역 변경.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC 이 정의/구현하는 것)

1. **신규 enum 타입**:
   - `internal/command/adapter/options.go` (또는 `adapter.go` 내) 에 `AliasResolveMode int` 타입 + 상수 3종(`AliasResolveStrict`, `AliasResolveModePermissiveProvider`, `AliasResolveModePermissive`) 정의.
   - 각 상수에 godoc 명기.

2. **`Options` struct 확장**:
   - 기존 `Options` 에 `AliasResolveMode AliasResolveMode` 필드 추가.
   - zero-value(`AliasResolveStrict`) 가 기본값 → backward compatibility 자동 보장.

3. **`ResolveModelAlias` 알고리즘 분기 추가**:
   - CMDCTX-001 §6.4 step 7 에 mode 별 switch 추가.
   - `Strict`: 기존 동작 유지(`return nil, ErrUnknownModel`).
   - `PermissiveProvider`: provider lookup 성공 + model not in SuggestedModels → `*ModelInfo` 반환, warn-log emit.
   - `Permissive`: PermissiveProvider 와 step 7 동작 동일. (provider-unknown 분기는 §6.4 step 6 에서 여전히 hard fail 유지 — 본 SPEC §6 알고리즘 표 참조.)

4. **잘못된 enum 값 처리**:
   - `AliasResolveMode` 가 정의되지 않은 정수값(예: `AliasResolveMode(99)`) 인 경우 default `Strict` 적용.

5. **warn-log 메시지 형식**:
   - `logger.Warn` 호출 시 메시지 형식: `"resolveAlias: model not in SuggestedModels but permissive mode allows"` 와 함께 fields `provider`, `model`, `mode` 포함.

6. **Optional `provider/*` wildcard alias map key**:
   - `aliasMap` 의 키가 `provider/*` 형태(예: `openrouter/*`) 인 경우 매칭된 입력을 strict 검증 우회 (=PermissiveProvider mode 와 동일 효과).
   - 본 기능은 Optional REQ 로 정의. 구현은 stub 가능 (REQ 미충족 시 향후 별도 SPEC).

7. **AC 신규 4종**:
   - permissive 모드의 happy path / strict default backward compat / unknown provider 경계 / warn-log 검증.
   - 기존 CMDCTX-001 의 19 AC 는 유지 (회귀 금지).

8. **CMDCTX-001 v0.2.0 amendment governance**:
   - 본 SPEC implementation 시점에 CMDCTX-001 spec.md 의 frontmatter `version: 0.1.1 → 0.2.0` 갱신.
   - HISTORY 에 v0.2.0 항목 1줄 추가 (본 SPEC ID 인용).
   - §6.2 / §6.4 본문 갱신 (step 7 분기 반영).
   - §9 Risks R2 완화 칸에 본 SPEC 링크.
   - 위 변경은 본 SPEC 의 산출물 일부이며, 동일 PR 에 포함되어야 한다.

### 3.2 OUT OF SCOPE (명시적 제외)

본 SPEC 은 다음을 정의하지 않는다 — 어느 후속 SPEC 이 채워야 하는지 명시:

1. **Telemetry counter** — `cmdctx_permissive_unsuggested_model_total` 등의 metrics emission. → SPEC-GOOSE-TELEMETRY-001 (TBD).
2. **Config hot-reload** — `aliasMap` / `AliasResolveMode` 의 런타임 갱신. → SPEC-GOOSE-HOTRELOAD-001 (TBD).
3. **`ProviderMeta.AllowUnsuggestedModels` per-provider flag** — research.md §3.2 옵션 B. ROUTER-001 (FROZEN) 변경 필요. 본 SPEC 채택안과 mutually exclusive 는 아니지만 별도 SPEC 으로 분리.
4. **Wildcard alias key 의 specificity / ordering 규칙** — 본 SPEC Optional REQ 는 단순 `provider/*` 형태만 지원. `provider/sub/*` 같은 다중 wildcard 는 후속 SPEC.
5. **Provider HTTP 4xx 응답의 user-facing 메시지 표준화** — SPEC-GOOSE-ERROR-CLASS-001 의 책임.
6. **mode 4 종 이상 추가** (`Experimental`, `DevPreview` 등) — 별도 SPEC 필요. 본 SPEC 은 3 종에 한정.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CMDCTX-PA-001** — The `Options` struct **shall** include a field `AliasResolveMode AliasResolveMode` whose zero-value (`AliasResolveStrict`) reproduces the behavior defined in `SPEC-GOOSE-CMDCTX-001` v0.1.1 §6.4 step 7 (return `command.ErrUnknownModel` for models not in `meta.SuggestedModels`).

**REQ-CMDCTX-PA-002** — The `AliasResolveMode` enum type **shall** define exactly three constants: `AliasResolveStrict` (= 0, default), `AliasResolveModePermissiveProvider`, and `AliasResolveModePermissive`. Adding additional modes **shall** require a separate SPEC amendment.

**REQ-CMDCTX-PA-003** — The `ContextAdapter.ResolveModelAlias` method **shall not** mutate any external state regardless of the active `AliasResolveMode` value (preserves SPEC-GOOSE-CMDCTX-001 REQ-CMDCTX-002 / REQ-CMDCTX-016 invariants).

### 4.2 Event-Driven (이벤트 기반)

**REQ-CMDCTX-PA-004** — **When** `AliasResolveMode == AliasResolveModePermissiveProvider` AND provider lookup via `registry.Get(provider)` succeeds AND `model NOT in meta.SuggestedModels`, the adapter **shall** return `(*ModelInfo{ID: provider+"/"+model, DisplayName: meta.DisplayName + " " + model}, nil)` instead of `(nil, command.ErrUnknownModel)`.

**REQ-CMDCTX-PA-005** — **When** the conditions of REQ-CMDCTX-PA-004 are met AND the adapter has a non-nil `Logger`, the adapter **shall** call `logger.Warn("resolveAlias: model not in SuggestedModels but permissive mode allows", "provider", provider, "model", model, "mode", modeName)` exactly once per `ResolveModelAlias` invocation.

**REQ-CMDCTX-PA-006** — **When** `AliasResolveMode == AliasResolveModePermissive`, the adapter **shall** behave identically to `PermissiveProvider` for step 7 (i.e., model-not-in-SuggestedModels is allowed) AND **shall** continue to return `(nil, command.ErrUnknownModel)` for the provider-unknown case (step 6 hard fail unchanged).

### 4.3 State-Driven (상태 기반)

**REQ-CMDCTX-PA-007** — **While** `AliasResolveMode == AliasResolveStrict` (default), every call to `ResolveModelAlias` **shall** behave bit-for-bit identically to `SPEC-GOOSE-CMDCTX-001` v0.1.1 (no observable difference in return values, no warn-log emission, no side effects).

**REQ-CMDCTX-PA-008** — **While** `registry == nil` (provider registry not injected), every call to `ResolveModelAlias` **shall** return `(nil, command.ErrUnknownModel)` regardless of the `AliasResolveMode` value (preserves SPEC-GOOSE-CMDCTX-001 REQ-CMDCTX-014).

### 4.4 Unwanted Behavior (방지)

**REQ-CMDCTX-PA-009** — **If** the `AliasResolveMode` field carries an integer value not corresponding to any defined constant (e.g., `AliasResolveMode(99)` as a result of unsafe casting or stale binary mismatch), **then** the adapter **shall** treat the value as `AliasResolveStrict` and **shall not** panic. The adapter **may** emit a warn-log noting the invalid mode.

**REQ-CMDCTX-PA-010** — **If** the active mode is `AliasResolveModePermissiveProvider` or `AliasResolveModePermissive` AND warn-log emission fails (e.g., logger panics internally), **then** the adapter **shall** still return the model info (logging is best-effort, must not block the resolution path). This SHALL be enforced via a recover() in the warn-log call site.

### 4.5 Optional (선택적)

**REQ-CMDCTX-PA-011** — **Where** the `aliasMap` injected via `Options.AliasMap` contains a key matching the wildcard pattern `provider/*` (where `provider` is a literal provider name and `*` is the wildcard), **then** invocations of `ResolveModelAlias` with input `provider/<any-model>` **shall** be treated equivalent to `PermissiveProvider` mode for that provider only (per-provider opt-in via aliasMap). Implementation may stub this REQ if the wildcard matcher is not yet wired; in that case the wildcard key is treated as a literal alias and would not match.

**REQ-CMDCTX-PA-012** — **Where** a future telemetry counter (SPEC-GOOSE-TELEMETRY-001) is wired, **then** every permissive-mode allow event **shall** increment a counter `cmdctx_permissive_unsuggested_model_total{provider=...}`. Counter wiring is out of scope of this SPEC; the hook point (e.g., a `metrics` interface in `Options`) is sufficient.

---

## 5. 수용 기준 (Acceptance Criteria)

| AC ID | 검증 대상 REQ | Given-When-Then |
|-------|---------------|-----------------|
| **AC-CMDCTX-PA-001** | REQ-CMDCTX-PA-001, REQ-CMDCTX-PA-007 | **Given** `New(Options{Registry: DefaultRegistry(), LoopController: fakeLoop})` (AliasResolveMode 미설정 → zero-value Strict) **And** `meta.SuggestedModels` 가 `model-x` 를 포함하지 않음 **When** `ResolveModelAlias("provider-y/model-x")` 호출 **Then** `(nil, command.ErrUnknownModel)` 반환 (CMDCTX-001 v0.1.1 동작과 동일, backward compat 검증) |
| **AC-CMDCTX-PA-002** | REQ-CMDCTX-PA-002, REQ-CMDCTX-PA-004 | **Given** `New(Options{Registry: DefaultRegistry(), LoopController: fakeLoop, AliasResolveMode: AliasResolveModePermissiveProvider})` **And** registry 에 `openrouter` provider 등록(`SuggestedModels=[]`) **When** `ResolveModelAlias("openrouter/deepseek/deepseek-r1:free")` 호출 **Then** `(*ModelInfo{ID:"openrouter/deepseek/deepseek-r1:free", ...}, nil)` 반환, error 없음 |
| **AC-CMDCTX-PA-003** | REQ-CMDCTX-PA-005 | **Given** AC-CMDCTX-PA-002 와 동일 환경 **And** `fakeWarnLogger` 주입 **When** AC-CMDCTX-PA-002 의 호출 수행 **Then** `fakeWarnLogger.WarnCount == 1` **And** Warn 메시지에 `provider="openrouter"`, `model="deepseek/deepseek-r1:free"`, `mode` 필드 포함 |
| **AC-CMDCTX-PA-004** | REQ-CMDCTX-PA-006, REQ-CMDCTX-PA-008 | **Given** `AliasResolveMode: AliasResolveModePermissive` **And** registry 에 `nonexistent` provider 미등록 **When** `ResolveModelAlias("nonexistent/foo")` 호출 **Then** `(nil, command.ErrUnknownModel)` 반환 (provider-unknown 은 permissive 모드에서도 hard fail) |
| **AC-CMDCTX-PA-005** | REQ-CMDCTX-PA-009 | **Given** `AliasResolveMode: AliasResolveMode(99)` (정의되지 않은 값) 으로 New 호출 **When** `ResolveModelAlias("provider/unknown-model")` 호출 (model not in SuggestedModels) **Then** `(nil, command.ErrUnknownModel)` 반환 (Strict fallback), panic 없음 |
| **AC-CMDCTX-PA-006** | REQ-CMDCTX-PA-003 | **Given** `AliasResolveMode: AliasResolveModePermissiveProvider` **And** registry / aliasMap 의 deep-copy snapshot 캡처 **When** `ResolveModelAlias` 임의 입력 100회 호출 **Then** snapshot 비교 시 등록 데이터 변경 0건 (ResolveModelAlias 의 비-mutation invariant) |
| **AC-CMDCTX-PA-007** | REQ-CMDCTX-PA-010 | **Given** `AliasResolveMode: AliasResolveModePermissiveProvider` **And** `panickingLogger` 가 Warn 호출 시 panic 을 일으키도록 주입 **When** `ResolveModelAlias("openrouter/some-model")` 호출 (permissive allow path) **Then** `(*ModelInfo, nil)` 반환, panic 이 호출자까지 전파되지 않음 (recover 동작) |
| **AC-CMDCTX-PA-008** | REQ-CMDCTX-PA-011 | **Given** `aliasMap = {"openrouter/*": ""}` **And** `AliasResolveMode: AliasResolveStrict` **When** `ResolveModelAlias("openrouter/deepseek/deepseek-r1:free")` 호출 **Then** wildcard 매칭 wired 시: `(*ModelInfo, nil)` 반환. wildcard 미구현(stub) 시: `(nil, command.ErrUnknownModel)` 반환 (REQ Optional 정책에 따라 양 결과 모두 허용, 단 결정된 동작이 일관되어야 함). Warn-log 미사용 (Strict 기본 정책). |

**커버리지 매트릭스**:

| REQ | AC들 |
|-----|------|
| REQ-CMDCTX-PA-001 | AC-CMDCTX-PA-001 |
| REQ-CMDCTX-PA-002 | AC-CMDCTX-PA-002 |
| REQ-CMDCTX-PA-003 | AC-CMDCTX-PA-006 |
| REQ-CMDCTX-PA-004 | AC-CMDCTX-PA-002 |
| REQ-CMDCTX-PA-005 | AC-CMDCTX-PA-003 |
| REQ-CMDCTX-PA-006 | AC-CMDCTX-PA-004 |
| REQ-CMDCTX-PA-007 | AC-CMDCTX-PA-001 |
| REQ-CMDCTX-PA-008 | AC-CMDCTX-PA-004 |
| REQ-CMDCTX-PA-009 | AC-CMDCTX-PA-005 |
| REQ-CMDCTX-PA-010 | AC-CMDCTX-PA-007 |
| REQ-CMDCTX-PA-011 | AC-CMDCTX-PA-008 |
| REQ-CMDCTX-PA-012 | (구현 hook 만 정의, 검증 AC 없음 — 후속 TELEMETRY-001 SPEC 의 책임) |

총 12 REQ / 8 AC. REQ-CMDCTX-PA-012 는 hook stub 만 정의하므로 별도 AC 없음. 나머지 11 REQ 가 모두 최소 1개의 AC 로 검증된다.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 변경 surface

```
internal/command/adapter/
├── adapter.go            # ⬅︎ Options struct 에 AliasResolveMode 필드 추가
├── alias.go              # ⬅︎ ResolveModelAlias step 7 분기 추가
├── options.go (또는 adapter.go 내)  # ⬅︎ AliasResolveMode enum + 상수 신규
├── adapter_test.go       # ⬅︎ AC-CMDCTX-PA-001..008 테이블 케이스 추가
├── alias_test.go         # ⬅︎ wildcard 매칭 테이블 케이스 (Optional)
└── (기존 파일들 — 변경 없음)
```

### 6.2 신규 enum 정의 (Go 시그니처)

```go
// AliasResolveMode controls how ResolveModelAlias treats models that pass
// the provider lookup but are not in meta.SuggestedModels.
//
// The zero value is AliasResolveStrict, which preserves backward
// compatibility with SPEC-GOOSE-CMDCTX-001 v0.1.1 behavior.
//
// @MX:NOTE: Adding new modes requires a separate SPEC amendment.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 REQ-CMDCTX-PA-002
type AliasResolveMode int

const (
    // AliasResolveStrict (default) returns command.ErrUnknownModel for any
    // model not in meta.SuggestedModels. CMDCTX-001 v0.1.1 behavior.
    AliasResolveStrict AliasResolveMode = iota

    // AliasResolveModePermissiveProvider allows models not in SuggestedModels
    // when the provider lookup itself succeeded. Emits a warn-log when this
    // path is taken. Recommended for OpenRouter-like multi-tenant providers
    // that do not enumerate every nested model ID.
    AliasResolveModePermissiveProvider

    // AliasResolveModePermissive is reserved for future expansion. Currently
    // behaves identically to AliasResolveModePermissiveProvider for step 7
    // (model check). Provider-unknown is still a hard fail.
    AliasResolveModePermissive
)
```

### 6.3 `Options` 변경 (Go 시그니처)

```go
// Options is the constructor parameter bag.
//
// @MX:NOTE: AliasResolveMode added in v0.2.0 amendment by
// SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001. Zero-value preserves CMDCTX-001
// v0.1.1 strict behavior.
type Options struct {
    Registry         *router.ProviderRegistry
    LoopController   LoopController
    AliasMap         map[string]string
    AliasResolveMode AliasResolveMode  // ⬅︎ NEW (zero-value = Strict)
    GetwdFn          func() (string, error)
    Logger           Logger
}
```

### 6.4 `ResolveModelAlias` 알고리즘 (CMDCTX-001 §6.4 amendment)

```
ResolveModelAlias(alias):
  1. if registry == nil: return nil, ErrUnknownModel             (REQ-CMDCTX-PA-008)
  2. if aliasMap matches alias (exact OR wildcard):              (REQ-CMDCTX-PA-011)
       canonical, useWildcardPermissive := aliasMap.lookup(alias)
     else:
       canonical, useWildcardPermissive := alias, false
  3. parts := SplitN(canonical, "/", 2)
  4. if len(parts) != 2: return nil, ErrUnknownModel
  5. provider, model := parts[0], parts[1]
  6. meta, ok := registry.Get(provider)
     if !ok: return nil, ErrUnknownModel                         (REQ-CMDCTX-PA-006: hard fail)
  7. if model NOT in meta.SuggestedModels:
       effectiveMode := a.aliasResolveMode
       if useWildcardPermissive:
         effectiveMode = AliasResolveModePermissiveProvider     (REQ-CMDCTX-PA-011)
       switch normalize(effectiveMode):                          (normalize: invalid → Strict, REQ-CMDCTX-PA-009)
         case AliasResolveStrict:
           return nil, ErrUnknownModel
         case AliasResolveModePermissiveProvider, AliasResolveModePermissive:
           safeWarn(a.logger, "resolveAlias: model not in SuggestedModels but permissive mode allows",
                    "provider", provider, "model", model, "mode", modeName(effectiveMode))
                                                                 (REQ-CMDCTX-PA-005, REQ-CMDCTX-PA-010)
           // fall through to step 8
  8. return &ModelInfo{
       ID:          provider + "/" + model,
       DisplayName: meta.DisplayName + " " + model,
     }, nil
```

`normalize(mode)` helper:

```go
// normalizeMode returns the effective mode, falling back to Strict for any
// undefined integer value (REQ-CMDCTX-PA-009).
func normalizeMode(m AliasResolveMode) AliasResolveMode {
    switch m {
    case AliasResolveStrict, AliasResolveModePermissiveProvider, AliasResolveModePermissive:
        return m
    default:
        return AliasResolveStrict
    }
}
```

`safeWarn(...)` helper:

```go
// safeWarn invokes logger.Warn with a recover guard. Logging is best-effort
// and must not block the resolution path (REQ-CMDCTX-PA-010).
func safeWarn(logger Logger, msg string, fields ...any) {
    if logger == nil {
        return
    }
    defer func() {
        _ = recover() // swallow logger panics; resolution path proceeds
    }()
    logger.Warn(msg, fields...)
}

func modeName(m AliasResolveMode) string {
    switch m {
    case AliasResolveStrict:
        return "strict"
    case AliasResolveModePermissiveProvider:
        return "permissive_provider"
    case AliasResolveModePermissive:
        return "permissive"
    default:
        return "unknown"
    }
}
```

### 6.5 warn-log 메시지 형식 (godoc)

`logger.Warn` 호출 시:

- **메시지 본문**: `"resolveAlias: model not in SuggestedModels but permissive mode allows"`
- **fields**:
  - `provider` (string) — registry lookup 에 사용된 provider 이름.
  - `model` (string) — `meta.SuggestedModels` 에 없는 모델 식별자.
  - `mode` (string) — `modeName()` 의 결과 (`"permissive_provider"` 또는 `"permissive"`).

향후 SPEC-GOOSE-TELEMETRY-001 가 wired 되면 동일 path 에서 counter 증가 (REQ-CMDCTX-PA-012 의 hook). 본 SPEC 은 hook point 만 정의 — 메시지 emit 자체가 그 hook 이다.

### 6.6 AliasResolveMode 가 backward compat 인 이유

- Go enum 의 zero-value 가 `AliasResolveStrict` 이므로, 기존 `Options{...}` 리터럴 (필드 미지정) 은 자동으로 `AliasResolveMode: 0` = `AliasResolveStrict` 를 가진다.
- CMDCTX-001 v0.1.1 의 모든 19 AC 가 그대로 통과 (Strict 분기 = v0.1.1 동작 동일).
- 신규 호출자만 mode 를 명시적으로 설정.

→ `Options` struct 의 필드 추가는 Go semver 상 minor 변경이며, 기존 caller 의 컴파일 깨짐 없음 (struct literal 이 named fields 를 사용한다는 일반 관행 가정).

### 6.7 wildcard alias key 의 stub 정책 (Optional REQ-CMDCTX-PA-011)

본 SPEC 은 wildcard 매칭의 단순 stub 만 의무화한다:

- **Wired path**: `aliasMap.lookup(alias)` 가 정확히 `provider/*` 형태의 키를 갖는다면, prefix 매칭(strings.HasPrefix(alias, provider+"/")) 시 `useWildcardPermissive = true` 반환. canonical 은 입력 alias 그대로.
- **Stub path**: 미구현 시 wildcard 키는 literal 로 취급되어 매칭되지 않음. AC-CMDCTX-PA-008 의 두 가지 결과 중 하나 (단, 결정된 동작이 일관되어야 함).

→ 본 SPEC 의 implementation 은 wired path 를 권장한다. stub 도 backward compatible.

### 6.8 race / concurrency 안전성

- `AliasResolveMode` 는 `New(...)` 시점에 set 후 immutable (struct field, no setter). race-free 보장.
- `safeWarn` 의 recover 는 단일 호출 경로 내 동작 (다른 goroutine 의 panic 을 swallow 하지 않음).
- 기존 CMDCTX-001 의 race 안전성 (REQ-CMDCTX-005, AC-CMDCTX-014) 회귀 금지.

### 6.9 CMDCTX-001 v0.2.0 amendment 변경 요약 (run phase 시점)

본 SPEC 의 implementation 은 다음을 동시 수행:

1. `internal/command/adapter/` 코드 변경 (위 §6.1~§6.5).
2. `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` 본문 갱신:
   - frontmatter `version: 0.1.1` → `0.2.0`, `updated_at` 갱신.
   - HISTORY 표에 v0.2.0 항목 1줄 추가:
     `| 0.2.0 | (구현일) | permissive alias mode 도입 (SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 amendment). Options.AliasResolveMode 필드 + ResolveModelAlias step 7 분기 추가. backward compatible (zero-value = Strict). | manager-spec |`
   - §6.2 `Options` 시그니처에 `AliasResolveMode` 필드 추가.
   - §6.4 알고리즘에 step 7 분기 반영 (위 §6.4 의 형식).
   - §9 R2 완화 칸을 다음으로 갱신: `Optional REQ-CMDCTX-017 + AliasResolveMode (v0.2.0, SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001) 가 strict default 를 유지하면서 opt-in permissive 경로 제공.`
   - §Exclusions #7 의 TBD-SPEC-ID 자리에 본 SPEC ID 명시 + status `(planned, see this SPEC)`.

3. 기존 19 AC 회귀 검증.

CMDCTX-001 의 기존 REQ ID / AC ID 는 보존. 신규 AC 는 본 SPEC 에 거주.

### 6.10 TDD 진입 순서 (RED → GREEN → REFACTOR)

| 순서 | 작업 | 검증 AC |
|------|------|---------|
| T-001 | `AliasResolveMode` enum + 상수 정의 (compile-only) | 컴파일 |
| T-002 | `Options.AliasResolveMode` 필드 추가, `ContextAdapter` 에 저장 | 컴파일 |
| T-003 | `normalizeMode` helper + 단위 테스트 | AC-CMDCTX-PA-005 (invalid 값 → Strict) |
| T-004 | `ResolveModelAlias` step 7 분기 + Strict default 회귀 | AC-CMDCTX-PA-001, AC-CMDCTX-PA-007 (CMDCTX-001 19 AC 회귀) |
| T-005 | PermissiveProvider 모드 happy path | AC-CMDCTX-PA-002 |
| T-006 | warn-log emit + fakeWarnLogger | AC-CMDCTX-PA-003 |
| T-007 | Permissive 모드 + provider-unknown hard fail | AC-CMDCTX-PA-004 |
| T-008 | `safeWarn` recover guard + panickingLogger | AC-CMDCTX-PA-007 |
| T-009 | 비-mutation invariant 검증 | AC-CMDCTX-PA-006 |
| T-010 | wildcard alias map (stub or wired) | AC-CMDCTX-PA-008 |
| T-011 | CMDCTX-001 spec.md 본문 v0.1.1 → v0.2.0 amendment 적용 | spec governance |

### 6.11 TRUST 5 매핑

| 차원 | 본 SPEC 적용 |
|------|-----------|
| Tested | 8 AC, 기존 19 AC 회귀 검증. coverage 감소 금지 (≥ 90% 유지). race detector pass. |
| Readable | godoc on every exported identifier (`AliasResolveMode`, 3 상수, `Options.AliasResolveMode`). English code comments. |
| Unified | gofmt + golangci-lint clean. 기존 `Options` 의 Go style 따름. |
| Secured | recover guard 로 logger panic 이 resolution path 차단 못함. nil registry / nil logger graceful. |
| Trackable | conventional commit 본문에 SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 + REQ-CMDCTX-PA-NNN trailer. CMDCTX-001 v0.2.0 amendment 도 동일 PR 에 포함. @MX:NOTE 태그로 enum 변경 시 SPEC amendment 강제. |

### 6.12 의존성 결정 (라이브러리)

- `errors` (stdlib) — 기존 `command.ErrUnknownModel` 재사용.
- `strings` (stdlib) — wildcard prefix 매칭(Optional REQ).
- 신규 외부 의존성 없음.

---

## 7. 의존성 (Dependencies)

| 종류 | 대상 SPEC | 관계 |
|------|---------|------|
| amendment 대상 | SPEC-GOOSE-CMDCTX-001 (implemented, v0.1.1) | 본 SPEC 의 implementation 시점에 v0.2.0 으로 amendment. spec.md 본문 동시 갱신 (§3.1 #8, §6.9). |
| 재사용 | SPEC-GOOSE-COMMAND-001 (implemented, FROZEN) | `command.ErrUnknownModel` (`internal/command/errors.go:23-25`). 변경 없음. |
| 재사용 | SPEC-GOOSE-ROUTER-001 (implemented, FROZEN) | `*router.ProviderRegistry`, `ProviderMeta.SuggestedModels` read-only. 변경 없음. |
| 재사용 | SPEC-GOOSE-CONTEXT-001 (implemented, FROZEN) | 간접 — `ContextAdapter` 가 LoopController 경유 사용. 본 SPEC 은 LoopController API 변경 없음. |
| 재사용 | SPEC-GOOSE-SUBAGENT-001 (implemented, FROZEN) | 간접 — `PlanModeActive` 경로. 본 SPEC 은 변경 없음. |
| 후속 의존자 | SPEC-GOOSE-TELEMETRY-001 (TBD) | REQ-CMDCTX-PA-012 의 hook point 를 metrics emission 으로 wire. |
| 후속 의존자 | SPEC-GOOSE-HOTRELOAD-001 (TBD) | aliasMap / AliasResolveMode 의 런타임 갱신. 본 SPEC scope 외. |

본 SPEC 은 의존 SPEC 들을 다음과 같이 다룬다:

- **CMDCTX-001 (implemented, v0.1.1)**: 본 SPEC 의 구현은 CMDCTX-001 의 v0.2.0 amendment 와 동시에 발생한다. CMDCTX-001 v0.1.1 의 19 AC 는 모두 보존되며, 신규 8 AC 는 본 SPEC 에 거주. CMDCTX-001 의 frontmatter status 는 implemented 유지 (amendment 가 implemented status 를 invalidate 하지 않음 — HISTORY 항목으로 추적성 보장).
- **나머지 SPEC들**: read-only 의존. 변경 없음.

---

## 8. Acceptance Test 전략

### 8.1 표 기반 (table-driven) 단위 테스트

```go
func TestContextAdapter_ResolveModelAlias_PermissiveModes(t *testing.T) {
    cases := []struct {
        name        string
        mode        AliasResolveMode
        registry    *router.ProviderRegistry
        aliasMap    map[string]string
        input       string
        wantID      string
        wantErr     error
        wantWarnHit bool
    }{
        {"strict_default_unknown_model", AliasResolveStrict, fakeRegistryWithProvider("anthropic", []string{"claude-opus-4-7"}), nil, "anthropic/typo-model", "", command.ErrUnknownModel, false},
        {"permissive_provider_unknown_model_allowed", AliasResolveModePermissiveProvider, fakeRegistryWithProvider("openrouter", []string{}), nil, "openrouter/deepseek/deepseek-r1:free", "openrouter/deepseek/deepseek-r1:free", nil, true},
        {"permissive_provider_unknown_provider_still_fails", AliasResolveModePermissiveProvider, fakeRegistryWithProvider("openrouter", []string{}), nil, "nonexistent/foo", "", command.ErrUnknownModel, false},
        {"permissive_unknown_provider_hard_fail", AliasResolveModePermissive, fakeRegistryWithProvider("openrouter", []string{}), nil, "nonexistent/foo", "", command.ErrUnknownModel, false},
        {"invalid_mode_falls_back_to_strict", AliasResolveMode(99), fakeRegistryWithProvider("anthropic", []string{"claude-opus-4-7"}), nil, "anthropic/typo-model", "", command.ErrUnknownModel, false},
        {"strict_with_suggested_model_happy", AliasResolveStrict, fakeRegistryWithProvider("anthropic", []string{"claude-opus-4-7"}), nil, "anthropic/claude-opus-4-7", "anthropic/claude-opus-4-7", nil, false},
    }
    for _, tc := range cases { /* ... */ }
}
```

### 8.2 fake `Logger` (warn-log 검증)

```go
type fakeWarnLogger struct {
    mu        sync.Mutex
    calls     []warnCall
}

type warnCall struct {
    msg    string
    fields []any
}

func (f *fakeWarnLogger) Warn(msg string, fields ...any) { /* capture */ }
func (f *fakeWarnLogger) Count() int                     { /* read */ }

type panickingLogger struct{}

func (p *panickingLogger) Warn(msg string, fields ...any) {
    panic("logger boom")
}
```

### 8.3 비-mutation invariant 검증 (AC-CMDCTX-PA-006)

```go
func TestContextAdapter_ResolveModelAlias_NoMutation(t *testing.T) {
    reg := fakeRegistryWithProvider("openrouter", []string{"sugg-1"})
    aliasMap := map[string]string{"opus": "anthropic/claude-opus-4-7"}
    snapshot := deepCopy(reg, aliasMap)

    a := New(Options{Registry: reg, AliasMap: aliasMap, AliasResolveMode: AliasResolveModePermissiveProvider, LoopController: fakeLoop()})
    for i := 0; i < 100; i++ {
        _, _ = a.ResolveModelAlias(randomInput())
    }

    if !equal(snapshot, deepCopy(reg, aliasMap)) {
        t.Fatal("registry/aliasMap mutated")
    }
}
```

### 8.4 race detector

```bash
go test -race -count=10 ./internal/command/adapter/...
```

CMDCTX-001 의 AC-CMDCTX-014 (100 goroutine × 1000 iter) 회귀 포함. 본 SPEC 은 immutable enum 만 추가하므로 신규 race risk 없음.

### 8.5 회귀 검증 (Backward compat)

기존 CMDCTX-001 의 19 AC (AC-CMDCTX-001~019) 가 모두 통과해야 한다. 특히:

- AC-CMDCTX-002, AC-CMDCTX-003, AC-CMDCTX-011 (ResolveModelAlias 의 strict 동작) — `Options{AliasResolveMode: 미설정}` 으로 호출 시 동일 결과.
- AC-CMDCTX-014 (race) — 신규 enum 도 race-free.

### 8.6 Coverage / Lint

- 라인 커버리지: ≥ 90% 유지 (CMDCTX-001 의 100% 에서 신규 분기 추가로 임시 감소 가능 → 신규 AC 로 회복).
- branch 커버리지: ≥ 85%.
- `gofmt -l . | grep . && exit 1` — clean.
- `golangci-lint run ./internal/command/adapter/...` — 0 issues.
- godoc on `AliasResolveMode`, 3 상수, `Options.AliasResolveMode`, helper functions.

---

## 9. 리스크 & 완화 (Risks & Mitigations)

| 리스크 | 영향 | 완화 |
|--------|----|------|
| R1 — typo 로 인한 잘못된 모델 호출 (permissive 모드에서 silent allow) | 중 | warn-log 의무화 (REQ-CMDCTX-PA-005) + 후속 SPEC-GOOSE-TELEMETRY-001 의 counter 연계 (REQ-CMDCTX-PA-012 hook 점) + provider HTTP 4xx 응답이 SPEC-GOOSE-ERROR-CLASS-001 의 표준 에러로 fallback 되어 user-facing 메시지 표준화. |
| R2 — CMDCTX-001 SPEC 본문 amendment 가 governance 깨짐 (FROZEN status 의 amendment 정책) | 중 | 본 SPEC 의 §3.1 #8 / §6.9 / §7 가 amendment 범위를 명시. CMDCTX-001 의 19 AC 보존 + HISTORY 항목 1줄 + frontmatter version 0.1.1 → 0.2.0 으로 추적성 보장. status 는 implemented 유지 (amendment 가 implementation 을 invalidate 하지 않음). |
| R3 — mode 폭주 (Strict / PermissiveProvider / Permissive 후 추가 mode 무한 추가) | 저 | REQ-CMDCTX-PA-002 가 "신규 mode 추가는 별도 SPEC amendment 필요" 명시. enum 정의 godoc 에 동일 문구 (`@MX:NOTE`). |
| R4 — Permissive 와 PermissiveProvider 의 의미론 차이가 모호 → 사용자 혼란 | 저 | 본 SPEC §6.4 알고리즘과 godoc 이 step 별 차이 명시. Permissive 는 향후 확장 여지로 reserve, 현재 step 7 동작은 PermissiveProvider 와 동일. AC-CMDCTX-PA-004 가 provider-unknown 경계 검증. |
| R5 — wildcard alias key (REQ-CMDCTX-PA-011) 의 stub vs wired 결정 모호 | 저 | AC-CMDCTX-PA-008 가 양 결과 모두 허용 + "결정된 동작 일관성" 검증. implementation 시점에 wired 를 권장하되 stub 도 backward compatible. |
| R6 — recover guard (REQ-CMDCTX-PA-010) 가 panic 을 silently swallow → 디버깅 어려움 | 저 | recover 는 logger.Warn 호출 site 에만 한정 적용 (resolution path 외 영역의 panic 은 영향 없음). recover() 결과는 향후 telemetry counter 로 통계화 가능 (TELEMETRY-001 hook). |

---

## 10. 참고 (References)

### 10.1 프로젝트 문서

- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` v0.1.1 — 부모 SPEC. amendment 대상.
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` — `command.ErrUnknownModel` 정의 위치.
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` — ProviderMeta / SuggestedModels.
- `.moai/specs/SPEC-GOOSE-ERROR-CLASS-001/spec.md` — provider HTTP 4xx 표준 에러 분류.
- `internal/command/adapter/adapter.go` — 변경 surface.
- `internal/command/adapter/alias.go` — ResolveModelAlias 본체.
- `internal/command/errors.go:23-25` — `ErrUnknownModel` sentinel.

### 10.2 부속 문서

- `research.md` (본 디렉토리) — 결정 옵션 비교, OpenRouter 사례 분석.
- `progress.md` (본 디렉토리) — phase log.

---

## Exclusions (What NOT to Build)

본 SPEC 이 **명시적으로 제외**하는 항목 (어느 후속 SPEC 이 채워야 하는지 명시):

1. **Telemetry counter** (`cmdctx_permissive_unsuggested_model_total`) — REQ-CMDCTX-PA-012 의 hook point 만 정의. 실제 metrics emission 은 SPEC-GOOSE-TELEMETRY-001 (TBD).
2. **Hot-reload** of `aliasMap` / `AliasResolveMode` — `New(...)` 시점 immutable 유지. SPEC-GOOSE-HOTRELOAD-001 (TBD).
3. **`ProviderMeta.AllowUnsuggestedModels` per-provider flag** — research.md §3.2 옵션 B. ROUTER-001 (FROZEN) 변경 필요. 별도 SPEC 으로 분리 가능 (TBD-SPEC-ID).
4. **다중 wildcard alias key** (`provider/sub/*` 등) — REQ-CMDCTX-PA-011 의 단순 `provider/*` 만 지원. 별도 SPEC 으로 확장 가능.
5. **Provider HTTP 4xx 응답의 user-facing 메시지 표준화** — SPEC-GOOSE-ERROR-CLASS-001 의 책임.
6. **mode 4 종 이상 추가** (`Experimental`, `DevPreview` 등) — 별도 SPEC amendment 필요. 본 SPEC 은 3 종에 한정 (REQ-CMDCTX-PA-002).
7. **Permissive 모드 의 provider-unknown 분기 변경** — 본 SPEC 은 step 6 의 hard fail 을 모든 모드에서 유지 (REQ-CMDCTX-PA-006, REQ-CMDCTX-PA-008). 향후 변경은 별도 SPEC.
8. **CMDCTX-001 §6.4 외 다른 알고리즘 영역의 변경** — 본 SPEC 은 step 7 분기에만 영향. 다른 step 의 변경은 별도 SPEC.

---

Version: 0.1.0
Status: planned
Last Updated: 2026-04-27
