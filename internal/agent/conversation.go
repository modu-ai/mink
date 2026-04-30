// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"sync"
)

// Conversation manages in-memory conversation history with thread-safe operations.
// @MX:ANCHOR: [AUTO] Thread-safe conversation history management
// @MX:REASON: Concurrent access from user goroutine (append) and observer goroutines (read)
type Conversation struct {
	mu       sync.RWMutex
	messages []Message
}

// NewConversation creates a new empty conversation.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Factory function for Conversation initialization
func NewConversation() *Conversation {
	return &Conversation{
		messages: make([]Message, 0),
	}
}

// Append adds a message to the conversation history.
// Thread-safe for concurrent appends from a single user goroutine.
// @MX:ANCHOR: [AUTO] Thread-safe message append
// @MX:REASON: Called from Ask (user goroutine) and read by observers
func (c *Conversation) Append(msg Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
}

// Messages returns a copy of the conversation history.
// Thread-safe for concurrent reads.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Returns copy to prevent external mutation
func (c *Conversation) Messages() []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external mutation
	result := make([]Message, len(c.messages))
	copy(result, c.messages)
	return result
}

// Trim removes oldest user/assistant pairs to fit within maxTokens.
// Implements REQ-AG-005: FIFO trim - oldest pairs removed first.
// @MX:ANCHOR: [AUTO] Context window trim implementation
// @MX:REASON: Called by every Ask invocation to manage context window
func (c *Conversation) Trim(maxTokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Calculate current tokens (approximate: len/4)
	currentTokens := c.calculateTokens()
	if currentTokens <= maxTokens {
		return // No trim needed
	}

	// Remove oldest pairs until we fit
	// Keep removing from the front (oldest) in pairs
	for len(c.messages) >= 2 && currentTokens > maxTokens {
		// Remove oldest pair (user + assistant)
		removed := c.removeOldestPair()
		currentTokens -= removed
	}
}

// Truncate removes messages from the end to achieve the desired length.
// Used for rolling back user messages on error.
func (c *Conversation) Truncate(length int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if length < 0 {
		length = 0
	}
	if length >= len(c.messages) {
		return
	}

	c.messages = c.messages[:length]
}

// TokenCount returns the approximate token count of all messages.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Approximate token counting (len/4)
func (c *Conversation) TokenCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.calculateTokens()
}

// calculateTokens estimates total tokens (rough approximation: chars/4).
// Phase 0 uses this approximation; later phases may use actual tokenizer.
func (c *Conversation) calculateTokens() int {
	total := 0
	for _, msg := range c.messages {
		total += len(msg.Content) / 4
		if total == 0 && len(msg.Content) > 0 {
			total = 1 // Minimum 1 token per message
		}
	}
	return total
}

// removeOldestPair removes the oldest user/assistant pair from the front.
// Returns the approximate token count of removed messages.
func (c *Conversation) removeOldestPair() int {
	if len(c.messages) < 2 {
		return 0
	}

	removedTokens := 0
	// Remove first two messages (should be user + assistant pair)
	for i := 0; i < 2 && len(c.messages) > 0; i++ {
		removedTokens += len(c.messages[0].Content) / 4
		c.messages = c.messages[1:]
	}

	return removedTokens
}
