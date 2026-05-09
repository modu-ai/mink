package v2_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/modu-ai/goose/internal/llm/router/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// TestLoadPolicy_FileNotFound_ReturnsDefault — REQ-RV2-002 / P1-T3, P1-T4
// --------------------------------------------------------------------------

// routing-policy.yaml 부재 시 LoadPolicy 는 PreferQuality + 0.80 threshold
// 의 zero-opinion default 를 반환해야 한다. 이는 v1 Router 와 byte-identical
// 동작을 보장하는 backward-compat fast path 의 핵심이다.
func TestLoadPolicy_FileNotFound_ReturnsDefault(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	rp, err := v2.LoadPolicy(context.Background(), missing)

	require.NoError(t, err, "missing file MUST NOT surface as an error (backward-compat)")
	assert.Equal(t, v2.PreferQuality, rp.Mode)
	assert.InDelta(t, v2.DefaultRateLimitThreshold, rp.RateLimitThreshold, 0.0001)
	assert.Nil(t, rp.RequiredCapabilities)
	assert.Nil(t, rp.ExcludedProviders)
	assert.Nil(t, rp.FallbackChain)
}

// --------------------------------------------------------------------------
// TestLoadPolicy_EmptyFile_ReturnsDefault
// --------------------------------------------------------------------------

// 빈 파일도 부재 파일과 동등하게 default 로 처리되어야 한다.
func TestLoadPolicy_EmptyFile_ReturnsDefault(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "empty.yaml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o600))

	rp, err := v2.LoadPolicy(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, v2.PreferQuality, rp.Mode)
	assert.InDelta(t, v2.DefaultRateLimitThreshold, rp.RateLimitThreshold, 0.0001)
}

// --------------------------------------------------------------------------
// TestLoadPolicy_UnknownMode_ReturnsError — P1-T5, P1-T6
// --------------------------------------------------------------------------

// mode 가 4 enum 이 아니면 init 시점에 명시적 ErrUnknownPolicyMode 를
// 반환해야 한다. silent fallback 으로 PreferQuality 처리하면 사용자 의도가
// 묻혀버리므로 sentinel 에러가 필수.
func TestLoadPolicy_UnknownMode_ReturnsError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad-mode.yaml")
	require.NoError(t, os.WriteFile(path, []byte("mode: prefer_unicorn\n"), 0o600))

	_, err := v2.LoadPolicy(context.Background(), path)
	require.Error(t, err)
	assert.ErrorIs(t, err, v2.ErrUnknownPolicyMode)
	assert.Contains(t, err.Error(), "prefer_unicorn",
		"error message must name the rejected mode for diagnosability")
}

// --------------------------------------------------------------------------
// TestLoadPolicy_ThresholdOutOfRange_ReturnsError — P1-T7, P1-T8
// --------------------------------------------------------------------------

// rate_limit_threshold 는 [0.0, 1.0] 범위 강제. 음수와 > 1.0 양쪽 모두
// 동일한 sentinel ErrInvalidThreshold 를 반환해야 한다.
func TestLoadPolicy_ThresholdOutOfRange_ReturnsError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
	}{
		{"negative", "rate_limit_threshold: -0.1\n"},
		{"above_one", "rate_limit_threshold: 1.5\n"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), tc.name+".yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.body), 0o600))

			_, err := v2.LoadPolicy(context.Background(), path)
			require.Error(t, err)
			assert.ErrorIs(t, err, v2.ErrInvalidThreshold)
		})
	}
}

// --------------------------------------------------------------------------
// TestLoadPolicy_ThresholdBoundaries_AreValid
// --------------------------------------------------------------------------

// 정확히 0.0 과 1.0 은 valid 경계 — 사용자가 RouterV2 를 사실상 끄거나
// (1.0 = bucket 100% 만 제외) 항상 제외 (0.0 = 모든 후보 제외) 할 수 있어야
// 한다. 경계 inclusive 정책 회귀 보호.
func TestLoadPolicy_ThresholdBoundaries_AreValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		body  string
		value float64
	}{
		{"zero", "rate_limit_threshold: 0.0\n", 0.0},
		{"one", "rate_limit_threshold: 1.0\n", 1.0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), tc.name+".yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.body), 0o600))

			rp, err := v2.LoadPolicy(context.Background(), path)
			require.NoError(t, err)
			assert.InDelta(t, tc.value, rp.RateLimitThreshold, 0.0001)
		})
	}
}

// --------------------------------------------------------------------------
// TestLoadPolicy_ValidYAML_ParsesAllFields
// --------------------------------------------------------------------------

// spec.md §6.3 의 example schema 모든 필드를 한 번에 검증한다. 필드별 매핑
// regression 보호.
func TestLoadPolicy_ValidYAML_ParsesAllFields(t *testing.T) {
	t.Parallel()

	const body = `mode: prefer_cheap
rate_limit_threshold: 0.65
required_capabilities:
  - function_calling
  - vision
excluded_providers:
  - anthropic
fallback_chain:
  - provider: groq
    model: llama-3.3-70b
  - provider: mistral
    model: nemo
  - provider: openai
    model: gpt-4o
`
	path := filepath.Join(t.TempDir(), "policy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	rp, err := v2.LoadPolicy(context.Background(), path)
	require.NoError(t, err)

	assert.Equal(t, v2.PreferCheap, rp.Mode)
	assert.InDelta(t, 0.65, rp.RateLimitThreshold, 0.0001)
	assert.Equal(t,
		[]v2.Capability{v2.CapFunctionCalling, v2.CapVision},
		rp.RequiredCapabilities)
	assert.Equal(t, []string{"anthropic"}, rp.ExcludedProviders)

	require.Len(t, rp.FallbackChain, 3)
	assert.Equal(t, v2.ProviderRef{Provider: "groq", Model: "llama-3.3-70b"}, rp.FallbackChain[0])
	assert.Equal(t, v2.ProviderRef{Provider: "mistral", Model: "nemo"}, rp.FallbackChain[1])
	assert.Equal(t, v2.ProviderRef{Provider: "openai", Model: "gpt-4o"}, rp.FallbackChain[2])
}

// --------------------------------------------------------------------------
// TestLoadPolicy_AllFourModes_ParseCorrectly
// --------------------------------------------------------------------------

// 4 enum 모두 정상 매핑되는지 회귀 보호. 빈 mode (생략) 도 PreferQuality 로
// 받아들여야 backward-compat 가능.
func TestLoadPolicy_AllFourModes_ParseCorrectly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		want v2.PolicyMode
	}{
		{"empty_mode_defaults_to_prefer_quality", "", v2.PreferQuality},
		{"prefer_quality", "mode: prefer_quality\n", v2.PreferQuality},
		{"prefer_local", "mode: prefer_local\n", v2.PreferLocal},
		{"prefer_cheap", "mode: prefer_cheap\n", v2.PreferCheap},
		{"always_specific", "mode: always_specific\n", v2.AlwaysSpecific},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), tc.name+".yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.body), 0o600))

			rp, err := v2.LoadPolicy(context.Background(), path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, rp.Mode)
		})
	}
}

// --------------------------------------------------------------------------
// TestLoadPolicy_MalformedYAML_WrapsError
// --------------------------------------------------------------------------

// YAML 자체가 깨진 경우 yaml.v3 의 파싱 에러를 wrap 해서 반환해야 한다.
// caller 가 errors.Is 로 ErrUnknownPolicyMode/ErrInvalidThreshold 와
// 구분할 수 있어야 한다.
func TestLoadPolicy_MalformedYAML_WrapsError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(path,
		[]byte("mode: [this is not a string\n"), 0o600))

	_, err := v2.LoadPolicy(context.Background(), path)
	require.Error(t, err)
	assert.NotErrorIs(t, err, v2.ErrUnknownPolicyMode)
	assert.NotErrorIs(t, err, v2.ErrInvalidThreshold)
	assert.Contains(t, err.Error(), "parse policy")
}

// --------------------------------------------------------------------------
// TestLoadPolicy_PermissionError_WrapsError
// --------------------------------------------------------------------------

// 비-부재 read 에러 (e.g. EACCES) 는 sentinel 이 아닌 wrapped error 로
// 표면화되어야 한다. backward-compat 경로는 IsNotExist 단독.
func TestLoadPolicy_PermissionError_WrapsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "no-read.yaml")
	require.NoError(t, os.WriteFile(path, []byte("mode: prefer_quality\n"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	if os.Geteuid() == 0 {
		t.Skip("root bypasses 0o000 — skip when running as root")
	}

	_, err := v2.LoadPolicy(context.Background(), path)
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrPermission,
		"non-not-exist read error must wrap os.ErrPermission")
	assert.Contains(t, err.Error(), "read policy")
}

// --------------------------------------------------------------------------
// TestLoadPolicy_CanceledContext_ReturnsContextError
// --------------------------------------------------------------------------

// 이미 취소된 ctx 로 호출 시 LoadPolicy 는 파일 read 전에 ctx.Err() 를
// wrap 해 반환해야 한다. caller 가 cancellation propagation 으로 즉시
// 중단할 수 있도록 보장.
func TestLoadPolicy_CanceledContext_ReturnsContextError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	path := filepath.Join(t.TempDir(), "irrelevant.yaml")
	require.NoError(t, os.WriteFile(path, []byte("mode: prefer_cheap\n"), 0o600))

	_, err := v2.LoadPolicy(ctx, path)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled,
		"canceled ctx must surface as context.Canceled wrap")
	assert.Contains(t, err.Error(), "load policy")
}
