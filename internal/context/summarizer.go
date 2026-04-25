// Package context는 QueryEngine의 context window 관리와 compaction 전략을 구현한다.
package context

import (
	"context"

	"github.com/modu-ai/goose/internal/message"
)

// Summarizer는 메시지를 LLM으로 요약하는 인터페이스이다.
// SPEC-GOOSE-CONTEXT-001 §6.2 summarizer.go
// 실제 구현은 SPEC-GOOSE-COMPRESSOR-001이 제공한다. 본 SPEC은 consumer.
type Summarizer interface {
	// Summarize는 메시지 목록을 요약한 단일 메시지를 반환한다.
	// 반환되는 메시지는 role:"system", content에 요약 텍스트 포함.
	Summarize(
		ctx context.Context,
		messages []message.Message,
		targetTokens int64,
	) (message.Message, error)
}
