package telegram

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/audit"
)

// mockTypingClient records SendChatAction calls and implements Client.
type mockTypingClient struct {
	mu              sync.Mutex
	chatActionCalls []string // action values received
}

func (m *mockTypingClient) GetMe(_ context.Context) (User, error) { return User{}, nil }
func (m *mockTypingClient) SendMessage(_ context.Context, req SendMessageRequest) (Message, error) {
	return Message{ID: 1, ChatID: req.ChatID, Text: req.Text}, nil
}
func (m *mockTypingClient) GetUpdates(_ context.Context, _, _ int) ([]Update, error) {
	return nil, nil
}
func (m *mockTypingClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (m *mockTypingClient) SendPhoto(_ context.Context, req SendMediaRequest) (Message, error) {
	return Message{ID: 10, ChatID: req.ChatID}, nil
}
func (m *mockTypingClient) SendDocument(_ context.Context, req SendMediaRequest) (Message, error) {
	return Message{ID: 11, ChatID: req.ChatID}, nil
}
func (m *mockTypingClient) EditMessageText(_ context.Context, req EditMessageTextRequest) (Message, error) {
	return Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (m *mockTypingClient) SetWebhook(_ context.Context, _ SetWebhookRequest) error { return nil }
func (m *mockTypingClient) DeleteWebhook(_ context.Context, _ bool) error           { return nil }

// SendChatAction records the action and returns nil (REQ-MTGM-O02).
func (m *mockTypingClient) SendChatAction(_ context.Context, _ int64, action string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatActionCalls = append(m.chatActionCalls, action)
	return nil
}

func (m *mockTypingClient) ActionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.chatActionCalls)
}

// TestStartTypingIndicator_FirstActionImmediate verifies that startTypingIndicator
// sends sendChatAction immediately when called (REQ-MTGM-O02).
func TestStartTypingIndicator_FirstActionImmediate(t *testing.T) {
	client := &mockTypingClient{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startTypingIndicator(ctx, client, 42, newNopLogger())

	// Give goroutine time to send first action (should be nearly instant).
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if client.ActionCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if client.ActionCount() < 1 {
		t.Errorf("expected at least 1 sendChatAction call immediately, got %d", client.ActionCount())
	}
}

// TestStartTypingIndicator_StopsOnContextCancel verifies that the typing goroutine
// stops after context cancellation (no more calls after cancel). This ensures
// the goroutine does not leak (REQ-MTGM-O02).
func TestStartTypingIndicator_StopsOnContextCancel(t *testing.T) {
	client := &mockTypingClient{}
	ctx, cancel := context.WithCancel(context.Background())

	startTypingIndicator(ctx, client, 42, newNopLogger())

	// Wait for first action then cancel.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if client.ActionCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	// Wait for goroutine to process cancellation.
	time.Sleep(50 * time.Millisecond)

	countAfterCancel := client.ActionCount()
	time.Sleep(100 * time.Millisecond)
	countLater := client.ActionCount()

	if countLater > countAfterCancel {
		t.Errorf("goroutine continued after ctx cancel: %d -> %d calls", countAfterCancel, countLater)
	}
}

// TestStartTypingIndicator_AllActionsAreTyping verifies that every recorded
// action uses the ChatActionTyping constant (REQ-MTGM-O02).
func TestStartTypingIndicator_AllActionsAreTyping(t *testing.T) {
	client := &mockTypingClient{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startTypingIndicator(ctx, client, 99, newNopLogger())

	// Collect a few calls.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if client.ActionCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	time.Sleep(20 * time.Millisecond)

	client.mu.Lock()
	defer client.mu.Unlock()
	for i, a := range client.chatActionCalls {
		if a != ChatActionTyping {
			t.Errorf("call[%d]: got action %q, want %q", i, a, ChatActionTyping)
		}
	}
}

// TestBridgeQueryHandler_TypingIndicator_Enabled verifies that when
// cfg.TypingIndicator=true the handler calls SendChatAction before the agent
// responds (REQ-MTGM-O02).
func TestBridgeQueryHandler_TypingIndicator_Enabled(t *testing.T) {
	// agentReady controls when the agent query returns.
	agentReady := make(chan struct{})
	// typingCalled is set to 1 when SendChatAction is observed.
	var typingCalled atomic.Int32

	// Use typingRecordingClient (package-level type) to capture SendChatAction calls.
	client := &typingRecordingClient{}
	client.SendChatActionFunc = func(_ context.Context, _ int64, action string) error {
		typingCalled.Store(1)
		client.mu.Lock()
		client.chatActionCalls = append(client.chatActionCalls, action)
		client.mu.Unlock()
		return nil
	}

	slowAgent := &slowAgentQuery{
		ready:    agentReady,
		response: "done",
	}

	store := newMockSenderStore(777)
	maw := &audit.MockWriter{}
	aw := NewAuditWrapper(maw, newNopLogger())

	cfg := &Config{
		AllowedUsers:    []int64{777},
		TypingIndicator: true,
	}

	h := NewBridgeQueryHandler(client, store, aw, slowAgent, cfg, newNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Close agentReady after a short delay so typing fires first.
	go func() {
		time.Sleep(60 * time.Millisecond)
		close(agentReady)
	}()

	update := Update{
		UpdateID: 1,
		Message:  &InboundMessage{ID: 1, ChatID: 777, Text: "hi"},
	}

	// Ensure mapping exists.
	_ = store.PutUserMapping(ctx, UserMapping{ChatID: 777, Allowed: true})

	err := h.Handle(ctx, update)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if typingCalled.Load() == 0 {
		t.Error("SendChatAction was not called when TypingIndicator=true")
	}
}

// slowAgentQuery delays Query until its ready channel is closed.
type slowAgentQuery struct {
	ready    chan struct{}
	response string
}

func (s *slowAgentQuery) Query(ctx context.Context, _ string, _ []string) (string, error) {
	select {
	case <-s.ready:
		return s.response, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// typingRecordingClient extends mockTypingClient with a replaceable function.
type typingRecordingClient struct {
	mockTypingClient
	SendChatActionFunc func(ctx context.Context, chatID int64, action string) error
}

func (c *typingRecordingClient) SendChatAction(ctx context.Context, chatID int64, action string) error {
	if c.SendChatActionFunc != nil {
		return c.SendChatActionFunc(ctx, chatID, action)
	}
	return c.mockTypingClient.SendChatAction(ctx, chatID, action)
}
