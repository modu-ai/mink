# Claude Code Agentic Core 심층 분석

> **분석일**: 2026-04-21 · **대상**: `claude-code-source-map/` · **수준**: Very Thorough · **용도**: GOOSE-AGENT SPEC-GOOSE-QUERY-001 / TASK-001 / CONTEXT-001 근거

## 1. Core Loop 구조

```
QueryEngine.submitMessage(prompt) → AsyncGenerator<SDKMessage, void>
  ↓
processUserInput() (slash commands, parsing)
  ↓
query(params) → AsyncGenerator<StreamEvent|Message, Terminal>
  ↓
queryLoop() — while true:
  1. yield stream_request_start
  2. API call: messages → response
  3. yield chunks (StreamEvent)
  4. parse tool_use blocks
  5. canUseTool permission gate
  6. runTools() dispatch & execute
  7. yield tool results (Message)
  8. check continuation (budget?)
  9. yield CompactBoundary (optional)
 10. compact/microcompact/snip
 11. state = continue | terminal
  ↓
Terminal { success, error, text }
```

**핵심 특성**:
- Async Generator (`yield*` nested generator 조합)
- Streaming-first (buffering 금지)
- 상태 지속성 (State mutable, continue site에서 재할당)
- Budget Tracking (task_budget + token_budget, compaction 경계마다 누적)

## 2. 5개 Task 타입 매트릭스

| Task Type | 역할 | 생명주기 | 컨텍스트 | 신원 |
|-----------|------|--------|---------|-----|
| **LocalAgentTask** | 동일 프로세스 agent spawn | fork → run → msg loop | sync (in-memory) | agentId, teamName |
| **RemoteAgentTask** | 원격 세션 agent | CLI spawn → SSH/RPC → msg queue | async (network) | 원격 세션 ID |
| **LocalShellTask** | 로컬 Bash 실행 | exec → capture → done | sync (subprocess) | PID, cwd |
| **InProcessTeammateTask** | in-process 팀원 | spawn → AsyncLocalStorage | sync + mailbox | agentId@teamName, color |
| **DreamTask** | 장기 백그라운드 | scheduled → async run | async (deferred) | 사용자정의 |

## 3. 핵심 인터페이스 (TS 원문)

```typescript
export type QueryEngineConfig = {
  cwd: string
  tools: Tools
  commands: Command[]
  mcpClients: MCPServerConnection[]
  agents: AgentDefinition[]
  canUseTool: CanUseToolFn  // permission gate
  getAppState: () => AppState
  setAppState: (f: (prev: AppState) => AppState) => void
  initialMessages?: Message[]
  maxTurns?: number
  maxBudgetUsd?: number
  taskBudget?: { total: number }
}

export class QueryEngine {
  async *submitMessage(
    prompt: string | ContentBlockParam[],
    options?: { uuid?: string; isMeta?: boolean }
  ): AsyncGenerator<SDKMessage, void, unknown>
}

type State = {
  messages: Message[]
  toolUseContext: ToolUseContext
  autoCompactTracking: AutoCompactTrackingState | undefined
  maxOutputTokensRecoveryCount: number
  turnCount: number
  transition: Continue | undefined
}

export type TeammateIdentity = {
  agentId: string               // "researcher@my-team"
  agentName: string
  teamName: string
  planModeRequired: boolean
  parentSessionId: string
}

export type InProcessTeammateTaskState = TaskStateBase & {
  type: 'in_process_teammate'
  identity: TeammateIdentity
  prompt: string
  permissionMode: PermissionMode
  awaitingPlanApproval: boolean
  messages?: Message[]          // UI mirror cap 50
  pendingUserMessages: string[] // mailbox
  isIdle: boolean
  shutdownRequested: boolean
}

export const getSystemContext = memoize(async () => ({
  gitStatus, cacheBreaker  // branch, commits, status + ant-only debug
}))

export const getUserContext = memoize(async () => ({
  claudeMd, currentDate  // CLAUDE.md walk + date injection
}))
```

## 4. GOOSE Go 포팅 매핑

| Claude Code (TS) | GOOSE (Go) | 패키지 |
|-----------------|-----------|-------|
| QueryEngine | QueryEngine | `internal/query/` |
| query() + queryLoop() | QueryLoop | `internal/query/loop/` |
| Message[] | []Message | `internal/message/` |
| ToolUseContext | ToolUseContext | `internal/context/` |
| canUseTool() | CanUseTool | `internal/permissions/` |
| runTools() | RunTools | `internal/tools/` |
| Task types | TaskState (discriminated) | `internal/task/` |
| TeammateIdentity | TeammateIdentity | `internal/teammate/` |
| getSystemContext() | GetSystemContext | `internal/context/` |
| upstreamproxy | LLMProxy | `internal/llm/proxy/` |

**포팅 패턴**:
- AsyncGenerator → Go channels + goroutines
- Mutable state → loop-local var + continue site 재할당
- Permission gate → middleware-style canUseTool wrapper
- Budget tracking → loop-local variables (taskBudgetRemaining)

## 5. 재사용 vs 재작성

### 재사용 가능 (로직 이식)
1. Query loop 상태 머신 (continue site 조건)
2. applyToolResultBudget (content replacement 추적)
3. Compaction 의사결정 (autoCompact, reactiveCompact, snip)
4. canUseTool → result.behavior (allow/deny/ask) 매핑
5. Teammate model resolution ('inherit' alias + fallback chain)
6. Git context 추출 + truncation
7. ToolUseSummaryMessage 포맷

### 부분 재작성
- AsyncGenerator → Go channels
- Message 직렬화 (TS ContentBlockParam → Go protobuf/JSON)
- AppState (React/Zustand → Go struct + mutex/channels)
- MCPServerConnection 드라이버 (TS MCP client → Go MCP client)
- File cache (TS Set/Map → sync.Map)

### 완전 재작성
- React/Ink 렌더링 (CLI-only)
- AsyncLocalStorage → context.Context
- Remote agent protocol (Go net/rpc 또는 gRPC)

## 6. 설계 원칙 (10가지)

1. **One QueryEngine per conversation**: 세션 생명주기 = engine 생명주기
2. **Streaming mandatory**: SDK 호출자는 응답을 즉시 받아야 함
3. **Tool call permission gate is async**: canUseTool() await 가능
4. **Message arrays mutable within a turn**: 다음 턴에는 새 스냅샷
5. **Budget tracking cumulative across compactions**: task_budget.remaining은 compaction 경계마다 누적
6. **Continue sites explicit**: 복원 경로는 state를 새로 할당
7. **Task types are union, not polymorphic**: discriminated union
8. **Async agents identity-scoped**: InProcessTeammate agentId+teamName + AsyncLocalStorage
9. **Coordinator mode changes tool visibility**: isCoordinatorMode() → worker 도구만
10. **Context memoized per session**: getSystemContext() + getUserContext() 한 번만 계산

## 7. GOOSE SPEC 도출

### SPEC-GOOSE-QUERY-001: QueryEngine & Query Loop

EARS 요구사항:
```
GIVEN QueryEngine configured with (cwd, tools, maxTurns, maxBudgetUsd)
WHEN submitMessage(prompt) is called
THEN return AsyncGenerator that yields SDKMessage in sequence
  (user ack → stream_request_start → StreamEvents → Message array 
   → ToolUseSummaryMessage) UNTIL terminal state

AND query loop MUST:
- Normalize messages before API call
- Apply tool result budget (content replacement)
- Apply compaction (autoCompact/reactiveCompact/snip) based on token window
- Parse tool_use blocks and invoke canUseTool gate
- Execute permitted tools via runTools()
- Track maxOutputTokensRecoveryCount (<= 3)
- Update task_budget.remaining cumulatively
- Yield CompactBoundary on compaction
- Return Terminal { success, error?, text }

AND streaming MUST NOT buffer, support abortController, support middleware hooks
```

### SPEC-GOOSE-TASK-001: Task Type Hierarchy

5 task types (discriminated union), 각각 생명주기 콜백(onStarted, onProgress, onCompleted, onError) + AppState.tasks[] 등록 + stop() 전파. InProcessTeammate는 TeammateIdentity + mailbox + UI cap 50 + permissionMode cycling + idle notification. RemoteAgent는 task-notification XML + resume from sessionId.

### SPEC-GOOSE-CONTEXT-001: Context Window Management

systemPrompt + userContext + systemContext 로드(memoized), normalize messages, tokenCountWithEstimation, calculateTokenWarningState 기반 auto-compact trigger, compaction 적용(autoCompact/reactiveCompact/snip), task_budget 누적 추적, CompactBoundary yield, snip 경계는 redacted_thinking 보존.

## 8. 고리스크 영역

- **Circular async generator**: queryLoop() 중 processUserInput 호출로 model/system prompt/message array 변경 가능
- **Mutable message array**: mutableMessages 수정이 후속 iteration에 영향
- **Budget undercount**: compaction 후 task_budget.remaining 미추적 시 오차 누적
- **Tool permission race**: canUseTool() 실행 중 AppState.toolPermissionContext 변경 가능

## 9. 결론

Claude Code agentic core = **async generator streaming query loop + 5 task type union**. GOOSE 포팅 시:
- streaming 채널
- context.Context로 AsyncLocalStorage 대체
- Go struct + mutex for AppState
- **쿼리 루프 상태 머신과 continue site 로직은 원문 그대로 이식 가능**

---

**재사용률**: 80% (로직 이식) / 15% (부분 재작성) / 5% (완전 재작성)
