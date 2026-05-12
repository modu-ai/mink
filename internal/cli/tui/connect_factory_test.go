// SPEC-GOOSE-CLI-001 Phase C1 — connectClientFactory tests.
package tui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/modu-ai/mink/internal/cli/transport"
	"github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1"
	"github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1/minkv1connect"
)

// mockAgentForTUI is a minimal AgentService stub serving a scripted stream.
type mockAgentForTUI struct {
	chatFn       func(context.Context, *connect.Request[minkv1.AgentChatRequest]) (*connect.Response[minkv1.AgentChatResponse], error)
	chatStreamFn func(context.Context, *connect.Request[minkv1.AgentChatStreamRequest], *connect.ServerStream[minkv1.AgentChatStreamEvent]) error
}

func (m *mockAgentForTUI) Chat(ctx context.Context, req *connect.Request[minkv1.AgentChatRequest]) (*connect.Response[minkv1.AgentChatResponse], error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, req)
	}
	return connect.NewResponse(&minkv1.AgentChatResponse{}), nil
}

func (m *mockAgentForTUI) ChatStream(ctx context.Context, req *connect.Request[minkv1.AgentChatStreamRequest], stream *connect.ServerStream[minkv1.AgentChatStreamEvent]) error {
	if m.chatStreamFn != nil {
		return m.chatStreamFn(ctx, req, stream)
	}
	return nil
}

func (m *mockAgentForTUI) ResolvePermission(_ context.Context, _ *connect.Request[minkv1.ResolvePermissionRequest]) (*connect.Response[minkv1.ResolvePermissionResponse], error) {
	return connect.NewResponse(&minkv1.ResolvePermissionResponse{Accepted: true}), nil
}

// startAgentServer spins up an httptest server hosting only the AgentService
// handler — sufficient for connectClientFactory exercising ChatStream.
func startAgentServer(t *testing.T, agent minkv1connect.AgentServiceHandler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	path, h := minkv1connect.NewAgentServiceHandler(agent)
	mux.Handle(path, h)
	return httptest.NewServer(mux)
}

// RED #1: connectClientFactory streams text events end-to-end via
// ConnectClient and surfaces them as tui.StreamEvent values.
func TestConnectClientFactory_ChatStream_StreamingEvents(t *testing.T) {
	t.Parallel()

	srv := startAgentServer(t, &mockAgentForTUI{
		chatStreamFn: func(_ context.Context, req *connect.Request[minkv1.AgentChatStreamRequest], stream *connect.ServerStream[minkv1.AgentChatStreamEvent]) error {
			if req.Msg.Message == "" {
				t.Errorf("AgentChatStreamRequest.Message must not be empty")
			}
			if err := stream.Send(&minkv1.AgentChatStreamEvent{Type: "text", PayloadJson: []byte(`{"text":"hello"}`)}); err != nil {
				return err
			}
			if err := stream.Send(&minkv1.AgentChatStreamEvent{Type: "text", PayloadJson: []byte(`{"text":" world"}`)}); err != nil {
				return err
			}
			return stream.Send(&minkv1.AgentChatStreamEvent{Type: "done", PayloadJson: []byte(`{}`)})
		},
	})
	defer srv.Close()

	factory := &connectClientFactory{
		daemonAddr: srv.URL,
		newClient:  transport.NewConnectClient,
	}

	out, err := factory.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var events []StreamEvent
	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev, ok := <-out:
			if !ok {
				goto done
			}
			events = append(events, ev)
		case <-timeout:
			t.Fatal("timed out draining stream")
		}
	}
done:

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3: %+v", len(events), events)
	}
	if events[0].Type != "text" || events[0].Content != "hello" {
		t.Errorf("event[0] = %+v, want {text, hello}", events[0])
	}
	if events[1].Type != "text" || events[1].Content != " world" {
		t.Errorf("event[1] = %+v, want {text,  world}", events[1])
	}
	if events[2].Type != "done" {
		t.Errorf("event[2] = %+v, want type=done", events[2])
	}
}

// RED #2: connectClientFactory propagates server-side error frames.
func TestConnectClientFactory_ChatStream_ErrorEvent(t *testing.T) {
	t.Parallel()

	srv := startAgentServer(t, &mockAgentForTUI{
		chatStreamFn: func(_ context.Context, _ *connect.Request[minkv1.AgentChatStreamRequest], stream *connect.ServerStream[minkv1.AgentChatStreamEvent]) error {
			return stream.Send(&minkv1.AgentChatStreamEvent{Type: "error", PayloadJson: []byte(`{"message":"upstream timeout"}`)})
		},
	})
	defer srv.Close()

	factory := &connectClientFactory{
		daemonAddr: srv.URL,
		newClient:  transport.NewConnectClient,
	}

	out, err := factory.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var events []StreamEvent
	for ev := range out {
		events = append(events, ev)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].Type != "error" || events[0].Content != "upstream timeout" {
		t.Errorf("event[0] = %+v, want {error, upstream timeout}", events[0])
	}
}

// connectClientFactory rejects empty message slices via the shared sentinel.
func TestConnectClientFactory_ChatStream_EmptyMessages(t *testing.T) {
	t.Parallel()

	factory := NewConnectClientFactory("127.0.0.1:9005")
	_, err := factory.ChatStream(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
	if !errors.Is(err, transport.ErrEmptyMessages()) {
		t.Errorf("expected ErrEmptyMessages sentinel, got %v", err)
	}
}

// connectClientFactory wraps factory failures with "failed to connect".
func TestConnectClientFactory_ChatStream_FactoryError(t *testing.T) {
	t.Parallel()

	factory := &connectClientFactory{
		daemonAddr: "127.0.0.1:9005",
		newClient: func(_ string, _ ...transport.ConnectOption) (*transport.ConnectClient, error) {
			return nil, errors.New("dial blocked")
		},
	}

	_, err := factory.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected factory error")
	}
}

// Close on a lazy factory is a no-op.
func TestConnectClientFactory_Close_NoOp(t *testing.T) {
	t.Parallel()

	factory := NewConnectClientFactory("127.0.0.1:9005")
	if err := factory.Close(); err != nil {
		t.Errorf("Close should be a no-op, got %v", err)
	}
}

// NewConnectClientFactory returns a non-nil factory wired to NewConnectClient.
func TestNewConnectClientFactory_Defaults(t *testing.T) {
	t.Parallel()

	factory := NewConnectClientFactory("daemon:9005")
	if factory == nil {
		t.Fatal("NewConnectClientFactory returned nil")
	}
	c, ok := factory.(*connectClientFactory)
	if !ok {
		t.Fatalf("unexpected concrete type %T", factory)
	}
	if c.daemonAddr != "daemon:9005" {
		t.Errorf("daemonAddr = %q, want %q", c.daemonAddr, "daemon:9005")
	}
	if c.newClient == nil {
		t.Fatal("default newClient must not be nil")
	}
}
