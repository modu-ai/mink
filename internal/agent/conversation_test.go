// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConversation_Append(t *testing.T) {
	// REQ-AG-002: Thread-safe append
 conv := NewConversation()

	// Test basic append
	msg := Message{Role: "user", Content: "hello"}
	conv.Append(msg)

	messages := conv.Messages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "hello", messages[0].Content)
}

func TestConversation_ConcurrentAppend(t *testing.T) {
	// REQ-AG-002: Concurrent append safety
	conv := NewConversation()
	var wg sync.WaitGroup

	numGoroutines := 10
	messagesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := Message{
					Role:    "user",
					Content: "test",
				}
				conv.Append(msg)
			}
		}(i)
	}

	wg.Wait()

	messages := conv.Messages()
	expectedCount := numGoroutines * messagesPerGoroutine
	assert.Len(t, messages, expectedCount)
}

func TestConversation_Trim(t *testing.T) {
	// REQ-AG-005: FIFO trim - oldest pairs removed first
	conv := NewConversation()

	// Add 5 user/assistant pairs (10 messages)
	for i := 0; i < 5; i++ {
		conv.Append(Message{Role: "user", Content: "user message"})
		conv.Append(Message{Role: "assistant", Content: "assistant message"})
	}

	assert.Len(t, conv.Messages(), 10)

	// Trim to fit in 8 tokens (approximately 4 messages at 2 tokens each)
	// Should remove oldest 2 messages (1 user + 1 assistant pair)
	conv.Trim(8)

	messages := conv.Messages()
	assert.LessOrEqual(t, len(messages), 4)
}

func TestConversation_TrimPreservesMostRecent(t *testing.T) {
	// REQ-AG-005: Most recent exchange remains
	conv := NewConversation()

	// Add 3 pairs
	conv.Append(Message{Role: "user", Content: "old1"})
	conv.Append(Message{Role: "assistant", Content: "old2"})
	conv.Append(Message{Role: "user", Content: "recent1"})
	conv.Append(Message{Role: "assistant", Content: "recent2"})

	// Trim to fit only 1 pair
	conv.Trim(2)

	messages := conv.Messages()
	assert.Len(t, messages, 2)
	assert.Equal(t, "recent1", messages[0].Content)
	assert.Equal(t, "recent2", messages[1].Content)
}

func TestConversation_TokenCount(t *testing.T) {
	// Test token counting (approximate: len/4)
	conv := NewConversation()

	conv.Append(Message{Role: "user", Content: "hello world"}) // ~11 chars / 4 = 3 tokens
	conv.Append(Message{Role: "assistant", Content: "hi!"})      // ~3 chars / 4 = 1 token

	count := conv.TokenCount()
	assert.Greater(t, count, 0)
	assert.LessOrEqual(t, count, 10) // Should be approximately 4-5 tokens
}
