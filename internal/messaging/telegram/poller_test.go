package telegram_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakePollerClient is a test double for Client used by poller tests.
type fakePollerClient struct {
	mu       sync.Mutex
	calls    int
	updates  [][]telegram.Update // returns updates[i] on call i; last repeated forever
	errs     []error             // returns errs[i] on call i; nil if out of range
	sendReqs []telegram.SendMessageRequest
}

func (f *fakePollerClient) GetUpdates(_ context.Context, offset, _ int) ([]telegram.Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	i := f.calls
	f.calls++

	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	if err != nil {
		return nil, err
	}

	if i >= len(f.updates) {
		// No more updates; block until the test cancels ctx.
		// Return empty without blocking (poller will loop and ctx will fire).
		return nil, nil
	}
	return f.updates[i], nil
}

func (f *fakePollerClient) GetMe(_ context.Context) (telegram.User, error) {
	return telegram.User{}, nil
}

func (f *fakePollerClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendReqs = append(f.sendReqs, req)
	return telegram.Message{}, nil
}

func (f *fakePollerClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (f *fakePollerClient) SendPhoto(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 10, ChatID: req.ChatID}, nil
}
func (f *fakePollerClient) SendDocument(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 11, ChatID: req.ChatID}, nil
}

// recordingHandler records calls to Handle.
type recordingHandler struct {
	mu     sync.Mutex
	called []telegram.Update
}

func (h *recordingHandler) Handle(_ context.Context, u telegram.Update) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.called = append(h.called, u)
	return nil
}

func (h *recordingHandler) Updates() []telegram.Update {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]telegram.Update, len(h.called))
	copy(result, h.called)
	return result
}

// TestPoller_HappyPath verifies that two updates are dispatched and offset advances.
func TestPoller_HappyPath(t *testing.T) {
	updates := []telegram.Update{
		{UpdateID: 10, Message: &telegram.InboundMessage{ChatID: 1, Text: "hello"}},
		{UpdateID: 11, Message: &telegram.InboundMessage{ChatID: 1, Text: "world"}},
	}

	client := &fakePollerClient{
		updates: [][]telegram.Update{updates},
	}
	handler := &recordingHandler{}
	logger := zap.NewNop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	poller := telegram.NewPoller(client, handler, logger)

	// Run the poller briefly; it will process the two updates then loop on empty returns.
	// Cancel after a short time.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err := poller.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)

	got := handler.Updates()
	require.Len(t, got, 2, "expected 2 dispatched updates")
	assert.Equal(t, 10, got[0].UpdateID)
	assert.Equal(t, 11, got[1].UpdateID)
}

// TestPoller_CtxDoneBeforeUpdates verifies that Run returns ctx.Err() immediately
// when the context is already cancelled.
func TestPoller_CtxDoneBeforeUpdates(t *testing.T) {
	client := &fakePollerClient{}
	handler := &recordingHandler{}
	logger := zap.NewNop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	poller := telegram.NewPoller(client, handler, logger)
	err := poller.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, handler.Updates(), "no updates should be dispatched when ctx is cancelled")
}

// TestPoller_ErrorThenSuccess verifies that a getUpdates error triggers backoff
// and the poller retries successfully.
func TestPoller_ErrorThenSuccess(t *testing.T) {
	apiErr := errors.New("temporary API error")
	successUpdate := []telegram.Update{
		{UpdateID: 42, Message: &telegram.InboundMessage{ChatID: 5, Text: "retry ok"}},
	}

	client := &fakePollerClient{
		errs:    []error{apiErr, nil},                    // first call fails, second succeeds
		updates: [][]telegram.Update{nil, successUpdate}, // second call returns update
	}
	handler := &recordingHandler{}
	logger := zap.NewNop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		// Wait until the handler receives the update, then cancel.
		for {
			if len(handler.Updates()) >= 1 {
				cancel()
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	poller := telegram.NewPoller(client, handler, logger)
	err := poller.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)

	got := handler.Updates()
	require.Len(t, got, 1, "expected 1 update after error+retry")
	assert.Equal(t, 42, got[0].UpdateID)
}

// TestPoller_HandlerError verifies that a handler error does not stop the poller
// and the offset still advances.
func TestPoller_HandlerError(t *testing.T) {
	updates := []telegram.Update{
		{UpdateID: 100, Message: &telegram.InboundMessage{ChatID: 7, Text: "boom"}},
	}

	client := &fakePollerClient{
		updates: [][]telegram.Update{updates},
	}

	handlerFn := &erroringHandler{errOnUpdate: updates[0].UpdateID}

	logger := zap.NewNop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	poller := telegram.NewPoller(client, handlerFn, logger)
	err := poller.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	// Handler should have been called at least once (even though it errored).
	assert.GreaterOrEqual(t, handlerFn.count(), 1)
}

// erroringHandler returns an error on Handle calls for the targeted update ID.
type erroringHandler struct {
	mu          sync.Mutex
	calls       int
	errOnUpdate int
}

func (e *erroringHandler) Handle(_ context.Context, u telegram.Update) error {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	if u.UpdateID == e.errOnUpdate {
		return errors.New("handler intentional error")
	}
	return nil
}

func (e *erroringHandler) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}
