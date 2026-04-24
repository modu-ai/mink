package provider

import (
	"context"

	"github.com/modu-ai/goose/internal/llm/cache"
	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"go.uber.org/zap"
)

// NewLLMCall은 QUERY-001의 LLMCallFunc 시그니처를 구현한 함수를 반환한다.
// 이 함수는 Route를 통해 provider를 조회하고 Provider.Stream을 호출한다.
//
// 파라미터:
//   - registry: Provider 인스턴스 레지스트리
//   - pool: credential pool (현재 사용하지 않음 — 어댑터 내부에서 처리)
//   - tracker: rate limit tracker (현재 사용하지 않음 — 어댑터 내부에서 처리)
//   - cachePlanner: 캐시 계획자 (현재 사용하지 않음 — 어댑터 내부에서 처리)
//   - cacheStrategy: 캐시 전략
//   - cacheTTL: 캐시 TTL
//   - secretStore: secret 저장소 (현재 사용하지 않음 — 어댑터 내부에서 처리)
//   - logger: 구조화 로거
//
// @MX:ANCHOR: [AUTO] NewLLMCall — QUERY-001 경계의 LLMCallFunc 생성자
// @MX:REASON: QUERY-001 QueryEngine이 이 함수를 통해 LLM 호출을 위임함
func NewLLMCall(
	registry *ProviderRegistry,
	_ *credential.CredentialPool,
	_ *ratelimit.Tracker,
	_ *cache.BreakpointPlanner,
	_ cache.CacheStrategy,
	_ cache.TTL,
	_ SecretStore,
	logger *zap.Logger,
) query.LLMCallFunc {
	return func(ctx context.Context, req query.LLMCallReq) (<-chan message.StreamEvent, error) {
		p, ok := registry.Get(req.Route.Provider)
		if !ok {
			return nil, ErrProviderNotFound{Name: req.Route.Provider}
		}

		// 로깅: PII 미포함 구조화 필드만 (REQ-ADAPTER-014)
		if logger != nil {
			logger.Debug("llm_call dispatch",
				zap.String("provider", req.Route.Provider),
				zap.String("model", req.Route.Model),
				zap.Int("message_count", len(req.Messages)),
			)
		}

		// Vision capability pre-check (REQ-ADAPTER-017, AC-ADAPTER-011)
		// 이미지 ContentBlock이 포함된 요청인데 provider가 Vision을 지원하지 않으면 즉시 에러 반환.
		if !p.Capabilities().Vision && hasImageContent(req.Messages) {
			return nil, ErrCapabilityUnsupported{Feature: "vision", ProviderName: p.Name()}
		}

		compReq := CompletionRequest{
			Route:           req.Route,
			Messages:        req.Messages,
			Tools:           req.Tools,
			MaxOutputTokens: req.MaxOutputTokens,
			Temperature:     req.Temperature,
			FallbackModels:  req.FallbackModels,
		}
		if req.Thinking != nil {
			compReq.Thinking = &ThinkingConfig{
				Enabled:      req.Thinking.Enabled,
				Effort:       req.Thinking.Effort,
				BudgetTokens: req.Thinking.BudgetTokens,
			}
		}

		// fallback chain 호출 (REQ-ADAPTER-008, AC-ADAPTER-009).
		// req.FallbackModels가 비어있으면 TryWithFallback은 primary만 호출한다.
		return TryWithFallback(ctx, p, compReq)
	}
}

// hasImageContent는 메시지 목록에 이미지 ContentBlock이 있는지 확인한다.
// Vision capability pre-check에 사용된다.
func hasImageContent(msgs []message.Message) bool {
	for _, m := range msgs {
		for _, block := range m.Content {
			if block.Type == "image" {
				return true
			}
		}
	}
	return false
}
