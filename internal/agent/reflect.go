// Package agent provides the outer orchestration layer for the Plan-Run-Reflect cycle.
// SPEC-GOOSE-AGENT-RUNNER-001, SPEC-GOOSE-SELF-CRITIQUE-001
package agent

import (
	"context"

	"github.com/modu-ai/goose/internal/query/loop"
)

// ReflectResult contains the self-critique output.
// SPEC-GOOSE-SELF-CRITIQUE-001
type ReflectResult struct {
	// Score is the overall quality score (0.0 to 1.0).
	Score float64
	// Gap describes what's missing from the task output.
	Gap string
	// Inconsistency describes any contradictions found.
	Inconsistency string
	// Unsupported describes claims without evidence.
	Unsupported string
	// RawOutput is the raw LLM response.
	RawOutput string
}

// ReflectHook is a function that performs post-task self-critique.
// SPEC-GOOSE-SELF-CRITIQUE-001
//
// @MX:ANCHOR: [AUTO] Hook signature for self-critique extension points
// @MX:REASON: SPEC-GOOSE-SELF-CRITIQUE-001 - Multiple hooks can be registered (LLM-based, heuristic)
type ReflectHook func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error)
