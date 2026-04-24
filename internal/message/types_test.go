package message_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/message"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestStreamEventTypes_AllDefined는 StreamEvent 타입 상수 10종이 정의되어 있는지 검증한다.
func TestStreamEventTypes_AllDefined(t *testing.T) {
	t.Parallel()

	expected := []string{
		message.TypeStreamRequestStart,
		message.TypeMessageStart,
		message.TypeTextDelta,
		message.TypeThinkingDelta,
		message.TypeInputJSONDelta,
		message.TypeContentBlockStart,
		message.TypeContentBlockStop,
		message.TypeMessageDelta,
		message.TypeMessageStop,
		message.TypeError,
	}

	for _, typ := range expected {
		if typ == "" {
			t.Errorf("빈 타입 상수 발견 — 모든 타입은 non-empty 문자열이어야 함")
		}
	}

	// 중복 없음 검증
	seen := make(map[string]bool)
	for _, typ := range expected {
		if seen[typ] {
			t.Errorf("중복 타입 상수: %s", typ)
		}
		seen[typ] = true
	}
}

// TestContentBlock_Fields는 ContentBlock 구조체의 핵심 필드를 검증한다.
func TestContentBlock_Fields(t *testing.T) {
	t.Parallel()

	cb := message.ContentBlock{
		Type:           "text",
		Text:           "hello",
		Image:          []byte{1, 2, 3},
		ImageMediaType: "image/jpeg",
		ToolUseID:      "tu-123",
		ToolResultJSON: `{"result":"ok"}`,
		Thinking:       "some thinking",
	}

	if cb.Type != "text" {
		t.Errorf("Type: got %q, want %q", cb.Type, "text")
	}
	if cb.Text != "hello" {
		t.Errorf("Text: got %q, want %q", cb.Text, "hello")
	}
	if len(cb.Image) != 3 {
		t.Errorf("Image length: got %d, want 3", len(cb.Image))
	}
	if cb.ToolUseID != "tu-123" {
		t.Errorf("ToolUseID: got %q, want %q", cb.ToolUseID, "tu-123")
	}
}

// TestMessage_Fields는 Message 구조체가 필요한 필드를 가지는지 검증한다.
func TestMessage_Fields(t *testing.T) {
	t.Parallel()

	msg := message.Message{
		Role: "user",
		Content: []message.ContentBlock{
			{Type: "text", Text: "hello"},
		},
		ToolUseID: "tu-456",
	}

	if msg.Role != "user" {
		t.Errorf("Role: got %q, want %q", msg.Role, "user")
	}
	if len(msg.Content) != 1 {
		t.Errorf("Content length: got %d, want 1", len(msg.Content))
	}
	if msg.ToolUseID != "tu-456" {
		t.Errorf("ToolUseID: got %q, want %q", msg.ToolUseID, "tu-456")
	}
}

// TestStreamEvent_Fields는 StreamEvent 구조체가 필요한 필드를 가지는지 검증한다.
func TestStreamEvent_Fields(t *testing.T) {
	t.Parallel()

	evt := message.StreamEvent{
		Type:       message.TypeTextDelta,
		Delta:      "some text",
		BlockType:  "text",
		ToolUseID:  "tu-789",
		StopReason: "end_turn",
		Error:      "some error",
		Raw:        map[string]any{"key": "value"},
	}

	if evt.Type != message.TypeTextDelta {
		t.Errorf("Type: got %q, want %q", evt.Type, message.TypeTextDelta)
	}
	if evt.Delta != "some text" {
		t.Errorf("Delta: got %q, want %q", evt.Delta, "some text")
	}
	if evt.StopReason != "end_turn" {
		t.Errorf("StopReason: got %q, want %q", evt.StopReason, "end_turn")
	}
}
