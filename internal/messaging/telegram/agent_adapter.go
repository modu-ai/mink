package telegram

import (
	"context"
	"fmt"

	"github.com/modu-ai/goose/internal/agent"
)

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
