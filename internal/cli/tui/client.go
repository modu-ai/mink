// Package tui provides the interactive chat TUI using bubbletea.
package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modu-ai/goose/internal/cli/transport"
)

// daemonClientAdapter wraps transport.DaemonClient to implement tui.DaemonClient.
// @MX:ANCHOR This adapter bridges the TUI and transport layers.
type daemonClientAdapter struct {
	transport *transport.DaemonClient
}

// NewDaemonClientAdapter creates a new TUI daemon client from a transport client factory.
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

// Run is the main entry point for the TUI.
// @MX:ANCHOR This function creates the model and starts the bubbletea program.
func Run(addr string, noColor bool) error {
	// Create daemon client factory
	clientFactory := NewDaemonClientFactory(func(daemonAddr string, timeout int) (*transport.DaemonClient, error) {
		return transport.NewDaemonClient(daemonAddr, time.Duration(timeout)*time.Second)
	})

	// Create model
	model := NewModel(clientFactory, "", noColor)
	model.daemonAddr = addr

	// Create and start program
	p := tea.NewProgram(model,
		tea.WithAltScreen(),        // Use alternate screen
		tea.WithMouseCellMotion(),  // Enable mouse support
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("goose: failed to start TUI: %w", err)
	}

	return nil
}
