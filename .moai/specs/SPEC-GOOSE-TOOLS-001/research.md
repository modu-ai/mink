# SPEC-GOOSE-TOOLS-001 — Research & Inheritance Analysis

> **목적**: Tool Registry + ToolSearch + 내장 tool 6종 구현의 자산 조사, deferred loading 전략 결정, permission matcher 구문 선택 근거 정리.
> **작성일**: 2026-04-21

---

## 1. 레포 상태

- `internal/tools/` 부재. 본 SPEC은 **신규 작성**.
- 기존 `SPEC-GOOSE-AGENT-001` (DEPRECATED)에 일부 tool 아이디어 스케치 있음 — 폐기 대상이므로 참고만.

---

## 2. 참조 자산별 분석

### 2.1 Claude Code — `tools/` 디렉토리 (원문 인용)

`.moai/project/research/claude-primitives.md` §3.4:

```
### 3.4 Deferred Loading (ToolSearch)

1. `connectToServer()` 호출 (도구 매니페스트 미로드)
2. 모델이 "mcp__..." 도구 언급 또는 MCPTool 직접 호출
3. Late binding: `fetchToolsForClient()`
4. 캐싱: memoize로 재연결 방지

**이름 충돌 해결**: `mcp__{serverName}__{toolName}` 접두사, 충돌 감지 + 넘버링
```

`.moai/project/research/claude-primitives.md` §2.4:

```
SkillTool.checkPermissions:
1. deny 규칙 검사 ("skill-name" 또는 "prefix:*")
2. disableModelInvocation 플래그 체크
3. SAFE_SKILL_PROPERTIES allowlist (새 프로퍼티 기본 deny)
4. allow 규칙 또는 자동 허가
```

**본 SPEC 계승**:
- Prefix 규약 `mcp__{serverID}__{toolName}` → REQ-TOOLS-004
- Deferred loading via lazy activate → REQ-TOOLS-007/008
- Memoize (sync.Once + atomic.Pointer) → §6.4 구현 전략
- Deny-before-allow 순서 → Permission Matcher 구현 (GlobMatcher는 deny 우선)
- `disableModelInvocation` 대응 → `Scope` enum (`ScopeLeaderOnly`는 모델에 미노출)

### 2.2 Claude Code — `tools/AgentTool`, `tools/MCPTool`

원문 정확 참조 부재. `.moai/project/research/claude-primitives.md` §3.1:

```
**클라이언트** (Claude Code → 외부 MCP 서버):
- `mcpClient.ts`: SSE, WebSocket, stdio 전송
- `connectToServer()` memoize (연결 풀링)
- `fetchToolsForClient()`: 도구 매니페스트 fetch (deferred)
```

**본 SPEC에서의 경계**: `mcpClient.ts`의 transport/OAuth는 MCP-001. 본 SPEC은 `fetchToolsForClient()`의 **캐싱 전략**만 채택 (Search.cache).

### 2.3 Hermes — `cli.py` + `model_tools.py` (인용)

Hermes는 Python으로 단일 cli.py에 tool inventory를 하드코딩:

```python
# hermes-agent-main/model_tools.py (대략 구조, 실제 소스 확인 불가)
TOOL_REGISTRY = {
    "read_file": read_file_tool,
    "write_file": write_file_tool,
    ...
}

def dispatch_tool(name, input):
    if name not in TOOL_REGISTRY:
        return {"error": "tool_not_found"}
    return TOOL_REGISTRY[name](input)
```

**본 SPEC 계승**:
- **Auto-registry inventory 패턴**: 각 tool 파일이 `init()` 에서 `builtin.Register(NewXxx())` 호출 → import하면 자동 등록. Go는 이 패턴에 최적화됨.
- **Dispatch 단순성**: `map[string]Tool` lookup + call. 복잡한 reflection 없음.

**본 SPEC에서의 차이**:
- Go interface 기반이므로 타입 안전. Python dict[str, callable]보다 schema 강제 가능.
- Hermes는 permission 없음 (single-user 전제). 본 SPEC은 `PermissionMatcher` 계층 추가.

### 2.4 기존 SPEC-GOOSE-QUERY-001 (소비자 계약)

§6.2:
```go
type QueryEngineConfig struct {
    ...
    Tools      tools.Registry   // TOOLS-001 제공
    ...
}
```

REQ-QUERY-006 (permission gate):
```
When the LLM response contains a tool_use content block, the queryLoop shall
invoke CanUseTool(...) and dispatch based on Allow/Deny/Ask.
```

REQ-QUERY-007 (budget replacement):
```
When a tool returns a result whose serialized content exceeds
taskBudget.toolResultCap bytes, the queryLoop shall apply tool-result budget replacement.
```

**본 SPEC의 책임 명확화**:
- `Registry`, `Executor` 타입 소유.
- `CanUseTool` 자체는 QUERY-001의 `internal/permissions` 소유. TOOLS-001의 `Executor`는 이를 **주입받아 호출**만.
- Budget replacement 로직 본체는 본 SPEC (`budget.go`), 호출 위치는 QUERY-001.

### 2.5 structure.md §172-211 (기대 레이아웃)

```
├── tools/                  # 내장 도구 (Claude Code 계승)
│   ├── registry.go         # Auto-registration (inventory)
│   ├── validator.go        # 입력 검증
│   ├── executor.go         # 도구 실행
│   │
│   ├── file/               ├── terminal/      ├── web/
│   ├── code/               ├── agent/         ├── memory/
│   └── vision/
```

**Phase 3 scope 축소**:
- 본 SPEC은 `file/`, `terminal/`만 구현. 나머지(`web/`, `code/`, `agent/`, `memory/`, `vision/`)는 후속 SPEC.
- `validator.go`는 `tool.Schema()` 기반 JSON Schema validation으로 통합 (별도 파일 불필요).

---

## 3. 외부 라이브러리 결정 매트릭스

### 3.1 JSON Schema Validation

| 라이브러리 | 버전 | draft 2020-12 | 유지 상태 | 의존성 | 채택 |
|----------|------|---------------|----------|-------|-----|
| `santhosh-tekuri/jsonschema/v6` | v6+ | ✅ full | active (2025) | stdlib only | ✅ |
| `xeipuuv/gojsonschema` | 1.2 | ❌ draft-07만 | stale (2022) | stdlib only | ❌ |
| `qri-io/jsonschema` | 0.2 | ⚠️ partial | slow | stdlib | ❌ |
| `invopop/jsonschema` | 0.12 | — (struct→schema) | active | reflect | ❌ (역방향 용도) |

**선택**: `santhosh-tekuri/jsonschema/v6`. draft 2020-12 완전 지원, 의존성 최소, 활발한 유지보수.

### 3.2 Glob 매처 (permission + Glob tool)

| 라이브러리 | 버전 | `**` 지원 | 의존성 | 채택 |
|----------|------|----------|-------|-----|
| `bmatcuk/doublestar/v4` | v4.6+ | ✅ | stdlib only | ✅ |
| 표준 `path/filepath` Match | - | ❌ (단일 레벨만) | stdlib | ❌ |
| `gobwas/glob` | 0.2 | ✅ | stdlib | ⚠️ gitignore 문법 다름 |

**선택**: `doublestar/v4`. gitignore 패턴(`**/*.go`) 지원, Cobra/많은 프로젝트에서 검증됨.

### 3.3 Subprocess 관리 (Bash tool)

| 라이브러리 | 용도 | 채택 |
|----------|-----|-----|
| 표준 `os/exec` | 기본 subprocess | ✅ |
| 표준 `syscall` | process group kill | ✅ (Setpgid + Kill(-pgid)) |
| `creack/pty` | PTY 할당 | ❌ Phase 3 제외 (CLI-001 TUI에서 필요 시) |
| `mitchellh/go-ps` | 프로세스 탐색 | ❌ 미사용 |

**Timeout kill 전략**:
```go
cmd := exec.CommandContext(ctx, "sh", "-c", userCmd)
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
cmd.Start()
// timeout: syscall.Kill(-cmd.Process.Pid, SIGTERM) → sleep 2s → SIGKILL
```

### 3.4 Grep/Regex

표준 `regexp` 패키지 사용. ripgrep 바이너리 의존은 Phase 3 제외 — 외부 바이너리 설치 전제는 프로젝트 이식성 저해.

성능 우려는 Phase 3 범위 밖 (현재는 기능성 우선). 성능 최적화는 향후 `grep/` 서브모듈에서 별도 백엔드 플러그인으로 처리.

---

## 4. Permission Matcher 구문 결정 근거

### 4.1 대안 비교

| 구문 | 예시 | 장점 | 단점 |
|------|------|------|-----|
| A) Plain ToolName | `"Bash"` | 간단 | 인자별 구분 불가 |
| B) ToolName + glob | `"Bash(git *)"` | Claude Code settings.json과 동형 | parser 필요 |
| C) ToolName + JSONPath | `"Bash:$.command == 'git status'"` | 정밀 | 복잡도 |
| D) YAML 객체 | `{tool: Bash, command: "git *"}` | 가독성 | config 구조 변경 |

**선택**: (A) + (B) 조합. Claude Code 사용자 친화성(`.claude/settings.json` `permissions.allow` 포맷 계승).

### 4.2 arg-pattern 해석 테이블

| 패턴 | Primary field | 적용 대상 |
|------|--------------|---------|
| `Bash(git *)` | `input.command` | `Bash` tool |
| `FileRead(/tmp/**)` | `input.path` | `FileRead` tool |
| `FileWrite(/tmp/**)` | `input.path` | `FileWrite` tool |
| `FileEdit(src/**/*.go)` | `input.path` | `FileEdit` tool |
| `Glob(src/**)` | `input.pattern` | `Glob` tool |
| `Grep(.*)` | `input.pattern` | `Grep` tool |
| `mcp__github__*` | (unchecked) | 모든 github MCP tool |
| `mcp__github__create_issue(...)` | first string field | github/create_issue만 |

**안전 기본**: 알려지지 않은 tool에 대한 `ToolName(pattern)` 구문은 `false` 반환. 사용자가 schema hint를 명시적으로 등록해야 매칭.

### 4.3 Deny precedence

settings.json 샘플:
```yaml
permissions:
  allow:
    - "Bash(git *)"
  deny:
    - "Bash(git push --force *)"
    - "Bash(rm -rf *)"
```

Deny 매칭이 allow 매칭보다 우선 (안전 기본). Matcher는 deny 먼저 순회 → 매칭 시 immediate false. 그 다음 allow.

---

## 5. Deferred Loading 구현 전략

### 5.1 세 가지 대안

| 전략 | 방식 | 장점 | 단점 |
|------|------|------|-----|
| Eager | 연결 즉시 전 tool fetch | 단순, Resolve 빠름 | 초기 지연, 미사용 tool도 fetch |
| Lazy stub | Resolve시 stub Tool, Call시 fetch | 초기 부담 없음 | stub/real 전환 복잡 |
| Lazy registry-level | Resolve에서 fetch | Resolve가 blocking | QUERY-001 Resolve는 non-blocking 전제 |

**선택**: **Lazy stub** (전략 2). 이유:
- QUERY-001 REQ-QUERY-006는 `CanUseTool.Check`를 async 호출 가능하도록 명시. 본 SPEC의 `Tool.Call`도 동일 계약(async OK)이므로 fetch가 `Call` 내부에서 수행되어도 의미론 유지.
- `sync.Once` + `atomic.Pointer[T]`로 lock-free replacement 가능.
- Resolve는 non-blocking (stub 반환) → 모델 시스템 프롬프트 생성 시 지연 없음.

### 5.2 실패 복구

- `sync.Once`는 1회만 실행. 실패 캐시(`realErr`)를 별도 `atomic.Pointer[error]`로 저장.
- MCP-001 reconnect 이벤트 발생 시 `Search.InvalidateCache(serverID)` 호출 → Registry가 해당 server의 모든 stub tool을 **교체(새 stub 인스턴스)** → 새 `sync.Once`로 재시도 기회 부여.

### 5.3 Manifest 캐시 공유

같은 tool이 여러 경로에서 resolve될 때(예: Registry.Resolve + Search.List + Inventory.ForModel) fetch를 1회로 제한해야 함. `Search.cache sync.Map`을 Registry에 주입하여 공유.

---

## 6. 테스트 전략 (TDD RED-first)

### 6.1 Unit 테스트 (internal/tools/*_test.go)

- `TestRegistry_RegisterBuiltins_HasSixCanonicalNames` — AC-TOOLS-001
- `TestRegistry_RegisterDuplicate_Panics` — REQ-TOOLS-013
- `TestRegistry_ReservedName_RejectsMCP` — REQ-TOOLS-003
- `TestRegistry_AdoptMCP_AppliesPrefix` — AC-TOOLS-002
- `TestRegistry_AdoptMCP_RejectsDoubleUnderscore` — REQ-TOOLS-017
- `TestRegistry_Resolve_ReturnsStubForMCP` — AC-TOOLS-003
- `TestRegistry_Draining_BlocksNewRun` — REQ-TOOLS-011
- `TestRegistry_ConnectionClosed_UnregistersTools` — REQ-TOOLS-009
- `TestExecutor_Run_SchemaFails` — AC-TOOLS-006
- `TestExecutor_Run_ToolNotFound` — AC-TOOLS-007
- `TestExecutor_Run_Preapproved_BypassesCanUseTool` — AC-TOOLS-004
- `TestExecutor_Run_CanUseToolDeny` — AC-TOOLS-005
- `TestExecutor_Run_InvocationLogged` — REQ-TOOLS-020
- `TestInventory_ForModel_Sorted` — REQ-TOOLS-005
- `TestInventory_ForModel_CoordinatorFilters` — REQ-TOOLS-012

### 6.2 Permission matcher 테스트

`permission/matcher_test.go`에 테이블 드리븐:

```go
cases := []struct{
    name     string
    tool     string
    input    string
    allow    []string
    deny     []string
    expected bool
}{
    {"bash-git-star allows git status", "Bash", `{"command":"git status"}`,
     []string{"Bash(git *)"}, nil, true},
    {"bash-git-star denies rm", "Bash", `{"command":"rm -rf /"}`,
     []string{"Bash(git *)"}, nil, false},
    {"deny-precedence", "Bash", `{"command":"git push --force origin main"}`,
     []string{"Bash(git *)"}, []string{"Bash(git push --force *)"}, false},
    {"mcp wildcard", "mcp__github__create_issue", `{}`,
     []string{"mcp__github__*"}, nil, true},
    // 20+ more
}
```

### 6.3 Builtin tool 테스트 (실 fs + 실 subprocess)

- `file/read_test.go`: t.TempDir fixture, offset/limit, non-UTF8 base64 fallback.
- `file/write_test.go`: atomic rename 검증 (쓰기 중 읽기 동시성 테스트), cwd 바깥 거부 (AC-TOOLS-009).
- `file/edit_test.go`: old_string 일치/불일치, replace_all true/false.
- `file/glob_test.go`: `**/*.go` 재귀 매칭, symlink 처리.
- `file/grep_test.go`: `-C`, `-i` 플래그, multiline.
- `terminal/bash_test.go`: stdout/stderr 분리, exit_code, timeout kill (AC-TOOLS-008, `ps` 자식 프로세스 확인), secret env 필터링 (REQ-TOOLS-016).

### 6.4 Integration 테스트 (build tag: integration)

- `tests/integration/tools/mcp_roundtrip_test.go`: 실 MCP-001 stdio 서버 스폰 → adopt → stub resolve → first call triggers fetch → result 반환.
- `tests/integration/tools/registry_drain_test.go`: Shutdown 시 in-flight tool 완료 허용, 신규 차단.

### 6.5 커버리지 목표

- `internal/tools/` 핵심: 95%+ (registry, executor, naming, budget, permission)
- `internal/tools/builtin/`: 90%+ (fs/subprocess 경로 모두)
- `internal/tools/search/`: 90%+

`go test -race -cover ./internal/tools/...` CI gate 통과 필수.

---

## 7. Go 이디엄

### 7.1 init() 기반 자동 등록

```go
// internal/tools/builtin/file/read.go

package file

import "github.com/gooseagent/goose/internal/tools/builtin"

func init() {
    builtin.MustRegister(&FileRead{})
}

type FileRead struct{}

func (*FileRead) Name() string { return "FileRead" }
func (*FileRead) Schema() json.RawMessage { return fileReadSchema }
func (*FileRead) Scope() tools.Scope { return tools.ScopeShared }
func (f *FileRead) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) { /* ... */ }
```

단, `internal/tools/builtin` 자체를 import해야 `init()`이 실행된다. `tools.NewRegistry(WithBuiltins())`는 `WithBuiltins()` 옵션 함수가 `import _ ".../builtin"` 형태로 blank import (side-effect import)로 등록을 트리거.

### 7.2 atomic.Pointer[T] 활용 (Go 1.19+)

```go
type mcpStubTool struct {
    realTool atomic.Pointer[mcpRealTool]
    realErr  atomic.Pointer[error]
    once     sync.Once
}
```

Lock-free replacement. Load-side는 nil check만.

### 7.3 Schema 사전 compile

```go
type compiledTool struct {
    Tool
    compiledSchema *jsonschema.Schema
}

func (c *compiledTool) Validate(input json.RawMessage) error {
    var v any
    if err := json.Unmarshal(input, &v); err != nil { return err }
    return c.compiledSchema.Validate(v)
}
```

Registry에 compile된 버전 저장. 호출마다 recompile 방지(R6 mitigation).

### 7.4 RWMutex 사용 원칙

- `Resolve`, `ListNames`, `ForModel`: RLock
- `Register`, `AdoptMCPServer`, `Unregister`, `Drain`: Lock
- 등록은 드물고 resolve는 빈번 → RWMutex 적절.

---

## 8. 오픈 이슈

1. **Inventory system prompt 포맷**: JSON 배열 vs 마크다운 테이블 vs YAML. 현재는 Claude Code 스타일 JSON (`[{name, description, input_schema}, ...]`) 채택. LLM adapter별 변환은 ADAPTER-001 책임.
2. **MCP prompt (resources/list) 통합**: claude-primitives §3.2에 `prompts/list` 언급. MCP prompt를 Skill로 변환하는 경로는 SKILLS-001 범위이므로 본 SPEC에서 제외.
3. **Conditional tool (paths:)**: claude-primitives §2.3 "Conditional Skill (paths:)" 개념을 tool에도 적용할지. 현재는 skill 전용으로 제한, tool은 항상 active. 향후 확장 여지.
4. **Tool Scope 세분화**: 현재 3단계(Shared/LeaderOnly/WorkerShareable). SUBAGENT-001이 `useExactTools` 도입 시 `Scope`와의 관계 재검토 필요.
5. **Binary safety in FileRead**: 비-UTF8 파일 처리. 현재는 base64 fallback 제안. 향후 explicit `binary: true` 요청 필요성 검토.
6. **Bash secret filtering false positive**: `API_KEY_DOC` 같은 정상 env도 필터링될 수 있음. 현재는 suffix match (`_TOKEN`, `_KEY`, `_SECRET`). precision은 문서화로 대응.

---

## 9. 결론

- **이식 자산**: 30% (Claude Code deferred loading 패턴, Hermes auto-registry 패턴). 나머지 70% 신규.
- **참조 자산**: claude-primitives.md §3.4, §2.4 / structure.md §172-211 / QUERY-001 REQ-QUERY-006/007/012/017.
- **기술 스택**: `jsonschema/v6` + `doublestar/v4` + 표준 `os/exec`/`regexp`.
- **구현 규모 예상**: ~2,000~2,500 LoC (테스트 포함 ~4,000). ROADMAP §9의 Phase 3 Go LoC 2,000 예산 내.
- **주요 리스크**: MCP fetch 실패 재시도 (R1), secret filter 정확도 (R3), symlink 우회 (R5). 모두 §8에서 mitigation 명시.

GREEN 완료 시점에서 **QUERY-001 AC-QUERY-002/003/007/009/010/017의 integration test가 실제 tool 실행으로 통과**하며, `goose ask "read foo.txt"` → `FileRead` 호출 → 응답의 end-to-end가 성립.

---

**End of research.md**
