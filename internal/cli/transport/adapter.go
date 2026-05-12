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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1"
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

// TranslatedChatEvent is the simplified two-field event shape consumed by
// the commands layer (commands.StreamEvent has the same structure). The
// type is duplicated here to keep transport free of a back-edge import on
// commands. Callers in cli package map TranslatedChatEvent → commands.StreamEvent.
type TranslatedChatEvent struct {
	Type    string
	Content string
}

// chatTextPayload mirrors the {"text": "..."} envelope emitted by the
// daemon for a "text" event.
type chatTextPayload struct {
	Text string `json:"text"`
}

// chatErrorPayload mirrors the {"message": "..."} or {"error": "..."}
// envelope emitted by the daemon for an "error" event.
type chatErrorPayload struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// TranslateChatEvent converts a wire ChatStreamEvent into the simplified
// two-field shape used by ask/tui. The boolean return is true when the
// event should be dropped (Phase B does not surface tool_use yet).
//
// @MX:NOTE Translation is shared between ask command and tui chat panel
// to keep payload-handling in one place.
func TranslateChatEvent(ev ChatStreamEvent) (TranslatedChatEvent, bool) {
	switch ev.Type {
	case "text":
		var p chatTextPayload
		if err := json.Unmarshal(ev.PayloadJSON, &p); err == nil && p.Text != "" {
			return TranslatedChatEvent{Type: "text", Content: p.Text}, false
		}
		return TranslatedChatEvent{Type: "text", Content: string(ev.PayloadJSON)}, false
	case "error":
		var p chatErrorPayload
		if err := json.Unmarshal(ev.PayloadJSON, &p); err == nil {
			if p.Message != "" {
				return TranslatedChatEvent{Type: "error", Content: p.Message}, false
			}
			if p.Error != "" {
				return TranslatedChatEvent{Type: "error", Content: p.Error}, false
			}
		}
		return TranslatedChatEvent{Type: "error", Content: string(ev.PayloadJSON)}, false
	case "done":
		return TranslatedChatEvent{Type: "done", Content: ""}, false
	case "tool_use":
		return TranslatedChatEvent{}, true
	default:
		return TranslatedChatEvent{Type: ev.Type, Content: string(ev.PayloadJSON)}, false
	}
}

// PickLastUserMessage returns the Content of the last "user" role message
// in the slice. If none has role "user", the last element's Content is
// returned. The empty slice yields ("", false).
//
// Phase B simplification: AgentService.ChatStream takes a single user
// message string; multi-turn replay is Phase C scope.
func PickLastUserMessage(messages []ChatMessageView) (string, bool) {
	if len(messages) == 0 {
		return "", false
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content, true
		}
	}
	return messages[len(messages)-1].Content, true
}

// ChatMessageView is a transport-agnostic view of a chat message, used by
// PickLastUserMessage to remain free of an import on commands.
type ChatMessageView struct {
	Role    string
	Content string
}

// ChatStreamFanIn drains the (events, errors) pair returned by
// ConnectClient.ChatStream into a single channel of TranslatedChatEvent.
// A trailing error from errCh is emitted as a synthetic
// TranslatedChatEvent{Type: "error"} so callers can keep a single
// for-range loop. The returned channel is closed when both upstream
// channels are exhausted.
//
// @MX:WARN This goroutine runs until both upstream channels close or ctx
// is cancelled — leaking the goroutine requires both abandonment of the
// returned channel and a never-cancelled ctx.
// @MX:REASON SPEC-GOOSE-CLI-001 Phase B2; ConnectClient already documents
// goroutine lifecycle dependent on ctx cancel.
func ChatStreamFanIn(ctx context.Context, rawEvents <-chan ChatStreamEvent, errCh <-chan error) <-chan TranslatedChatEvent {
	out := make(chan TranslatedChatEvent, 16)
	go func() {
		defer close(out)
		for ev := range rawEvents {
			translated, drop := TranslateChatEvent(ev)
			if drop {
				continue
			}
			select {
			case out <- translated:
			case <-ctx.Done():
				return
			}
		}
		if streamErr, ok := <-errCh; ok && streamErr != nil {
			select {
			case out <- TranslatedChatEvent{Type: "error", Content: streamErr.Error()}:
			case <-ctx.Done():
			}
		}
	}()
	return out
}

// SplitMessagesAtLastUser partitions a chat history into the prior turns
// (everything strictly before the last user message) and the trailing
// user prompt content. ok is false only for an empty input slice.
//
// When no message has Role == "user", the function treats the whole slice
// as priors and returns the final element's Content as the prompt — this
// matches PickLastUserMessage's fallback behaviour and keeps callers
// from having to special-case roles other than "user".
//
// Phase B/C wiring used PickLastUserMessage to surface only the last
// user message; this split helper enables multi-turn replay by routing
// everything else through ChatOption WithInitialMessages.
func SplitMessagesAtLastUser(messages []ChatMessageView) (priors []ChatMessageView, lastUser string, ok bool) {
	if len(messages) == 0 {
		return nil, "", false
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			priors = make([]ChatMessageView, 0, i)
			priors = append(priors, messages[:i]...)
			return priors, messages[i].Content, true
		}
	}
	priors = make([]ChatMessageView, 0, len(messages)-1)
	priors = append(priors, messages[:len(messages)-1]...)
	return priors, messages[len(messages)-1].Content, true
}

// WithInitialMessages threads a multi-turn history into an Agent.Chat or
// Agent.ChatStream call. Each ChatMessageView becomes a single-block
// AgentMessage where the block's kind is "text" and the data is the
// {"text": "..."} JSON envelope used by the daemon's text payloads.
//
// Empty slices produce a no-op (chatOptions.initialMessages stays nil).
//
// @MX:NOTE Phase A's connect.go already routes chatOptions.initialMessages
// into the RPC; this builder simply exposes the field through the public
// ChatOption surface so adapters can opt in to multi-turn replay.
func WithInitialMessages(messages []ChatMessageView) ChatOption {
	return func(o *chatOptions) {
		if len(messages) == 0 {
			return
		}
		converted := make([]*minkv1.AgentMessage, 0, len(messages))
		for _, m := range messages {
			payload, err := json.Marshal(map[string]string{"text": m.Content})
			if err != nil {
				// json.Marshal of a string map cannot fail in practice; fall
				// back to a literal envelope so the caller still sees their
				// content on the wire even in pathological cases.
				payload = []byte(`{"text":""}`)
			}
			converted = append(converted, &minkv1.AgentMessage{
				Role: m.Role,
				Content: []*minkv1.AgentContentBlock{{
					Kind:     "text",
					DataJson: payload,
				}},
			})
		}
		o.initialMessages = converted
	}
}

// errEmptyMessages is exported for adapter tests; the commands wrapper in
// the cli package surfaces this when AskCommand is invoked with no input.
var errEmptyMessages = errors.New("ask: empty messages")

// ErrEmptyMessages is the sentinel returned when a chat call is invoked
// with no messages.
func ErrEmptyMessages() error { return errEmptyMessages }

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
