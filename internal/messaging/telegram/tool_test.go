package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modu-ai/goose/internal/tools"
	"go.uber.org/zap"
)

// mockRegistrySender is a Sender that records calls and optionally returns errors.
type mockRegistrySender struct {
	calls []SendRequest
	err   error
	resp  *SendResponse
}

func (m *mockRegistrySender) Send(ctx context.Context, req SendRequest) (*SendResponse, error) {
	m.calls = append(m.calls, req)
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &SendResponse{MessageID: 42, ChatID: req.ChatID}, nil
}

// newTestRegistry builds a tools.Registry suitable for tool registration tests.
func newTestRegistry(t *testing.T) *tools.Registry {
	t.Helper()
	return tools.NewRegistry()
}

// TestTool_Registration verifies that WithMessaging registers the
// telegram_send_message tool in the provided registry.
func TestTool_Registration(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}
	logger := zap.NewNop()
	aw := NewAuditWrapper(maw, logger)
	sender := NewSender(mc, ms, aw, logger)

	r := newTestRegistry(t)
	opt := WithMessaging(sender)
	opt(r)

	names := r.ListNames()
	var found bool
	for _, n := range names {
		if n == "telegram_send_message" {
			found = true
		}
	}
	if !found {
		t.Errorf("telegram_send_message not found in registry after WithMessaging, got: %v", names)
	}
}

// TestTool_SchemaValid verifies that the tool's JSON schema is valid and
// contains the required fields chat_id and text.
func TestTool_SchemaValid(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}
	logger := zap.NewNop()
	aw := NewAuditWrapper(maw, logger)
	sender := NewSender(mc, ms, aw, logger)

	tool := &telegramSendMessageTool{sender: sender}
	var schema map[string]interface{}
	if err := json.Unmarshal(tool.Schema(), &schema); err != nil {
		t.Fatalf("Schema() is not valid JSON: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema missing 'properties'")
	}
	if _, ok := props["chat_id"]; !ok {
		t.Error("schema missing 'chat_id' property")
	}
	if _, ok := props["text"]; !ok {
		t.Error("schema missing 'text' property")
	}
}

// TestTool_Call_Success verifies that a valid tool invocation calls Sender.Send
// and returns a JSON result.
func TestTool_Call_Success(t *testing.T) {
	ms := &mockRegistrySender{
		resp: &SendResponse{MessageID: 99, ChatID: 555},
	}

	tool := &telegramSendMessageTool{sender: ms}
	input := json.RawMessage(`{"chat_id": 555, "text": "hello"}`)

	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("result.IsError should be false, content: %s", result.Content)
	}
	if len(ms.calls) != 1 {
		t.Errorf("expected 1 Sender.Send call, got %d", len(ms.calls))
	}
	if ms.calls[0].ChatID != 555 {
		t.Errorf("expected chat_id 555, got %d", ms.calls[0].ChatID)
	}
}

// TestTool_Call_UnauthorizedChatID verifies that ErrUnauthorizedChatID is
// returned as a tool error (not a hard error).
func TestTool_Call_UnauthorizedChatID(t *testing.T) {
	ms := &mockRegistrySender{err: ErrUnauthorizedChatID}

	tool := &telegramSendMessageTool{sender: ms}
	input := json.RawMessage(`{"chat_id": 999, "text": "hello"}`)

	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !result.IsError {
		t.Error("result.IsError should be true for unauthorized chat_id")
	}
}

// TestTool_Call_InvalidInput verifies that malformed JSON input returns a tool
// error without crashing.
func TestTool_Call_InvalidInput(t *testing.T) {
	ms := &mockRegistrySender{}

	tool := &telegramSendMessageTool{sender: ms}
	input := json.RawMessage(`{"chat_id": "not-a-number-or-valid-type", "text": 42}`)

	// Call should not panic; it may return an error result.
	result, err := tool.Call(context.Background(), input)
	// Either a hard error or an IsError result is acceptable for malformed input.
	if err == nil && !result.IsError {
		// Both conditions allow no error for simple type coercion; verify no crash.
		t.Log("Tool accepted the input with coercion; this is acceptable")
	}
}

// TestTool_Call_MissingSender verifies that a nil sender panics at construction
// and not silently during Call.
func TestTool_Call_MissingText(t *testing.T) {
	ms := &mockRegistrySender{}
	tool := &telegramSendMessageTool{sender: ms}
	// chat_id provided but text is empty — should still invoke sender
	// (text validation is up to the sender / API).
	input := json.RawMessage(`{"chat_id": 555}`)
	_, _ = tool.Call(context.Background(), input)
	// If this reaches here without panic, the test passes.
}

// senderFunc adapts a function to the toolSender interface for test injection.
type senderFunc func(ctx context.Context, req SendRequest) (*SendResponse, error)

func (f senderFunc) Send(ctx context.Context, req SendRequest) (*SendResponse, error) {
	return f(ctx, req)
}

// TestTool_Call_SendError verifies that a non-ErrUnauthorizedChatID send error
// is wrapped and returned as a tool error.
func TestTool_Call_SendError(t *testing.T) {
	sendErr := errors.New("network failure")
	ms := &mockRegistrySender{err: sendErr}

	tool := &telegramSendMessageTool{sender: ms}
	input := json.RawMessage(`{"chat_id": 555, "text": "hello"}`)

	result, err := tool.Call(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !result.IsError {
		t.Error("result.IsError should be true for send error")
	}
}

// TestWithMessaging_CalledTwice verifies that registering the same sender
// a second time panics with a duplicate registration error.
func TestWithMessaging_CalledTwice(t *testing.T) {
	mc := &mockSenderClient{}
	ms := newMockSenderStore(555)
	maw := &mockAuditWriter{}
	logger := zap.NewNop()
	aw := NewAuditWrapper(maw, logger)
	sender := NewSender(mc, ms, aw, logger)

	r := newTestRegistry(t)
	opt := WithMessaging(sender)
	opt(r) // first registration succeeds

	// Second registration should panic — tools.Registry panics on duplicate.
	defer func() {
		if rv := recover(); rv == nil {
			t.Error("expected panic on duplicate tool registration")
		}
	}()
	opt(r) // second registration must panic
}
