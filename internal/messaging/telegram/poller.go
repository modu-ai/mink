package telegram

import (
	"context"
	"time"

	"go.uber.org/zap"
)

const (
	// pollTimeoutSec is the long-poll timeout passed to the Telegram API.
	// The HTTP client must have a higher timeout than this value.
	pollTimeoutSec = 30

	// backoffBase is the initial backoff duration after a getUpdates error.
	backoffBase = 2 * time.Second

	// backoffCap is the maximum backoff duration.
	backoffCap = 30 * time.Second
)

// Poller drives the long-polling loop, fetching Updates from the Telegram API
// and dispatching each to a Handler.
//
// @MX:WARN: [AUTO] Poller.Run owns a blocking loop with exponential backoff and
// external I/O. Complexity >= 15 due to error/backoff/context branches.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P1; long-poll loop must handle API
// errors gracefully without spinning; complexity is inherent to the pattern.
type Poller struct {
	client  Client
	handler Handler
	offset  int
	logger  *zap.Logger
}

// NewPoller creates a Poller that fetches updates from client and dispatches
// them to handler. Logger is used for error and info messages.
func NewPoller(client Client, handler Handler, logger *zap.Logger) *Poller {
	return &Poller{
		client:  client,
		handler: handler,
		logger:  logger,
	}
}

// Run blocks until ctx is cancelled. It fetches updates in a long-polling loop,
// dispatches each to the Handler, and advances the offset. On getUpdates error
// it applies exponential backoff (2s/4s/8s/cap=30s).
//
// @MX:ANCHOR: [AUTO] Poller.Run is the daemon polling entry point.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P1; fan_in via bootstrap.Start, tests.
func (p *Poller) Run(ctx context.Context) error {
	backoff := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Apply backoff after errors.
		if backoff > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		// @MX:NOTE: [AUTO] offset is the Telegram Update ID watermark. We pass
		// offset+1 on subsequent calls to avoid re-delivering the same update.
		updates, err := p.client.GetUpdates(ctx, p.offset, pollTimeoutSec)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			p.logger.Warn("getUpdates error, retrying", zap.Error(err), zap.Duration("backoff", nextBackoff(backoff)))
			backoff = nextBackoff(backoff)
			continue
		}

		// Reset backoff on success.
		backoff = 0

		for _, upd := range updates {
			if err := p.handler.Handle(ctx, upd); err != nil {
				// Handler errors are best-effort; log and continue.
				// Advance offset regardless to avoid reprocessing.
				p.logger.Warn("handler error", zap.Error(err), zap.Int("update_id", upd.UpdateID))
			}
			// Advance offset past this update.
			if upd.UpdateID >= p.offset {
				p.offset = upd.UpdateID + 1
			}
		}
	}
}

// nextBackoff returns the next exponential backoff duration, capped at backoffCap.
func nextBackoff(current time.Duration) time.Duration {
	if current == 0 {
		return backoffBase
	}
	next := current * 2
	if next > backoffCap {
		return backoffCap
	}
	return next
}
