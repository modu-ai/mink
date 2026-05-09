package telegram

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"go.uber.org/zap"
)

// --- test doubles used only in callback tests ---

type callbackTestClient struct {
	sentMessages           []SendMessageRequest
	answerCallbackQueryIDs []string
	answerCallbackQueryErr error
}

func (c *callbackTestClient) GetMe(_ context.Context) (User, error) { return User{}, nil }
func (c *callbackTestClient) GetUpdates(_ context.Context, _, _ int) ([]Update, error) {
	return nil, nil
}
func (c *callbackTestClient) SendMessage(_ context.Context, req SendMessageRequest) (Message, error) {
	c.sentMessages = append(c.sentMessages, req)
	return Message{ID: 1, ChatID: req.ChatID, Text: req.Text}, nil
}
func (c *callbackTestClient) AnswerCallbackQuery(_ context.Context, id string) error {
	c.answerCallbackQueryIDs = append(c.answerCallbackQueryIDs, id)
	return c.answerCallbackQueryErr
}
func (c *callbackTestClient) SendPhoto(_ context.Context, req SendMediaRequest) (Message, error) {
	return Message{ID: 10, ChatID: req.ChatID}, nil
}
func (c *callbackTestClient) SendDocument(_ context.Context, req SendMediaRequest) (Message, error) {
	return Message{ID: 11, ChatID: req.ChatID}, nil
}

// recordingAgentQuery captures both text and attachments passed to Query.
type recordingAgentQuery struct {
	texts       []string
	attachments [][]string
	response    string
}

func (r *recordingAgentQuery) Query(_ context.Context, text string, attachments []string) (string, error) {
	r.texts = append(r.texts, text)
	r.attachments = append(r.attachments, attachments)
	return r.response, nil
}

func openCallbackStore(t *testing.T) Store {
	t.Helper()
	s, err := NewSqliteStore(filepath.Join(t.TempDir(), "cb.db"))
	if err != nil {
		t.Fatalf("NewSqliteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestHandleCallback_Normal verifies that a callback_query for an allowed user
// triggers answerCallbackQuery, audits the event, and calls the agent with the
// callback data as text.
func TestHandleCallback_Normal(t *testing.T) {
	client := &callbackTestClient{}
	store := openCallbackStore(t)
	maw := &audit.MockWriter{}
	aw := NewAuditWrapper(maw, zap.NewNop())
	agent := &recordingAgentQuery{response: "ack"}

	// Register user as allowed.
	ctx := context.Background()
	if err := store.PutUserMapping(ctx, UserMapping{ChatID: 555, Allowed: true, UserProfileID: "u1"}); err != nil {
		t.Fatalf("PutUserMapping: %v", err)
	}

	cfg := &Config{BotUsername: "testbot", Mode: "polling", AutoAdmitFirstUser: false}
	h := NewBridgeQueryHandler(client, store, aw, agent, cfg, zap.NewNop())

	update := Update{
		UpdateID: 1,
		CallbackQuery: &CallbackQuery{
			ID:         "cq-001",
			FromUserID: 555,
			ChatID:     555,
			MessageID:  42,
			Data:       "opt_a",
			ReceivedAt: time.Now(),
		},
	}

	if err := h.Handle(ctx, update); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// answerCallbackQuery must be called.
	if len(client.answerCallbackQueryIDs) == 0 {
		t.Error("expected answerCallbackQuery to be called")
	}

	// Agent must receive callback data in the query text.
	if len(agent.texts) == 0 {
		t.Fatal("expected agent to be called")
	}
	if !strings.Contains(agent.texts[0], "opt_a") {
		t.Errorf("agent text should contain callback data 'opt_a', got: %q", agent.texts[0])
	}

	// Response should be sent back to the user.
	if len(client.sentMessages) == 0 {
		t.Error("expected a response message to the user")
	}
}

// TestHandleCallback_ExpiredTimeout verifies that a callback received > 60s ago
// still gets processed but is audited with callback_expired=true.
func TestHandleCallback_ExpiredTimeout(t *testing.T) {
	client := &callbackTestClient{}
	store := openCallbackStore(t)
	maw := &audit.MockWriter{}
	aw := NewAuditWrapper(maw, zap.NewNop())
	agent := &recordingAgentQuery{response: "ack"}

	ctx := context.Background()
	if err := store.PutUserMapping(ctx, UserMapping{ChatID: 555, Allowed: true, UserProfileID: "u1"}); err != nil {
		t.Fatalf("PutUserMapping: %v", err)
	}

	cfg := &Config{BotUsername: "testbot", Mode: "polling"}
	h := NewBridgeQueryHandler(client, store, aw, agent, cfg, zap.NewNop())

	update := Update{
		UpdateID: 2,
		CallbackQuery: &CallbackQuery{
			ID:         "cq-002",
			ChatID:     555,
			MessageID:  43,
			Data:       "opt_b",
			ReceivedAt: time.Now().Add(-90 * time.Second), // expired
		},
	}

	if err := h.Handle(ctx, update); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Check that at least one audit event has callback_expired=true.
	var foundExpired bool
	for _, ev := range maw.Events {
		if v, ok := ev.Metadata["callback_expired"]; ok && v == "true" {
			foundExpired = true
		}
	}
	if !foundExpired {
		t.Error("expected an audit event with callback_expired=true")
	}
}

// TestHandleCallback_BlockedUser verifies that a blocked user's callback is
// silently dropped.
func TestHandleCallback_BlockedUser(t *testing.T) {
	client := &callbackTestClient{}
	store := openCallbackStore(t)
	maw := &audit.MockWriter{}
	aw := NewAuditWrapper(maw, zap.NewNop())
	agent := &recordingAgentQuery{response: "should not be called"}

	ctx := context.Background()
	// Blocked user.
	if err := store.PutUserMapping(ctx, UserMapping{ChatID: 666, Allowed: false, UserProfileID: "blocked"}); err != nil {
		t.Fatalf("PutUserMapping: %v", err)
	}

	cfg := &Config{BotUsername: "testbot", Mode: "polling"}
	h := NewBridgeQueryHandler(client, store, aw, agent, cfg, zap.NewNop())

	update := Update{
		UpdateID: 3,
		CallbackQuery: &CallbackQuery{
			ID:         "cq-003",
			ChatID:     666,
			MessageID:  44,
			Data:       "blocked_data",
			ReceivedAt: time.Now(),
		},
	}

	if err := h.Handle(ctx, update); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(agent.texts) > 0 {
		t.Error("agent must not be called for blocked user callback")
	}
}
