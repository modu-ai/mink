// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessage_Roles(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{"System role", "system"},
		{"User role", "user"},
		{"Assistant role", "assistant"},
		{"Tool role", "tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{
				Role:    tt.role,
				Content: "test content",
			}
			assert.Equal(t, tt.role, msg.Role)
		})
	}
}

func TestMessage_Metadata(t *testing.T) {
	now := time.Now()
	msg := Message{
		Role:    "user",
		Content: "test",
		Metadata: MessageMetadata{
			Timestamp:  now,
			TokenCount: 10,
			ModelName:  "test-model",
			ResponseID: "resp-123",
		},
	}

	assert.Equal(t, now, msg.Metadata.Timestamp)
	assert.Equal(t, 10, msg.Metadata.TokenCount)
	assert.Equal(t, "test-model", msg.Metadata.ModelName)
	assert.Equal(t, "resp-123", msg.Metadata.ResponseID)
}

func TestMessage_ToLLMMessage(t *testing.T) {
	// Test conversion from agent Message to LLM message.Message
	msg := Message{
		Role:    "user",
		Content: "hello",
	}

	llmMsg := msg.ToLLMMessage()
	assert.Equal(t, "user", llmMsg.Role)
	assert.Equal(t, "hello", llmMsg.Content[0].Text)
}
