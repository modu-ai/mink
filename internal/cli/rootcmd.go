// Package cli provides the root CLI command and subcommands for the goose tool.
package cli

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/cli/commands"
	"github.com/modu-ai/mink/internal/cli/transport"
	"github.com/modu-ai/mink/internal/cli/tui"
	memorycli "github.com/modu-ai/mink/internal/memory/cli"
	"github.com/modu-ai/mink/internal/userpath"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// @MX:ANCHOR Execute is the main entry point for the CLI.
// It is called by cmd/goose/main.go and can be called by tests.
// Returns the appropriate exit code.
func Execute(version, commit, builtAt string) int {
	rootCmd := NewRootCommand(version, commit, builtAt)
	if err := rootCmd.Execute(); err != nil {
		// Error already printed by cobra
		return ExitError
	}
	return ExitOK
}

// NewRootCommand creates the root cobra command for the goose CLI.
func NewRootCommand(version, commit, builtAt string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "goose",
		Short:         "AI Daily Companion",
		SilenceUsage:  true,
		SilenceErrors: true,
		// PersistentPreRunE initializes App for all subcommands
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Create logger
			logLevel, _ := cmd.PersistentFlags().GetString("log-level")
			logger := newLogger(logLevel)

			// T-015: ~/.goose/ → ~/.mink/ 최초 1회 자동 마이그레이션.
			// 비치명적: 마이그레이션 실패 시 경고 로그 후 계속 진행.
			result, migrateErr := userpath.MigrateOnce(cmd.Context())
			if migrateErr != nil {
				logger.Warn("userdata migration failed (non-fatal)", zap.Error(migrateErr))
			} else if result.Migrated {
				logger.Info(result.Notice,
					zap.String("from", result.SourcePath),
					zap.String("to", result.DestPath),
					zap.String("method", result.Method),
				)
			}

			// Get flags for App initialization
			daemonAddr, _ := cmd.PersistentFlags().GetString("daemon-addr")
			aliasFile, _ := cmd.PersistentFlags().GetString("config")

			// Initialize App (singleton via sync.Once)
			app, err := InitApp(AppConfig{
				AliasFile:   aliasFile,
				StrictAlias: false, // TODO: make this a flag
				DaemonAddr:  daemonAddr,
				Logger:      logger,
			})
			if err != nil {
				return fmt.Errorf("failed to initialize app: %w", err)
			}

			// Store App in command context for subcommands
			ctx := WithApp(cmd.Context(), app)
			cmd.SetContext(ctx)

			return nil
		},
		// Default to TUI chat mode when no subcommand is provided
		RunE: func(cmd *cobra.Command, args []string) error {
			app := AppFromContext(cmd.Context())
			addr, _ := cmd.PersistentFlags().GetString("daemon-addr")
			noColor, _ := cmd.PersistentFlags().GetBool("no-color")
			return tui.RunWithApp(app, addr, noColor)
		},
	}

	// Global flags
	rootCmd.PersistentFlags().String("config", "", "Path to configuration file")
	rootCmd.PersistentFlags().String("daemon-addr", "127.0.0.1:9005", "Address of the goose daemon")
	rootCmd.PersistentFlags().String("format", "text", "Output format (text|json)")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug|info|warn|error)")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")

	// Add subcommands
	rootCmd.AddCommand(commands.NewVersionCommand(version, commit, builtAt))
	rootCmd.AddCommand(commands.NewInitCommand())

	// Ping command — Phase B1 wiring: PingClientAdapter delegates to
	// ConnectClient.Ping (Phase A). The legacy NewGRPCPingClient remains
	// available in commands/ping.go for fallback / regression testing.
	pingClient := transport.NewPingClientAdapter()
	rootCmd.AddCommand(commands.NewPingCommand(pingClient, "127.0.0.1:9005"))

	// Ask command — Phase B2 wiring: askClientAdapter delegates to
	// ConnectClient.ChatStream and uses transport-package helpers
	// (TranslateChatEvent + ChatStreamFanIn) for event translation.
	askClient := newAskClientAdapter("127.0.0.1:9005")
	rootCmd.AddCommand(commands.NewAskCommand(askClient, "127.0.0.1:9005"))

	// Session commands
	rootCmd.AddCommand(commands.NewSessionCommand("127.0.0.1:9005"))

	// Config commands — Phase B3 wiring: ConnectConfigStore delegates to
	// ConnectClient.ConfigService. MemoryConfigStore remains as the
	// in-process fallback used by tests.
	rootCmd.AddCommand(commands.NewConfigCommand(commands.NewConnectConfigStore("127.0.0.1:9005")))

	// Tool commands — Phase B4 wiring: ConnectToolRegistry delegates to
	// ConnectClient.ToolService. StaticToolRegistry remains as the
	// offline fallback used by tests.
	rootCmd.AddCommand(commands.NewToolCommand(commands.NewConnectToolRegistry("127.0.0.1:9005")))

	// Plugin commands (stub)
	rootCmd.AddCommand(commands.NewPluginCommand())

	// Daemon commands (reuse pingClient)
	rootCmd.AddCommand(commands.NewDaemonCommand(pingClient, "127.0.0.1:9005"))

	// Audit command
	rootCmd.AddCommand(commands.NewAuditCommand())

	// Messaging commands — Telegram channel setup, status, and start.
	rootCmd.AddCommand(commands.NewMessagingCommand())

	// Doctor command — subsystem health checks (auth-keyring, etc.).
	rootCmd.AddCommand(commands.NewDoctorCommand())

	// Login / logout commands — interactive credential registration and
	// removal (mink login {provider} / mink logout {provider}).
	// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (T-016, AC-CR-012, AC-CR-015)
	rootCmd.AddCommand(commands.NewLoginCommand())

	// Memory commands — QMD-based lifelong memory vault (M1: add only).
	// SPEC: SPEC-MINK-MEMORY-QMD-001 (T1.9)
	rootCmd.AddCommand(memorycli.NewMemoryCommand())

	return rootCmd
}

// newLogger creates a zap logger with the specified level.
// @MX:NOTE Helper function for logger creation in root command.
func newLogger(level string) *zap.Logger {
	lvl := zap.InfoLevel
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zap.InfoLevel
	}

	logger, _ := zap.NewProduction()
	if level != "" {
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(lvl)
		logger, _ = config.Build()
	}

	return logger
}

// askClientAdapter wraps the Connect-protocol ConnectClient as a
// commands.AskClient. Translation between the wire ChatStreamEvent and
// the simpler StreamEvent shape lives in the transport package
// (TranslateChatEvent + ChatStreamFanIn) so the same logic is shared
// with future tui chat consumers.
//
// @MX:ANCHOR askClientAdapter bridges Connect transport to the ask
// subcommand; replaces the legacy gRPC-go adapter as of Phase B2.
// @MX:REASON SPEC-GOOSE-CLI-001 Phase B2; fan_in == 1 today.
type askClientAdapter struct {
	daemonAddr string
	newClient  func(daemonURL string, opts ...transport.ConnectOption) (*transport.ConnectClient, error)
}

func newAskClientAdapter(daemonAddr string) *askClientAdapter {
	return &askClientAdapter{
		daemonAddr: daemonAddr,
		newClient:  transport.NewConnectClient,
	}
}

// ChatStream implements commands.AskClient.
func (a *askClientAdapter) ChatStream(ctx context.Context, messages []commands.Message) (<-chan commands.StreamEvent, error) {
	if len(messages) == 0 {
		return nil, transport.ErrEmptyMessages()
	}

	client, err := a.newClient(transport.NormalizeDaemonURL(a.daemonAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	views := make([]transport.ChatMessageView, len(messages))
	for i, m := range messages {
		views[i] = transport.ChatMessageView{Role: m.Role, Content: m.Content}
	}
	priors, lastMsg, _ := transport.SplitMessagesAtLastUser(views)

	rawEvents, errCh := client.ChatStream(ctx, "", lastMsg, transport.WithInitialMessages(priors))
	fan := transport.ChatStreamFanIn(ctx, rawEvents, errCh)

	out := make(chan commands.StreamEvent, 16)
	go func() {
		defer close(out)
		for ev := range fan {
			select {
			case out <- commands.StreamEvent{Type: ev.Type, Content: ev.Content}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// ExecuteWithCommand is a test helper that executes a command and returns the exit code.
// @MX:ANCHOR This function is used by tests to verify exit codes.
func ExecuteWithCommand(cmd *cobra.Command) int {
	if err := cmd.Execute(); err != nil {
		return ExitError
	}
	return ExitOK
}
