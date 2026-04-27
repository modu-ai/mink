---
id: SPEC-GOOSE-PLUGIN-001
version: 0.1.0
status: completed
completed: 2026-04-27
created_at: 2026-04-21
updated_at: 2026-04-27
author: manager-spec
priority: P1
issue_number: null
phase: 2
size: 중(M)
lifecycle: spec-anchored
labels: [phase-2, plugin, primitive, marketplace, manifest, priority/p1-high]
---

# SPEC-GOOSE-PLUGIN-001 — Plugin Host (manifest.json + MCPB + 4 Primitive 패키징)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-primitives §6 + 4 primitive SPEC 합의 기반) | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE의 **Plugin Host**를 정의한다. Claude Code의 `manifest.json` 중심 플러그인 아키텍처를 Go로 포팅하여, 하나의 plugin 디렉토리(또는 MCPB 번들 파일)에 Skills + Agents + MCP Servers + Slash Commands + Hooks **4 primitive를 한꺼번에 패키징**하고, 3단계 discovery(user / project / marketplace)로 로드하며, `clearThenRegister` atomic swap으로 hot-reload를 지원한다.

본 SPEC이 통과한 시점에서 `internal/plugin` 패키지는:

- `PluginManifest`(name, version, skills, agents, mcpServers, commands, hooks) JSON 스키마 파싱,
- 3-level discovery: `$HOME/.goose/plugins/` (user), `./.goose/plugins/` (project), 마켓플레이스 URI(Phase 5+ 스텁),
- Validator가 manifest 스키마 + reserved 이름 + hook event 이름 + primitive 존재성 검증,
- Walker가 각 primitive 하위 파일(`skills/*/SKILL.md`, `agents/*.md`, `commands/*.md`)을 4 primitive 패키지로 라우팅,
- `MCPBLoader`가 `.mcpb` 파일(DXT 매니페스트 + user config variables)을 읽어 MCP-001에 주입,
- `PluginRegistry.ClearThenRegister(snapshot)` atomic hot-reload,
- Plugin 활성화/비활성화 + `.goose/plugins.yaml` user config 연동.

본 SPEC은 **4 primitive 본체를 재구현하지 않으며**, 각 primitive 패키지(SKILLS-001/MCP-001/SUBAGENT-001/HOOK-001)의 **로드 entry point를 호출하는 조정자(orchestrator)** 역할만 수행한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Phase 2 4 primitive 중 마지막 퍼즐. 개별 primitive는 자체 SPEC으로 로드 가능하나, **plugin 개념이 없으면 생태계 확장 불가**(한 파일에 여러 primitive 묶기).
- `.moai/project/research/claude-primitives.md` §6이 Claude Code의 `manifest.json` 스키마 + loading pipeline + MCPB 파일을 제시한다. 본 SPEC은 그 구조를 Go로 확정.
- 4 primitive SPEC 모두 plugin consumer entry point를 `LoadFromPlugin(manifest)` 형태로 노출하므로, 본 SPEC이 이들을 오케스트레이트.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code TypeScript**: `utils/plugins/`. manifest schema validator + loader. 직접 포트 없음 — Zod validation은 Go struct tag + custom validator로 번역.
- **Anthropic MCPB (Model Context Protocol Bundle)**: DXT 매니페스트 포맷 + user config variable 치환. 공개 스펙 활용.
- **Hermes Agent Python**: Plugin 개념 없음. 본 SPEC 재사용 없음.

### 2.3 범위 경계

- **IN**: `PluginManifest` 스키마(JSON), 3-level discovery, manifest validation(이름 예약어, hook event 스키마), `LoadPlugin(path)` 오케스트레이터, 각 primitive entry point 호출(`skill.LoadPluginSkills`, `mcp.LoadPluginServers`, `subagent.LoadPluginAgents`, `hook.LoadPluginHooks`), `command.LoadPluginCommands`(COMMAND-001 스텁), MCPB 파일 파서(zip 해제 + DXT 매니페스트 + user config variables 치환), `PluginRegistry.ClearThenRegister` atomic swap, `.goose/plugins.yaml` 활성화 상태 관리, Plugin 버전 semver 검증.
- **OUT**: Plugin 자체의 코드 실행(각 primitive 소비), Plugin marketplace UI / publish flow, Signature 검증 / 코드 서명(Phase 5+), Plugin 샌드박스 실행(WASM 등), MCPB 저작 도구(외부), 자동 업데이트 메커니즘, 플러그인 의존성 해결(semver range).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/plugin/` 패키지.
2. 타입: `PluginManifest`, `PluginInstance`, `PluginSource`, `UserConfigVariable`.
3. `PluginManifest` JSON 스키마:
   - `name` (string, required),
   - `version` (semver, required),
   - `description` (string),
   - `skills` (array of `{id, path}`),
   - `agents` (array of `{id, path}`),
   - `mcpServers` (array of `MCPServerConfig` subset),
   - `commands` (array of `{name, description, path}`),
   - `hooks` (map of `HookEvent` → array of `{matcher?, hooks: [{command}]}`).
4. Discovery:
   - User level: `$HOME/.goose/plugins/<name>/manifest.json`,
   - Project level: `./.goose/plugins/<name>/manifest.json`,
   - Marketplace URI: Phase 5+ stub (`LoadPluginFromMarketplace(uri) → ErrNotImplemented`).
5. Validator:
   - name이 reserved list(`goose`, `claude`, `mcp`, `plugin`, `_*`)에 없음,
   - hook event 이름이 HOOK-001의 `HookEvent` 24개 중 하나,
   - 각 primitive path는 manifest 기준 상대경로 + walker가 실제 파일 존재 검증,
   - version이 valid semver (`Masterminds/semver` lib).
6. Walker:
   - `skills/*/SKILL.md` → `skill.LoadPluginSkills(manifest, pluginDir)`,
   - `agents/*.md` → `subagent.LoadPluginAgents(manifest, pluginDir)`,
   - `commands/*.md` → `command.LoadPluginCommands(manifest, pluginDir)` (COMMAND-001 스텁),
   - manifest `hooks` → `hook.LoadPluginHooks(manifest, pluginDir)`,
   - manifest `mcpServers` → `mcp.LoadPluginServers(manifest, pluginDir)`.
7. `MCPBLoader`:
   - `.mcpb`는 zip 파일(DXT 형식),
   - `manifest.json` + `dxt-manifest.json` + 기타 리소스,
   - `user_config_variables` 치환: `${VAR}` 패턴을 `plugins.yaml`의 사용자 값으로 치환.
8. `PluginRegistry`:
   - `Load(source, path) (*PluginInstance, error)`,
   - `ClearThenRegister(snapshot map[PluginID]*PluginInstance) error` atomic,
   - `List() []*PluginInstance`,
   - `Unload(id PluginID) error`.
9. 활성화 상태: `./.goose/plugins.yaml` — enabled plugins + user-provided variables.
10. Hot-reload: `ReloadAll()` → 전체 재로드 + `ClearThenRegister` atomic swap (primitive registry들도 동시에 swap되도록 단일 트랜잭션 조정).

### 3.2 OUT OF SCOPE

- **Primitive 본체 구현**: 각각 SKILLS/MCP/SUBAGENT/HOOK-001. 본 SPEC은 load entry point 호출만.
- **Plugin code execution**: 본 SPEC은 primitive를 **등록**만; 실행은 각 primitive의 런타임(QueryEngine, hook dispatcher 등).
- **Marketplace UI**: 별도 SPEC / 추후 Phase 7.
- **Plugin signing**: Phase 5+ Safety gates 이후.
- **WASM 샌드박스**: Phase 6+ Rust 통합 이후.
- **Dependency resolution**: plugin A가 plugin B의 skill을 참조하는 경우 본 SPEC은 로드 순서 보장 없음. 순환 의존 검증 없음.
- **Telemetry / usage metrics**: 추후 SPEC.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-PL-001 [Ubiquitous]** — Every `PluginManifest` **shall** have a valid `name` matching `^[a-z][a-z0-9-]{1,63}$` and a valid semver `version`; violations **shall** cause `ErrInvalidManifest` and the plugin **shall not** be loaded.

**REQ-PL-002 [Ubiquitous]** — Plugin names in the `PluginRegistry` **shall** be unique; loading a second plugin with an existing name **shall** return `ErrDuplicatePluginName` and leave the prior instance intact.

**REQ-PL-003 [Ubiquitous]** — The `ClearThenRegister` operation across primitive registries (SKILLS-001, SUBAGENT-001, HOOK-001, MCP-001) **shall** be coordinated so that observers either see all pre-reload snapshots or all post-reload snapshots; partial state exposure **shall not** occur.

**REQ-PL-004 [Ubiquitous]** — Every hook entry in `manifest.hooks` **shall** reference a `HookEvent` constant defined in HOOK-001's `HookEventNames()`; unknown events **shall** cause `ErrUnknownHookEvent` during manifest validation.

### 4.2 Event-Driven (이벤트 기반)

**REQ-PL-005 [Event-Driven]** — **When** `LoadPlugin(source, path)` is invoked, the loader **shall** (a) read `manifest.json`, (b) parse into `PluginManifest` with strict field checking, (c) run all validators (name, version, hook events, primitive paths), (d) walk each primitive's files and invoke the primitive's `LoadPluginXxx` entry point, (e) assemble a `PluginInstance` with the primitive-registered handles, (f) return the instance.

**REQ-PL-006 [Event-Driven]** — **When** `LoadMCPB(path)` is invoked on a `.mcpb` file, the loader **shall** (a) unzip into a temp directory, (b) read `dxt-manifest.json` + `manifest.json`, (c) substitute `${USER_CONFIG_VAR}` patterns using values from `plugins.yaml` `userConfigVariables`, (d) delegate to `LoadPlugin` on the temp directory, (e) on success, retain the temp dir for lifetime of the plugin and clean up on unload.

**REQ-PL-007 [Event-Driven]** — **When** `ReloadAll()` is invoked, the host **shall** (a) scan all discovery paths, (b) build a new snapshot of `PluginInstance` map, (c) for each primitive, compute merged snapshot (inline + all plugins), (d) call `ClearThenRegister` on each primitive's registry with the merged snapshot — in a fixed order(Skills → Agents → Hooks → MCP → Commands), (e) atomically update its own registry last.

**REQ-PL-008 [Event-Driven]** — **When** a plugin's `enabled` flag in `plugins.yaml` is set to `false`, `LoadPlugin` **shall** skip that plugin; enabling it later via `plugins.yaml` edit **shall** require explicit `ReloadAll()` invocation.

**REQ-PL-009 [Event-Driven]** — **When** MCPB file parsing encounters a `${VAR}` that has no value in `userConfigVariables`, the loader **shall** return `ErrMissingUserConfigVariable` with the variable name; substitution **shall not** default to empty string or env variable.

### 4.3 State-Driven (상태 기반)

**REQ-PL-010 [State-Driven]** — **While** `PluginRegistry.IsLoading == true`, any `HookRegistry.Register` call from plugin primitives **shall** be staged rather than committed; commit happens only after all primitive loads succeed, as part of the atomic `ClearThenRegister`.

**REQ-PL-011 [State-Driven]** — **While** a plugin is in state `Loading` (primitive registration in progress) and an error occurs on any primitive, the partial registrations **shall** be rolled back — no visible state change to any primitive registry.

### 4.4 Unwanted Behavior (방지)

**REQ-PL-012 [Unwanted]** — The loader **shall not** execute any plugin-provided shell commands at load time; shell commands declared in `manifest.hooks[*].hooks[*].command` are data only, executed later by HOOK-001's dispatcher at runtime.

**REQ-PL-013 [Unwanted]** — Plugin manifests **shall not** declare `mcpServers` entries pointing to URIs containing credentials (`https://user:pass@host`); such URIs **shall** cause `ErrCredentialsInURI` during validation.

**REQ-PL-014 [Unwanted]** — MCPB zip files **shall not** be allowed to contain symlinks or path components that escape the target temp directory (zip slip); the extractor **shall** reject such entries with `ErrZipSlip`.

**REQ-PL-015 [Unwanted]** — `LoadPlugin` **shall not** register any primitive if manifest validation fails; all-or-nothing — no partial registration.

**REQ-PL-016 [Unwanted]** — Plugin names reserved by the host **shall not** be loaded: `goose`, `claude`, `mcp`, `plugin`, `_*` (underscore prefix); such names cause `ErrReservedPluginName`.

### 4.5 Optional (선택적)

**REQ-PL-017 [Optional]** — **Where** `manifest.permissions` declares required permissions (e.g., `network`, `filesystem:project`), the loader **shall** record them in `PluginInstance.Permissions`; enforcement is deferred to primitive runtimes (future SPEC).

**REQ-PL-018 [Optional]** — **Where** `plugins.yaml:marketplace.enabled == true`, `LoadPluginFromMarketplace(uri)` **shall** attempt HTTP fetch of the manifest; for v0.1 this returns `ErrNotImplemented` but the code path exists.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-PL-001 — 최소 manifest 로드**
- **Given** `/tmp/plugins/mini/manifest.json`에 `{"name":"mini","version":"1.0.0"}`, 빈 skills/agents/hooks/mcpServers
- **When** `LoadPlugin(Source:"project", path:"/tmp/plugins/mini")`
- **Then** `PluginInstance{Name:"mini", Version:"1.0.0"}`, 모든 primitive의 LoadPluginXxx가 빈 결과 반환, 에러 없음

**AC-PL-002 — 4 primitive 완전 로드**
- **Given** manifest가 skills/agents/mcpServers/commands/hooks 각 1개 이상 포함, 파일 실존
- **When** `LoadPlugin`
- **Then** SKILLS-001 registry에 plugin skill 1개 등록, SUBAGENT-001의 agent 목록에 1개, HOOK-001 registry에 hook handler 1개, MCP-001 connection config 큐에 1개, COMMAND-001 stub에 1개

**AC-PL-003 — Reserved 이름 거부**
- **Given** manifest `{"name":"_evil", ...}`
- **When** `LoadPlugin`
- **Then** `ErrReservedPluginName` 반환, 어떤 primitive에도 등록 없음

**AC-PL-004 — Unknown hook event 거부**
- **Given** manifest `{"hooks":{"FrobnicateStart":[...]}}` (24 event 아님)
- **When** `LoadPlugin`
- **Then** `ErrUnknownHookEvent{event: "FrobnicateStart"}` 반환, 로드 실패

**AC-PL-005 — 중복 plugin 이름**
- **Given** 동일 `name: "foo"`인 두 manifest (user + project level)
- **When** 순차 `LoadPlugin`
- **Then** 두 번째 호출이 `ErrDuplicatePluginName` 반환, 첫 번째 instance는 그대로 유지

**AC-PL-006 — MCPB 파일 로드 + user config 치환**
- **Given** `plugin.mcpb` (zip), dxt-manifest `{"userConfigVariables":[{"name":"API_KEY","required":true}]}`, `plugins.yaml`에 `userConfigVariables: {API_KEY: "xyz"}`, manifest에 `${API_KEY}` 포함
- **When** `LoadMCPB("plugin.mcpb")`
- **Then** 치환 완료된 manifest로 정상 로드, 런타임 config에 `xyz` 반영

**AC-PL-007 — MCPB 미제공 user config**
- **Given** MCPB의 `API_KEY`가 required인데 `plugins.yaml`에 미지정
- **When** `LoadMCPB`
- **Then** `ErrMissingUserConfigVariable{name:"API_KEY"}` 반환, 임시 디렉토리 cleanup

**AC-PL-008 — MCPB zip slip 공격 차단**
- **Given** 악의적 `.mcpb`가 `../../etc/evil` 경로 포함
- **When** `LoadMCPB`
- **Then** `ErrZipSlip` 반환, 대상 디렉토리 외부 파일은 생성되지 않음

**AC-PL-009 — Atomic ClearThenRegister**
- **Given** 3개 plugin 로드됨, `ReloadAll` 실행 중 manifest 하나를 삭제
- **When** 동시에 다른 goroutine이 `skill.Registry.Get("pluginA-skill")` 호출
- **Then** race-free, observer는 pre-reload(3 plugin) 또는 post-reload(2 plugin) 중 하나의 완전한 상태만 관찰

**AC-PL-010 — Credentials in URI 거부**
- **Given** manifest `{"mcpServers":[{"uri":"https://user:secret@host.com/mcp"}]}`
- **When** `LoadPlugin`
- **Then** `ErrCredentialsInURI` 반환, 플러그인 로드 실패

**AC-PL-011 — enabled=false skip**
- **Given** `plugins.yaml`에 `{foo: {enabled: false}}`, `/tmp/plugins/foo/manifest.json` 존재
- **When** `ReloadAll()`
- **Then** foo plugin은 `PluginRegistry.List()`에 미포함, 다른 plugin은 정상 로드

**AC-PL-012 — Primitive load 중 실패 롤백**
- **Given** manifest가 skill 2개, agent 1개를 선언; 두 번째 skill 파일이 손상(YAML 에러)
- **When** `LoadPlugin`
- **Then** 첫 번째 skill, agent, hook 모두 미등록 (rollback); 에러 반환; 이전 registry 상태 유지

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/
└── plugin/
    ├── manifest.go          # PluginManifest 스키마 + JSON 파서
    ├── validator.go         # name/version/hook event/URI 검증
    ├── walker.go            # 각 primitive 하위 파일 discovery
    ├── loader.go            # LoadPlugin 오케스트레이터
    ├── mcpb.go              # MCPB 파일 파서 (zip + DXT + var 치환)
    ├── mcp_integration.go   # mcp.LoadPluginServers 호출
    ├── registry.go          # PluginRegistry atomic swap
    ├── config.go            # plugins.yaml 로더
    └── *_test.go
```

### 6.2 핵심 Go 타입

```go
type PluginSource int

const (
    SourceUser PluginSource = iota  // ~/.goose/plugins/
    SourceProject                   // ./.goose/plugins/
    SourceMarketplace               // remote URI (v1.0+)
)

type PluginID string   // = plugin.Name

type PluginManifest struct {
    Name        string                        `json:"name"`
    Version     string                        `json:"version"`
    Description string                        `json:"description,omitempty"`
    Skills      []PluginSkillRef              `json:"skills,omitempty"`
    Agents      []PluginAgentRef              `json:"agents,omitempty"`
    MCPServers  []PluginMCPServerConfig       `json:"mcpServers,omitempty"`
    Commands    []PluginCommandRef            `json:"commands,omitempty"`
    Hooks       map[string][]PluginHookGroup  `json:"hooks,omitempty"`
    Permissions []string                      `json:"permissions,omitempty"`
}

type PluginSkillRef struct {
    ID   string `json:"id"`
    Path string `json:"path"`  // manifest dir 상대 경로
}

type PluginAgentRef struct {
    ID   string `json:"id"`
    Path string `json:"path"`
}

type PluginMCPServerConfig struct {
    Name      string                       `json:"name"`
    Transport string                       `json:"transport"`
    Command   string                       `json:"command,omitempty"`
    Args      []string                     `json:"args,omitempty"`
    Env       map[string]string            `json:"env,omitempty"`
    URI       string                       `json:"uri,omitempty"`
}

type PluginCommandRef struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Path        string `json:"path"`
}

type PluginHookGroup struct {
    Matcher string              `json:"matcher,omitempty"`
    Hooks   []PluginHookEntry   `json:"hooks"`
}

type PluginHookEntry struct {
    Command string `json:"command"`
    Timeout int    `json:"timeout,omitempty"`
}

type PluginInstance struct {
    ID              PluginID
    Source          PluginSource
    BaseDir         string                  // plugin 파일 루트
    Manifest        PluginManifest
    LoadedSkills    []string                // skill IDs
    LoadedAgents    []string                // agent types
    LoadedHooks     []HookBindingHandle
    LoadedMCP       []string                // server IDs
    LoadedCommands  []string
    Permissions     []string
    TempDir         string                  // MCPB extraction
}

type UserConfigVariable struct {
    Name     string
    Required bool
    Default  *string
}

// Registry.
type PluginRegistry struct {
    mu       sync.Mutex
    current  atomic.Pointer[map[PluginID]*PluginInstance]
    logger   *zap.Logger
    loading  atomic.Bool
}

func (r *PluginRegistry) Load(src PluginSource, path string) (*PluginInstance, error)
func (r *PluginRegistry) LoadMCPB(path string) (*PluginInstance, error)
func (r *PluginRegistry) ClearThenRegister(snapshot map[PluginID]*PluginInstance) error
func (r *PluginRegistry) ReloadAll() error
func (r *PluginRegistry) Unload(id PluginID) error
func (r *PluginRegistry) List() []*PluginInstance

// Reserved names.
var reservedPluginNames = map[string]struct{}{
    "goose": {}, "claude": {}, "mcp": {}, "plugin": {},
}
```

### 6.3 Loading Pipeline (REQ-PL-005)

```
LoadPlugin(source, path)
  │
  ├─ [1] Read manifest.json
  ├─ [2] Parse → PluginManifest (strict JSON)
  ├─ [3] Validators:
  │    ├─ name format regex
  │    ├─ semver version
  │    ├─ reserved name check
  │    ├─ hook event check (HOOK-001.HookEventNames())
  │    ├─ mcpServer URI credentials check
  │    └─ permissions set subset check
  ├─ [4] Walker:
  │    ├─ foreach skill → resolve path, read, call skill.LoadPluginSkills (staged)
  │    ├─ foreach agent → resolve path, read, call subagent.LoadPluginAgents (staged)
  │    ├─ foreach mcpServer → call mcp.LoadPluginServers (staged)
  │    ├─ foreach command → call command.LoadPluginCommands (staged, COMMAND-001 stub)
  │    └─ hooks map → call hook.LoadPluginHooks (staged)
  ├─ [5] If all staged: call each primitive's Commit() (atomic)
  │   Else: call Rollback() on all staged, return error
  └─ [6] Return PluginInstance
```

**Staged/Commit/Rollback 패턴**: 각 primitive의 Load 인터페이스는 `(staged, commit, rollback)` 3-함수를 반환하여 본 SPEC이 트랜잭션처럼 orchestrate.

### 6.4 MCPB 파일 파서

```
.mcpb = zip 파일
├─ manifest.json          # PluginManifest (본 SPEC)
├─ dxt-manifest.json      # Anthropic DXT 확장 (user config vars)
├─ skills/*.md
├─ agents/*.md
└─ (기타 리소스)
```

**User Config Variable 치환**:

1. `dxt-manifest.json`의 `userConfigVariables: [{name, required, default?}]` 읽기.
2. `plugins.yaml`의 `userConfigVariables.<pluginName>.<varName>` 값 읽기.
3. 누락 required → `ErrMissingUserConfigVariable`.
4. `manifest.json` + 모든 텍스트 파일에서 `${VAR}` 패턴을 사용자 값으로 치환.
5. 치환 결과를 temp dir에 저장.
6. `LoadPlugin(temp_dir)` 위임.

**Zip Slip 방지** (REQ-PL-014):

```go
targetPath := filepath.Join(tempDir, header.Name)
if !strings.HasPrefix(targetPath, tempDir+string(os.PathSeparator)) {
    return ErrZipSlip
}
```

### 6.5 plugins.yaml 스키마

```yaml
plugins:
  foo:
    enabled: true
    source: user          # user | project | marketplace
    userConfigVariables:
      API_KEY: "secret-123"
      ENDPOINT: "https://api.example.com"
  bar:
    enabled: false
marketplace:
  enabled: false
  registries:
    - https://plugins.goose.ai/
```

CONFIG-001의 loader가 1차 파싱, 본 SPEC의 `config.go`가 2차 validation + `PluginRegistry`에 주입.

### 6.6 Atomic Multi-Registry ClearThenRegister

4 primitive registry(Skills/Subagent/Hook/MCP)가 모두 atomic swap을 제공하지만, **cross-registry 일관성**이 과제. 본 SPEC의 전략:

1. 순서: Skills → Agents → Hooks → MCP → Commands → Plugin (본 registry 자체).
2. 각 swap은 atomic.Pointer 기반 lock-free read.
3. Observer가 "Skills에는 A, Agents에는 B" 같은 crossgap 관찰 가능성 존재 — 이는 허용됨(REQ-PL-003의 "observers either see all pre-reload snapshots or all post-reload snapshots"는 **per-registry** 보장).
4. 본 SPEC은 cross-registry transactional isolation은 **약속하지 않음** (문서에 명시). 필요 시 Phase 5+ 에서 2PC.

### 6.7 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| JSON 파싱 (manifest) | stdlib `encoding/json` + strict mode | 표준 |
| YAML (plugins.yaml) | `gopkg.in/yaml.v3` | CONFIG-001 공유 |
| Semver | `github.com/Masterminds/semver/v3` | 산업 표준 |
| Zip 해제 (MCPB) | stdlib `archive/zip` | 표준 |
| 문자열 치환 (${VAR}) | `text/template` 또는 직접 regex | regex 채택(의존성 최소화) |
| 로깅 | `go.uber.org/zap` | CORE-001 공유 |
| Atomic pointer | stdlib `sync/atomic` | 표준 |

### 6.8 TDD 진입 순서

1. **RED #1** — `TestPluginManifest_Parse_Minimal` (AC-PL-001)
2. **RED #2** — `TestValidator_ReservedName_Rejected` (AC-PL-003)
3. **RED #3** — `TestValidator_UnknownHookEvent` (AC-PL-004)
4. **RED #4** — `TestValidator_CredentialsInURI` (AC-PL-010)
5. **RED #5** — `TestRegistry_DuplicatePluginName` (AC-PL-005)
6. **RED #6** — `TestLoader_LoadsAllPrimitives` (AC-PL-002)
7. **RED #7** — `TestLoader_RollbackOnPartialFailure` (AC-PL-012)
8. **RED #8** — `TestMCPB_Unzip_UserConfigSubstitution` (AC-PL-006)
9. **RED #9** — `TestMCPB_MissingRequiredVar` (AC-PL-007)
10. **RED #10** — `TestMCPB_ZipSlip_Rejected` (AC-PL-008)
11. **RED #11** — `TestRegistry_ReloadAll_Atomic` (AC-PL-009)
12. **RED #12** — `TestConfig_EnabledFalse_Skip` (AC-PL-011)
13. **GREEN** — manifest + validator + loader + MCPB.
14. **REFACTOR** — staged/commit/rollback 공통 코드 추출(generic `Transaction[T]`).

### 6.9 TRUST 5 매핑

| 차원 | 본 SPEC 달성 방법 |
|-----|-----------------|
| **T**ested | 25+ unit test, 12 integration test (AC 1:1), MCPB fixture zip, race detector |
| **R**eadable | manifest/validator/walker/loader/mcpb 5 파일 분리 |
| **U**nified | `go fmt`, `golangci-lint` (gosec 추가 — zip slip, URI credential) |
| **S**ecured | Reserved names, URI credentials, zip slip, ReloadAll atomic, 로드 시 실행 없음 |
| **T**rackable | plugin ID 기반 zap 로그, 로드 실패 시 full error chain |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-SKILLS-001 | `skill.LoadPluginSkills(manifest, dir) (staged, commit, rollback)` entry point |
| 선행 SPEC | SPEC-GOOSE-MCP-001 | `mcp.LoadPluginServers` entry point |
| 선행 SPEC | SPEC-GOOSE-SUBAGENT-001 | `subagent.LoadPluginAgents` entry point |
| 선행 SPEC | SPEC-GOOSE-HOOK-001 | `hook.LoadPluginHooks` + `HookEventNames()` |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `plugins.yaml` loader, feature gate |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap 로거, context 루트 |
| 후속 SPEC | SPEC-GOOSE-COMMAND-001 | `command.LoadPluginCommands` entry point (본 SPEC은 stub 호출) |
| 외부 | Go 1.22+ | generics, atomic.Pointer |
| 외부 | `github.com/Masterminds/semver/v3` | 버전 비교 |
| 외부 | `gopkg.in/yaml.v3` v3.0+ | plugins.yaml |
| 외부 | `go.uber.org/zap` v1.27+ | |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 4 primitive entry point가 본 SPEC 기대 시그니처와 다름 | 중 | 고 | 본 SPEC에서 `LoadPluginXxx(manifest, dir) (staged, commit, rollback)` 명시. 각 primitive SPEC의 §7 "후속 SPEC 의존성"에서 인터페이스 확정 |
| R2 | Cross-registry 부분 swap의 관찰 가능성이 문제되는 사용자 시나리오 | 낮 | 중 | 본 SPEC은 per-registry atomic만 약속. Cross-registry 2PC는 Phase 5+. 대부분 소비자(QueryEngine)는 한 번에 한 registry만 읽음 |
| R3 | MCPB 치환이 `${VAR}`가 코드 내 문자열과 충돌 | 중 | 중 | 치환 대상 파일 타입 제한: manifest.json + `*.md` + `*.yaml` 만. 바이너리/zip 내 파일은 미치환 |
| R4 | Plugin 수 50+ 시 `ReloadAll` 소요 시간 증가 | 중 | 낮 | parallel load 최적화는 REFACTOR 단계. MVP는 순차 로드 |
| R5 | plugins.yaml 손상 시 부팅 실패 | 중 | 고 | yaml 파싱 에러 시 빈 plugin state로 부팅 + WARN (daemon 구동 우선) |
| R6 | Semver v3 라이브러리 브레이킹 체인지 | 낮 | 낮 | `Masterminds/semver/v3` v3.x는 안정. pin |
| R7 | MCPB temp dir 누수 (unload 실패) | 중 | 중 | `PluginInstance.TempDir`를 `os.RemoveAll`로 청소. startup scan으로 orphan 제거 |
| R8 | 악의적 plugin이 reserved name으로 impersonation 시도 | 낮 | 고 | REQ-PL-016 strict 검증. Phase 5+ 서명 검증 도입 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/claude-primitives.md` §6 Plugin 매니페스트 스키마, Plugin Loading Pipeline, MCPB 파일
- `.moai/specs/ROADMAP.md` §4 Phase 2 row 15 (PLUGIN-001)
- `.moai/specs/SPEC-GOOSE-SKILLS-001/spec.md` — LoadPluginSkills 계약
- `.moai/specs/SPEC-GOOSE-MCP-001/spec.md` — LoadPluginServers
- `.moai/specs/SPEC-GOOSE-SUBAGENT-001/spec.md` — LoadPluginAgents
- `.moai/specs/SPEC-GOOSE-HOOK-001/spec.md` — LoadPluginHooks + HookEventNames

### 9.2 외부 참조

- Anthropic DXT(MCPB) 스펙: https://github.com/anthropics/dxt
- Semver spec: https://semver.org/spec/v2.0.0.html
- Claude Code source map: `./claude-code-source-map/` (`utils/plugins/` 패턴만)
- Go `archive/zip` 보안 가이드(Zip Slip): https://snyk.io/research/zip-slip-vulnerability

### 9.3 부속 문서

- `./research.md` — claude-primitives.md §6 원문 + MCPB DXT 스펙 + cross-registry 일관성 분석
- `../SPEC-GOOSE-SKILLS-001/spec.md`
- `../SPEC-GOOSE-MCP-001/spec.md`
- `../SPEC-GOOSE-SUBAGENT-001/spec.md`
- `../SPEC-GOOSE-HOOK-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **4 primitive 본체를 재구현하지 않는다**. 각 primitive의 `LoadPluginXxx` entry point 호출만.
- 본 SPEC은 **Plugin marketplace UI / publish flow를 구현하지 않는다**. Phase 7 또는 별도 SPEC.
- 본 SPEC은 **Plugin signing / 코드 서명 검증을 구현하지 않는다**. Phase 5+ SAFETY 이후.
- 본 SPEC은 **WASM 샌드박스를 구현하지 않는다**. Phase 6+ Rust 통합.
- 본 SPEC은 **Plugin 간 의존성 해결을 구현하지 않는다**. 로드 순서 보장 없음, 순환 의존 미검증.
- 본 SPEC은 **자동 업데이트 / 버전 migration을 구현하지 않는다**. 사용자 수동 업데이트.
- 본 SPEC은 **Cross-registry 2PC transactional swap을 구현하지 않는다**. per-registry atomic만 보장; cross-registry 부분 관찰 허용.
- 본 SPEC은 **Plugin permissions 집행을 구현하지 않는다**. `Permissions` 필드 저장만; 집행은 primitive 런타임 책임(후속 SPEC).
- 본 SPEC은 **MCPB 저작 도구를 포함하지 않는다**. 외부 도구.
- 본 SPEC은 **Plugin 사용 통계 / telemetry를 수집하지 않는다**.

---

## Implementation Notes (sync 정합화 2026-04-27)

- **Status Transition**: planned → implemented
- **Package**: `internal/plugin/` (19 파일)
- **Core**: `manifest.go`(`ParseManifestFile` — fan_in ≥ 3 단일 진입점), `mcpb.go`(`extractMCPB` zip 해제), `loader.go`(4-primitive 순차 로드 + 실패 시 전체 롤백), `registry.go`(`ClearThenRegister` atomic swap), `validator.go`, `types.go`, `errors.go`, `config.go`, `hook_handler.go`
- **Verified REQs (spot-check)**: REQ-PL-005 manifest 파싱 진입점, MCPB 번들 zip 해제, 4-primitive(Skills+Agents+MCP+Hooks) 통합 로딩, REQ-PL-002/003 PluginRegistry per-instance atomic swap (`atomic.Pointer[map[PluginID]*PluginInstance]` + `sync.Mutex`로 lock-free 읽기 / mu 보호 쓰기) — atomicity 보장 범위는 단일 PluginRegistry 인스턴스 한정, cross-registry는 globally atomic 아님
- **Test Coverage**: 8+ `_test.go` 파일 (manifest, mcpb, loader, registry, swap, validator, config, coverage)
- **Lifecycle**: spec-anchored Level 2 — v0.1.0 초안 frontmatter 그대로지만 코드 충실도 v0.2급 수준

---

**End of SPEC-GOOSE-PLUGIN-001**
