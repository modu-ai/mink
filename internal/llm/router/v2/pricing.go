// Package v2 — pricing.go: 정적 per-million $ 표 + PreferCheap 정렬.
//
// SPEC §6.2 의 16 entry 가 source-of-truth 이며, provider 공식 가격 변동
// 시 본 SPEC §13 manual update 정책에 따라 amendment 가 필요하다 (6 개월
// 주기). 자동 갱신 / scrape 는 OUT (spec.md §14).
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-007
package v2

import (
	"math"
	"sort"
)

// Price 는 단일 provider:model 의 per-million 토큰 비용 ($USD).
//
// Input 은 prompt 토큰 비용, Output 은 generation 토큰 비용.
// PreferCheap 정렬은 Average() 오름차순으로 결정한다.
type Price struct {
	// Input 는 per-million prompt 토큰 비용 ($USD, e.g. 2.50 for $2.50/M).
	Input float64
	// Output 는 per-million generation 토큰 비용 ($USD).
	Output float64
}

// Average 는 (Input + Output) / 2. PreferCheap 정렬 키.
func (p Price) Average() float64 {
	return (p.Input + p.Output) / 2.0
}

// defaultPrices 는 spec.md §6.2 의 16 entry 정적 표.
//
// key 형식: "provider:model" — 정확 일치 우선, ":*" wildcard 가 fallback.
// 가격 변동 시 본 변수 + spec.md §6.2 양쪽 모두 amendment 필요.
//
// 유효 시점: 2026-05-05. 다음 review 시점: 2026-11.
var defaultPrices = map[string]Price{
	"ollama:*":                     {Input: 0.00, Output: 0.00},
	"groq:llama-3.3-70b":           {Input: 0.00, Output: 0.00},
	"mistral:nemo":                 {Input: 0.02, Output: 0.02},
	"together:llama-3.3-70b-turbo": {Input: 0.88, Output: 0.88},
	"fireworks:llama-3.1-405b":     {Input: 3.00, Output: 3.00},
	"deepseek:deepseek-chat":       {Input: 0.27, Output: 1.10},
	"zai_glm:glm-4.6":              {Input: 0.50, Output: 1.50},
	"qwen:qwen3-max":               {Input: 0.80, Output: 2.40},
	"kimi:k2.6":                    {Input: 0.60, Output: 1.80},
	"google:gemini-2.0-flash":      {Input: 0.075, Output: 0.30},
	"openai:gpt-4o":                {Input: 2.50, Output: 10.00},
	"openai:o1-preview":            {Input: 15.00, Output: 60.00},
	"anthropic:claude-sonnet-4.6":  {Input: 3.00, Output: 15.00},
	"anthropic:claude-opus-4-7":    {Input: 15.00, Output: 75.00},
	"xai:grok-3":                   {Input: 5.00, Output: 15.00},
	"cerebras:llama-3.3-70b":       {Input: 0.85, Output: 1.20},
}

// LookupPrice 는 "provider:model" 키에 해당하는 Price 를 반환한다.
//
// 매칭 우선순위:
//  1. 정확 매칭 ("provider:model")
//  2. wildcard ("provider:*") — 같은 provider 의 모든 model 에 동일 가격
//
// 매칭 없으면 (Price{}, false). caller 는 SortByCost 에서 unknown 을
// "비싼" 쪽으로 보내기 위해 사용한다.
func LookupPrice(key string) (Price, bool) {
	if p, ok := defaultPrices[key]; ok {
		return p, true
	}
	// wildcard fallback — "provider:" prefix 매칭.
	for i, ch := range key {
		if ch == ':' {
			wildcard := key[:i+1] + "*"
			if p, ok := defaultPrices[wildcard]; ok {
				return p, true
			}
			break
		}
	}
	return Price{}, false
}

// SortByCost 는 candidates 를 평균 비용 오름차순으로 정렬한 새 slice 를
// 반환한다 (REQ-RV2-007).
//
// 정책:
//   - 평균 비용 동률 시 입력 순서 보존 (sort.SliceStable).
//   - LookupPrice 가 매칭 못한 후보는 +Inf 로 취급 → 마지막에 배치.
//   - 입력 slice 는 변경되지 않는다 (caller 입력 보호).
//   - nil 또는 빈 입력 → 빈 non-nil slice.
func SortByCost(candidates []ProviderRef) []ProviderRef {
	out := make([]ProviderRef, len(candidates))
	copy(out, candidates)
	sort.SliceStable(out, func(i, j int) bool {
		return averageCost(out[i]) < averageCost(out[j])
	})
	return out
}

// averageCost 는 LookupPrice 가 매칭한 Price.Average() 또는 +Inf 를 반환한다.
// SortByCost 의 less 함수 내부 사용 — 직접 호출은 권장하지 않음.
func averageCost(ref ProviderRef) float64 {
	key := ref.Provider + ":" + ref.Model
	if p, ok := LookupPrice(key); ok {
		return p.Average()
	}
	return math.Inf(1)
}
