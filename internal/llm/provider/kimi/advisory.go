// Package kimi의 advisory.go: 장문 context 사용 시 관측용 INFO 로그.
// SPEC-GOOSE-ADAPTER-002 OI-3 (v0.3) — REQ-ADP2-022 / AC-ADP2-013.
package kimi

import (
	"strings"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/message"
	"go.uber.org/zap"
)

// longContextThreshold는 INFO 로그를 발생시키는 입력 token 임계치이다.
// AC-ADP2-013: 64K 초과 시 advisory 로그 1건.
const longContextThreshold int64 = 64 * 1024

// longContextModelMarker는 모델명에 포함되면 long-context advisory 평가 대상으로 간주되는 substring이다.
// 예: "moonshot-v1-128k", "kimi-moonshot-v1-128k" 등.
const longContextModelMarker = "128k"

// estimateInputTokens는 메시지 배열의 입력 token 수를 4-byte/char 휴리스틱으로 근사한다.
// SPEC-GOOSE-CONTEXT-001의 TokenCountWithEstimation과 동일 모델이지만,
// kimi 단독 의존성을 위해 inline 구현 (cross-package import 회피).
func estimateInputTokens(msgs []message.Message) int64 {
	var total int64
	for _, msg := range msgs {
		// role overhead: ~4 tokens per message
		total += 4
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				total += int64(len(block.Text)) / 4
			case "image":
				total += 765 // standard image token cost
			case "tool_use", "tool_result":
				total += int64(len(block.Text))/4 + 8
			}
		}
	}
	return total
}

// maybeLogLongContextAdvisory는 모델명이 long-context marker를 포함하고
// 입력 token이 64K를 초과하면 INFO 레벨 advisory 로그 1건을 발생시킨다.
// AC-ADP2-013: streaming 정상 동작에는 영향 없음 (관측 전용).
func maybeLogLongContextAdvisory(logger *zap.Logger, model string, msgs []message.Message) {
	if logger == nil {
		return
	}
	if !strings.Contains(strings.ToLower(model), longContextModelMarker) {
		return
	}
	tokens := estimateInputTokens(msgs)
	if tokens <= longContextThreshold {
		return
	}
	logger.Info("kimi.long_context_advisory",
		zap.String("model", model),
		zap.Int64("estimated_input_tokens", tokens),
		zap.Int64("threshold", longContextThreshold),
		zap.String("note", "long-context model in use; observational only (REQ-ADP2-022)"),
	)
}

// applyAdvisory는 CompletionRequest에서 모델명/메시지를 추출하여 advisory를 발생시킨다.
// 어댑터 Stream/Complete의 시작 지점에서 1회 호출한다.
func applyAdvisory(logger *zap.Logger, req provider.CompletionRequest) {
	maybeLogLongContextAdvisory(logger, req.Route.Model, req.Messages)
}
