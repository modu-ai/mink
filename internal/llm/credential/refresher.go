package credential

import "context"

// Refresher는 OAuth 크레덴셜 갱신 인터페이스이다.
//
// 구현체는 ADAPTER-001 및 CREDENTIAL-PROXY-001 SPEC에서 제공된다.
// 현재 MVP에서는 인터페이스 정의만 포함한다.
//
// 중요: 구현체는 시크릿 값이 agent 프로세스 메모리에 들어오지 않도록
// goose-proxy를 경유하여 갱신을 수행해야 한다.
type Refresher interface {
	// Refresh는 만료되거나 갱신이 필요한 크레덴셜을 갱신한다.
	// 구현체는 시크릿 값을 agent 메모리에 노출해서는 안 된다.
	Refresh(ctx context.Context, cred *PooledCredential) error
}
