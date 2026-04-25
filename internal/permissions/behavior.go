// Package permissions는 tool 실행 권한 게이트 타입과 인터페이스를 정의한다.
// SPEC-GOOSE-QUERY-001 S0 T0.2
package permissions

// PermissionBehavior는 CanUseTool.Check 반환값으로 loop 분기를 결정한다.
// REQ-QUERY-006 Allow/Deny/Ask 3종 분기.
type PermissionBehavior int

const (
	// Allow는 tool 즉시 실행을 허가한다.
	Allow PermissionBehavior = iota
	// Deny는 tool 실행을 거부하고 에러 결과를 합성한다.
	Deny
	// Ask는 외부 결정을 대기하고 loop를 suspend한다.
	Ask
)

// String은 PermissionBehavior의 문자열 표현을 반환한다.
func (b PermissionBehavior) String() string {
	switch b {
	case Allow:
		return "allow"
	case Deny:
		return "deny"
	case Ask:
		return "ask"
	default:
		return "unknown"
	}
}

// Decision은 CanUseTool.Check의 반환 타입이다.
type Decision struct {
	// Behavior는 권한 결정 결과이다.
	Behavior PermissionBehavior
	// Reason은 Deny/Ask 시 이유이다.
	Reason string
}
