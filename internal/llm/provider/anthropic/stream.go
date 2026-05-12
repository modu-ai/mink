package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/modu-ai/mink/internal/message"
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

// readResult는 reader goroutine이 반환하는 라인 또는 에러이다.
type readResult struct {
	line string
	err  error
}

// ParseAndConvert는 Anthropic SSE 스트림을 파싱하여 StreamEvent로 변환한다.
// goroutine 소유권: 호출자가 spawn, 이 함수에서 defer close(out)으로 닫는다.
// ctx 취소 또는 hbTimeout 초과 시 즉시 종료한다.
//
// @MX:WARN: [AUTO] reader goroutine + reslide-timer watchdog — goroutine 누수 위험
// @MX:REASON: readerLoop goroutine은 body.Close() 호출로 정리된다.
//
//	hbTimeout 타임아웃 경로에서도 defer body.Close()가 반드시 실행되어야 한다.
//
// 10종 이벤트 → StreamEvent 변환 (spec §6.5 테이블)
func ParseAndConvert(ctx context.Context, body io.ReadCloser, out chan<- message.StreamEvent, hbTimeout time.Duration, logger *zap.Logger) {
	defer close(out)
	defer body.Close()

	// reader goroutine: body를 라인 단위로 읽어 lineCh에 전달한다.
	// body.Close() 호출 시 scanner.Scan()이 에러를 반환하여 goroutine이 종료된다.
	lineCh := make(chan readResult, 4)
	go func() {
		defer close(lineCh)
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			lineCh <- readResult{line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			lineCh <- readResult{err: err}
		}
	}()

	hb := time.NewTimer(hbTimeout)
	defer hb.Stop()

	var currentEvent sseEvent

	emit := func(evt message.StreamEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- evt:
			return true
		}
	}

	resetHB := func() {
		if !hb.Stop() {
			select {
			case <-hb.C:
			default:
			}
		}
		hb.Reset(hbTimeout)
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-hb.C:
			emit(message.StreamEvent{
				Type:  message.TypeError,
				Error: fmt.Sprintf("anthropic: heartbeat timeout: no data for %s", hbTimeout),
			})
			return

		case r, ok := <-lineCh:
			if !ok {
				// reader goroutine이 종료됨 = 스트림 끝
				return
			}
			resetHB()

			if r.err != nil {
				emit(message.StreamEvent{Type: message.TypeError, Error: r.err.Error()})
				return
			}

			line := r.line

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
						if !emit(*evt) {
							return
						}
					}
				}
				currentEvent = sseEvent{}
			}
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
