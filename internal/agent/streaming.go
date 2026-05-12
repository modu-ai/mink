// Package agent — streaming extension for ChatService.
package agent

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/query"
)

// ChatChunk represents a single streaming output chunk from the agent.
//
// @MX:NOTE: [AUTO] ChatChunk emits Final=true exactly once when the stream closes.
type ChatChunk struct {
	// Content is the text fragment emitted in this chunk. May be empty for
	// non-text SDK messages (which are silently dropped).
	Content string
	// Final is true on the last chunk emitted for the request. Consumers MUST
	// perform final flush logic when Final=true.
	Final bool
}

// StreamingChatService is the streaming counterpart of ChatService. It returns
// a receive-only channel of ChatChunk that closes when the stream ends.
//
// @MX:ANCHOR: [AUTO] StreamingChatService is the streaming domain boundary for
// the telegram channel.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P4 REQ-MTGM-E02; fan_in via
// AgentStreamAdapter, streaming handler, and unit tests (>= 3 callers).
type StreamingChatService interface {
	// ChatStream opens a streaming query against the Mink agent and returns a
	// channel of ChatChunk values. The channel is closed when the stream ends.
	// The caller must drain the channel fully or cancel ctx to avoid goroutine leaks.
	ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// ChatStream implements StreamingChatService for QueryEngineChatService.
//
// It wraps query.SubmitMessage's <-chan message.SDKMessage and emits one
// ChatChunk per text-bearing SDKMsgMessage. The last chunk has Final=true.
// If the underlying stream produces no text, a single ChatChunk{Final:true}
// is emitted so that consumers always receive the terminating signal.
//
// @MX:WARN: [AUTO] ChatStream spawns a goroutine that reads from the engine's
// SDKMessage channel; the goroutine is bounded by ctx and the upstream channel.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P4 REQ-MTGM-E02; goroutine lifetime
// is bounded by context cancellation and channel close.
func (s *QueryEngineChatService) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	// Create a placeholder Task to satisfy the factory signature.
	task := Task{
		ID:     "telegram-chat-stream",
		Prompt: req.Text,
		State:  TaskPending,
	}

	engineCfg, err := s.factory(task)
	if err != nil {
		return nil, fmt.Errorf("agent.StreamingChatService: engine config: %w", err)
	}

	engine, err := query.New(engineCfg)
	if err != nil {
		return nil, fmt.Errorf("agent.StreamingChatService: engine: %w", err)
	}

	msgCh, err := engine.SubmitMessage(ctx, req.Text)
	if err != nil {
		return nil, fmt.Errorf("agent.StreamingChatService: submit: %w", err)
	}

	out := make(chan ChatChunk)
	go func() {
		defer close(out)
		var pending ChatChunk
		haveAny := false

		for sdkMsg := range msgCh {
			if sdkMsg.Type != message.SDKMsgMessage {
				continue
			}
			pm, ok := sdkMsg.Payload.(message.PayloadMessage)
			if !ok {
				continue
			}
			for _, block := range pm.Msg.Content {
				if block.Type != "text" || block.Text == "" {
					continue
				}
				// Emit the previously buffered chunk before replacing it.
				if haveAny {
					select {
					case out <- pending:
					case <-ctx.Done():
						return
					}
				}
				pending = ChatChunk{Content: block.Text, Final: false}
				haveAny = true
			}
		}

		// Emit the final chunk (or an empty terminator if nothing was produced).
		pending.Final = true
		select {
		case out <- pending:
		case <-ctx.Done():
		}
	}()

	return out, nil
}
