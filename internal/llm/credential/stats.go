// Package credential stats.go는 PoolStats 관측성 타입과 Stats() 메서드를 제공한다.
// OI-07: PoolStats{Total, Available, Exhausted, RefreshesTotal, RotationsTotal, SelectsTotal}
package credential

import (
	"sync/atomic"
	"time"
)

// PoolStats는 CredentialPool의 현재 관측 가능한 통계이다.
// Prometheus 라벨로 변환 예정 (후속 메트릭 SPEC).
type PoolStats struct {
	// 현재 상태 스냅샷
	Total     int // 전체 크레덴셜 수
	Available int // 선택 가능한 크레덴셜 수
	Exhausted int // 현재 고갈(쿨다운 or 영구) 중인 크레덴셜 수

	// 누적 카운터 (atomic)
	SelectsTotal   int64 // 총 Select 성공 횟수
	RefreshesTotal int64 // 총 Refresh 성공 횟수
	RotationsTotal int64 // 총 MarkExhaustedAndRotate 호출 횟수
}

// statsCounters는 원자적 누적 카운터를 보유한다.
// CredentialPool의 내부 필드로 embedding되며, 뮤텍스 없이 업데이트 가능하다.
type statsCounters struct {
	selects   atomic.Int64
	refreshes atomic.Int64
	rotations atomic.Int64
}

// Stats는 풀의 현재 관측 통계를 반환한다.
// 읽기 락으로 스냅샷을 취득한다.
func (p *CredentialPool) Stats() PoolStats {
	p.mu.RLock()

	total := len(p.creds)
	avail := p.availableCount()

	now := time.Now()
	exhausted := 0
	for _, id := range p.order {
		c, ok := p.creds[id]
		if !ok {
			continue
		}
		if c.Status == CredExhausted && now.Before(c.exhaustedUntil) {
			exhausted++
		}
	}

	p.mu.RUnlock()

	return PoolStats{
		Total:          total,
		Available:      avail,
		Exhausted:      exhausted,
		SelectsTotal:   p.counters.selects.Load(),
		RefreshesTotal: p.counters.refreshes.Load(),
		RotationsTotal: p.counters.rotations.Load(),
	}
}
