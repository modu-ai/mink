package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/router"
	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fallbackStubProvider는 테스트용 Provider 구현이다.
type fallbackStubProvider struct {
	name         string
	caps         provider.Capabilities
	streamErr    error
	streamEvents []message.StreamEvent
}

func (s *fallbackStubProvider) Name() string                        { return s.name }
func (s *fallbackStubProvider) Capabilities() provider.Capabilities { return s.caps }

func (s *fallbackStubProvider) Stream(_ context.Context, _ provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	ch := make(chan message.StreamEvent, len(s.streamEvents)+1)
	for _, e := range s.streamEvents {
		ch <- e
	}
	close(ch)
	return ch, nil
}

func (s *fallbackStubProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return nil, errors.New("not implemented in stub")
}

// TestFallback_PrimarySuccess는 primary 호출 성공 시 바로 반환하는지 검증한다.
func TestFallback_PrimarySuccess(t *testing.T) {
	t.Parallel()
	primary := &fallbackStubProvider{
		name: "primary",
		streamEvents: []message.StreamEvent{
			{Type: message.TypeTextDelta, Delta: "hello"},
			{Type: message.TypeMessageStop},
		},
	}

	req := provider.CompletionRequest{
		Route:          router.Route{Provider: "primary", Model: "model-a"},
		FallbackModels: []string{"fallback-model"},
	}

	ch, err := provider.TryWithFallback(context.Background(), primary, req)
	require.NoError(t, err)

	var evts []message.StreamEvent
	for e := range ch {
		evts = append(evts, e)
	}

	textDeltas := fallbackFilterByType(evts, message.TypeTextDelta)
	require.Len(t, textDeltas, 1)
	assert.Equal(t, "hello", textDeltas[0].Delta)
}

// TestFallback_FirstFailsSecondSucceeds는 primary 실패 시 첫 번째 fallback으로 전환하는지 검증한다.
// AC-ADAPTER-009 커버.
func TestFallback_FirstFailsSecondSucceeds(t *testing.T) {
	t.Parallel()
	failProvider := &fallbackStubProvider{
		name:      "fail",
		streamErr: errors.New("internal server error"),
	}
	successProvider := &fallbackStubProvider{
		name: "fail", // 같은 provider지만 다른 model로 retry
		streamEvents: []message.StreamEvent{
			{Type: message.TypeTextDelta, Delta: "fallback response"},
			{Type: message.TypeMessageStop},
		},
	}

	callCount := 0
	mockProvider := &callCountingProvider{
		name: "fail",
		streamFn: func(req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
			callCount++
			if callCount == 1 {
				return failProvider.Stream(context.Background(), req)
			}
			return successProvider.Stream(context.Background(), req)
		},
	}

	req := provider.CompletionRequest{
		Route:          router.Route{Provider: "fail", Model: "model-primary"},
		FallbackModels: []string{"model-fallback"},
	}

	ch, err := provider.TryWithFallback(context.Background(), mockProvider, req)
	require.NoError(t, err)

	var evts []message.StreamEvent
	for e := range ch {
		evts = append(evts, e)
	}

	textDeltas := fallbackFilterByType(evts, message.TypeTextDelta)
	require.Len(t, textDeltas, 1)
	assert.Equal(t, "fallback response", textDeltas[0].Delta)
	assert.Equal(t, 2, callCount, "fallback으로 두 번 호출되어야 함")
}

// TestFallback_AllFail은 모든 fallback 소진 시 원래 에러를 반환하는지 검증한다.
func TestFallback_AllFail(t *testing.T) {
	t.Parallel()
	originalErr := errors.New("all failed: connection refused")
	failProvider := &fallbackStubProvider{
		name:      "fail",
		streamErr: originalErr,
	}

	req := provider.CompletionRequest{
		Route:          router.Route{Provider: "fail", Model: "model-a"},
		FallbackModels: []string{"model-b", "model-c"},
	}

	_, err := provider.TryWithFallback(context.Background(), failProvider, req)
	require.Error(t, err)
	// 원래 에러가 포함되어야 함
	assert.Contains(t, err.Error(), "all failed")
}

// TestFallback_NoFallbacks는 FallbackModels이 비어있을 때 primary 결과를 반환하는지 검증한다.
func TestFallback_NoFallbacks(t *testing.T) {
	t.Parallel()
	primary := &fallbackStubProvider{
		name: "primary",
		streamEvents: []message.StreamEvent{
			{Type: message.TypeTextDelta, Delta: "ok"},
			{Type: message.TypeMessageStop},
		},
	}

	req := provider.CompletionRequest{
		Route:          router.Route{Provider: "primary", Model: "model-a"},
		FallbackModels: nil, // fallback 없음
	}

	ch, err := provider.TryWithFallback(context.Background(), primary, req)
	require.NoError(t, err)

	var evts []message.StreamEvent
	for e := range ch {
		evts = append(evts, e)
	}
	assert.Len(t, fallbackFilterByType(evts, message.TypeTextDelta), 1)
}

// callCountingProvider는 호출 수를 추적하는 Provider이다.
type callCountingProvider struct {
	name     string
	streamFn func(req provider.CompletionRequest) (<-chan message.StreamEvent, error)
}

func (p *callCountingProvider) Name() string                        { return p.name }
func (p *callCountingProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *callCountingProvider) Stream(_ context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	return p.streamFn(req)
}
func (p *callCountingProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func fallbackFilterByType(evts []message.StreamEvent, typ string) []message.StreamEvent {
	var result []message.StreamEvent
	for _, e := range evts {
		if e.Type == typ {
			result = append(result, e)
		}
	}
	return result
}
