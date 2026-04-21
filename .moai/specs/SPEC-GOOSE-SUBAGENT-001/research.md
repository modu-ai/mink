# SPEC-GOOSE-SUBAGENT-001 — Research & Porting Analysis

> **목적**: Claude Code의 `runAgent()` 3단계 생명주기 + 3-isolation + 3-memory scope + role profile override → GOOSE Go 포팅 계약. `.moai/project/research/claude-primitives.md` §4를 본 SPEC REQ와 1:1 매핑한다.
> **작성일**: 2026-04-21
> **범위**: `internal/subagent/` 단일 패키지.

---

## 1. 레포 현재 상태 스캔

```
/Users/goos/MoAI/AgentOS/
├── claude-code-source-map/      # tools/AgentTool/, runAgent 관련 TS
├── .claude/agents/              # MoAI-ADK 26개 agent 정의 (재사용 대상)
├── .claude/worktrees/           # 기존 MoAI worktree 경로
├── hermes-agent-main/           # subagent 개념 없음
└── .moai/specs/                 # QUERY-001/SKILLS-001/HOOK-001 합의 완료
```

- `internal/subagent/` → **전부 부재**. Phase 2 신규.
- Claude Code source map 내 AgentTool 구현 TS 다수. 직접 포트 대상 아님.
- MoAI-ADK `.claude/agents/`의 26개 기존 정의는 loader 테스트 fixture + 호환성 검증 대상.

**결론**: GREEN 단계는 `internal/subagent/` 10개 파일 **zero-to-one** 신규. 기존 MoAI-ADK 자산과의 호환성 초기 조사 필수.

---

## 2. claude-primitives.md §4 원문 인용 → REQ 매핑

### 2.1 Agent 생명주기 3단계 (§4.1)

원문:

```
[1] 초기화
  - agentDefinition 로드 (built-in / plugin / user)
  - MCP 서버 초기화 (initializeAgentMcpServers)
  - File state cache 생성
  - Transcript 서브디렉토리 설정

[2] 쿼리 루프
  - query() 호출 (QUERY-001)
  - 도구 호출 → tool.call() → tool_result 수집

[3] 종료
  - final message 수집
  - Compaction 트리거
  - MCP 서버 정리
  - shell 작업 종료
  - transcript 저장
```

본 SPEC은 단계 [1]과 [3]을 **직접 구현**하고, [2]는 **QUERY-001의 `QueryEngine`을 재사용**한다.

| 원문 | 본 SPEC REQ |
|---|---|
| agentDefinition 로드 | REQ-SA-004 (allowlist 공유), `LoadAgentsDir` |
| MCP 서버 초기화 | REQ-SA-020 (optional, MCP-001 consumer) |
| Transcript 서브디렉토리 | REQ-SA-002 |
| transcript 저장 | REQ-SA-008 |

### 2.2 Agent Memory 디렉토리 (§4.2)

원문:

```
scopes:
- user: ~/.claude/agent-memory/{agentType}/
- project: ./.claude/agent-memory/{agentType}/
- local: ./.claude/agent-memory-local/{agentType}/  (git-ignored)

각 디렉토리:
  {scope}/
  ├─ memdir.jsonl
  ├─ metadata.json
  ├─ transcript-{sessionId}/
  └─ (커스텀 파일)
```

→ **REQ-SA-002, REQ-SA-003, REQ-SA-012, REQ-SA-017**. GOOSE 포팅:

- 경로 `~/.claude/` → `~/.goose/`.
- `{sessionId}` → `{agentId}` (본 SPEC은 AgentID 기반 naming).
- `buildMemoryPrompt()`는 §4.2의 "모델이 메모리 쿼리/업데이트 가능"을 구체화: system prompt에 memdir.jsonl 항목을 `# Memory\n- {key}: {value}` 포맷으로 삽입.

### 2.3 Isolation 3 Mode (§4.3)

원문:

```
1. Fork (FORK_SUBAGENT):
   - 부모 컨텍스트 상속
   - 독립 token budget
   - agentId 별도
   - 백그라운드 실행

2. Worktree (EnterWorktreeTool):
   - 격리된 git worktree
   - WorktreeCreate hook
   - file state cache 독립
   - shell cwd 격리

3. Background (background: true):
   - subprocess 아님
   - 동일 프로세스, Task 중심
   - 백그라운드 polling
```

→ **REQ-SA-005, REQ-SA-006, REQ-SA-007**.

**Go 포팅 결정**:

- Fork = 새 `QueryEngine` + `context.WithValue` (AsyncLocalStorage 대체).
- Worktree = Fork + `exec.Command("git", "worktree", "add", ...)`.
- Background = Fork + goroutine 스핀업 즉시 반환, idle timer.

### 2.4 Role Profile Override (§4.4)

원문 TS:

```typescript
AgentDefinition {
  agentType: string
  tools: string[]
  useExactTools?: boolean
  model?: "inherit" | ModelAlias
  maxTurns?: number
  permissionMode?: "bubble" | "isolated"
  effort?: EffortValue
  getSystemPrompt: () => string
  mcpServers?: (string | {...})[]
  source: "plugin" | "built-in" | "user"
}
```

→ **본 SPEC §6.2의 Go 타입과 1:1 매핑**. Go 이디엄:

- `ModelAlias` → 문자열(ROUTER-001이 해석).
- `EffortValue` → `string`("L0"/"L1"/...) 또는 SKILLS-001의 `EffortLevel`.
- `getSystemPrompt: () => string` → `SystemPrompt string` (markdown body 직접 저장, build 시점이 아닌 load 시점).

### 2.5 설계 원칙 (§10) — Isolation + Bubbling

원문:

> **Isolation + Bubbling**: Fork 백그라운드, Worktree 디렉토리, permissionMode bubble

→ **REQ-SA-010**의 `bubble` mode가 HOOK-001의 `SwarmWorkerHandler`로 위임. 코드 경로:

```
sub-agent CanUseTool
  → TeammateCanUseTool.Check
  → def.PermissionMode == "bubble"?
    → parentCanUseTool.Check (with permCtx.Role = "swarmWorker")
      → Parent useCanUseTool (HOOK-001)
        → (may bubble further up if parent is also a sub-agent)
```

---

## 3. Go 포팅 매핑표 (claude-primitives.md §7)

| Claude Code (TS) | GOOSE (Go) 패키지 | 결정 |
|---|---|---|
| `tools/AgentTool/` (runAgent, spawn) | `internal/subagent/run.go` | 단일 진입점 `RunAgent` |
| `FORK_SUBAGENT` | `internal/subagent/fork.go` | `context.WithValue` 기반 |
| `EnterWorktreeTool` | `internal/subagent/worktree.go` | `exec.Command("git", "worktree", ...)` |
| `AsyncLocalStorage` | `context.WithValue(teammateKey, identity)` | 명시적 ctx 전달 |
| `memdir.jsonl` / `metadata.json` | `internal/subagent/memory.go` | append-only + JSON I/O |
| `AgentDefinition` | `internal/subagent/loader.go` | YAML frontmatter (SKILLS-001 allowlist 공유) |
| `permissionMode: bubble` | `internal/subagent/permission.go:TeammateCanUseTool` | HOOK-001 SwarmWorkerHandler 경유 |

---

## 4. Go 이디엄 선택 (상세 근거)

### 4.1 AsyncLocalStorage → `context.Context` 전파

**TS 원문**:

```typescript
asyncLocalStorage.run(teammateIdentity, async () => {
  await queryLoop()  // 내부 어디서든 getStore()로 identity 접근 가능
})
```

**Go 이디엄**:

```go
ctx := WithTeammateIdentity(parentCtx, id)
engine.SubmitMessage(ctx, prompt)  // 모든 downstream이 ctx 받아서 전파
```

**명시적 ctx 전달 장점**:

- 누락 시 컴파일 에러 (implicit storage 오염 없음).
- goroutine 경계 통과 시 `ctx` 그대로 넘기면 자동 유지.
- Go stdlib이 `context.Context`를 표준 패턴으로 확립.

**단점**:

- 모든 함수 시그니처에 `ctx` 첫 인자. 본 SPEC의 모든 API가 이를 준수.

### 4.2 Isolation 3 모드의 Go 구현 차이

```go
// Fork: 순수 in-process, ctx value로 identity 격리
func forkSpawn(parent, def, input) (*Subagent, <-chan SDKMessage, error) {
    ctx := WithTeammateIdentity(parent, newIdentity(def))
    engine := query.New(buildForkConfig(parent, def))
    ch := make(chan SDKMessage)
    go func() {
        defer close(ch)
        outCh, _ := engine.SubmitMessage(ctx, input.Prompt)
        for msg := range outCh { ch <- msg }
    }()
    return &Subagent{...}, ch, nil
}

// Worktree: fork + git worktree + CWD switch
func worktreeSpawn(parent, def, input) (*Subagent, <-chan SDKMessage, error) {
    wtPath, cleanup, err := createWorktree(newIdentity(def).AgentID)
    if err != nil { /* fallback to fork */ }
    // fork 기반 spawn, cfg.Cwd = wtPath
    ...
    go func() {
        defer cleanup()
        defer hook.DispatchWorktreeRemove(...)
        ...
    }()
}

// Background: fork + non-blocking, idle timer
func backgroundSpawn(...) {
    // 동일하나 caller에게 즉시 반환 (fork도 실은 즉시 반환하므로 차이는 idle timer)
    go monitorIdle(ch)
}
```

### 4.3 Memdir 동시성 (REQ-SA-012)

`memdir.jsonl`은 append-only log. 동시 쓰기 race:

```go
func (m *MemdirManager) Append(entry MemoryEntry) error {
    line, _ := json.Marshal(entry)
    line = append(line, '\n')

    f, err := os.OpenFile(m.path(), os.O_APPEND|os.O_WRONLY|os.O_SYNC, 0600)
    if err != nil { return err }
    defer f.Close()
    // POSIX: atomic append guaranteed for writes < PIPE_BUF (~4KB)
    _, err = f.Write(line)
    return err
}
```

POSIX `O_APPEND` + `write()` < PIPE_BUF는 atomic. 한 줄 entry가 4KB 초과 시 큰 value는 별도 파일로 외부화.

### 4.4 Worktree fallback 정책 (R1)

git이 없거나, detached HEAD, bare repo, submodule 꼬임 등 worktree 생성 실패 가능. Fallback 정책:

```go
type WorktreeFailBehavior int
const (
    WorktreeFailFallbackFork WorktreeFailBehavior = iota  // default
    WorktreeFailStrict
)
```

기본은 fork로 다운그레이드 + zap WARN. 사용자 설정 `subagent.worktree.onFail: strict`면 에러 반환.

### 4.5 goroutine 누수 방지 (R2)

모든 spawn goroutine은:

1. `ctx` 인자 수신.
2. `ctx.Done()` 감시 (`select case`에 포함).
3. parent ctx cancel 시 출력 채널 close + 조용히 return.

테스트에서 `go.uber.org/goleak` 도입 검토 — `defer goleak.VerifyNone(t)`로 goroutine 누수 검출.

---

## 5. 참조 가능한 외부 자산 분석

### 5.1 Claude Code TypeScript

- `tools/AgentTool/*.ts`: `runAgent` 구현이 다수 파일에 분산(lifecycle, memory, isolation 각각). 직접 포트 대상 아님.
- `AgentDefinition`의 `getSystemPrompt: () => string` 함수 형태는 Go에서 필요 없음 — 정적 문자열로 단순화.
- **직접 포트 대상 없음**.

### 5.2 MoAI-ADK `.claude/agents/`

26개 기존 agent 정의 (manager-spec, expert-backend, builder-agent 등). 각 파일은 다음 패턴:

```markdown
---
name: manager-spec
description: SPEC authoring
model: opus-4.7
tools: [Read, Write, Grep]
---

# Manager SPEC
...agent system prompt...
```

본 SPEC `AgentDefinition`과 호환 여부:

| 기존 필드 | 본 SPEC 필드 | 호환성 |
|---|---|---|
| `name` | `Name` | ✅ |
| `description` | `Description` | ✅ |
| `model` | `Model` | ✅ |
| `tools` (slice) | `Tools` | ✅ (대문자 시작 이름은 Go 측에서 lowercase 변환) |
| body markdown | `SystemPrompt` | ✅ |

R7 리스크: 기존 26개 파일 중 본 SPEC 스키마와 100% 호환되는 개수는 로드 테스트에서 확인. 초기 예측 ~80%. 미호환 case는 `source: "legacy"` + WARN.

### 5.3 MoAI-ADK `.claude/worktrees/`

기존 worktree 경로 convention:

- `{project-root}/.claude/worktrees/{agent-name}-{index}/`

본 SPEC은 이 규약 그대로 계승.

---

## 6. 외부 의존성 합계

| 모듈 | 버전 | 본 SPEC 사용 | 결정 근거 |
|------|-----|-----------|---------|
| `gopkg.in/yaml.v3` | v3.0.1+ | ✅ agent frontmatter | SKILLS-001 공유 |
| `go.uber.org/zap` | v1.27+ | ✅ 로깅 | CORE-001 공유 |
| `github.com/stretchr/testify` | v1.9+ | ✅ 테스트 | CORE-001 공유 |
| `go.uber.org/goleak` | v1.3+ | ✅ goroutine 누수 검증 | 본 SPEC 신규 도입 |
| Go stdlib `context`, `os/exec`, `encoding/json` | 1.22+ | ✅ | 표준 |
| git binary (external) | 2.30+ | ✅ worktree | `exec.Command` |

**의도적 미사용**:

- `go-git/go-git`: 순수 Go git 라이브러리. worktree 지원 제한 + 테스트 커버리지 부족. `exec.Command("git", ...)` 선택.
- `hashicorp/go-plugin`: gRPC 기반 plugin 과설계. 본 SPEC의 agent는 코드 내 spawn.
- Tokenizer 라이브러리: memory prompt 길이는 consumer(ROUTER-001)이 제한. 본 SPEC은 근사 길이만 zap에 로그.

---

## 7. 테스트 전략 (TDD RED → GREEN)

### 7.1 Unit 테스트 (35~45개)

**Loader 레이어**:
- `TestLoadAgentsDir_FrontmatterAllowlist_Reject` — REQ-SA-004
- `TestLoadAgentsDir_InvalidAgentName_Rejected` — REQ-SA-018
- `TestLoadAgentsDir_LoadsMoaiADKCompatibleAgents` — 기존 26개 fixture

**Isolation 레이어**:
- `TestForkSpawn_InjectsTeammateIdentity`
- `TestWorktreeSpawn_CreatesWorktree_Cleanups`
- `TestWorktreeSpawn_FallbackToFork_OnGitError` — R1
- `TestBackgroundSpawn_ReturnsImmediately` — AC-SA-003
- `TestBackgroundSpawn_IdleTimerDispatchesTeammateIdle`

**Identity 레이어**:
- `TestTeammateIdentity_UniqueAgentID` — REQ-SA-001
- `TestTeammateIdentity_ContextPropagation`

**Memory 레이어**:
- `TestMemdir_AppendAtomic_O_APPEND` — REQ-SA-012
- `TestMemdir_ScopeResolutionOrder_NearestWins` — REQ-SA-003
- `TestMemdir_Permissions_0700_0600` — REQ-SA-017 (unix only)
- `TestBuildMemoryPrompt_InjectsEntries`

**Permission 레이어**:
- `TestTeammateCanUseTool_IsolatedMode_Local`
- `TestTeammateCanUseTool_BubbleMode_DelegatesToParent` — AC-SA-007
- `TestTeammateCanUseTool_Background_WriteDenyDefault` — AC-SA-011
- `TestTeammateCanUseTool_ParentCtxCancelled_Deny` — R6

**Lifecycle 레이어**:
- `TestRunAgent_DispatchesStart_Stop_Hooks`
- `TestRunAgent_TranscriptPersisted` — AC-SA-005
- `TestRunAgent_NestedCoordinator_Warns` — AC-SA-008
- `TestRunAgent_SpawnDepthExceeded` — AC-SA-009
- `TestResumeAgent_LoadsTranscript` — AC-SA-006
- `TestResumeAgent_CorruptedTranscript_Error`

### 7.2 Integration 테스트 (AC 1:1, `integration` build tag)

| AC | Test | 특수 요구 |
|---|---|---|
| AC-SA-001 | `TestSubagent_ForkIsolation` | stub LLM |
| AC-SA-002 | `TestSubagent_WorktreeIsolation` | temp git repo fixture |
| AC-SA-003 | `TestSubagent_BackgroundNonBlocking` | timing assertion |
| AC-SA-004 | `TestSubagent_MemoryScopeResolution` | tempdir 3개 |
| AC-SA-005 | `TestSubagent_TranscriptPersisted` | fs 검증 |
| AC-SA-006 | `TestSubagent_ResumeAgent` | pre-populated transcript |
| AC-SA-007 | `TestSubagent_PermissionBubbling` | stub parent CanUseTool |
| AC-SA-008 | `TestSubagent_NestedCoordinatorWarn` | zap test observer |
| AC-SA-009 | `TestSubagent_SpawnDepthLimit` | 6단 spawn chain |
| AC-SA-010 | `TestSubagent_ToolsFilterApplied` | tool registry assertion |
| AC-SA-011 | `TestSubagent_BackgroundWriteDeny` | write tool mock |
| AC-SA-012 | `TestSubagent_MemdirPermissions` | `os.Stat` |

### 7.3 goleak & race

- `-race` 필수.
- 모든 lifecycle 테스트에 `defer goleak.VerifyNone(t)` (고아 goroutine 검출).
- Git fixture는 `t.TempDir()` + `git init` + `git commit -m "init"` 기반.

### 7.4 커버리지 목표

- `internal/subagent/`: 88%+.
- `permission.go`: 95%+ (보안 크리티컬).
- `worktree.go`: 85%+ (git fallback 경로 포함).

---

## 8. 오픈 이슈

1. **기존 26개 MoAI-ADK agent 호환성**: 초기 로드 테스트 후 미호환 리스트 수집 → 본 SPEC v0.2에서 legacy adapter 또는 migration 가이드.
2. **ResumeAgent의 semantic**: 현재는 transcript만 복원. 이전 Task 진행률/pending tool use는 복원 범위 외 → MEMORY-001(Phase 4)에서 확장.
3. **Worktree branch naming**: `goose/agent/{slug}` 제안. 기존 사용자 branch와 충돌 가능 → 접두사 설정 가능화.
4. **Background agent의 idle threshold**: 기본 5s. 학습 기반 적응은 Phase 4 INSIGHTS-001.
5. **Team mode와의 관계**: 본 SPEC은 **단일 sub-agent spawn**만. Multi-agent 동시 실행, SendMessage, TaskCreate/Update는 별도 SPEC.
6. **MCP connection 생명주기**: `def.MCPServers`의 connection은 sub-agent 종료 시 disconnect. 부모와 공유하지 않음(격리 강화).
7. **Permission bubbling 성능**: Deep chain(A→B→C→D)에서 매 tool call마다 bubble up은 O(depth). 실측 필요.

---

## 9. 구현 규모 예상

| 영역 | 파일 수 | 신규 LoC | 테스트 LoC |
|---|---|---|---|
| `run.go` (RunAgent) | 1 | 280 | 450 |
| `fork.go` | 1 | 150 | 200 |
| `worktree.go` | 1 | 220 | 380 |
| `background.go` | 1 | 150 | 250 |
| `resume.go` | 1 | 180 | 280 |
| `memory.go` (3-scope) | 1 | 280 | 400 |
| `loader.go` | 1 | 200 | 350 |
| `isolation.go` (abstract) | 1 | 80 | 120 |
| `permission.go` (bubble) | 1 | 180 | 320 |
| `identity.go` | 1 | 60 | 100 |
| **합계** | **10** | **~1,780** | **~2,850** |

테스트 비율: 62%.

---

## 10. 결론

- **상속 자산**: TypeScript source map 설계 참조만. MoAI-ADK 기존 agent 정의는 loader 테스트 fixture로 재활용.
- **핵심 결정**:
  - AsyncLocalStorage → `context.WithValue(teammateKey, identity)` (명시 전달).
  - Isolation 3 모드는 동일 spawn 경로의 configuration variant로 통합.
  - Worktree 실패 시 fork fallback 기본(사용자 설정으로 strict 전환 가능).
  - Memory 3-scope은 파일 기반 append-only jsonl. 구조화 쿼리는 Phase 4.
  - Permission bubbling은 HOOK-001의 `SwarmWorkerHandler` 경로 재사용.
- **Go 버전**: 1.22+.
- **다음 단계 선행 요건**: HOOK-001의 `SwarmWorkerHandler` 정확한 시그니처 확정. SKILLS-001의 `SkillFrontmatter` allowlist와 본 SPEC의 `AgentDefinition` 필드 교차 검증(공유 파서 코드 추출 여부 결정).
