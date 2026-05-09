package telegram

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Deps bundles the runtime dependencies required to start the Telegram channel.
type Deps struct {
	Config *Config
	Client Client
	Logger *zap.Logger
}

// Start blocks until ctx is cancelled. It constructs the EchoHandler and Poller
// then runs the polling loop.
//
// Token absence is handled at the caller layer (the start subcommand); Start
// assumes deps are already fully populated.
//
// @MX:ANCHOR: [AUTO] Start is the daemon entry point for the Telegram channel.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P1; fan_in via start subcommand, bootstrap tests.
func Start(ctx context.Context, deps Deps) error {
	if deps.Client == nil {
		return fmt.Errorf("telegram: Start called with nil Client")
	}
	if deps.Config == nil {
		return fmt.Errorf("telegram: Start called with nil Config")
	}

	logger := deps.Logger
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return fmt.Errorf("telegram: create logger: %w", err)
		}
	}

	handler := NewEchoHandler(deps.Client, logger)
	poller := NewPoller(deps.Client, handler, logger)

	logger.Info("telegram channel starting", zap.String("bot", deps.Config.BotUsername))
	return poller.Run(ctx)
}
