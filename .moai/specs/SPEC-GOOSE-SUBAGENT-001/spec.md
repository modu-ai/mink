---
id: SPEC-GOOSE-SUBAGENT-001
version: 0.3.0
status: completed
completed: 2026-04-27
created_at: 2026-04-21
updated_at: 2026-04-27
author: manager-spec
priority: P0
issue_number: null
phase: 2
size: 대(L)
lifecycle: spec-anchored
labels: [subagent, runtime, isolation, memory, phase-2]
---

# SPEC-GOOSE-SUBAGENT-001 — Sub-agent Runtime (Fork / Worktree / Background Isolation + 3 Memory Scope)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (claude-primitives §4 + QUERY-001/SKILLS-001/HOOK-001 합의 기반) | manager-spec |
| 0.2.0 | 2026-04-25 | plan-auditor iter1 FAIL 결함 수정: D13(ResumeAgent 시그니처 통일) / D15(memory update REQ-SA-021 신설) / D17(PlanModeRequired REQ-SA-022 신설) / D18(goroutine lifecycle Unwanted REQ-SA-023 신설) / D7(REQ-SA-012 peer sub-agent 동시 쓰기 semantics 명확화) / D1(REQ-SA-004/012/015/018/019/020 AC 커버리지 AC-SA-013~018 신설) | manager-spec |
| 0.3.0 | 2026-04-25 | plan-auditor iter2 FAIL 결함 수정: N2(§6.2 AgentDefinition.MemoryScopes 필드 신설 — REQ-SA-021/AC-SA-019 구현 가능성 확보) / N3(§6.2 PlanModeApprove API 시그니처 신설) / N1(§4 REQ ID 카테고리 그룹화 정책 명시 — informational) / D4(REQ-SA-001 atomic spawnIndex 명시) / D5(REQ-SA-007 DefaultBackgroundIdleThreshold 명명 상수 참조) / D8(REQ-SA-016 settings.json 경로 명시) / D9(REQ-SA-015 HOOK-001 SessionEnd 의존성 conditional framing) / D10(REQ-SA-008 (d)→(c) 순서 명시) / D11(REQ-SA-018 agentName 문자 집합에서 `-` 제외 + AgentID delimiter 명시) / D12(REQ-SA-005 failure path 신설) | manager-spec |

---

## 1. 개요 (Overview)

AI.MINK의 **Sub-agent 런타임**을 정의한다. Claude Code의 `runAgent()` 생명주기(3단계) + 3종 isolation(fork / worktree / background) + 3-scope memory(user / project / local) + role profile override를 Go로 포팅하여, 하나의 부모 `QueryEngine`(QUERY-001)에서 여러 sub-agent를 순차·병렬로 spawn하고, 각각의 tool budget·권한 정책·메모리 디렉토리를 독립적으로 유지한다.

본 SPEC이 통과한 시점에서 `internal/subagent` 패키지는:

- `AgentDefinition`을 `.claude/agents/{name}.md` 또는 plugin 소스에서 로드하고,
- `RunAgent(ctx, def, input) (*Subagent, <-chan message.SDKMessage, error)`로 새 `QueryEngine` 인스턴스를 spawn하며,
- Isolation 모드:
  - **Fork**: 부모 컨텍스트 상속 + 독립 token budget + `TeammateIdentity` 부여,
  - **Worktree**: git worktree 생성(`EnterWorktreeTool`) + CWD 격리 + `WorktreeCreate` hook 발동,
  - **Background**: 동일 프로세스에서 non-blocking goroutine + polling,
- 3-scope memory: `~/.goose/agent-memory/{agentType}/`(user), `./.goose/agent-memory/{agentType}/`(project), `./.goose/agent-memory-local/{agentType}/`(local, gitignored),
- `buildMemoryPrompt()`로 memdir.jsonl 항목을 system prompt에 삽입하여 모델이 memory를 쿼리할 수 있고, `memory.append` 내장 tool을 통해 새 entry를 업데이트할 수 있다(REQ-SA-021 참조),
- `ResumeAgent(agentId string)`로 중단된 sub-agent를 재개(`resumable agents`).

본 SPEC은 **부모-자식 permission bubbling**(HOOK-001의 `SwarmWorkerHandler`)과 **AsyncLocalStorage 대체로서 `context.Context`** 전파를 Go 이디엄으로 구현한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Phase 2의 4 primitive 중 Subagent는 **multi-agent 협업의 최소 단위**. MoAI Agent Teams, Agency 파이프라인, 사용자 주도 fork는 모두 본 SPEC 위에 구축.
- `.moai/project/research/claude-primitives.md` §4가 Claude Code의 Agent lifecycle(3단계) + isolation + memory scope + role profile override를 제시한다. 본 SPEC은 그 구조를 Go 이디엄으로 확정.
- QUERY-001의 `TeammateIdentity` optional 필드, SKILLS-001의 `TriggerFork`, HOOK-001의 `CoordinatorHandler`/`SwarmWorkerHandler` 모두 본 SPEC에서 활성화된다.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code TypeScript**: `tools/AgentTool/`, `runAgent()`, `AgentDefinition`, `FORK_SUBAGENT`, `EnterWorktreeTool`. 언어 상이 직접 포트 없음 — 상태 머신 + isolation 3 모드의 토폴로지만 번역.
- **MoAI-ADK `.claude/agents/`**: 26개 에이전트 정의 파일 존재(manager-spec, expert-backend 등). 본 SPEC의 `AgentDefinition` 파서는 이들을 로드 가능.
- **Hermes Agent Python**: Subagent 개념 없음. 본 SPEC은 Hermes 자산 재사용 없음.

### 2.3 범위 경계

- **IN**: `AgentDefinition`/`Subagent`/`TeammateIdentity` 타입, `.claude/agents/*.md` 로더(YAML frontmatter + markdown body), `RunAgent(ctx, def, input)` spawn API, 3종 isolation 구현(Fork / Worktree / Background), 3-scope memory 디렉토리 layout + `buildMemoryPrompt`, `ResumeAgent(agentId)` 재개, Role profile override(`tools`/`model`/`maxTurns`/`effort`/`permissionMode`), `SubagentStart`/`SubagentStop`/`TeammateIdle` hook 통합, Coordinator mode tool visibility switch 제어, `CanUseTool` teammate 정책 주입.
- **OUT**: QueryEngine 자체(QUERY-001), Context compaction(CONTEXT-001), Tool 실행(TOOLS-001), LLM 호출(ADAPTER-001), MCP 서버 초기화 본체(MCP-001의 `ConnectToServer`만 사용), Agent definition marketplace/sync(PLUGIN-001), Team 전체 오케스트레이션(workflow.yaml 기반 team orchestrator — 별도 SPEC 또는 CLI-001), gRPC remote agent spawn(TRANSPORT-001 후속).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/subagent/` 패키지.
2. 타입: `AgentDefinition`, `Subagent`, `IsolationMode`, `MemoryScope`, `ResumableAgent`, `TeammateIdentity`.
3. Loader: `LoadAgentsDir(root)` — `.claude/agents/*.md`의 YAML frontmatter + body 파싱.
4. Spawn API:
   - `RunAgent(ctx, def, input SubagentInput) (*Subagent, <-chan message.SDKMessage, error)`.
   - 내부적으로 부모 `QueryEngine`의 일부 설정을 override하여 새 `QueryEngine` 인스턴스 생성.
5. Isolation 3구현:
   - `Fork`: `context.WithValue(parent, teammateKey, identity)` + 독립 `TaskBudget` + `messages[]` 부모 상속 복사.
   - `Worktree`: `git worktree add ./.claude/worktrees/{name}` + agent CWD 변경 + `WorktreeCreate` hook (HOOK-001) 발동.
   - `Background`: 동일 프로세스, 별도 goroutine, non-blocking.
6. Memory 3-scope:
   - `user`: `$HOME/.goose/agent-memory/{agentType}/`,
   - `project`: `{projectRoot}/.goose/agent-memory/{agentType}/`,
   - `local`: `{projectRoot}/.goose/agent-memory-local/{agentType}/` (gitignored).
7. `buildMemoryPrompt(agentType, scopes []MemoryScope) string` — memdir.jsonl 읽고 system prompt 템플릿에 삽입.
8. `Memdir` 파일 I/O — `memdir.jsonl` 읽기/쓰기 + `metadata.json` 관리.
9. `ResumeAgent(ctx, agentID, opts...) (*Subagent, <-chan message.SDKMessage, error)` — 이전 세션의 transcript 복원 + 진행 재개. 시그니처는 §6.2를 정본(canonical)으로 하며 AC-SA-006도 동일하게 `(ctx, "researcher@sess-old-2")` 형태로 호출한다.
10. Role profile override 병합: `AgentDefinition.Tools = ["*"]`(부모 상속) 또는 명시 목록; `AgentDefinition.Model = "inherit"` 또는 model alias; `AgentDefinition.MaxTurns`/`Effort`/`PermissionMode` override.
11. HOOK-001 통합: `SubagentStart`/`SubagentStop`/`TeammateIdle` dispatch.
12. `CoordinatorMode` 플래그 반영 (QUERY-001의 `CoordinatorMode` 설정자).
13. `CanUseTool` teammate 정책 구현체(`TeammateCanUseTool`) — permission bubbling via `SwarmWorkerHandler`.

### 3.2 OUT OF SCOPE

- **QueryEngine 내부 로직**: QUERY-001. 본 SPEC은 `QueryEngine`을 consumer로 사용만.
- **Team orchestration (workflow.yaml team 모드)**: 별도 SPEC(`TEAM-001` 또는 CLI-001). 본 SPEC은 **단일 agent spawn 단위**만.
- **Agent 간 SendMessage / mailbox 메시징**: 별도 SPEC. 본 SPEC은 stdin/stdout 스트리밍만.
- **Agent checkpointing 완전 자동화**: `ResumeAgent`는 transcript 기반 단순 복원. 체크포인트 정책은 MEMORY-001/REFLECT-001.
- **gRPC remote agent**: 본 SPEC은 **동일 프로세스 내 sub-agent**만.
- **Plugin-loaded agent definition**: PLUGIN-001이 `LoadAgentsDir`의 확장 경로 주입.
- **Coordinator/Worker 팀 분업 semantics**: 본 SPEC은 `CoordinatorMode` 플래그 전파만. 실제 분업 로직은 consumer.
- **Background agent의 Ctrl+X Ctrl+K kill UX**: CLI-001.

---

## 4. EARS 요구사항 (Requirements)

> **REQ ID 정렬 정책 (informational, N1)**: REQ-SA-NNN 식별자는 EARS 카테고리(§4.1 Ubiquitous → §4.2 Event-Driven → §4.3 State-Driven → §4.4 Unwanted → §4.5 Optional) 그룹으로 배치한다. iter2 추가분(REQ-SA-022, 023)은 §4.4 Unwanted에 카테고리-결합 우선으로 배치되어 문서 라인 순서 상으로는 §4.5 Optional의 REQ-SA-019/020/021보다 먼저 등장한다. 식별자는 단조 증가(001..023)이며 중복·결번 없음(MP-1 통과). 카테고리 그룹화 일관성을 ID 단조성보다 우선한 의도적 결정이다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-SA-001 [Ubiquitous]** — Every spawned `Subagent` **shall** have a unique `AgentID` composed as `{agentName}@{sessionId}-{spawnIndex}`; collisions **shall not** occur within a single parent session's lifetime. The `spawnIndex` **shall** be allocated atomically per parent session via `atomic.AddInt64(&parentSpawnCounter, 1)` so concurrent `RunAgent` calls from the same parent receive monotonically increasing, non-overlapping indices. The delimiter `@` separates `agentName` from `{sessionId}-{spawnIndex}` and is reserved; agent names are constrained by REQ-SA-018 to exclude `-` and `@`, eliminating round-trip parsing ambiguity.

**REQ-SA-002 [Ubiquitous]** — The `Subagent.Transcript` **shall** be persisted to `{memoryDir}/transcript-{agentId}/` regardless of isolation mode; persistence is independent of completion status (in-progress, completed, failed).

**REQ-SA-003 [Ubiquitous]** — The `MemoryScope` resolution order for `buildMemoryPrompt` **shall** be `local → project → user` (nearest first); duplicate keys in memdir.jsonl entries **shall** be resolved by taking the nearest scope's value.

**REQ-SA-004 [Ubiquitous]** — Every `AgentDefinition` loaded via `LoadAgentsDir` **shall** pass the same YAML frontmatter allowlist validation as `SkillFrontmatter` (see SKILLS-001 REQ-SK-001); unknown properties **shall** cause `ErrUnsafeAgentProperty`.

### 4.2 Event-Driven (이벤트 기반)

**REQ-SA-005 [Event-Driven]** — **When** `RunAgent(ctx, def, input)` is invoked with `def.Isolation == "fork"`, the spawner **shall** (a) create a new `QueryEngine` instance with override config(inherited tools, independent `TaskBudget`, new `AgentID`), (b) inject `TeammateIdentity{AgentId, AgentName, TeamName, ParentSessionId}` into the engine's `ctx` via `context.WithValue`, (c) invoke `DispatchSubagentStart(ctx, input)` (HOOK-001), (d) spawn a background goroutine to call `engine.SubmitMessage(input.Prompt)`, (e) return `Subagent` + output channel + nil error.

**REQ-SA-005-F [Event-Driven, Failure Path]** — **When** any step of REQ-SA-005 fails, the spawner **shall** unwind in reverse order and surface a typed error: (i) if QueryEngine creation fails, return `(nil, nil, ErrEngineInitFailed)` and **shall not** dispatch `SubagentStart`; (ii) if `DispatchSubagentStart` returns an error, the spawner **shall** abort, release the partially constructed engine, return `(nil, nil, ErrHookDispatchFailed)`; (iii) if goroutine spawn fails (e.g., `ctx` already cancelled), the spawner **shall** dispatch `DispatchSubagentStop` with `Terminal{Success: false, Reason: "spawn_aborted"}` to maintain hook-pair invariant, close the output channel, and return `(nil, nil, ErrSpawnAborted)`. In all failure modes the partially allocated `AgentID` and `spawnIndex` **shall not** be reused.

**REQ-SA-006 [Event-Driven]** — **When** `def.Isolation == "worktree"`, the spawner **shall** additionally (before step b of REQ-SA-005) execute `git worktree add ./.claude/worktrees/{agent-slug}` with a branch derived from `HEAD`, set the new engine's `cfg.Cwd` to that worktree path, invoke `DispatchWorktreeCreate` (HOOK-001), and on subagent completion invoke `DispatchWorktreeRemove`.

**REQ-SA-007 [Event-Driven]** — **When** `def.Isolation == "background"`, the spawner **shall** spawn the goroutine with non-blocking semantics — the returned channel **shall** receive messages asynchronously, and `DispatchTeammateIdle` **shall** be invoked after an inactivity period equal to `DefaultBackgroundIdleThreshold` (default 5 s, configurable via `subagent.background.idle_threshold` in `settings.json`) without new messages. AC-SA-003 verifies the same configurable constant.

**REQ-SA-008 [Event-Driven]** — **When** a sub-agent's `QueryEngine` returns a terminal `Terminal{...}` (see QUERY-001 REQ-QUERY-011), the spawner **shall** execute the following ordered steps: (a) write the final transcript to `transcript-{agentId}/`, (b) invoke `DispatchSubagentStop(ctx, result)` (HOOK-001), (d) **before** step (c), mark `Subagent.State == Completed|Failed` based on `Terminal.Success` (state mutation **shall** happen-before channel close), (c) close the output channel. The (d)→(c) ordering **shall** be enforced so that any consumer observing the channel close via `range` or `<-` receives a committed `Subagent.State` (no transient `Running` observation post-close). The Go memory model guarantee is provided by `sync/atomic` store on `State` followed by `close(ch)` in the same goroutine.

**REQ-SA-009 [Event-Driven]** — **When** `ResumeAgent(agentId)` is invoked, the function **shall** (a) load `transcript-{agentId}/` and `metadata.json` from the matching memory scope, (b) reconstruct `AgentDefinition` from metadata, (c) reconstruct parent `ctx` (new one, but with the agent's original `TeammateIdentity` restored), (d) call `RunAgent` with `input.Prompt = "[[RESUME]]"` so the model receives a resume cue.

**REQ-SA-010 [Event-Driven]** — **When** a sub-agent requests tool permission via `CanUseTool`, the `TeammateCanUseTool` policy **shall** (a) if `def.PermissionMode == "isolated"`, evaluate locally using the teammate's own rules, (b) if `def.PermissionMode == "bubble"`, delegate to parent's `CanUseTool` via `HOOK-001.SwarmWorkerHandler`, forwarding the parent's decision back as the sub-agent's decision.

### 4.3 State-Driven (상태 기반)

**REQ-SA-011 [State-Driven]** — **While** a parent `QueryEngine` is in `CoordinatorMode == true`, spawned sub-agents **shall** inherit `CoordinatorMode == false` by default; explicit override via `def.CoordinatorMode = true` is allowed but logs a WARN (nested coordinator is rarely correct).

**REQ-SA-012 [State-Driven]** — **While** an agent's `memdir.jsonl` is being written, concurrent reads **shall** see either the prior state or the new state — never a partial line. Writes **shall** obey the following semantics:
(a) **Single-writer within a process**: all writes **shall** use `os.O_APPEND | os.O_SYNC` with full-line atomic append (guaranteed torn-free for writes ≤ `PIPE_BUF` on POSIX);
(b) **Peer sub-agents sharing the same scope directory (e.g., project scope)**: writers **shall** acquire an advisory file lock via `golang.org/x/sys/unix.Flock(fd, LOCK_EX)` (or equivalent `flock(2)` on darwin/linux) for the full write+fsync critical section; on Windows, `LockFileEx` **shall** be used. Locks **shall** be released before returning from `MemdirManager.Append`;
(c) **NFS / network filesystems**: if the underlying filesystem does not support advisory locking, `MemdirManager.Append` **shall** return `ErrMemdirLockUnsupported` rather than silently risk corruption.

**REQ-SA-013 [State-Driven]** — **While** `def.Tools == ["*"]`, the sub-agent **shall** inherit the parent's tool registry as-is; **while** `def.Tools` lists specific tool names, only those tools (plus the baseline `agent-critical` set: `read`, `task-update`) **shall** be exposed to the sub-agent's `QueryEngine`.

### 4.4 Unwanted Behavior (방지)

**REQ-SA-014 [Unwanted]** — The spawner **shall not** allow cyclic agent spawning (A spawns B, B spawns A); the spawner maintains a `spawnDepth` counter in `ctx`, and if depth exceeds `MaxSpawnDepth` (default 5), `RunAgent` **shall** return `ErrSpawnDepthExceeded`.

**REQ-SA-015 [Unwanted]** — Worktree isolation **shall not** leave orphan worktrees on crash. **Given** HOOK-001 emits a `SessionEnd` event (see SPEC-GOOSE-HOOK-001 REQ-HK-SESSIONEND, treated here as a precondition dependency), the registered `SessionEnd` hook handler **shall** invoke `git worktree prune` followed by `os.RemoveAll` on the orphaned worktree path. If HOOK-001 does not emit `SessionEnd` for any reason (process crash, hook subsystem disabled), the SUBAGENT runtime **shall** additionally run an idempotent startup-time scan during `RunAgent` initialization that prunes any `./.claude/worktrees/*` orphan whose corresponding agent is not active in the current parent session — providing defense-in-depth.

**REQ-SA-016 [Unwanted]** — Background-isolated sub-agents **shall not** consume Write/Edit permissions that the parent has not pre-approved; if `def.Isolation == "background"` and `def.PermissionMode == "bubble"` and a Write tool is requested, the permission flow **shall** default to `Deny` with reason `"background_agent_write_denied"` unless an explicit allow rule is present in `settings.json` at the path `subagent.permissions.allow` (an array of tool-name patterns matched against the requested `toolName`). AC-SA-011 verifies the same `settings.json` source.

**REQ-SA-017 [Unwanted]** — The memory directory **shall not** be created with permissions broader than `0700` for directories or `0600` for files; on existing directories with wider permissions, a zap WARN is logged and permissions are **not** changed (sysadmin's responsibility).

**REQ-SA-018 [Unwanted]** — `LoadAgentsDir` **shall not** load agents whose name starts with `_` (reserved for internal namespaces) or contains characters outside `[a-zA-Z0-9_]`; violations **shall** produce `ErrInvalidAgentName`. Note: `-` (hyphen) and `@` (at-sign) **shall** be excluded from the agent-name character set because they are reserved as `AgentID` delimiters in REQ-SA-001's format `{agentName}@{sessionId}-{spawnIndex}`. This exclusion guarantees unambiguous round-trip parsing of `AgentID` strings (split first on `@`, then split the right side on the last `-` for `spawnIndex`). Existing agent definitions with hyphens (e.g., `manager-spec`, `expert-backend`) **shall** be migrated by the legacy compatibility scanner (see R7) to use underscores or be loaded with `source: "legacy"` tag and a deprecation WARN.

**REQ-SA-022 [Unwanted]** — **If** a sub-agent is spawned with `def.PermissionMode == "plan"` (indicated internally by `TeammateIdentity.PlanModeRequired == true`), **then** the spawner **shall not** execute any Write/Edit/Bash tool invocations until an explicit approval signal is received. The approval protocol is:
(a) `PlanModeRequired` **shall** be set to `true` by the loader when `def.PermissionMode == "plan"`;
(b) While `PlanModeRequired == true`, `TeammateCanUseTool` **shall** return `Decision{Behavior: AskParent}` for any write-class tool and **shall** allow read-class tools;
(c) Approval arrives via a `PlanModeApprove(agentID)` API call from the parent (orchestrator) which sets `PlanModeRequired = false`;
(d) If approval is not received within `PlanApprovalTimeout` (default 300 s, configurable), the sub-agent **shall** terminate with `Subagent.State == Failed` and reason `plan_mode_timeout`.

**REQ-SA-023 [Unwanted]** — The spawner **shall not** leave background goroutines running after the parent `ctx.Done()` is signaled. Every goroutine spawned by `RunAgent`, `ResumeAgent`, or isolation helpers (fork / worktree / background) **shall** (a) select on `ctx.Done()` in its main loop, (b) release file handles / worktree locks / memdir locks before returning, (c) terminate within `GoroutineShutdownGrace` (default 100 ms) after `ctx.Done()`. Compliance **shall** be verified in CI via `go.uber.org/goleak` across all isolation modes.

### 4.5 Optional (선택적)

**REQ-SA-019 [Optional]** — **Where** `def.Model == "inherit"`, the sub-agent's LLM call **shall** use the parent's resolved model; **where** `def.Model` is an explicit alias, it **shall** be passed to ROUTER-001 for resolution.

**REQ-SA-020 [Optional]** — **Where** `def.MCPServers` is non-empty, `RunAgent` **shall** initialize MCP connections via MCP-001's `ConnectToServer` before spawning the `QueryEngine`; the resulting tools are merged into the sub-agent's tool registry with namespacing `mcp__{server}__{tool}`.

**REQ-SA-021 [Optional]** — **Where** a sub-agent's system prompt contains memory entries injected by `buildMemoryPrompt` (i.e., at least one of `def.MemoryScopes` is non-empty), the spawner **shall** expose a built-in tool named `memory.append` to the sub-agent's `QueryEngine.cfg.Tools`. The tool **shall** (a) accept `{scope: "user"|"project"|"local", category, key, value}` JSON input, (b) validate `scope` is a member of the sub-agent's enabled scopes, (c) invoke `MemdirManager.Append(MemoryEntry{...})` under the concurrency semantics of REQ-SA-012, (d) return `{id, written_at}` on success or typed error on failure. The tool name `memory.append` **shall** be reserved and **shall not** collide with user-defined or MCP tools.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-SA-001 — Fork isolation spawn** (Verifies: REQ-SA-001, REQ-SA-005)
- **Given** parent `QueryEngine` 세션, `AgentDefinition{Name:"researcher", Isolation:"fork", Tools:["*"], Model:"inherit"}`, stub LLM
- **When** `RunAgent(parentCtx, def, input)` 호출 후 output 채널 drain
- **Then** 새 `QueryEngine` 생성, `TeammateIdentity{AgentId:"researcher@parentSession-1"}` 주입, 부모 tools 상속, `DispatchSubagentStart`/`SubagentStop` 호출 각 1회, 최종 `Subagent.State == Completed`

**AC-SA-002 — Worktree isolation** (Verifies: REQ-SA-006)
- **Given** git 초기화된 저장소, `def.Isolation == "worktree"`, 더미 agent
- **When** `RunAgent`
- **Then** `./.claude/worktrees/researcher-1/` 디렉토리 생성 + 새 branch 체크아웃, sub-agent의 `cfg.Cwd`가 해당 path, `DispatchWorktreeCreate`/`WorktreeRemove` 각 1회 호출, 완료 후 `git worktree list`에 잔존 없음

**AC-SA-003 — Background isolation non-blocking** (Verifies: REQ-SA-007)
- **Given** `def.Isolation == "background"`, stub LLM이 응답 전 2초 지연
- **When** `RunAgent` 호출 (본 호출은 즉시 반환), 호출자가 500ms 이내 다음 코드 실행
- **Then** 호출자 로직이 LLM 응답 이전에 실행 완료, sub-agent는 background에서 진행, 2초 후 첫 메시지가 channel로 도착, 그 이후 `DefaultBackgroundIdleThreshold`(5 s) 이상 메시지 없으면 `DispatchTeammateIdle` 1회 호출

**AC-SA-004 — Memory 3-scope resolution order** (Verifies: REQ-SA-003)
- **Given** 3 scope에 동일 키 `user.preference`가 각기 다른 값으로 존재 (user="U", project="P", local="L")
- **When** `buildMemoryPrompt("researcher", [local, project, user])`
- **Then** 결과 system prompt에 `user.preference = "L"`만 포함 (REQ-SA-003 "nearest wins": local→project→user 우선순위), 각 scope에만 존재하는 고유 키는 모두 포함 (합집합 의미의 union)

**AC-SA-005 — Transcript persistence** (Verifies: REQ-SA-002, REQ-SA-008)
- **Given** 정상 완료한 sub-agent
- **When** 완료 후 `~/.goose/agent-memory/researcher/transcript-{agentId}/` 조회
- **Then** 해당 디렉토리 존재, `messages.jsonl` 파일에 모든 `SDKMessage`가 순서대로 기록됨

**AC-SA-006 — ResumeAgent** (Verifies: REQ-SA-009)
- **Given** 이전 세션에서 중단된 sub-agent `researcher@sess-old-2`, 해당 transcript 디스크 존재
- **When** `ResumeAgent(ctx, "researcher@sess-old-2")` (시그니처는 §6.2 정본: `ResumeAgent(ctx, agentID, opts...)`)
- **Then** 새 `Subagent` 인스턴스 생성, 이전 transcript 로드, `input.Prompt == "[[RESUME]]"` 전달, 이전 `TeammateIdentity` 복원

**AC-SA-007 — Permission bubbling** (Verifies: REQ-SA-010)
- **Given** `def.PermissionMode == "bubble"`, 부모 engine의 `CanUseTool`이 `Deny{reason:"parent-policy"}` 반환하도록 설정
- **When** sub-agent가 tool 호출 시도
- **Then** `TeammateCanUseTool`이 부모 `CanUseTool`로 위임, 결과 `Deny{reason:"parent-policy"}` 반환, sub-agent는 synthetic ToolResult로 진행

**AC-SA-008 — Coordinator mode nested warn** (Verifies: REQ-SA-011)
- **Given** 부모가 `CoordinatorMode == true`, `def.CoordinatorMode == true`로 override
- **When** `RunAgent`
- **Then** zap WARN 로그 출력("nested coordinator mode is rarely correct"), sub-agent는 정상 spawn, `CoordinatorMode == true` 설정됨

**AC-SA-009 — Cyclic spawn 방지** (Verifies: REQ-SA-014)
- **Given** A → B → C → D → E → F 가 순차 spawn, `MaxSpawnDepth == 5`
- **When** F가 G를 spawn 시도
- **Then** `RunAgent`가 `ErrSpawnDepthExceeded` 반환, G는 생성되지 않음, 다른 spawn은 영향 없음

**AC-SA-010 — `def.Tools` 명시 시 필터링** (Verifies: REQ-SA-013)
- **Given** 부모 tools 10개, `def.Tools = ["read", "search"]`
- **When** 해당 sub-agent의 `QueryEngine` 초기화
- **Then** sub-agent `QueryEngine.cfg.Tools`에 `read`, `search` + baseline(`task-update`) 만 포함. 다른 8개 tool은 접근 불가

**AC-SA-011 — Background agent write denied by default** (Verifies: REQ-SA-016)
- **Given** `def.Isolation == "background"`, `def.PermissionMode == "bubble"`, write tool 호출 시도, 사전 allow 규칙 없음 (allow rule 소스는 `settings.json` `subagent.permissions.allow`)
- **When** `CanUseTool`
- **Then** `Deny{reason: "background_agent_write_denied"}`, write 실제 실행 0회

**AC-SA-012 — Memory directory permission enforcement** (Verifies: REQ-SA-017)
- **Given** `~/.goose/agent-memory/researcher/` 디렉토리 생성 시
- **When** `RunAgent`가 해당 scope에 memdir 준비
- **Then** 생성된 디렉토리 권한은 `0700`, 파일 권한은 `0600` (테스트: `os.Stat().Mode().Perm()`)

**AC-SA-013 — Agent frontmatter allowlist validation** (Verifies: REQ-SA-004)
- **Given** `.claude/agents/evil.md` 파일의 frontmatter가 SKILLS-001 REQ-SK-001의 allowlist에 없는 속성(`exec_on_load: true`)을 포함
- **When** `LoadAgentsDir(root)` 호출
- **Then** 반환값에 `evil.md` 정의는 포함되지 않고, 누적 에러 리스트에 `ErrUnsafeAgentProperty{agent:"evil", key:"exec_on_load"}` 1건 포함. 허용된 frontmatter만 가진 다른 agent는 정상 로드

**AC-SA-014 — Peer sub-agents concurrent memdir write** (Verifies: REQ-SA-012)
- **Given** 동일 project scope 디렉토리(`./.goose/agent-memory/researcher/`)를 공유하는 peer sub-agent A, B가 동시에 `MemdirManager.Append`를 각 100회 호출
- **When** 두 sub-agent가 병렬로 append 완료 후 `memdir.jsonl` 전체 재파싱
- **Then** 200개 entry 모두 완전한 JSON line으로 파싱 성공(torn line 0), 각 entry는 REQ-SA-012(b)의 advisory lock(flock/LockFileEx)을 거쳐 기록됨이 race detector(`go test -race`) + 로그 검증으로 확인됨. lock 미지원 FS에서는 `ErrMemdirLockUnsupported` 반환 확인

**AC-SA-015 — Orphan worktree cleanup on SessionEnd** (Verifies: REQ-SA-015)
- **Given** worktree isolation으로 spawn된 sub-agent가 crash(SIGKILL)로 비정상 종료되어 `./.claude/worktrees/researcher-crash/` 디렉토리와 `goose/agent/researcher-crash` branch가 잔존
- **When** HOOK-001의 `SessionEnd` dispatcher 발동
- **Then** `git worktree list`에서 해당 경로 제거, 디스크 디렉토리 `os.RemoveAll` 확인(`os.Stat` → `fs.ErrNotExist`), 관련 branch 제거. cleanup은 idempotent(재호출 시 에러 없음)

**AC-SA-016 — Invalid agent name rejection** (Verifies: REQ-SA-018)
- **Given** `.claude/agents/` 내 `_hidden.md`, `foo bar.md`, `foo/bar.md`, `légal.md`(non-ASCII) 파일 존재, 그리고 정상 `valid-agent_1.md` 파일 존재
- **When** `LoadAgentsDir(root)` 호출
- **Then** 4개 무효 이름 각각에 대해 `ErrInvalidAgentName` 누적, 로드 결과에는 `valid-agent_1`만 포함

**AC-SA-017 — Model inherit and explicit alias** (Verifies: REQ-SA-019)
- **Given** 부모 resolved model = `anthropic/claude-opus-4-7`, 두 AgentDefinition: (1) `def.Model == "inherit"`, (2) `def.Model == "sonnet-fast"`(ROUTER-001 alias), ROUTER-001 stub이 `sonnet-fast` → `anthropic/claude-sonnet-4-5` 매핑
- **When** 각각 `RunAgent` 호출 후 sub-agent `QueryEngine.cfg.Model` 조회
- **Then** (1)은 `anthropic/claude-opus-4-7`(부모 그대로), (2)는 `anthropic/claude-sonnet-4-5`(alias 해석됨). 매핑 실패 시 `ErrUnknownModelAlias` 반환(negative case)

**AC-SA-018 — MCP servers initialization and tool namespacing** (Verifies: REQ-SA-020)
- **Given** `def.MCPServers == ["context7", "pencil"]`, MCP-001 stub이 각각 `{search, resolve}` / `{batch_get, batch_design}` tool 제공
- **When** `RunAgent` 호출
- **Then** (a) `ConnectToServer("context7")`, `ConnectToServer("pencil")`가 `QueryEngine` 생성 전에 호출됨, (b) sub-agent `QueryEngine.cfg.Tools`에 `mcp__context7__search`, `mcp__context7__resolve`, `mcp__pencil__batch_get`, `mcp__pencil__batch_design` 4개 tool이 병합됨, (c) 부모 또는 다른 sub-agent의 tool과 namespace 충돌 없음

**AC-SA-019 — Memory append tool exposure** (Verifies: REQ-SA-021)
- **Given** `def.MemoryScopes == [ScopeProject]`, sub-agent가 system prompt에 memdir 항목을 받아 spawn
- **When** sub-agent가 `memory.append({scope:"project", category:"fact", key:"k1", value:"v1"})` 호출, 그리고 `memory.append({scope:"user", ...})` 호출(user scope는 미허용)
- **Then** 첫 호출은 성공, `{id, written_at}` 반환, `memdir.jsonl`에 entry 1건 append(REQ-SA-012 semantics 준수). 두 번째 호출은 `ErrScopeNotEnabled{scope:"user"}` 반환

**AC-SA-020 — Plan-mode approval wait** (Verifies: REQ-SA-022)
- **Given** `def.PermissionMode == "plan"`으로 spawn된 sub-agent, `TeammateIdentity.PlanModeRequired == true` 확인
- **When** sub-agent가 Write tool 호출 시도 → 호출자가 2초 뒤 `PlanModeApprove(agentID)` 호출 → sub-agent가 다시 Write tool 호출
- **Then** 첫 번째 시도는 `Decision{Behavior: AskParent}`로 보류되어 실제 Write 0회, `PlanModeApprove` 이후 두 번째 시도는 정상 통과하여 Write 실행 1회. 별도 케이스로 300s 내 미승인 시 `Subagent.State == Failed`, `reason == "plan_mode_timeout"`

**AC-SA-021 — Goroutine lifecycle on ctx cancel** (Verifies: REQ-SA-023)
- **Given** 3개 sub-agent가 각기 fork / worktree / background isolation으로 동시 실행 중
- **When** 공통 부모 `ctx` cancel
- **Then** (a) 모든 sub-agent의 goroutine이 `GoroutineShutdownGrace`(100 ms) 내 종료, (b) `goleak.VerifyNone(t)` 통과(leak 0), (c) memdir / worktree lock이 모두 release(재획득 성공으로 검증)

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/
└── subagent/
    ├── run.go              # RunAgent, Subagent 생명주기
    ├── fork.go             # Fork isolation
    ├── worktree.go         # Worktree isolation (git 조작)
    ├── background.go       # Background isolation
    ├── resume.go           # ResumeAgent, transcript 복원
    ├── memory.go           # 3-scope memdir, buildMemoryPrompt
    ├── loader.go           # LoadAgentsDir (.claude/agents/*.md)
    ├── isolation.go        # IsolationMode 추상화
    ├── permission.go       # TeammateCanUseTool
    ├── identity.go         # TeammateIdentity, ctx key
    └── *_test.go
```

### 6.2 핵심 Go 타입

```go
type IsolationMode string

const (
    IsolationFork       IsolationMode = "fork"
    IsolationWorktree   IsolationMode = "worktree"
    IsolationBackground IsolationMode = "background"
)

type MemoryScope string

const (
    ScopeUser    MemoryScope = "user"
    ScopeProject MemoryScope = "project"
    ScopeLocal   MemoryScope = "local"
)

// AgentDefinition은 .claude/agents/{name}.md의 frontmatter 파싱 결과.
type AgentDefinition struct {
    AgentType      string           // 파일명 기반 slug
    Name           string           // frontmatter name
    Description    string
    Tools          []string         // ["*"] = 부모 상속
    UseExactTools  bool
    Model          string           // "inherit" | alias
    MaxTurns       int
    PermissionMode string           // "bubble" | "isolated" | "plan"
    Effort         string           // L0/L1/L2/L3
    SystemPrompt   string           // markdown body
    MCPServers     []string         // MCP-001 ConnectToServer 호출 대상
    MemoryScopes   []MemoryScope    // REQ-SA-021: enabled scopes for memory.append tool;
                                    //   non-empty triggers built-in memory.append registration;
                                    //   loader-time default: [ScopeProject] when frontmatter omits it
    Isolation      IsolationMode
    Source         string           // "user" | "plugin" | "builtin"
    Background     bool             // shortcut for Isolation=Background
    CoordinatorMode bool
}

type TeammateIdentity struct {
    AgentID          string   // "researcher@parentSession-1"
    AgentName        string
    TeamName         string
    PlanModeRequired bool
    ParentSessionID  string
}

type Subagent struct {
    AgentID    string
    Definition AgentDefinition
    Engine     *query.QueryEngine
    State      SubagentState   // Running | Completed | Failed | Idle
    Identity   TeammateIdentity
    MemoryDir  string          // 선택된 scope의 디렉토리
    StartedAt  time.Time
    FinishedAt *time.Time
}

type SubagentInput struct {
    Prompt        string
    InitialMessages []message.Message    // 부모 context 상속 시
    Metadata      map[string]any
}

type SubagentResult struct {
    AgentID   string
    Terminal  query.Terminal
    Transcript []message.SDKMessage
}

// Memdir 엔트리.
type MemoryEntry struct {
    ID         string            `json:"id"`
    Timestamp  time.Time         `json:"ts"`
    Category   string            `json:"category"`
    Key        string            `json:"key"`
    Value      any               `json:"value"`
    Scope      MemoryScope       `json:"scope,omitempty"` // runtime 필드
}

// Context key (type aliasing으로 충돌 방지).
type teammateContextKey struct{}

func WithTeammateIdentity(ctx context.Context, id TeammateIdentity) context.Context {
    return context.WithValue(ctx, teammateContextKey{}, id)
}

func TeammateIdentityFromContext(ctx context.Context) (TeammateIdentity, bool) {
    v, ok := ctx.Value(teammateContextKey{}).(TeammateIdentity)
    return v, ok
}

// Spawn API.
func RunAgent(
    parentCtx context.Context,
    def AgentDefinition,
    input SubagentInput,
    opts ...RunOption,
) (*Subagent, <-chan message.SDKMessage, error)

func ResumeAgent(
    parentCtx context.Context,
    agentID string,
    opts ...RunOption,
) (*Subagent, <-chan message.SDKMessage, error)

// PlanModeApprove (REQ-SA-022, AC-SA-020)는 plan-mode로 spawn된 sub-agent의
// PlanModeRequired 게이트를 해제하는 부모-측 승인 API.
// 호출 후 해당 agent의 TeammateIdentity.PlanModeRequired = false 가 되어
// write-class tool 호출이 진행된다.
//
// 반환:
//   - nil:                  승인 성공
//   - ErrAgentNotFound:     agentID에 해당하는 활성 sub-agent 없음
//   - ErrAgentNotInPlanMode: 해당 agent가 plan mode가 아님
//   - ctx.Err():            parentCtx가 cancel/deadline-exceeded 상태
func PlanModeApprove(parentCtx context.Context, agentID string) error

// 3-scope memory.
type MemdirManager struct {
    agentType string
    scopes    []MemoryScope
    baseDirs  map[MemoryScope]string  // 각 scope의 실제 절대경로
}

func (m *MemdirManager) BuildMemoryPrompt() (string, error)
func (m *MemdirManager) Append(entry MemoryEntry) error
func (m *MemdirManager) Query(predicate func(MemoryEntry) bool) ([]MemoryEntry, error)
```

### 6.3 Isolation 3 모드 구현 비교

| 모드 | 구현 수단 | 부모로부터 상속 | 격리 대상 |
|---|---|---|---|
| **Fork** | `context.WithValue` + new QueryEngine | tools, messages[], cancellation, CanUseTool bubble | TaskBudget, AgentID, transcript dir |
| **Worktree** | Fork + `git worktree add` + CWD switch | Fork과 동일 | + 파일시스템(CWD), branch |
| **Background** | Fork + non-blocking goroutine | Fork과 동일 | + 병렬 실행(caller는 block 안 됨), idle timer |

### 6.4 AsyncLocalStorage → `context.Context` 매핑

| TypeScript | Go |
|---|---|
| `AsyncLocalStorage.run(identity, fn)` | `RunAgent(WithTeammateIdentity(ctx, id), ...)` |
| `AsyncLocalStorage.getStore()` | `TeammateIdentityFromContext(ctx)` |
| 중첩 scope | `context.WithValue`의 자연스런 중첩 (파생 ctx) |

TS는 async 콜스택 자동 전파. Go는 **명시적 ctx 전달**이 필요 — 모든 함수가 `ctx` 첫 인자로 받음. 본 SPEC의 모든 API가 이를 준수.

### 6.5 Memory Directory 구조

```
~/.goose/agent-memory/researcher/           # user scope
├── memdir.jsonl                            # 누적 memory entries (append-only)
├── metadata.json                           # { agentType, lastSession, ... }
├── transcript-{agentId}/
│   ├── messages.jsonl                      # SDKMessage stream
│   └── terminal.json                       # final Terminal
└── custom-*.json                           # agent 자유 저장

./.goose/agent-memory/researcher/           # project scope (동일 구조)
./.goose/agent-memory-local/researcher/     # local scope (동일 구조, .gitignore)
```

### 6.6 Worktree 라이프사이클

```go
func createWorktree(agentID string) (string, func(), error) {
    path := filepath.Join(".claude/worktrees", sanitize(agentID))
    branch := fmt.Sprintf("goose/agent/%s", sanitize(agentID))

    // add
    if err := exec.Command("git", "worktree", "add", "-b", branch, path).Run(); err != nil {
        return "", nil, err
    }

    cleanup := func() {
        _ = exec.Command("git", "worktree", "remove", "--force", path).Run()
        _ = exec.Command("git", "branch", "-D", branch).Run()
    }
    return path, cleanup, nil
}
```

cleanup은 `SubagentStop` hook + `SessionEnd` hook 양쪽에서 호출 보장 (idempotent).

### 6.7 Permission Bubbling (HOOK-001 SwarmWorkerHandler 연계)

```go
type TeammateCanUseTool struct {
    def           AgentDefinition
    parentCanUseTool permissions.CanUseTool
    hookDispatcher   HookDispatcher  // HOOK-001
}

func (t *TeammateCanUseTool) Check(ctx context.Context, toolName string, input json.RawMessage, permCtx permissions.ToolPermissionContext) permissions.Decision {
    // 1. Background agent write deny 사전 차단
    if t.def.Isolation == IsolationBackground && isWriteTool(toolName) && !hasAllowRule(toolName) {
        return permissions.Decision{Behavior: permissions.Deny, Reason: "background_agent_write_denied"}
    }

    // 2. PermissionMode 분기
    switch t.def.PermissionMode {
    case "isolated":
        return t.teammateLocalPolicy(ctx, toolName, input, permCtx)
    case "bubble":
        // HOOK-001의 SwarmWorkerHandler를 통해 부모에게 전달
        permCtx.Role = "swarmWorker"
        return t.parentCanUseTool.Check(ctx, toolName, input, permCtx)
    default:
        return t.teammateLocalPolicy(ctx, toolName, input, permCtx)
    }
}
```

### 6.8 TDD 진입 순서

1. **RED #1** — `TestLoadAgentsDir_ParsesFrontmatter` (minimal agent)
2. **RED #2** — `TestRunAgent_ForkIsolation` (AC-SA-001)
3. **RED #3** — `TestRunAgent_WorktreeIsolation` (AC-SA-002) — requires git fixture
4. **RED #4** — `TestRunAgent_BackgroundIsolation_NonBlocking` (AC-SA-003)
5. **RED #5** — `TestMemdir_ScopeResolutionOrder` (AC-SA-004)
6. **RED #6** — `TestSubagent_TranscriptPersisted` (AC-SA-005)
7. **RED #7** — `TestResumeAgent_LoadsTranscript` (AC-SA-006)
8. **RED #8** — `TestTeammateCanUseTool_BubbleToParent` (AC-SA-007)
9. **RED #9** — `TestRunAgent_NestedCoordinator_Warn` (AC-SA-008)
10. **RED #10** — `TestRunAgent_SpawnDepthExceeded` (AC-SA-009)
11. **RED #11** — `TestSubagent_ExplicitToolsFilter` (AC-SA-010)
12. **RED #12** — `TestBackgroundAgent_WriteDeniedByDefault` (AC-SA-011)
13. **RED #13** — `TestMemoryDir_Permission0700` (AC-SA-012)
14. **GREEN** — spawn + isolation 3 mode + memdir.
15. **REFACTOR** — permission bubbling을 HOOK-001 consumer로 clean abstract.

### 6.9 TRUST 5 매핑

| 차원 | 본 SPEC 달성 방법 |
|-----|-----------------|
| **T**ested | 35+ unit test, 12 integration test (AC 1:1), git fixture + fs tempdir, race detector |
| **R**eadable | 모드별 파일 분리(fork/worktree/background), identity/memory/permission 각 단일 책임 |
| **U**nified | `go fmt`, `golangci-lint`, 모든 spawn 경로가 `RunAgent` 단일 진입점 |
| **S**ecured | Memory dir 0700/0600 enforcement, background write deny, spawn depth 제한, cyclic 방지, sanitize(agent slug) |
| **T**rackable | `AgentID` 기반 zap 구조화 로그, transcript 영속화, Subagent lifecycle(Run→Complete)마다 이벤트 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | sub-agent를 감싸는 `QueryEngine` 재사용 |
| 선행 SPEC | SPEC-GOOSE-SKILLS-001 | `AgentDefinition`의 frontmatter 파서 allowlist 공유 |
| 선행 SPEC | SPEC-GOOSE-HOOK-001 | `SubagentStart`/`Stop`/`TeammateIdle`/`WorktreeCreate`/`Remove` dispatcher, `SwarmWorkerHandler` |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap 로거, context 루트, graceful shutdown |
| 후속 SPEC | SPEC-GOOSE-MCP-001 | `def.MCPServers`의 `ConnectToServer` 호출 |
| 후속 SPEC | SPEC-GOOSE-PLUGIN-001 | plugin manifest `agents:` 로딩 |
| 후속 SPEC | SPEC-GOOSE-ROUTER-001 | `def.Model` alias 해석 |
| 후속 SPEC | SPEC-GOOSE-MEMORY-001 (Phase 4) | memdir.jsonl의 구조화 저장 — 본 SPEC은 파일 기반만 |
| 외부 | Go 1.22+ | context, generics |
| 외부 | git binary | worktree isolation (`git worktree add/remove`) |
| 외부 | `gopkg.in/yaml.v3` v3.0+ | agent frontmatter 파싱 (SKILLS-001 공유) |
| 외부 | `go.uber.org/zap` v1.27+ | 로깅 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Worktree 생성 실패(git 없음, detached HEAD 등) | 중 | 고 | Fallback: `def.Isolation`을 `fork`로 자동 다운그레이드 + WARN 로그. 사용자 설정 `subagent.worktree.allowFallback=false`면 에러 반환 |
| R2 | Background goroutine 누수 | 고 | 고 | 모든 goroutine은 `ctx` 수신, parent `ctx.Done()` 시 자동 종료. `goleak` CI 검증 |
| R3 | Memdir concurrent write race | 중 | 중 | REQ-SA-012에 따라 `O_APPEND|O_SYNC` + single-write-per-entry. 여러 sub-agent가 동일 scope를 공유 시 file lock(`golang.org/x/sys/unix.Flock`) 추가 검토 |
| R4 | Cyclic spawn이 MaxSpawnDepth 이내에서 무한 루프 | 중 | 중 | `spawnDepth` 제한 + `MaxTurns`(QUERY-001) 이중 안전 장치 |
| R5 | ResumeAgent가 transcript 손상 시 crash | 낮 | 중 | 로드 실패 시 `ErrTranscriptCorrupted` 반환, 사용자에게 재시작 권고 |
| R6 | Permission bubbling이 parent 종료 후에도 호출됨 | 중 | 중 | `parentCanUseTool` 캐시 대신 매 호출마다 `ctx.Err()` 확인. parent ctx cancel되면 `Deny{parent_terminated}` |
| R7 | `.claude/agents/*.md`의 MoAI-ADK existing agent 26개가 본 SPEC 스키마와 100% 호환되지 않음 | 고 | 중 | 초기 스캔 도구로 호환성 리포트 생성. 미호환 agent는 `source: "legacy"` 태그 + WARN 로그, 점진적 마이그레이션 |
| R8 | Worktree cleanup 실패로 디스크 누수 | 중 | 낮 | `SessionEnd` hook + startup scan(`git worktree prune` + orphan directory 제거). 수동 `mink worktree gc` 커맨드 제공 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/research/claude-primitives.md` §4 Agent System, §4.1 runAgent 3단계, §4.2 Agent Memory 디렉토리, §4.3 Isolation 3 Mode, §4.4 Role Profile Override
- `.moai/specs/ROADMAP.md` §4 Phase 2 row 13 (SUBAGENT-001)
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` — `TeammateIdentity`, `CoordinatorMode`, `CanUseTool`
- `.moai/specs/SPEC-GOOSE-SKILLS-001/spec.md` — frontmatter allowlist 공유
- `.moai/specs/SPEC-GOOSE-HOOK-001/spec.md` — subagent lifecycle hooks, permission flow
- `.claude/rules/moai/workflow/worktree-integration.md` — 기존 MoAI worktree 규칙

### 9.2 외부 참조

- Claude Code source map: `./claude-code-source-map/` (`tools/AgentTool/` 패턴만)
- Git worktree docs: https://git-scm.com/docs/git-worktree
- MoAI-ADK `.claude/agents/`: 26개 기존 정의 파일

### 9.3 부속 문서

- `./research.md` — claude-primitives.md §4 원문 인용 + AsyncLocalStorage → context.Context 매핑 세부
- `../SPEC-GOOSE-QUERY-001/spec.md`
- `../SPEC-GOOSE-HOOK-001/spec.md`
- `../SPEC-GOOSE-MCP-001/spec.md`

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **QueryEngine 로직을 재구현하지 않는다**. QUERY-001의 `QueryEngine`을 재사용.
- 본 SPEC은 **Team 오케스트레이션(workflow.yaml role profiles, 다수 agent 동시 spawn 정책)을 구현하지 않는다**. 별도 SPEC(`TEAM-001`) 또는 CLI-001.
- 본 SPEC은 **Agent 간 mailbox 메시징을 구현하지 않는다**. stdin/stdout 스트리밍만.
- 본 SPEC은 **Agent definition marketplace / 자동 업데이트를 구현하지 않는다**. PLUGIN-001.
- 본 SPEC은 **LLM HTTP 호출을 구현하지 않는다**. ADAPTER-001.
- 본 SPEC은 **Context compaction 알고리즘을 구현하지 않는다**. CONTEXT-001.
- 본 SPEC은 **MCP 서버 본체를 구현하지 않는다**. MCP-001의 `ConnectToServer` 소비만.
- 본 SPEC은 **Remote agent spawn(gRPC)을 구현하지 않는다**. 단일 프로세스 내 sub-agent만.
- 본 SPEC은 **Memory의 구조화 쿼리/embedding 검색을 구현하지 않는다**. 순수 jsonl append-only. 구조화는 MEMORY-001(Phase 4).
- 본 SPEC은 **Coordinator/Worker 팀 분업 semantics를 정의하지 않는다**. `CoordinatorMode` 플래그 전파만.
- 본 SPEC은 **Legacy .claude/agents/*.md 전환 도구를 구현하지 않는다**. 호환성 스캔 리포트만.

---

## Implementation Notes (sync 정합화 2026-04-27)

- **Status Transition**: planned → implemented
- **Package**: `internal/subagent/` (23 파일, 5 `coverage*_test.go` 포함)
- **Core**: `run.go` (14KB main runtime), `loader.go`(AgentDefinition + MemoryScopes 필드), `memory.go`(3 scope 관리 — `ScopeUser`/`ScopeProject`/local), `permission.go`(`PlanModeApprove(agentID)` REQ-SA-022 + background write 게이트 차단), `worktree.go`(IsolationWorktree), `resume.go`(AgentID round-trip 파싱), `identity.go`, `types.go`
- **Isolation Modes**: `IsolationFork`, `IsolationBackground`, `IsolationWorktree` — `permission.go`에서 `IsolationBackground && isWriteTool(toolName)` 차단
- **Verified REQs (spot-check)**:
  - REQ-SA-001 `AgentID = {agentName}@{sessionId}-{spawnIndex}` 포맷 + `spawnIndex` 단조증가는 `atomic.AddInt64(&parentSpawnCounter, 1)` 보장
  - REQ-SA-012 `memdir.jsonl` 동시 쓰기 시 partial line 금지 — `O_APPEND|O_SYNC` + flock(LockFileEx) advisory lock 의무
  - REQ-SA-018 agentName 문자 집합 `[a-zA-Z0-9_]` (delimiter `-`/`@` 제외)
  - REQ-SA-021 `memory.append` 빌트인 tool 노출
  - REQ-SA-022 `PlanModeApprove(agentID)` API
  - 3 isolation mode + 3 memory scope
- **Test Coverage**: 9+ `_test.go` 파일 (loader, memory, types, spawn 16KB, terminal, resume, session_end, worktree, 5 coverage)
- **Lifecycle**: spec-anchored Level 2 — v0.3.0 plan-auditor iter1+iter2 결함 수정 모두 코드 반영

---

**End of SPEC-GOOSE-SUBAGENT-001**
