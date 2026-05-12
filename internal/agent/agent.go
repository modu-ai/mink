// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/message"
)

// Agent represents an AI agent with persona and conversation management.
// @MX:ANCHOR: [AUTO] Agent interface - core runtime contract
// @MX:REASON: Primary interface for agent lifecycle (Create-Load-Ask-Close)
type Agent interface {
	// Name returns the agent's name.
	Name() string

	// Spec returns the agent's immutable spec.
	Spec() *AgentSpec

	// Ask performs a single-turn synchronous conversation.
	// Implements REQ-AG-004: Complete Ask workflow.
	Ask(ctx context.Context, userMsg string) (string, error)

	// AskStream performs a single-turn streaming conversation.
	AskStream(ctx context.Context, userMsg string) (<-chan message.StreamEvent, error)

	// History returns a copy of the conversation history.
	History() []Message

	// Close releases resources.
	Close() error
}

// defaultAgent is the standard Agent implementation.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Core agent runtime implementation
type defaultAgent struct {
	spec         *AgentSpec
	provider     provider.Provider
	observer     InteractionObserver
	toolInvoker  ToolInvoker
	conversation *Conversation
	capabilities provider.Capabilities
	logger       *slog.Logger
}

// AgentOption is a functional option for configuring Agent creation.
type AgentOption func(*defaultAgent)

// WithLogger sets a custom logger for the agent.
// If not provided, defaults to slog.Default().
func WithLogger(logger *slog.Logger) AgentOption {
	return func(a *defaultAgent) {
		a.logger = logger
	}
}

// NewAgent creates a new agent from a spec.
// @MX:ANCHOR: [AUTO] Agent factory function
// @MX:REASON: Entry point for agent instantiation, validates spec
func NewAgent(spec *AgentSpec, llmProvider provider.Provider, observer InteractionObserver, opts ...AgentOption) (Agent, error) {
	// Validate spec (REQ-AG-009)
	if err := validateSpec(spec); err != nil {
		return nil, err
	}

	agent := &defaultAgent{
		spec:         spec,
		provider:     llmProvider,
		observer:     observer,
		toolInvoker:  &NoopToolInvoker{},
		conversation: NewConversation(),
		logger:       slog.Default(), // Default to slog.Default()
	}

	// Apply functional options
	for _, opt := range opts {
		opt(agent)
	}

	// Fetch capabilities on first creation (REQ-AG-008)
	agent.capabilities = llmProvider.Capabilities()

	return agent, nil
}

// Name returns the agent's name.
func (a *defaultAgent) Name() string {
	return a.spec.Name
}

// Spec returns the agent's immutable spec (REQ-AG-001).
func (a *defaultAgent) Spec() *AgentSpec {
	return a.spec
}

// Ask performs a single-turn synchronous conversation.
// Implements REQ-AG-004: Complete Ask workflow with error handling and history rollback.
// @MX:ANCHOR: [AUTO] Single-turn conversation implementation
// @MX:REASON: Core interaction loop called by CLI, handles all LLM interaction logic
func (a *defaultAgent) Ask(ctx context.Context, userMsg string) (string, error) {
	// Append user message to history
	userMessage := Message{
		Role:    "user",
		Content: userMsg,
		Metadata: MessageMetadata{
			Timestamp: time.Now(),
		},
	}
	a.conversation.Append(userMessage)

	// Calculate reserved tokens for completion
	reservedTokens := a.capabilities.MaxOutputTokens
	if reservedTokens == 0 {
		reservedTokens = 512 // Default fallback
	}

	// Trim conversation if needed (REQ-AG-005)
	maxTokens := a.capabilities.MaxContextTokens - reservedTokens
	a.conversation.Trim(maxTokens)

	// Build message list for LLM (after trim)
	messages := a.buildMessages(userMsg)

	// Convert to LLM message format
	llmMessages := make([]message.Message, len(messages))
	for i, msg := range messages {
		llmMessages[i] = msg.ToLLMMessage()
	}

	// Parse provider and model from spec
	_, modelName := a.spec.ParseModel()

	// Create completion request
	req := provider.CompletionRequest{
		Messages: llmMessages,
		Metadata: provider.RequestMetadata{
			UserID: "user", // TODO: multi-user support
		},
	}

	// Call LLM provider
	resp, err := a.provider.Complete(ctx, req)
	if err != nil {
		// Rollback user message on error (REQ-AG-006)
		a.rollbackUserMessage()

		// Wrap error with agent context
		return "", &AgentError{
			AgentName: a.spec.Name,
			Cause:     err,
		}
	}

	// Extract response text
	responseText := resp.Message.Content[0].Text

	// Append assistant response to history
	assistantMessage := Message{
		Role:    "assistant",
		Content: responseText,
		Metadata: MessageMetadata{
			Timestamp:  time.Now(),
			TokenCount: resp.Usage.OutputTokens,
			ModelName:  modelName,
			ResponseID: resp.ResponseID,
		},
	}
	a.conversation.Append(assistantMessage)

	// Notify observer (REQ-AG-014)
	a.notifyObserver(Interaction{
		Timestamp:    time.Now(),
		AgentName:    a.spec.Name,
		UserMsg:      userMsg,
		AssistantMsg: responseText,
		Err:          nil,
		TokenUsage: TokenUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	})

	return responseText, nil
}

// AskStream performs a single-turn streaming conversation.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Streaming variant of Ask
func (a *defaultAgent) AskStream(ctx context.Context, userMsg string) (<-chan message.StreamEvent, error) {
	// TODO: Implement streaming with recordingReader
	return nil, fmt.Errorf("streaming not yet implemented")
}

// History returns a copy of the conversation history.
func (a *defaultAgent) History() []Message {
	return a.conversation.Messages()
}

// Close releases resources.
func (a *defaultAgent) Close() error {
	// No resources to release in Phase 0
	return nil
}

// buildMessages constructs the complete message list for LLM.
// Implements REQ-AG-003: Prepend system prompt (not in history).
func (a *defaultAgent) buildMessages(userMsg string) []Message {
	// Start with persona (system prompt + examples)
	messages := BuildPersona(a.spec)

	// Append conversation history
	history := a.conversation.Messages()
	messages = append(messages, history...)

	return messages
}

// rollbackUserMessage removes the last user message from history.
// Called when LLM call fails (REQ-AG-006).
func (a *defaultAgent) rollbackUserMessage() {
	// Remove the last message (user message we just added)
	history := a.conversation.Messages()
	if len(history) > 0 {
		a.conversation.Truncate(len(history) - 1)
	}
}

// notifyObserver calls the observer with panic recovery (REQ-AG-014, AC-AG-008).
func (a *defaultAgent) notifyObserver(ix Interaction) {
	if a.observer == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			a.logger.Warn("observer panic recovered",
				"panic", r,
				"agent", a.spec.Name,
			)
		}
	}()

	a.observer.OnInteraction(ix)
}
