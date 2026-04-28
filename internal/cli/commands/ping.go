package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/modu-ai/goose/internal/cli/transport"
	"github.com/spf13/cobra"
)

// PingClient defines the interface for pinging the daemon.
// @MX:ANCHOR This interface allows mocking in tests and different implementations.
type PingClient interface {
	Ping(ctx context.Context, addr string, writer io.Writer) error
}

// NewPingCommand creates the ping subcommand.
// @MX:NOTE The daemon address is configurable via global flag, with default "127.0.0.1:9005".
func NewPingCommand(client PingClient, defaultAddr string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Check if the goose daemon is running",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Get address from persistent flag (inherited from parent) or use default
			addr, err := cmd.PersistentFlags().GetString("daemon-addr")
			if err != nil {
				return err
			}
			if addr == "" {
				addr = defaultAddr
			}

			if err := client.Ping(ctx, addr, cmd.OutOrStdout()); err != nil {
				// @MX:NOTE Error prefix "goose:" is required by REQ-CLI-002
				fmt.Fprintf(cmd.ErrOrStderr(), "goose: daemon unreachable at %s: %v\n", addr, err)
				// Return error to trigger exit code 69
				return fmt.Errorf("daemon unreachable")
			}

			return nil
		},
	}

	return cmd
}

// GRPCPingClient adapts DaemonClient to PingClient interface.
// @MX:ANCHOR This adapter bridges transport and command layers.
type GRPCPingClient struct {
	newClient func(addr string, timeout time.Duration) (DaemonClientInterface, error)
}

// DaemonClientInterface defines the minimal interface needed for ping.
// This allows mocking and testing without full transport dependency.
type DaemonClientInterface interface {
	Ping(ctx context.Context) (*transport.PingResponse, error)
	Close() error
}

// NewGRPCPingClient creates a ping client that uses gRPC transport.
func NewGRPCPingClient() *GRPCPingClient {
	return &GRPCPingClient{
		newClient: func(addr string, timeout time.Duration) (DaemonClientInterface, error) {
			return transport.NewDaemonClient(addr, timeout)
		},
	}
}

// Ping sends a ping request to the daemon.
func (g *GRPCPingClient) Ping(ctx context.Context, addr string, writer io.Writer) error {
	client, err := g.newClient(addr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	resp, err := client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	fmt.Fprintf(writer, "pong (version=%s, state=%s, uptime=%dms)\n",
		resp.Version, resp.State, resp.UptimeMs)
	return nil
}
