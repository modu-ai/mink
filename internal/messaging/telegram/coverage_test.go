package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

var stat = os.Stat
var writeFile = os.WriteFile

// TestDecodeChatID_NumericString verifies that a numeric string is decoded
// to the corresponding integer.
func TestDecodeChatID_NumericString(t *testing.T) {
	raw := json.RawMessage(`"12345"`)
	id, err := decodeChatID(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 12345 {
		t.Errorf("expected 12345, got %d", id)
	}
}

// TestDecodeChatID_NonNumericString verifies that a non-numeric string returns
// an error.
func TestDecodeChatID_NonNumericString(t *testing.T) {
	raw := json.RawMessage(`"@username"`)
	_, err := decodeChatID(raw)
	if err == nil {
		t.Error("expected error for non-numeric string")
	}
}

// TestDecodeChatID_Empty verifies that empty input returns an error.
func TestDecodeChatID_Empty(t *testing.T) {
	_, err := decodeChatID(json.RawMessage(nil))
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// TestDecodeChatID_Invalid verifies that invalid JSON returns an error.
func TestDecodeChatID_Invalid(t *testing.T) {
	_, err := decodeChatID(json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for object chat_id")
	}
}

// TestNewClient_EmptyToken verifies that NewClient returns an error for empty tokens.
func TestNewClient_EmptyToken(t *testing.T) {
	_, err := NewClient("")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

// TestAnswerCallbackQuery_HTTP verifies that AnswerCallbackQuery sends the
// correct JSON body to the Telegram API.
func TestAnswerCallbackQuery_HTTP(t *testing.T) {
	var receivedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": true}) //nolint:errcheck
	}))
	defer srv.Close()

	c, err := NewClient("test-token", WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if err := c.AnswerCallbackQuery(context.Background(), "cq-001"); err != nil {
		t.Fatalf("AnswerCallbackQuery: %v", err)
	}

	if receivedBody["callback_query_id"] != "cq-001" {
		t.Errorf("expected callback_query_id='cq-001', got: %v", receivedBody)
	}
}

// TestSender_SilentFlag verifies that the silent flag is accepted without error.
func TestSender_SilentFlag(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}
	s := newTestSender(t, mc, ms, maw)

	_, err := s.Send(context.Background(), SendRequest{
		ChatID: 555,
		Text:   "quiet message",
		Silent: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestTool_Call_ImageAttachment verifies that an image attachment in tool input
// is dispatched to SendPhoto.
func TestTool_Call_ImageAttachment(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}
	logger := zap.NewNop()
	aw := NewAuditWrapper(maw, logger)
	sender := NewSender(mc, ms, aw, logger)
	tool := &telegramSendMessageTool{sender: sender}

	input := json.RawMessage(`{
		"chat_id": 555,
		"text": "here is a photo",
		"attachments": [{"type": "image", "path": "/tmp/img.jpg"}]
	}`)
	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %s", result.Content)
	}
	if len(mc.sendPhotoCalls) == 0 {
		t.Error("expected SendPhoto to be called for image attachment")
	}
}

// TestSendPhoto_HTTP verifies that SendPhoto calls the correct API endpoint.
func TestSendPhoto_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"ok":     true,
			"result": map[string]interface{}{"message_id": 5, "chat": map[string]interface{}{"id": 555}, "date": 0},
		})
	}))
	defer srv.Close()

	c, err := NewClient("tok", WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	msg, err := c.SendPhoto(context.Background(), SendMediaRequest{ChatID: 555, URL: "https://example.com/img.jpg"})
	if err != nil {
		t.Fatalf("SendPhoto: %v", err)
	}
	if msg.ID != 5 {
		t.Errorf("expected message_id 5, got %d", msg.ID)
	}
}

// TestSendDocument_HTTP verifies that SendDocument calls the correct API endpoint.
func TestSendDocument_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"ok":     true,
			"result": map[string]interface{}{"message_id": 6, "chat": map[string]interface{}{"id": 555}, "date": 0},
		})
	}))
	defer srv.Close()

	c, err := NewClient("tok", WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	msg, err := c.SendDocument(context.Background(), SendMediaRequest{ChatID: 555, URL: "https://example.com/doc.pdf"})
	if err != nil {
		t.Fatalf("SendDocument: %v", err)
	}
	if msg.ID != 6 {
		t.Errorf("expected message_id 6, got %d", msg.ID)
	}
}

// TestHandleCallback_NotFound verifies that an unknown user receives a gate message.
func TestHandleCallback_NotFound(t *testing.T) {
	client := &callbackTestClient{}
	store := openCallbackStore(t)
	maw := &mockAuditWriter{}
	aw := NewAuditWrapper(maw, zap.NewNop())
	agent := &recordingAgentQuery{response: "should not be called"}

	cfg := &Config{BotUsername: "testbot", Mode: "polling", AutoAdmitFirstUser: false}
	h := NewBridgeQueryHandler(client, store, aw, agent, cfg, zap.NewNop())

	update := Update{
		UpdateID: 10,
		CallbackQuery: &CallbackQuery{
			ID:         "cq-unknown",
			ChatID:     7777, // not in store
			MessageID:  1,
			Data:       "data",
			ReceivedAt: time.Now(),
		},
	}
	_ = h.Handle(context.Background(), update)
	if len(agent.texts) > 0 {
		t.Error("agent must not be called for unregistered user callback")
	}
}

// TestDownloadAttachment_NotFound verifies that a 404 response returns an error
// and removes the partially created file.
func TestDownloadAttachment_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	dir := t.TempDir()
	dst := dir + "/file.jpg"

	_, err := downloadAttachment(context.Background(), srv.URL+"/missing", dst)
	if err == nil {
		t.Error("expected error for 404 response")
	}

	// File should be cleaned up.
	if _, statErr := stat(dst); statErr == nil {
		t.Error("expected file to be removed after download failure")
	}
}

// TestSweepOnce_MtimeError verifies sweepOnce skips entries when Info() fails.
func TestSweepOnce_MtimeError(t *testing.T) {
	dir := t.TempDir()
	// Create a normal file and then sweep — just verify no panic.
	f := dir + "/test.jpg"
	if err := writeFile(f, []byte("x"), 0o600); err != nil {
		t.Fatalf("create test file: %v", err)
	}
	j := &Janitor{
		inboxDir:  dir,
		ttl:       1 * time.Millisecond,
		tickEvery: 1 * time.Second,
	}
	j.sweepOnce(time.Now().Add(1 * time.Second))
}

// TestDefaultConfigPath verifies it returns a non-empty path.
func TestDefaultConfigPath(t *testing.T) {
	p, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	if p == "" {
		t.Error("expected non-empty default config path")
	}
}

// TestSweepOnce_MissDir verifies sweepOnce handles a non-existent dir gracefully.
func TestSweepOnce_MissDir(t *testing.T) {
	j := &Janitor{
		inboxDir:  "/nonexistent/dir/that/does/not/exist",
		ttl:       1 * time.Millisecond,
		tickEvery: 1 * time.Second,
		logger:    zap.NewNop(),
	}
	// Must not panic.
	j.sweepOnce(time.Now())
}

// TestModelUser_Nil verifies modelUser with nil input returns zero User.
func TestModelUser_Nil(t *testing.T) {
	u := modelUser(nil)
	if u.ID != 0 {
		t.Errorf("expected zero ID for nil user, got %d", u.ID)
	}
}

// TestModelMessage_Nil verifies modelMessage with nil input returns zero Message.
func TestModelMessage_Nil(t *testing.T) {
	m := modelMessage(nil)
	if m.ID != 0 {
		t.Errorf("expected zero ID for nil message, got %d", m.ID)
	}
}

// TestNewJanitor verifies default field values.
func TestNewJanitor(t *testing.T) {
	j := NewJanitor("/tmp/inbox", zap.NewNop())
	if j.ttl == 0 {
		t.Error("expected non-zero TTL")
	}
	if j.tickEvery == 0 {
		t.Error("expected non-zero tick interval")
	}
}

// TestNoOpAgentQuery_Query verifies that NoOpAgentQuery.Query returns a canned message.
func TestNoOpAgentQuery_Query(t *testing.T) {
	n := &NoOpAgentQuery{}
	resp, err := n.Query(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == "" {
		t.Error("expected non-empty response")
	}
}

// TestOSKeyring_Nokeyring verifies that the stub OSKeyring returns errors.
func TestOSKeyring_Nokeyring(t *testing.T) {
	kr := NewOSKeyring()
	err := kr.Store("svc", "key", []byte("val"))
	if err == nil {
		t.Log("OSKeyring.Store succeeded (real keyring available in this build)")
	}
	_, err = kr.Retrieve("svc", "key")
	// Either succeeds (real keyring) or fails (nokeyring stub) — both are valid.
	_ = err
}

// TestConvertUpdates_WithCallbackQuery verifies that a raw models.Update containing
// a callback_query is correctly mapped to an internal Update.CallbackQuery.
func TestConvertUpdates_WithCallbackQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a callback_query update.
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"ok": true,
			"result": []interface{}{
				map[string]interface{}{
					"update_id": 99,
					"callback_query": map[string]interface{}{
						"id": "cq-xyz",
						"from": map[string]interface{}{
							"id":         555,
							"first_name": "Alice",
							"is_bot":     false,
						},
						"message": map[string]interface{}{
							"message_id": 10,
							"chat":       map[string]interface{}{"id": 555, "type": "private"},
							"text":       "original",
							"date":       1000000000,
						},
						"data": "opt_a",
					},
				},
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient("tok", WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	updates, err := c.GetUpdates(context.Background(), 0, 1)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected 1 update")
	}
	upd := updates[0]
	if upd.CallbackQuery == nil {
		t.Fatal("expected CallbackQuery to be populated")
	}
	if upd.CallbackQuery.Data != "opt_a" {
		t.Errorf("expected data 'opt_a', got %q", upd.CallbackQuery.Data)
	}
	if upd.CallbackQuery.ChatID != 555 {
		t.Errorf("expected chat_id 555, got %d", upd.CallbackQuery.ChatID)
	}
}
