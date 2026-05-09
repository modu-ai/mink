package telegram_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- test doubles ---

// fakeBridgeClient is a Client test double for handler tests.
type fakeBridgeClient struct {
	sentReqs  []telegram.SendMessageRequest
	sendErr   error
	returnMsg telegram.Message
}

func (f *fakeBridgeClient) GetMe(_ context.Context) (telegram.User, error) {
	return telegram.User{}, nil
}
func (f *fakeBridgeClient) GetUpdates(_ context.Context, _ int, _ int) ([]telegram.Update, error) {
	return nil, nil
}
func (f *fakeBridgeClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	f.sentReqs = append(f.sentReqs, req)
	if f.sendErr != nil {
		return telegram.Message{}, f.sendErr
	}
	msg := f.returnMsg
	if msg.ID == 0 {
		msg = telegram.Message{ID: len(f.sentReqs), ChatID: req.ChatID, Text: req.Text}
	}
	return msg, nil
}
func (f *fakeBridgeClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (f *fakeBridgeClient) SendPhoto(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 200, ChatID: req.ChatID}, nil
}
func (f *fakeBridgeClient) SendDocument(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 201, ChatID: req.ChatID}, nil
}
func (f *fakeBridgeClient) EditMessageText(_ context.Context, req telegram.EditMessageTextRequest) (telegram.Message, error) {
	return telegram.Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (f *fakeBridgeClient) SetWebhook(_ context.Context, _ telegram.SetWebhookRequest) error {
	return nil
}
func (f *fakeBridgeClient) DeleteWebhook(_ context.Context, _ bool) error             { return nil }
func (f *fakeBridgeClient) SendChatAction(_ context.Context, _ int64, _ string) error { return nil }

// fakeAgentQuery is an AgentQuery test double.
type fakeAgentQuery struct {
	response string
	err      error
	delay    time.Duration
}

func (f *fakeAgentQuery) Query(ctx context.Context, _ string, _ []string) (string, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return f.response, f.err
}

// openHandlerStore creates a SqliteStore in t.TempDir().
func openHandlerStore(t *testing.T) telegram.Store {
	t.Helper()
	s, err := telegram.NewSqliteStore(filepath.Join(t.TempDir(), "handler.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// newTestBridgeHandler creates a BridgeQueryHandler with sensible test defaults.
func newTestBridgeHandler(
	t *testing.T,
	client telegram.Client,
	store telegram.Store,
	mw *audit.MockWriter,
	agent telegram.AgentQuery,
	autoAdmit bool,
) *telegram.BridgeQueryHandler {
	t.Helper()
	cfg := &telegram.Config{
		BotUsername:        "testbot",
		Mode:               "polling",
		AutoAdmitFirstUser: autoAdmit,
	}
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	return telegram.NewBridgeQueryHandler(client, store, aw, agent, cfg, zap.NewNop())
}

// makeTextUpdate builds a minimal text Update.
func makeTextUpdate(updateID int, chatID int64, text string) telegram.Update {
	return telegram.Update{
		UpdateID: updateID,
		Message: &telegram.InboundMessage{
			ID:     updateID * 10,
			ChatID: chatID,
			Text:   text,
		},
	}
}

// --- tests ---

// TestBridgeQueryHandler_HappyPath verifies the full success path:
// mapped + allowed user receives an agent response.
func TestBridgeQueryHandler_HappyPath(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()
	agent := &fakeAgentQuery{response: "pong"}

	// Pre-register an allowed user.
	now := time.Now()
	require.NoError(t, store.PutUserMapping(ctx, telegram.UserMapping{
		ChatID:        100,
		UserProfileID: "tg-100",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}))

	h := newTestBridgeHandler(t, client, store, mw, agent, false)
	err := h.Handle(ctx, makeTextUpdate(1, 100, "ping"))
	require.NoError(t, err)

	// Response sent.
	require.Len(t, client.sentReqs, 1)
	assert.Equal(t, "pong", client.sentReqs[0].Text)

	// Two audit events: inbound + outbound.
	assert.Equal(t, 2, mw.EventCount())

	// Offset advanced.
	offset, _ := store.GetLastOffset(ctx)
	assert.Equal(t, int64(2), offset) // updateID + 1 = 2
}

// TestBridgeQueryHandler_FirstMessageGate_AutoAdmitOff verifies that an unknown
// user is sent the gate notice and no Query is performed.
func TestBridgeQueryHandler_FirstMessageGate_AutoAdmitOff(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()
	agent := &fakeAgentQuery{response: "should not be called"}

	h := newTestBridgeHandler(t, client, store, mw, agent, false /*autoAdmit=off*/)
	err := h.Handle(ctx, makeTextUpdate(5, 200, "hello"))
	require.NoError(t, err)

	// Gate notice sent.
	require.Len(t, client.sentReqs, 1)
	assert.Contains(t, client.sentReqs[0].Text, "사전 승인")

	// Placeholder row stored with Allowed=false.
	m, found, storeErr := store.GetUserMapping(ctx, 200)
	require.NoError(t, storeErr)
	require.True(t, found)
	assert.False(t, m.Allowed)

	// Offset advanced.
	offset, _ := store.GetLastOffset(ctx)
	assert.Equal(t, int64(6), offset)
}

// TestBridgeQueryHandler_FirstMessageGate_AutoAdmitOn verifies that when
// AutoAdmitFirstUser is true, an unknown user is auto-admitted, the agent is
// queried, and the response is delivered.
func TestBridgeQueryHandler_FirstMessageGate_AutoAdmitOn(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()
	agent := &fakeAgentQuery{response: "auto response"}

	h := newTestBridgeHandler(t, client, store, mw, agent, true /*autoAdmit=on*/)
	err := h.Handle(ctx, makeTextUpdate(7, 300, "first message"))
	require.NoError(t, err)

	// Agent response sent.
	require.Len(t, client.sentReqs, 1)
	assert.Equal(t, "auto response", client.sentReqs[0].Text)

	// Mapping stored with Allowed=true and AutoAdmitted=true.
	m, found, _ := store.GetUserMapping(ctx, 300)
	require.True(t, found)
	assert.True(t, m.Allowed)
	assert.True(t, m.AutoAdmitted)
}

// TestBridgeQueryHandler_BlockedUser_SilentDrop verifies that messages from
// a revoked user produce no SendMessage and no Query call.
func TestBridgeQueryHandler_BlockedUser_SilentDrop(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()

	queryCalled := false
	agent := &fakeAgentQuery{}
	agent.response = "should not be called"

	// Register user then revoke.
	now := time.Now()
	require.NoError(t, store.PutUserMapping(ctx, telegram.UserMapping{
		ChatID:        400,
		UserProfileID: "tg-400",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}))
	require.NoError(t, store.Revoke(ctx, 400))
	_ = queryCalled

	h := newTestBridgeHandler(t, client, store, mw, agent, false)
	err := h.Handle(ctx, makeTextUpdate(10, 400, "I am blocked"))
	require.NoError(t, err)

	// No messages sent.
	assert.Empty(t, client.sentReqs)

	// Audit event records the dropped_blocked flag.
	require.NotEmpty(t, mw.Events)
	found := false
	for _, ev := range mw.Events {
		if v, ok := ev.Metadata["dropped_blocked"]; ok && v == "true" {
			found = true
		}
	}
	assert.True(t, found, "audit should have dropped_blocked=true")

	// Offset must be advanced (REQ: every update must advance offset).
	offset, _ := store.GetLastOffset(ctx)
	assert.Equal(t, int64(11), offset)
}

// TestBridgeQueryHandler_OverLengthRejected verifies that messages exceeding
// 4096 characters are rejected without calling Query.
func TestBridgeQueryHandler_OverLengthRejected(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()
	agent := &fakeAgentQuery{response: "should not be called"}

	// Register an allowed user.
	now := time.Now()
	require.NoError(t, store.PutUserMapping(ctx, telegram.UserMapping{
		ChatID:        500,
		UserProfileID: "tg-500",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}))

	longText := strings.Repeat("a", 4097)
	h := newTestBridgeHandler(t, client, store, mw, agent, false)
	err := h.Handle(ctx, makeTextUpdate(11, 500, longText))
	require.NoError(t, err)

	// Rejection notice sent.
	require.Len(t, client.sentReqs, 1)
	assert.Contains(t, client.sentReqs[0].Text, "4096")

	// Audit event has length_exceeded flag.
	foundFlag := false
	for _, ev := range mw.Events {
		if v, ok := ev.Metadata["length_exceeded"]; ok && v == "true" {
			foundFlag = true
		}
	}
	assert.True(t, foundFlag)
}

// TestBridgeQueryHandler_QueryTimeout verifies that a context.DeadlineExceeded
// from the agent produces a timeout response and does not propagate the error.
func TestBridgeQueryHandler_QueryTimeout(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()

	// Agent that blocks longer than the handler timeout.
	agent := &fakeAgentQuery{delay: 35 * time.Second}

	now := time.Now()
	require.NoError(t, store.PutUserMapping(ctx, telegram.UserMapping{
		ChatID:        600,
		UserProfileID: "tg-600",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}))

	// Wrap the context with a very short timeout to trigger the internal
	// 30s deadline without waiting 30s in tests.
	shortCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	h := newTestBridgeHandler(t, client, store, mw, agent, false)
	err := h.Handle(shortCtx, makeTextUpdate(12, 600, "timeout me"))
	// Error must not propagate for timeout.
	require.NoError(t, err)

	// Timeout notice sent.
	require.Len(t, client.sentReqs, 1)
	assert.Contains(t, client.sentReqs[0].Text, "시간 초과")
}

// TestBridgeQueryHandler_QueryError verifies that a non-timeout agent error
// produces a graceful error response.
func TestBridgeQueryHandler_QueryError(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()
	agent := &fakeAgentQuery{err: errors.New("agent unavailable")}

	now := time.Now()
	require.NoError(t, store.PutUserMapping(ctx, telegram.UserMapping{
		ChatID:        700,
		UserProfileID: "tg-700",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}))

	h := newTestBridgeHandler(t, client, store, mw, agent, false)
	err := h.Handle(ctx, makeTextUpdate(13, 700, "cause error"))
	// Error response is graceful; no error propagated.
	require.NoError(t, err)

	// Error notice sent.
	require.Len(t, client.sentReqs, 1)
	assert.Contains(t, client.sentReqs[0].Text, "오류")
}

// TestBridgeQueryHandler_PutsOffsetAfterEachUpdate verifies that every
// processed update advances the stored offset.
func TestBridgeQueryHandler_PutsOffsetAfterEachUpdate(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()
	agent := &fakeAgentQuery{response: "ok"}

	now := time.Now()
	require.NoError(t, store.PutUserMapping(ctx, telegram.UserMapping{
		ChatID:        800,
		UserProfileID: "tg-800",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}))

	h := newTestBridgeHandler(t, client, store, mw, agent, false)

	for i, uid := range []int{1, 5, 10} {
		err := h.Handle(ctx, makeTextUpdate(uid, 800, fmt.Sprintf("msg %d", i)))
		require.NoError(t, err)
		offset, _ := store.GetLastOffset(ctx)
		assert.Equal(t, int64(uid+1), offset, "offset after updateID=%d", uid)
	}
}

// TestBridgeQueryHandler_NilMessage_SkipsGracefully verifies that an Update
// with no Message is silently skipped and the offset is still advanced.
func TestBridgeQueryHandler_NilMessage_SkipsGracefully(t *testing.T) {
	ctx := context.Background()
	client := &fakeBridgeClient{}
	store := openHandlerStore(t)
	mw := audit.NewMockWriter()
	agent := &fakeAgentQuery{response: "ok"}

	h := newTestBridgeHandler(t, client, store, mw, agent, false)
	err := h.Handle(ctx, telegram.Update{UpdateID: 99, Message: nil})
	require.NoError(t, err)
	assert.Empty(t, client.sentReqs)

	offset, _ := store.GetLastOffset(ctx)
	assert.Equal(t, int64(100), offset)
}
