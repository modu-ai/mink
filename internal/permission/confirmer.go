package permission

import (
	"context"
)

// AlwaysAllowConfirmer는 모든 요청에 AlwaysAllow를 반환하는 테스트용 Confirmer다.
type AlwaysAllowConfirmer struct{}

func (AlwaysAllowConfirmer) Ask(_ context.Context, _ PermissionRequest) (Decision, error) {
	return Decision{Allow: true, Choice: DecisionAlwaysAllow}, nil
}

// DefaultDenyConfirmer는 모든 요청에 Deny를 반환하는 폴백 Confirmer다.
// R7: Confirmer가 nil일 때 사용되는 폴백.
type DefaultDenyConfirmer struct{}

func (DefaultDenyConfirmer) Ask(_ context.Context, _ PermissionRequest) (Decision, error) {
	return Decision{Allow: false, Choice: DecisionDeny, Reason: "no confirmer configured"}, nil
}

// OnceOnlyConfirmer는 항상 OnceOnly를 반환하는 테스트용 Confirmer다.
type OnceOnlyConfirmer struct{}

func (OnceOnlyConfirmer) Ask(_ context.Context, _ PermissionRequest) (Decision, error) {
	return Decision{Allow: true, Choice: DecisionOnceOnly}, nil
}
