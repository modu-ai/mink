package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/goose/internal/query"
)

// TestStreamingChatService_Implements verifies that *QueryEngineChatService
// satisfies the StreamingChatService interface at compile time.
func TestStreamingChatService_Implements(t *testing.T) {
	var _ StreamingChatService = (*QueryEngineChatService)(nil)
}

// TestChatChunk_Fields verifies ChatChunk field semantics.
func TestChatChunk_Fields(t *testing.T) {
	c := ChatChunk{Content: "hello", Final: false}
	if c.Content != "hello" {
		t.Errorf("unexpected Content: %q", c.Content)
	}
	if c.Final {
		t.Error("expected Final=false")
	}

	last := ChatChunk{Content: "done", Final: true}
	if !last.Final {
		t.Error("expected Final=true on last chunk")
	}
}

// TestQueryEngineChatService_ChatStream_FactoryError verifies that a factory
// error is propagated before any streaming begins.
func TestQueryEngineChatService_ChatStream_FactoryError(t *testing.T) {
	factoryErr := errors.New("factory error")
	factory := func(task Task) (query.QueryEngineConfig, error) {
		return query.QueryEngineConfig{}, factoryErr
	}
	svc, err := NewQueryEngineChatService(QueryEngineChatServiceConfig{Factory: factory})
	if err != nil {
		t.Fatalf("construct error: %v", err)
	}

	ch, streamErr := svc.ChatStream(context.Background(), ChatRequest{Text: "test"})
	if streamErr == nil {
		t.Fatal("expected error from ChatStream on factory failure")
	}
	if !errors.Is(streamErr, factoryErr) {
		t.Errorf("expected wrapped factoryErr, got: %v", streamErr)
	}
	if ch != nil {
		t.Error("expected nil channel on error")
	}
}

// TestQueryEngineChatService_ChatStream_DefaultLogger verifies that nil logger
// does not panic during streaming setup.
func TestQueryEngineChatService_ChatStream_DefaultLogger(t *testing.T) {
	factory := func(task Task) (query.QueryEngineConfig, error) {
		return query.QueryEngineConfig{}, nil
	}
	svc, err := NewQueryEngineChatService(QueryEngineChatServiceConfig{
		Factory: factory,
		Logger:  nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// We only verify construction succeeds; ChatStream itself will fail because
	// QueryEngineConfig has no LLMCall, but that tests deeper integration.
	if svc == nil {
		t.Error("expected non-nil service")
	}
}
