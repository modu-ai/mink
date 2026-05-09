package telegram

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Deps bundles the runtime dependencies required to start the Telegram channel.
//
// P2 expands Deps with Store, Audit, and Agent. When Store/Audit/Agent are nil,
// Start falls back to the P1 EchoHandler for backward compatibility.
type Deps struct {
	Config *Config
	Client Client
	// Store persists chat_id mappings and polling offset (P2+).
	Store Store
	// Audit records inbound/outbound messaging events (P2+).
	Audit *AuditWrapper
	// Agent forwards user messages to the Goose AI backend (P2+).
	Agent  AgentQuery
	Logger *zap.Logger
}

// NoOpAgentQuery is a placeholder AgentQuery that returns a canned message.
// It is used when the production Agent wiring is deferred (P3).
//
// @MX:TODO P3 — replace NoOpAgentQuery with a real AgentServiceClient
// wired to the goosed daemon's internal handler or loopback gRPC client.
// The daemon does not yet have a self-accessible AgentServiceClient;
// requires architectural work in SPEC-GOOSE-MSG-TELEGRAM-001 P3.
type NoOpAgentQuery struct{}

// Query returns a fixed notice that agent wiring is pending.
func (n *NoOpAgentQuery) Query(_ context.Context, _ string) (string, error) {
	return "Telegram BRIDGE wiring deferred to P3. (goosed self-gRPC client not yet wired)", nil
}

// Start blocks until ctx is cancelled. It selects the appropriate handler based
// on the completeness of Deps:
//   - Full P2 Deps (Store + Audit + Agent all non-nil) → BridgeQueryHandler
//   - Partial Deps → EchoHandler (P1 backward compatibility)
//
// Token absence is handled at the caller layer (the start subcommand); Start
// assumes deps.Client is already fully populated.
//
// @MX:ANCHOR: [AUTO] Start is the daemon entry point for the Telegram channel.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P1/P2; fan_in via start subcommand, bootstrap tests, and daemon hook.
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

	var handler Handler
	if deps.Store != nil && deps.Audit != nil && deps.Agent != nil {
		handler = NewBridgeQueryHandler(deps.Client, deps.Store, deps.Audit, deps.Agent, deps.Config, logger)
		logger.Info("telegram channel starting with BridgeQueryHandler (P2)", zap.String("bot", deps.Config.BotUsername))
	} else {
		// Fall back to echo handler when P2 deps are not wired.
		handler = NewEchoHandler(deps.Client, logger)
		logger.Info("telegram channel starting with EchoHandler (P1 fallback)", zap.String("bot", deps.Config.BotUsername))
	}

	poller := NewPoller(deps.Client, handler, logger)
	return poller.Run(ctx)
}
