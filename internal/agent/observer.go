// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import "time"

// Interaction records a single agent interaction for observability.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Interaction record for observer hook
type Interaction struct {
	// Timestamp is when the interaction occurred
	Timestamp time.Time `json:"timestamp"`
	// AgentName is the name of the agent
	AgentName string `json:"agent_name"`
	// UserMsg is the user's input message
	UserMsg string `json:"user_msg"`
	// AssistantMsg is the assistant's response
	AssistantMsg string `json:"assistant_msg"`
	// Err is any error that occurred (nil on success)
	Err error `json:"err,omitempty"`
	// TokenUsage is the token consumption statistics
	TokenUsage TokenUsage `json:"token_usage"`
}

// TokenUsage records token statistics for an interaction.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Token usage tracking
type TokenUsage struct {
	// InputTokens is the number of input tokens consumed
	InputTokens int `json:"input_tokens"`
	// OutputTokens is the number of output tokens generated
	OutputTokens int `json:"output_tokens"`
	// TotalTokens is the sum of input and output tokens
	TotalTokens int `json:"total_tokens"`
}

// InteractionObserver is called after each agent interaction.
// Implements REQ-AG-014: Observer hook for telemetry and learning.
// @MX:ANCHOR: [AUTO] Observer interface for interaction telemetry
// @MX:REASON: Called after every LLM interaction; implemented by TELEM-001, FEEDBACK-001
type InteractionObserver interface {
	// OnInteraction is called after each LLM interaction completes.
	// Called for both success and failure paths.
	// Observer errors MUST NOT fail the Ask() operation.
	OnInteraction(Interaction)
}

// NoopObserver is a no-op implementation of InteractionObserver.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Default observer implementation
type NoopObserver struct{}

// OnInteraction does nothing.
func (o *NoopObserver) OnInteraction(ix Interaction) {
	// No-op
}
