// Package cache는 LLM prompt caching 계획 타입을 정의한다.
// SPEC-GOOSE-ADAPTER-001 M0 T-005 + SPEC-GOOSE-PROMPT-CACHE-001
package cache

import (
	"errors"
	"sort"

	"github.com/modu-ai/goose/internal/message"
)

// CacheStrategy는 캐시 적용 전략이다.
type CacheStrategy int

const (
	// StrategyNone은 캐시를 적용하지 않는다.
	StrategyNone CacheStrategy = iota
	// StrategySystemOnly는 system 메시지에만 캐시를 적용한다.
	StrategySystemOnly
	// StrategySystemAnd3는 system 메시지와 마지막 3개 non-system 메시지에 캐시를 적용한다.
	StrategySystemAnd3
)

// TTL은 캐시 TTL 타입이다.
type TTL string

const (
	// TTLEphemeral은 임시 TTL이다 (Anthropic "ephemeral" 키워드용).
	TTLEphemeral TTL = "ephemeral"
	// TTLDefault는 기본 TTL이다 (Anthropic 기본 5분).
	// @MX:NOTE: [AUTO] SPEC-GOOSE-PROMPT-CACHE-001 AC에서 "5m"을 TTLDefault로 정의
	TTLDefault TTL = "5m"
	// TTL1Hour는 1시간 TTL이다.
	TTL1Hour TTL = "1h"
)

// CacheMarker는 특정 메시지·content block 위치에 캐시를 적용하는 마커이다.
// @MX:ANCHOR: [AUTO] ADAPTER-001이 이 구조체를 소비해 content block에 cache_control 주입
// @MX:REASON: SPEC-GOOSE-PROMPT-CACHE-001 REQ-PC-003 — MessageIndex/ContentBlockIndex 정확성 보장
type CacheMarker struct {
	// MessageIndex는 캐시를 적용할 메시지의 인덱스이다.
	MessageIndex int
	// ContentBlockIndex는 마커를 삽입할 content block 인덱스이다 (마지막 블록).
	ContentBlockIndex int
	// TTL은 캐시 유지 시간이다.
	TTL TTL
}

// CachePlan은 캐시 적용 계획이다.
type CachePlan struct {
	// Strategy는 계획 생성에 사용된 전략이다.
	Strategy CacheStrategy
	// Markers는 캐시를 적용할 위치 목록이다 (최대 4개, Anthropic 제한).
	Markers []CacheMarker
}

// ErrTooManyBreakpoints는 Anthropic 4-breakpoint 제한 초과 시 반환된다.
var ErrTooManyBreakpoints = errors.New("cache: requested >4 breakpoints (Anthropic limit)")

// ErrInvalidStrategy는 알 수 없는 전략 값 전달 시 반환된다.
var ErrInvalidStrategy = errors.New("cache: invalid strategy")

// BreakpointPlanner는 캐시 브레이크포인트 계획자이다.
// stateless: 동시 Plan() 호출이 안전하다 (REQ-PC-001).
// @MX:ANCHOR: [AUTO] ADAPTER-001의 AnthropicAdapter.BuildRequest 에서 호출
// @MX:REASON: SPEC-GOOSE-PROMPT-CACHE-001 REQ-PC-001 — stateless, concurrent-safe
type BreakpointPlanner struct{}

// NewBreakpointPlanner는 새로운 BreakpointPlanner를 반환한다.
func NewBreakpointPlanner() *BreakpointPlanner {
	return &BreakpointPlanner{}
}

// Plan은 메시지 목록에 대한 캐시 계획을 반환한다.
// strategy에 따라 최대 4개의 CacheMarker를 포함하는 *CachePlan을 반환한다.
// REQ-PC-010: nil/empty messages 시 빈 plan, 에러 없음.
// REQ-PC-002: 4개 초과 시 ErrTooManyBreakpoints 반환.
func (p *BreakpointPlanner) Plan(msgs []message.Message, strategy CacheStrategy, ttl TTL) (*CachePlan, error) {
	// REQ-PC-010: nil 또는 empty messages → no-op plan
	if len(msgs) == 0 {
		return &CachePlan{Strategy: strategy, Markers: []CacheMarker{}}, nil
	}

	switch strategy {
	case StrategyNone:
		// REQ-PC-006: None 전략은 항상 빈 plan 반환
		return &CachePlan{Strategy: strategy, Markers: []CacheMarker{}}, nil

	case StrategySystemOnly:
		return planSystemOnly(msgs, strategy, ttl)

	case StrategySystemAnd3:
		return planSystemAnd3(msgs, strategy, ttl)

	default:
		return nil, ErrInvalidStrategy
	}
}

// planSystemOnly는 StrategySystemOnly 전략을 실행한다.
// REQ-PC-005: system 메시지가 있으면 1개 marker, 없으면 빈 plan 반환.
func planSystemOnly(msgs []message.Message, strategy CacheStrategy, ttl TTL) (*CachePlan, error) {
	if msgs[0].Role == "system" {
		cbIdx := lastContentBlockIndex(msgs[0])
		return &CachePlan{
			Strategy: strategy,
			Markers: []CacheMarker{
				{MessageIndex: 0, ContentBlockIndex: cbIdx, TTL: ttl},
			},
		}, nil
	}
	return &CachePlan{Strategy: strategy, Markers: []CacheMarker{}}, nil
}

// planSystemAnd3는 system_and_3 알고리즘을 실행한다.
// Hermes Agent prompt_cache.py의 plan_cache_markers를 Go로 포팅.
// spec §6.3 알고리즘:
//  1. messages[0]이 system role이면 첫 번째 marker 추가
//  2. messages[1..] 중 마지막 3개 non-system을 역순으로 수집 후 오름차순 정렬
//  3. 중복 (MessageIndex, ContentBlockIndex) 방지
//  4. 4개 초과 시 ErrTooManyBreakpoints
func planSystemAnd3(msgs []message.Message, strategy CacheStrategy, ttl TTL) (*CachePlan, error) {
	markers := make([]CacheMarker, 0, 4)

	// 1. system marker: messages[0]이 system role이면 추가
	if msgs[0].Role == "system" {
		cbIdx := lastContentBlockIndex(msgs[0])
		markers = append(markers, CacheMarker{
			MessageIndex:      0,
			ContentBlockIndex: cbIdx,
			TTL:               ttl,
		})
	}

	// 2. 마지막 3개 non-system 메시지 인덱스 수집 (역순, 최대 3개)
	nonSystemIndices := collectLastNNonSystem(msgs, 3)

	// 중복 방지를 위한 set (system marker가 이미 있을 수 있는 경우 대비)
	seen := markerSet(markers)

	// non-system 메시지에 marker 추가 (오름차순으로 정렬된 인덱스 순서)
	for _, idx := range nonSystemIndices {
		cbIdx := lastContentBlockIndex(msgs[idx])
		key := [2]int{idx, cbIdx}
		if !seen[key] {
			markers = append(markers, CacheMarker{
				MessageIndex:      idx,
				ContentBlockIndex: cbIdx,
				TTL:               ttl,
			})
			seen[key] = true
		}
	}

	// REQ-PC-002: 4개 초과 guard (system_and_3는 이론상 최대 4개이나 안전장치)
	if len(markers) > 4 {
		return nil, ErrTooManyBreakpoints
	}

	return &CachePlan{Strategy: strategy, Markers: markers}, nil
}

// collectLastNNonSystem은 msgs에서 마지막 n개의 non-system 메시지 인덱스를
// 오름차순으로 반환한다.
func collectLastNNonSystem(msgs []message.Message, n int) []int {
	indices := make([]int, 0, n)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "system" {
			indices = append(indices, i)
			if len(indices) == n {
				break
			}
		}
	}
	sort.Ints(indices)
	return indices
}

// markerSet은 기존 markers의 (MessageIndex, ContentBlockIndex) 집합을 반환한다.
// REQ-PC-009: 중복 marker 방지에 사용된다.
func markerSet(markers []CacheMarker) map[[2]int]bool {
	set := make(map[[2]int]bool, len(markers))
	for _, m := range markers {
		set[[2]int{m.MessageIndex, m.ContentBlockIndex}] = true
	}
	return set
}

// lastContentBlockIndex는 메시지의 마지막 content block 인덱스를 반환한다.
// REQ-PC-012: 다중 content block 메시지에서 마지막 블록 인덱스를 반환한다.
func lastContentBlockIndex(msg message.Message) int {
	if len(msg.Content) == 0 {
		return 0
	}
	return len(msg.Content) - 1
}
