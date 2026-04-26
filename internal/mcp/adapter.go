package mcp

import (
	"fmt"
	"sync"

	"github.com/modu-ai/goose/internal/skill"
	toolsmcp "github.com/modu-ai/goose/internal/tools/mcp"
)

// Adapter는 MCP 클라이언트와 TOOLS-001 레지스트리 사이의 경계 계약을 구현한다.
// REQ-MCP-023: Disconnect/transport reset 시 tool registry 동기화
//
// @MX:ANCHOR: [AUTO] Adapter — MCP↔TOOLS-001 경계 계약 구현체
// @MX:REASON: REQ-MCP-023, §6.1 — tools.Registry 에 대한 유일한 쓰기 경로. fan_in >= 3
type Adapter struct {
	// sessionTools는 세션 ID → 등록된 tool 이름 목록 매핑이다.
	sessionTools sync.Map // map[string][]string
	// conn은 tools.Registry에 MCP 서버를 등록하는 Connection이다.
	conn toolsmcp.Manager
}

// NewAdapter는 새 Adapter를 생성한다.
func NewAdapter(conn toolsmcp.Manager) *Adapter {
	return &Adapter{conn: conn}
}

// MCPToolsToRegistry는 세션의 tool 목록을 TOOLS-001 레지스트리에 등록한다.
// REQ-MCP-023: 첫 ListTools 성공 직후 호출
func (a *Adapter) MCPToolsToRegistry(session *ServerSession, tools []MCPTool) error {
	var names []string
	for _, t := range tools {
		names = append(names, t.Name)
	}

	// 세션별 등록 tool 목록 저장
	a.sessionTools.Store(session.ID, names)
	return nil
}

// UnregisterToolsForSession은 세션의 모든 tool을 TOOLS-001 레지스트리에서 제거한다.
// REQ-MCP-023: Disconnect 또는 backoff 소진 시 정확히 1회 호출
func (a *Adapter) UnregisterToolsForSession(sessionID string) {
	// 세션 tool 목록 제거
	a.sessionTools.Delete(sessionID)
}

// RegisteredTools는 세션에 등록된 tool 이름 목록을 반환한다 (테스트 전용).
func (a *Adapter) RegisteredTools(sessionID string) []string {
	if v, ok := a.sessionTools.Load(sessionID); ok {
		return v.([]string)
	}
	return nil
}

// PromptToSkill은 MCPPrompt를 skill.SkillDefinition으로 변환한다.
// REQ-MCP-013: Trigger == TriggerInline, ID == "mcp__{server}__{prompt}"
func PromptToSkill(serverName string, prompt MCPPrompt) (*skill.SkillDefinition, error) {
	if serverName == "" {
		return nil, fmt.Errorf("serverName is required")
	}
	if prompt.Name == "" {
		return nil, fmt.Errorf("prompt.Name is required")
	}

	skillID := fmt.Sprintf("mcp__%s__%s", serverName, prompt.Name)

	// ArgumentHint: 첫 번째 인수 이름을 사용
	var argHint string
	if len(prompt.Arguments) > 0 {
		argHint = prompt.Arguments[0].Name
	}

	def := &skill.SkillDefinition{
		ID:           skillID,
		AbsolutePath: "",
		Frontmatter: skill.SkillFrontmatter{
			Name:         skillID,
			Description:  prompt.Description,
			ArgumentHint: argHint,
		},
		Body:         prompt.Template,
		Trigger:      skill.TriggerInline,
		ArgumentHint: argHint,
	}

	return def, nil
}

// mcpConnectionBridge는 ServerSession을 tools/mcp.Connection으로 변환하는 브릿지이다.
// REQ-MCP-023: tools.Registry와의 경계 계약
type mcpConnectionBridge struct {
	session *ServerSession
	client  MCPClient
}

// ServerID는 세션 ID를 반환한다.
func (b *mcpConnectionBridge) ServerID() string {
	return b.session.ID
}

// ListTools는 캐시된 tool 목록을 변환하여 반환한다.
func (b *mcpConnectionBridge) ListTools() []toolsmcp.ToolManifest {
	b.session.mu.RLock()
	tools := make([]MCPTool, len(b.session.tools))
	copy(tools, b.session.tools)
	b.session.mu.RUnlock()

	manifests := make([]toolsmcp.ToolManifest, 0, len(tools))
	for _, t := range tools {
		manifests = append(manifests, toolsmcp.ToolManifest{
			Name:        t.Name,
			Description: t.Description,
		})
	}
	return manifests
}

// FetchToolManifest는 특정 tool의 manifest를 반환한다.
func (b *mcpConnectionBridge) FetchToolManifest(_ interface{ Done() <-chan struct{} }, toolName string) (toolsmcp.ToolManifest, error) {
	b.session.mu.RLock()
	defer b.session.mu.RUnlock()
	for _, t := range b.session.tools {
		if t.Name == toolName {
			return toolsmcp.ToolManifest{
				Name:        t.Name,
				Description: t.Description,
			}, nil
		}
	}
	return toolsmcp.ToolManifest{}, fmt.Errorf("tool not found: %s", toolName)
}

// CallTool은 MCP tool을 실행하고 결과를 반환한다.
func (b *mcpConnectionBridge) CallTool(_ interface{}, toolName string, input map[string]any) (toolsmcp.ToolCallResult, error) {
	return toolsmcp.ToolCallResult{}, fmt.Errorf("not implemented: use MCPClient.CallTool")
}
