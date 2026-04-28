// Package cli provides CLI application wiring and initialization.
package cli

import (
	"context"
	"sync"

	"github.com/modu-ai/goose/internal/cli/transport"
	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/command/adapter"
	"github.com/modu-ai/goose/internal/command/adapter/aliasconfig"
	"github.com/modu-ai/goose/internal/command/builtin"
	"github.com/modu-ai/goose/internal/llm/router"
	"go.uber.org/zap"
)

// App holds wiring dependencies for the goose CLI.
// One instance per process, initialized via NewApp.
// @MX:ANCHOR: [AUTO] Core CLI application struct combining dispatcher, adapter, and transport.
// @MX:REASON: Fan-in >= 3 - Used by rootcmd, TUI, ask command, and tests.
type App struct {
	Adapter    *adapter.ContextAdapter
	Dispatcher *command.Dispatcher
	Client     *transport.DaemonClient // May be nil if daemon unreachable
	Logger     *zap.Logger
}

// AppConfig carries flags/env for App initialization.
type AppConfig struct {
	AliasFile   string
	StrictAlias bool
	DaemonAddr  string
	Logger      *zap.Logger
}

// zapAdapterLogger wraps *zap.Logger to implement adapter.Logger interface.
// @MX:NOTE: [AUTO] Adapter wrapper - zap.Logger uses zap.Field, adapter.Logger uses ...any.
type zapAdapterLogger struct {
	logger *zap.Logger
}

func (l *zapAdapterLogger) Warn(msg string, fields ...any) {
	if l.logger == nil {
		return
	}
	// Convert variadic fields to zap.String for simplicity
	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			zapFields = append(zapFields, zap.Any(key, fields[i+1]))
		}
	}
	l.logger.Warn(msg, zapFields...)
}

// NewApp creates the CLI App with 4-stage wiring:
// 1. cmdRegistry := command.NewRegistry() + builtin.Register()
// 2. providerRegistry := router.DefaultRegistry()
// 3. aliasMap := aliasconfig loader (graceful fallback to empty map)
// 4. adapter := adapter.New(...)
// 5. dispatcher := command.NewDispatcher(...)
// Also attempts transport connection (Client may be nil on failure).
// @MX:ANCHOR: [AUTO] Primary App constructor - single initialization entry point.
// @MX:REASON: Called by InitApp (sync.Once wrapper) and tests directly - fan_in >= 3.
func NewApp(cfg AppConfig) (*App, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	// Stage 1: Create command registry and register built-in commands
	cmdRegistry, err := command.NewRegistry(command.WithLogger(logger))
	if err != nil {
		return nil, err
	}
	builtin.Register(cmdRegistry)

	// Stage 2: Create provider registry for model resolution
	providerRegistry := router.DefaultRegistry()

	// Stage 3: Load alias config with graceful fallback
	aliasMap, err := loadAliasConfig(cfg.AliasFile, cfg.StrictAlias, providerRegistry, logger)
	if err != nil {
		// Log warning but continue with empty map
		logger.Warn("alias config load failed; using empty map", zap.Error(err))
		aliasMap = map[string]string{}
	}

	// Stage 4: Create ContextAdapter
	adapterLogger := &zapAdapterLogger{logger: logger}
	adp := adapter.New(adapter.Options{
		Registry:       providerRegistry,
		LoopController: nil, // Client-side omit - no loop controller in CLI
		AliasMap:       aliasMap,
		Logger:         adapterLogger,
	})

	// Stage 5: Create Dispatcher
	dispatcher := command.NewDispatcher(cmdRegistry, command.Config{}, logger)

	// Stage 6: Attempt transport client connection (may fail gracefully)
	var client *transport.DaemonClient
	if cfg.DaemonAddr != "" {
		client, err = transport.NewDaemonClient(cfg.DaemonAddr, 0) // Use default timeout
		if err != nil {
			// Daemon not running - that's OK for CLI-only commands
			logger.Debug("daemon unreachable at App init", zap.String("addr", cfg.DaemonAddr), zap.Error(err))
			client = nil
		}
	}

	return &App{
		Adapter:    adp,
		Dispatcher: dispatcher,
		Client:     client,
		Logger:     logger,
	}, nil
}

// loadAliasConfig loads alias configuration from file with graceful fallback.
// Returns empty map on error instead of failing.
// @MX:NOTE: [AUTO] Graceful degradation pattern - alias config is optional.
func loadAliasConfig(aliasFile string, strict bool, registry *router.ProviderRegistry, logger *zap.Logger) (map[string]string, error) {
	// Create loader with options
	opts := aliasconfig.Options{
		ConfigPath: aliasFile,
		Logger:     logger,
	}
	loader := aliasconfig.New(opts)

	// Load config
	aliasMap, err := loader.Load()
	if err != nil {
		// File not found is OK - return empty map
		if err == aliasconfig.ErrConfigNotFound {
			return map[string]string{}, nil
		}
		return nil, err
	}

	// Validate if strict mode enabled
	if strict {
		errs := aliasconfig.Validate(aliasMap, registry, strict)
		if len(errs) > 0 {
			logger.Warn("alias config validation failed", zap.Int("errors", len(errs)))
			// Return errors for caller to decide
			return nil, errs[0] // Return first error
		}
	}

	return aliasMap, nil
}

var (
	appOnce    sync.Once
	appInstance *App
	initErr    error
)

// InitApp creates or returns the singleton App instance.
// Thread-safe via sync.Once. Returns error on first initialization failure.
// @MX:ANCHOR: [AUTO] Singleton App accessor with sync.Once protection.
// @MX:REASON: Called by rootcmd PersistentPreRunE and all subcommands - fan_in >= 3.
func InitApp(cfg AppConfig) (*App, error) {
	appOnce.Do(func() {
		appInstance, initErr = NewApp(cfg)
	})
	return appInstance, initErr
}

// appKey is the context key for storing/retrieving App from context.
type appKey struct{}

// WithApp stores the App in a context for retrieval by subcommands.
// @MX:NOTE: [AUTO] Context propagation pattern for dependency injection.
func WithApp(ctx context.Context, app *App) context.Context {
	return context.WithValue(ctx, appKey{}, app)
}

// AppFromContext retrieves the App from a context.
// Returns nil if context is nil or doesn't contain an App.
// @MX:ANCHOR: [AUTO] Context accessor for App dependency injection.
// @MX:REASON: Used by all CLI subcommands (ask, TUI, etc.) - fan_in >= 3.
func AppFromContext(ctx context.Context) *App {
	if ctx == nil {
		return nil
	}
	app, _ := ctx.Value(appKey{}).(*App)
	return app
}
