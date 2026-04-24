// Package provider는 LLM provider 인터페이스와 요청/응답 타입을 정의한다.
// SPEC-GOOSE-ADAPTER-001 M1 T-010
package provider

import (
	"context"
	"net/http"

	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/tool"
)

// Capabilities는 LLM provider가 지원하는 기능 목록이다.
type Capabilities struct {
	// Streaming은 스트리밍 응답 지원 여부이다.
	Streaming bool
	// Tools는 function/tool calling 지원 여부이다.
	Tools bool
	// Vision은 이미지/비전 입력 지원 여부이다.
	Vision bool
	// Embed는 임베딩 생성 지원 여부이다.
	Embed bool
	// AdaptiveThinking은 Anthropic Opus 4.7 style Adaptive Thinking 지원 여부이다.
	AdaptiveThinking bool
	// MaxContextTokens는 최대 컨텍스트 토큰 수이다.
	MaxContextTokens int
	// MaxOutputTokens는 최대 출력 토큰 수이다.
	MaxOutputTokens int
}

// ThinkingConfig는 LLM thinking 모드 설정이다.
type ThinkingConfig struct {
	// Enabled는 thinking 모드 활성화 여부이다.
	Enabled bool
	// Effort는 thinking 노력 수준이다 ("low" | "medium" | "high" | "xhigh" | "max").
	// Anthropic Opus 4.7+ Adaptive Thinking에서 사용된다.
	Effort string
	// BudgetTokens는 non-adaptive 모델의 thinking 예산 토큰 수이다.
	BudgetTokens int
}

// VisionConfig는 비전/이미지 처리 설정이다.
type VisionConfig struct {
	// Enabled는 force-override로 비전 처리를 활성화/비활성화한다.
	Enabled bool
}

// RequestMetadata는 요청 메타데이터이다.
type RequestMetadata struct {
	// UserID는 사용자 식별자이다 (남용 추적용).
	UserID string
}

// UsageStats는 LLM 응답의 토큰 사용량이다.
type UsageStats struct {
	// InputTokens는 입력 토큰 수이다.
	InputTokens int
	// OutputTokens는 출력 토큰 수이다.
	OutputTokens int
	// CacheReadTokens는 캐시에서 읽은 토큰 수이다 (Anthropic prompt cache).
	CacheReadTokens int
	// CacheCreateTokens는 캐시에 새로 저장된 토큰 수이다.
	CacheCreateTokens int
}

// CompletionRequest는 LLM 완성 요청이다.
type CompletionRequest struct {
	// Route는 라우터가 결정한 경로이다.
	Route router.Route
	// Messages는 대화 메시지 목록이다.
	Messages []message.Message
	// Tools는 이번 호출에서 사용할 tool 정의 목록이다.
	Tools []tool.Definition
	// MaxOutputTokens는 최대 출력 토큰 수이다.
	MaxOutputTokens int
	// Temperature는 생성 다양성 파라미터이다.
	Temperature float64
	// Thinking은 thinking 모드 설정이다 (optional).
	Thinking *ThinkingConfig
	// Vision은 비전 처리 설정이다 (optional).
	Vision *VisionConfig
	// ResponseFormat은 응답 형식이다 ("" | "json").
	ResponseFormat string
	// FallbackModels는 primary 모델 실패 시 순서대로 시도할 모델 목록이다.
	FallbackModels []string
	// Metadata는 요청 메타데이터이다.
	Metadata RequestMetadata
}

// CompletionResponse는 LLM 완성 응답이다.
type CompletionResponse struct {
	// Message는 어시스턴트 응답 메시지이다.
	Message message.Message
	// StopReason은 생성 종료 이유이다 ("end_turn" | "tool_use" | "max_tokens").
	StopReason string
	// Usage는 토큰 사용량이다.
	Usage UsageStats
	// ResponseID는 응답 고유 ID이다.
	ResponseID string
	// RawHeaders는 원본 HTTP 응답 헤더이다 (RATELIMIT 전달용).
	RawHeaders http.Header
}

// Provider는 LLM 어댑터 인터페이스이다.
// 모든 provider 구현체는 이 인터페이스를 만족해야 한다.
// @MX:ANCHOR: [AUTO] Provider 인터페이스 — 모든 LLM provider의 공통 계약
// @MX:REASON: Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama 6개 provider가 이 인터페이스를 구현
type Provider interface {
	// Name은 provider 이름을 반환한다 (예: "anthropic").
	Name() string
	// Capabilities는 provider의 기능 목록을 반환한다.
	Capabilities() Capabilities
	// Complete는 blocking 방식으로 LLM 완성 응답을 반환한다.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	// Stream은 스트리밍 방식으로 LLM 응답 채널을 반환한다.
	Stream(ctx context.Context, req CompletionRequest) (<-chan message.StreamEvent, error)
}
