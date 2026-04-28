// Package commands provides daemon management commands for goose CLI.
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// NewDaemonCommand creates the daemon command with subcommands.
// @MX:ANCHOR This function creates the complete daemon command tree.
func NewDaemonCommand(client PingClient, addr string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage goose daemon",
		Long:  `Check status and control the goose daemon.`,
	}

	// Add subcommands
	cmd.AddCommand(newDaemonStatusCommand(client, addr))
	cmd.AddCommand(newDaemonShutdownCommand())

	return cmd
}

// newDaemonStatusCommand creates the daemon status subcommand.
func newDaemonStatusCommand(client PingClient, addr string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check if the daemon is running",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			out := cmd.OutOrStdout()
			start := time.Now()

			err := client.Ping(ctx, addr, out)
			if err != nil {
				return fmt.Errorf("goose: daemon is not running: %w", err)
			}

			latency := time.Since(start)
			fmt.Fprintf(out, "\nDaemon is running at %s\n", addr)
			fmt.Fprintf(out, "Latency: %v\n", latency)

			return nil
		},
	}
}

// newDaemonShutdownCommand creates the daemon shutdown subcommand.
// @MX:TODO Implement actual daemon shutdown in future phases.
func newDaemonShutdownCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown the daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("goose: daemon shutdown not yet implemented")
		},
	}
}
