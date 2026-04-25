package message

// @MX:NOTE: [AUTO] Normalize는 LLM에 전달하기 전 메시지 배열을 정규화한다.
// @MX:SPEC: REQ-QUERY-003 — State 변환 진입점
//
// 수행 작업:
// 1. 연속한 동일 role 메시지를 Content 배열 병합으로 통합 (주로 user 역할)
// 2. assistant 메시지에서 "signature" 타입 ContentBlock 제거
//
// Normalize normalizes a slice of Message before LLM transmission.
// signature 제거와 consecutive merge를 단일 패스로 처리한다.
func Normalize(msgs []Message) []Message {
	if len(msgs) == 0 {
		return nil
	}

	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		// assistant 메시지에서 signature 블록 제거
		if m.Role == "assistant" {
			m = stripSignature(m)
		}
		// 연속한 동일 role 메시지 병합
		if len(out) > 0 && out[len(out)-1].Role == m.Role {
			last := &out[len(out)-1]
			last.Content = append(last.Content, m.Content...)
		} else {
			out = append(out, m)
		}
	}

	return out
}

// stripSignature는 assistant 메시지에서 type="signature" ContentBlock을 제거한다.
// claude-core.md §7의 normalize 로직 번역.
func stripSignature(m Message) Message {
	filtered := make([]ContentBlock, 0, len(m.Content))
	for _, cb := range m.Content {
		if cb.Type != "signature" {
			filtered = append(filtered, cb)
		}
	}
	m.Content = filtered
	return m
}
