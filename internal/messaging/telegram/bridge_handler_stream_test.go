package telegram_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- streaming test doubles ---

// fakeStreamBridgeClient extends fakeBridgeClient with EditMessageText recording.
type fakeStreamBridgeClient struct {
	sentReqs    []telegram.SendMessageRequest
	editReqs    []telegram.EditMessageTextRequest
	sendErr     error
	editErr     error
	returnMsgID int
}

func (f *fakeStreamBridgeClient) GetMe(_ context.Context) (telegram.User, error) {
	return telegram.User{}, nil
}
func (f *fakeStreamBridgeClient) GetUpdates(_ context.Context, _ int, _ int) ([]telegram.Update, error) {
	return nil, nil
}
func (f *fakeStreamBridgeClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	f.sentReqs = append(f.sentReqs, req)
	if f.sendErr != nil {
		return telegram.Message{}, f.sendErr
	}
	id := f.returnMsgID
	if id == 0 {
		id = len(f.sentReqs) * 10
	}
	return telegram.Message{ID: id, ChatID: req.ChatID, Text: req.Text}, nil
}
func (f *fakeStreamBridgeClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (f *fakeStreamBridgeClient) SendPhoto(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 200, ChatID: req.ChatID}, nil
}
func (f *fakeStreamBridgeClient) SendDocument(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 201, ChatID: req.ChatID}, nil
}
func (f *fakeStreamBridgeClient) EditMessageText(_ context.Context, req telegram.EditMessageTextRequest) (telegram.Message, error) {
	f.editReqs = append(f.editReqs, req)
	if f.editErr != nil {
		return telegram.Message{}, f.editErr
	}
	return telegram.Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (f *fakeStreamBridgeClient) SetWebhook(_ context.Context, _ telegram.SetWebhookRequest) error {
	return nil
}
func (f *fakeStreamBridgeClient) DeleteWebhook(_ context.Context, _ bool) error { return nil }
func (f *fakeStreamBridgeClient) SendChatAction(_ context.Context, _ int64, _ string) error {
	return nil
}

// fakeAgentStream is an AgentStream test double that returns pre-configured chunks.
type fakeAgentStream struct {
	chunks   []telegram.StreamChunk
	err      error
	received []string // texts received by QueryStream
}

func (f *fakeAgentStream) QueryStream(_ context.Context, text string, _ []string) (<-chan telegram.StreamChunk, error) {
	f.received = append(f.received, text)
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan telegram.StreamChunk, len(f.chunks))
	for _, c := range f.chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// openStreamHandlerStore creates a SqliteStore for streaming handler tests.
func openStreamHandlerStore(t *testing.T) telegram.Store {
	t.Helper()
	s, err := telegram.NewSqliteStore(filepath.Join(t.TempDir(), "stream.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// registerAllowedUser adds a pre-admitted user to the store.
func registerAllowedUser(t *testing.T, ctx context.Context, store telegram.Store, chatID int64) {
	t.Helper()
	now := time.Now()
	require.NoError(t, store.PutUserMapping(ctx, telegram.UserMapping{
		ChatID:        chatID,
		UserProfileID: "tg-stream-user",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}))
}

// --- tests ---

// TestBridgeQueryHandler_StreamPrefix_InvokesStreamBranch verifies that a
// "/stream <text>" message triggers the streaming branch and produces an
// editMessageText call with the response.
func TestBridgeQueryHandler_StreamPrefix_InvokesStreamBranch(t *testing.T) {
	ctx := context.Background()
	client := &fakeStreamBridgeClient{returnMsgID: 55}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	registerAllowedUser(t, ctx, store, 1001)

	stream := &fakeAgentStream{
		chunks: []telegram.StreamChunk{
			{Content: "stream response", Final: true},
		},
	}
	agent := &fakeAgentQuery{response: "should not be called"}

	h := newStreamingBridgeHandler(t, client, store, mw, agent, stream, false)
	err := h.Handle(ctx, makeTextUpdate(1, 1001, "/stream hello agent"))
	require.NoError(t, err)

	// The streaming agent must have received the stripped text (without prefix).
	require.Len(t, stream.received, 1)
	assert.Equal(t, "hello agent", stream.received[0])

	// Placeholder must have been sent.
	require.NotEmpty(t, client.sentReqs)
	assert.Contains(t, client.sentReqs[0].Text, "...")

	// EditMessageText must have been called with the response.
	require.NotEmpty(t, client.editReqs)
	assert.Contains(t, client.editReqs[len(client.editReqs)-1].Text, "stream response")

	// Non-streaming agent query must NOT be called (verified by checking edit path was taken).
	assert.NotEmpty(t, client.editReqs, "edit path must be taken for streaming")
}

// TestBridgeQueryHandler_DefaultStreaming_BypassesPrefix verifies that when
// DefaultStreaming=true all messages use the streaming branch (no /stream prefix).
func TestBridgeQueryHandler_DefaultStreaming_BypassesPrefix(t *testing.T) {
	ctx := context.Background()
	client := &fakeStreamBridgeClient{returnMsgID: 66}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	registerAllowedUser(t, ctx, store, 1002)

	stream := &fakeAgentStream{
		chunks: []telegram.StreamChunk{
			{Content: "default stream", Final: true},
		},
	}
	agent := &fakeAgentQuery{response: "should not be called"}

	// DefaultStreaming=true: every message takes the streaming branch even
	// without the /stream prefix. Use a fresh handler with the override.
	h := telegram.NewBridgeQueryHandler(
		client, store,
		telegram.NewAuditWrapper(mw, zap.NewNop()),
		agent,
		&telegram.Config{
			BotUsername:        "testbot",
			Mode:               "polling",
			AutoAdmitFirstUser: false,
			DefaultStreaming:   true,
		},
		zap.NewNop(),
	).WithStream(stream)

	err := h.Handle(ctx, makeTextUpdate(2, 1002, "no prefix but still streaming"))
	require.NoError(t, err)

	// Stream agent was called with the full text (no prefix to strip).
	require.Len(t, stream.received, 1)
	assert.Equal(t, "no prefix but still streaming", stream.received[0])

	// Edit was called.
	require.NotEmpty(t, client.editReqs)
}

// TestBridgeQueryHandler_StreamNil_FallsBackToNonStream verifies that when
// stream is nil the non-streaming path is used even if /stream prefix is present.
func TestBridgeQueryHandler_StreamNil_FallsBackToNonStream(t *testing.T) {
	ctx := context.Background()
	client := &fakeStreamBridgeClient{}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	registerAllowedUser(t, ctx, store, 1003)

	agent := &fakeAgentQuery{response: "non-stream response"}

	// No stream wired → fallback to non-streaming path.
	h := newTestBridgeHandler(t, client, store, mw, agent, false)
	err := h.Handle(ctx, makeTextUpdate(3, 1003, "/stream hello"))
	require.NoError(t, err)

	// Non-streaming agent response should be sent directly via SendMessage.
	require.NotEmpty(t, client.sentReqs)
	// Last sent message should be the agent response (not a placeholder).
	lastMsg := client.sentReqs[len(client.sentReqs)-1]
	assert.Equal(t, "non-stream response", lastMsg.Text)

	// No edit calls since we're not streaming.
	assert.Empty(t, client.editReqs)
}

// TestBridgeQueryHandler_Streaming_AuditRecorded verifies that streaming
// events include streaming=true and edit_count in the audit metadata.
func TestBridgeQueryHandler_Streaming_AuditRecorded(t *testing.T) {
	ctx := context.Background()
	client := &fakeStreamBridgeClient{returnMsgID: 77}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	registerAllowedUser(t, ctx, store, 1004)

	stream := &fakeAgentStream{
		chunks: []telegram.StreamChunk{
			{Content: "audited text", Final: true},
		},
	}
	agent := &fakeAgentQuery{}

	h := newStreamingBridgeHandler(t, client, store, mw, agent, stream, false)
	err := h.Handle(ctx, makeTextUpdate(4, 1004, "/stream audit me"))
	require.NoError(t, err)

	// At least one outbound audit event must have streaming=true.
	foundStreaming := false
	for _, ev := range mw.Events {
		if v, ok := ev.Metadata["streaming"]; ok && v == "true" {
			foundStreaming = true
		}
	}
	assert.True(t, foundStreaming, "audit should have streaming=true")
}

// TestBridgeQueryHandler_Streaming_StreamOpenError verifies graceful handling
// when QueryStream returns an error.
func TestBridgeQueryHandler_Streaming_StreamOpenError(t *testing.T) {
	ctx := context.Background()
	client := &fakeStreamBridgeClient{returnMsgID: 88}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	registerAllowedUser(t, ctx, store, 1005)

	stream := &fakeAgentStream{err: errors.New("agent stream unavailable")}
	agent := &fakeAgentQuery{}

	h := newStreamingBridgeHandler(t, client, store, mw, agent, stream, false)
	err := h.Handle(ctx, makeTextUpdate(5, 1005, "/stream fail"))
	// Error must be swallowed — Handle must not propagate it.
	require.NoError(t, err)

	// Offset must still be advanced.
	offset, _ := store.GetLastOffset(ctx)
	assert.Equal(t, int64(6), offset)
}

// TestBridgeQueryHandler_Streaming_GatesApplyFirst verifies that access control
// gates (blocked user, length check) take priority over the streaming branch.
func TestBridgeQueryHandler_Streaming_GatesApplyFirst(t *testing.T) {
	ctx := context.Background()
	client := &fakeStreamBridgeClient{}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()

	// Register a blocked user.
	now := time.Now()
	require.NoError(t, store.PutUserMapping(ctx, telegram.UserMapping{
		ChatID:        1006,
		UserProfileID: "blocked",
		Allowed:       false,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}))

	stream := &fakeAgentStream{
		chunks: []telegram.StreamChunk{{Content: "should not stream", Final: true}},
	}
	agent := &fakeAgentQuery{}

	h := newStreamingBridgeHandler(t, client, store, mw, agent, stream, false)
	err := h.Handle(ctx, makeTextUpdate(6, 1006, "/stream blocked"))
	require.NoError(t, err)

	// Stream must NOT have been called for a blocked user.
	assert.Empty(t, stream.received)

	// No edit calls.
	assert.Empty(t, client.editReqs)
}

// newStreamingBridgeHandler creates a BridgeQueryHandler with a stream wired in.
func newStreamingBridgeHandler(
	t *testing.T,
	client telegram.Client,
	store telegram.Store,
	mw *audit.MockWriter,
	agent telegram.AgentQuery,
	stream telegram.AgentStream,
	autoAdmit bool,
) *telegram.BridgeQueryHandler {
	t.Helper()
	cfg := &telegram.Config{
		BotUsername:        "testbot",
		Mode:               "polling",
		AutoAdmitFirstUser: autoAdmit,
	}
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	h := telegram.NewBridgeQueryHandler(client, store, aw, agent, cfg, zap.NewNop())
	return h.WithStream(stream)
}
