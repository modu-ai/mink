// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import "fmt"

// ErrInvalidSpec is returned when AgentSpec validation fails.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Error type for spec validation failures
type ErrInvalidSpec struct {
	Field  string
	Reason string
}

func (e *ErrInvalidSpec) Error() string {
	return fmt.Sprintf("invalid spec: %s %s", e.Field, e.Reason)
}

// ensure ErrInvalidSpec implements error
var _ error = (*ErrInvalidSpec)(nil)

// AgentError wraps errors from LLM provider with agent context.
// @MX:ANCHOR: [AUTO] Error wrapping for all LLM provider errors
// @MX:REASON: All LLM calls return errors wrapped with agent name for debugging
type AgentError struct {
	AgentName string
	Cause     error
}

func (e *AgentError) Error() string {
	return fmt.Sprintf("agent %s: %v", e.AgentName, e.Cause)
}

func (e *AgentError) Unwrap() error {
	return e.Cause
}

// ErrAgentNotReady is returned when Ask is called before agent is ready.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] State machine error for premature calls
type ErrAgentNotReady struct {
	AgentName string
	State     string
}

func (e *ErrAgentNotReady) Error() string {
	return fmt.Sprintf("agent %s not ready (state: %s)", e.AgentName, e.State)
}

// ensure ErrAgentNotReady implements error
var _ error = (*ErrAgentNotReady)(nil)
