// Package message는 LLM 스트리밍 응답과 메시지 타입을 정의한다.
// SPEC-GOOSE-ADAPTER-001 M0 T-001
package message

// StreamEvent 타입 상수 (10종)
const (
	// TypeStreamRequestStart는 스트림 요청 시작 이벤트이다.
	TypeStreamRequestStart = "stream_request_start"
	// TypeMessageStart는 메시지 시작 이벤트이다.
	TypeMessageStart = "message_start"
	// TypeTextDelta는 텍스트 델타 이벤트이다.
	TypeTextDelta = "text_delta"
	// TypeThinkingDelta는 thinking 델타 이벤트이다.
	TypeThinkingDelta = "thinking_delta"
	// TypeInputJSONDelta는 tool 입력 JSON 델타 이벤트이다.
	TypeInputJSONDelta = "input_json_delta"
	// TypeContentBlockStart는 콘텐츠 블록 시작 이벤트이다.
	TypeContentBlockStart = "content_block_start"
	// TypeContentBlockStop는 콘텐츠 블록 종료 이벤트이다.
	TypeContentBlockStop = "content_block_stop"
	// TypeMessageDelta는 메시지 델타 이벤트이다.
	TypeMessageDelta = "message_delta"
	// TypeMessageStop는 메시지 종료 이벤트이다.
	TypeMessageStop = "message_stop"
	// TypeError는 에러 이벤트이다.
	TypeError = "error"
)

// ContentBlock은 메시지의 콘텐츠 블록이다.
// 텍스트, 이미지, tool_use, tool_result, thinking 블록을 표현한다.
type ContentBlock struct {
	// Type은 블록 타입이다 ("text" | "image" | "tool_use" | "tool_result" | "thinking").
	Type string
	// Text는 텍스트 블록의 내용이다.
	Text string
	// Image는 이미지 블록의 원본 바이트이다.
	Image []byte
	// ImageMediaType은 이미지의 MIME 타입이다 (예: "image/jpeg").
	ImageMediaType string
	// ToolUseID는 tool_use 또는 tool_result 블록의 ID이다.
	ToolUseID string
	// ToolResultJSON은 tool_result 블록의 JSON 직렬화 결과이다.
	ToolResultJSON string
	// Thinking은 thinking 블록의 내용이다.
	Thinking string
}

// Message는 LLM 대화의 단일 메시지이다.
type Message struct {
	// Role은 메시지 역할이다 ("user" | "assistant" | "system").
	Role string
	// Content는 메시지의 콘텐츠 블록 목록이다.
	Content []ContentBlock
	// ToolUseID는 tool_result 메시지의 경우 대응하는 tool_use ID이다.
	ToolUseID string
}

// StreamEvent는 LLM 스트리밍 응답의 단일 이벤트이다.
type StreamEvent struct {
	// Type은 이벤트 타입이다 (TypeXxx 상수 중 하나).
	Type string
	// Delta는 텍스트/thinking/input_json 델타의 내용이다.
	Delta string
	// BlockType은 content_block_start 이벤트의 블록 타입이다.
	BlockType string
	// ToolUseID는 tool_use 관련 이벤트의 tool use ID이다.
	ToolUseID string
	// StopReason은 message_delta/message_stop 이벤트의 종료 이유이다.
	// "end_turn" | "tool_use" | "max_output_tokens" 등.
	StopReason string
	// Error는 error 이벤트의 메시지이다.
	Error string
	// InputTokens는 TypeMessageDelta 이벤트의 입력 토큰 수이다.
	// REQ-QUERY-011: budget 차감에 사용된다.
	InputTokens int
	// OutputTokens는 TypeMessageDelta 이벤트의 출력 토큰 수이다.
	// REQ-QUERY-011: budget 차감에 사용된다.
	OutputTokens int
	// Raw는 원본 이벤트 데이터이다 (디버깅용).
	Raw any
}
