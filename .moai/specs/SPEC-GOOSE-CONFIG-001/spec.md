---
id: SPEC-GOOSE-CONFIG-001
version: 0.3.1
status: implemented
created_at: 2026-04-21
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 소(S)
lifecycle: spec-anchored
labels: [phase-0, area/config, area/runtime, type/feature, priority/p0-critical]
---

# SPEC-GOOSE-CONFIG-001 — 계층형 설정 로더 (Hierarchical Config Loader)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (ROADMAP Phase 0, CORE-001 확장) | manager-spec |
| 0.2.0 | 2026-04-25 | 감사 결함 수정 (plan-audit mass-20260425). MP-2 FAIL 해소: REQ-CFG-011/012 Unwanted-EARS 재작성 (If/then). D18 critical 해소: §6.4 deep merge zero-value 버그 명시 + REQ-CFG-015 추가 + AC-CFG-009 추가. D15 major 해소: REQ-CFG-016 secret/CREDPOOL 연동 forward-reference 추가 + §3.2 OUT-OF-SCOPE 확장. D17 major 해소: AC-CFG-010 (int env overlay) + AC-CFG-011 (URL env overlay) + AC-CFG-012 (secret env overlay) 추가. D20 관련: §3.2 OUT-OF-SCOPE에 JSON Schema 미채택 명시. frontmatter `labels`, `created_at`는 Phase B에서 이미 수정됨 (검증만). | manager-spec |
| 0.3.0 | 2026-04-25 | iter 3 감사 결함 수정 (CONFIG-001-review-2). **D22 critical 해소**: AC-CFG-001~008에 `Satisfies: REQ-CFG-XXX` 줄 추가 — AC 블록 traceability 100% 확보. **D23~D29 major (stagnation) 해소**: 7개 미커버 REQ(002/003/007/010/011/012/013)에 대응하는 AC-CFG-013~019 신설. **D33 major 해소**: `Config.Redacted()` 계약을 REQ-CFG-017 [Ubiquitous]로 승격, AC-CFG-012의 참조 REQ를 REQ-CFG-017로 교정. **D34 major 해소**: AC-CFG-010을 AC-CFG-010a (happy-path, `Satisfies: REQ-CFG-006`)과 AC-CFG-010b (env parse-failure fallback, `Satisfies: REQ-CFG-006` + R2 리스크 완화)로 분리. REQ 번호 재배치 없음 (015/016은 4.6 유지, 017은 새 4.7로 신설). | manager-spec |
| 0.3.1 | 2026-04-25 | Implementation 완료를 frontmatter에 반영 (PR #20, commit da8d7f1). status: planned → implemented. labels 보강. 코드: `internal/config/{config,defaults,merge(inline),env,validate(via config),source,errors,redact(via config)}.go` + 41 테스트. AC-CFG-001~019 모두 GREEN, race detector clean, coverage 85.8%, 회귀 0건. 기존 `bootstrap_config.go` 흡수 + `cmd/goosed/main.go` 마이그레이션 완료. ShutdownTimeout은 30s 상수로 환원 (CORE-001 §11 R3 부합). SPEC 본문 변경 없음 — 문서 정합화 전용 엔트리. | manager-tdd |

---

## 1. 개요 (Overview)

CORE-001이 정의한 **단일 파일 bootstrap config**를 확장하여, `goosed`와 `goose` CLI가 공유하는 **계층형 설정 로더**를 정의한다. 로딩 우선순위는:

```
(낮음) defaults → project(YAML) → user(YAML) → runtime(env/flag) (높음)
```

본 SPEC은 `internal/config/` 패키지를 제공하며, 후속 SPEC들(TRANSPORT/LLM/AGENT/CLI)은 전부 `config.Load()`로 시작한다. 수락 조건 통과 시점에서 유저는 `~/.goose/config.yaml` 한 줄만 바꿔도 LLM provider, gRPC port, log level을 전부 바꿀 수 있어야 한다.

---

## 2. 배경 (Background)

### 2.1 왜 별도 SPEC인가

- CORE-001 §6.3은 `gopkg.in/yaml.v3`로 최소 파싱만 수행. 복수 설정 소스(project/user/runtime) 병합 로직은 범위 외로 지정됨.
- ROADMAP §4 Phase 0 row 02는 CONFIG-001을 `CORE-001` 의존 후속으로 명시하며, 근거 문서 `tech §11, structure §1`을 지목.
- `tech.md` §11.2는 `GOOSE_HOME`, `GOOSE_LOCALE`, `GOOSE_LOG_LEVEL`, `OLLAMA_HOST`, `GOOSE_LEARNING_ENABLED` 등 **환경변수 15+종**을 나열. 이들은 runtime 레이어에서 YAML 값을 override해야 한다.
- `.moai/project/structure.md` §1의 `.moai/config/sections/*.yaml` 패턴(user.yaml, language.yaml, workflow.yaml, quality.yaml)을 GOOSE의 `~/.goose/config.yaml`로 대응시킨다.

### 2.2 viper 사용 여부

tech.md §3.1이 `spf13/viper` 1.19+를 후보로 명시. 그러나 viper는:

- 자동 타입 추론의 예측 불가성(특히 slice/nested struct)
- 테스트 시 전역 싱글턴(`viper.*` 패키지 함수) 격리 어려움
- Go 1.22+ 제네릭 + 명시적 `struct` 매핑만으로 계층 병합이 충분

따라서 본 SPEC은 **viper 미사용**. `yaml.v3`로 각 레이어를 개별 unmarshal한 뒤 **struct-level deep merge**를 자체 구현한다. 추후 MoAI-ADK-Go가 viper를 채택한 것이 확인되고 마이그레이션이 저렴하다고 판단되면 별도 SPEC으로 재평가.

### 2.3 범위 경계

- **IN**: 계층 정의 + 병합 알고리즘, 환경변수 오버레이, 스키마 검증, 감시 없이 로드-once, 테스트용 in-memory 로더.
- **OUT**: Hot reload/watch(파일 변경 감시), remote config server, secret manager 통합, JSON/TOML 형식, CLI 명령(`goose config get/set`) — CLI-001 또는 후속 SPEC.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/config/config.go`에 `type Config struct`와 `func Load(opts LoadOptions) (*Config, error)` 제공.
2. 3-계층 소스 병합: **project** (`$PWD/.goose/config.yaml` 선택적) → **user** (`$GOOSE_HOME/config.yaml`) → **runtime** (env vars + CLI flags 예약, 본 SPEC은 env만).
3. `yaml.v3`로 각 레이어 unmarshal. 존재하지 않는 파일은 오류가 아니며 기본값 유지.
4. 환경변수 → `Config` 필드 매핑 표(§6.2).
5. 스키마 검증: 필수 필드 누락, 잘못된 enum 값(예: `log.level: "blah"`), 포트 범위(1~65535) 검증.
6. `Config.Validate() error`와 `Config.Source() map[string]string` 제공 (어느 필드가 어느 소스에서 왔는지 로그 대상).
7. "dry-run" API: `LoadFromMap(m map[string]any) (*Config, error)` — 테스트용.
8. 구조체는 **불변**(함수 호출 후 mutation 금지). 변경은 새 `Config` 반환.

### 3.2 OUT OF SCOPE

- Hot reload / fsnotify 감시 (후속 SPEC).
- Secret/credential 저장소 통합 (키체인, 1Password, OS keychain 등). 본 SPEC은 secret-typed 필드(`llm.providers.*.api_key`)를 env/YAML 평문으로만 다룬다. **단, REQ-CFG-016이 향후 `SPEC-GOOSE-CREDPOOL-XXX` (credential-pool SPEC)와의 연동 훅을 forward-reference로 제공한다.** 해당 SPEC이 ROADMAP 상위 Phase에서 채택되면, secret-typed 필드는 자동으로 credential-pool resolver 경유로 전환된다. 본 SPEC은 그 전환 시점까지 평문 경로를 유지할 뿐, credential pool 자체의 구현·저장 형식·암호화 방식에는 개입하지 않는다.
- **JSON Schema export / 생성 / 런타임 검증 (명시적 미채택)**: 본 SPEC은 Config 스키마를 **Go 타입 (§6.3)**으로만 정의한다. JSON Schema 파일(`config.schema.json` 등)은 생성하지 않으며, `encoding/json` 기반 스키마 검증, IDE autocomplete용 schema export, OpenAPI 3.x 스키마 매핑도 수행하지 않는다. 이유: (1) 본 SPEC의 validation은 Go 수준의 `Config.Validate()` 메서드로 충분하고, (2) YAML-first 경로에서 JSON Schema는 중복 유지보수 부담이 되며, (3) 향후 필요 시 별도 SPEC(예: `SPEC-GOOSE-CONFIG-SCHEMA-XXX`)로 분리 가능하다.
- CLI `goose config get/set` 하위명령 (CLI-001 범위).
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

**REQ-CFG-004 [Event-Driven]** — **When** `Load()` is invoked and `$GOOSE_HOME/config.yaml` exists, the loader **shall** parse it as YAML and merge its fields over the defaults before applying runtime overlays.

**REQ-CFG-005 [Event-Driven]** — **When** `Load()` is invoked and a project-local `.goose/config.yaml` is present in `cwd`, the loader **shall** overlay it on top of the user-level config.

**REQ-CFG-006 [Event-Driven]** — **When** any environment variable from the documented override table (§6.2) is set, its value **shall** take precedence over all file-based sources for the mapped field.

### 4.3 State-Driven

**REQ-CFG-007 [State-Driven]** — **While** `Config.Validate()` has not been called, `Config.IsValid()` **shall** return false, and all public getters **shall not** be considered authoritative.

**REQ-CFG-008 [State-Driven]** — **While** the loader processes a YAML file, unknown top-level keys **shall** be preserved in `Config.Unknown map[string]any` and logged at WARN level but **shall not** cause load failure.

### 4.4 Unwanted Behavior

**REQ-CFG-009 [Unwanted]** — **If** a YAML file contains a syntax error, **then** `Load()` **shall** return a `*ConfigError` that includes file path, line number, and column; the function **shall not** silently fall back to defaults.

**REQ-CFG-010 [Unwanted]** — **If** a numeric field (e.g., `transport.grpc_port`) receives a string value in YAML, **then** `Load()` **shall** return a validation error naming the field path (`transport.grpc_port`) and the expected type.

**REQ-CFG-011 [Unwanted]** — **If** `$GOOSE_HOME` is empty or unset at the time `Load()` is invoked, **then** the loader **shall** resolve the user config directory to `$HOME/.goose` exactly once per `Load()` call and **shall not** dereference `$HOME` at any other location in the lookup chain.

**REQ-CFG-012 [Unwanted]** — **If** a YAML string value contains shell-variable syntax (e.g. `${FOO}`, `$BAR`), **then** the loader **shall** treat the entire value as a literal string and **shall not** perform shell-variable expansion, environment substitution, or command substitution on it.

### 4.5 Optional

**REQ-CFG-013 [Optional]** — **Where** a caller supplies `LoadOptions.OverrideFiles []string`, the loader **shall** consume those paths instead of the default file lookup chain (test-only).

**REQ-CFG-014 [Optional]** — **Where** environment variable `GOOSE_CONFIG_STRICT=true` is set, unknown top-level keys (REQ-CFG-008) **shall** instead cause load failure with an error listing all unknowns.

### 4.6 Addenda (0.2.0 감사 수정)

**REQ-CFG-015 [Ubiquitous]** — The deep-merge algorithm (§6.4) **shall** distinguish "field absent from overlay YAML" from "field present with Go zero-value (false/0/empty-string/empty-slice)". **When** an overlay explicitly declares a key with a zero-value, the loader **shall** treat that zero-value as the user's authoritative choice and **shall** override the lower-layer value; **when** an overlay omits the key entirely, the loader **shall** preserve the lower-layer value. This presence-aware semantic applies to all scalar and slice fields including `learning.enabled`, `log.level`, and `transport.grpc_port`.

**REQ-CFG-016 [Optional]** — **Where** a credential-pool SPEC (e.g., a future `SPEC-GOOSE-CREDPOOL-XXX`) is adopted in a later ROADMAP phase, secret-typed fields (currently `llm.providers.*.api_key` per §6.2) **shall** be sourced from that credential-pool resolver in preference to env vars and YAML plaintext. Until such SPEC lands, this SPEC keeps secret fields in env/YAML as documented in §3.2 OUT OF SCOPE. No runtime behavior change is mandated by REQ-CFG-016 in Phase 0; this REQ exists as an explicit **forward-reference hook** so downstream SPECs can cite it.

### 4.7 Addenda (0.3.0 감사 수정)

**REQ-CFG-017 [Ubiquitous]** — The loader **shall** expose a `Config.Redacted() string` method that returns a human-readable snapshot of the Config where every secret-typed field (i.e., every field mapped as `secret` in §6.2, currently `llm.providers.*.api_key` including but not limited to `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`) is replaced by a constant-length mask of the form `sk-***` (8-character ASCII with `sk-` prefix and three asterisks). The original secret value **shall** remain accessible via the typed getter (e.g., `cfg.LLM.Providers["openai"].APIKey`) in memory; only the `Redacted()` output **shall** be masked. The method **shall not** panic on nil or empty secrets (empty string returns empty string). The mask constant **shall not** vary by secret length so the output length does not leak the original secret length.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-CFG-001 — 계층 병합 순서**
- **Given** defaults `{log.level: "info", transport.grpc_port: 17891}`, user YAML `{log.level: "debug"}`, env `GOOSE_GRPC_PORT=9999`
- **When** `Load()` 실행
- **Then** 결과는 `log.level="debug"` (user), `transport.grpc_port=9999` (env), 그 외는 default. `Source("log.level")=="user"`, `Source("transport.grpc_port")=="env"`.
- **Satisfies**: REQ-CFG-001, REQ-CFG-004, REQ-CFG-006

**AC-CFG-002 — 파일 부재 시 기본값 유지**
- **Given** `$GOOSE_HOME=t.TempDir()` (빈 디렉토리), 프로젝트 config 없음, env 없음
- **When** `Load()`
- **Then** 에러 없이 기본값만으로 `*Config` 반환, `Validate()==nil`.
- **Satisfies**: REQ-CFG-004

**AC-CFG-003 — YAML 구문 오류 거부**
- **Given** `$GOOSE_HOME/config.yaml` 내용이 `log:\n  level: [unclosed`
- **When** `Load()`
- **Then** `*ConfigError`가 반환되며 `File` 필드가 입력 경로와 일치하고, `Line` 필드는 파서가 라인 번호를 보고하는 경우에 한해 채워진다 (yaml.v3가 라인을 보고하지 않는 malformation의 경우 0 허용). `errors.Is(err, ErrSyntax)==true`이며, 함수는 기본값으로 silent fallback하지 **않는다**.
- **Satisfies**: REQ-CFG-009

**AC-CFG-004 — 포트 범위 검증**
- **Given** user YAML `transport.grpc_port: 0`
- **When** `Load()` → `Validate()`
- **Then** `ErrInvalidField{Path:"transport.grpc_port", Msg:"must be 1..65535"}` 반환.
- **Satisfies**: REQ-CFG-010

**AC-CFG-005 — 프로젝트 > 유저 오버라이드**
- **Given** user에 `llm.default_provider: "openai"`, project에 `llm.default_provider: "ollama"`
- **When** `Load()`
- **Then** `cfg.LLM.DefaultProvider == "ollama"`, `Source("llm.default_provider")=="project"`.
- **Satisfies**: REQ-CFG-005

**AC-CFG-006 — Unknown 키 보존 (비-strict)**
- **Given** user YAML에 `future_feature: {x: 1}` 포함, `GOOSE_CONFIG_STRICT` 미설정
- **When** `Load()`
- **Then** 에러 없음, `cfg.Unknown["future_feature"]` 존재, WARN 로그 1건.
- **Satisfies**: REQ-CFG-008

**AC-CFG-007 — Strict 모드 거부**
- **Given** AC-CFG-006 상황 + `GOOSE_CONFIG_STRICT=true`
- **When** `Load()`
- **Then** `ErrStrictUnknown{Keys: ["future_feature"]}` 반환.
- **Satisfies**: REQ-CFG-014

**AC-CFG-008 — 환경변수 오버레이 단순 타입**
- **Given** env `GOOSE_LOG_LEVEL=error`, `GOOSE_LEARNING_ENABLED=false`
- **When** `Load()`
- **Then** `cfg.Log.Level=="error"`, `cfg.Learning.Enabled==false`.
- **Satisfies**: REQ-CFG-006

**AC-CFG-009 — Zero-value 명시 override (D18 회귀 방지)**
- **Given** defaults `{learning.enabled: true}`, user YAML `learning:\n  enabled: false` (명시적 false 선언), env 미설정
- **When** `Load()`
- **Then** `cfg.Learning.Enabled == false`, `Source("learning.enabled")=="user"`. 추가 케이스: user YAML `learning:` 키 자체 부재 시 `cfg.Learning.Enabled == true` (default 유지).
- **Satisfies**: REQ-CFG-015

**AC-CFG-010a — Env overlay int happy-path (D17 / D34 분리)**
- **Given** user YAML `transport.grpc_port: 17891`, env `GOOSE_GRPC_PORT=9999`
- **When** `Load()`
- **Then** `cfg.Transport.GRPCPort == 9999`, `Source("transport.grpc_port")=="env"`.
- **Satisfies**: REQ-CFG-006

**AC-CFG-010b — Env overlay int 파싱 실패 fallback (D34 분리)**
- **Given** user YAML `transport.grpc_port: 17891`, env `GOOSE_GRPC_PORT=abc` (정수 파싱 실패)
- **When** `Load()`
- **Then** WARN 로그 1건 기록, `cfg.Transport.GRPCPort == 17891` (하위 레이어 값 유지), `Source("transport.grpc_port")=="user"`, `Load()`는 에러 없이 반환. 최종 값에 대한 범위 검증은 `Validate()` 단계에서 별도 수행.
- **Satisfies**: REQ-CFG-006, R2 리스크 완화 (§8)

**AC-CFG-011 — Env overlay URL 타입 (D17)**
- **Given** defaults `llm.providers.ollama.host: "http://localhost:11434"`, env `OLLAMA_HOST=http://10.0.0.5:11434`
- **When** `Load()`
- **Then** `cfg.LLM.Providers["ollama"].Host == "http://10.0.0.5:11434"`, `Source("llm.providers.ollama.host")=="env"`.
- **Satisfies**: REQ-CFG-006

**AC-CFG-012 — Env overlay secret 타입 + Redacted 마스킹 (D17 / D33)**
- **Given** defaults `llm.providers.openai.api_key: ""`, env `OPENAI_API_KEY=sk-test-123`
- **When** `Load()` → `cfg.Redacted()`
- **Then** `cfg.LLM.Providers["openai"].APIKey == "sk-test-123"` (메모리상 원본 보존), `cfg.Redacted()` 문자열에 `sk-test-123`이 포함되지 않고 `sk-***` (8자 고정 길이) 또는 동등한 마스킹 문자열로 대체됨. 원본 길이가 마스크 길이로 노출되지 않는다.
- **Satisfies**: REQ-CFG-006, REQ-CFG-017

**AC-CFG-013 — fs.FS stub 주입 동등성 (D23)**
- **Given** 동일한 YAML 내용을 (a) 실제 디스크의 `$GOOSE_HOME/config.yaml`에 배치한 케이스와 (b) in-memory `fs.FS`(예: `fstest.MapFS`)로 `LoadOptions.FS`를 통해 주입한 케이스 두 가지로 준비
- **When** 각각 `Load()` 실행
- **Then** 두 경우 모두 동일한 `*Config` 값(깊은 비교로 `reflect.DeepEqual == true`)과 동일한 `Source` 맵을 반환한다. 디스크 I/O를 전혀 수행하지 않은 (b) 경로에서도 성공적으로 Config을 생성할 수 있다.
- **Satisfies**: REQ-CFG-002

**AC-CFG-014 — 동시 읽기 안전성 (D24)**
- **Given** `Load()`가 반환한 `*Config` 인스턴스
- **When** N개(≥ 16) 고루틴이 `cfg.LLM.DefaultProvider`, `cfg.Log.Level`, `cfg.Source("log.level")` 등 공개 getter를 병렬로 호출
- **Then** 모든 고루틴이 동일한 값을 읽으며, `go test -race`가 데이터 레이스를 보고하지 않는다 (race detector clean). Config 내부에 락이 존재하지 않아도 안전하다 (returned pointer is effectively frozen).
- **Satisfies**: REQ-CFG-003

**AC-CFG-015 — Validate() 호출 전 IsValid() false (D25)**
- **Given** 방금 `Load()`가 성공적으로 반환한 `*Config`이지만 `Validate()`가 아직 호출되지 않은 상태
- **When** `cfg.IsValid()`를 호출
- **Then** `false`를 반환한다. 이어서 `cfg.Validate()`가 nil을 반환한 직후 `cfg.IsValid() == true`가 된다. `Validate()`가 에러를 반환한 경우에는 `IsValid()`가 여전히 `false`를 유지한다.
- **Satisfies**: REQ-CFG-007

**AC-CFG-016 — 타입 mismatch 필드 경로 명명 (D26)**
- **Given** user YAML에 `transport:\n  grpc_port: "not-a-number"` (수치 필드에 문자열 주입)
- **When** `Load()`
- **Then** `ErrInvalidField{Path: "transport.grpc_port", Expected: "int", Got: "string"}` 또는 동등한 validation error를 반환하며, 에러 메시지에 **필드 경로(`transport.grpc_port`)**와 **기대 타입(`int`)**이 모두 포함된다. 이는 AC-CFG-004(범위 검증)와는 구별되는 타입 불일치 계약이다.
- **Satisfies**: REQ-CFG-010

**AC-CFG-017 — $GOOSE_HOME 미설정 시 $HOME/.goose fallback (D27)**
- **Given** `os.Unsetenv("GOOSE_HOME")`, `os.Setenv("HOME", "/tmp/goose-test-xyz")`, `/tmp/goose-test-xyz/.goose/config.yaml`에 `log.level: "warn"` 배치
- **When** `Load()`
- **Then** loader가 `/tmp/goose-test-xyz/.goose/config.yaml`을 읽어 `cfg.Log.Level == "warn"`, `Source("log.level")=="user"`. `$HOME` 참조는 단 1회만 발생하며 다른 경로 조회 단계에서는 재참조되지 않는다 (관찰 방법: `fs.FS` 스텁으로 파일 열기 요청 경로를 캡처, `/tmp/goose-test-xyz/.goose/*` 외의 `$HOME` 유도 경로가 열리지 않음을 검증).
- **Satisfies**: REQ-CFG-011

**AC-CFG-018 — 쉘 변수 literal 처리 (D28)**
- **Given** user YAML `log:\n  level: "${FOO}"`, env `FOO=info` (실제로 `FOO`가 설정되어 있음)
- **When** `Load()`
- **Then** `cfg.Log.Level == "${FOO}"` (literal 문자열). loader는 쉘 변수 확장(`os.ExpandEnv`), 환경 치환, 명령 치환을 수행하지 **않는다**. 추가 케이스: `${BAR}`(미설정 env)도 동일하게 `"${BAR}"` literal 유지.
- **Satisfies**: REQ-CFG-012

**AC-CFG-019 — LoadOptions.OverrideFiles 테스트 전용 경로 (D29)**
- **Given** `$GOOSE_HOME/config.yaml`에 `log.level: "info"` (default chain), 별도 경로 `/tmp/override-a.yaml`에 `log.level: "error"`
- **When** `Load(LoadOptions{OverrideFiles: []string{"/tmp/override-a.yaml"}})`
- **Then** loader가 default chain (`$GOOSE_HOME/config.yaml`, 프로젝트 `.goose/config.yaml`)을 **bypass**하고 `OverrideFiles`만 처리한다. 결과는 `cfg.Log.Level == "error"`, `Source("log.level")` 값은 override path를 식별하는 구현-정의 Source 값(예: `SourceOverride` 또는 `"override:/tmp/override-a.yaml"`)을 반환한다. env 오버레이는 여전히 최상위로 적용된다.
- **Satisfies**: REQ-CFG-013

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
| `GOOSE_HOME` | (특수: 검색 경로) | path | `~/.goose` |
| `GOOSE_LOG_LEVEL` | `log.level` | enum{debug,info,warn,error} | info |
| `GOOSE_HEALTH_PORT` | `transport.health_port` | int | 17890 (CORE-001) |
| `GOOSE_GRPC_PORT` | `transport.grpc_port` | int | 17891 |
| `GOOSE_LOCALE` | `ui.locale` | enum{en,ko,ja,zh} | en |
| `OLLAMA_HOST` | `llm.providers.ollama.host` | URL | `http://localhost:11434` |
| `OPENAI_API_KEY` | `llm.providers.openai.api_key` | secret | "" |
| `ANTHROPIC_API_KEY` | `llm.providers.anthropic.api_key` | secret | "" |
| `GOOSE_LEARNING_ENABLED` | `learning.enabled` | bool | false (Phase 0 default) |
| `GOOSE_CONFIG_STRICT` | (특수: loader 동작) | bool | false |

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

**핵심 원칙**: "unset"과 "explicitly set to zero-value"를 구분한다. Go의 bool/int/string zero-value (false/0/"")는 사용자가 **명시적으로 선언한 값**일 수 있으므로, 단순한 zero-value 검사만으로는 override 여부를 판단할 수 없다.

- **규칙 1 (map 재귀)**: `map[string]any` nested 구조는 key-by-key recursive merge. overlay의 key가 존재하면 그 key만 재귀 처리.
- **규칙 2 (scalar presence-aware)**: scalar 필드는 **"overlay YAML에 key가 존재하는가"**로 override 여부를 판단한다. overlay가 zero-value인지 여부는 판단 기준이 **아니다**. 구현 접근:
  - (a) 1차 파싱은 `yaml.Node` 또는 `map[string]any`로 수행하여 key presence를 보존.
  - (b) presence가 확인된 key에 한해 overlay 값을 `*Config`의 대응 필드에 적용.
  - (c) 선택적: struct 필드를 pointer-wrapped (`*bool`, `*int`, `*string`)로 설계하여 nil=unset, non-nil=set을 표현. 단, 본 SPEC은 (a)+(b) 경로를 권장한다.
- **규칙 3 (slice)**: overlay YAML에 key가 존재하고 slice가 선언되면(빈 슬라이스 포함) 완전 대체 (append 없음). overlay key 부재 시 하위 레이어 유지.
- **규칙 4 (pointer/struct)**: struct는 필드별 규칙 1~3 재귀 적용. nested struct 내부도 동일 presence-aware 원칙 적용.

**명시적 반례 (필수 처리)**:
- defaults: `learning.enabled = true`
- user YAML: `learning:\n  enabled: false`
- 결과: `cfg.Learning.Enabled == false` (user의 명시적 false가 override). zero-value 기반 단순 검사로는 이 케이스에서 user의 의도를 무시하게 되므로 금지.

이 규칙은 `yaml.Node`의 `IsZero()`/`Kind != 0` 검사 또는 2단계 unmarshal (`map[string]any` 1차 → `Config` 적용)로 구현 가능. 단위 테스트는 각 규칙별 최소 2케이스(key 부재 / key 존재하며 zero-value)를 격리 검증한다.

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
| 선행 SPEC | **SPEC-GOOSE-CORE-001** | 동일 프로세스의 `zap` logger 인스턴스 공유, `GOOSE_HOME` 의미 상속 |
| 후속 SPEC | SPEC-GOOSE-TRANSPORT-001 | `TransportConfig` 소비 |
| 후속 SPEC | SPEC-GOOSE-LLM-001 | `LLMConfig.Providers` 확장 + `Validate()` 구현 |
| 후속 SPEC | SPEC-GOOSE-AGENT-001 | `LearningConfig` 플래그 소비 |
| 후속 SPEC | SPEC-GOOSE-CLI-001 | `--config /path` 플래그가 `LoadOptions.OverrideFiles`에 주입 |
| 후속 SPEC (forward-ref) | SPEC-GOOSE-CREDPOOL-XXX (미작성) | REQ-CFG-016 경유. 해당 SPEC 채택 시 secret-typed 필드(`llm.providers.*.api_key`)가 credential-pool resolver로 전환됨. 본 SPEC은 훅만 제공. |
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
- `.moai/specs/SPEC-GOOSE-CORE-001/spec.md` §3.1 IN SCOPE 3번 (`~/.goose/config.yaml` 최소 로더)

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
- 본 SPEC은 **CLI `goose config get/set` 하위명령을 포함하지 않는다** (CLI-001).
- 본 SPEC은 **secret manager / keychain 통합을 구현하지 않는다**. API key는 평문 YAML 또는 env. 단 REQ-CFG-016이 향후 credential-pool SPEC과의 연동 훅을 forward-reference로 제공한다 (본 SPEC은 훅 선언만 수행, 동작 전환은 해당 SPEC이 수행).
- 본 SPEC은 **JSON Schema export / 검증 / 생성을 수행하지 않는다** (§3.2 참조). Config 스키마는 Go 타입으로만 정의된다.
- 본 SPEC은 **JSON/TOML/HCL 형식을 지원하지 않는다**. YAML 전용.
- 본 SPEC은 **remote config (Consul/etcd)를 지원하지 않는다**.
- 본 SPEC은 **viper나 다른 설정 프레임워크를 도입하지 않는다**. 명시적 reject.
- 본 SPEC은 **프로바이더별 세부 검증을 수행하지 않는다**. LLM-001 등이 자기 영역 검증을 담당.
- 본 SPEC은 **Windows 레지스트리 / plist 기반 설정을 지원하지 않는다**.
- 본 SPEC은 **설정 암호화를 구현하지 않는다**. 파일 시스템 권한(0600)에 의존.

---

**End of SPEC-GOOSE-CONFIG-001**
