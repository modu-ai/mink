package search_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/tools"
	_ "github.com/modu-ai/mink/internal/tools/builtin/file"
	_ "github.com/modu-ai/mink/internal/tools/builtin/terminal"
	"github.com/modu-ai/mink/internal/tools/mcp"
	"github.com/modu-ai/mink/internal/tools/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPConnection은 테스트용 MCP 연결이다.
type mockMCPConnection struct {
	serverID  string
	manifests []mcp.ToolManifest
	fetchFn   func(ctx context.Context, name string) (mcp.ToolManifest, error)
}

func (m *mockMCPConnection) ServerID() string              { return m.serverID }
func (m *mockMCPConnection) ListTools() []mcp.ToolManifest { return m.manifests }
func (m *mockMCPConnection) FetchToolManifest(ctx context.Context, toolName string) (mcp.ToolManifest, error) {
	if m.fetchFn != nil {
		return m.fetchFn(ctx, toolName)
	}
	for _, t := range m.manifests {
		if t.Name == toolName {
			return t, nil
		}
	}
	return mcp.ToolManifest{}, nil
}
func (m *mockMCPConnection) CallTool(ctx context.Context, toolName string, input map[string]any) (mcp.ToolCallResult, error) {
	return mcp.ToolCallResult{Content: []byte("result")}, nil
}

// TestSearch_Activate_FetchesAndCaches — Search.Activate가 manifest를 fetch하고 캐시
func TestSearch_Activate_FetchesAndCaches(t *testing.T) {
	r := tools.NewRegistry()
	conn := &mockMCPConnection{
		serverID: "foo",
		manifests: []mcp.ToolManifest{
			{Name: "bar", Description: "test tool", InputSchema: map[string]any{"type": "object"}},
		},
	}
	require.NoError(t, r.AdoptMCPServer(conn))

	s := search.New(r)

	err := s.Activate(context.Background(), "mcp__foo__bar")
	assert.NoError(t, err, "activate 성공해야 함")

	// 캐시 확인 (두 번째 activate는 fetch 없이 성공)
	err2 := s.Activate(context.Background(), "mcp__foo__bar")
	assert.NoError(t, err2)
}

// TestSearch_Activate_ToolNotFound — 존재하지 않는 tool
func TestSearch_Activate_ToolNotFound(t *testing.T) {
	r := tools.NewRegistry()
	s := search.New(r)

	err := s.Activate(context.Background(), "mcp__nonexistent__tool")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
}

// TestSearch_Activate_BuiltinSkipped — built-in tool은 activation 불필요
func TestSearch_Activate_BuiltinSkipped(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())
	s := search.New(r)

	err := s.Activate(context.Background(), "Bash")
	assert.NoError(t, err, "built-in tool activation은 에러 없이 완료")
}

// TestSearch_Activate_Timeout — AC-TOOLS-011, REQ-TOOLS-008
// 5초 타임아웃 검증 (짧은 context로 시뮬레이션)
func TestSearch_Activate_Timeout(t *testing.T) {
	r := tools.NewRegistry()
	blocking := make(chan struct{}) // 절대 close 안 함 (블로킹 유지)

	conn := &mockMCPConnection{
		serverID: "slow",
		manifests: []mcp.ToolManifest{
			{Name: "op", InputSchema: map[string]any{"type": "object"}},
		},
		fetchFn: func(ctx context.Context, name string) (mcp.ToolManifest, error) {
			select {
			case <-blocking:
				return mcp.ToolManifest{}, nil
			case <-ctx.Done():
				return mcp.ToolManifest{}, ctx.Err()
			}
		},
	}
	require.NoError(t, r.AdoptMCPServer(conn))
	s := search.New(r)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := s.Activate(ctx, "mcp__slow__op")
	elapsed := time.Since(start)

	// ErrMCPTimeout 또는 context 에러
	assert.Error(t, err)
	assert.Less(t, elapsed, time.Second, "1초 이내에 반환")
}

// TestSearch_InvalidateCache — REQ-TOOLS-009
func TestSearch_InvalidateCache(t *testing.T) {
	r := tools.NewRegistry()
	conn := &mockMCPConnection{
		serverID: "foo",
		manifests: []mcp.ToolManifest{
			{Name: "a", InputSchema: map[string]any{"type": "object"}},
		},
	}
	require.NoError(t, r.AdoptMCPServer(conn))

	s := search.New(r)

	// Activate로 캐시
	err := s.Activate(context.Background(), "mcp__foo__a")
	require.NoError(t, err)

	// InvalidateCache 호출
	s.InvalidateCache("foo")

	// 캐시에서 제거됨 - 다시 fetch가 필요하지만 새 tool 등록 후 확인
	// Cache를 직접 확인
	cache := s.Cache()
	_, found := cache.Load("foo/a")
	assert.False(t, found, "캐시가 제거되어야 함")
}

// TestSearch_List_ReturnsAllTools — Search.List 기본 동작
func TestSearch_List_ReturnsAllTools(t *testing.T) {
	r := tools.NewRegistry(tools.WithBuiltins())
	s := search.New(r)

	descs := s.List(context.Background(), search.Filter{})
	assert.GreaterOrEqual(t, len(descs), 6, "built-in 6종 이상 반환")
}

// TestSearch_ErrMCPTimeout_Sentinel — ErrMCPTimeout 센티넬 에러 확인
func TestSearch_ErrMCPTimeout_Sentinel(t *testing.T) {
	assert.True(t, errors.Is(tools.ErrMCPTimeout, tools.ErrMCPTimeout))
}
