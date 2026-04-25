// Package permissions는 tool 실행 권한 게이트 타입과 인터페이스를 정의한다.
// SPEC-GOOSE-QUERY-001 S0 T0.2
package permissions

import "context"

// CanUseTool는 tool 실행 전 권한을 확인하는 단일 gate 인터페이스이다.
// REQ-QUERY-006: 모든 tool_use는 이 인터페이스를 경유해야 한다.
//
// @MX:ANCHOR: [AUTO] 모든 tool 실행의 단일 security gate
// @MX:REASON: REQ-QUERY-006 - Allow/Deny/Ask 분기의 중앙 진입점. fan_in >= 3 예상(loop, test, future callers)
type CanUseTool interface {
	// Check는 주어진 컨텍스트와 권한 정보를 바탕으로 허용 여부를 결정한다.
	// Allow: 즉시 실행, Deny: 에러 결과 합성, Ask: 외부 결정 대기.
	Check(ctx context.Context, tpc ToolPermissionContext) Decision
}
