package provider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/modu-ai/goose/internal/llm/cache"
	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewLLMCall_WithStubProvider_DrainsStream은 stub provider로 스트림을 드레인하는지 검증한다.
func TestNewLLMCall_WithStubProvider_DrainsStream(t *testing.T) {
	t.Parallel()

	// 스트리밍 이벤트를 방출하는 stub provider
	streamProvider := &streamingStubProvider{
		events: []message.StreamEvent{
			{Type: message.TypeTextDelta, Delta: "hello"},
			{Type: message.TypeMessageStop},
		},
	}

	reg := provider.NewRegistry()
	require.NoError(t, reg.Register(streamProvider))

	pool := makeTestPool(t, "cred-1")
	tracker := ratelimit.NewTracker()
	planner := &cache.BreakpointPlanner{}
	logger, _ := zap.NewDevelopment()

	fn := provider.NewLLMCall(
		reg,
		pool,
		tracker,
		planner,
		cache.StrategyNone,
		cache.TTLEphemeral,
		nil, // secretStore
		logger,
	)

	req := query.LLMCallReq{
		Route: router.Route{Model: "test-model", Provider: "streaming-stub"},
	}

	ch, err := fn(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	var events []message.StreamEvent
	for evt := range ch {
		events = append(events, evt)
	}

	assert.Len(t, events, 2)
	assert.Equal(t, message.TypeTextDelta, events[0].Type)
	assert.Equal(t, "hello", events[0].Delta)
	assert.Equal(t, message.TypeMessageStop, events[1].Type)
}

// TestNewLLMCall_UnknownProvider_ReturnsError는 미등록 provider 시 에러를 검증한다.
func TestNewLLMCall_UnknownProvider_ReturnsError(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()
	pool := makeTestPool(t, "cred-1")
	tracker := ratelimit.NewTracker()
	planner := &cache.BreakpointPlanner{}
	logger, _ := zap.NewDevelopment()

	fn := provider.NewLLMCall(
		reg,
		pool,
		tracker,
		planner,
		cache.StrategyNone,
		cache.TTLEphemeral,
		nil,
		logger,
	)

	_, err := fn(context.Background(), query.LLMCallReq{
		Route: router.Route{Provider: "nonexistent"},
	})
	assert.Error(t, err)
	assert.ErrorAs(t, err, &provider.ErrProviderNotFound{})
}

// makeTestPool은 테스트용 단일 크레덴셜 풀을 생성한다.
func makeTestPool(t *testing.T, credID string) *credential.CredentialPool {
	t.Helper()
	creds := []*credential.PooledCredential{
		{ID: credID, Provider: "test", KeyringID: "kr-" + credID, Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	require.NoError(t, err)
	return pool
}

// TestNewLLMCall_VisionUnsupported_ReturnsError는 AC-ADAPTER-011을 검증한다.
// Vision=false provider에 이미지 포함 요청 시 ErrCapabilityUnsupported를 반환해야 한다.
func TestNewLLMCall_VisionUnsupported_ReturnsError(t *testing.T) {
	t.Parallel()

	// Vision=false provider (DeepSeek 패턴)
	noVisionProvider := &noVisionStubProvider{}

	reg := provider.NewRegistry()
	require.NoError(t, reg.Register(noVisionProvider))

	pool := makeTestPool(t, "cred-1")
	tracker := ratelimit.NewTracker()
	planner := &cache.BreakpointPlanner{}
	logger, _ := zap.NewDevelopment()

	fn := provider.NewLLMCall(
		reg,
		pool,
		tracker,
		planner,
		cache.StrategyNone,
		cache.TTLEphemeral,
		nil,
		logger,
	)

	// 이미지 ContentBlock이 포함된 요청
	req := query.LLMCallReq{
		Route: router.Route{Model: "deepseek-chat", Provider: "no-vision"},
		Messages: []message.Message{
			{
				Role: "user",
				Content: []message.ContentBlock{
					{Type: "image", Image: []byte("fake-image"), ImageMediaType: "image/jpeg"},
				},
			},
		},
	}

	_, err := fn(context.Background(), req)
	require.Error(t, err)

	var capErr provider.ErrCapabilityUnsupported
	assert.ErrorAs(t, err, &capErr)
	assert.Equal(t, "vision", capErr.Feature)
}

// TestLLMCall_UsesFallbackChain_InProduction는 I3 결함 수정을 검증한다.
// NewLLMCall이 반환하는 LLMCallFunc가 FallbackModels를 실제로 사용하는지 검증한다.
// primary 실패 시 fallback model로 재시도해야 한다 (REQ-ADAPTER-008, AC-ADAPTER-009).
func TestLLMCall_UsesFallbackChain_InProduction(t *testing.T) {
	t.Parallel()

	callCount := 0
	// primaryModel 호출 시 에러, fallbackModel 호출 시 성공하는 provider
	fakeProvider := &fallbackTrackingProvider{
		name: "fake",
		streamFn: func(req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
			callCount++
			if req.Route.Model == "primary-model" {
				return nil, fmt.Errorf("primary model internal server error")
			}
			// fallback model 호출 시 성공
			ch := make(chan message.StreamEvent, 2)
			ch <- message.StreamEvent{Type: message.TypeTextDelta, Delta: "fallback ok"}
			ch <- message.StreamEvent{Type: message.TypeMessageStop}
			close(ch)
			return ch, nil
		},
	}

	reg := provider.NewRegistry()
	require.NoError(t, reg.Register(fakeProvider))

	pool := makeTestPool(t, "cred-1")
	tracker := ratelimit.NewTracker()
	planner := &cache.BreakpointPlanner{}
	logger, _ := zap.NewDevelopment()

	fn := provider.NewLLMCall(
		reg,
		pool,
		tracker,
		planner,
		cache.StrategyNone,
		cache.TTLEphemeral,
		nil,
		logger,
	)

	req := query.LLMCallReq{
		Route:          router.Route{Model: "primary-model", Provider: "fake"},
		FallbackModels: []string{"fallback-model"},
	}

	ch, err := fn(context.Background(), req)
	require.NoError(t, err, "fallback chain이 성공해야 함")

	var events []message.StreamEvent
	for evt := range ch {
		events = append(events, evt)
	}

	// fallback을 통해 응답을 받아야 한다
	assert.Equal(t, 2, callCount, "primary(1회) + fallback(1회) = 총 2회 호출")
	textDeltas := llmCallFilterByType(events, message.TypeTextDelta)
	require.Len(t, textDeltas, 1)
	assert.Equal(t, "fallback ok", textDeltas[0].Delta)
}

// fallbackTrackingProvider는 호출 model을 추적하는 테스트용 Provider이다.
type fallbackTrackingProvider struct {
	name     string
	streamFn func(req provider.CompletionRequest) (<-chan message.StreamEvent, error)
}

func (p *fallbackTrackingProvider) Name() string                        { return p.name }
func (p *fallbackTrackingProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *fallbackTrackingProvider) Stream(_ context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	return p.streamFn(req)
}
func (p *fallbackTrackingProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func llmCallFilterByType(evts []message.StreamEvent, typ string) []message.StreamEvent {
	var result []message.StreamEvent
	for _, e := range evts {
		if e.Type == typ {
			result = append(result, e)
		}
	}
	return result
}

// streamingStubProvider는 미리 정해진 이벤트를 방출하는 테스트용 Provider이다.
type streamingStubProvider struct {
	events []message.StreamEvent
}

func (s *streamingStubProvider) Name() string { return "streaming-stub" }

func (s *streamingStubProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Streaming: true}
}

func (s *streamingStubProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{}, nil
}

func (s *streamingStubProvider) Stream(_ context.Context, _ provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	ch := make(chan message.StreamEvent, len(s.events))
	for _, evt := range s.events {
		ch <- evt
	}
	close(ch)
	return ch, nil
}

// noVisionStubProvider는 Vision=false인 테스트용 Provider이다.
type noVisionStubProvider struct{}

func (p *noVisionStubProvider) Name() string { return "no-vision" }

func (p *noVisionStubProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		Streaming: true,
		Tools:     true,
		Vision:    false, // vision 미지원
	}
}

func (p *noVisionStubProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return nil, nil
}

func (p *noVisionStubProvider) Stream(_ context.Context, _ provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	ch := make(chan message.StreamEvent, 1)
	close(ch)
	return ch, nil
}
