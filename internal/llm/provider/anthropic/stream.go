package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/modu-ai/goose/internal/message"
	"go.uber.org/zap"
)

// sseEvent는 Anthropic SSE 라인 파싱 결과이다.
type sseEvent struct {
	eventType string
	data      string
}

// anthropicEventData는 Anthropic SSE data 필드의 공통 구조이다.
type anthropicEventData struct {
	Type string `json:"type"`

	// message_start
	Message struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	} `json:"message"`

	// content_block_start
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"` // tool_use block ID
		Name string `json:"name"`
	} `json:"content_block"`

	// content_block_delta
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`

	// error
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ParseAndConvert는 Anthropic SSE 스트림을 파싱하여 StreamEvent로 변환한다.
// goroutine 소유권: 호출자가 spawn, 이 함수에서 defer close(out)으로 닫는다.
// ctx 취소 시 즉시 종료한다.
//
// 10종 이벤트 → StreamEvent 변환 (spec §6.5 테이블)
func ParseAndConvert(ctx context.Context, body io.ReadCloser, out chan<- message.StreamEvent, logger *zap.Logger) {
	defer close(out)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var currentEvent sseEvent

	for scanner.Scan() {
		// ctx 취소 확인
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "event: "):
			currentEvent.eventType = strings.TrimPrefix(line, "event: ")

		case strings.HasPrefix(line, "data: "):
			currentEvent.data = strings.TrimPrefix(line, "data: ")

		case line == "":
			// 빈 라인 = 이벤트 종료
			if currentEvent.data != "" {
				evt := convertEvent(currentEvent, logger)
				if evt != nil {
					select {
					case <-ctx.Done():
						return
					case out <- *evt:
					}
				}
			}
			currentEvent = sseEvent{}
		}
	}

	// 스캐너 에러 처리
	if err := scanner.Err(); err != nil && err != io.EOF {
		select {
		case <-ctx.Done():
		case out <- message.StreamEvent{Type: message.TypeError, Error: err.Error()}:
		}
	}
}

// convertEvent는 SSE 이벤트를 StreamEvent로 변환한다.
func convertEvent(evt sseEvent, logger *zap.Logger) *message.StreamEvent {
	if evt.data == "" {
		return nil
	}

	var d anthropicEventData
	if err := json.Unmarshal([]byte(evt.data), &d); err != nil {
		if logger != nil {
			logger.Debug("SSE data 파싱 실패", zap.Error(err))
		}
		return nil
	}

	switch d.Type {
	case "message_start":
		return &message.StreamEvent{
			Type: message.TypeMessageStart,
			Raw:  d,
		}

	case "content_block_start":
		evt := &message.StreamEvent{
			Type:      message.TypeContentBlockStart,
			BlockType: d.ContentBlock.Type,
		}
		// tool_use 블록의 ToolUseID 추출
		if d.ContentBlock.Type == "tool_use" {
			evt.ToolUseID = d.ContentBlock.ID
		}
		return evt

	case "content_block_delta":
		switch d.Delta.Type {
		case "text_delta":
			return &message.StreamEvent{
				Type:  message.TypeTextDelta,
				Delta: d.Delta.Text,
			}
		case "thinking_delta":
			return &message.StreamEvent{
				Type:  message.TypeThinkingDelta,
				Delta: d.Delta.Thinking,
			}
		case "input_json_delta":
			return &message.StreamEvent{
				Type:  message.TypeInputJSONDelta,
				Delta: d.Delta.PartialJSON,
			}
		}

	case "content_block_stop":
		return &message.StreamEvent{
			Type: message.TypeContentBlockStop,
		}

	case "message_delta":
		return &message.StreamEvent{
			Type:       message.TypeMessageDelta,
			StopReason: d.Delta.StopReason,
		}

	case "message_stop":
		return &message.StreamEvent{
			Type: message.TypeMessageStop,
		}

	case "error":
		return &message.StreamEvent{
			Type:  message.TypeError,
			Error: d.Error.Message,
		}
	}

	return nil
}
