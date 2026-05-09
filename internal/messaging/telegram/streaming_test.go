package telegram

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

// streamingTestClient is a minimal Client double for streaming tests.
// It records editMessageText calls and can simulate errors.
type streamingTestClient struct {
	sentMessages        []SendMessageRequest
	editRequests        []EditMessageTextRequest
	sendMessageErr      error
	editMessageErr      error
	sendMessageReturnID int
}

func (c *streamingTestClient) GetMe(_ context.Context) (User, error) { return User{}, nil }
func (c *streamingTestClient) GetUpdates(_ context.Context, _, _ int) ([]Update, error) {
	return nil, nil
}
func (c *streamingTestClient) SendMessage(_ context.Context, req SendMessageRequest) (Message, error) {
	c.sentMessages = append(c.sentMessages, req)
	if c.sendMessageErr != nil {
		return Message{}, c.sendMessageErr
	}
	id := c.sendMessageReturnID
	if id == 0 {
		id = len(c.sentMessages) * 100
	}
	return Message{ID: id, ChatID: req.ChatID, Text: req.Text}, nil
}
func (c *streamingTestClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (c *streamingTestClient) SendPhoto(_ context.Context, req SendMediaRequest) (Message, error) {
	return Message{ID: 10, ChatID: req.ChatID}, nil
}
func (c *streamingTestClient) SendDocument(_ context.Context, req SendMediaRequest) (Message, error) {
	return Message{ID: 11, ChatID: req.ChatID}, nil
}
func (c *streamingTestClient) EditMessageText(_ context.Context, req EditMessageTextRequest) (Message, error) {
	c.editRequests = append(c.editRequests, req)
	if c.editMessageErr != nil {
		return Message{}, c.editMessageErr
	}
	return Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (c *streamingTestClient) SetWebhook(_ context.Context, _ SetWebhookRequest) error   { return nil }
func (c *streamingTestClient) DeleteWebhook(_ context.Context, _ bool) error             { return nil }
func (c *streamingTestClient) SendChatAction(_ context.Context, _ int64, _ string) error { return nil }

// makeChunkChan creates a closed channel pre-filled with the given chunks.
func makeChunkChan(chunks ...StreamChunk) <-chan StreamChunk {
	ch := make(chan StreamChunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch
}

// newNopLogger returns a no-op zap logger for streaming tests.
func newNopLogger() *zap.Logger {
	return zap.NewNop()
}

// TestRunStreaming_SingleFinalChunk verifies that a single Final chunk triggers
// one flush (one editMessageText call).
func TestRunStreaming_SingleFinalChunk(t *testing.T) {
	client := &streamingTestClient{sendMessageReturnID: 42}
	ch := makeChunkChan(StreamChunk{Content: "hello world", Final: true})

	result, err := runStreaming(context.Background(), client, 100, ch, false, newNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Placeholder must have been sent.
	if len(client.sentMessages) != 1 {
		t.Fatalf("expected 1 SendMessage (placeholder), got %d", len(client.sentMessages))
	}
	if client.sentMessages[0].Text != streamingPlaceholder {
		t.Errorf("placeholder text mismatch: %q", client.sentMessages[0].Text)
	}

	// EditMessageText must have been called exactly once.
	if len(client.editRequests) != 1 {
		t.Fatalf("expected 1 EditMessageText, got %d", len(client.editRequests))
	}
	if client.editRequests[0].Text != "hello world" {
		t.Errorf("edit text mismatch: %q", client.editRequests[0].Text)
	}

	// Result reflects the edit.
	if result.EditCount != 1 {
		t.Errorf("expected EditCount=1, got %d", result.EditCount)
	}
	if result.Final != "hello world" {
		t.Errorf("expected Final='hello world', got %q", result.Final)
	}
	if result.PlaceholderID != 42 {
		t.Errorf("expected PlaceholderID=42, got %d", result.PlaceholderID)
	}
}

// TestRunStreaming_MultipleChunksAccumulate verifies that chunks are concatenated
// and the full text is present in the final result.
func TestRunStreaming_MultipleChunksAccumulate(t *testing.T) {
	client := &streamingTestClient{sendMessageReturnID: 50}
	ch := makeChunkChan(
		StreamChunk{Content: "Hello", Final: false},
		StreamChunk{Content: " there", Final: false},
		StreamChunk{Content: "", Final: true},
	)

	result, err := runStreaming(context.Background(), client, 200, ch, false, newNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Final != "Hello there" {
		t.Errorf("expected Final='Hello there', got %q", result.Final)
	}
	// EditCount >= 1 (final flush must happen).
	if result.EditCount < 1 {
		t.Errorf("expected at least 1 edit, got %d", result.EditCount)
	}
}

// TestRunStreaming_NoDuplicateFlush verifies that sending the same buffer content
// twice does not increment EditCount (lastSent deduplication).
func TestRunStreaming_NoDuplicateFlush(t *testing.T) {
	client := &streamingTestClient{sendMessageReturnID: 60}
	ch := makeChunkChan(
		StreamChunk{Content: "same", Final: false},
		StreamChunk{Content: "", Final: true},
	)

	result, err := runStreaming(context.Background(), client, 300, ch, false, newNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "same" flushed once on Final. A second flush of identical content is skipped.
	if result.EditCount != 1 {
		t.Errorf("expected EditCount=1 (no duplicate flush), got %d", result.EditCount)
	}
}

// TestRunStreaming_PlaceholderSendError verifies that a sendMessage failure
// returns an error without proceeding to stream chunks.
func TestRunStreaming_PlaceholderSendError(t *testing.T) {
	sendErr := errors.New("network error")
	client := &streamingTestClient{sendMessageErr: sendErr}
	ch := makeChunkChan(StreamChunk{Content: "hi", Final: true})

	_, err := runStreaming(context.Background(), client, 400, ch, false, newNopLogger())
	if err == nil {
		t.Fatal("expected error when placeholder send fails")
	}
	if !errors.Is(err, sendErr) {
		t.Errorf("expected wrapped sendErr, got: %v", err)
	}

	// No edit calls must have been made.
	if len(client.editRequests) != 0 {
		t.Errorf("expected 0 editMessageText calls, got %d", len(client.editRequests))
	}
}

// TestRunStreaming_CtxCancelReturnsAborted verifies that cancelling the context
// returns Aborted=true and a non-nil error.
func TestRunStreaming_CtxCancelReturnsAborted(t *testing.T) {
	client := &streamingTestClient{sendMessageReturnID: 70}

	// An unbuffered channel that is never written to; we rely on ctx cancellation.
	infinite := make(chan StreamChunk)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := runStreaming(ctx, client, 500, infinite, false, newNopLogger())
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !result.Aborted {
		t.Error("expected Aborted=true on ctx cancellation")
	}
}

// TestRunStreaming_EditError_ContinuesWithoutPanic verifies that an editMessageText
// error is logged and swallowed (stream continues; no panic).
func TestRunStreaming_EditError_ContinuesWithoutPanic(t *testing.T) {
	client := &streamingTestClient{
		sendMessageReturnID: 80,
		editMessageErr:      errors.New("rate limited"),
	}
	ch := makeChunkChan(StreamChunk{Content: "text", Final: true})

	// Should not panic or return an error — edit failures are non-fatal.
	result, err := runStreaming(context.Background(), client, 600, ch, false, newNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Final buffer text is preserved even though edit failed.
	if result.Final != "text" {
		t.Errorf("expected Final='text', got %q", result.Final)
	}
	// EditCount stays 0 because the edit call returned an error (no increment).
	if result.EditCount != 0 {
		t.Errorf("expected EditCount=0 (edit failed), got %d", result.EditCount)
	}
}

// TestRunStreaming_TotalDurationPositive verifies that TotalDuration is non-negative.
func TestRunStreaming_TotalDurationPositive(t *testing.T) {
	client := &streamingTestClient{sendMessageReturnID: 90}
	ch := makeChunkChan(StreamChunk{Content: "partial", Final: false}, StreamChunk{Content: "", Final: true})

	result, err := runStreaming(context.Background(), client, 700, ch, false, newNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalDuration < 0 {
		t.Error("unexpected negative duration")
	}
}

// TestRunStreaming_Silent_True verifies that when silent=true the placeholder
// SendMessage is called with Silent=true (REQ-MTGM-O01).
func TestRunStreaming_Silent_True(t *testing.T) {
	client := &streamingTestClient{sendMessageReturnID: 99}
	ch := makeChunkChan(StreamChunk{Content: "response", Final: true})

	_, err := runStreaming(context.Background(), client, 800, ch, true, newNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.sentMessages) != 1 {
		t.Fatalf("expected 1 SendMessage call, got %d", len(client.sentMessages))
	}
	if !client.sentMessages[0].Silent {
		t.Errorf("placeholder message: Silent got false, want true when silent=true")
	}
}

// TestRunStreaming_Silent_False verifies that when silent=false the placeholder
// SendMessage does not set Silent.
func TestRunStreaming_Silent_False(t *testing.T) {
	client := &streamingTestClient{sendMessageReturnID: 100}
	ch := makeChunkChan(StreamChunk{Content: "text", Final: true})

	_, err := runStreaming(context.Background(), client, 900, ch, false, newNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.sentMessages) != 1 {
		t.Fatalf("expected 1 SendMessage call, got %d", len(client.sentMessages))
	}
	if client.sentMessages[0].Silent {
		t.Errorf("placeholder message: Silent got true, want false when silent=false")
	}
}
