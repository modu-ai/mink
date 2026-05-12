package telegram_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendMessage_Silent_True verifies that SendMessageRequest.Silent=true
// causes disable_notification=true in the Telegram API request body (REQ-MTGM-O01).
func TestSendMessage_Silent_True(t *testing.T) {
	var gotParams map[string]interface{}
	ts := makeTelegramServer(t, map[string]http.HandlerFunc{
		"sendMessage": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotParams)
			jsonOK(w, map[string]interface{}{
				"message_id": 1,
				"chat":       map[string]interface{}{"id": 100},
				"text":       "hello",
				"date":       1715000000,
			})
		},
	})
	defer ts.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(ts.URL))
	require.NoError(t, err)

	_, err = client.SendMessage(context.Background(), telegram.SendMessageRequest{
		ChatID: 100,
		Text:   "hello",
		Silent: true,
	})
	require.NoError(t, err)

	dn, _ := gotParams["disable_notification"].(bool)
	assert.True(t, dn, "disable_notification must be true when Silent=true")
}

// TestSendMessage_Silent_False verifies that SendMessageRequest.Silent=false
// does not set disable_notification in the request body.
func TestSendMessage_Silent_False(t *testing.T) {
	var gotParams map[string]interface{}
	ts := makeTelegramServer(t, map[string]http.HandlerFunc{
		"sendMessage": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotParams)
			jsonOK(w, map[string]interface{}{
				"message_id": 1,
				"chat":       map[string]interface{}{"id": 100},
				"text":       "hello",
				"date":       1715000000,
			})
		},
	})
	defer ts.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(ts.URL))
	require.NoError(t, err)

	_, err = client.SendMessage(context.Background(), telegram.SendMessageRequest{
		ChatID: 100,
		Text:   "hello",
		Silent: false,
	})
	require.NoError(t, err)

	_, hasDN := gotParams["disable_notification"]
	assert.False(t, hasDN, "disable_notification must not be set when Silent=false")
}

// TestSendChatAction_Typing verifies that SendChatAction sends the correct
// chat_id and action parameters in the request body (REQ-MTGM-O02).
func TestSendChatAction_Typing(t *testing.T) {
	var gotParams map[string]interface{}
	ts := makeTelegramServer(t, map[string]http.HandlerFunc{
		"sendChatAction": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotParams)
			jsonOK(w, true)
		},
	})
	defer ts.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(ts.URL))
	require.NoError(t, err)

	err = client.SendChatAction(context.Background(), 42, telegram.ChatActionTyping)
	require.NoError(t, err)

	chatID, _ := gotParams["chat_id"].(float64)
	assert.Equal(t, float64(42), chatID)
	assert.Equal(t, "typing", gotParams["action"])
}

// TestSendChatAction_ServerError verifies that SendChatAction returns an error
// when the Telegram API reports a failure.
func TestSendChatAction_ServerError(t *testing.T) {
	ts := makeTelegramServer(t, map[string]http.HandlerFunc{
		"sendChatAction": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":          false,
				"description": "Bad Request",
			})
		},
	})
	defer ts.Close()

	client, err := telegram.NewClient(testToken, telegram.WithServerURL(ts.URL))
	require.NoError(t, err)

	err = client.SendChatAction(context.Background(), 99, telegram.ChatActionTyping)
	assert.Error(t, err)
}
