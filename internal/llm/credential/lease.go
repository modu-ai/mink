// Package credential lease.go는 소프트 리스 관련 타입과 함수를 제공한다.
// SPEC-GOOSE-ADAPTER-001 T-007 확장: AcquireLease + Lease.Release
package credential

// Lease는 크레덴셜에 대한 소프트 리스를 나타낸다.
// Lease를 보유한 동안 해당 크레덴셜은 다른 goroutine에서 선택되지 않는다.
// 사용 완료 후 반드시 Release()를 호출해야 한다.
type Lease struct {
	credID string
	pool   *CredentialPool
}

// Release는 리스를 반환하여 크레덴셜을 다시 선택 가능한 상태로 만든다.
// 이미 반환된 리스에 대한 Release는 no-op이다.
func (l *Lease) Release() {
	if l == nil || l.pool == nil {
		return
	}
	l.pool.mu.Lock()
	defer l.pool.mu.Unlock()

	c, ok := l.pool.creds[l.credID]
	if !ok {
		return
	}
	c.leased = false
	// pool 참조를 nil로 만들어 이중 Release를 no-op으로 처리
	l.pool = nil
}

// AcquireLease는 credID에 해당하는 크레덴셜에 대한 Lease를 반환한다.
// 크레덴셜이 이미 Select()를 통해 leased=true 상태라면 Lease 객체만 생성한다.
// 크레덴셜이 존재하지 않으면 nil을 반환한다.
//
// 주의: Select()로 이미 리스를 획득한 크레덴셜에 대해 AcquireLease를 사용할 것.
// AcquireLease는 Select()의 리스 획득을 대체하지 않는다.
func (p *CredentialPool) AcquireLease(id string) *Lease {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.creds[id]; !ok {
		return nil
	}
	return &Lease{credID: id, pool: p}
}
