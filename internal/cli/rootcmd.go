// Package cli provides the root CLI command and subcommands for the goose tool.
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/modu-ai/goose/internal/cli/commands"
	"github.com/modu-ai/goose/internal/cli/tui"
	"github.com/modu-ai/goose/internal/cli/transport"
	"github.com/spf13/cobra"
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
		// Default to TUI chat mode when no subcommand is provided
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, _ := cmd.PersistentFlags().GetString("daemon-addr")
			noColor, _ := cmd.PersistentFlags().GetBool("no-color")
			return tui.Run(addr, noColor)
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

	// Ping command with real gRPC client
	pingClient := commands.NewGRPCPingClient()
	rootCmd.AddCommand(commands.NewPingCommand(pingClient, "127.0.0.1:9005"))

	// Ask command with gRPC client adapter
	askClient := &askClientAdapter{newClient: transport.NewDaemonClient}
	rootCmd.AddCommand(commands.NewAskCommand(askClient, "127.0.0.1:9005"))

	// Session commands
	rootCmd.AddCommand(commands.NewSessionCommand("127.0.0.1:9005"))

	// Config commands
	rootCmd.AddCommand(commands.NewConfigCommand(commands.NewMemoryConfigStore()))

	// Tool commands
	rootCmd.AddCommand(commands.NewToolCommand(commands.NewStaticToolRegistry()))

	// Plugin commands (stub)
	rootCmd.AddCommand(commands.NewPluginCommand())

	// Daemon commands (reuse pingClient)
	rootCmd.AddCommand(commands.NewDaemonCommand(pingClient, "127.0.0.1:9005"))

	return rootCmd
}

// askClientAdapter adapts transport.DaemonClient to commands.AskClient interface.
// @MX:ANCHOR This adapter bridges transport and command layers for ask command.
type askClientAdapter struct {
	newClient func(addr string, timeout time.Duration) (*transport.DaemonClient, error)
}

// ChatStream implements commands.AskClient interface.
func (a *askClientAdapter) ChatStream(ctx context.Context, messages []commands.Message) (<-chan commands.StreamEvent, error) {
	// Get daemon address from context (set by command)
	addr := "127.0.0.1:9005" // Default

	client, err := a.newClient(addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Convert commands.Message to transport.Message
	transportMessages := make([]transport.Message, len(messages))
	for i, msg := range messages {
		transportMessages[i] = transport.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	eventCh, err := client.ChatStream(ctx, transportMessages)
	if err != nil {
		client.Close()
		return nil, err
	}

	// Convert transport events to command events
	resultCh := make(chan commands.StreamEvent, 10)
	go func() {
		defer client.Close()
		defer close(resultCh)
		for event := range eventCh {
			resultCh <- commands.StreamEvent{
				Type:    event.Type,
				Content: event.Content,
			}
		}
	}()

	return resultCh, nil
}

// ExecuteWithCommand is a test helper that executes a command and returns the exit code.
// @MX:ANCHOR This function is used by tests to verify exit codes.
func ExecuteWithCommand(cmd *cobra.Command) int {
	if err := cmd.Execute(); err != nil {
		return ExitError
	}
	return ExitOK
}
