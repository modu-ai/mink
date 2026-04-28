package llm

import (
	"context"
)

// LLMProvider is the unified interface for all LLM providers.
// SPEC-GOOSE-LLM-001 REQ-LLM-001: All methods accept context.Context and honor cancellation.
//
// @MX:ANCHOR: [AUTO] LLMProvider — unified interface for all LLM providers
// @MX:REASON: This interface will be implemented by Ollama and future providers (Anthropic, OpenAI, etc.)
type LLMProvider interface {
	// Name returns the provider name (e.g., "ollama", "anthropic").
	Name() string

	// Complete performs a blocking LLM completion request.
	// REQ-LLM-002: Response must include Usage with token counts.
	// REQ-LLM-003: All errors must be typed LLMError subclasses.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// Stream performs a streaming LLM completion request.
	// REQ-LLM-004: Returns receive-only channel that emits chunks.
	// Channel closes when stream ends normally or with error.
	Stream(ctx context.Context, req CompletionRequest) (<-chan Chunk, error)

	// CountTokens estimates token count for the given text.
	// Returns 0 and Usage.Unknown=true if provider does not support tokenization.
	CountTokens(ctx context.Context, text string) (int, error)

	// Capabilities returns model capabilities (max context, tool support, etc.).
	// REQ-LLM-009: First call fetches from provider API, cached thereafter.
	Capabilities(ctx context.Context, model string) (Capabilities, error)
}

// CompletionRequest represents a unified LLM completion request.
// SPEC-GOOSE-LLM-001 §6.2
type CompletionRequest struct {
	// Model is the model identifier (e.g., "qwen2.5:3b", "claude-3-5-sonnet-20241022").
	Model string

	// Messages is the conversation history.
	Messages []Message

	// Temperature is the sampling temperature (optional).
	// REQ-LLM-015: Unset leaves provider default (no implicit 0.0 or 1.0).
	Temperature *float64

	// MaxTokens is the maximum tokens to generate (optional).
	MaxTokens *int

	// Stop sequences where the model should stop generating (optional).
	// REQ-LLM-014: Provider-specific mapping (Ollama uses options.stop).
	Stop []string
}

// Validate checks if the request is valid.
// Returns error if Model is empty or Messages is empty.
func (r CompletionRequest) Validate() error {
	if r.Model == "" {
		return &ErrInvalidRequest{BaseLLMError: &BaseLLMError{Msg: "model is required", TypeName: "invalid_request"}}
	}
	if len(r.Messages) == 0 {
		return &ErrInvalidRequest{BaseLLMError: &BaseLLMError{Msg: "messages cannot be empty", TypeName: "invalid_request"}}
	}
	return nil
}

// Message represents a single message in the conversation.
type Message struct {
	// Role is the message role: "system", "user", "assistant", "tool".
	Role string

	// Content is the message text content.
	Content string
}

// CompletionResponse represents a unified LLM completion response.
// SPEC-GOOSE-LLM-001 §6.2
type CompletionResponse struct {
	// Text is the generated text content.
	Text string

	// Usage contains token usage statistics.
	// REQ-LLM-002: Must be populated from provider response.
	Usage Usage

	// Model is the model that generated the response.
	Model string

	// Raw is the provider-specific raw response (debug only).
	Raw any
}

// Chunk represents a streaming response chunk.
// SPEC-GOOSE-LLM-001 §6.2
type Chunk struct {
	// Delta is the text fragment in this chunk.
	Delta string

	// Done indicates if this is the final chunk.
	Done bool

	// Usage is populated only in the final chunk (optional).
	Usage *Usage
}

// Usage represents token usage statistics.
// SPEC-GOOSE-LLM-001 REQ-LLM-002
type Usage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int

	// CompletionTokens is the number of tokens generated.
	CompletionTokens int

	// TotalTokens is the total token count (PromptTokens + CompletionTokens).
	TotalTokens int

	// Unknown is true if the provider does not expose token counts.
	// REQ-LLM-002: If true, all counts should be zero.
	Unknown bool
}

// Capabilities represents model capabilities.
// SPEC-GOOSE-LLM-001 §6.2
type Capabilities struct {
	// MaxContextTokens is the maximum context window size.
	MaxContextTokens int

	// SupportsTools indicates function/tool calling support.
	SupportsTools bool

	// SupportsJSON indicates JSON mode support.
	SupportsJSON bool

	// Family is the model family (e.g., "llama", "qwen", "gpt").
	Family string
}
