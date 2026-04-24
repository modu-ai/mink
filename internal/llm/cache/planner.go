// Package cache는 LLM prompt caching 계획 타입을 정의한다.
// SPEC-GOOSE-ADAPTER-001 M0 T-005
// PROMPT-CACHE-001 구현 시 BreakpointPlanner.Plan 로직이 채워진다.
package cache

import "github.com/modu-ai/goose/internal/message"

// CacheStrategy는 캐시 적용 전략이다.
type CacheStrategy int

const (
	// StrategyNone은 캐시를 적용하지 않는다.
	StrategyNone CacheStrategy = iota
	// StrategySystemOnly는 system 메시지에만 캐시를 적용한다.
	StrategySystemOnly
	// StrategySystemAnd3는 system 메시지와 마지막 3개 메시지에 캐시를 적용한다.
	StrategySystemAnd3
)

// TTL은 캐시 TTL 타입이다.
type TTL string

const (
	// TTLEphemeral은 임시 TTL이다 (Anthropic "ephemeral").
	TTLEphemeral TTL = "ephemeral"
	// TTL1Hour는 1시간 TTL이다.
	TTL1Hour TTL = "1h"
)

// CacheMarker는 특정 메시지에 캐시를 적용하는 마커이다.
type CacheMarker struct {
	// MessageIndex는 캐시를 적용할 메시지의 인덱스이다.
	MessageIndex int
	// TTL은 캐시 유지 시간이다.
	TTL TTL
}

// CachePlan은 캐시 적용 계획이다.
type CachePlan struct {
	// Markers는 캐시를 적용할 메시지 목록이다.
	// PROMPT-CACHE-001이 구현될 때까지 항상 빈 슬라이스이다.
	Markers []CacheMarker
}

// BreakpointPlanner는 캐시 브레이크포인트 계획자이다.
// PROMPT-CACHE-001에서 실 구현이 제공될 예정이다.
type BreakpointPlanner struct{}

// Plan은 메시지 목록에 대한 캐시 계획을 반환한다.
// 현재 stub: 항상 빈 Markers를 반환한다 (REQ-ADAPTER-015 준수).
func (p *BreakpointPlanner) Plan(_ []message.Message, _ CacheStrategy, _ TTL) CachePlan {
	return CachePlan{Markers: []CacheMarker{}}
}
