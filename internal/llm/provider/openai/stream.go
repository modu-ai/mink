package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/modu-ai/goose/internal/message"
)

// openAIChunk는 OpenAI chat completions 스트림 청크이다.
type openAIChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string          `json:"role"`
			Content   string          `json:"content"`
			ToolCalls []toolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// toolCallDelta는 스트리밍 tool_call 조각이다.
type toolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// partialCall은 index별로 누적 중인 tool_call 데이터이다.
type partialCall struct {
	id        string
	name      string
	arguments strings.Builder
}

// readLine은 reader goroutine이 반환하는 라인 또는 에러이다.
type readLine struct {
	line string
	err  error
}

// ParseAndConvert는 OpenAI SSE 스트림을 파싱하여 StreamEvent로 변환한다.
// goroutine 소유권: 호출자가 spawn, 이 함수에서 defer close(out)으로 닫는다.
// ctx 취소 또는 hbTimeout 초과 시 즉시 종료한다.
//
// @MX:WARN: [AUTO] reader goroutine + reslide-timer watchdog — goroutine 누수 위험
// @MX:REASON: readerLoop goroutine은 body.Close() 호출로 정리된다.
//
//	hbTimeout 타임아웃 경로에서도 defer body.Close()가 반드시 실행되어야 한다.
func ParseAndConvert(ctx context.Context, body io.ReadCloser, out chan<- message.StreamEvent, hbTimeout time.Duration) {
	defer close(out)
	defer body.Close()

	// reader goroutine: body를 라인 단위로 읽어 lineCh에 전달한다.
	lineCh := make(chan readLine, 4)
	go func() {
		defer close(lineCh)
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			lineCh <- readLine{line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			lineCh <- readLine{err: err}
		}
	}()

	hb := time.NewTimer(hbTimeout)
	defer hb.Stop()

	// tool_call index별 누적 버퍼
	toolBuf := make(map[int]*partialCall)

	send := func(evt message.StreamEvent) bool {
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
			send(message.StreamEvent{
				Type:  message.TypeError,
				Error: fmt.Sprintf("openai: heartbeat timeout: no data for %s", hbTimeout),
			})
			return

		case r, ok := <-lineCh:
			if !ok {
				return
			}
			resetHB()

			if r.err != nil {
				send(message.StreamEvent{Type: message.TypeError, Error: r.err.Error()})
				return
			}

			line := r.line
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			// 스트림 종료 신호
			if data == "[DONE]" {
				send(message.StreamEvent{Type: message.TypeMessageStop})
				return
			}

			var chunk openAIChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			for _, choice := range chunk.Choices {
				delta := choice.Delta

				// 텍스트 delta
				if delta.Content != "" {
					if !send(message.StreamEvent{
						Type:  message.TypeTextDelta,
						Delta: delta.Content,
					}) {
						return
					}
				}

				// tool_calls 누적
				for _, tc := range delta.ToolCalls {
					pc, exists := toolBuf[tc.Index]
					if !exists {
						pc = &partialCall{}
						toolBuf[tc.Index] = pc
					}
					if tc.ID != "" {
						pc.id = tc.ID
					}
					if tc.Function.Name != "" {
						pc.name = tc.Function.Name
						// name이 처음 도착했을 때 content_block_start emit
						if !send(message.StreamEvent{
							Type:      message.TypeContentBlockStart,
							BlockType: "tool_use",
							ToolUseID: pc.id,
						}) {
							return
						}
					}
					if tc.Function.Arguments != "" {
						pc.arguments.WriteString(tc.Function.Arguments)
						// arguments 조각 즉시 emit
						if !send(message.StreamEvent{
							Type:  message.TypeInputJSONDelta,
							Delta: tc.Function.Arguments,
						}) {
							return
						}
					}
				}

				// finish_reason 처리
				if choice.FinishReason != nil {
					switch *choice.FinishReason {
					case "tool_calls":
						// 모든 누적된 tool_call에 대해 content_block_stop emit
						for range toolBuf {
							if !send(message.StreamEvent{Type: message.TypeContentBlockStop}) {
								return
							}
						}
						// tool_call 완료 후 message_stop
						send(message.StreamEvent{Type: message.TypeMessageStop})
						return
					case "stop":
						// 정상 종료: message_stop은 [DONE]에서 emit하므로 여기서는 skip
					}
				}
			}
		}
	}
}
