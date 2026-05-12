package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/modu-ai/mink/internal/tools/mcp"
)

// mcpFetchTimeout은 MCP manifest fetch 최대 대기 시간이다.
// REQ-TOOLS-008: 5초 이내 완료 또는 ErrMCPTimeout
const mcpFetchTimeout = 5 * time.Second

// mcpStubTool은 deferred loading을 지원하는 MCP tool stub이다.
// REQ-TOOLS-007: 첫 Call 시 manifest fetch 후 real tool로 위임.
//
// @MX:WARN: [AUTO] sync.Once 사용 — fetch 실패 시 재시도 불가
// @MX:REASON: REQ-TOOLS-007 (Option a) - Once 재시도는 Search.InvalidateCache + MCP reconnect로만 가능. R1 리스크 참조
type mcpStubTool struct {
	conn        mcp.Connection
	serverID    string
	toolName    string
	manifest    mcp.ToolManifest
	realOnce    sync.Once
	realTool    mcpRealTool
	fetchErr    error
	schema      json.RawMessage
	sharedCache *sync.Map // Search.cache와 공유 (key: "serverID/toolName")
}

// mcpRealTool은 실제 MCP 연결로 tool을 실행하는 구조체이다.
type mcpRealTool struct {
	conn     mcp.Connection
	manifest mcp.ToolManifest
}

// newMCPStubTool은 deferred loading 스텁 tool을 생성한다.
func newMCPStubTool(conn mcp.Connection, serverID, toolName string, manifest mcp.ToolManifest) *mcpStubTool {
	return &mcpStubTool{
		conn:     conn,
		serverID: serverID,
		toolName: toolName,
		manifest: manifest,
		schema:   buildMCPSchema(manifest),
	}
}

// newMCPStubToolWithCache는 Search.cache를 공유하는 스텁 tool을 생성한다.
func newMCPStubToolWithCache(conn mcp.Connection, serverID, toolName string, manifest mcp.ToolManifest, cache *sync.Map) *mcpStubTool {
	stub := newMCPStubTool(conn, serverID, toolName, manifest)
	stub.sharedCache = cache
	return stub
}

// Name은 canonical MCP tool 이름을 반환한다.
func (m *mcpStubTool) Name() string {
	return "mcp__" + m.serverID + "__" + m.toolName
}

// Schema는 tool의 JSON Schema를 반환한다.
func (m *mcpStubTool) Schema() json.RawMessage {
	return m.schema
}

// Scope는 기본적으로 ScopeShared를 반환한다.
func (m *mcpStubTool) Scope() Scope {
	return ScopeShared
}

// Call은 첫 호출 시 manifest를 fetch하고, 이후 MCP 연결을 통해 tool을 실행한다.
// REQ-TOOLS-007: stub의 첫 Call이 FetchToolManifest를 정확히 1회 호출.
func (m *mcpStubTool) Call(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	// 캐시 먼저 확인
	cacheKey := m.serverID + "/" + m.toolName
	if m.sharedCache != nil {
		if cached, ok := m.sharedCache.Load(cacheKey); ok {
			manifest := cached.(mcp.ToolManifest)
			return callMCPTool(ctx, m.conn, manifest.Name, input)
		}
	}

	// fetch (Once로 1회 보장)
	m.realOnce.Do(func() {
		fetchCtx, cancel := context.WithTimeout(ctx, mcpFetchTimeout)
		defer cancel()
		manifest, err := m.conn.FetchToolManifest(fetchCtx, m.toolName)
		if err != nil {
			m.fetchErr = err
			return
		}
		m.realTool = mcpRealTool{conn: m.conn, manifest: manifest}
		// 캐시에 저장
		if m.sharedCache != nil {
			m.sharedCache.Store(cacheKey, manifest)
		}
	})

	if m.fetchErr != nil {
		return ToolResult{IsError: true, Content: []byte("mcp_activation_failed: " + m.fetchErr.Error())}, nil
	}

	return callMCPTool(ctx, m.realTool.conn, m.realTool.manifest.Name, input)
}

// FetchManifest는 MCP manifest를 fetch하고 캐시한다.
// Search.Activate에서 호출된다.
func (m *mcpStubTool) FetchManifest(ctx context.Context) (mcp.ToolManifest, error) {
	var fetchResult mcp.ToolManifest
	var outerErr error

	m.realOnce.Do(func() {
		manifest, err := m.conn.FetchToolManifest(ctx, m.toolName)
		if err != nil {
			m.fetchErr = err
			outerErr = err
			return
		}
		m.realTool = mcpRealTool{conn: m.conn, manifest: manifest}
		fetchResult = manifest
		if m.sharedCache != nil {
			m.sharedCache.Store(m.serverID+"/"+m.toolName, manifest)
		}
	})

	if m.fetchErr != nil {
		return mcp.ToolManifest{}, m.fetchErr
	}
	if outerErr != nil {
		return mcp.ToolManifest{}, outerErr
	}
	return fetchResult, nil
}

// callMCPTool은 MCP 연결을 통해 tool을 실행한다.
func callMCPTool(ctx context.Context, conn mcp.Connection, toolName string, input json.RawMessage) (ToolResult, error) {
	var inputMap map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &inputMap); err != nil {
			return ToolResult{IsError: true, Content: []byte(fmt.Sprintf("invalid input JSON: %v", err))}, nil
		}
	}

	result, err := conn.CallTool(ctx, toolName, inputMap)
	if err != nil {
		return ToolResult{IsError: true, Content: []byte(err.Error())}, nil
	}
	return ToolResult{
		Content: result.Content,
		IsError: result.IsError,
	}, nil
}
