package telegram

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/goose/internal/audit"
	"go.uber.org/zap"
)

// mockSenderClient is a minimal mock for the Client interface used in sender tests.
type mockSenderClient struct {
	sendMessageFunc   func(ctx context.Context, req SendMessageRequest) (Message, error)
	sendPhotoFunc     func(ctx context.Context, req SendMediaRequest) (Message, error)
	sendDocumentFunc  func(ctx context.Context, req SendMediaRequest) (Message, error)
	sendMessageCalls  []SendMessageRequest
	sendPhotoCalls    []SendMediaRequest
	sendDocumentCalls []SendMediaRequest
}

func (m *mockSenderClient) GetMe(ctx context.Context) (User, error) {
	return User{ID: 1, Username: "testbot", IsBot: true}, nil
}

func (m *mockSenderClient) SendMessage(ctx context.Context, req SendMessageRequest) (Message, error) {
	m.sendMessageCalls = append(m.sendMessageCalls, req)
	if m.sendMessageFunc != nil {
		return m.sendMessageFunc(ctx, req)
	}
	return Message{ID: 100, ChatID: req.ChatID, Text: req.Text}, nil
}

func (m *mockSenderClient) GetUpdates(ctx context.Context, offset, timeoutSec int) ([]Update, error) {
	return nil, nil
}

func (m *mockSenderClient) AnswerCallbackQuery(ctx context.Context, callbackQueryID string) error {
	return nil
}

func (m *mockSenderClient) SendPhoto(ctx context.Context, req SendMediaRequest) (Message, error) {
	m.sendPhotoCalls = append(m.sendPhotoCalls, req)
	if m.sendPhotoFunc != nil {
		return m.sendPhotoFunc(ctx, req)
	}
	return Message{ID: 101, ChatID: req.ChatID}, nil
}

func (m *mockSenderClient) SendDocument(ctx context.Context, req SendMediaRequest) (Message, error) {
	m.sendDocumentCalls = append(m.sendDocumentCalls, req)
	if m.sendDocumentFunc != nil {
		return m.sendDocumentFunc(ctx, req)
	}
	return Message{ID: 102, ChatID: req.ChatID}, nil
}
func (m *mockSenderClient) EditMessageText(_ context.Context, req EditMessageTextRequest) (Message, error) {
	return Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (m *mockSenderClient) SetWebhook(_ context.Context, _ SetWebhookRequest) error   { return nil }
func (m *mockSenderClient) DeleteWebhook(_ context.Context, _ bool) error             { return nil }
func (m *mockSenderClient) SendChatAction(_ context.Context, _ int64, _ string) error { return nil }

// mockSenderStore is a minimal Store mock for sender tests.
type mockSenderStore struct {
	allowedUsers map[int64]bool
}

func newMockSenderStore(allowed ...int64) *mockSenderStore {
	m := &mockSenderStore{allowedUsers: make(map[int64]bool)}
	for _, id := range allowed {
		m.allowedUsers[id] = true
	}
	return m
}

func (m *mockSenderStore) GetUserMapping(ctx context.Context, chatID int64) (UserMapping, bool, error) {
	if allowed, ok := m.allowedUsers[chatID]; ok {
		return UserMapping{ChatID: chatID, Allowed: allowed}, true, nil
	}
	return UserMapping{}, false, nil
}

func (m *mockSenderStore) PutUserMapping(ctx context.Context, um UserMapping) error { return nil }
func (m *mockSenderStore) ListAllowed(ctx context.Context) ([]UserMapping, error)   { return nil, nil }
func (m *mockSenderStore) Approve(ctx context.Context, chatID int64) error          { return nil }
func (m *mockSenderStore) Revoke(ctx context.Context, chatID int64) error           { return nil }
func (m *mockSenderStore) GetLastOffset(ctx context.Context) (int64, error)         { return 0, nil }
func (m *mockSenderStore) PutLastOffset(ctx context.Context, offset int64) error    { return nil }
func (m *mockSenderStore) Close() error                                             { return nil }

// mockAuditWriter records all audit events for assertion.
type mockAuditWriter struct {
	events []audit.AuditEvent
}

func (m *mockAuditWriter) Write(ev audit.AuditEvent) error {
	m.events = append(m.events, ev)
	return nil
}
func (m *mockAuditWriter) Close() error { return nil }

func newTestSender(t *testing.T, client Client, store Store, auditW audit.Writer, chatIDs ...int64) *Sender {
	t.Helper()
	logger := zap.NewNop()
	aw := NewAuditWrapper(auditW, logger)
	return NewSender(client, store, aw, logger)
}

// TestSender_SendTextMessage verifies that a simple text-only send reaches
// Client.SendMessage with the correct parameters and records an outbound
// audit event.
func TestSender_SendTextMessage(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}

	s := newTestSender(t, mc, ms, maw)

	resp, err := s.Send(context.Background(), SendRequest{
		ChatID: 555,
		Text:   "Hello!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MessageID == 0 {
		t.Error("expected non-zero message ID in response")
	}
	if len(mc.sendMessageCalls) != 1 {
		t.Fatalf("expected 1 SendMessage call, got %d", len(mc.sendMessageCalls))
	}
	if mc.sendMessageCalls[0].Text != "Hello!" {
		t.Errorf("unexpected text: %q", mc.sendMessageCalls[0].Text)
	}
	if len(maw.events) == 0 {
		t.Error("expected at least one audit event")
	}
}

// TestSender_UnauthorizedChatID verifies that sending to a chat_id not in
// allowed_users returns ErrUnauthorizedChatID (REQ-MTGM-N02).
func TestSender_UnauthorizedChatID(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555) // 999 is NOT allowed
	maw := &mockAuditWriter{}

	s := newTestSender(t, mc, ms, maw)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 999,
		Text:   "Should fail",
	})
	if !errors.Is(err, ErrUnauthorizedChatID) {
		t.Errorf("expected ErrUnauthorizedChatID, got: %v", err)
	}
	if len(mc.sendMessageCalls) != 0 {
		t.Error("SendMessage must not be called for unauthorized chat_id")
	}
}

// TestSender_MarkdownV2Escape verifies that when parse_mode is MarkdownV2,
// the text is escaped before sending.
func TestSender_MarkdownV2Escape(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}

	s := newTestSender(t, mc, ms, maw)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID:    555,
		Text:      "Hello *world*!",
		ParseMode: ParseModeMarkdownV2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mc.sendMessageCalls) != 1 {
		t.Fatalf("expected 1 SendMessage call")
	}
	// Original asterisks should be escaped.
	sent := mc.sendMessageCalls[0].Text
	if sent == "Hello *world*!" {
		t.Error("text was not escaped for MarkdownV2")
	}
	// The escaped version should contain \*
	if len(sent) == 0 {
		t.Error("sent text must not be empty")
	}
}

// TestSender_ImageAttachment verifies that a send request with an image
// attachment uses Client.SendPhoto instead of Client.SendMessage.
func TestSender_ImageAttachment(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}

	s := newTestSender(t, mc, ms, maw)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 555,
		Text:   "See this image",
		Attachments: []Attachment{
			{Type: AttachmentTypeImage, Path: "/tmp/test.jpg"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mc.sendPhotoCalls) != 1 {
		t.Fatalf("expected 1 SendPhoto call, got %d", len(mc.sendPhotoCalls))
	}
	if len(mc.sendMessageCalls) != 0 {
		t.Errorf("expected 0 SendMessage calls, got %d", len(mc.sendMessageCalls))
	}
}

// TestSender_DocumentAttachment verifies that a send request with a document
// attachment uses Client.SendDocument instead of Client.SendMessage.
func TestSender_DocumentAttachment(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}

	s := newTestSender(t, mc, ms, maw)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 555,
		Text:   "See this document",
		Attachments: []Attachment{
			{Type: AttachmentTypeDocument, Path: "/tmp/test.pdf"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mc.sendDocumentCalls) != 1 {
		t.Fatalf("expected 1 SendDocument call, got %d", len(mc.sendDocumentCalls))
	}
}

// TestSender_AuditOutbound verifies that a successful send always records an
// outbound audit event with the correct direction metadata.
func TestSender_AuditOutbound(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}

	s := newTestSender(t, mc, ms, maw)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 555,
		Text:   "Audited message",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(maw.events) == 0 {
		t.Fatal("expected audit events, got none")
	}

	var foundOutbound bool
	for _, ev := range maw.events {
		if meta, ok := ev.Metadata["direction"]; ok && meta == "outbound" {
			foundOutbound = true
		}
	}
	if !foundOutbound {
		t.Error("expected an outbound audit event")
	}
}

// TestSender_BlockedUser verifies that a user with Allowed=false is treated
// as unauthorized (consistent with REQ-MTGM-N05 and REQ-MTGM-N02).
func TestSender_BlockedUser(t *testing.T) {
	mc := &mockSenderClient{}
	// chatID 666 exists but is blocked (Allowed=false).
	ms := &mockSenderStore{allowedUsers: map[int64]bool{666: false}}
	maw := &mockAuditWriter{}

	s := newTestSender(t, mc, ms, maw)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 666,
		Text:   "Should be blocked",
	})
	if !errors.Is(err, ErrUnauthorizedChatID) {
		t.Errorf("expected ErrUnauthorizedChatID for blocked user, got: %v", err)
	}
}
