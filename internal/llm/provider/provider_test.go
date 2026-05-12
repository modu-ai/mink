package provider_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/router"
	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/tool"
	"github.com/stretchr/testify/assert"
)

// TestCapabilities_Fields는 Capabilities 구조체의 필드를 검증한다.
func TestCapabilities_Fields(t *testing.T) {
	t.Parallel()

	caps := provider.Capabilities{
		Streaming:        true,
		Tools:            true,
		Vision:           false,
		Embed:            false,
		AdaptiveThinking: true,
		MaxContextTokens: 200000,
		MaxOutputTokens:  8192,
	}

	assert.True(t, caps.Streaming)
	assert.True(t, caps.Tools)
	assert.False(t, caps.Vision)
	assert.True(t, caps.AdaptiveThinking)
	assert.Equal(t, 200000, caps.MaxContextTokens)
}

// TestCompletionRequest_Fields는 CompletionRequest 구조체를 검증한다.
func TestCompletionRequest_Fields(t *testing.T) {
	t.Parallel()

	req := provider.CompletionRequest{
		Route:           router.Route{Model: "claude-opus-4-7", Provider: "anthropic"},
		Messages:        []message.Message{{Role: "user"}},
		Tools:           []tool.Definition{{Name: "test"}},
		MaxOutputTokens: 1024,
		Temperature:     0.5,
		Thinking:        &provider.ThinkingConfig{Enabled: true, Effort: "high"},
		Vision:          &provider.VisionConfig{Enabled: true},
		ResponseFormat:  "json",
		FallbackModels:  []string{"claude-sonnet"},
		Metadata:        provider.RequestMetadata{UserID: "u-123"},
	}

	assert.Equal(t, "anthropic", req.Route.Provider)
	assert.Equal(t, 1024, req.MaxOutputTokens)
	assert.NotNil(t, req.Thinking)
	assert.Equal(t, "high", req.Thinking.Effort)
}

// TestCompletionResponse_Fields는 CompletionResponse 구조체를 검증한다.
func TestCompletionResponse_Fields(t *testing.T) {
	t.Parallel()

	resp := provider.CompletionResponse{
		Message:    message.Message{Role: "assistant"},
		StopReason: "end_turn",
		Usage: provider.UsageStats{
			InputTokens:       100,
			OutputTokens:      50,
			CacheReadTokens:   200,
			CacheCreateTokens: 10,
		},
		ResponseID: "resp-xyz",
		RawHeaders: http.Header{"X-Request-Id": []string{"req-123"}},
	}

	assert.Equal(t, "end_turn", resp.StopReason)
	assert.Equal(t, 100, resp.Usage.InputTokens)
	assert.Equal(t, "resp-xyz", resp.ResponseID)
}

// TestErrProviderNotFound_Error는 에러 메시지가 올바른지 검증한다.
func TestErrProviderNotFound_Error(t *testing.T) {
	t.Parallel()

	err := provider.ErrProviderNotFound{Name: "unknown-provider"}
	assert.Contains(t, err.Error(), "unknown-provider")
}

// TestErrCapabilityUnsupported_Error는 에러 메시지가 올바른지 검증한다.
func TestErrCapabilityUnsupported_Error(t *testing.T) {
	t.Parallel()

	err := provider.ErrCapabilityUnsupported{Feature: "vision", ProviderName: "deepseek"}
	assert.Contains(t, err.Error(), "vision")
	assert.Contains(t, err.Error(), "deepseek")
}

// TestProvider_Interface는 Provider 인터페이스를 구현하는 stub을 검증한다.
func TestProvider_Interface(t *testing.T) {
	t.Parallel()

	// stub이 Provider 인터페이스를 충족하는지 컴파일 시간 확인
	var _ provider.Provider = (*stubProvider)(nil)
}

// stubProvider는 테스트용 Provider 구현이다.
type stubProvider struct{}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Streaming: true}
}

func (s *stubProvider) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{}, nil
}

func (s *stubProvider) Stream(_ context.Context, _ provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	ch := make(chan message.StreamEvent)
	close(ch)
	return ch, nil
}
