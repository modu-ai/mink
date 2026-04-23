package credential

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrExhausted는 사용 가능한 크레덴셜이 없을 때 반환되는 에러이다.
var ErrExhausted = errors.New("credential: 사용 가능한 크레덴셜이 없음")

// ErrNotFound는 풀에서 해당 크레덴셜을 찾을 수 없을 때 반환되는 에러이다.
var ErrNotFound = errors.New("credential: 크레덴셜을 찾을 수 없음")

// ErrAlreadyReleased는 이미 반환된 크레덴셜을 다시 Release하려 할 때 반환되는 에러이다.
var ErrAlreadyReleased = errors.New("credential: 크레덴셜이 이미 반환됨")

// Option은 CredentialPool 생성 옵션 함수 타입이다.
type Option func(*CredentialPool)

// CredentialPool은 LLM 프로바이더 크레덴셜의 풀 오케스트레이터이다.
//
// 이 풀은 크레덴셜 참조와 메타데이터만 관리한다.
// 실제 시크릿 값은 OS 키링에 있으며 이 구조체에 저장되지 않는다.
// 모든 공개 메서드는 스레드 안전하다.
type CredentialPool struct {
	mu       sync.RWMutex
	source   CredentialSource
	strategy SelectionStrategy
	// creds는 ID → PooledCredential 맵이다. 풀의 전체 크레덴셜을 보유한다.
	creds map[string]*PooledCredential
	// order는 로드 순서를 보존하기 위한 ID 슬라이스이다.
	order []string
}

// New는 주어진 Source와 SelectionStrategy로 CredentialPool을 생성한다.
// Source.Load()를 호출하여 초기 크레덴셜을 로드한다.
// @MX:ANCHOR: [AUTO] 풀 생성 진입점 — pool.go 전체 공개 API의 루트 생성자
// @MX:REASON: credential 패키지의 모든 사용자가 이 함수를 통해 풀을 생성 (fan_in >= 3 예상)
// @MX:SPEC: SPEC-GOOSE-CREDPOOL-001
func New(source CredentialSource, strategy SelectionStrategy, opts ...Option) (*CredentialPool, error) {
	p := &CredentialPool{
		source:   source,
		strategy: strategy,
		creds:    make(map[string]*PooledCredential),
	}
	for _, opt := range opts {
		opt(p)
	}

	if err := p.load(context.Background()); err != nil {
		return nil, err
	}
	return p, nil
}

// load는 Source로부터 크레덴셜을 로드하여 풀을 (재)초기화한다.
// 이미 리스 중인 크레덴셜의 상태는 보존된다.
func (p *CredentialPool) load(ctx context.Context) error {
	loaded, err := p.source.Load(ctx)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 새 맵과 순서 슬라이스를 구성한다.
	newCreds := make(map[string]*PooledCredential, len(loaded))
	newOrder := make([]string, 0, len(loaded))

	for _, c := range loaded {
		// 기존 크레덴셜이 있으면 리스 상태와 에러 상태를 보존한다.
		if existing, ok := p.creds[c.ID]; ok {
			c.leased = existing.leased
			c.exhaustedUntil = existing.exhaustedUntil
			c.LastErrorAt = existing.LastErrorAt
			c.LastErrorReset = existing.LastErrorReset
			c.UsageCount = existing.UsageCount
		}
		newCreds[c.ID] = c
		newOrder = append(newOrder, c.ID)
	}

	p.creds = newCreds
	p.order = newOrder
	return nil
}

// available은 현재 선택 가능한 크레덴셜 목록을 반환한다.
// 쓰기 락이 획득된 컨텍스트에서 호출되어야 한다 (쿨다운 만료 시 상태를 변경하기 때문).
func (p *CredentialPool) available() []*PooledCredential {
	now := time.Now()
	result := make([]*PooledCredential, 0, len(p.creds))
	for _, id := range p.order {
		c, ok := p.creds[id]
		if !ok {
			continue
		}
		if c.leased {
			continue
		}
		if c.Status == CredExhausted {
			if now.Before(c.exhaustedUntil) {
				// 아직 쿨다운 중: 선택 불가
				continue
			}
			// 쿨다운 만료: 상태 복구 (쓰기 락 필요)
			c.Status = CredOK
		}
		result = append(result, c)
	}
	return result
}

// availableCount는 쓰기 잠금 없이 선택 가능한 크레덴셜 수를 반환한다.
// 쿨다운 상태 복구를 수행하지 않으므로 Size() 읽기 락 컨텍스트에서 사용된다.
func (p *CredentialPool) availableCount() int {
	now := time.Now()
	count := 0
	for _, id := range p.order {
		c, ok := p.creds[id]
		if !ok {
			continue
		}
		if c.leased {
			continue
		}
		if c.Status == CredExhausted && now.Before(c.exhaustedUntil) {
			continue
		}
		count++
	}
	return count
}

// Select는 전략에 따라 사용 가능한 크레덴셜 중 하나를 선택하고 리스를 획득한다.
// 사용 가능한 크레덴셜이 없으면 ErrExhausted를 반환한다.
// ctx가 취소되어 있으면 ctx.Err()를 반환한다.
// @MX:ANCHOR: [AUTO] 크레덴셜 선택 핫패스 — 모든 LLM 요청이 이 메서드를 통과
// @MX:REASON: LLM 요청 경로의 핵심 게이트웨이; 동시성 버그 수정 시 race 테스트 필수
// @MX:SPEC: SPEC-GOOSE-CREDPOOL-001
func (p *CredentialPool) Select(ctx context.Context) (*PooledCredential, error) {
	// 컨텍스트 취소 확인
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	avail := p.available()
	if len(avail) == 0 {
		return nil, ErrExhausted
	}

	cred, err := p.strategy.Select(avail)
	if err != nil {
		return nil, ErrExhausted
	}

	// 리스 획득
	cred.leased = true
	cred.UsageCount++
	return cred, nil
}

// Release는 리스 중인 크레덴셜을 풀로 반환한다.
// 이미 반환된 크레덴셜을 다시 반환하려 하면 ErrAlreadyReleased를 반환한다.
func (p *CredentialPool) Release(cred *PooledCredential) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	c, ok := p.creds[cred.ID]
	if !ok {
		return ErrNotFound
	}
	if !c.leased {
		return ErrAlreadyReleased
	}
	c.leased = false
	return nil
}

// MarkExhausted는 크레덴셜을 쿨다운 기간 동안 Exhausted 상태로 표시한다.
// 쿨다운이 만료되면 크레덴셜은 자동으로 CredOK 상태로 복구된다.
func (p *CredentialPool) MarkExhausted(cred *PooledCredential, cooldownDur time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	c, ok := p.creds[cred.ID]
	if !ok {
		return ErrNotFound
	}
	c.Status = CredExhausted
	c.exhaustedUntil = time.Now().Add(cooldownDur)
	c.leased = false
	return nil
}

// MarkError는 크레덴셜에 에러 발생 시각을 기록한다.
// 기본적으로 Exhausted 상태로 전환하지 않는다.
func (p *CredentialPool) MarkError(cred *PooledCredential, _ error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	c, ok := p.creds[cred.ID]
	if !ok {
		return ErrNotFound
	}
	c.LastErrorAt = time.Now()
	// 동일 포인터를 업데이트하여 호출자가 즉시 반영 확인 가능하게 함
	cred.LastErrorAt = c.LastErrorAt
	return nil
}

// Size는 풀의 총 크레덴셜 수와 현재 선택 가능한 크레덴셜 수를 반환한다.
func (p *CredentialPool) Size() (total, available int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total = len(p.creds)
	available = p.availableCount()
	return
}

// Reload는 Source.Load()를 다시 호출하여 풀을 재로드한다.
// 기존 리스 중인 크레덴셜의 상태는 보존된다.
func (p *CredentialPool) Reload(ctx context.Context) error {
	return p.load(ctx)
}
