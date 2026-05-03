// SPEC-GOOSE-CLI-001 Phase B1 — PingClientAdapter tests.
// Reuses mockDaemonConnectServer + newTestServer from connect_test.go.
package transport

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1"
)

// RED #1: PingClientAdapter writes a byte-identical status line for a
// successful Ping response.
func TestPingClientAdapter_Ping_Success(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&mockDaemonConnectServer{
		pingFunc: func(_ context.Context, _ *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error) {
			return connect.NewResponse(&goosev1.PingResponse{
				Version:  "v0.2.0",
				UptimeMs: 12345,
				State:    "serving",
			}), nil
		},
	}, nil, nil, nil)
	defer srv.Close()

	adapter := &PingClientAdapter{
		newClient: func(_ string, opts ...ConnectOption) (*ConnectClient, error) {
			return NewConnectClient(srv.URL, opts...)
		},
	}

	var buf bytes.Buffer
	if err := adapter.Ping(context.Background(), "127.0.0.1:9005", &buf); err != nil {
		t.Fatalf("adapter.Ping: %v", err)
	}

	got := buf.String()
	want := "pong (version=v0.2.0, state=serving, uptime=12345ms)\n"
	if got != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", got, want)
	}
}

// RED #2: PingClientAdapter propagates context cancellation.
func TestPingClientAdapter_Ping_Timeout(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&mockDaemonConnectServer{
		pingFunc: func(ctx context.Context, _ *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}, nil, nil, nil)
	defer srv.Close()

	adapter := &PingClientAdapter{
		newClient: func(_ string, opts ...ConnectOption) (*ConnectClient, error) {
			return NewConnectClient(srv.URL, opts...)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	err := adapter.Ping(ctx, "127.0.0.1:9005", &buf)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "ping failed") {
		t.Errorf("expected wrapped 'ping failed' error, got: %v", err)
	}
}

// RED #3 (RPC error path): PingClientAdapter wraps connect errors and writes
// nothing to the output stream when Ping fails.
func TestPingClientAdapter_Ping_RPCError(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&mockDaemonConnectServer{
		pingFunc: func(_ context.Context, _ *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error) {
			return nil, connect.NewError(connect.CodeInternal, errors.New("boom"))
		},
	}, nil, nil, nil)
	defer srv.Close()

	adapter := &PingClientAdapter{
		newClient: func(_ string, opts ...ConnectOption) (*ConnectClient, error) {
			return NewConnectClient(srv.URL, opts...)
		},
	}

	var buf bytes.Buffer
	err := adapter.Ping(context.Background(), "127.0.0.1:9005", &buf)
	if err == nil {
		t.Fatal("expected RPC error, got nil")
	}
	if !strings.Contains(err.Error(), "ping failed") {
		t.Errorf("expected wrapped 'ping failed' error, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("writer should be empty on error, got: %q", buf.String())
	}
}

// PingClientAdapter surfaces factory failures to callers.
func TestPingClientAdapter_Ping_FactoryError(t *testing.T) {
	t.Parallel()

	adapter := &PingClientAdapter{
		newClient: func(_ string, _ ...ConnectOption) (*ConnectClient, error) {
			return nil, errors.New("dial blocked")
		},
	}

	var buf bytes.Buffer
	err := adapter.Ping(context.Background(), "127.0.0.1:9005", &buf)
	if err == nil {
		t.Fatal("expected factory error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to connect") {
		t.Errorf("expected 'failed to connect' wrap, got: %v", err)
	}
}

// NewPingClientAdapter returns a non-nil adapter wired to NewConnectClient.
func TestNewPingClientAdapter_DefaultsToNewConnectClient(t *testing.T) {
	t.Parallel()

	adapter := NewPingClientAdapter()
	if adapter == nil {
		t.Fatal("NewPingClientAdapter returned nil")
	}
	if adapter.newClient == nil {
		t.Fatal("default factory must not be nil")
	}
}

// RED #4 (text path): TranslateChatEvent extracts the JSON-encoded
// {"text": ...} payload into a flat StreamEvent.
func TestTranslateChatEvent_TextPayloadExtracted(t *testing.T) {
	t.Parallel()

	ev := ChatStreamEvent{Type: "text", PayloadJSON: []byte(`{"text":"hello world"}`)}
	got, drop := TranslateChatEvent(ev)
	if drop {
		t.Fatal("text events must not be dropped")
	}
	if got.Type != "text" || got.Content != "hello world" {
		t.Errorf("unexpected translation: %+v", got)
	}
}

// TranslateChatEvent falls back to the raw JSON when the text envelope is
// missing the expected key.
func TestTranslateChatEvent_TextPayloadFallback(t *testing.T) {
	t.Parallel()

	ev := ChatStreamEvent{Type: "text", PayloadJSON: []byte(`{"unknown":"value"}`)}
	got, drop := TranslateChatEvent(ev)
	if drop {
		t.Fatal("text events must not be dropped")
	}
	if got.Type != "text" || got.Content != `{"unknown":"value"}` {
		t.Errorf("expected raw fallback, got %+v", got)
	}
}

// TranslateChatEvent prefers {"message"} over {"error"} for error envelopes.
func TestTranslateChatEvent_ErrorPayloadVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"message field", `{"message":"boom"}`, "boom"},
		{"error field", `{"error":"kaput"}`, "kaput"},
		{"raw fallback", `oops`, "oops"},
	}
	for _, tc := range cases {
		ev := ChatStreamEvent{Type: "error", PayloadJSON: []byte(tc.payload)}
		got, _ := TranslateChatEvent(ev)
		if got.Type != "error" {
			t.Errorf("%s: expected error type, got %q", tc.name, got.Type)
		}
		if got.Content != tc.want {
			t.Errorf("%s: content = %q, want %q", tc.name, got.Content, tc.want)
		}
	}
}

// TranslateChatEvent drops tool_use frames in Phase B.
func TestTranslateChatEvent_ToolUseDropped(t *testing.T) {
	t.Parallel()

	ev := ChatStreamEvent{Type: "tool_use", PayloadJSON: []byte(`{"name":"x"}`)}
	if _, drop := TranslateChatEvent(ev); !drop {
		t.Fatal("tool_use must be dropped")
	}
}

// TranslateChatEvent leaves done events with empty content.
func TestTranslateChatEvent_Done(t *testing.T) {
	t.Parallel()

	ev := ChatStreamEvent{Type: "done", PayloadJSON: []byte(`{}`)}
	got, drop := TranslateChatEvent(ev)
	if drop {
		t.Fatal("done must not be dropped")
	}
	if got.Type != "done" || got.Content != "" {
		t.Errorf("unexpected done translation: %+v", got)
	}
}

// PickLastUserMessage prefers the last role=user entry.
func TestPickLastUserMessage_PrefersLastUser(t *testing.T) {
	t.Parallel()

	msgs := []ChatMessageView{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "ack"},
		{Role: "user", Content: "second"},
		{Role: "assistant", Content: "ack2"},
	}
	got, ok := PickLastUserMessage(msgs)
	if !ok || got != "second" {
		t.Errorf("got (%q,%v), want (%q,true)", got, ok, "second")
	}
}

// PickLastUserMessage falls back to the trailing element when no user role exists.
func TestPickLastUserMessage_FallbackToTail(t *testing.T) {
	t.Parallel()

	msgs := []ChatMessageView{
		{Role: "system", Content: "sys"},
		{Role: "assistant", Content: "ack"},
	}
	got, ok := PickLastUserMessage(msgs)
	if !ok || got != "ack" {
		t.Errorf("got (%q,%v), want (%q,true)", got, ok, "ack")
	}
}

// PickLastUserMessage on an empty slice returns ("", false).
func TestPickLastUserMessage_Empty(t *testing.T) {
	t.Parallel()

	got, ok := PickLastUserMessage(nil)
	if ok || got != "" {
		t.Errorf("got (%q,%v), want (\"\",false)", got, ok)
	}
}

// RED #5: ChatStreamFanIn forwards translated events and emits a synthetic
// error event when the upstream errCh delivers a non-nil error.
func TestChatStreamFanIn_ForwardsAndEmitsError(t *testing.T) {
	t.Parallel()

	rawEvents := make(chan ChatStreamEvent, 3)
	errCh := make(chan error, 1)

	rawEvents <- ChatStreamEvent{Type: "text", PayloadJSON: []byte(`{"text":"a"}`)}
	rawEvents <- ChatStreamEvent{Type: "tool_use", PayloadJSON: []byte(`{}`)}
	rawEvents <- ChatStreamEvent{Type: "text", PayloadJSON: []byte(`{"text":"b"}`)}
	close(rawEvents)
	errCh <- errors.New("stream busted")
	close(errCh)

	out := ChatStreamFanIn(context.Background(), rawEvents, errCh)

	var got []TranslatedChatEvent
	for ev := range out {
		got = append(got, ev)
	}

	want := []TranslatedChatEvent{
		{Type: "text", Content: "a"},
		{Type: "text", Content: "b"},
		{Type: "error", Content: "stream busted"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// ChatStreamFanIn closes the output channel even when ctx is cancelled
// mid-stream.
func TestChatStreamFanIn_CtxCancel(t *testing.T) {
	t.Parallel()

	rawEvents := make(chan ChatStreamEvent)
	errCh := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	out := ChatStreamFanIn(ctx, rawEvents, errCh)

	cancel()

	// Without anything closing the upstream channels, the fan-in goroutine
	// stays blocked on rawEvents read; close them so the goroutine exits.
	close(rawEvents)
	close(errCh)

	// Drain — must close cleanly.
	for range out {
	}
}

// ErrEmptyMessages exposes the sentinel used by AskClientAdapter callers.
func TestErrEmptyMessages_Sentinel(t *testing.T) {
	t.Parallel()

	if ErrEmptyMessages() == nil {
		t.Fatal("expected non-nil sentinel")
	}
	if ErrEmptyMessages().Error() == "" {
		t.Fatal("sentinel must carry a message")
	}
}

// NormalizeDaemonURL prepends http:// only when scheme is missing.
func TestNormalizeDaemonURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"127.0.0.1:9005", "http://127.0.0.1:9005"},
		{"localhost:8080", "http://localhost:8080"},
		{"http://127.0.0.1:9005", "http://127.0.0.1:9005"},
		{"https://daemon.example:443", "https://daemon.example:443"},
	}

	for _, tc := range cases {
		if got := NormalizeDaemonURL(tc.in); got != tc.want {
			t.Errorf("NormalizeDaemonURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
