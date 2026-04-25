package message

import "fmt"

// SDKMessageHandlers는 SDKMessage 10종의 핸들러 묶음이다.
// SwitchSDKMessage에서 exhaustive dispatch에 사용한다.
// 모든 핸들러 필드는 선택 사항이지만, nil 핸들러가 있는 타입의 메시지가
// 수신되면 무시된다 (에러 없음).
type SDKMessageHandlers struct {
	OnUserAck            func(PayloadUserAck)
	OnStreamRequestStart func(PayloadStreamRequestStart)
	OnStreamEvent        func(PayloadStreamEvent)
	OnMessage            func(PayloadMessage)
	OnToolUseSummary     func(PayloadToolUseSummary)
	OnPermissionRequest  func(PayloadPermissionRequest)
	OnPermissionCheck    func(PayloadPermissionCheck)
	OnCompactBoundary    func(PayloadCompactBoundary)
	OnError              func(PayloadError)
	OnTerminal           func(PayloadTerminal)
}

// SwitchSDKMessage는 SDKMessage.Type에 따라 해당 핸들러를 호출하는 exhaustive type-switch helper이다.
// 알 수 없는 타입은 에러를 반환한다.
//
// @MX:NOTE: [AUTO] 10종 SDKMessageType에 대한 중앙 집중식 dispatch 지점이다.
// @MX:SPEC: REQ-QUERY-002, REQ-QUERY-007
//
// SwitchSDKMessage dispatches the SDKMessage to the appropriate handler.
func SwitchSDKMessage(msg SDKMessage, h SDKMessageHandlers) error {
	switch msg.Type {
	case SDKMsgUserAck:
		if h.OnUserAck != nil {
			p, _ := msg.Payload.(PayloadUserAck)
			h.OnUserAck(p)
		}
	case SDKMsgStreamRequestStart:
		if h.OnStreamRequestStart != nil {
			p, _ := msg.Payload.(PayloadStreamRequestStart)
			h.OnStreamRequestStart(p)
		}
	case SDKMsgStreamEvent:
		if h.OnStreamEvent != nil {
			p, _ := msg.Payload.(PayloadStreamEvent)
			h.OnStreamEvent(p)
		}
	case SDKMsgMessage:
		if h.OnMessage != nil {
			p, _ := msg.Payload.(PayloadMessage)
			h.OnMessage(p)
		}
	case SDKMsgToolUseSummary:
		if h.OnToolUseSummary != nil {
			p, _ := msg.Payload.(PayloadToolUseSummary)
			h.OnToolUseSummary(p)
		}
	case SDKMsgPermissionRequest:
		if h.OnPermissionRequest != nil {
			p, _ := msg.Payload.(PayloadPermissionRequest)
			h.OnPermissionRequest(p)
		}
	case SDKMsgPermissionCheck:
		if h.OnPermissionCheck != nil {
			p, _ := msg.Payload.(PayloadPermissionCheck)
			h.OnPermissionCheck(p)
		}
	case SDKMsgCompactBoundary:
		if h.OnCompactBoundary != nil {
			p, _ := msg.Payload.(PayloadCompactBoundary)
			h.OnCompactBoundary(p)
		}
	case SDKMsgError:
		if h.OnError != nil {
			p, _ := msg.Payload.(PayloadError)
			h.OnError(p)
		}
	case SDKMsgTerminal:
		if h.OnTerminal != nil {
			p, _ := msg.Payload.(PayloadTerminal)
			h.OnTerminal(p)
		}
	default:
		return fmt.Errorf("SwitchSDKMessage: 알 수 없는 SDKMessageType %q", msg.Type)
	}
	return nil
}
