// Package v2 — fallback_test.go: 14 FailoverReason 분기 검증.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-005 / REQ-RV2-013 / REQ-RV2-014
package v2

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/evolve/errorclass"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClassifier 는 errorclass.Classifier 의 테스트용 구현이다.
// 호출 시점에 미리 큐잉된 reason 을 순서대로 반환한다.
type fakeClassifier struct {
	reasons []errorclass.FailoverReason
	calls   int
}

func (f *fakeClassifier) Classify(_ context.Context, err error, _ errorclass.ErrorMeta) errorclass.ClassifiedError {
	if f.calls >= len(f.reasons) {
		return errorclass.ClassifiedError{Reason: errorclass.Unknown, RawError: err}
	}
	r := f.reasons[f.calls]
	f.calls++
	return errorclass.ClassifiedError{Reason: r, RawError: err}
}

// chainFn 은 Execute 의 fn 매개변수 시뮬레이션이다.
// errsByProvider 에 등록된 provider 호출 시 해당 에러를 반환,
// successProviders 에 등록된 provider 는 nil 에러로 성공.
type chainFn struct {
	errsByProvider   map[string]error
	successProviders map[string]bool
	callOrder        []string
}

func (c *chainFn) call(_ context.Context, ref ProviderRef) (any, error) {
	c.callOrder = append(c.callOrder, ref.Provider)
	if c.successProviders[ref.Provider] {
		return "ok-" + ref.Provider, nil
	}
	if e, ok := c.errsByProvider[ref.Provider]; ok {
		return nil, e
	}
	return nil, errors.New("no-config-for-" + ref.Provider)
}

// stopChainReasons 는 spec.md §4.4 REQ-RV2-013 의 chain 즉시 중단 reasons.
// ContextOverflow / FormatError / PayloadTooLarge — 다음 후보로도 같은 입력
// 이라 회복 불가능.
var stopChainReasons = []errorclass.FailoverReason{
	errorclass.ContextOverflow,
	errorclass.FormatError,
	errorclass.PayloadTooLarge,
}

// nextCandidateReasons 는 다음 후보 시도가 의미 있는 11 reasons.
var nextCandidateReasons = []errorclass.FailoverReason{
	errorclass.Unknown,
	errorclass.Auth,
	errorclass.AuthPermanent,
	errorclass.Billing,
	errorclass.RateLimit,
	errorclass.Overloaded,
	errorclass.ServerError,
	errorclass.ModelNotFound,
	errorclass.Timeout,
	errorclass.ThinkingSignature,
	errorclass.TransportError,
}

// TestFallback_AllReasonsCovered 는 14 reason 이 빠짐없이 분류표에 들어가
// 있는지 검증한다 (P3-T1 RED).
func TestFallback_AllReasonsCovered(t *testing.T) {
	all := errorclass.AllFailoverReasons()
	assert.Equal(t, 14, len(all), "ERROR-CLASS-001 가 14 reason 이 아님 — drift 의심")
	covered := make(map[errorclass.FailoverReason]bool, 14)
	for _, r := range stopChainReasons {
		covered[r] = true
	}
	for _, r := range nextCandidateReasons {
		covered[r] = true
	}
	assert.Equal(t, 14, len(covered), "stop+next 합계가 14 가 아님 — 누락 또는 중복")
}

// TestFallback_StopChainReasons 는 3 stop reason 이 즉시 중단을 트리거하는지
// 검증한다 (REQ-RV2-013).
func TestFallback_StopChainReasons(t *testing.T) {
	for _, reason := range stopChainReasons {
		t.Run(reason.String(), func(t *testing.T) {
			cls := &fakeClassifier{reasons: []errorclass.FailoverReason{reason}}
			fn := &chainFn{errsByProvider: map[string]error{
				"anthropic": errors.New("blocked"),
			}}
			exec := NewFallbackExecutor(cls)
			chain := []ProviderRef{
				{Provider: "anthropic", Model: "claude-opus-4-7"},
				{Provider: "openai", Model: "gpt-4o"},
				{Provider: "google", Model: "gemini-2.0-flash"},
			}
			_, err := exec.Execute(context.Background(), chain, fn.call)
			require.Error(t, err)
			assert.Equal(t, []string{"anthropic"}, fn.callOrder,
				"%s 는 즉시 중단되어야 하는데 다음 후보가 시도됨", reason)
			// stop chain 시 wrapped 에러는 stop reason 을 포함해야 한다.
			var ferr *FallbackError
			require.True(t, errors.As(err, &ferr))
			assert.Equal(t, reason, ferr.LastReason)
			assert.True(t, ferr.Stopped)
		})
	}
}

// TestFallback_NextCandidateReasons 는 11 next reason 이 다음 후보를 시도
// 하는지 검증한다 (REQ-RV2-005).
func TestFallback_NextCandidateReasons(t *testing.T) {
	for _, reason := range nextCandidateReasons {
		t.Run(reason.String(), func(t *testing.T) {
			cls := &fakeClassifier{reasons: []errorclass.FailoverReason{reason}}
			fn := &chainFn{
				errsByProvider:   map[string]error{"anthropic": errors.New("first-fail")},
				successProviders: map[string]bool{"openai": true},
			}
			exec := NewFallbackExecutor(cls)
			chain := []ProviderRef{
				{Provider: "anthropic", Model: "claude-opus-4-7"},
				{Provider: "openai", Model: "gpt-4o"},
			}
			result, err := exec.Execute(context.Background(), chain, fn.call)
			require.NoError(t, err)
			assert.Equal(t, "ok-openai", result)
			assert.Equal(t, []string{"anthropic", "openai"}, fn.callOrder,
				"%s 는 다음 후보로 진행되어야 함", reason)
		})
	}
}

// TestFallback_AllCandidatesFail 는 모든 후보가 실패하면 MultiError 가
// 반환되는지 검증한다 (chain exhausted 시나리오).
func TestFallback_AllCandidatesFail_ReturnsMultiError(t *testing.T) {
	cls := &fakeClassifier{reasons: []errorclass.FailoverReason{
		errorclass.RateLimit,
		errorclass.Overloaded,
		errorclass.Timeout,
	}}
	fn := &chainFn{errsByProvider: map[string]error{
		"anthropic": errors.New("rate-limited"),
		"openai":    errors.New("overloaded"),
		"google":    errors.New("timeout"),
	}}
	exec := NewFallbackExecutor(cls)
	chain := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "google", Model: "gemini-2.0-flash"},
	}
	_, err := exec.Execute(context.Background(), chain, fn.call)
	require.Error(t, err)
	assert.Equal(t, []string{"anthropic", "openai", "google"}, fn.callOrder)

	var ferr *FallbackError
	require.True(t, errors.As(err, &ferr))
	assert.False(t, ferr.Stopped, "exhaustion 은 Stopped 가 아님")
	assert.Len(t, ferr.Attempts, 3, "3 후보 모두 시도해야 함")
}

// TestFallback_EmptyChain 는 빈 chain 호출 시 ErrEmptyChain 반환을 검증한다.
func TestFallback_EmptyChain_ReturnsError(t *testing.T) {
	cls := &fakeClassifier{}
	fn := &chainFn{}
	exec := NewFallbackExecutor(cls)
	_, err := exec.Execute(context.Background(), nil, fn.call)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyChain)
}

// TestFallback_FirstSuccess_ShortCircuits 는 첫 후보 성공 시 나머지 chain 을
// 건너뛰는지 검증한다 (불필요한 호출 방지).
func TestFallback_FirstSuccess_ShortCircuits(t *testing.T) {
	cls := &fakeClassifier{}
	fn := &chainFn{successProviders: map[string]bool{"anthropic": true}}
	exec := NewFallbackExecutor(cls)
	chain := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	result, err := exec.Execute(context.Background(), chain, fn.call)
	require.NoError(t, err)
	assert.Equal(t, "ok-anthropic", result)
	assert.Equal(t, []string{"anthropic"}, fn.callOrder)
}

// TestFallback_ContextCanceled 는 ctx 가 취소되면 즉시 ctx.Err 을 wrap 해
// 반환하는지 검증한다.
func TestFallback_ContextCanceled_ReturnsCtxErr(t *testing.T) {
	cls := &fakeClassifier{}
	fn := &chainFn{}
	exec := NewFallbackExecutor(cls)
	chain := []ProviderRef{{Provider: "anthropic", Model: "claude-opus-4-7"}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := exec.Execute(ctx, chain, fn.call)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestFallback_ExcludedProviders_SilentSkip 는 excludedProviders 가 chain
// 에 포함되어도 silent skip 되는지 검증한다 (REQ-RV2-012).
func TestFallback_ExcludedProviders_SilentSkip(t *testing.T) {
	cls := &fakeClassifier{}
	fn := &chainFn{successProviders: map[string]bool{"openai": true}}
	exec := NewFallbackExecutor(cls)
	exec.SetExcluded([]string{"anthropic"})
	chain := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	result, err := exec.Execute(context.Background(), chain, fn.call)
	require.NoError(t, err)
	assert.Equal(t, "ok-openai", result)
	assert.Equal(t, []string{"openai"}, fn.callOrder, "anthropic 는 silent skip 되어야 함")
}

// TestFallback_AllExcluded_ReturnsErrAllExcluded 는 chain 의 모든 후보가
// excluded 면 ErrAllExcluded 를 반환하는지 검증한다.
func TestFallback_AllExcluded_ReturnsErr(t *testing.T) {
	cls := &fakeClassifier{}
	fn := &chainFn{}
	exec := NewFallbackExecutor(cls)
	exec.SetExcluded([]string{"anthropic", "openai"})
	chain := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	_, err := exec.Execute(context.Background(), chain, fn.call)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAllExcluded)
	assert.Empty(t, fn.callOrder, "어느 후보도 호출되지 않아야 함")
}

// TestFallbackError_Error 는 *FallbackError.Error() 형식을 검증한다.
func TestFallbackError_Error(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var e *FallbackError
		assert.Equal(t, "v2: fallback error <nil>", e.Error())
	})
	t.Run("empty attempts", func(t *testing.T) {
		e := &FallbackError{}
		assert.Equal(t, "v2: fallback error: no attempts", e.Error())
	})
	t.Run("stopped chain", func(t *testing.T) {
		e := &FallbackError{
			Stopped:    true,
			LastReason: errorclass.ContextOverflow,
			Attempts: []Attempt{{
				Provider: "anthropic", Model: "claude-opus-4-7",
				Reason: errorclass.ContextOverflow,
				Err:    errors.New("ctx overflow"),
			}},
		}
		msg := e.Error()
		assert.Contains(t, msg, "stopped")
		assert.Contains(t, msg, "anthropic/claude-opus-4-7")
		assert.Contains(t, msg, "context_overflow")
	})
	t.Run("exhausted chain", func(t *testing.T) {
		e := &FallbackError{
			Stopped:    false,
			LastReason: errorclass.RateLimit,
			Attempts: []Attempt{
				{Provider: "anthropic", Model: "claude-opus-4-7", Err: errors.New("a")},
				{Provider: "openai", Model: "gpt-4o", Err: errors.New("b")},
			},
		}
		msg := e.Error()
		assert.Contains(t, msg, "exhausted")
		assert.Contains(t, msg, "openai/gpt-4o")
		assert.Contains(t, msg, "2 attempt(s)")
	})
}

// TestFallbackError_Unwrap 는 errors.Is/As 호환성 검증.
func TestFallbackError_Unwrap(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var e *FallbackError
		assert.Nil(t, e.Unwrap())
	})
	t.Run("empty attempts returns nil", func(t *testing.T) {
		e := &FallbackError{}
		assert.Nil(t, e.Unwrap())
	})
	t.Run("non-empty attempts returns last err", func(t *testing.T) {
		sentinel := errors.New("last-err-sentinel")
		e := &FallbackError{
			Attempts: []Attempt{
				{Err: errors.New("first")},
				{Err: sentinel},
			},
		}
		assert.Same(t, sentinel, e.Unwrap())
		assert.True(t, errors.Is(e, sentinel), "errors.Is 가 마지막 attempt err 를 통해 매칭")
	})
}

// TestFallback_NilClassifier_DefaultsToUnknown 는 classifier 가 nil 일 때
// 모든 에러가 Unknown 으로 분류되어 next candidate 분기로 진행되는지 검증.
func TestFallback_NilClassifier_TreatsAllAsUnknown(t *testing.T) {
	fn := &chainFn{
		errsByProvider:   map[string]error{"anthropic": errors.New("first-fail")},
		successProviders: map[string]bool{"openai": true},
	}
	exec := NewFallbackExecutor(nil) // classifier nil
	chain := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	result, err := exec.Execute(context.Background(), chain, fn.call)
	require.NoError(t, err)
	assert.Equal(t, "ok-openai", result)
	attempts := exec.LastAttempts()
	require.Len(t, attempts, 2)
	assert.Equal(t, errorclass.Unknown, attempts[0].Reason, "nil classifier → Unknown 분류")
}

// TestFallback_SetExcluded_NilOrEmpty_ClearsState 는 SetExcluded(nil) 호출
// 시 이전 exclude 가 해제되는지 검증한다 (re-use 시 깨끗한 상태).
func TestFallback_SetExcluded_NilOrEmpty_ClearsState(t *testing.T) {
	exec := NewFallbackExecutor(nil)
	exec.SetExcluded([]string{"anthropic"})
	exec.SetExcluded(nil) // clear
	fn := &chainFn{successProviders: map[string]bool{"anthropic": true}}
	chain := []ProviderRef{{Provider: "anthropic", Model: "claude-opus-4-7"}}
	result, err := exec.Execute(context.Background(), chain, fn.call)
	require.NoError(t, err)
	assert.Equal(t, "ok-anthropic", result, "exclude 가 해제되어 호출되어야 함")
}

// TestFallback_AttemptsRecorded 는 각 시도가 Attempts 에 기록되는지 검증한다
// (REQ-RV2-011 trace 보호).
func TestFallback_AttemptsRecorded(t *testing.T) {
	cls := &fakeClassifier{reasons: []errorclass.FailoverReason{
		errorclass.RateLimit,
		errorclass.Timeout,
	}}
	fn := &chainFn{
		errsByProvider:   map[string]error{"anthropic": errors.New("rate"), "openai": errors.New("timeout")},
		successProviders: map[string]bool{"google": true},
	}
	exec := NewFallbackExecutor(cls)
	chain := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "google", Model: "gemini-2.0-flash"},
	}
	result, err := exec.Execute(context.Background(), chain, fn.call)
	require.NoError(t, err)
	assert.Equal(t, "ok-google", result)
	attempts := exec.LastAttempts()
	require.Len(t, attempts, 3)
	assert.Equal(t, errorclass.RateLimit, attempts[0].Reason)
	assert.Equal(t, "anthropic", attempts[0].Provider)
	assert.Equal(t, errorclass.Timeout, attempts[1].Reason)
	assert.Equal(t, "openai", attempts[1].Provider)
	// 마지막 성공한 attempt 는 Reason=Unknown (분류 호출 안 됨).
	assert.Equal(t, "google", attempts[2].Provider)
	assert.True(t, attempts[2].Success)
}
