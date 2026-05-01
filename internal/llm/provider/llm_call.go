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

		caps := p.Capabilities()

		// JSON mode fail-fast gate (REQ-AMEND-003, AC-AMEND-002).
		// Block before any HTTP call; use ProviderName (not Provider) per Correction 3.
		// @MX:NOTE: [AUTO] Capability gate — single-point JSON mode enforcement
		// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-003
		if req.ResponseFormat == "json" && !caps.JSONMode {
			return nil, ErrCapabilityUnsupported{Feature: "json_mode", ProviderName: p.Name()}
		}

		// Vision capability pre-check (REQ-ADAPTER-017, AC-ADAPTER-011)
		// Block before any HTTP call when provider does not support vision.
		if !caps.Vision && hasImageContent(req.Messages) {
			return nil, ErrCapabilityUnsupported{Feature: "vision", ProviderName: p.Name()}
		}

		// UserID silent drop gate (REQ-AMEND-004, AC-AMEND-008).
		// Operate on a copy of req — caller-owned struct must not be mutated (REQ-AMEND-011).
		// @MX:NOTE: [AUTO] UserID silent drop — reqCopy guards caller immutability
		// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-004
		reqCopy := req
		if reqCopy.Metadata.UserID != "" && !caps.UserID {
			if logger != nil {
				logger.Debug("user_id_dropped",
					zap.String("provider", p.Name()),
					zap.String("user_id_redacted", redactUserID(reqCopy.Metadata.UserID)),
				)
			}
			reqCopy.Metadata.UserID = ""
		}

		compReq := CompletionRequest{
			Route:           reqCopy.Route,
			Messages:        reqCopy.Messages,
			Tools:           reqCopy.Tools,
			MaxOutputTokens: reqCopy.MaxOutputTokens,
			Temperature:     reqCopy.Temperature,
			FallbackModels:  reqCopy.FallbackModels,
			ResponseFormat:  reqCopy.ResponseFormat,
			Metadata: RequestMetadata{
				UserID: reqCopy.Metadata.UserID,
			},
		}
		if reqCopy.Thinking != nil {
			compReq.Thinking = &ThinkingConfig{
				Enabled:      reqCopy.Thinking.Enabled,
				Effort:       reqCopy.Thinking.Effort,
				BudgetTokens: reqCopy.Thinking.BudgetTokens,
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

// redactUserID returns the first 4 characters of s followed by "..." to avoid
// logging personally identifying information at INFO level or higher (REQ-AMEND-010).
func redactUserID(s string) string {
	if len(s) <= 4 {
		return "..."
	}
	return s[:4] + "..."
}
