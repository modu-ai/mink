package message_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamEvent_DeltaOrdering은 StreamEvent 슬라이스의 순서가
// append 순서대로 보존되는지 검증한다.
// plan.md T1.3 / REQ-QUERY-002
func TestStreamEvent_DeltaOrdering(t *testing.T) {
	t.Parallel()

	deltas := []string{"Hello", " ", "World", "!"}
	events := make([]message.StreamEvent, 0, len(deltas))

	for _, d := range deltas {
		events = append(events, message.StreamEvent{
			Type:  message.TypeTextDelta,
			Delta: d,
		})
	}

	require.Len(t, events, 4)
	for i, d := range deltas {
		assert.Equal(t, d, events[i].Delta, "index %d의 delta가 일치해야 함", i)
		assert.Equal(t, message.TypeTextDelta, events[i].Type)
	}
}

// TestStreamEvent_DeltaOrdering_ForwardOnly는 ForwardStreamEvents가
// 입력 순서대로 이벤트를 전달하는지 검증한다.
// plan.md T1.3 / REQ-QUERY-002
func TestStreamEvent_DeltaOrdering_ForwardOnly(t *testing.T) {
	t.Parallel()

	input := []message.StreamEvent{
		{Type: message.TypeTextDelta, Delta: "first"},
		{Type: message.TypeTextDelta, Delta: "second"},
		{Type: message.TypeTextDelta, Delta: "third"},
	}

	received := message.ForwardStreamEvents(input)

	require.Len(t, received, 3)
	assert.Equal(t, "first", received[0].Delta)
	assert.Equal(t, "second", received[1].Delta)
	assert.Equal(t, "third", received[2].Delta)
}

// TestStreamEvent_ForwardEmpty는 빈 슬라이스 입력 시 nil을 반환하는지 검증한다.
func TestStreamEvent_ForwardEmpty(t *testing.T) {
	t.Parallel()

	result := message.ForwardStreamEvents(nil)
	assert.Nil(t, result)

	result2 := message.ForwardStreamEvents([]message.StreamEvent{})
	assert.Nil(t, result2)
}

// TestStreamEvent_MixedTypes는 서로 다른 타입의 이벤트가 순서 보존되는지 검증한다.
func TestStreamEvent_MixedTypes(t *testing.T) {
	t.Parallel()

	input := []message.StreamEvent{
		{Type: message.TypeMessageStart},
		{Type: message.TypeContentBlockStart, BlockType: "text"},
		{Type: message.TypeTextDelta, Delta: "hello"},
		{Type: message.TypeContentBlockStop},
		{Type: message.TypeMessageStop},
	}

	received := message.ForwardStreamEvents(input)

	require.Len(t, received, 5)
	assert.Equal(t, message.TypeMessageStart, received[0].Type)
	assert.Equal(t, message.TypeContentBlockStart, received[1].Type)
	assert.Equal(t, "hello", received[2].Delta)
	assert.Equal(t, message.TypeContentBlockStop, received[3].Type)
	assert.Equal(t, message.TypeMessageStop, received[4].Type)
}
