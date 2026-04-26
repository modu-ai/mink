// Package permission은 AI.GOOSE의 선언 기반 권한 시스템(Declared Permission)을 구현한다.
// frontmatter requires: 스키마 파싱, 첫 호출 확인(Confirmer), grant 영속화(Store),
// 감사 로그(Auditor), sub-agent 상속을 제공한다.
//
// SPEC: SPEC-GOOSE-PERMISSION-001 v0.2.0
package permission

import "fmt"

// ErrUnknownCapability는 requires: 에 알 수 없는 카테고리 키가 있을 때 반환된다.
// REQ-PE-002
type ErrUnknownCapability struct {
	Key string
}

func (e ErrUnknownCapability) Error() string {
	return fmt.Sprintf("unknown capability category: %q", e.Key)
}

// ErrInvalidScopeShape는 카테고리 값이 배열이 아닌 스칼라이거나 중첩 구조일 때 반환된다.
// REQ-PE-010, REQ-PE-018
type ErrInvalidScopeShape struct {
	Category string
	Value    any
	Nested   bool
}

func (e ErrInvalidScopeShape) Error() string {
	if e.Nested {
		return "invalid requires: shape: nested requires: is not allowed"
	}
	return fmt.Sprintf("invalid scope shape for category %q: expected []string, got %T (%v)", e.Category, e.Value, e.Value)
}

// ErrUndeclaredCapability는 subject의 manifest에 선언되지 않은 capability를 요청할 때 반환된다.
// REQ-PE-001
type ErrUndeclaredCapability struct {
	Capability Capability
	Scope      string
}

func (e ErrUndeclaredCapability) Error() string {
	return fmt.Sprintf("capability %s (scope %q) is not declared in manifest requires:", e.Capability, e.Scope)
}

// ErrBlockedByPolicy는 security.yaml blocked_always 목록과 교차 차단될 때 반환된다.
// REQ-PE-009
type ErrBlockedByPolicy struct {
	Capability Capability
	Scope      string
}

func (e ErrBlockedByPolicy) Error() string {
	return fmt.Sprintf("capability %s (scope %q) is blocked by security policy (blocked_always)", e.Capability, e.Scope)
}

// ErrSubjectNotReady는 Manager.Register 없이 Check를 호출할 때 반환된다.
// REQ-PE-012
type ErrSubjectNotReady struct {
	SubjectID string
}

func (e ErrSubjectNotReady) Error() string {
	return fmt.Sprintf("subject %q is not registered; call Manager.Register first", e.SubjectID)
}

// ErrStoreFilePermissions는 grant 파일 모드가 0600을 초과할 때 반환된다.
// REQ-PE-004, AC-PE-007
type ErrStoreFilePermissions struct {
	Path string
	Mode uint32
}

func (e ErrStoreFilePermissions) Error() string {
	return fmt.Sprintf("grant store file %q has insecure permissions %04o (must be 0600 or stricter)", e.Path, e.Mode)
}

// ErrIncompatibleStoreVersion은 grants.json schema_version이 현재 코드와 다를 때 반환된다.
// REQ-PE-017
type ErrIncompatibleStoreVersion struct {
	Path     string
	Got      int
	Expected int
}

func (e ErrIncompatibleStoreVersion) Error() string {
	return fmt.Sprintf("grant store %q has schema_version %d but expected %d; run 'goose permission migrate --from %d'",
		e.Path, e.Got, e.Expected, e.Got)
}

// ErrStoreNotReady는 Store가 아직 Open()되지 않았거나 실패한 상태에서 접근할 때 반환된다.
type ErrStoreNotReady struct {
	Reason string
}

func (e ErrStoreNotReady) Error() string {
	return fmt.Sprintf("grant store is not ready: %s", e.Reason)
}

// ErrConfirmerRequired는 Manager.New에 nil Confirmer를 전달할 때 반환된다.
// R7 mitigate
var ErrConfirmerRequired = fmt.Errorf("confirmer must not be nil; provide at least DefaultDenyConfirmer")

// ErrIntegrityCheckFailed는 plugin integrity check가 실패할 때 반환된다.
// REQ-PE-020
type ErrIntegrityCheckFailed struct {
	SubjectID string
	Reason    string
}

func (e ErrIntegrityCheckFailed) Error() string {
	return fmt.Sprintf("integrity check failed for %q: %s", e.SubjectID, e.Reason)
}
