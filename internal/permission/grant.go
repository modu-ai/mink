package permission

import (
	"context"
	"time"
)

// DecisionChoice는 Confirmer.Ask의 반환 선택지다.
// REQ-PE-006
type DecisionChoice int

const (
	// DecisionAlwaysAllow는 영구 grant 생성 후 Allow를 반환한다.
	DecisionAlwaysAllow DecisionChoice = iota
	// DecisionOnceOnly는 저장 없이 이번 1회만 Allow를 반환한다.
	DecisionOnceOnly
	// DecisionDeny는 거부한다.
	DecisionDeny
)

// SubjectType은 권한을 요청하는 주체의 유형이다.
type SubjectType string

const (
	SubjectSkill  SubjectType = "skill"
	SubjectMCP    SubjectType = "mcp"
	SubjectAgent  SubjectType = "agent"
	SubjectPlugin SubjectType = "plugin"
)

// PermissionRequest는 Manager.Check에 전달되는 권한 요청 구조체다.
// REQ-PE-006, REQ-PE-011
type PermissionRequest struct {
	// SubjectID는 "skill:my-tool", "mcp:github", "agent:planner" 형태의 식별자다.
	SubjectID string
	// SubjectType은 주체 유형이다.
	SubjectType SubjectType
	// ParentSubjectID는 sub-agent 상속용 부모 식별자다 (REQ-PE-011).
	ParentSubjectID string
	// InheritGrants는 부모 grant를 상속할지 여부다 (기본값 false, REQ-PE-011).
	InheritGrants bool
	// Capability는 요청하는 권한 카테고리다.
	Capability Capability
	// Scope는 매칭된 manifest scope 토큰 1건이다.
	Scope string
	// RequestedAt은 요청 시각이다.
	RequestedAt time.Time
}

// Grant는 영속화된 단일 권한 허가 기록이다.
// REQ-PE-003
type Grant struct {
	// ID는 UUIDv4 문자열이다.
	ID string `json:"id"`
	// SubjectID는 권한을 허가받은 주체 식별자다.
	SubjectID string `json:"subject_id"`
	// SubjectType은 주체 유형이다.
	SubjectType SubjectType `json:"subject_type"`
	// Capability는 허가된 권한 카테고리다.
	Capability Capability `json:"capability"`
	// Scope는 허가된 단일 scope 토큰이다.
	Scope string `json:"scope"`
	// GrantedAt은 허가 시각 (RFC3339)이다.
	GrantedAt time.Time `json:"granted_at"`
	// GrantedBy는 허가를 결정한 주체 (Confirmer가 채운다)다.
	GrantedBy string `json:"granted_by"`
	// ExpiresAt은 만료 시각이다. nil이면 영구 허가다.
	ExpiresAt *time.Time `json:"expires_at"`
	// Revoked는 취소 여부다.
	Revoked bool `json:"revoked"`
	// RevokedAt은 취소 시각이다.
	RevokedAt *time.Time `json:"revoked_at"`
}

// IsExpired는 grant가 만료되었는지 확인한다.
// REQ-PE-013
func (g *Grant) IsExpired(now time.Time) bool {
	return g.ExpiresAt != nil && now.After(*g.ExpiresAt)
}

// Decision은 Manager.Check의 최종 반환 결과다.
type Decision struct {
	// Allow는 요청이 허가되었는지 여부다.
	Allow bool
	// Choice는 Confirmer가 채운 DecisionChoice 값이다 (감사용).
	Choice DecisionChoice
	// ExpiresAt은 Confirmer가 부여한 TTL이다.
	ExpiresAt *time.Time
	// Reason은 거부 시 사람-친화 사유다.
	Reason string
}

// PermissionEvent는 Auditor.Record에 전달되는 이벤트 구조체다.
// REQ-PE-005
type PermissionEvent struct {
	// Type은 이벤트 유형이다: "grant_created" | "grant_reused" | "grant_denied" | "grant_revoked"
	Type string
	// SubjectID는 주체 식별자다.
	SubjectID string
	// Capability는 권한 카테고리다.
	Capability Capability
	// Scope는 scope 토큰이다.
	Scope string
	// Reason은 거부 사유다.
	Reason string
	// InheritedFrom은 부모 SubjectID (상속 시에만 사용)다.
	InheritedFrom string
	// Timestamp는 이벤트 시각이다.
	Timestamp time.Time
}

// Confirmer는 첫 호출 시 사용자 동의를 수집하는 인터페이스다.
// 실 구현은 orchestrator(MoAI / daemon)가 제공한다.
// REQ-PE-006, §6.1
type Confirmer interface {
	Ask(ctx context.Context, req PermissionRequest) (Decision, error)
}

// Auditor는 모든 결정 이벤트를 AUDIT-001 채널로 dispatch하는 인터페이스다.
// REQ-PE-005, §6.1
type Auditor interface {
	Record(event PermissionEvent) error
}

// NoopAuditor는 테스트 / SPEC-AUDIT-001 미배선 환경용 폴백이다.
type NoopAuditor struct{}

func (NoopAuditor) Record(PermissionEvent) error { return nil }

// BlockedAlwaysMatcher는 security.yaml의 blocked_always 목록을 검사하는 인터페이스다.
// 실 구현은 FS-ACCESS-001이 제공한다.
// REQ-PE-009
type BlockedAlwaysMatcher interface {
	Matches(cap Capability, scope string) bool
}

// NoopBlockedAlwaysMatcher는 항상 false를 반환하는 폴백 구현이다.
type NoopBlockedAlwaysMatcher struct{}

func (NoopBlockedAlwaysMatcher) Matches(Capability, string) bool { return false }

// AlwaysBlockMatcher는 특정 scope를 항상 차단하는 테스트용 구현이다.
type AlwaysBlockMatcher struct {
	BlockedScopes map[string]bool
}

func (m *AlwaysBlockMatcher) Matches(_ Capability, scope string) bool {
	return m.BlockedScopes[scope]
}
