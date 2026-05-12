// Package context는 QueryEngine의 context window 관리와 compaction 전략을 구현한다.
package context

import (
	"context"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/query"
	"github.com/modu-ai/mink/internal/query/loop"
	"go.uber.org/zap"
)

// 전략 이름 상수
const (
	// StrategyAutoCompact는 AutoCompact 전략 이름이다.
	StrategyAutoCompact = "AutoCompact"
	// StrategyReactiveCompact는 ReactiveCompact 전략 이름이다.
	StrategyReactiveCompact = "ReactiveCompact"
	// StrategySnip은 Snip 전략 이름이다.
	StrategySnip = "Snip"
)

// loggerIface는 내부 로깅 인터페이스이다 (zap.Logger 추상화).
type loggerIface interface {
	Errorf(format string, args ...any)
}

// zapLoggerAdapter는 *zap.Logger를 loggerIface로 래핑한다.
type zapLoggerAdapter struct {
	logger *zap.Logger
}

func (z *zapLoggerAdapter) Errorf(format string, args ...any) {
	z.logger.Sugar().Errorf(format, args...)
}

// DefaultCompactor는 query.Compactor 인터페이스 구현체이다.
// SPEC-GOOSE-CONTEXT-001 §6.2 compactor.go
//
// 전략 우선순위: ReactiveCompact > AutoCompact > Snip (REQ-CTX-008)
//
// @MX:ANCHOR: [AUTO] query.Compactor 인터페이스 구현체
// @MX:REASON: SPEC-GOOSE-CONTEXT-001 REQ-CTX-008 - ShouldCompact/Compact 계약 구현, fan_in >= 3
type DefaultCompactor struct {
	// Summarizer는 LLM 요약 인터페이스이다. nil이면 Snip only.
	// REQ-CTX-012: nil이면 AutoCompact/ReactiveCompact 선택 불가.
	Summarizer Summarizer
	// ProtectedHead는 Snip 전략에서 보호하는 앞 메시지 수이다 (기본값 3).
	ProtectedHead int
	// ProtectedTail은 Snip 전략에서 보호하는 뒤 메시지 수이다 (기본값 5).
	ProtectedTail int
	// MaxMessageCount는 메시지 최대 허용 수이다 (기본값 500).
	// state.MaxMessageCount가 0이 아니면 state 값을 우선 사용.
	MaxMessageCount int
	// TokenLimit은 session token window 크기이다.
	// state.TokenLimit이 0이 아니면 state 값을 우선 사용.
	TokenLimit int64
	// HistorySnipOnly는 GOOSE_HISTORY_SNIP=1 feature gate이다.
	// REQ-CTX-016: true이면 Summarizer가 있어도 Snip만 선택.
	HistorySnipOnly bool
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// Ensure DefaultCompactor implements query.Compactor at compile time.
// compactor_contract_test.go에서도 검증.
var _ query.Compactor = (*DefaultCompactor)(nil)

// effectiveProtectedHead는 ProtectedHead의 유효값을 반환한다 (0이면 기본값 3).
func (c *DefaultCompactor) effectiveProtectedHead() int {
	if c.ProtectedHead <= 0 {
		return 3
	}
	return c.ProtectedHead
}

// effectiveProtectedTail는 ProtectedTail의 유효값을 반환한다 (0이면 기본값 5).
func (c *DefaultCompactor) effectiveProtectedTail() int {
	if c.ProtectedTail <= 0 {
		return 5
	}
	return c.ProtectedTail
}

// effectiveTokenLimit는 state 또는 compactor의 TokenLimit를 반환한다.
func (c *DefaultCompactor) effectiveTokenLimit(s loop.State) int64 {
	if s.TokenLimit > 0 {
		return s.TokenLimit
	}
	return c.TokenLimit
}

// effectiveMaxMessageCount는 state 또는 compactor의 MaxMessageCount를 반환한다.
func (c *DefaultCompactor) effectiveMaxMessageCount(s loop.State) int {
	if s.MaxMessageCount > 0 {
		return s.MaxMessageCount
	}
	if c.MaxMessageCount > 0 {
		return c.MaxMessageCount
	}
	return 500 // 기본값
}

// logger는 내부 loggerIface를 반환한다.
func (c *DefaultCompactor) logger() loggerIface {
	if c.Logger == nil {
		return nil
	}
	return &zapLoggerAdapter{logger: c.Logger}
}

// ShouldCompact는 현재 상태에서 compaction이 필요한지 판단한다.
// REQ-CTX-007: 80% 임계 OR ReactiveTriggered OR len(Messages) > MaxMessageCount
// REQ-CTX-011: Red level (>92%)이면 80% 임계 조건과 무관하게 true (Red 자체가 >92% > 80%이므로 동일)
//
// @MX:WARN: [AUTO] Red level override — token 사용률이 92% 초과이면 다른 조건 무관하게 compact 강제
// @MX:REASON: SPEC-GOOSE-CONTEXT-001 REQ-CTX-011 - context overflow 방지 안전망
func (c *DefaultCompactor) ShouldCompact(s loop.State) bool {
	// ReactiveTriggered는 무조건 compact
	if s.AutoCompactTracking.ReactiveTriggered {
		return true
	}

	// len(Messages) > MaxMessageCount 이면 compact
	maxMsgCount := c.effectiveMaxMessageCount(s)
	if len(s.Messages) > maxMsgCount {
		return true
	}

	// token 사용률 계산
	tokenLimit := c.effectiveTokenLimit(s)
	if tokenLimit <= 0 {
		return false
	}

	tokenCount := TokenCountWithEstimation(s.Messages)
	warningLevel := CalculateTokenWarningState(tokenCount, tokenLimit)

	// REQ-CTX-011: Red level이면 강제 true (Red는 >92% > 80%이므로 아래 80% 조건에 포함)
	if warningLevel >= WarningRed {
		return true
	}

	// REQ-CTX-007: tokenCount / tokenLimit >= 0.80 이면 true
	// 정수 연산: tokenCount*100 >= 80*tokenLimit
	return tokenCount*100 >= 80*tokenLimit
}

// Compact는 현재 상태를 압축하고 새 상태와 경계 정보를 반환한다.
// REQ-CTX-008: 전략 선택 우선순위 ReactiveCompact > AutoCompact > Snip
// REQ-CTX-010: 결과 State.TaskBudgetRemaining은 입력과 동일
// REQ-CTX-013: 결과 Messages 길이는 ProtectedTail+1 이상
func (c *DefaultCompactor) Compact(s loop.State) (loop.State, query.CompactBoundary, error) {
	protectedHead := c.effectiveProtectedHead()
	protectedTail := c.effectiveProtectedTail()
	tokenLimit := c.effectiveTokenLimit(s)

	tokensBefore := TokenCountWithEstimation(s.Messages)
	messagesBefore := len(s.Messages)

	strategy, newMessages, droppedThinkingCount := c.selectAndExecuteStrategy(
		context.Background(),
		s,
		protectedHead,
		protectedTail,
		tokenLimit,
	)

	// REQ-CTX-013: 결과 메시지 수 최소 길이 불변식
	if len(newMessages) == 0 {
		// 최소한 원본 메시지 반환
		newMessages = s.Messages
	}

	tokensAfter := TokenCountWithEstimation(newMessages)

	newState := s
	newState.Messages = newMessages
	// REQ-CTX-010: TaskBudgetRemaining은 변경하지 않음
	// (s를 복사하므로 이미 보존됨)

	boundary := query.CompactBoundary{
		Turn:                 s.TurnCount,
		Strategy:             strategy,
		MessagesBefore:       messagesBefore,
		MessagesAfter:        len(newMessages),
		TokensBefore:         tokensBefore,
		TokensAfter:          tokensAfter,
		TaskBudgetPreserved:  int64(s.TaskBudgetRemaining),
		DroppedThinkingCount: droppedThinkingCount,
	}

	return newState, boundary, nil
}

// selectAndExecuteStrategy는 전략을 선택하고 실행한다.
// 반환: (strategyName, newMessages, droppedThinkingCount)
func (c *DefaultCompactor) selectAndExecuteStrategy(
	ctx context.Context,
	s loop.State,
	protectedHead, protectedTail int,
	tokenLimit int64,
) (string, []message.Message, int) {
	log := c.logger()

	var targetTokens int64
	if tokenLimit > 0 {
		targetTokens = tokenLimit * 60 / 100 // 60% 목표
	} else {
		targetTokens = 50000 // fallback
	}

	// REQ-CTX-016: HistorySnipOnly이면 Snip 강제
	if c.HistorySnipOnly {
		result := snip(s.Messages, protectedHead, protectedTail)
		return StrategySnip, result.messages, result.droppedThinkingCount
	}

	// REQ-CTX-017: ReactiveTriggered이면 ReactiveCompact 최우선
	if s.AutoCompactTracking.ReactiveTriggered && c.Summarizer != nil {
		msgs, thinking, usedSummarizer, _ := reactiveCompact(ctx, s, c.Summarizer, protectedHead, protectedTail, targetTokens, log)
		_ = usedSummarizer
		strategy := StrategyReactiveCompact
		if !usedSummarizer {
			strategy = StrategySnip
		}
		return strategy, msgs, thinking
	}

	// ReactiveTriggered but Summarizer == nil → Snip
	if s.AutoCompactTracking.ReactiveTriggered && c.Summarizer == nil {
		result := snip(s.Messages, protectedHead, protectedTail)
		return StrategySnip, result.messages, result.droppedThinkingCount
	}

	// REQ-CTX-018: token >= 80% AND ReactiveTriggered==false → AutoCompact
	if c.Summarizer != nil && tokenLimit > 0 {
		tokenCount := TokenCountWithEstimation(s.Messages)
		if tokenCount*100 >= 80*tokenLimit {
			msgs, thinking, usedSummarizer, _ := autoCompact(ctx, s, c.Summarizer, protectedHead, protectedTail, targetTokens, log)
			_ = usedSummarizer
			strategy := StrategyAutoCompact
			if !usedSummarizer {
				strategy = StrategySnip
			}
			return strategy, msgs, thinking
		}
	}

	// MaxMessageCount 초과 but token < 80% → Summarizer 있어도 Snip
	// (AutoCompact는 token 비율 기반으로만 선택됨, MaxMessageCount 초과는 Snip)
	result := snip(s.Messages, protectedHead, protectedTail)
	return StrategySnip, result.messages, result.droppedThinkingCount
}
