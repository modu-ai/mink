package message

// ForwardStreamEvents는 입력 StreamEvent 슬라이스를 입력 순서 그대로 복사하여 반환한다.
// LLM 스트림에서 수신한 delta 이벤트의 순서 보존을 보장하는 pass-through helper이다.
//
// @MX:NOTE: [AUTO] delta 순서 보존은 REQ-QUERY-002의 핵심 불변식이다.
// @MX:SPEC: REQ-QUERY-002
//
// ForwardStreamEvents returns a copy of the input events preserving their order.
func ForwardStreamEvents(events []StreamEvent) []StreamEvent {
	if len(events) == 0 {
		return nil
	}
	out := make([]StreamEvent, len(events))
	copy(out, events)
	return out
}
