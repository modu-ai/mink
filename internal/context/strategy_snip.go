// Package context는 QueryEngine의 context window 관리와 compaction 전략을 구현한다.
package context

import (
	"fmt"

	"github.com/modu-ai/goose/internal/message"
)

// snipResult는 Snip 전략의 결과이다.
type snipResult struct {
	// messages는 snip 후 메시지 배열이다.
	messages []message.Message
	// droppedCount는 삭제된 메시지 수이다.
	droppedCount int
	// droppedThinkingCount는 보존된 redacted_thinking 블록 수이다.
	droppedThinkingCount int
}

// snip은 Snip 전략을 실행한다.
// REQ-CTX-009: ProtectedHead 개 + snipMarker + ProtectedTail 개 구조.
// REQ-CTX-003: 삭제된 메시지의 redacted_thinking 블록은 snipMarker에 보존.
// REQ-CTX-013: 결과 메시지 수는 ProtectedTail+1 이상 (원본이 그보다 작으면 원본 반환).
//
// @MX:ANCHOR: [AUTO] Snip 전략 구현 - redacted_thinking 보존 불변식
// @MX:REASON: SPEC-GOOSE-CONTEXT-001 REQ-CTX-003/009/013 - 삭제 금지 + 최소 길이 불변식
func snip(messages []message.Message, protectedHead, protectedTail int) snipResult {
	minResult := protectedTail + 1 // snipMarker 포함

	// 원본 메시지가 최소 길이보다 작거나 같으면 snip 없이 반환
	if len(messages) <= protectedHead+protectedTail {
		return snipResult{
			messages:     messages,
			droppedCount: 0,
		}
	}

	head := messages[:protectedHead]
	tail := messages[len(messages)-protectedTail:]
	dropped := messages[protectedHead : len(messages)-protectedTail]

	// 삭제된 메시지에서 redacted_thinking 블록 수집
	var thinkingBlocks []message.ContentBlock
	for _, msg := range dropped {
		for _, block := range msg.Content {
			if block.Type == "redacted_thinking" {
				thinkingBlocks = append(thinkingBlocks, block)
			}
		}
	}

	// snip marker 생성
	markerContent := fmt.Sprintf("[%d messages omitted]", len(dropped))
	markerBlocks := []message.ContentBlock{
		{Type: "text", Text: markerContent},
	}
	// redacted_thinking 블록을 auxiliary content로 보존
	markerBlocks = append(markerBlocks, thinkingBlocks...)

	snipMarker := message.Message{
		Role:    "system",
		Content: markerBlocks,
	}

	var result []message.Message
	result = append(result, head...)
	result = append(result, snipMarker)
	result = append(result, tail...)

	// REQ-CTX-013: 결과가 최소 길이보다 작으면 불변식 위반 — 원본 반환
	if len(result) < minResult {
		return snipResult{
			messages:     messages,
			droppedCount: 0,
		}
	}

	return snipResult{
		messages:             result,
		droppedCount:         len(dropped),
		droppedThinkingCount: len(thinkingBlocks),
	}
}
