package subagent

import (
	"context"
	"sync"

	"github.com/modu-ai/mink/internal/permissions"
)

// writeTool은 write-class tool 이름 집합이다.
// REQ-SA-016: background agent의 write tool 기본 거부.
var writeTools = map[string]bool{
	"write":       true,
	"edit":        true,
	"bash":        true,
	"create_file": true,
	"delete_file": true,
	"move_file":   true,
}

// isWriteTool은 tool 이름이 write-class인지 반환한다.
func isWriteTool(toolName string) bool {
	return writeTools[toolName]
}

// SettingsPermissions는 settings.json의 subagent.permissions.allow 설정이다.
// REQ-SA-016 / AC-SA-011
type SettingsPermissions struct {
	mu    sync.RWMutex
	allow []string // tool-name pattern 목록
}

// HasAllowRule은 toolName이 allow 규칙에 매치되는지 반환한다.
func (s *SettingsPermissions) HasAllowRule(toolName string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, pattern := range s.allow {
		if pattern == toolName || pattern == "*" {
			return true
		}
	}
	return false
}

// AddAllowRule은 allow 규칙을 추가한다 (테스트용).
func (s *SettingsPermissions) AddAllowRule(pattern string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allow = append(s.allow, pattern)
}

// planModeRegistry는 plan mode 상태를 관리하는 전역 레지스트리이다.
// REQ-SA-022: PlanModeApprove(agentID) 구현.
//
// @MX:WARN: [AUTO] 전역 sync.Map — 동시 spawn에서 plan mode 상태 공유
// @MX:REASON: REQ-SA-022 — PlanModeApprove는 부모가 별도 goroutine에서 호출. sync.Map으로 동시성 안전
var planModeRegistry sync.Map // map[agentID string]*planModeEntry

// planModeEntry는 plan mode 상태와 승인 채널을 담는다.
type planModeEntry struct {
	required bool
	approved chan struct{} // close시 승인
}

// registerPlanMode는 agentID를 plan mode 레지스트리에 등록한다.
func registerPlanMode(agentID string) *planModeEntry {
	entry := &planModeEntry{
		required: true,
		approved: make(chan struct{}),
	}
	planModeRegistry.Store(agentID, entry)
	return entry
}

// deregisterPlanMode는 agentID를 plan mode 레지스트리에서 제거한다.
func deregisterPlanMode(agentID string) {
	planModeRegistry.Delete(agentID)
}

// PlanModeApprove는 plan mode로 spawn된 sub-agent의 write 게이트를 해제한다.
// REQ-SA-022(c) / §6.2 API
//
// @MX:ANCHOR: [AUTO] plan mode 승인의 단일 API
// @MX:REASON: REQ-SA-022 — 부모 orchestrator만 이 함수를 호출할 수 있다. fan_in >= 3 예상
func PlanModeApprove(parentCtx context.Context, agentID string) error {
	if parentCtx.Err() != nil {
		return parentCtx.Err()
	}
	v, ok := planModeRegistry.Load(agentID)
	if !ok {
		return ErrAgentNotFound
	}
	entry := v.(*planModeEntry)
	if !entry.required {
		return ErrAgentNotInPlanMode
	}
	entry.required = false
	close(entry.approved)
	return nil
}

// TeammateCanUseTool은 TeammateIdentity 정책을 반영한 CanUseTool 구현이다.
// REQ-SA-010/016/022
//
// @MX:ANCHOR: [AUTO] sub-agent tool permission의 단일 gate
// @MX:REASON: SPEC-GOOSE-SUBAGENT-001 REQ-SA-010/016/022 — background write deny + bubble + plan mode
type TeammateCanUseTool struct {
	def              AgentDefinition
	parentCanUseTool permissions.CanUseTool
	settingsPerms    *SettingsPermissions
	planEntry        *planModeEntry // nil if not plan mode
}

// Check는 tool 사용 권한을 결정한다.
func (t *TeammateCanUseTool) Check(ctx context.Context, tpc permissions.ToolPermissionContext) permissions.Decision {
	toolName := tpc.ToolName

	// 1. plan mode write 차단 (REQ-SA-022b)
	if t.planEntry != nil && t.planEntry.required && isWriteTool(toolName) {
		return permissions.Decision{
			Behavior: permissions.Ask,
			Reason:   "plan_mode_required",
		}
	}

	// 2. Background agent write deny (REQ-SA-016)
	if t.def.Isolation == IsolationBackground && isWriteTool(toolName) {
		if t.settingsPerms == nil || !t.settingsPerms.HasAllowRule(toolName) {
			return permissions.Decision{
				Behavior: permissions.Deny,
				Reason:   "background_agent_write_denied",
			}
		}
	}

	// 3. PermissionMode 분기 (REQ-SA-010)
	switch t.def.PermissionMode {
	case "bubble":
		if t.parentCanUseTool != nil {
			return t.parentCanUseTool.Check(ctx, tpc)
		}
	case "isolated":
		// local policy만 사용
	}
	return permissions.Decision{Behavior: permissions.Allow}
}
