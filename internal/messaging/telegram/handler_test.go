package telegram_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeHandlerClient is a test double for Client used in handler tests.
type fakeHandlerClient struct {
	sendErr  error
	sentReqs []telegram.SendMessageRequest
}

func (f *fakeHandlerClient) GetMe(_ context.Context) (telegram.User, error) {
	return telegram.User{}, nil
}

func (f *fakeHandlerClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	f.sentReqs = append(f.sentReqs, req)
	if f.sendErr != nil {
		return telegram.Message{}, f.sendErr
	}
	return telegram.Message{ID: 1, ChatID: req.ChatID, Text: req.Text}, nil
}

func (f *fakeHandlerClient) GetUpdates(_ context.Context, _ int, _ int) ([]telegram.Update, error) {
	return nil, nil
}
func (f *fakeHandlerClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (f *fakeHandlerClient) SendPhoto(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 10, ChatID: req.ChatID}, nil
}
func (f *fakeHandlerClient) SendDocument(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 11, ChatID: req.ChatID}, nil
}
func (f *fakeHandlerClient) EditMessageText(_ context.Context, req telegram.EditMessageTextRequest) (telegram.Message, error) {
	return telegram.Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (f *fakeHandlerClient) SetWebhook(_ context.Context, _ telegram.SetWebhookRequest) error {
	return nil
}
func (f *fakeHandlerClient) DeleteWebhook(_ context.Context, _ bool) error             { return nil }
func (f *fakeHandlerClient) SendChatAction(_ context.Context, _ int64, _ string) error { return nil }

// TestEchoHandler_TextUpdate verifies that a text update triggers SendMessage
// with the same chat ID and text.
func TestEchoHandler_TextUpdate(t *testing.T) {
	client := &fakeHandlerClient{}
	handler := telegram.NewEchoHandler(client, zap.NewNop())

	update := telegram.Update{
		UpdateID: 1,
		Message: &telegram.InboundMessage{
			ID:     1,
			ChatID: 12345,
			Text:   "echo me",
		},
	}

	err := handler.Handle(context.Background(), update)
	require.NoError(t, err)

	require.Len(t, client.sentReqs, 1)
	assert.Equal(t, int64(12345), client.sentReqs[0].ChatID)
	assert.Equal(t, "echo me", client.sentReqs[0].Text)
}

// TestEchoHandler_NilMessage verifies that a nil Message update is a no-op.
func TestEchoHandler_NilMessage(t *testing.T) {
	client := &fakeHandlerClient{}
	handler := telegram.NewEchoHandler(client, zap.NewNop())

	err := handler.Handle(context.Background(), telegram.Update{UpdateID: 2, Message: nil})
	require.NoError(t, err)
	assert.Empty(t, client.sentReqs)
}

// TestEchoHandler_EmptyText verifies that an update with empty text is skipped.
func TestEchoHandler_EmptyText(t *testing.T) {
	client := &fakeHandlerClient{}
	handler := telegram.NewEchoHandler(client, zap.NewNop())

	update := telegram.Update{
		UpdateID: 3,
		Message:  &telegram.InboundMessage{ChatID: 99, Text: ""},
	}

	err := handler.Handle(context.Background(), update)
	require.NoError(t, err)
	assert.Empty(t, client.sentReqs)
}

// TestEchoHandler_SendError verifies that a SendMessage failure is propagated.
func TestEchoHandler_SendError(t *testing.T) {
	sendErr := errors.New("network failure")
	client := &fakeHandlerClient{sendErr: sendErr}
	handler := telegram.NewEchoHandler(client, zap.NewNop())

	update := telegram.Update{
		UpdateID: 4,
		Message:  &telegram.InboundMessage{ChatID: 1, Text: "hello"},
	}

	err := handler.Handle(context.Background(), update)
	require.Error(t, err)
	assert.ErrorContains(t, err, "echo send")
}
