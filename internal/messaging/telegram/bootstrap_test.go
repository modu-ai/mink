package telegram_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// bootstrapClient is a test client that returns one update then blocks.
type bootstrapClient struct {
	mu     sync.Mutex
	served bool
	echoed bool
	done   chan struct{} // closed when echo is observed
}

func newBootstrapClient() *bootstrapClient {
	return &bootstrapClient{done: make(chan struct{})}
}

func (b *bootstrapClient) GetMe(_ context.Context) (telegram.User, error) {
	return telegram.User{ID: 1, Username: "bot"}, nil
}

func (b *bootstrapClient) GetUpdates(_ context.Context, _ int, _ int) ([]telegram.Update, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.served {
		b.served = true
		return []telegram.Update{
			{UpdateID: 1, Message: &telegram.InboundMessage{ChatID: 99, Text: "ping"}},
		}, nil
	}
	return nil, nil
}

func (b *bootstrapClient) SendMessage(_ context.Context, req telegram.SendMessageRequest) (telegram.Message, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.echoed && req.Text == "ping" {
		b.echoed = true
		close(b.done)
	}
	return telegram.Message{ID: 1, ChatID: req.ChatID, Text: req.Text}, nil
}

// TestStart_EchoRoundTrip verifies that Start wires handler+poller and an
// inbound update produces a SendMessage echo call before ctx is cancelled.
func TestStart_EchoRoundTrip(t *testing.T) {
	client := newBootstrapClient()
	cfg := &telegram.Config{
		BotUsername: "testbot",
		Mode:        "polling",
	}
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
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

	// Wait for the echo to be delivered or timeout.
	select {
	case <-client.done:
		cancel() // success path
	case <-ctx.Done():
		t.Fatal("timeout waiting for echo round-trip")
	}

	wg.Wait()
	assert.ErrorIs(t, runErr, context.Canceled)
}

// TestStart_NilClient verifies that Start returns an error immediately for nil Client.
func TestStart_NilClient(t *testing.T) {
	deps := telegram.Deps{
		Config: &telegram.Config{Mode: "polling"},
		Client: nil,
	}
	err := telegram.Start(context.Background(), deps)
	require.Error(t, err)
	assert.ErrorContains(t, err, "nil Client")
}

// TestStart_NilConfig verifies that Start returns an error immediately for nil Config.
func TestStart_NilConfig(t *testing.T) {
	deps := telegram.Deps{
		Config: nil,
		Client: &bootstrapClient{done: make(chan struct{})},
	}
	err := telegram.Start(context.Background(), deps)
	require.Error(t, err)
	assert.ErrorContains(t, err, "nil Config")
}

// TestStart_NilStore_FallsBackToEcho verifies that Start falls back to the
// EchoHandler when the Store dep is nil (backward compat with P1).
func TestStart_NilStore_FallsBackToEcho(t *testing.T) {
	client := newBootstrapClient()
	cfg := &telegram.Config{
		BotUsername: "testbot",
		Mode:        "polling",
	}
	// Partial deps — Store is nil → EchoHandler path.
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
		Logger: zap.NewNop(),
		// Store/Audit/Agent intentionally nil
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
		t.Fatal("timeout waiting for echo round-trip in partial-deps fallback")
	}

	wg.Wait()
	assert.ErrorIs(t, runErr, context.Canceled)
}

// TestStart_GracefulShutdown verifies that Start returns context.Canceled when
// ctx is cancelled before any updates arrive.
func TestStart_GracefulShutdown(t *testing.T) {
	client := newBootstrapClient()
	cfg := &telegram.Config{
		BotUsername: "testbot",
		Mode:        "polling",
	}
	deps := telegram.Deps{
		Config: cfg,
		Client: client,
		Logger: zap.NewNop(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := telegram.Start(ctx, deps)
	assert.ErrorIs(t, err, context.Canceled)
}
