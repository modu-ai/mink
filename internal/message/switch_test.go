package message_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSDKMessage_TypeSwitchExhaustive는 10개 SDKMessageType 각각이
// SwitchSDKMessage에서 올바른 핸들러를 호출하는지 검증한다.
// plan.md T1.2 / REQ-QUERY-002, REQ-QUERY-007
func TestSDKMessage_TypeSwitchExhaustive(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		msgType message.SDKMessageType
		payload any
	}{
		{
			name:    "UserAck",
			msgType: message.SDKMsgUserAck,
			payload: message.PayloadUserAck{Prompt: "hi"},
		},
		{
			name:    "StreamRequestStart",
			msgType: message.SDKMsgStreamRequestStart,
			payload: message.PayloadStreamRequestStart{Turn: 1},
		},
		{
			name:    "StreamEvent",
			msgType: message.SDKMsgStreamEvent,
			payload: message.PayloadStreamEvent{Event: message.StreamEvent{Type: message.TypeTextDelta}},
		},
		{
			name:    "Message",
			msgType: message.SDKMsgMessage,
			payload: message.PayloadMessage{Msg: message.Message{Role: "assistant"}},
		},
		{
			name:    "ToolUseSummary",
			msgType: message.SDKMsgToolUseSummary,
			payload: message.PayloadToolUseSummary{ToolUseID: "tu-1"},
		},
		{
			name:    "PermissionRequest",
			msgType: message.SDKMsgPermissionRequest,
			payload: message.PayloadPermissionRequest{ToolUseID: "tu-2"},
		},
		{
			name:    "PermissionCheck",
			msgType: message.SDKMsgPermissionCheck,
			payload: message.PayloadPermissionCheck{ToolUseID: "tu-3", Behavior: "allow"},
		},
		{
			name:    "CompactBoundary",
			msgType: message.SDKMsgCompactBoundary,
			payload: message.PayloadCompactBoundary{Turn: 2},
		},
		{
			name:    "Error",
			msgType: message.SDKMsgError,
			payload: message.PayloadError{Err: "oops"},
		},
		{
			name:    "Terminal",
			msgType: message.SDKMsgTerminal,
			payload: message.PayloadTerminal{Success: true},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			msg := message.SDKMessage{Type: tc.msgType, Payload: tc.payload}
			called := false

			handlers := message.SDKMessageHandlers{
				OnUserAck: func(p message.PayloadUserAck) {
					assert.Equal(t, message.SDKMsgUserAck, tc.msgType)
					called = true
				},
				OnStreamRequestStart: func(p message.PayloadStreamRequestStart) {
					assert.Equal(t, message.SDKMsgStreamRequestStart, tc.msgType)
					called = true
				},
				OnStreamEvent: func(p message.PayloadStreamEvent) {
					assert.Equal(t, message.SDKMsgStreamEvent, tc.msgType)
					called = true
				},
				OnMessage: func(p message.PayloadMessage) {
					assert.Equal(t, message.SDKMsgMessage, tc.msgType)
					called = true
				},
				OnToolUseSummary: func(p message.PayloadToolUseSummary) {
					assert.Equal(t, message.SDKMsgToolUseSummary, tc.msgType)
					called = true
				},
				OnPermissionRequest: func(p message.PayloadPermissionRequest) {
					assert.Equal(t, message.SDKMsgPermissionRequest, tc.msgType)
					called = true
				},
				OnPermissionCheck: func(p message.PayloadPermissionCheck) {
					assert.Equal(t, message.SDKMsgPermissionCheck, tc.msgType)
					called = true
				},
				OnCompactBoundary: func(p message.PayloadCompactBoundary) {
					assert.Equal(t, message.SDKMsgCompactBoundary, tc.msgType)
					called = true
				},
				OnError: func(p message.PayloadError) {
					assert.Equal(t, message.SDKMsgError, tc.msgType)
					called = true
				},
				OnTerminal: func(p message.PayloadTerminal) {
					assert.Equal(t, message.SDKMsgTerminal, tc.msgType)
					called = true
				},
			}

			err := message.SwitchSDKMessage(msg, handlers)
			require.NoError(t, err)
			assert.True(t, called, "핸들러가 호출되어야 함: %s", tc.name)
		})
	}
}

// TestSDKMessage_TypeSwitchUnknownType은 알 수 없는 타입 시 에러를 반환하는지 검증한다.
func TestSDKMessage_TypeSwitchUnknownType(t *testing.T) {
	t.Parallel()

	msg := message.SDKMessage{Type: "unknown_type", Payload: nil}
	err := message.SwitchSDKMessage(msg, message.SDKMessageHandlers{})
	assert.Error(t, err, "알 수 없는 타입은 에러를 반환해야 함")
}
