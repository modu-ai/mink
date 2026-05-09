package telegram

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/goose/internal/agent"
)

// stubStreamingChatService is a minimal agent.StreamingChatService for stream adapter tests.
type stubStreamingChatService struct {
	chunks []agent.ChatChunk
	err    error
}

func (s *stubStreamingChatService) ChatStream(_ context.Context, _ agent.ChatRequest) (<-chan agent.ChatChunk, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan agent.ChatChunk, len(s.chunks))
	for _, c := range s.chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// TestAgentStreamAdapter_Implements verifies that *AgentStreamAdapter satisfies AgentStream.
func TestAgentStreamAdapter_Implements(t *testing.T) {
	var _ AgentStream = (*AgentStreamAdapter)(nil)
}

// TestAgentStreamAdapter_QueryStream_HappyPath verifies that chunks are forwarded as StreamChunk values.
func TestAgentStreamAdapter_QueryStream_HappyPath(t *testing.T) {
	svc := &stubStreamingChatService{
		chunks: []agent.ChatChunk{
			{Content: "hello", Final: false},
			{Content: " world", Final: true},
		},
	}
	a := NewAgentStreamAdapter(svc)

	ch, err := a.QueryStream(context.Background(), "hi", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []StreamChunk
	for c := range ch {
		got = append(got, c)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}
	if got[0].Content != "hello" || got[0].Final {
		t.Errorf("unexpected chunk[0]: %+v", got[0])
	}
	if got[1].Content != " world" || !got[1].Final {
		t.Errorf("unexpected chunk[1]: %+v", got[1])
	}
}

// TestAgentStreamAdapter_QueryStream_UpstreamError verifies that a service error is propagated.
func TestAgentStreamAdapter_QueryStream_UpstreamError(t *testing.T) {
	upstreamErr := errors.New("llm unavailable")
	svc := &stubStreamingChatService{err: upstreamErr}
	a := NewAgentStreamAdapter(svc)

	ch, err := a.QueryStream(context.Background(), "hello", nil)
	if err == nil {
		t.Fatal("expected error from QueryStream")
	}
	if !errors.Is(err, upstreamErr) {
		t.Errorf("expected wrapped upstream error, got: %v", err)
	}
	if ch != nil {
		t.Error("expected nil channel on error")
	}
}

// TestAgentStreamAdapter_QueryStream_CtxCancel verifies that cancellation stops forwarding.
func TestAgentStreamAdapter_QueryStream_CtxCancel(t *testing.T) {
	// Create a channel that never closes to simulate a slow producer.
	infiniteSvc := &infiniteStreamService{}
	a := NewAgentStreamAdapter(infiniteSvc)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := a.QueryStream(ctx, "hi", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cancel immediately and drain — must not hang.
	cancel()

	// Read until closed.
	for range ch {
	}
	// Reaching here means no goroutine leak / hang.
}

// infiniteStreamService emits chunks continuously until the context is cancelled.
type infiniteStreamService struct{}

func (s *infiniteStreamService) ChatStream(ctx context.Context, _ agent.ChatRequest) (<-chan agent.ChatChunk, error) {
	ch := make(chan agent.ChatChunk)
	go func() {
		defer close(ch)
		for {
			select {
			case ch <- agent.ChatChunk{Content: "x", Final: false}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// TestNewAgentStreamAdapter_NilPanic verifies that NewAgentStreamAdapter panics when
// a nil StreamingChatService is provided.
func TestNewAgentStreamAdapter_NilPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil StreamingChatService")
		}
	}()
	NewAgentStreamAdapter(nil)
}

// TestAgentStreamAdapter_QueryStream_AttachmentsForwarded verifies that attachment paths
// are converted to agent.Attachment values and passed to ChatStream.
func TestAgentStreamAdapter_QueryStream_AttachmentsForwarded(t *testing.T) {
	svc := &recordingStreamService{}
	a := NewAgentStreamAdapter(svc)

	paths := []string{"/tmp/img.jpg", "/tmp/doc.pdf"}
	ch, err := a.QueryStream(context.Background(), "see attached", paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	if len(svc.calls) == 0 {
		t.Fatal("expected ChatStream to be called")
	}
	call := svc.calls[0]
	if len(call.Attachments) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(call.Attachments))
	}
	if call.Attachments[0].Path != "/tmp/img.jpg" {
		t.Errorf("first attachment path mismatch: %q", call.Attachments[0].Path)
	}
}

// recordingStreamService records each ChatStream call.
type recordingStreamService struct {
	calls []agent.ChatRequest
}

func (s *recordingStreamService) ChatStream(_ context.Context, req agent.ChatRequest) (<-chan agent.ChatChunk, error) {
	s.calls = append(s.calls, req)
	ch := make(chan agent.ChatChunk, 1)
	ch <- agent.ChatChunk{Content: "", Final: true}
	close(ch)
	return ch, nil
}
