// Package credential는 LLM 프로바이더 크레덴셜 풀 오케스트레이션 레이어를 구현한다.
//
// Zero-Knowledge 원칙: 이 패키지는 크레덴셜 참조와 메타데이터만 관리한다.
// 실제 토큰, API 키 등의 시크릿 값은 이 패키지 어디에도 저장되지 않는다.
// 시크릿 조회는 goose-proxy (SPEC-GOOSE-CREDENTIAL-PROXY-001, M5)가 담당한다.
package credential

// CredStatus는 크레덴셜의 현재 상태를 나타낸다.
type CredStatus int

const (
	// CredOK는 크레덴셜이 정상적으로 사용 가능한 상태이다.
	CredOK CredStatus = iota
	// CredPending은 크레덴셜이 갱신 대기 중인 상태이다 (OAuth 리프레시 등).
	CredPending
	// CredExhausted는 크레덴셜이 쿨다운 기간 동안 사용 불가한 상태이다.
	CredExhausted
)
