// SPEC-GOOSE-CLI-001 Phase B4 — ToolRegistry backed by ConnectClient.ToolService.
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/modu-ai/mink/internal/cli/transport"
)

// connectToolClient captures the subset of *transport.ConnectClient that
// ConnectToolRegistry depends on. Tests inject a fake by implementing this
// interface so the registry can be exercised without a live daemon.
type connectToolClient interface {
	ListTools(ctx context.Context) ([]transport.ToolDescriptor, error)
}

// connectToolDefaultTimeout bounds the per-call deadline applied when
// ToolRegistry.ListTools is invoked, since the commands.ToolRegistry
// interface is context-less.
const connectToolDefaultTimeout = 5 * time.Second

// ConnectToolRegistry satisfies ToolRegistry by delegating to
// ConnectClient.ToolService over the daemon. The legacy
// StaticToolRegistry remains as the offline fallback (used by tests and
// when a live daemon is unavailable).
//
// @MX:ANCHOR ConnectToolRegistry is the production ToolRegistry, replacing
// the static placeholder for the rootcmd-wired binary.
// @MX:REASON SPEC-GOOSE-CLI-001 Phase B4; fan_in == 1 today via rootcmd.
type ConnectToolRegistry struct {
	daemonAddr string
	newClient  func(daemonURL string, opts ...transport.ConnectOption) (*transport.ConnectClient, error)
	timeout    time.Duration

	// clientOverride lets tests bypass ConnectClient construction and inject
	// a stub implementing connectToolClient. When non-nil, newClient is
	// not called.
	clientOverride connectToolClient
}

// NewConnectToolRegistry returns a ToolRegistry that issues a Connect-protocol
// ListTools RPC against the daemon at daemonAddr. The address may be
// supplied in either "host:port" or "http://host:port" form.
func NewConnectToolRegistry(daemonAddr string) *ConnectToolRegistry {
	return &ConnectToolRegistry{
		daemonAddr: daemonAddr,
		newClient:  transport.NewConnectClient,
		timeout:    connectToolDefaultTimeout,
	}
}

// ListTools fetches tool descriptors from the daemon and projects them
// onto the simpler commands.ToolInfo (Name + Description only). Source
// and ServerID fields are intentionally dropped — they are surfaced by
// future tui/insights paths, not by the non-interactive `goose tool`
// command.
func (r *ConnectToolRegistry) ListTools() ([]ToolInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.effectiveTimeout())
	defer cancel()

	client, err := r.client()
	if err != nil {
		return nil, err
	}

	descs, err := client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	infos := make([]ToolInfo, 0, len(descs))
	for _, d := range descs {
		infos = append(infos, ToolInfo{
			Name:        d.Name,
			Description: d.Description,
		})
	}
	return infos, nil
}

func (r *ConnectToolRegistry) client() (connectToolClient, error) {
	if r.clientOverride != nil {
		return r.clientOverride, nil
	}
	c, err := r.newClient(transport.NormalizeDaemonURL(r.daemonAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	return c, nil
}

func (r *ConnectToolRegistry) effectiveTimeout() time.Duration {
	if r.timeout > 0 {
		return r.timeout
	}
	return connectToolDefaultTimeout
}
