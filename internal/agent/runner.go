// Package agent provides the outer orchestration layer for the Plan-Run-Reflect cycle.
// SPEC-GOOSE-AGENT-RUNNER-001
package agent

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/query"
	"github.com/modu-ai/mink/internal/query/loop"
	"go.uber.org/zap"
)

const (
	// MinPassScore is the minimum reflection score required to avoid re-planning.
	MinPassScore = 0.7
	// DefaultMaxPlans is the default maximum number of re-plan iterations.
	DefaultMaxPlans = 2
)

// EngineConfigFactory creates a QueryEngineConfig for a given task.
// SPEC-GOOSE-AGENT-RUNNER-001 §2.1
type EngineConfigFactory func(task Task) (query.QueryEngineConfig, error)

// AgentRunnerConfig holds configuration for the AgentRunner.
// SPEC-GOOSE-AGENT-RUNNER-001 §2.2
type AgentRunnerConfig struct {
	// NewEngineConfig creates QueryEngineConfig for each task execution.
	NewEngineConfig EngineConfigFactory
	// ReflectHooks are the post-task self-critique hooks (executed in order).
	ReflectHooks []ReflectHook
	// MaxReplans is the maximum number of re-plan iterations (default 2).
	MaxReplans int
	// Logger receives structured output.
	Logger *zap.Logger
}

// AgentRunner orchestrates the Plan-Run-Reflect cycle.
// SPEC-GOOSE-AGENT-RUNNER-001 §2.3
//
// @MX:ANCHOR: [AUTO] Outer orchestration layer for Plan-Run-Reflect cycle
// @MX:REASON: SPEC-GOOSE-AGENT-RUNNER-001 - Coordinates QueryEngine + ReflectHooks with re-plan loop
type AgentRunner struct {
	cfg AgentRunnerConfig
}

// NewAgentRunner creates a new AgentRunner.
// SPEC-GOOSE-AGENT-RUNNER-001 §2.3.1
// MaxReplans defaults to 2 if <= 0.
func NewAgentRunner(cfg AgentRunnerConfig) (*AgentRunner, error) {
	if cfg.NewEngineConfig == nil {
		return nil, fmt.Errorf("NewEngineConfig factory is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.MaxReplans <= 0 {
		cfg.MaxReplans = DefaultMaxPlans
	}
	return &AgentRunner{cfg: cfg}, nil
}

// RunTask executes a single task through the Plan-Run-Reflect cycle.
// SPEC-GOOSE-AGENT-RUNNER-001 §2.3.2
//
// @MX:NOTE: [AUTO] Re-plan loop with reflection-driven iteration
// @MX:REASON: SPEC-GOOSE-AGENT-RUNNER-001 - Score < 0.7 triggers re-plan with critique feedback
func (r *AgentRunner) RunTask(ctx context.Context, task *Task) (*TaskResult, error) {
	result := &TaskResult{TaskID: task.ID}

	for attempt := 0; attempt <= r.cfg.MaxReplans; attempt++ {
		// Check context before starting
		if ctx.Err() != nil {
			task.State = TaskCancelled
			result.Error = ctx.Err()
			return result, nil
		}

		// Create engine config via factory
		engineCfg, err := r.cfg.NewEngineConfig(*task)
		if err != nil {
			task.State = TaskFailed
			result.Error = fmt.Errorf("engine config creation failed: %w", err)
			return result, nil
		}

		// Create QueryEngine
		engine, err := query.New(engineCfg)
		if err != nil {
			task.State = TaskFailed
			result.Error = fmt.Errorf("engine creation failed: %w", err)
			return result, nil
		}

		// Run the inner loop
		task.State = TaskRunning
		msgCh, err := engine.SubmitMessage(ctx, task.Prompt)
		if err != nil {
			task.State = TaskFailed
			result.Error = fmt.Errorf("submit message failed: %w", err)
			return result, nil
		}

		// Drain the message channel (streaming to caller would happen here in production)
		for range msgCh {
			// In production, these would be forwarded to the caller/streamed
		}

		task.State = TaskCompleted

		// Capture final state for reflection
		finalState := engine.State()

		// Run reflect hooks
		if len(r.cfg.ReflectHooks) > 0 {
			reflectResult, reflectErr := r.runReflectHooks(ctx, *task, finalState)
			if reflectErr != nil {
				r.cfg.Logger.Warn("reflect hook error", zap.Error(reflectErr))
				// Continue without reflection on error
			}
			if reflectResult != nil {
				result.Reflect = reflectResult

				// Check if re-plan needed
				if reflectResult.Score >= MinPassScore {
					task.State = TaskReflected
					result.ReplanCount = attempt
					result.FinalState = finalState
					return result, nil
				}

				// Score < MinPassScore — prepare re-plan if attempts remaining
				if attempt < r.cfg.MaxReplans {
					r.cfg.Logger.Info("re-plan triggered",
						zap.Float64("score", reflectResult.Score),
						zap.Int("attempt", attempt+1),
					)
					// Append critique feedback to prompt for re-plan
					task.Prompt = fmt.Sprintf("%s\n\n[Self-Critique Feedback - score %.2f]\nGaps: %s\nInconsistencies: %s\nUnsupported claims: %s\nPlease address these issues.",
						task.Prompt,
						reflectResult.Score,
						reflectResult.Gap,
						reflectResult.Inconsistency,
						reflectResult.Unsupported,
					)
					task.State = TaskPending
					continue
				}
			}
		} else {
			// No reflect hooks — just complete
			task.State = TaskReflected
			result.ReplanCount = attempt
			result.FinalState = finalState
			return result, nil
		}

		// Max replans exhausted
		result.ReplanCount = attempt
		task.State = TaskReflected
		result.FinalState = finalState
		return result, nil
	}

	return result, nil
}

// runReflectHooks executes all registered reflect hooks and returns the minimum score.
// SPEC-GOOSE-AGENT-RUNNER-001 §2.3.3
//
// @MX:NOTE: [AUTO] Multi-hook composition - minimum score wins
// @MX:REASON: SPEC-GOOSE-AGENT-RUNNER-001 - Multiple hooks can be registered; strictest hook drives re-plan decision
func (r *AgentRunner) runReflectHooks(ctx context.Context, task Task, state loop.State) (*ReflectResult, error) {
	var minResult *ReflectResult

	// Run hooks in sequence (could be parallelized in the future)
	for _, hook := range r.cfg.ReflectHooks {
		result, err := hook(ctx, task, state)
		if err != nil {
			r.cfg.Logger.Warn("reflect hook failed", zap.Error(err))
			continue
		}
		if result != nil {
			if minResult == nil || result.Score < minResult.Score {
				minResult = result
			}
		}
	}

	return minResult, nil
}
