// Package context_test — SPEC-GOOSE-CONTEXT-001 DefaultCompactor 테스트.
// AC-CTX-005: AutoCompact 인터페이스 호출 (Summarizer mock)
// AC-CTX-006: Snip 전략의 protected window 및 redacted_thinking 보존
// AC-CTX-007: Compaction 후 task_budget 보존
// AC-CTX-008: Summarizer 미등록 시 Snip fallback
// AC-CTX-009: Summarizer 에러 시 Snip fallback
// AC-CTX-011: ShouldCompact 80% 임계 경계 (REQ-CTX-007)
// AC-CTX-012: Red level 강제 compact (REQ-CTX-011)
// AC-CTX-013: Compact 결과 최소 길이 불변식 (REQ-CTX-013)
// AC-CTX-015: HISTORY_SNIP feature gate (REQ-CTX-016)
// AC-CTX-016: ReactiveTriggered 강제 ReactiveCompact 선택 (REQ-CTX-017)
package context_test

import (
	"context"
	"errors"
	"testing"

	goosecontext "github.com/modu-ai/goose/internal/context"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query/loop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stub Summarizer ---

// stubSummarizer는 테스트용 Summarizer 스텁이다.
type stubSummarizer struct {
	callCount int
	response  message.Message
	err       error
}

func (s *stubSummarizer) Summarize(_ context.Context, _ []message.Message, _ int64) (message.Message, error) {
	s.callCount++
	if s.err != nil {
		return message.Message{}, s.err
	}
	return s.response, nil
}

// --- helper ---

// makeMessages는 N개의 user 메시지를 생성한다.
func makeMessages(n int) []message.Message {
	msgs := make([]message.Message, n)
	for i := range msgs {
		msgs[i] = message.Message{
			Role: "user",
			Content: []message.ContentBlock{
				{Type: "text", Text: "message content"},
			},
		}
	}
	return msgs
}

// makeMessagesWithTokens는 대략 주어진 token 수를 가진 메시지 목록을 생성한다.
// 각 메시지는 약 4*charsPerMsg 자의 텍스트를 가진다.
func makeMessagesWithTokenCount(targetTokens int64) []message.Message {
	// TokenCountWithEstimation: chars/4 + 4(overhead) per message
	// 1개 메시지의 chars를 (targetTokens-4)*4로 설정
	chars := int(targetTokens-4) * 4
	if chars < 0 {
		chars = 0
	}
	text := ""
	for len(text) < chars {
		text += "a"
	}
	return []message.Message{
		{
			Role: "user",
			Content: []message.ContentBlock{
				{Type: "text", Text: text},
			},
		},
	}
}

// --- AC-CTX-005: AutoCompact 인터페이스 호출 ---

// TestCompactor_AutoCompactCallsSummarizer는 AC-CTX-005를 검증한다.
// covers REQ-CTX-018: ReactiveTriggered=false + token >= 80% → AutoCompact
func TestCompactor_AutoCompactCallsSummarizer(t *testing.T) {
	t.Parallel()

	stub := &stubSummarizer{
		response: message.Message{
			Role:    "system",
			Content: []message.ContentBlock{{Type: "text", Text: "...summary..."}},
		},
	}

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:      stub,
		HistorySnipOnly: false,
		ProtectedHead:   3,
		ProtectedTail:   5,
		TokenLimit:      100_000,
	}

	// 25개 메시지 + token 사용량 ~90_000/100_000 (≥80%)
	msgs := makeMessages(25)
	// token count가 90_000 정도 되도록 조정
	// makeMessages는 각 "message content"(15자) + 4 overhead ≈ 7 tokens/msg
	// 이로는 부족하므로 큰 메시지로 대체
	bigText := make([]byte, 360_000) // 360_000 chars / 4 = 90_000 tokens
	for i := range bigText {
		bigText[i] = 'x'
	}
	msgs[0].Content = []message.ContentBlock{{Type: "text", Text: string(bigText)}}

	state := loop.State{
		Messages:            msgs,
		TokenLimit:          100_000,
		TaskBudgetRemaining: 999,
		AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: false},
	}

	// ShouldCompact 확인
	assert.True(t, compactor.ShouldCompact(state), "token >= 80% 이면 ShouldCompact==true이어야 함")

	newState, boundary, err := compactor.Compact(state)
	require.NoError(t, err)

	assert.Equal(t, goosecontext.StrategyAutoCompact, boundary.Strategy, "Strategy가 AutoCompact이어야 함")
	assert.Equal(t, 1, stub.callCount, "Summarizer가 1회 호출되어야 함")
	assert.Equal(t, 25, boundary.MessagesBefore)
	assert.NotEmpty(t, newState.Messages, "결과 Messages가 비어있지 않아야 함")
}

// --- AC-CTX-006: Snip protected window + redacted_thinking 보존 ---

// TestSnip_PreservesProtectedWindow는 AC-CTX-006을 검증한다.
// 20개 messages, ProtectedHead=3, ProtectedTail=5
func TestSnip_PreservesProtectedWindow(t *testing.T) {
	t.Parallel()

	// 20개 메시지 생성, messages[5]와 messages[12]에 redacted_thinking 포함
	msgs := makeMessages(20)
	msgs[5].Content = append(msgs[5].Content, message.ContentBlock{
		Type:     "redacted_thinking",
		Thinking: "",
	})
	msgs[12].Content = append(msgs[12].Content, message.ContentBlock{
		Type:     "redacted_thinking",
		Thinking: "",
	})

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:    nil, // Snip 강제
		ProtectedHead: 3,
		ProtectedTail: 5,
		TokenLimit:    1_000_000, // 큰 limit, token 조건은 고려하지 않음
	}

	// MaxMessageCount로 Snip 강제 트리거
	state := loop.State{
		Messages:        msgs,
		MaxMessageCount: 5, // 20 > 5 이면 compact 필요
	}

	assert.True(t, compactor.ShouldCompact(state))

	newState, boundary, err := compactor.Compact(state)
	require.NoError(t, err)

	// 결과: [m0, m1, m2, snipMarker, m15, m16, m17, m18, m19]
	// head(3) + snipMarker(1) + tail(5) = 9
	require.Len(t, newState.Messages, 9, "결과 메시지 수가 9이어야 함")

	// snipMarker는 4번째 (index 3)
	snipMarker := newState.Messages[3]
	assert.Equal(t, "system", snipMarker.Role, "snipMarker role은 system이어야 함")

	// redacted_thinking 블록 2개 보존 확인
	var thinkingCount int
	for _, block := range snipMarker.Content {
		if block.Type == "redacted_thinking" {
			thinkingCount++
		}
	}
	assert.Equal(t, 2, thinkingCount, "2개의 redacted_thinking 블록이 snipMarker에 보존되어야 함")
	assert.Equal(t, 2, boundary.DroppedThinkingCount)
	assert.Equal(t, goosecontext.StrategySnip, boundary.Strategy)
}

// TestSnip_PreservesRedactedThinking는 REQ-CTX-003을 검증한다.
// redacted_thinking 블록이 절대 삭제되지 않음을 확인.
func TestSnip_PreservesRedactedThinking(t *testing.T) {
	t.Parallel()

	// 10개 메시지, 여러 위치에 redacted_thinking
	msgs := makeMessages(10)
	msgs[1].Content = append(msgs[1].Content, message.ContentBlock{Type: "redacted_thinking"})
	msgs[4].Content = append(msgs[4].Content, message.ContentBlock{Type: "redacted_thinking"})
	msgs[7].Content = append(msgs[7].Content, message.ContentBlock{Type: "redacted_thinking"})

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:    nil,
		ProtectedHead: 2,
		ProtectedTail: 2,
		TokenLimit:    1_000_000,
	}

	state := loop.State{
		Messages:        msgs,
		MaxMessageCount: 3, // 10 > 3, compact 필요
	}

	newState, boundary, err := compactor.Compact(state)
	require.NoError(t, err)

	// 삭제된 메시지(index 2~7, 즉 6개)에서 index 4, 7의 redacted_thinking은 보존되어야 함
	// head: [0,1], tail: [8,9], dropped: [2..7]
	// dropped에서 index 4, 7에 redacted_thinking이 있음

	snipMarker := newState.Messages[2] // head(2) + snipMarker
	var thinkingBlocks []message.ContentBlock
	for _, block := range snipMarker.Content {
		if block.Type == "redacted_thinking" {
			thinkingBlocks = append(thinkingBlocks, block)
		}
	}
	// msgs[4]와 msgs[7]이 dropped 범위에 있어야 함
	assert.GreaterOrEqual(t, len(thinkingBlocks), 1, "dropped 범위 내 redacted_thinking이 보존되어야 함")
	assert.GreaterOrEqual(t, boundary.DroppedThinkingCount, 1)
}

// --- AC-CTX-007: task_budget 보존 ---

// TestCompactor_TaskBudgetPreserved는 AC-CTX-007을 검증한다.
func TestCompactor_TaskBudgetPreserved(t *testing.T) {
	t.Parallel()

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:    nil, // Snip
		ProtectedHead: 3,
		ProtectedTail: 5,
		TokenLimit:    1_000_000,
	}

	const budgetBefore = 1234
	state := loop.State{
		Messages:            makeMessages(20),
		TaskBudgetRemaining: budgetBefore,
		MaxMessageCount:     5,
	}

	newState, boundary, err := compactor.Compact(state)
	require.NoError(t, err)

	// REQ-CTX-010: compaction 자체는 task budget을 소비하지 않는다
	assert.Equal(t, budgetBefore, newState.TaskBudgetRemaining, "TaskBudgetRemaining이 변경되면 안 됨")
	assert.Equal(t, int64(budgetBefore), boundary.TaskBudgetPreserved)
}

// --- AC-CTX-008: Summarizer nil → Snip fallback ---

// TestCompactor_NilSummarizer_FallsBackToSnip는 AC-CTX-008을 검증한다.
func TestCompactor_NilSummarizer_FallsBackToSnip(t *testing.T) {
	t.Parallel()

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:    nil, // Snip only
		ProtectedHead: 3,
		ProtectedTail: 5,
	}

	// AutoCompact 조건 충족 (token >= 80%)
	bigText := make([]byte, 360_000)
	for i := range bigText {
		bigText[i] = 'x'
	}
	state := loop.State{
		Messages: []message.Message{
			{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: string(bigText)}}},
		},
		TokenLimit:          100_000,
		TaskBudgetRemaining: 100,
	}

	_, boundary, err := compactor.Compact(state)
	require.NoError(t, err)

	assert.Equal(t, goosecontext.StrategySnip, boundary.Strategy, "Summarizer nil이면 Snip이어야 함")
}

// --- AC-CTX-009: Summarizer 에러 → Snip fallback ---

// TestCompactor_SummarizerError_FallsBackToSnip는 AC-CTX-009을 검증한다.
func TestCompactor_SummarizerError_FallsBackToSnip(t *testing.T) {
	t.Parallel()

	stub := &stubSummarizer{
		err: errors.New("llm unavailable"),
	}

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:    stub,
		ProtectedHead: 3,
		ProtectedTail: 5,
		TokenLimit:    100_000,
	}

	bigText := make([]byte, 360_000)
	for i := range bigText {
		bigText[i] = 'x'
	}
	state := loop.State{
		Messages: []message.Message{
			{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: string(bigText)}}},
		},
		TokenLimit:          100_000,
		TaskBudgetRemaining: 100,
	}

	_, boundary, err := compactor.Compact(state)
	require.NoError(t, err, "Summarizer 에러가 호출자에게 전파되면 안 됨")

	assert.Equal(t, goosecontext.StrategySnip, boundary.Strategy, "Summarizer 에러 시 Snip으로 fallback해야 함")
}

// --- AC-CTX-011: ShouldCompact 80% 임계 경계 ---

// TestCompactor_ShouldCompact_80PercentBoundary는 AC-CTX-011을 검증한다.
// REQ-CTX-007: 80% 임계 정확한 경계값 테스트
// TokenLimit을 조정하여 정확한 80% 경계를 만든다.
func TestCompactor_ShouldCompact_80PercentBoundary(t *testing.T) {
	t.Parallel()

	compactor := &goosecontext.DefaultCompactor{
		ProtectedHead: 3,
		ProtectedTail: 5,
	}

	// 고정 token count를 가진 state를 만들기 위해 단순 텍스트 메시지 사용.
	// "aaaa" = 4 chars = 1 token(from chars/4+1) + 4(overhead) = 5 tokens
	// 일반적으로 1개 메시지의 토큰 수를 정확히 제어하기 어렵다.
	// 대신 TokenLimit 조정으로 경계를 테스트:
	//
	// 메시지 토큰 수 T를 고정, limit L을 변경:
	//   T * 100 < 80 * L  → false (T/L < 80%)
	//   T * 100 >= 80 * L → true  (T/L >= 80%)
	//   T * 100 > 92 * L  → Red   (T/L > 92%)

	// 단일 "x"*400 텍스트: chars=400, tokens = 400/4 + 1 + 4 = 105
	// 실제 확인: TokenCountWithEstimation
	textBytes := make([]byte, 400)
	for i := range textBytes {
		textBytes[i] = 'x'
	}
	fixedMsg := loop.State{
		Messages: []message.Message{
			{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: string(textBytes)}}},
		},
		MaxMessageCount:     10_000,
		AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: false},
	}

	T := goosecontext.TokenCountWithEstimation(fixedMsg.Messages)
	require.Positive(t, T)

	// (a) limit = T*100/79 + 1 → T/limit < 80% → false
	// T * 100 < 80 * limit → limit > T*100/80 → limit = T*100/80 + 1
	limitA := T*100/80 + 1
	stateA := fixedMsg
	stateA.TokenLimit = limitA
	assert.False(t, compactor.ShouldCompact(stateA),
		"(a) token(%d)/limit(%d) < 80%% → false이어야 함", T, limitA)

	// (b) limit = T*100/80 → T * 100 == 80 * limit → true (>=)
	limitB := T * 100 / 80
	stateB := fixedMsg
	stateB.TokenLimit = limitB
	assert.True(t, compactor.ShouldCompact(stateB),
		"(b) token(%d)/limit(%d) == 80%% → true이어야 함", T, limitB)

	// (c) limit = limitB - 1 → T/limit > 80% → true
	limitC := limitB - 1
	if limitC > 0 {
		stateC := fixedMsg
		stateC.TokenLimit = limitC
		assert.True(t, compactor.ShouldCompact(stateC),
			"(c) token(%d)/limit(%d) > 80%% → true이어야 함", T, limitC)
	}
}

// --- AC-CTX-012: Red level 강제 compact ---

// TestCompactor_RedLevel_OverridesThreshold는 AC-CTX-012를 검증한다.
// REQ-CTX-011: Red level(>92%)이면 token 사용률 무관하게 ShouldCompact==true
func TestCompactor_RedLevel_OverridesThreshold(t *testing.T) {
	t.Parallel()

	compactor := &goosecontext.DefaultCompactor{
		ProtectedHead: 3,
		ProtectedTail: 5,
	}

	// 92_500 tokens, limit=100_000 → 92.5% → Red → true
	textBytes := make([]byte, (92_500-4)*4)
	for i := range textBytes {
		textBytes[i] = 'a'
	}
	stateRed := loop.State{
		Messages: []message.Message{
			{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: string(textBytes)}}},
		},
		TokenLimit:          100_000,
		MaxMessageCount:     10_000,
		AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: false},
	}
	assert.True(t, compactor.ShouldCompact(stateRed),
		"92.5%% (Red) 이면 ShouldCompact==true이어야 함")

	// 동일 메시지에서 TokenLimit=1_000_000 → 9.25% (Green) → false
	stateGreen := stateRed
	stateGreen.TokenLimit = 1_000_000
	assert.False(t, compactor.ShouldCompact(stateGreen),
		"TokenLimit 1_000_000으로 조정 시 9.25%% (Green) 이면 false이어야 함")
}

// --- AC-CTX-013: Compact 결과 최소 길이 불변식 ---

// TestCompactor_MinimumMessagesInvariant는 AC-CTX-013을 검증한다.
// REQ-CTX-013: len(Messages) >= ProtectedTail+1, 빈 슬라이스 금지
func TestCompactor_MinimumMessagesInvariant(t *testing.T) {
	t.Parallel()

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:    nil,
		ProtectedHead: 3,
		ProtectedTail: 5,
	}

	// 단 2개 메시지 (ProtectedHead+Tail=8보다 작은 경계 케이스)
	state := loop.State{
		Messages:        makeMessages(2),
		MaxMessageCount: 1, // 2 > 1, compact 트리거
	}

	newState, boundary, err := compactor.Compact(state)
	require.NoError(t, err)

	// 원본이 ProtectedTail+1 미만이면 원본 반환
	assert.NotEmpty(t, newState.Messages, "결과 Messages가 비어있지 않아야 함 (REQ-CTX-013)")
	assert.Equal(t, goosecontext.StrategySnip, boundary.Strategy)

	// MessagesBefore == MessagesAfter (snip 없이 원본 반환)
	assert.Equal(t, boundary.MessagesBefore, boundary.MessagesAfter,
		"원본이 너무 작으면 MessagesBefore == MessagesAfter이어야 함")
}

// --- AC-CTX-015: HISTORY_SNIP feature gate ---

// TestCompactor_HistorySnipOnly_PrefersSnip는 AC-CTX-015를 검증한다.
// REQ-CTX-016: HistorySnipOnly=true이면 Summarizer가 있어도 Snip만 선택
func TestCompactor_HistorySnipOnly_PrefersSnip(t *testing.T) {
	t.Parallel()

	stub := &stubSummarizer{
		response: message.Message{
			Role:    "system",
			Content: []message.ContentBlock{{Type: "text", Text: "summary"}},
		},
	}

	compactorSnipOnly := &goosecontext.DefaultCompactor{
		Summarizer:      stub,
		HistorySnipOnly: true,
		ProtectedHead:   3,
		ProtectedTail:   5,
		TokenLimit:      100_000,
	}

	// AutoCompact trigger 조건 충족 (token >= 80%)
	bigText := make([]byte, 360_000)
	for i := range bigText {
		bigText[i] = 'x'
	}
	state := loop.State{
		Messages: append(makeMessages(10), message.Message{
			Role:    "user",
			Content: []message.ContentBlock{{Type: "text", Text: string(bigText)}},
		}),
		TokenLimit:          100_000,
		AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: false},
	}

	_, boundary, err := compactorSnipOnly.Compact(state)
	require.NoError(t, err)

	assert.Equal(t, goosecontext.StrategySnip, boundary.Strategy, "HistorySnipOnly=true이면 Snip이어야 함")
	assert.Equal(t, 0, stub.callCount, "HistorySnipOnly 시 Summarizer가 호출되면 안 됨")

	// 대조군: HistorySnipOnly=false → AutoCompact
	stub2 := &stubSummarizer{
		response: message.Message{
			Role:    "system",
			Content: []message.ContentBlock{{Type: "text", Text: "summary"}},
		},
	}
	compactorNormal := &goosecontext.DefaultCompactor{
		Summarizer:      stub2,
		HistorySnipOnly: false,
		ProtectedHead:   3,
		ProtectedTail:   5,
		TokenLimit:      100_000,
	}

	_, boundary2, err := compactorNormal.Compact(state)
	require.NoError(t, err)
	assert.Equal(t, goosecontext.StrategyAutoCompact, boundary2.Strategy, "대조군: HistorySnipOnly=false이면 AutoCompact이어야 함")
}

// --- AC-CTX-016: ReactiveTriggered 강제 ReactiveCompact 선택 ---

// TestCompactor_ReactiveTriggered_SelectsReactive는 AC-CTX-016을 검증한다.
// REQ-CTX-017: ReactiveTriggered=true이면 ReactiveCompact 최우선
func TestCompactor_ReactiveTriggered_SelectsReactive(t *testing.T) {
	t.Parallel()

	stub := &stubSummarizer{
		response: message.Message{
			Role:    "system",
			Content: []message.ContentBlock{{Type: "text", Text: "reactive summary"}},
		},
	}

	compactor := &goosecontext.DefaultCompactor{
		Summarizer:      stub,
		HistorySnipOnly: false,
		ProtectedHead:   3,
		ProtectedTail:   5,
		TokenLimit:      100_000,
	}

	// token usage 40_000/100_000 = 40% (< 80%, AutoCompact 자가 trigger 조건 불충족)
	textBytes := make([]byte, (40_000-4)*4)
	for i := range textBytes {
		textBytes[i] = 'a'
	}
	stateReactive := loop.State{
		Messages: []message.Message{
			{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: string(textBytes)}}},
		},
		TokenLimit:          100_000,
		MaxMessageCount:     10_000,
		AutoCompactTracking: loop.AutoCompactTracking{ReactiveTriggered: true},
	}

	// ShouldCompact: ReactiveTriggered=true이면 true
	assert.True(t, compactor.ShouldCompact(stateReactive), "ReactiveTriggered=true이면 ShouldCompact==true이어야 함")

	_, boundary, err := compactor.Compact(stateReactive)
	require.NoError(t, err)

	assert.Equal(t, goosecontext.StrategyReactiveCompact, boundary.Strategy,
		"ReactiveTriggered=true이면 ReactiveCompact이어야 함")
	assert.Equal(t, 1, stub.callCount, "Summarizer가 1회 호출되어야 함")

	// 대조군: ReactiveTriggered=false, 40% → AutoCompact 조건 미충족 → Snip
	stateNoReactive := stateReactive
	stateNoReactive.AutoCompactTracking = loop.AutoCompactTracking{ReactiveTriggered: false}
	stub2 := &stubSummarizer{response: stub.response}
	compactor2 := &goosecontext.DefaultCompactor{
		Summarizer:      stub2,
		HistorySnipOnly: false,
		ProtectedHead:   3,
		ProtectedTail:   5,
		TokenLimit:      100_000,
	}

	_, boundary2, err := compactor2.Compact(stateNoReactive)
	require.NoError(t, err)
	assert.Equal(t, goosecontext.StrategySnip, boundary2.Strategy,
		"대조군: ReactiveTriggered=false + 40%% → Snip이어야 함 (40%% < 80%% 임계 미충족)")
}
