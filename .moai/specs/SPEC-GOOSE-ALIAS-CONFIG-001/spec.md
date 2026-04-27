---
id: SPEC-GOOSE-ALIAS-CONFIG-001
version: 0.1.0
status: planned
created_at: 2026-04-27
updated_at: 2026-04-27
author: manager-spec
priority: P2
issue_number: null
phase: 2
size: 소(S)
lifecycle: spec-anchored
labels: [area/config, area/cli, type/feature, priority/p2-medium]
---

# SPEC-GOOSE-ALIAS-CONFIG-001 — Model Alias Config File Loader

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. SPEC-GOOSE-CMDCTX-001 (PR #52, c018ec5 머지)에서 노출된 `adapter.Options.AliasMap` 필드를 `~/.goose/aliases.yaml` 파일에서 채워주는 wiring SPEC. flat `aliases:` 스키마, GOOSE_HOME 경로 컨벤션, strict/lenient validation, project-local overlay 포함. hot-reload 미포함 (별도 SPEC). | manager-spec |

---

## 1. 개요 (Overview)

`SPEC-GOOSE-CMDCTX-001` (implemented, FROZEN) 의 머지로 `internal/command/adapter/adapter.go` 에 다음 surface가 노출되었다:

```go
type Options struct {
    Registry       *router.ProviderRegistry
    LoopController LoopController
    AliasMap       map[string]string  // ← REQ-CMDCTX-017
    GetwdFn        func() (string, error)
    Logger         Logger
}
```

`AliasMap` 은 옵션 필드이지만 **현재 호출자가 채워주는 데이터 소스가 정의되어 있지 않다**. 본 SPEC 은 그 빈 자리를 채운다.

본 SPEC 수락 시점에서:

- `internal/command/adapter/aliasconfig/` 신규 패키지가 존재한다.
- `aliasconfig.Loader` 가 `~/.goose/aliases.yaml` (또는 `GOOSE_ALIAS_FILE` 지정 경로)을 파싱하여 `map[string]string` 을 반환한다.
- `aliasconfig.Validate(map, registry, strict)` 가 alias canonical 값을 `router.ProviderRegistry` 와 대조 검증한다.
- daemon bootstrap 에서 `aliasconfig.LoadDefault()` → `adapter.New(Options{ AliasMap: ... })` wiring이 이루어진다.
- 파일 부재 / 빈 파일 / 권한 없음에 대해 graceful default (빈 맵 + warn log).

본 SPEC 은 **alias 데이터 공급 layer 만** 정의한다. resolution 알고리즘 자체는 CMDCTX-001 §6.4 `resolveAlias` 함수 (FROZEN) 가 이미 구현했다.

---

## 2. 배경 (Background)

### 2.1 의존 SPEC (모두 FROZEN)

- **SPEC-GOOSE-CMDCTX-001** (implemented, PR #52, c018ec5): `adapter.Options.AliasMap` 필드와 `resolveAlias` 함수의 알고리즘. 본 SPEC 은 `Options.AliasMap` 에 데이터를 주입할 뿐, 이 SPEC 의 surface는 변경하지 않는다.
- **SPEC-GOOSE-ROUTER-001** (implemented): `*router.ProviderRegistry`, `ProviderMeta.SuggestedModels` API를 read-only validation 에 사용.
- **SPEC-GOOSE-CONFIG-001** (implemented): `internal/config/` 의 `resolveGooseHome()` 패턴 (`GOOSE_HOME` env → `$HOME/.goose` fallback) 을 동일 의미론으로 차용. 본 SPEC 은 별도 패키지로 분리되며 `Config` struct 자체에는 alias 필드를 추가하지 않는다.

### 2.2 왜 지금 필요한가

- **CMDCTX-001 implemented 직후 가치 회수**: alias 데이터 소스가 없으면 CMDCTX-001 가 만들어둔 alias resolution 분기는 dead code.
- **사용자 UX**: `/model anthropic/claude-opus-4-7` 같은 long-form 입력 대신 `/model opus` 가 동작해야 슬래시 명령 시스템의 가치가 완성된다.
- **다중 머신 alias 동기화**: 파일 기반이어야 dotfiles repo / config 매니지먼트로 공유 가능.

### 2.3 명시적 비대상

- Hot-reload (파일 watch → in-process map 교체): SPEC-GOOSE-HOTRELOAD-001 (작성 중)에 위임.
- Dynamic alias 등록 명령(`/alias add ...`): 후속 SPEC.
- Alias namespace / group / 환경별 layering: v0.1 비대상.

---

## 3. 용어 (Terminology)

| 용어 | 정의 |
|-----|------|
| alias | 사용자가 짧게 타이핑할 식별자 (`opus`, `sonnet`). |
| canonical | `provider/model` 형식의 정식 식별자 (`anthropic/claude-opus-4-7`). |
| alias map | `map[string]string` 형태로 alias → canonical 을 저장. `adapter.Options.AliasMap` 에 주입되는 자료구조. |
| user file | `$GOOSE_HOME/aliases.yaml` (default `$HOME/.goose/aliases.yaml`). |
| project file | `$CWD/.goose/aliases.yaml` (project-local overlay, optional). |
| effective file | `GOOSE_ALIAS_FILE` env 가 설정된 경우 그 경로. 미설정 시 user file → project file 순서로 merge. |
| strict mode | validation 시 canonical 이 `ProviderRegistry` 에 등록되지 않으면 error 반환. |
| lenient mode | validation 시 canonical 미등록을 warn log 로만 처리, 해당 entry skip. |

---

## 4. 요구사항 (Requirements, EARS)

### 4.1 Ubiquitous Requirements

#### REQ-ALIAS-001 — Loader 패키지 존재

The system **shall** expose a Go package at `internal/command/adapter/aliasconfig/` that provides a `Loader` type with `Load(path string) (map[string]string, error)` and `LoadDefault(opts Options) (map[string]string, error)` methods.

근거: 본 SPEC 의 outcome — daemon bootstrap 호출자가 의존할 surface.

#### REQ-ALIAS-002 — 부재 파일에 graceful

The system **shall** return an empty `map[string]string` (non-nil) and a nil error when the resolved alias file path does not exist on disk.

근거: alias 파일은 항상 optional. 부재가 fail 사유가 되면 첫 사용자가 부팅 불가.

#### REQ-ALIAS-003 — 빈 파일에 graceful

The system **shall** return an empty `map[string]string` and a nil error when the alias file exists but contains no `aliases:` key (또는 `aliases:` 가 빈 맵).

근거: 사용자가 파일을 만들고 비워두는 경우는 정상 상태.

#### REQ-ALIAS-004 — Validate 함수 존재

The system **shall** expose a `Validate(m map[string]string, registry *router.ProviderRegistry, strict bool) []error` function returning per-entry errors.

근거: loader 자체와 registry-coupled validation 의 책임 분리. 호출자가 검증 실행 시점을 제어.

#### REQ-ALIAS-005 — Loader 는 panic 없음

The system **shall** not panic on any input — malformed YAML, nil registry, oversize file, or missing file all return structured errors.

근거: `internal/config/` 와 동일한 robust contract.

#### REQ-ALIAS-006 — daemon bootstrap wiring

The system **shall** wire `aliasconfig.LoadDefault` 의 결과를 `adapter.New(Options{AliasMap: ...})` 호출에 전달하는 코드를 daemon bootstrap (`cmd/goosed/main.go` 또는 등가) 에 포함한다.

근거: wiring 이 없으면 새 loader 는 dead code.

### 4.2 Event-Driven Requirements

#### REQ-ALIAS-010 — 정상 파일 → map 반환

**When** the resolved alias file exists, is readable, and parses as a valid YAML document with shape `{ aliases: <flat string-to-string map> }`, the system **shall** return that map with all entries preserved.

근거: 본 SPEC 의 정상 경로.

#### REQ-ALIAS-011 — Validate 호출 → 결과 합산

**When** `Validate(m, registry, true)` is called with a non-empty map, the system **shall** return a slice of errors — one per entry that fails validation — empty slice if all entries pass.

근거: 호출자가 모든 invalid entry 를 한 번에 보고받아야 (한 번에 하나만 나오면 디버깅 회귀 비용 큼).

#### REQ-ALIAS-012 — 부트스트랩 wiring → AliasMap 주입

**When** daemon bootstrap calls `adapter.New`, the resulting `ContextAdapter` **shall** have `aliasMap` field equal to the value returned by `aliasconfig.LoadDefault` (after Validate).

근거: REQ-ALIAS-006 의 부작용 검증.

### 4.3 State-Driven Requirements

#### REQ-ALIAS-020 — `GOOSE_ALIAS_FILE` 우선

**While** environment variable `GOOSE_ALIAS_FILE` is set to a non-empty string, the system **shall** treat that value as the absolute alias file path and ignore the user/project file chain.

근거: 테스트 / 비표준 배포 / 멀티테넌트 격리를 위한 explicit override.

#### REQ-ALIAS-021 — `GOOSE_HOME` 기반 fallback

**While** `GOOSE_ALIAS_FILE` is unset and `GOOSE_HOME` is set, the system **shall** resolve the user file as `$GOOSE_HOME/aliases.yaml`.

#### REQ-ALIAS-022 — `$HOME` 기반 final fallback

**While** both `GOOSE_ALIAS_FILE` and `GOOSE_HOME` are unset, the system **shall** resolve the user file as `$HOME/.goose/aliases.yaml`.

근거: SPEC-GOOSE-CONFIG-001 의 `resolveGooseHome` 동작과 동치.

#### REQ-ALIAS-023 — strict 모드 기본 동작

**While** environment variable `GOOSE_ALIAS_STRICT` is unset or equals `"true"`, `Validate(m, registry, strict=true)` **shall** be the default mode used by daemon bootstrap.

#### REQ-ALIAS-024 — lenient 모드 opt-in

**While** environment variable `GOOSE_ALIAS_STRICT` equals `"false"`, daemon bootstrap **shall** call `Validate(m, registry, strict=false)` and treat returned errors as warn logs only — invalid entries are removed from the map but the daemon continues.

근거: third-party provider plugin 등록이 늦은 경우의 fail-fast 회피.

### 4.4 Unwanted Behaviour

#### REQ-ALIAS-030 — Malformed YAML reject

**If** the alias file exists but fails YAML parsing, **then** the system **shall** return a wrapped error of sentinel type `ErrMalformedAliasFile` containing the file path and the underlying yaml.v3 error (line/column when available).

#### REQ-ALIAS-031 — 빈 alias key reject

**If** the parsed YAML contains an entry whose alias key is the empty string (`"" : "anthropic/claude-opus-4-7"`), **then** `Loader.Load` **shall** return `ErrEmptyAliasEntry` and **shall not** include the entry in the returned map.

#### REQ-ALIAS-032 — 빈 canonical value reject

**If** the parsed YAML contains an entry whose canonical value is the empty string (`"opus": ""`), **then** `Loader.Load` **shall** return `ErrEmptyAliasEntry`.

#### REQ-ALIAS-033 — Slash 없는 canonical reject

**If** the canonical value does not contain exactly one `/` separator with non-empty provider and model parts (e.g., `"opus": "claudeopus47"` or `"opus": "/claude"` or `"opus": "anthropic/"`), **then** `Loader.Load` **shall** return `ErrInvalidCanonical` for that entry.

근거: alias chaining 비허용 (research §4.2). canonical 형식을 강제하면 cycle 구조적 불가능.

#### REQ-ALIAS-034 — strict 모드에서 unknown provider reject

**If** strict mode is active **AND** the canonical's provider segment does not exist in `router.ProviderRegistry.Get()`, **then** `Validate` **shall** return `ErrUnknownProviderInAlias` for that entry.

#### REQ-ALIAS-035 — strict 모드에서 unknown model reject

**If** strict mode is active **AND** the canonical's model segment is not in `meta.SuggestedModels` of the matched provider, **then** `Validate` **shall** return `ErrUnknownModelInAlias` for that entry.

#### REQ-ALIAS-036 — Oversize file reject

**If** the alias file size exceeds 1 MiB (1,048,576 bytes), **then** `Loader.Load` **shall** return `ErrAliasFileTooLarge` without parsing the file.

근거: yaml.v3 메모리 폭발 방지. CONFIG-001 과 동일 패턴.

#### REQ-ALIAS-037 — Permission denied 처리

**If** the alias file exists but cannot be opened due to permission errors, **then** `Loader.Load` **shall** return a wrapped `os.PathError` (not a sentinel), and daemon bootstrap **shall** log a warning and proceed with an empty map.

근거: 권한 문제는 파일 부재와 다르지만 부팅 차단은 과잉. 명시적 warn log.

### 4.5 Optional Features

#### REQ-ALIAS-040 — Project-local overlay

**Where** a file at `$CWD/.goose/aliases.yaml` exists in addition to the user file, the system **shall** parse both and merge them with project-file entries overriding user-file entries on key conflict, and **shall** log each override at info level.

근거: SPEC-GOOSE-CONFIG-001 의 priority order (project > user) 와 동일.

#### REQ-ALIAS-041 — Logger 주입

**Where** `Loader.Options.Logger` is non-nil, the system **shall** emit zap log entries for: file resolution path, override events, validation warnings (lenient mode), and oversize/permission errors.

#### REQ-ALIAS-042 — `fs.FS` injection (테스트)

**Where** `Loader.Options.FS` is non-nil, the system **shall** read alias files via the injected `fs.FS` instead of `os.DirFS("/")`, enabling table-driven tests with `fstest.MapFS`.

근거: CONFIG-001 의 testability 패턴 차용. 신규 외부 의존성 0건.

---

## 5. Acceptance Criteria

| ID | 검증 대상 | 검증 방법 |
|----|----------|---------|
| AC-ALIAS-001 | REQ-ALIAS-001 | `internal/command/adapter/aliasconfig/loader.go` 존재 + `Loader`, `Options`, `Load`, `LoadDefault` exported. `go doc` 출력. |
| AC-ALIAS-002 | REQ-ALIAS-002 | Unit test: 부재 경로 입력 → 빈 map (`len() == 0`, `m != nil`), `err == nil`. |
| AC-ALIAS-003 | REQ-ALIAS-003 | Unit test: 파일 내용 `aliases: {}` 와 빈 파일 `""` 두 케이스 모두 빈 map 반환. |
| AC-ALIAS-004 | REQ-ALIAS-004 | `Validate(m, reg, strict)` 시그니처 정확 (`[]error` 반환). 빈 맵 입력 → 빈 슬라이스. |
| AC-ALIAS-005 | REQ-ALIAS-005 | Fuzz 테스트 또는 negative table test: 임의 byte stream 입력에 panic 없음 (`recover()` 미발동). |
| AC-ALIAS-006 | REQ-ALIAS-006 | `cmd/goosed/main.go` (또는 등가) 정적 grep: `aliasconfig.LoadDefault` 호출 + `adapter.New` 호출에 `AliasMap:` 필드 전달 동시 존재. |
| AC-ALIAS-010 | REQ-ALIAS-010 | Unit test: `aliases: { opus: anthropic/claude-opus-4-7, sonnet: anthropic/claude-sonnet-4-6 }` → 2-entry map, 키-값 정확 일치. |
| AC-ALIAS-011 | REQ-ALIAS-011 | Unit test: 3개 entry 중 2개 invalid → `len(errs) == 2`, 각 error 가 해당 alias key 식별 가능. |
| AC-ALIAS-012 | REQ-ALIAS-012 | Integration test: daemon bootstrap (또는 helper) 호출 후 `adapter.ContextAdapter` 의 alias map 이 file 내용과 일치. |
| AC-ALIAS-020 | REQ-ALIAS-020 | Unit test: `GOOSE_ALIAS_FILE=/tmp/custom.yaml` 설정 + GOOSE_HOME 도 설정 → `/tmp/custom.yaml` 만 읽힘 (`fs.FS` 호출 spy). |
| AC-ALIAS-021 | REQ-ALIAS-021 | Unit test: `GOOSE_ALIAS_FILE` unset, `GOOSE_HOME=/opt/goose` → `/opt/goose/aliases.yaml` 조회. |
| AC-ALIAS-022 | REQ-ALIAS-022 | Unit test: 둘 다 unset, `HOME=/home/user` (env override) → `/home/user/.goose/aliases.yaml` 조회. |
| AC-ALIAS-023 | REQ-ALIAS-023 | Unit test: `GOOSE_ALIAS_STRICT` unset → bootstrap 의 strict 인자 `true`. |
| AC-ALIAS-024 | REQ-ALIAS-024 | Unit test: `GOOSE_ALIAS_STRICT=false` → bootstrap 의 strict 인자 `false`, invalid entry 가 map 에서 제거된 채 daemon 계속 진행. |
| AC-ALIAS-030 | REQ-ALIAS-030 | Unit test: `aliases: [` 같은 malformed input → `errors.Is(err, ErrMalformedAliasFile)` true. |
| AC-ALIAS-031 | REQ-ALIAS-031 | Unit test: `aliases: { "": "anthropic/claude-opus-4-7" }` → `errors.Is(err, ErrEmptyAliasEntry)` true. |
| AC-ALIAS-032 | REQ-ALIAS-032 | Unit test: `aliases: { opus: "" }` → `errors.Is(err, ErrEmptyAliasEntry)` true. |
| AC-ALIAS-033 | REQ-ALIAS-033 | Unit test: 3 케이스 (`claudeopus47`, `/claude`, `anthropic/`) 모두 `ErrInvalidCanonical`. |
| AC-ALIAS-034 | REQ-ALIAS-034 | Unit test: registry 에 `unknown` provider 미등록 + entry `opus: unknown/x` + strict=true → `ErrUnknownProviderInAlias`. |
| AC-ALIAS-035 | REQ-ALIAS-035 | Unit test: registry 에 `anthropic` 등록 + `SuggestedModels=["claude-opus-4-7"]` + entry `x: anthropic/nonexistent` + strict=true → `ErrUnknownModelInAlias`. |
| AC-ALIAS-036 | REQ-ALIAS-036 | Unit test: 1 MiB + 1 byte 파일 → `ErrAliasFileTooLarge`, 파싱 시도 없음 (memory profile 검증 또는 spy). |
| AC-ALIAS-037 | REQ-ALIAS-037 | Unit test: chmod 0000 파일 → wrapped `os.PathError` 반환, sentinel 아님. bootstrap helper 호출 시 빈 map 반환 + warn log 1회 (zap testing observer). |
| AC-ALIAS-040 | REQ-ALIAS-040 | Unit test: user file `{opus: anthropic/claude-opus-4-7}` + project file `{opus: anthropic/claude-sonnet-4-6}` → merged map `{opus: anthropic/claude-sonnet-4-6}` + info log 1회 (override 알림). |
| AC-ALIAS-041 | REQ-ALIAS-041 | Unit test: zaptest observer 로 file resolution / override / warn 로그가 각각 발생함을 검증. |
| AC-ALIAS-042 | REQ-ALIAS-042 | Unit test: `fstest.MapFS` 주입 시 디스크 접근 없이 동작함 — `os.Open` 호출 0회 (httptest 스타일 spy). |
| AC-ALIAS-050 | 신규 외부 의존성 0건 | `go.mod` diff 검증: yaml.v3, zap, gotest 외 신규 require 없음. CI에서 `go mod why` 정적 체크. |
| AC-ALIAS-051 | 패키지 격리 | `internal/command/adapter/aliasconfig/` 가 `internal/llm/router` 외 다른 internal 패키지를 import 하지 않음 (`go list -deps` 정적 검증). |

---

## 6. 데이터 모델 / API 설계

### 6.1 패키지 layout

```
internal/command/adapter/aliasconfig/
├── loader.go         // Loader, Options, Load, LoadDefault
├── validate.go       // Validate
├── errors.go         // sentinel errors
├── loader_test.go
└── validate_test.go
```

### 6.2 타입 정의

```go
// Package aliasconfig loads model alias maps from YAML files for
// consumption by command/adapter.ContextAdapter.
//
// SPEC-GOOSE-ALIAS-CONFIG-001
// @MX:ANCHOR: [AUTO] alias map data source for SPEC-GOOSE-CMDCTX-001.
// @MX:REASON: Single bootstrap consumer; misroute breaks /model alias UX.
// @MX:SPEC: SPEC-GOOSE-ALIAS-CONFIG-001 REQ-ALIAS-001
package aliasconfig

import (
    "io/fs"

    "go.uber.org/zap"

    "github.com/modu-ai/goose/internal/llm/router"
)

// Options configure a Loader instance.
type Options struct {
    // FS overrides the filesystem for test injection. Defaults to os.DirFS("/").
    FS fs.FS
    // Logger receives info/warn entries. zap.NewNop() if nil.
    Logger *zap.Logger
    // GooseHome forces a specific GOOSE_HOME for test isolation.
    // Empty string means "read environment".
    GooseHome string
    // EnvOverrides supplies test-only env values. nil means use os.Getenv.
    EnvOverrides map[string]string
    // WorkDir overrides the project-overlay search root. Empty means os.Getwd.
    WorkDir string
}

// Loader reads alias maps from disk.
type Loader struct {
    opts Options
}

// New constructs a Loader. Zero-valued Options are valid.
func New(opts Options) *Loader { /* ... */ }

// Load reads a single absolute path and returns the parsed alias map.
// Returns (empty-map, nil) on missing file (REQ-ALIAS-002).
func (l *Loader) Load(path string) (map[string]string, error) { /* ... */ }

// LoadDefault resolves the effective alias file chain and returns the merged map.
// Resolution order:
//   1. GOOSE_ALIAS_FILE  (absolute, REQ-ALIAS-020)
//   2. $GOOSE_HOME/aliases.yaml (REQ-ALIAS-021)
//   3. $HOME/.goose/aliases.yaml (REQ-ALIAS-022)
//   4. $WorkDir/.goose/aliases.yaml (project overlay, REQ-ALIAS-040)
// Project entries override user entries on key conflict.
func (l *Loader) LoadDefault() (map[string]string, error) { /* ... */ }

// Validate checks each entry against the registry. Returns a slice of errors,
// one per failing entry. An empty slice means all entries pass.
//
// strict=true:  unknown provider/model -> error in slice (REQ-ALIAS-034/035)
// strict=false: unknown provider/model -> warn-log only, entry kept in map
func Validate(
    m map[string]string,
    registry *router.ProviderRegistry,
    strict bool,
) []error { /* ... */ }
```

### 6.3 Sentinel errors

```go
// errors.go
package aliasconfig

import "errors"

var (
    // ErrMalformedAliasFile wraps yaml.v3 parse failures.
    ErrMalformedAliasFile = errors.New("aliasconfig: malformed alias file")
    // ErrEmptyAliasEntry indicates a blank key or value.
    ErrEmptyAliasEntry = errors.New("aliasconfig: empty alias key or value")
    // ErrInvalidCanonical indicates a canonical value missing the "provider/model" form.
    ErrInvalidCanonical = errors.New("aliasconfig: canonical must be provider/model")
    // ErrUnknownProviderInAlias indicates strict-mode validation failure: provider not in registry.
    ErrUnknownProviderInAlias = errors.New("aliasconfig: unknown provider in alias canonical")
    // ErrUnknownModelInAlias indicates strict-mode validation failure: model not in SuggestedModels.
    ErrUnknownModelInAlias = errors.New("aliasconfig: unknown model in alias canonical")
    // ErrAliasFileTooLarge indicates the file exceeds 1 MiB.
    ErrAliasFileTooLarge = errors.New("aliasconfig: alias file exceeds 1 MiB cap")
)
```

### 6.4 YAML 스키마 (정식)

```yaml
# ~/.goose/aliases.yaml
# SPEC-GOOSE-ALIAS-CONFIG-001
#
# Each entry maps a short alias to a canonical "provider/model" string.
# The canonical value MUST exist in router.ProviderRegistry SuggestedModels
# unless GOOSE_ALIAS_STRICT=false is set.
#
# Recommended file mode: chmod 600

aliases:
  opus:   anthropic/claude-opus-4-7
  sonnet: anthropic/claude-sonnet-4-6
  haiku:  anthropic/claude-haiku-4-5
  gpt4o:  openai/gpt-4o
```

**Top-level shape (Go struct)**:

```go
type fileSchema struct {
    Aliases map[string]string `yaml:"aliases"`
}
```

Top-level 다른 키는 yaml.v3 default behavior 로 silent-ignore (CONFIG-001 의 strict 모드처럼 추가 검증을 본 SPEC 에 도입할지는 v0.2 결정 사항).

### 6.5 Wiring (daemon bootstrap)

```go
// cmd/goosed/main.go (or equivalent bootstrap)
// SPEC-GOOSE-ALIAS-CONFIG-001 REQ-ALIAS-006

loader := aliasconfig.New(aliasconfig.Options{
    Logger:    logger,
    GooseHome: cfg.GooseHome(),
})
aliasMap, err := loader.LoadDefault()
if err != nil {
    logger.Warn("alias config load failed; continuing with empty map",
        zap.Error(err))
    aliasMap = map[string]string{}
}

strict := envBool("GOOSE_ALIAS_STRICT", true)
if errs := aliasconfig.Validate(aliasMap, registry, strict); len(errs) > 0 {
    if strict {
        return fmt.Errorf("alias validation failed: %w",
            errors.Join(errs...))
    }
    for _, e := range errs {
        logger.Warn("alias entry rejected (lenient)", zap.Error(e))
    }
    // lenient: invalid entries removed by Validate (REQ-ALIAS-024)
}

ctxAdapter := adapter.New(adapter.Options{
    Registry:       registry,
    LoopController: loopCtrl,
    AliasMap:       aliasMap,
    Logger:         logger,
})
```

> `Validate` lenient 모드의 "invalid entries 제거" 의미론은 §4.3 REQ-ALIAS-024 에 의해 보장된다 — 구현 시 `Validate` 가 in-place 삭제할지, 호출자가 errors 보고 직접 삭제할지는 implementation detail. 권장: `Validate` 가 lenient 모드일 때 in-place 삭제 (호출자 단순화).

---

## 7. Test Plan

### 7.1 Unit tests (loader_test.go)

테이블 드리븐, `fstest.MapFS` 주입:

- `TestLoader_Load_Missing` (REQ-ALIAS-002)
- `TestLoader_Load_Empty` (REQ-ALIAS-003)
- `TestLoader_Load_Valid` (REQ-ALIAS-010)
- `TestLoader_Load_Malformed` (REQ-ALIAS-030)
- `TestLoader_Load_EmptyKey` (REQ-ALIAS-031)
- `TestLoader_Load_EmptyValue` (REQ-ALIAS-032)
- `TestLoader_Load_InvalidCanonical` (REQ-ALIAS-033, 3 sub-cases)
- `TestLoader_Load_OversizeFile` (REQ-ALIAS-036)
- `TestLoader_Load_PermissionDenied` (REQ-ALIAS-037)
- `TestLoader_LoadDefault_EnvFile` (REQ-ALIAS-020)
- `TestLoader_LoadDefault_GooseHome` (REQ-ALIAS-021)
- `TestLoader_LoadDefault_HomeFallback` (REQ-ALIAS-022)
- `TestLoader_LoadDefault_ProjectOverlay` (REQ-ALIAS-040)
- `TestLoader_NoPanicOnRandomBytes` (REQ-ALIAS-005, fuzz seed 또는 large negative table)

### 7.2 Unit tests (validate_test.go)

- `TestValidate_AllValid` (REQ-ALIAS-011 happy)
- `TestValidate_PartialInvalid` (REQ-ALIAS-011 multi-error)
- `TestValidate_StrictUnknownProvider` (REQ-ALIAS-034)
- `TestValidate_StrictUnknownModel` (REQ-ALIAS-035)
- `TestValidate_LenientKeepsValid` (REQ-ALIAS-024 — invalid 제거 + valid 유지)

### 7.3 Integration test

- `TestBootstrap_AliasMapInjected` (REQ-ALIAS-006/012): daemon bootstrap helper 호출 → returned `*adapter.ContextAdapter` 의 `aliasMap` 이 file content 와 일치.

### 7.4 정적 검증

- `AC-ALIAS-050`: `go.mod` diff CI gate.
- `AC-ALIAS-051`: `go list -deps` 출력에서 `internal/command/adapter/aliasconfig` 의 import set 화이트리스트 (`io/fs`, `gopkg.in/yaml.v3`, `go.uber.org/zap`, `github.com/modu-ai/goose/internal/llm/router`) 외 internal 패키지 차단.

---

## 8. Constitution Alignment

본 SPEC 은 다음 프로젝트 헌법(`.moai/project/tech.md`) 항목을 준수한다:

- **Go 1.23+ tooling**: yaml.v3, zap 기존 의존성. 신규 외부 의존성 0건 (AC-ALIAS-050).
- **TRUST 5**: Tested (≥85% coverage 목표), Readable (godoc 영어, naming snake/camel 표준), Unified (gofmt + golangci-lint), Secured (sentinel error 우회 금지, 1 MiB cap), Trackable (REQ-AC 매핑 + @MX:ANCHOR).
- **Code comment 영어 정책** (`language.yaml code_comments: en`): 모든 신규 godoc / @MX 본문 영어로 작성.
- **TIME ESTIMATION 금지** (.claude/rules/moai/core/agent-common-protocol.md §Time Estimation): plan.md 의 milestone 은 priority + ordering 으로만 표현, 시간 단위 미사용.

---

## 9. Risks & Limitations

| # | 위험 | 완화 |
|---|-----|-----|
| R-1 | hot-reload 미지원 — alias 변경 시 daemon 재시작 필요 | SPEC-GOOSE-HOTRELOAD-001 에서 별도 해결. 본 SPEC 의 `Limitations` 절로 명시. |
| R-2 | strict 모드 false-fail (third-party plugin 의 model이 SuggestedModels 에 없음) | `GOOSE_ALIAS_STRICT=false` lenient 모드 제공 (REQ-ALIAS-024). |
| R-3 | XDG 비준수 (`~/.config/goose/aliases.yaml` 미지원) | SPEC-GOOSE-XDG-MIGRATION-001 별도 처리. 본 SPEC 의 `GOOSE_ALIAS_FILE` env 로 즉시 우회 가능. |
| R-4 | project overlay 가 git-commit 되어 팀원 alias 충돌 | sample 파일 헤더에 `# git-ignore recommended` 주석 추가 권장. CONFIG-001 동일 모델. |
| R-5 | yaml.v3 가 unknown top-level key silent-ignore | v0.1 허용 (사용자 확장 여지). v0.2 에서 strict-unknown 모드 검토. |

---

## 10. Exclusions (What NOT to Build)

본 SPEC 은 다음을 명시적으로 **포함하지 않는다**:

1. **Hot-reload / file watching**: SPEC-GOOSE-HOTRELOAD-001 위임.
2. **Dynamic alias 등록 (`/alias add` 슬래시 명령)**: 후속 SPEC.
3. **Alias namespace / group / context layering** (예: `aliases.work.opus`): v0.1 비대상.
4. **XDG `~/.config/goose/aliases.yaml` 지원**: SPEC-GOOSE-XDG-MIGRATION-001 위임.
5. **Alias chaining** (alias → alias → canonical): canonical 형식 강제로 구조적 차단.
6. **Resolution 알고리즘 변경**: SPEC-GOOSE-CMDCTX-001 §6.4 `resolveAlias` FROZEN.
7. **`Config` struct 에 alias 필드 추가**: 별도 패키지 분리 결정 (research §2.1).
8. **provider plugin 자동 등록 / discovery**: registry 운영은 ROUTER-001 책임.
9. **사용자에게 권한 / 보안 가이드 문서**: README/docs 별도 작업.
10. **Migration tool** (기존 환경변수 기반 alias 가 있다면 yaml 으로 이전): 기존 데이터 부재 — 불필요.

---

## 11. References

- 의존 SPEC: `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` (FROZEN, REQ-CMDCTX-017)
- 의존 SPEC: `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` (FROZEN, ProviderRegistry API)
- 의존 SPEC: `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` (FROZEN, GOOSE_HOME 컨벤션)
- 후속 SPEC: SPEC-GOOSE-HOTRELOAD-001 (작성 중) — 본 SPEC 의 hot-reload 책임 위임 대상
- 후속 SPEC: SPEC-GOOSE-XDG-MIGRATION-001 (계획) — `~/.config/goose` 경로 마이그레이션
- 코드 anchor: `internal/command/adapter/adapter.go:49-62` (Options.AliasMap 정의)
- 코드 anchor: `internal/command/adapter/alias.go:21-62` (resolveAlias 알고리즘)
- 코드 anchor: `internal/config/config.go:256-266` (resolveGooseHome 패턴)
- Local convention: `CLAUDE.local.md §2.5` (코드 주석 영어 정책)

---

## 12. Acceptance Summary

- **REQ count**: 24 (Ubiquitous 6, Event-Driven 3, State-Driven 5, Unwanted 8, Optional 3 — `REQ-ALIAS-{001..006, 010..012, 020..024, 030..037, 040..042}`)
- **AC count**: 27 (`AC-ALIAS-{001..006, 010..012, 020..024, 030..037, 040..042, 050..051}`)
- **신규 외부 의존성**: 0
- **신규 패키지**: 1 (`internal/command/adapter/aliasconfig/`)
- **수정 기존 패키지**: 1 (daemon bootstrap, `cmd/goosed/main.go` 또는 등가) — wiring 1 호출 추가
- **FROZEN 의존 SPEC**: 3 (CMDCTX-001, ROUTER-001, CONFIG-001) — 모두 read-only consume
