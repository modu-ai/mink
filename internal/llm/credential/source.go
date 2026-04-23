package credential

import "context"

// CredentialSource는 크레덴셜 목록을 로드하는 인터페이스이다.
//
// 반환되는 슬라이스는 크레덴셜 참조(메타데이터)만 포함한다.
// 실제 시크릿 값은 이 인터페이스를 통해 반환되지 않는다.
type CredentialSource interface {
	// Load는 크레덴셜 목록을 로드하여 반환한다.
	// 반환된 슬라이스는 소유권이 호출자로 이전된다.
	Load(ctx context.Context) ([]*PooledCredential, error)
}
