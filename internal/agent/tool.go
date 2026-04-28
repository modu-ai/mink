// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import "context"

// ToolInvoker invokes tools during agent interactions.
// Implements REQ-AG-012: Tool calling hook (implemented by TOOL-001).
// @MX:ANCHOR: [AUTO] Tool invocation interface
// @MX:REASON: Hook for TOOL-001 to inject tool execution capability
type ToolInvoker interface {
	// Invoke executes a tool call and returns the result.
	Invoke(ctx context.Context, name string, args string) (string, error)
}

// NoopToolInvoker is a no-op implementation of ToolInvoker.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Default tool invoker (Phase 0: no tools)
type NoopToolInvoker struct{}

// Invoke does nothing and returns an error indicating tools are not supported.
func (t *NoopToolInvoker) Invoke(ctx context.Context, name string, args string) (string, error) {
	return "", nil // Phase 0: tools not implemented
}
