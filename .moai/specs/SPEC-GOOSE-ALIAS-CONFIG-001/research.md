# SPEC-GOOSE-ALIAS-CONFIG-001 — Research

> Model alias config 파일 로드 — `~/.goose/aliases.yaml` (또는 등가 경로)에서 alias map을 파싱·검증하여 `command/adapter.ContextAdapter.Options.AliasMap` 에 주입하는 wiring layer를 도입한다.

작성: 2026-04-27
대상 SPEC: SPEC-GOOSE-ALIAS-CONFIG-001
의존 SPEC: SPEC-GOOSE-CMDCTX-001 (FROZEN, implemented 2026-04-27, PR #52, c018ec5 머지),
            SPEC-GOOSE-ROUTER-001 (FROZEN, implemented),
            SPEC-GOOSE-CONFIG-001 (FROZEN, implemented — `internal/config/` 계층형 로더)

---

## 1. 배경 (Why now)

### 1.1 현재 결손

`SPEC-GOOSE-CMDCTX-001` 의 `internal/command/adapter/adapter.go` 는 다음 surface를 노출한다:

```go
// internal/command/adapter/adapter.go (현재 main 기준, line 49-62)
type Options struct {
    Registry       *router.ProviderRegistry
    LoopController LoopController
    AliasMap       map[string]string  // ← REQ-CMDCTX-017
    GetwdFn        func() (string, error)
    Logger         Logger
}
```

`AliasMap` 은 옵션 필드로만 정의되어 있고, **실제 호출자(daemon bootstrap / CLI entrypoint)가 이 필드를 채우지 않으면 빈 맵으로 동작**한다.

CMDCTX-001 의 wiring SPEC 자체는 의도적으로 alias 데이터 소스를 정의하지 않았다 — 그것이 본 SPEC의 경계이다. 현재 상태에서:

- `/model opus` 같은 짧은 alias 입력은 registry SuggestedModels lookup으로 fallback (대부분 fail).
- 사용자가 매번 `/model anthropic/claude-opus-4-7` 처럼 canonical 형식을 외워야 한다.
- 같은 alias 정의를 여러 머신/세션 간 공유할 수단이 없다.

### 1.2 본 SPEC 의 책임

`~/.goose/aliases.yaml` (또는 GOOSE_HOME 기반 등가 경로)에서 alias 정의를 읽고, 그 결과 `map[string]string` 을 `adapter.Options.AliasMap` 에 주입하는 데까지가 본 SPEC. 그 이후 resolution 로직은 CMDCTX-001 의 `resolveAlias` 가 이미 구현했으므로 손대지 않는다.

### 1.3 명시적으로 본 SPEC이 다루지 않는 것

- **Hot-reload**: 파일 변경 watch → 런타임 AliasMap 교체. 별도 SPEC (SPEC-GOOSE-HOTRELOAD-001, 작성 중)이 다룬다.
- **Dynamic alias 등록**: `/alias add opus anthropic/claude-opus-4-7` 같은 in-process command. 별도 검토.
- **Alias namespace / group**: `aliases.work.opus` 처럼 컨텍스트별 그룹화. 향후 SPEC.
- **Resolution 알고리즘 자체**: CMDCTX-001 §6.4 / `resolveAlias` 함수 — FROZEN, 본 SPEC은 입력 데이터만 공급한다.

---

## 2. 기존 인프라 조사 (Reuse opportunities)

### 2.1 `internal/config/` 패키지 (SPEC-GOOSE-CONFIG-001)

`internal/config/config.go` 는 이미 다음 인프라를 제공한다:

| 자산 | 시그니처 / 위치 | 본 SPEC에서의 활용 |
|------|----------------|-------------------|
| `LoadOptions.GooseHome` | config.go:147 | GOOSE_HOME 환경변수 / `$HOME/.goose` fallback 로직 — 그대로 차용 가능 |
| `resolveGooseHome()` | config.go:258 | 동일 fallback (`GOOSE_HOME` → `$HOME/.goose`) — 본 SPEC 도 이 헬퍼 또는 동일 의미론을 따른다 |
| `mergeYAMLFile(fsys, path, ..., logger)` | config.go (private) | YAML 파싱 패턴 참조용 — 본 SPEC은 별도 단순 yaml.Unmarshal 로 충분 (alias 스키마가 단순) |
| `LoadOptions.FS fs.FS` | config.go:140 | testability 패턴 — 본 SPEC 의 `Loader` 도 동일하게 `fs.FS` 주입 허용 |
| `LoadOptions.EnvOverrides` | config.go:153 | 테스트 격리 — 본 SPEC 도 동일 패턴 채택 |
| 의존성 | `gopkg.in/yaml.v3`, `go.uber.org/zap` | 본 SPEC 도 동일 의존성 재사용 (신규 의존성 도입 금지) |

**결정 1**: `Config` struct 자체에 `Aliases` 필드를 박아넣는 방식은 **거부**.

근거:
- `internal/config/config.go` 의 `Config` 는 LLM/Transport/Log/UI 의 무거운 contract를 담고 있고, alias map은 별도 파일에 살림.
- `LoadOptions.OverrideFiles` 는 단일 파일 경로 체인이라 alias 파일을 별도 위치로 배치하기 어렵다.
- alias hot-reload(미래) 시 config.Load() 전체를 재로드할 필요가 없다 — 격리된 loader가 변경 surface 를 작게 유지한다.

**결정 2**: 별도 패키지 `internal/command/adapter/aliasconfig/` 로 분리.

근거:
- alias map은 `command/adapter` 만의 consumer를 가지며 (router/llm/transport 는 사용하지 않음), `internal/command/adapter/aliasconfig/` 가 fan_in 단일 boundary 를 명확화.
- 또 다른 후보 `internal/config/aliasconfig/` 는 config 패키지가 LLM/Transport 외 alias라는 별도 concern을 갖게 만들어 `Config` 응집도를 흐림.
- daemon bootstrap (`cmd/goosed`) 에서 `aliasconfig.Load()` → `adapter.New(Options{ AliasMap: ... })` 으로 직접 wiring 하는 흐름이 자연스럽다.

→ **권장 패키지 경로**: `internal/command/adapter/aliasconfig/`

### 2.2 `internal/llm/router/registry.go` (validation 측)

`router.ProviderRegistry` 는 `Get(name) (*ProviderMeta, bool)` 와 `meta.SuggestedModels` 를 노출한다. Alias 대상이 `provider/model` 형식이라면 다음 검증이 가능하다:

```go
// pseudo-code
parts := strings.SplitN(canonical, "/", 2)
meta, ok := registry.Get(parts[0])
if !ok { return ErrUnknownProvider }
if !slices.Contains(meta.SuggestedModels, parts[1]) {
    return ErrUnknownModel
}
```

이는 사실상 `resolveAlias` (SPEC-CMDCTX-001 §6.4) 의 step 5-6 와 동일한 코드다.

**결정 3**: 본 SPEC 의 `Validate(aliasMap, registry)` 는 **선택 (optional, 권장 default-on)** 으로 제공.

근거:
- 강한 검증(canonical이 registry에 없으면 alias 전체 reject)은 deploy 순서 강결합 — registry plugin 이 늦게 등록되는 future scenario에서 false-fail 위험.
- 따라서 validation은 두 모드:
  - **strict** (default): canonical 미등록 시 `Load` 가 error 반환 → 부팅 실패 (fail-fast).
  - **lenient** (env / opt-in): warn log + 해당 entry skip → 부팅 계속.
- 모드 선택은 env `GOOSE_ALIAS_STRICT` 또는 `Loader.Strict` 옵션 필드로 제공.

### 2.3 hot-reload 인프라 부재

현재 main에는 `fsnotify` 류 watcher 가 없다. SPEC-GOOSE-HOTRELOAD-001 (작성 중)이 그 인프라를 별도 도입한다. **본 SPEC은 hot-reload 비대응**:

- `Load(path)` 는 1회 호출 시점의 파일 스냅샷만 반환.
- 호출자가 파일을 다시 읽으려면 `Load` 를 재호출해야 한다.
- 즉 daemon 부팅 시 1회 호출 후 process lifetime 동안 동결.

---

## 3. 스키마 결정 (YAML shape)

### 3.1 후보 비교

#### 후보 A: 단순 flat map (권장)

```yaml
# ~/.goose/aliases.yaml
aliases:
  opus: anthropic/claude-opus-4-7
  sonnet: anthropic/claude-sonnet-4-6
  gpt4o: openai/gpt-4o
  haiku: anthropic/claude-haiku-4-5
```

**장점**:
- `map[string]string` 직접 매핑 — adapter.Options.AliasMap 과 1:1.
- yaml.Unmarshal → `struct { Aliases map[string]string }` 한 줄.
- 사용자 학습 비용 0.

**단점**:
- 동일 alias 다른 컨텍스트(개인/팀/실험) 분리 불가 — 후속 SPEC(group/namespace)이 필요하면 스키마 v2 마이그레이션.

#### 후보 B: nested provider group

```yaml
aliases:
  anthropic:
    opus: claude-opus-4-7
    sonnet: claude-sonnet-4-6
  openai:
    gpt4o: gpt-4o
```

**장점**: provider 별 정렬, alias 충돌(`opus`가 두 provider에 동시 존재) 문법 차단.
**단점**:
- adapter.AliasMap 은 flat `map[string]string` 이므로 loader가 `provider/alias → provider/model` 로 합성해야 함 — 추가 복잡도.
- 사용자가 `/model opus` 라 입력했을 때 어느 provider 로 resolve할지 추가 규칙 필요.
- alias가 짧은 nick(`opus`)이라는 사용성과 충돌.

→ **결정 4**: **후보 A (flat map)** 채택. v0.1 단순성 우선. group/namespace는 후속 SPEC.

### 3.2 최종 스키마

```yaml
# ~/.goose/aliases.yaml
# SPEC-GOOSE-ALIAS-CONFIG-001
#
# Each entry maps a short alias to a canonical "provider/model" string.
# The canonical value MUST exist in router.ProviderRegistry SuggestedModels
# unless GOOSE_ALIAS_STRICT=false is set.

aliases:
  opus: anthropic/claude-opus-4-7
  sonnet: anthropic/claude-sonnet-4-6
  haiku: anthropic/claude-haiku-4-5
  gpt4o: openai/gpt-4o
```

**Top-level key**: `aliases:` (단일 — 다른 키는 unknown 으로 무시 또는 strict 모드에서 reject).

---

## 4. Validation 전략

### 4.1 검증 단계

1. **Syntactic**: yaml.Unmarshal 성공 여부. 실패 시 `ErrMalformedAliasFile` (라인/컬럼 포함).
2. **Empty key/value**: alias 또는 canonical이 빈 문자열이면 reject (`ErrEmptyAliasEntry`).
3. **Canonical format**: canonical에 `/` 가 정확히 1번 등장 (provider/model). 위반 시 `ErrInvalidCanonical`.
4. **Registry lookup (strict mode)**: `parts[0]` provider 가 registry 에 등록되었는지, `parts[1]` model 이 `meta.SuggestedModels` 에 있는지. 미등록 시 strict=true → fail, strict=false → warn+skip.
5. **Cycle detection**: alias → canonical → ... → alias 가 환을 이루면 reject.

### 4.2 Cycle detection 알고리즘

스키마 A (flat) 에서도 사용자가 실수로 alias 값에 다른 alias 를 적을 수 있다:

```yaml
aliases:
  a: b              # canonical 형식 위반 (slash 없음) → step 3 에서 즉시 reject
  opus: sonnet      # canonical 형식 위반 → reject
```

본 스키마(canonical은 반드시 `provider/model`)에서는 **alias값이 다른 alias의 키와 같을 수 없다** (canonical은 slash를 포함해야 하므로). 따라서 step 3이 통과되면 cycle은 구조적으로 불가능.

다만 미래 확장에서 alias chaining (`opus → my-opus → anthropic/claude-opus-4-7`)을 허용한다면 DFS visited-set 알고리즘이 필요. 본 SPEC은 chaining 비허용 — single-hop only. cycle detection은 정의상 zero-cost (canonical에 `/` 강제로 충분).

→ **결정 5**: chaining 비허용. Validation step 3 (canonical에 `/` 강제)이 cycle 을 구조적으로 차단.

### 4.3 Validation 실행 시점

- `Loader.Load(path)` 내부에서 step 1-3 (file-only validation) 수행.
- `Loader.Validate(map, registry)` 별도 메서드로 step 4 (registry-coupled validation) 수행.
- 호출자(daemon bootstrap)가 둘 모두 실행하는 것이 권장 — 다음 절 wiring 참조.

---

## 5. Filesystem layout 결정

### 5.1 경로 후보

| 후보 | 예시 | 장단점 |
|-----|------|-------|
| A. GOOSE_HOME 기반 | `$GOOSE_HOME/aliases.yaml` (default `$HOME/.goose/aliases.yaml`) | CONFIG-001 과 일관 / 구현 단순. 단점: XDG_CONFIG_HOME 미준수. |
| B. XDG 호환 | `$XDG_CONFIG_HOME/goose/aliases.yaml` (default `$HOME/.config/goose/aliases.yaml`) | Linux 표준 준수. 단점: macOS 사용자가 `~/.config` 를 안 씀, GOOSE_HOME 과 분리되어 일관성 결여. |
| C. 둘 다 | A → B 순서로 fallback | 복잡, 둘 중 어느 파일이 우선인지 사용자 혼란. |

→ **결정 6**: **후보 A 채택**.

근거:
- SPEC-GOOSE-CONFIG-001 이 이미 GOOSE_HOME 컨벤션을 확립 (`config.go:258` `resolveGooseHome`).
- alias 파일이 다른 컨벤션을 따르면 사용자가 `config.yaml`은 `~/.goose/`에, `aliases.yaml`은 `~/.config/goose/`에 두게 되어 인지 부담 증가.
- XDG 마이그레이션은 별도 SPEC (`SPEC-GOOSE-XDG-MIGRATION-001`)에서 `~/.goose` 와 `~/.config/goose` 양쪽을 함께 처리.

### 5.2 환경 override

- `GOOSE_ALIAS_FILE` (env) 가 설정되면 그 절대 경로를 사용 (테스트 / 비표준 배포 / 멀티테넌트).
- 우선순위: `GOOSE_ALIAS_FILE` > `$GOOSE_HOME/aliases.yaml` > `$HOME/.goose/aliases.yaml`.
- 두 후보 모두 부재 시 → 빈 맵 반환 (graceful default).

### 5.3 Project-local overlay (Optional)

CONFIG-001 은 `cwd/.goose/config.yaml` 도 로드한다 (project overlay). 동일 패턴을:

- `$CWD/.goose/aliases.yaml` 이 존재하면 user-level과 merge.
- Merge 규칙: project alias 가 user alias 를 override (CONFIG-001 의 priority order: project > user).
- 충돌(같은 key 양쪽 정의) 시: project 가 user를 silent override + Logger.Info 로 기록.

→ **결정 7**: project-local overlay **포함** (Optional REQ). v0.1 가치는 작지만 인프라 비용도 작아 동시 도입.

---

## 6. 패키지 / 모듈 결정

### 6.1 신규 패키지

**경로**: `internal/command/adapter/aliasconfig/`

**파일**:
- `loader.go` — `Loader` struct, `Load(path) (map[string]string, error)`, `LoadDefault() (map[string]string, error)` (위 5.1 의 fallback 체인).
- `validate.go` — `Validate(m map[string]string, registry *router.ProviderRegistry, strict bool) []error`.
- `errors.go` — `ErrMalformedAliasFile`, `ErrEmptyAliasEntry`, `ErrInvalidCanonical`, `ErrUnknownProviderInAlias`, `ErrUnknownModelInAlias`.
- `loader_test.go`, `validate_test.go` — 테이블 드리븐 테스트.

### 6.2 의존성

- `gopkg.in/yaml.v3` (이미 main에 존재 — `internal/config/` 사용 중)
- `go.uber.org/zap` (이미 존재)
- `io/fs` (테스트용 fs.FS 주입)
- `github.com/modu-ai/goose/internal/llm/router` (validation 시 registry 참조 — read-only)

신규 외부 의존성 0건.

### 6.3 Wiring 위치

`cmd/goosed/main.go` (또는 등가 부트스트랩)에서:

```go
// pseudo-code
aliasMap, err := aliasconfig.LoadDefault(aliasconfig.Options{
    Logger: logger,
    GooseHome: cfg.GooseHome(),  // SPEC-GOOSE-CONFIG-001 cfg
})
if err != nil { /* handle */ }

if err := aliasconfig.Validate(aliasMap, registry, /*strict=*/true); err != nil {
    /* handle */
}

ctxAdapter := adapter.New(adapter.Options{
    Registry:       registry,
    LoopController: loopCtrl,
    AliasMap:       aliasMap,
    Logger:         logger,
})
```

본 SPEC은 daemon main 의 wiring 코드 변경 자체는 **포함**한다 (그렇지 않으면 새 loader가 dead code).

---

## 7. 위험 (Risks)

| # | 위험 | 영향 | 대응 |
|---|-----|------|-----|
| R-1 | hot-reload 별도 SPEC 분리 → 사용자가 alias 추가 후 daemon 재시작 필요 | UX 마찰 | SPEC-HOTRELOAD-001 으로 위임. 본 SPEC `Limitations` 절에 명시. |
| R-2 | strict 모드 false-fail (registry 에 등록 안 된 third-party plugin model) | 부팅 실패 | env `GOOSE_ALIAS_STRICT=false` lenient 모드 제공. 기본은 strict (안전 default). |
| R-3 | `~/.goose/aliases.yaml` 권한 노출 (다른 사용자 read) | secret 유출 | alias map 은 secret 아님 (provider/model 식별자뿐) — 위험도 낮음. 다만 chmod 600 권장 주석을 file 헤더 sample 에 포함. |
| R-4 | XDG 비준수 결정으로 Linux 사용자 항의 가능 | 커뮤니티 마찰 | SPEC-GOOSE-XDG-MIGRATION-001 으로 계획됨 명시. 본 SPEC 의 `GOOSE_ALIAS_FILE` env 로 즉시 우회 가능. |
| R-5 | project overlay (`$CWD/.goose/aliases.yaml`) 가 git-commit되어 팀원 간 alias 충돌 | 협업 마찰 | project overlay는 OPTIONAL feature. 사용자 책임. CONFIG-001 도 같은 모델. |
| R-6 | Validate 결과를 호출자가 무시 (loader 가 검증을 강제하지 않음) | 잘못된 alias가 런타임에 fail-late | API 설계: `Validate` 가 errors 배열 반환 — 호출자가 zero-len 체크 필요. doc comment 강조. |
| R-7 | yaml.v3 가 unknown key 를 silent-ignore (현재 동작) → 사용자 오타 디버깅 어려움 | UX | strict mode 에서 unknown top-level key 발견 시 warn log. CONFIG-001 의 `StrictUnknownError` 패턴 참조 가능. |
| R-8 | Loader 가 `gopkg.in/yaml.v3` 의 큰 파일에서 메모리 폭발 (수만 entry) | DoS | 파일 사이즈 cap 1MB (CONFIG-001 패턴 동일). 초과 시 reject. |

---

## 8. 결정 요약

| 결정 # | 내용 |
|------|------|
| 1 | `Config` struct 에 alias 필드 박지 않고 별도 모듈 분리 |
| 2 | 패키지 경로 `internal/command/adapter/aliasconfig/` |
| 3 | Validation strict (기본 true) / lenient (env opt-in) 양 모드 제공 |
| 4 | 스키마 후보 A (flat `aliases:` map) 채택 |
| 5 | Alias chaining 비허용 — canonical 은 `provider/model` 강제로 cycle 구조적 차단 |
| 6 | 파일 경로 `$GOOSE_HOME/aliases.yaml` (XDG 비준수, CONFIG-001 일관) |
| 7 | Project-local overlay (`$CWD/.goose/aliases.yaml`) Optional 포함 |
| 8 | `GOOSE_ALIAS_FILE` env override 로 절대 경로 지정 가능 |
| 9 | 신규 외부 의존성 0건 (yaml.v3, zap, io/fs 모두 기존) |
| 10 | hot-reload 비대응 — SPEC-HOTRELOAD-001 으로 위임 |

---

## 9. SPEC 작성 시 반영 항목

다음 절은 spec.md 에서 정식 REQ/AC 로 변환된다:

- **REQ Ubiquitous**: 파일 부재 시 빈 맵 반환 (graceful default), Loader 는 nil 의존성에 panic 없음.
- **REQ Event-Driven**: 파일 존재 + 파싱 성공 → `map[string]string` 반환 → adapter.Options.AliasMap 로 주입.
- **REQ State-Driven**: `GOOSE_ALIAS_FILE` env 설정 시 그 경로 우선, 미설정 시 GOOSE_HOME chain.
- **REQ Unwanted**: malformed yaml / 빈 alias / canonical에 slash 없음 / strict 모드에서 registry 미등록 → 명시적 error.
- **REQ Optional**: project-local overlay (`$CWD/.goose/aliases.yaml`) 지원.
- **AC**: Loader unit test (테이블 드리븐), Validate unit test, 빈 파일 / 부재 파일 graceful, env override 동작, project overlay merge 우선순위, 신규 외부 의존성 0건 정적 검증.

---

작성 완료 — spec.md 작성으로 진행.
