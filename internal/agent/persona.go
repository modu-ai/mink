// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

// BuildPersona constructs the initial messages list with system prompt and examples.
// Implements REQ-AG-003: Prepend system prompt on every LLM call.
// Implements REQ-AG-013: Append few-shot examples if present.
// @MX:ANCHOR: [AUTO] Persona message builder
// @MX:REASON: Called by every Ask/AskStream to build LLM message list
func BuildPersona(spec *AgentSpec) []Message {
	messages := make([]Message, 0, 1+len(spec.Examples)*2)

	// System prompt is always first (REQ-AG-003)
	// Note: Not stored in conversation history to avoid duplication
	messages = append(messages, Message{
		Role:    "system",
		Content: spec.SystemPrompt,
	})

	// Add few-shot examples if present (REQ-AG-013)
	for _, ex := range spec.Examples {
		messages = append(messages,
			Message{Role: "user", Content: ex.User},
			Message{Role: "assistant", Content: ex.Assistant},
		)
	}

	return messages
}
