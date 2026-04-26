// Package store는 permission grant의 영속화 구현을 제공한다.
// permission.Store 인터페이스는 permission 패키지에 정의되어 있다.
// SPEC: SPEC-GOOSE-PERMISSION-001 v0.2.0 Phase 2
package store

// CurrentSchemaVersion은 grants.json의 현재 schema 버전이다.
// REQ-PE-017: 미스매치 시 ErrIncompatibleStoreVersion 반환.
const CurrentSchemaVersion = 1
