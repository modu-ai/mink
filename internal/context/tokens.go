// Package context는 QueryEngine의 context window 관리와 compaction 전략을 구현한다.
// SPEC-GOOSE-CONTEXT-001
package context

import (
	"unicode/utf8"

	"github.com/modu-ai/mink/internal/message"
)

// WarningLevel은 token 사용률 경고 단계이다.
// SPEC-GOOSE-CONTEXT-001 §6.2 tokens.go
type WarningLevel int

const (
	// WarningGreen은 token 사용률 60% 미만 상태이다.
	WarningGreen WarningLevel = iota
	// WarningYellow는 token 사용률 60-80% 구간이다.
	WarningYellow
	// WarningOrange는 token 사용률 80-92% 구간이다.
	WarningOrange
	// WarningRed는 token 사용률 92% 초과 상태이다.
	WarningRed
)

// TokenCountWithEstimation은 메시지 배열의 token 수를 근사 계산한다.
// REQ-CTX-004: 동일 입력에 대해 결정적(deterministic)이다.
// 알고리즘: characters/4 + tool_use/tool_result overhead 근사 (SPEC-GOOSE-CONTEXT-001 §6.4)
//
// @MX:ANCHOR: [AUTO] Compactor.ShouldCompact의 핵심 판단 함수
// @MX:REASON: SPEC-GOOSE-CONTEXT-001 REQ-CTX-007 - 80% 임계 판단에 사용, fan_in >= 3
func TokenCountWithEstimation(messages []message.Message) int64 {
	var total int64
	for _, msg := range messages {
		total += estimateMessageTokens(msg)
	}
	return total
}

// estimateMessageTokens는 단일 메시지의 token 수를 근사 계산한다.
func estimateMessageTokens(msg message.Message) int64 {
	var tokens int64
	tokens += 4 // role/boundary overhead
	for _, block := range msg.Content {
		tokens += estimateBlockTokens(block)
	}
	return tokens
}

// estimateBlockTokens는 단일 ContentBlock의 token 수를 근사 계산한다.
func estimateBlockTokens(block message.ContentBlock) int64 {
	switch block.Type {
	case "text":
		return int64(utf8.RuneCountInString(block.Text)/4) + 1
	case "tool_use":
		// tool_use 블록당 12 토큰 overhead
		inputLen := int64(utf8.RuneCountInString(block.Text) / 4)
		return 12 + inputLen
	case "tool_result":
		return int64(utf8.RuneCountInString(block.ToolResultJSON)/4) + 1
	case "thinking", "redacted_thinking":
		// redacted_thinking: opaque 블록당 8 토큰 근사
		if block.Type == "redacted_thinking" {
			return 8
		}
		return int64(utf8.RuneCountInString(block.Thinking)/4) + 1
	default:
		return 4
	}
}

// CalculateTokenWarningState는 token 사용률을 기반으로 경고 단계를 반환한다.
// REQ-CTX-004: 결정적. 임계값:
//   - Green: < 60%
//   - Yellow: 60% ~ 80%
//   - Orange: 80% ~ 92%
//   - Red: > 92%
//
// @MX:ANCHOR: [AUTO] ShouldCompact의 Red-level override 판단 함수
// @MX:REASON: SPEC-GOOSE-CONTEXT-001 REQ-CTX-011 - Red level에서 ShouldCompact 강제 true
func CalculateTokenWarningState(used, limit int64) WarningLevel {
	if limit <= 0 {
		return WarningGreen
	}
	// 정수 연산: 임계값을 100배 스케일로 비교하여 부동소수점 없이 정확한 백분율 판정.
	// used/limit >= threshold ↔ used * 100 >= threshold * limit
	// Red: > 92% → used*100 > 92*limit
	if used*100 > 92*limit {
		return WarningRed
	}
	// Orange: >= 80%
	if used*100 >= 80*limit {
		return WarningOrange
	}
	// Yellow: >= 60%
	if used*100 >= 60*limit {
		return WarningYellow
	}
	return WarningGreen
}
