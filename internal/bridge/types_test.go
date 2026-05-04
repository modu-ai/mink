// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-001, REQ-BR-003
// AC: AC-BR-001, AC-BR-003
// M0-T1, M0-T2 — sanity assertions on enum string values and close code matrix.

package bridge

import "testing"

func TestTransportConstants(t *testing.T) {
	t.Parallel()

	if string(TransportWebSocket) != "websocket" {
		t.Errorf("TransportWebSocket = %q, want \"websocket\"", TransportWebSocket)
	}
	if string(TransportSSE) != "sse" {
		t.Errorf("TransportSSE = %q, want \"sse\"", TransportSSE)
	}
}

func TestSessionStateConstants(t *testing.T) {
	t.Parallel()

	want := map[SessionState]string{
		SessionStateOpen:         "open",
		SessionStateActive:       "active",
		SessionStateIdle:         "idle",
		SessionStateReconnecting: "reconnecting",
		SessionStateClosed:       "closed",
	}
	for got, expect := range want {
		if string(got) != expect {
			t.Errorf("%v = %q, want %q", got, string(got), expect)
		}
	}
}

func TestInboundOutboundTypeConstants(t *testing.T) {
	t.Parallel()

	inbound := map[InboundType]string{
		InboundChat:               "chat",
		InboundAttachment:         "attachment",
		InboundPermissionResponse: "permission_response",
		InboundControl:            "control",
	}
	for got, expect := range inbound {
		if string(got) != expect {
			t.Errorf("inbound %v = %q, want %q", got, string(got), expect)
		}
	}

	outbound := map[OutboundType]string{
		OutboundChunk:             "chunk",
		OutboundNotification:      "notification",
		OutboundPermissionRequest: "permission_request",
		OutboundStatus:            "status",
		OutboundError:             "error",
	}
	for got, expect := range outbound {
		if string(got) != expect {
			t.Errorf("outbound %v = %q, want %q", got, string(got), expect)
		}
	}
}

func TestCloseCodeMatrix(t *testing.T) {
	t.Parallel()

	cases := map[CloseCode]uint16{
		CloseNormal:            1000,
		CloseGoingAway:         1001,
		CloseMessageTooBig:     1009,
		CloseInternalError:     1011,
		CloseUnauthenticated:   4401,
		CloseSessionRevoked:    4403,
		CloseSessionTimeout:    4408,
		ClosePayloadTooLarge:   4413,
		CloseRateLimited:       4429,
		CloseBridgeUnavailable: 4500,
	}
	for code, want := range cases {
		if uint16(code) != want {
			t.Errorf("close code %v = %d, want %d", code, uint16(code), want)
		}
	}
}
