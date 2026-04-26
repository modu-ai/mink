package permission

import "time"

// Filter는 Store.List 조회 필터다.
type Filter struct {
	// SubjectID는 정확한 일치 필터다 (empty = any).
	SubjectID string
	// Capability는 선택적 카테고리 필터다.
	Capability *Capability
	// IncludeRevoked는 revoke된 grant도 포함할지 여부다.
	IncludeRevoked bool
	// IncludeExpired는 만료된 grant도 포함할지 여부다.
	IncludeExpired bool
}

// Store는 grant 영속화 인터페이스다.
// §6.1 공개 API surface.
// REQ-PE-004, REQ-PE-014
type Store interface {
	// Open은 store를 초기화한다 (파일 생성, schema 검증 등).
	Open() error
	// Lookup은 (subjectID, capability, scope) 조합의 유효한 grant를 반환한다.
	// 만료/revoke된 grant는 (_, false)를 반환한다.
	Lookup(subjectID string, cap Capability, scope string) (Grant, bool)
	// Save는 grant를 atomic write로 저장한다.
	Save(g Grant) error
	// Revoke는 subjectID의 모든 grant를 soft-delete한다.
	// 반환값은 revoke된 grant 수이다.
	Revoke(subjectID string) (revoked int, err error)
	// List는 필터 조건에 맞는 grant 목록을 반환한다.
	List(filter Filter) ([]Grant, error)
	// GC는 만료된 grant를 정리한다.
	GC(now time.Time) (pruned int, err error)
	// Close는 store를 정리한다.
	Close() error
}
