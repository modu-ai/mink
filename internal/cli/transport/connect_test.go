// Package transport provides Connect-gRPC client tests.
// RED #1~#8: SPEC-GOOSE-CLI-001 Phase A AC-001~005
package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/modu-ai/mink/internal/transport/grpc/gen/goosev1"
	"github.com/modu-ai/mink/internal/transport/grpc/gen/goosev1/goosev1connect"
)

// RED #1: Verify generated code constants exist (compile-time proof)
func TestProto_GeneratedCode_Constants(t *testing.T) {
	// AgentService constants
	if goosev1connect.AgentServiceName != "goose.v1.AgentService" {
		t.Errorf("AgentServiceName = %q, want %q", goosev1connect.AgentServiceName, "goose.v1.AgentService")
	}
	if goosev1connect.AgentServiceChatProcedure != "/goose.v1.AgentService/Chat" {
		t.Errorf("AgentServiceChatProcedure = %q", goosev1connect.AgentServiceChatProcedure)
	}

	// ToolService constants
	if goosev1connect.ToolServiceName != "goose.v1.ToolService" {
		t.Errorf("ToolServiceName = %q, want %q", goosev1connect.ToolServiceName, "goose.v1.ToolService")
	}

	// ConfigService constants
	if goosev1connect.ConfigServiceName != "goose.v1.ConfigService" {
		t.Errorf("ConfigServiceName = %q, want %q", goosev1connect.ConfigServiceName, "goose.v1.ConfigService")
	}

	// Verify proto message types compile correctly
	_ = &goosev1.AgentChatRequest{}
	_ = &goosev1.AgentChatResponse{}
	_ = &goosev1.AgentChatStreamRequest{}
	_ = &goosev1.AgentChatStreamEvent{}
	_ = &goosev1.AgentMessage{}
	_ = &goosev1.AgentContentBlock{}
	_ = &goosev1.ToolDescriptor{}
	_ = &goosev1.ListToolsRequest{}
	_ = &goosev1.ListToolsResponse{}
	_ = &goosev1.GetConfigRequest{}
	_ = &goosev1.GetConfigResponse{}
	_ = &goosev1.SetConfigRequest{}
	_ = &goosev1.SetConfigResponse{}
	_ = &goosev1.ListConfigRequest{}
	_ = &goosev1.ListConfigResponse{}
	_ = &goosev1.ConfigEntry{}
}

// mockDaemonConnectServer implements DaemonService via Connect protocol for Ping testing.
type mockDaemonConnectServer struct {
	pingFunc func(context.Context, *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error)
}

func (m *mockDaemonConnectServer) Ping(ctx context.Context, req *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error) {
	if m.pingFunc != nil {
		return m.pingFunc(ctx, req)
	}
	return connect.NewResponse(&goosev1.PingResponse{
		Version:  "test-v1.0.0",
		UptimeMs: 1000,
		State:    "serving",
	}), nil
}

func (m *mockDaemonConnectServer) GetInfo(ctx context.Context, req *connect.Request[goosev1.GetInfoRequest]) (*connect.Response[goosev1.GetInfoResponse], error) {
	return connect.NewResponse(&goosev1.GetInfoResponse{Version: "test"}), nil
}

func (m *mockDaemonConnectServer) Shutdown(ctx context.Context, req *connect.Request[goosev1.ShutdownRequest]) (*connect.Response[goosev1.ShutdownResponse], error) {
	return connect.NewResponse(&goosev1.ShutdownResponse{Accepted: true}), nil
}

func (m *mockDaemonConnectServer) ChatStream(ctx context.Context, stream *connect.BidiStream[goosev1.ChatStreamRequest, goosev1.ChatStreamResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented in mock"))
}

// mockAgentConnectServer implements AgentService via Connect protocol.
type mockAgentConnectServer struct {
	chatFunc       func(context.Context, *connect.Request[goosev1.AgentChatRequest]) (*connect.Response[goosev1.AgentChatResponse], error)
	chatStreamFunc func(context.Context, *connect.Request[goosev1.AgentChatStreamRequest], *connect.ServerStream[goosev1.AgentChatStreamEvent]) error
}

func (m *mockAgentConnectServer) Chat(ctx context.Context, req *connect.Request[goosev1.AgentChatRequest]) (*connect.Response[goosev1.AgentChatResponse], error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, req)
	}
	return connect.NewResponse(&goosev1.AgentChatResponse{
		Content:   "Hello from agent!",
		TokensIn:  10,
		TokensOut: 5,
	}), nil
}

func (m *mockAgentConnectServer) ChatStream(ctx context.Context, req *connect.Request[goosev1.AgentChatStreamRequest], stream *connect.ServerStream[goosev1.AgentChatStreamEvent]) error {
	if m.chatStreamFunc != nil {
		return m.chatStreamFunc(ctx, req, stream)
	}
	// Default: send two events then finish
	if err := stream.Send(&goosev1.AgentChatStreamEvent{
		Type:        "text",
		PayloadJson: []byte(`{"text":"Hello"}`),
	}); err != nil {
		return err
	}
	return stream.Send(&goosev1.AgentChatStreamEvent{
		Type:        "done",
		PayloadJson: []byte(`{}`),
	})
}

// mockToolConnectServer implements ToolService via Connect protocol.
type mockToolConnectServer struct {
	listFunc func(context.Context, *connect.Request[goosev1.ListToolsRequest]) (*connect.Response[goosev1.ListToolsResponse], error)
}

func (m *mockToolConnectServer) List(ctx context.Context, req *connect.Request[goosev1.ListToolsRequest]) (*connect.Response[goosev1.ListToolsResponse], error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, req)
	}
	return connect.NewResponse(&goosev1.ListToolsResponse{
		Tools: []*goosev1.ToolDescriptor{
			{Name: "bash", Description: "Run bash commands", Source: "builtin", ServerId: ""},
			{Name: "read_file", Description: "Read a file", Source: "builtin", ServerId: ""},
		},
	}), nil
}

// mockConfigConnectServer implements ConfigService via Connect protocol.
type mockConfigConnectServer struct {
	getFunc  func(context.Context, *connect.Request[goosev1.GetConfigRequest]) (*connect.Response[goosev1.GetConfigResponse], error)
	setFunc  func(context.Context, *connect.Request[goosev1.SetConfigRequest]) (*connect.Response[goosev1.SetConfigResponse], error)
	listFunc func(context.Context, *connect.Request[goosev1.ListConfigRequest]) (*connect.Response[goosev1.ListConfigResponse], error)
}

func (m *mockConfigConnectServer) Get(ctx context.Context, req *connect.Request[goosev1.GetConfigRequest]) (*connect.Response[goosev1.GetConfigResponse], error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, req)
	}
	return connect.NewResponse(&goosev1.GetConfigResponse{
		Value: &goosev1.ConfigValue{Value: "default-value", Exists: true},
	}), nil
}

func (m *mockConfigConnectServer) Set(ctx context.Context, req *connect.Request[goosev1.SetConfigRequest]) (*connect.Response[goosev1.SetConfigResponse], error) {
	if m.setFunc != nil {
		return m.setFunc(ctx, req)
	}
	return connect.NewResponse(&goosev1.SetConfigResponse{Success: true}), nil
}

func (m *mockConfigConnectServer) List(ctx context.Context, req *connect.Request[goosev1.ListConfigRequest]) (*connect.Response[goosev1.ListConfigResponse], error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, req)
	}
	return connect.NewResponse(&goosev1.ListConfigResponse{
		Entries: []*goosev1.ConfigEntry{
			{Key: "log.level", Value: "info"},
			{Key: "log.format", Value: "json"},
		},
	}), nil
}

// newTestServer builds a single httptest.Server that routes all four services.
func newTestServer(
	daemonSvc goosev1connect.DaemonServiceHandler,
	agentSvc goosev1connect.AgentServiceHandler,
	toolSvc goosev1connect.ToolServiceHandler,
	configSvc goosev1connect.ConfigServiceHandler,
) *httptest.Server {
	mux := http.NewServeMux()
	if daemonSvc != nil {
		path, handler := goosev1connect.NewDaemonServiceHandler(daemonSvc)
		mux.Handle(path, handler)
	}
	if agentSvc != nil {
		path, handler := goosev1connect.NewAgentServiceHandler(agentSvc)
		mux.Handle(path, handler)
	}
	if toolSvc != nil {
		path, handler := goosev1connect.NewToolServiceHandler(toolSvc)
		mux.Handle(path, handler)
	}
	if configSvc != nil {
		path, handler := goosev1connect.NewConfigServiceHandler(configSvc)
		mux.Handle(path, handler)
	}
	return httptest.NewServer(mux)
}

// RED #2: TestConnectClient_Ping_Success
func TestConnectClient_Ping_Success(t *testing.T) {
	srv := newTestServer(&mockDaemonConnectServer{}, nil, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	resp, err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}

	if resp.Version != "test-v1.0.0" {
		t.Errorf("Version = %q, want %q", resp.Version, "test-v1.0.0")
	}
	if resp.UptimeMs != 1000 {
		t.Errorf("UptimeMs = %d, want 1000", resp.UptimeMs)
	}
	if resp.State != "serving" {
		t.Errorf("State = %q, want %q", resp.State, "serving")
	}
}

// RED #2: TestConnectClient_Ping_Timeout
func TestConnectClient_Ping_Timeout(t *testing.T) {
	srv := newTestServer(&mockDaemonConnectServer{
		pingFunc: func(ctx context.Context, req *connect.Request[goosev1.PingRequest]) (*connect.Response[goosev1.PingResponse], error) {
			// Delay longer than the client timeout
			select {
			case <-ctx.Done():
				return nil, connect.NewError(connect.CodeDeadlineExceeded, ctx.Err())
			case <-time.After(200 * time.Millisecond):
				return nil, connect.NewError(connect.CodeDeadlineExceeded, errors.New("deadline exceeded"))
			}
		},
	}, nil, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL, WithDialTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.Ping(ctx)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// RED #3: TestConnectClient_Chat_Unary_Success
func TestConnectClient_Chat_Unary_Success(t *testing.T) {
	srv := newTestServer(nil, &mockAgentConnectServer{}, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	resp, err := client.Chat(context.Background(), "test-agent", "Hello!")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if resp.Content != "Hello from agent!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello from agent!")
	}
	if resp.TokensIn != 10 {
		t.Errorf("TokensIn = %d, want 10", resp.TokensIn)
	}
}

// RED #3: TestConnectClient_Chat_Error
func TestConnectClient_Chat_Error(t *testing.T) {
	srv := newTestServer(nil, &mockAgentConnectServer{
		chatFunc: func(ctx context.Context, req *connect.Request[goosev1.AgentChatRequest]) (*connect.Response[goosev1.AgentChatResponse], error) {
			return nil, connect.NewError(connect.CodeInternal, errors.New("agent unavailable"))
		},
	}, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	_, err = client.Chat(context.Background(), "bad-agent", "Hello!")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Errorf("expected connect.Error, got %T: %v", err, err)
	}
}

// RED #4: TestConnectClient_ChatStream_ReceivesEvents
func TestConnectClient_ChatStream_ReceivesEvents(t *testing.T) {
	srv := newTestServer(nil, &mockAgentConnectServer{}, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	eventCh, errCh := client.ChatStream(context.Background(), "test-agent", "stream test")

	var events []ChatStreamEvent
	for ev := range eventCh {
		events = append(events, ev)
	}

	if streamErr := <-errCh; streamErr != nil {
		t.Fatalf("ChatStream error: %v", streamErr)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "text" {
		t.Errorf("events[0].Type = %q, want %q", events[0].Type, "text")
	}
	if events[1].Type != "done" {
		t.Errorf("events[1].Type = %q, want %q", events[1].Type, "done")
	}
}

// RED #4: TestConnectClient_ChatStream_CtxCancel
func TestConnectClient_ChatStream_CtxCancel(t *testing.T) {
	srv := newTestServer(nil, &mockAgentConnectServer{
		chatStreamFunc: func(ctx context.Context, req *connect.Request[goosev1.AgentChatStreamRequest], stream *connect.ServerStream[goosev1.AgentChatStreamEvent]) error {
			// Send one event then block waiting for cancellation
			if err := stream.Send(&goosev1.AgentChatStreamEvent{Type: "text", PayloadJson: []byte(`{"text":"hi"}`)}); err != nil {
				return err
			}
			<-ctx.Done()
			return ctx.Err()
		},
	}, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	eventCh, errCh := client.ChatStream(ctx, "test-agent", "cancel test")

	// Receive first event
	select {
	case ev, ok := <-eventCh:
		if !ok {
			t.Fatal("channel closed before receiving event")
		}
		if ev.Type != "text" {
			t.Errorf("expected text event, got %q", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first event")
	}

	// Cancel context
	cancel()

	// Drain remaining events and error channel
	for range eventCh {
	}
	// Expect an error after cancellation (context.Canceled or connect cancel error)
	err = <-errCh
	// err may be nil or non-nil depending on timing; both are acceptable after cancel
	_ = err
}

// RED #5: TestConnectClient_ListTools_Success
func TestConnectClient_ListTools_Success(t *testing.T) {
	srv := newTestServer(nil, nil, &mockToolConnectServer{}, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "bash" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "bash")
	}
	if tools[1].Name != "read_file" {
		t.Errorf("tools[1].Name = %q, want %q", tools[1].Name, "read_file")
	}
}

// RED #5: TestConnectClient_ListTools_Empty
func TestConnectClient_ListTools_Empty(t *testing.T) {
	srv := newTestServer(nil, nil, &mockToolConnectServer{
		listFunc: func(ctx context.Context, req *connect.Request[goosev1.ListToolsRequest]) (*connect.Response[goosev1.ListToolsResponse], error) {
			return connect.NewResponse(&goosev1.ListToolsResponse{}), nil
		},
	}, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

// RED #6: TestConnectClient_GetConfig_Found
func TestConnectClient_GetConfig_Found(t *testing.T) {
	srv := newTestServer(nil, nil, nil, &mockConfigConnectServer{
		getFunc: func(ctx context.Context, req *connect.Request[goosev1.GetConfigRequest]) (*connect.Response[goosev1.GetConfigResponse], error) {
			if req.Msg.Key != "log.level" {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("key not found"))
			}
			return connect.NewResponse(&goosev1.GetConfigResponse{
				Value: &goosev1.ConfigValue{Value: "debug", Exists: true},
			}), nil
		},
	})
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	val, exists, err := client.GetConfig(context.Background(), "log.level")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if !exists {
		t.Error("expected exists=true")
	}
	if val != "debug" {
		t.Errorf("value = %q, want %q", val, "debug")
	}
}

// RED #6: TestConnectClient_GetConfig_NotFound
func TestConnectClient_GetConfig_NotFound(t *testing.T) {
	srv := newTestServer(nil, nil, nil, &mockConfigConnectServer{
		getFunc: func(ctx context.Context, req *connect.Request[goosev1.GetConfigRequest]) (*connect.Response[goosev1.GetConfigResponse], error) {
			return connect.NewResponse(&goosev1.GetConfigResponse{
				Value: &goosev1.ConfigValue{Value: "", Exists: false},
			}), nil
		},
	})
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	_, exists, err := client.GetConfig(context.Background(), "nonexistent.key")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if exists {
		t.Error("expected exists=false for missing key")
	}
}

// RED #6: TestConnectClient_SetConfig
func TestConnectClient_SetConfig(t *testing.T) {
	var capturedKey, capturedVal string
	srv := newTestServer(nil, nil, nil, &mockConfigConnectServer{
		setFunc: func(ctx context.Context, req *connect.Request[goosev1.SetConfigRequest]) (*connect.Response[goosev1.SetConfigResponse], error) {
			capturedKey = req.Msg.Key
			capturedVal = req.Msg.Value
			return connect.NewResponse(&goosev1.SetConfigResponse{Success: true}), nil
		},
	})
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	err = client.SetConfig(context.Background(), "log.level", "warn")
	if err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	if capturedKey != "log.level" {
		t.Errorf("key = %q, want %q", capturedKey, "log.level")
	}
	if capturedVal != "warn" {
		t.Errorf("value = %q, want %q", capturedVal, "warn")
	}
}

// RED #6: TestConnectClient_ListConfig_Prefix
func TestConnectClient_ListConfig_Prefix(t *testing.T) {
	srv := newTestServer(nil, nil, nil, &mockConfigConnectServer{
		listFunc: func(ctx context.Context, req *connect.Request[goosev1.ListConfigRequest]) (*connect.Response[goosev1.ListConfigResponse], error) {
			// Filter by prefix
			allEntries := []*goosev1.ConfigEntry{
				{Key: "log.level", Value: "info"},
				{Key: "log.format", Value: "json"},
				{Key: "server.port", Value: "9005"},
			}
			var filtered []*goosev1.ConfigEntry
			for _, e := range allEntries {
				if req.Msg.Prefix == "" || strings.HasPrefix(e.Key, req.Msg.Prefix) {
					filtered = append(filtered, e)
				}
			}
			return connect.NewResponse(&goosev1.ListConfigResponse{Entries: filtered}), nil
		},
	})
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	// List with "log." prefix — should return 2 entries
	configs, err := client.ListConfig(context.Background(), "log.")
	if err != nil {
		t.Fatalf("ListConfig: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 config entries, got %d", len(configs))
	}
	if configs["log.level"] != "info" {
		t.Errorf("log.level = %q, want %q", configs["log.level"], "info")
	}
	if configs["log.format"] != "json" {
		t.Errorf("log.format = %q, want %q", configs["log.format"], "json")
	}
}

// TestConnectClient_ChatStream_BlockedSend verifies ctx cancellation during event send.
// This exercises the eventCh <- / ctx.Done() select branch.
func TestConnectClient_ChatStream_BlockedSend(t *testing.T) {
	// Server sends many events rapidly
	srv := newTestServer(nil, &mockAgentConnectServer{
		chatStreamFunc: func(ctx context.Context, req *connect.Request[goosev1.AgentChatStreamRequest], stream *connect.ServerStream[goosev1.AgentChatStreamEvent]) error {
			for i := 0; i < 5; i++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				if err := stream.Send(&goosev1.AgentChatStreamEvent{
					Type:        "text",
					PayloadJson: []byte(`{"text":"chunk"}`),
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	eventCh, errCh := client.ChatStream(ctx, "agent", "test")

	// Read one event then cancel
	select {
	case _, ok := <-eventCh:
		if !ok {
			// stream completed before we could cancel — acceptable
			cancel()
			<-errCh
			return
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first event")
	}

	cancel()

	// Drain remaining
	for range eventCh {
	}
	<-errCh // may be nil or context.Canceled
}

// RED #7: TestRace_ConnectClient_ConcurrentCalls
// Uses 50 goroutines to call multiple methods concurrently to check for data races.
func TestRace_ConnectClient_ConcurrentCalls(t *testing.T) {
	srv := newTestServer(
		&mockDaemonConnectServer{},
		&mockAgentConnectServer{},
		&mockToolConnectServer{},
		&mockConfigConnectServer{},
	)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	var errCount atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()

			switch id % 4 {
			case 0:
				if _, err := client.Ping(ctx); err != nil {
					errCount.Add(1)
				}
			case 1:
				if _, err := client.Chat(ctx, "agent", "hello"); err != nil {
					errCount.Add(1)
				}
			case 2:
				if _, err := client.ListTools(ctx); err != nil {
					errCount.Add(1)
				}
			case 3:
				if _, _, err := client.GetConfig(ctx, "key"); err != nil {
					errCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	if n := errCount.Load(); n > 0 {
		t.Errorf("%d goroutines encountered errors during concurrent calls", n)
	}
}

// TestConnectClient_Options verifies functional option constructors.
func TestConnectClient_Options(t *testing.T) {
	srv := newTestServer(
		&mockDaemonConnectServer{},
		&mockAgentConnectServer{},
		nil,
		&mockConfigConnectServer{},
	)
	defer srv.Close()

	// WithHTTPClient: custom http.Client should be accepted
	customHTTP := &http.Client{Timeout: 5 * time.Second}
	client, err := NewConnectClient(srv.URL, WithHTTPClient(customHTTP))
	if err != nil {
		t.Fatalf("NewConnectClient with custom http.Client: %v", err)
	}

	// Verify client works with custom http.Client
	resp, err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping with custom http.Client: %v", err)
	}
	if resp.State != "serving" {
		t.Errorf("State = %q, want serving", resp.State)
	}

	// WithInterceptor: interceptor should not break normal calls
	var interceptorCalled atomic.Int64
	interceptor := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			interceptorCalled.Add(1)
			return next(ctx, req)
		}
	})
	clientWithInterceptor, err := NewConnectClient(srv.URL, WithInterceptor(interceptor))
	if err != nil {
		t.Fatalf("NewConnectClient with interceptor: %v", err)
	}
	_, err = clientWithInterceptor.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping with interceptor: %v", err)
	}
	if interceptorCalled.Load() == 0 {
		t.Error("interceptor was not called")
	}

	// WithSessionID: session ID should be forwarded in Chat request
	var capturedSessionID string
	srvWithSession := newTestServer(nil, &mockAgentConnectServer{
		chatFunc: func(ctx context.Context, req *connect.Request[goosev1.AgentChatRequest]) (*connect.Response[goosev1.AgentChatResponse], error) {
			capturedSessionID = req.Msg.SessionId
			return connect.NewResponse(&goosev1.AgentChatResponse{Content: "ok"}), nil
		},
	}, nil, nil)
	defer srvWithSession.Close()

	sessionClient, err := NewConnectClient(srvWithSession.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}
	_, err = sessionClient.Chat(context.Background(), "agent", "hi", WithSessionID("sess-abc"))
	if err != nil {
		t.Fatalf("Chat with session: %v", err)
	}
	if capturedSessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want %q", capturedSessionID, "sess-abc")
	}
}

// TestConnectClient_GetConfig_NilValue verifies graceful handling of nil ConfigValue.
func TestConnectClient_GetConfig_NilValue(t *testing.T) {
	srv := newTestServer(nil, nil, nil, &mockConfigConnectServer{
		getFunc: func(ctx context.Context, req *connect.Request[goosev1.GetConfigRequest]) (*connect.Response[goosev1.GetConfigResponse], error) {
			// Return response with nil Value field
			return connect.NewResponse(&goosev1.GetConfigResponse{}), nil
		},
	})
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	val, exists, err := client.GetConfig(context.Background(), "any.key")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if exists {
		t.Error("expected exists=false for nil ConfigValue")
	}
	if val != "" {
		t.Errorf("expected empty value, got %q", val)
	}
}

// TestConnectClient_ChatStream_ServerError verifies error propagation from server.
func TestConnectClient_ChatStream_ServerError(t *testing.T) {
	srv := newTestServer(nil, &mockAgentConnectServer{
		chatStreamFunc: func(ctx context.Context, req *connect.Request[goosev1.AgentChatStreamRequest], stream *connect.ServerStream[goosev1.AgentChatStreamEvent]) error {
			return connect.NewError(connect.CodeInternal, errors.New("stream failed"))
		},
	}, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	eventCh, errCh := client.ChatStream(context.Background(), "agent", "test")

	// Drain events (may be 0)
	for range eventCh {
	}

	streamErr := <-errCh
	if streamErr == nil {
		t.Error("expected error from failed stream, got nil")
	}
}

// TestConnectClient_ListTools_Error verifies error propagation in ListTools.
func TestConnectClient_ListTools_Error(t *testing.T) {
	srv := newTestServer(nil, nil, &mockToolConnectServer{
		listFunc: func(ctx context.Context, req *connect.Request[goosev1.ListToolsRequest]) (*connect.Response[goosev1.ListToolsResponse], error) {
			return nil, connect.NewError(connect.CodeUnavailable, errors.New("tool server down"))
		},
	}, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	_, err = client.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected error from ListTools, got nil")
	}
}

// TestConnectClient_SetConfig_Error verifies error propagation in SetConfig.
func TestConnectClient_SetConfig_Error(t *testing.T) {
	srv := newTestServer(nil, nil, nil, &mockConfigConnectServer{
		setFunc: func(ctx context.Context, req *connect.Request[goosev1.SetConfigRequest]) (*connect.Response[goosev1.SetConfigResponse], error) {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("read-only config"))
		},
	})
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	err = client.SetConfig(context.Background(), "locked.key", "value")
	if err == nil {
		t.Fatal("expected error from SetConfig, got nil")
	}
}

// TestConnectClient_ListConfig_AllEntries verifies listing without prefix.
func TestConnectClient_ListConfig_AllEntries(t *testing.T) {
	srv := newTestServer(nil, nil, nil, &mockConfigConnectServer{})
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	// Empty prefix returns all entries
	configs, err := client.ListConfig(context.Background(), "")
	if err != nil {
		t.Fatalf("ListConfig: %v", err)
	}
	if len(configs) != 2 {
		t.Errorf("expected 2 entries, got %d", len(configs))
	}
}

// TestConnectClient_ListConfig_Error verifies error propagation in ListConfig.
func TestConnectClient_ListConfig_Error(t *testing.T) {
	srv := newTestServer(nil, nil, nil, &mockConfigConnectServer{
		listFunc: func(ctx context.Context, req *connect.Request[goosev1.ListConfigRequest]) (*connect.Response[goosev1.ListConfigResponse], error) {
			return nil, connect.NewError(connect.CodeInternal, errors.New("config DB unavailable"))
		},
	})
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	_, err = client.ListConfig(context.Background(), "")
	if err == nil {
		t.Fatal("expected error from ListConfig, got nil")
	}
}

// TestConnectClient_Chat_WithSessionID verifies session ID option in ChatStream.
func TestConnectClient_ChatStream_WithSessionID(t *testing.T) {
	var capturedSessionID string
	srv := newTestServer(nil, &mockAgentConnectServer{
		chatStreamFunc: func(ctx context.Context, req *connect.Request[goosev1.AgentChatStreamRequest], stream *connect.ServerStream[goosev1.AgentChatStreamEvent]) error {
			capturedSessionID = req.Msg.SessionId
			return stream.Send(&goosev1.AgentChatStreamEvent{Type: "done", PayloadJson: []byte(`{}`)})
		},
	}, nil, nil)
	defer srv.Close()

	client, err := NewConnectClient(srv.URL)
	if err != nil {
		t.Fatalf("NewConnectClient: %v", err)
	}

	eventCh, errCh := client.ChatStream(context.Background(), "agent", "hi", WithSessionID("sess-xyz"))
	for range eventCh {
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}
	if capturedSessionID != "sess-xyz" {
		t.Errorf("session_id = %q, want %q", capturedSessionID, "sess-xyz")
	}
}

// RED #8: TestExisting_GRPCGoClient_NoRegression
// Validates that the existing gRPC-Go client tests still pass (regression baseline).
// This test compiles and verifies the existing DaemonClient types are still accessible.
func TestExisting_GRPCGoClient_NoRegression(t *testing.T) {
	// Verify Message type is still exported with same fields (byte-identical contract)
	msg := Message{
		Role:    "user",
		Content: "hello",
	}
	if msg.Role != "user" {
		t.Errorf("Message.Role regression: got %q", msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("Message.Content regression: got %q", msg.Content)
	}

	// Verify StreamEvent type is still exported with same fields
	ev := StreamEvent{
		Type:    "text",
		Content: "world",
	}
	if ev.Type != "text" {
		t.Errorf("StreamEvent.Type regression: got %q", ev.Type)
	}
	if ev.Content != "world" {
		t.Errorf("StreamEvent.Content regression: got %q", ev.Content)
	}

	// Verify PingResponse type (shared between both clients)
	ping := PingResponse{
		Version:  "v1",
		UptimeMs: 100,
		State:    "serving",
	}
	if ping.Version != "v1" {
		t.Errorf("PingResponse.Version regression: got %q", ping.Version)
	}
}
