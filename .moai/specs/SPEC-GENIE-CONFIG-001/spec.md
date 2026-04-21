---
id: SPEC-GENIE-CONFIG-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 소(S)
lifecycle: spec-anchored
---

# SPEC-GENIE-CONFIG-001 — 계층형 설정 로더 (Hierarchical Config Loader)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (ROADMAP Phase 0, CORE-001 확장) | manager-spec |

---

## 1. 개요 (Overview)

CORE-001이 정의한 **단일 파일 bootstrap config**를 확장하여, `genied`와 `genie` CLI가 공유하는 **계층형 설정 로더**를 정의한다. 로딩 우선순위는:

```
(낮음) defaults → project(YAML) → user(YAML) → runtime(env/flag) (높음)
```

본 SPEC은 `internal/config/` 패키지를 제공하며, 후속 SPEC들(TRANSPORT/LLM/AGENT/CLI)은 전부 `config.Load()`로 시작한다. 수락 조건 통과 시점에서 유저는 `~/.genie/config.yaml` 한 줄만 바꿔도 LLM provider, gRPC port, log level을 전부 바꿀 수 있어야 한다.

---

## 2. 배경 (Background)

### 2.1 왜 별도 SPEC인가

- CORE-001 §6.3은 `gopkg.in/yaml.v3`로 최소 파싱만 수행. 복수 설정 소스(project/user/runtime) 병합 로직은 범위 외로 지정됨.
- ROADMAP §4 Phase 0 row 02는 CONFIG-001을 `CORE-001` 의존 후속으로 명시하며, 근거 문서 `tech §11, structure §1`을 지목.
- `tech.md` §11.2는 `GENIE_HOME`, `GENIE_LOCALE`, `GENIE_LOG_LEVEL`, `OLLAMA_HOST`, `GENIE_LEARNING_ENABLED` 등 **환경변수 15+종**을 나열. 이들은 runtime 레이어에서 YAML 값을 override해야 한다.
- `.moai/project/structure.md` §1의 `.moai/config/sections/*.yaml` 패턴(user.yaml, language.yaml, workflow.yaml, quality.yaml)을 GENIE의 `~/.genie/config.yaml`로 대응시킨다.

### 2.2 viper 사용 여부

tech.md §3.1이 `spf13/viper` 1.19+를 후보로 명시. 그러나 viper는:

- 자동 타입 추론의 예측 불가성(특히 slice/nested struct)
- 테스트 시 전역 싱글턴(`viper.*` 패키지 함수) 격리 어려움
- Go 1.22+ 제네릭 + 명시적 `struct` 매핑만으로 계층 병합이 충분

따라서 본 SPEC은 **viper 미사용**. `yaml.v3`로 각 레이어를 개별 unmarshal한 뒤 **struct-level deep merge**를 자체 구현한다. 추후 MoAI-ADK-Go가 viper를 채택한 것이 확인되고 마이그레이션이 저렴하다고 판단되면 별도 SPEC으로 재평가.

### 2.3 범위 경계

- **IN**: 계층 정의 + 병합 알고리즘, 환경변수 오버레이, 스키마 검증, 감시 없이 로드-once, 테스트용 in-memory 로더.
- **OUT**: Hot reload/watch(파일 변경 감시), remote config server, secret manager 통합, JSON/TOML 형식, CLI 명령(`genie config get/set`) — CLI-001 또는 후속 SPEC.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/config/config.go`에 `type Config struct`와 `func Load(opts LoadOptions) (*Config, error)` 제공.
2. 3-계층 소스 병합: **project** (`$PWD/.genie/config.yaml` 선택적) → **user** (`$GENIE_HOME/config.yaml`) → **runtime** (env vars + CLI flags 예약, 본 SPEC은 env만).
3. `yaml.v3`로 각 레이어 unmarshal. 존재하지 않는 파일은 오류가 아니며 기본값 유지.
4. 환경변수 → `Config` 필드 매핑 표(§6.2).
5. 스키마 검증: 필수 필드 누락, 잘못된 enum 값(예: `log.level: "blah"`), 포트 범위(1~65535) 검증.
6. `Config.Validate() error`와 `Config.Source() map[string]string` 제공 (어느 필드가 어느 소스에서 왔는지 로그 대상).
7. "dry-run" API: `LoadFromMap(m map[string]any) (*Config, error)` — 테스트용.
8. 구조체는 **불변**(함수 호출 후 mutation 금지). 변경은 새 `Config` 반환.

### 3.2 OUT OF SCOPE

- Hot reload / fsnotify 감시 (후속 SPEC).
- Secret/credential 저장소 통합 (키체인, 1Password 등).
- JSON Schema export 또는 SDK 생성.
- CLI `genie config get/set` 하위명령 (CLI-001 범위).
- 프로바이더별 세부 설정 검증 — 각 SPEC이 자기 필드 검증 위임 (LLM-001이 `providers[*]` 검증).
- Windows 레지스트리 기반 설정.
- 암호화된 설정 파일.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-CFG-001 [Ubiquitous]** — The config loader **shall** expose an immutable `*Config` value where every field's concrete source (default, project, user, env) can be retrieved via `Config.Source(path string) string`.

**REQ-CFG-002 [Ubiquitous]** — All config file I/O **shall** occur through injectable `fs.FS` or `afero.Fs`-style abstraction so tests can stub without touching disk.

**REQ-CFG-003 [Ubiquitous]** — The `Config` struct **shall** be safe for concurrent reads after `Load()` returns (no internal locking needed; returned pointer is effectively frozen).

### 4.2 Event-Driven

**REQ-CFG-004 [Event-Driven]** — **When** `Load()` is invoked and `$GENIE_HOME/config.yaml` exists, the loader **shall** parse it as YAML and merge its fields over the defaults before applying runtime overlays.

**REQ-CFG-005 [Event-Driven]** — **When** `Load()` is invoked and a project-local `.genie/config.yaml` is present in `cwd`, the loader **shall** overlay it on top of the user-level config.

**REQ-CFG-006 [Event-Driven]** — **When** any environment variable from the documented override table (§6.2) is set, its value **shall** take precedence over all file-based sources for the mapped field.

### 4.3 State-Driven

**REQ-CFG-007 [State-Driven]** — **While** `Config.Validate()` has not been called, `Config.IsValid()` **shall** return false, and all public getters **shall not** be considered authoritative.

**REQ-CFG-008 [State-Driven]** — **While** the loader processes a YAML file, unknown top-level keys **shall** be preserved in `Config.Unknown map[string]any` and logged at WARN level but **shall not** cause load failure.

### 4.4 Unwanted Behavior

**REQ-CFG-009 [Unwanted]** — **If** a YAML file contains a syntax error, **then** `Load()` **shall** return a `*ConfigError` that includes file path, line number, and column; the function **shall not** silently fall back to defaults.

**REQ-CFG-010 [Unwanted]** — **If** a numeric field (e.g., `transport.grpc_port`) receives a string value in YAML, **then** `Load()` **shall** return a validation error naming the field path (`transport.grpc_port`) and the expected type.

**REQ-CFG-011 [Unwanted]** — The loader **shall not** read from `$HOME` when `$GENIE_HOME` is empty; it **shall** instead fall back to `$HOME/.genie` explicitly computed once per `Load()` call.

**REQ-CFG-012 [Unwanted]** — The loader **shall not** expand shell variables (`${FOO}`) inside YAML string values; literal strings are literal.

### 4.5 Optional

**REQ-CFG-013 [Optional]** — **Where** a caller supplies `LoadOptions.OverrideFiles []string`, the loader **shall** consume those paths instead of the default file lookup chain (test-only).

**REQ-CFG-014 [Optional]** — **Where** environment variable `GENIE_CONFIG_STRICT=true` is set, unknown top-level keys (REQ-CFG-008) **shall** instead cause load failure with an error listing all unknowns.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-CFG-001 — 계층 병합 순서**
- **Given** defaults `{log.level: "info", transport.grpc_port: 17891}`, user YAML `{log.level: "debug"}`, env `GENIE_GRPC_PORT=9999`
- **When** `Load()` 실행
- **Then** 결과는 `log.level="debug"` (user), `transport.grpc_port=9999` (env), 그 외는 default. `Source("log.level")=="user"`, `Source("transport.grpc_port")=="env"`.

**AC-CFG-002 — 파일 부재 시 기본값 유지**
- **Given** `$GENIE_HOME=t.TempDir()` (빈 디렉토리), 프로젝트 config 없음, env 없음
- **When** `Load()`
- **Then** 에러 없이 기본값만으로 `*Config` 반환, `Validate()==nil`.

**AC-CFG-003 — YAML 구문 오류 거부**
- **Given** `$GENIE_HOME/config.yaml` 내용이 `log:\n  level: [unclosed`
- **When** `Load()`
- **Then** `*ConfigError{File:"...", Line:2}` 반환, `errors.Is(err, ErrSyntax)==true`.

**AC-CFG-004 — 포트 범위 검증**
- **Given** user YAML `transport.grpc_port: 0`
- **When** `Load()` → `Validate()`
- **Then** `ErrInvalidField{Path:"transport.grpc_port", Msg:"must be 1..65535"}` 반환.

**AC-CFG-005 — 프로젝트 > 유저 오버라이드**
- **Given** user에 `llm.default_provider: "openai"`, project에 `llm.default_provider: "ollama"`
- **When** `Load()`
- **Then** `cfg.LLM.DefaultProvider == "ollama"`, `Source("llm.default_provider")=="project"`.

**AC-CFG-006 — Unknown 키 보존 (비-strict)**
- **Given** user YAML에 `future_feature: {x: 1}` 포함, `GENIE_CONFIG_STRICT` 미설정
- **When** `Load()`
- **Then** 에러 없음, `cfg.Unknown["future_feature"]` 존재, WARN 로그 1건.

**AC-CFG-007 — Strict 모드 거부**
- **Given** AC-CFG-006 상황 + `GENIE_CONFIG_STRICT=true`
- **When** `Load()`
- **Then** `ErrStrictUnknown{Keys: ["future_feature"]}` 반환.

**AC-CFG-008 — 환경변수 오버레이 단순 타입**
- **Given** env `GENIE_LOG_LEVEL=error`, `GENIE_LEARNING_ENABLED=false`
- **When** `Load()`
- **Then** `cfg.Log.Level=="error"`, `cfg.Learning.Enabled==false`.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/config/
├── config.go              # type Config struct + Load()
├── defaults.go            # func defaultConfig() *Config
├── merge.go               # struct-level deep merge (generic)
├── env.go                 # envOverlay(cfg *Config)
├── validate.go            # Config.Validate()
├── source.go              # 필드별 source 추적
├── errors.go              # ErrSyntax, ErrInvalidField, ErrStrictUnknown
└── *_test.go              # 파일 기반 + in-memory 테스트
```

### 6.2 환경변수 매핑 (초안)

| ENV | Config 경로 | 타입 | 기본값 |
|-----|-----------|-----|-------|
| `GENIE_HOME` | (특수: 검색 경로) | path | `~/.genie` |
| `GENIE_LOG_LEVEL` | `log.level` | enum{debug,info,warn,error} | info |
| `GENIE_HEALTH_PORT` | `transport.health_port` | int | 17890 (CORE-001) |
| `GENIE_GRPC_PORT` | `transport.grpc_port` | int | 17891 |
| `GENIE_LOCALE` | `ui.locale` | enum{en,ko,ja,zh} | en |
| `OLLAMA_HOST` | `llm.providers.ollama.host` | URL | `http://localhost:11434` |
| `OPENAI_API_KEY` | `llm.providers.openai.api_key` | secret | "" |
| `ANTHROPIC_API_KEY` | `llm.providers.anthropic.api_key` | secret | "" |
| `GENIE_LEARNING_ENABLED` | `learning.enabled` | bool | false (Phase 0 default) |
| `GENIE_CONFIG_STRICT` | (특수: loader 동작) | bool | false |

후속 SPEC들은 이 표에 ENV를 추가할 수 있으나, 동일 경로를 두 ENV가 매핑하는 경우는 금지(스키마 린트).

### 6.3 Config 스키마 초안

```go
type Config struct {
    Log       LogConfig       `yaml:"log"`
    Transport TransportConfig `yaml:"transport"`
    LLM       LLMConfig       `yaml:"llm"`
    Learning  LearningConfig  `yaml:"learning"`
    UI        UIConfig        `yaml:"ui"`
    Unknown   map[string]any  `yaml:",inline"`
}

type LogConfig struct {
    Level string `yaml:"level"` // debug|info|warn|error
}

type TransportConfig struct {
    HealthPort int `yaml:"health_port"`
    GRPCPort   int `yaml:"grpc_port"`
}

type LLMConfig struct {
    DefaultProvider string                     `yaml:"default_provider"`
    Providers       map[string]ProviderConfig  `yaml:"providers"`
}
```

각 후속 SPEC(LLM-001 등)이 자기 sub-struct 검증 함수를 export한다. `Config.Validate()`는 각 sub-struct `Validate()`를 차례로 호출.

### 6.4 Deep Merge 알고리즘

- 규칙 1: `map[string]any` nested — key-by-key recursive merge.
- 규칙 2: scalar — overlay가 zero-value가 **아니면** override.
- 규칙 3: slice — overlay가 비어있지 않으면 완전 대체 (append 없음).
- 규칙 4: pointer/struct — 필드별 규칙 1~3 재귀 적용.

이 규칙은 Go 제네릭 없이 `reflect` 기반 15~20줄로 구현 가능. 단위 테스트 10개로 각 규칙 격리 검증.

### 6.5 Source Tracking

```go
type sourceMap map[string]Source // "log.level" → SourceUser
type Source string // SourceDefault|SourceProject|SourceUser|SourceEnv|SourceOverride

func (c *Config) Source(path string) Source
```

병합 각 단계에서 "이 키가 non-zero였는가"를 기록. 구현은 flat map (필드 path를 dot-joined string으로).

### 6.6 TDD 진입 순서

1. **RED**: `TestLoad_DefaultsOnly_NoFiles_NoEnv` → AC-CFG-002.
2. **RED**: `TestLoad_UserYamlOverridesDefault` → AC-CFG-001.
3. **RED**: `TestLoad_EnvOverridesUser` → AC-CFG-001/008.
4. **RED**: `TestLoad_MalformedYaml_ReturnsSyntaxError` → AC-CFG-003.
5. **RED**: `TestValidate_InvalidPort_Returns Error` → AC-CFG-004.
6. ... (전체 AC 8개).
7. **GREEN**: `defaults.go`, `merge.go`, `env.go`, `validate.go` 최소 구현.
8. **REFACTOR**: `source tracking`을 `merge.go`에 통합, 중복 제거.

### 6.7 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| Tested | 테이블 주도 테스트, `testdata/*.yaml` 픽스처 20+개, merge 알고리즘 규칙별 커버 |
| Readable | struct 중심 설계, `config_test.go`에 YAML-Go 왕복 예시 |
| Unified | gofmt + golangci-lint(yaml struct tag 일관성), 테스트명은 `Test<Func>_<Given>_<Expect>` |
| Secured | API key는 `string`으로 받되 `Config.Redacted()` 메서드로 로그용 비식별화 반환 |
| Trackable | 모든 Load 실행 시 `source map`을 로그로 출력 (DEBUG 레벨) |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GENIE-CORE-001** | 동일 프로세스의 `zap` logger 인스턴스 공유, `GENIE_HOME` 의미 상속 |
| 후속 SPEC | SPEC-GENIE-TRANSPORT-001 | `TransportConfig` 소비 |
| 후속 SPEC | SPEC-GENIE-LLM-001 | `LLMConfig.Providers` 확장 + `Validate()` 구현 |
| 후속 SPEC | SPEC-GENIE-AGENT-001 | `LearningConfig` 플래그 소비 |
| 후속 SPEC | SPEC-GENIE-CLI-001 | `--config /path` 플래그가 `LoadOptions.OverrideFiles`에 주입 |
| 외부 | `gopkg.in/yaml.v3` | CORE-001에서 이미 채택 |
| 외부 | `github.com/stretchr/testify` | 테스트 assertion |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Deep merge 알고리즘의 edge case (nil vs empty slice) | 중 | 중 | 규칙 명문화 + 20+ 테이블 테스트 |
| R2 | Env overlay가 잘못된 타입 파싱(`int("abc")`) | 중 | 낮 | 파싱 실패를 개별 경고로 로그 + 기본값 유지 (validate에서 최종 거부 여부 결정) |
| R3 | `Config.Unknown`에 고의적으로 거대한 YAML 주입 시 메모리 이슈 | 낮 | 낮 | 파일 크기 256KB cap, 초과 시 `ErrTooLarge` |
| R4 | 기존 MoAI-ADK-Go가 나중에 viper로 정리될 경우 상이한 API | 중 | 중 | `config.Load()` 시그니처는 안정화, 내부 구현 교체 경로 확보 |
| R5 | 비밀값 로그 유출 (API key) | 높 | 높 | `Config.Redacted()`로 `"sk-***"` 치환 후 로그, 전체 cfg `.String()` 금지 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/tech.md` §3.1 (런타임), §11.2 (환경변수 목록)
- `.moai/project/structure.md` §1 (`.moai/config/sections/*.yaml` 패턴)
- `.moai/specs/SPEC-GENIE-CORE-001/spec.md` §3.1 IN SCOPE 3번 (`~/.genie/config.yaml` 최소 로더)

### 9.2 외부 참조

- `gopkg.in/yaml.v3` 문서 (strict mode: `decoder.KnownFields(true)`)
- MoAI-ADK-Go `internal/config/` (미러 없음, research.md 참조)
- `.moai/config/sections/*.yaml` 레이아웃 (본 프로젝트 메타)

### 9.3 부속 문서

- `./research.md` — 이식 가능 자산 분석 + 테스트 전략
- `../ROADMAP.md` §4 Phase 0 row 02

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **hot reload / fsnotify 감시를 구현하지 않는다**. `Load()`는 1회성.
- 본 SPEC은 **CLI `genie config get/set` 하위명령을 포함하지 않는다** (CLI-001).
- 본 SPEC은 **secret manager / keychain 통합을 포함하지 않는다**. API key는 평문 YAML 또는 env.
- 본 SPEC은 **JSON/TOML/HCL 형식을 지원하지 않는다**. YAML 전용.
- 본 SPEC은 **remote config (Consul/etcd)를 지원하지 않는다**.
- 본 SPEC은 **viper나 다른 설정 프레임워크를 도입하지 않는다**. 명시적 reject.
- 본 SPEC은 **프로바이더별 세부 검증을 수행하지 않는다**. LLM-001 등이 자기 영역 검증을 담당.
- 본 SPEC은 **Windows 레지스트리 / plist 기반 설정을 지원하지 않는다**.
- 본 SPEC은 **설정 암호화를 구현하지 않는다**. 파일 시스템 권한(0600)에 의존.

---

**End of SPEC-GENIE-CONFIG-001**
