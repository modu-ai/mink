package message

// ToolUseSummaryMessage는 tool 호출의 요약 정보를 담는 메시지이다.
// 1MB 초과 tool result를 축약 표현으로 대체할 때 사용한다.
// spec.md §6.2 / REQ-QUERY-007 / AC-QUERY-009
type ToolUseSummaryMessage struct {
	// ToolUseID는 대응하는 tool_use 블록의 ID이다.
	ToolUseID string
	// Content는 원본 tool result 내용이다 (nil이면 빈 결과).
	Content map[string]any
	// Truncated는 원본이 잘렸는지 여부이다.
	Truncated bool
	// BytesOriginal은 원본 결과의 바이트 크기이다.
	BytesOriginal int
	// BytesKept는 잘린 후 유지된 바이트 크기이다.
	BytesKept int
}

// FormatToolUseSummary는 tool 호출 결과의 요약 메시지를 생성한다.
//
// @MX:NOTE: [AUTO] 1MB 초과 tool result를 요약 표현으로 치환할 때의 포맷 함수이다.
// @MX:SPEC: REQ-QUERY-007, AC-QUERY-009
//
// FormatToolUseSummary creates a ToolUseSummaryMessage from the given parameters.
func FormatToolUseSummary(
	toolUseID string,
	content map[string]any,
	truncated bool,
	bytesOriginal int,
	bytesKept int,
) ToolUseSummaryMessage {
	return ToolUseSummaryMessage{
		ToolUseID:     toolUseID,
		Content:       content,
		Truncated:     truncated,
		BytesOriginal: bytesOriginal,
		BytesKept:     bytesKept,
	}
}
