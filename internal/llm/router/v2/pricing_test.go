// Package v2 — pricing_test.go: 정적 pricing 표 + SortByCost 검증.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-007
package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPricing_StaticTable_AllSpecEntries 는 spec.md §6.2 의 16 entry 가
// pricing 표에 정의되어 있는지 검증한다 (P3-T3 RED).
func TestPricing_StaticTable_AllSpecEntries(t *testing.T) {
	expected := map[string]Price{
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
	for key, want := range expected {
		t.Run(key, func(t *testing.T) {
			got, ok := LookupPrice(key)
			require.True(t, ok, "pricing entry missing: %s", key)
			assert.InDelta(t, want.Input, got.Input, 1e-9)
			assert.InDelta(t, want.Output, got.Output, 1e-9)
		})
	}
}

// TestPricing_LookupPrice_FallbackToWildcard 는 ollama 의 모든 모델이
// "ollama:*" wildcard 에 매칭되는지 검증한다 (정확 매칭 우선, 없으면 wildcard).
func TestPricing_LookupPrice_FallbackToWildcard(t *testing.T) {
	got, ok := LookupPrice("ollama:llama3-8b")
	require.True(t, ok, "ollama:* wildcard 가 매칭되어야 함")
	assert.InDelta(t, 0.00, got.Input, 1e-9)
	assert.InDelta(t, 0.00, got.Output, 1e-9)
}

// TestPricing_LookupPrice_UnknownEntry 는 정의되지 않은 entry 에 대해
// (Price{}, false) 를 반환하는지 검증한다.
func TestPricing_LookupPrice_UnknownEntry(t *testing.T) {
	_, ok := LookupPrice("not-a-real-provider:not-a-model")
	assert.False(t, ok)
}

// TestPricing_AverageCost 는 (Input + Output) / 2 계산을 검증한다.
func TestPricing_AverageCost(t *testing.T) {
	cases := []struct {
		key  string
		want float64
	}{
		{"ollama:*", 0.00},
		{"groq:llama-3.3-70b", 0.00},
		{"mistral:nemo", 0.02},
		{"google:gemini-2.0-flash", 0.1875}, // (0.075 + 0.30) / 2
		{"openai:gpt-4o", 6.25},             // (2.50 + 10.00) / 2
		{"anthropic:claude-opus-4-7", 45.0}, // (15 + 75) / 2
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			p, ok := LookupPrice(tc.key)
			require.True(t, ok)
			assert.InDelta(t, tc.want, p.Average(), 1e-9)
		})
	}
}

// TestSortByCost_Ascending 는 SortByCost 가 평균 비용 오름차순으로 정렬하는지
// 검증한다 (REQ-RV2-007).
func TestSortByCost_Ascending(t *testing.T) {
	input := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"}, // 45.0
		{Provider: "openai", Model: "gpt-4o"},             // 6.25
		{Provider: "ollama", Model: "llama3-8b"},          // 0.0
		{Provider: "mistral", Model: "nemo"},              // 0.02
		{Provider: "groq", Model: "llama-3.3-70b"},        // 0.0
	}
	got := SortByCost(input)
	expectedOrder := []string{"ollama", "groq", "mistral", "openai", "anthropic"}
	require.Len(t, got, len(input))
	for i, want := range expectedOrder {
		assert.Equal(t, want, got[i].Provider, "index %d", i)
	}
}

// TestSortByCost_StableTieBreak 는 평균 비용 동률 시 입력 순서를 보존하는지
// 검증한다 (sort.SliceStable).
func TestSortByCost_StableTieBreak(t *testing.T) {
	// ollama:* 와 groq:llama-3.3-70b 모두 평균 0.0 — 입력 순서 보존되어야.
	input := []ProviderRef{
		{Provider: "groq", Model: "llama-3.3-70b"},
		{Provider: "ollama", Model: "llama3-8b"},
	}
	got := SortByCost(input)
	require.Len(t, got, 2)
	assert.Equal(t, "groq", got[0].Provider)
	assert.Equal(t, "ollama", got[1].Provider)
}

// TestSortByCost_UnknownProvider_GoesLast 는 pricing 표에 없는 후보가
// 정렬 시 가장 비싼 것으로 취급되어 마지막에 배치되는지 검증한다.
func TestSortByCost_UnknownProvider_GoesLast(t *testing.T) {
	input := []ProviderRef{
		{Provider: "weird-provider", Model: "weird-model"},
		{Provider: "ollama", Model: "llama3-8b"},
	}
	got := SortByCost(input)
	require.Len(t, got, 2)
	assert.Equal(t, "ollama", got[0].Provider, "known provider 가 먼저 와야 함")
	assert.Equal(t, "weird-provider", got[1].Provider, "unknown 은 마지막")
}

// TestSortByCost_DoesNotMutateInput 는 SortByCost 가 입력 slice 를 변경
// 하지 않는지 검증한다.
func TestSortByCost_DoesNotMutateInput(t *testing.T) {
	input := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "ollama", Model: "llama3-8b"},
	}
	original := make([]ProviderRef, len(input))
	copy(original, input)
	_ = SortByCost(input)
	assert.Equal(t, original, input, "입력 slice 가 변경되어선 안 됨")
}

// TestSortByCost_EmptyInput 는 빈 입력에 빈 결과를 반환하는지 검증한다.
func TestSortByCost_EmptyInput(t *testing.T) {
	got := SortByCost(nil)
	assert.Empty(t, got)
	got = SortByCost([]ProviderRef{})
	assert.Empty(t, got)
}
