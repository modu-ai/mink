package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modu-ai/mink/internal/tools"
	_ "github.com/modu-ai/mink/internal/tools/builtin/file"
	_ "github.com/modu-ai/mink/internal/tools/builtin/terminal"
	"github.com/modu-ai/mink/internal/tools/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInventory_ForModel_Sorted — AC-TOOLS-010, REQ-TOOLS-005
// 동일 tool 집합 다른 등록 순서 → 바이트 동일 출력
func TestInventory_ForModel_Sorted(t *testing.T) {
	// Registry 1: built-in + MCP
	r1 := tools.NewRegistry(tools.WithBuiltins())
	conn1 := newMockMCPConn("foo", "bar", "baz")
	require.NoError(t, r1.AdoptMCPServer(conn1))

	// Registry 2: MCP 먼저, then 내부 built-in은 같음
	r2 := tools.NewRegistry(tools.WithBuiltins())
	conn2 := newMockMCPConn("foo", "baz", "bar") // 역순
	require.NoError(t, r2.AdoptMCPServer(conn2))

	inv1 := tools.NewInventory(r1)
	inv2 := tools.NewInventory(r2)

	ctx := context.Background()
	filter := tools.InventoryFilter{}

	descs1 := inv1.ForModel(ctx, filter)
	descs2 := inv2.ForModel(ctx, filter)

	b1, err1 := json.Marshal(descs1)
	b2, err2 := json.Marshal(descs2)
	require.NoError(t, err1)
	require.NoError(t, err2)

	assert.Equal(t, string(b1), string(b2), "동일 tool 집합은 바이트 동일 출력이어야 함")

	// 알파벳 순서 확인
	names := make([]string, len(descs1))
	for i, d := range descs1 {
		names[i] = d.Name
	}
	expected := []string{"Bash", "FileEdit", "FileRead", "FileWrite", "Glob", "Grep", "mcp__foo__bar", "mcp__foo__baz"}
	assert.Equal(t, expected, names, "descriptor는 알파벳 오름차순이어야 함")
}

// TestInventory_ForModel_CoordinatorFilters — AC-TOOLS-014, REQ-TOOLS-012
// CoordinatorMode=true 시 ScopeLeaderOnly tool 제외
func TestInventory_ForModel_CoordinatorFilters(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())

	// ScopeLeaderOnly tool 추가
	leaderTool := &mockTool{
		name:   "TeamSpawn",
		scope:  tools.ScopeLeaderOnly,
		schema: validSchema,
	}
	require.NoError(t, r.Register(leaderTool, tools.SourcePlugin))

	inv := tools.NewInventory(r)
	ctx := context.Background()

	// CoordinatorMode=true
	descs := inv.ForModel(ctx, tools.InventoryFilter{CoordinatorMode: true})
	names := make([]string, len(descs))
	for i, d := range descs {
		names[i] = d.Name
	}

	assert.NotContains(t, names, "TeamSpawn", "CoordinatorMode에서 ScopeLeaderOnly는 제외되어야 함")
	assert.Contains(t, names, "Bash", "ScopeShared tool은 포함되어야 함")
}

// TestInventory_ForModel_NormalMode_IncludesLeaderOnly
// CoordinatorMode=false 시 ScopeLeaderOnly tool 포함
func TestInventory_ForModel_NormalMode_IncludesLeaderOnly(t *testing.T) {
	r := tools.NewRegistry()

	leaderTool := &mockTool{
		name:   "TeamSpawn",
		scope:  tools.ScopeLeaderOnly,
		schema: validSchema,
	}
	require.NoError(t, r.Register(leaderTool, tools.SourcePlugin))

	inv := tools.NewInventory(r)
	descs := inv.ForModel(context.Background(), tools.InventoryFilter{CoordinatorMode: false})

	names := make([]string, len(descs))
	for i, d := range descs {
		names[i] = d.Name
	}
	assert.Contains(t, names, "TeamSpawn")
}

// TestInventory_ForModelJSON — ForModelJSON 직렬화 검증
func TestInventory_ForModelJSON(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())
	inv := tools.NewInventory(r)

	b, err := inv.ForModelJSON(context.Background(), tools.InventoryFilter{})
	require.NoError(t, err)
	assert.NotEmpty(t, b)

	var descs []tools.ToolDescriptor
	require.NoError(t, json.Unmarshal(b, &descs))
	assert.Len(t, descs, 6)
}

// mockMCPConn용 보조 함수 (이미 registry_test.go에 정의됨, 재사용)
var _ mcp.Connection = (*mockMCPConnection)(nil)
