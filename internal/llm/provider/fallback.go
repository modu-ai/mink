package provider

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/message"
	"go.uber.org/zap"
)

// fallbackLogger는 fallback 로깅용 zap 로거이다. nil이면 로깅을 생략한다.
var fallbackLogger *zap.Logger

// TryWithFallback은 primary Provider로 Stream을 시도하고, 실패 시 req.FallbackModels를
// 순차적으로 시도한다. 모든 fallback 소진 시 원래 에러를 반환한다.
//
// fallback 전환은 사용자에게 노출되지 않는다 — 하나의 StreamEvent 시퀀스를 반환한다.
// HTTP 5xx / network error 시 fallback을 트리거한다.
//
// AC-ADAPTER-009: Fallback chain 검증.
func TryWithFallback(ctx context.Context, p Provider, req CompletionRequest) (<-chan message.StreamEvent, error) {
	// primary 시도
	ch, err := p.Stream(ctx, req)
	if err == nil {
		return ch, nil
	}

	originalErr := err

	// FallbackModels 순차 시도
	for _, fallbackModel := range req.FallbackModels {
		// model 교체 후 재시도
		fallbackReq := req
		fallbackReq.Route.Model = fallbackModel

		if fallbackLogger != nil {
			fallbackLogger.Info("fallback triggered",
				zap.String("primary", req.Route.Model),
				zap.String("using", fallbackModel),
				zap.Error(originalErr),
			)
		}

		ch, err = p.Stream(ctx, fallbackReq)
		if err == nil {
			return ch, nil
		}
	}

	// 모든 fallback 소진
	return nil, fmt.Errorf("fallback: all providers exhausted (original: %w)", originalErr)
}
