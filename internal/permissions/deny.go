// Package permissions의 Deny 결정 처리 helper.
// SPEC-GOOSE-QUERY-001 S2 T2.2
package permissions

import "fmt"

// DeniedResult는 Deny 결정 시 합성되는 tool 실행 거부 결과이다.
// REQ-QUERY-006: Deny 분기에서 Executor.Run 호출 없이 에러 결과를 직접 합성한다.
// REQ-QUERY-014: is_error=true 결과는 loop 종료 없이 tool_result로 포함된다.
//
// @MX:NOTE: [AUTO] Deny 결정 시 Executor를 호출하지 않고 이 구조체를 직접 합성한다.
// @MX:SPEC: SPEC-GOOSE-QUERY-001 REQ-QUERY-006
type DeniedResult struct {
	// ToolUseID는 거부된 tool_use 블록의 ID이다.
	ToolUseID string
	// IsError는 항상 true이다 (Deny = 에러 결과).
	IsError bool
	// Content는 거부 이유를 포함하는 사람이 읽을 수 있는 메시지이다.
	Content string
}

// SynthesizeDeniedResult는 Deny 결정으로부터 DeniedResult를 합성한다.
// loop가 Deny 결정을 받으면 Executor.Run을 호출하지 않고
// 이 함수를 통해 is_error=true인 tool_result를 생성한다.
// REQ-QUERY-006, REQ-QUERY-014.
//
// @MX:ANCHOR: [AUTO] Deny 분기의 에러 결과 합성 단일 진입점
// @MX:REASON: S4 T4.2 TestQueryLoop_PermissionDeny_SynthesizesErrorResult에서 호출. fan_in >= 2 예상
func SynthesizeDeniedResult(toolUseID string, d Decision) DeniedResult {
	content := d.Reason
	if content == "" {
		content = fmt.Sprintf("tool use %q was denied by permission gate", toolUseID)
	}
	return DeniedResult{
		ToolUseID: toolUseID,
		IsError:   true,
		Content:   content,
	}
}
