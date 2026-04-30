# SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 — Research

## 0. 메타

- 작성일: 2026-04-30
- 부모 SPEC: SPEC-GOOSE-ALIAS-CONFIG-001 v0.1.0 (status=completed, FROZEN, completed_at=2026-04-27)
- 작성자: manager-spec
- 목표: 부모 SPEC v0.1.0 의 export 표면은 보존하면서, 후속 SPEC (특히 SPEC-GOOSE-CMDCTX-HOTRELOAD-001 planned) 정합과 부모 progress.md 의 Open Questions 를 해소하는 amendment 범위를 식별.

## 1. 부모 v0.1.0 표면 인벤토리 (FROZEN, 변경 금지)

`internal/command/adapter/aliasconfig/loader.go` 직접 검토 결과, 다음 export 가 v0.1.0 시점에 확정되어 본 amendment 가 변경할 수 없다:

| Symbol | 종류 | 시그니처 / 정의 |
|--------|------|--------------|
| `Loader` | type (struct) | `configPath string`, `fsys fs.FS`, `logger Logger`, `stdLogger *log.Logger` |
| `Options` | type (struct) | `ConfigPath string`, `FS fs.FS`, `Logger Logger`, `StdLogger *log.Logger` |
| `Logger` | interface | `Debug/Info/Warn/Error(string, ...zap.Field)` |
| `AliasConfig` | type (struct) | `Aliases map[string]string` (yaml: aliases) |
| `New(opts Options) *Loader` | func | `Loader` 생성자 |
| `(*Loader).Load() (map[string]string, error)` | method | 인자 없음 — `configPath` 사용 |
| `(*Loader).LoadDefault() (map[string]string, error)` | method | 현재는 `Load()` 동의어 |
| `Validate(m map[string]string, registry *router.ProviderRegistry, strict bool) []error` | func | per-entry 에러 슬라이스 |
| `ErrConfigNotFound` | sentinel | (loader.go:24) |

> spec.md §6.2 가 제안한 `Load(path string)` 시그니처는 실제 구현이 인자 없이 채택. `EntryWarning` 타입은 spec.md 에 언급되었으나 구현 미반영. 본 amendment 는 이 두 비대칭을 변경하지 않는다 (현 구현이 권위).

추가로 외부 모듈에서 import 되는 상수/sentinel:

- `command.ErrEmptyAliasEntry`, `command.ErrInvalidCanonical`, `command.ErrUnknownProviderInAlias`, `command.ErrUnknownModelInAlias`, `command.ErrAliasFileTooLarge`, `command.ErrMalformedAliasFile` — 본 패키지 외부에 위치, 변경 금지.

### 1.1 Backward-compat 원칙 (HARD)

본 amendment 는 다음 보장한다:

1. 기존 호출자 (`cmd/goosed/main.go`, 테스트 파일) 의 코드 변경 0건이어도 동작 보존.
2. 기존 5개 테스트 파일 (`loader_test.go`, `loader_p3_test.go`, `integration_test.go`, `validate_test.go`) 의 모든 test 가 PASS 유지 (REGRESSION 금지).
3. 기존 sentinel error 는 유지 — 새 sentinel 만 추가 가능.
4. 기존 함수 시그니처 변경 금지 — overload 가 필요하면 신규 함수 추가.
5. yaml schema 의 기존 flat `aliases: {a: p/m}` 형식 지원 보존 — 신규 schema 는 union 으로만 추가.

## 2. 후속 의존 SPEC 분석 (HOTRELOAD-001 정합)

`SPEC-GOOSE-CMDCTX-HOTRELOAD-001` (planned, P4) 가 본 패키지에 요구하는 contract (HOTRELOAD spec.md §6.7 기준):

```go
type Loader interface {
    LoadDefault() (map[string]string, error)
}

type Validator interface {
    Validate(m map[string]string, registry *router.ProviderRegistry, strict bool) []error
}
```

본 amendment 가 이 두 시그니처를 변경하면 HOTRELOAD-001 가 깨진다. 따라서:

- `LoadDefault()` 는 보존 — 신규 ergonomic API 가 추가되더라도 기존 시그니처 유지.
- `Validate(m, registry, strict)` 는 보존 — 신규 policy-based API 가 추가되더라도 기존 시그니처 유지.

또한 HOTRELOAD-001 §10 Exclusions #9 가 명시:

> "multi-file alias overlay reload — `aliasconfig.LoadDefault` 가 매번 호출 시 fresh overlay 를 반환한다고 가정. 별도 검증은 ALIAS-CONFIG-001 책임."

→ 본 amendment 가 multi-source merge 를 명시화하면 위 가정의 검증 책임을 본 패키지에서 해소.

HOTRELOAD-001 §10 Exclusions #10 (telemetry / metrics 위임) 와 본 amendment Area 6 (observability hooks) 는 **부분적으로 선제 해결** 관계.

## 3. 부모 Open Questions 매핑

부모 `progress.md` (33-36행) 에 다음 3개 미해결 질문:

1. **패키지 위치 최종 확정** — 이미 `internal/command/adapter/aliasconfig/` 로 구현됨. 본 amendment 비대상 (RESOLVED).
2. **Lenient 모드 in-place 삭제 의미론** — 현 구현이 in-place 삭제 미수행. spec.md §6.5 권장은 in-place 였으나 caller 가 직접 처리. **본 amendment Area 3 에서 명시화** (NEW SURFACE).
3. **GOOSE_ALIAS_STRICT 기본값** — strict=true 로 결정됨 (RESOLVED).

→ Open Question #2 는 본 amendment 가 해소.

## 4. 6개 amendment 영역 분석

각 영역은 (현 상태 / 발견 결손 / 개선 후보 / 권장 / 신규 REQ 추정 / 부모 surface 영향 / HOTRELOAD-001 영향) 7-축으로 평가.

### Area 1 — Hot-reload 호환 API

**현 상태**:
- `Loader.configPath` 가 unexported. 외부에서 watcher 가 fsnotify 대상 경로를 알 방법은 별도 인자 주입뿐.
- `Load()` 는 매 호출 시 디스크 read — fresh 보장. 그러나 명시적 "reload" contract 부재.
- 호출자가 reload 결과를 관찰할 hook (Before/After, success/failure) 없음.

**발견 결손**:
- HOTRELOAD-001 의 `Watcher.Options.AliasFilePath` 는 호출자가 `aliasconfig` 외부에서 path 결정 로직을 다시 구현해야 함 (또는 `aliasconfig.New` 시점에 ConfigPath 를 명시적으로 줘야 함). DRY 위반.
- reload 성공/실패 카운트 emit 불가.

**개선 후보**:

| ID | 후보 | 영향 | 권장 |
|----|------|----|------|
| A1.1 | `(*Loader).ConfigPath() string` getter 신설 | 신규 method, backward compat OK | **YES** (HOTRELOAD-001 watcher 가 직접 호출) |
| A1.2 | `(*Loader).Reload() (map[string]string, error)` 신설 | 신규 method, 의미는 `Load()` 와 동일하나 명시적 의도 | **YES** (의도 명시화, deprecation 없이 공존) |
| A1.3 | `LoadHook func(map, error)` callback 옵션 (Options.OnLoad) | Options 추가 — backward compat OK (zero-value 무시) | **NO** (v0.2 결정. observer pattern 은 Area 6 metrics 와 중복) |
| A1.4 | `LoadResult` struct 반환 함수 (path + map + lastErr + entryCount) | 신규 함수, 의미는 풍부하나 즉시 가치 낮음 | **NO** (v0.2 결정) |

**권장 채택**: A1.1, A1.2.

**부모 surface 영향**: 0 (신규 method 만 추가).
**HOTRELOAD-001 정합**: 강화 — watcher 가 `loader.ConfigPath()` 로 path 자동 동기화, `loader.Reload()` 로 의도 명시.

**신규 REQ 추정**: 2건 (A1.1 + A1.2).

### Area 2 — Multi-source 합성 (User + Project overlay)

**현 상태**:
- `New(opts)` 의 path 결정 로직 (loader.go:62-78) 이 user file `OR` project file 중 **하나만** 선택. project file 이 있으면 user file 무시.
- 부모 spec.md §4.5 REQ-ALIAS-040 은 "둘 다 parse 후 merge with project override + 각 override info-log" 명세. **실제 구현이 SPEC 미준수**.
- AC-ALIAS-040 가 SPEC 에서는 정의되어 있으나 실제 테스트 파일 (`loader_p3_test.go`, `integration_test.go`) 에는 merge 시나리오 검증 부재.

**발견 결손**:
- v0.1.0 가 SPEC 본문대로 구현되지 않은 영역. amendment 가 본문 구현을 채워야 함.
- 단, **backward-compat 영향 큼**: 현재 user-only 시나리오 사용자가 project file 추가 시 동작 변화 (이전: project 단독 / 이후: project override on top of user).

**개선 후보**:

| ID | 후보 | 영향 | 권장 |
|----|------|----|------|
| A2.1 | User + project 동시 read, project entry override + info-log emit | `LoadDefault` 의미 변경 (의도된 것) | **YES** (REQ-ALIAS-040 본문 구현) |
| A2.2 | `Options.MergePolicy` enum (UserOnly / ProjectOverride / Disjoint) | 옵션 추가 — backward compat OK (default ProjectOverride 가 amendment 후 새 default) | **YES** (마이그레이션 안전판) |
| A2.3 | conflict 발생 시 entry 별 info-log 1건 + entry counter | 로깅 보강 | **YES** (REQ-ALIAS-040 본문 명시 사항) |
| A2.4 | `MergedSources []string` 디버그 메타데이터 노출 | 신규 type/method | **NO** (v0.2) |

**권장 채택**: A2.1, A2.2, A2.3.

**부모 surface 영향**: 0 (sigs 변경 0, 의미만 확장 + Options 신규 필드).
**HOTRELOAD-001 정합**: §10 #9 가정의 검증 책임 본 amendment 에서 해소.

**신규 REQ 추정**: 3건.

### Area 3 — Validation strict/lenient 모드 세분화

**현 상태**:
- `Validate(m, registry, strict bool)` 가 단일 bool 토글.
- lenient 모드에서 invalid entry 가 caller 측 map 에서 **자동 제거되지 않음** — caller 가 errs 를 보고 직접 삭제 책임. spec.md §6.5 "권장: in-place 삭제" 와 차이.
- per-entry skip 정책 (continue vs halt-on-first-error) 명시 부재. 현재는 항상 continue + collect.
- error aggregation 형태 단일 (`[]error` slice) — `errors.Join` 통합 옵션 없음.

**발견 결손**:
- 부모 progress.md Open Question #2 미해결 영역.
- HOTRELOAD-001 의 watcher 가 strict=true 시 invalid entry 가 있으면 reload 전체 abort — partial reload 옵션 부재로 운영 시 brittle.

**개선 후보**:

| ID | 후보 | 영향 | 권장 |
|----|------|----|------|
| A3.1 | `ValidationPolicy` struct (Strict / SkipOnError / MaxErrors / AggregateAsJoin) 신설 | 신규 type | **YES** |
| A3.2 | `ValidateAndPrune(m, registry, policy) (cleaned map[string]string, errs []error)` 신설 — in-place 삭제 의미 명시화 | 신규 함수 | **YES** (Open Q #2 해소) |
| A3.3 | per-entry classification: reject / skip / warn-only | A3.1 의 SkipOnError 가 흡수 | **YES** (A3.1 내) |
| A3.4 | `ValidationReport` 풍부한 메타데이터 type | 풍부하나 v0.1 과잉 | **NO** (v0.2) |

**권장 채택**: A3.1 + A3.2 (A3.3 흡수).

**부모 surface 영향**: 0 (기존 `Validate` 보존, 신규 add-on).
**HOTRELOAD-001 정합**: `Validator` interface 의 `Validate(m, registry, strict)` 시그니처 보존 — HOTRELOAD watcher 동작 무영향. 신규 `ValidateAndPrune` 는 advanced 호출자 전용.

**신규 REQ 추정**: 2건.

### Area 4 — Error message i18n hooks

**현 상태**:
- 모든 sentinel error 메시지가 영어 고정 (`language.yaml error_messages: en` 정합).
- `ErrConfigNotFound`, `command.ErrEmptyAliasEntry` 등은 stable 한 한국어/일본어 매핑 키 부재.
- user-facing 메시지 (`/reload aliases` 실패 출력 등) 는 호출자가 conversation_language 로 변환해야 하나, 안정 키 없음 — string match 가 fragile.

**발견 결손**:
- 후속 SPEC 가 i18n 인프라 도입 시 본 패키지의 error 분류를 stable key 로 식별할 방법 없음.

**개선 후보**:

| ID | 후보 | 영향 | 권장 |
|----|------|----|------|
| A4.1 | `ErrorFormatter` interface 옵션 (`Format(err error, locale string) string`) | 옵션 type 추가 — 사용 안 하면 zero-impact | **NO** (i18n 인프라 부재 시 추상 과잉) |
| A4.2 | `Options.ErrorLocale string` 필드 | 옵션 필드 추가 — 기본 "en" | **NO** (A4.1 와 동일 사유) |
| A4.3 | 각 sentinel 에 안정적인 error code 부여 — `ErrorCode(err error) string` 함수 신설 (e.g., "ALIAS-001") | 신규 함수 + 매핑 테이블 — backward compat OK | **YES** (i18n 매핑 테이블 안정 키 제공) |
| A4.4 | `Categorize(err error) Category` 함수 (Loader / Validate / Permission / Format 분류) | 신규 enum + 함수 | **YES** (호출자 분기 단순화) |

**권장 채택**: A4.3, A4.4.

**부모 surface 영향**: 0.
**HOTRELOAD-001 정합**: HOTRELOAD §6.9 의 watcher 로그 분류 (load fail vs validate fail) 가 안정 코드로 단순화 가능.

**신규 REQ 추정**: 1-2건.

### Area 5 — Schema 확장 (ProviderHints / ContextWindow / Deprecated)

**현 상태**:
- yaml schema 가 `aliases: {alias: "provider/model"}` flat string only.
- yaml.v3 가 unknown top-level key silent-ignore (부모 spec.md §6.4 v0.2 결정 사항).
- alias 별 부가 메타 (deprecated 안내, 권장 ContextWindow, provider 별 hint) 부재.

**발견 결손**:
- alias 가 모델 마이그레이션 신호로 사용될 때 (예: `claude-sonnet-3.5` → `claude-sonnet-4.6` 권장 alias) deprecated 표기 불가.
- provider 가 동일 모델을 다른 시점에 다른 ContextWindow 로 노출하면, alias 가 그 hint 를 들고 있을 수 없음.

**개선 후보**:

| ID | 후보 | 영향 | 권장 |
|----|------|----|------|
| A5.1 | `AliasEntry` struct (Canonical string + ContextWindow int + Deprecated bool + ReplacedBy string + ProviderHints map[string]string) | 신규 type | **YES** |
| A5.2 | yaml union 지원 — string 또는 map 중 하나 (yaml.v3 의 `yaml.Node` 커스텀 unmarshal) | 신규 unmarshaler | **YES** (backward compat 핵심) |
| A5.3 | 신규 export `(*Loader).LoadEntries() (map[string]AliasEntry, error)` 추가 — 기존 `Load()` 는 string-only flat map 반환 유지 | 신규 method | **YES** |
| A5.4 | Validation 시 deprecated 항목은 warn-log + 옵션에 따라 skip | A3 (ValidationPolicy) 와 결합 | **YES** (A3 흡수 또는 별도 REQ) |
| A5.5 | ContextWindow 음수 / 0 검증 | A3 흡수 | **YES** (A3 내) |

**권장 채택**: A5.1, A5.2, A5.3 (A5.4/A5.5 는 A3 와 결합).

**부모 surface 영향**: 0 (기존 `Load()` 보존, 신규 `LoadEntries()` 추가).
**HOTRELOAD-001 정합**: HOTRELOAD watcher 의 `Loader` interface 가 `LoadDefault() (map[string]string, error)` 만 요구 → 변경 없음. 신규 schema 는 호출자가 명시적으로 `LoadEntries()` 사용 시에만 노출.

**신규 REQ 추정**: 3건.

### Area 6 — Observability hooks (OBS-METRICS-001 연동)

**현 상태**:
- zap.Logger + log.Logger 만 emit.
- 카운터/히스토그램 metric emit 부재.
- HOTRELOAD-001 §10 #10 가 telemetry / metrics emission 을 후속 SPEC 위임 — 본 amendment 가 패키지 수준 hook 을 마련하면 후속 telemetry SPEC 통합 비용 ↓.

**발견 결손**:
- reload 카운트 (success / failure), validation error count by type, load duration histogram, entry count gauge — 운영 가시성 결손.

**개선 후보**:

| ID | 후보 | 영향 | 권장 |
|----|------|----|------|
| A6.1 | `Metrics` interface 옵션 — `IncLoadCount(success bool)`, `IncValidationError(code string)`, `RecordLoadDuration(d time.Duration)`, `ObserveEntryCount(n int)` | 신규 interface — Options.Metrics 필드 (default = noopMetrics) | **YES** |
| A6.2 | noop default 구현 — 외부 의존성 0건 보장 | 보조 type | **YES** |
| A6.3 | OBS-METRICS-001 SPEC 가 별도 존재 시 그 패키지 import — 아니면 noop 유지 | 의존성 결정 — v0.1.0 비대상 | **NO** (직접 import 회피) |

**권장 채택**: A6.1, A6.2.

**부모 surface 영향**: 0 (Options 신규 필드 추가, default 무시).
**HOTRELOAD-001 정합**: HOTRELOAD §10 #10 의 telemetry 위임 항목과 협력 가능 — 본 amendment 가 hook 을 마련하면 HOTRELOAD 의 watcher metrics 통합 비용 감소.

**신규 REQ 추정**: 1-2건.

## 5. 권장 우선순위 표

| 영역 | 후보 | 우선순위 | 신규 REQ | 신규 AC | HOTRELOAD-001 영향 |
|------|------|--------|--------|-------|------------------|
| Area 1 | A1.1 ConfigPath getter | **P1** (HOTRELOAD 직접 의존) | 1 | 1 | 강화 |
| Area 1 | A1.2 Reload() 명시화 | **P1** | 1 | 1 | 강화 |
| Area 2 | A2.1 user+project merge 실제 구현 | **P1** (부모 SPEC 본문 미구현 결손) | 1 | 2 | §10 #9 정합 |
| Area 2 | A2.2 MergePolicy enum | **P2** | 1 | 1 | 무영향 |
| Area 2 | A2.3 override info-log | **P2** | 1 | 1 | 무영향 |
| Area 3 | A3.1 ValidationPolicy | **P2** (Open Q #2 해소) | 1 | 1 | 무영향 (Validator interface 보존) |
| Area 3 | A3.2 ValidateAndPrune | **P2** | 1 | 2 | 무영향 |
| Area 4 | A4.3 ErrorCode 함수 | **P3** | 1 | 1 | 권장 (로그 분류 단순화) |
| Area 4 | A4.4 Categorize 함수 | **P3** | (A4.3 흡수) | (흡수) | 권장 |
| Area 5 | A5.1 AliasEntry struct | **P2** | 1 | 1 | 무영향 |
| Area 5 | A5.2 yaml union unmarshal | **P2** | 1 | 1 | 무영향 |
| Area 5 | A5.3 LoadEntries() method | **P2** | 1 | 1 | 무영향 |
| Area 6 | A6.1 Metrics interface | **P3** | 1 | 1 | 협력 강화 |
| Area 6 | A6.2 noop default | **P3** | (A6.1 흡수) | (흡수) | 무영향 |

**총 추정**:
- 신규 REQ: 12건 (P1: 3, P2: 7, P3: 2 — 일부 흡수 후 본 amendment 에서 12 REQ 채택 권장)
- 신규 AC: 14건

## 6. 부모 surface 보존 검증 체크리스트

본 amendment 의 모든 변경은 다음을 만족해야 한다:

- [ ] `Loader`, `Options`, `Logger`, `AliasConfig` 기존 필드 시그니처 보존
- [ ] `New(opts Options) *Loader` 시그니처 보존
- [ ] `(*Loader).Load() (map[string]string, error)` 시그니처 보존
- [ ] `(*Loader).LoadDefault() (map[string]string, error)` 시그니처 보존
- [ ] `Validate(m map[string]string, registry *router.ProviderRegistry, strict bool) []error` 시그니처 보존
- [ ] 기존 sentinel `ErrConfigNotFound` 보존
- [ ] 기존 5개 테스트 파일 PASS 유지 (회귀 금지)
- [ ] yaml schema 의 flat `aliases: {alias: string}` 형식 지원 보존
- [ ] HOTRELOAD-001 의 `Loader` / `Validator` interface 호환 유지

## 7. 코드 인용 (구현 vs SPEC 차이 발견)

### 7.1 user file vs project file: OR 분기 (SPEC 위반)

`internal/command/adapter/aliasconfig/loader.go:62-78`:

```go
func New(opts Options) *Loader {
    configPath := opts.ConfigPath
    if configPath == "" {
        // P3: Check project-local overlay first
        if projectLocalPath := detectProjectLocalAliasFile(); projectLocalPath != "" {
            configPath = projectLocalPath              // ← project 단독 우선
        } else if home := os.Getenv(homeEnv); home != "" {
            configPath = filepath.Join(home, "aliases.yaml")  // ← user fallback
        } else {
            // ...
        }
    }
    // ...
}
```

부모 spec.md §4.5 REQ-ALIAS-040 명세는 "user + project 둘 다 parse 후 merge with project override". 실제 구현은 단일 path 선택. 본 amendment Area 2 가 본문 구현을 채운다.

### 7.2 lenient 모드 in-place 삭제 미수행

`internal/command/adapter/aliasconfig/loader.go:192-236`:

```go
func Validate(aliasMap map[string]string, registry *router.ProviderRegistry, strict bool) []error {
    if aliasMap == nil { return nil }
    var errs []error
    for alias, canonical := range aliasMap {
        // ...
        if strict && registry != nil {
            // strict-only 검증
        }
    }
    return errs
}
```

→ map 자체에 대한 mutation 없음. lenient 모드에서 caller 가 errs 를 보고 직접 invalid 항목을 삭제해야 함. 본 amendment Area 3 가 `ValidateAndPrune` 으로 의미 명시화.

### 7.3 ConfigPath getter 부재

`Loader.configPath` 가 unexported. HOTRELOAD-001 watcher 가 fsnotify 대상 경로를 알려면 호출자가 `New` 시점 `Options.ConfigPath` 를 명시적으로 알고 있어야 함. 자동 resolve 결과 (fallback 적용 후 path) 는 외부 관찰 불가.

## 8. 결정된 amendment 범위 요약

본 amendment 가 채택할 변경:

1. **Area 1** (P1): `ConfigPath()` getter + `Reload()` 명시화
2. **Area 2** (P1): user + project merge 실제 구현 + MergePolicy 옵션 + override info-log
3. **Area 3** (P2): ValidationPolicy struct + ValidateAndPrune (in-place 삭제 명시)
4. **Area 4** (P3): ErrorCode + Categorize 안정 키 함수
5. **Area 5** (P2): AliasEntry struct + yaml union + LoadEntries() method
6. **Area 6** (P3): Metrics interface + noop default

## 9. Open Questions (본 amendment plan 단계에서 사용자 결정 필요)

1. **Area 2 backward-compat**: user-only 사용자가 amendment 후 project file 을 추가하면 동작 변화. release notes 필요 여부?
2. **Area 5 yaml schema**: `claude: anthropic/claude-opus-4-7` (string) 와 `claude: {canonical: anthropic/claude-opus-4-7, deprecated: false}` (map) 두 형식 동시 지원 시 yaml.v3 unmarshal 복잡도. 단순화를 위해 v0.1 에서는 union 미지원하고 별도 `aliases_v2:` top-level key 로 분리 옵션도 가능 — 사용자 결정.
3. **Area 6 Metrics interface**: prometheus / opentelemetry 어느 표준 collector 시그니처에 align 할지 — 현재 OBS-METRICS-001 SPEC 부재로 noop only 채택 권장.
4. **HOTRELOAD-001 머지 순서**: 본 amendment 가 HOTRELOAD-001 plan 보다 먼저 implementation 되어야 하는가 (Area 1 의 ConfigPath getter 가 HOTRELOAD watcher 에 즉시 사용됨)?

## 10. References

- 부모 SPEC: `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` (v0.1.0, FROZEN)
- 부모 progress: `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/progress.md`
- 후속 SPEC: `.moai/specs/SPEC-GOOSE-CMDCTX-HOTRELOAD-001/spec.md` (planned, P4)
- 부모 코드: `internal/command/adapter/aliasconfig/loader.go`
- 부모 테스트: `internal/command/adapter/aliasconfig/{loader_test.go, loader_p3_test.go, integration_test.go, validate_test.go}`
- Local convention: `CLAUDE.local.md §2.5` (코드 주석 영어)

---

Version: 0.1.0
Last Updated: 2026-04-30
Author: manager-spec
