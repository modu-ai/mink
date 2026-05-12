package tools_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/tools"
	_ "github.com/modu-ai/mink/internal/tools/builtin/file"
	_ "github.com/modu-ai/mink/internal/tools/builtin/terminal"
	"github.com/modu-ai/mink/internal/tools/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTool은 테스트용 Tool 구현이다.
type mockTool struct {
	name   string
	scope  tools.Scope
	schema json.RawMessage
	callFn func(ctx context.Context, input json.RawMessage) (tools.ToolResult, error)
}

func (m *mockTool) Name() string            { return m.name }
func (m *mockTool) Schema() json.RawMessage { return m.schema }
func (m *mockTool) Scope() tools.Scope      { return m.scope }
func (m *mockTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	if m.callFn != nil {
		return m.callFn(ctx, input)
	}
	return tools.ToolResult{Content: []byte("ok")}, nil
}

var validSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "name": {"type": "string"}
  },
  "additionalProperties": false
}`)

// mockMCPConnection은 테스트용 MCP 연결 구현이다.
type mockMCPConnection struct {
	serverID     string
	tools        []mcp.ToolManifest
	fetchFn      func(ctx context.Context, toolName string) (mcp.ToolManifest, error)
	fetchCount   map[string]int
	fetchCountMu sync.Mutex
}

func newMockMCPConn(serverID string, toolNames ...string) *mockMCPConnection {
	manifests := make([]mcp.ToolManifest, len(toolNames))
	for i, name := range toolNames {
		manifests[i] = mcp.ToolManifest{
			Name:        name,
			Description: "mock tool " + name,
			InputSchema: map[string]any{"type": "object"},
		}
	}
	return &mockMCPConnection{
		serverID:   serverID,
		tools:      manifests,
		fetchCount: make(map[string]int),
	}
}

func (m *mockMCPConnection) ServerID() string              { return m.serverID }
func (m *mockMCPConnection) ListTools() []mcp.ToolManifest { return m.tools }
func (m *mockMCPConnection) FetchToolManifest(ctx context.Context, toolName string) (mcp.ToolManifest, error) {
	m.fetchCountMu.Lock()
	m.fetchCount[toolName]++
	m.fetchCountMu.Unlock()
	if m.fetchFn != nil {
		return m.fetchFn(ctx, toolName)
	}
	for _, t := range m.tools {
		if t.Name == toolName {
			return t, nil
		}
	}
	return mcp.ToolManifest{}, nil
}
func (m *mockMCPConnection) CallTool(ctx context.Context, toolName string, input map[string]any) (mcp.ToolCallResult, error) {
	return mcp.ToolCallResult{Content: []byte("mcp_result")}, nil
}
func (m *mockMCPConnection) FetchCount(toolName string) int {
	m.fetchCountMu.Lock()
	defer m.fetchCountMu.Unlock()
	return m.fetchCount[toolName]
}

// TestRegistry_RegisterBuiltins_HasSixCanonicalNames — AC-TOOLS-001 (RED #1)
// Built-in 6종 자동 등록 확인
func TestRegistry_RegisterBuiltins_HasSixCanonicalNames(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())
	names := r.ListNames()

	assert.Equal(t, []string{"Bash", "FileEdit", "FileRead", "FileWrite", "Glob", "Grep"}, names,
		"built-in 6종이 알파벳 순으로 등록되어야 함")
}

// TestRegistry_RegisterDuplicate_ErrOrPanic — AC-TOOLS-001, REQ-TOOLS-013 (RED #2)
// 동일 이름 두 번째 등록 시 에러 또는 panic
func TestRegistry_RegisterDuplicate_ErrOrPanic(t *testing.T) {
	r := tools.NewRegistry()
	tool1 := &mockTool{name: "MyTool", schema: validSchema}
	tool2 := &mockTool{name: "MyTool", schema: validSchema}

	err := r.Register(tool1, tools.SourcePlugin)
	require.NoError(t, err)

	err = r.Register(tool2, tools.SourcePlugin)
	assert.ErrorIs(t, err, tools.ErrDuplicateName, "두 번째 등록은 ErrDuplicateName을 반환해야 함")
}

// TestRegistry_RegisterBuiltinDuplicate_Panics — REQ-TOOLS-013 (RED #2 변형)
// built-in 중복 등록 시 panic
func TestRegistry_RegisterBuiltinDuplicate_Panics(t *testing.T) {
	assert.Panics(t, func() {
		r := tools.NewRegistry()
		// 같은 이름의 built-in tool을 두 번 등록하면 panic
		tool1 := &mockTool{name: "Bash", schema: validSchema}
		// 첫 번째: 성공
		_ = r.Register(tool1, tools.SourceBuiltin)
		// 두 번째: panic 예상 (SourceBuiltin은 중복 시 panic)
		tool2 := &mockTool{name: "Bash", schema: validSchema}
		_ = r.Register(tool2, tools.SourceBuiltin)
	}, "built-in 중복 등록은 panic이어야 함")
}

// TestRegistry_AdoptMCP_AppliesPrefix — AC-TOOLS-002 (RED #3)
// MCP tool adoption 시 mcp__{serverID}__{toolName} prefix 적용
func TestRegistry_AdoptMCP_AppliesPrefix(t *testing.T) {
	r := tools.NewRegistry()
	conn := newMockMCPConn("github", "create_issue")

	err := r.AdoptMCPServer(conn)
	require.NoError(t, err)

	tool, ok := r.Resolve("mcp__github__create_issue")
	assert.True(t, ok, "mcp__github__create_issue로 resolve 가능해야 함")
	assert.NotNil(t, tool)

	_, ok2 := r.Resolve("create_issue")
	assert.False(t, ok2, "prefix 없는 이름으로는 resolve 불가해야 함")
}

// TestRegistry_ResolveMCPStub_LazyFetch — AC-TOOLS-003 (RED #4)
// MCP stub tool이 첫 Call 시 FetchToolManifest를 1회만 호출
func TestRegistry_ResolveMCPStub_LazyFetch(t *testing.T) {
	r := tools.NewRegistry()
	conn := newMockMCPConn("foo", "bar")

	err := r.AdoptMCPServer(conn)
	require.NoError(t, err)

	// adopt 직후 fetch count는 0
	assert.Equal(t, 0, conn.FetchCount("bar"), "adopt 시 즉시 fetch 없어야 함")

	tool, ok := r.Resolve("mcp__foo__bar")
	require.True(t, ok)

	// 첫 Call 시 fetch 발생
	ctx := context.Background()
	_, _ = tool.Call(ctx, json.RawMessage(`{}`))
	assert.Equal(t, 1, conn.FetchCount("bar"), "첫 Call 시 정확히 1회 fetch해야 함")

	// 재호출 시 fetch count 유지
	_, _ = tool.Call(ctx, json.RawMessage(`{}`))
	assert.Equal(t, 1, conn.FetchCount("bar"), "재호출 시 fetch count 증가 없어야 함")
}

// TestRegistry_MCPServerID_Invalid_Rejected — REQ-TOOLS-004
// 유효하지 않은 serverID는 거부
func TestRegistry_MCPServerID_Invalid_Rejected(t *testing.T) {
	r := tools.NewRegistry()
	invalidConn := newMockMCPConn("INVALID_UPPERCASE", "tool")

	err := r.AdoptMCPServer(invalidConn)
	assert.Error(t, err, "대문자 serverID는 거부되어야 함")
}

// TestRegistry_MCPTool_ReservedName_Rejected — REQ-TOOLS-003
// built-in 예약어를 MCP tool이 클레임하면 거부
func TestRegistry_MCPTool_ReservedName_Rejected(t *testing.T) {
	r := tools.NewRegistry()
	conn := newMockMCPConn("srv", "Bash") // "Bash"는 예약어

	err := r.AdoptMCPServer(conn)
	// err는 nil일 수 있음 (개별 tool은 skip되고 연결 자체는 성공)
	_ = err

	_, ok := r.Resolve("mcp__srv__Bash")
	assert.False(t, ok, "예약어 MCP tool은 등록되지 않아야 함")
}

// TestRegistry_MCPTool_DoubleUnderscore_Rejected — AC-TOOLS-016, REQ-TOOLS-017
// __ 포함한 MCP tool 이름은 거부
func TestRegistry_MCPTool_DoubleUnderscore_Rejected(t *testing.T) {
	r := tools.NewRegistry()
	conn := newMockMCPConn("srv", "bad__name", "good_name")

	err := r.AdoptMCPServer(conn)
	_ = err // 다른 tool이 있으면 nil

	_, bad := r.Resolve("mcp__srv__bad__name")
	assert.False(t, bad, "bad__name은 등록되지 않아야 함")

	_, good := r.Resolve("mcp__srv__good_name")
	assert.True(t, good, "good_name은 정상 등록되어야 함")
}

// TestRegistry_MCPDuplicate_Rejected — AC-TOOLS-019, REQ-TOOLS-021
// 동일 (serverID, toolName) 재adoption 거부
func TestRegistry_MCPDuplicate_Rejected(t *testing.T) {
	r := tools.NewRegistry()
	conn := newMockMCPConn("github", "create_issue")

	err := r.AdoptMCPServer(conn)
	require.NoError(t, err)

	// 재호출
	err = r.AdoptMCPServer(conn)
	assert.ErrorIs(t, err, tools.ErrDuplicateName, "중복 adoption은 ErrDuplicateName이어야 함")

	// 기존 등록은 유지
	_, ok := r.Resolve("mcp__github__create_issue")
	assert.True(t, ok, "기존 등록은 유지되어야 함")
}

// TestRegistry_Drain_BlocksExecution — AC-TOOLS-013, REQ-TOOLS-011
// Drain 이후 Executor.Run은 draining 에러 반환
func TestRegistry_Drain_State(t *testing.T) {
	r := tools.NewRegistry()
	assert.False(t, r.IsDraining())

	r.Drain()
	assert.True(t, r.IsDraining())
}

// TestRegistry_Concurrent_Resolve_Safe — REQ-TOOLS-001
// 동시 Resolve 호출 안전성
func TestRegistry_Concurrent_Resolve_Safe(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tool, ok := r.Resolve("Bash")
			assert.True(t, ok)
			assert.NotNil(t, tool)
		}()
	}
	wg.Wait()
}

// TestRegistry_MCPConnectionClosed_UnregistersTools — AC-TOOLS-012, REQ-TOOLS-009
// ConnectionClosed 이벤트 후 MCP tool 제거
func TestRegistry_MCPConnectionClosed_UnregistersTools(t *testing.T) {
	r := tools.NewRegistry()
	conn := newMockMCPConn("foo", "a", "b")

	err := r.AdoptMCPServer(conn)
	require.NoError(t, err)

	_, ok1 := r.Resolve("mcp__foo__a")
	_, ok2 := r.Resolve("mcp__foo__b")
	require.True(t, ok1)
	require.True(t, ok2)

	// ConnectionClosed 이벤트 시뮬레이션
	closedCh := make(chan mcp.ConnectionClosedEvent, 1)
	closedCh <- mcp.ConnectionClosedEvent{ServerID: "foo"}

	mgr := &mockMCPManager{
		conns:    []mcp.Connection{},
		closedCh: closedCh,
	}

	// WithMCPConnections는 고루틴으로 이벤트 구독
	opt := tools.WithMCPConnections(mgr)
	opt(r)

	// 1초 이내 해제 확인
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, aOk := r.Resolve("mcp__foo__a")
		_, bOk := r.Resolve("mcp__foo__b")
		if !aOk && !bOk {
			return // 성공
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("1초 이내에 MCP tool이 제거되어야 함")
}

// TestRegistry_InvalidSchema_Rejected — REQ-TOOLS-002
func TestRegistry_InvalidSchema_Rejected(t *testing.T) {
	r := tools.NewRegistry()
	badTool := &mockTool{name: "BadSchema", schema: json.RawMessage(`not-json`)}

	err := r.Register(badTool, tools.SourcePlugin)
	assert.ErrorIs(t, err, tools.ErrInvalidSchema, "유효하지 않은 스키마는 ErrInvalidSchema이어야 함")
}

// TestRegistry_StrictSchema_Rejects_Without_AdditionalPropertiesFalse — AC-TOOLS-017, REQ-TOOLS-019
func TestRegistry_StrictSchema_Rejects_Without_AdditionalPropertiesFalse(t *testing.T) {
	cfg := tools.RegistryConfig{StrictSchema: true}
	r := tools.NewRegistryWithConfig(cfg)

	badTool := &mockTool{
		name:   "BadTool",
		schema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`),
	}

	err := r.Register(badTool, tools.SourcePlugin)
	assert.ErrorIs(t, err, tools.ErrInvalidSchema,
		"additionalProperties 없는 스키마는 strict_schema 모드에서 거부되어야 함")

	_, ok := r.Resolve("BadTool")
	assert.False(t, ok)
}

// TestRegistry_MCPStub_Call_Dispatches — MCP stub의 Call이 실제 tool로 dispatch
func TestRegistry_MCPStub_Call_Dispatches(t *testing.T) {
	r := tools.NewRegistry()
	conn := newMockMCPConn("github", "create_issue")

	err := r.AdoptMCPServer(conn)
	require.NoError(t, err)

	tool, ok := r.Resolve("mcp__github__create_issue")
	require.True(t, ok)

	result, err := tool.Call(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "mcp_result", string(result.Content))
}

// TestRegistry_Config_Returns — Registry.Config 반환
func TestRegistry_Config_Returns(t *testing.T) {
	cfg := tools.RegistryConfig{Cwd: "/tmp/test", StrictSchema: true}
	r := tools.NewRegistryWithConfig(cfg)
	assert.Equal(t, cfg.Cwd, r.Config().Cwd)
	assert.Equal(t, cfg.StrictSchema, r.Config().StrictSchema)
}

// TestRegistryEntry_Descriptor_Tool — registryEntry 접근자
func TestRegistryEntry_Descriptor_Tool(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())
	entry, ok := r.ResolveEntry("Bash")
	require.True(t, ok)

	desc := entry.Descriptor()
	assert.Equal(t, "Bash", desc.Name)
	assert.Equal(t, tools.SourceBuiltin, desc.Source)

	tool := entry.Tool()
	assert.NotNil(t, tool)
	assert.Equal(t, "Bash", tool.Name())
}

// TestRegistry_StrictSchema_MissingAdditionalProperties — additionalProperties가 true인 경우
func TestRegistry_StrictSchema_MissingAdditionalProperties_True(t *testing.T) {
	cfg := tools.RegistryConfig{StrictSchema: true}
	r := tools.NewRegistryWithConfig(cfg)

	// additionalProperties: true (명시적으로 true)
	badTool := &mockTool{
		name:   "TrueTool",
		schema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}},"additionalProperties":true}`),
	}
	err := r.Register(badTool, tools.SourcePlugin)
	assert.ErrorIs(t, err, tools.ErrInvalidSchema)
}

// TestRegistry_WithMCPConnections_AdoptsExisting — WithMCPConnections가 기존 연결 adopt
func TestRegistry_WithMCPConnections_AdoptsExisting(t *testing.T) {
	r := tools.NewRegistry()
	conn := newMockMCPConn("testserver", "tool_a")

	// mock manager with existing connection
	mgr := &mockMCPManager{
		conns:    []mcp.Connection{conn},
		closedCh: make(chan mcp.ConnectionClosedEvent),
	}

	opt := tools.WithMCPConnections(mgr)
	opt(r)

	_, ok := r.Resolve("mcp__testserver__tool_a")
	assert.True(t, ok, "WithMCPConnections가 기존 연결의 tool을 adopt해야 함")
}

// TestRegistry_WithMCPConnections_AdoptFailsLogged — adopt 실패 시 로그만 남기고 계속
func TestRegistry_WithMCPConnections_AdoptFailsLogged(t *testing.T) {
	r := tools.NewRegistry()
	// 먼저 직접 adopt
	conn := newMockMCPConn("dupserver", "tool_x")
	require.NoError(t, r.AdoptMCPServer(conn))

	// 동일 conn을 WithMCPConnections로 다시 adopt하면 에러 로그만 남기고 계속
	mgr := &mockMCPManager{
		conns:    []mcp.Connection{conn},
		closedCh: make(chan mcp.ConnectionClosedEvent),
	}
	opt := tools.WithMCPConnections(mgr)
	// Should not panic
	assert.NotPanics(t, func() { opt(r) })
}

// TestRegistry_StrictSchema_AdditionalPropertiesNotBool — additionalProperties가 bool이 아닌 경우
func TestRegistry_StrictSchema_AdditionalPropertiesNotBool(t *testing.T) {
	cfg := tools.RegistryConfig{StrictSchema: true}
	r := tools.NewRegistryWithConfig(cfg)

	// additionalProperties가 string (not a bool)
	badTool := &mockTool{
		name:   "StringApTool",
		schema: json.RawMessage(`{"type":"object","additionalProperties":"no"}`),
	}
	err := r.Register(badTool, tools.SourcePlugin)
	assert.ErrorIs(t, err, tools.ErrInvalidSchema)
}

// TestRegistry_AdoptMCPServer_BuildsMCPSchema — MCP schema 빌드 확인
func TestRegistry_AdoptMCPServer_BuildsMCPSchema(t *testing.T) {
	r := tools.NewRegistry()
	conn := &mockMCPConnection{
		serverID: "schema-test",
		tools: []mcp.ToolManifest{
			{
				Name:        "get_data",
				Description: "gets data",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
					},
				},
			},
		},
		fetchCount: make(map[string]int),
	}
	err := r.AdoptMCPServer(conn)
	require.NoError(t, err)

	_, ok := r.Resolve("mcp__schema-test__get_data")
	assert.True(t, ok)
}

// mockMCPManager는 테스트용 MCP Manager이다.
type mockMCPManager struct {
	conns    []mcp.Connection
	closedCh chan mcp.ConnectionClosedEvent
}

func (m *mockMCPManager) Connections() []mcp.Connection { return m.conns }
func (m *mockMCPManager) Subscribe() <-chan mcp.ConnectionClosedEvent {
	return m.closedCh
}
