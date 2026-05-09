package v2_test

import (
	"testing"

	v2 "github.com/modu-ai/goose/internal/llm/router/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// TestPolicyModeZeroValueIsPreferQuality — REQ-RV2-002 / P1-T1, P1-T2
// --------------------------------------------------------------------------

// 사용자 정책 파일이 없을 때 RoutingPolicy{} 가 v1 byte-identical 동작을
// 보장하려면 zero value 가 PreferQuality 여야 한다 (REQ-RV2-002).
// PolicyMode iota 의 첫 상수가 PreferQuality 임을 회귀 보호한다.
func TestPolicyModeZeroValueIsPreferQuality(t *testing.T) {
	t.Parallel()

	var pm v2.PolicyMode // zero value
	assert.Equal(t, v2.PreferQuality, pm,
		"PolicyMode zero value MUST be PreferQuality so RoutingPolicy{} forwards to v1 unchanged")

	var rp v2.RoutingPolicy
	assert.Equal(t, v2.PreferQuality, rp.Mode,
		"RoutingPolicy{}.Mode MUST be PreferQuality (zero value)")
}

// --------------------------------------------------------------------------
// TestPolicyMode_String_RoundTrip — sanity for trace builder
// --------------------------------------------------------------------------

// PolicyMode.String() 이 routing-policy.yaml 의 정규 표기와 일치해야 한다.
// trace.go (P3) 가 이 표기를 그대로 RoutingReason 에 차용한다.
func TestPolicyMode_String_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mode v2.PolicyMode
		want string
	}{
		{v2.PreferQuality, "prefer_quality"},
		{v2.PreferLocal, "prefer_local"},
		{v2.PreferCheap, "prefer_cheap"},
		{v2.AlwaysSpecific, "always_specific"},
		{v2.PolicyMode(99), "unknown"}, // out-of-range falls through
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.mode.String())
		})
	}
}

// --------------------------------------------------------------------------
// TestRoutingPolicy_ZeroValue_HasEmptyChainAndCaps — REQ-RV2-002
// --------------------------------------------------------------------------

// RoutingPolicy{} 는 모든 slice 필드가 nil 이어야 한다 — 명시적 비할당 상태가
// "no opinion" 의미이며 v2 decorator 의 fast path 통과 조건이다.
func TestRoutingPolicy_ZeroValue_HasEmptyChainAndCaps(t *testing.T) {
	t.Parallel()

	var rp v2.RoutingPolicy
	require.Nil(t, rp.RequiredCapabilities, "zero value must keep slice nil")
	require.Nil(t, rp.ExcludedProviders, "zero value must keep slice nil")
	require.Nil(t, rp.FallbackChain, "zero value must keep slice nil")
	assert.Equal(t, 0.0, rp.RateLimitThreshold,
		"RoutingPolicy{}.RateLimitThreshold zero value remains 0; loader fills DefaultRateLimitThreshold")
}

// --------------------------------------------------------------------------
// TestDefaultRateLimitThreshold_IsEightyPercent — REQ-RV2-009
// --------------------------------------------------------------------------

// REQ-RV2-009 의 기본 임계값이 0.80 임을 회귀 보호한다.
func TestDefaultRateLimitThreshold_IsEightyPercent(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 0.80, v2.DefaultRateLimitThreshold, 0.0001)
}
