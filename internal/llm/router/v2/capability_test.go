package v2_test

import (
	"sort"
	"testing"

	v2 "github.com/modu-ai/mink/internal/llm/router/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// TestCapabilityMatrix_StaticConsistency_15x4 — REQ-RV2-004 / P2-T1
// --------------------------------------------------------------------------

// spec.md §6.1 표 의 15 provider × 4 capability 60 cells 가 매트릭스에
// 정확히 동일하게 저장되어 있는지 회귀 보호한다. 신모델 추가로 매트릭스가
// 변경되면 spec.md amendment 가 필수이며, 본 테스트가 fail 하는 것이
// "실수로 코드만 바뀌었다" 의 alarm bell 역할을 한다.
//
// "model dependent" / "some" 카테고리 (openrouter, together, fireworks,
// ollama vision, qwen vision) 은 보수적 true 로 표기 — capability.go
// godoc 의 결정 근거와 일치.
func TestCapabilityMatrix_StaticConsistency_15x4(t *testing.T) {
	t.Parallel()

	type want struct {
		promptCaching, functionCalling, vision, realtime bool
	}
	expected := map[string]want{
		"anthropic":  {true, true, true, false},
		"openai":     {false, true, true, true},
		"google":     {false, true, true, false},
		"xai":        {false, true, true, false},
		"deepseek":   {false, true, false, false},
		"ollama":     {false, true, true, false},
		"zai_glm":    {false, true, false, false},
		"groq":       {false, true, false, false},
		"openrouter": {false, true, true, false},
		"together":   {false, true, true, false},
		"fireworks":  {false, true, true, false},
		"cerebras":   {false, true, false, false},
		"mistral":    {false, true, false, false},
		"qwen":       {false, true, true, false},
		"kimi":       {false, true, false, false},
	}

	matrix := v2.DefaultMatrix()
	require.Len(t, matrix, 15, "matrix must contain exactly 15 providers (spec.md §6.1)")

	for provider, exp := range expected {
		provider, exp := provider, exp
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, exp.promptCaching,
				matrix.Match(provider, []v2.Capability{v2.CapPromptCaching}),
				"%s prompt_caching mismatch", provider)
			assert.Equal(t, exp.functionCalling,
				matrix.Match(provider, []v2.Capability{v2.CapFunctionCalling}),
				"%s function_calling mismatch", provider)
			assert.Equal(t, exp.vision,
				matrix.Match(provider, []v2.Capability{v2.CapVision}),
				"%s vision mismatch", provider)
			assert.Equal(t, exp.realtime,
				matrix.Match(provider, []v2.Capability{v2.CapRealtime}),
				"%s realtime mismatch", provider)
		})
	}
}

// --------------------------------------------------------------------------
// TestCapabilityMatrix_Match_AllRequired — P2-T2
// --------------------------------------------------------------------------

// 다중 capability 가 모두 충족되어야 true. anthropic 의 (PromptCaching +
// FunctionCalling + Vision) 셋은 모두 true 이므로 통과. realtime 추가
// 시 false (anthropic 은 realtime 미지원).
func TestCapabilityMatrix_Match_AllRequired(t *testing.T) {
	t.Parallel()
	matrix := v2.DefaultMatrix()

	t.Run("anthropic_three_caps_pass", func(t *testing.T) {
		t.Parallel()
		assert.True(t, matrix.Match("anthropic",
			[]v2.Capability{v2.CapPromptCaching, v2.CapFunctionCalling, v2.CapVision}))
	})

	t.Run("anthropic_with_realtime_fails", func(t *testing.T) {
		t.Parallel()
		assert.False(t, matrix.Match("anthropic",
			[]v2.Capability{v2.CapPromptCaching, v2.CapRealtime}),
			"anthropic does not support realtime")
	})

	t.Run("empty_required_passes", func(t *testing.T) {
		t.Parallel()
		assert.True(t, matrix.Match("anthropic", nil),
			"no capability requirement = pass")
		assert.True(t, matrix.Match("openai", []v2.Capability{}),
			"empty slice = pass")
	})
}

// --------------------------------------------------------------------------
// TestCapabilityMatrix_Match_OneMissing_Rejects — REQ-RV2-010
// --------------------------------------------------------------------------

// N capability 중 하나라도 미지원이면 reject (REQ-RV2-010). vision 미지원
// provider (deepseek, groq, cerebras, mistral, kimi, zai_glm) 가
// vision 요구 시 false 를 정확히 반환해야 한다. AC-RV2-004 회귀 보호.
func TestCapabilityMatrix_Match_OneMissing_Rejects(t *testing.T) {
	t.Parallel()
	matrix := v2.DefaultMatrix()

	visionRejected := []string{"deepseek", "groq", "cerebras", "mistral", "kimi", "zai_glm"}
	for _, p := range visionRejected {
		p := p
		t.Run(p+"_vision_rejected", func(t *testing.T) {
			t.Parallel()
			assert.False(t, matrix.Match(p, []v2.Capability{v2.CapVision}),
				"%s should not support vision per spec.md §6.1", p)
		})
	}
}

// --------------------------------------------------------------------------
// TestCapabilityMatrix_Match_UnknownProvider_Rejects
// --------------------------------------------------------------------------

// 매트릭스에 없는 provider 는 보수적으로 false (어떤 capability 든) —
// caller 가 unknown provider 를 silent pass 시키지 못하게 보장.
func TestCapabilityMatrix_Match_UnknownProvider_Rejects(t *testing.T) {
	t.Parallel()
	matrix := v2.DefaultMatrix()

	assert.False(t, matrix.Match("unknown_provider",
		[]v2.Capability{v2.CapFunctionCalling}))
	assert.False(t, matrix.Match("",
		[]v2.Capability{v2.CapFunctionCalling}),
		"empty provider id rejected")
}

// --------------------------------------------------------------------------
// TestCapabilityMatrix_Providers_Returns15
// --------------------------------------------------------------------------

// Providers() 는 15 개 provider id 를 반환해야 한다. 정렬 순서는 보장
// 안 함 (map iteration) — caller 가 결정성이 필요하면 직접 sort.
func TestCapabilityMatrix_Providers_Returns15(t *testing.T) {
	t.Parallel()
	matrix := v2.DefaultMatrix()

	providers := matrix.Providers()
	assert.Len(t, providers, 15)
	sort.Strings(providers)
	assert.Equal(t, "anthropic", providers[0])
	assert.Equal(t, "zai_glm", providers[14])
}

// --------------------------------------------------------------------------
// TestCapabilityMatrix_DefaultMatrix_IsCopy
// --------------------------------------------------------------------------

// DefaultMatrix() 가 매번 신규 deep copy 를 반환해야 한다. caller 가
// 받은 매트릭스를 수정해도 다음 호출자에 leak 되지 않아야 함 (race-free).
func TestCapabilityMatrix_DefaultMatrix_IsCopy(t *testing.T) {
	t.Parallel()

	first := v2.DefaultMatrix()
	first["anthropic"][v2.CapVision] = false // 임의 변경
	delete(first, "openai")

	second := v2.DefaultMatrix()
	assert.True(t, second.Match("anthropic", []v2.Capability{v2.CapVision}),
		"second copy must not see first copy's mutation")
	assert.True(t, second.Match("openai", []v2.Capability{v2.CapFunctionCalling}),
		"second copy must still contain openai")
}
