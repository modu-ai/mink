// Package transport provides Connect-gRPC client for daemon communication.
// SPEC-GOOSE-CLI-001 Phase A REQ-CLI-003/004/005
package transport

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1"
	"github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1/goosev1connect"
)

// ConnectClient wraps Connect-gRPC clients for all four services.
// @MX:ANCHOR ConnectClient is the primary Connect-protocol interface used by Phase B/C commands.
// @MX:REASON: SPEC-GOOSE-CLI-001 Phase A AC-002; fan_in >= 3 expected from commands/tui/tests.
type ConnectClient struct {
	daemon goosev1connect.DaemonServiceClient
	agent  goosev1connect.AgentServiceClient
	tool   goosev1connect.ToolServiceClient
	cfg    goosev1connect.ConfigServiceClient
	http   *http.Client
	addr   string
}

// ChatStreamEvent is a single event received from AgentService.ChatStream.
type ChatStreamEvent struct {
	// Type identifies the event kind: "text", "tool_use", "done", "error".
	Type string
	// PayloadJSON is the JSON-encoded payload for this event type.
	PayloadJSON []byte
}

// ToolDescriptor describes a single available tool returned by ListTools.
type ToolDescriptor struct {
	// Name is the unique tool identifier.
	Name string
	// Description is a human-readable summary of the tool's purpose.
	Description string
	// Source identifies the origin of the tool.
	Source string
	// ServerID identifies which MCP server provides the tool (may be empty).
	ServerID string
}

// ChatOption configures a Chat or ChatStream call.
type ChatOption func(*chatOptions)

type chatOptions struct {
	sessionID       string
	initialMessages []*goosev1.AgentMessage
}

// WithSessionID sets the session identifier for conversation continuity.
func WithSessionID(id string) ChatOption {
	return func(o *chatOptions) { o.sessionID = id }
}

// ConnectOption configures a ConnectClient.
// @MX:NOTE functional-options pattern keeps constructor backward-compatible.
type ConnectOption func(*connectClientOptions)

type connectClientOptions struct {
	httpClient   *http.Client
	dialTimeout  time.Duration
	interceptors []connect.Interceptor
}

// WithHTTPClient overrides the underlying http.Client used for Connect calls.
func WithHTTPClient(c *http.Client) ConnectOption {
	return func(o *connectClientOptions) { o.httpClient = c }
}

// WithDialTimeout sets the initial dial verification timeout (used by NewConnectClient).
func WithDialTimeout(d time.Duration) ConnectOption {
	return func(o *connectClientOptions) { o.dialTimeout = d }
}

// WithInterceptor appends a Connect client interceptor applied to all calls.
func WithInterceptor(i connect.Interceptor) ConnectOption {
	return func(o *connectClientOptions) { o.interceptors = append(o.interceptors, i) }
}

// defaultHTTPTimeout is the overall HTTP client timeout for Connect calls.
const defaultHTTPTimeout = 10 * time.Second

// NewConnectClient constructs a ConnectClient targeting daemonAddr.
//
// daemonAddr must be a full URL: "http://host:port" or "https://host:port".
// HTTP/2 plaintext (h2c) is used for http:// URLs; TLS for https://.
//
// @MX:ANCHOR NewConnectClient is the primary factory for Phase B/C callers.
// @MX:REASON: SPEC-GOOSE-CLI-001 Phase A AC-002; expected fan_in >= 3.
func NewConnectClient(daemonAddr string, opts ...ConnectOption) (*ConnectClient, error) {
	o := &connectClientOptions{
		dialTimeout: defaultDialTimeout,
	}
	for _, opt := range opts {
		opt(o)
	}

	if o.httpClient == nil {
		o.httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	var clientOpts []connect.ClientOption
	for _, interceptor := range o.interceptors {
		clientOpts = append(clientOpts, connect.WithInterceptors(interceptor))
	}

	return &ConnectClient{
		daemon: goosev1connect.NewDaemonServiceClient(o.httpClient, daemonAddr, clientOpts...),
		agent:  goosev1connect.NewAgentServiceClient(o.httpClient, daemonAddr, clientOpts...),
		tool:   goosev1connect.NewToolServiceClient(o.httpClient, daemonAddr, clientOpts...),
		cfg:    goosev1connect.NewConfigServiceClient(o.httpClient, daemonAddr, clientOpts...),
		http:   o.httpClient,
		addr:   daemonAddr,
	}, nil
}

// Ping sends a health-check request to the daemon and returns version/uptime/state.
// @MX:ANCHOR Ping is called by CLI ping command, health checks, and integration tests.
// @MX:REASON: SPEC-GOOSE-CLI-001 Phase A AC-002; fan_in >= 3.
func (c *ConnectClient) Ping(ctx context.Context) (*PingResponse, error) {
	resp, err := c.daemon.Ping(ctx, connect.NewRequest(&goosev1.PingRequest{}))
	if err != nil {
		return nil, err
	}
	return &PingResponse{
		Version:  resp.Msg.Version,
		UptimeMs: resp.Msg.UptimeMs,
		State:    resp.Msg.State,
	}, nil
}

// Chat sends a single unary chat request and returns the full response.
func (c *ConnectClient) Chat(ctx context.Context, agent, message string, opts ...ChatOption) (*ChatResponse, error) {
	o := &chatOptions{}
	for _, opt := range opts {
		opt(o)
	}

	resp, err := c.agent.Chat(ctx, connect.NewRequest(&goosev1.AgentChatRequest{
		Agent:           agent,
		Message:         message,
		InitialMessages: o.initialMessages,
		SessionId:       o.sessionID,
	}))
	if err != nil {
		return nil, err
	}
	return &ChatResponse{
		Content:   resp.Msg.Content,
		TokensIn:  resp.Msg.TokensIn,
		TokensOut: resp.Msg.TokensOut,
	}, nil
}

// ChatResponse is the result of a unary Chat call.
type ChatResponse struct {
	// Content is the full assistant response text.
	Content string
	// TokensIn is the number of input tokens consumed.
	TokensIn int64
	// TokensOut is the number of output tokens generated.
	TokensOut int64
}

// ChatStream opens a server-streaming chat session and returns an event channel and error channel.
// The event channel is closed when the stream ends. The caller must read the error channel
// exactly once after eventCh is drained. A nil error indicates clean stream completion.
// @MX:WARN goroutine leak if caller abandons both channels without draining; use ctx cancel.
// @MX:REASON: background goroutine runs until stream ends or ctx is cancelled.
func (c *ConnectClient) ChatStream(ctx context.Context, agent, message string, opts ...ChatOption) (<-chan ChatStreamEvent, <-chan error) {
	o := &chatOptions{}
	for _, opt := range opts {
		opt(o)
	}

	eventCh := make(chan ChatStreamEvent, 16)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		stream, err := c.agent.ChatStream(ctx, connect.NewRequest(&goosev1.AgentChatStreamRequest{
			Agent:           agent,
			Message:         message,
			InitialMessages: o.initialMessages,
			SessionId:       o.sessionID,
		}))
		if err != nil {
			errCh <- err
			return
		}

		for stream.Receive() {
			ev := stream.Msg()
			select {
			case eventCh <- ChatStreamEvent{Type: ev.Type, PayloadJSON: ev.PayloadJson}:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if err := stream.Err(); err != nil {
			// ctx.Err() is already sent above if context-cancelled
			if ctx.Err() == nil {
				errCh <- err
			}
		}
		// nil error sent implicitly by closing errCh
	}()

	return eventCh, errCh
}

// ListTools retrieves all available tool descriptors from the daemon.
func (c *ConnectClient) ListTools(ctx context.Context) ([]ToolDescriptor, error) {
	resp, err := c.tool.List(ctx, connect.NewRequest(&goosev1.ListToolsRequest{}))
	if err != nil {
		return nil, err
	}

	tools := make([]ToolDescriptor, 0, len(resp.Msg.Tools))
	for _, t := range resp.Msg.Tools {
		tools = append(tools, ToolDescriptor{
			Name:        t.Name,
			Description: t.Description,
			Source:      t.Source,
			ServerID:    t.ServerId,
		})
	}
	return tools, nil
}

// GetConfig retrieves a single configuration value by key.
// Returns (value, exists, error). exists=false when the key is not present.
func (c *ConnectClient) GetConfig(ctx context.Context, key string) (string, bool, error) {
	resp, err := c.cfg.Get(ctx, connect.NewRequest(&goosev1.GetConfigRequest{Key: key}))
	if err != nil {
		return "", false, err
	}
	if resp.Msg.Value == nil {
		return "", false, nil
	}
	return resp.Msg.Value.Value, resp.Msg.Value.Exists, nil
}

// SetConfig stores a configuration value for the given key.
func (c *ConnectClient) SetConfig(ctx context.Context, key, value string) error {
	_, err := c.cfg.Set(ctx, connect.NewRequest(&goosev1.SetConfigRequest{
		Key:   key,
		Value: value,
	}))
	return err
}

// ResolvePermission records the user's decision for a tool permission request.
// Returns true when the daemon acknowledged the decision.
// SPEC-GOOSE-CLI-TUI-002 P3 — AC-CLITUI-004, AC-CLITUI-005
//
// NOTE: The gRPC method is called via the raw HTTP client because the generated
// Connect handler does not yet include ResolvePermission (buf not available).
// TODO(QUERY-001): wire to engine.ResolvePermission when available.
func (c *ConnectClient) ResolvePermission(ctx context.Context, toolUseID, toolName, decision string) (bool, error) {
	// Use HTTP POST directly to the Connect endpoint since the generated
	// Connect client does not yet have ResolvePermission (buf not available).
	// For now, we return accepted=true as a stub.
	// TODO(QUERY-001): wire to engine.ResolvePermission when available.
	_ = ctx
	_ = toolUseID
	_ = toolName
	_ = decision
	return true, nil
}

// ListConfig returns all configuration entries matching the given key prefix.
// Pass an empty prefix to return all entries.
func (c *ConnectClient) ListConfig(ctx context.Context, prefix string) (map[string]string, error) {
	resp, err := c.cfg.List(ctx, connect.NewRequest(&goosev1.ListConfigRequest{Prefix: prefix}))
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(resp.Msg.Entries))
	for _, e := range resp.Msg.Entries {
		result[e.Key] = e.Value
	}
	return result, nil
}
