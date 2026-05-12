// Package v2 — trace_test.go: RoutingReason v2 prefix 형식 검증.
//
// SPEC §6.4 의 7 가지 prefix 형식이 builder 함수로 정확히 생성되는지
// 검증한다. 형식 회귀가 발생하면 hook 소비자 (logging, metrics) 가
// 깨지므로 본 test 가 source-of-truth 역할을 한다.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-006 / REQ-RV2-011 / REQ-RV2-014
package v2

import (
	"testing"

	"github.com/modu-ai/mink/internal/evolve/errorclass"
	"github.com/stretchr/testify/assert"
)

func TestTrace_V1Simple(t *testing.T) {
	assert.Equal(t, "v1:simple", TraceV1Simple())
}

func TestTrace_V1Complex(t *testing.T) {
	assert.Equal(t, "v1:complex", TraceV1Complex())
}

func TestTrace_V2Policy(t *testing.T) {
	cases := []struct {
		mode     PolicyMode
		provider string
		want     string
	}{
		{PreferLocal, "ollama", "v2:policy_prefer_local_ollama"},
		{PreferCheap, "groq", "v2:policy_prefer_cheap_groq"},
		{PreferQuality, "anthropic", "v2:policy_prefer_quality_anthropic"},
		{AlwaysSpecific, "openai", "v2:policy_always_specific_openai"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, TraceV2Policy(tc.mode, tc.provider))
		})
	}
}

func TestTrace_V2Capability(t *testing.T) {
	cases := []struct {
		cap   Capability
		count int
		want  string
	}{
		{CapVision, 3, "v2:capability_vision_required_3_candidates"},
		{CapPromptCaching, 1, "v2:capability_prompt_caching_required_1_candidates"},
		{CapFunctionCalling, 15, "v2:capability_function_calling_required_15_candidates"},
		{CapRealtime, 0, "v2:capability_realtime_required_0_candidates"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, TraceV2Capability(tc.cap, tc.count))
		})
	}
}

func TestTrace_V2RateLimit(t *testing.T) {
	cases := []struct {
		provider string
		bucket   string
		usage    float64
		want     string
	}{
		{"anthropic", "rpm", 0.85, "v2:rate_limit_avoid_anthropic_rpm_0.85"},
		{"openai", "tpm", 0.92, "v2:rate_limit_avoid_openai_tpm_0.92"},
		{"groq", "rph", 0.80, "v2:rate_limit_avoid_groq_rph_0.80"},
		{"google", "tph", 1.00, "v2:rate_limit_avoid_google_tph_1.00"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, TraceV2RateLimit(tc.provider, tc.bucket, tc.usage))
		})
	}
}

func TestTrace_V2FallbackStep(t *testing.T) {
	cases := []struct {
		step   int
		reason errorclass.FailoverReason
		want   string
	}{
		{1, errorclass.RateLimit, "v2:fallback_chain_step_1_rate_limit"},
		{2, errorclass.Overloaded, "v2:fallback_chain_step_2_overloaded"},
		{3, errorclass.ContextOverflow, "v2:fallback_chain_step_3_context_overflow"},
		{5, errorclass.ThinkingSignature, "v2:fallback_chain_step_5_thinking_signature"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, TraceV2FallbackStep(tc.step, tc.reason))
		})
	}
}

func TestTrace_V2FallbackExhausted(t *testing.T) {
	assert.Equal(t, "v2:fallback_exhausted_recover_v1", TraceV2FallbackExhausted())
}

// TestTrace_V2RateLimit_RoundsToTwoDecimals 는 부동소수 노이즈 (예: 0.799999)
// 가 출력에 새어 나오지 않도록 2자리 반올림 검증한다.
func TestTrace_V2RateLimit_RoundsToTwoDecimals(t *testing.T) {
	got := TraceV2RateLimit("anthropic", "rpm", 0.799999999)
	assert.Equal(t, "v2:rate_limit_avoid_anthropic_rpm_0.80", got)
}
