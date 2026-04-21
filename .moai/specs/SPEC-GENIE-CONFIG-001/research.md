# SPEC-GENIE-CONFIG-001 — Research & Inheritance Analysis

> **목적**: 계층형 설정 로더 구현에 재활용 가능한 자산을 식별하고, 자체 구현 범위를 확정한다.
> **작성일**: 2026-04-21

---

## 1. 레포 상태 스캔 (재확인)

CORE-001의 `research.md` §1과 동일:

- `cmd/`, `internal/`, `pkg/`, `go.mod` 부재. Go 소스 0 LoC.
- MoAI-ADK-Go 외부 레포 미러 없음.

따라서 `internal/config/` 패키지는 **신규 작성**. 이식 가능한 사전 구현이 없다.

---

## 2. 참조 자산별 분석

### 2.1 Claude Code TypeScript (`./claude-code-source-map/`)

Config 관련 파일 스캔:

```
$ grep -rlni "config" claude-code-source-map/bootstrap/ claude-code-source-map/entrypoints/
claude-code-source-map/bootstrap/state.ts   (56KB)
claude-code-source-map/entrypoints/init.ts  (13KB)
```

`init.ts` 초기화 시퀀스 참고 결과:

- Claude Code는 JSON 기반 `settings.json` 계층을 사용 (`~/.claude/settings.json` vs project `.claude/settings.json`).
- 계층 순서: managed → project → user → local. 본 SPEC의 3-layer(project < user < env)와 유사하지만 "managed policy" 레이어는 GENIE에서는 생략.
- **계승 대상**: 계층 아이디어만. 구현은 Go로 재작성.

### 2.2 Hermes Agent Python (`./hermes-agent-main/`)

```
$ ls hermes-agent-main/ | grep -i config
(결과 없음; hermes는 CLI flag만 사용하는 구조)
```

- Hermes는 `cli.py`(409KB 단일 파일)에서 argparse 기반 런타임 flag만 수용. 파일 기반 설정 계층 없음.
- **계승 대상**: 없음.

### 2.3 MoAI-ADK (현 레포 `.moai/config/sections/*.yaml`)

본 AgentOS 레포 자체의 MoAI 설정이 좋은 레퍼런스이다:

```
.moai/config/sections/
├── user.yaml         # 계정/역할
├── language.yaml     # 언어 정책
├── workflow.yaml     # 워크플로우 매개변수
├── harness.yaml      # 품질 레벨
└── quality.yaml      # TDD/DDD 설정
```

**계승할 원칙**:
- 단일 거대 YAML 대신 **관심사별 분할 가능한 구조**를 설계. 본 SPEC v0.1은 **단일 파일**로 시작하되, `Config` struct 자체는 "sectional"(`log`, `transport`, `llm`, ...)로 구성하여 향후 파일 분할이 저렴하도록 함.

**비-계승**:
- YAML 키 네이밍은 snake_case(MoAI 관습) 유지.
- "agent_prompt_language"같은 MoAI 전용 필드 이동 없음.

### 2.4 Viper 검토 (사용 안 함)

`tech.md` §3.1이 `spf13/viper` 1.19+를 목록에 포함. 그러나 본 SPEC은 **의도적으로 viper를 도입하지 않음**:

| 관점 | Viper | 자체 구현 |
|-----|------|---------|
| Package coupling | 전역 싱글턴 기본, 명시적 `viper.New()`로 우회 가능하나 예제/생태계는 전역 중심 | 순수 함수 `Load()` → `*Config` |
| Test isolation | `viper.Reset()` 필요, 병렬 테스트에서 flaky | `testdata/*.yaml` + `Load(opts)` 독립 |
| Type safety | `viper.GetString("a.b.c")` — path mistyping을 runtime에만 감지 | struct 태그, 컴파일 타임 |
| 기능 범위 | fsnotify, remote, etcd, JSON/TOML/HCL — 전부 Phase 0 불필요 | YAML only, deep merge만 |
| 의존성 무게 | viper + pflag + fsnotify + hcl + ... (~15개 transitive) | `yaml.v3` 하나 |

**결정**: 본 SPEC은 viper 미사용. 후속 SPEC이 fsnotify 기반 hot reload를 필요로 하면 그 시점에 viper 재평가 SPEC을 신설.

### 2.5 BurntSushi/toml, pelletier/go-toml (불필요)

TOML은 tech.md에서 언급되지 않음. YAML 단일 형식 충분.

---

## 3. Go 이디엄 선택

### 3.1 Struct 기반 vs map[string]any

**Struct 채택** 이유:
- 컴파일 타임 타입 안전.
- IDE 자동완성 + go doc으로 스키마 문서화.
- `yaml.v3`의 KnownFields 기능으로 strict 모드 무료.

단점(처리 방법):
- Unknown 키 보존은 `yaml:",inline"` + `map[string]any`로 해결 (REQ-CFG-008).

### 3.2 Deep Merge: reflect vs code gen

**reflect 기반** (본 SPEC 채택):
- 패키지 ~40 LoC 추정.
- 런타임 오버헤드: Load는 프로세스당 1회 → 성능 무관.
- `encoding/json`의 stdlib 처리와 유사한 수준.

code gen (기각):
- `structcopier` 류 필요. 빌드 복잡도 증가. Phase 0 과분.

### 3.3 Env 오버레이: 선언적 매핑 테이블

```go
var envMap = []envBinding{
    {Env: "GENIE_LOG_LEVEL",    Path: "log.level",             Kind: KindString},
    {Env: "GENIE_GRPC_PORT",    Path: "transport.grpc_port",   Kind: KindInt},
    {Env: "GENIE_LEARNING_ENABLED", Path: "learning.enabled",  Kind: KindBool},
    // ...
}
```

- 런타임에 table-driven loop.
- 신규 env 추가 시 한 줄 + 테스트 한 줄.
- 각 후속 SPEC이 자기 엔트리를 append (같은 path 금지를 init-time assert).

### 3.4 에러 설계

- `ErrSyntax` — YAML 파싱 실패 (위치 정보 포함).
- `ErrInvalidField` — 타입/범위 위반.
- `ErrStrictUnknown` — REQ-CFG-014.
- `ErrTooLarge` — 파일 크기 cap(256KB) 초과.

전부 `errors.Is`/`errors.As`로 식별 가능. 구조화 로깅(zap)과 결합해 필드 경로 직접 노출.

---

## 4. 외부 의존성 합계

| 모듈 | 용도 | 본 SPEC 채택 |
|------|------|-----------|
| `gopkg.in/yaml.v3` | 파싱 | ✅ (CORE-001과 동일) |
| `github.com/stretchr/testify` | 테스트 | ✅ |
| 표준 `reflect`, `os`, `path/filepath`, `errors` | merge/env/path | ✅ |
| `spf13/viper` | config framework | ❌ 미도입 |
| `fsnotify/fsnotify` | 파일 감시 | ❌ OUT OF SCOPE |

**의도적 미사용**: knadh/koanf, cristalhq/aconfig 등 마이너 대안도 제외. 표준 라이브러리 + yaml.v3로 충분.

---

## 5. 테스트 전략

### 5.1 Unit 테스트 (예상 20+개)

- `TestMerge_DefaultsOnly`
- `TestMerge_UserOverridesDefault_Scalar`
- `TestMerge_UserOverridesDefault_Nested`
- `TestMerge_ProjectOverridesUser`
- `TestMerge_EnvBeatsFiles`
- `TestMerge_EmptySliceDoesNotOverride`
- `TestMerge_NonEmptySliceReplaces`
- `TestValidate_LogLevel_UnknownRejected`
- `TestValidate_Port_ZeroRejected`
- `TestValidate_Port_OutOfRange_Rejected`
- `TestValidate_Port_Valid_Accepted`
- `TestSource_TrackingSingleField`
- `TestSource_TrackingAcrossLayers`
- `TestEnv_BoolParsing_TrueFalse`
- `TestEnv_IntParsing_InvalidIgnored`
- `TestEnv_UnknownEnv_Ignored`
- `TestUnknown_PreservedInMap`
- `TestUnknown_StrictRejects`
- `TestLoad_MissingFile_NoError`
- `TestLoad_MalformedYaml_IncludesLineCol`
- `TestLoad_TooLargeFile_Rejected`

### 5.2 Integration / 파일시스템 테스트

- `testdata/full.yaml` (모든 필드 채움) + round-trip 비교.
- `t.TempDir()` 기반 실제 파일 생성 후 `Load(LoadOptions{Home: tempDir})`.

### 5.3 Race Detector

`go test -race`. Config 읽기 전용 불변이므로 race는 없어야 한다. 읽기 경합 테스트 1개(`TestConfig_ConcurrentReads`)로 확인.

### 5.4 커버리지 목표

- `internal/config/`: 95%+ (알고리즘 코어는 100%).

---

## 6. 오픈 이슈

1. **`.genie/config.yaml` vs `genie.yaml`**: 프로젝트 로컬 config 파일명. 본 SPEC은 `.genie/config.yaml`로 고정 (dotfile). 후속 논의 시 `genie.yaml` alias 추가 가능.
2. **Env binding 중복 방지**: `envMap`에 동일 `Path`가 두 번 들어가면 어떻게? → 본 SPEC은 `init()`에서 `panic`으로 빌드 시점 실패 유도.
3. **API Key 로깅**: `Config.Redacted()` 구현 위치. zap logger의 `zap.ObjectMarshaler` 인터페이스 준수로 `logger.With("cfg", cfg)`가 자동 마스킹하도록 할지 결정 필요. 본 SPEC은 별도 메서드로 우선 제공, integration은 CLI-001에서.
4. **XDG Base Directory**: Linux 사용자 중 `$XDG_CONFIG_HOME/genie/`를 선호. 본 SPEC은 `$GENIE_HOME` 단일 override 허용. XDG는 차후 SPEC.

---

## 7. 결론

- **이식 자산**: 없음. MoAI-ADK-Go config 패키지 미러 없음.
- **참조 자산**: Claude Code `settings.json` 계층 아이디어, 본 레포 `.moai/config/sections/*.yaml` 레이아웃.
- **기술 스택**: `yaml.v3` + reflect 기반 자체 merge + table-driven env overlay. viper 미도입.
- **구현 규모 예상**: 400~600 LoC (테스트 포함 1,000~1,400 LoC).
- **리스크**: deep merge edge case와 env 타입 파싱. 전부 단위 테스트로 격리 가능.

GREEN 단계 완료 시점에서 TRANSPORT-001, LLM-001, AGENT-001, CLI-001이 **같은 `*Config`를 참조**하는 결정적 지점이 확보된다.

---

**End of research.md**
