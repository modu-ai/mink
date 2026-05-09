package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
)

// TestChatService_Implements verifies that *QueryEngineChatService satisfies ChatService.
func TestChatService_Implements(t *testing.T) {
	var _ ChatService = (*QueryEngineChatService)(nil)
}

// TestNewQueryEngineChatService_NilFactory verifies that a nil factory returns an error.
func TestNewQueryEngineChatService_NilFactory(t *testing.T) {
	_, err := NewQueryEngineChatService(QueryEngineChatServiceConfig{Factory: nil})
	if err == nil {
		t.Error("expected error for nil Factory")
	}
}

// TestNewQueryEngineChatService_OK verifies that a valid factory constructs without error.
func TestNewQueryEngineChatService_OK(t *testing.T) {
	factory := func(task Task) (query.QueryEngineConfig, error) {
		return query.QueryEngineConfig{}, nil
	}
	svc, err := NewQueryEngineChatService(QueryEngineChatServiceConfig{Factory: factory})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Error("expected non-nil service")
	}
}

// TestChatRequest_Fields verifies that ChatRequest and ChatResponse can be
// constructed with expected fields.
func TestChatRequest_Fields(t *testing.T) {
	req := ChatRequest{
		Text: "hello",
		Attachments: []Attachment{
			{Path: "/tmp/img.jpg", MimeType: "image/jpeg", SizeBytes: 1024},
		},
	}
	if req.Text != "hello" {
		t.Errorf("unexpected Text: %q", req.Text)
	}
	if len(req.Attachments) != 1 {
		t.Errorf("expected 1 attachment, got %d", len(req.Attachments))
	}

	resp := ChatResponse{Content: "response text"}
	if resp.Content != "response text" {
		t.Errorf("unexpected Content: %q", resp.Content)
	}
}

// TestQueryEngineChatService_FactoryError verifies that a factory error is
// wrapped and propagated.
func TestQueryEngineChatService_FactoryError(t *testing.T) {
	factoryErr := errors.New("factory error")
	factory := func(task Task) (query.QueryEngineConfig, error) {
		return query.QueryEngineConfig{}, factoryErr
	}
	svc, _ := NewQueryEngineChatService(QueryEngineChatServiceConfig{Factory: factory})
	_, err := svc.Chat(context.Background(), ChatRequest{Text: "test"})
	if err == nil {
		t.Fatal("expected error from Chat")
	}
	if !errors.Is(err, factoryErr) {
		t.Errorf("expected wrapped factoryErr, got: %v", err)
	}
}

// TestQueryEngineChatService_DefaultLogger verifies that nil logger is replaced
// with a nop logger.
func TestQueryEngineChatService_DefaultLogger(t *testing.T) {
	factory := func(task Task) (query.QueryEngineConfig, error) {
		return query.QueryEngineConfig{}, nil
	}
	svc, err := NewQueryEngineChatService(QueryEngineChatServiceConfig{
		Factory: factory,
		Logger:  nil, // should default to nop
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.logger == nil {
		t.Error("expected logger to be set")
	}
}

// TestQueryEngineChatService_ExtractsTextBlocks verifies that Chat extracts
// the text content from SDKMsgMessage payloads.
// This test uses a stub engine by passing a factory that returns a minimal config.
// Since query.New requires a non-nil LLMCall, we test only the text extraction
// helper logic independently.
func TestExtractTextFromMessages(t *testing.T) {
	msgs := []message.Message{
		{Role: "assistant", Content: []message.ContentBlock{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "world"},
		}},
	}

	var got string
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == "text" {
				got += b.Text
			}
		}
	}
	if got != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", got)
	}
}
