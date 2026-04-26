// Package errorclass — TDD RED 단계: AC-ERRCLASS-001~024 전체 failing 테스트
package errorclass_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/goose/internal/evolve/errorclass"
)

// ─── AC-ERRCLASS-001: 14 FailoverReason 열거형 완전성 ───────────────────

func TestAllFailoverReasons_14Items(t *testing.T) {
	all := errorclass.AllFailoverReasons()
	assert.Len(t, all, 14, "AllFailoverReasons()는 정확히 14개를 반환해야 한다")

	// 각 reason의 String()이 snake_case 반환
	expected := map[errorclass.FailoverReason]string{
		errorclass.Auth:              "auth",
		errorclass.AuthPermanent:     "auth_permanent",
		errorclass.Billing:           "billing",
		errorclass.RateLimit:         "rate_limit",
		errorclass.Overloaded:        "overloaded",
		errorclass.ServerError:       "server_error",
		errorclass.ContextOverflow:   "context_overflow",
		errorclass.PayloadTooLarge:   "payload_too_large",
		errorclass.ModelNotFound:     "model_not_found",
		errorclass.Timeout:           "timeout",
		errorclass.FormatError:       "format_error",
		errorclass.ThinkingSignature: "thinking_signature",
		errorclass.TransportError:    "transport_error",
		errorclass.Unknown:           "unknown",
	}
	for reason, want := range expected {
		assert.Equal(t, want, reason.String(), "reason %v String() 불일치", reason)
	}
}

// ─── AC-ERRCLASS-002: Anthropic thinking_signature 우선 분기 ────────────

func TestClassify_Anthropic_ThinkingSignature(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("thinking_signature mismatch between request and response")
	meta := errorclass.ErrorMeta{Provider: "anthropic", StatusCode: 400}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.ThinkingSignature, got.Reason)
	assert.False(t, got.Retryable)
	assert.True(t, got.ShouldFallback)
}

// ─── AC-ERRCLASS-003: HTTP 401 → Auth retryable+rotate ──────────────────

func TestClassify_HTTP_401_Auth(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("invalid api key")
	meta := errorclass.ErrorMeta{StatusCode: 401}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.Auth, got.Reason)
	assert.True(t, got.Retryable)
	assert.True(t, got.ShouldRotateCredential)
	assert.False(t, got.ShouldFallback)
	assert.False(t, got.ShouldCompress)
}

// ─── AC-ERRCLASS-004: HTTP 402 billing → fallback ───────────────────────

func TestClassify_HTTP_402_Billing(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("insufficient_quota")
	meta := errorclass.ErrorMeta{StatusCode: 402}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.Billing, got.Reason)
	assert.False(t, got.Retryable)
	assert.True(t, got.ShouldRotateCredential)
	assert.True(t, got.ShouldFallback)
}

// ─── AC-ERRCLASS-005: HTTP 413 payload → compress ───────────────────────

func TestClassify_HTTP_413_PayloadCompress(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("request body too large")
	meta := errorclass.ErrorMeta{StatusCode: 413}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.PayloadTooLarge, got.Reason)
	assert.True(t, got.Retryable)
	assert.True(t, got.ShouldCompress)
}

// ─── AC-ERRCLASS-006: 400 + context_length_exceeded message ─────────────

func TestClassify_400_ContextMessage_OverridesGenericBadRequest(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("context length exceeded: got 150000 tokens, max is 128000")
	meta := errorclass.ErrorMeta{StatusCode: 400}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.ContextOverflow, got.Reason)
	assert.True(t, got.Retryable)
	assert.True(t, got.ShouldCompress)
}

// ─── AC-ERRCLASS-007: 429 rate limit + rotate ───────────────────────────

func TestClassify_HTTP_429_RateLimit(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	meta := errorclass.ErrorMeta{StatusCode: 429}

	got := c.Classify(context.Background(), errors.New("too many requests"), meta)

	assert.Equal(t, errorclass.RateLimit, got.Reason)
	assert.True(t, got.Retryable)
	assert.True(t, got.ShouldRotateCredential)
}

// ─── AC-ERRCLASS-008: 503 overloaded → fallback ─────────────────────────

func TestClassify_HTTP_503_Overloaded(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("service unavailable")
	meta := errorclass.ErrorMeta{StatusCode: 503}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.Overloaded, got.Reason)
	assert.True(t, got.Retryable)
	assert.True(t, got.ShouldFallback)
}

// ─── AC-ERRCLASS-009: 529 anthropic overloaded ──────────────────────────

func TestClassify_HTTP_529_Overloaded(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	meta := errorclass.ErrorMeta{StatusCode: 529, Provider: "anthropic"}

	got := c.Classify(context.Background(), errors.New("overloaded"), meta)

	assert.Equal(t, errorclass.Overloaded, got.Reason)
}

// ─── AC-ERRCLASS-010: context.DeadlineExceeded → Timeout ───────────────

func TestClassify_DeadlineExceeded_Timeout(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	meta := errorclass.ErrorMeta{StatusCode: 0}

	got := c.Classify(context.Background(), context.DeadlineExceeded, meta)

	assert.Equal(t, errorclass.Timeout, got.Reason)
	assert.True(t, got.Retryable)
}

// ─── AC-ERRCLASS-011: Transport 휴리스틱: 큰 컨텍스트 → ContextOverflow ──

func TestClassify_TransportDisconnect_BigContext_Overflow(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("server disconnected")
	// 125_000 > 120_000 threshold 만족
	meta := errorclass.ErrorMeta{
		StatusCode:    0,
		ApproxTokens:  125_000,
		ContextLength: 200_000,
	}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.ContextOverflow, got.Reason)
	assert.True(t, got.ShouldCompress)
}

// ─── AC-ERRCLASS-012: 404 model not found → fallback ────────────────────

func TestClassify_404_ModelNotFound(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("model 'gpt-5-turbo-nonexistent' not found")
	meta := errorclass.ErrorMeta{StatusCode: 404}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.ModelNotFound, got.Reason)
	assert.False(t, got.Retryable)
	assert.True(t, got.ShouldFallback)
}

// ─── AC-ERRCLASS-013: nil 오류 안전 처리 ────────────────────────────────

func TestClassify_NilError(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})

	require.NotPanics(t, func() {
		got := c.Classify(context.Background(), nil, errorclass.ErrorMeta{})
		assert.Equal(t, errorclass.Unknown, got.Reason)
		assert.False(t, got.Retryable)
		assert.Equal(t, "nil error", got.Message)
	})
}

// ─── AC-ERRCLASS-014: 알 수 없는 오류 fallback ──────────────────────────

func TestClassify_UnknownFallbackRetryable(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("strange ufo error 🛸")
	meta := errorclass.ErrorMeta{StatusCode: 0, Provider: "unknown_cloud"}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.Unknown, got.Reason)
	assert.True(t, got.Retryable)
}

// ─── AC-ERRCLASS-015: 파이프라인 순서(provider 우선) ────────────────────

func TestClassify_PipelineOrder_ProviderBeforeHTTP(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	// status 429이지만 anthropic thinking_signature 오류 — stage 1이 우선
	err := errors.New("thinking_signature mismatch")
	meta := errorclass.ErrorMeta{Provider: "anthropic", StatusCode: 429}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.ThinkingSignature, got.Reason, "stage1(provider)이 stage2(HTTP 429)보다 우선해야 한다")
}

// ─── AC-ERRCLASS-016: 패닉 방어 ─────────────────────────────────────────

func TestClassify_PanicRecovered(t *testing.T) {
	// nil *regexp.Regexp를 Pattern 필드에 넣어 패닉 유발
	opts := errorclass.ClassifierOptions{
		ExtraPatterns: []errorclass.ProviderPattern{
			{Provider: "panic_test", Pattern: nil, Reason: errorclass.Overloaded},
		},
	}
	c := errorclass.New(opts)
	err := errors.New("trigger panic")
	meta := errorclass.ErrorMeta{Provider: "panic_test"}

	require.NotPanics(t, func() {
		got := c.Classify(context.Background(), err, meta)
		assert.Equal(t, errorclass.Unknown, got.Reason)
		assert.False(t, got.Retryable)
		assert.Equal(t, "classification panic recovered", got.Message)
	})
}

// ─── AC-ERRCLASS-017: ExtraPatterns 주입 ────────────────────────────────

func TestOptions_ExtraPatterns(t *testing.T) {
	// SPEC AC-017: ExtraPatterns 주입으로 신규 provider 지원 검증
	// pattern은 err 메시지에 실제로 매칭되어야 하므로 "overloaded" 키워드 사용
	importRe := regexp.MustCompile(`(?i)overloaded`)
	opts := errorclass.ClassifierOptions{
		ExtraPatterns: []errorclass.ProviderPattern{
			{Provider: "mistral", Pattern: importRe, Reason: errorclass.Overloaded},
		},
	}
	c := errorclass.New(opts)
	err := errors.New("our model is temporarily overloaded")
	meta := errorclass.ErrorMeta{Provider: "mistral"}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.Overloaded, got.Reason)
}

// ─── AC-ERRCLASS-018: OverrideFlags 정책 변경 ────────────────────────────

func TestOptions_OverrideFlags(t *testing.T) {
	opts := errorclass.ClassifierOptions{
		OverrideFlags: map[errorclass.FailoverReason]errorclass.FlagProfile{
			errorclass.Timeout: {Retryable: false, ShouldFallback: true},
		},
	}
	c := errorclass.New(opts)

	got := c.Classify(context.Background(), context.DeadlineExceeded, errorclass.ErrorMeta{})

	assert.Equal(t, errorclass.Timeout, got.Reason)
	assert.False(t, got.Retryable, "OverrideFlags로 Retryable=false가 되어야 한다")
	assert.True(t, got.ShouldFallback, "OverrideFlags로 ShouldFallback=true가 되어야 한다")
}

// ─── AC-ERRCLASS-019: RawError 보존 ─────────────────────────────────────

func TestClassify_RawError_PreservesUnwrapChain(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	innerErr := errors.New("provider inner failure")
	wrapped := fmt.Errorf("outer: %w", innerErr)
	meta := errorclass.ErrorMeta{StatusCode: 500}

	got := c.Classify(context.Background(), wrapped, meta)

	require.NotNil(t, got.RawError)
	// unwrap chain을 통해 innerErr에 도달 가능
	assert.True(t, errors.Is(got.RawError, innerErr), "errors.Is로 innerErr에 도달 가능해야 한다")

	// 3단계 체인 검증
	level3 := errors.New("deepest")
	level2 := fmt.Errorf("level2: %w", level3)
	level1 := fmt.Errorf("level1: %w", level2)

	got2 := c.Classify(context.Background(), level1, meta)
	assert.True(t, errors.Is(got2.RawError, level3), "3단계 체인도 errors.Is로 도달 가능해야 한다")
}

// ─── AC-ERRCLASS-020: HTTP 403 permission → AuthPermanent ───────────────

func TestClassify_HTTP_403_Permission_AuthPermanent(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	err := errors.New("permission denied: this API key does not have access to the requested resource")
	meta := errorclass.ErrorMeta{StatusCode: 403}

	got := c.Classify(context.Background(), err, meta)

	assert.Equal(t, errorclass.AuthPermanent, got.Reason)
	assert.False(t, got.Retryable)
	assert.True(t, got.ShouldRotateCredential)
	assert.True(t, got.ShouldFallback)
	assert.False(t, got.ShouldCompress)
}

// ─── AC-ERRCLASS-021: HTTP 500/502 → ServerError ─────────────────────────

func TestClassify_HTTP_500_502_ServerError(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	cases := []struct {
		statusCode int
		errMsg     string
	}{
		{500, "internal server error"},
		{502, "bad gateway"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("HTTP_%d", tc.statusCode), func(t *testing.T) {
			meta := errorclass.ErrorMeta{StatusCode: tc.statusCode}
			got := c.Classify(context.Background(), errors.New(tc.errMsg), meta)

			assert.Equal(t, errorclass.ServerError, got.Reason)
			assert.True(t, got.Retryable)
			assert.True(t, got.ShouldFallback)
			assert.False(t, got.ShouldRotateCredential)
			assert.False(t, got.ShouldCompress)
		})
	}
}

// ─── AC-ERRCLASS-022: 미지원 provider에서 stage 1 skip ──────────────────

func TestClassify_UnknownProvider_SkipsStage1(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	// anthropic 특화 패턴이 메시지에 우연히 포함됨 — groq는 stage 1 skip
	err := errors.New("thinking_signature looks suspicious")
	meta := errorclass.ErrorMeta{Provider: "groq", StatusCode: 0}

	got := c.Classify(context.Background(), err, meta)

	// stage1_provider로 분류되면 안 됨
	assert.NotEqual(t, "stage1_provider", got.MatchedBy, "groq는 stage 1을 건너뛰어야 한다")
	// ThinkingSignature로 분류되면 안 됨 (provider 매칭 없이는)
	assert.NotEqual(t, errorclass.ThinkingSignature, got.Reason)
}

// ─── AC-ERRCLASS-023: meta 불변성 ────────────────────────────────────────

func TestClassify_MetaImmutability(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	rawErr := errors.New("rate limit")
	meta := errorclass.ErrorMeta{
		Provider:      "openai",
		Model:         "gpt-4",
		StatusCode:    429,
		ApproxTokens:  50_000,
		ContextLength: 128_000,
		MessageCount:  42,
		RawError:      rawErr,
	}
	snapshot := meta

	_ = c.Classify(context.Background(), meta.RawError, meta)

	assert.Equal(t, snapshot.Provider, meta.Provider)
	assert.Equal(t, snapshot.Model, meta.Model)
	assert.Equal(t, snapshot.StatusCode, meta.StatusCode)
	assert.Equal(t, snapshot.ApproxTokens, meta.ApproxTokens)
	assert.Equal(t, snapshot.ContextLength, meta.ContextLength)
	assert.Equal(t, snapshot.MessageCount, meta.MessageCount)
}

// ─── AC-ERRCLASS-024: retryable+fallback 플래그 조합 불변식 ─────────────

func TestClassify_RetryableFallback_MutualExclusion(t *testing.T) {
	// defaults 표에서 retryable=true AND should_fallback=true 는
	// Overloaded와 ServerError 두 reason에서만 허용
	allowedBoth := map[errorclass.FailoverReason]bool{
		errorclass.Overloaded:  true,
		errorclass.ServerError: true,
	}

	for _, reason := range errorclass.AllFailoverReasons() {
		flags := errorclass.DefaultFlags(reason)
		if flags.Retryable && flags.ShouldFallback {
			assert.True(t, allowedBoth[reason],
				"reason %s는 retryable=true AND should_fallback=true 조합이 허용되지 않는다", reason)
		}
	}
}

// ─── MarshalText / UnmarshalText ─────────────────────────────────────────

func TestFailoverReason_MarshalUnmarshal(t *testing.T) {
	for _, reason := range errorclass.AllFailoverReasons() {
		b, err := reason.MarshalText()
		require.NoError(t, err, "MarshalText(%s) 오류", reason)

		var got errorclass.FailoverReason
		require.NoError(t, got.UnmarshalText(b), "UnmarshalText(%s) 오류", string(b))
		assert.Equal(t, reason, got)
	}
}

func TestFailoverReason_UnmarshalText_InvalidInput(t *testing.T) {
	var r errorclass.FailoverReason
	err := r.UnmarshalText([]byte("not_a_valid_reason"))
	assert.Error(t, err)
}

// ─── net.Error 구현 (Timeout 테스트용) ───────────────────────────────────

type mockNetError struct {
	timeout bool
}

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return true }

var _ net.Error = (*mockNetError)(nil)

func TestClassify_NetError_Timeout(t *testing.T) {
	c := errorclass.New(errorclass.ClassifierOptions{})
	netErr := &mockNetError{timeout: true}
	meta := errorclass.ErrorMeta{StatusCode: 0}

	got := c.Classify(context.Background(), netErr, meta)

	assert.Equal(t, errorclass.Timeout, got.Reason)
	assert.True(t, got.Retryable)
}
