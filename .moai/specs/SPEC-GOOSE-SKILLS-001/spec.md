---
id: SPEC-GOOSE-SKILLS-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 2
size: 대(L)
lifecycle: spec-anchored
---

# SPEC-GOOSE-SKILLS-001 — Progressive Disclosure Skill System (L0~L3, YAML, 4 Trigger)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-primitives §2 + ROADMAP v2.0 Phase 2 기반) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 **Skill 시스템**을 정의한다. Claude Code의 Progressive Disclosure(L0~L3 effort 티어) + YAML frontmatter + 4-trigger 활성화 모델을 Go로 포팅하여, `QueryEngine`(SPEC-GOOSE-QUERY-001)이 매 iteration 직전 적절한 Skill을 로드·치환·권한 검증한 후 system/user prompt에 반영하게 한다.

본 SPEC이 통과한 시점에서 `internal/skill` 패키지는:

- `LoadSkillsDir(root)`로 디스크 상의 `SKILL.md` 파일을 walk하여 `SkillDefinition` 레지스트리를 구성하고,
- YAML frontmatter를 allowlist 기반(`SAFE_SKILL_PROPERTIES`)으로 파싱하여 **알 수 없는 속성은 default-deny**하며,
- 4-trigger(inline / fork / conditional / remote) 각각을 결정적으로 활성화하고,
- `${CLAUDE_SKILL_DIR}`, `${CLAUDE_SESSION_ID}` 등 변수 치환을 skill 본문 로드 시점에 적용하며,
- `paths:` 조건부 매칭은 gitignore 문법(`!` 부정 포함)으로 처리하고,
- Progressive Disclosure(L0=50 tokens, L1=200, L2=500, L3=1000+)에 따라 **frontmatter만 먼저 파싱**해 발견 비용을 최소화한다(`estimateSkillFrontmatterTokens`).

본 SPEC은 **skill 자체 정의/저장/버전관리/마켓플레이스**는 다루지 않는다. 그것은 PLUGIN-001 또는 별도 생태계 SPEC.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Phase 2의 4 primitive(Skills/MCP/Agents/Hooks) 중 Skill은 Agent/Hook/Plugin 모두의 의존 대상. `SUBAGENT-001`이 agent system prompt를 forked skill로 구성할 수 있어야 하고, `PLUGIN-001`이 plugin manifest의 `skills:` 배열을 로드할 수 있어야 한다.
- `.moai/project/research/claude-primitives.md` §2가 Claude Code의 Skill 아키텍처(frontmatter schema, 4 trigger, allowlist permission)를 제시한다. 본 SPEC은 그 구조를 Go 이디엄(struct tag + `go-yaml/yaml` + `fsnotify` optional)으로 확정한다.
- QUERY-001의 continue site는 `State.Messages` 외의 프롬프트 구성 요소(system/user prompt)를 주입받기만 한다. Skill이 그 **주입자**가 된다.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code TypeScript** (`./claude-code-source-map/`): `skills/`, `loadSkillsDir.ts`, `parseSkill*`, `estimateSkillFrontmatterTokens`. Go 포팅 시 `go-yaml/yaml` + 직접 ignore matcher 사용. TS async 동적 로딩은 Go에서는 eager walk + lazy body read.
- **MoAI-ADK `.claude/skills/`**: SKILL.md + `modules/` + `references/` 구조. 본 SPEC의 스키마는 MoAI-ADK의 진화형.
- **Hermes Agent `agent/prompts/`**: 정적 prompt 템플릿. Progressive Disclosure는 Hermes에 없으며, 본 SPEC에서 신규 도입.

### 2.3 범위 경계

- **IN**: `SkillDefinition`/`SkillFrontmatter` 구조체, YAML frontmatter parser(allowlist-default-deny), Progressive Disclosure(effort L0/L1/L2/L3) 토큰 추정, 4-trigger(inline/fork/conditional/remote) 결정 로직, `${VAR}` 치환, `paths:` gitignore 매칭, `disable-model-invocation`/`user-invocable`/`allowed-tools` 권한 게이트, `FileChanged` hook consumer(조건부 활성화 경로), Remote skill(`_canonical_{slug}`) 로더 skeleton.
- **OUT**: 실제 LLM 호출(ADAPTER-001), Forked skill agent spawning(SUBAGENT-001), Hook 이벤트 디스패치(HOOK-001), Plugin manifest 통합(PLUGIN-001), Remote skill의 AKI/GCS 인증 구현(Phase 5+), Skill SDK(유저 작성 도구), Skill editor UI(CLI-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/skill/` 패키지 생성.
2. `SkillDefinition`, `SkillFrontmatter`, `TriggerMode`, `EffortLevel` 타입.
3. `SkillRegistry` 컨테이너(map + RWMutex, atomic replace).
4. `LoadSkillsDir(root string, opts ...LoadOption) (*SkillRegistry, error)` walker.
5. `ParseSkillFile(path string, content []byte) (*SkillDefinition, error)` — YAML frontmatter + body 분리.
6. `ValidateFrontmatter(fm SkillFrontmatter) error` — `SAFE_SKILL_PROPERTIES` allowlist 기반.
7. `EstimateSkillFrontmatterTokens(fm SkillFrontmatter) int` — name + description + when-to-use 길이 기반 휴리스틱.
8. `ResolveEffort(fm SkillFrontmatter) EffortLevel` — `effort:` 필드 → L0/L1/L2/L3 매핑 (숫자 또는 알파넘).
9. 4-trigger 결정자:
   - `IsInline(fm)` / `IsForked(fm)` / `IsConditional(fm)` / `IsRemote(fm)`.
10. Conditional 활성화 — `paths:` 패턴 gitignore 매칭(`github.com/sabhiram/go-gitignore` 또는 자체 구현).
11. 변수 치환 — `${CLAUDE_SKILL_DIR}`, `${CLAUDE_SESSION_ID}`, `${USER_HOME}` 등 body/hooks 로드 시점 치환.
12. 권한 게이트 — `disableModelInvocation`, `userInvocable`, `allowedTools`의 런타임 검증 함수.
13. `FileChangedConsumer(paths []string) []string` — 변경된 파일 경로를 활성화 대상 skill ID 목록으로 변환(HOOK-001에서 FileChanged 이벤트를 본 패키지로 라우팅).
14. Remote skill loader skeleton — `LoadRemoteSkill(uri string) (*SkillDefinition, error)` (HTTP fetch만; 인증은 Phase 5+ TODO).

### 3.2 OUT OF SCOPE

- **Forked skill 실행**: `context: fork` 플래그 감지는 본 SPEC 범위이나, 실제 sub-agent spawn은 SUBAGENT-001.
- **Slash command 전개**: `!command` + `$ARG` 치환은 COMMAND-001.
- **Hook 이벤트 dispatcher**: `SessionStart`/`PostToolUse` 등 frontmatter의 `hooks:` 필드는 본 SPEC이 파싱만. 실제 라우팅은 HOOK-001.
- **Remote skill 인증/캐시**: 본 SPEC은 `http.Get`까지만. AKI/GCS/OAuth는 Phase 5+.
- **Skill hot-reload**: `fsnotify` 기반 watcher는 PLUGIN-001에서 통합(atomic clearThenRegister).
- **Skill 편집 UI**: CLI-001 / 별도 SPEC.
- **MCP prompts/list로부터 Skill 변환**: MCP-001의 책임.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-SK-001 [Ubiquitous]** — The `SkillRegistry` **shall** reject any frontmatter key that is not in `SAFE_SKILL_PROPERTIES` allowlist; unknown keys **shall** cause `ParseSkillFile` to return `ErrUnsafeFrontmatterProperty` without partial registration.

**REQ-SK-002 [Ubiquitous]** — The `SkillFrontmatter` parser **shall** preserve exact YAML ordering for `hooks:` arrays; event handler order **shall** match source document order (no sorting, no dedup).

**REQ-SK-003 [Ubiquitous]** — The `EstimateSkillFrontmatterTokens` function **shall** compute token count from `name + description + whenToUse` fields only, without loading the skill body file contents.

**REQ-SK-004 [Ubiquitous]** — Every `SkillDefinition` in the registry **shall** have a unique `ID` (path-derived slug); duplicate IDs **shall** cause `LoadSkillsDir` to return `ErrDuplicateSkillID` and **shall not** replace the prior entry.

### 4.2 Event-Driven (이벤트 기반)

**REQ-SK-005 [Event-Driven]** — **When** `LoadSkillsDir(root)` is invoked, the walker **shall** (a) recursively scan `root` for files named `SKILL.md`, (b) parse each file's YAML frontmatter, (c) validate against `SAFE_SKILL_PROPERTIES`, (d) populate `SkillRegistry` atomically, and (e) return the populated registry plus an error slice (one entry per skipped file; no partial failure aborts the full load).

**REQ-SK-006 [Event-Driven]** — **When** a skill body contains `${CLAUDE_SKILL_DIR}`, the `ResolveBody(session)` function **shall** substitute with the absolute path of the skill's parent directory; **when** it contains `${CLAUDE_SESSION_ID}`, substitute with `session.ID`; unknown variables **shall** remain literal.

**REQ-SK-007 [Event-Driven]** — **When** `FileChangedConsumer(changedPaths)` is invoked, the function **shall** iterate the registry, match each conditional skill's `paths:` patterns against `changedPaths` using gitignore semantics, and return the list of skill IDs whose patterns matched.

**REQ-SK-008 [Event-Driven]** — **When** a skill's frontmatter sets `context: fork`, `IsForked(fm)` **shall** return `true` and the skill **shall not** be eligible for inline prompt-body injection; instead the consumer(SUBAGENT-001) **shall** route it through `runAgent`.

**REQ-SK-009 [Event-Driven]** — **When** a skill's frontmatter sets `disable-model-invocation: true`, the registry's `CanInvoke(skill, actor)` gate **shall** return `false` for `actor = "model"` and `true` only for `actor = "user"` or `actor = "hook"`.

### 4.3 State-Driven (상태 기반)

**REQ-SK-010 [State-Driven]** — **While** the `effort:` frontmatter field is absent, the resolved `EffortLevel` **shall** default to `L1` (200 tokens budget); explicit `L0`/`L1`/`L2`/`L3` strings override; integer values 0/1/2/3 **shall** map to `L0`/`L1`/`L2`/`L3`.

**REQ-SK-011 [State-Driven]** — **While** `paths:` frontmatter is absent, `IsConditional(fm)` **shall** return `false`; the skill **shall not** be included in any `FileChangedConsumer` result.

**REQ-SK-012 [State-Driven]** — **While** a skill is remote(`ID` has `_canonical_` prefix), `LoadRemoteSkill` **shall** bypass the disk walker and fetch the `SKILL.md` over HTTP; cached remote skills **shall** be re-validated on every `LoadSkillsDir` call (no disk persistence in this SPEC).

### 4.4 Unwanted Behavior (방지)

**REQ-SK-013 [Unwanted]** — The parser **shall not** execute any `shell:` directive at parse time; `shell:` is parsed as metadata only, and actual shell invocation is deferred to the consumer (hook dispatcher / agent runtime).

**REQ-SK-014 [Unwanted]** — **If** a skill body contains `${` followed by an unknown variable name (e.g., `${ENV.SECRET}`), **then** the substitution **shall** log a warning and leave the literal intact; it **shall not** fail the load nor expose `os.Getenv` values.

**REQ-SK-015 [Unwanted]** — The `ParseSkillFile` function **shall not** follow symbolic links that resolve outside the `root` passed to `LoadSkillsDir`; symlink escape attempts **shall** cause `ErrSymlinkEscape` for that file only.

**REQ-SK-016 [Unwanted]** — The registry **shall not** mutate a `SkillDefinition` in-place after first insertion; updates **shall** go through `Replace(id, newDef)` which performs an atomic swap via a new map copy.

### 4.5 Optional (선택적)

**REQ-SK-017 [Optional]** — **Where** `model: opus[1m]` is set in frontmatter, the skill loader **shall** record the preferred model alias in `SkillDefinition.PreferredModel`; the consumer(ROUTER-001) **may** honor or override.

**REQ-SK-018 [Optional]** — **Where** `argument-hint:` is non-empty, the skill loader **shall** expose it via `SkillDefinition.ArgumentHint` for COMMAND-001's slash-command autocomplete consumer.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-SK-001 — 최소 SKILL.md 로드**
- **Given** `/tmp/skills/hello/SKILL.md`에 `name: hello`, `description: "say hi"` frontmatter + 본문 "Hello"
- **When** `LoadSkillsDir("/tmp/skills")`
- **Then** 레지스트리에 1개 skill(`ID="hello"`, `Effort=L1`, `IsInline=true`, `IsConditional=false`)이 등록되고, `EstimateSkillFrontmatterTokens()`가 `len("hello")+len("say hi")` 기반 추정값을 반환

**AC-SK-002 — Allowlist-default-deny로 unknown property 거부**
- **Given** SKILL.md에 `frobnicate: true`(allowlist 미포함) 포함
- **When** `ParseSkillFile`
- **Then** `err = ErrUnsafeFrontmatterProperty`, 해당 skill은 레지스트리에 등록되지 않음, 다른 정상 skill은 영향 없음

**AC-SK-003 — Progressive Disclosure effort 매핑**
- **Given** 4개 skill: `effort: L0`, `effort: 2`, `effort: L3`, 미지정
- **When** 각각 `ResolveEffort`
- **Then** 반환값 `L0`, `L2`, `L3`, `L1`(기본)

**AC-SK-004 — Conditional 활성화 (gitignore 매칭)**
- **Given** SKILL.md에 `paths: ["src/**/*.ts", "!**/test/**"]`, `FileChangedConsumer`에 `["src/foo/bar.ts", "src/test/baz.ts", "README.md"]` 전달
- **When** consumer 호출
- **Then** 결과 skill ID 리스트에 해당 skill이 포함(첫 경로 매칭 + 두번째는 부정 패턴으로 제외 + 세번째 미매칭). 반환 리스트에는 skill ID가 정확히 1회 포함

**AC-SK-005 — Forked skill 감지**
- **Given** SKILL.md에 `context: fork` 설정
- **When** `IsForked`/`IsInline`
- **Then** `IsForked == true`, `IsInline == false`; consumer는 본 skill 본문을 inline 주입하지 않음

**AC-SK-006 — 변수 치환**
- **Given** SKILL.md 본문이 `"Working in ${CLAUDE_SKILL_DIR} session ${CLAUDE_SESSION_ID}"`, session.ID = `"sess-abc"`, 파일 경로 `/tmp/skills/hello/SKILL.md`
- **When** `ResolveBody(session)`
- **Then** 결과 문자열이 `"Working in /tmp/skills/hello session sess-abc"` (마지막 경로 구성요소 포함, 미지 변수는 그대로 유지)

**AC-SK-007 — Model invocation 차단**
- **Given** SKILL.md에 `disable-model-invocation: true`
- **When** `CanInvoke(skill, "model")` 및 `CanInvoke(skill, "user")`
- **Then** 전자는 false, 후자는 true

**AC-SK-008 — Symlink escape 방지**
- **Given** `/tmp/skills/evil/SKILL.md`가 `/etc/passwd`로의 symlink
- **When** `LoadSkillsDir("/tmp/skills")`
- **Then** `evil` skill은 error slice에 `ErrSymlinkEscape`로 포함, 다른 skill 로드는 성공

**AC-SK-009 — 중복 ID 탐지**
- **Given** `/tmp/skills/a/SKILL.md`와 `/tmp/skills/b/SKILL.md`가 모두 `name: same`
- **When** `LoadSkillsDir`
- **Then** 두 번째 파일은 `ErrDuplicateSkillID`로 error slice에 포함, 첫 번째만 레지스트리 진입

**AC-SK-010 — Remote skill 로드 (HTTP)**
- **Given** 테스트 httptest.Server가 경로 `/skills/remote.md`에 valid SKILL.md 콘텐츠 반환
- **When** `LoadRemoteSkill("http://127.0.0.1:PORT/skills/remote.md")`
- **Then** `SkillDefinition.ID`가 `_canonical_remote` 접두사를 가지며, `IsRemote == true`

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/
└── skill/
    ├── loader.go            # LoadSkillsDir, 파일 walker
    ├── parser.go            # ParseSkillFile, frontmatter 분리
    ├── schema.go            # SkillDefinition, SkillFrontmatter, SAFE_SKILL_PROPERTIES
    ├── conditional.go       # paths: gitignore 매칭
    ├── remote.go            # LoadRemoteSkill HTTP fetcher
    ├── runtime.go           # ResolveBody, ResolveEffort, CanInvoke, EstimateTokens
    ├── registry.go          # SkillRegistry (atomic swap)
    └── *_test.go
```

### 6.2 핵심 Go 타입 시그니처

```go
// 지원되는 trigger 모드. discriminated union.
type TriggerMode int
const (
    TriggerInline TriggerMode = iota
    TriggerFork
    TriggerConditional
    TriggerRemote
)

// Progressive Disclosure 레벨.
type EffortLevel int
const (
    EffortL0 EffortLevel = iota  // ~50 tokens
    EffortL1                      // ~200 tokens (default)
    EffortL2                      // ~500 tokens
    EffortL3                      // ~1000+ tokens
)

// YAML frontmatter의 스키마. 알려진 속성만 필드로 존재.
type SkillFrontmatter struct {
    Name                   string            `yaml:"name,omitempty"`
    Description            string            `yaml:"description,omitempty"`
    WhenToUse              string            `yaml:"when-to-use,omitempty"`
    ArgumentHint           string            `yaml:"argument-hint,omitempty"`
    Arguments              []string          `yaml:"arguments,omitempty"`
    Model                  string            `yaml:"model,omitempty"`
    Effort                 string            `yaml:"effort,omitempty"`   // "L0"|"L1"|"L2"|"L3"|숫자
    Context                string            `yaml:"context,omitempty"`  // "fork"|"inline"
    Agent                  string            `yaml:"agent,omitempty"`
    AllowedTools           []string          `yaml:"allowed-tools,omitempty"`
    DisableModelInvocation bool              `yaml:"disable-model-invocation,omitempty"`
    UserInvocable          *bool             `yaml:"user-invocable,omitempty"`  // nil=기본 true
    Paths                  []string          `yaml:"paths,omitempty"`
    Shell                  *SkillShellConfig `yaml:"shell,omitempty"`
    Hooks                  map[string][]SkillHookEntry `yaml:"hooks,omitempty"`
}

type SkillShellConfig struct {
    Executable string `yaml:"executable"`
    DenyWrite  bool   `yaml:"deny-write,omitempty"`
}

type SkillHookEntry struct {
    Matcher string `yaml:"matcher,omitempty"`
    Command string `yaml:"command"`
}

// 런타임에서 유지되는 완전한 skill.
type SkillDefinition struct {
    ID              string            // path-derived slug
    AbsolutePath    string            // SKILL.md 절대경로
    Frontmatter     SkillFrontmatter
    Body            string            // raw body (치환 전)
    Trigger         TriggerMode
    Effort          EffortLevel
    PreferredModel  string
    FrontmatterTokens int              // estimate
    IsRemote        bool
}

// allowlist. 새 속성 추가는 코드 수정 + 테스트 필수.
var SAFE_SKILL_PROPERTIES = map[string]struct{}{
    "name": {}, "description": {}, "when-to-use": {},
    "argument-hint": {}, "arguments": {}, "model": {},
    "effort": {}, "context": {}, "agent": {}, "allowed-tools": {},
    "disable-model-invocation": {}, "user-invocable": {},
    "paths": {}, "shell": {}, "hooks": {},
}

// Registry. atomic replace 기반.
type SkillRegistry struct {
    mu      sync.RWMutex
    skills  map[string]*SkillDefinition
    logger  *zap.Logger
}

func LoadSkillsDir(root string, opts ...LoadOption) (*SkillRegistry, []error)
func (r *SkillRegistry) Get(id string) (*SkillDefinition, bool)
func (r *SkillRegistry) Replace(newSkills map[string]*SkillDefinition)
func (r *SkillRegistry) FileChangedConsumer(changed []string) []string
func (r *SkillRegistry) CanInvoke(id string, actor string) bool
```

### 6.3 Trigger 결정 규칙

| Trigger | 조건 | 소비자 |
|---------|------|--------|
| `TriggerRemote` | `ID` prefix `_canonical_` | Remote loader skeleton |
| `TriggerFork` | `Context == "fork"` | SUBAGENT-001 `runAgent` |
| `TriggerConditional` | `len(Paths) > 0` | `FileChangedConsumer` → HOOK-001 |
| `TriggerInline` | 위 조건 모두 아님 (default) | QueryEngine prompt 주입자 |

우선순위: remote > fork > conditional > inline. 하나의 skill은 정확히 하나의 trigger에 속한다.

### 6.4 Variable Substitution 규약

| 변수 | 치환 값 |
|------|--------|
| `${CLAUDE_SKILL_DIR}` | `filepath.Dir(def.AbsolutePath)` |
| `${CLAUDE_SESSION_ID}` | `session.ID` (런타임 주입) |
| `${USER_HOME}` | `os.UserHomeDir()` |
| 기타 `${XXX}` | 치환 없이 그대로 유지 + zap warn |

`os.Getenv`는 **금지** (REQ-SK-014). 민감 정보 노출 위험 때문.

### 6.5 Conditional 매칭 (gitignore 문법)

- `github.com/sabhiram/go-gitignore` 또는 `github.com/denormal/go-gitignore` 중 후자 채택(테스트 커버리지 + 유지보수 활성도).
- `!**/test/**` 같은 부정 패턴 지원.
- 대소문자 민감(OS 무관 일관성).

### 6.6 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| YAML frontmatter 파싱 | `gopkg.in/yaml.v3` | stdlib 없음, v3가 strict mode 지원 |
| gitignore 매칭 | `github.com/denormal/go-gitignore` | 부정 패턴 지원, 활성 유지보수 |
| 파일 walker | stdlib `io/fs.WalkDir` | symlink 제어 용이 |
| HTTP client | stdlib `net/http` | Remote skill fetch |
| 로깅 | `go.uber.org/zap` | CORE-001 공유 |

### 6.7 Allowlist-Default-Deny 파싱 전략

1. YAML을 우선 `map[string]any`로 loose unmarshal.
2. 키를 순회하며 `SAFE_SKILL_PROPERTIES`에 있는지 확인. 없으면 `ErrUnsafeFrontmatterProperty`.
3. 통과한 항목만 `SkillFrontmatter` struct로 2차 unmarshal.
4. 이유: `yaml.v3`의 strict=true는 알 수 없는 필드 전체를 에러로 만들지만, 어떤 키가 범인인지 보고하기 어렵다. 2단계 전략이 진단 메시지 품질 높음.

### 6.8 TDD 진입 순서

1. **RED #1** — `TestSchema_SafeSkillProperties_ContainsExpected` (목록 확정)
2. **RED #2** — `TestParseSkillFile_MinimalValid` (AC-SK-001)
3. **RED #3** — `TestParseSkillFile_UnknownProperty_Rejected` (AC-SK-002)
4. **RED #4** — `TestResolveEffort_Mappings` (AC-SK-003)
5. **RED #5** — `TestFileChangedConsumer_GitignoreMatching` (AC-SK-004)
6. **RED #6** — `TestIsForked_ContextFork` (AC-SK-005)
7. **RED #7** — `TestResolveBody_VariableSubstitution` (AC-SK-006)
8. **RED #8** — `TestCanInvoke_DisableModelInvocation` (AC-SK-007)
9. **RED #9** — `TestLoadSkillsDir_SymlinkEscape` (AC-SK-008)
10. **RED #10** — `TestLoadSkillsDir_DuplicateID` (AC-SK-009)
11. **RED #11** — `TestLoadRemoteSkill_HTTP` (AC-SK-010)
12. **GREEN** — 최소 구현.
13. **REFACTOR** — registry atomic swap을 `atomic.Pointer[map]`로 최적화 검토.

### 6.9 TRUST 5 매핑

| 차원 | 본 SPEC 달성 방법 |
|-----|-----------------|
| **T**ested | 25+ unit test, 10 integration test (AC 1:1), 커버리지 90%+ |
| **R**eadable | 파일별 단일 책임(loader/parser/schema/conditional/remote/runtime/registry), 한국어 주석 + 영어 식별자 |
| **U**nified | `go fmt`, `golangci-lint` (errcheck, govet, staticcheck) |
| **S**ecured | Allowlist-default-deny, symlink escape 방지, env var 치환 금지, shell directive 실행 금지 |
| **T**rackable | 각 skill 로드 시 zap 구조화 로그 (`skill_id`, `effort`, `trigger`), error slice로 부분 실패 가시화 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | Skill consumer(QueryEngine prompt 주입자)의 인터페이스 계약 |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap 로거, context 루트 |
| 후속 SPEC | SPEC-GOOSE-HOOK-001 | `FileChanged` 이벤트 → `FileChangedConsumer` 라우팅 |
| 후속 SPEC | SPEC-GOOSE-SUBAGENT-001 | `context: fork` skill의 agent spawn |
| 후속 SPEC | SPEC-GOOSE-PLUGIN-001 | plugin manifest `skills:` 로딩 |
| 후속 SPEC | SPEC-GOOSE-COMMAND-001 | `argument-hint:` 소비 |
| 외부 | Go 1.22+ | `io/fs.WalkDir`, generics |
| 외부 | `gopkg.in/yaml.v3` v3.0+ | YAML 파싱 |
| 외부 | `github.com/denormal/go-gitignore` v0.3+ | paths 매칭 |
| 외부 | `go.uber.org/zap` v1.27+ | 구조화 로깅 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | `SAFE_SKILL_PROPERTIES` 확장 시 생태계 호환성 파괴 | 중 | 고 | 모든 추가는 allowlist 테이블 테스트 + semver minor로 공지 |
| R2 | gitignore 라이브러리가 deprecated되어 재작성 필요 | 낮 | 중 | 인터페이스 `Matcher` 추상화, 내부 구현 교체 가능 |
| R3 | Remote skill 로드가 네트워크 장애로 전체 `LoadSkillsDir` 실패 | 중 | 중 | Remote는 error slice로만 보고, 로컬 skill 로드는 성공 |
| R4 | 변수 치환 누락으로 `${SECRET_XXX}` 유출 | 고 | 고 | env var 치환 절대 금지(REQ-SK-014), 알 수 없는 변수는 리터럴 유지 + warn |
| R5 | Forked skill의 실제 semantics가 SUBAGENT-001과 어긋나 재설계 | 중 | 중 | 본 SPEC은 **감지만**(`IsForked`). 실행 계약은 SUBAGENT-001에서 확정 |
| R6 | Hot-reload 시 registry race | 중 | 중 | `Replace(newMap)`가 map 복사 + 포인터 교체 (atomic). 호출자는 `Replace` 외부 진입로 미허용 |
| R7 | Progressive Disclosure 토큰 휴리스틱이 실제 tokenizer와 괴리 | 낮 | 낮 | `EstimateSkillFrontmatterTokens`는 상한 근사. 정확도는 soft guarantee |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/claude-primitives.md` §2 Skill System, §2.1 YAML Frontmatter, §2.2 Progressive Disclosure, §2.3 Trigger 4종, §2.4 Model Invocation 제약
- `.moai/specs/ROADMAP.md` §4 Phase 2 row 11 (SKILLS-001)
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` — consumer 인터페이스
- `.moai/project/tech.md` §1 (Go 오케스트레이션 계층)

### 9.2 외부 참조

- `gopkg.in/yaml.v3`: https://pkg.go.dev/gopkg.in/yaml.v3
- `github.com/denormal/go-gitignore`: gitignore 매칭
- Claude Code source map: `./claude-code-source-map/` (패턴만)
- MoAI-ADK `.claude/skills/`: SKILL.md 구조 선례

### 9.3 부속 문서

- `./research.md` — claude-primitives.md §2 원문 인용, Go 라이브러리 선택 근거, 테스트 매트릭스
- `../SPEC-GOOSE-QUERY-001/spec.md`
- `../SPEC-GOOSE-HOOK-001/spec.md`
- `../SPEC-GOOSE-SUBAGENT-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **skill의 의미 있는 실행(prompt 주입/agent spawn/shell 실행)을 구현하지 않는다**. 레지스트리 + 메타데이터 + 활성화 결정만.
- 본 SPEC은 **skill SDK나 작성 가이드를 포함하지 않는다**.
- 본 SPEC은 **MCP prompt → Skill 변환을 구현하지 않는다**. MCP-001의 책임.
- 본 SPEC은 **Hook 이벤트 라우터를 구현하지 않는다**. `FileChangedConsumer`는 순수 함수, dispatch는 HOOK-001.
- 본 SPEC은 **Remote skill의 AKI/GCS/OAuth 인증을 구현하지 않는다**. HTTP fetch 스켈레톤만.
- 본 SPEC은 **Skill hot-reload watcher를 구현하지 않는다**. PLUGIN-001.
- 본 SPEC은 **Slash command(!command) 전개를 구현하지 않는다**. COMMAND-001.
- 본 SPEC은 **`$ARG` 치환을 구현하지 않는다**. COMMAND-001이 argument 파싱 담당.
- 본 SPEC은 **env variable 치환을 지원하지 않는다** — 보안상 금지(REQ-SK-014).
- 본 SPEC은 **Marketplace UI/publish flow를 구현하지 않는다**. 별도 ROADMAP-ECOSYSTEM.

---

**End of SPEC-GOOSE-SKILLS-001**
