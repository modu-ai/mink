// Package query는 LLM 호출 요청/응답 타입과 함수 시그니처를 정의한다.
// SPEC-GOOSE-ADAPTER-001 M0 T-003
// SPEC-GOOSE-QUERY-001과의 인터페이스 경계이다.
package query

import (
	"context"

	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/tool"
)

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

// RequestMetadata holds optional per-request metadata fields.
// It mirrors provider.RequestMetadata to avoid an import cycle
// (provider already imports query; query must not import provider).
// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-011
type RequestMetadata struct {
	// UserID is an opaque end-user identifier for abuse tracking.
	// Zero value means no identifier. Forwarded to providers that support it;
	// silently dropped by the capability gate for providers that do not.
	UserID string
}

// LLMCallReq는 LLM 호출 요청이다.
// QUERY-001의 LLMCall 시그니처에서 사용된다.
type LLMCallReq struct {
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
	// FallbackModels는 primary 모델 실패 시 순서대로 시도할 모델 목록이다.
	FallbackModels []string
	// SystemHeader는 TeammateIdentity 주입을 위한 system 파트 구조화 헤더이다.
	// REQ-QUERY-020: nil이면 주입 없음.
	//
	// @MX:NOTE: [AUTO] TeammateIdentity system header 경로 - engine이 주입, LLMCallFunc 구현이 소비.
	SystemHeader map[string]any
	// ResponseFormat specifies the desired output format ("json" or "").
	// Zero value means no format constraint.
	// When set to "json", the capability gate in NewLLMCall checks JSONMode support;
	// providers without JSONMode return ErrCapabilityUnsupported.
	// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-003
	ResponseFormat string
	// Metadata carries optional per-request metadata (currently UserID).
	// Zero value is backward compatible — existing callers need no changes.
	// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-004
	Metadata RequestMetadata
}

// LLMCallFunc는 QUERY-001의 LLMCall 인터페이스 함수 타입이다.
// 이 타입은 QUERY-001 구현 시 흡수된다.
type LLMCallFunc = func(ctx context.Context, req LLMCallReq) (<-chan message.StreamEvent, error)
