// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"time"

	"github.com/modu-ai/mink/internal/message"
)

// Message represents a single message in the agent's conversation history.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Agent-level message with metadata (vs LLM-level message.Message)
type Message struct {
	// Role is the message role: system, user, assistant, tool
	Role string `json:"role"`
	// Content is the text content of the message
	Content string `json:"content"`
	// Metadata contains additional information about the message
	Metadata MessageMetadata `json:"metadata,omitempty"`
}

// MessageMetadata holds metadata for agent-level messages.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Metadata for observability and debugging
type MessageMetadata struct {
	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`
	// TokenCount is the approximate token count
	TokenCount int `json:"token_count"`
	// ModelName is the model that generated this message (for assistant messages)
	ModelName string `json:"model_name,omitempty"`
	// ResponseID is the unique response identifier from LLM provider
	ResponseID string `json:"response_id,omitempty"`
}

// ToLLMMessage converts an agent Message to LLM provider message.Message.
// @MX:ANCHOR: [AUTO] Conversion between agent and LLM message formats
// @MX:REASON: Multiple paths convert messages (Ask, AskStream, buildMessages)
func (m *Message) ToLLMMessage() message.Message {
	return message.Message{
		Role: m.Role,
		Content: []message.ContentBlock{
			{Type: "text", Text: m.Content},
		},
	}
}
