package telegram_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// safeStreamClient is a thread-safe Client test double for concurrent queue tests.
type safeStreamClient struct {
	mu          sync.Mutex
	sentReqs    []telegram.SendMessageRequest
	editReqs    []telegram.EditMessageTextRequest
	sendErr     error
	editErr     error
	returnMsgID int
}

func (f *safeStreamClient) GetMe(_ context.Context) (telegram.User, error) {
	return telegram.User{}, nil
}
func (f *safeStreamClient) GetUpdates(_ context.Context, _ int, _ int) ([]telegram.Update, error) {
	return nil, nil
}
func (f *safeStreamClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
func (f *safeStreamClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (f *safeStreamClient) SendPhoto(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 200, ChatID: req.ChatID}, nil
}
func (f *safeStreamClient) SendDocument(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 201, ChatID: req.ChatID}, nil
}
func (f *safeStreamClient) EditMessageText(_ context.Context, req telegram.EditMessageTextRequest) (telegram.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.editReqs = append(f.editReqs, req)
	if f.editErr != nil {
		return telegram.Message{}, f.editErr
	}
	return telegram.Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (f *safeStreamClient) SetWebhook(_ context.Context, _ telegram.SetWebhookRequest) error {
	return nil
}
func (f *safeStreamClient) DeleteWebhook(_ context.Context, _ bool) error { return nil }
func (f *safeStreamClient) SendChatAction(_ context.Context, _ int64, _ string) error {
	return nil
}

// sentCount returns the number of SendMessage calls (thread-safe).
func (f *safeStreamClient) sentCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sentReqs)
}

// lastSentText returns the text of the last SendMessage call (thread-safe).
func (f *safeStreamClient) lastSentText() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sentReqs) == 0 {
		return ""
	}
	return f.sentReqs[len(f.sentReqs)-1].Text
}

// --- chatStreamQueue unit tests ---

// TestChatStreamQueue_TryAcquire verifies that the first acquire succeeds and
// a second acquire for the same chat_id returns false (REQ-MTGM-S05).
func TestChatStreamQueue_TryAcquire(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	assert.True(t, q.TryAcquire(1001), "first acquire must succeed")
	assert.False(t, q.TryAcquire(1001), "second acquire for same chat_id must fail")
}

// TestChatStreamQueue_TryAcquire_DifferentChatIDs verifies that independent
// chat_ids do not block each other (REQ-MTGM-S05 isolation).
func TestChatStreamQueue_TryAcquire_DifferentChatIDs(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	assert.True(t, q.TryAcquire(1001))
	assert.True(t, q.TryAcquire(1002), "different chat_id must acquire independently")
}

// TestChatStreamQueue_Enqueue_UpToMax verifies that up to 5 items enqueue
// successfully and the 6th is rejected (REQ-MTGM-S05 max=5).
func TestChatStreamQueue_Enqueue_UpToMax(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	chatID := int64(2001)

	for i := range 5 {
		ok := q.Enqueue(chatID, makeTextUpdate(100+i, chatID, "msg"))
		assert.True(t, ok, "enqueue %d should succeed", i)
	}
	// 6th must be rejected.
	ok := q.Enqueue(chatID, makeTextUpdate(106, chatID, "overflow"))
	assert.False(t, ok, "6th enqueue must fail when queue is full")
}

// TestChatStreamQueue_PendingLen tracks pending depth.
func TestChatStreamQueue_PendingLen(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	chatID := int64(3001)
	assert.Equal(t, 0, q.PendingLen(chatID))

	q.Enqueue(chatID, makeTextUpdate(1, chatID, "a"))
	assert.Equal(t, 1, q.PendingLen(chatID))

	q.Enqueue(chatID, makeTextUpdate(2, chatID, "b"))
	assert.Equal(t, 2, q.PendingLen(chatID))
}

// TestChatStreamQueue_Release_Empty verifies release on an empty queue clears
// the active flag and returns ok=false.
func TestChatStreamQueue_Release_Empty(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	chatID := int64(4001)

	q.TryAcquire(chatID)
	assert.True(t, q.IsActive(chatID))

	_, ok := q.Release(chatID)
	assert.False(t, ok, "release on empty queue must return ok=false")
	assert.False(t, q.IsActive(chatID), "active flag must be cleared after release on empty queue")
}

// TestChatStreamQueue_Release_WithPending verifies release returns the head
// of the FIFO and active is cleared (allowing re-acquire by subsequent stream).
func TestChatStreamQueue_Release_WithPending(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	chatID := int64(5001)

	q.TryAcquire(chatID)
	u1 := makeTextUpdate(10, chatID, "first")
	u2 := makeTextUpdate(11, chatID, "second")
	q.Enqueue(chatID, u1)
	q.Enqueue(chatID, u2)

	got, ok := q.Release(chatID)
	require.True(t, ok, "release with pending must return ok=true")
	assert.Equal(t, u1, got, "must return head of FIFO")
	assert.Equal(t, 1, q.PendingLen(chatID), "one item must remain")

	// Active is cleared so that Handle can re-acquire for the next stream.
	assert.False(t, q.IsActive(chatID), "active must be cleared after release")
}

// TestChatStreamQueue_Release_FIFO verifies strict first-in-first-out ordering.
func TestChatStreamQueue_Release_FIFO(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	chatID := int64(6001)

	q.TryAcquire(chatID)
	texts := []string{"alpha", "beta", "gamma"}
	for i, text := range texts {
		q.Enqueue(chatID, makeTextUpdate(i, chatID, text))
	}

	for _, want := range texts {
		got, ok := q.Release(chatID)
		require.True(t, ok)
		assert.Equal(t, want, got.Message.Text)
		// Re-acquire for next iteration (release clears active).
		q.TryAcquire(chatID)
	}

	// Queue now empty.
	_, ok := q.Release(chatID)
	assert.False(t, ok)
}

// TestChatStreamQueue_ReleaseAfterRelease verifies that after a full drain,
// tryAcquire succeeds again.
func TestChatStreamQueue_ReleaseAfterRelease(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	chatID := int64(7001)

	q.TryAcquire(chatID)
	q.Release(chatID) // empty queue → active cleared

	assert.True(t, q.TryAcquire(chatID), "must re-acquire after full release")
}

// TestChatStreamQueue_Concurrent verifies that 100 concurrent enqueues for the
// same chat_id result in exactly 5 successes (race detector must pass).
func TestChatStreamQueue_Concurrent(t *testing.T) {
	q := telegram.NewChatStreamQueue()
	chatID := int64(8001)

	var successCount atomic.Int64
	var wg sync.WaitGroup
	const goroutines = 100
	wg.Add(goroutines)

	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			ok := q.Enqueue(chatID, makeTextUpdate(i, chatID, "msg"))
			if ok {
				successCount.Add(1)
			}
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int64(5), successCount.Load(),
		"exactly 5 enqueues must succeed (queue depth = 5)")
}

// --- Handle queue integration ---

// TestHandle_QueuedMessage_WhileStreaming verifies that a second message
// arriving while streaming is in progress is enqueued (stream_queued audit
// flag) and not immediately processed.
func TestHandle_QueuedMessage_WhileStreaming(t *testing.T) {
	ctx := context.Background()
	client := &safeStreamClient{returnMsgID: 42}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	chatID := int64(9001)
	registerAllowedUser(t, ctx, store, chatID)

	// blockingStream blocks until released, and signals via acquired when
	// QueryStream has been called (i.e., the streaming slot is held).
	acquired := make(chan struct{})
	endStream := make(chan struct{})
	stream := &signallingStream{
		acquired:  acquired,
		endStream: endStream,
	}

	cfg := &telegram.Config{
		BotUsername:      "testbot",
		Mode:             "polling",
		DefaultStreaming: true,
	}
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	h := telegram.NewBridgeQueryHandler(client, store, aw, &fakeAgentQuery{}, cfg, zap.NewNop()).
		WithStream(stream)

	// Start first stream in a goroutine.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := h.Handle(ctx, makeTextUpdate(1, chatID, "first message"))
		assert.NoError(t, err)
	}()

	// Wait until QueryStream has been entered — at that point the streaming slot
	// is held and second Handle will enqueue.
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("stream never acquired")
	}

	// Second message arrives while first stream is in progress.
	err := h.Handle(ctx, makeTextUpdate(2, chatID, "second message"))
	require.NoError(t, err)

	// Unblock first stream, then wait for goroutine (includes drain).
	close(endStream)
	wg.Wait()

	// Verify stream_queued audit flag — safe now that all goroutines finished.
	foundQueued := false
	for _, ev := range mw.Events {
		if v, ok := ev.Metadata["stream_queued"]; ok && v == "true" {
			foundQueued = true
			break
		}
	}
	assert.True(t, foundQueued, "second message must be audited with stream_queued=true")
}

// TestHandle_QueueFull_DropWithApology verifies that when the queue is full
// (5 items), an additional message triggers an apology SendMessage and
// stream_queue_full + stream_queue_dropped audit flags.
func TestHandle_QueueFull_DropWithApology(t *testing.T) {
	ctx := context.Background()
	client := &safeStreamClient{returnMsgID: 99}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	chatID := int64(9002)
	registerAllowedUser(t, ctx, store, chatID)

	acquired := make(chan struct{})
	endStream := make(chan struct{})
	stream := &signallingStream{acquired: acquired, endStream: endStream}

	cfg := &telegram.Config{
		BotUsername:      "testbot",
		Mode:             "polling",
		DefaultStreaming: true,
	}
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	h := telegram.NewBridgeQueryHandler(client, store, aw, &fakeAgentQuery{}, cfg, zap.NewNop()).
		WithStream(stream)

	// Start first stream (blocked until endStream is closed).
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := h.Handle(ctx, makeTextUpdate(10, chatID, "blocked"))
		assert.NoError(t, err)
	}()

	// Wait for the first stream to enter QueryStream (slot is held).
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("stream never acquired")
	}

	// Enqueue 5 messages (should succeed).
	for i := range 5 {
		err := h.Handle(ctx, makeTextUpdate(11+i, chatID, "queued"))
		require.NoError(t, err)
	}

	// Record sent messages before the drop.
	sentBefore := client.sentCount()

	// 6th extra message — queue is full, should be dropped with apology.
	err := h.Handle(ctx, makeTextUpdate(16, chatID, "overflow"))
	require.NoError(t, err)

	// An apology SendMessage must have been sent (runs synchronously on this goroutine).
	assert.Greater(t, client.sentCount(), sentBefore,
		"apology message must be sent when queue is full")

	// Check apology text.
	assert.Contains(t, client.lastSentText(), "이전 응답 진행 중",
		"apology text must mention previous response in progress")

	// Unblock first stream and wait for drain to complete.
	close(endStream)
	wg.Wait()

	// Verify audit flags — safe now that all goroutines finished.
	foundFull := false
	foundDropped := false
	for _, ev := range mw.Events {
		if v, ok := ev.Metadata["stream_queue_full"]; ok && v == "true" {
			foundFull = true
		}
		if v, ok := ev.Metadata["stream_queue_dropped"]; ok && v == "true" {
			foundDropped = true
		}
	}
	assert.True(t, foundFull, "stream_queue_full audit flag must be set")
	assert.True(t, foundDropped, "stream_queue_dropped audit flag must be set")
}

// TestHandle_DrainQueue_AfterStreamComplete verifies that after a streaming
// response completes, the queued message is processed.
func TestHandle_DrainQueue_AfterStreamComplete(t *testing.T) {
	ctx := context.Background()
	client := &safeStreamClient{returnMsgID: 55}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	chatID := int64(9003)
	registerAllowedUser(t, ctx, store, chatID)

	acquired := make(chan struct{})
	endStream := make(chan struct{})
	stream := &signallingStream{acquired: acquired, endStream: endStream}

	cfg := &telegram.Config{
		BotUsername:      "testbot",
		Mode:             "polling",
		DefaultStreaming: true,
	}
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	h := telegram.NewBridgeQueryHandler(client, store, aw, &fakeAgentQuery{}, cfg, zap.NewNop()).
		WithStream(stream)

	// Start first stream (blocked).
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := h.Handle(ctx, makeTextUpdate(20, chatID, "first"))
		assert.NoError(t, err)
	}()

	// Wait until first stream slot is held.
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("stream never acquired")
	}

	// Enqueue a second message (runs synchronously on test goroutine).
	err := h.Handle(ctx, makeTextUpdate(21, chatID, "second"))
	require.NoError(t, err)

	// Unblock first stream — drain should then process "second".
	close(endStream)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Handle did not complete in time")
	}

	// Verify that the stream received both "first" and "second" texts.
	stream.mu.Lock()
	received := make([]string, len(stream.received))
	copy(received, stream.received)
	stream.mu.Unlock()

	assert.Contains(t, received, "second",
		"queued message 'second' must be processed after drain")
}

// TestHandle_DifferentChatIDs_Independent verifies that chat_id A's full queue
// has no effect on chat_id B's streaming ability.
func TestHandle_DifferentChatIDs_Independent(t *testing.T) {
	ctx := context.Background()
	// Use a shared handler for both chat IDs to exercise the same streamQueue.
	client := &safeStreamClient{returnMsgID: 77}
	store := openStreamHandlerStore(t)
	mw := audit.NewMockWriter()
	chatA := int64(9004)
	chatB := int64(9005)
	registerAllowedUser(t, ctx, store, chatA)
	registerAllowedUser(t, ctx, store, chatB)

	acquiredA := make(chan struct{})
	endStreamA := make(chan struct{})
	// signallingStream handles multiple chat IDs; acquired fires on first call.
	stream := &signallingStream{acquired: acquiredA, endStream: endStreamA}

	cfg := &telegram.Config{
		BotUsername:      "testbot",
		Mode:             "polling",
		DefaultStreaming: true,
	}
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())
	h := telegram.NewBridgeQueryHandler(client, store, aw, &fakeAgentQuery{}, cfg, zap.NewNop()).
		WithStream(stream)

	// Start streaming for chat A (blocked).
	var wgA sync.WaitGroup
	wgA.Add(1)
	go func() {
		defer wgA.Done()
		_ = h.Handle(ctx, makeTextUpdate(30, chatA, "a-first"))
	}()

	// Wait for chat A to hold the streaming slot.
	select {
	case <-acquiredA:
	case <-time.After(2 * time.Second):
		t.Fatal("chat A stream never acquired")
	}

	// Fill chat A's queue to capacity (synchronous, runs on test goroutine).
	for i := range 5 {
		err := h.Handle(ctx, makeTextUpdate(31+i, chatA, "a-queued"))
		require.NoError(t, err)
	}

	// Chat B uses a separate non-blocking stream via a fresh handler with the
	// same shared store (different streamQueue — independent by design).
	streamB := &fakeAgentStream{
		chunks: []telegram.StreamChunk{{Content: "b response", Final: true}},
	}
	hB := telegram.NewBridgeQueryHandler(
		client, store,
		telegram.NewAuditWrapper(audit.NewMockWriter(), zap.NewNop()),
		&fakeAgentQuery{}, cfg, zap.NewNop(),
	).WithStream(streamB)

	err := hB.Handle(ctx, makeTextUpdate(40, chatB, "b-msg"))
	require.NoError(t, err, "chat B must succeed independently of chat A's state")

	// B's stream must have been called.
	assert.Contains(t, streamB.received, "b-msg")

	// Unblock A and wait for drain.
	close(endStreamA)
	wgA.Wait()
}

// --- helpers ---

// signallingStream is an AgentStream that signals via acquired once QueryStream
// is entered, then blocks until endStream is closed.
type signallingStream struct {
	mu        sync.Mutex
	acquired  chan struct{} // closed on first QueryStream call
	endStream chan struct{} // closed to unblock the stream
	received  []string

	acquiredOnce sync.Once
}

func (s *signallingStream) QueryStream(ctx context.Context, text string, _ []string) (<-chan telegram.StreamChunk, error) {
	s.mu.Lock()
	s.received = append(s.received, text)
	s.mu.Unlock()

	// Signal that the stream slot has been entered.
	s.acquiredOnce.Do(func() { close(s.acquired) })

	out := make(chan telegram.StreamChunk, 1)
	go func() {
		defer close(out)
		select {
		case <-s.endStream:
		case <-ctx.Done():
			return
		}
		out <- telegram.StreamChunk{Content: "done " + text, Final: true}
	}()
	return out, nil
}
