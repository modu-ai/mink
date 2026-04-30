---
id: SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001
version: 0.1.0
status: planned
created_at: 2026-04-30
updated_at: 2026-04-30
author: manager-spec
priority: P3
issue_number: null
phase: 2
size: 소(S)
lifecycle: spec-anchored
labels: ["area/config", "type/refactor", "amendment", "hot-reload-prep"]
parent_spec: SPEC-GOOSE-ALIAS-CONFIG-001
parent_version: 0.1.0
---

# SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 — Loader Hot-Reload Prep & Schema Extension Amendment

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-30 | 초안 작성. 부모 SPEC-GOOSE-ALIAS-CONFIG-001 v0.1.0 (completed, FROZEN) 의 (1) progress.md Open Question #2 (lenient in-place 삭제 의미론) 해소, (2) §4.5 REQ-ALIAS-040 (user+project merge) 본문 미구현 결손 보강, (3) 후속 SPEC-GOOSE-CMDCTX-HOTRELOAD-001 (planned) 의 watcher path 자동 동기화 / reload 의도 명시화 / metrics hook 사전 마련, (4) v0.2 candidate 영역 (AliasEntry schema 확장, ErrorCode 안정 키) 일부를 0.1 amendment 로 흡수. 부모 export 표면 (`Loader`, `Options`, `New`, `Load`, `LoadDefault`, `Validate`, `AliasConfig`, `ErrConfigNotFound`) 변경 0건 — 신규 type/method/함수만 추가. | manager-spec |

---

## 1. 개요 (Overview)

본 amendment 는 `SPEC-GOOSE-ALIAS-CONFIG-001` v0.1.0 (status=completed, FROZEN, completed_at=2026-04-27) 의 `internal/command/adapter/aliasconfig/` 패키지에 다음 결손을 채운다:

1. **Hot-reload 호환 표면 부재**: 후속 `SPEC-GOOSE-CMDCTX-HOTRELOAD-001` (planned) 의 fsnotify watcher 가 `Loader.configPath` (unexported) 를 알 방법이 없어 path 결정 로직을 외부에 재구현해야 함. 또한 reload 의도를 명시하는 API 부재.
2. **Multi-source merge 본문 미구현**: 부모 spec.md §4.5 REQ-ALIAS-040 은 "user file + project file 동시 read, project override + override info-log" 명세. 실제 구현 (`loader.go:62-78`) 은 OR 분기로 한 path 만 선택. SPEC 본문 위반.
3. **Lenient 모드 in-place 삭제 의미 미명시 (Open Question #2)**: 부모 progress.md §Open questions 의 미해결 항목. 현재 caller 가 errs 보고 수동 삭제 필요.
4. **Schema 확장 여지**: alias 별 deprecated / replacedBy / contextWindow 메타 부재. 후속 마이그레이션 시점에 미리 마련하지 않으면 schema break.
5. **Observability hook 부재**: `SPEC-GOOSE-CMDCTX-HOTRELOAD-001` §10 #10 (telemetry 위임) 와 협력 가능한 metrics emit 부재.
6. **Error 안정 분류 부재**: i18n / 호출자 분기 단순화를 위한 ErrorCode + Categorize 함수 부재.

본 amendment 의 모든 변경은 **backward compatible** — 부모 v0.1.0 export 표면 (시그니처) 변경 없음. 신규 type / method / 함수만 추가.

---

## 2. 배경 (Background)

### 2.1 부모 SPEC 상태

- `SPEC-GOOSE-ALIAS-CONFIG-001` v0.1.0: status=completed, FROZEN, completed_at=2026-04-27
- 구현 위치: `internal/command/adapter/aliasconfig/{loader.go, loader_test.go, loader_p3_test.go, integration_test.go, validate_test.go}` (~700 LOC)
- 미해결 progress.md Open Question: #2 (lenient in-place 삭제 의미론)

### 2.2 후속 의존 SPEC

- `SPEC-GOOSE-CMDCTX-HOTRELOAD-001` (planned, P4): fsnotify watcher 가 본 패키지의 `LoadDefault()` / `Validate(m, reg, strict)` 호출. 본 amendment 가 이 두 시그니처를 보존해야 watcher 구현 영향 0.

### 2.3 본 amendment 가 변경하지 않는 항목 (HARD)

- `Loader` struct 의 기존 필드 5개 (`configPath`, `fsys`, `logger`, `stdLogger`)
- `Options` struct 의 기존 필드 4개 (`ConfigPath`, `FS`, `Logger`, `StdLogger`)
- `Logger` interface 시그니처
- `AliasConfig` struct (`Aliases map[string]string` yaml:"aliases")
- `New(opts Options) *Loader`
- `(*Loader).Load() (map[string]string, error)`
- `(*Loader).LoadDefault() (map[string]string, error)`
- `Validate(m, registry, strict bool) []error`
- `ErrConfigNotFound` sentinel
- yaml schema 의 flat `aliases: {alias: "provider/model"}` 형식 지원
- 기존 5개 테스트 파일의 모든 test (회귀 금지)

---

## 3. 용어 (Terminology)

| 용어 | 정의 |
|-----|------|
| amendment | 부모 SPEC v0.1.0 의 FROZEN export 표면을 보존하면서 신규 표면만 추가하는 backward-compat 변경 단위. |
| effective config path | `New(opts)` 가 fallback chain 적용 후 결정한 최종 절대 경로 (현재 unexported `Loader.configPath`). |
| user file | `$GOOSE_HOME/aliases.yaml` 또는 `$HOME/.goose/aliases.yaml`. |
| project file | `$CWD/.goose/aliases.yaml` (project-local overlay). |
| merge policy | user file + project file 동시 존재 시 합성 정책. ProjectOverride / UserOnly / ProjectOnly 3종. |
| validation policy | `Validate` 호출 시 strict / SkipOnError / MaxErrors / AggregateAsJoin 4축 결정. |
| alias entry (확장 schema) | `provider/model` canonical + 부가 메타 (deprecated, replacedBy, contextWindow, providerHints). |
| error code | sentinel error 의 안정적 식별자 (e.g., `ALIAS-001`) — i18n 매핑 키 / 로그 분류 키. |
| metrics hook | `Options.Metrics` interface — 외부 observer 가 reload count / validation error / load duration / entry count 관찰. |

---

## 4. 요구사항 (Requirements, EARS)

본 amendment 는 **12 REQ** (Ubiquitous 4, Event-Driven 2, State-Driven 3, Unwanted 2, Optional 1) 정의.

### 4.1 Ubiquitous Requirements

#### REQ-AMEND-001 — ConfigPath getter

The `aliasconfig.Loader` **shall** expose `(l *Loader) ConfigPath() string` returning the effective config path resolved at `New(...)` time, including any fallback applied (project-local / GOOSE_HOME / HOME chain).

근거: 후속 HOTRELOAD-001 watcher 가 fsnotify 대상 path 결정 로직을 외부 재구현 없이 가져오기 위함. 부모 surface 0 변경.

#### REQ-AMEND-002 — Explicit Reload method

The `aliasconfig.Loader` **shall** expose `(l *Loader) Reload() (map[string]string, error)` with semantics identical to `Load()` (fresh disk read of the same configPath, parse, return). The method **shall** be a separate exported symbol (not a rename) so that the call-site intent (re-read after change) is grep-able and doc-aligned with HOTRELOAD-001's reload chain.

근거: `Load()` 보존하면서 의도 명시화. HOTRELOAD-001 §6.7 의 `Loader` interface 와 직교.

#### REQ-AMEND-003 — ValidateAndPrune in-place semantics

The `aliasconfig` package **shall** expose `ValidateAndPrune(m map[string]string, registry *router.ProviderRegistry, policy ValidationPolicy) (cleaned map[string]string, errs []error)` that returns a NEW map containing only entries that passed validation, alongside the slice of per-entry errors. The input map `m` **shall not** be mutated.

근거: 부모 progress.md Open Question #2 명시적 해소. 신규 함수 — 기존 `Validate(m, reg, strict)` 시그니처 보존.

#### REQ-AMEND-004 — ErrorCode stable identifier

The `aliasconfig` package **shall** expose `ErrorCode(err error) string` returning a stable identifier from a closed enumeration when `err` wraps a known sentinel; returns empty string when `err` is nil or unrecognized.

근거: i18n 매핑 / 호출자 분기 / 로그 grouping 안정 키 제공. 신규 함수.

### 4.2 Event-Driven Requirements

#### REQ-AMEND-010 — Multi-source merge on Load

**When** `(l *Loader).LoadDefault()` is invoked AND both the user file (`$GOOSE_HOME/aliases.yaml` 또는 `$HOME/.goose/aliases.yaml`) and the project file (`$CWD/.goose/aliases.yaml`) exist AND `Options.MergePolicy` is unset OR equals `MergePolicyProjectOverride`, the system **shall** parse both files, build the user map first, then overlay the project map's entries, and **shall** emit one info-level log entry per overridden alias key (text contains both source paths and the alias key).

근거: 부모 SPEC §4.5 REQ-ALIAS-040 본문 구현. 현재 OR 분기 미준수 결손 보강.

#### REQ-AMEND-011 — Categorize on error

**When** `Categorize(err error) ErrorCategory` is called with a non-nil error, the system **shall** return one of `{CategoryLoad, CategoryParse, CategoryPermission, CategorySize, CategoryFormat, CategoryRegistry, CategoryUnknown}` based on the wrapped sentinel chain.

근거: 호출자가 type assertion 없이 분기 가능. HOTRELOAD-001 §6.9 의 watcher 로그 분류 단순화.

### 4.3 State-Driven Requirements

#### REQ-AMEND-020 — MergePolicy honored

**While** `Options.MergePolicy` equals `MergePolicyUserOnly`, `LoadDefault` **shall** ignore the project file even if present. **While** equal to `MergePolicyProjectOnly`, **shall** ignore the user file. **While** unset OR equals `MergePolicyProjectOverride`, REQ-AMEND-010 applies.

근거: backward-compat migration 안전판. 사용자가 v0.1.0 의 OR 분기 동작을 명시적으로 옵트인 가능.

#### REQ-AMEND-021 — Metrics emission when configured

**While** `Options.Metrics` is non-nil, every successful `Load`/`Reload`/`LoadDefault` call **shall** invoke `Metrics.IncLoadCount(true)` and `Metrics.RecordLoadDuration(d)` and `Metrics.ObserveEntryCount(len(result))`. Every failed call **shall** invoke `Metrics.IncLoadCount(false)`. Every error in `ValidateAndPrune` **shall** invoke `Metrics.IncValidationError(ErrorCode(err))`.

근거: 후속 telemetry SPEC 통합 비용 ↓. noop default 보장 (REQ-AMEND-022).

#### REQ-AMEND-022 — Metrics noop default

**While** `Options.Metrics` is nil OR not provided, the system **shall** use an internal noop implementation that satisfies the `Metrics` interface but performs no I/O and adds no allocation in steady-state.

근거: backward-compat 보장 — 기존 호출자가 Metrics 무시.

### 4.4 Unwanted Behaviour

#### REQ-AMEND-030 — Reload preserves on parse error

**If** `(l *Loader).Reload()` encounters a malformed YAML or oversize file, **then** the method **shall** return the wrapped sentinel error AND **shall not** affect any in-memory state of the Loader (Loader is stateless — this REQ is asserting that no hidden caching mutates).

근거: HOTRELOAD-001 §REQ-HOTRELOAD-033 와 정합 — reload 실패 시 호출자가 기존 map 보존 가능.

### 4.5 Optional Features

#### REQ-AMEND-031 — LoadEntries backward graceful (재분류 — plan-audit 2026-04-30: Unwanted → Event-Driven)

**When** `LoadEntries() (map[string]AliasEntry, error)` is invoked with a YAML file in legacy flat form (`aliases: {alias: "provider/model"}`), the method **shall** transparently lift each value into `AliasEntry{Canonical: "provider/model"}` with all extension fields zero-valued, returning the same logical content as the legacy form.

근거: schema 확장이 backward-compat 보장. 기존 yaml 파일 그대로 동작.

(메모: 초기 작성 시 Unwanted 카테고리에 분류되었으나, "legacy form 감지 시 graceful lift"는 negative 패턴이 아닌 Event-Driven 정상 분기 동작이므로 §4.5 Optional 또는 §4.2 Event-Driven 으로 재분류. plan-audit 2026-04-30 권장에 따라 §4.5 Optional 직속으로 이동 — Optional 패턴이 "권장 동작" 의미를 가장 잘 표현.)


#### REQ-AMEND-040 — Extended schema with metadata

**Where** an alias entry in the YAML uses the extended map form (`opus: {canonical: "anthropic/claude-opus-4-7", deprecated: true, replacedBy: "anthropic/claude-opus-4-9", contextWindow: 200000}`), `LoadEntries` **shall** preserve those fields in the returned `AliasEntry` value. `Load()` (legacy) **shall** continue to return only the `canonical` value as the map's string value (silently dropping extension fields).

근거: schema 확장 — Optional 로 두어 v0.1 amendment 에서 점진 도입.

---

## 5. Acceptance Criteria

본 amendment 는 **14 AC** 정의. 모든 non-Optional REQ 가 최소 1개 AC 로 검증.

| AC ID | 검증 대상 REQ | Given-When-Then |
|-------|--------------|-----------------|
| **AC-AMEND-001** | REQ-AMEND-001 | **Given** `loader := New(Options{})` (fallback resolution applied) **When** `loader.ConfigPath()` 호출 **Then** 반환 값이 absolute path, `loader.configPath` (unexported) 와 정확히 일치, empty string 아님 |
| **AC-AMEND-002** | REQ-AMEND-002 | **Given** valid aliases.yaml 파일 + Loader **When** `loader.Load()` 호출 후 파일 내용을 `os.WriteFile` 로 변경하고 `loader.Reload()` 호출 **Then** Reload 결과가 변경된 내용 반영, Load 결과와 다름. 두 method 가 동일 configPath 사용 |
| **AC-AMEND-003** | REQ-AMEND-003 | **Given** map `{"good": "openai/gpt-4o", "bad": "invalid", "empty": ""}`, registry, strict policy **When** `cleaned, errs := ValidateAndPrune(m, reg, ValidationPolicy{Strict: true})` **Then** cleaned 는 `{"good": "openai/gpt-4o"}` 만 포함 (1 entry), errs 는 2건, 입력 m 은 mutate 되지 않음 (`len(m) == 3`) |
| **AC-AMEND-004** | REQ-AMEND-004 | **Given** sentinel error 4종 (`ErrConfigNotFound`, `command.ErrEmptyAliasEntry`, `command.ErrInvalidCanonical`, `command.ErrAliasFileTooLarge`) wrapped in `fmt.Errorf("%w: ...", sentinel)` **When** `ErrorCode(wrapped)` 호출 **Then** 각각 stable code 반환 (예: `ALIAS-001`, `ALIAS-010`, `ALIAS-011`, `ALIAS-020`), nil error 입력 시 empty string |
| **AC-AMEND-010-A** | REQ-AMEND-010 | **Given** user file `{opus: anthropic/claude-opus-4-7, sonnet: anthropic/claude-sonnet-4-6}` + project file `{opus: anthropic/claude-opus-4-9}` + MergePolicy unset **When** `loader.LoadDefault()` 호출 **Then** 결과 map 이 `{opus: anthropic/claude-opus-4-9, sonnet: anthropic/claude-sonnet-4-6}` (project override + user retain) |
| **AC-AMEND-010-B** | REQ-AMEND-010 | AC-AMEND-010-A 시나리오에서 zaptest observer 사용 **When** LoadDefault 완료 **Then** info-level 로그 1건 발생, 메시지에 alias key `opus`, user file path, project file path 모두 포함 |
| **AC-AMEND-011** | REQ-AMEND-011 | **Given** sentinel error 6종 (Load / Parse / Permission / Size / Format / Registry) **When** 각각 `Categorize(err)` 호출 **Then** 정확히 6개 카테고리 enum 매핑. 비정형 fmt.Errorf 입력 시 `CategoryUnknown` 반환 |
| **AC-AMEND-020** | REQ-AMEND-020 | **Given** user + project 둘 다 존재, 3가지 MergePolicy 각각 **When** `LoadDefault()` 호출 **Then** UserOnly → user 만, ProjectOnly → project 만, ProjectOverride → 합성 결과 (REQ-AMEND-010 와 동일) |
| **AC-AMEND-021** | REQ-AMEND-021 | **Given** Options.Metrics = fakeMetrics (call counter spy), valid aliases.yaml **When** `loader.LoadDefault()` 1회 호출 **Then** `IncLoadCount(true)` 1회, `RecordLoadDuration(d)` 1회 (d > 0), `ObserveEntryCount(N)` 1회 (N == 결과 map size) |
| **AC-AMEND-022** | REQ-AMEND-022 | **Given** Options.Metrics = nil (default) **When** loader.LoadDefault() 호출 **Then** panic 없음, return 정상. allocation profile (`testing.AllocsPerRun`) 비교 시 noop 경로 추가 alloc 0 (즉, runtime overhead 없음) |
| **AC-AMEND-030** | REQ-AMEND-030 | **Given** valid aliases.yaml → `loader.Load()` 1회 → 파일을 malformed YAML 로 덮어씀 **When** `loader.Reload()` 호출 **Then** 에러 반환 (`errors.Is(err, command.ErrMalformedAliasFile)` true), Loader 인스턴스의 동작 (configPath / 후속 Load() 동작) 변화 없음 |
| **AC-AMEND-031** | REQ-AMEND-031 | **Given** legacy yaml `aliases: {opus: anthropic/claude-opus-4-7}` (flat string) **When** `loader.LoadEntries()` 호출 **Then** 결과 `map[string]AliasEntry` 가 `{"opus": {Canonical: "anthropic/claude-opus-4-7", Deprecated: false, ReplacedBy: "", ContextWindow: 0, ProviderHints: nil}}` |
| **AC-AMEND-040** | REQ-AMEND-040 | **Given** extended yaml `aliases: {opus: {canonical: "anthropic/claude-opus-4-9", deprecated: true, replacedBy: "anthropic/claude-opus-5-0", contextWindow: 200000}}` **When** `loader.LoadEntries()` 호출 + `loader.Load()` 호출 **Then** LoadEntries 는 모든 메타 필드 보존, Load 는 legacy 형식 (`{"opus": "anthropic/claude-opus-4-9"}`) 반환 (canonical 만 추출) |
| **AC-AMEND-050** | 부모 surface 보존 | **When** `git diff` 가 부모 SPEC 의 export 시그니처 변화를 검출 **Then** 변경 0건. 정적 검증: `go doc ./internal/command/adapter/aliasconfig` 출력에서 v0.1.0 시점 export 와 비교, 시그니처 동일 |
| **AC-AMEND-051** | 부모 회귀 금지 | **When** 본 amendment 머지 후 `go test ./internal/command/adapter/aliasconfig/... -count=10` 실행 **Then** 부모 v0.1.0 의 5개 테스트 파일의 모든 test PASS (0 fail) |
| **AC-AMEND-052** | HOTRELOAD-001 정합 | **When** HOTRELOAD-001 의 `Loader` / `Validator` interface 시그니처와 본 amendment 후 `aliasconfig` 의 `LoadDefault` / `Validate` 시그니처 비교 **Then** 100% 호환 (interface satisfaction 정적 검증) |

**커버리지 매트릭스**:

| REQ | AC들 |
|-----|------|
| REQ-AMEND-001 | AC-AMEND-001 |
| REQ-AMEND-002 | AC-AMEND-002 |
| REQ-AMEND-003 | AC-AMEND-003 |
| REQ-AMEND-004 | AC-AMEND-004 |
| REQ-AMEND-010 | AC-AMEND-010-A, AC-AMEND-010-B |
| REQ-AMEND-011 | AC-AMEND-011 |
| REQ-AMEND-020 | AC-AMEND-020 |
| REQ-AMEND-021 | AC-AMEND-021 |
| REQ-AMEND-022 | AC-AMEND-022 |
| REQ-AMEND-030 | AC-AMEND-030 |
| REQ-AMEND-031 | AC-AMEND-031 |
| REQ-AMEND-040 | AC-AMEND-040 |
| (governance) | AC-AMEND-050, AC-AMEND-051, AC-AMEND-052 |

---

## 6. 데이터 모델 / 신규 API surface

### 6.1 신규 type / 함수 (added only — 부모 시그니처 보존)

```go
// Package aliasconfig (amendment AFTER, v0.1.0 export 보존 + 신규 추가)

// ============================================================================
// Area 1 — Hot-reload 호환 API
// ============================================================================

// ConfigPath returns the effective config path resolved at New(...) time.
// Includes fallback resolution result (project-local / GOOSE_HOME / HOME).
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-001
func (l *Loader) ConfigPath() string { return l.configPath }

// Reload re-reads the alias file from disk with semantics identical to Load().
// Exported as a separate symbol to make hot-reload call sites grep-able.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-002
func (l *Loader) Reload() (map[string]string, error) { return l.Load() }

// ============================================================================
// Area 2 — Multi-source merge
// ============================================================================

// MergePolicy controls user-file vs project-file overlay behavior.
type MergePolicy int

const (
    // MergePolicyProjectOverride: parse both, project entries override on conflict.
    // Default when Options.MergePolicy is zero-value.
    MergePolicyProjectOverride MergePolicy = 0
    // MergePolicyUserOnly: parse only the user file, ignore project file.
    MergePolicyUserOnly MergePolicy = 1
    // MergePolicyProjectOnly: parse only the project file (when present), ignore user file.
    MergePolicyProjectOnly MergePolicy = 2
)

// Options is extended with new fields. Existing fields preserved.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001
type Options struct {
    // ... existing fields preserved (ConfigPath, FS, Logger, StdLogger) ...

    // MergePolicy controls user/project overlay. Zero-value = ProjectOverride.
    MergePolicy MergePolicy

    // Metrics receives observability events. Zero-value (nil) = noop.
    Metrics Metrics
}

// ============================================================================
// Area 3 — ValidationPolicy
// ============================================================================

// ValidationPolicy controls how Validate / ValidateAndPrune behave.
type ValidationPolicy struct {
    // Strict enables provider/model existence checks (requires non-nil registry).
    Strict bool
    // SkipOnError continues processing remaining entries after each error.
    // Always true in v0.1 — reserved for future halt-on-first-error mode.
    SkipOnError bool
    // MaxErrors caps the returned error slice. 0 = unlimited.
    MaxErrors int
    // AggregateAsJoin returns errors.Join(...) of all errors as the first slice
    // element (slice length 1) when true; default false returns per-entry errors.
    AggregateAsJoin bool
}

// ValidateAndPrune validates each entry against the registry and returns a new
// map containing only entries that passed validation. Input map m is not mutated.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-003
func ValidateAndPrune(
    m map[string]string,
    registry *router.ProviderRegistry,
    policy ValidationPolicy,
) (cleaned map[string]string, errs []error) { /* ... */ }

// ============================================================================
// Area 4 — Error stable codes & categorization
// ============================================================================

// ErrorCategory enumerates the high-level classification of aliasconfig errors.
type ErrorCategory int

const (
    CategoryUnknown    ErrorCategory = 0
    CategoryLoad       ErrorCategory = 1 // file read / not found
    CategoryParse      ErrorCategory = 2 // YAML malformed
    CategoryPermission ErrorCategory = 3 // OS permission
    CategorySize       ErrorCategory = 4 // file > 1 MiB
    CategoryFormat     ErrorCategory = 5 // empty entry / invalid canonical
    CategoryRegistry   ErrorCategory = 6 // unknown provider / model in strict mode
)

// ErrorCode returns a stable identifier (e.g., "ALIAS-001") for a wrapped sentinel.
// Returns empty string for nil or unrecognized errors.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-004
func ErrorCode(err error) string { /* ... */ }

// Categorize maps a wrapped sentinel chain to a high-level ErrorCategory.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-011
func Categorize(err error) ErrorCategory { /* ... */ }

// ============================================================================
// Area 5 — Extended schema (AliasEntry)
// ============================================================================

// AliasEntry is the extended form of an alias value. Backward-compat: legacy
// flat string YAML form lifts to AliasEntry{Canonical: "provider/model"}.
type AliasEntry struct {
    Canonical     string            `yaml:"canonical"`
    Deprecated    bool              `yaml:"deprecated,omitempty"`
    ReplacedBy    string            `yaml:"replacedBy,omitempty"`
    ContextWindow int               `yaml:"contextWindow,omitempty"`
    ProviderHints map[string]string `yaml:"providerHints,omitempty"`
}

// LoadEntries loads the alias file and returns the extended-schema map. Legacy
// flat string entries are lifted to AliasEntry{Canonical: ...} transparently.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-031, REQ-AMEND-040
func (l *Loader) LoadEntries() (map[string]AliasEntry, error) { /* ... */ }

// ============================================================================
// Area 6 — Metrics interface
// ============================================================================

// Metrics is the observability hook for aliasconfig. Zero-value Options.Metrics
// (nil) uses an internal noop implementation — no allocation in steady state.
type Metrics interface {
    IncLoadCount(success bool)
    IncValidationError(code string)
    RecordLoadDuration(d time.Duration)
    ObserveEntryCount(n int)
}
```

### 6.2 신규 sentinel error (선택적 — codes only via `ErrorCode`)

본 amendment 는 **신규 sentinel error 추가 없음**. 기존 sentinel 에 `ErrorCode` 함수가 stable string code 매핑만 부여 (호출자 분기 / i18n 키 / 로그 분류 용도).

매핑 예시:
- `ErrConfigNotFound` → `"ALIAS-001"`
- `command.ErrEmptyAliasEntry` → `"ALIAS-010"`
- `command.ErrInvalidCanonical` → `"ALIAS-011"`
- `command.ErrAliasFileTooLarge` → `"ALIAS-020"`
- `command.ErrMalformedAliasFile` → `"ALIAS-021"`
- `command.ErrUnknownProviderInAlias` → `"ALIAS-030"`
- `command.ErrUnknownModelInAlias` → `"ALIAS-031"`
- 기타 → `""`

### 6.3 yaml schema (확장 — backward compat)

```yaml
# Legacy flat (v0.1.0, 보존):
aliases:
  opus:   anthropic/claude-opus-4-7
  sonnet: anthropic/claude-sonnet-4-6

# Extended (amendment 신규, optional):
aliases:
  opus:
    canonical:     anthropic/claude-opus-4-9
    deprecated:    false
  sonnet-old:
    canonical:     anthropic/claude-sonnet-3-5
    deprecated:    true
    replacedBy:    anthropic/claude-sonnet-4-6
  bigctx:
    canonical:     anthropic/claude-opus-4-7
    contextWindow: 200000
```

두 형식 동시 지원: yaml.v3 의 `yaml.Node` 커스텀 unmarshal 로 string 또는 map 분기. 동일 파일 내 혼합 허용.

### 6.4 패키지 layout (변경 후)

```
internal/command/adapter/aliasconfig/
├── loader.go              # ⬆ Options 확장 (MergePolicy, Metrics) + ConfigPath/Reload methods
├── loader_test.go         # 보존 — 회귀 금지
├── loader_p3_test.go      # 보존 — 회귀 금지
├── integration_test.go    # 보존 — 회귀 금지
├── validate_test.go       # 보존 — 회귀 금지
├── merge.go               # ⬅ 신규 (Area 2): user+project merge logic
├── merge_test.go          # ⬅ 신규
├── policy.go              # ⬅ 신규 (Area 3): ValidationPolicy + ValidateAndPrune
├── policy_test.go         # ⬅ 신규
├── codes.go               # ⬅ 신규 (Area 4): ErrorCode + Categorize + ErrorCategory enum
├── codes_test.go          # ⬅ 신규
├── entries.go             # ⬅ 신규 (Area 5): AliasEntry + LoadEntries + yaml union unmarshal
├── entries_test.go        # ⬅ 신규
├── metrics.go             # ⬅ 신규 (Area 6): Metrics interface + noopMetrics
└── metrics_test.go        # ⬅ 신규
```

---

## 7. 부모 surface 보존 검증 (HARD)

본 amendment 는 다음 정적 검증을 머지 게이트로 채택:

1. **Export signature diff**: `go doc ./internal/command/adapter/aliasconfig` 출력에서 v0.1.0 baseline 과 비교 → 기존 export 0 변경 (AC-AMEND-050).
2. **Test regression**: `go test ./internal/command/adapter/aliasconfig/... -count=10 -race` → 부모 5개 테스트 파일 PASS 유지 (AC-AMEND-051).
3. **HOTRELOAD-001 interface 호환**: HOTRELOAD-001 spec.md §6.7 의 `Loader` / `Validator` interface 와 본 amendment 의 시그니처 비교 — interface satisfaction 정적 검증 (AC-AMEND-052).

---

## 8. Test Plan

### 8.1 Unit tests (per-area)

- **Area 1**: `TestLoader_ConfigPath_*`, `TestLoader_Reload_*` (configpath round-trip, reload after file change)
- **Area 2**: `TestLoadDefault_MergeProjectOverride`, `TestLoadDefault_MergeUserOnly`, `TestLoadDefault_MergeProjectOnly`, `TestLoadDefault_OverrideLog` (zaptest observer)
- **Area 3**: `TestValidateAndPrune_StrictHappy`, `TestValidateAndPrune_StrictMixedErrors`, `TestValidateAndPrune_LenientPreservesValid`, `TestValidateAndPrune_InputNotMutated`, `TestValidateAndPrune_AggregateAsJoin`
- **Area 4**: `TestErrorCode_AllSentinels`, `TestErrorCode_NilEmpty`, `TestErrorCode_UnknownEmpty`, `TestCategorize_*`
- **Area 5**: `TestLoadEntries_LegacyFlat`, `TestLoadEntries_ExtendedFull`, `TestLoadEntries_MixedYAML`, `TestLoad_LegacyDropsExtensionFields`
- **Area 6**: `TestMetrics_LoadSuccess`, `TestMetrics_LoadFailure`, `TestMetrics_ValidationErrors`, `TestMetrics_NoopDefault_ZeroAlloc`

### 8.2 회귀 테스트

- 부모 5개 테스트 파일을 변경 없이 `go test -count=10` 로 재실행 → 모두 PASS.

### 8.3 정적 검증

- `gofmt -l . | grep aliasconfig | grep .` → empty (clean).
- `golangci-lint run ./internal/command/adapter/aliasconfig/...` → 0 issues.
- `go vet ./internal/command/adapter/aliasconfig/...` → 0 violations.
- `go doc -all ./internal/command/adapter/aliasconfig` 출력 baseline diff → 기존 export 0 변경 (CI gate).

### 8.4 Coverage 목표

- 라인 커버리지: ≥ 90% (신규 5개 파일 합산)
- branch 커버리지: ≥ 85%

---

## 9. 의존성 (Dependencies)

| 종류 | 대상 | 관계 |
|------|----|------|
| amendment 부모 | SPEC-GOOSE-ALIAS-CONFIG-001 v0.1.0 | FROZEN export 표면 보존, 신규 표면만 추가 |
| 후속 협력 | SPEC-GOOSE-CMDCTX-HOTRELOAD-001 (planned) | 본 amendment 의 ConfigPath / Reload / Metrics 가 watcher 통합 비용 ↓ |
| read-only consume | SPEC-GOOSE-ROUTER-001 (FROZEN) | `*router.ProviderRegistry` 변경 없음 |
| read-only consume | SPEC-GOOSE-CONFIG-001 (FROZEN) | `resolveGooseHome` 패턴 보존 |

신규 외부 의존성: **0건**. 본 amendment 는 기존 imports (`io/fs`, `gopkg.in/yaml.v3`, `go.uber.org/zap`, `github.com/modu-ai/goose/internal/llm/router`, stdlib `errors`/`time`) 만 사용.

---

## 10. Exclusions (What NOT to Build)

본 amendment 가 **명시적으로 제외**하는 항목:

1. **Hot-reload 자체 구현 (fsnotify watcher)**: SPEC-GOOSE-CMDCTX-HOTRELOAD-001 책임. 본 amendment 는 hook (ConfigPath, Reload, Metrics) 만 마련.
2. **Sentinel error 신규 추가**: 기존 sentinel 에 stable code 매핑만 부여. 신규 sentinel 은 v0.2 결정.
3. **i18n 메시지 변환 인프라**: `ErrorFormatter` interface / `ErrorLocale` 옵션은 본 amendment 비대상 — `language.yaml error_messages: en` 정합. ErrorCode 안정 키만 제공하여 후속 i18n SPEC 가 매핑 테이블 작성 가능하도록 함.
4. **`Validate` 기존 함수의 in-place 삭제**: 기존 `Validate(m, reg, strict)` 시그니처 보존. in-place 삭제 의미는 **신규 `ValidateAndPrune`** 에서만 명시. 기존 호출자 (`integration_test.go` 의 lenient 모드 검증) 는 변화 없음.
5. **AliasEntry 의 추가 필드** (예: `tags []string`, `cost map[string]float64`): v0.2 결정. 본 amendment 는 4개 메타 필드 (`Deprecated`, `ReplacedBy`, `ContextWindow`, `ProviderHints`) 만 도입.
6. **`Options.MergePolicy` 외 추가 합성 정책** (예: `Disjoint` / `StrictDisjoint` / `Manual`): v0.2 결정.
7. **Metrics 의 prometheus / opentelemetry 직접 의존성**: noop interface 만 도입. 표준 collector 통합은 후속 OBS-METRICS SPEC 책임.
8. **`Reload` 의 partial reload / atomic swap 동시성 보장**: HOTRELOAD-001 의 `*atomic.Pointer[map]` 패턴이 ContextAdapter 측에서 처리. 본 amendment 의 `Reload` 는 단순 fresh disk read.
9. **YAML schema validation strict-unknown 모드**: 부모 spec.md §6.4 의 v0.2 검토 사항. 본 amendment 비대상.
10. **Fork chain**: 사용자 file → org file → project file 3-level merge. v0.1 amendment 는 user + project 2-level 만.

---

## 11. References

- 부모 SPEC: `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` (v0.1.0, FROZEN)
- 부모 progress: `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/progress.md` (Open Question #2)
- 후속 협력 SPEC: `.moai/specs/SPEC-GOOSE-CMDCTX-HOTRELOAD-001/spec.md` (planned, P4)
- 부모 코드: `internal/command/adapter/aliasconfig/loader.go`
- 부모 테스트: `internal/command/adapter/aliasconfig/{loader_test.go, loader_p3_test.go, integration_test.go, validate_test.go}`
- Research 동반 문서: `./research.md` (본 디렉토리)
- Local convention: `CLAUDE.local.md §2.5` (코드 주석 영어)

---

## 12. Constitution Alignment

본 amendment 는 다음 프로젝트 헌법(`.moai/project/tech.md`) 항목을 준수:

- **Go 1.23+ tooling**: 기존 의존성 (yaml.v3, zap) 만 사용. 신규 외부 의존성 0건.
- **TRUST 5**: Tested (≥90% 커버리지 목표 + 회귀 100%), Readable (godoc 영어), Unified (gofmt + golangci-lint 0 issues), Secured (input mutation 금지, sentinel 보존, noop default), Trackable (REQ↔AC 1:1+ 매핑 + @MX:ANCHOR on Loader.Reload + Metrics).
- **Code comment 영어 정책** (`language.yaml code_comments: en`): 모든 신규 godoc / @MX 본문 영어.
- **TIME ESTIMATION 금지**: §13 milestone 은 priority + ordering 만 사용.
- **AskUserQuestion-Only**: 본 amendment 자체 결정 사항은 plan 단계에서 사용자 confirm 필요 — research §9 Open Questions 4건 (backward-compat release notes, yaml union 복잡도, Metrics 표준 align, HOTRELOAD-001 머지 순서).

---

## 13. Acceptance Summary

- **REQ count**: 12 (Ubiquitous 4, Event-Driven 2, State-Driven 3, Unwanted 2, Optional 1)
  - REQ-AMEND-{001..004, 010..011, 020..022, 030..031, 040}
- **AC count**: 14 (governance 3건 포함)
  - AC-AMEND-{001..004, 010-A, 010-B, 011, 020..022, 030..031, 040, 050..052}
- **신규 외부 의존성**: 0
- **신규 패키지**: 0 (기존 `aliasconfig/` 패키지에 5개 파일 추가)
- **수정 기존 패키지**: 1 (`internal/command/adapter/aliasconfig/loader.go` Options 확장 + 2개 method 추가)
- **부모 surface 변경**: 0 (HARD)
- **부모 회귀 허용**: 0 (HARD — 5개 테스트 파일 PASS 유지)
- **HOTRELOAD-001 정합 영향**: 강화 (ConfigPath / Reload / Metrics hook 사전 마련)
- **Open Question 해소**: 부모 progress.md OQ #2 (lenient in-place 삭제 의미론)
