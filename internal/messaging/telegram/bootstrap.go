package telegram

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Deps bundles the runtime dependencies required to start the Telegram channel.
//
// P2 expands Deps with Store, Audit, and Agent. When Store/Audit/Agent are nil,
// Start falls back to the P1 EchoHandler for backward compatibility.
// P4 adds the optional Stream field for editMessageText-based streaming (REQ-MTGM-E02).
// P4-T2 adds the optional Mux field for webhook mode (REQ-MTGM-E07).
type Deps struct {
	Config *Config
	Client Client
	// Store persists chat_id mappings and polling offset (P2+).
	Store Store
	// Audit records inbound/outbound messaging events (P2+).
	Audit *AuditWrapper
	// Agent forwards user messages to the Mink AI backend (P2+).
	Agent AgentQuery
	// Stream enables editMessageText-based streaming responses (P4+, optional).
	// When nil, streaming is disabled and all responses use the non-streaming path.
	Stream AgentStream
	// Mux is the HTTP mux used to register the webhook endpoint when
	// Config.Mode == "webhook". Optional; if nil, webhook mode falls back to
	// polling regardless of FallbackToPolling (REQ-MTGM-E07).
	Mux    *http.ServeMux
	Logger *zap.Logger
}

// NoOpAgentQuery is a placeholder AgentQuery used in tests and during early
// bootstrap when a real ChatService is not yet available.
//
// @MX:NOTE: [AUTO] OS keyring backend wired via OSKeyring (keyring_os.go),
// fallback MemoryKeyring for tests and -tags=nokeyring builds. Selection rule:
// production main.go uses OSKeyring; tests inject MemoryKeyring via Deps.
// Production Agent wiring uses NewAgentAdapter(chatService) as of P3.
// NoOpAgentQuery is retained for test/dev use only (strategy-p3.md §A.5).
type NoOpAgentQuery struct{}

// Query returns a fixed notice that agent wiring is pending.
func (n *NoOpAgentQuery) Query(_ context.Context, _ string, _ []string) (string, error) {
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
		h := NewBridgeQueryHandler(deps.Client, deps.Store, deps.Audit, deps.Agent, deps.Config, logger)
		if deps.Stream != nil {
			h = h.WithStream(deps.Stream)
			logger.Info("telegram channel starting with BridgeQueryHandler + streaming (P4)", zap.String("bot", deps.Config.BotUsername))
		} else {
			logger.Info("telegram channel starting with BridgeQueryHandler (P2)", zap.String("bot", deps.Config.BotUsername))
		}
		handler = h
	} else {
		// Fall back to echo handler when P2 deps are not wired.
		handler = NewEchoHandler(deps.Client, logger)
		logger.Info("telegram channel starting with EchoHandler (P1 fallback)", zap.String("bot", deps.Config.BotUsername))
	}

	// Select update ingestion strategy based on configured mode.
	mode := strings.ToLower(deps.Config.Mode)
	if mode == "" {
		mode = "polling"
	}

	switch mode {
	case "webhook":
		return startWebhookMode(ctx, deps, handler, logger)
	case "polling":
		return NewPoller(deps.Client, handler, logger).Run(ctx)
	default:
		return fmt.Errorf("telegram bootstrap: unknown mode %q", deps.Config.Mode)
	}
}

// startWebhookMode handles the webhook update ingestion path (REQ-MTGM-E07).
// If the mux is nil or SetWebhook fails and FallbackToPolling is true, it
// falls back to long polling automatically.
func startWebhookMode(ctx context.Context, deps Deps, handler Handler, logger *zap.Logger) error {
	if deps.Mux == nil {
		logger.Warn("webhook mode requested but no Mux supplied; falling back to polling",
			zap.String("bot", deps.Config.BotUsername))
		return NewPoller(deps.Client, handler, logger).Run(ctx)
	}

	if err := registerWebhookFromConfig(ctx, deps, handler, logger); err != nil {
		if !deps.Config.Webhook.FallbackToPolling {
			return fmt.Errorf("telegram bootstrap: webhook registration failed (no fallback): %w", err)
		}
		logger.Warn("webhook registration failed, falling back to polling",
			zap.Error(err))
		return NewPoller(deps.Client, handler, logger).Run(ctx)
	}

	logger.Info("telegram channel running in webhook mode",
		zap.String("bot", deps.Config.BotUsername))
	// Block until ctx is cancelled.
	<-ctx.Done()

	// Graceful shutdown: best-effort deleteWebhook to let Telegram switch back
	// to getUpdates mode on the next bot restart.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := deps.Client.DeleteWebhook(shutdownCtx, false); err != nil {
		logger.Warn("deleteWebhook on shutdown failed", zap.Error(err))
	}
	return ctx.Err()
}

// registerWebhookFromConfig calls RegisterWebhook using the values from
// deps.Config.Webhook. It auto-generates a secret when the config field is empty.
func registerWebhookFromConfig(ctx context.Context, deps Deps, handler Handler, logger *zap.Logger) error {
	wcfg := deps.Config.Webhook
	if wcfg.PublicURL == "" {
		return fmt.Errorf("webhook mode requires webhook.public_url in config")
	}
	secret := wcfg.Secret
	if secret == "" {
		s, err := GenerateWebhookSecret()
		if err != nil {
			return err
		}
		secret = s
	}
	return RegisterWebhook(ctx, deps.Client, deps.Mux, handler, wcfg.PublicURL, secret, logger)
}
