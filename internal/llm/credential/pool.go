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

// defaultRefreshMargin은 OAuth 만료 전 refresh를 트리거하는 기본 여유 시간이다.
// REQ-CREDPOOL-018: 기본값 5분
const defaultRefreshMargin = 5 * time.Minute

// permanentExhaustSentinel은 영구 고갈 상태의 exhaustedUntil 센티넬 값이다.
// REQ-CREDPOOL-007: 402 영구 고갈
var permanentExhaustSentinel = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

// CredentialPool은 LLM 프로바이더 크레덴셜의 풀 오케스트레이터이다.
//
// 이 풀은 크레덴셜 참조와 메타데이터만 관리한다.
// 실제 시크릿 값은 OS 키링에 있으며 이 구조체에 저장되지 않는다.
// 모든 공개 메서드는 스레드 안전하다.
type CredentialPool struct {
	mu            sync.RWMutex
	source        CredentialSource
	strategy      SelectionStrategy
	storage       Storage       // 메타데이터 영속 (nil이면 비활성)
	refresher     Refresher     // OAuth refresh 구현 주입 (nil이면 비활성)
	refreshMargin time.Duration // OAuth refresh 트리거 여유 시간
	// refreshFailCounts는 엔트리별 연속 refresh 실패 카운터이다.
	// @MX:WARN: [AUTO] 고루틴 언락 구간에서 refresher.Refresh 호출 중 뮤텍스 해제
	// @MX:REASON: Refresh는 외부 HTTP 호출이 될 수 있어 락 보유 불가; Unlock → Refresh → Lock 패턴 사용
	refreshFailCounts map[string]int
	// counters는 원자적 누적 통계 카운터이다.
	counters statsCounters
	// creds는 ID → PooledCredential 맵이다. 풀의 전체 크레덴셜을 보유한다.
	creds map[string]*PooledCredential
	// order는 로드 순서를 보존하기 위한 ID 슬라이스이다.
	order []string
}

// New는 주어진 Source와 SelectionStrategy로 CredentialPool을 생성한다.
// Source.Load()를 호출하여 초기 크레덴셜을 로드한다.
// WithStorage가 설정되어 있으면 영속된 메타데이터를 병합한다.
// @MX:ANCHOR: [AUTO] 풀 생성 진입점 — pool.go 전체 공개 API의 루트 생성자
// @MX:REASON: credential 패키지의 모든 사용자가 이 함수를 통해 풀을 생성 (fan_in >= 3 예상)
// @MX:SPEC: SPEC-GOOSE-CREDPOOL-001
func New(source CredentialSource, strategy SelectionStrategy, opts ...Option) (*CredentialPool, error) {
	p := &CredentialPool{
		source:            source,
		strategy:          strategy,
		refreshMargin:     defaultRefreshMargin,
		creds:             make(map[string]*PooledCredential),
		refreshFailCounts: make(map[string]int),
	}
	for _, opt := range opts {
		opt(p)
	}

	if err := p.load(context.Background()); err != nil {
		return nil, err
	}
	return p, nil
}

// WithRefresher는 CredentialPool에 Refresher를 설정하는 Option이다.
// OI-02: Select 경로에 Refresher 배선
func WithRefresher(r Refresher) Option {
	return func(p *CredentialPool) {
		p.refresher = r
	}
}

// WithRefreshMargin은 OAuth refresh 트리거 여유 시간을 설정하는 Option이다.
// REQ-CREDPOOL-018: 기본값 5분
func WithRefreshMargin(d time.Duration) Option {
	return func(p *CredentialPool) {
		p.refreshMargin = d
	}
}

// PersistState는 현재 풀의 메타데이터를 Storage에 저장한다.
// Storage가 설정되지 않은 경우 no-op이다.
// REQ-CREDPOOL-004: mutation 후 영속화
func (p *CredentialPool) PersistState(ctx context.Context) error {
	if p.storage == nil {
		return nil
	}

	p.mu.RLock()
	creds := make([]*PooledCredential, 0, len(p.order))
	for _, id := range p.order {
		if c, ok := p.creds[id]; ok {
			creds = append(creds, c)
		}
	}
	p.mu.RUnlock()

	return p.storage.Save(ctx, creds)
}

// load는 Source로부터 크레덴셜을 로드하여 풀을 (재)초기화한다.
// 이미 리스 중인 크레덴셜의 상태는 보존된다.
// storage가 설정되어 있으면 영속된 메타데이터를 병합한다 (OI-01).
func (p *CredentialPool) load(ctx context.Context) error {
	loaded, err := p.source.Load(ctx)
	if err != nil {
		return err
	}

	// storage에서 영속 메타데이터 로드 (재기동 상태 복원용)
	var persisted map[string]*PooledCredential
	if p.storage != nil {
		persistedList, err := p.storage.Load(ctx)
		if err == nil && len(persistedList) > 0 {
			persisted = make(map[string]*PooledCredential, len(persistedList))
			for _, c := range persistedList {
				persisted[c.ID] = c
			}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 새 맵과 순서 슬라이스를 구성한다.
	newCreds := make(map[string]*PooledCredential, len(loaded))
	newOrder := make([]string, 0, len(loaded))

	for _, c := range loaded {
		// 기존 인메모리 크레덴셜이 있으면 리스 상태와 에러 상태를 보존한다.
		if existing, ok := p.creds[c.ID]; ok {
			c.leased = existing.leased
			c.exhaustedUntil = existing.exhaustedUntil
			c.LastErrorAt = existing.LastErrorAt
			c.LastErrorReset = existing.LastErrorReset
			c.UsageCount = existing.UsageCount
		} else if saved, ok := persisted[c.ID]; ok {
			// 인메모리에 없으면 영속 메타데이터에서 상태를 복원한다 (재기동 복원).
			c.Status = saved.Status
			c.exhaustedUntil = saved.exhaustedUntil
			c.LastErrorAt = saved.LastErrorAt
			c.LastErrorReset = saved.LastErrorReset
			c.UsageCount = saved.UsageCount
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
//
// 선택 제외 조건:
//   - leased: 이미 리스 중
//   - CredExhausted + 쿨다운 미만료
//   - ExpiresAt != zero && ExpiresAt <= now: OAuth 토큰 만료
//     (REQ-CREDPOOL-001 (b): ExpiresAt.IsZero() == 영구 유효 API key)
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
		// OAuth 토큰 만료 필터: ExpiresAt이 설정됐고 현재 시각 이전이면 선택 불가
		// ExpiresAt.IsZero()는 API key 등 영구 유효 크레덴셜을 나타냄 (REQ-CREDPOOL-001 b)
		if !c.ExpiresAt.IsZero() && !now.Before(c.ExpiresAt) {
			continue
		}
		result = append(result, c)
	}
	return result
}

// availableCount는 쓰기 잠금 없이 선택 가능한 크레덴셜 수를 반환한다.
// 쿨다운 상태 복구를 수행하지 않으므로 Size() 읽기 락 컨텍스트에서 사용된다.
// available()과 동일한 필터 조건을 적용한다 (ExpiresAt 포함).
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
		// OAuth 토큰 만료 필터: available()과 동일 조건 (REQ-CREDPOOL-001 b)
		if !c.ExpiresAt.IsZero() && !now.Before(c.ExpiresAt) {
			continue
		}
		count++
	}
	return count
}

// Select는 전략에 따라 사용 가능한 크레덴셜 중 하나를 선택하고 리스를 획득한다.
// Refresher가 설정된 경우 OAuth 만료 임박 엔트리에 대해 refresh를 트리거한다.
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

	// Refresher가 설정된 경우: 만료 임박 OAuth 엔트리 처리
	// REQ-CREDPOOL-005 (c): expires_at - refreshMargin < now 조건
	if p.refresher != nil {
		p.triggerRefreshLocked(ctx)
	}

	avail := p.available()
	if len(avail) == 0 {
		p.mu.Unlock()
		return nil, ErrExhausted
	}

	cred, err := p.strategy.Select(avail)
	if err != nil {
		p.mu.Unlock()
		return nil, ErrExhausted
	}

	// 리스 획득
	cred.leased = true
	cred.UsageCount++
	p.mu.Unlock()

	// 선택 카운터 증가 (뮤텍스 밖에서 atomic 업데이트)
	p.counters.selects.Add(1)
	return cred, nil
}

// triggerRefreshLocked는 뮤텍스 보유 상태에서 refresh가 필요한 엔트리를 처리한다.
// refresh 호출 시 뮤텍스를 해제하고 재획득한다 (외부 HTTP 호출 방지).
// @MX:WARN: [AUTO] Unlock→Refresh→Lock 패턴 — 뮤텍스 해제 구간
// @MX:REASON: Refresh는 네트워크 호출이 될 수 있어 락 보유 불가; 엔트리 포인터를 미리 캡처하여 안전 보장
func (p *CredentialPool) triggerRefreshLocked(ctx context.Context) {
	now := time.Now()
	for _, id := range p.order {
		c, ok := p.creds[id]
		if !ok || c.ExpiresAt.IsZero() || c.Status == CredExhausted {
			continue
		}
		// expires_at - refreshMargin < now: refresh trigger
		if !c.ExpiresAt.Add(-p.refreshMargin).Before(now) {
			continue
		}

		// 뮤텍스 해제 후 refresh 호출 (REQ-CREDPOOL-015: 다른 엔트리 블로킹 방지)
		p.mu.Unlock()
		refreshErr := p.refresher.Refresh(ctx, c)
		p.mu.Lock()

		// refresh 결과 처리
		if c, ok = p.creds[id]; !ok {
			// refresh 중 엔트리 삭제됨 — 무시
			continue
		}

		if refreshErr != nil {
			p.refreshFailCounts[id]++
			// REQ-CREDPOOL-013: 3회 연속 실패 → 영구 고갈
			if p.refreshFailCounts[id] >= 3 {
				c.Status = CredExhausted
				c.exhaustedUntil = permanentExhaustSentinel
			} else {
				// 일시적 실패: 5분 쿨다운
				c.Status = CredExhausted
				c.exhaustedUntil = time.Now().Add(5 * time.Minute)
			}
			c.leased = false
		} else {
			// 성공 시 실패 카운터 초기화, refresh 카운터 증가
			p.refreshFailCounts[id] = 0
			p.counters.refreshes.Add(1)
		}
	}
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

// Reset은 id에 해당하는 크레덴셜의 Exhausted 상태를 수동으로 해제한다.
// 영구 고갈 포함 모든 쿨다운을 초기화하고 CredOK 상태로 복원한다.
// 운영자가 credential을 재로그인/복구한 후 사용한다.
// OI-08: 운영자 수동 영구 고갈 해제 API
// REQ-CREDPOOL-013 참조: 영구 고갈 해제는 운영자 개입으로만 가능
func (p *CredentialPool) Reset(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	c, ok := p.creds[id]
	if !ok {
		return ErrNotFound
	}

	c.Status = CredOK
	c.exhaustedUntil = time.Time{} // zero: 쿨다운 없음
	c.leased = false               // 리스 상태도 해제
	// 연속 실패 카운터 초기화
	p.refreshFailCounts[id] = 0
	return nil
}

// defaultCooldown은 retryAfter 미지정 시 기본 쿨다운 기간이다.
// REQ-CREDPOOL-006: defaultCooldown = 1h
const defaultCooldown = 1 * time.Hour

// MarkExhaustedAndRotate는 id에 해당하는 크레덴셜을 exhausted로 표시하고,
// 다음 사용 가능한 크레덴셜을 선택하여 반환한다.
//
// 인자:
//   - id: exhausted 처리할 크레덴셜 ID
//   - statusCode: HTTP 응답 코드 (429 → 쿨다운, 402 → 영구 고갈)
//   - retryAfter: exhausted 쿨다운 기간 (429 전용; 0이면 defaultCooldown 사용)
//
// 반환:
//   - 다음 크레덴셜: 선택 성공 시 non-nil
//   - ErrExhausted: 사용 가능한 크레덴셜이 없을 때
//   - ErrNotFound: id에 해당하는 크레덴셜이 없을 때
//
// SPEC-GOOSE-ADAPTER-001 T-007 (CREDPOOL-001 §3.1 rule 6 선행 구현)
// @MX:ANCHOR: [AUTO] MarkExhaustedAndRotate — 429/402 회전 핵심 메서드
// @MX:REASON: Anthropic adapter의 429 rotation 경로에서 호출됨 (AC-ADAPTER-008)
func (p *CredentialPool) MarkExhaustedAndRotate(ctx context.Context, id string, statusCode int, retryAfter time.Duration) (*PooledCredential, error) {
	p.mu.Lock()

	c, ok := p.creds[id]
	if !ok {
		p.mu.Unlock()
		return nil, ErrNotFound
	}

	// exhausted 표시 및 리스 해제
	c.Status = CredExhausted
	c.leased = false

	// REQ-CREDPOOL-007: 402 → 영구 고갈 (far-future sentinel)
	// REQ-CREDPOOL-006: 429 → max(retryAfter, defaultCooldown)
	if statusCode == 402 {
		c.exhaustedUntil = permanentExhaustSentinel
	} else {
		cooldown := retryAfter
		if cooldown < defaultCooldown {
			cooldown = defaultCooldown
		}
		c.exhaustedUntil = time.Now().Add(cooldown)
	}

	// id를 제외한 사용 가능한 크레덴셜 탐색
	avail := p.available()
	// id에 해당하는 크레덴셜은 이미 exhausted이므로 available()에서 제외됨

	if len(avail) == 0 {
		p.mu.Unlock()
		return nil, ErrExhausted
	}

	next, err := p.strategy.Select(avail)
	if err != nil {
		p.mu.Unlock()
		return nil, ErrExhausted
	}

	next.leased = true
	next.UsageCount++
	p.mu.Unlock()

	// rotation 카운터 증가 (뮤텍스 밖에서 atomic 업데이트)
	p.counters.rotations.Add(1)

	// ctx 취소 확인
	select {
	case <-ctx.Done():
		// 리스를 획득했으므로 반환
		p.mu.Lock()
		if nc, ok := p.creds[next.ID]; ok {
			nc.leased = false
		}
		p.mu.Unlock()
		return nil, ctx.Err()
	default:
	}

	return next, nil
}
