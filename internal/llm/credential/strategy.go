package credential

import (
	"errors"
	"math/rand/v2"
	"sync/atomic"
)

// SelectionStrategy는 사용 가능한 크레덴셜 중 하나를 선택하는 전략 인터페이스이다.
type SelectionStrategy interface {
	// Select는 사용 가능한 크레덴셜 목록에서 하나를 선택하여 반환한다.
	// available이 비어 있으면 에러를 반환한다.
	Select(available []*PooledCredential) (*PooledCredential, error)
	// Name은 전략 이름을 반환한다.
	Name() string
}

// errNoAvailable은 선택 가능한 크레덴셜이 없을 때 반환되는 내부 에러이다.
var errNoAvailable = errors.New("credential: 선택 가능한 크레덴셜이 없음")

// ── RoundRobin 전략 ──────────────────────────────────────────────────────────

// RoundRobinStrategy는 카운터 기반 순환 선택 전략이다.
// 스레드 안전하다.
type RoundRobinStrategy struct {
	counter atomic.Uint64
}

// NewRoundRobinStrategy는 새 RoundRobinStrategy를 생성한다.
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

// Select는 내부 카운터를 기반으로 다음 크레덴셜을 순환 선택한다.
func (s *RoundRobinStrategy) Select(available []*PooledCredential) (*PooledCredential, error) {
	if len(available) == 0 {
		return nil, errNoAvailable
	}
	idx := s.counter.Add(1) - 1
	return available[idx%uint64(len(available))], nil
}

// Name은 "round-robin"을 반환한다.
func (s *RoundRobinStrategy) Name() string { return "round-robin" }

// ── LRU 전략 ────────────────────────────────────────────────────────────────

// LRUStrategy는 UsageCount가 가장 낮은 크레덴셜을 선택하는 전략이다.
// 사용 빈도가 낮은 크레덴셜을 우선 선택하여 균등한 부하 분산을 달성한다.
type LRUStrategy struct{}

// NewLRUStrategy는 새 LRUStrategy를 생성한다.
func NewLRUStrategy() *LRUStrategy { return &LRUStrategy{} }

// Select는 UsageCount가 가장 낮은 크레덴셜을 반환한다.
func (s *LRUStrategy) Select(available []*PooledCredential) (*PooledCredential, error) {
	if len(available) == 0 {
		return nil, errNoAvailable
	}
	best := available[0]
	for _, c := range available[1:] {
		if c.UsageCount < best.UsageCount {
			best = c
		}
	}
	return best, nil
}

// Name은 "lru"를 반환한다.
func (s *LRUStrategy) Name() string { return "lru" }

// ── Weighted 전략 ────────────────────────────────────────────────────────────

// WeightedStrategy는 Weight 필드에 비례한 확률로 크레덴셜을 무작위 선택하는 전략이다.
// Weight=0인 크레덴셜은 선택에서 제외된다.
type WeightedStrategy struct{}

// NewWeightedStrategy는 새 WeightedStrategy를 생성한다.
func NewWeightedStrategy() *WeightedStrategy { return &WeightedStrategy{} }

// Select는 Weight에 비례하는 확률로 크레덴셜을 선택한다.
func (s *WeightedStrategy) Select(available []*PooledCredential) (*PooledCredential, error) {
	if len(available) == 0 {
		return nil, errNoAvailable
	}

	totalWeight := 0
	for _, c := range available {
		if c.Weight > 0 {
			totalWeight += c.Weight
		}
	}
	if totalWeight == 0 {
		// 모든 Weight가 0이면 RoundRobin으로 폴백
		return available[rand.IntN(len(available))], nil
	}

	target := rand.IntN(totalWeight)
	cumulative := 0
	for _, c := range available {
		if c.Weight <= 0 {
			continue
		}
		cumulative += c.Weight
		if target < cumulative {
			return c, nil
		}
	}
	// 부동소수점 오차 방어: 마지막 양수 Weight 크레덴셜 반환
	for i := len(available) - 1; i >= 0; i-- {
		if available[i].Weight > 0 {
			return available[i], nil
		}
	}
	return nil, errNoAvailable
}

// Name은 "weighted"를 반환한다.
func (s *WeightedStrategy) Name() string { return "weighted" }

// ── Priority 전략 ────────────────────────────────────────────────────────────

// PriorityStrategy는 Priority 값이 가장 높은 크레덴셜을 선택하는 전략이다.
// 동일 Priority일 경우 라운드로빈으로 타이브레이크한다.
type PriorityStrategy struct {
	counter atomic.Uint64
}

// NewPriorityStrategy는 새 PriorityStrategy를 생성한다.
func NewPriorityStrategy() *PriorityStrategy { return &PriorityStrategy{} }

// Select는 가장 높은 Priority를 가진 크레덴셜을 반환한다.
// 동일 Priority가 여러 개면 라운드로빈으로 선택한다.
func (s *PriorityStrategy) Select(available []*PooledCredential) (*PooledCredential, error) {
	if len(available) == 0 {
		return nil, errNoAvailable
	}

	// 최고 우선순위 값 탐색
	maxPriority := available[0].Priority
	for _, c := range available[1:] {
		if c.Priority > maxPriority {
			maxPriority = c.Priority
		}
	}

	// 최고 우선순위 크레덴셜 수집
	var top []*PooledCredential
	for _, c := range available {
		if c.Priority == maxPriority {
			top = append(top, c)
		}
	}

	// 단일 최고 우선순위이면 바로 반환
	if len(top) == 1 {
		return top[0], nil
	}

	// 동일 우선순위 타이브레이크: 라운드로빈
	idx := s.counter.Add(1) - 1
	return top[idx%uint64(len(top))], nil
}

// Name은 "priority"를 반환한다.
func (s *PriorityStrategy) Name() string { return "priority" }
