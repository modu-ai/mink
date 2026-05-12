package builtin_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modu-ai/mink/internal/tools"
	"github.com/modu-ai/mink/internal/tools/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTool은 테스트용 최소 Tool 구현이다.
type stubTool struct{ name string }

func (s *stubTool) Name() string { return s.name }
func (s *stubTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}
func (s *stubTool) Scope() tools.Scope { return tools.ScopeShared }
func (s *stubTool) Call(_ context.Context, _ json.RawMessage) (tools.ToolResult, error) {
	return tools.ToolResult{Content: []byte("ok")}, nil
}

// TestBuiltin_Register — Register가 전역 builtin 목록에 추가됨을 확인
func TestBuiltin_Register(t *testing.T) {
	// Register를 호출 후 WithBuiltins로 등록 확인
	t.Parallel()

	// 고유 이름으로 등록
	name := "TestBuiltinRegisterTool"
	builtin.Register(&stubTool{name: name})

	r := tools.NewRegistry(tools.WithBuiltins())
	names := r.ListNames()

	found := false
	for _, n := range names {
		if n == name {
			found = true
			break
		}
	}
	require.True(t, found, "Register된 tool이 WithBuiltins()에 의해 등록되어야 함")

	tool, ok := r.Resolve(name)
	assert.True(t, ok)
	assert.NotNil(t, tool)
}
