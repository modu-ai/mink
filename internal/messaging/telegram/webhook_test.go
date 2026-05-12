package telegram_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- webhook test doubles ---

// webhookMockClient records SetWebhook/DeleteWebhook calls and can return errors.
type webhookMockClient struct {
	setWebhookReqs     []telegram.SetWebhookRequest
	deleteWebhookCalls int
	setWebhookErr      error
	deleteWebhookErr   error
}

func (c *webhookMockClient) GetMe(_ context.Context) (telegram.User, error) {
	return telegram.User{ID: 1, Username: "bot"}, nil
}
func (c *webhookMockClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	return telegram.Message{ID: 1, ChatID: req.ChatID, Text: req.Text}, nil
}
func (c *webhookMockClient) GetUpdates(_ context.Context, _ int, _ int) ([]telegram.Update, error) {
	return nil, nil
}
func (c *webhookMockClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (c *webhookMockClient) SendPhoto(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 10, ChatID: req.ChatID}, nil
}
func (c *webhookMockClient) SendDocument(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 11, ChatID: req.ChatID}, nil
}
func (c *webhookMockClient) EditMessageText(_ context.Context, req telegram.EditMessageTextRequest) (telegram.Message, error) {
	return telegram.Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (c *webhookMockClient) SetWebhook(_ context.Context, req telegram.SetWebhookRequest) error {
	c.setWebhookReqs = append(c.setWebhookReqs, req)
	return c.setWebhookErr
}
func (c *webhookMockClient) DeleteWebhook(_ context.Context, _ bool) error {
	c.deleteWebhookCalls++
	return c.deleteWebhookErr
}
func (c *webhookMockClient) SendChatAction(_ context.Context, _ int64, _ string) error { return nil }

// recordingHandler records calls to Handle.
type recordingWebhookHandler struct {
	updates []telegram.Update
	err     error
}

func (h *recordingWebhookHandler) Handle(_ context.Context, upd telegram.Update) error {
	h.updates = append(h.updates, upd)
	return h.err
}

// --- GenerateWebhookSecret tests ---

// TestGenerateWebhookSecret_Length verifies that the generated secret is 64 hex chars.
func TestGenerateWebhookSecret_Length(t *testing.T) {
	s, err := telegram.GenerateWebhookSecret()
	require.NoError(t, err)
	assert.Len(t, s, 64, "expected 64 hex chars (32 bytes)")
}

// TestGenerateWebhookSecret_Uniqueness verifies that two generated secrets differ.
func TestGenerateWebhookSecret_Uniqueness(t *testing.T) {
	s1, err := telegram.GenerateWebhookSecret()
	require.NoError(t, err)
	s2, err := telegram.GenerateWebhookSecret()
	require.NoError(t, err)
	assert.NotEqual(t, s1, s2, "generated secrets must be unique")
}

// TestGenerateWebhookSecret_HexOnly verifies the secret contains only hex characters.
func TestGenerateWebhookSecret_HexOnly(t *testing.T) {
	s, err := telegram.GenerateWebhookSecret()
	require.NoError(t, err)
	for _, c := range s {
		assert.True(t,
			(c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"expected lowercase hex char, got %q", c)
	}
}

// --- WebhookPath tests ---

// TestWebhookPath_Prefix verifies the path contains the expected prefix.
func TestWebhookPath_Prefix(t *testing.T) {
	secret := "abc123"
	path := telegram.WebhookPath(secret)
	assert.Equal(t, "/webhook/telegram/abc123", path)
}

// --- WebhookHandler tests ---

// makeUpdateBody builds a minimal Telegram update JSON body.
func makeUpdateBody(t *testing.T, chatID int64, text string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"update_id": 1,
		"message": map[string]interface{}{
			"message_id": 1,
			"chat":       map[string]interface{}{"id": chatID},
			"text":       text,
			"date":       1715000000,
			"from":       map[string]interface{}{"id": 42, "is_bot": false, "first_name": "Alice"},
		},
	})
	require.NoError(t, err)
	return body
}

// TestWebhookHandler_ValidRequest_CallsHandler verifies that a valid POST with
// correct secret token calls the handler and returns 200.
func TestWebhookHandler_ValidRequest_CallsHandler(t *testing.T) {
	secret := "test-secret-token"
	handler := &recordingWebhookHandler{}
	h := telegram.WebhookHandler(handler, secret, zap.NewNop())

	body := makeUpdateBody(t, 99, "hello")
	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram/"+secret, bytes.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, handler.updates, 1)
	assert.Equal(t, int64(99), handler.updates[0].Message.ChatID)
	assert.Equal(t, "hello", handler.updates[0].Message.Text)
}

// TestWebhookHandler_WrongSecret verifies that a mismatched secret returns 403.
func TestWebhookHandler_WrongSecret(t *testing.T) {
	handler := &recordingWebhookHandler{}
	h := telegram.WebhookHandler(handler, "correct-secret", zap.NewNop())

	body := makeUpdateBody(t, 1, "hi")
	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram/x", bytes.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Empty(t, handler.updates)
}

// TestWebhookHandler_MethodNotAllowed verifies that GET requests return 405.
func TestWebhookHandler_MethodNotAllowed(t *testing.T) {
	handler := &recordingWebhookHandler{}
	h := telegram.WebhookHandler(handler, "secret", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/webhook/telegram/secret", nil)
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// TestWebhookHandler_InvalidJSON verifies that malformed JSON returns 400.
func TestWebhookHandler_InvalidJSON(t *testing.T) {
	handler := &recordingWebhookHandler{}
	h := telegram.WebhookHandler(handler, "secret", zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram/secret",
		strings.NewReader("this is not json"))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, handler.updates)
}

// TestWebhookHandler_EmptySecret_SkipsSecretCheck verifies that when no secret
// is configured, all POST requests are accepted regardless of the header.
func TestWebhookHandler_EmptySecret_SkipsSecretCheck(t *testing.T) {
	handler := &recordingWebhookHandler{}
	h := telegram.WebhookHandler(handler, "", zap.NewNop())

	body := makeUpdateBody(t, 55, "open")
	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram/", bytes.NewReader(body))
	// No X-Telegram-Bot-Api-Secret-Token header.
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, handler.updates, 1)
}

// --- RegisterWebhook tests ---

// TestRegisterWebhook_Success verifies that RegisterWebhook registers the mux
// handler and calls client.SetWebhook with the correct URL.
func TestRegisterWebhook_Success(t *testing.T) {
	client := &webhookMockClient{}
	handler := &recordingWebhookHandler{}
	mux := http.NewServeMux()
	secret := "my-secret-token"
	publicURL := "https://goose.example.com"

	err := telegram.RegisterWebhook(context.Background(), client, mux, handler, publicURL, secret, zap.NewNop())
	require.NoError(t, err)

	// Client must have been called with the correct parameters.
	require.Len(t, client.setWebhookReqs, 1)
	req := client.setWebhookReqs[0]
	assert.Equal(t, publicURL+telegram.WebhookPath(secret), req.URL)
	assert.Equal(t, secret, req.SecretToken)
	assert.Contains(t, req.AllowedUpdates, "message")
	assert.Contains(t, req.AllowedUpdates, "callback_query")

	// The mux must have the endpoint registered: POST a valid update to it.
	rr := httptest.NewRecorder()
	body := makeUpdateBody(t, 77, "wh msg")
	httpReq := httptest.NewRequest(http.MethodPost, telegram.WebhookPath(secret), bytes.NewReader(body))
	httpReq.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	mux.ServeHTTP(rr, httpReq)
	assert.Equal(t, http.StatusOK, rr.Code)
	require.Len(t, handler.updates, 1)
}

// TestRegisterWebhook_EmptyPublicURL returns an error without calling SetWebhook.
func TestRegisterWebhook_EmptyPublicURL(t *testing.T) {
	client := &webhookMockClient{}
	mux := http.NewServeMux()
	handler := &recordingWebhookHandler{}

	err := telegram.RegisterWebhook(context.Background(), client, mux, handler, "", "secret", zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publicBaseURL")
	assert.Empty(t, client.setWebhookReqs)
}

// TestRegisterWebhook_EmptySecret returns an error without calling SetWebhook.
func TestRegisterWebhook_EmptySecret(t *testing.T) {
	client := &webhookMockClient{}
	mux := http.NewServeMux()
	handler := &recordingWebhookHandler{}

	err := telegram.RegisterWebhook(context.Background(), client, mux, handler, "https://example.com", "", zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret")
	assert.Empty(t, client.setWebhookReqs)
}

// TestRegisterWebhook_NilMux returns an error without calling SetWebhook.
func TestRegisterWebhook_NilMux(t *testing.T) {
	client := &webhookMockClient{}
	handler := &recordingWebhookHandler{}

	err := telegram.RegisterWebhook(context.Background(), client, nil, handler, "https://example.com", "secret", zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mux")
	assert.Empty(t, client.setWebhookReqs)
}

// TestRegisterWebhook_SetWebhookError propagates the SetWebhook error.
func TestRegisterWebhook_SetWebhookError(t *testing.T) {
	client := &webhookMockClient{setWebhookErr: fmt.Errorf("TLS certificate rejected")}
	mux := http.NewServeMux()
	handler := &recordingWebhookHandler{}

	err := telegram.RegisterWebhook(context.Background(), client, mux, handler,
		"https://goose.example.com", "secret", zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "setWebhook")
}

// TestSetWebhook_HTTP verifies that the httpClient sends the correct JSON body
// to the Telegram setWebhook endpoint and parses a true result without error.
func TestSetWebhook_HTTP(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"ok":true,"result":true}`)
	}))
	defer srv.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(srv.URL))
	require.NoError(t, err)

	err = client.SetWebhook(context.Background(), telegram.SetWebhookRequest{
		URL:            "https://goose.example.com/webhook/telegram/abc",
		SecretToken:    "abc",
		AllowedUpdates: []string{"message"},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://goose.example.com/webhook/telegram/abc", capturedBody["url"])
	assert.Equal(t, "abc", capturedBody["secret_token"])
}

// TestDeleteWebhook_HTTP verifies that the httpClient sends the correct JSON body
// to the deleteWebhook endpoint.
func TestDeleteWebhook_HTTP(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"ok":true,"result":true}`)
	}))
	defer srv.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(srv.URL))
	require.NoError(t, err)

	err = client.DeleteWebhook(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, true, capturedBody["drop_pending_updates"])
}
