package credential

import (
	"context"
	"sync"
)

// DummySource는 테스트에서 사용하는 인메모리 CredentialSource 구현체이다.
// 실제 프로덕션 코드에서는 사용하지 않는다.
type DummySource struct {
	mu    sync.RWMutex
	creds []*PooledCredential
}

// NewDummySource는 주어진 크레덴셜 목록을 반환하는 DummySource를 생성한다.
func NewDummySource(creds []*PooledCredential) *DummySource {
	return &DummySource{creds: creds}
}

// Load는 현재 설정된 크레덴셜 목록의 복사본을 반환한다.
func (d *DummySource) Load(_ context.Context) ([]*PooledCredential, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.creds) == 0 {
		return nil, nil
	}
	// 깊은 복사: 원본 포인터를 유지하되 슬라이스 헤더는 복사
	result := make([]*PooledCredential, len(d.creds))
	copy(result, d.creds)
	return result, nil
}

// Update는 DummySource의 크레덴셜 목록을 교체한다. 테스트에서 Reload 시나리오에 사용한다.
func (d *DummySource) Update(creds []*PooledCredential) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.creds = creds
}
