//go:build integration

// Package loop — SPEC-GOOSE-QUERY-001 S7 내부 함수 단위 테스트 (white-box).
// coverage 보강: after_compact 경로 — compactErr 무시, compact_boundary send 실패 경로.
package loop

import (
	"context"
	"fmt"
	"testing"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/permissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAllowDecision은 Allow를 반환하는 CanUseTool 스텁이다. (S7 internal test 전용)
type stubAllowDecision struct{}

func (s *stubAllowDecision) Check(_ context.Context, _ permissions.ToolPermissionContext) permissions.Decision {
	return permissions.Decision{Behavior: permissions.Allow}
}

// --- after_compact continue site 경로 커버 ---

// TestQueryLoop_CompactError_ContinuesWithOriginalState는 Compact가 에러를 반환할 때
// loop가 원래 state로 계속 진행하는지 검증한다.
// (compactErr != nil → 원래 state 유지 경로)
func TestQueryLoop_CompactError_ContinuesWithOriginalState(t *testing.T) {
	t.Parallel()

	out := make(chan message.SDKMessage, 32)

	// ShouldCompact는 첫 iteration에서만 true
	callCount := 0
	shouldCompact := func(_ State) bool {
		callCount++
		return callCount == 1
	}

	// Compact는 에러를 반환 — loop는 원래 state로 계속 진행해야 한다
	compact := func(state State) (State, message.PayloadCompactBoundary, error) {
		return state, message.PayloadCompactBoundary{}, fmt.Errorf("compact error")
	}

	// LLM은 즉시 stop 반환
	streamCh := make(chan message.StreamEvent, 2)
	streamCh <- message.StreamEvent{Type: message.TypeTextDelta, Delta: "ok"}
	streamCh <- message.StreamEvent{Type: message.TypeMessageStop, StopReason: "end_turn"}
	close(streamCh)

	callLLM := func(_ context.Context) (<-chan message.StreamEvent, error) {
		return streamCh, nil
	}

	cfg := LoopConfig{
		Out:           out,
		InitialState:  State{TaskBudgetRemaining: 10000},
		Prompt:        "test",
		MaxTurns:      10,
		PermInbox:     make(chan PermissionDecision, 1),
		CallLLM:       callLLM,
		CanUseTool:    &stubAllowDecision{},
		Execute:       func(_ context.Context, _, _ string, _ map[string]any) (string, error) { return "", nil },
		ShouldCompact: shouldCompact,
		Compact:       compact,
	}

	go queryLoop(context.Background(), cfg)

	// drain
	var msgs []message.SDKMessage
	for m := range out {
		msgs = append(msgs, m)
	}

	// compact_boundary는 yield되지 않아야 한다 (Compact 실패)
	var compactBoundaryCount int
	for _, m := range msgs {
		if m.Type == message.SDKMsgCompactBoundary {
			compactBoundaryCount++
		}
	}
	assert.Equal(t, 0, compactBoundaryCount, "Compact 실패 시 compact_boundary가 yield되면 안 된다")

	// terminal{success:true}: loop가 정상 진행되어야 한다
	require.NotEmpty(t, msgs)
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	term, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, term.Success, "compactErr 후에도 loop가 정상 종료되어야 한다")
}

// TestQueryLoop_CompactBoundarySendFail_Aborts는 compact_boundary send 실패(ctx 취소) 시
// loop가 종료됨을 검증한다. (compact_boundary send → ctx Done 경로)
func TestQueryLoop_CompactBoundarySendFail_Aborts(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// out은 user_ack 1개만 허용 (buf=1), 이후 ctx 취소로 send 실패
	out := make(chan message.SDKMessage, 1)

	callCount := 0
	shouldCompact := func(_ State) bool {
		callCount++
		if callCount == 1 {
			// ShouldCompact가 true를 반환하기 직전 ctx 취소
			// → compact_boundary send 시 ctx.Done() 선택
			cancel()
			return true
		}
		return false
	}

	compact := func(state State) (State, message.PayloadCompactBoundary, error) {
		return state, message.PayloadCompactBoundary{Turn: 1, MessagesBefore: 1, MessagesAfter: 1}, nil
	}

	streamCh := make(chan message.StreamEvent, 2)
	streamCh <- message.StreamEvent{Type: message.TypeTextDelta, Delta: "ok"}
	streamCh <- message.StreamEvent{Type: message.TypeMessageStop}
	close(streamCh)

	callLLM := func(_ context.Context) (<-chan message.StreamEvent, error) {
		return streamCh, nil
	}

	cfg := LoopConfig{
		Out:           out,
		InitialState:  State{TaskBudgetRemaining: 10000},
		Prompt:        "test",
		MaxTurns:      10,
		PermInbox:     make(chan PermissionDecision, 1),
		CallLLM:       callLLM,
		CanUseTool:    &stubAllowDecision{},
		Execute:       func(_ context.Context, _, _ string, _ map[string]any) (string, error) { return "", nil },
		ShouldCompact: shouldCompact,
		Compact:       compact,
	}

	go queryLoop(ctx, cfg)

	// drain — ctx 취소로 loop가 종료되어야 한다 (deadlock 없음)
	var msgs []message.SDKMessage
	for m := range out {
		msgs = append(msgs, m)
	}

	// loop가 user_ack 이후 abort — deadlock 없이 채널이 닫혀야 한다
	t.Logf("ctx 취소 후 수신된 메시지 수: %d", len(msgs))
}
