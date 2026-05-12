package telegram

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/agent"
)

// stubChatService is a minimal agent.ChatService for adapter tests.
type stubChatService struct {
	response string
	err      error
	calls    []agent.ChatRequest
}

func (s *stubChatService) Chat(ctx context.Context, req agent.ChatRequest) (agent.ChatResponse, error) {
	s.calls = append(s.calls, req)
	return agent.ChatResponse{Content: s.response}, s.err
}

// TestAgentAdapter_Query verifies that AgentAdapter.Query calls ChatService.Chat
// and returns the content string.
func TestAgentAdapter_Query(t *testing.T) {
	svc := &stubChatService{response: "hello from agent"}
	a := NewAgentAdapter(svc)

	result, err := a.Query(context.Background(), "what is AI?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello from agent" {
		t.Errorf("expected 'hello from agent', got %q", result)
	}
	if len(svc.calls) != 1 {
		t.Errorf("expected 1 Chat call, got %d", len(svc.calls))
	}
	if svc.calls[0].Text != "what is AI?" {
		t.Errorf("expected text 'what is AI?', got %q", svc.calls[0].Text)
	}
}

// TestAgentAdapter_Attachments verifies that attachment paths are forwarded
// as agent.Attachment values.
func TestAgentAdapter_Attachments(t *testing.T) {
	svc := &stubChatService{response: "processed with attachments"}
	a := NewAgentAdapter(svc)

	paths := []string{"/tmp/img.jpg", "/tmp/doc.pdf"}
	_, err := a.Query(context.Background(), "see attached", paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svc.calls) == 0 {
		t.Fatal("expected Chat to be called")
	}
	call := svc.calls[0]
	if len(call.Attachments) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(call.Attachments))
	}
	if call.Attachments[0].Path != "/tmp/img.jpg" {
		t.Errorf("first attachment path mismatch: %q", call.Attachments[0].Path)
	}
}

// TestAgentAdapter_ChatError verifies that a ChatService error is wrapped and
// returned by Query.
func TestAgentAdapter_ChatError(t *testing.T) {
	svcErr := errors.New("llm provider unreachable")
	svc := &stubChatService{err: svcErr}
	a := NewAgentAdapter(svc)

	_, err := a.Query(context.Background(), "hello", nil)
	if err == nil {
		t.Fatal("expected error from Query")
	}
	if !errors.Is(err, svcErr) {
		t.Errorf("expected wrapped svcErr, got: %v", err)
	}
}

// TestAgentAdapter_NilPanic verifies that NewAgentAdapter panics on nil service.
func TestAgentAdapter_NilPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil ChatService")
		}
	}()
	NewAgentAdapter(nil)
}
