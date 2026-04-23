// Package credential lease.go는 소프트 리스 관련 도우미 함수를 제공한다.
// 현재 MVP에서는 CredentialPool의 Select/Release가 리스 기능을 직접 제공하며
// 이 파일은 향후 확장을 위한 구조를 정의한다.
package credential

// 이 파일은 SPEC-GOOSE-CREDPOOL-001 MVP에서는 구조 정의 역할을 한다.
// 구체적인 소프트 리스 구현 (context.Context 기반 TTL, 자동 반환 등)은
// SPEC-GOOSE-LEASE-001에서 구현될 예정이다.
