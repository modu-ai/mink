// Package agent provides the outer orchestration layer for GOOSE agent operations.
// This file defines the ChatService domain interface used by the Telegram messaging
// channel (strategy-p3.md §A.2 option b).
package agent

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/query"
	"go.uber.org/zap"
)

// Attachment carries a local file path and optional metadata for an inbound
// file that should be forwarded to the LLM (strategy-p3.md §C.5).
// In P3, the ChatService implementation ignores attachments and forwards text only.
// Multimodal forwarding is deferred to a future SPEC.
type Attachment struct {
	// Path is the local filesystem path (e.g. ~/.goose/messaging/telegram/inbox/42.jpg).
	Path string
	// MimeType is an optional MIME hint (e.g. "image/jpeg").
	MimeType string
	// SizeBytes is the file size in bytes.
	SizeBytes int64
}

// ChatRequest represents a single-turn chat request forwarded from a messaging
// channel to the Goose agent.
type ChatRequest struct {
	// Text is the user message body.
	Text string
	// Attachments holds optional inbound files (P3: text only; multimodal P4+).
	Attachments []Attachment
}

// ChatResponse carries the agent's reply.
type ChatResponse struct {
	// Content is the full response text.
	Content string
}

// ChatService is the domain interface for single-turn chat with the Goose agent.
// It is consumed by the telegram messaging channel (via AgentAdapter) and
// may be exposed over gRPC in a future phase.
//
// @MX:ANCHOR: [AUTO] ChatService is the internal domain boundary for agent chat.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 §A; fan_in via AgentAdapter, gRPC server
// (future), and unit tests (>= 3 callers).
type ChatService interface {
	// Chat executes a single-turn query against the Goose agent and returns
	// the full response text.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// QueryEngineChatService implements ChatService by creating a fresh QueryEngine
// for each call. This is a stateless wrapper suitable for single-turn channels
// such as Telegram polling.
//
// @MX:NOTE: [AUTO] QueryEngineChatService creates a new QueryEngine per call,
// meaning each Telegram message starts a fresh conversation context with no memory
// of prior turns. Persistent session support is a future SPEC item.
type QueryEngineChatService struct {
	factory EngineConfigFactory
	logger  *zap.Logger
}

// QueryEngineChatServiceConfig holds construction parameters.
type QueryEngineChatServiceConfig struct {
	// Factory produces a QueryEngineConfig from a prompt.
	Factory EngineConfigFactory
	// Logger is used for structured output.
	Logger *zap.Logger
}

// NewQueryEngineChatService constructs a ChatService backed by QueryEngine.
//
// @MX:ANCHOR: [AUTO] NewQueryEngineChatService is the constructor for the production ChatService.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001; fan_in via cmd/goosed/main.go, telegram adapter, and tests (>= 3 callers).
func NewQueryEngineChatService(cfg QueryEngineChatServiceConfig) (*QueryEngineChatService, error) {
	if cfg.Factory == nil {
		return nil, fmt.Errorf("agent.NewQueryEngineChatService: Factory is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	return &QueryEngineChatService{
		factory: cfg.Factory,
		logger:  cfg.Logger,
	}, nil
}

// Chat runs a single-turn query and collects the response content.
// Attachments are accepted but silently ignored in P3; multimodal support
// is deferred to a future SPEC.
func (s *QueryEngineChatService) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	// Create a placeholder Task to satisfy the factory signature.
	task := Task{
		ID:     "telegram-chat",
		Prompt: req.Text,
		State:  TaskPending,
	}

	engineCfg, err := s.factory(task)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("agent.ChatService: engine config: %w", err)
	}

	engine, err := query.New(engineCfg)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("agent.ChatService: engine: %w", err)
	}

	msgCh, err := engine.SubmitMessage(ctx, req.Text)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("agent.ChatService: submit: %w", err)
	}

	// Collect the final text response from the SDKMessage stream.
	var content string
	for sdkMsg := range msgCh {
		if sdkMsg.Type == message.SDKMsgMessage {
			if pm, ok := sdkMsg.Payload.(message.PayloadMessage); ok {
				for _, block := range pm.Msg.Content {
					if block.Type == "text" {
						content += block.Text
					}
				}
			}
		}
	}

	return ChatResponse{Content: content}, nil
}
