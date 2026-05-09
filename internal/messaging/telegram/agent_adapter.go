package telegram

import (
	"context"
	"fmt"

	"github.com/modu-ai/goose/internal/agent"
)

// StreamChunk is the telegram-package counterpart of agent.ChatChunk. The
// duplication keeps the package boundary clean (no agent import in the streaming
// hot path beyond this adapter).
type StreamChunk struct {
	// Content is the text fragment emitted in this chunk.
	Content string
	// Final is true on the last chunk. Consumers MUST drain the channel until
	// Final=true is received or the channel is closed.
	Final bool
}

// AgentStream is the streaming counterpart of AgentQuery. It returns a
// receive-only channel of StreamChunk values that closes when the stream ends.
//
// @MX:ANCHOR: [AUTO] AgentStream decouples the streaming branch from the
// agent.StreamingChatService transport.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P4 REQ-MTGM-E02; fan_in via
// BridgeQueryHandler streaming branch, bootstrap wiring, and unit tests (>= 3 callers).
type AgentStream interface {
	// QueryStream sends text (and optional attachment paths) to the agent and
	// returns a channel of StreamChunk values. The channel closes when the
	// stream ends. Cancelling ctx stops the stream.
	QueryStream(ctx context.Context, text string, attachments []string) (<-chan StreamChunk, error)
}

// AgentStreamAdapter adapts agent.StreamingChatService to AgentStream.
//
// @MX:NOTE: [AUTO] AgentStreamAdapter is the narrow bridge between the telegram
// streaming branch and the in-process StreamingChatService. It mirrors AgentAdapter
// for the streaming path.
type AgentStreamAdapter struct {
	svc agent.StreamingChatService
}

// NewAgentStreamAdapter constructs an AgentStreamAdapter wrapping the given StreamingChatService.
func NewAgentStreamAdapter(svc agent.StreamingChatService) *AgentStreamAdapter {
	if svc == nil {
		panic("telegram.NewAgentStreamAdapter: StreamingChatService must not be nil")
	}
	return &AgentStreamAdapter{svc: svc}
}

// QueryStream implements AgentStream by forwarding to StreamingChatService.ChatStream
// and re-emitting each agent.ChatChunk as a StreamChunk.
//
// @MX:WARN: [AUTO] QueryStream spawns a forwarding goroutine bounded by ctx and
// the upstream channel close.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P4 REQ-MTGM-E02; goroutine is necessary
// to bridge channel types without blocking the caller.
func (a *AgentStreamAdapter) QueryStream(ctx context.Context, text string, attachments []string) (<-chan StreamChunk, error) {
	atts := make([]agent.Attachment, 0, len(attachments))
	for _, p := range attachments {
		atts = append(atts, agent.Attachment{Path: p})
	}

	src, err := a.svc.ChatStream(ctx, agent.ChatRequest{Text: text, Attachments: atts})
	if err != nil {
		return nil, err
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		for c := range src {
			select {
			case out <- StreamChunk{Content: c.Content, Final: c.Final}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// AgentAdapter adapts agent.ChatService to the AgentQuery interface required by
// BridgeQueryHandler (strategy-p3.md §A.2 option b).
//
// The adapter converts the (text, attachments []string) signature of AgentQuery.Query
// into a ChatRequest for the domain service. Attachment paths are forwarded as
// agent.Attachment values; the P3 ChatService ignores them (multimodal is P4+).
//
// @MX:NOTE: [AUTO] AgentAdapter is the narrow bridge between the telegram channel
// and the in-process ChatService. It avoids a direct import of the gRPC proto package
// in the telegram layer.
type AgentAdapter struct {
	svc agent.ChatService
}

// NewAgentAdapter constructs an AgentAdapter wrapping the given ChatService.
func NewAgentAdapter(svc agent.ChatService) *AgentAdapter {
	if svc == nil {
		panic("telegram.NewAgentAdapter: ChatService must not be nil")
	}
	return &AgentAdapter{svc: svc}
}

// Query implements AgentQuery by forwarding to ChatService.Chat.
// attachments paths are converted to agent.Attachment values.
func (a *AgentAdapter) Query(ctx context.Context, text string, attachments []string) (string, error) {
	agentAttachments := make([]agent.Attachment, 0, len(attachments))
	for _, p := range attachments {
		agentAttachments = append(agentAttachments, agent.Attachment{Path: p})
	}

	resp, err := a.svc.Chat(ctx, agent.ChatRequest{
		Text:        text,
		Attachments: agentAttachments,
	})
	if err != nil {
		return "", fmt.Errorf("telegram agent adapter: %w", err)
	}
	return resp.Content, nil
}
