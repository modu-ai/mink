# Claude Code 4 Primitives 심층 분석 (Skills · MCP · Agents · Hooks)

> **분석일**: 2026-04-21 · **대상**: `claude-code-source-map/` · **용도**: SPEC-GOOSE-SKILLS-001 / MCP-001 / SUBAGENT-001 / HOOK-001 / PLUGIN-001

## 1. 4-Primitive 아키텍처

```
┌──────────────────────────────────────────────────────────┐
│                  Claude Code Runtime                      │
├──────────────────────────────────────────────────────────┤
│  Skills (Prompts) │ MCP (Tools) │ Agents (Sub-agents)    │
│        │                │                │                │
│        └────────────────┼────────────────┘                │
│  Hook System: 24 Runtime Events                          │
│  (SessionStart → PreToolUse → PostToolUse → Stop)       │
│        ▲                                                  │
│  Plugin Host (manifest.json 중심 4 primitive 패키징)     │
│        ▲                                                  │
│  Permission Hook: useCanUseTool (40KB)                   │
│  (classifier/deny/allow/ask · coordinator/swarm worker)  │
└──────────────────────────────────────────────────────────┘
```

## 2. Skill System

### 2.1 YAML Frontmatter 스키마

```yaml
---
name: custom-name                # 선택, 기본 디렉토리명
description: |                   # 선택, markdown
when-to-use: |                   # 선택, 모델 노출
argument-hint: "--flag value"
arguments: [arg1, arg2]
model: opus[1m]                  # 선택, "inherit" default
effort: L2                       # L0/L1/L2/L3 또는 정수
context: fork                    # "fork" | inline default
agent: expert                    # 에이전트 override
allowed-tools: [bash:readonly, read]
disable-model-invocation: false
user-invocable: true             # 모델 발견 가능
paths: [src/**/*.ts, "!**/test/**"]  # gitignore 조건부
shell: {executable: bash, deny-write: true}
hooks:
  SessionStart: [{command: setup.sh}]
  PostToolUse: [{command: log.sh}]
---
# Skill 본문
```

### 2.2 Progressive Disclosure (L0-L3)

| Level | 토큰 | 용도 |
|-------|-----|------|
| L0 | ~50 | 매우 경량, 빠른 응답 |
| L1 | ~200 | 표준 (default) |
| L2 | ~500 | 중급 |
| L3 | ~1000+ | 고급 |

`estimateSkillFrontmatterTokens()`: frontmatter만 파싱 (name + description + whenToUse), 전체 콘텐츠는 호출 시만 로드.

### 2.3 Trigger 메커니즘 (4종)

1. **Inline Skill** (기본): processPromptSlashCommand로 전개, !command + $ARG 치환
2. **Forked Skill** (context:fork): executeForkedSkill → runAgent, 독립 token budget + agentId
3. **Conditional Skill** (paths:): FileChanged hook → activateConditionalSkillsForPaths, gitignore 매칭
4. **Remote Skill** (EXPERIMENTAL_SKILL_SEARCH): _canonical_{slug} 접두사 → AKI/GCS에서 SKILL.md 로드

### 2.4 Model Invocation 제약

```
SkillTool.checkPermissions:
1. deny 규칙 검사 ("skill-name" 또는 "prefix:*")
2. disableModelInvocation 플래그 체크
3. SAFE_SKILL_PROPERTIES allowlist (새 프로퍼티 기본 deny)
4. allow 규칙 또는 자동 허가
```

## 3. MCP System

### 3.1 이중 역할

**클라이언트** (Claude Code → 외부 MCP 서버):
- `mcpClient.ts`: SSE, WebSocket, stdio 전송
- `connectToServer()` memoize (연결 풀링)
- `fetchToolsForClient()`: 도구 매니페스트 fetch (deferred)

**서버** (Claude Code → SDK 프로그램):
- `server/` 번들
- `createSdkMcpServer()`: tool() + handlers
- WebSocket over stdio

### 3.2 Transport & 프로토콜

```typescript
export class WebSocketTransport implements Transport {
  async request(req) → res
  async notify(msg) → void
  async close() → void
}

// MCP 메시지 타입:
- resources/read, resources/list
- tools/list, tools/call
- prompts/list (MCP prompt → Skill로 변환)
```

### 3.3 OAuth 흐름

```
1. OAuth 플로우 시작 → 브라우저
2. callback_url → code + state
3. Token exchange (백그라운드)
4. 저장: ~/.claude/mcp-credentials/{server-id}
5. 갱신: 자동 refresh_token
```

### 3.4 Deferred Loading (ToolSearch)

1. `connectToServer()` 호출 (도구 매니페스트 미로드)
2. 모델이 "mcp__..." 도구 언급 또는 MCPTool 직접 호출
3. Late binding: `fetchToolsForClient()`
4. 캐싱: memoize로 재연결 방지

**이름 충돌 해결**: `mcp__{serverName}__{toolName}` 접두사, 충돌 감지 + 넘버링

## 4. Agent System

### 4.1 생명주기 (runAgent())

```
[1] 초기화
  - agentDefinition 로드 (built-in / plugin / user)
  - MCP 서버 초기화 (initializeAgentMcpServers)
  - File state cache 생성
  - Transcript 서브디렉토리 설정

[2] 쿼리 루프
  - query() 호출
  - system prompt 렌더링
  - 권한 검사 (useCanUseTool hook)
  - API 요청 (token budget: effort 적용)
  - response 스트리밍 (assistant + tool_uses + text)
  - ToolCallProgress 발행
  - CompactionBoundary 검사
  - [도구 호출] → tool.call() → tool_result 수집

[3] 종료
  - final message 수집
  - Compaction 트리거 (조건부)
  - MCP 서버 정리
  - shell 작업 종료
  - transcript 저장
```

### 4.2 Agent Memory 디렉토리

```
scopes:
- user: ~/.claude/agent-memory/{agentType}/
- project: ./.claude/agent-memory/{agentType}/
- local: ./.claude/agent-memory-local/{agentType}/  (git-ignored)

각 디렉토리:
  {scope}/
  ├─ memdir.jsonl            # 구조화된 메모리 항목
  ├─ metadata.json           # 에이전트 메타데이터
  ├─ transcript-{sessionId}/
  └─ (커스텀 파일)
```

`buildMemoryPrompt()`: memdir.jsonl → 시스템 프롬프트 삽입. 모델이 메모리 쿼리/업데이트 가능.

### 4.3 Isolation 3 Mode

```
1. Fork (FORK_SUBAGENT):
   - 부모 컨텍스트 상속 (conversation, system prompt)
   - 독립 token budget (effort 재정의)
   - agentId 별도 생성
   - 백그라운드 실행

2. Worktree (EnterWorktreeTool):
   - 격리된 git worktree
   - WorktreeCreate hook 발동
   - file state cache 독립
   - shell cwd 격리

3. Background (background: true):
   - subprocess 아님
   - 동일 프로세스, Task 중심
   - 백그라운드 polling
```

### 4.4 Role Profile Override

```typescript
AgentDefinition {
  agentType: string
  tools: string[]          // ["*"] = 부모 도구 상속
  useExactTools?: boolean  // true: 정확히 부모와 동일
  model?: "inherit" | ModelAlias
  maxTurns?: number
  permissionMode?: "bubble" | "isolated"
  effort?: EffortValue
  getSystemPrompt: () => string
  mcpServers?: (string | {...})[]
  source: "plugin" | "built-in" | "user"
}
```

## 5. Hook System

### 5.1 24개 런타임 이벤트

```
[초기화]
- Setup, SessionStart, SubagentStart

[쿼리 루프]
- UserPromptSubmit
- PreToolUse, PostToolUse, PostToolUseFailure

[컨텍스트 변화]
- CwdChanged, FileChanged
- WorktreeCreate, WorktreeRemove

[권한 & 사용자 상호작용]
- PermissionRequest, PermissionDenied
- Notification
- Elicitation, ElicitationResult

[종료]
- PreCompact, PostCompact
- Stop, StopFailure

[팀 & 백그라운드]
- SubagentStop, TeammateIdle
- TaskCreated, TaskCompleted

[기타]
- SessionEnd, ConfigChange, InstructionsLoaded
```

### 5.2 Permission Hook 플로우 (useCanUseTool 40KB)

```
1. hasPermissionsToUseTool() → PermissionResult
   - behavior: "allow" | "deny" | "ask"
   - decisionReason?: { type, reason, ... }

2. Permission Decision 핸들러:
   - interactiveHandler (사용자 터미널 프롬프트)
   - coordinatorHandler (Classifier 통과 대기)
   - swarmWorkerHandler (팀장 권한 버블링)

3. PermissionQueueOps:
   - setYoloClassifierApproval() (auto-mode)
   - recordAutoModeDenial() (거부 추적)
   - logPermissionDecision() (텔레메트리)
```

### 5.3 Hook Callback 페이로드

```typescript
type HookInput = {
  hookEvent: HookEvent
  toolUseID?: string
  tool?: SdkToolInfo
  input?: Record<string, unknown>
  output?: string | Record<string, unknown>
  error?: { code?: string; message: string }
  message?: SDKMessage
}

type HookJSONOutput =
  | { continue?: boolean; suppressOutput?: boolean }
  | { async: true; asyncTimeout?: number }
```

**Blocking 방식**:
- PreToolUse: `{ continue: false }` → 도구 호출 차단 + `permissionDecision` 승인/거부 명시
- SessionStart: `initialUserMessage`, `watchPaths` 설정
- PostToolUse: `updatedMCPToolOutput`, `additionalContext`

## 6. Plugin 매니페스트 스키마

```json
{
  "name": "plugin-name",
  "description": "...",
  "version": "1.0.0",
  "skills": [{"id": "...", "path": "skills/.../SKILL.md"}],
  "agents": [{"id": "...", "path": "agents/....md"}],
  "mcpServers": [{"name": "...", "command": "node", "args": [...]}],
  "commands": [{"name": "...", "description": "..."}],
  "hooks": {
    "SessionStart": [
      {"matcher": "**/*.ts", "hooks": [{"command": "setup.sh"}]}
    ]
  }
}
```

**Plugin Loading Pipeline**:
1. Discovery (`~/.claude/plugins/`, `.claude/plugins/`, 마켓플레이스)
2. Validation (manifest 스키마, 이름 예약어, hook 스키마)
3. Primitive 로드 (loadPlugin{Skills,Agents,Commands,Hooks})
4. Runtime Registration (atomic swap)

**MCPB 파일** (복합 서버): DXT 매니페스트 + user config variables 치환.

## 7. GOOSE Go 포팅 매핑

| Primitive | Claude Code | GOOSE Go |
|-----------|------------|----------|
| **Skills** | skills/, loadSkillsDir.ts, parseSkill* | internal/skill/{loader, parser, schema, conditional, remote}.go |
| **MCP** | tools/MCPTool/, mcpWebSocket*, mcpAuth* | internal/mcp/{client, server, transport, validation, auth}.go |
| **Agents** | tools/AgentTool/ | internal/subagent/{run, fork, resume, memory, loader}.go |
| **Hooks** | hooks/, useCanUseTool, toolPermission/ | internal/hook/{types, permission, handlers, plugin_loader}.go |
| **Plugin Host** | utils/plugins/ | internal/plugin/{validator, walker, loader, mcp_integration, mcpb}.go |

## 8. GOOSE SPEC 도출

### SPEC-GOOSE-SKILLS-001
- YAML frontmatter parsing
- 조건부 활성화 (gitignore 패턴)
- 원격 로드 (${CLAUDE_SKILL_DIR}, ${CLAUDE_SESSION_ID} 치환)
- 실행 모드: inline / fork / conditional
- 권한 제어: disableModelInvocation, allowedTools, SAFE_SKILL_PROPERTIES

### SPEC-GOOSE-MCP-001
- 양방향 통신 (클라이언트 + 서버)
- 전송: stdio/WebSocket/SSE
- 리소스: resources/read, tools/list, prompts/list
- OAuth 2.0 + refresh token
- Deferred loading (ToolSearch)
- 이름 충돌: mcp__{server}__{tool}

### SPEC-GOOSE-SUBAGENT-001
- 생명주기 3단계
- Isolation: fork / worktree / background
- Memory scopes: user / project / local
- Role profile override

### SPEC-GOOSE-HOOK-001
- 24 runtime events
- Blocking 방식 (PreToolUse continue=false, watchPaths, additionalContext)
- Permission flow (allow/deny/ask)
- Plugin hook 통합 (clearThenRegister 원자성)

### SPEC-GOOSE-PLUGIN-001
- manifest.json 스키마
- 4 primitive 패키징
- MCPB 파일 + user config variables 치환

## 9. 재사용 vs 재작성

**재사용 가능 (80%)**:
- Skill YAML Parser
- Hook Event 정의 (24개 enum)
- MCP Schema (transport-agnostic)
- Permission Decision Tree
- Agent Memory Directory Layout
- Conditional Skill (paths:) — Go gitignore 라이브러리

**부분 재사용 (15%)**:
- Skill Loading (TS 동적 로딩 → Go 정적 빌드)
- MCP WebSocket (Bun stdio → Go goroutines)
- Agent Lifecycle (async/await → channels)
- Plugin Host (Zod validation → Go struct tags)

**완전 재작성 (5%)**:
- useCanUseTool React Hook (React-specific)
- Terminal UI Rendering (ink.js → Go TUI)
- File State Cache (TS Set/Map → sync.Map)
- Session Compaction

## 10. 설계 원칙 (GOOSE 계승)

1. **Progressive Disclosure (L0-L3)**: Effort 기반 토큰 예산
2. **Atomic State Transitions**: clearThenRegister (hot-reload)
3. **Deferred Loading**: MCP 도구 매니페스트 지연 + Skill 조건부 활성화
4. **Allowlist-Default Deny**: SAFE_SKILL_PROPERTIES, 명시적 allow
5. **Isolation + Bubbling**: Fork 백그라운드, Worktree 디렉토리, permissionMode bubble
6. **Composable Plugins**: 4 primitive 각각 plugin-loadable, manifest.json 중심

---

**결론**: 4-primitive는 강결합 모놀리식 → 플러그 앤 플레이 모듈식. GOOSE 포팅 **재사용 80% / 부분재작성 15% / 완전재작성 5%**.
