package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modu-ai/goose/internal/permissions"
	"github.com/modu-ai/goose/internal/tools"
	_ "github.com/modu-ai/goose/internal/tools/builtin/file"
	_ "github.com/modu-ai/goose/internal/tools/builtin/terminal"
	"github.com/modu-ai/goose/internal/tools/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// mockCanUseTool은 테스트용 CanUseTool 구현이다.
type mockCanUseTool struct {
	behavior  permissions.PermissionBehavior
	reason    string
	callCount int
}

func (m *mockCanUseTool) Check(ctx context.Context, tpc permissions.ToolPermissionContext) permissions.Decision {
	m.callCount++
	return permissions.Decision{Behavior: m.behavior, Reason: m.reason}
}

// newTestExecutor는 테스트용 Executor를 생성한다.
func newTestExecutor(r *tools.Registry, can permissions.CanUseTool, permCfg permission.Config) *tools.Executor {
	return tools.NewExecutor(tools.ExecutorConfig{
		Registry:   r,
		CanUseTool: can,
		PermConfig: permCfg,
		Logger:     zap.NewNop(),
	})
}

// TestExecutor_Run_SchemaValidationFails — AC-TOOLS-006 (RED #5)
// 잘못된 입력으로 schema validation 실패
func TestExecutor_Run_SchemaValidationFails(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())
	can := &mockCanUseTool{behavior: permissions.Allow}
	e := newTestExecutor(r, can, permission.Config{})

	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "FileRead",
		Input:    json.RawMessage(`{"wrong_field": 1}`),
	})

	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "schema_validation_failed",
		"스키마 검증 실패 메시지를 포함해야 함")
	assert.Equal(t, 0, can.callCount, "schema 실패 시 CanUseTool은 호출되지 않아야 함")
}

// TestExecutor_Run_ToolNotFound — AC-TOOLS-007 (RED #6)
// 존재하지 않는 tool 호출
func TestExecutor_Run_ToolNotFound(t *testing.T) {
	r := tools.NewRegistry()
	e := newTestExecutor(r, nil, permission.Config{})

	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "NonExistent",
		Input:    json.RawMessage(`{}`),
	})

	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "tool_not_found: NonExistent")
}

// TestExecutor_Run_PreapprovalBypassesCanUseTool — AC-TOOLS-004 (RED #7)
// pre-approval이 CanUseTool Deny를 bypass
func TestExecutor_Run_PreapprovalBypassesCanUseTool(t *testing.T) {
	r := tools.NewRegistry()

	// "FileRead" 대신 간단한 mock tool 등록
	callCount := 0
	readTool := &mockTool{
		name:   "FileRead",
		schema: validSchema,
		callFn: func(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
			callCount++
			return tools.ToolResult{Content: []byte("file_content")}, nil
		},
	}
	require.NoError(t, r.Register(readTool, tools.SourceBuiltin))

	// CanUseTool는 Deny 반환
	can := &mockCanUseTool{behavior: permissions.Deny, reason: "test_deny"}

	// permission allow에 "FileRead" 패턴 포함
	permCfg := permission.Config{Allow: []string{"FileRead"}}
	e := newTestExecutor(r, can, permCfg)

	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "FileRead",
		Input:    json.RawMessage(`{"name": "test"}`),
	})

	assert.False(t, result.IsError, "pre-approval로 실행되어야 함")
	assert.Equal(t, 0, can.callCount, "CanUseTool이 호출되지 않아야 함")
	assert.Equal(t, 1, callCount, "Tool.Call이 호출되어야 함")
}

// TestExecutor_Run_CanUseToolDeny — AC-TOOLS-005 (RED #8)
// CanUseTool Deny 시 Tool.Call 미호출
func TestExecutor_Run_CanUseToolDeny(t *testing.T) {
	r := tools.NewRegistry()

	callCount := 0
	bashTool := &mockTool{
		name:   "Bash",
		schema: validSchema,
		callFn: func(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
			callCount++
			return tools.ToolResult{Content: []byte("executed")}, nil
		},
	}
	require.NoError(t, r.Register(bashTool, tools.SourceBuiltin))

	can := &mockCanUseTool{behavior: permissions.Deny, reason: "destructive"}
	e := newTestExecutor(r, can, permission.Config{})

	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "Bash",
		Input:    json.RawMessage(`{"name": "rm"}`),
	})

	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "denied: destructive")
	assert.Equal(t, 0, callCount, "Tool.Call은 호출되지 않아야 함")
}

// TestExecutor_Run_Draining — AC-TOOLS-013, REQ-TOOLS-011
// Drain 후 Run은 draining 에러 반환
func TestExecutor_Run_Draining(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())
	r.Drain()

	e := newTestExecutor(r, nil, permission.Config{})
	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "FileRead",
		Input:    json.RawMessage(`{"path": "/tmp/x"}`),
	})

	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "registry draining")
}

// TestExecutor_Run_LogInvocations — AC-TOOLS-018, REQ-TOOLS-020
// log_invocations=true 시 INFO 로그 출력
func TestExecutor_Run_LogInvocations(t *testing.T) {
	r := tools.NewRegistry()

	readTool := &mockTool{
		name:   "FileRead",
		schema: validSchema,
		callFn: func(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
			return tools.ToolResult{Content: []byte("content")}, nil
		},
	}
	require.NoError(t, r.Register(readTool, tools.SourceBuiltin))

	// observer 로거 생성
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	e := tools.NewExecutor(tools.ExecutorConfig{
		Registry:       r,
		CanUseTool:     &mockCanUseTool{behavior: permissions.Allow},
		Logger:         logger,
		LogInvocations: true,
	})

	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "FileRead",
		Input:    json.RawMessage(`{"name": "test"}`),
	})

	_ = result

	require.Equal(t, 1, logs.Len(), "INFO 로그가 1건 출력되어야 함")
	entry := logs.All()[0]
	assert.Equal(t, "tool invocation", entry.Message)

	fields := entry.ContextMap()
	assert.Equal(t, "FileRead", fields["tool"])
	_, hasDuration := fields["duration_ms"]
	assert.True(t, hasDuration, "duration_ms 필드가 있어야 함")
}

// TestExecutor_Run_CanUseToolAsk — permissions.Ask 처리
func TestExecutor_Run_CanUseToolAsk(t *testing.T) {
	r := tools.NewRegistry()
	callCount := 0
	mockT := &mockTool{
		name:   "SomeTool",
		schema: validSchema,
		callFn: func(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
			callCount++
			return tools.ToolResult{Content: []byte("ok")}, nil
		},
	}
	require.NoError(t, r.Register(mockT, tools.SourcePlugin))

	can := &mockCanUseTool{behavior: permissions.Ask, reason: "needs_user_approval"}
	e := newTestExecutor(r, can, permission.Config{})

	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "SomeTool",
		Input:    json.RawMessage(`{"name": "x"}`),
	})

	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "permission_required")
	assert.Equal(t, 0, callCount, "Ask 시 tool.Call 미호출")
}

// TestExecutor_Run_ToolCallReturnsError — tool.Call이 error 반환
func TestExecutor_Run_ToolCallReturnsError(t *testing.T) {
	r := tools.NewRegistry()
	errTool := &mockTool{
		name:   "ErrTool",
		schema: validSchema,
		callFn: func(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
			return tools.ToolResult{}, assert.AnError
		},
	}
	require.NoError(t, r.Register(errTool, tools.SourcePlugin))

	e := newTestExecutor(r, &mockCanUseTool{behavior: permissions.Allow}, permission.Config{})

	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "ErrTool",
		Input:    json.RawMessage(`{"name": "x"}`),
	})

	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "tool_error")
}

// TestExecutor_Run_InvalidJSONInput — 잘못된 JSON은 schema validation에서 실패
func TestExecutor_Run_InvalidJSONInput(t *testing.T) {
	r := tools.NewRegistry()
	mt := &mockTool{name: "SomeTool", schema: validSchema}
	require.NoError(t, r.Register(mt, tools.SourcePlugin))

	e := newTestExecutor(r, nil, permission.Config{})
	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "SomeTool",
		Input:    json.RawMessage(`not-valid-json`),
	})

	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "schema_validation_failed")
}

// TestExecutor_NilMatcher_UsesDefault — nil matcher 시 GlobMatcher 사용
func TestExecutor_NilMatcher_UsesDefault(t *testing.T) {
	r := tools.NewRegistry()
	mt := &mockTool{name: "SomeTool", schema: validSchema}
	require.NoError(t, r.Register(mt, tools.SourcePlugin))

	// nil matcher → NewExecutor가 GlobMatcher로 대체
	e := tools.NewExecutor(tools.ExecutorConfig{
		Registry:   r,
		Matcher:    nil,
		CanUseTool: &mockCanUseTool{behavior: permissions.Allow},
	})

	result := e.Run(context.Background(), tools.ExecRequest{
		ToolName: "SomeTool",
		Input:    json.RawMessage(`{"name": "x"}`),
	})
	assert.False(t, result.IsError)
}

// TestExecutor_Sequential_MultipleToolUse — AC-TOOLS-020, REQ-TOOLS-022
// 여러 tool_use 블록의 순차 실행 보장
func TestExecutor_Sequential_MultipleToolUse(t *testing.T) {
	r := tools.NewRegistry()

	// 호출 순서 기록
	var callOrder []string
	var callMu mockCanUseTool // mu 대신 단순 카운터
	_ = callMu

	makeOrderedTool := func(name string) tools.Tool {
		return &mockTool{
			name:   name,
			schema: validSchema,
			callFn: func(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
				callOrder = append(callOrder, name)
				return tools.ToolResult{Content: []byte(name + "_result")}, nil
			},
		}
	}

	require.NoError(t, r.Register(makeOrderedTool("Glob"), tools.SourceBuiltin))
	require.NoError(t, r.Register(makeOrderedTool("FileRead"), tools.SourceBuiltin))
	require.NoError(t, r.Register(makeOrderedTool("Bash"), tools.SourceBuiltin))

	e := newTestExecutor(r, &mockCanUseTool{behavior: permissions.Allow}, permission.Config{})

	// 순차 실행
	toolUses := []string{"Glob", "FileRead", "Bash"}
	for _, name := range toolUses {
		result := e.Run(context.Background(), tools.ExecRequest{
			ToolName: name,
			Input:    json.RawMessage(`{"name":"test"}`),
		})
		assert.False(t, result.IsError)
	}

	assert.Equal(t, []string{"Glob", "FileRead", "Bash"}, callOrder,
		"tool 실행 순서가 요청 순서와 일치해야 함")
}
