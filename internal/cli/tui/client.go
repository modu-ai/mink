// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/mink/internal/cli/transport"
)

// daemonClientAdapter wraps transport.DaemonClient to implement tui.DaemonClient.
// @MX:ANCHOR This adapter bridges the TUI and transport layers.
type daemonClientAdapter struct {
	transport *transport.DaemonClient
}

// NewDaemonClientFactory creates a new TUI daemon client from a transport client factory.
// @MX:ANCHOR This is the primary factory function for creating TUI clients.
func NewDaemonClientFactory(transportClient func(addr string, timeout int) (*transport.DaemonClient, error)) DaemonClient {
	return &clientFactory{
		newClient: transportClient,
	}
}

// clientFactory creates daemon clients on demand.
// @MX:NOTE This allows lazy connection to the daemon.
type clientFactory struct {
	newClient func(addr string, timeout int) (*transport.DaemonClient, error)
}

// ChatStream implements tui.DaemonClient interface.
// @MX:ANCHOR This method converts TUI message types to transport types.
func (f *clientFactory) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error) {
	// For now, we'll create a new client for each stream
	// In Phase E, we can implement connection pooling
	client, err := f.newClient("127.0.0.1:9005", 30)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	// Convert TUI messages to transport messages
	transportMessages := make([]transport.Message, len(messages))
	for i, msg := range messages {
		transportMessages[i] = transport.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Call transport ChatStream
	eventCh, err := client.ChatStream(ctx, transportMessages)
	if err != nil {
		client.Close()
		return nil, err
	}

	// Convert transport events to TUI events
	resultCh := make(chan StreamEvent, 10)
	go func() {
		defer client.Close()
		defer close(resultCh)
		for event := range eventCh {
			resultCh <- StreamEvent{
				Type:    event.Type,
				Content: event.Content,
			}
		}
	}()

	return resultCh, nil
}

// Close implements tui.DaemonClient interface.
// @MX:NOTE No-op for factory since each stream creates its own client.
func (f *clientFactory) Close() error {
	// Factory doesn't hold a connection, so nothing to close
	return nil
}

// ResolvePermission implements tui.DaemonClient interface for legacy factory.
// Stub returns accepted=true.
// TODO(QUERY-001): wire to engine.ResolvePermission when available.
func (f *clientFactory) ResolvePermission(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

// NewDaemonClientAdapter creates a TUI client adapter from an existing transport client.
// @MX:NOTE Use this when you already have a connected transport client.
func NewDaemonClientAdapter(transportClient *transport.DaemonClient) DaemonClient {
	return &daemonClientAdapter{
		transport: transportClient,
	}
}

// ChatStream implements tui.DaemonClient interface for adapter.
func (a *daemonClientAdapter) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error) {
	// Convert TUI messages to transport messages
	transportMessages := make([]transport.Message, len(messages))
	for i, msg := range messages {
		transportMessages[i] = transport.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Call transport ChatStream
	eventCh, err := a.transport.ChatStream(ctx, transportMessages)
	if err != nil {
		return nil, err
	}

	// Convert transport events to TUI events
	resultCh := make(chan StreamEvent, 10)
	go func() {
		defer close(resultCh)
		for event := range eventCh {
			resultCh <- StreamEvent{
				Type:    event.Type,
				Content: event.Content,
			}
		}
	}()

	return resultCh, nil
}

// Close implements tui.DaemonClient interface for adapter.
func (a *daemonClientAdapter) Close() error {
	return a.transport.Close()
}

// ResolvePermission implements tui.DaemonClient interface for adapter.
// Delegates to the transport client's ResolvePermission stub.
func (a *daemonClientAdapter) ResolvePermission(ctx context.Context, toolUseID, toolName, decision string) (bool, error) {
	return a.transport.ResolvePermission(ctx, toolUseID, toolName, decision)
}

// connectClientFactory implements DaemonClient by issuing Connect-protocol
// streaming RPCs against the daemon. Each ChatStream call constructs a
// fresh ConnectClient targeting the configured daemon address; this
// matches the lazy-connect pattern of the legacy gRPC-go factory and
// avoids holding a long-lived HTTP/2 connection inside the TUI Model.
//
// @MX:ANCHOR connectClientFactory bridges the Connect transport layer to
// the TUI Model; replaces the legacy gRPC-go clientFactory in Phase C1.
// @MX:REASON SPEC-GOOSE-CLI-001 Phase C1; fan_in == 1 (RunWithApp).
type connectClientFactory struct {
	daemonAddr string
	newClient  func(daemonURL string, opts ...transport.ConnectOption) (*transport.ConnectClient, error)
}

// NewConnectClientFactory returns a DaemonClient that uses ConnectClient
// for chat streaming. The address may be supplied as either "host:port" or
// "http://host:port"; transport.NormalizeDaemonURL handles the conversion.
func NewConnectClientFactory(daemonAddr string) DaemonClient {
	return &connectClientFactory{
		daemonAddr: daemonAddr,
		newClient:  transport.NewConnectClient,
	}
}

// ChatStream satisfies DaemonClient. It dials the daemon, sends the last
// user message as the agent prompt, and translates wire ChatStreamEvent
// frames into the simpler tui.StreamEvent shape via the transport
// package's shared helpers (TranslateChatEvent + ChatStreamFanIn).
//
// Phase B/C contract: the AgentService takes a single user message string;
// multi-turn replay arrives in a later phase.
func (f *connectClientFactory) ChatStream(ctx context.Context, messages []ChatMessage) (<-chan StreamEvent, error) {
	if len(messages) == 0 {
		return nil, transport.ErrEmptyMessages()
	}

	client, err := f.newClient(transport.NormalizeDaemonURL(f.daemonAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	views := make([]transport.ChatMessageView, len(messages))
	for i, m := range messages {
		views[i] = transport.ChatMessageView{Role: m.Role, Content: m.Content}
	}
	priors, lastMsg, _ := transport.SplitMessagesAtLastUser(views)

	rawEvents, errCh := client.ChatStream(ctx, "", lastMsg, transport.WithInitialMessages(priors))
	fan := transport.ChatStreamFanIn(ctx, rawEvents, errCh)

	out := make(chan StreamEvent, 16)
	go func() {
		defer close(out)
		for ev := range fan {
			select {
			case out <- StreamEvent{Type: ev.Type, Content: ev.Content}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// Close satisfies DaemonClient. The lazy factory holds no persistent
// connection so there is nothing to release.
func (f *connectClientFactory) Close() error {
	return nil
}

// ResolvePermission satisfies DaemonClient. Dials the daemon and calls the
// ResolvePermission Connect RPC stub.
// TODO(QUERY-001): wire to engine.ResolvePermission when available.
func (f *connectClientFactory) ResolvePermission(ctx context.Context, toolUseID, toolName, decision string) (bool, error) {
	client, err := f.newClient(transport.NormalizeDaemonURL(f.daemonAddr))
	if err != nil {
		return false, fmt.Errorf("failed to connect to daemon: %w", err)
	}
	return client.ResolvePermission(ctx, toolUseID, toolName, decision)
}

// Run is the main entry point for the TUI.
// @MX:ANCHOR This function creates the model and starts the bubbletea program.
func Run(addr string, noColor bool) error {
	return RunWithApp(nil, addr, noColor)
}

// RunWithApp is the main entry point for the TUI with App integration.
// @MX:ANCHOR This function creates the model with App and starts the bubbletea program.
// @MX:REASON: Called by rootcmd when App is initialized - fan_in >= 2 (rootcmd, tests).
func RunWithApp(app AppInterface, addr string, noColor bool) error {
	// Phase C1 wiring: ConnectClient-backed factory. The legacy gRPC-go
	// factory (NewDaemonClientFactory) remains in this file as a fallback
	// path used by tests and for regression coverage.
	client := NewConnectClientFactory(addr)

	// Create model with App integration
	model := NewModel(client, "", noColor)
	model.app = app
	model.daemonAddr = addr

	// Create and start program
	p := tea.NewProgram(model,
		tea.WithAltScreen(),       // Use alternate screen
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("goose: failed to start TUI: %w", err)
	}

	return nil
}
