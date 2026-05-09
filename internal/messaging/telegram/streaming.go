package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// streamingEditInterval is the minimum gap between successive editMessageText
// calls to comply with Telegram's rate limit (REQ-MTGM-E02).
const streamingEditInterval = 1 * time.Second

// streamingPlaceholder is the initial body of the placeholder message sent
// while the streaming response is being assembled.
const streamingPlaceholder = "..."

// streamingResult summarises the outcome of a streaming run for audit purposes.
type streamingResult struct {
	// Final is the complete accumulated text at stream end.
	Final string
	// EditCount is the number of successful editMessageText calls.
	EditCount int
	// TotalDuration is the elapsed time from placeholder send to stream end.
	TotalDuration time.Duration
	// Aborted is true when the context was cancelled before the stream closed.
	Aborted bool
	// PlaceholderID is the message ID of the placeholder message sent at the start.
	PlaceholderID int
}

// runStreaming drives a single streaming response:
//  1. Sends a placeholder message ("...") immediately.
//  2. Reads chunks from chunkCh, accumulating them in a buffer.
//  3. Flushes the buffer via editMessageText once per streamingEditInterval tick.
//  4. Performs a final flush when the channel closes or a Final chunk arrives.
//
// Edit failures are logged and swallowed (non-fatal). Context cancellation sets
// Aborted=true and returns ctx.Err().
//
// The silent flag sets disable_notification=true on the placeholder SendMessage
// call (REQ-MTGM-O01). editMessageText does not support disable_notification.
//
// @MX:WARN: [AUTO] runStreaming reads chunkCh in a select loop with a time.Ticker;
// it must be called with a bounded context to avoid indefinite blocking.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P4 REQ-MTGM-E02; the ticker-based edit
// loop is intentional and inherent to the Telegram rate-limit model.
func runStreaming(
	ctx context.Context,
	client Client,
	chatID int64,
	chunkCh <-chan StreamChunk,
	silent bool,
	logger *zap.Logger,
) (streamingResult, error) {
	// Send the placeholder message that will be edited as chunks arrive.
	// Apply silent flag for the initial placeholder (REQ-MTGM-O01).
	placeholder, err := client.SendMessage(ctx, SendMessageRequest{
		ChatID: chatID,
		Text:   streamingPlaceholder,
		Silent: silent,
	})
	if err != nil {
		return streamingResult{}, fmt.Errorf("telegram streaming: placeholder: %w", err)
	}

	var buf strings.Builder
	var lastSent string
	editCount := 0
	start := time.Now()
	ticker := time.NewTicker(streamingEditInterval)
	defer ticker.Stop()

	// flush sends an editMessageText call if the buffer has new content since
	// the last successful edit. Failures are logged but do not increment editCount.
	flush := func() {
		body := buf.String()
		if body == "" || body == lastSent {
			return
		}
		_, editErr := client.EditMessageText(ctx, EditMessageTextRequest{
			ChatID:    chatID,
			MessageID: placeholder.ID,
			Text:      body,
		})
		if editErr != nil {
			logger.Warn("editMessageText failed",
				zap.Error(editErr),
				zap.Int64("chat_id", chatID),
				zap.Int("message_id", placeholder.ID))
			return
		}
		lastSent = body
		editCount++
	}

	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				// Channel was closed without a Final chunk — treat as end of stream.
				flush()
				return streamingResult{
					Final:         buf.String(),
					EditCount:     editCount,
					TotalDuration: time.Since(start),
					PlaceholderID: placeholder.ID,
				}, nil
			}
			if chunk.Content != "" {
				buf.WriteString(chunk.Content)
			}
			if chunk.Final {
				flush()
				return streamingResult{
					Final:         buf.String(),
					EditCount:     editCount,
					TotalDuration: time.Since(start),
					PlaceholderID: placeholder.ID,
				}, nil
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			return streamingResult{
				Final:         buf.String(),
				EditCount:     editCount,
				TotalDuration: time.Since(start),
				Aborted:       true,
				PlaceholderID: placeholder.ID,
			}, ctx.Err()
		}
	}
}
