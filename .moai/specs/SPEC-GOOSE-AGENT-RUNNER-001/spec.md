---
id: SPEC-GOOSE-AGENT-RUNNER-001
version: 0.1.0
status: completed
created_at: 2026-04-28
updated_at: 2026-05-04
author: MoAI orchestrator
priority: P1
issue_number: null
phase: 3
milestone: M3
size: 중(M)
lifecycle: spec-first
labels: [area/runtime, area/query, type/feature, priority/p1-high]
---

# SPEC-GOOSE-AGENT-RUNNER-001 — Agent Runner: Plan-Run-Reflect Outer Orchestration

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-28 | SELF-CRITIQUE-001의 "Plan-Run-Sync engine" 종속성 해제를 위해 신규 작성. QueryEngine(QUERY-001) 위에 올라가는 outer orchestration layer 정의. | MoAI |

---

## 1. Overview

SPEC-GOOSE-QUERY-001의 `QueryEngine`은 per-message inner loop(LLM→tool→LLM→…)을 처리한다. 본 SPEC은 그 위에 올라가는 **outer orchestration layer**를 정의한다: Plan → Run → Reflect → Sync.

이 레이어는 사용자의 고수준 목표를 Task 단위로 관리하고, 각 Task 완료 후 self-critique(Reflect)를 실행하며, 품질 미달 시 re-plan을 트리거한다.

### 기존 인프라 (재사용)

| Component | Location | Role |
|-----------|----------|------|
| QueryEngine | `internal/query/engine.go` | Per-message inner loop, SubmitMessage, OnComplete |
| loop.Run | `internal/query/loop/loop.go` | LLM→tool iteration state machine |
| PostSamplingHooks | `internal/query/engine.go` | Post-LLM-sampling hook injection |
| LoopController | `internal/query/cmdctrl/controller.go` | Slash command side-effect bridge |
| ContextAdapter | `internal/command/adapter/adapter.go` | Slash command context (plan mode, model, aliases) |

### 본 SPEC이 새로 만드는 것

1. `Task` lifecycle type (pending → running → completed → reflected)
2. `AgentRunner` orchestrator (Plan → Run → Reflect cycle)
3. `ReflectHook` interface (post-task critique injection point)
4. Re-plan decision logic (score-based, max 2 retries)

---

## 2. Scope

### IN SCOPE

1. `Task` type definition with lifecycle states
2. `AgentRunner` struct that orchestrates Plan-Run-Reflect cycle using QueryEngine
3. `ReflectHook` function type for post-task self-critique
4. Re-plan trigger: when reflect score < 0.7, re-plan up to 2 times
5. `TaskResult` type containing reflection output
6. Integration point: QueryEngine's `OnComplete` → AgentRunner's reflect phase

### OUT OF SCOPE

1. QueryEngine internals (QUERY-001 territory)
2. Slash command dispatch (COMMAND-001 territory)
3. LLM provider details (LLM-001 territory)
4. RPC/transport layer (TRANSPORT-001 territory)
5. Daily reflection aggregation (SELF-CRITIQUE-001 REQ-SELF-CRITIQUE-004)
6. File persistence of reflection results (SELF-CRITIQUE-001 REQ-SELF-CRITIQUE-003)

---

## 3. Requirements (EARS)

### REQ-ARUN-001
**WHEN** `AgentRunner.RunTask(ctx, task)` is invoked, the system **SHALL** create a new `QueryEngine` session (or reuse existing), call `SubmitMessage(ctx, task.Prompt)`, and wait for the inner loop to complete via the `OnComplete` callback.

### REQ-ARUN-002
**WHEN** the QueryEngine's inner loop completes (OnComplete fires), the system **SHALL** invoke all registered `ReflectHook` functions with the final `loop.State` and collect the resulting `ReflectResult`.

### REQ-ARUN-003
**WHEN** `ReflectResult.Score < 0.7`, the system **SHALL** trigger re-plan: construct a new `Task` with the critique feedback appended to the prompt, and re-run up to 2 times (total 3 attempts including initial).

### REQ-ARUN-004
**WHEN** `ReflectResult.Score >= 0.7` or max re-plan attempts exhausted, the system **SHALL** return the `TaskResult` containing the final loop state, reflection output, and re-plan count.

### REQ-ARUN-005
**UBIQUITOUS** The `AgentRunner` **SHALL** accept a `QueryEngineConfig` factory function to create per-task QueryEngine instances with consistent configuration.

### REQ-ARUN-006
**WHEN** the context is cancelled during RunTask execution, the system **SHALL** propagate cancellation to the QueryEngine and return the partial `TaskResult` with `TaskStateCancelled`.

---

## 4. Data Model

### Task Lifecycle

```
TaskPending → TaskRunning → TaskCompleted → TaskReflected
                                ↓ (re-plan)
                           TaskPending (new task with critique feedback)
```

### Types

```go
// TaskState represents the lifecycle state of a task.
type TaskState int

const (
    TaskPending    TaskState = iota
    TaskRunning
    TaskCompleted
    TaskReflected
    TaskCancelled
    TaskFailed
)

// Task represents a unit of work for the AgentRunner.
type Task struct {
    ID       string
    Prompt   string
    State    TaskState
    Metadata map[string]any // optional context (SPEC reference, requirements, etc.)
}

// TaskResult contains the outcome of a completed or failed task.
type TaskResult struct {
    TaskID       string
    FinalState   loop.State
    Reflect      *ReflectResult
    ReplanCount  int
    Error        error
}

// ReflectResult contains the self-critique output.
type ReflectResult struct {
    Score         float64 // 0.0 to 1.0
    Gap           string  // identified gaps
    Inconsistency string  // identified inconsistencies
    Unsupported   string  // unsupported claims
    RawOutput     string  // full LLM output
}

// ReflectHook is a function that performs post-task self-critique.
// It receives the task prompt and final loop state, returns critique result.
type ReflectHook func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error)
```

### AgentRunner

```go
// AgentRunnerConfig holds configuration for the AgentRunner.
type AgentRunnerConfig struct {
    // NewEngineConfig creates a QueryEngineConfig for a new task.
    NewEngineConfig func(task Task) (query.QueryEngineConfig, error)
    // ReflectHooks are invoked after each task completion.
    ReflectHooks []ReflectHook
    // MaxReplans is the maximum number of re-plan attempts (default: 2).
    MaxReplans int
    // Logger receives structured log output.
    Logger *zap.Logger
}

// AgentRunner orchestrates the Plan-Run-Reflect cycle.
type AgentRunner struct {
    cfg AgentRunnerConfig
}

// NewAgentRunner creates a new AgentRunner with the given config.
func NewAgentRunner(cfg AgentRunnerConfig) (*AgentRunner, error)

// RunTask executes a single task through the Plan-Run-Reflect cycle.
func (r *AgentRunner) RunTask(ctx context.Context, task Task) (*TaskResult, error)
```

---

## 5. RunTask Flow

```
RunTask(ctx, task):
  1. Validate task (non-empty prompt)
  2. For attempt = 0..MaxReplans:
     a. Create QueryEngine via cfg.NewEngineConfig(task)
     b. Set OnComplete callback to capture final loop.State
     c. Call engine.SubmitMessage(ctx, task.Prompt)
     d. Stream SDKMessages to caller (passthrough)
     e. Wait for OnComplete (loop finished)
     f. Set task.State = TaskCompleted
     g. For each ReflectHook:
        - Call reflectHook(ctx, task, finalState)
        - Collect ReflectResult
     h. If reflectResult.Score >= 0.7:
        - Set task.State = TaskReflected
        - Return TaskResult (success)
     i. If reflectResult.Score < 0.7 and attempt < MaxReplans:
        - Construct re-plan prompt: original + critique feedback
        - Update task.Prompt with re-plan prompt
        - Increment ReplanCount
        - Continue loop (retry)
     j. If max replans exhausted:
        - Return TaskResult with last reflectResult
  3. On ctx cancellation:
     - Return TaskResult with TaskCancelled + partial state
```

---

## 6. Acceptance Criteria

### AC-ARUN-001 — Basic task execution

**Given** a valid Task with prompt "list files in /tmp"
**When** `AgentRunner.RunTask(ctx, task)` is called with no ReflectHooks
**Then** the QueryEngine.SubmitMessage is called exactly once, the loop completes, and TaskResult.FinalState contains the accumulated messages.

### AC-ARUN-002 — Reflect hook invocation

**Given** a Task and one ReflectHook that returns score=0.8
**When** RunTask completes
**Then** the ReflectHook is called with the correct Task and final loop.State, and TaskResult.Reflect.Score == 0.8, TaskResult.ReplanCount == 0.

### AC-ARUN-003 — Re-plan trigger (score < 0.7)

**Given** a Task and a ReflectHook that returns score=0.4 on first call, score=0.85 on second call
**When** RunTask is called with MaxReplans=2
**Then** the QueryEngine.SubmitMessage is called exactly twice (initial + 1 re-plan), TaskResult.ReplanCount == 1, TaskResult.Reflect.Score == 0.85.

### AC-ARUN-004 — Max replans exhausted

**Given** a Task and a ReflectHook that always returns score=0.3
**When** RunTask is called with MaxReplans=2
**Then** SubmitMessage is called 3 times (initial + 2 replans), TaskResult.ReplanCount == 2, TaskResult.Reflect.Score == 0.3.

### AC-ARUN-005 — Context cancellation

**Given** a running task with context that gets cancelled mid-loop
**When** the context is cancelled
**Then** RunTask returns TaskResult with TaskCancelled state, no panic, and the partial FinalState is populated.

### AC-ARUN-006 — Multiple reflect hooks

**Given** 2 ReflectHooks returning scores 0.6 and 0.9
**When** RunTask completes
**Then** both hooks are called in registration order, and the minimum score (0.6) is used for re-plan decision.

---

## 7. Dependencies

### Resolved

| Dependency | Status | Notes |
|------------|--------|-------|
| SPEC-GOOSE-QUERY-001 (QueryEngine) | Implemented | Core inner loop |
| SPEC-GOOSE-CORE-001 (Runtime) | Implemented | State management, drain |
| SPEC-GOOSE-CMDCTX-001 (ContextAdapter) | Implemented | Slash command context |
| SPEC-GOOSE-CMDLOOP-WIRE-001 (LoopController) | Implemented | Command side-effect bridge |

### Enables

| Dependent SPEC | What this SPEC provides |
|---------------|------------------------|
| SPEC-GOOSE-SELF-CRITIQUE-001 | ReflectHook interface, Task lifecycle, re-plan logic |

---

## 8. Non-Functional

| 항목 | 기준 |
|------|------|
| Race detector | `go test -race -count=10` clean |
| Coverage | ≥ 85% for new code |
| Latency | ReflectHook overhead ≤ 100ms per hook (excludes LLM call time) |
| No goroutine leak | All spawned goroutines complete on context cancellation |

---

## 9. File Layout

```
internal/agent/
├── runner.go          # AgentRunner, RunTask orchestration
├── runner_test.go     # Unit tests for re-plan logic, cancellation
├── task.go            # Task, TaskState, TaskResult types
├── reflect.go         # ReflectResult, ReflectHook type definitions
└── reflect_test.go    # ReflectHook integration tests
```

---

## 10. Risks

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | ReflectHook LLM call latency adds to task time | Acceptable per SELF-CRITIQUE-001 AC-04 (+20-40%) |
| R-2 | Re-plan infinite loop if MaxReplans misconfigured | Hard cap with MaxReplans validation (0-5 range) |
| R-3 | QueryEngine not reusable across replans | Create new instance per attempt via factory function |

---

Version: 0.1.0
Last Updated: 2026-04-28
Status: planned
