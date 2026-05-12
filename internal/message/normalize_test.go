package message_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessage_Normalize_MergesConsecutiveUser는 연속한 user 메시지를 하나로 병합하는지 검증한다.
// research.md §8.1 / REQ-QUERY-003
func TestMessage_Normalize_MergesConsecutiveUser(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}},
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "world"}}},
		{Role: "assistant", Content: []message.ContentBlock{{Type: "text", Text: "ok"}}},
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "again"}}},
	}

	result := message.Normalize(msgs)

	// 연속 user 2개 → 1개로 병합
	require.Len(t, result, 3, "연속 user 병합 후 3개 메시지여야 함")
	assert.Equal(t, "user", result[0].Role)
	assert.Len(t, result[0].Content, 2, "병합된 user 메시지는 2개 ContentBlock을 가져야 함")
	assert.Equal(t, "hello", result[0].Content[0].Text)
	assert.Equal(t, "world", result[0].Content[1].Text)
	assert.Equal(t, "assistant", result[1].Role)
	assert.Equal(t, "user", result[2].Role)
}

// TestMessage_Normalize_StripsSignatureFromAssistant는 assistant 메시지에서
// signature 블록을 제거하는지 검증한다.
// research.md §8.1 / REQ-QUERY-003
func TestMessage_Normalize_StripsSignatureFromAssistant(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{
			Role: "assistant",
			Content: []message.ContentBlock{
				{Type: "text", Text: "answer"},
				{Type: "signature", Text: "sig-abc"},
			},
		},
	}

	result := message.Normalize(msgs)

	require.Len(t, result, 1)
	require.Len(t, result[0].Content, 1, "signature 블록 제거 후 ContentBlock 1개여야 함")
	assert.Equal(t, "text", result[0].Content[0].Type)
	assert.Equal(t, "answer", result[0].Content[0].Text)
}

// TestMessage_Normalize_EmptyInput은 빈 slice 입력 시 빈 slice를 반환하는지 검증한다.
func TestMessage_Normalize_EmptyInput(t *testing.T) {
	t.Parallel()

	result := message.Normalize(nil)
	assert.Empty(t, result)

	result2 := message.Normalize([]message.Message{})
	assert.Empty(t, result2)
}

// TestMessage_Normalize_SingleMessage는 단일 메시지가 변형 없이 통과하는지 검증한다.
func TestMessage_Normalize_SingleMessage(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}},
	}

	result := message.Normalize(msgs)

	require.Len(t, result, 1)
	assert.Equal(t, "user", result[0].Role)
	assert.Equal(t, "hi", result[0].Content[0].Text)
}

// TestMessage_Normalize_PreservesOrder는 정규화 후 메시지 순서가 유지되는지 검증한다.
func TestMessage_Normalize_PreservesOrder(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{Role: "assistant", Content: []message.ContentBlock{{Type: "text", Text: "first"}}},
		{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "second"}}},
		{Role: "assistant", Content: []message.ContentBlock{{Type: "text", Text: "third"}}},
	}

	result := message.Normalize(msgs)

	require.Len(t, result, 3)
	assert.Equal(t, "assistant", result[0].Role)
	assert.Equal(t, "user", result[1].Role)
	assert.Equal(t, "assistant", result[2].Role)
}
