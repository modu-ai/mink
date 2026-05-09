package telegram

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Handler dispatches an inbound Update to the appropriate action.
type Handler interface {
	Handle(ctx context.Context, update Update) error
}

// EchoHandler is a simple Handler that replies the received text back to the
// same chat. Used in P1 for basic smoke testing.
//
// @MX:ANCHOR: [AUTO] EchoHandler.Handle is the P1 inbound dispatch entry point.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P1; fan_in via Poller, bootstrap, and tests.
type EchoHandler struct {
	client Client
	logger *zap.Logger
}

// NewEchoHandler creates an EchoHandler wired to the given client and logger.
func NewEchoHandler(client Client, logger *zap.Logger) *EchoHandler {
	return &EchoHandler{client: client, logger: logger}
}

// Handle replies the inbound text back to the same chat (echo bot behaviour).
// Updates without a text message payload are silently skipped.
func (h *EchoHandler) Handle(ctx context.Context, update Update) error {
	if update.Message == nil {
		return nil
	}
	if update.Message.Text == "" {
		return nil
	}

	_, err := h.client.SendMessage(ctx, SendMessageRequest{
		ChatID: update.Message.ChatID,
		Text:   update.Message.Text,
	})
	if err != nil {
		return fmt.Errorf("echo send: %w", err)
	}
	return nil
}
