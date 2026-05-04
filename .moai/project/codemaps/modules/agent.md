# agent 패키지 — Agent Executor (Plan-Run-Reflect)

**위치**: internal/agent/  
**파일**: 24개 (.go + _test.go, plus subdirs)  
**상태**: ✅ Active (SPEC-GOOSE-AGENT-001)

---

## 목적

에이전트 런타임: Plan (목표 분석) → Run (도구 실행) → Reflect (학습). 각 세션마다 새 Agent 인스턴스.

---

## 공개 API

### Agent
```go
type Agent interface {
    // @MX:ANCHOR [AUTO] Core execution loop
    // @MX:REASON: Fan-in ≥3 (Query, Core, Subagent)
    Execute(ctx context.Context, task Task) (Result, error)
    
    LearnFrom(interaction Interaction) error
}

type Task struct {
    Prompt       string
    Messages     []Message
    Tools        []Tool
    Budget       int64
}

type Result struct {
    FinalMessage   string
    ToolUses       []ToolUse
    TokensUsed     int64
    InteractionLog Interaction
}
```

### AgentRunner
```go
type AgentRunner struct {
    manifest      Manifest        // Agent definition (YAML)
    llm           llm.Provider    // LLM for reasoning
    memory        memory.Provider // Context recall
    toolExecutor  *ToolExecutor
    logger        *zap.Logger
}

func (ar *AgentRunner) Execute(ctx context.Context, task Task) (Result, error)
```

---

## Plan-Run-Reflect 루프

### Phase 1: Plan (Analyze Intent)
```go
// 1. Recall context (memory.Recall)
//    └─ UserProfile, RecentInteractions, PreferenceVector

// 2. Formulate plan (LLM reasoning)
//    ├─ Understand task
//    ├─ Identify required tools
//    └─ Allocate budget

// 3. Build initial prompt
//    ├─ Include user message
//    ├─ Add recalled context
//    ├─ Insert tool definitions
//    └─ Estimate tokens
```

### Phase 2: Run (Execute & Tool Dispatch)
```go
// 1. LLM complete (stream mode)
//    └─ Chunks sent to output channel

// 2. Parse tool-use
//    ├─ Identify tool requests in response
//    ├─ Validate tool exists
//    └─ Check permission (REQ-PERM-001)

// 3. Execute tool
//    ├─ toolExecutor.Execute(toolName, args)
//    ├─ Sandbox execution (Extism WASM)
//    └─ Capture output

// 4. Feed result back to LLM (agentic loop)
//    └─ LLM consumes tool output, generates next response

// 5. Repeat until agent chooses to stop (no more tool-use)
```

### Phase 3: Reflect (Learn from Interaction)
```go
// 1. Collect interaction data
//    ├─ Initial prompt
//    ├─ All messages exchanged
//    ├─ Tools used + results
//    └─ User satisfaction (if available)

// 2. Send to learning engine (fire-and-forget)
//    └─ learning.Engine.Observe(interaction)

// 3. Update user profile (async)
//    ├─ Update preference vector
//    ├─ Log trajectory
//    └─ Store in memory
```

---

## Manifest (YAML Agent Definition)

```yaml
# internal/agent/manifest.go
name: developer
version: "1.0"
persona: |
  You are an expert software developer...
tools:
  - name: bash
    enabled: true
  - name: file_read
    enabled: true
  - name: file_write
    enabled: true
    restricted: true  # Requires permission
model: claude-opus-4
budget: 100000  # tokens
```

---

## Conversation State Machine

```go
type Conversation struct {
    messages    []Message
    toolUses    []ToolUse
    turnCount   int
    budgetUsed  int64
}

// Message role: "user" | "assistant" | "tool"
// Tool result messages inserted automatically after LLM tool-use

func (c *Conversation) AppendLLMResponse(content string, tools []ToolUse) error
    // Add assistant message
    // For each tool-use:
    //   1. Execute tool
    //   2. Append tool result message
    // Return final message
```

---

## Tool Execution Flow

```
Agent wants to use tool:
  │
  ├─ LLM response contains <tool_use>
  │   └─ toolName, args, toolUseID
  │
  ├─ Lookup tool in registry
  │   └─ tools.Registry.Resolve(toolName)
  │
  ├─ Check permission (REQ-PERM-001)
  │   ├─ permission.Request(triple=(user, tool, resource))
  │   └─ Wait for approval (or auto-approve if pre-approved)
  │
  ├─ Execute in sandbox
  │   ├─ toolExecutor.Execute(tool, args, input)
  │   ├─ Sandbox: Extism WASM (timeout 5s)
  │   └─ Capture: stdout, stderr, exit code
  │
  └─ Feed back to LLM
      └─ Append tool result to conversation
```

---

## @MX 주석

### @MX:ANCHOR
```go
// @MX:ANCHOR [AUTO] All agentic execution flows through here
// @MX:REASON: Query spawns Agent, Agent dispatches all tools/LLM
// @MX:SPEC: SPEC-GOOSE-AGENT-001
type Agent interface { Execute(...) }

// @MX:ANCHOR [AUTO] Tool sandbox entry point
// @MX:REASON: Security boundary - untrusted tool output
// @MX:WARN: [AUTO] goroutine spawn - timeout required
// @MX:REASON: Tool may hang - 5s deadline enforced
func (ar *AgentRunner) executeToolWithTimeout(...)
```

---

## 동시성 안전성

### Agent Isolation
```
QueryEngine.SubmitMessage
  └─ spawn Agent goroutine
      ├─ Single Agent instance per conversation
      ├─ No shared state with other agents
      └─ Can execute tools in parallel (future work)

External: Only Agent owns its Conversation.messages
  → Safe to mutate without locks
```

### Tool Execution
```go
// @MX:WARN [AUTO] Extism sandbox may panic
// @MX:REASON: Untrusted WASM - timeout + recover required

toolCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

result, err := toolExecutor.Execute(toolCtx, tool, args)
// timeout → SIGALRM → recover
```

---

## 에러 처리

### Tool Execution Errors
```
Error scenarios:
  1. Tool not found → send error to LLM
  2. Permission denied → send permission error, ask user
  3. Execution timeout (>5s) → return timeout error
  4. Sandbox panic → recover, return panic error
  5. Output too large (>10MB) → truncate + warn
```

### Recovery Strategy
```
Tool error in agentic loop:
  1. Capture error as tool result message
  2. Send to LLM with error context
  3. LLM can retry or use different tool
  4. Continue loop (max 5 turns default)
```

---

## 테스트

| 파일 | 커버리지 |
|------|---------|
| agent_test.go | Basic execution, tool dispatch |
| agent_extra_test.go | Edge cases, error scenarios |
| conversation_test.go | Message ordering, tool results |

---

## SPEC 참조

| SPEC | 상태 |
|------|------|
| SPEC-GOOSE-AGENT-001 | ✅ Plan-Run-Reflect |
| SPEC-TOOL-001 | ✅ Tool registry + sandbox |
| SPEC-PERM-001 | ✅ Permission checking |

---

**Version**: agent v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~240  
**@MX:ANCHOR Candidates**: 2 (Execute, executeToolWithTimeout)
