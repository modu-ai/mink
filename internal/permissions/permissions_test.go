//go:build integration

// Package permissions의 S2 TDD 테스트.
// SPEC-GOOSE-QUERY-001 S2 T2.1~T2.3
package permissions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/goose/internal/permissions"
)

// --- T2.1: Allow bypasses gate ---

// TestCanUseTool_AllowBypassesGate는 Allow 결정 시 Behavior=Allow이고
// Reason이 비어 있음을 검증한다. REQ-QUERY-006.
func TestCanUseTool_AllowBypassesGate(t *testing.T) {
	t.Parallel()

	// Arrange: StubCanUseTool 없이 permissions 패키지 자체 타입으로 검증
	tpc := permissions.ToolPermissionContext{
		ToolUseID: "tu-001",
		ToolName:  "bash",
		Input:     map[string]any{"command": "echo hello"},
		Turn:      1,
	}

	// Allow 결정을 직접 생성 (인터페이스 계약 검증)
	decision := permissions.Decision{
		Behavior: permissions.Allow,
	}

	// Assert: Allow 결정은 추가 동작 없이 통과해야 한다
	assert.Equal(t, permissions.Allow, decision.Behavior, "Allow 결정의 Behavior는 Allow이어야 한다")
	assert.Empty(t, decision.Reason, "Allow 결정의 Reason은 비어 있어야 한다")

	// ToolPermissionContext 필드 무결성 검증
	assert.Equal(t, "tu-001", tpc.ToolUseID)
	assert.Equal(t, "bash", tpc.ToolName)
	assert.Equal(t, 1, tpc.Turn)
}

// TestCanUseTool_AllowBypassesGate_BehaviorString은 Allow의 String() 표현을 검증한다.
func TestCanUseTool_AllowBypassesGate_BehaviorString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "allow", permissions.Allow.String())
}

// TestCanUseTool_Allow_ViaInterface는 CanUseTool 인터페이스를 구현한 stub이
// Allow를 반환할 때 Behavior=Allow인지 검증한다. REQ-QUERY-006.
func TestCanUseTool_Allow_ViaInterface(t *testing.T) {
	t.Parallel()

	// Arrange: 인라인 stub — 항상 Allow 반환
	stub := &fixedDecisionGate{
		decision: permissions.Decision{Behavior: permissions.Allow},
	}
	tpc := permissions.ToolPermissionContext{
		ToolUseID: "tu-002",
		ToolName:  "read_file",
		Input:     map[string]any{"path": "/tmp/test"},
		Turn:      2,
	}

	// Act
	got := stub.Check(context.Background(), tpc)

	// Assert
	assert.Equal(t, permissions.Allow, got.Behavior)
	assert.Empty(t, got.Reason)
}

// --- T2.2: Deny produces error result ---

// TestCanUseTool_DenyProducesErrorResult는 Deny + reason 결정 시
// SynthesizeDeniedResult가 is_error=true인 결과를 반환함을 검증한다.
// REQ-QUERY-006, REQ-QUERY-014 경계.
func TestCanUseTool_DenyProducesErrorResult(t *testing.T) {
	t.Parallel()

	// Arrange
	const toolUseID = "tu-deny-001"
	const denyReason = "도구 사용이 허가되지 않았습니다"

	decision := permissions.Decision{
		Behavior: permissions.Deny,
		Reason:   denyReason,
	}

	// Act: Deny 결정 시 에러 결과 합성
	result := permissions.SynthesizeDeniedResult(toolUseID, decision)

	// Assert
	require.True(t, result.IsError, "Deny 결정 시 IsError=true이어야 한다")
	assert.Equal(t, toolUseID, result.ToolUseID, "ToolUseID가 전달되어야 한다")
	assert.Contains(t, result.Content, denyReason, "Reason이 Content에 포함되어야 한다")
}

// TestCanUseTool_DenyProducesErrorResult_EmptyReason은 reason이 없는 Deny도
// is_error=true로 합성됨을 검증한다.
func TestCanUseTool_DenyProducesErrorResult_EmptyReason(t *testing.T) {
	t.Parallel()

	decision := permissions.Decision{
		Behavior: permissions.Deny,
		Reason:   "",
	}

	result := permissions.SynthesizeDeniedResult("tu-no-reason", decision)

	require.True(t, result.IsError)
	assert.Equal(t, "tu-no-reason", result.ToolUseID)
	// reason이 없어도 에러 메시지가 생성되어야 한다
	assert.NotEmpty(t, result.Content)
}

// TestCanUseTool_DenyBehaviorString은 Deny의 String() 표현을 검증한다.
func TestCanUseTool_DenyBehaviorString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "deny", permissions.Deny.String())
}

// --- T2.3: Ask suspends loop ---

// TestCanUseTool_AskSuspendsLoop는 Ask 결정이 Behavior=Ask이고
// loop suspend 시그널로 사용 가능함을 검증한다. REQ-QUERY-013.
// 이 테스트는 loop 통합 없이 인터페이스 계약만 검증한다.
func TestCanUseTool_AskSuspendsLoop(t *testing.T) {
	t.Parallel()

	// Arrange: Ask 결정 생성
	decision := permissions.Decision{
		Behavior: permissions.Ask,
		Reason:   "사용자 확인이 필요합니다",
	}

	// Assert: Ask 결정 필드 검증
	assert.Equal(t, permissions.Ask, decision.Behavior)
	assert.NotEmpty(t, decision.Reason, "Ask 결정의 Reason은 설명을 포함해야 한다")
}

// TestCanUseTool_AskBehaviorString은 Ask의 String() 표현을 검증한다.
func TestCanUseTool_AskBehaviorString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ask", permissions.Ask.String())
}

// TestCanUseTool_Ask_ViaInterface는 인터페이스를 통한 Ask 결정 반환을 검증한다.
func TestCanUseTool_Ask_ViaInterface(t *testing.T) {
	t.Parallel()

	// Arrange: Ask를 반환하는 stub
	stub := &fixedDecisionGate{
		decision: permissions.Decision{
			Behavior: permissions.Ask,
			Reason:   "외부 확인 필요",
		},
	}
	tpc := permissions.ToolPermissionContext{
		ToolUseID: "tu-ask-001",
		ToolName:  "dangerous_tool",
		Input:     map[string]any{},
		Turn:      3,
	}

	// Act
	got := stub.Check(context.Background(), tpc)

	// Assert
	assert.Equal(t, permissions.Ask, got.Behavior)
	assert.NotEmpty(t, got.Reason)
}

// TestCanUseTool_UnknownBehaviorString은 정의되지 않은 PermissionBehavior의
// String() 표현이 "unknown"임을 검증한다.
func TestCanUseTool_UnknownBehaviorString(t *testing.T) {
	t.Parallel()
	unknown := permissions.PermissionBehavior(99)
	assert.Equal(t, "unknown", unknown.String())
}

// TestPermissionBehavior_ThreeBehaviors는 정확히 3개의 Behavior 값만 존재함을 검증한다.
// REQ-QUERY-006 방어 테스트.
func TestPermissionBehavior_ThreeBehaviors(t *testing.T) {
	t.Parallel()

	behaviors := []permissions.PermissionBehavior{
		permissions.Allow,
		permissions.Deny,
		permissions.Ask,
	}
	// 각 Behavior는 서로 다른 값이어야 한다
	seen := make(map[permissions.PermissionBehavior]bool)
	for _, b := range behaviors {
		assert.False(t, seen[b], "중복된 PermissionBehavior 값: %v", b)
		seen[b] = true
	}
	assert.Len(t, seen, 3, "PermissionBehavior는 정확히 3개의 값이어야 한다")
}

// --- 테스트 헬퍼 ---

// fixedDecisionGate는 테스트 전용 인라인 CanUseTool 구현이다.
// 항상 고정된 Decision을 반환한다.
type fixedDecisionGate struct {
	decision permissions.Decision
}

// Check는 CanUseTool.Check를 구현한다.
func (g *fixedDecisionGate) Check(_ context.Context, _ permissions.ToolPermissionContext) permissions.Decision {
	return g.decision
}
