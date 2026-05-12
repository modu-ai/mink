// Package transport provides gRPC client wrapper for daemon communication.
package transport

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Default timeout for daemon connection.
// @MX:NOTE 3-second timeout balances responsiveness and reliability (REQ-CLI-008).
const defaultDialTimeout = 3 * time.Second

// Message represents a chat message.
// @MX:ANCHOR This type is used across CLI and transport layers.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// StreamEvent represents a streaming response event.
// @MX:ANCHOR This type defines the contract for streaming responses.
type StreamEvent struct {
	Type    string // "text", "error", "done"
	Content string
}

// toProto converts Message to proto Message.
func (m *Message) toProto() *minkv1.Message {
	return &minkv1.Message{
		Role:    m.Role,
		Content: m.Content,
	}
}

// DaemonClient wraps gRPC connection to the goose daemon.
// @MX:ANCHOR Ping and ChatStream have high fan-in (called by commands, tests, future services).
type DaemonClient struct {
	conn   *grpc.ClientConn
	client minkv1.DaemonServiceClient
}

// NewDaemonClient creates a new gRPC client connection to the daemon.
// addr format: "host:port" (e.g., "127.0.0.1:9005").
// Empty addr defaults to "127.0.0.1:9005".
// timeout defaults to 3 seconds (REQ-CLI-008).
// @MX:NOTE Dial timeout prevents indefinite hanging (REQ-CLI-008).
func NewDaemonClient(addr string, timeout time.Duration) (*DaemonClient, error) {
	if addr == "" {
		addr = "127.0.0.1:9005"
	}
	if timeout == 0 {
		timeout = defaultDialTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon at %s: %w", addr, err)
	}

	// Verify daemon is reachable within timeout
	_, err = minkv1.NewDaemonServiceClient(conn).Ping(ctx, &minkv1.PingRequest{})
	if err != nil {
		// Best-effort cleanup; the meaningful error is the Ping failure below.
		_ = conn.Close()
		return nil, fmt.Errorf("failed to connect to daemon at %s: %w", addr, err)
	}

	return &DaemonClient{
		conn:   conn,
		client: minkv1.NewDaemonServiceClient(conn),
	}, nil
}

// Ping sends a ping request to the daemon and returns the response.
// @MX:ANCHOR Ping is called by CLI command, health checks, and tests.
func (c *DaemonClient) Ping(ctx context.Context) (*PingResponse, error) {
	resp, err := c.client.Ping(ctx, &minkv1.PingRequest{})
	if err != nil {
		return nil, err
	}
	return &PingResponse{
		Version:  resp.Version,
		UptimeMs: resp.UptimeMs,
		State:    resp.State,
	}, nil
}

// PingResponse is a simplified wrapper for ping response data.
type PingResponse struct {
	Version  string
	UptimeMs int64
	State    string
}

// ChatStream opens a bidirectional streaming chat with the daemon.
// It sends the provided messages and returns a channel for streaming events.
// @MX:ANCHOR ChatStream is the primary interface for LLM interaction.
func (c *DaemonClient) ChatStream(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}

	stream, err := c.client.ChatStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open chat stream: %w", err)
	}

	// Send first message
	if err := stream.Send(&minkv1.ChatStreamRequest{
		Message: messages[0].toProto(),
	}); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Create event channel
	eventCh := make(chan StreamEvent, 10) // Buffer for async events

	// Start goroutine to receive events
	go func() {
		defer close(eventCh)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				// Server closed stream normally
				eventCh <- StreamEvent{Type: "done", Content: ""}
				return
			}
			if err != nil {
				// Error receiving
				eventCh <- StreamEvent{Type: "error", Content: err.Error()}
				return
			}

			// Process response based on event type
			switch ev := resp.Event.(type) {
			case *minkv1.ChatStreamResponse_Text:
				eventCh <- StreamEvent{Type: "text", Content: ev.Text.Content}
			case *minkv1.ChatStreamResponse_Error:
				eventCh <- StreamEvent{Type: "error", Content: ev.Error.Message}
			case *minkv1.ChatStreamResponse_Done:
				eventCh <- StreamEvent{Type: "done", Content: ""}
				return
			default:
				eventCh <- StreamEvent{Type: "error", Content: "unknown event type"}
			}
		}
	}()

	return eventCh, nil
}

// ResolvePermission records the user's decision for a tool permission request.
// SPEC-GOOSE-CLI-TUI-002 P3 — stub returns accepted=true always.
// TODO(QUERY-001): wire to engine.ResolvePermission when available.
func (c *DaemonClient) ResolvePermission(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

// Close closes the gRPC connection to the daemon.
// Safe to call multiple times.
func (c *DaemonClient) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil // Prevent double-close
		return err
	}
	return nil
}
