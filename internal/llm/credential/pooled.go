package credential

import "time"

// PooledCredential은 풀에서 관리되는 크레덴셜의 참조 및 메타데이터를 나타낸다.
//
// Zero-Knowledge 설계: 이 구조체는 OS 키링에 대한 참조(KeyringID)만 포함한다.
// 실제 토큰, API 키 등의 시크릿 값은 절대 이 구조체에 저장되지 않는다.
// (SPEC-GOOSE-ARCH-REDESIGN-v0.2 Tier 4 준수)
type PooledCredential struct {
	// ID는 이 크레덴셜의 고유 식별자이다.
	ID string
	// Provider는 LLM 프로바이더 이름이다 (예: "anthropic", "openai").
	Provider string
	// KeyringID는 OS 키링의 참조 키이다. 실제 시크릿을 보유하지 않는다.
	KeyringID string
	// Status는 현재 크레덴셜 상태이다.
	Status CredStatus
	// ExpiresAt은 OAuth 크레덴셜의 만료 시각이다. API 키는 zero value이다.
	ExpiresAt time.Time
	// LastErrorAt은 마지막으로 에러가 발생한 시각이다.
	LastErrorAt time.Time
	// LastErrorReset은 마지막으로 에러 상태가 초기화된 시각이다.
	LastErrorReset time.Time
	// UsageCount는 이 크레덴셜이 선택된 총 횟수이다.
	UsageCount uint64
	// Priority는 PriorityStrategy에서 사용하는 우선순위 값이다. 값이 높을수록 우선 선택된다.
	Priority int
	// Weight는 WeightedStrategy에서 사용하는 가중치 값이다.
	Weight int

	// exhaustedUntil은 MarkExhausted 쿨다운 만료 시각이다 (내부 사용).
	exhaustedUntil time.Time
	// leased는 현재 리스 중인지 여부이다 (내부 사용).
	leased bool
}
