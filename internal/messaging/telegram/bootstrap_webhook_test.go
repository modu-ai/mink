package telegram_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// webhookBootstrapClient is a test Client for bootstrap webhook tests.
// It records SetWebhook/DeleteWebhook calls, allows configuring errors,
// and behaves like a minimal poller client.
type webhookBootstrapClient struct {
	mu                  sync.Mutex
	setWebhookErr       error
	deleteWebhookErr    error
	setWebhookCalls     []telegram.SetWebhookRequest
	deleteWebhookCalled bool

	// poller behavior: served tracks whether a single update was sent
	served bool
	echoed bool
	done   chan struct{}
}

func newWebhookBootstrapClient() *webhookBootstrapClient {
	return &webhookBootstrapClient{done: make(chan struct{})}
}

func (c *webhookBootstrapClient) GetMe(_ context.Context) (telegram.User, error) {
	return telegram.User{ID: 1, Username: "bot"}, nil
}
func (c *webhookBootstrapClient) GetUpdates(_ context.Context, _ int, _ int) ([]telegram.Update, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.served {
		c.served = true
		return []telegram.Update{
			{UpdateID: 1, Message: &telegram.InboundMessage{ChatID: 77, Text: "hi"}},
		}, nil
	}
	return nil, nil
}
func (c *webhookBootstrapClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.echoed && req.Text == "hi" {
		c.echoed = true
		close(c.done)
	}
	return telegram.Message{ID: 1, ChatID: req.ChatID, Text: req.Text}, nil
}
func (c *webhookBootstrapClient) AnswerCallbackQuery(_ context.Context, _ string) error { return nil }
func (c *webhookBootstrapClient) SendPhoto(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 10, ChatID: req.ChatID}, nil
}
func (c *webhookBootstrapClient) SendDocument(_ context.Context, req telegram.SendMediaRequest) (telegram.Message, error) {
	return telegram.Message{ID: 11, ChatID: req.ChatID}, nil
}
func (c *webhookBootstrapClient) EditMessageText(_ context.Context, req telegram.EditMessageTextRequest) (telegram.Message, error) {
	return telegram.Message{ID: req.MessageID, ChatID: req.ChatID, Text: req.Text}, nil
}
func (c *webhookBootstrapClient) SetWebhook(_ context.Context, req telegram.SetWebhookRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setWebhookCalls = append(c.setWebhookCalls, req)
	return c.setWebhookErr
}
func (c *webhookBootstrapClient) DeleteWebhook(_ context.Context, _ bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteWebhookCalled = true
	return c.deleteWebhookErr
}
func (c *webhookBootstrapClient) SendChatAction(_ context.Context, _ int64, _ string) error {
	return nil
}

// TestStart_WebhookMode_NilMux_FallsBackToPolling verifies that webhook mode
// with a nil Mux falls back to polling and completes an echo round-trip.
func TestStart_WebhookMode_NilMux_FallsBackToPolling(t *testing.T) {
	client := newWebhookBootstrapClient()
	cfg := &telegram.Config{
		BotUsername: "testbot",
		Mode:        "webhook",
		Webhook: telegram.WebhookConfig{
			PublicURL:         "https://example.com",
			FallbackToPolling: true,
		},
	}
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
		Logger: zap.NewNop(),
		// Mux is nil — webhook mode without mux must fall back to polling
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var runErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = telegram.Start(ctx, deps)
	}()

	select {
	case <-client.done:
		cancel()
	case <-ctx.Done():
		t.Fatal("timeout: expected polling fallback echo round-trip")
	}

	wg.Wait()
	assert.ErrorIs(t, runErr, context.Canceled)
	// setWebhook must NOT have been called because mux was nil.
	assert.Empty(t, client.setWebhookCalls)
}

// TestStart_WebhookMode_SetWebhookError_FallbackEnabled verifies that when
// SetWebhook returns an error and FallbackToPolling is true, Start falls back
// to polling and completes an echo round-trip.
func TestStart_WebhookMode_SetWebhookError_FallbackEnabled(t *testing.T) {
	client := newWebhookBootstrapClient()
	client.setWebhookErr = errors.New("TLS not available")

	mux := http.NewServeMux()
	cfg := &telegram.Config{
		BotUsername: "testbot",
		Mode:        "webhook",
		Webhook: telegram.WebhookConfig{
			PublicURL:         "https://example.com",
			Secret:            "secret123",
			FallbackToPolling: true,
		},
	}
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
		Mux:    mux,
		Logger: zap.NewNop(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var runErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = telegram.Start(ctx, deps)
	}()

	select {
	case <-client.done:
		cancel()
	case <-ctx.Done():
		t.Fatal("timeout: expected polling fallback after setWebhook failure")
	}

	wg.Wait()
	assert.ErrorIs(t, runErr, context.Canceled)
}

// TestStart_WebhookMode_SetWebhookError_FallbackDisabled verifies that when
// SetWebhook returns an error and FallbackToPolling is false, Start returns
// an error.
func TestStart_WebhookMode_SetWebhookError_FallbackDisabled(t *testing.T) {
	client := newWebhookBootstrapClient()
	client.setWebhookErr = errors.New("TLS not available")

	mux := http.NewServeMux()
	cfg := &telegram.Config{
		BotUsername: "testbot",
		Mode:        "webhook",
		Webhook: telegram.WebhookConfig{
			PublicURL:         "https://example.com",
			Secret:            "secret123",
			FallbackToPolling: false,
		},
	}
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
		Mux:    mux,
		Logger: zap.NewNop(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer cancel()

	err := telegram.Start(ctx, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook")
}

// TestStart_WebhookMode_Unknown_ReturnsError verifies that an unrecognised mode
// returns an error immediately.
func TestStart_WebhookMode_Unknown_ReturnsError(t *testing.T) {
	client := newWebhookBootstrapClient()
	cfg := &telegram.Config{
		BotUsername: "testbot",
		Mode:        "grpc-push",
	}
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
		Logger: zap.NewNop(),
	}

	err := telegram.Start(context.Background(), deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mode")
}

// TestStart_WebhookMode_Success_DeletesWebhookOnShutdown verifies that when
// the webhook is registered successfully and ctx is cancelled, the shutdown
// path calls DeleteWebhook.
func TestStart_WebhookMode_Success_DeletesWebhookOnShutdown(t *testing.T) {
	client := newWebhookBootstrapClient()
	mux := http.NewServeMux()
	cfg := &telegram.Config{
		BotUsername: "testbot",
		Mode:        "webhook",
		Webhook: telegram.WebhookConfig{
			PublicURL:         "https://example.com",
			Secret:            "secretXYZ",
			FallbackToPolling: true,
		},
	}
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
		Mux:    mux,
		Logger: zap.NewNop(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	var runErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = telegram.Start(ctx, deps)
	}()

	// Allow Start to reach the webhook wait loop, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()

	// Start must return (context error or nil — either acceptable).
	_ = runErr
	// DeleteWebhook must have been called during graceful shutdown.
	client.mu.Lock()
	called := client.deleteWebhookCalled
	client.mu.Unlock()
	assert.True(t, called, "expected DeleteWebhook to be called on shutdown")
	// SetWebhook must have been called once.
	client.mu.Lock()
	setCalls := len(client.setWebhookCalls)
	client.mu.Unlock()
	assert.Equal(t, 1, setCalls)
}
