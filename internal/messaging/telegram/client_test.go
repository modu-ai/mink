package telegram_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/goose/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// telegramResponse wraps the standard Telegram Bot API response envelope.
type telegramResponse struct {
	OK     bool        `json:"ok"`
	Result interface{} `json:"result"`
}

// makeTelegramServer returns an httptest server that handles common Bot API endpoints.
// The caller provides handlers for each endpoint path suffix (e.g. "getMe").
func makeTelegramServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc("/bot"+testToken+"/"+path, h)
	}
	return httptest.NewServer(mux)
}

const testToken = "test-token-123"

// jsonOK writes a successful Telegram API response with the given result.
func jsonOK(w http.ResponseWriter, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(telegramResponse{OK: true, Result: result})
}

// TestGetMe verifies that NewClient + GetMe correctly parses a getMe response.
func TestGetMe(t *testing.T) {
	ts := makeTelegramServer(t, map[string]http.HandlerFunc{
		"getMe": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, map[string]interface{}{
				"id":         42,
				"is_bot":     true,
				"first_name": "Test",
				"username":   "testbot",
			})
		},
	})
	defer ts.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(ts.URL))
	require.NoError(t, err)

	user, err := client.GetMe(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(42), user.ID)
	assert.Equal(t, "testbot", user.Username)
	assert.True(t, user.IsBot)
	assert.Equal(t, "Test", user.FirstName)
}

// TestSendMessage verifies that SendMessage correctly serialises the request and
// parses the Telegram API response.
func TestSendMessage(t *testing.T) {
	var gotChatID float64
	var gotText string

	ts := makeTelegramServer(t, map[string]http.HandlerFunc{
		"sendMessage": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotChatID, _ = body["chat_id"].(float64)
			gotText, _ = body["text"].(string)

			jsonOK(w, map[string]interface{}{
				"message_id": 1,
				"chat":       map[string]interface{}{"id": gotChatID},
				"text":       gotText,
				"date":       1715000000,
			})
		},
	})
	defer ts.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(ts.URL))
	require.NoError(t, err)

	msg, err := client.SendMessage(context.Background(), telegram.SendMessageRequest{
		ChatID: 999,
		Text:   "hello world",
	})
	require.NoError(t, err)

	// gotChatID captured from server as float64, compare with int64 cast
	assert.Equal(t, int64(999), int64(gotChatID))
	assert.Equal(t, "hello world", gotText)
	assert.Equal(t, 1, msg.ID)
	assert.Equal(t, "hello world", msg.Text)
}

// TestGetUpdates verifies that GetUpdates returns a parsed slice of Update values.
func TestGetUpdates(t *testing.T) {
	ts := makeTelegramServer(t, map[string]http.HandlerFunc{
		"getUpdates": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []map[string]interface{}{
				{
					"update_id": 100,
					"message": map[string]interface{}{
						"message_id": 1,
						"chat":       map[string]interface{}{"id": 111},
						"text":       "ping",
						"date":       1715000001,
						"from":       map[string]interface{}{"id": 555, "is_bot": false, "first_name": "Alice"},
					},
				},
				{
					"update_id": 101,
					"message": map[string]interface{}{
						"message_id": 2,
						"chat":       map[string]interface{}{"id": 111},
						"text":       "pong",
						"date":       1715000002,
						"from":       map[string]interface{}{"id": 555, "is_bot": false, "first_name": "Alice"},
					},
				},
			})
		},
	})
	defer ts.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(ts.URL))
	require.NoError(t, err)

	updates, err := client.GetUpdates(context.Background(), 0, 5)
	require.NoError(t, err)
	require.Len(t, updates, 2)

	assert.Equal(t, 100, updates[0].UpdateID)
	assert.NotNil(t, updates[0].Message)
	assert.Equal(t, "ping", updates[0].Message.Text)

	assert.Equal(t, 101, updates[1].UpdateID)
	assert.Equal(t, "pong", updates[1].Message.Text)
}

// TestGetUpdates_Empty verifies empty result list is handled without error.
func TestGetUpdates_Empty(t *testing.T) {
	ts := makeTelegramServer(t, map[string]http.HandlerFunc{
		"getUpdates": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []interface{}{})
		},
	})
	defer ts.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(ts.URL))
	require.NoError(t, err)

	updates, err := client.GetUpdates(context.Background(), 50, 5)
	require.NoError(t, err)
	assert.Empty(t, updates)
}
