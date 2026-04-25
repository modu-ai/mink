// Package message는 QUERY-001 엔진이 사용하는 SDKMessage 및 관련 타입을 정의한다.
// SPEC-GOOSE-QUERY-001 S0 T0.1
package message

// SDKMessageType은 QueryEngine이 출력 채널에 yield하는 메시지 타입 열거형이다.
// spec.md §6.2 SDKMessage 10종 정의.
type SDKMessageType string

const (
	// SDKMsgUserAck는 SubmitMessage 즉시 yield되는 사용자 요청 확인 메시지이다.
	SDKMsgUserAck SDKMessageType = "user_ack"
	// SDKMsgStreamRequestStart는 LLM API 호출 시작 알림이다.
	SDKMsgStreamRequestStart SDKMessageType = "stream_request_start"
	// SDKMsgStreamEvent는 LLM 스트림에서 수신한 delta 이벤트이다.
	SDKMsgStreamEvent SDKMessageType = "stream_event"
	// SDKMsgMessage는 완성된 assistant/user 메시지이다.
	SDKMsgMessage SDKMessageType = "message"
	// SDKMsgToolUseSummary는 tool 호출 요약 메시지이다.
	SDKMsgToolUseSummary SDKMessageType = "tool_use_summary"
	// SDKMsgPermissionRequest는 Ask 분기에서 외부 결정을 요청하는 메시지이다.
	SDKMsgPermissionRequest SDKMessageType = "permission_request"
	// SDKMsgPermissionCheck는 Allow/Deny 분기에서 권한 결과를 알리는 메시지이다.
	SDKMsgPermissionCheck SDKMessageType = "permission_check"
	// SDKMsgCompactBoundary는 Compaction 발생 시 경계 정보를 알리는 메시지이다.
	SDKMsgCompactBoundary SDKMessageType = "compact_boundary"
	// SDKMsgError는 비복구 오류를 알리는 메시지이다.
	SDKMsgError SDKMessageType = "error"
	// SDKMsgTerminal는 loop 종료 상태를 알리는 최종 메시지이다.
	SDKMsgTerminal SDKMessageType = "terminal"
)

// SDKMessage는 QueryEngine이 출력 채널(chan SDKMessage)에 전송하는 discriminated union이다.
// Type 필드로 Payload 구조체 종류를 결정한다.
type SDKMessage struct {
	// Type은 메시지 종류를 결정하는 discriminator이다.
	Type SDKMessageType
	// Payload는 Type에 따른 가변 페이로드이다.
	Payload any
	// Meta는 선택적 메타데이터이다 (trace_id, turn, iteration, teammate identity 등).
	Meta map[string]any
}

// PayloadUserAck는 SDKMsgUserAck 타입의 페이로드이다.
type PayloadUserAck struct {
	// Prompt는 사용자가 전달한 원본 메시지이다.
	Prompt string
}

// PayloadStreamRequestStart는 SDKMsgStreamRequestStart 타입의 페이로드이다.
type PayloadStreamRequestStart struct {
	// Turn은 현재 turn 번호이다.
	Turn int
}

// PayloadStreamEvent는 SDKMsgStreamEvent 타입의 페이로드이다.
type PayloadStreamEvent struct {
	// Event는 원본 StreamEvent이다.
	Event StreamEvent
}

// PayloadMessage는 SDKMsgMessage 타입의 페이로드이다.
type PayloadMessage struct {
	// Msg는 완성된 Message이다.
	Msg Message
}

// PayloadToolUseSummary는 SDKMsgToolUseSummary 타입의 페이로드이다.
type PayloadToolUseSummary struct {
	// ToolUseID는 tool_use 블록 ID이다.
	ToolUseID string
	// ToolName은 호출된 도구 이름이다.
	ToolName string
	// InputSummary는 입력 요약이다.
	InputSummary string
	// ResultSummary는 결과 요약이다.
	ResultSummary string
}

// PayloadPermissionRequest는 SDKMsgPermissionRequest 타입의 페이로드이다.
// Ask 분기에서 외부 결정을 요청할 때 yield된다.
type PayloadPermissionRequest struct {
	// ToolUseID는 권한 확인이 필요한 tool_use 블록 ID이다.
	ToolUseID string
	// ToolName은 도구 이름이다.
	ToolName string
	// Input은 도구 입력 파라미터이다.
	Input map[string]any
}

// PayloadPermissionCheck는 SDKMsgPermissionCheck 타입의 페이로드이다.
// Allow/Deny 결과를 알릴 때 yield된다.
type PayloadPermissionCheck struct {
	// ToolUseID는 확인된 tool_use 블록 ID이다.
	ToolUseID string
	// Behavior는 권한 결정 결과이다 ("allow" | "deny").
	Behavior string
	// Reason은 Deny 시 이유이다 (선택적).
	Reason string
}

// PayloadCompactBoundary는 SDKMsgCompactBoundary 타입의 페이로드이다.
type PayloadCompactBoundary struct {
	// Turn은 compaction이 발생한 turn 번호이다.
	Turn int
	// MessagesBefore는 compaction 전 메시지 수이다.
	MessagesBefore int
	// MessagesAfter는 compaction 후 메시지 수이다.
	MessagesAfter int
}

// PayloadError는 SDKMsgError 타입의 페이로드이다.
type PayloadError struct {
	// Err는 오류 내용이다.
	Err string
	// Code는 오류 코드이다 (선택적).
	Code string
}

// PayloadTerminal는 SDKMsgTerminal 타입의 페이로드이다.
type PayloadTerminal struct {
	// Success는 loop가 성공적으로 완료되었는지 여부이다.
	Success bool
	// Error는 종료 이유이다 ("max_turns" | "budget_exceeded" | "aborted" | "max_output_tokens_exhausted" 등).
	// Success=true이더라도 "max_turns"처럼 제한 도달 이유가 있을 수 있다.
	Error string
}
