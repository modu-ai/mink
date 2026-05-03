// Package transport — Connect-protocol adapters bridging transport types
// to the commands-layer interfaces.
//
// SPEC-GOOSE-CLI-001 Phase B: PingClientAdapter (Phase B1) wires
// ConnectClient.Ping into commands.PingClient. Additional adapters
// (AskClientAdapter, ConnectConfigStore, ConnectToolRegistry) follow in
// Phase B2~B4.
package transport

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// connectClientFactory builds a *ConnectClient for the given daemon URL.
// The signature mirrors NewConnectClient and exists so adapter tests can
// inject a test-server-targeting factory without touching production code.
type connectClientFactory func(daemonURL string, opts ...ConnectOption) (*ConnectClient, error)

// PingClientAdapter implements commands.PingClient by delegating to a
// per-call ConnectClient. The lazy-connect semantics match the legacy
// GRPCPingClient: each Ping invocation builds a transient client targeting
// the address supplied by cobra's --daemon-addr flag, so flag overrides
// continue to work without a long-lived client.
//
// @MX:ANCHOR PingClientAdapter bridges Connect transport to the commands
// layer; consumed by rootcmd for ping and daemon-status subcommands.
// @MX:REASON SPEC-GOOSE-CLI-001 Phase B1; fan_in >= 2 (ping + daemon).
type PingClientAdapter struct {
	newClient connectClientFactory
}

// NewPingClientAdapter returns a PingClientAdapter that uses NewConnectClient
// to build a fresh client on each Ping call. The adapter is safe for
// concurrent use; each call constructs an independent http.Client.
func NewPingClientAdapter() *PingClientAdapter {
	return &PingClientAdapter{newClient: NewConnectClient}
}

// Ping satisfies commands.PingClient. It dials the daemon at addr,
// invokes ConnectClient.Ping, and writes a single status line in the
// byte-identical format used by the legacy GRPCPingClient.
//
// The host:port form accepted by --daemon-addr is normalized to a full
// http:// URL via NormalizeDaemonURL before dialling.
func (a *PingClientAdapter) Ping(ctx context.Context, addr string, writer io.Writer) error {
	client, err := a.newClient(NormalizeDaemonURL(addr))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	resp, err := client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	fmt.Fprintf(writer, "pong (version=%s, state=%s, uptime=%dms)\n",
		resp.Version, resp.State, resp.UptimeMs)
	return nil
}

// NormalizeDaemonURL prepends "http://" to a bare host:port address.
// Inputs that already carry an http:// or https:// scheme are returned
// unchanged.
//
// @MX:NOTE Connect requires a full URL, while the cobra --daemon-addr flag
// defaults to "host:port" for backward compatibility with legacy gRPC-go.
func NormalizeDaemonURL(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
}
