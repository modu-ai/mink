// Package agent provides the outer orchestration layer for the Plan-Run-Reflect cycle.
// SPEC-GOOSE-AGENT-RUNNER-001
package agent

// TaskState represents the lifecycle state of a task.
type TaskState int

const (
	// TaskPending is the initial state before execution.
	TaskPending TaskState = iota
	// TaskRunning indicates the task is currently executing.
	TaskRunning
	// TaskCompleted indicates the task finished successfully (awaiting reflection).
	TaskCompleted
	// TaskReflected indicates reflection has been performed (final state).
	TaskReflected
	// TaskCancelled indicates the task was cancelled by context.
	TaskCancelled
	// TaskFailed indicates the task failed with an error.
	TaskFailed
)

// String returns the string representation of TaskState.
func (s TaskState) String() string {
	switch s {
	case TaskPending:
		return "pending"
	case TaskRunning:
		return "running"
	case TaskCompleted:
		return "completed"
	case TaskReflected:
		return "reflected"
	case TaskCancelled:
		return "cancelled"
	case TaskFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Task represents a unit of work for the AgentRunner.
type Task struct {
	// ID is the unique task identifier.
	ID string
	// Prompt is the user prompt for this task.
	Prompt string
	// State is the current lifecycle state.
	State TaskState
	// Metadata holds arbitrary task metadata.
	Metadata map[string]any
}

// TaskResult contains the outcome of a completed task.
type TaskResult struct {
	// TaskID is the ID of the completed task.
	TaskID string
	// FinalState is the final loop.State after task execution.
	FinalState interface{} // loop.State from internal/query/loop
	// Reflect contains the reflection result if hooks were run.
	Reflect *ReflectResult
	// ReplanCount is the number of re-plan iterations performed.
	ReplanCount int
	// Error is any error that occurred during execution.
	Error error
}
