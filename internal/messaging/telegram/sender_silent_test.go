package telegram

import (
	"context"
	"testing"

	"github.com/modu-ai/goose/internal/audit"
	"go.uber.org/zap"
)

// mockSilentClient records whether disable_notification was set in SendMessage
// requests. It implements the Client interface used in sender tests.
type mockSilentClient struct {
	sentMessages  []SendMessageRequest
	sentPhotos    []SendMediaRequest
	sentDocuments []SendMediaRequest
}

func (m *mockSilentClient) GetMe(_ context.Context) (User, error) {
	return User{ID: 1, Username: "testbot", IsBot: true}, nil
}
func (m *mockSilentClient) SendMessage(_ context.Context, req SendMessageRequest) (Message, error) {
	m.sentMessages = append(m.sentMessages, req)
	return Message{ID: 10, ChatID: req.ChatID, Text: req.Text}, nil
}
func (m *mockSilentClient) GetUpdates(_ context.Context, _, _ int) ([]Update, error) {
	return nil, nil
}
func (m *mockSilentClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (m *mockSilentClient) SendPhoto(_ context.Context, req SendMediaRequest) (Message, error) {
	m.sentPhotos = append(m.sentPhotos, req)
	return Message{ID: 20, ChatID: req.ChatID}, nil
}
func (m *mockSilentClient) SendDocument(_ context.Context, req SendMediaRequest) (Message, error) {
	m.sentDocuments = append(m.sentDocuments, req)
	return Message{ID: 30, ChatID: req.ChatID}, nil
}
func (m *mockSilentClient) EditMessageText(_ context.Context, req EditMessageTextRequest) (Message, error) {
	return Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (m *mockSilentClient) SendChatAction(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockSilentClient) SetWebhook(_ context.Context, _ SetWebhookRequest) error   { return nil }
func (m *mockSilentClient) DeleteWebhook(_ context.Context, _ bool) error             { return nil }

// newSilentSender constructs a Sender wired to the given mockSilentClient with
// a store that allows the given chatID.
func newSilentSender(t *testing.T, client *mockSilentClient, chatID int64, silentDefault bool) *Sender {
	t.Helper()
	store := newMockSenderStore(chatID)
	maw := &audit.MockWriter{}
	aw := NewAuditWrapper(maw, zap.NewNop())
	s := NewSender(client, store, aw, zap.NewNop())
	if silentDefault {
		s = s.WithSilentDefault(true)
	}
	return s
}

// TestSender_SilentDefault_True verifies that when silentDefault=true a text
// message is sent with Silent=true (REQ-MTGM-O01).
func TestSender_SilentDefault_True(t *testing.T) {
	client := &mockSilentClient{}
	s := newSilentSender(t, client, 111, true)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 111,
		Text:   "hello",
		Silent: false, // explicit false, silentDefault overrides
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(client.sentMessages) != 1 {
		t.Fatalf("expected 1 sendMessage call, got %d", len(client.sentMessages))
	}
	if !client.sentMessages[0].Silent {
		t.Errorf("SendMessageRequest.Silent: got false, want true (silentDefault=true)")
	}
}

// TestSender_SilentDefault_False_ReqSilentTrue verifies that when silentDefault=false
// but req.Silent=true the request is still sent with Silent=true.
func TestSender_SilentDefault_False_ReqSilentTrue(t *testing.T) {
	client := &mockSilentClient{}
	s := newSilentSender(t, client, 111, false)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 111,
		Text:   "hello",
		Silent: true,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(client.sentMessages) != 1 {
		t.Fatalf("expected 1 sendMessage call, got %d", len(client.sentMessages))
	}
	if !client.sentMessages[0].Silent {
		t.Errorf("SendMessageRequest.Silent: got false, want true (req.Silent=true)")
	}
}

// TestSender_SilentDefault_False verifies that when both silentDefault and
// req.Silent are false the request is sent without Silent flag.
func TestSender_SilentDefault_False(t *testing.T) {
	client := &mockSilentClient{}
	s := newSilentSender(t, client, 111, false)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 111,
		Text:   "hello",
		Silent: false,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(client.sentMessages) != 1 {
		t.Fatalf("expected 1 sendMessage call, got %d", len(client.sentMessages))
	}
	if client.sentMessages[0].Silent {
		t.Errorf("SendMessageRequest.Silent: got true, want false")
	}
}

// TestSender_SilentDefault_Photo verifies that silentDefault=true propagates
// to sendPhoto calls as well.
func TestSender_SilentDefault_Photo(t *testing.T) {
	client := &mockSilentClient{}
	s := newSilentSender(t, client, 222, true)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 222,
		Text:   "caption",
		Attachments: []Attachment{
			{Type: AttachmentTypeImage, Path: "/tmp/img.jpg"},
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(client.sentPhotos) != 1 {
		t.Fatalf("expected 1 sendPhoto call, got %d", len(client.sentPhotos))
	}
	if !client.sentPhotos[0].Silent {
		t.Errorf("SendMediaRequest.Silent for photo: got false, want true (silentDefault=true)")
	}
}
