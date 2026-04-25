// Package context는 QueryEngine의 context window 관리와 compaction 전략을 구현한다.
package context

import (
	"context"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query/loop"
)

// autoCompact는 AutoCompact 전략을 실행한다.
// REQ-CTX-018: Summarizer를 통해 메시지를 요약한다.
// REQ-CTX-014: Summarizer 에러 시 snip fallback.
func autoCompact(
	ctx context.Context,
	s loop.State,
	summarizer Summarizer,
	protectedHead, protectedTail int,
	targetTokens int64,
	logger loggerIface,
) ([]message.Message, int, bool, error) {
	// Summarizer 호출
	summary, err := summarizer.Summarize(ctx, s.Messages, targetTokens)
	if err != nil {
		// REQ-CTX-014: 에러 로그 후 snip fallback
		if logger != nil {
			logger.Errorf("AutoCompact: Summarizer.Summarize failed, falling back to Snip: %v", err)
		}
		result := snip(s.Messages, protectedHead, protectedTail)
		return result.messages, result.droppedThinkingCount, false, nil
	}

	// 요약 결과: [summary, ...ProtectedTail]
	var newMessages []message.Message
	newMessages = append(newMessages, summary)
	if protectedTail > 0 && len(s.Messages) > protectedTail {
		newMessages = append(newMessages, s.Messages[len(s.Messages)-protectedTail:]...)
	} else if len(s.Messages) <= protectedTail {
		newMessages = append(newMessages, s.Messages...)
	}

	return newMessages, 0, true, nil
}
