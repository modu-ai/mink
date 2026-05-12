package v2_test

import (
	"testing"

	v2 "github.com/modu-ai/mink/internal/llm/router/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRLView 는 provider → 4-bucket usage map 으로 BucketUsage 를
// 구현하는 테스트용 fake 이다. 실제 RATELIMIT-001 reader 와의 어댑터는
// P3 RouterV2 가 wiring 한다 (plan.md §1 P2-T5).
type fakeRLView struct {
	usage map[string][4]float64 // [rpm, tpm, rph, tph]
}

func (f *fakeRLView) BucketUsage(provider string) (rpm, tpm, rph, tph float64) {
	u, ok := f.usage[provider]
	if !ok {
		return 0, 0, 0, 0
	}
	return u[0], u[1], u[2], u[3]
}

// --------------------------------------------------------------------------
// TestFilterByRateLimit_RPMBucketAt80Percent_ExcludesProvider — P2-T3 / AC-RV2-005
// --------------------------------------------------------------------------

// 4 bucket 중 RPM 단독으로 임계 (0.80) 도달 시 해당 provider 가 후보
// pool 에서 제외된다 (REQ-RV2-009). AC-RV2-005 의 "anthropic RPM 0.85
// 시 openai 로 전환" 시나리오 회귀 보호.
func TestFilterByRateLimit_RPMBucketAt80Percent_ExcludesProvider(t *testing.T) {
	t.Parallel()

	view := &fakeRLView{usage: map[string][4]float64{
		"anthropic": {0.85, 0.10, 0.50, 0.30}, // RPM 임계 초과
		"openai":    {0.20, 0.10, 0.30, 0.20}, // 모두 여유
	}}

	candidates := []v2.ProviderRef{
		{Provider: "anthropic", Model: "claude-sonnet-4.6"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	filtered := v2.FilterByRateLimit(candidates, view, v2.DefaultRateLimitThreshold)

	require.Len(t, filtered, 1, "anthropic must be excluded by RPM threshold")
	assert.Equal(t, "openai", filtered[0].Provider)
}

// --------------------------------------------------------------------------
// TestFilterByRateLimit_AnyBucketAtThreshold_ExcludesProvider
// --------------------------------------------------------------------------

// 4 bucket 중 어느 하나라도 임계 도달 시 제외 — RPM 단독뿐 아니라 TPM,
// RPH, TPH 모두 동일 정책 (REQ-RV2-009 "어느 하나라도").
func TestFilterByRateLimit_AnyBucketAtThreshold_ExcludesProvider(t *testing.T) {
	t.Parallel()

	cases := []struct {
		bucket string
		usage  [4]float64
	}{
		{"rpm", [4]float64{0.80, 0.0, 0.0, 0.0}},
		{"tpm", [4]float64{0.0, 0.80, 0.0, 0.0}},
		{"rph", [4]float64{0.0, 0.0, 0.80, 0.0}},
		{"tph", [4]float64{0.0, 0.0, 0.0, 0.80}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.bucket+"_at_threshold_excludes", func(t *testing.T) {
			t.Parallel()
			view := &fakeRLView{usage: map[string][4]float64{"x": tc.usage}}
			out := v2.FilterByRateLimit(
				[]v2.ProviderRef{{Provider: "x", Model: "m"}},
				view, v2.DefaultRateLimitThreshold)
			assert.Len(t, out, 0,
				"%s exactly at threshold must exclude (>= comparison)", tc.bucket)
		})
	}
}

// --------------------------------------------------------------------------
// TestFilterByRateLimit_AllBucketsBelowThreshold_KeepsProvider — P2-T4
// --------------------------------------------------------------------------

// 4 bucket 모두 임계 미만 시 후보 유지. 0.79 (임계 0.80 직전) 는 통과해야
// 함 — 임계 inclusive 가 ">=" 인 정책 회귀 보호.
func TestFilterByRateLimit_AllBucketsBelowThreshold_KeepsProvider(t *testing.T) {
	t.Parallel()

	view := &fakeRLView{usage: map[string][4]float64{
		"anthropic": {0.79, 0.79, 0.79, 0.79}, // 임계 직전
		"openai":    {0.0, 0.0, 0.0, 0.0},
	}}

	candidates := []v2.ProviderRef{
		{Provider: "anthropic", Model: "claude-sonnet-4.6"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	filtered := v2.FilterByRateLimit(candidates, view, v2.DefaultRateLimitThreshold)

	assert.Len(t, filtered, 2, "0.79 < 0.80 — both providers must remain")
	assert.Equal(t, "anthropic", filtered[0].Provider, "input order preserved")
	assert.Equal(t, "openai", filtered[1].Provider)
}

// --------------------------------------------------------------------------
// TestFilterByRateLimit_OverrideThresholdTo50Percent_AppliesCorrectly — P2-T3 보강
// --------------------------------------------------------------------------

// routing-policy.yaml 의 rate_limit_threshold 를 0.50 으로 override 하면
// 0.50 이상 bucket 을 가진 provider 가 모두 제외된다 (REQ-RV2-009 의
// "Threshold 는 ... override 가능").
func TestFilterByRateLimit_OverrideThresholdTo50Percent_AppliesCorrectly(t *testing.T) {
	t.Parallel()

	view := &fakeRLView{usage: map[string][4]float64{
		"anthropic": {0.55, 0.10, 0.10, 0.10}, // RPM 0.55 ≥ 0.50 → 제외
		"openai":    {0.45, 0.10, 0.10, 0.10}, // 모두 < 0.50 → 통과
		"google":    {0.10, 0.10, 0.10, 0.49}, // TPH 0.49 < 0.50 → 통과
	}}

	candidates := []v2.ProviderRef{
		{Provider: "anthropic", Model: "claude-sonnet-4.6"},
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "google", Model: "gemini-2.0-flash"},
	}
	filtered := v2.FilterByRateLimit(candidates, view, 0.50)

	require.Len(t, filtered, 2)
	assert.Equal(t, "openai", filtered[0].Provider)
	assert.Equal(t, "google", filtered[1].Provider)
}

// --------------------------------------------------------------------------
// TestFilterByRateLimit_NilView_ReturnsCandidatesUnchanged
// --------------------------------------------------------------------------

// view==nil 은 RATELIMIT-001 미통합 환경 (테스트, 초기 부트스트랩) 의
// graceful 처리 — 후보를 그대로 통과시킨다. defensive coding 으로 panic
// 회피.
func TestFilterByRateLimit_NilView_ReturnsCandidatesUnchanged(t *testing.T) {
	t.Parallel()

	candidates := []v2.ProviderRef{
		{Provider: "anthropic", Model: "claude-sonnet-4.6"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	filtered := v2.FilterByRateLimit(candidates, nil, v2.DefaultRateLimitThreshold)

	require.Len(t, filtered, 2)
	assert.Equal(t, candidates[0], filtered[0])
	assert.Equal(t, candidates[1], filtered[1])
}

// --------------------------------------------------------------------------
// TestFilterByRateLimit_UnknownProvider_KeepsAsZeroUsage
// --------------------------------------------------------------------------

// view 에 등록되지 않은 provider 는 모든 bucket 0.0 (zero value) 으로
// 취급되어 후보에 유지된다 — RATELIMIT-001 tracker 미등록 = "usage 정보
// 없음" = 보수적으로 여유로 가정.
func TestFilterByRateLimit_UnknownProvider_KeepsAsZeroUsage(t *testing.T) {
	t.Parallel()

	view := &fakeRLView{usage: map[string][4]float64{
		// "anthropic" 는 등록 — 0.85 (제외 대상)
		"anthropic": {0.85, 0, 0, 0},
		// "openai" 는 미등록 — view 가 zero 반환 → 통과
	}}

	candidates := []v2.ProviderRef{
		{Provider: "anthropic", Model: "claude-sonnet-4.6"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	filtered := v2.FilterByRateLimit(candidates, view, v2.DefaultRateLimitThreshold)

	require.Len(t, filtered, 1)
	assert.Equal(t, "openai", filtered[0].Provider,
		"unregistered provider treated as zero-usage = passes filter")
}

// --------------------------------------------------------------------------
// TestFilterByRateLimit_EmptyCandidates_ReturnsEmptyNonNil
// --------------------------------------------------------------------------

// nil 또는 빈 입력은 빈 non-nil slice — caller 가 nil 검사 부담 없이
// len() 으로 일관 처리 가능.
func TestFilterByRateLimit_EmptyCandidates_ReturnsEmptyNonNil(t *testing.T) {
	t.Parallel()

	view := &fakeRLView{}

	t.Run("nil_input", func(t *testing.T) {
		t.Parallel()
		out := v2.FilterByRateLimit(nil, view, v2.DefaultRateLimitThreshold)
		assert.NotNil(t, out)
		assert.Empty(t, out)
	})

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()
		out := v2.FilterByRateLimit([]v2.ProviderRef{}, view, v2.DefaultRateLimitThreshold)
		assert.NotNil(t, out)
		assert.Empty(t, out)
	})
}

// --------------------------------------------------------------------------
// TestFilterByRateLimit_DoesNotMutateInput
// --------------------------------------------------------------------------

// 입력 candidates slice 가 수정되지 않음을 보장. caller 가 동일 slice
// 를 여러 호출에 reuse 해도 안전.
func TestFilterByRateLimit_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	view := &fakeRLView{usage: map[string][4]float64{
		"anthropic": {0.85, 0, 0, 0}, // 제외 대상
	}}

	candidates := []v2.ProviderRef{
		{Provider: "anthropic", Model: "claude-sonnet-4.6"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	original := append([]v2.ProviderRef(nil), candidates...)

	_ = v2.FilterByRateLimit(candidates, view, v2.DefaultRateLimitThreshold)

	assert.Equal(t, original, candidates,
		"FilterByRateLimit must not mutate input slice")
}
